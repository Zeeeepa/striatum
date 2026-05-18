# STRIATUM Architecture Review - CODEX_GPT_5 - 2026-05-18
author: reviewer-codex-gpt-5-001

## 0. Files reviewed

stated: The architecture-review prompt asks for an architecture review of the project rooted at the current working directory, with every reviewed file listed before any claims. The repo instructions additionally say to start with `README.md`, `docs/INDEX.md`, `docs/SPEC.md`, `docs/DECISION_LOG.md`, `docs/UBIQUITOUS_LANGUAGE.md`, and `docs/TODO.md`.

actual: I reviewed the Striatum working tree during the 2026-05-18 Go/PostgreSQL daemon remediation pass. The first review snapshot was taken before the daemon CLI/admin cleanup commit; this document has been refreshed to note that production daemon dispatch now uses `src/striatum/daemon_pg/client_admin.py` and the CLI-side legacy daemon registry wrapper has been removed.

mine: I treated the dirty files as live architecture evidence because they affect the Go/PG cutover boundary. I did not execute tests or edit source.

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
- `go/pkg/crossrepo/lifecycle.go`
- `go/pkg/crossrepo/lifecycle_test.go`
- `go/pkg/crossrepo/prepare.go`
- `go/pkg/rpc/registry.go`
- `go/pkg/rpc/registry_methods.go`
- `tests/architecture/test_authority_guardrails.py`
- `tests/architecture/test_legacy_sqlite_quarantine.py`
- `tests/cli/test_daemon_core.py`
- `tests/cli/test_dispatch_daemon_doctor.py`

## 1. Executive summary

stated: Striatum is supposed to be a standalone, local-first workflow runner for terminal-based AI coding agents, with no hosted control plane, no transcript capture, and no telemetry (`README.md:3-9`, `docs/SPEC.md:10-18`). The authoritative state model is a daemon-owned PostgreSQL database scoped by registered target repository, while `.striatum/` is scratch (`README.md:13-43`, `docs/SPEC.md:20-39`, `docs/POSTGRES_TRANSITION.md:12-42`). The current product direction is also explicit: Go is the production daemon target, Python remains useful as CLI/web/client code, and the old Python daemon plus SQLite substrate are transitional deletion targets (`docs/SPEC.md:41-46`, `docs/DECISION_LOG.md:29-33`).

actual: The implementation is much closer to that stated architecture than the shape of the codebase first suggests. `striatum daemon start` now launches the Go daemon (`src/striatum/cli/daemon.py:35-50`), `striatumd` is a shim into that path rather than an import of the Python daemon (`src/striatum/daemon_entrypoint.py:1-17`), the parser rejects `--core python` and leaves only a no-op `--core go` compatibility flag (`src/striatum/cli/parser.py:254-381`), and the Go daemon refuses to bind before PostgreSQL is configured and migrated (`go/cmd/striatumd/main.go:81-177`). Active method coverage is guarded by generated Go registry data and a handler coverage test that fails if an active method is absent or still wired to a `not_implemented` handler (`go/pkg/rpc/registry_methods.go:5-107`, `go/cmd/striatumd/handler_coverage_test.go:40-69`).

mine: I do not see a present P0 architecture failure in the live path. The important risks are serious but bounded: large legacy SQLite/Python-daemon code remains packaged and importable; service and MCP compatibility surfaces still expose a broader CLI-shaped facade than the north-star architecture needs; direct PostgreSQL bootstrap/admin code is necessary but should be named as a bounded plane; and the Go daemon has weak release provenance because it still reports `go-dev`. The next maintainer work should be deletion and boundary tightening, not new orchestration infrastructure.

stated: The review should not recommend Kubernetes, cloud services, telemetry, or a multi-person operating model.

actual: The local architecture already has the right primitive set for a single operator: a local Go process, a local PostgreSQL instance, Unix socket auth with a local runtime token, local web/service clients, and filesystem scratch for supervised agent process material (`README.md:40-43`, `src/striatum/daemon_runtime.py:24-73`, `go/cmd/striatumd/main.go:143-225`).

mine: The north-star should be smaller than the current repo, not more elaborate. Delete the old Python daemon and most SQLite compatibility instead of adding abstractions around them. Make the daemon method contract the single routing surface. Keep Python where it is a client, renderer, authoring helper, or test harness, not as a second live-state backend.

