# BRIEFING — 2026-05-29T07:48:00Z

## Mission
Research the codebase to plan the implementation of RFC 0090 (Workspace Security & Attestation Parity) addressing 6 specific hardening dimensions.

## 🔒 My Identity
- Archetype: Workspace Security & Attestation Hardening Analyst
- Roles: Security Researcher, Codebase Analyst
- Working directory: ~/git/striatum/.agents/teamwork_preview_explorer_m1_2_gen2
- Original parent: 4e1e5f9b-1c93-49d1-a75b-81fc57deb5ff
- Milestone: RFC 0090 Planning

## 🔒 Key Constraints
- Read-only investigation — do NOT implement
- Do NOT write or modify any source code files
- Do NOT run build/test commands
- Focus strictly on exploration, analysis, finding relevant code segments, and proposing implementation strategy

## Current Parent
- Conversation ID: 4e1e5f9b-1c93-49d1-a75b-81fc57deb5ff
- Updated: 2026-05-29T07:48:00Z

## Investigation State
- **Explored paths**:
  - `go/pkg/mutations/mutations.go` (lines 1-1044)
  - `go/pkg/mutations/artifact.go` (lines 1-450)
  - `go/pkg/mutations/write_scope_guard.go` (lines 1-200)
  - `go/pkg/db/migrations.go` (lines 1-200)
  - `go/pkg/db/connection.go` (lines 1-259)
  - `go/pkg/mutations/supervision_control.go` (lines 1150-1280)
  - `go/pkg/pgtest/pgtest.go` (lines 1-100)
  - `go/pkg/supervisor/liveness.go` (lines 1-279)
  - `go/pkg/supervisor/start_time_darwin.go` (lines 1-41)
  - `go/cmd/striatumd/main.go` (lines 95-150, 320-370, 400-450, 690-723)
  - `go/pkg/admin/bootstrap.go` (lines 1-100)
- **Key findings**:
  - Path-Jail breakout vulnerability: `repoRelativePath` resolves paths lexically without evaluating symlinks, leaving the system open to breakout via directory symlinks.
  - Migration lock key: Hardcoded `MigrationLockKey = 332933` in `migrations.go` can cause lock collisions; it is best resolved by a dynamic hash of DB name + schema name.
  - Named-pipe resilience: Linux ENXIO errors occur when child processes aren't actively reading. This is easily handled with a thread-safe memory ring-buffer.
  - DB privilege verification: `pgtest` lacks unprivileged testing structures, preventing verification of revoking DELETE/UPDATE permissions on `events` and `artifacts` tables.
  - Darwin attestation: Process liveness start-time attestation currently shells out to `/bin/ps`, which is slow and insecure; it can be replaced with direct kernel `sysctl` queries.
  - Port discovery: MCP binds to dynamic loopback `127.0.0.1:0` but needs a secure, permission-guarded `discovery.json` file for automatic client discovery.
- **Unexplored areas**: none, all 6 dimensions are fully researched and mapped.

## Key Decisions Made
- Design secure `ValidateSandboxJail` employing `filepath.EvalSymlinks` + `sameOrInside` checks.
- Establish `deriveMigrationLockKey` using SHA-256 of `current_database()` + `:striatumd` mapped to `int64`.
- Propose thread-safe `NamedPipeBuffer` ring-buffer to gracefully buffer named pipe writes during `ENXIO` errors.
- Propose `UnprivilegedPool` helper in `pgtest.go` to test database revoking logic.
- Replace macOS `/bin/ps` invocation with direct kernel MIB query via `sysctl`.
- Write atomic, owner-only (`0o600`) `discovery.json` containing endpoints, port, pid, and client token.

## Artifact Index
- ~/git/striatum/.agents/teamwork_preview_explorer_m1_2_gen2/original_prompt.md — Original prompt
- ~/git/striatum/.agents/teamwork_preview_explorer_m1_2_gen2/progress.md — Liveness progress
- ~/git/striatum/.agents/teamwork_preview_explorer_m1_2_gen2/handoff.md — Analysis and Handoff Report
