# Track C Design Prompt: Repo-local State to Postgres RFC 0042

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/042/track_c/design/<lane>/`).

Design **RFC 0042 V1 acceptance criteria** for moving repo-local state (the contents of `.striatum/state.sqlite3`) into the daemon's PostgreSQL DB, keyed by `repo_id`.

This RFC supersedes D006 (SQLite as v1 live coordination layer), D007 (repo-local state under `.striatum/`), D028 (local-only writes through CLI). It does NOT touch D083 — single-user trust boundary stays. "Multi-tenancy" in this context means schema-level per-repo isolation (rows keyed by `repo_id`), not user-level auth.

Cover concretely:

- Schema changes: add `repo_id` column to every repo-local table (`runs`, `jobs`, `sessions`, `queue_messages`, `leases`, `events`, `blockers`, `verdicts`, `decisions`, `artifacts_metadata`, `process_executions`, `job_worktrees`, `process_supervisor_pointers`, etc.). Composite keys `(repo_id, run_id)`, etc.
- `registered_repositories` table (daemon-DB-side, exists) becomes foreign-key anchor.
- New CLI verb: `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--keep-sqlite-readonly] [--dry-run]`.
- `.striatum/` directory becomes operational scratch only (FIFOs, pidfiles, supervisor stdout logs). No authoritative state.
- Daemon becomes mandatory. CLI behavior when no daemon running: refuse cleanly with named remediation (not silent fallback).
- RFC 0039 scope revision: Go daemon becomes gateway for ALL repo-local ops from day 1 of the rewrite (vs RFC 0039's original scope which only covered daemon-owned state).
- Migration story: existing `.striatum/state.sqlite3` files migrated per-repo on first daemon start with the new flow. SQLite copy kept read-only as rollback.
- Audit chain integrity preserved across migration.

Reference D093 (encoded in parallel session by the operator) for the explicit supersession decision-log entry.

Out of scope: cross-machine semantics, hosted mode, per-user auth, OS-keyring integration.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.

One-shot supervised invocation. Write the artifact directly.
