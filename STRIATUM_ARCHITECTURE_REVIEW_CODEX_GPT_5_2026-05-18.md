# STRIATUM Architecture Review - CODEX_GPT_5 - 2026-05-18
author: reviewer-codex-gpt-5-001

## 0. Files reviewed

stated: The architecture-review prompt asks for an architecture review of the project rooted at the current working directory, with every reviewed file listed before any claims. The repo instructions additionally say to start with `README.md`, `docs/INDEX.md`, `docs/SPEC.md`, `docs/DECISION_LOG.md`, `docs/UBIQUITOUS_LANGUAGE.md`, and `docs/TODO.md`.

actual: I reviewed the Striatum working tree during the 2026-05-18 Go/PostgreSQL daemon remediation pass. The first review snapshot was taken before the daemon CLI/admin cleanup commit; this document has been refreshed again after maintainer decisions clarified the operating topology, public adoption target, workflow-authoring path, Python `daemon_pg` deletion status, and human-principal escalation priority.

mine: I treated the dirty files as live architecture evidence because they affect the Go/PG cutover boundary. I did not execute tests; CI health on `main` is not assumed green in this revision.

Files read:

- Architecture-review prompt outside this repository
- `README.md`
- `docs/INDEX.md`
- `docs/SPEC.md`
- `docs/DECISION_LOG.md`
- `docs/UBIQUITOUS_LANGUAGE.md`
- `docs/TODO.md`
- `docs/POSTGRES_TRANSITION.md`
- `docs/operator/BRIEF.md`
- `docs/operator/plans/rfc-0068-go-daemon-port.md`
- `docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md`
- `docs/rfcs/0068-go-production-daemon-port.md`
- `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- `docs/architecture/DAEMON_METHOD_TABLES.md`
- `docs/MCP.md`
- `docs/WRITING_WORKFLOWS.md`
- `contracts/daemon_methods.json`
- `pyproject.toml`
- `Makefile`
- `src/striatum/__init__.py`
- `src/striatum/api.py`
- `src/striatum/day_zero.py`
- `src/striatum/dashboard.py`
- `src/striatum/daemon.py`
- `src/striatum/daemon_entrypoint.py`
- `src/striatum/daemon_runtime.py`
- `src/striatum/mcp.py`
- `src/striatum/service.py`
- `src/striatum/service_api_routes.py`
- `src/striatum/service_command_policy.py`
- `src/striatum/service_daemon.py`
- `src/striatum/web/artifacts.py`
- `src/striatum/web/run_actions.py`
- `src/striatum/web/run_pages.py`
- `src/striatum/web/workflow_generation.py`
- `src/striatum/cli/daemon.py`
- `src/striatum/cli/daemon_required.py`
- `src/striatum/cli/daemon_rpc_route.py`
- `src/striatum/cli/dispatch.py`
- `src/striatum/cli/parser.py`
- `src/striatum/daemon_pg/client_admin.py`
- `src/striatum/daemon_pg/handlers/registry.py`
- `src/striatum/daemon_rpc/server.py`
- `src/striatum/legacy_sqlite/daemon_registry.py` (reviewed in the intermediate snapshot; removed by the follow-up cleanup)
- `src/striatum/legacy_sqlite/service.py`
- `go/cmd/striatumd/main.go`
- `go/cmd/striatumd/handler_coverage_test.go`
- `go/pkg/admin/tokens.go`
- `go/pkg/admin/tokens_test.go`
- `go/pkg/crossrepo/lifecycle.go`
- `go/pkg/crossrepo/lifecycle_test.go`
- `go/pkg/crossrepo/prepare.go`
- `go/pkg/mcp/capabilities.go`
- `go/pkg/mcp/capabilities_test.go`
- `go/pkg/rpc/auth_pg.go`
- `go/pkg/rpc/capability.go`
- `go/pkg/rpc/registry.go`
- `go/pkg/rpc/registry_methods.go`
- `tests/architecture/test_authority_guardrails.py`
- `tests/architecture/test_legacy_sqlite_quarantine.py`
- `tests/cli/test_daemon_core.py`
- `tests/cli/test_dispatch_daemon_doctor.py`

## 1. Executive summary

stated: Striatum is supposed to be a standalone, local-first workflow runner for terminal-based AI coding agents, with no hosted control plane, no transcript capture, and no telemetry (`README.md:3-9`, `docs/SPEC.md:10-18`). The authoritative state model is a daemon-owned PostgreSQL database scoped by registered target repository, while `.striatum/` is scratch (`README.md:13-43`, `docs/SPEC.md:20-39`, `docs/POSTGRES_TRANSITION.md:12-42`). The clarified operator topology is one human principal piloting 8+ concurrent AI operators across 3+ repositories; the first external user is a team adopting Striatum, not another tool-builder and not an Engram-class internal dogfood case. Go is the only daemon going forward; Python `daemon_pg`, the old Python daemon, and SQLite are deletion targets, not alternate substrates.

actual: The implementation is much closer to that stated architecture than the shape of the codebase first suggests. `striatum daemon start` now launches the Go daemon (`src/striatum/cli/daemon.py:35-50`), `striatumd` is a shim into that path rather than an import of the Python daemon (`src/striatum/daemon_entrypoint.py:1-17`), the parser rejects `--core python` and leaves only a no-op `--core go` compatibility flag (`src/striatum/cli/parser.py:254-381`), and the Go daemon refuses to bind before PostgreSQL is configured and migrated (`go/cmd/striatumd/main.go:81-177`). Active method coverage is guarded by generated Go registry data and a handler coverage test that fails if an active method is absent or still wired to a `not_implemented` handler (`go/pkg/rpc/registry_methods.go:5-107`, `go/cmd/striatumd/handler_coverage_test.go:40-69`). Capability-token usage is implemented, not speculative: tokens are stored in PostgreSQL with per-client capability grants and repository scope checks (`go/pkg/rpc/auth_pg.go:43-135`, `go/pkg/admin/tokens.go:203-242`), and MCP visible tools are filtered by the presented token (`go/pkg/mcp/capabilities.go:16-37`, `go/pkg/mcp/capabilities_test.go:30-67`).

mine: I do not see a present P0 architecture failure in the live path, but I would raise the substrate-deletion work in priority because it is the maintainer's top day-to-day friction and the stated definition of done. Done means legacy SQLite deleted, Python daemon and Python `daemon_pg` deleted, test fixtures collapsed, and a clean PyPI install story where fresh clone -> `pip install` -> `adopt` -> workflow run works on macOS and Linux. The release-provenance gap noted in the original review was closed on 2026-05-18. The next maintainer work should be substrate deletion and install verification; after that, the next product priority is human-principal escalation UX.

stated: The review should not recommend Kubernetes, cloud services, telemetry, or a hosted multi-tenant operating model.

actual: The local architecture already has the right primitive set for one human principal coordinating many local AI operators: a local Go process, a local PostgreSQL instance, Unix socket auth with scoped capability tokens, local web/service clients, and filesystem scratch for supervised agent process material (`README.md:40-43`, `src/striatum/daemon_runtime.py:24-73`, `go/cmd/striatumd/main.go:143-225`). PostgreSQL is correct for stacked reasons: concurrent appender contention, audit-chain row-lock semantics, and operator ergonomics.

mine: The north-star should be smaller than the current repo, not more elaborate. Delete the old Python daemon, Python `daemon_pg`, and SQLite compatibility instead of adding abstractions around them. Make the daemon method contract the single live-operation routing surface. Keep Python where it is a CLI client, renderer, or workflow-generation authoring helper, not as a daemon or direct state backend.

## 2. What the project is trying to be

stated: Striatum's product boundary is clear and unusually useful: it is a local workflow runner for terminal coding agents, not an Engram-specific script, not a SaaS product, and not a hosted orchestration platform (`README.md:3-9`, `docs/UBIQUITOUS_LANGUAGE.md:10-15`). The docs define durable repository files as provenance and workflow definitions, while live workflow state belongs to the daemon PostgreSQL store (`docs/SPEC.md:20-39`, `docs/UBIQUITOUS_LANGUAGE.md:33`). The useful docs consumers are AI operators, future-you cold-start, and provenance; RFCs are forward-looking design proposals, not prompt context and not a generic archive.

actual: The CLI and docs reflect the local-first intent. Quick start starts a local daemon and adopts a local target repo (`README.md:62-67`). PostgreSQL is expected to be operator-provided, not bundled in Docker as a hidden service (`docs/POSTGRES_TRANSITION.md:44-57`). The Makefile defaults to the Go core, builds a local `striatumd`, runs Go conformance, and packages platform binaries into the Python package data (`Makefile:3`, `Makefile:83-121`, `pyproject.toml:43-57`). The daemon runtime token and socket live in a local runtime directory with owner-only permissions where the platform allows it (`src/striatum/daemon_runtime.py:24-73`).

mine: The architecture target is a single-machine control loop with auditable local state for one human principal coordinating many AI operators. The right comparison class is not Airflow, Temporal, GitHub Actions, or a managed queue. It is closer to a durable local supervisor plus a structured task ledger for cross-repo coding-agent runs. That makes the current Go/PG direction right: process lifetime, auth, method dispatch, leases, audit metadata, and capability-scoped operator tokens belong in one daemon.

stated: The daemon is now a hard prerequisite for Striatum verbs, and the retired `--no-daemon` / repo-local SQLite era should not return (`docs/POSTGRES_TRANSITION.md:12-42`, `docs/UBIQUITOUS_LANGUAGE.md:190-192`).

actual: The CLI enforces daemon reachability by default, with only a paired test-harness opt-out using `STRIATUM_TEST_HARNESS=1` and `STRIATUM_DAEMON_REQUIRED=0` (`src/striatum/cli/daemon_required.py:1-20`, `src/striatum/cli/daemon_required.py:60-79`). It then routes mapped commands through a generated daemon RPC route table and fails closed when routing should happen but cannot (`src/striatum/cli/daemon_rpc_route.py:47-87`, `src/striatum/cli/dispatch.py:224-313`). The old migrate command names still parse but refuse before opening SQLite code (`src/striatum/cli/daemon.py:17-32`, `src/striatum/cli/parser.py:254-381`).

mine: The remaining ambiguity is not what Striatum wants to be; it is how quickly the repo can shed compatibility material from what it used to be and make the public adoption path boring. The docs should optimize for AI operators, cold-start maintainership, and provenance, not external contributor onboarding.

## 3. Current architecture

stated: The documented live architecture is CLI, MCP, and web clients talking to a local daemon, which owns PostgreSQL state (`README.md:13-38`). Repository-local `.striatum/` holds operational scratch such as FIFOs, pidfiles, and token cache material, not the live message bus (`README.md:40-43`, `docs/SPEC.md:64-70`).

actual: The main runtime path is now Go daemon first. `striatum daemon start` resolves a packaged, environment-provided, repo-local, or PATH Go binary, verifies its contract metadata, and execs it with socket, PostgreSQL, sweep, and migration-sha arguments (`src/striatum/cli/daemon.py:35-80`, `src/striatum/cli/daemon.py:107-150`). The Go process describes its method registry, requires a PostgreSQL URL, applies migrations, bootstraps the runtime token, sets up audit/request recording, registers handlers, listens on a Unix socket, and starts the recovery scheduler (`go/cmd/striatumd/main.go:81-225`). RPC methods are generated from `contracts/daemon_methods.json` into Go registry data (`contracts/daemon_methods.json:68-260`, `go/pkg/rpc/registry_methods.go:5-107`).

mine: This is the right core shape. The Python launcher contract check is particularly valuable because it catches stale packaged Go binaries before they become confusing local failures, and the newer provenance stamping closes the release/version identity gap.

stated: Python remains allowed as CLI, web, authoring, and client code; Python daemon behavior and Python `daemon_pg` direct-state ownership are transitional deletion targets (`docs/SPEC.md:41-46`, `docs/rfcs/0068-go-production-daemon-port.md:40-64`).

actual: Python now acts mostly as a client boundary. `service_daemon.call_repo_method` resolves the repo through daemon RPC and sends method calls through the socket, without opening PG or SQLite itself (`src/striatum/service_daemon.py:30-93`). `/v1/invoke` routes daemon-backed commands through the daemon route first (`src/striatum/service_api_routes.py:56-87`). Web run pages, actions, and artifact reads use daemon methods and only drop into legacy fixture fallbacks when the paired test harness permits it (`src/striatum/web/run_pages.py:53-294`, `src/striatum/web/run_actions.py:42-365`, `src/striatum/web/artifacts.py:78-135`). The terminal dashboard renders locally but collects its payload through the daemon unless explicitly in the legacy test harness (`src/striatum/dashboard.py:80-108`).

mine: This split is pragmatic only if "Python client" stays literal. Renderers and thin web handlers do not need to be in Go, but Python `daemon_pg` should not remain as a second admin substrate after cutover. Go is the daemon; Python should wrap daemon RPC or local workflow generation.

stated: Local authoring and fixture migration are special cases, not production state authority (`docs/SPEC.md:97-119`, `docs/DECISION_LOG.md:27`).

actual: The old Python daemon and SQLite modules remain. `src/striatum/daemon.py` still imports SQLite-backed registry code and contains legacy repository operations, but `connect_registry` refuses production use unless the paired test harness is enabled (`src/striatum/daemon.py:1-73`, `src/striatum/daemon.py:141-209`, `src/striatum/daemon.py:963-1030`). `src/striatum/cli/dispatch.py` still imports `sqlite3` and `striatum.db`, and it still contains a long local fallback dispatcher, but front-end enforcement and daemon RPC routing prevent mapped production commands from reaching that code (`src/striatum/cli/dispatch.py:5-25`, `src/striatum/cli/dispatch.py:224-313`, `src/striatum/cli/dispatch.py:686-1145`). The quarantine test intentionally classifies remaining SQLite references and fails on unclassified growth (`tests/architecture/test_legacy_sqlite_quarantine.py:21-179`, `tests/architecture/test_legacy_sqlite_quarantine.py:211-390`).

mine: The guards are real, but the code volume is still architectural risk. The correct long-term change is not another guardrail layer; it is converting the remaining fixtures and deleting the old substrate.

## 4. Strengths

stated: The docs say Striatum should have one authoritative live state and local-only operations.

actual: The code now backs that up for the active path. The Go daemon refuses to start without PostgreSQL, the CLI defaults to daemon-required execution, mapped CLI routes fail closed, and MCP daemon resources refuse to provide legacy local fallbacks when PG is missing (`go/cmd/striatumd/main.go:131-177`, `src/striatum/cli/daemon_required.py:60-79`, `src/striatum/cli/daemon_rpc_route.py:47-87`, `src/striatum/mcp.py:461-595`, `src/striatum/mcp.py:694-698`).

mine: The project has crossed the most important architectural threshold: old local files are no longer casually competing with daemon state for active workflow authority.

stated: Go daemon parity and method authority should be observable, not aspirational.

actual: The repo has several good guardrails. The Go handler coverage test instantiates handlers and fails active registered methods that are missing or still `not_implemented` (`go/cmd/striatumd/handler_coverage_test.go:40-69`). The Python authority tests check method registry coverage, matrix drift, fallback removal, route fail-closed behavior, and CLI production refusal before SQLite connect (`tests/architecture/test_authority_guardrails.py:138-459`). `daemon doctor --authority` reports PostgreSQL, legacy registry status, method counts, and fallback route counts from the same registries used at runtime (`src/striatum/cli/dispatch.py:1606-1714`).

mine: These are the right tests for a local project with one human principal. They turn architectural intent into cheap local checks instead of relying on a reviewer's memory, but the CI matrix still needs to be proven green for public adoption.

stated: The old SQLite migration window is closed except for fixture paths.

actual: Retired migrate commands parse and refuse with a specific exit code before importing the old migration path (`src/striatum/cli/daemon.py:17-32`). The service and web layers gate legacy fallback behind paired test-harness env vars (`src/striatum/legacy_sqlite/service.py:29-51`, `src/striatum/service_command_policy.py:41-58`). `tests/architecture/test_legacy_sqlite_quarantine.py` is explicit about which remaining SQLite references are allowed and why (`tests/architecture/test_legacy_sqlite_quarantine.py:21-179`).

mine: The quarantine is a good temporary architecture. It should not become a permanent compatibility promise.

stated: Day-zero adoption should make local operation viable without cloud scaffolding.

actual: `adopt` initializes `.striatum/`, installs optional skills/plugins, optionally creates the DDD layout, registers the repo in PostgreSQL when configured, and offers a first-run smoke path that checks socket, token, PG doctor, registration, MCP, and a sample daemon read (`src/striatum/day_zero.py:72-181`, `src/striatum/day_zero.py:184-328`).

mine: This is exactly the kind of adoption-centered feature Striatum needs, but the acceptance bar should be higher than a local checkout smoke: fresh clone, `pip install`, `adopt`, and a workflow run should work on macOS and Linux.

## 5. Concerns

### Concern 1 - legacy substrate still dominates the code shape

stated: D107/D111/D113 say production ownership moved to Go/PG and Python daemon/SQLite should retire from production and compatibility paths (`docs/DECISION_LOG.md:27-33`). The clarified done state adds Python `daemon_pg` deletion, collapsed fixtures, and a clean PyPI install/adopt/run path on macOS and Linux.

actual: The old substrate remains large and importable. `src/striatum/daemon.py` still implements legacy Python daemon operations and SQLite-backed registry access, albeit with production guards (`src/striatum/daemon.py:141-209`, `src/striatum/daemon.py:963-1030`). `src/striatum/cli/dispatch.py` still imports SQLite and still contains local implementations for many older commands behind routing gates (`src/striatum/cli/dispatch.py:5-25`, `src/striatum/cli/dispatch.py:686-1145`). The legacy quarantine map is broad enough to be its own subsystem (`tests/architecture/test_legacy_sqlite_quarantine.py:21-179`).

mine: This is serious, not blocking. I would downgrade any claim that production is currently broken, because the guards and route tests are strong. I would upgrade the deletion work as the top architectural priority because substrate-migration drag is the maintainer's day-to-day friction and it blocks the public adoption definition of done.

### Concern 2 - MCP is on the right path; service/admin compatibility remains

stated: Daemon/MCP is mandatory for operator-driven runs, and marker files or provider hooks are not authoritative (`docs/operator/BRIEF.md:17-46`). Capability tokens are meant to be different per AI operator with differentiated scopes, and that is already exercised in practice.

actual: The daemon RPC server has a capability-filtered method/resource surface and hides local workflow file authoring (`src/striatum/daemon_rpc/server.py:26-42`, `src/striatum/daemon_rpc/server.py:167-207`). Since this review was written, local stdio MCP `tools/list` is empty and `tools/call` returns `local_tools_unavailable`; live MCP discovery now belongs to the daemon surface (`docs/MCP.md:8-15`, `docs/MCP.md:77-80`). Go MCP tool visibility filters through the same capability model that authorizes RPC (`go/pkg/mcp/capabilities.go:16-37`). The service layer still keeps lazy legacy wrappers and compatibility fallback points, even though production reads now use the daemon (`src/striatum/service.py:95-145`, `src/striatum/service_api_routes.py:163-234`).

mine: The MCP side is no longer a broad CLI-shaped facade; that part is materially improved. The remaining risk is service/admin compatibility and documentation drift: agent docs must teach token-scoped daemon MCP, while human-principal web/service surfaces should be framed around escalation and oversight, not AI-operator use.

### Concern 3 - Python daemon_pg bootstrap/admin is transitional, not a plane to preserve

stated: The daemon is described as a hard prerequisite for every Striatum verb (`docs/SPEC.md:20-39`, `docs/POSTGRES_TRANSITION.md:12-42`).

actual: Some commands necessarily operate before or beside a daemon. `adopt` and repo registration use direct PostgreSQL helpers when a daemon may not yet be running (`src/striatum/day_zero.py:72-141`, `src/striatum/day_zero.py:255-276`). `daemon doctor`, `repo add/list/remove`, `daemon status`, `daemon sweep`, and audit/health admin helpers use `src/striatum/daemon_pg/client_admin.py` directly from CLI dispatch (`src/striatum/cli/dispatch.py:1450-1539`, `src/striatum/cli/dispatch.py:1717-1746`, `src/striatum/daemon_pg/client_admin.py:1-5`, `src/striatum/daemon_pg/client_admin.py:205-725`). `daemon_required.py` also has a deliberate optional-command allowlist for `adopt`, `daemon`, `init`, skills, plugins, and self-update (`src/striatum/cli/daemon_required.py:36-49`).

mine: I agree that bootstrap/admin commands need a bounded exception, but I no longer think Python `daemon_pg` should be named as a durable architecture plane. Name the exception so live workflow mutations cannot sneak through it, then migrate its implementation toward Go-owned admin/bootstrap surfaces or a thin Python wrapper over the Go daemon/binary.

### Concern 4 - CI and install health are not known-green

stated: Public adoption requires a clean install story, not only local development confidence.

actual: Updated 2026-05-18, Go daemon release provenance was remediated: the Go source keeps `go-dev` only as an unstamped fallback, the Makefile injects package version/git SHA/dirty state, and the Python launcher rejects unstamped binaries before socket bind (`go/Makefile:1-52`, `go/cmd/striatumd/main.go:81-119`, `src/striatum/cli/daemon.py:107-164`). But the maintainer does not currently know that the full CI matrix is green on `main`, and this review did not run it.

mine: Treat release provenance as done but install health as unresolved. The adoption gate should be an explicit macOS/Linux package smoke: fresh clone, `pip install`, `adopt`, and a real workflow run.

### Concern 5 - the contract is real, but source-of-truth sprawl remains

stated: D108 makes the command authority matrix an architecture test input, and RFC 0068/0069 use contract and guardrail tests to control routing (`docs/DECISION_LOG.md:32`, `docs/rfcs/0068-go-production-daemon-port.md:90-157`, `docs/rfcs/0069-pg-only-daemon-global-surfaces.md:67-129`).

actual: Method metadata lives in `contracts/daemon_methods.json`, generated Go registry files, generated docs tables, a curated authority matrix, Python registries, and multiple guardrail tests (`contracts/daemon_methods.json:3-260`, `go/pkg/rpc/registry_methods.go:5-107`, `docs/architecture/DAEMON_METHOD_TABLES.md:7-177`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md:52-220`, `tests/architecture/test_authority_guardrails.py:138-459`).

