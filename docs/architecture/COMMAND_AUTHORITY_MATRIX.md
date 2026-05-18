# Command Authority Matrix

Status: Phase 1 Go-port tracking
Date: 2026-05-18
Source inputs: `src/striatum/cli/parser.py`,
`src/striatum/cli/daemon_rpc_route.py`,
`src/striatum/daemon_rpc/registry.py`,
`src/striatum/daemon_rpc/server.py`,
`src/striatum/daemon_pg/handlers/`, and `go/pkg/`.

This matrix names the current authority path for every registered daemon RPC
method plus CLI commands that are intentionally outside the production workflow
mutation path. It is transition scaffolding for the architecture remediation
plan. Contract metadata and CLI route reference tables are generated in
`docs/architecture/DAEMON_METHOD_TABLES.md`; authority classification,
SQLite-dependency notes, and Go-port status still live here. Every new RPC
method or handwritten route map must update this file. Per D108, executable
guardrails in `tests/architecture/test_authority_guardrails.py` keep route
labels, capabilities, scopes, and CLI fallback cells aligned with the daemon
contract/runtime while this matrix remains curated for authority and status
classification. Go daemon handler coverage is also executable in
`go/cmd/striatumd/handler_coverage_test.go`, which fails if active contract
methods are missing Go handlers or regress to generic `not_implemented`
placeholders.

D107 / RFC 0068 supersedes D105. The Go columns below are no longer
D105-bounded reference material; they are the production-port backlog. Any
`placeholder` or SQLite-backed row is active debt before the Python daemon can
retire. D110 removed the SQLite-bound `daemon.migrate_repo_local`,
`dogfood.publish_on_behalf`, and `dogfood.surgical_recovery` RPC names from
the production contract; D112 removed `apply.reviewed_patch` as well. These
names no longer appear as registered methods, and stale calls audit as
`method_unknown`.

Legend:

- **python authority**: `pg` means a native Python Postgres handler is
  registered. `direct` means `DaemonRpcRouter` handles the method without
  `CLI_ROUTES`.
  `local_file_authoring` means the CLI implements a repository-file helper
  directly and daemon RPC fails closed instead of falling back.
- **go authority**: `real` means a production Go handler is registered.
  `placeholder` means the Go fixture returns `not_implemented`.
  Removed unsupported methods are absent from this table and audit as
  `method_unknown`.
- **sqlite dependency** names whether production execution can still open
  repo-local SQLite through dogfood compatibility helpers, local legacy
  service surfaces, or migration-only paths.

## Direct PostgreSQL Bootstrap/Admin Plane

These Python client/CLI touchpoints may configure or open daemon PostgreSQL
directly because they run before a daemon is healthy, inspect daemon health, or
guard local authoring against active runs. This is not a general live-state
mutation escape hatch: ordinary workflow state changes must route through
daemon RPC. `tests/architecture/test_authority_guardrails.py` scans for these
imports and fails on unlisted direct PostgreSQL client helpers.

| File/function | Allowed surface | Direct PostgreSQL helper imports | Constraint |
|---|---|---|---|
| `src/striatum/day_zero.py::<module>` | guided adoption, first-run smoke, service status helpers | `resolve_config`, `connect`, `connect_and_migrate`, `doctor` | day-zero setup/diagnostic only; workflow mutations still route through daemon RPC |
| `src/striatum/day_zero.py::adopt` | guided first-run repo registration | `repo_add_pg` | initializes operational scratch and repository registration only |
| `src/striatum/cli/dispatch.py::_dispatch_daemon` | `daemon doctor`, lifecycle/status/audit/sweep/service commands | `client_admin`, `doctor` | daemon-global admin/diagnostic plane only |
| `src/striatum/cli/dispatch.py::_daemon_doctor_repo_cutover_report` | `daemon doctor --repo --authority` cutover verification | `resolve_config` | verify-only repository cutover report |
| `src/striatum/cli/dispatch.py::_daemon_authority_report` | authority diagnostic report | `client_admin` | diagnostic environment/escape reporting only |
| `src/striatum/cli/dispatch.py::_dispatch_daemon_repo` | `repo add/list/remove` CLI bridge | `client_admin` | calls PostgreSQL admin client helpers; no repo-local SQLite state |
| `src/striatum/cli/dispatch.py::_dispatch_cross_repo` | paired legacy test-harness fallback for cross-repo commands | `connect_and_migrate` | production path refuses before this branch unless the paired test-harness escape is enabled |
| `src/striatum/cli/workflow.py::_running_runs_for_workflow_pg` | local workflow-upgrade running-run guard | `resolve_config`, `connect` | read-only guard; fail closed when daemon PostgreSQL state is unknown |

