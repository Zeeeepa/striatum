---
schema_version: "striatum.operator_report.v1"
artifact_kind: "operator_report"
---

author: operator

# dogfood-057: RFC 0048 Phase A — daemon RPC handler port (operator report)

## Header

- Workflow: `docs/dogfood/057/workflow.json` (10 jobs, dual-track impl, max parallel: 6).
- RFC: 0048 Phase A — port 16 single-repo handlers from SQLite-backed to PG-backed under `src/striatum/daemon_pg/handlers/`, swap `DaemonRpcRouter._route` delegation per method.
- Branch: `striatum/dogfood-057-rfc-0048-daemon-rpc-substrate` (cut from `striatum/gh-issues-parallel` HEAD; preserves GH-issues parallel context).
- Operator session: claude-opus-4-7 driving via Claude Code.
- Operating mode: **legacy SQLite via `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`**. Recorded as break-glass because daemon-required CLI is non-functional on the current branch (the gap RFC 0048 itself addresses).

## Pre-flight (2026-05-14)

Setup work landed before run prepare:

- CLI: 1.48.1 → 1.48.2 (force-reinstall editable; `striatum-orchestrator[daemon-pg]` extra to pull psycopg).
- Plugin (claude_code profile) + skills reinstalled at v1.48.2.
- Postgres 16.13 reachable; new database `striatum_daemon` owned by `halbritt`.
- New role `striatumd_rw` with login + password, append-only-correct privileges (`REVOKE UPDATE, DELETE ON striatumd.audit_log, .events, .artifacts`).
- `~/.config/striatum/daemon.toml` (0600) with the role URL.
- `striatum daemon doctor --apply-migrations` clean; schema at version 5.
- `striatum daemon migrate-repo-local --dry-run` showed 73 runs / 5365 events / 548 jobs / 508 artifacts in the repo-local SQLite. Full migration ran, then was **rolled back** — see below.

## Architecture friction observed during pre-flight

1. **POSTGRES_TRANSITION.md missing role-provisioning runbook.** A fresh local install where the DB owner is the connecting role trips the `unsafe_privileges` doctor check (owner has implicit UPDATE/DELETE on `striatumd.audit_log`). Worked around by provisioning `striatumd_rw` manually with explicit REVOKEs.
2. **`daemon migrate-repo-local` requires CREATE on db + schema** even when migrations are at HEAD (it runs `CREATE TABLE IF NOT EXISTS` regardless). Worked around by `GRANT CREATE ON DATABASE`, `GRANT CREATE ON SCHEMA`. Should be a no-op skip when current.
3. **Daemon RPC accept loop is missing.** `run_daemon_foreground` in `src/striatum/daemon.py` binds a Unix socket and runs sweeps but never `accept()`s incoming connections. Confirmed via `ss -lx` (LISTEN with stuck `Recv-Q`). Daemon-required CLI verbs therefore fail with exit 11 even though the daemon process is up. This is **out of RFC 0048 scope** (separate transport-layer concern) but it's what makes the operator break-glass mandatory for this run.
4. **Migration rollback.** Because daemon-required CLI is non-functional and the substrate facade is the RFC 0048 subject matter, I reverted `state.sqlite3.tombstone` back to `state.sqlite3` and kept the populated Postgres `striatum_daemon` DB in place for when Phase A handlers land. `_route` delegation swap will switch handlers to PG one by one; SQLite remains as fallback for un-ported methods.

The three friction items above need a follow-on GH issue / TODO entry after the dogfood lands.

## Run state (append below per intervention)