mine: This is manageable if the tests are green, but the matrix is not currently known-green. The right fix is not a new service registry; it is to generate more of the docs and keep only human judgment as curated deltas. Since docs consumers are AI operators, future-you, and provenance, optimize docs for operational accuracy and reconstruction rather than external-contributor explanation.

### Concern 6 - some Go interfaces preserve future shapes the product does not yet use

stated: Cross-repo orchestration exists, but the project should prefer implemented local behavior over speculative surfaces.

actual: The Go cross-repo runner interface includes `Prepare`, `Start`, `Cancel`, `ParticipantIntact`, and `HumanCheckpoint`, but the local runner currently returns "not wired" for `Prepare` and fixed/no-op values for some other methods (`go/cmd/striatumd/main.go:391-439`). The active contract does not expose a `cross_repo.prepare` method, and current cancellation logic is implemented and tested (`go/pkg/crossrepo/lifecycle.go:94-165`, `go/pkg/crossrepo/lifecycle_test.go`).

mine: This is a smell, not a bug. Trim the interface to what is actually active unless a near-term RFC needs those hooks.

## 6. North-star architecture

stated: The desired architecture is local daemon plus PostgreSQL authority, Python clients, local scratch, and generic workflow terminology.

actual: The repo is already close enough that the north-star can be expressed as deletion plus consolidation:

