# System Context

## Environment
- OS: Linux
- Working Directory: `~/git/striatum`
- App Data Directory: `~/.gemini/antigravity-cli`
- Mode: CODE_ONLY (No external network access)

## Previous Generation Verdict
- Previous orchestrator (`orchestrator_gen2`) completed all tasks for Follow-up 2026-05-29T07:45:46Z.
- All RFC 0090 (security/sandbox) and RFC 0091 (lanehealth module) implementations were successfully completed and audited with a CLEAN verdict.
- Current codebase state compiles cleanly, with all unit tests passing (`go test -race ./...`).

## Current Objective
Triage and resolve six new outstanding issues (#49, #54, #57, #58, #59, #60) from Follow-up 2026-05-29T12:00:25Z:
1. Issue #57 (Write-Scope Strictness)
2. Issue #58 (Duplicate Artifact Publication in `submit-review`)
3. Issue #59 (Strict Front-Matter List Formatting)
4. Issue #60 (Rigid Session Lifetime Enforcement)
5. Issues #49 & #54 (PTY Supervision, Rebridge, & Re-queueing)
