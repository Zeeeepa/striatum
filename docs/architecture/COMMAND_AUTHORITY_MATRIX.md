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
plan. Until generated method contracts land, every new RPC method or handwritten
route map must update this file and the guardrails in
`tests/architecture/test_authority_guardrails.py`.

D105 names Python as the primary production daemon core. The Go columns below
remain transition/developer-harness evidence and should not be read as a
long-term second product surface.

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
| `archive.create` | `archive create` | read | single_repo | pg | placeholder | no | no | foundation |
| `run.summary` | `run summary` | read | single_repo | pg | real | no | no | stable |
| `run.graph` | `run graph` | read | single_repo | pg | placeholder | no | no | stable |
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
| `artifact.show` | web artifact raw DTO | read | single_repo | pg | placeholder | no | no | stable |
| `list.workflows` | `list workflows` | read | single_repo | pg | real | no | no | stable |
| `worktree.list` | `worktree list` | read | single_repo | pg | placeholder | no | no | stable |
| `dashboard.all` | `dashboard --all` | read | daemon_global | direct | placeholder | no | no | Python stable, Go gap |
| `repo.list` | `repo list` | read | daemon_global | bootstrap CLI helper | placeholder | no | no | bootstrap/admin |
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
| `supervise.reattach_status` | n/a | read | single_repo | not implemented in Python RPC | placeholder | no | no | not implemented |
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
| `escalation.resolve` | `escalation resolve` | admin | single_repo | pg | placeholder | no | no | stable |
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
| `recovery.auto` | `recovery auto`; `recovery auto-publish` currently maps here | recovery | single_repo | pg | real | no | no | overloaded, needs contract cleanup |
| `recovery.auto_finalize` | `recovery auto-finalize` | recovery | single_repo | pg | real | no | no | experimental/workflow-opt-in |
| `recovery.auto_publish_stale_artifacts` | deprecated alias | recovery | single_repo | no pg handler | absent from Go registry | no | no | deprecated alias |
| `recovery.watch` | `recovery watch` | recovery | single_repo | pg fail_closed | placeholder | no | no | not implemented in daemon RPC; use `recovery.auto` |
| `apply.reviewed_patch` | n/a | apply | single_repo | direct apply service | fail_closed | no | no | fail closed until apply authority |
| `apply.receipt.show` | n/a | read | single_repo | direct apply service | real | no | no | stable |
| `apply.receipt.verify` | n/a | read | single_repo | direct apply service | real | no | no | stable |
| `dogfood.surgical_recovery` | MCP/chat dogfood tool | surgical_recovery | single_repo | direct dogfood helper | placeholder | no | yes, direct `striatum.db.connect` | dogfood compatibility |
| `repo.add` | `repo add` | admin | daemon_global | bootstrap CLI helper | placeholder | no | no ordinary repo-local SQLite | bootstrap/admin |
| `repo.remove` | `repo remove` | admin | daemon_global | bootstrap CLI helper | placeholder | no | no | bootstrap/admin |
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
| `cross_repo.cancel` | `cross-repo cancel` | recovery | cross_repo | direct not_implemented | explicit not_implemented | no | no | not implemented |

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
| `repo add` / `repo list` / `repo remove` | daemon registry helpers outside RPC | optional `--init` may create scratch SQLite | bootstrap_admin |
| `cross-repo list` / `describe` / `why` | direct PG helper | no | daemon_read_out_of_band |
| `cross-repo cancel` | direct refusal | no | not_implemented |
| `workflow validate` / `lint` / `plan` / `graph` | local authoring helpers; daemon RPC fails closed | no live state | local_file_authoring |
| `workflow init` / `generate` / `templates` | local authoring helpers; daemon RPC fails closed | no live state | local_file_authoring |
| `workflow upgrade` | local authoring helper with PG running-run guard | legacy SQLite only before cutover; fails closed after cutover if PG unavailable | local_file_authoring |
| `run graph` | daemon RPC to PG handler | no | daemon_native |
| `send` | daemon RPC to PG handler | no | daemon_native |
| `adapter run` | local process adapter over SQLite state | yes | transition debt |
| `byline` | local helper over current packet state | yes | operator-helper transition |
| `inbox --session-id` | local session-packet helper | yes | operator-helper transition |
| `serve` | local HTTP/web service over `striatum.api.invoke` plus direct reads | yes | service transition debt |

## Immediate Findings

1. `CLI_ROUTES` is empty. Phase 1 removed daemon fallback routes for
   PG-backed reads, workflow-loop mutations, recovery handlers, run/admin
   lifecycle mutations, worktree/supervision handlers, and CLI-local workflow
   authoring helpers.
2. `recovery.auto_finalize` is now split out for RFC 0051 front-matter
   auto-finalize. `recovery.auto` still carries the older overload:
   `parser.py` exposes `recovery auto` as the RFC 0020 sweep, while
   `recovery auto-publish` maps to the V1.41 stale-artifact auto-publish
   behavior. A later contract cleanup should split that stale-artifact method
   instead of preserving the overload.
3. Dogfood composite tools still open repo-local SQLite directly from
   `DaemonRpcRouter._route_dogfood`. They are compatibility helpers, not a
   production authority path.
4. `striatum.db` remains the legacy SQLite engine, but substrate-neutral
   helpers now live in `primitives.py` and `repo_policy.py`; guardrails keep
   daemon PG/RPC production modules from importing SQLite helpers.
5. Go has real handlers for the core reads, workflow loop, recovery, apply
   receipts, and cross-repo reads, but a large placeholder set remains around
   supervisor, workflow authoring, repo/admin, dogfood tools, and dashboard-all.
