# STRIATUM Architecture Review

Date: 2026-05-17
Reviewer: Codex GPT-5
Status: architecture review artifact

## 0. Files Reviewed

- `/home/halbritt/git/prompts/ARCHITECTURE_REVIEW.md`
- `README.md`
- `docs/INDEX.md`
- `docs/SPEC.md`
- `docs/DECISION_LOG.md`
- `docs/UBIQUITOUS_LANGUAGE.md`
- `docs/TODO.md`
- `docs/POSTGRES_TRANSITION.md`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- `docs/architecture/DAEMON_METHOD_TABLES.md`
- `STRIATUM_ARCHITECTURE_REVIEW_2026-05-16.md`
- `STRIATUM_ARCHITECTURE_REMEDIATION_PLAN_2026-05-16.md`
- `pyproject.toml`
- `Makefile`
- `contracts/daemon_methods.json`
- `src/striatum/__init__.py`
- `src/striatum/api.py`
- `src/striatum/artifacts.py`
- `src/striatum/cli/daemon.py`
- `src/striatum/cli/daemon_required.py`
- `src/striatum/cli/daemon_rpc_route.py`
- `src/striatum/cli/dispatch.py`
- `src/striatum/cli/evidence.py`
- `src/striatum/daemon.py`
- `src/striatum/daemon_pg/config.py`
- `src/striatum/daemon_pg/connection.py`
- `src/striatum/daemon_pg/mcp_dispatch.py`
- `src/striatum/daemon_pg/migrations.py`
- `src/striatum/daemon_pg/repo_local_migration.py`
- `src/striatum/daemon_pg/sql/0001_baseline.sql`
- `src/striatum/daemon_rpc/registry.py`
- `src/striatum/daemon_rpc/request_log.py`
- `src/striatum/daemon_rpc/server.py`
- `src/striatum/day_zero.py`
- `src/striatum/db.py`
- `src/striatum/legacy_sqlite/service.py`
- `src/striatum/mcp.py`
- `src/striatum/service.py`
- `src/striatum/service_daemon.py`
- `src/striatum/web/chat_tools.py`
- `src/striatum/workflow.py`
- `go/cmd/striatumd/main.go`
- `go/cmd/striatum-supervisor-helper/main.go`
- `go/pkg/supervisor/helper.go`
- `tests/architecture/test_authority_guardrails.py`
- `tests/architecture/test_go_helper_boundary.py`
- `tests/architecture/test_legacy_sqlite_quarantine.py`

Repository inventory also used `git ls-files`, `find`, and `rg` output. A generated frontend bundle appeared in search output; I did not treat generated bundle code as reviewed source.

## 1. Executive Summary

stated: Striatum presents itself as "a local workflow runner for terminal-based AI coding agents" with daemon-owned PostgreSQL as live state, no hosted coordinator, no transcript capture, no telemetry, and no model-vendor SDK import path (`README.md:3-7`, `docs/SPEC.md:10-18`). The current implementation contract is sharper: daemon-owned PostgreSQL is authoritative for daemon-global and per-repository workflow state, `.striatum/` is operational scratch, and the daemon is required for every Striatum verb (`docs/SPEC.md:20-35`, `docs/SPEC.md:63-103`).

actual: The architecture has moved a long way toward that contract. There is a single JSON method contract (`contracts/daemon_methods.json:1-61`), a registry loaded from it (`src/striatum/daemon_rpc/registry.py:1-18`, `src/striatum/daemon_rpc/registry.py:225-238`), an empty daemon-side CLI fallback map (`src/striatum/daemon_rpc/server.py:26`), native PostgreSQL handler routing (`src/striatum/daemon_rpc/server.py:180-202`), metadata-only request/audit recording with hash-chain locking (`src/striatum/daemon_rpc/request_log.py:22-67`, `src/striatum/daemon_rpc/request_log.py:88-130`), and architecture tests that classify authority paths (`tests/architecture/test_authority_guardrails.py:86-133`). That is real progress.