## 2. What the project is trying to be

stated: Striatum's product boundary is clear and unusually useful: it is a local workflow runner for terminal coding agents, not an Engram-specific script, not a SaaS product, and not a hosted orchestration platform (`README.md:3-9`, `docs/UBIQUITOUS_LANGUAGE.md:10-15`). The docs define durable repository files as provenance and workflow definitions, while live workflow state belongs to the daemon PostgreSQL store (`docs/SPEC.md:20-39`, `docs/UBIQUITOUS_LANGUAGE.md:33`). The language guide also explicitly separates target repositories, workflow fixtures, runner state, artifacts, adapters, lanes, sessions, and work packets so that older Engram incubation vocabulary does not leak back into product code (`docs/UBIQUITOUS_LANGUAGE.md:216-221`).

actual: The CLI and docs reflect the local-first intent. Quick start starts a local daemon and adopts a local target repo (`README.md:62-67`). PostgreSQL is expected to be operator-provided, not bundled in Docker as a hidden service (`docs/POSTGRES_TRANSITION.md:44-57`). The Makefile defaults to the Go core, builds a local `striatumd`, runs Go conformance, and packages platform binaries into the Python package data (`Makefile:3`, `Makefile:83-121`, `pyproject.toml:43-57`). The daemon runtime token and socket live in a local runtime directory with owner-only permissions where the platform allows it (`src/striatum/daemon_runtime.py:24-73`).

mine: The architecture target is a single-machine control loop with auditable local state. The right comparison class is not Airflow, Temporal, GitHub Actions, or a managed queue. It is closer to a durable local supervisor plus a structured task ledger for human-operated coding-agent runs. That makes the current Go/PG direction right: process lifetime, auth, method dispatch, leases, and audit metadata belong in one daemon; Python can remain an excellent operator interface and renderer.

stated: The daemon is now a hard prerequisite for Striatum verbs, and the retired `--no-daemon` / repo-local SQLite era should not return (`docs/POSTGRES_TRANSITION.md:12-42`, `docs/UBIQUITOUS_LANGUAGE.md:190-192`).

actual: The CLI enforces daemon reachability by default, with only a paired test-harness opt-out using `STRIATUM_TEST_HARNESS=1` and `STRIATUM_DAEMON_REQUIRED=0` (`src/striatum/cli/daemon_required.py:1-20`, `src/striatum/cli/daemon_required.py:60-79`). It then routes mapped commands through a generated daemon RPC route table and fails closed when routing should happen but cannot (`src/striatum/cli/daemon_rpc_route.py:47-87`, `src/striatum/cli/dispatch.py:224-313`). The old migrate command names still parse but refuse before opening SQLite code (`src/striatum/cli/daemon.py:17-32`, `src/striatum/cli/parser.py:254-381`).

mine: The remaining ambiguity is not what Striatum wants to be; it is how quickly the repo can shed compatibility material from what it used to be. The docs have mostly caught up. The code still carries both the old and new worlds.

## 3. Current architecture

stated: The documented live architecture is CLI, MCP, and web clients talking to a local daemon, which owns PostgreSQL state (`README.md:13-38`). Repository-local `.striatum/` holds operational scratch such as FIFOs, pidfiles, and token cache material, not the live message bus (`README.md:40-43`, `docs/SPEC.md:64-70`).

actual: The main runtime path is now Go daemon first. `striatum daemon start` resolves a packaged, environment-provided, repo-local, or PATH Go binary, verifies its contract metadata, and execs it with socket, PostgreSQL, sweep, and migration-sha arguments (`src/striatum/cli/daemon.py:35-80`, `src/striatum/cli/daemon.py:107-150`). The Go process describes its method registry, requires a PostgreSQL URL, applies migrations, bootstraps the runtime token, sets up audit/request recording, registers handlers, listens on a Unix socket, and starts the recovery scheduler (`go/cmd/striatumd/main.go:81-225`). RPC methods are generated from `contracts/daemon_methods.json` into Go registry data (`contracts/daemon_methods.json:68-260`, `go/pkg/rpc/registry_methods.go:5-107`).

mine: This is the right core shape. The Python launcher contract check is particularly valuable because it catches stale packaged Go binaries before they become confusing local failures. The freshness check is not complete, though: it validates schema, migration count, and method etag, but not the daemon's release/version identity.

