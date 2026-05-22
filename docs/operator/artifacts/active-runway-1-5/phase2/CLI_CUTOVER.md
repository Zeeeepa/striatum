---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/CLI_REFERENCE.md", "src/striatum/cli/parser.py", "contracts/daemon_methods.json", "docs/architecture/COMMAND_AUTHORITY_MATRIX.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md", "docs/rfcs/0077-mcp-activity-liveness-deadlines.md", "docs/operator/plans/rfc-0050-cli-retirement-cutover.md", "docs/operator/artifacts/rfc-0075-and-mcp-cutover/final/SUMMARY.md", "src/striatum/web/run_actions.py", "go/pkg/mcp/capabilities.go"]
---

# CLI Cutover Ledger
author: cutover-mapper-claude-code-001

## Scope

This ledger maps the remaining `striatum` CLI verbs against their current
daemon RPC authority and against MCP/UI parity. It is a precondition document
for RFC 0050 Phase F: until every workflow-control verb has a tested MCP or
operator-UI path, the CLI cannot be hidden or removed.

The mapping is taken from `src/striatum/cli/parser.py`, the
`contracts/daemon_methods.json` schema, the
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md` curated table, and the
landed Go MCP visibility rules in `go/pkg/mcp/capabilities.go`. Capability
vocabulary is the closed set `read`, `write`, `review`, `claim`, `apply`,
`admin`, `recovery`, `surgical_recovery`.

## Classification Vocabulary

Each surviving CLI verb is assigned exactly one classification:

| Class | Meaning | Retirement Stance |
|---|---|---|
| `mcp_parity` | Daemon method exists and is visible to MCP `tools/list`. Agents can drive the action without the CLI. | Can be hidden after agent-loop docs/skills stop teaching it. |
| `ui_parity` | Web UI route in `striatum serve --web` already mutates the same daemon method (with `--allow-mutations`). | Can be hidden after operator docs stop teaching it. |
| `mcp_and_ui_parity` | Both paths cover the verb. | Strongest retirement candidate. |
| `agent_lifecycle` | Verb is invoked by the work-packet loop and already runs over MCP for autonomous agents (`work.*`, `artifact.publish`, `review.*`, `worktree.*`, `supervise.*`). The CLI surface remains for legacy non-MCP supervisors and tests. | Hide from agent-facing docs; preserve for compatibility lanes. |
| `bootstrap_admin` | CLI must run before a daemon is reachable, configures the daemon itself, or is the supported way to install local artifacts. | Permanent CLI surface; do not retire. |
| `diagnostics` | Read-only inspection; daemon-routed but consumed primarily by humans in a terminal. | Permanent CLI surface; keep as `--json`-clean operator tools. |
| `local_file_authoring` | Edits files in the target repository without live workflow state. Daemon method is hidden in MCP `tools/list` on purpose. | Keep CLI; surface via UI authoring island where appropriate. |
| `legacy_fixture_only` | Already retired outside explicit legacy test-fixture compatibility. | No further retirement needed. |
| `retired_compatibility_refusal` | Argparse stub that refuses with a fixed exit code; remains for stale automation. | Keep until automation upgrades complete; never re-enable. |

`mcp_parity`, `ui_parity`, and `mcp_and_ui_parity` rows can move into a hide
phase. The other classes are out of scope for retirement.

## Ledger

### Bootstrap and adoption

| Verb | Class | Daemon method / authority | Capability | Justification |
|---|---|---|---|---|
| `init` | `bootstrap_admin` | local `init_repo` then `repo.init` when daemon present | `admin` (only when registering) | Creates `.striatum/` scratch and DDD/striatum layout; runs before any daemon registration. |
| `adopt` | `bootstrap_admin` | composes `init` + plugin/skills + `repo.add` | `admin` (registration step) | Day-zero guided flow; touches PostgreSQL through `client_admin` helpers as documented in the authority matrix. |
| `repo add` / `repo list` / `repo remove` | `bootstrap_admin` | `repo.add`, `repo.list`, `repo.remove` | `admin` (write) / `read` | Daemon-global registration plane; cannot be invoked from inside MCP because tools/list is repo-scoped. |
| `daemon start` / `striatumd` | `bootstrap_admin` | local Go daemon process launch | n/a (process control) | Required to bring the RPC/MCP surface online. |
| `daemon stop` | `bootstrap_admin` | `daemon.shutdown` (out-of-band) | `admin` | Lifecycle control; surfaced via systemd/launchd unit, not MCP. |
| `daemon status` / `health` / `audit` / `sweep` | `diagnostics` / `bootstrap_admin` | local PG helpers and `daemon.recovery_sweep` glue | `read` / `admin` | Operator lifecycle observation; `sweep` is the manual companion to the resident sweeper. |
| `daemon service install` / `start` / `status` | `bootstrap_admin` | local systemd/launchd helper | n/a | Local OS service plumbing; never workflow state. |
| `daemon doctor` (incl. `--first-run`, `--repo`, `--authority`, `--as-owner`, `--apply-migrations`, `--provision-rw-role`, `--repair-grants`) | `bootstrap_admin` | `daemon.migrate` and local PG diagnostic helpers | `admin` (when applying) | Bootstrap before daemon health; reports cutover authority and emits remediation SQL. |
| `daemon migrate` / `daemon migrate-repo-local` | `retired_compatibility_refusal` | refuses with exit code 12 | n/a | Already refuses; preserved so old automation gets a structured error. |
| `skills install` | `bootstrap_admin` | local filesystem | n/a | Writes agent skill bundle; no daemon state. |
| `plugin install` / `plugin uninstall` | `bootstrap_admin` | local filesystem | n/a | Writes agent-CLI plugin bundle; no daemon state. |
| `self-update` | `bootstrap_admin` | `pip` + plugin/skills install | n/a | Operator-only update path; documented in [[project_self_update_command]]. |
| `serve` | `bootstrap_admin` | local HTTP/UI service over daemon RPC | n/a | Launches the only operator UI surface; needed for the UI half of the cutover. |
| `operator current-brief` | `bootstrap_admin` | local Markdown reader (RFC 0058 V1.5) | n/a | Exempt from daemon-required guard by design; reads `docs/operator/BRIEF.md`. |

These rows are not retirement candidates. They are kept on the table to make
the operator boundary explicit before any hide phase ships.

### Diagnostics (read-only)

| Verb | Class | Daemon method | Capability | MCP visibility | UI visibility |
|---|---|---|---|---|---|
| `status` | `diagnostics` + `mcp_parity` | `status` | `read` | yes | yes (`/run/<id>`) |
| `why` | `diagnostics` + `mcp_parity` | `why` | `read` | yes | yes (per-job blocker reasoning) |
| `doctor` | `diagnostics` + `mcp_parity` | `doctor` | `read` | yes | yes (`/doctor`) |
| `dashboard` | `diagnostics` + `mcp_parity` | `dashboard` | `read` | yes | yes (run list) |
| `dashboard --all` | `diagnostics` + `mcp_parity` | `dashboard.all` | `read` | yes | gap: web UI shows per-repo dashboards but no cross-repo summary page |
| `run graph` | `diagnostics` + `mcp_parity` | `run.graph` | `read` | yes | yes (`/run/<id>` dependency graph) |
| `run summary` | `diagnostics` + `mcp_parity` | `run.summary` | `read` | yes | yes |
| `list runs/sessions/jobs/artifacts/workflows` | `diagnostics` + `mcp_parity` | `list.*` | `read` | yes | yes (run list, artifacts pane, workflow browser) |
| `evidence export` | `diagnostics` + `mcp_parity` | `evidence.export` | `read` | yes | no UI button; ledger gap (see Missing UI parity §). |
| `corpus export` / `corpus verify` | `diagnostics` + `mcp_parity` | `corpus.export` (export) / local (verify) | `read` | yes (export) | no UI; ledger gap. |
| `archive create` / `archive verify` | `diagnostics` + `mcp_parity` | `archive.create` (create) / local (verify) | `read` | yes (create) | no UI; ledger gap. |
| `cross-repo list` / `describe` / `why` | `diagnostics` + `mcp_parity` | `cross_repo.list/describe/why` | `read` | yes | gap: no `/cross-repo/` UI route. |
| `supervise status` / `list` | `diagnostics` + `mcp_parity` | `supervise.status` / `supervise.list` | `read` | yes | gap: web supervisor reattach DTO exists; no full status pane. |
| `worktree list` | `diagnostics` + `mcp_parity` | `worktree.list` | `read` | yes | not exposed; low-priority gap. |
| `inbox` (no `--session-id`) | `diagnostics` + `mcp_parity` | `escalation.list` | `read` | yes | gap: `/escalations/` page not implemented. |
| `escalation list` / `show` | `diagnostics` + `mcp_parity` | `escalation.list` / `escalation.show` | `read` | yes | gap: same escalations page. |

Diagnostics verbs are intentionally CLI-permanent. Their MCP parity rows
exist because the daemon methods are already advertised to capability-scoped
MCP tokens; nothing in this ledger asks the daemon to remove that visibility.
The UI gaps in this section block UI-only operator flows but are smaller than
the live workflow-control gaps below.

### Workflow control (live mutation candidates for retirement)

This is the section that gates Phase F. Each verb here must reach
`mcp_and_ui_parity` (or `agent_lifecycle` for verbs only an agent should
invoke) before it is hidden from operator docs.

| Verb | Class | Daemon method | Capability | MCP visibility | UI visibility | Retirement gate |
|---|---|---|---|---|---|---|
| `run prepare` | `mcp_and_ui_parity` | `run.prepare` | `admin` | yes | yes (`POST /workflows/run/<path>` chains prepare + branch + start) | needs explicit UI "prepare-only" affordance; today it auto-starts. |
| `run start` | `mcp_and_ui_parity` | `run.start` | `admin` | yes | yes (`POST /run/<id>/branch-confirm` chains start) | UI requires `--allow-mutations`; operator-doc switch from CLI to UI is the remaining work. |
| `run pause` / `resume` / `cancel` / `retry-job` | `mcp_and_ui_parity` | `run.pause/resume/cancel/retry_job` | `admin` | yes | yes (`POST /run/<id>/{pause,resume,cancel}`, `POST /run/<id>/job/<id>/{cancel,retry}`) | requires UI parity tests and operator-docs flip. |
| `run summary` (mutation form does not exist; read-only is in diagnostics) | n/a | n/a | n/a | n/a | n/a | n/a |
| `run graph` | covered under diagnostics | n/a | n/a | yes | yes | n/a |
| `branch confirm` | `mcp_and_ui_parity` | `branch.confirm` | `admin` | yes | yes (chained inside run-actions) | UI does not expose `--strict` or `--use-current`; needs flag parity before docs flip. |
| `register-session` | `agent_lifecycle` | `session.register` | `claim` | yes | not surfaced | Lane responsibility; agents register via MCP `session.register` after PTY bootstrap. CLI form stays for non-MCP harnesses and tests. |
| `session close` | `agent_lifecycle` | `session.close` | `claim` | yes | not surfaced | Same as `register-session`. |
| `claim-next` | `agent_lifecycle` | `work.claim_next` | `claim` | yes | n/a (agent-only) | Use `work.await_packet` for MCP agents (RFC 0050 Phase D landed). CLI form remains for non-MCP fixture lanes. |
| `ack` / `heartbeat` / `release` / `send` / `block` / `complete` | `agent_lifecycle` | `work.ack` / `work.heartbeat` / `work.release` / `work.send_message` / `work.block` / `work.complete` | `claim` (`write` for `work.send_message`/`work.block`/`work.complete`) | yes | n/a (agent-only) | MCP cutover landed; non-MCP lanes still rely on the CLI shape. |
| `publish-artifact` | `agent_lifecycle` | `artifact.publish` | `write` | yes | n/a (agent-only) | MCP path proven by Phase D harness. CLI compat retained. |
| `verdict` / `submit-review` | `agent_lifecycle` | `review.verdict` / `review.submit` | `review` | yes | not surfaced | UI gap: no reviewer page lets an operator submit a verdict; ledger gap below. |
| `override-verdict` | `agent_lifecycle` (operator subcommand) | `review.override` | `admin` | yes | not surfaced | UI gap: no override affordance from artifact page; ledger gap below. |
| `decision record` | `agent_lifecycle` (operator subcommand) | `decision.record` | `admin` | yes | not surfaced | UI gap: no decision-record form. |
| `checkpoint resolve` | `agent_lifecycle` (operator subcommand) | `checkpoint.resolve` | `admin` | yes | not surfaced | UI gap: blocker resolution UI missing; ledger gap below. |
| `escalation resolve` | `agent_lifecycle` (operator subcommand) | `escalation.resolve` | `admin` | yes | not surfaced | UI gap: pairs with the escalation page gap. |
| `worktree create` / `release` | `agent_lifecycle` | `worktree.create` / `worktree.release` | `write` | yes | not surfaced | Lane plumbing; surfaced inside the agent loop. CLI remains for tests. |
| `supervise start` / `send` / `stop` | `agent_lifecycle` | `supervise.start/send/stop` | `claim` | yes | not surfaced (only reattach status DTO) | Lane plumbing; CLI remains for non-MCP supervisors. |
| `recovery stale-leases` / `requeue-stale` / `cancel-job` / `process-reconcile` / `resume` / `auto` / `auto-publish` / `auto-finalize` | `agent_lifecycle` (operator + scheduler) | `recovery.*` | `recovery` | yes | not surfaced as discrete UI buttons | UI gap: no recovery surface. The resident sweeper is the autonomous path; operators still need CLI for ad-hoc remediation. |
| `recovery watch` | `bootstrap_admin` (foreground scheduler) | repeated `recovery.sweep` | `recovery` | yes (`recovery.sweep`) | n/a | The CLI scheduler is itself a substitute for `systemd timer`; not a UI candidate. |

### Local file authoring (intentionally hidden from MCP)

`go/pkg/mcp/capabilities.go::isHiddenProductionTool` already hides the
following methods from MCP `tools/list` so they cannot be invoked by an
agent over MCP. The CLI is the maintained authoring surface; the web UI
mounts UI islands over the same routes for chosen flows.

| Verb | Class | Daemon method (hidden from MCP) | UI parity |
|---|---|---|---|
| `workflow validate` | `local_file_authoring` | `workflow.validate` | yes (workflow detail) |
| `workflow lint` | `local_file_authoring` | not in `cli_routes` map; CLI runs the lint engine locally | partial (workflow detail surfaces lint warnings) |
| `workflow plan` / `graph` | `local_file_authoring` | `workflow.plan` / `workflow.graph` | yes |
| `workflow init` | `local_file_authoring` | `workflow.init` | gap: chooser-wizard island exists for `generate`; no `init` flow. |
| `workflow templates list` / `show` / `render-md` | `local_file_authoring` | `workflow.templates.*` | partial (templates inform the chooser-wizard but render-md has no UI). |
| `workflow generate` | `local_file_authoring` | `workflow.generate` and `workflow.generate.preview` | yes (`/workflows/new` chooser-wizard island). |
| `workflow upgrade` | `local_file_authoring` | `workflow.upgrade` | gap: no UI button; preserve CLI. |

These verbs are not retirement candidates. The cutover work for them is to
keep MCP hiding and UI islands consistent.

### Legacy fixture-only

| Verb | Class | Notes |
|---|---|---|
| `adapter run` | `legacy_fixture_only` | Retired outside the explicit legacy test-fixture compatibility environment. Documented in CLI_REFERENCE. |
| `byline` | `legacy_fixture_only` | V1.41 historical harness helper; production clients use daemon reads. |
| `inbox --session-id` | `legacy_fixture_only` | Retained for legacy test fixtures only. The default `inbox` (without `--session-id`) is `diagnostics` for the escalation inbox. |

## Missing MCP/UI Parity Before CLI Hide

Below is the parity backlog. Each item is a precondition for hiding (not
deleting) the corresponding CLI verbs from operator docs and skills. None of
these are scoped beyond what is already implied by RFC 0050 Phase F.

### MCP gaps

MCP `tools/list` already exposes every workflow-control daemon method the
runner cares about. The remaining MCP work is observability and policy, not
new tools:

1. RFC 0077 V1 has landed protocol-liveness fields on `status`, `dashboard`,
   and `supervise.status`. Skills/docs still need to teach agents to read
   those fields so retirement of CLI `status` polling is safe.
2. RFC 0075 follow-up (tmux observability) does not block CLI retirement,
   but operator UI parity for `supervise.status` reattach metadata depends
   on it. Keep the CLI `supervise status` until at least the protocol
   liveness panel ships in `/run/<id>`.
3. Workflow authoring methods stay hidden from MCP by design
   (`isHiddenProductionTool` in `go/pkg/mcp/capabilities.go`). Documentation
   should restate that this is intentional, not a gap.

### UI gaps (web `striatum serve --web`)

These UI affordances are missing today and must land (with tests and an
`--allow-mutations` gate) before the matching CLI verbs can be hidden from
the operator playbook:

1. **Reviewer console.** A page that lets an operator publish a verdict on a
   review job (`review.verdict`, `review.submit`, and `review.override`).
   Today the artifact page renders findings but has no verdict form. This
   blocks hiding `verdict`, `submit-review`, and `override-verdict`.
2. **Checkpoint and escalation surface.** A page over `escalation.list`,
   `escalation.show`, and `escalation.resolve` (plus `checkpoint.resolve`
   for non-escalation blockers). Blocks hiding `escalation resolve`,
   `checkpoint resolve`, and the human-principal `inbox`.
3. **Recovery panel.** Discrete buttons for `recovery.stale_leases`,
   `recovery.requeue_stale`, `recovery.cancel_job`,
   `recovery.process_reconcile`, `recovery.resume`,
   `recovery.auto_publish_stale_artifacts`, and `recovery.auto_finalize`.
   The resident sweeper covers the autonomous path; operators still need a
   UI for the manual remediation path before `recovery *` CLI hides.
4. **Decision recorder.** A `decision.record` form attached to the artifact
   viewer so operators can record decision rows from a published artifact
   without dropping to the CLI.
5. **Evidence/Archive/Corpus exporters.** Operator-side buttons in the run
   detail page that fire `evidence.export`, `archive.create`, and
   `corpus.export` with explicit destination paths. Required before hiding
   the `evidence export`, `archive create`, and `corpus export` CLI verbs
   from operator-facing docs.
6. **Cross-repo console.** A `/cross-repo/` index plus per-run detail page
   covering `cross_repo.list`, `cross_repo.describe`, `cross_repo.why`, and
   `cross_repo.cancel`. Required before the `cross-repo *` CLI verbs are
   demoted from operator docs.
7. **Worktree list/release panel.** A small affordance on `/run/<id>/job/<id>`
   for `worktree.list` and `worktree.release`. Lower priority because
   worktrees are agent-owned.
8. **Branch-confirm flag parity.** The current web flow always passes
   `create=true` (or `use_current=true` when supplied) and never `--strict`.
   CLI `branch confirm --strict` needs UI parity before the CLI is hidden;
   without it CI operators lose a safety net.
9. **Prepare-only flow.** `POST /workflows/run/<path>` chains prepare and
   start. Hiding CLI `run prepare` requires a UI affordance to inspect a
   prepared run without immediately starting it (the existing CLI two-step
   is the only "stop after prepare" path today).
10. **Operator-current-brief panel.** Surface `operator current-brief`
    metadata in the `/` (run-list) header. Required before the CLI form is
    declared "diagnostics only."

The other diagnostics verbs (`status`, `why`, `doctor`, `dashboard`,
`run graph`, `list *`, `supervise status`, `supervise list`) already have
adequate UI parity; their CLI forms remain for terminal-first operators and
do not need to be hidden.

## Tests And Guardrails Required Before Demotion

CLI retirement cannot precede automated parity coverage. The required
guardrails are:

1. **Authority matrix coverage.** `tests/architecture/test_authority_guardrails.py`
   already cross-checks `parser.py`, `contracts/daemon_methods.json`, and
   `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`. Before any verb is hidden
   from operator docs, its row must remain in the matrix marked as
   `cli_hidden` (or equivalent) so the guard refuses silent removal.
2. **MCP visibility tests.** Extend `go/pkg/mcp/capabilities_test.go` and the
   contract-derived MCP `tools/list` tests to assert every retirement-track
   method stays advertised to the right capabilities and stays hidden under
   the wrong capability, so retirement does not depend on MCP surface drift.
3. **Fake-agent end-to-end coverage.** The RFC 0050 Phase D harness covers
   the work-packet loop. Extend it to cover the operator subcommands once
   the UI surfaces ship: review submission, override, decision record,
   checkpoint resolve, escalation resolve, recovery sweep, and run lifecycle
   (`pause`/`resume`/`cancel`/`retry-job`).
4. **Web UI parity tests.** For each UI gap listed in §"UI gaps" above, add a
   `tests/web/*` HTTP test that hits the route with `--allow-mutations`
   enabled and asserts the daemon method is called with the expected
   parameters (and that the route refuses without `--allow-mutations`).
5. **Skill/doc lint.** Add a `make lint` check that grep-fails operator
   skills, `docs/HOW_TO_HUMAN.md`, and `docs/SPEC.md` for any reference to a
   verb whose ledger row is in the hide phase. Each retired CLI mention must
   be replaced with the MCP method name or UI route, not deleted silently.
6. **Liveness contract guard.** Until RFC 0077 V1 liveness fields are
   referenced from the operator-facing UI, keep `status` and `supervise
   status` CLI verbs surfaced; gate their demotion behind a test that asserts
   `/run/<id>` renders `liveness.stall_class`.
7. **Bootstrap exclusion test.** Add a test that the bootstrap/admin rows in
   the ledger above never enter a hide phase. The guard runs against the
   ledger directly, not against operator docs.

## Bootstrap And Diagnostics To Preserve

The following CLI surface remains permanent because no MCP/UI surface can
replace it without violating the daemon-required boundary (D094) or the
pre-daemon boostrap step:

- `init`, `adopt`, `repo add|list|remove`, `daemon start`/`stop`/`status`/
  `health`/`audit`/`sweep`, `daemon service install|start|status`,
  `daemon doctor` (incl. `--first-run`, `--repo`, `--authority`, `--apply-migrations`,
  `--as-owner`, `--provision-rw-role`, `--repair-grants`), `daemon migrate*`
  refusals, `skills install`, `plugin install|uninstall`, `self-update`,
  `serve`, `operator current-brief`.
- Diagnostics CLI verbs are preserved for the terminal-first operator path:
  `status`, `why`, `doctor`, `dashboard` (incl. `--all`), `run graph`,
  `run summary`, `list *`, `evidence export`, `archive verify`,
  `corpus verify`, `supervise status`, `supervise list`, `worktree list`,
  `inbox` (no `--session-id`), `escalation list|show`, `cross-repo
  list|describe|why`.
- Legacy-fixture verbs (`adapter run`, `byline`, `inbox --session-id`) stay as
  retired-but-parseable stubs.
- Local file authoring (`workflow validate|lint|plan|graph|init|generate|
  upgrade|templates *`) stays as CLI; the matching daemon methods stay
  hidden from MCP per the existing `isHiddenProductionTool` policy.

## Explicit Boundary: CLI Retirement Cannot Precede Parity

This ledger is a *prerequisite map*, not a hide order. The boundary is the
same one stated in RFC 0050 Phase F, `docs/operator/BRIEF.md` Hazards, and
the active runway plan:

- No workflow-control CLI verb may be hidden from operator docs, skills,
  examples, or the agent skill bundle before:
  1. its daemon RPC method is visible to the appropriate MCP token; and
  2. either an operator UI surface exists for it (for operator subcommands)
     or the MCP agent loop covers it (for agent subcommands); and
  3. the matching test in §"Tests And Guardrails Required Before Demotion"
     is enforced by CI.
- No workflow-control CLI verb may be *deleted* until at least one release
  has shipped with the verb hidden and no production runner has emitted it
  for a defined deprecation window. The current scope keeps every verb
  parseable.
- The bootstrap and diagnostics rows above are out of scope for retirement
  entirely; they remain permanent CLI surface.

The Phase 2 runway only commissions this ledger. Phase 3 onward may begin
landing UI affordances and corresponding tests; Phase F may begin hiding
verbs only after each row's gate is satisfied. CLI retirement does not lead
parity work — parity work clears CLI retirement.