actual: The biggest remaining issue is not product direction; it is authority ambiguity. `src/striatum/daemon.py` still owns a SQLite daemon registry (`src/striatum/daemon.py:97-180`), `repo_add` still initializes and registers repos through repo-local SQLite (`src/striatum/daemon.py:657-728`), daemon startup still opens that registry after bootstrapping PostgreSQL (`src/striatum/daemon.py:923-965`), and global dashboard, sweep, and MCP resource paths still read the SQLite registry and repo-local SQLite state (`src/striatum/daemon.py:1288-1422`, `src/striatum/daemon.py:1480-1593`). Separately, CLI dispatch can swallow unexpected daemon-route exceptions and continue into the legacy SQLite path (`src/striatum/cli/dispatch.py:245-254`, `src/striatum/cli/dispatch.py:556-560`).

mine: Striatum's thesis is strong: model the workflow, keep providers behind lanes, preserve local authority, and make provenance inspectable without capturing transcripts. The highest-value architecture work now is to make the substrate promise literally true in code. Until that happens, every feature is taxed by the question "which state authority did this path actually use?"

## 2. What Project Is Trying To Be

stated: The product is a standalone, generic runner for target repositories, not an Engram-specific script. Its core nouns are runs, sessions, lanes, leases, jobs, work packets, artifacts, verdicts, blockers, decisions, and daemon capabilities. `docs/UBIQUITOUS_LANGUAGE.md` says live state belongs in daemon-owned PostgreSQL, while durable provenance belongs in repository artifacts (`docs/UBIQUITOUS_LANGUAGE.md:28-45`, `docs/UBIQUITOUS_LANGUAGE.md:173-190`).

actual: The source reflects that domain model. Workflow validation is not a token wrapper around prompts; it validates schema versions, lanes, roles, edges, cycles, expected artifacts, review posture, apply gates, write scope, and bounded revision loops (`src/striatum/workflow.py:26-80`, `src/striatum/workflow.py:620-674`). The artifact layer understands durable artifact kinds and front-matter schemas for decisions, findings, syntheses, ledgers, proposals, and escalations (`src/striatum/artifacts.py:31-42`, `src/striatum/artifacts.py:159-275`). The evidence exporter is default-deny for free text rather than a broad transcript dump (`src/striatum/cli/evidence.py:28-48`).

mine: The project is trying to occupy a useful gap between plain "run an agent CLI" scripts and hosted agent orchestration platforms. Its differentiator is not launching processes. Its differentiator is making terminal-agent work auditable as a local workflow with explicit state transitions, bounded permissions, review gates, and durable artifacts.