- One production daemon: Go `striatumd`, method registry generated from the daemon contract, no selectable Python daemon core and no durable Python `daemon_pg` state substrate.
- One live-state store: daemon-owned PostgreSQL, keyed by registered target repository.
- One operating topology: one human principal coordinates 8+ AI operators across 3+ repositories with per-operator capability tokens and repository-scoped grants.
- One live-operation route: CLI/MCP/service call daemon RPC for registered workflow state changes and reads.
- One human-principal UI: the web UI is for escalation, oversight, and recovery by the human principal, not an AI-operator interaction surface.
- One workflow-authoring path: `striatum workflow generate` is the canonical authoring path; hand-edited JSON and React Flow editing are not the product default.
- One bootstrap path: setup/admin exists only to reach the daemon-owned Go/PG state, not as a lasting Python `daemon_pg` substrate.
- One compatibility fence: legacy SQLite remains only inside named fixture modules until converted or deleted.

mine: This north-star is intentionally unglamorous. It avoids distributed scheduling, queue brokers, cloud secret stores, feature-flag services, and telemetry. The burden on the human principal drops when the codebase has fewer ways to do the same local thing and when eight concurrent operators can only see/call the capabilities their tokens grant.

## 7. Recommended changes

### R1 - finish substrate deletion as the definition of done

