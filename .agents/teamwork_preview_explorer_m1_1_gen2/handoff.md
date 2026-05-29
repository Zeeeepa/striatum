# Handoff Report: GitHub Issues & TODOs Research

## 1. Observation

This read-only investigation analyzed the Striatum codebase under `go/` to address three tracked issues:

### Issue 1: MCP Settings Cleanup (Issue #51)
*   **Settings File Creation & Teardown**: Inside `go/pkg/agentloop/mcpconfig.go` (lines 80-123), `writeEphemeralGeminiSettings` creates the `.gemini/settings.json` file and returns a `cleanup` function:
    ```go
    // lines 115-121
    cleanup := func() {
        if hadExisting {
            _ = os.WriteFile(path, backup, 0o600)
            return
        }
        _ = os.Remove(path)
    }
    ```
*   **Startup & Environment**: In `go/pkg/agentloop/loop.go` (lines 202-206), `runWithIO` defer-calls `cleanupMCP()`. However, if the process receives a termination signal (`SIGKILL`, `SIGTERM`), or stops unexpectedly, this `cleanup` closure is bypassed.
*   **Supervisor Env**: In `go/pkg/mutations/supervision_control.go` (lines 2139-2149), `supervisedEnvEntries` injects the `STRIATUM_SUPERVISOR_ID` environment variable into the supervised agent loop process:
    ```go
    "STRIATUM_SUPERVISOR_ID=" + supervisorID,
    ```
*   **Supervisor Scratch**: The scratch directory `.striatum/scratch/<supervisor_id>` is guaranteed to be created and writable at supervisor start (`supervision_control.go`, lines 128-133).

### Issue 2: Supervised Exit Terminal Persistence (D146)
*   **Supervisor State Read**: In `go/pkg/reads/supervision.go` (lines 125-148), `HandleSuperviseStatus` checks process liveness dynamically using `gosupervisor.ProbeLaneLiveness`, but it does not update the database state, leaving the database state as `attached`:
    ```go
    // lines 125-126
    if hasPID && supervisorActiveStatesRead[state] {
        live := gosupervisor.ProbeLaneLiveness(ctx, superviseTmuxRunner, pointerMetadata, pid, superviseString(supervisor["pid_start_time"]))
    ```
*   **Recovery Reconcile Handler**: In `go/pkg/mutations/recovery.go` (lines 127-139), `HandleRecoveryProcessReconcile` identifies a process as `lost` and writes a `process.lost` message to the bus, but it does NOT update the `striatumd.process_supervisor_pointers` state to `stopped`:
    ```go
    // lines 127-130
    if err := tx.Exec(ctx, `
        UPDATE striatumd.process_executions
           SET state = 'lost', ended_at = $1
         WHERE repository_id = $2 AND process_id = $3`, now, repositoryID, processID); err != nil {
    ```
*   **DB State Updating Helper**: `go/pkg/mutations/supervision_control.go` contains `updateSupervisorState` (lines 1790-1835) which is a fully general-purpose database transaction helper for changing state to `stopped`:
    ```go
    func updateSupervisorState(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID, daemonSupervisorID, state, updatedAt string, pid int, pidStartTime, heartbeatAt string, endedAt *string, stopReason *string) error {
    ```

### Issue 3: Conversation UI Rendering (F43)
*   **REST API Routing**: In `go/pkg/webservice/service.go` (lines 125-163), `routeRunGET` parses REST path parameters but completely lacks support for `conversations`:
    ```go
    switch parts[1] {
    case "why":
        ...
    case "dashboard":
        ...
    case "interrogations":
        ...
    ```
*   **Tailnet Identity Route Allowed Filter**: In `go/pkg/webservice/identity.go` (lines 78-89), `PermitIdentityRoute` enforces security permissions for `tailscale serve` read-only views, but lacks `conversations`:
    ```go
    switch len(parts) {
    case 1:
        return true
    case 2:
        return parts[1] == "interrogations"
    case 3:
        return parts[1] == "interrogations" && parts[2] != ""
    }
    ```
*   **Conversation Query RPCs**: `go/pkg/mutations/conversation.go` already implements the symmetric RPC query endpoints `conversation.list` (lines 370-405) and `conversation.show` (lines 330-368).
*   **HTML Template Renderer**: `go/pkg/webassets/assets.go` contains `RenderInterrogation` (lines 89-120) which context-escapes using `html/template` (loaded from `templates/interrogation.html`).

---

## 2. Logic Chain

1. **MCP Settings Cleanup**: Since the `agent-loop` runs under the `STRIATUM_SUPERVISOR_ID` env, it has access to a dedicated scratch directory `.striatum/scratch/<supervisor_id>`. By writing a backup/marker into this scratch folder, the daemon (which coordinates supervisor termination, recovery, and session closure) can look up the scratch folder and restore/remove the settings file regardless of child process signal termination or unexpected crashes.
2. **Supervised Exit Persistence**: Currently, when a supervised process dies unexpectedly, both `supervise.status` and `recovery.process_reconcile` detect that the process is dead (`live.Alive == false`). However, they do not update the DB state. By invoking `updateSupervisorState` in both locations during exit/lost detection, we transition the database states durably to `stopped` in Postgres, resolving the read-only projection disparity.
3. **Conversation UI Rendering**: We can cleanly expose `/v1/runs/{runID}/conversations[/{id}]` by mapping the REST paths in `service.go` to the existing `conversation.list` and `conversation.show` RPC methods (matching IDOR protection logic from interrogations). By adding a server-side HTML rendering function (`RenderConversation`) and template `templates/conversation.html` using Go's context-aware `html/template` package, we guarantee safe read-only rendering of multi-party conversations.