stated: D105 says Python remains the primary production daemon core and Go is narrowed to helper/runtime work (`docs/DECISION_LOG.md:24-28`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md:20-22`).

actual: That strategic decision is partly encoded. The Go supervisor helper is cleanly narrow: `RunHelper` explicitly avoids Postgres, daemon RPC, workflow inspection, artifact publishing, job completion, and acknowledgements (`go/pkg/supervisor/helper.go:57-65`), and the architecture test forbids helper dependencies on DB/RPC/mutation/read/apply/cross-repo packages (`tests/architecture/test_go_helper_boundary.py:11-32`). But the repo still contains a launchable Go daemon with read/mutation registrations and many not-implemented placeholders (`go/cmd/striatumd/main.go:78-155`, `go/cmd/striatumd/main.go:170-209`), and the Python CLI can still select `python` or `go` as daemon core (`src/striatum/cli/daemon.py:19-80`).

mine: The product wants one production domain core, not two almost-cores. Go is valuable as a PTY/process helper. A launchable packaged Go daemon should either be explicitly developer-harness-only or removed from operator-facing launch paths.

## 3. Current Architecture

stated: The intended spine is simple:

```text
CLI / daemon MCP / local web UI
  -> daemon RPC method registry and capability checks
  -> Python daemon PostgreSQL handlers
  -> daemon-owned Postgres under repository_id scope
  -> durable repository artifacts, with .striatum as scratch
```

This is visible in the README diagram and text (`README.md:13-40`), the product boundary (`docs/SPEC.md:20-35`), and the generated method/CLI tables (`docs/architecture/DAEMON_METHOD_TABLES.md:5-40`, `docs/architecture/DAEMON_METHOD_TABLES.md:115-175`).

actual: The repository is now a substantial mixed codebase: 2,563 tracked files, including 631 under `src/striatum`, 212 test files, 1,540 docs files, and 64 Go files. Packaging is Python-first under the `striatum-orchestrator` distribution, with `striatum` and `striatumd` console scripts (`pyproject.toml:5-16`, `pyproject.toml:43-56`). Optional PostgreSQL support is a package extra (`pyproject.toml:32-35`). The Makefile codifies the expected contributor checks: lint, typecheck, tests, UI bundle checks, Go helper checks, PostgreSQL tests, multi-repo tests, smoke, and release checks (`Makefile:17-36`, `Makefile:38-80`, `Makefile:83-100`, `Makefile:120-170`).

actual: The daemon RPC layer is well-shaped. `DaemonRpcRouter.handle` enforces handshake, method registry lookup, repository scope, authorization, routing, and request/audit recording (`src/striatum/daemon_rpc/server.py:49-112`). `_route` sends PostgreSQL-backed methods to registered handlers, fails closed for local workflow authoring methods, and no longer has a generic `CLI_ROUTES` escape (`src/striatum/daemon_rpc/server.py:165-202`). MCP tool calls go through the same daemon router and audit path (`src/striatum/daemon_pg/mcp_dispatch.py:16-125`).

actual: The CLI route layer is less clean. `dispatch` performs daemon-required enforcement, then daemon route translation, then still contains the old in-process dispatch body behind `ensure_initialized(repo)` and `connect(repo)` (`src/striatum/cli/dispatch.py:192-254`, `src/striatum/cli/dispatch.py:556-560`). The route translator loads its method mappings from the contract, but repository id lookup connects directly to PostgreSQL from the client process (`src/striatum/cli/daemon_rpc_route.py:173-206`), and the RPC handshake still sends a hardcoded `client_version="1.51.0"` while the package version is 1.55.0 (`src/striatum/cli/daemon_rpc_route.py:129-140`, `pyproject.toml:5-8`).

actual: The local web service is partially modernized. The service docstring admits the transition state (`src/striatum/service.py:1-9`), production read paths call `service_daemon.call_repo_method` where DTOs exist (`src/striatum/service.py:499-520`, `src/striatum/service.py:590-620`), and legacy SQLite page fallbacks are quarantined under `legacy_sqlite/service.py` by tests (`tests/architecture/test_legacy_sqlite_quarantine.py:244-280`). But `/v1/invoke` still routes through `striatum.api.invoke` (`src/striatum/service.py:350-382`), and `service_daemon` shares the direct client-side PostgreSQL repo-id lookup with CLI routing (`src/striatum/service_daemon.py:29-52`).

mine: Architecturally, Striatum is in a late transition stage. The method contract, PG handlers, tests, and docs point in the right direction. The remaining substrate split is concentrated enough to finish, but still central enough to matter.

## 4. Strengths

stated: Striatum's docs insist that vocabulary is the model, not decorative bookkeeping (`docs/INDEX.md:22-35`, `docs/UBIQUITOUS_LANGUAGE.md:17-45`).

actual: The code validates and uses that vocabulary across workflow validation, artifact schemas, evidence redaction, daemon capabilities, and run-state methods. Same-model review risk is not left to prose; the linter detects same-model review pairs and revision cycles (`src/striatum/workflow.py:2279-2366`) and review jobs without fresh context (`src/striatum/workflow.py:2368-2387`). TODO item 55 shows this has been wired through CLI, web, generator preview, and validation policy (`docs/TODO.md:1032-1053`).

mine: This is the main asset. Striatum has resisted becoming a bag of prompt templates. It has a domain.

stated: The project says every daemon method should have one contract source and one auditable authority boundary (`docs/TODO.md:860-872`).

actual: `contracts/daemon_methods.json` drives registry loading, CLI route lookup, generated docs, and Go registry parity (`contracts/daemon_methods.json:1-61`, `src/striatum/daemon_rpc/registry.py:88-120`, `docs/architecture/DAEMON_METHOD_TABLES.md:1-8`). Guardrails classify every registered daemon method, assert no daemon CLI fallback routes, and assert local workflow authoring RPC methods fail closed (`tests/architecture/test_authority_guardrails.py:86-160`).

mine: This is the right kind of architecture scaffolding. It turns authority drift into a testable artifact instead of relying on reviewer memory.

stated: Audit is metadata-only and privacy-conscious, not transcript capture (`README.md:5-9`, `docs/DECISION_LOG.md:36-37`).

actual: The PostgreSQL schema has audit, request log, chain head, client, capability, and repository tables (`src/striatum/daemon_pg/sql/0001_baseline.sql:22-66`, `src/striatum/daemon_pg/sql/0001_baseline.sql:83-136`). Audit append locks the chain head `FOR UPDATE` so concurrent appenders serialize (`src/striatum/daemon_rpc/request_log.py:88-101`). Evidence redaction is typed and default-deny (`src/striatum/cli/evidence.py:28-48`).

mine: This gives Striatum a credible local trust story: record control-plane facts and hashes, not agent stdout and user prose.

stated: The web and Go helper migrations are supposed to narrow legacy surfaces, not enlarge them.

actual: `service.py` no longer imports SQLite directly, and the test suite enforces that (`tests/architecture/test_legacy_sqlite_quarantine.py:244-280`). The Go helper boundary is tight and tested (`go/pkg/supervisor/helper.go:57-65`, `tests/architecture/test_go_helper_boundary.py:21-32`).

mine: The project is not ignoring cleanup. It is actively adding fences. The remaining question is whether those fences are now enough, or whether some fenced code should be deleted.

## 5. Concerns

stated: Daemon-owned PostgreSQL is the authoritative live state, and `.striatum/` is scratch only (`README.md:40`, `docs/SPEC.md:29-39`, `docs/POSTGRES_TRANSITION.md:12-44`).

actual: `src/striatum/daemon.py` still creates and opens `striatumd.sqlite3` as a daemon registry (`src/striatum/daemon.py:97-180`). It still has SQLite tables for repositories, clients, capabilities, audit log, scheduler cursors, and daemon metadata (`src/striatum/daemon.py:183-190` and following schema block). `repo_add` requires or creates repo-local SQLite and inserts repository rows into the SQLite registry (`src/striatum/daemon.py:657-728`). `run_daemon_foreground` still opens the SQLite registry after PostgreSQL doctor/bootstrap (`src/striatum/daemon.py:923-965`). `dashboard_all`, daemon sweep, and MCP resource reads still walk SQLite registry rows and repo-local SQLite state (`src/striatum/daemon.py:1288-1422`, `src/striatum/daemon.py:1480-1593`).

mine: This is the top architectural mismatch. The current docs are too absolute relative to this code. Either those paths are still supported bootstrap/admin compatibility, in which case the docs need tighter qualifiers, or they are bugs against D094/D104 and should be ported or fail-closed.

stated: Mapped CLI verbs fail closed instead of falling back to SQLite (`docs/SPEC.md:41-55`, `docs/POSTGRES_TRANSITION.md:180-194`).

actual: The translator does raise on unreachable daemon or missing registration for mapped methods (`src/striatum/cli/daemon_rpc_route.py:78-89`). But `dispatch` catches any unexpected exception from `try_route` and silently continues (`src/striatum/cli/dispatch.py:245-254`). The fallback body then opens repo-local SQLite (`src/striatum/cli/dispatch.py:556-560`). The existing guardrail test covers missing socket before SQLite, not an unexpected translator/runtime exception after enforcement (`tests/architecture/test_authority_guardrails.py:197-233`).

mine: This is a concrete fail-closed hole. It may be rare, but it is exactly the kind of rare path that reintroduces split-brain behavior during incidents.

stated: CLI, MCP, and web are clients of the daemon (`docs/SPEC.md:12-18`, `docs/UBIQUITOUS_LANGUAGE.md:173-190`).

actual: Both CLI and service resolve `repository_id` by opening PostgreSQL directly from the client process (`src/striatum/cli/daemon_rpc_route.py:173-206`, `src/striatum/service_daemon.py:29-52`). That means a client can fail before reaching the daemon even if the daemon itself has the correct DB configuration. It also blurs the authority model: the client is not just a Unix-socket RPC client; it is also a direct database reader.

mine: This is probably a pragmatic bridge, but it should not remain the north-star. The daemon should resolve repo roots to repository ids, or the client should use a daemon-issued local token/cache that does not require client-side SQL.

stated: Python is the primary production daemon; Go is helper/runtime (`docs/DECISION_LOG.md:24-28`).

actual: `src/striatum/cli/daemon.py` still exposes a selectable Go daemon core (`src/striatum/cli/daemon.py:19-80`), the Makefile still stages full Go daemon binaries into wheel package data (`Makefile:101-118`), and `go/cmd/striatumd/main.go` still has a substantial daemon server (`go/cmd/striatumd/main.go:78-209`).

mine: Keeping a developer harness is fine. Shipping an operator-selectable second daemon core weakens D105 unless it is clearly hidden, unsupported, and tested only as harness evidence.

stated: Daemon MCP is the agent-facing mutation surface (`docs/UBIQUITOUS_LANGUAGE.md:202-207`).

actual: `DaemonRpcServer` does expose capability-filtered tools through the method registry (`src/striatum/mcp.py:461-576`). But `LocalRpcServer` still exposes an MCP-like wrapper backed by `striatum.api.invoke` (`src/striatum/mcp.py:377-458`), and chat dogfood lifecycle tools call `striatum.api.invoke` as well (`src/striatum/web/chat_tools.py:30-36`, `src/striatum/web/chat_tools.py:840-862`).

mine: This is mostly naming and lifecycle debt, but it matters for operator guidance. There should be one obvious production MCP story. The local invoke-backed surface should be marked legacy/local fixture or removed from the ordinary docs.

## 6. North-Star Architecture

stated: The remediation plan already names the right shape: daemon RPC/MCP -> one transition engine -> PostgreSQL -> durable artifacts (`STRIATUM_ARCHITECTURE_REMEDIATION_PLAN_2026-05-16.md:8-24`).

actual: The code is close enough that the north star does not require a rewrite. It requires deleting or moving remaining authority-like compatibility paths. PostgreSQL schema, PG handlers, method contract, capability authorization, audit logging, workflow validators, and service DTOs already exist.

mine: The target architecture should be:

```text
striatum CLI
daemon MCP
local web service
  -> Unix socket daemon RPC
  -> method registry and capability check
  -> Python PG handler or explicit local-file authoring refusal
  -> striatumd PostgreSQL schema
  -> durable repo artifacts and scratch files

Go
  -> PTY/process helper protocol only
```

mine: In that target, `src/striatum/db.py`, `src/striatum/cli/mutations.py`, old SQLite workflow creation, and legacy service readers live under a `legacy_sqlite` or `migration` namespace. Production modules do not import them. `repo add`, `repo list`, `dashboard --all`, daemon sweep, daemon MCP resources, and daemon startup all use the PostgreSQL registry. `striatum.api.invoke` becomes a compatibility wrapper for tests/local authoring only, not a mutation foundation.

mine: Local file authoring should remain allowed where it truly does not touch live workflow state: workflow validate, lint, plan, graph, templates, and workflow file generation. But the contract should call these "local authoring helpers" rather than implying they are daemon live-state methods.

## 7. Recommended Changes

stated: Phase 1 claims production daemon RPC fallback is closed, but remaining follow-up includes quarantining the legacy SQLite domain (`docs/TODO.md:845-858`).

actual: There is still an unexpected-exception CLI fallthrough and a live SQLite daemon registry.

mine: P0 change: make registered daemon-routed CLI commands fail closed on any route failure. Replace the broad catch in `dispatch` with a fail-closed `StriatumError` for commands present in the contract route lookup. Add a test that monkeypatches `try_route` to raise `RuntimeError` for `status` or `run start`, sets `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1`, and asserts dispatch raises without opening SQLite.

mine: P0 change: port or disable SQLite daemon registry surfaces. Start with `repo add/list/remove`, `dashboard.all`, daemon sweep, daemon MCP resources, and `_repo_path_for_id`. These are central daemon-global surfaces, not obscure migration readers. If a PG-native implementation is not ready, fail closed with a migration/admin message rather than reading `striatumd.sqlite3`.

mine: P1 change: add daemon-side repository resolution. A `repo.resolve` or envelope-supported `repo_root` resolution path would let CLI/service call the daemon over the socket without opening PostgreSQL directly. Keep direct PG only for bootstrap commands such as `daemon doctor`, migration, or admin repair.

mine: P1 change: remove the hardcoded client versions. `src/striatum/__init__.py` already reads `__version__` from package metadata (`src/striatum/__init__.py:9-17`); use it in CLI handshake and first-run smoke instead of `1.51.0` and `1.67.0` (`src/striatum/cli/daemon_rpc_route.py:136-140`, `src/striatum/day_zero.py:319-324`).

mine: P1 change: enforce D105 in operator surfaces. Remove `go` from `VALID_DAEMON_CORES`, or gate it behind a clearly named developer harness flag. Keep `striatum-supervisor-helper` and its boundary tests.

mine: P2 change: split the remaining local service command surface. `/v1/invoke` can stay as an ergonomic endpoint, but internally it should translate to daemon RPC for daemon-routed commands and only use `invoke` for local authoring helpers. That aligns the service with its own docstring (`src/striatum/service.py:188-195`).

mine: P2 change: keep the command authority matrix, but make it generated plus annotated. The generated method table is already useful; the non-generated matrix still carries human classifications (`docs/architecture/COMMAND_AUTHORITY_MATRIX.md:11-18`). Keep human rationale, but drive as much of the table as possible from the contract so drift gets cheaper to catch.

## 8. Functionality I'd Add

stated: The TODO roadmap is already cautious about new product surface. Git/PR integration is blocked on product decisions, and corpus v2 still has open RFC decisions (`docs/TODO.md:1094-1116`).

actual: The most useful additions are operator-facing diagnostics for the transition, not big new workflow features.

mine: Add `striatum doctor --authority --json`. It should report, for the current install, which commands are daemon-native, local authoring, bootstrap/admin, migration-only, legacy-fixture, or unsupported; whether the daemon registry is PostgreSQL-only; whether any repo-local SQLite file would be opened by production paths; and whether the current CLI/service can resolve the repo without direct SQL.

mine: Add `striatum repo resolve --json` as a daemon-backed command. It should show the repository id, repo root, lifecycle state, Postgres schema version, scratch status, and whether old `.striatum/state.sqlite3` files are source, tombstone, missing, or unsafe.

mine: Add a migration cleanup report. After `migrate-repo-local`, operators need one command that says: PG row exists, event chain reanchored, tombstone mode is correct, no production verb will open SQLite, and removable scratch candidates are limited to known transient paths.

mine: Add an archive/replay inspection command before adding any hosted or PR surface. TODO 59 already has local archive and replay foundations (`docs/TODO.md:1094-1109`). A local `archive inspect` view would strengthen the provenance story without crossing product boundaries.

mine: Add durable accepted-risk linkage only after the product decision requested by TODO 55. The risk lint is useful now, but accepted risk needs one explicit authority home: decision artifact, daemon audit row, run-preparation record, or workflow metadata (`docs/TODO.md:1045-1053`).

## 9. Execution Roadmap

stated: The current remediation backlog says several phases are done, but Phase 4 service cleanup, Phase 5 escalation policy, Phase 7 durable risk policy, Phase 8 default auto-finalize policy, Phase 11 corpus/archive v2, and Phase 12 Git/PR decisions remain open (`docs/TODO.md:879-966`, `docs/TODO.md:968-1116`).

actual: The immediate architecture blockers are narrower than the whole roadmap.

mine: Today or next slice:

- Fail closed on daemon-route exceptions in `dispatch`.
- Replace hardcoded RPC client versions with `striatum.__version__`.
- Add tests for both changes.

mine: Next one or two weeks:

- Make daemon repository registry reads and writes PostgreSQL-native for `repo add/list/remove`.
- Convert `dashboard.all`, daemon MCP resources, and daemon sweep to PG registry + PG handlers.
- Move remaining SQLite daemon registry code under an explicit legacy migration/admin namespace.
- Tighten docs where README/SPEC/transition claims are ahead of code.

mine: Next month:

- Remove client-side PostgreSQL repo-id lookup from CLI and service.
- Make `/v1/invoke` daemon-routed internally for production commands.
- Decide and enforce whether local invoke-backed MCP remains as a legacy adapter or disappears from normal guidance.
- Narrow Go daemon packaging to helper-only or developer-harness-only.

mine: Quarter-scale:

- Finish the escalation policy decision: artifact-only versus typed escalation table or stricter blocker payload schema.
- Decide durable accepted-risk persistence.
- Continue archive/replay/corpus v2 without adding external memory or hosted dependencies.
- Revisit Git/PR integration only after commit authority and hosted-provider boundaries are explicitly accepted.

## 10. Open Questions

stated: D094/D104/D105 are accepted decisions, not proposals (`docs/DECISION_LOG.md:24-28`).

actual: Some code still behaves like transitional compatibility is product surface.

mine: The questions I would resolve explicitly are:

- Is `repo add` still allowed to create and rely on repo-local SQLite, or should it now be PG-native registration plus scratch creation only?
- Should `striatum daemon start` ever open `striatumd.sqlite3` after PostgreSQL is configured, or should the old registry be readable only by a migration command?
- Is the local `striatum.mcp.LocalRpcServer` still a supported operator surface, or only a legacy/test adapter?
- Should the Go daemon binary remain packaged and selectable, or should Go be reduced to `striatum-supervisor-helper` plus tests?
- What is the durable authority for accepted workflow risk: decision artifact, daemon audit row, run preparation record, workflow metadata, or another explicit object?
- Should `workflow.generate.preview` be registered as a daemon method while not implemented in Python RPC (`docs/architecture/COMMAND_AUTHORITY_MATRIX.md:63`), or should it be marked local-authoring only until a real daemon route exists?
- How should docs state transition reality without weakening the product promise? My preference: SPEC should describe the intended contract, but any currently live compatibility path should be named and bounded in `POSTGRES_TRANSITION.md` and the authority matrix until deleted.

mine: I would not broaden product scope until these are settled. The project does not need hosted services, PR automation, model SDKs, or more UI surface to become architecturally cleaner. It needs the remaining authority split closed.