stated: RFC 0068 names Python daemon retirement and SQLite deletion as remaining work (`docs/rfcs/0068-go-production-daemon-port.md:143-157`, `docs/operator/plans/rfc-0068-go-daemon-port.md:26-35`). The clarified finish line is stricter: legacy SQLite deleted, Python daemon deleted, Python `daemon_pg` deleted, fixtures collapsed, and clean PyPI install/adopt/run on macOS and Linux.

actual: The production path is guarded, but legacy modules remain broad (`src/striatum/daemon.py:1-1030`, `src/striatum/cli/dispatch.py:686-1145`, `tests/architecture/test_legacy_sqlite_quarantine.py:21-179`).

mine: Convert the remaining legacy fixture tests to PG/Go where they protect current behavior, delete fixture-only coverage where it protects retired behavior, then remove `src/striatum/daemon.py`, `src/striatum/db.py`, old migration helpers, most of `src/striatum/legacy_sqlite/`, and Python `daemon_pg` direct-state code. Keep a tiny importer only if a current test cannot be rewritten without losing valuable historical coverage. Acceptance: no production code imports `sqlite3`, `striatum.db`, or Python `daemon_pg`; macOS and Linux package smoke can run a generated workflow from a fresh install.

### R2 - retire Python daemon_pg without losing bootstrap/admin ergonomics

