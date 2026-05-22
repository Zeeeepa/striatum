---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/active-runway-1-5/phase2/CLI_CUTOVER.md", "docs/CLI_REFERENCE.md", "src/striatum/cli/parser.py", "contracts/daemon_methods.json", "docs/architecture/COMMAND_AUTHORITY_MATRIX.md", "go/pkg/mcp/capabilities.go", "src/striatum/web/run_actions.py", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/operator/plans/rfc-0050-cli-retirement-cutover.md"]
---

# CLI Parity Map
author: operator [self-declared: codex-operator]

## Cutover Rule

No CLI workflow-control deletion is authorized before MCP and operator-UI
parity tests pass for the verb being hidden or removed. Hiding from docs/help
may happen per slice only after the replacement path is covered by tests and
the survivor category is recorded.

## Category Map

| Category | CLI surfaces | Daemon/MCP mapping | Operator UI today | Retirement stance |
|---|---|---|---|---|
| Bootstrap | `init`, `adopt`, `repo add/list/remove`, `daemon start/stop/status/health/audit/sweep/doctor/service`, `skills install`, `plugin install/uninstall`, `self-update`, `serve`, `operator current-brief`, `recovery watch` | Mixed local bootstrap plus `repo.*`, `daemon.shutdown`, `daemon.migrate`, repeated `recovery.sweep`; mostly not agent workflow tools | `serve --web` launches the UI; doctor page covers diagnostics | Survive as CLI. These bring up or repair the local system and are not workflow-control deletion candidates. |
| Diagnostics | `status`, `why`, `doctor`, `dashboard`, `list *`, `git snapshot`, `run summary`, `run graph`, `evidence export`, `corpus export/verify`, `archive create/verify`, `supervise status/list`, `worktree list`, `inbox`, `escalation list/show`, `cross-repo list/describe/why` | Registered read methods where applicable: `status`, `why`, `doctor`, `dashboard`, `list.*`, `git.snapshot`, `run.*`, `evidence.export`, `corpus.export`, `archive.create`, `supervise.*`, `worktree.list`, `escalation.*`, `cross_repo.*` | Run list/detail/job/artifact/doctor/workflow pages cover common reads; exporter, cross-repo, escalation inbox, and full supervisor views remain partial | Survive as terminal-first diagnostics unless a separate decision removes them. MCP parity does not imply deletion. |
| Local authoring | `workflow validate/lint/plan/graph/templates/init/generate/upgrade` | `workflow.validate`, `workflow.plan`, `workflow.graph`, `workflow.templates.*`, `workflow.init`, `workflow.generate`, and `workflow.upgrade` are intentionally hidden from production MCP; `workflow.lint`, `workflow.generate.preview`, `workflow.accept_risk`, and accepted-risk reads remain daemon/MCP-visible where daemon state is authoritative | Workflow browser/editor and chooser cover validate/generate/edit flows; upgrade and render-md remain CLI-first | Survive as authoring tools, not live workflow-control. Keep MCP hiding intentional and tested. |
| Parity-backed workflow control | `run pause/resume/cancel/retry-job`, job cancel through `recovery cancel-job`, `override-verdict`; partial `run prepare/start` and `branch confirm` | `run.pause`, `run.resume`, `run.cancel`, `run.retry_job`, `recovery.cancel_job`, `review.override`, `run.prepare`, `run.start`, `branch.confirm` are registered MCP tools for authorized tokens | Run detail has pause/resume/cancel; job detail has cancel/retry and override-verdict modal; workflow run-now chains prepare/branch/start; branch-confirm panel starts the run | Hide candidates only after parity tests and docs/skills flip. `run prepare` lacks prepare-only UI; `branch confirm` lacks `--strict`; start is often chained rather than standalone. |
| Workflow-control temporary compatibility | `register-session`, `session close`, `claim-next`, `ack`, `heartbeat`, `release`, `send`, `block`, `publish-artifact`, `complete`, `verdict`, `submit-review`, `supervise start/send/stop`, `worktree create/release`, `decision record`, `checkpoint resolve`, `escalation resolve`, `recovery stale-leases/requeue-stale/process-reconcile/resume/auto/auto-publish/auto-finalize`, `cross-repo cancel` | Registered methods include `session.*`, `work.await_packet`, `work.*`, `artifact.publish`, `review.*`, `supervise.*`, `worktree.*`, `decision.record`, `checkpoint.resolve`, `escalation.resolve`, `recovery.*`, `cross_repo.cancel`; the fake MCP agent covers the minimal packet loop | Agent lifecycle is intentionally not a human UI flow. UI gaps remain for first-class review submit/verdict, decisions, checkpoint/escalation resolve, most recovery actions, worktree release, supervisor control, and cross-repo cancel | Keep CLI compatibility until MCP coverage and UI replacement tests exist for each non-agent-only action. |
| Retired compatibility | `daemon migrate --from sqlite --to pg`, `daemon migrate-repo-local`, `adapter run`, `byline`, `inbox --session-id`, deprecated RPC aliases such as bare `ack`/`heartbeat` method names | Retired SQLite commands refuse before opening SQLite; legacy fixture helpers are not production workflow control; deprecated aliases are hidden/deprecated contract compatibility | None required | Do not re-enable. Preserve structured refusals only while stale automation needs them. |