stated: Python remains allowed as CLI, web, authoring, and client code; Python daemon behavior is transitional (`docs/SPEC.md:41-46`, `docs/rfcs/0068-go-production-daemon-port.md:40-64`).

actual: Python now acts mostly as a client boundary. `service_daemon.call_repo_method` resolves the repo through daemon RPC and sends method calls through the socket, without opening PG or SQLite itself (`src/striatum/service_daemon.py:30-93`). `/v1/invoke` routes daemon-backed commands through the daemon route first (`src/striatum/service_api_routes.py:56-87`). Web run pages, actions, and artifact reads use daemon methods and only drop into legacy fixture fallbacks when the paired test harness permits it (`src/striatum/web/run_pages.py:53-294`, `src/striatum/web/run_actions.py:42-365`, `src/striatum/web/artifacts.py:78-135`). The terminal dashboard renders locally but collects its payload through the daemon unless explicitly in the legacy test harness (`src/striatum/dashboard.py:80-108`).

mine: This split is pragmatic: renderers and thin web handlers do not need to be in Go. The project should preserve Python client ergonomics while continuing to delete Python live-state ownership.

stated: Local authoring and fixture migration are special cases, not production state authority (`docs/SPEC.md:97-119`, `docs/DECISION_LOG.md:27`).

actual: The old Python daemon and SQLite modules remain. `src/striatum/daemon.py` still imports SQLite-backed registry code and contains legacy repository operations, but `connect_registry` refuses production use unless the paired test harness is enabled (`src/striatum/daemon.py:1-73`, `src/striatum/daemon.py:141-209`, `src/striatum/daemon.py:963-1030`). `src/striatum/cli/dispatch.py` still imports `sqlite3` and `striatum.db`, and it still contains a long local fallback dispatcher, but front-end enforcement and daemon RPC routing prevent mapped production commands from reaching that code (`src/striatum/cli/dispatch.py:5-25`, `src/striatum/cli/dispatch.py:224-313`, `src/striatum/cli/dispatch.py:686-1145`). The quarantine test intentionally classifies remaining SQLite references and fails on unclassified growth (`tests/architecture/test_legacy_sqlite_quarantine.py:21-179`, `tests/architecture/test_legacy_sqlite_quarantine.py:211-390`).

mine: The guards are real, but the code volume is still architectural risk. The correct long-term change is not another guardrail layer; it is converting the remaining fixtures and deleting the old substrate.

## 4. Strengths

stated: The docs say Striatum should have one authoritative live state and local-only operations.

actual: The code now backs that up for the active path. The Go daemon refuses to start without PostgreSQL, the CLI defaults to daemon-required execution, mapped CLI routes fail closed, and MCP daemon resources refuse to provide legacy local fallbacks when PG is missing (`go/cmd/striatumd/main.go:131-177`, `src/striatum/cli/daemon_required.py:60-79`, `src/striatum/cli/daemon_rpc_route.py:47-87`, `src/striatum/mcp.py:461-595`, `src/striatum/mcp.py:694-698`).

mine: The project has crossed the most important architectural threshold: old local files are no longer casually competing with daemon state for active workflow authority.

stated: Go daemon parity and method authority should be observable, not aspirational.

actual: The repo has several good guardrails. The Go handler coverage test instantiates handlers and fails active registered methods that are missing or still `not_implemented` (`go/cmd/striatumd/handler_coverage_test.go:40-69`). The Python authority tests check method registry coverage, matrix drift, fallback removal, route fail-closed behavior, and CLI production refusal before SQLite connect (`tests/architecture/test_authority_guardrails.py:138-459`). `daemon doctor --authority` reports PostgreSQL, legacy registry status, method counts, and fallback route counts from the same registries used at runtime (`src/striatum/cli/dispatch.py:1606-1714`).

mine: These are the right tests for a one-maintainer project. They turn architectural intent into cheap local checks instead of relying on a reviewer's memory.

stated: The old SQLite migration window is closed except for fixture paths.

actual: Retired migrate commands parse and refuse with a specific exit code before importing the old migration path (`src/striatum/cli/daemon.py:17-32`). The service and web layers gate legacy fallback behind paired test-harness env vars (`src/striatum/legacy_sqlite/service.py:29-51`, `src/striatum/service_command_policy.py:41-58`). `tests/architecture/test_legacy_sqlite_quarantine.py` is explicit about which remaining SQLite references are allowed and why (`tests/architecture/test_legacy_sqlite_quarantine.py:21-179`).