stated: The docs say daemon is mandatory, but current code needs pre-daemon operations.

actual: `adopt`, `daemon doctor`, repo registration, PG doctor, service install/start/status, and daemon admin commands already bypass daemon RPC by design (`src/striatum/day_zero.py:72-181`, `src/striatum/cli/dispatch.py:1450-1539`, `src/striatum/cli/dispatch.py:1717-1746`, `src/striatum/daemon_pg/client_admin.py:205-725`).

mine: Add a small architecture section and guardrail test that names the temporary bootstrap/admin exception, then move those operations behind Go-owned admin/bootstrap surfaces or a thin CLI wrapper. Make the test fail if a workflow live-state verb joins that set, and make Python `daemon_pg` shrink monotonically.

### R3 - keep daemon MCP token-scoped for AI operators

stated: MCP should expose daemon-backed workflow state, not terminal output or old local marker assumptions, and each AI operator should carry its own differentiated capability token.

actual: `DaemonRpcServer` is close to the intended shape, local stdio MCP no longer exposes CLI-shaped compatibility tools, and Go MCP filters visible tools by token capability (`go/pkg/mcp/capabilities.go:16-37`). Remaining MCP cleanup is documentation/template clarity and any daemon-backed web chat/tool-list parity.

mine: Keep generating the live MCP tool/resource list from `contracts/daemon_methods.json` and capability metadata. Agent guidance should assume per-operator tokens are normal operations, not a future feature.

