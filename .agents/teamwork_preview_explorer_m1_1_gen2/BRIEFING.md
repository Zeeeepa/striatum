# BRIEFING — 2026-05-29T07:46:58Z

## Mission
Research the codebase to locate files, functions, and lines relevant to the three tracked issues (MCP settings cleanup, supervised exit persistence, conversation UI rendering) and draft a recommended implementation strategy.

## 🔒 My Identity
- Archetype: GitHub Issues & TODOs Researcher
- Roles: GitHub Issues & TODOs Researcher
- Working directory: ~/git/striatum/.agents/teamwork_preview_explorer_m1_1_gen2
- Original parent: 4e1e5f9b-1c93-49d1-a75b-81fc57deb5ff
- Milestone: M1_1_Gen2

## 🔒 Key Constraints
- Read-only investigation — do NOT implement
- Do NOT run build/test commands
- Focus strictly on exploration, analysis, finding relevant code segments, and proposing implementation strategy

## Current Parent
- Conversation ID: 4e1e5f9b-1c93-49d1-a75b-81fc57deb5ff
- Updated: 2026-05-29T07:48:35Z

## Investigation State
- **Explored paths**:
  - `go/pkg/agentloop/mcpconfig.go` — settings creation and cleanup structure
  - `go/pkg/agentloop/loop.go` — PTY agent loop startup and deferred cleanup execution
  - `go/pkg/mutations/supervision_control.go` — supervisor starting/stopping handlers and env variables
  - `go/pkg/mutations/recovery.go` — recovery sweep process reconciliation handlers
  - `go/pkg/reads/supervision.go` — supervise status liveness checking
  - `go/pkg/webservice/service.go` — web service REST routing and handlers
  - `go/pkg/webservice/identity.go` — identity routing and read allowed filters
  - `go/pkg/webassets/assets.go` — interrogation HTML rendering
  - `go/pkg/webassets/templates/interrogation.html` — interrogation layout
- **Key findings**:
  - MCP settings file `.gemini/settings.json` can be cleanly recovered or deleted by utilizing a scratch backup/marker folder structure at `.striatum/scratch/<supervisor_id>`.
  - Probed liveness failures in `HandleSuperviseStatus` and lost processes in `HandleRecoveryProcessReconcile` currently keep database state as `attached` dynamically; they must execute `updateSupervisorState` to commit `stopped` status durably in Postgres.
  - REST routes for conversations can be seamlessly added by registering under `IdentityReadRoutes` / `PermitIdentityRoute` in `identity.go` and `routeRunGET` in `service.go`, querying existing RPC methods `conversation.list`/`show`, and rendering using `html/template` context-aware auto-escaping.
- **Unexplored areas**: None.

## Key Decisions Made
- Chose an elegant scratch-directory based backup and marker file approach for Issue #51 settings cleanup to ensure clean deletion/restoration without adding database schema clutter.
- Grounded D146 exit persistence directly in the existing `updateSupervisorState` DB helper.
- Fully mapped REST endpoints and HTML template rendering paths for F43, including the Tailscale identity filter.

## Artifact Index
- ~/git/striatum/.agents/teamwork_preview_explorer_m1_1_gen2/handoff.md — Handoff report containing research findings and implementation strategy.
