# Handoff Report: GitHub Issues & TODOs Implementation

## 1. Observation

All requested tasks were successfully implemented, tested, and verified against the Go test suite:

### Task 1: MCP Settings Cleanup
*   **Settings File Creation & Teardown**: Updated `writeEphemeralGeminiSettings` in `go/pkg/agentloop/mcpconfig.go` to support `STRIATUM_SUPERVISOR_ID` env. If present, it writes a backup of pre-existing settings (`settings.json.backup`) or a created marker (`settings.json.created`) into `filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)`.
*   **Cleanup Implementation**: Implemented `CleanupGeminiSettings(repoRoot, supervisorID string)` in `go/pkg/agentloop/mcpconfig.go` to locate the scratch directory and cleanly restore pre-existing settings or delete the created settings.
*   **Teardown Integration**: Integrated `CleanupGeminiSettings` in:
    *   `HandleSuperviseStop` in `go/pkg/mutations/supervision_control.go` (line 473) inside the transaction before returning.
    *   `HandleRecoveryProcessReconcile` in `go/pkg/mutations/recovery.go` (line 144) when a process is marked lost.
    *   `HandleCloseSession` in `go/pkg/mutations/lifecycle.go` (line 263) when closing an active session, scanning for and cleaning up all supervisor IDs associated with the closed session.

### Task 2: Supervised Exit Terminal Persistence
*   **Read Layer Status Projections**: In `go/pkg/reads/supervision.go` (`HandleSuperviseStatus`), when a supervisor is probed gone (`!live.Alive`), and the database state is attached (`state == "attached"`), it now starts a transaction and updates the supervisor state to `stopped` using a local copy of `updateSupervisorState` with `stopReason = "unexpected exit (probed gone)"`.
*   **Recovery Reconcile Mutation**: In `go/pkg/mutations/recovery.go` (`HandleRecoveryProcessReconcile`), updated the lateral query to select `ptr.supervisor_id` and `ptr.daemon_supervisor_id` and invoked `updateSupervisorState` with `stopReason = "unexpected child exit (lost)"` inside the transaction when the process is marked lost due to `!alive`.

### Task 3: Conversation UI Rendering
*   **Tailnet Identity Route Permit**: In `go/pkg/webservice/identity.go`, updated allowed route permit list and `PermitIdentityRoute` to permit `/v1/runs/{run_id}/conversations` and `/v1/runs/{run_id}/conversations/{conversation_id}`.
*   **REST Routing**: In `go/pkg/webservice/service.go`, updated REST router endpoint cases under `routeRunGET` to handle `"conversations"`. Implemented `showConversation` and `renderConversationPage`.
*   **Template Rendering**: In `go/pkg/webassets/assets.go`, implemented `RenderConversation` template rendering. Created the HTML template file `go/pkg/webassets/templates/conversation.html` using context-aware Go `html/template` blocks mapping speaker and body turns.

### Pre-Existing Data Race Fix
*   **Map Data Race**: During `-race` test suite verification, detected a pre-existing map data race in `go/pkg/supervisor/supervisor_test.go` where `TestLivenessDegradesBeforeLostOnPersistentTmuxUnavailable` was concurrently reading `row.Metadata` while `liveness.run` wrote to it.
*   **Fix**: Modified the `fakeStore.GetSupervisorPointer` helper inside `go/pkg/supervisor/supervisor_test.go` (line 46) to deep copy `row.Metadata` before returning, fully solving the data race.

---

## 2. Logic Chain

1. **MCP Settings Cleanup**: Ephemeral settings creation is tied to the supervised child's lifecycle. However, since the child can stop abnormally, the daemon needs a persistent marker in the supervisor scratch directory `.striatum/scratch/<supervisor_id>`. By writing a backup/marker there, the daemon can asynchronously clean up or restore the `.gemini/settings.json` file inside `HandleSuperviseStop`, `HandleRecoveryProcessReconcile`, and `HandleCloseSession` using `CleanupGeminiSettings`.
2. **Supervised Exit Terminal Persistence**: Status probes and recovery runs are the core mechanisms that detect unexpectedly terminated supervised processes. By ensuring both `HandleSuperviseStatus` and `HandleRecoveryProcessReconcile` transition the Postgres state of defunct supervisors to `stopped` via `updateSupervisorState`, we align the live process liveness projection with durable database state, solving state-desync issues.
3. **Conversation UI Rendering**: The conversations endpoints require REST-level parity with interrogations. By mapping GET `/v1/runs/{run_id}/conversations` and `/v1/runs/{run_id}/conversations/{conversation_id}` in `webservice`, auditing them in `identity`, and rendering them server-side via `webassets` (with context-aware HTML template escaping), we safely expose and display multi-party conversation histories to operators.
4. **Data Race Fix**: Go map variables are passed by reference. Deep-copying `row.Metadata` inside the test `fakeStore` ensures concurrent status inspections do not access the same map instance being mutated by the supervisor liveness background thread.

---

## 3. Caveats

*   **Repository Access**: Assumes that the daemon running has proper OS write permission to `.gemini/settings.json` under the target repository's path.
*   No other caveats are identified.

---

## 4. Conclusion

All 3 tasks have been natively integrated into the Go codebase following the minimal-change principle. The entire test suite compiles and runs cleanly with race detection enabled (`go test -race ./...`), and all style, formatting, and lint requirements are completely satisfied.

---

## 5. Verification Method

To independently verify the changes, execute the following commands in `~/git/striatum/go`:

1.  **Run All Tests**:
    ```bash
    go test -race ./...
    ```
    Confirm that the entire test suite passes cleanly with zero race conditions or errors.
2.  **Run Vet & Lint Checks**:
    ```bash
    go vet ./...
    ```
    Verify that static analysis passes with zero warnings or errors.
3.  **Inspect Modified Files**:
    *   `go/pkg/agentloop/mcpconfig.go` (gemini settings backup/marker and CleanupGeminiSettings logic)
    *   `go/pkg/mutations/supervision_control.go` (teardown settings cleanup)
    *   `go/pkg/mutations/recovery.go` (recovery state transitions and settings cleanup)
    *   `go/pkg/mutations/lifecycle.go` (session close settings cleanup)
    *   `go/pkg/reads/supervision.go` (gone supervisor status transition to stopped)
    *   `go/pkg/webservice/service.go` & `go/pkg/webservice/identity.go` (conversation route permit and mapping)
    *   `go/pkg/webassets/assets.go` & `go/pkg/webassets/templates/conversation.html` (safe conversation chat view rendering)