### R4 - make install health a first-class release gate

stated: Public adoption by a team requires a repeatable install path.

actual: Build provenance is already stamped, but CI matrix health is not currently known. `workflow generate` is the intended authoring path and the CLI has a full generator surface (`src/striatum/cli/parser.py:542-566`, `src/striatum/cli/dispatch.py:1122-1160`, `docs/WRITING_WORKFLOWS.md:38-56`).

mine: Add a package smoke gate that starts from a clean environment on macOS and Linux, installs the published/wheel artifact, runs `adopt`, generates a workflow with `striatum workflow generate`, and completes a small local run. This is the adoption proof that matters more than another internal fixture.

### R5 - generate more authority documentation from the method contract

stated: The authority matrix is an accepted test input, and the method tables are generated.

actual: The matrix contains curated state that overlaps with generated contract tables (`docs/architecture/COMMAND_AUTHORITY_MATRIX.md:52-220`, `docs/architecture/DAEMON_METHOD_TABLES.md:7-177`, `contracts/daemon_methods.json:3-260`).

mine: Keep the matrix only for human judgment: exceptional commands, bootstrap/admin exceptions, and rationale. Generate method rows, capability names, repo scope, and CLI route data from the contract. RFCs should stay forward-looking design proposals; do not use them as prompt context dumps or archive buckets.

