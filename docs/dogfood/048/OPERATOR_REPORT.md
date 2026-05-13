# Dogfood-048 Operator Report

**Run ID:** _TBD_
**Branch:** `striatum/dogfood-048-rfc-0043-v1`
**Workflow:** 10-job two-track for RFC 0043 V1 (Postgres as sole substrate + daemon-required runtime).
**Operator:** _TBD_
**Started:** _TBD_

## Scope

RFC 0043 V1 — the substrate flip per D094 (supersedes D006/D007/D036 and the SQLite half of D009).

- **Track A (codex)**: daemon-side Postgres schema for the 15 repo-local tables (`runs`, `sessions`, `jobs`, `queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`, `blockers`, `command_requests`, `process_executions`, `events`, `job_worktrees`, `process_supervisors`, `process_supervisor_pointers`) under `repository_id` namespace + new CLI verb `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--dry-run] [--keep-sqlite-readonly] [--confirm-delete]` with SERIALIZABLE single-tx migrate + audit-chain byte-equivalent re-anchor.
- **Track B (claude)**: retire `--no-daemon` flag (unknown-option error); wire exit code 11 (`daemon_unreachable`) + 12 (`repo_not_migrated`) with platform-specific remediation; expand RFC 0030 method registry to cover every mutation in `src/striatum/cli/mutations.py` per RFC 0043 §5.

Out of scope: RFC 0039 (Go daemon) scope-delta revision (separate follow-up); bundled Postgres distribution; multi-tenancy enforcement; hosted-mode auth; rewriting historical dogfood scaffolds. README / TODO / CHANGELOG / SPEC / HOW_TO updates are operator-only after the dogfood lands.

## Interventions

### Intervention 1: Kickoff
- _TBD_ — scaffold committed, run prepared+started, 3 designer sessions (codex/claude/gemini) registered with `--fresh`, supervisors attached, claim-next per session triggered packet delivery.
- Codex session: _sess_id_
- Claude session: _sess_id_
- Gemini session: _sess_id_

## Run Outcome

_TBD_

## Anti-patterns observed

_TBD_

## Follow-ups

_TBD_