mine: The quarantine is a good temporary architecture. It should not become a permanent compatibility promise.

stated: Day-zero adoption should make local operation viable without cloud scaffolding.

actual: `adopt` initializes `.striatum/`, installs optional skills/plugins, optionally creates the DDD layout, registers the repo in PostgreSQL when configured, and offers a first-run smoke path that checks socket, token, PG doctor, registration, MCP, and a sample daemon read (`src/striatum/day_zero.py:72-181`, `src/striatum/day_zero.py:184-328`).

mine: This is exactly the kind of operator-centered feature Striatum needs. It reduces setup ambiguity without inventing a hosted control plane.

## 5. Concerns

### Concern 1 - legacy substrate still dominates the code shape

stated: D107/D111/D113 say production ownership moved to Go/PG and Python daemon/SQLite should retire from production and compatibility paths (`docs/DECISION_LOG.md:27-33`).

actual: The old substrate remains large and importable. `src/striatum/daemon.py` still implements legacy Python daemon operations and SQLite-backed registry access, albeit with production guards (`src/striatum/daemon.py:141-209`, `src/striatum/daemon.py:963-1030`). `src/striatum/cli/dispatch.py` still imports SQLite and still contains local implementations for many older commands behind routing gates (`src/striatum/cli/dispatch.py:5-25`, `src/striatum/cli/dispatch.py:686-1145`). The legacy quarantine map is broad enough to be its own subsystem (`tests/architecture/test_legacy_sqlite_quarantine.py:21-179`).

mine: This is serious, not blocking. I would downgrade any claim that production is currently broken, because the guards and route tests are strong. I would not downgrade the deletion work: a large quarantined backend still slows every architecture change and raises the chance that a future command accidentally creates a new bypass.

### Concern 2 - MCP and service still expose too much compatibility surface

stated: Daemon/MCP is mandatory for operator-driven runs, and marker files or provider hooks are not authoritative (`docs/operator/BRIEF.md:17-46`).

actual: The daemon RPC server has a capability-filtered method/resource surface and hides local workflow file authoring (`src/striatum/daemon_rpc/server.py:26-42`, `src/striatum/daemon_rpc/server.py:167-207`). But `src/striatum/mcp.py` also defines a `LocalRpcServer` that exposes older CLI-shaped tools and maps them through `invoke_argv_through_daemon_or_api` (`src/striatum/mcp.py:49-133`, `src/striatum/mcp.py:377-459`, `src/striatum/mcp.py:620-674`). The service layer similarly keeps lazy legacy wrappers and compatibility fallback points, even though production reads now use the daemon (`src/striatum/service.py:95-145`, `src/striatum/service_api_routes.py:163-234`).

mine: This is a boundary clarity problem. It is acceptable during the cutover, but the normal agent-facing MCP surface should be generated from the daemon method contract and capability filtering. Local authoring tools can remain, but they should be named and isolated as local file-authoring tools, not live workflow state tools.

### Concern 3 - bootstrap/admin direct PostgreSQL is necessary but under-named

stated: The daemon is described as a hard prerequisite for every Striatum verb (`docs/SPEC.md:20-39`, `docs/POSTGRES_TRANSITION.md:12-42`).

actual: Some commands necessarily operate before or beside a daemon. `adopt` and repo registration use direct PostgreSQL helpers when a daemon may not yet be running (`src/striatum/day_zero.py:72-141`, `src/striatum/day_zero.py:255-276`). `daemon doctor`, `repo add/list/remove`, `daemon status`, `daemon sweep`, and audit/health admin helpers use `src/striatum/daemon_pg/client_admin.py` directly from CLI dispatch (`src/striatum/cli/dispatch.py:1450-1539`, `src/striatum/cli/dispatch.py:1717-1746`, `src/striatum/daemon_pg/client_admin.py:1-5`, `src/striatum/daemon_pg/client_admin.py:205-725`). `daemon_required.py` also has a deliberate optional-command allowlist for `adopt`, `daemon`, `init`, skills, plugins, and self-update (`src/striatum/cli/daemon_required.py:36-49`).

mine: I agree with the implementation, but the architecture needs a name for this: a bounded bootstrap/admin plane. Without that name, future code will either over-constrain useful setup commands or use "bootstrap" as a loophole for live workflow mutations outside daemon RPC.

