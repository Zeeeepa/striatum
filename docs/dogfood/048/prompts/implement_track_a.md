# Implement Track A: RFC 0043 V1 — Schema + migrate-repo-local (codex Python)

Blocked until `review_design` returns an accepting verdict.

Implement Track A per `docs/dogfood/048/DESIGN_SYNTHESIS.md`. **You write Python only. Track A is the daemon-side schema move + the `migrate-repo-local` CLI verb body.** Sister Track B (CLI surface + RPC registry, claude) runs in parallel — do not cross into its write scope.

**Your scope (codex Python-side):**

- `src/striatum/daemon_pg/sql/` — new SQL migration that creates the 15 repo-local tables (`runs`, `sessions`, `jobs`, `queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`, `blockers`, `command_requests`, `process_executions`, `events`, `job_worktrees`, `process_supervisors`, `process_supervisor_pointers`) with `repository_id UUID NOT NULL REFERENCES repositories(repository_id)` + `(repository_id, ...)` indexes per the synthesis access patterns. Append-only grants on `events`/`artifacts` per RFC 0033 §3.
- `src/striatum/daemon_pg/` — `cutover.py` (or named module per synthesis) for the migrate-repo-local body. SERIALIZABLE single-transaction migrate. Audit-chain byte-equivalent re-anchor. Tombstone rename + chmod 0444. `repo_migrations` checkpoint row for idempotency.
- `src/striatum/cli/daemon.py` — register and dispatch `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--dry-run] [--keep-sqlite-readonly] [--confirm-delete]`. Mirror the RFC 0033 daemon-migration command's auth posture (explicit admin token).
- `tests/daemon_pg/`, `tests/migrations/`, `tests/fixtures/v1_repo_local_sqlite/state.sqlite3` — golden V1 SQLite fixture + tests for dry-run, full-run, idempotent re-run, tombstone behavior, audit-chain byte-equivalence.
- `docs/dogfood/048/build/track_a/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per concern, dispatched in parallel:

- Sub-agent SQL migration: 15 tables, indexes, grants, namespace decision per synthesis.
- Sub-agent migrate body: SERIALIZABLE transaction, row replay preserving `created_at`/`event_id` ordering.
- Sub-agent audit-chain re-anchor: byte-equivalent hash verification end-to-end.
- Sub-agent tombstone + idempotency: rename + chmod, `repo_migrations` checkpoint row.
- Sub-agent fixture + tests: build the V1 SQLite fixture under `tests/fixtures/v1_repo_local_sqlite/`; cover dry-run, full-run, idempotent re-run, `--keep-sqlite-readonly` default, `--confirm-delete` required path.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT touch**: `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`, `src/striatum/cli/mutations.py`, or `src/striatum/daemon_rpc/` (sister Track B owns those). **Do NOT write to**: README / TODO / CHANGELOG / RFC index / SPEC / HOW_TO. Operator handles those manually after the dogfood lands.

**Backward-compat (non-negotiable)**: existing test fixtures must continue to pass against `daemon_mode=on`. The migration command preserves D028 (no transcripts in SQLite means no transcripts in Postgres).

Verification: `make lint`, `make typecheck`, `make test` all pass. The migration tests exercise the V1 fixture end-to-end and assert byte-equivalent audit-chain hashes.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.
