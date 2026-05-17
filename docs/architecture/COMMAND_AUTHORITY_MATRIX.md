# Command Authority Matrix

Status: Phase 0 inventory
Date: 2026-05-16
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
method or handwritten route map must update this file and the guardrails in
`tests/architecture/test_authority_guardrails.py`. Go daemon handler coverage
is also executable in `go/cmd/striatumd/handler_coverage_test.go`, which
classifies contract methods as missing, placeholder-backed, or implemented.

D107 / RFC 0068 supersedes D105. The Go columns below are no longer
D105-bounded reference material; they are the production-port backlog. Any
`placeholder` or SQLite-backed row is active debt before the Python daemon can
retire.

Legend:

- **python authority**: `pg` means a native Python Postgres handler is
  registered. `direct` means `DaemonRpcRouter` handles the method without
  `CLI_ROUTES`.
  `local_file_authoring` means the CLI implements a repository-file helper
  directly and daemon RPC fails closed instead of falling back.
- **go authority**: `real` means a Go transition/harness handler is
  registered. `placeholder` means the Go fixture returns `not_implemented`.
  `fail_closed` means a handler exists but intentionally refuses the
  operation.
- **sqlite dependency** names whether production execution can still open
  repo-local SQLite through dogfood compatibility helpers, local legacy
  service surfaces, or migration-only paths.

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
| `run.graph` | `run graph` | read | single_repo | pg | placeholder | no | no | stable |
| `run.events` | web SSE event stream DTO | read | single_repo | pg | real | no | no | stable |
| `run.posture_verdicts` | web posture verdict drill-down | read | single_repo | pg | real | no | no | stable |
| `workflow.validate` | `workflow validate` | read | single_repo | local_file_authoring | placeholder | no | no live state | CLI-local |
| `workflow.plan` | `workflow plan` | read | single_repo | local_file_authoring | placeholder | no | no live state | CLI-local |
| `workflow.graph` | `workflow graph` | read | single_repo | local_file_authoring | placeholder | no | no live state | CLI-local |
| `workflow.templates.list` | `workflow templates list` | read | single_repo | local_file_authoring | placeholder | no | no live state | CLI-local |
| `workflow.templates.show` | `workflow templates show` | read | single_repo | local_file_authoring | placeholder | no | no live state | CLI-local |
| `workflow.generate.preview` | web/chat preview | read | single_repo | not implemented in Python RPC | placeholder | no | no | not implemented |
| `list.runs` | `list runs` | read | single_repo | pg | real | no | no | stable |
| `list.sessions` | `list sessions` | read | single_repo | pg | real | no | no | stable |
| `list.jobs` | `list jobs` | read | single_repo | pg | real | no | no | stable |
| `list.artifacts` | `list artifacts` | read | single_repo | pg | real | no | no | stable |
| `artifact.show` | web artifact raw/detail DTO | read | single_repo | pg | real | no | no | stable |
| `list.workflows` | `list workflows` | read | single_repo | pg | real | no | no | stable |
| `worktree.list` | `worktree list` | read | single_repo | pg | real | no | no | stable |
| `dashboard.all` | `dashboard --all` | read | daemon_global | direct | real | no | no | Go read-only subset; residual parity gaps documented in TODO 62 |
| `repo.list` | `repo list` | read | daemon_global | pg repo registrar | real | no | no | bootstrap/admin |
| `session.register` | `register-session` | claim | single_repo | pg | real | no | no | stable |
| `session.close` | `session close` | claim | single_repo | pg | placeholder | no | no | stable |
| `work.claim_next` | `claim-next` | claim | single_repo | pg | real | no | no | stable |
| `work.ack` | `ack` | claim | single_repo | pg | real | no | no | stable |
| `work.heartbeat` | `heartbeat` | claim | single_repo | pg | real | no | no | stable |
| `work.release` | `release` | claim | single_repo | pg | real | no | no | stable |
| `supervise.start` | `supervise start` | claim | single_repo | pg | placeholder | no | no | stable |
| `supervise.send` | `supervise send` | claim | single_repo | pg | placeholder | no | no | stable |
| `supervise.report` | wrapper control report | claim | single_repo | pg | placeholder | no | no | stable |
| `supervise.stop` | `supervise stop` | claim | single_repo | pg | placeholder | no | no | stable |
| `supervise.status` | `supervise status` | read | single_repo | pg | placeholder | no | no | stable |
| `supervise.list` | `supervise list` | read | single_repo | pg | placeholder | no | no | stable |
| `supervise.reattach_status` | supervisor reattach-status DTO | read | single_repo | pg | real | no | no | stable |
| `work.send_message` | `send` | write | single_repo | pg | placeholder | no | no | stable |
| `work.block` | `block` | write | single_repo | pg | real | no | no | stable |
| `work.complete` | `complete` | write | single_repo | pg | real | no | no | stable |
| `artifact.publish` | `publish-artifact` | write | single_repo | pg | real | no | no | stable |
| `worktree.create` | `worktree create` | write | single_repo | pg | placeholder | no | no | stable |
| `worktree.release` | `worktree release` | write | single_repo | pg | placeholder | no | no | stable |
| `workflow.init` | `workflow init` | write | single_repo | local_file_authoring | placeholder | no | no live state | CLI-local |
| `workflow.generate` | `workflow generate` | write | single_repo | local_file_authoring | placeholder | no | no live state | CLI-local |
| `workflow.upgrade` | `workflow upgrade` | write | single_repo | local_file_authoring | placeholder | no | PG running-run guard; legacy SQLite only before cutover | CLI-local with fail-closed cutover guard |
| `dogfood.publish_on_behalf` | MCP/chat dogfood tool | write | single_repo | direct dogfood helper | placeholder | no | yes, direct `striatum.db.connect` | dogfood compatibility |
| `review.submit` | `submit-review` | review | single_repo | pg | real | no | no | stable |
| `review.verdict` | `verdict` | review | single_repo | pg | real | no | no | stable |
| `review.override` | `override-verdict` | admin | single_repo | pg | real | no | no | stable |
| `decision.record` | `decision record` | admin | single_repo | pg | real | no | no | stable |
| `checkpoint.resolve` | `checkpoint resolve` | admin | single_repo | pg | real | no | no | stable |
| `escalation.list` | `escalation list`; `inbox` without `--session-id` | read | single_repo | pg | placeholder | no | no | stable |
| `escalation.show` | `escalation show` | read | single_repo | pg | placeholder | no | no | stable |
| `escalation.resolve` | `escalation resolve` | admin | single_repo | pg | real | no | no | stable |
| `branch.confirm` | `branch confirm` | admin | single_repo | pg | real | no | no | stable |
| `run.prepare` | `run prepare` | admin | single_repo | pg | real | no | no | stable |
| `run.start` | `run start` | admin | single_repo | pg | real | no | no | stable |
| `run.pause` | `run pause` | admin | single_repo | pg | real | no | no | stable |
| `run.resume` | `run resume` | admin | single_repo | pg | real | no | no | stable |
| `run.cancel` | `run cancel` | admin | single_repo | pg | real | no | no | stable |
| `run.retry_job` | `run retry-job` | admin | single_repo | pg | real | no | no | stable |
| `repo.init` | `init` | admin | single_repo | bootstrap CLI helper | placeholder | no | yes, bootstrap SQLite compatibility | bootstrap/admin |
| `recovery.stale_leases` | `recovery stale-leases` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.requeue_stale` | `recovery requeue-stale` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.cancel_job` | `recovery cancel-job` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.process_reconcile` | `recovery process-reconcile` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.resume` | `recovery resume` | recovery | single_repo | pg | real | no | no | stable |
| `recovery.sweep` | `recovery auto` | recovery | single_repo | pg | real | no | no | canonical one-shot recovery sweep; runs workflow-opt-in auto-finalize before lazy lease expiry |
| `recovery.auto_publish_stale_artifacts` | `recovery auto-publish` | recovery | single_repo | pg | real | no | no | explicit stale-artifact auto-publish |
| `recovery.auto` | deprecated alias | recovery | single_repo | pg alias | real | no | no | deprecated compatibility alias for stale-artifact auto-publish; current CLI does not emit it |
| `recovery.auto_finalize` | `recovery auto-finalize` | recovery | single_repo | pg | real | no | no | experimental/workflow-opt-in |
| `apply.reviewed_patch` | n/a | apply | single_repo | direct apply service | fail_closed | no | no | fail closed until apply authority |
| `apply.receipt.show` | n/a | read | single_repo | direct apply service | real | no | no | stable |
| `apply.receipt.verify` | n/a | read | single_repo | direct apply service | real | no | no | stable |
| `dogfood.surgical_recovery` | MCP/chat dogfood tool | surgical_recovery | single_repo | direct dogfood helper | placeholder | no | yes, direct `striatum.db.connect` | dogfood compatibility |
| `repo.add` | `repo add` | admin | daemon_global | pg repo registrar | real | no | no ordinary repo-local SQLite | bootstrap/admin |
| `repo.remove` | `repo remove` | admin | daemon_global | pg repo registrar | real | no | no | bootstrap/admin |
| `daemon.token.create` | n/a | admin | daemon_global | not implemented in Python RPC | placeholder | no | no | bootstrap/admin placeholder |
| `daemon.token.revoke` | n/a | admin | daemon_global | not implemented in Python RPC | placeholder | no | no | bootstrap/admin placeholder |
| `daemon.token.rotate` | n/a | admin | daemon_global | not implemented in Python RPC | placeholder | no | no | bootstrap/admin placeholder |
| `daemon.key.rotate` | n/a | admin | daemon_global | not implemented in Python RPC | placeholder | no | no | bootstrap/admin placeholder |
| `daemon.shutdown` | `daemon stop` out of band | admin | daemon_global | daemon lifecycle helper | placeholder | no | no | bootstrap/admin |
| `daemon.migrate` | `daemon migrate` | admin | daemon_global | migration CLI helper | placeholder | no | no repo-local workflow SQLite except source registry migration | migration |
| `daemon.migrate_repo_local` | `daemon migrate-repo-local` | admin | daemon_global | migration CLI helper | placeholder | no | yes, intentional source import | migration |
| `cross_repo.list` | `cross-repo list` | read | cross_repo | direct cross-repo service | real | no | no | stable |
| `cross_repo.describe` | `cross-repo describe` | read | cross_repo | direct cross-repo service | real | no | no | stable |
| `cross_repo.why` | `cross-repo why` | read | cross_repo | direct cross-repo service | real | no | no | stable |
| `cross_repo.cancel` | `cross-repo cancel` | recovery | cross_repo | daemon RPC + PG participant cancel | placeholder | no | no | stable |

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
| `daemon service install` / `start` / `status` | local service-manager helper | no workflow state | bootstrap_admin |
| `daemon doctor` | daemon PG doctor plus legacy registry probe | legacy registry probe only | bootstrap_admin |
| `doctor --first-run` | day-zero smoke over daemon socket, PG doctor, token, MCP, and sample read route | no workflow state | bootstrap_admin |
| `daemon migrate` | daemon registry migration helper | source registry migration only | legacy_migration |
| `daemon migrate-repo-local` | per-repo SQLite -> Postgres migration | intentional source SQLite import | legacy_migration |
| `daemon status` / `stop` / `health` / `audit` / `sweep` | daemon lifecycle helpers | daemon registry/audit paths | bootstrap_admin |
| `cross-repo list` / `describe` / `why` | daemon RPC cross-repo helpers | no | daemon_read_out_of_band |
| `cross-repo cancel` | daemon RPC + PG participant cancel | no | daemon_recovery |
| `workflow validate` / `lint` / `plan` / `graph` | local authoring helpers; daemon RPC fails closed | no live state | local_file_authoring |
| `workflow init` / `generate` / `templates` | local authoring helpers; daemon RPC fails closed | no live state | local_file_authoring |
| `workflow upgrade` | local authoring helper with PG running-run guard | legacy SQLite only before cutover; fails closed after cutover if PG unavailable | local_file_authoring |
| `recovery watch` | foreground scheduler repeatedly calling daemon `recovery.sweep` | no production SQLite | daemon_scheduler |
| `run graph` | daemon RPC to PG handler | no | daemon_native |
| `send` | daemon RPC to PG handler | no | daemon_native |
| `adapter run` | local single-shot process-adapter compatibility path | yes | transition debt |
| `byline` | local helper over current packet state | yes | operator-helper transition |
| `inbox --session-id` | local session-packet helper | yes | operator-helper transition |
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
4. Dogfood composite tools still open repo-local SQLite directly from
   `DaemonRpcRouter._route_dogfood`. They are compatibility helpers, not a
   production authority path.
5. `striatum.db` remains the legacy SQLite engine, but substrate-neutral
   helpers now live in `primitives.py` and `repo_policy.py`; guardrails keep
   daemon PG/RPC production modules from importing SQLite helpers.
6. Go has real handlers for the core reads, workflow loop, recovery, apply
   receipts, read-detail projections, and cross-repo reads, but a large
   repository registration, and cross-repo reads, but a large placeholder set
   remains around supervisor, workflow authoring, daemon admin, dogfood tools,
   and dashboard-all.
   Under D107 these placeholders are production-port blockers, not accepted
   D105-era gaps.
