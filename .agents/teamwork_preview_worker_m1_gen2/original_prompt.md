## 2026-05-29T07:49:00Z

<USER_REQUEST>
You are the teamwork_preview_worker (M1_Gen2).
Your role is: GitHub Issues & TODOs Implementer.
Your working directory is: ~/git/striatum/.agents/teamwork_preview_worker_m1_gen2

### Objective:
Implement the required improvements to resolve the tracked GitHub issues and TODOs. You must carefully follow the detailed findings and step-by-step implementation strategy formulated by the Explorer in their handoff report:
`~/git/striatum/.agents/teamwork_preview_explorer_m1_1_gen2/handoff.md`

### Tasks to Execute:
1. **MCP Settings Cleanup (Issue #51)**:
   - In `go/pkg/agentloop/mcpconfig.go`:
     - Modify `writeEphemeralGeminiSettings` to read `STRIATUM_SUPERVISOR_ID` env. If present, write a backup or marker file into `filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)` (backup: `settings.json.backup`, created marker: `settings.json.created`).
     - Implement `CleanupGeminiSettings(repoRoot, supervisorID string)` to locate the scratch directory and restore the pre-existing settings or delete the created settings.
   - Integrate `CleanupGeminiSettings` in:
     - `HandleSuperviseStop` in `supervision_control.go` before returning.
     - `HandleRecoveryProcessReconcile` in `recovery.go` when process is marked lost.
     - `HandleSessionClose` in `lifecycle.go` when closing an active session.

2. **Supervised Exit Terminal Persistence (D146)**:
   - In `go/pkg/reads/supervision.go` (`HandleSuperviseStatus`): If the database state is attached but `live.Alive` returns `false` (i.e. probed gone), update state to `stopped` by calling `updateSupervisorState(ctx, tx, repositoryID, supervisorID, daemonSupervisorID, "stopped", now, 0, "", "", &now, &stopReason)` with `stopReason = "unexpected exit (probed gone)"` inside a transaction.
   - In `go/pkg/mutations/recovery.go` (`HandleRecoveryProcessReconcile`): select `ptr.supervisor_id` and `ptr.daemon_supervisor_id` as well. When process is marked lost due to `!alive`, call `updateSupervisorState` with `stopReason = "unexpected child exit (lost)"` inside a transaction.

3. **Conversation UI Rendering (F43)**:
   - In `go/pkg/webservice/identity.go`: Update allowed route permit list and `PermitIdentityRoute` to permit `/v1/runs/{run_id}/conversations` and `/v1/runs/{run_id}/conversations/{conversation_id}`.
   - In `go/pkg/webservice/service.go`: Update REST router endpoint cases under `routeRunGET` to handle `"conversations"`. Implement `showConversation` and `renderConversationPage`.
   - In `go/pkg/webassets/assets.go`: Implement `RenderConversation` template rendering. Create and map the new `templates/conversation.html` file using safe context-aware Go `html/template` blocks mapping speaker and body turns.

### Verification Requirement:
- Run `go test -race ./...` to compile and pass the entire Go test suite.
- Ensure zero lint/typecheck errors.

### MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

### Completion Criteria:
- Code changes successfully integrated in Go codebase.
- Entire Go test suite compiles and passes cleanly with zero race conditions.
- Call send_message to report completion back to the Project Orchestrator (Gen 2).

</USER_REQUEST>