- 2026-05-14 17:00: Scaffold created. Branch cut. Workflow + prompts + roles + this report written.
- 2026-05-14 17:09: Workflow validated. Initial validation caught: (a) missing `cycles` field — added 7 revision cycles modeled on dogfood-048; (b) overlapping write scope between Track A + Track B — split into per-track sub-directories (`workflow_loop/` + `recovery_evidence/`). Updated implement prompts to call out the new boundary and the synthesis-locked registration pattern (Track A owns server.py + registry.py + handlers/__init__.py; Track B handlers integrate via decorator self-registration by default).
- 2026-05-14 17:10: Run prepared (run_id `run_f6c7076a63484f9daedd9ef3f0850130`) and started. State `running`.
- 2026-05-14 17:10-11: Registered 3 designer sessions (codex `sess_e638…`, claude_code `sess_45d5…`, gemini `sess_c7b2…`). Started 3 attached supervisors (sup_06ef, sup_5c6f, sup_9cd9). Claimed design_codex / design_claude / design_gemini packets — all 3 dispatched to wrappers; acks recorded.
- 2026-05-14 17:11: pstree confirms all 3 designer agents are spinning (codex 21 threads, claude 18 threads, gemini multi-thread). Waiting on `DESIGN.md` artifacts under `docs/dogfood/057/design/<lane>/`.
- 2026-05-14 17:13-17:15: Design phase complete. Sizes: claude 703 lines, codex 617 lines, gemini 109 lines. Gemini's design proposed flat `handlers/{mutations,recovery,evidence}.py` layout rather than the per-track sub-dirs the workflow write-scope enforces — synth will reconcile.
- 2026-05-14 17:16: Synth packet dispatched to codex supervisor (reusing existing designer-codex session — synth is not fresh-session-required). Lease expires 17:46Z. Expected: `docs/dogfood/057/DESIGN_SYNTHESIS.md`.
- 2026-05-14 ~17:25: DESIGN_SYNTHESIS.md landed (21KB), synth job completed.
- 2026-05-14 ~17:26: Fresh claude_code reviewer session registered for review_design. Supervisor sup_61a121, PID 1036247. Claimed; REVIEW.md landed shortly after.
- 2026-05-14 ~17:30: review_design verdict `accept_with_findings` (low severity) — all 7 mandatory checks pass, 3 ergonomics_dx degradations recorded as follow-ups, no cycle.
- 2026-05-14 ~17:31: Fresh codex+claude_code implementer sessions registered. Supervisors PID 1044181 (Track A codex) + PID 1044233 (Track B claude). Both implement packets claimed in parallel. Expected: `docs/dogfood/057/build/track_a/HANDOFF.md` (Track A — 9 workflow-loop handlers) + `docs/dogfood/057/build/track_b/HANDOFF.md` (Track B — 7 recovery/evidence handlers).
- 2026-05-14 ~17:55: Implement phase complete. Track A HANDOFF 129 lines (7.6KB), Track B HANDOFF 330 lines (19KB). All 16 handler files + 16 test files landed under `src/striatum/daemon_pg/handlers/{workflow_loop,recovery_evidence}/` and `tests/daemon_pg/handlers/{workflow_loop,recovery_evidence}/`. Plus shared infra: `handlers/__init__.py`, `handlers/registry.py`, `handlers/context.py`.
- 2026-05-14 ~17:56: 3 fresh build-reviewer sessions registered (codex/claude_code/gemini). Supervisors PIDs 1076069 / 1076197 / 1076230. All three review_build packets claimed in parallel — codex `threat_model`, claude `ergonomics_dx`, gemini adversarial `threat_model`.
- 2026-05-14 ~17:48: Build review verdicts in:
  - codex: **reject** (substantive — F1 PG handler error → silent SQLite fallback split-brain risk; F2 no capability-denial tests before PG writes; etc).
  - claude: `needs_revision` (substantive ergonomics_dx — substrate-port affordances + regression guard gaps). Auto-cycled `implement_track_a_codex` to attempt 2; `implement_track_b_claude` was NOT auto-cycled (runtime only fired one cycle target).
  - gemini: body says `ACCEPTED WITH FINDINGS` (no YAML frontmatter — runtime still recorded the verdict).
- 2026-05-14 ~17:49: Run transitioned to `failed` due to codex `reject` (terminal). `striatum run resume` refused with exit 4 → `use retry_job to revive`.
- 2026-05-14 17:51: Operator intervention — `striatum run retry-job --job-id <review_build_codex>` revived the run to `running` (`run_revived: true`). Codex review re-queued; will be claimed AFTER implement_track_a attempt 2 lands.
- 2026-05-14 17:52: `retry-job` on `implement_track_b_claude` refused (state `completed`, not retriable). Track A's write scope owns `server.py`+`registry.py`+`handlers/__init__.py` — the natural home for fail-closed routing + capability-token enforcement. F1+F2 are structurally addressable in Track A attempt 2 alone.
- 2026-05-14 17:52: Fresh codex implementer-2 session (sess_45c7df…, supervisor PID 1090911) claimed implement_track_a attempt 2 (lease `lease_fa97…`). Awaiting revised HANDOFF.

### Friction (parking lot — file as issues post-landing)

1. POSTGRES_TRANSITION.md missing role-provisioning runbook.
2. `daemon migrate-repo-local` over-requires CREATE on db + schema.
3. Daemon RPC accept loop missing (`run_daemon_foreground`) — out of RFC 0048 V1 scope but mandates the operator break-glass.
4. Workflow validation requires `cycles` field even on a simple flow — error message says "missing required fields: cycles" but a starter `striatum workflow init` doesn't emit one. Worth a doc note or a permissive default.
5. Parallel-group write-scope overlap check is path-string-identity, not directory-prefix. Forced per-track sub-directory split for `daemon_pg/handlers/`. Acceptable but worth documenting.