### R6 - trim unused cross-repo runner hooks

stated: The project should keep the smallest viable architecture.

actual: Some Go runner hooks are placeholder-shaped and not active contract methods (`go/cmd/striatumd/main.go:391-439`, `contracts/daemon_methods.json:68-260`).

mine: Remove or unexport placeholder methods until an accepted RFC wires them. Keep tested cancellation and describe/list behavior. This prevents future maintainers from assuming a feature exists because an interface name suggests it.

### R7 - make workflow generate the canonical authoring path

stated: Workflow definitions and repository files are durable provenance; live state is daemon PG.

actual: The CLI exposes `striatum workflow generate` with shape, lane-set, artifact root, lane commands, dry-run, and JSON output (`src/striatum/cli/parser.py:542-566`), and dispatch writes generated workflows through `striatum.workflow_generator` (`src/striatum/cli/dispatch.py:1122-1160`). Docs still include hand-editing and web generation paths (`docs/WRITING_WORKFLOWS.md:1-56`, `docs/HOW_TO_HUMAN.md:968-980`).

mine: Make CLI generation the public/default authoring story. Keep preview APIs and web helpers if useful, but stop presenting hand-edited JSON or React Flow editing as the normal path for a team adopting Striatum.

## 8. Functionality I'd add

stated: The product needs operator-facing functionality that improves local runs, not hosted management features.

actual: The TODO already points to useful local additions: escalation inbox partials, accepted-risk persistence, archive/corpus foundations, optional Git/PR integration, and auto-finalize defaults that remain blocked on product decisions (`docs/TODO.md:107-114`, `docs/TODO.md:268-279`).

mine: I would add four small capabilities after the boundary cleanup above, ordered by the new priorities.