### Concern 4 - Go daemon release provenance is weak

stated: The release model packages Go daemon binaries into the Python distribution and checks their contract freshness (`pyproject.toml:54-57`, `Makefile:107-121`, `src/striatum/cli/daemon.py:107-150`).

actual: The Go daemon still reports `daemonVersion = "go-dev"` in source (`go/cmd/striatumd/main.go:81`). The Python launcher checks schema version, migration count, and method etag, but not a package version or build id (`src/striatum/cli/daemon.py:107-150`). Audit/request metadata is designed to include daemon identity, so an unchanging dev string reduces diagnostic value (`go/cmd/striatumd/main.go:143-177`).

mine: This is not a correctness blocker, but it is cheap operational debt. A single operator debugging a local installation benefits from being able to prove which binary is running.

### Concern 5 - the contract is real, but source-of-truth sprawl remains

stated: D108 makes the command authority matrix an architecture test input, and RFC 0068/0069 use contract and guardrail tests to control routing (`docs/DECISION_LOG.md:32`, `docs/rfcs/0068-go-production-daemon-port.md:90-157`, `docs/rfcs/0069-pg-only-daemon-global-surfaces.md:67-129`).

actual: Method metadata lives in `contracts/daemon_methods.json`, generated Go registry files, generated docs tables, a curated authority matrix, Python registries, and multiple guardrail tests (`contracts/daemon_methods.json:3-260`, `go/pkg/rpc/registry_methods.go:5-107`, `docs/architecture/DAEMON_METHOD_TABLES.md:7-177`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md:52-220`, `tests/architecture/test_authority_guardrails.py:138-459`).

mine: This is manageable because the tests are good. It is still a maintainer tax. The right fix is not a new service registry; it is to generate more of the docs and keep only human judgment as curated deltas.

### Concern 6 - some Go interfaces preserve future shapes the product does not yet use

stated: Cross-repo orchestration exists, but the project should prefer implemented local behavior over speculative surfaces.

actual: The Go cross-repo runner interface includes `Prepare`, `Start`, `Cancel`, `ParticipantIntact`, and `HumanCheckpoint`, but the local runner currently returns "not wired" for `Prepare` and fixed/no-op values for some other methods (`go/cmd/striatumd/main.go:391-439`). The active contract does not expose a `cross_repo.prepare` method, and current cancellation logic is implemented and tested (`go/pkg/crossrepo/lifecycle.go:94-165`, `go/pkg/crossrepo/lifecycle_test.go`).

mine: This is a smell, not a bug. Trim the interface to what is actually active unless a near-term RFC needs those hooks.

## 6. North-star architecture

stated: The desired architecture is local daemon plus PostgreSQL authority, Python clients, local scratch, and generic workflow terminology.

actual: The repo is already close enough that the north-star can be expressed as deletion plus consolidation:

- One production daemon: Go `striatumd`, method registry generated from the daemon contract, no selectable Python daemon core.
- One live-state store: daemon-owned PostgreSQL, keyed by registered target repository.
- One live-operation route: CLI/web/MCP call daemon RPC for registered workflow state changes and reads.
- One exception plane: bounded bootstrap/admin commands can configure PG, register repos, install service files, and inspect daemon health without requiring an already-healthy daemon.
- One local authoring plane: Python can create/edit workflow definition files, render UI, and perform non-live authoring tasks against the target repository.
- One compatibility fence: legacy SQLite remains only inside named fixture modules until converted or deleted.

mine: This north-star is intentionally unglamorous. It avoids distributed scheduling, queue brokers, cloud secret stores, feature-flag services, and telemetry. The single-operator burden drops when the codebase has fewer ways to do the same local thing.

## 7. Recommended changes

### R1 - delete the legacy Python daemon and SQLite substrate in stages

stated: RFC 0068 names Python daemon retirement and SQLite deletion as remaining work (`docs/rfcs/0068-go-production-daemon-port.md:143-157`, `docs/operator/plans/rfc-0068-go-daemon-port.md:26-35`).

actual: The production path is guarded, but legacy modules remain broad (`src/striatum/daemon.py:1-1030`, `src/striatum/cli/dispatch.py:686-1145`, `tests/architecture/test_legacy_sqlite_quarantine.py:21-179`).

mine: Convert the remaining legacy fixture tests to PG/Go where they protect current behavior, delete fixture-only coverage where it protects retired behavior, then remove `src/striatum/daemon.py`, `src/striatum/db.py`, old migration helpers, and most of `src/striatum/legacy_sqlite/`. Keep one tiny fixture importer only if a current test cannot be rewritten without losing valuable historical coverage. Acceptance: the quarantine allowlist shrinks instead of grows, and production code imports no `sqlite3` or `striatum.db` outside explicitly named fixture/import modules.

### R2 - define and test the bootstrap/admin plane

stated: The docs say daemon is mandatory, but current code needs pre-daemon operations.

actual: `adopt`, `daemon doctor`, repo registration, PG doctor, service install/start/status, and daemon admin commands already bypass daemon RPC by design (`src/striatum/day_zero.py:72-181`, `src/striatum/cli/dispatch.py:1450-1539`, `src/striatum/cli/dispatch.py:1717-1746`, `src/striatum/daemon_pg/client_admin.py:205-725`).

mine: Add a small architecture section and guardrail test that names the only allowed direct-PG bootstrap/admin commands. Make the test fail if a workflow live-state verb joins that set. This preserves useful setup behavior without weakening the "daemon owns workflow state" contract.

### R3 - make daemon MCP the normal live-operation MCP surface

stated: MCP should expose daemon-backed workflow state, not terminal output or old local marker assumptions.

actual: `DaemonRpcServer` is close to the intended shape, while `LocalRpcServer` still exposes CLI-shaped compatibility tools (`src/striatum/mcp.py:461-595`, `src/striatum/mcp.py:49-133`, `src/striatum/mcp.py:377-459`).

mine: Generate the live MCP tool/resource list from `contracts/daemon_methods.json` and capability metadata. Leave local authoring commands available only under clearly named local-authoring tools. Do not let an agent discover old live-state verbs through a CLI compatibility facade.

### R4 - stamp the Go daemon binary with release/build metadata

stated: Packaged Go daemon binaries are part of the release artifact.

actual: The source reports `go-dev`, and freshness checks do not verify binary version (`go/cmd/striatumd/main.go:81`, `src/striatum/cli/daemon.py:107-150`).

mine: Inject version, git SHA, contract etag, and migration SHA at build time or via a generated Go file. Make `striatumd --describe` expose them and make package smoke assert that the packaged binary matches the Python package version and contract etag. This is small and high-leverage for local support.

### R5 - generate more authority documentation from the method contract

stated: The authority matrix is an accepted test input, and the method tables are generated.

actual: The matrix contains curated state that overlaps with generated contract tables (`docs/architecture/COMMAND_AUTHORITY_MATRIX.md:52-220`, `docs/architecture/DAEMON_METHOD_TABLES.md:7-177`, `contracts/daemon_methods.json:3-260`).

mine: Keep the matrix only for human judgment: exceptional commands, bootstrap/admin exceptions, and rationale. Generate method rows, capability names, repo scope, and CLI route data from the contract. Acceptance: changing a method in `contracts/daemon_methods.json` regenerates docs and tests without hand-editing parallel tables.

### R6 - trim unused cross-repo runner hooks

stated: The project should keep the smallest viable architecture.

actual: Some Go runner hooks are placeholder-shaped and not active contract methods (`go/cmd/striatumd/main.go:391-439`, `contracts/daemon_methods.json:68-260`).

mine: Remove or unexport placeholder methods until an accepted RFC wires them. Keep tested cancellation and describe/list behavior. This prevents future maintainers from assuming a feature exists because an interface name suggests it.

### R7 - keep local authoring separate from live workflow state

stated: Workflow definitions and repository files are durable provenance; live state is daemon PG.

actual: Workflow generation preview routes through the daemon, but writing generated workflow files remains local authoring (`src/striatum/web/workflow_generation.py:64-158`). The daemon RPC server intentionally fails local authoring methods closed (`src/striatum/daemon_rpc/server.py:26-42`, `src/striatum/daemon_rpc/server.py:167-207`).

mine: Preserve that split. Do not force file-authoring tools into the daemon just for purity, but label them as local file operations and keep them out of live-state method claims.

## 8. Functionality I'd add

stated: The product needs operator-facing functionality that improves local runs, not hosted management features.

actual: The TODO already points to useful local additions: escalation inbox partials, accepted-risk persistence, archive/corpus foundations, optional Git/PR integration, and auto-finalize defaults that remain blocked on product decisions (`docs/TODO.md:107-114`, `docs/TODO.md:268-279`).

mine: I would add four small capabilities after the boundary cleanup above.

First, add a generated, capability-gated MCP manifest for live operations. Agents should see only methods they can call, with arguments generated from the contract. This reduces stale tool discovery and makes the daemon contract more valuable without changing the runtime model.

Second, persist accepted risk decisions in PostgreSQL. The minimum viable shape is a table keyed by repository, run, artifact/job, risk id, accepting role, timestamp, and rationale. Surface it in dashboard/read DTOs and make risk lint check it. Do not add a separate approval service.

Third, add read-only archive/corpus browsing over the daemon. The project already has archive/corpus TODOs and recent manifest work; expose list/show/export reads through the daemon so a maintainer can inspect historical run artifacts without reopening SQLite or scraping files.

Fourth, make first-run and package diagnostics a single operator command. `adopt --first-run-smoke` and `daemon doctor --authority` already have most pieces; consolidate their output into one JSON shape that says "binary, contract, PG, token, repo registration, MCP, dashboard read" with direct next actions.

I would not add team dashboards, hosted auth, long-running cloud workers, external tracing, or a feature-flag control plane. Those would raise the operating burden and conflict with the product boundary.

## 9. Execution roadmap

stated: The repo should favor the smallest viable change.

actual: There is no need for a broad rewrite. The live architecture is already enforced enough to let deletion work proceed behind tests.

mine: I would run the work in this order:

1. Freeze new legacy growth. Keep the existing quarantine tests, but add a failure mode that requires every new SQLite/Python-daemon reference to include a deletion issue or fixture justification. This is one small PR and prevents backsliding while other work proceeds.

2. Name the bootstrap/admin plane. Update the spec/architecture docs and add a guardrail test that enumerates direct-PG CLI/admin commands. This removes ambiguity around necessary setup operations before anyone deletes more code.

3. Convert or delete legacy fixtures. Work from the quarantine map, not from intuition. For each allowed reference, decide "convert to PG/Go", "keep as isolated importer", or "delete as retired behavior." The acceptance test is a smaller allowlist after every PR.

4. Remove the Python daemon module and SQLite live-state helpers. Do this only after the fixture decisions make the deletion boring. The target is that production packages no longer contain a second daemon implementation that a maintainer has to reason about.

5. Normalize the agent-facing MCP surface. Make daemon MCP the live-operation surface generated from the contract, then split local file-authoring tools into an explicit local namespace.

6. Stamp and smoke packaged Go binaries. Inject build metadata and assert packaged binary freshness in release/package smoke. This can land independently of the deletion work.

7. Generate authority docs from the method contract. Once the method surface is stable, reduce hand-maintained duplication and keep the authority matrix focused on exceptions and rationale.

8. Add local operator functionality: accepted-risk persistence, read-only archive browsing, and consolidated first-run diagnostics. These are worthwhile only after the daemon boundary is simpler; otherwise they will add more paths to migrate later.

## 10. Open questions

stated: Some open decisions are already tracked in TODO and RFC notes.

actual: `docs/TODO.md` still marks accepted-risk persistence, auto-finalize defaults, archive/corpus foundations, and optional Git/PR integration as partial or blocked (`docs/TODO.md:107-114`). RFC 0068 still asks whether remaining composites should stay out of production, and it flags legacy SQLite fixture conversion/deletion as the retirement gate (`docs/operator/plans/rfc-0068-go-daemon-port.md:46-51`).

mine: The open questions I would resolve before adding more functionality are:

- Which MCP server is the supported entry point for agents after cutover: daemon-only for live state, or local wrapper plus daemon routing? I recommend daemon-only for live state.
- Which legacy SQLite fixtures still test behavior that matters to Striatum's current product? The answer should drive deletion, not sentiment about historical coverage.
- Should `daemon.migrate` remain the RPC method name now that `daemon migrate` is a retired SQLite CLI command? If kept, document it as PG migration/admin only.
- Is optional Git/PR integration still inside the standalone product boundary, and if so, is it local Git-only first? Anything hosted should require a separate product decision.
- Should cross-repo prepare/start grow into real behavior soon? If not, trim the placeholder interface now.

None of these block current local operation. They do affect how much compatibility surface the sole maintainer must keep in their head.
