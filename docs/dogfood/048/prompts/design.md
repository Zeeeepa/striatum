# Design Prompt: RFC 0043 V1 (Postgres as sole substrate, daemon-required runtime)

Produce DESIGN.md at the path your work packet specifies (under `docs/dogfood/048/design/<lane>/`).

Read `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` first — especially §1 (substrate boundary), §3 (daemon-required CLI), §4 (migration command), §5 (method registry expansion). Also skim RFC 0030 (RPC server), RFC 0033 (substrate rewrite), RFC 0039 (Go daemon scope delta), and D094 in `docs/DECISION_LOG.md` (which supersedes D006/D007/D036 and the SQLite half of D009).

Design the implementation across **two tracks**:

**Track A — Schema + migrate-repo-local (codex):**
- New daemon-side SQL migration under `src/striatum/daemon_pg/sql/` that creates the 15 repo-local tables (`runs`, `sessions`, `jobs`, `queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`, `blockers`, `command_requests`, `process_executions`, `events`, `job_worktrees`, `process_supervisors`, `process_supervisor_pointers`) with `repository_id UUID NOT NULL REFERENCES repositories(repository_id)` and `(repository_id, ...)` indexes per current access patterns. Reuse RFC 0033 roles: daemon read-write, append-only on `events`/`artifacts` via revoked UPDATE/DELETE, migration role for DDL.
- CLI verb `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--dry-run] [--keep-sqlite-readonly] [--confirm-delete]`. SERIALIZABLE single-transaction migrate. Audit-chain byte-equivalent re-anchor (RFC 0043 §4 step 5). `--keep-sqlite-readonly` default renames `state.sqlite3` → `state.sqlite3.tombstone` mode 0444. Idempotent re-run via a `repo_migrations` checkpoint row.
- Test fixture at `tests/fixtures/v1_repo_local_sqlite/state.sqlite3` (V1 schema snapshot). Tests cover dry-run, full-run, idempotent re-run, tombstone behavior.

**Track B — CLI surface + RPC registry (claude):**
- Retire `--no-daemon` from `src/striatum/cli/parser.py` — parsing must return the standard unknown-option error per RFC 0043 §3.
- Exit code **11** (`daemon_unreachable`) wired in `src/striatum/cli/dispatch.py`. Stderr names the daemon socket path the client tried + platform-specific remediation (`systemctl --user start striatumd` / `launchctl bootstrap` / `striatumd --foreground` / Postgres install hints).
- Exit code **12** (`repo_not_migrated`) for unmigrated repos — stderr names `striatum daemon migrate-repo-local`.
- Expand RFC 0030 method registry under `src/striatum/daemon_rpc/` to cover every mutation in `src/striatum/cli/mutations.py` per RFC 0043 §5 table (session.register, work.claim_next/ack/heartbeat/complete/block/release, artifact.publish, review.submit/verdict, decision.record, checkpoint.resolve, recovery.requeue_stale/cancel_job/resume, worktree.create, branch.confirm, run.prepare/start/pause/resume/cancel, supervise.* already present, workflow.validate/generate) plus matching read-capability methods (status/why/dashboard/doctor/run summary/run graph/evidence export).
- `daemon doctor` continues to run without the daemon — it touches configuration only.

Cover concretely per track: exact file paths, function-level entry points (cite current names), capability mapping, test paths. Cite RFC 0030/0033 patterns being mirrored. Hand-waving "we add a method" without a pinpoint citation is grounds for review to bounce.

Out of scope: bundled Postgres distribution, multi-tenancy (`tenant_id` enforcement), hosted-mode auth, the Go-core revision itself (RFC 0039 follow-up), rewriting historical dogfood scaffolds. README / TODO / CHANGELOG / SPEC / HOW_TO updates are operator-only after the dogfood lands.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes, NO lane prefix. Lowercase `author:`. Slug shape: `<role>-unknown-model-<NN>`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
