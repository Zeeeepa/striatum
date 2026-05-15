# Design Prompt: RFC 0048 Phase C read-surface PG handlers

Produce DESIGN.md at the path your work packet specifies (under `docs/dogfood/060/design/<lane>/`).

## Why this dogfood

RFC 0048 V1 Phase A (dogfood-057) ported the 16 mutation handlers. V1.5
(dogfood-058 → v1.50.0) wired the daemon's Unix-socket accept loop +
role-provisioning runbook. Phase C scaffold (v1.51.0) routes CLI verbs
through daemon RPC over Unix socket. **The substrate flip is mechanically
complete for mutations.** The gap that prevents
`striatum migrate-repo-local` from producing a working Postgres-only
workspace: the read CLI verbs (status / dashboard / list.\* / run.summary
/ why / doctor / evidence.export / corpus.export) still fall through
`DaemonRpcRouter._route` → `CLI_ROUTES[<method>]` → `invoke()` → in-process
repo-local SQLite. Post-migration that SQLite is tombstoned, so the read
verbs return exit 3 (`state is not initialized`).

This dogfood ports those 8 read methods to PG handlers so post-migration
reads return real state.

## Required reading

- `docs/rfcs/0048-daemon-side-substrate-migration.md` — particularly the
  V1 Phase A landing summary and the V1.5 follow-up list.
- `docs/dogfood/057/build/track_a/HANDOFF.md` and
  `docs/dogfood/057/build/track_b/HANDOFF.md` — the V1 Phase A handler
  pattern you'll mirror.
- `docs/POSTGRES_TRANSITION.md` — operator runbook + role provisioning.

## Legacy contracts to mirror

For each method, the legacy function is the contract — the PG handler
must return the same top-level JSON shape. Read the function bodies
before designing:

| Method | Legacy contract source |
|---|---|
| `status` | `src/striatum/status.py::compute_status` (search project for actual symbol) |
| `dashboard` | `src/striatum/dashboard.py::render_dashboard` |
| `list.runs` | `src/striatum/cli/list.py` (the runs subcommand) |
| `list.sessions` | `src/striatum/cli/list.py` (sessions subcommand) |
| `list.jobs` | `src/striatum/cli/list.py` (jobs subcommand) |
| `list.artifacts` | `src/striatum/cli/list.py` (artifacts subcommand) |
| `list.workflows` | `src/striatum/cli/list.py` (workflows subcommand) |
| `run.summary` | `src/striatum/run_summary.py` |
| `why` | `src/striatum/why.py::compute_why` |
| `doctor` | `src/striatum/cli/doctor.py` |
| `evidence.export` | `src/striatum/evidence.py` |
| `corpus.export` | `src/striatum/corpus_export.py` |

If a legacy function has moved or been renamed, cite the actual location
in your DESIGN.md — do not hand-wave with "the read code path".

## Per-method specificity required

For each read method, specify:

1. **Legacy source** — exact `path:line_range` and function name.
2. **New PG handler path** — `src/striatum/daemon_pg/handlers/reads/<method>.py` (or alternate path you propose).
3. **Decorator + signature** — `@register_pg_handler("<method.name>", read_only=True)` and `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
4. **striatumd.\* tables queried (read set)** — every read must scope by `ctx.repository_id`. List the tables and the columns selected.
5. **Return-shape parity contract** — exact top-level JSON keys the handler returns. The CLI/UI must not detect the substrate flip.
6. **Test path + parity strategy** — `tests/daemon_pg/handlers/reads/test_<method>.py`. Either a per-key diff vs the legacy SQLite path on a known fixture (preferred if claude HIGH#1's parity rig is wired) OR a shape-only smoke test.
7. **Error modes** — `repo_not_registered`, malformed `run_id`, no rows. Each returns an RPC error envelope, not silent empty results.

## Cross-cutting decisions (propose; synthesis locks)

- **Module layout** — single file per method (recommended; matches Phase A pattern) vs grouped by cluster.
- **repository_id scoping mechanism** — `WHERE repository_id = $1` discipline (recommended; explicit), wrapper function, or stored-procedure scope. Pick one.
- **Parity test strategy** — wire the unused parity rig from `tests/daemon_pg/handlers/recovery_evidence/conftest.py` (claude HIGH#1 was deferred from V1.5) or smoke-test only. If you wire it: enumerate the changes needed. If smoke-only: justify (e.g., the legacy reads have been stable for 50 dogfoods).
- **Single implement track** — synthesis MUST lock single track. Splitting (e.g., "core reads" / "reporting reads") creates the same boundary conflicts that caused dogfood-058's cycle exhaustion. Use sub-agents inside the single track for parallelism, not separate tracks.

## Out of scope

- Phase B (Go core parity) and the V1.5 follow-up items from dogfood-058
  (codex F2 capability-denial matrix, F3 chain-locking, F4 append-only
  grants, claude HIGH#2 dead code, schema 0006). They have their own
  dogfood; do not pull them in here.
- The CLI hook in `src/striatum/cli/daemon_rpc_route.py` (already shipped
  in v1.51.0). Track A in V1.5 owned it; you are forbidden from editing
  it (see workflow `forbidden_paths`).
- Any change to `DaemonRpcRouter._route` — the existing
  `resolve_pg_handler()` lookup is what wires your new handlers in; you
  do not touch the router.

## Byline discipline

The work packet supplies an exact `author: <slug>` line. Copy it
verbatim. Plain Markdown line, lowercase `author:`, no decoration. Slug
shape: `<role>-unknown-model-<NN>`.

One-shot supervised invocation. Write the artifact directly. If
`striatum ack` is denied, write the artifact and exit normally; the
operator publishes on your behalf.