First, build the human-principal escalation UX. The web UI's consumer is the human principal, not AI operators, so it should foreground blocked lanes, human checkpoints, stale leases, capability denials, review overrides, and "what should I do next?" actions across the active repos.

Second, keep the generated, capability-gated MCP manifest tight for live operations. Agents should see only methods their own token can call, with arguments generated from the contract. This reduces stale tool discovery and makes differentiated per-operator tokens operationally visible.

Third, persist accepted risk decisions in PostgreSQL. The minimum viable shape is a table keyed by repository, run, artifact/job, risk id, accepting role, timestamp, and rationale. Surface it in dashboard/read DTOs and make risk lint check it. Do not add a separate approval service.

Fourth, add read-only archive/corpus browsing over the daemon. The project already has archive/corpus TODOs and recent manifest work; expose list/show/export reads through the daemon so the human principal can inspect historical run artifacts without reopening SQLite or scraping files.

I would not add hosted auth, long-running cloud workers, external tracing, a feature-flag control plane, or a general external-contributor portal. Those would raise the operating burden and conflict with the product boundary.

## 9. Execution roadmap

stated: The repo should favor the smallest viable change.

actual: There is no need for a broad rewrite. The live architecture is already enforced enough to let deletion work proceed behind tests.

mine: I would run the work in this order:

1. Freeze new legacy growth. Keep the existing quarantine tests, but add a failure mode that requires every new SQLite, Python daemon, or Python `daemon_pg` reference to include a deletion issue or fixture justification. This is one small PR and prevents backsliding while other work proceeds.

2. Convert or delete legacy fixtures. Work from the quarantine map, not from intuition. For each allowed reference, decide "convert to PG/Go", "keep as isolated importer", or "delete as retired behavior." The acceptance test is a smaller allowlist after every PR.

3. Retire Python `daemon_pg` and the Python daemon. Keep bootstrap/admin ergonomics, but migrate implementation toward Go-owned admin/bootstrap surfaces or wrappers over the Go binary. The target is no second daemon/state substrate in the Python package.

4. Prove the install story. Add macOS and Linux package smoke that starts from a clean environment, installs the artifact, runs `adopt`, generates a workflow with `striatum workflow generate`, and completes a small run. This is the public-adoption gate.

5. Normalize the agent-facing MCP and docs. Make daemon MCP the live-operation surface generated from the contract, teach per-operator scoped tokens as normal operation, and keep docs aimed at AI operators, cold-start maintainership, and provenance.

6. Canonicalize workflow authoring. Make `striatum workflow generate` the documented path for a team adopting Striatum; demote hand-edited JSON and React Flow editing to advanced/internal paths.

7. Build human-principal escalation UX. After substrate cutover, focus the web UI on principal oversight across repos and operators: escalation inbox, blocked work, stale leases, review overrides, and next actions.

8. Generate authority docs from the method contract and clean up RFC/doc purpose. Once the method surface is stable, reduce hand-maintained duplication and keep RFCs forward-looking rather than archival or prompt-context storage.

## 10. Open questions

stated: Some open decisions are already tracked in TODO and RFC notes.

actual: `docs/TODO.md` still marks accepted-risk persistence, auto-finalize defaults, archive/corpus foundations, and optional Git/PR integration as partial or blocked (`docs/TODO.md:107-114`). RFC 0068 still asks whether remaining composites should stay out of production, and it flags legacy SQLite fixture conversion/deletion as the retirement gate (`docs/operator/plans/rfc-0068-go-daemon-port.md:46-51`).

mine: The open questions I would resolve before adding more functionality are:

- Which legacy SQLite fixtures still test behavior that matters to Striatum's current product? The answer should drive deletion, not sentiment about historical coverage.
- Which Python `daemon_pg` helpers are still needed only because Go bootstrap/admin parity is incomplete?
- What exact CI/package matrix should define "known green" for `main`, macOS, Linux, and PyPI install?
- What is the smallest human-principal escalation screen that materially improves 8+ operator / 3+ repo oversight?
- Should `daemon.migrate` remain the RPC method name now that `daemon migrate` is a retired SQLite CLI command? If kept, document it as PG migration/admin only.
- Is optional Git/PR integration still inside the standalone product boundary, and if so, is it local Git-only first? Anything hosted should require a separate product decision.
- Should cross-repo prepare/start grow into real behavior soon? If not, trim the placeholder interface now.

None of these block current local operation. They do affect whether Striatum is ready for the first adopting team rather than only the current maintainer's working checkout.
