# Implementer Role (Dogfood 048 — two tracks)

Two parallel implementer tracks: Track A codex (schema + migrate-repo-local),
Track B claude (CLI surface + RPC registry). The workflow validator
enforces per-track write scope — stay strictly inside your job's
`write_scope.allowed_paths`.

## Track A (codex Python)

Owns:

- `src/striatum/daemon_pg/sql/` — new SQL migration creating the 15
  repo-local tables (`runs`, `sessions`, `jobs`, `queue_messages`,
  `leases`, `work_packets`, `artifacts`, `verdicts`, `blockers`,
  `command_requests`, `process_executions`, `events`, `job_worktrees`,
  `process_supervisors`, `process_supervisor_pointers`) with
  `repository_id UUID NOT NULL REFERENCES repositories(repository_id)`
  + `(repository_id, ...)` indexes. Append-only grants on `events` and
  `artifacts` per RFC 0033 §3.
- `src/striatum/daemon_pg/cutover.py` (or named module per synthesis) —
  migrate-repo-local body. SERIALIZABLE single-transaction migrate,
  audit-chain byte-equivalent re-anchor, tombstone rename + chmod 0444,
  `repo_migrations` checkpoint row for idempotency.
- `src/striatum/cli/daemon.py` — register / dispatch
  `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>
   [--dry-run] [--keep-sqlite-readonly] [--confirm-delete]`.
- `tests/daemon_pg/`, `tests/migrations/`,
  `tests/fixtures/v1_repo_local_sqlite/state.sqlite3` — golden V1 SQLite
  fixture + dry-run, full-run, idempotent re-run, tombstone tests.

Forbidden in Track A: `src/striatum/cli/parser.py`,
`src/striatum/cli/dispatch.py`, `src/striatum/cli/mutations.py`,
`src/striatum/daemon_rpc/`.

## Track B (claude Python)

Owns:

- `src/striatum/cli/parser.py` — retire `--no-daemon` (unknown-option
  error per RFC 0043 §3).
- `src/striatum/cli/dispatch.py` — exit code 11 (`daemon_unreachable`)
  with platform-specific remediation; exit code 12 (`repo_not_migrated`)
  with `migrate-repo-local` remediation.
- `src/striatum/errors.py`, `src/striatum/daemon.py` — exit-code
  constants + daemon doctor remediation (continues to work without the
  daemon).
- `src/striatum/daemon_rpc/` — expand RFC 0030 method registry to cover
  every mutation in `src/striatum/cli/mutations.py` per RFC 0043 §5
  table, plus read-capability methods.
- `tests/cli/`, `tests/exit_codes/`, `tests/daemon_rpc/` — exit-code
  fires, `--no-daemon` retirement assertion, method-registry
  exhaustiveness.

Forbidden in Track B: `src/striatum/daemon_pg/sql/`,
`src/striatum/daemon_pg/cutover.py`, `src/striatum/cli/daemon.py`.

## Common (both tracks)

Use sub-agents aggressively. Dispatch one per concern in parallel.
Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **Neither
implementer nor any sub-agent updates `docs/rfcs/README.md`,
`docs/TODO.md`, `CHANGELOG.md`, `docs/SPEC.md`, `docs/HOW_TO_AGENT.md`,
`docs/HOW_TO_HUMAN.md`, or `docs/UBIQUITOUS_LANGUAGE.md`** — operator
handles those manually after the dogfood lands (dogfood-042 cascade
lesson).

**Backward-compat (non-negotiable)**: existing test fixtures must
continue to pass against `daemon_mode=on`. Integration tests use the
daemon-mediated path only.

**Byline discipline**: copy the work packet's `author: <slug>` verbatim.
Plain markdown line, no bold/italics/lane-prefix. Lowercase `author:`.
Slug shape: `implementer-unknown-model-<NN>`.

Operational notes:

- Lease can expire if `make test` exceeds ~30 minutes. Prefer focused
  pytest before wider verification.
- One-shot supervised invocation. Do not ask the operator follow-up
  questions. If `striatum ack` is denied, write the artifact and exit
  normally; the operator publishes on your behalf.
- Per D089/D091, OPERATOR_REPORT.md is the operator's responsibility,
  written incrementally — not yours.