## Workflow-Control Map

| CLI verb group | Daemon MCP method(s) | UI surface | Current gap |
|---|---|---|---|
| Run creation and start: `run prepare`, `run start`, `branch confirm` | `run.prepare`, `run.start`, `branch.confirm` | `/workflows/run/<path>` run-now and `/run/<id>/branch-confirm` | Need prepare-only UI and `branch confirm --strict` parity before hiding CLI two-step. |
| Run control: `run pause`, `run resume`, `run cancel`, `run retry-job` | `run.pause`, `run.resume`, `run.cancel`, `run.retry_job` | `/run/<id>` and `/run/<id>/job/<job_id>/retry` | Mostly parity-backed; keep until web tests are the retirement gate and docs no longer teach CLI as default. |
| Agent packet loop | `session.register`, `session.close`, `work.await_packet`, `work.ack`, `work.heartbeat`, `work.release`, `work.send_message`, `work.block`, `work.complete`, `artifact.publish` | No human UI expected | MCP minimal completion loop is covered; add review/block/release/send coverage before removing CLI compatibility for non-MCP lanes. |
| Review control | `review.verdict`, `review.submit`, `review.override` | Job detail shows verdicts and has override-verdict modal | First-class verdict and submit-review UI is missing; override modal still posts CLI-shaped argv through `/v1/invoke`. |
| Human-principal decisions | `decision.record`, `checkpoint.resolve`, `escalation.resolve` plus `escalation.list/show` reads | Doctor/recovery panels show recipes; no dedicated decision or escalation inbox form | Blocks hiding `decision record`, `checkpoint resolve`, `escalation resolve`, and human-principal `inbox`. |
| Recovery | `recovery.stale_leases`, `recovery.requeue_stale`, `recovery.cancel_job`, `recovery.process_reconcile`, `recovery.resume`, `recovery.sweep`, `recovery.auto_publish_stale_artifacts`, `recovery.auto_finalize` | Job cancel uses `recovery.cancel_job`; recovery island previews auto-publish dry-run and copies recipes | Need full recovery panel with mutation buttons, dry-run/live policy display, and `--allow-mutations` refusals. |
| Supervision and worktrees | `supervise.start/send/stop/status/list`, `worktree.create/release/list` | Session chips and job evidence are visible; status/list are read paths | Control actions remain agent/lane plumbing; do not hide CLI until non-MCP supervisor compatibility is explicitly retired. |
| Cross-repo control | `cross_repo.cancel` | No cross-repo console | Need cross-repo index/detail/cancel UI before demoting CLI from operator docs. |

## Known Parity Gaps

MCP gaps are narrow: registered workflow-control methods are visible through
`tools/list` for authorized capabilities, while local authoring methods are
hidden by `go/pkg/mcp/capabilities.go` on purpose. The remaining MCP work is
test coverage and teaching: extend fake-agent coverage beyond the minimal
author loop to review verdicts, block/release/send, recovery refusal paths,
and liveness fields.

UI gaps are the gating work: prepare-only run setup, `branch confirm --strict`,
review submit/verdict, decision record, checkpoint resolve, escalation
list/show/resolve, full recovery controls, exporter buttons, cross-repo
console, worktree release, and supervisor control/liveness views.

Docs and skills still present CLI loops as normal operator behavior in places.
Those references cannot be flipped until the relevant MCP/UI path is tested.

## Required Tests Before Hide/Delete

1. Contract guard: keep `parser.py`, `contracts/daemon_methods.json`,
   `COMMAND_AUTHORITY_MATRIX.md`, MCP visibility, and this classification in
   sync; fail if a workflow-control verb disappears without a survivor class.
2. MCP guard: assert every retirement-track method appears for the right
   capability, is hidden for wrong capabilities, and local authoring methods
   stay hidden even for write/admin tokens.
3. MCP end-to-end: extend the fake-agent workflow to exercise review verdict,
   submit-review, block, release, send, stale-lease refusal, and liveness
   reporting through `/mcp`.
4. Web parity: for each UI replacement, test success, parameter mapping to the
   daemon method, `--allow-mutations` refusal, same-origin/context protection,
   and no SQLite fallback.
5. Documentation/skill lint: once a CLI verb enters hide phase, fail docs and
   installed skill fixtures that still teach that verb as the normal control
   path.
6. Bootstrap exclusion: assert bootstrap, diagnostics, local authoring, and
   retired compatibility rows cannot be marked as workflow-control deletion
   candidates.

## Next Slice

Make the next slice a UI-and-test gate, not deletion. Start with the
human-principal cluster: escalation inbox/detail/resolve, checkpoint resolve,
and decision record. That cluster removes the largest operator-only CLI gap,
has clear daemon methods, and can share one `--allow-mutations` and
same-origin test pattern. After it lands, update docs/skills to prefer the UI
for those actions while keeping the CLI as temporary compatibility.
