# STRIATUM Remediation Plan

Date: 2026-05-17
Planner: Codex GPT-5
Status: superseded historical input

Supersession note, 2026-05-18: This root-level planning artifact is retained
as source material for RFC 0068-0071 and the architecture remediation
synthesis. It predates D107 and the subsequent Go daemon parity work. Current
source-of-truth status lives in `docs/SPEC.md`, `docs/TODO.md`,
`docs/ROADMAP.md`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`, and
`docs/rfcs/0068-go-production-daemon-port.md`. Several tasks below have since
landed or inverted: `repo.resolve`, hardcoded RPC client-version cleanup,
`/v1/invoke` daemon routing, daemon MCP filtering/resources, dogfood composite
fail-closed behavior, PostgreSQL daemon sweep/global reads, Go
`workflow.generate --shape multi_phase`, and `daemon doctor --authority`.

## 0. Source Review

review: Source consumed: `STRIATUM_ARCHITECTURE_REVIEW_CODEX_GPT_5_2026-05-17.md`. Review date: 2026-05-17. Review model: Codex GPT-5.

actual: No committed changes landed after the source review in this working tree, because the source review itself is untracked. The tree is dirty, and several reviewed files have uncommitted changes. Two review claims are now stale against the current tree: `src/striatum/cli/dispatch.py` already fails closed on unexpected daemon-route exceptions at lines 247-260, with a guardrail test at `tests/architecture/test_authority_guardrails.py:237-258`; and `repo.add`, `repo.list`, and `repo.remove` now have PostgreSQL-native paths in `src/striatum/daemon.py:767-1117` plus tests in `tests/daemon_pg/test_repo_registration.py:36-170`. The remaining SQLite registry evidence is re-anchored below to current lines: daemon startup still opens `connect_registry()` at `src/striatum/daemon.py:1317-1320`, `dashboard_all` still uses SQLite registry/repo-local state at `src/striatum/daemon.py:1643-1683`, `daemon_sweep_once` still uses SQLite registry/repo-local state at `src/striatum/daemon.py:1686-1777`, and daemon MCP resources still use SQLite registry/repo-local state at `src/striatum/daemon.py:1835-1948`.

mine: The review's central diagnosis still stands: Striatum is close to the intended daemon/Postgres spine, but remaining production-looking daemon-global surfaces can still read or mutate the legacy SQLite registry. The plan below treats already-landed fixes as done, narrows the top priority to the remaining SQLite authority paths, and avoids broad rewrites.

## 1. Executive Summary

- review: The source review named authority ambiguity as the main issue. actual: Current code has already closed the CLI route-crash fallthrough and added PG-native repo registration. mine: Do not reopen those as new work; protect them with tests and move on.
- review: The review grouped `repo add/list/remove`, `dashboard.all`, daemon sweep, and daemon MCP resources together. actual: `repo add/list/remove` are now partly fixed, but daemon startup, dashboard-all, sweep, audit/health, and MCP resource reads still use `connect_registry()`. mine: P0 is now "stop production daemon-global surfaces from using SQLite," not "port repo registrar."
- review: The review put client-side PG repo lookup in P1. actual: CLI and service still call `_lookup_repository_id()` which opens Postgres from the client process. mine: Keep this P1; it is serious, but it does not create split-brain state by itself.
- review: The review called hardcoded client versions P1. actual: `client_version="1.51.0"` and `client_version="1.67.0"` remain. mine: This is small and should ride with the repo-resolve/client cleanup or land independently.
- review: The review called Go production-core exposure a P1 cleanup. actual: `VALID_DAEMON_CORES` still includes `go`, and release targets still stage a full Go daemon binary. mine: Keep as P1 because it contradicts D105 and expands the support surface.
- review: The review put `/v1/invoke` cleanup in P2. actual: it remains a production web POST path through `striatum.api.invoke`. mine: Upgrade to P1; it keeps the web service coupled to legacy CLI semantics.
- review: The review suggested authority diagnostics and migration cleanup reports. actual: those would help after the authority path is cleaner. mine: Keep them P2, not P0/P1.
- review: The review suggested archive inspection and durable accepted-risk linkage. actual: both are outside the immediate authority cleanup. mine: Defer them until a concrete operator need or accepted product decision appears.

## 2. Disagreements With The Review

review: "P0 change: make registered daemon-routed CLI commands fail closed on any route failure."

actual: Already done in the current worktree. `dispatch()` now raises `StriatumError("daemon_route_failed...")` instead of swallowing unexpected route exceptions (`src/striatum/cli/dispatch.py:247-260`), and `test_daemon_routed_command_fails_closed_when_route_layer_crashes` pins it (`tests/architecture/test_authority_guardrails.py:237-258`).

mine: Drop as completed. Do not create more work here unless the current uncommitted fix fails review.

review: "P0 change: port or disable SQLite daemon registry surfaces. Start with `repo add/list/remove`, `dashboard.all`, daemon sweep, daemon MCP resources, and `_repo_path_for_id`."

actual: `repo add/list/remove` now have PG-native implementations and RPC routes (`src/striatum/daemon.py:767-1117`, `src/striatum/daemon_rpc/server.py:172-230`), with tests. Legacy fallback still exists when no PG config is found (`src/striatum/daemon.py:802-807`, `src/striatum/daemon.py:987`, `src/striatum/daemon.py:1084`).

mine: Narrow the P0. The remaining blocker is not the registrar path itself; it is the unguarded production fallback to legacy registry plus global daemon surfaces that still read SQLite.

review: `/v1/invoke` cleanup was classified as P2.

actual: `/v1/invoke` is a mutation-capable web endpoint and still calls `invoke(argv, repo=...)` directly (`src/striatum/service.py:350-382`). The daemon-required preflight reduces risk, but the service still depends on CLI fallback shape.

mine: Upgrade to P1. It is not a data-loss blocker today, but it keeps an important surface coupled to the wrong abstraction.

review: Add archive/replay inspection before hosted or PR surface.

actual: Archive foundations exist, but nothing in the current P0/P1 authority cleanup needs an archive browser.

mine: Drop from this plan. It is good product work later, not remediation work now.

## 3. P0 - Blocking

### P0-LEGACY-REGISTRY-GATE

- id: `P0-LEGACY-REGISTRY-GATE`
- source: Review Section 5, "daemon-owned PostgreSQL is authoritative"; Review Section 7, "port or disable SQLite daemon registry surfaces."
- review: The review claimed daemon-global and per-repo live state should not use SQLite, and identified daemon registry fallback as the top mismatch.
- actual: `connect_registry()` still creates/opens `striatumd.sqlite3` (`src/striatum/daemon.py:140-180`). Current `repo_add`, `repo_list`, and `repo_remove` use PG when configured, but silently fall back to legacy SQLite when no PG URL is configured (`src/striatum/daemon.py:767-807`, `src/striatum/daemon.py:970-987`, `src/striatum/daemon.py:1066-1084`).
- mine: Make production fallback impossible before porting every last legacy helper. SQLite registry code can remain for migration/tests, but normal daemon-required commands should refuse without PG.
- what: Add an explicit legacy-registry guard so `repo add/list/remove`, `daemon start`, `dashboard --all`, daemon sweep, daemon MCP resources, `health`, and daemon audit refuse legacy SQLite unless running an explicit migration/test compatibility path.
- why: Without this, a missing or misread `STRIATUM_DAEMON_DB_URL` can create or read a parallel SQLite authority after D094 says PostgreSQL is the sole substrate.
- touches: `src/striatum/daemon.py`, `src/striatum/cli/daemon.py`, `tests/architecture/test_legacy_sqlite_quarantine.py`, new or existing daemon PG/global-surface tests.
- effort: 1-2 days.
- depends on: none.
- acceptance: With `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1` and no explicit test/migration escape, production invocations of `repo list`, `repo add`, `repo remove`, `daemon start`, `dashboard --all`, `daemon_mcp_resources`, and `daemon_sweep_once` either use PG or fail before `connect_registry()`; add a focused test such as `test_production_daemon_global_surfaces_refuse_legacy_registry`.

### P0-PG-DAEMON-SWEEP

- id: `P0-PG-DAEMON-SWEEP`
- source: Review Section 5, daemon startup and sweep still use SQLite; Review Section 7, port daemon sweep.
- review: The review cited daemon startup and recovery sweep as active SQLite registry users.
- actual: `run_daemon_foreground()` still calls `connect_registry()` for bootstrap and instance id (`src/striatum/daemon.py:1317-1320`), and its main loop calls `daemon_sweep_once()` (`src/striatum/daemon.py:1417-1424`). `daemon_sweep_once()` reads SQLite `repositories` and `scheduler_cursors`, then opens repo-local SQLite and calls legacy recovery helpers (`src/striatum/daemon.py:1686-1777`).
- mine: The smallest viable fix is not a new scheduler. Reuse the existing PG recovery handlers and PG scheduler tables.
- what: Replace `daemon_sweep_once()` with a PG-backed implementation that lists active repositories/running runs from `striatumd.*`, records scheduler cursor state in PostgreSQL, and invokes the existing PG `recovery.sweep` handler per run.
- why: The daemon's background loop is an authority path; if it writes recovery events to repo-local SQLite, live recovery can diverge from PG state.
- touches: `src/striatum/daemon.py`, `src/striatum/daemon_pg/handlers/recovery_evidence/sweep.py`, `src/striatum/daemon_pg/sql/*` only if cursor schema is missing, `tests/daemon_pg/`.
- effort: several days.
- depends on: `P0-LEGACY-REGISTRY-GATE`.
- acceptance: A PG-backed integration test creates a running PG run, calls `daemon_sweep_once()` with `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1`, observes a PG recovery event/cursor update, and verifies no `.striatum/state.sqlite3` is opened or created.

### P0-PG-GLOBAL-READS-MCP

- id: `P0-PG-GLOBAL-READS-MCP`
- source: Review Section 5, dashboard-all and daemon MCP resources still use SQLite; Review Section 8, authority diagnostics depend on truthful global reads.
- review: The review identified `dashboard.all`, daemon MCP resources, and `_repo_path_for_id` as central daemon-global surfaces still using SQLite.
- actual: `dashboard_all()` reads the SQLite registry and repo-local status (`src/striatum/daemon.py:1643-1683`). `daemon_mcp_resources()`, `daemon_mcp_read_resource()`, `_repo_path_for_id()`, `_mcp_repo_list()`, and `_mcp_stale_leases()` still read SQLite registry/repo state (`src/striatum/daemon.py:1835-1948`).
- mine: These are read paths, but wrong reads are still contract failures because they tell operators the wrong live state.
- what: Reimplement `dashboard_all` and daemon MCP resource list/read paths over PostgreSQL repository rows and PG read handlers, or fail closed where an equivalent PG DTO is missing.
- why: A dashboard or MCP resource view backed by stale SQLite can hide active PG runs, show removed repos, or report stale lease/recovery state from the wrong substrate.
- touches: `src/striatum/daemon.py`, `src/striatum/daemon_rpc/server.py`, PG read handler modules under `src/striatum/daemon_pg/handlers/reads/`, `tests/daemon_pg/`, `tests/architecture/test_authority_guardrails.py`.
- effort: several days.
- depends on: `P0-LEGACY-REGISTRY-GATE`.
- acceptance: `dashboard.all`, `striatum://daemon/repos`, `striatum://daemon/dashboard`, and per-repo MCP resources work against a PG-only test fixture with `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1`; missing DTOs return explicit `not_implemented` rather than reading SQLite.

## 4. P1 - Serious

### P1-DAEMON-REPO-RESOLVE

- id: `P1-DAEMON-REPO-RESOLVE`
- source: Review Section 5, CLI/web clients open PostgreSQL directly; Review Section 7, add daemon-side repository resolution.
- review: The review said CLI and service should be Unix-socket daemon clients, not direct database readers.
- actual: `_lookup_repository_id()` opens PG in the client process (`src/striatum/cli/daemon_rpc_route.py:177-210`), and `service_daemon.call_repo_method()` reuses it (`src/striatum/service_daemon.py:29-52`).
- mine: This is serious because it adds configuration coupling, but it is not P0 because it reads registry metadata and does not mutate workflow state.
- what: Add daemon-side repo-root resolution, either as `repo.resolve` or as an envelope path where daemon-routed single-repo calls may supply `repo_root` and let the daemon resolve `repository_id`.
- why: A CLI/web client should not fail because it lacks DB config while the daemon has a valid DB connection and socket.
- touches: `contracts/daemon_methods.json`, `src/striatum/daemon_rpc/server.py`, `src/striatum/cli/daemon_rpc_route.py`, `src/striatum/service_daemon.py`, generated method docs, tests.
- effort: several days.
- depends on: `P0-PG-GLOBAL-READS-MCP`.
- acceptance: Client-side modules no longer import `striatum.daemon_pg.connection`; a test monkeypatches client-side PG connect to raise while daemon RPC calls still resolve the repo through the socket.

### P1-RPC-VERSION-SOURCE

- id: `P1-RPC-VERSION-SOURCE`
- source: Review Section 7, remove hardcoded client versions.
- review: The review flagged `client_version="1.51.0"` and `client_version="1.67.0"` as drift.
- actual: Both literals remain (`src/striatum/cli/daemon_rpc_route.py:140-144`, `src/striatum/day_zero.py:319-324`), while `striatum.__version__` is already derived from package metadata (`src/striatum/__init__.py:9-17`).
- mine: Small fix, meaningful because version skew is already part of the RPC refusal contract.
- what: Use `striatum.__version__` for CLI and first-run smoke daemon handshakes.
- why: Hardcoded versions turn release bumps into latent compatibility failures.
- touches: `src/striatum/cli/daemon_rpc_route.py`, `src/striatum/day_zero.py`, tests around daemon handshake/first-run smoke.
- effort: hours.
- depends on: none.
- acceptance: Unit tests assert handshake client versions equal `striatum.__version__`; no source file contains the old `"1.51.0"` or `"1.67.0"` handshake literals.

### P1-WEB-INVOKE-DAEMON-DIRECT

- id: `P1-WEB-INVOKE-DAEMON-DIRECT`
- source: Review Section 7, service `/v1/invoke` cleanup.
- review: The review called this P2 cleanup.
- actual: `/v1/invoke` still calls `striatum.api.invoke` after the web mutation gate (`src/striatum/service.py:350-382`).
- mine: Upgrade to P1 because `/v1/invoke` is the local web mutation entry point. It should not preserve legacy CLI fallback behavior as its core implementation.
- what: Route daemon-mapped `/v1/invoke` commands through `service_daemon.call_repo_method()` and reserve `striatum.api.invoke` for explicit local authoring helpers and test fixtures.
- why: The service should fail in the same shape as daemon RPC, not whatever legacy dispatch path happens to do.
- touches: `src/striatum/service.py`, `src/striatum/service_command_policy.py`, `src/striatum/service_daemon.py`, web service tests.
- effort: 2-4 days.
- depends on: `P1-DAEMON-REPO-RESOLVE`.
- acceptance: A web test monkeypatches `striatum.api.invoke` to fail for a daemon-routed mutation, posts `/v1/invoke` for that command, and observes successful daemon RPC dispatch; local workflow authoring commands still use the allowed local path.

### P1-GO-CORE-GATE

- id: `P1-GO-CORE-GATE`
- source: Review Section 5 and Section 7, D105 enforcement.
- review: The review said Go should be helper/runtime only, not an operator-selectable second production daemon core.
- actual: `VALID_DAEMON_CORES` still includes `go`, `launch_daemon_start()` can exec the Go daemon, and Makefile targets still stage full Go daemon binaries for package data (`src/striatum/cli/daemon.py:19-80`, `Makefile:101-118`).
- mine: Keep Go helper support; remove production-core ambiguity.
- what: Remove `go` from normal daemon core selection or hide it behind an explicit developer-harness variable/name; stop release/package targets from staging a full Go daemon as operator runtime.
- why: A second launchable daemon core creates support and parity obligations that D105 explicitly rejected.
- touches: `src/striatum/cli/daemon.py`, CLI parser/help, Makefile Go targets, docs, Go smoke tests.
- effort: 1-2 days.
- depends on: none.
- acceptance: `striatum daemon start --core go` is rejected in normal mode or clearly requires a developer-harness escape; helper targets and `tests/architecture/test_go_helper_boundary.py` still pass.

### P1-MCP-SURFACE-BOUNDARY

- id: `P1-MCP-SURFACE-BOUNDARY`
- source: Review Section 5, daemon MCP versus local invoke-backed MCP.
- review: The review said there should be one obvious production MCP story.
- actual: `DaemonRpcServer` is capability-filtered and daemon-backed (`src/striatum/mcp.py:461-576`), but `LocalRpcServer` exposes tools and raw `striatum/invoke` over `striatum.api.invoke` (`src/striatum/mcp.py:377-458`, `src/striatum/mcp.py:601-655`). Chat dogfood lifecycle tools also call `invoke` (`src/striatum/web/chat_tools.py:840-862`).
- mine: Do not build a new MCP product. Label or remove the old one.
- what: Mark `LocalRpcServer` and invoke-backed chat dogfood tools as legacy/test/local-authoring only, or remove them from production-facing docs and tool listings.
- why: Agents should not have two mutation stories with different capability and audit semantics.
- touches: `src/striatum/mcp.py`, `src/striatum/web/chat_tools.py`, `docs/MCP.md`, skill/plugin templates, MCP tests.
- effort: 1-3 days.
- depends on: `P1-WEB-INVOKE-DAEMON-DIRECT`.
- acceptance: Production docs and generated skill templates point to daemon MCP only; local invoke-backed tools are either hidden behind a legacy flag or documented as test/local-authoring compatibility.

### P1-DOGFOOD-COMPOSITES-PG-OR-DELETE

- id: `P1-DOGFOOD-COMPOSITES-PG-OR-DELETE`
- source: Review Section 5 MCP concern plus authority matrix dogfood compatibility note.
- review: The review noted registered dogfood composite methods still open repo-local SQLite.
- actual: `DaemonRpcRouter._route_dogfood()` imports `striatum.db.connect` and calls legacy dogfood helpers (`src/striatum/daemon_rpc/server.py:230-260`). The matrix still classifies them as dogfood compatibility (`docs/architecture/COMMAND_AUTHORITY_MATRIX.md:95`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md:125`).
- mine: If they are still used, port them to PG. If they are historical, delete or unregister them. Keeping registered daemon methods that mutate SQLite is too confusing.
- what: Either implement `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` against PG state or remove them from production MCP/daemon registry and keep only historical fixtures.
- why: Compatibility labels are easy to ignore once a method is exposed through daemon RPC/MCP.
- touches: `contracts/daemon_methods.json`, `src/striatum/daemon_rpc/server.py`, dogfood helper modules, generated docs, tests.
- effort: several days.
- depends on: `P1-MCP-SURFACE-BOUNDARY`.
- acceptance: No production daemon RPC method imports `striatum.db.connect`; dogfood composite behavior is covered by PG tests or absent from exposed daemon method lists.

## 5. P2 - Smell / Nice-To-Have

### P2-AUTHORITY-DOCTOR

- id: `P2-AUTHORITY-DOCTOR`
- source: Review Section 8, add `doctor --authority`.
- review: The review proposed an operator-facing authority report.
- actual: The authority matrix and tests exist, but there is no runtime command that tells an operator which paths are PG-native, local-authoring, migration-only, or legacy.
- mine: Useful after P0/P1, premature before then.
- what: Add `striatum doctor --authority --json` to report current authority classifications and whether production SQLite tripwires would fire.
- why: Maintainers need a cheap way to verify the installed binary matches the intended substrate boundary.
- touches: CLI parser/dispatch, authority matrix helper code, docs, tests.
- effort: 1-2 days.
- depends on: `P0-PG-GLOBAL-READS-MCP`, `P1-MCP-SURFACE-BOUNDARY`.
- acceptance: Command emits machine-checkable JSON with classifications for registered methods and key CLI-only commands; tests pin several known classifications.

### P2-MIGRATION-CLEANUP-REPORT

- id: `P2-MIGRATION-CLEANUP-REPORT`
- source: Review Section 8, migration cleanup report.
- review: The review proposed a post-migration command/report that proves PG registration, tombstone state, and no production SQLite opens.
- actual: Migration code and repo registration tests exist, but post-cutover cleanliness is spread across doctor, migration output, and tripwire tests.
- mine: Worth doing after legacy registry paths are gated, because only then can the report be honest.
- what: Add a verify-only migration cleanup report for a target repo, emitted by `daemon migrate-repo-local` or a small `repo verify-cutover` command.
- why: Operators need one command to know an old `.striatum/state.sqlite3` is no longer live state.
- touches: `src/striatum/daemon_pg/repo_local_migration.py`, CLI parser/dispatch, docs, tests.
- effort: 1-2 days.
- depends on: `P0-LEGACY-REGISTRY-GATE`.
- acceptance: A test fixture with a migrated/tombstoned repo reports PG row present, tombstone mode correct, no fresh SQLite creation risk, and scratch paths only.

### P2-AUTHORITY-MATRIX-GENERATION

- id: `P2-AUTHORITY-MATRIX-GENERATION`
- source: Review Section 7, generated plus annotated authority matrix.
- review: The review suggested driving more of `COMMAND_AUTHORITY_MATRIX.md` from the method contract.
- actual: `DAEMON_METHOD_TABLES.md` is generated, but `COMMAND_AUTHORITY_MATRIX.md` still carries manually maintained classifications.
- mine: Keep the human rationale, but reduce rote drift.
- what: Generate the stable method/route columns of `COMMAND_AUTHORITY_MATRIX.md` from `contracts/daemon_methods.json` and keep only classification/rationale as checked-in annotations.
- why: Manual method tables become stale exactly when architecture transitions are active.
- touches: `scripts/generate_daemon_method_tables.py` or a new script, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`, architecture tests.
- effort: 1-3 days.
- depends on: `P1-DOGFOOD-COMPOSITES-PG-OR-DELETE`.
- acceptance: A `--check` script fails if contract methods or CLI routes drift from the matrix; existing human status notes remain readable.

## 6. Dependency Map

mine: The critical path is short: first prevent new production use of the SQLite registry, then port the daemon-global surfaces that still need a live implementation. Client cleanup and surface simplification should wait until those global surfaces are PG-backed, because otherwise they will route more callers into incomplete daemon-global behavior.

- `P0-LEGACY-REGISTRY-GATE` must land before `P0-PG-DAEMON-SWEEP`.
- `P0-LEGACY-REGISTRY-GATE` must land before `P0-PG-GLOBAL-READS-MCP`.
- `P0-PG-GLOBAL-READS-MCP` must land before `P1-DAEMON-REPO-RESOLVE`.
- `P1-DAEMON-REPO-RESOLVE` must land before `P1-WEB-INVOKE-DAEMON-DIRECT`.
- `P1-WEB-INVOKE-DAEMON-DIRECT` must land before `P1-MCP-SURFACE-BOUNDARY`.
- `P1-MCP-SURFACE-BOUNDARY` must land before `P1-DOGFOOD-COMPOSITES-PG-OR-DELETE`.
- `P0-LEGACY-REGISTRY-GATE` must land before `P2-MIGRATION-CLEANUP-REPORT`.

## 7. What I'd Defer Indefinitely

review: The review mentioned archive/replay inspection as a useful addition.

actual: Archive verify/replay work is already tracked elsewhere, and it does not close the live authority split.

mine: Defer archive inspection until someone needs to consume archives outside tests or dogfood reports. It is product polish, not remediation.

review: The review mentioned durable accepted-risk linkage after a product decision.

actual: TODO already blocks this on an authority decision.

mine: Do not implement a storage shape now. Keep using explicit operator decisions until the decision artifact versus daemon row versus workflow metadata question is settled.

review: The review mentioned future Git/PR integration only after boundaries are accepted.

actual: TODO 60 already blocks hosted-provider authentication, PR creation, and commit authority.

mine: Defer indefinitely for this plan. Read-only local git snapshots are fine later, but PR automation is unrelated to the current architecture debt.

review: The review allowed either hiding or removing the Go daemon.

actual: D105 already says Go is not a peer production daemon.

mine: Do not fund Go daemon parity. Keep the helper. Delete or gate the daemon.

## 8. Open Questions

review: The review asked whether legacy registry compatibility should remain accessible.

actual: Current code still has legacy registry functions and fallback.

mine: The P0 choice does not require deleting all legacy code. It does require an explicit answer: is legacy registry reachable only under `STRIATUM_TEST_HARNESS=1`/migration, or is there a supported pre-PG operator mode? If the latter, the SPEC is wrong.

review: The review asked whether dogfood composite tools are still real tools.

actual: They are registered daemon methods and still SQLite-backed.

mine: If current dogfoods still depend on them, port them. If not, unregister them. This affects P1 scope, not P0 selection.

review: The review asked whether local invoke-backed MCP is supported.

actual: The code still ships it.

mine: Decide whether it is legacy/local-authoring only or production. If production, it needs daemon capability semantics. If legacy, hide it.

review: The review asked how docs should describe transition reality.

actual: Some docs now describe repo registration progress, but SPEC still states the ideal contract.

mine: Keep SPEC as the target contract and make transition docs/matrix name bounded exceptions until P0/P1 close them.
