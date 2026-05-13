# Dogfood-048 Operator Report

**Run ID:** `run_892cbad2b1954cfd9d23e72f74ea3a96`

## Interventions

### Intervention 1: Kickoff
- 3 designers registered: codex sess_7554bf4b9e5449789a174b0b1f20411e, claude sess_413cae89880048caba1cf1666100881a, gemini sess_8d08b40fa53b492097fdc06dc38185d6. Supervisors + claim-next.

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

### Intervention 2: Design publish-on-behalf
- codex completed naturally. claude+gemini stuck. publish-on-behalf with conformant bylines.

### Intervention 3: Synth + design review natural
- Both via supervisor flow.

### Intervention 4: 2 impls parallel
- Track A codex (schema+migration) completed naturally; shipped 17-table migration SQL + migration handler.
- Track B claude (CLI+RPC) stuck claimed; HANDOFF written but operator publish-on-behalf needed. Wrong logical_name on publish first attempt — required SQL surgery on artifacts table (drop append-only trigger, UPDATE logical_name, restore trigger). Lesson: always check `expected_artifacts[].logical_name` from workflow.json before publish-on-behalf.

### Intervention 5: Build review + D102 double override
- codex needs_revision (high, real findings)
- claude no-artifact (3rd instance) — operator-composed accept_with_findings
- gemini no-frontmatter (3rd instance) — operator-fixed
- D102 recorded. Override codex+gemini to accept_with_findings. Track A attempt-2 canceled (Track B had no a2 to cancel).
- Real findings (crash recovery, CLI escape path, migrate-repo-local subcommand wiring, test gaps) folded into RFC 0043 V1.5 follow-up.

## Run Outcome

- Run state `completed`. 10 jobs done, 2 canceled.
- v1.37.0: RFC 0043 V1 — Postgres-sole-substrate + daemon-required executable.
- Anti-pattern frequency: claude-no-artifact (3rd), gemini-no-frontmatter (3rd), codex-reviewer-of-claude-implementer (3rd? distinct from codex/codex at 5). Need harness fixes.