## Registered Daemon Methods

| RPC method | CLI command | Capability | Scope | Python authority | Go authority | CLI fallback | SQLite dependency | Status |
|---|---|---:|---|---|---|---:|---|---|
| `daemon.hello` | n/a | none | daemon_global | direct | real | no | no | stable |
| `daemon.describe` | n/a | read | daemon_global | direct | real | no | no | stable |
| `status` | `status` | read | single_repo | pg | real | no | no | stable |
| `why` | `why` | read | single_repo | pg | real | no | no | stable |
| `doctor` | `doctor` | read | single_repo | pg | real | no | no | stable |
| `dashboard` | `dashboard` | read | single_repo | pg | real | no | no | stable |
| `evidence.export` | `evidence export` | read | single_repo | pg | real | no | no | stable |
| `corpus.export` | `corpus export` | read | single_repo | pg | real | no | no | stable |
| `archive.create` | `archive create` | read | single_repo | pg | real | no | no | Go V1 run archive writer |
| `run.summary` | `run summary` | read | single_repo | pg | real | no | no | stable |
| `run.detail` | web run detail DTO | read | single_repo | pg | real | no | no | stable |
| `job.detail` | web job detail DTO | read | single_repo | pg | real | no | no | stable |
| `run.graph` | `run graph` | read | single_repo | pg | real | no | no | stable |
| `run.events` | web SSE event stream DTO | read | single_repo | pg | real | no | no | stable |
| `run.posture_verdicts` | web posture verdict drill-down | read | single_repo | pg | real | no | no | stable |
| `workflow.validate` | `workflow validate` | read | single_repo | local_file_authoring | real | no | no live state | Go file-authoring validator; no PG state mutation |
| `workflow.plan` | `workflow plan` | read | single_repo | local_file_authoring | real | no | no live state | Go file-authoring plan projection |
| `workflow.graph` | `workflow graph` | read | single_repo | local_file_authoring | real | no | no live state | Go file-authoring JSON/Mermaid/DOT projection |
| `workflow.templates.list` | `workflow templates list` | read | single_repo | local_file_authoring | real | no | no live state | Go embedded catalog read; CLI remains local authoring surface |
| `workflow.templates.show` | `workflow templates show` | read | single_repo | local_file_authoring | real | no | no live state | Go embedded catalog read; CLI remains local authoring surface |
| `workflow.generate.preview` | web/chat preview | read | single_repo | not implemented in Python RPC | real | no | no live state | Go read-only planned-write preview |
| `list.runs` | `list runs` | read | single_repo | pg | real | no | no | stable |
| `list.sessions` | `list sessions` | read | single_repo | pg | real | no | no | stable |
| `list.jobs` | `list jobs` | read | single_repo | pg | real | no | no | stable |
| `list.artifacts` | `list artifacts` | read | single_repo | pg | real | no | no | stable |
| `artifact.show` | web artifact raw/detail DTO | read | single_repo | pg | real | no | no | stable |
| `list.workflows` | `list workflows` | read | single_repo | pg | real | no | no | stable |
| `worktree.list` | `worktree list` | read | single_repo | pg | real | no | no | stable |
| `dashboard.all` | `dashboard --all` | read | daemon_global | direct | real | no | no | Go/PostgreSQL read-only projection with per-active-run `run_progress` parity; remaining TODO 62 gaps are outside the dashboard-all run-progress slice |
| `repo.list` | `repo list` | read | daemon_global | pg repo registrar | real | no | no | bootstrap/admin |
| `repo.resolve` | client repository resolution | read | daemon_global | pg repo resolver | real | no | no | daemon-global bootstrap read for path -> repository_id resolution |
| `session.register` | `register-session` | claim | single_repo | pg | real | no | no | stable |
| `session.close` | `session close` | claim | single_repo | pg | real | no | no | stable |
| `work.claim_next` | `claim-next` | claim | single_repo | pg | real | no | no | stable |
| `work.ack` | `ack` | claim | single_repo | pg | real | no | no | stable |
| `work.heartbeat` | `heartbeat` | claim | single_repo | pg | real | no | no | stable |
| `work.release` | `release` | claim | single_repo | pg | real | no | no | stable |
| `supervise.start` | `supervise start` | claim | single_repo | pg | real | no | no | Go process-control launch over PG supervisor rows and FIFO/helper transport |
| `supervise.send` | `supervise send` | claim | single_repo | pg | real | no | no | Go packet delivery with delivered-unacknowledged semantics |
| `supervise.report` | wrapper control report | claim | single_repo | pg | real | no | no | Go records direct control events and helper JSONL batches |
| `supervise.stop` | `supervise stop` | claim | single_repo | pg | real | no | no | Go terminal supervisor state update and process signaling |
| `supervise.status` | `supervise status` | read | single_repo | pg | real | no | no | read-only liveness/stall projection; no pointer repair or lost-state mutation |
| `supervise.list` | `supervise list` | read | single_repo | pg | real | no | no | stable |
| `supervise.reattach_status` | supervisor reattach-status DTO | read | single_repo | pg | real | no | no | read-only reattach DTO |
| `work.send_message` | `send` | write | single_repo | pg | real | no | no | stable |
| `work.block` | `block` | write | single_repo | pg | real | no | no | stable |
| `work.complete` | `complete` | write | single_repo | pg | real | no | no | stable |
| `artifact.publish` | `publish-artifact` | write | single_repo | pg | real | no | no | stable |
| `worktree.create` | `worktree create` | write | single_repo | pg | real | no | no | Go shells out to `git worktree add --detach` after PG lease/workflow validation |
| `worktree.release` | `worktree release` | write | single_repo | pg | real | no | no | Go shells out to `git worktree remove --force` and records release state |
| `workflow.init` | `workflow init` | write | single_repo | local_file_authoring | real | no | no live state | Go scaffold writer; refuses unsafe paths/overwrites |
| `workflow.generate` | `workflow generate` | write | single_repo | local_file_authoring | real | no | no live state | Go generator writer; refuses unsafe paths/overwrites |
| `workflow.upgrade` | `workflow upgrade` | write | single_repo | local_file_authoring | real | no | PG running-run guard only; no Go SQLite import | Go upgrade supports harness-profile updates and `--add-phases` V1.1 rewrites |
| `review.submit` | `submit-review` | review | single_repo | pg | real | no | no | stable |
| `review.verdict` | `verdict` | review | single_repo | pg | real | no | no | stable |
| `review.override` | `override-verdict` | admin | single_repo | pg | real | no | no | stable |
| `decision.record` | `decision record` | admin | single_repo | pg | real | no | no | stable |
| `checkpoint.resolve` | `checkpoint resolve` | admin | single_repo | pg | real | no | no | stable |
| `escalation.list` | `escalation list`; `inbox` without `--session-id` | read | single_repo | pg | real | no | no | stable |
| `escalation.show` | `escalation show` | read | single_repo | pg | real | no | no | stable |
| `escalation.resolve` | `escalation resolve` | admin | single_repo | pg | real | no | no | stable |
| `branch.confirm` | `branch confirm` | admin | single_repo | pg | real | no | no | stable |
| `run.prepare` | `run prepare` | admin | single_repo | pg | real | no | no | stable |
| `run.start` | `run start` | admin | single_repo | pg | real | no | no | stable |
| `run.pause` | `run pause` | admin | single_repo | pg | real | no | no | stable |
| `run.resume` | `run resume` | admin | single_repo | pg | real | no | no | stable |
| `run.cancel` | `run cancel` | admin | single_repo | pg | real | no | no | stable |
| `run.retry_job` | `run retry-job` | admin | single_repo | pg | real | no | no | stable |
| `repo.init` | `init` | admin | single_repo | bootstrap CLI helper | real | no | no Go SQLite import; Python bootstrap compatibility remains | Go registers PG-backed repo state and operational scratch only |
| `recovery.stale_leases` | `recovery stale-leases` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.requeue_stale` | `recovery requeue-stale` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.cancel_job` | `recovery cancel-job` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.process_reconcile` | `recovery process-reconcile` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.resume` | `recovery resume` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.sweep` | `recovery auto` | recovery | single_repo | pg | real | no | no | canonical one-shot recovery sweep; runs workflow-opt-in auto-finalize before lazy lease expiry |
| `recovery.auto_publish_stale_artifacts` | `recovery auto-publish` | recovery | single_repo | pg | real | no | no | explicit stale-artifact auto-publish |
| `recovery.auto` | deprecated alias | recovery | single_repo | pg alias | real | no | no | deprecated compatibility alias for stale-artifact auto-publish; current CLI does not emit it |
| `recovery.auto_finalize` | `recovery auto-finalize` | recovery | single_repo | pg | real | no | no | dry-run by default; Go handler registered; live mode requires workflow opt-in or force |
| `apply.receipt.show` | n/a | read | single_repo | direct apply service | real | no | no | stable |
| `apply.receipt.verify` | n/a | read | single_repo | direct apply service | real | no | no | stable |
| `repo.add` | `repo add` | admin | daemon_global | pg repo registrar | real | no | no ordinary repo-local SQLite | bootstrap/admin |
| `repo.remove` | `repo remove` | admin | daemon_global | pg repo registrar | real | no | no | bootstrap/admin |
| `daemon.token.create` | n/a | admin | daemon_global | not implemented in Python RPC | real | no | no | Go PostgreSQL token issuance; cleartext token returned once |
| `daemon.token.revoke` | n/a | admin | daemon_global | not implemented in Python RPC | real | no | no | Go PostgreSQL token revocation by token id or full token |
| `daemon.token.rotate` | n/a | admin | daemon_global | not implemented in Python RPC | real | no | no | Go PostgreSQL token rotation with ambiguous-scope refusal |
| `daemon.key.rotate` | n/a | admin | daemon_global | not implemented in Python RPC | real | no | no | Go rotates the local Ed25519 sealed-apply fallback key file and returns key id/public-key metadata; full apply-gate mutation remains separate |
| `daemon.shutdown` | `daemon stop` out of band | admin | daemon_global | daemon lifecycle helper | real | no | no | Go process-cancel hook returns accepted shutdown response; handler still fails closed only when embedded without a hook |
| `daemon.migrate` | `daemon migrate` | admin | daemon_global | migration CLI helper | real | no | no | Go applies embedded PostgreSQL migrations; no SQLite/Python dependency |
| `cross_repo.list` | `cross-repo list` | read | cross_repo | direct cross-repo service | real | no | no | stable |
| `cross_repo.describe` | `cross-repo describe` | read | cross_repo | direct cross-repo service | real | no | no | stable |
| `cross_repo.why` | `cross-repo why` | read | cross_repo | direct cross-repo service | real | no | no | stable |
| `cross_repo.cancel` | `cross-repo cancel` | recovery | cross_repo | daemon RPC + PG participant cancel | real | no | no | stable |

## Deprecated Alias Methods

These pre-RFC-0043 method names remain registered so older clients receive a
known response. They should not be emitted by current CLI routing.

| Alias method | Canonical method | Python authority | Go authority | CLI fallback | Status |
|---|---|---|---|---:|---|
| `ack` | `work.ack` | pg | real | no | deprecated |
| `heartbeat` | `work.heartbeat` | pg | real | no | deprecated |
| `release` | `work.release` | pg | real | no | deprecated |
| `block` | `work.block` | pg | real | no | deprecated |
| `complete` | `work.complete` | pg | real | no | deprecated |
| `publish_artifact` | `artifact.publish` | pg | real | no | deprecated |
| `claim_next` | `work.claim_next` | pg | real | no | deprecated |
| `verdict` | `review.verdict` | pg | real | no | deprecated |
| `submit_review` | `review.submit` | pg | real | no | deprecated |

## CLI-Only Or Out-Of-Band Commands

These commands are in `parser.py` but are not production workflow mutation
methods routed by `daemon_rpc_route.py`. Some remain valid bootstrap/admin
helpers; others are local authoring or legacy service surfaces that later
remediation phases should either daemon-route, quarantine, or delete.

| CLI command | Current authority | SQLite dependency | Classification |
|---|---|---|---|
| `init` | `dispatch.py` -> `init_repo` | yes, bootstrap compatibility | bootstrap_admin |
| `adopt` | guided bootstrap over init/install/scaffold + PG repo migration | intentional bootstrap/migration only | bootstrap_admin |
| `skills install` | local filesystem installer | no workflow state | bootstrap_admin |
| `plugin install` / `plugin uninstall` | local filesystem installer | no workflow state | bootstrap_admin |
| `self-update` | local pip + installer helper | no workflow state | bootstrap_admin |
| `daemon start` | daemon lifecycle launcher | no repo-local workflow SQLite | bootstrap_admin |
| `daemon service install` / `start` / `status` | local service-manager helper; Go start forwards resident recovery scheduler flags | no workflow state | bootstrap_admin |
| `daemon doctor` | daemon PG doctor plus disabled legacy-registry diagnostic; registry probe only when PG diagnostics are unavailable or under explicit fixture escapes | diagnostic/test-only registry probe | bootstrap_admin |
| `doctor --first-run` | day-zero smoke over daemon socket, PG doctor, token, MCP, and sample read route | no workflow state | bootstrap_admin |
| `daemon migrate` | retired compatibility refusal | no SQLite open | bootstrap_admin |
| `daemon migrate-repo-local` | retired compatibility refusal; verify evidence lives under `daemon doctor --repo --authority` | no SQLite open | bootstrap_admin |
| `daemon status` / `stop` / `health` / `audit` / `sweep` | daemon lifecycle helpers | PostgreSQL daemon audit/metadata paths; sweep owns PostgreSQL scheduler cursors | bootstrap_admin |
| `cross-repo list` / `describe` / `why` | daemon RPC cross-repo helpers | no | daemon_read_out_of_band |
| `cross-repo cancel` | daemon RPC + PG participant cancel | no | daemon_recovery |
| `workflow validate` / `lint` / `plan` / `graph` | local authoring helpers; daemon RPC fails closed | no live state | local_file_authoring |
| `workflow init` / `generate` / `templates` | local authoring helpers; daemon RPC fails closed | no live state | local_file_authoring |
| `workflow upgrade` | local authoring helper with PG running-run guard | PostgreSQL-only running-run check; fails closed when PG state is unknown and never opens repo-local SQLite | local_file_authoring |
| `recovery watch` | foreground scheduler repeatedly calling daemon `recovery.sweep` | no production SQLite | daemon_scheduler |
| `run graph` | daemon RPC to PG handler | no | daemon_native |
| `send` | daemon RPC to PG handler | no | daemon_native |
| `adapter run` | retired outside explicit legacy test fixtures | no production SQLite | legacy fixture only |
| `byline` | retired outside explicit legacy test fixtures | no production SQLite | legacy fixture only |
| `inbox --session-id` | retired outside explicit legacy test fixtures | no production SQLite | legacy fixture only |
| `serve` | local HTTP/web service over daemon RPC plus explicit CLI-local authoring helpers | no production SQLite | service cleanup debt |

## Immediate Findings

1. Handwritten daemon fallback route tables are gone. Runtime CLI route
   translation now comes from the contract `cli_routes` map plus CLI-local
   parameter extraction, and production route-layer failures fail closed.
2. `recovery.sweep` is now the canonical RFC 0020 one-shot recovery
   sweep emitted by `striatum recovery auto`. `recovery auto-publish`
   emits the explicit `recovery.auto_publish_stale_artifacts` method.
   `recovery.auto` remains only as a deprecated compatibility alias for
   older stale-artifact auto-publish clients. `striatum recovery watch`
   is CLI-local scheduler glue over `recovery.sweep`, not a registered
   `recovery.watch` RPC method.
3. `repo.add`, `repo.list`, and `repo.remove` now route through daemon RPC
   and register against `striatumd.repositories` without opening or creating
   `.striatum/state.sqlite3`; `--init` creates only operational scratch.
4. `repo.resolve` is a daemon-global bootstrap read because repository-scoped
   authorization cannot know the repository id before resolution. Python CLI
   and service clients now resolve repositories through daemon RPC instead of
   direct PostgreSQL imports.
5. Both Python and Go daemon routes now fail closed for SQLite-bound dogfood
   composites. Operators should use primitive daemon methods (`work.ack`,
   `artifact.publish`, `review.verdict`, `work.complete`, and ordinary
   `recovery.*`) until a PostgreSQL-native composite is designed.
6. Go daemon startup now owns the resident active-run recovery scheduler:
   it calls Go `recovery.sweep`, records `daemon.recovery_sweep`, and upserts
   `striatumd.scheduler_cursors` without production SQLite.
7. `/v1/invoke`, local MCP, and web chat tools route daemon-mapped production
   reads and mutations through daemon RPC; `striatum.api.invoke` remains only
   for local authoring and explicit test/fixture compatibility paths.
8. `striatum.db` remains the legacy SQLite engine, but substrate-neutral
   helpers now live in `primitives.py` and `repo_policy.py`; guardrails keep
   daemon PG/RPC production modules from importing SQLite helpers.
9. Go no longer has generic `not_implemented` handlers for active contract
   methods. D110 removed the SQLite-bound dogfood composites and the
   repo-local SQLite import RPC from the production contract; D112 removed
   `apply.reviewed_patch`. Removed names stay absent from the registry and
   stale MCP/RPC calls return `method_unknown`; contract, architecture, and
   MCP/RPC tests pin that behavior. Web/service DTO parity gaps are tracked
   separately under RFC 0069-0071.
10. Daemon MCP resources (`resources/list` and `resources/read`) use
    PostgreSQL-backed repository visibility and read projections. A missing
    daemon PostgreSQL connection fails closed; the legacy registry-backed
    no-`pg_conn` fallback is retired.