---

## 3. Caveats

*   **Repository Access**: It is assumed that the daemon runs on the same host and has file access permission to `.gemini/settings.json` under the target repository's path.
*   **IDOR Security**: In F43, we explicitly check that the path `runID` matches the database row's `run_id` to prevent cross-run IDOR attacks.
*   No other caveats are identified.

---

## 4. Conclusion

All 3 issues have grounded, unambiguous code paths and concrete solutions that fit natively into Striatum's architecture without database schema adjustments or structural disruption.

---

## 5. Recommended Implementation Strategy

### Issue 1: MCP Settings Cleanup (#51)
1.  **Modify `writeEphemeralGeminiSettings`** in `go/pkg/agentloop/mcpconfig.go`:
    *   Read `STRIATUM_SUPERVISOR_ID` env variable.
    *   If present, write a backup or marker file into `filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)`:
        *   If `hadExisting`: write the backup bytes to `settings.json.backup`.
        *   If `!hadExisting`: write an empty file `settings.json.created`.
2.  **Implement `CleanupGeminiSettings(repoRoot, supervisorID string)`** in `go/pkg/agentloop/mcpconfig.go`:
    *   Locate scratch directory: `scratchDir := filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)`.
    *   If `settings.json.backup` exists: read it, restore to `<repoRoot>/.gemini/settings.json`, and delete backup file.
    *   If `settings.json.created` exists: remove `<repoRoot>/.gemini/settings.json`, and delete marker file.
3.  **Call `CleanupGeminiSettings`** in:
    *   `HandleSuperviseStop` in `supervision_control.go` before returning.
    *   `HandleRecoveryProcessReconcile` in `recovery.go` when marking a process as lost.
    *   `HandleSessionClose` in `lifecycle.go` when closing an active session.

### Issue 2: Supervised Exit Persistence (D146)
1.  **Modify `HandleSuperviseStatus`** in `go/pkg/reads/supervision.go`:
    *   If the database state is active (`attached` or similar) and `live.Alive` returns `false` (i.e., probed gone):
    *   Start a transaction and call `updateSupervisorState(ctx, tx, repositoryID, supervisorID, daemonSupervisorID, "stopped", now, 0, "", "", &now, &stopReason)` with `stopReason = "unexpected exit (probed gone)"`.
2.  **Modify `HandleRecoveryProcessReconcile`** in `go/pkg/mutations/recovery.go`:
    *   In the primary `SELECT` query, select `ptr.supervisor_id` and `ptr.daemon_supervisor_id` as well.
    *   When process is marked `lost` due to `!alive`:
    *   If `supervisorID != ""`, call `updateSupervisorState(ctx, tx, repositoryID, supervisorID, daemonSupervisorID, "stopped", now, 0, "", "", &now, &stopReason)` with `stopReason = "unexpected child exit (lost)"`.

### Issue 3: Conversation UI Rendering (F43)
1.  **Update Route Permit list** in `go/pkg/webservice/identity.go`:
    *   Add `/v1/runs/{run_id}/conversations` and `/v1/runs/{run_id}/conversations/{conversation_id}` to `IdentityReadRoutes`.
    *   In `PermitIdentityRoute`'s `strings.HasPrefix(clean, "/v1/runs/")` case switch:
        ```go
        case 2:
            return parts[1] == "interrogations" || parts[1] == "conversations"
        case 3:
            return (parts[1] == "interrogations" || parts[1] == "conversations") && parts[2] != ""
        ```
2.  **Add Endpoint case** to `routeRunGET` in `go/pkg/webservice/service.go`:
    *   Under `case "conversations"`:
        *   If `len(parts) >= 3 && parts[2] != ""`, call `h.showConversation(w, r, runID, parts[2])`.
        *   Otherwise, call `h.callAndWrite(w, r.Context(), "conversation.list", map[string]any{"run_id": runID})`.
3.  **Implement `showConversation` and `renderConversationPage`** in `service.go`:
    *   `showConversation` queries `conversation.show` using `h.call`.
    *   Asserts `conversation.run_id == runID` (protects against IDOR).
    *   If query parameter `view == "chat"`, call `h.renderConversationPage(w, data, conversation)`.
    *   Otherwise, return JSON response.
4.  **Implement `RenderConversation`** in `go/pkg/webassets/assets.go` and add the HTML template `templates/conversation.html` using context-aware Go `html/template` rendering turns withcorrect `Speaker` (`author_session_id`) and `Body`.

---

## 6. Verification Method

To verify these changes independently after implementation:
1.  **Issue 1 settings cleanup**:
    *   Start an `agy` supervised lane session. Verify `.gemini/settings.json` is created.
    *   Expose or simulate a crash of the supervised child, or run `striatum supervise stop`.
    *   Verify `.gemini/settings.json` is cleanly deleted (or restored if pre-existing).
2.  **Issue 2 exit persistence**:
    *   Start a supervised lane, wait for it to be `attached`, then externally kill its child process via standard OS `kill -9 <PID>`.
    *   Query the supervisor status via `striatum supervise status` or run `striatum recovery auto`.
    *   Verify the supervisor row state in Postgres changes durably to `'stopped'`.
3.  **Issue 3 conversation UI rendering**:
    *   Start a 3-way conversation, then invoke `GET /v1/runs/{runID}/conversations/{id}?view=chat` using a curl client or browser.
    *   Confirm it displays the multi-party conversation thread and all speaker turns successfully.
4.  **Run All Tests**:
    *   Run `make test` and `make lint` to assert authority guards, web routing allowed filters, and types match.
