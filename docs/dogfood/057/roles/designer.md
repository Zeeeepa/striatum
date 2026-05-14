# Designer Role (Dogfood 057)

Three fresh-design lanes (codex, claude, gemini) produce independent perspectives on RFC 0048 Phase A (port the 16 daemon RPC single-repo handlers from SQLite-backed to PG-backed). Synthesis picks one path across two implementer tracks (A workflow-loop, B recovery+evidence). Cite existing code that your design changes — do not propose green-field shapes.

Required citations (read these before designing):

- `docs/rfcs/0048-daemon-side-substrate-migration.md` — §Background, §Goals, §Phasing (Phase A scope), §Acceptance.
- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` — substrate boundary, daemon-required runtime.
- `docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md` — daemon Postgres schema, audit chain anchoring, role grants.
- `docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md` — RPC envelope, method registry.
- `docs/POSTGRES_TRANSITION.md` — operator runbook (you are wiring this substrate path).
- `docs/DECISION_LOG.md` — D094 supersedes D006/D007/D036 and the SQLite half of D009.

Current code to read:

- `src/striatum/cli/mutations.py` — the 9 workflow-loop functions today (Track A scope).
- `src/striatum/cli/recovery.py` — the 6 recovery functions today (Track B scope).
- `src/striatum/cli/evidence.py` — `evidence_export` today (Track B scope).
- `src/striatum/daemon_rpc/server.py` — `DaemonRpcRouter._route`; how it currently delegates.
- `src/striatum/daemon_rpc/registry.py` — `METHOD_REGISTRY`.
- `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql` — 15 repo-local PG tables.
- `src/striatum/daemon_pg/repo_local_migration.py` — `repository_id` identity.

## Output

`docs/dogfood/057/design/<lane>/DESIGN.md`. Cover both tracks (A + B); the synthesis will reconcile. Be exact: file paths, function names, PG tables touched per method, audit-chain anchor strategy, test paths.

Out of scope: Phase B (Go core), Phase C (SQLite removal), the Unix-socket accept-loop gap. Note them only in a "deferred" section.

## Byline discipline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.
