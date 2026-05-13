# Changelog

## v1.34.0 — 2026-05-13

### Added

- RFC 0038 V1.5 — web UI integration gaps landed under dogfood-045.
  (F1) `placeholderIslandPlugin` removed from
  `src/striatum/web/frontend/vite.config.ts`; `plugins` is now
  `[react()]`. `manifest` flipped to `false` so the build no longer
  emits `.vite/manifest.json`; the existing `manifest.sha256` remains
  the single committed manifest. A new `make ui-verify-bundle` target
  rejects (a) any stable island entry whose body contains the V1
  sentinel `Striatum frontend island placeholder loaded`, (b) any
  `island-shared-*.js` chunk containing the same sentinel, and
  (c) any stable island entry under 1024 bytes (unless a sibling
  `island-shared-*.js` chunk ≥ 1024 bytes covers the legitimate
  factored-chunk case). `make ui-check-bundle` now depends on both
  `ui-build` and `ui-verify-bundle`. Python sentinel guard
  `tests/test_web_ui.py::test_island_bundles_have_no_placeholder_sentinel`
  reads each stable island bundle through `importlib.resources` and
  asserts the sentinel is absent so the guard survives `pip install`.
  (F2) `/workflows/new` chooser prop-contract fix: the
  `/workflow-templates` route is unchanged (it already returns
  `{"ok": true, "data": {"templates": list_templates(kind=kind)}}`);
  `src/striatum/web/frontend/src/shared/types.ts` adds
  `WorkflowTemplate` + `WorkflowTemplateListResponse` mirroring the
  server fields and removes the dead `WorkflowShape` /
  `WorkflowLaneSet` / `WorkflowTemplateCatalog` types.
  `WorkflowChooser.tsx` reads `res.data.templates`, partitions by
  `kind`, derives `shape` from the picked `kind: "shape"` row's
  `template_id`, pre-fills `lane_set` from the first overlapping
  `default_lane_sets` entry, and drops the V1 modifier UI (the server
  never returned `catalog.modifiers`). Wizard is now four steps:
  Template → Details → Preview → Save. `__testing` exports `buildSpec`
  + `recommendedForText`; the V1 `isModifierEnabled` export is gone.
  (F3) Island-shared double-mount fix: new
  `src/striatum/web/frontend/src/shared/island-shared-entry.ts`
  (`import "./theme.css"; export {};`) is the new Rollup input for the
  `island-shared` bundle. `src/main.ts` still exists for the Vite dev
  server (`make ui-dev`) but is no longer a production Rollup input,
  so it cannot mount islands twice. Vitest regression
  `src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts`
  mocks `react-dom/client.createRoot`, imports the shared entry plus
  the chooser entry into a JSDOM page with only
  `#island-workflow-chooser`, and asserts `createRoot` is called
  exactly once. (F4) Vite output semantics aligned with the
  package-data layout: output stays at `src/striatum/web/static/build/`,
  public URLs unchanged, and `pyproject.toml`
  `[tool.setuptools.package-data]` already matches the `manifest: false`
  layout (`"striatum.web.static" = [..., "build/*.js", "build/*.css",
  "build/*.sha256"]` + explicit `"striatum.web.static.build" = ["*.js",
  "*.css", "*.sha256"]` sub-package entry). New
  `tests/test_web_workflows.py::test_workflows_edit_renders_graph_editor_island`
  pins `/workflows/edit/<path>`. Supply-chain hygiene: `ui-install`
  now uses `npm ci` (lockfile-reproducible installs); new
  `ui-update-lock` for intentional dependency bumps; new `ui-audit`
  runs `npm audit --audit-level=high`;
  `src/striatum/web/frontend/npm-audit-baseline.json` ships as the
  accepted-findings tracker.
  Files: `Makefile`, `src/striatum/web/frontend/vite.config.ts`,
  `src/striatum/web/frontend/src/shared/api-client.ts`,
  `src/striatum/web/frontend/src/shared/types.ts`,
  `src/striatum/web/frontend/src/shared/island-shared-entry.ts` (new),
  `src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`,
  `src/striatum/web/frontend/src/__tests__/workflow-chooser.test.ts`,
  `src/striatum/web/frontend/src/__tests__/workflow-chooser-fetch.test.tsx` (new),
  `src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts` (new),
  `src/striatum/web/frontend/npm-audit-baseline.json` (new),
  `tests/test_web_ui.py`, `tests/test_web_workflows.py`. Implementer
  was **claude** (TypeScript/Vite work), the first dogfood deliberately
  not using codex as implementer to avoid the codex/codex anti-pattern
  (precedents D095-D098). Real-bundle commit and `make` verification
  remain operator-side follow-up — the new sentinel/size guard + Python
  resource test refuse another placeholder commit from reaching CI.

### Decided

- D099: Reject override for the dogfood-045 build review
  (`dec_ccfa1685878d41d69ccc6496cd6612fd`, `accepted_with_follow_up`).
  Codex `review_build_codex` returned `reject severity=critical` under
  the threat_model posture; cross-lane consensus disagreed (claude
  `accept_with_findings` medium, gemini `accept` low). Codex critical
  rests on (a) committed bundles still being V1 placeholders pending
  operator-side rebuild, (b) build verification gates not executed in
  the implementer run, and (c) source-side mitigations being unproven
  against real output. The HANDOFF explicitly documents the
  real-bundle commit as an operator follow-up and the new sentinel
  guard refuses to ship another placeholder commit, so the
  cross-lane 2-of-3 majority overrides. Codex findings absorbed into
  RFC 0038 V1.6 follow-up (TODO item 29). First dogfood with a
  codex-reviewer-of-claude-implementer pattern (not codex/codex);
  the harsh codex verdict suggests codex-as-reviewer baseline
  conservatism is independent of the codex/codex convergent
  blind-spot anti-pattern. Recovery path on this run was non-trivial:
  the codex reject pushed the run state to `failed`, requiring SQL
  surgery + `striatum verdict --override` to recover.

### Notes

- Dogfood-045 ran the 9-job single-track workflow for RFC 0038 V1.5
  (F1-F4 + supply-chain hygiene findings from dogfood-041 deferred
  under cycle-exhaustion override
  `dec_251e8a5f3d674c409de0dad9eacd5844`). Like dogfood-044, the
  `consolidate` job was not in the workflow; the operator authored
  this changelog entry, `docs/rfcs/README.md` status update,
  `docs/TODO.md` follow-ups, `docs/dogfood/045/BUILD_HANDOFF.md`, and
  `docs/dogfood/045/PHASE_1_OPERATOR_NOTES.md` out-of-band.
- The codex review verdict surfaced an operator-facing harness gap:
  a reviewer `reject` verdict transitions the run state to `failed`
  before the operator can decide whether to override. Recovery here
  required SQL surgery on the verdict + run state followed by
  `striatum verdict --override` (the override-accepting verdict path
  landed in v1.32.x). A future RFC could plumb an explicit
  "operator-pending" run state distinct from `failed` so verdicts
  awaiting override do not require manual SQL recovery.

## v1.33.0 — 2026-05-13

### Added

- RFC 0040 V1.5 — daemon-side dispatch + composite tools + watcher
  invocation landed under dogfood-044. (F1) Daemon MCP `tools/call`
  now dispatches through the RFC 0030 method registry via new
  `src/striatum/daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call`
  which owns lookup, capability authorization, envelope build, and
  routing through `DaemonRpcRouter.handle(...)`; the previous stub
  that returned a fake `ok: true` is gone. `DaemonRpcRouter.handle`
  accepts `transport` (default `"rpc"`; MCP passes `"mcp"`) and
  `require_handshake` (default `True`; MCP passes `False`). Audit
  rows are post-dispatch: unknown methods + authorization denials
  emit one `transport="mcp"` deny row; allowed calls emit exactly
  one row carrying the real handler exit code. MCP response shape
  `{content, structuredContent, isError}` preserved; structuredContent
  carries `ok`, `method`, `audit_id`, and `data` on success.
  (F2/F3) `dogfood.publish_on_behalf` runs ack/publish/verdict inside
  one outer `with transaction(conn):` block via new transaction-free
  helpers `_ack_on_behalf_locked`, `_publish_artifact_locked`,
  `_record_verdict_locked`, and `_complete_locked`; review jobs
  require `verdict` up front (validated against the same enum the
  direct-Python helper accepts); `findings_artifact_id` defaults to
  the published artifact id when kind=`finding`; on success exactly
  one `dogfood.publish_on_behalf` event is inserted in-transaction
  with `composition_steps` covering ack/publish/verdict-or-complete;
  on failure the transaction rolls back and a best-effort
  `dogfood.publish_on_behalf_failed` event is written tagged
  `outcome: "rolled_back"`. (F4) New
  `src/striatum/process_progress.py::progress_loop_once` runs one
  bounded supervised-progress pass per repository joined to the
  `runs` table so only attached supervisors under running/paused
  runs tick; called from `daemon.daemon_sweep_once` inside
  `connect_repo(repo)` immediately before per-run auto-sweep work
  and folded into the sweep return payload as `"progress"`. The
  loop materializes each row as a `SupervisedProgressTarget` and
  ticks `SupervisedProgressWatcher`, whose heartbeat callback calls
  `striatum.cli.mutations.heartbeat` on the same repo connection.
  Metadata-only events:
  `supervisor.progress_watcher_heartbeat`,
  `supervisor.progress_watcher_idle`,
  `supervisor.progress_watcher_lost`. Log contents are never read.
  (F5) `ProcessProgressConfig.startup_grace_seconds` defaults to
  60 s; within grace a missing scratch path returns `waiting_for_log`
  with no warning; watcher catches `FileNotFoundError`/`OSError`
  while scanning `*.log` files so rotated logs follow without
  recreating the target; loop accepts a `should_stop` predicate and
  checks it between supervisors so SIGTERM cannot start a new
  heartbeat after shutdown; `progress_advisory_lock(repo,
  job_id=...)` is shared with `surgical_recovery` (watcher tick
  returns `lock_busy`, surgical recovery returns
  `progress_lock_busy`); PID-reuse guard via `process_start_time(pid)`
  flips the row to `state='lost'` on mismatch versus stored
  `pid_start_time` and emits `supervisor.progress_watcher_lost`.
  (F6) New `tests/test_mcp_dogfood_e2e.py` drives MCP `tools/call`
  round-trips for `dogfood.publish_on_behalf` covering completion
  and review-verdict paths (marked `pytest.mark.multi_repo`);
  `tests/test_supervised_progress_watcher.py` extended with
  `test_progress_loop_once_heartbeats_attached_supervisor` +
  `test_progress_loop_once_refuses_pid_identity_mismatch`. Files:
  `src/striatum/mcp.py`, `src/striatum/process_progress.py`,
  `src/striatum/db.py`, `src/striatum/daemon.py`,
  `src/striatum/daemon_pg/mcp_dispatch.py` (new),
  `src/striatum/daemon_rpc/server.py`,
  `src/striatum/dogfood/operator_tools.py`. Tests: 42 passed,
  10 skipped (multi_repo skips without PG harness).

### Decided

- D098: Cycle-exhaustion override for the dogfood-044 build review.
  4th instance of the codex/codex implementer+reviewer
  convergent-blind-spot anti-pattern (precedents D095 dogfood-042
  Track A, D096 dogfood-042 Track C, D097 dogfood-043 Python
  build). Codex needs_revision overridden; cross-lane claude
  accept_with_findings (medium), gemini accept (low). Codex
  findings absorbed into RFC 0040 V1.6 follow-up (TODO item 28).
  Anti-pattern now well-characterized across four independent
  runs; refuse-by-default validator rule (TODO item 26) remains
  the deferred half.

### Notes

- Dogfood-044 ran the 9-job single-track workflow for RFC 0040
  V1.5 (F1-F6 codex findings from dogfood-040). The `consolidate`
  job was not present in the workflow; the operator authored this
  changelog entry, `docs/rfcs/README.md` status update,
  `docs/TODO.md` follow-ups, `docs/dogfood/044/BUILD_HANDOFF.md`,
  and `docs/dogfood/044/PHASE_1_OPERATOR_NOTES.md` out-of-band
  (dogfood-043 lesson applied).
- Stale-lease intervention: codex finished writing code, but the
  supervisor lease expired at ~30 min default before the
  HANDOFF.md was published; operator composed the build HANDOFF
  on behalf of the implementer (per-finding status read from
  source). 30-min default-lease issue noted as a harness gap
  separate from the V1.5 implementation scope.
- Byline-prefix bug observed in 3 of 4 reviewed dogfoods now:
  both gemini and claude reviewers emit
  `(role)-lane-unknown-model-NN` instead of
  `(role)-unknown-model-NN`. Operator hand-edited the bylines
  before publication.

## v1.32.0 — 2026-05-13

### Added

- RFC 0045 V1 multi-phase workflow schema landed
  (`striatum.workflow.v1.1`): new top-level `phases` array, a
  `phase_synthesis` job type that gates phase transitions, validator
  rules refusing cross-phase dependencies that bypass the synthesis
  gate, runtime materialization of phase synthesis fan-in edges,
  `status --json` derives `phases` + `current_phase_id` from the
  workflow snapshot plus latest job attempts, dashboard + service
  run-detail surfaces receive phase progress from the status payload,
  workflow generator gains `shape: "multi_phase"` emitting v1.1
  workflows with phased track jobs + synthesis gates, and
  `striatum workflow upgrade --add-phases` previews by default and
  writes with `--apply`. V1 workflows continue to validate and run
  unchanged. Files: `src/striatum/workflow.py`,
  `src/striatum/cli/{introspect,mutations,parser,dispatch,workflow}.py`,
  `src/striatum/{dashboard,service}.py`,
  `src/striatum/workflow_generator/{core,catalog}.py`,
  plus tests under `tests/test_workflow_phases.py`,
  `tests/test_workflow_generator.py`, `tests/test_workflow_upgrade.py`,
  `tests/test_cli_mvp.py`, `tests/test_dashboard.py`,
  `tests/test_service.py`, and fixture
  `tests/fixtures/multi_phase_workflow.json`.
- RFC 0045 V1 React Flow editor extensions (Track B): phase color
  bands rendered via `<ViewportPortal>` so bands pan/zoom with nodes;
  cross-phase edges receive distinct styling
  (`className: "cross-phase-edge"`, thick black stroke,
  `data: { crossPhase, sourcePhase, targetPhase }`); new
  `PhaseInspector` swaps into the right-hand inspector slot when a
  band header is clicked (edit `title`/`description`, show
  `synthesis_job_id`, list jobs in phase); drag-drop refuses
  cross-band moves with snap-back + inline `role="alert"` error;
  new `phase` selector in the job inspector (gated on
  `workflow.phases?.length > 0`); `syncWorkflowEdges` strips
  derived `crossPhase`/`sourcePhase`/`targetPhase` keys on save;
  `selectedJobId` upgraded to a `GraphSelection` discriminated union.
  V1 workflows keep the original square-grid layout, thin grey edges,
  and job-only inspector with no visual change. Files:
  `src/striatum/web/frontend/src/shared/types.ts`,
  `src/striatum/web/frontend/src/shared/theme.css`,
  `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`,
  and new unit suites in
  `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`.

### Decided

- D097: Cycle-exhaustion override for the dogfood-043 Python build
  review. 2-of-3 cross-lane reviewers accept (claude
  accept_with_findings low, gemini accept low); codex needs_revision
  (high) overridden because the codex/codex implementer+reviewer
  pairing produced the third instance of the convergent-blind-spot
  anti-pattern (precedents D095 dogfood-042 Track A, D096 dogfood-042
  Track C). Codex findings (cycle phase-jump validator gap, strict
  phase-skip restriction, phase_id strict-on-v1 check, drag-drop
  dropdown bypass, malformed v1.1 tolerance) absorbed into RFC 0045
  V1.5 follow-up (TODO item 27). Anti-pattern now well-characterized
  across three independent runs; full validator refuse-by-default
  remains the deferred half of TODO item 26 (a soft warning landed in
  the dogfood-043 prep commit).

### Notes

- Dogfood-043 ran with two parallel tracks (Track A Python core
  implemented by codex; Track B React Flow editor implemented by
  claude) and 3-way build review postures (codex threat_model,
  claude ergonomics_dx, gemini adversarial). The `consolidate` job
  was not present in the workflow; the operator authored this
  changelog entry, the `docs/rfcs/README.md` status update, the
  `docs/TODO.md` follow-ups, the `docs/dogfood/043/BUILD_HANDOFF.md`
  cross-track handoff, and the `docs/dogfood/043/PHASE_1_OPERATOR_NOTES.md`
  operator narrative in its place (dogfood-042 lesson applied: the
  in-workflow consolidate job was the wrong locus when the operator
  is already the synthesizing surface).

## v1.31.0 — 2026-05-13

### Added

- Track A (dogfood-042): RFC 0039 V1 Steps 1+2 Go daemon core landed
  under `go/`. New `go/cmd/striatumd` entry point, `go/pkg/rpc`
  (envelope-v1 validation/serialization, RFC 0030 method registry,
  capability vocabulary, in-memory capability helpers, handshake,
  `daemon.describe`, duplicate request detection, RPC server framework
  for read-only routes), `go/pkg/db` (daemon Postgres config
  resolution/redaction, dependency-free `psql` runner, migration
  loading/application, embedded SQL migrations, audit hash/recording),
  `go/go.mod` + `go/go.sum` + `go/Makefile`, and root `Makefile`
  `daemon-go-build` / `daemon-go-test` / `daemon-go-lint` targets.
  Python harness gained `daemon_core: Literal["python","go"]` parameter
  on `DaemonProcess` and `MultiRepoHarness` (default `"python"`,
  backward-compatible); Go invocation resolves the binary via
  `STRIATUMD_GO_BIN` or `<repo>/go/bin/striatumd` and runs
  `make -C go build` on demand. Phase 1 partial — Steps 3-6 (CLI
  integration, mutating verbs, supervised processes, distribution)
  deferred to a Phase 2 dogfood per RFC 0039 §9. Documentation
  updated in `docs/HOW_TO_HUMAN.md`, `docs/SPEC.md`, and
  `docs/UBIQUITOUS_LANGUAGE.md`.
- Track B (dogfood-042): RFC 0044 drafted as the Engram Phase 1
  implementation spec — Engram as an optional read-only memory
  augmentation for Striatum operators. Pull-mode ingestion with
  Striatum-owned redacted JSONL export, Engram-owned
  `ingest-striatum`, standalone `engram-mcp-stdio` MCP server, four
  read-only retrieval tools, Engram-local `memory.*` capabilities,
  and a hard augmentation-not-dependency boundary. RFC text only;
  implementation lands via a future dogfood.
- Track C (dogfood-042): repo-local-state-to-Postgres design work
  superseded by main's RFC 0043 (Postgres as Sole Substrate and
  Daemon-Required Runtime, accepted via D094) which landed during this
  dogfood from the parallel session. Track C dogfood artifacts (3
  designs + synthesis + 3 reviews + decision) retained under
  `docs/dogfood/042/track_c/` as historical provenance; the draft
  `docs/rfcs/0042-repo-local-state-to-postgres.md` is NOT shipped
  (collides with main's RFC 0042 number, scope absorbed by RFC 0043).

### Decided

- D095: Cycle-exhaustion override for Track A Go daemon build.
  2-of-3 reviewers accept_with_findings (claude, gemini); codex
  needs_revision overridden because the codex/codex
  implementer+reviewer pairing converged on its own findings. Codex
  findings absorbed into RFC 0039 V1.5 follow-up (TODO item 24).
  Follow-up: forbid codex/codex implementer+reviewer pairs in the
  workflow validator (TODO item 26).
- D096: Cycle-exhaustion override for Track C build review.
  2-of-3 reviewers accept-equivalent (claude accept, gemini
  accept_with_findings); codex needs_revision overridden because the
  same codex/codex anti-pattern recurred. Track C's repo-local-PG
  design intent is absorbed by main's RFC 0043; the draft RFC file is
  not shipped.

### Notes

- The dogfood-042 multi-phase workflow with three parallel tracks
  completed with two cycle-exhaustion overrides (D095 Track A, D096
  Track C). The `consolidate_phase_1` job was cascaded into
  cancellation; the operator wrote this changelog entry, the
  `docs/rfcs/README.md` index updates, the `docs/TODO.md` follow-ups,
  and the `docs/dogfood/042/BUILD_HANDOFF.md` cross-track handoff in
  its place.

## 1.30.0 — 2026-05-13

### Added

- RFC 0038 V1 web UI feature additions and frontend toolchain. Vite +
  React + TypeScript contributor-side toolchain bundled into the wheel
  via `src/striatum/web/static/build/` (operators stay pip-only). New
  `make ui-install` / `make ui-build` / `make ui-dev` / `make ui-test`
  targets. Five user-facing additions: workflow detail's Edit
  affordance promoted from a muted text link to a button next to "Run
  this workflow now"; new `/view/` repo file browser with lazy
  expansion via `GET /v1/repo/tree`; new `/workflows/new` chooser
  wizard over the RFC 0034 V1 generator endpoints with a
  `<dialog>`-driven operator confirmation gate; drag-drop React Flow
  workflow graph editor with structured per-node widgets at
  `/workflows/edit/<path>`; Shiki-based syntax-highlighted code viewer
  for non-Markdown files at `/view/<path>` with line numbers, copy,
  raw link, and wrap toggle. New `docs/FRONTEND_DEVELOPMENT.md`
  contributor guide. Dark-mode parity inherited from `base.css`.
- New shared TypeScript prop contract in
  `src/striatum/web/frontend/src/shared/types.ts` mirroring the
  workflow validator's closed vocabularies.

### Decided

- D092 (re-cited): supersede D073's implicit "no node toolchain" rule
  for the contributor-side build path. Operator install remains pip
  only; bundled JavaScript ships in the wheel under
  `src/striatum/web/static/build/`. Bundle drift is detected by a
  committed `manifest.sha256` in CI.

## 1.29.0 — 2026-05-12

### Added

- RFC 0040 V1 operator-side slice of the MCP-driven dogfood harness:
  twelve dogfood-lifecycle chat-tool entries (`run_prepare`,
  `run_start`, `register_session`, `supervise_start`, `claim_next`,
  `ack`, `publish_artifact`, `verdict`, `complete`, `supervise_stop`,
  `run_summary`, `evidence_export`) over `striatum.api.invoke`. Ten
  state-mutating tools are gated behind `serve --allow-mutations`; two
  read-shaped tools (`run_summary`, `evidence_export`) stay available
  unconditionally.
- Per-model harness-profile fragments baked into the bundled workflow
  template catalog (`claude_code_default`, `codex_default`,
  `gemini_default`, `generic_default`). `workflow generate` enriches
  any user-supplied profile body with the catalog defaults when
  `native_delegation.instruction` is missing; existing instructions
  are preserved verbatim.
- `striatum workflow upgrade <path>` CLI verb that backports the
  catalog fragments into existing workflow.json files. Refuse-on-
  conflict default with `--force` to overwrite; `--dry-run` reports
  the change set without writing; refuses when any non-terminal run
  references the workflow.
- New `docs/HARNESS_FRICTION_PATTERNS.md` documenting the four
  observed friction patterns (036 strategy-then-exit, 037 ask-and-
  exit, 038 lease-expiry-under-active-load, 038/039 front-matter
  completeness) and the V1 fixes.
- `docs/MCP.md` "Dogfood-Lifecycle Tools" section listing each new
  tool, its capability requirement, and an example sequence.
- `docs/HOW_TO_HUMAN.md` walkthrough of driving a dogfood through the
  MCP chat tools instead of bash CLI, plus a `workflow upgrade`
  recipe.
- `docs/HOW_TO_AGENT.md` note for operator-AI sessions to prefer the
  MCP chat tools over shelling out to bash; supervised roles still
  use the work-packet `commands` block verbatim.

### Decided

- D093: Accept RFC 0040 V1 as the operator-side slice. Composite
  tools (`dogfood.publish_on_behalf`, `dogfood.surgical_recovery`)
  and the daemon-side supervised-progress heartbeat land in the
  systems half of the RFC. See `docs/HARNESS_FRICTION_PATTERNS.md`
  for the long-form record.

## 1.28.0 — 2026-05-13

### Added

- RFC 0037 V1 web UI ergonomics: run/workflow filters, run duration
  and workflow last-modified columns, grouped doctor problems with a
  terminal-run filter, UTC/local timestamp toggle, graph tooltips,
  keyboard shortcuts, app-specific dark-mode parity, promoted run next
  actions, and empty states for the main triage pages.

## 1.27.0 — 2026-05-13

### Added

- RFC 0035 V1 test infrastructure: `tests/_harness/MultiRepoHarness`
  for ephemeral Postgres + daemon + multi-repository e2e coverage, five
  cross-repo harness-backed test modules, and `make test-multi-repo`.

## 1.26.0 — 2026-05-13

### Added

- RFC 0036 V1: `striatum-mcp` skill coverage for loose skill installs
  and plugin bundles, plus chat tools `generate_workflow_preview` and
  `generate_workflow_write` over the RFC 0034 workflow generator.
- Chat workflow writes are hidden when `serve` lacks
  `--allow-mutations`; crafted write calls fail with
  `mutations_disabled`.
- Chat workflow writes queue a one-shot operator confirmation in the web
  UI before generated workflow files are written.

## 1.25.0 — 2026-05-12

### Added

- RFC 0034 V1 workflow generator: bundled shape/lane-set catalog,
  `workflow templates list/show`, `workflow generate`, local service
  catalog and generation endpoints, custom-plan compilation, and
  `workflow init --style` compatibility over the generator.

### Decided

- D091: OPERATOR_REPORT.md is written incrementally during a dogfood
  run, not only at the end. Refines D089. The operator appends a dated
  entry per intervention (publish-on-behalf, recovery sweep,
  override-verdict, decision recording) at the moment it occurs;
  end-of-run only writes the wrap-up sections.

## 1.24.0 — 2026-05-12

### Added

- RFC 0032 V2 slice: cross-repo workflow schema validation, repo-local
  `runs.cross_repo_run_id`, daemon DB migration v3 for cross-repo run
  metadata, daemon RPC method scope modes, `recovery` capability, daemon
  MCP `tools/list` filtering and `tools/call` re-authorization/audit
  scaffolding, and mocked cross-repo lifecycle helpers.

### Documentation

- Documented the dogfood-035 deferral: real two-repo daemon end-to-end
  integration tests and live scheduler progression wait for TODO Open
  item 19, the multi-repo test harness RFC.

## 1.23.0 — 2026-05-11

### Added

- RFC 0030 daemon RPC foundation: envelope-v1 codec, newline JSON
  framing helpers, owner-local Unix socket and loopback HTTP guards,
  `daemon.hello` / `daemon.welcome`, `daemon.describe`, a
  capability-bound method registry, and PostgreSQL request/audit helper
  wiring.
- RFC 0031 daemon-owned supervision/apply foundation: daemon DB
  migration v2 for method metadata, daemon supervisor ownership, and
  apply receipts; repo-local migration v13 for supervisor pointers; and
  fail-closed apply-key/refusal helpers.

### Documentation

- RFC 0030 and RFC 0031 are now marked accepted for the V2 foundation.
  Docs distinguish the landed RPC/schema boundary from deferred
  cross-repo workflows, MCP mutation expansion, hosted services,
  Windows daemon support, and any claim of third-party cryptographic
  non-repudiation.

## 1.22.1 — 2026-05-11

### Fixed

- Byline tolerance: a Markdown-decorated byline like
  `**Author:** value`, `# Author: value`, or `_author_: value` is now
  recognised by the publisher and stored in `artifacts.author_line`
  as the canonical lowercase `author: value` form. Models seen in
  dogfood-031 and dogfood-033 produced the bold-decorated form, which
  previously caused the publisher to silently drop the byline (stored
  as NULL); the canonicaliser now normalises decoration before
  matching. Mismatched bylines still refuse with the documented error.

### Added

- `publish-artifact` auto-attaches default front matter for the
  `synthesis` artifact kind when the file omits the `---` block. The
  publisher prepends `schema_version: "striatum.synthesis.v1"` and
  `artifact_kind: "synthesis"` (the only required fields, both
  constants the publisher already knows from `--kind synthesis`),
  rewrites the file on disk so the stored SHA agrees with downstream
  reads, and proceeds with the rest of validation. The agent's body
  is preserved verbatim after the prepended block. Other
  schema-bearing kinds (`finding`, `decision`, `findings_ledger`,
  `support_ledger`, `action_item_ledger`,
  `harness_improvement_proposal`) have semantic required fields the
  publisher cannot invent (`verdict_intent`, `outcome`, etc.) and
  continue to silently accept missing front matter — adding an
  explicit refusal there would be a policy break and should land
  behind a workflow-level opt-in.

- Hard byline + front matter discipline section in every dogfood-033
  design prompt (`design_codex.md`, `design_claude_code.md`,
  `design_gemini.md`, `synthesize_design.md`): forbid Markdown bold
  (`**Author:**`), heading prefix (`# author`), italics (`_author_`),
  and quotes around the value; require lowercase `author:` exactly;
  include the JSON-encoded front matter template for schema-bearing
  kinds.

## 1.22.0 — 2026-05-11

### Added

- RFC 0033 daemon PostgreSQL substrate scaffolding: optional
  `striatum-orchestrator[daemon-pg]` driver dependency, packaged
  forward-only daemon DB baseline migration, `daemon doctor`
  PostgreSQL onboarding checks, `daemon start --postgres-url`, and
  `daemon migrate --from sqlite --to pg` cutover wiring with V1 audit
  hash preservation.

### Documentation

- RFC 0033 is now documented as accepted V2: daemon-owned state moves to
  operator-supplied system PostgreSQL, with forward-only daemon DB
  migrations and `striatum daemon migrate --from sqlite --to pg` for the
  V1 registry cutover. The docs keep repo-local
  `.striatum/state.sqlite3` as workflow truth and avoid claiming daemon
  RPC, MCP mutations, daemon-owned supervision, cross-repo mutation, or
  sealed apply before their later RFCs.

## 1.21.1 — 2026-05-11

### Fixed

- Parallel-reviewer cascade-child UNIQUE collision: when two
  reviewer postures fan out from a single cycle target (e.g. three
  parallel design-review postures all routing back to one
  `synthesize_design` via `needs_revision` cycles), the second
  `submit-review` no longer raises
  `UNIQUE constraint failed: jobs.run_id, jobs.idempotency_key`.
  Cycle-target cloning is now idempotent on
  `(run_id, workflow_job_id, attempt)`; parallel reviewers share a
  single revision attempt of the shared cycle target. Surfaced in
  dogfood-031 by `dec_operator_security_cascade_collision_2026_05_11`.

### Added

- `.striatum/bin/codex-supervised-wrapper.sh` mirroring the existing
  claude/gemini supervised wrappers. Codex `exec ... -` hangs on
  empty FIFO stdin in supervised mode in some environments
  (observed during dogfood-031); the wrapper spawns a fresh
  `codex exec` per packet, matching the RFC 0010 V2 one-packet-per-
  invocation model. Updated the bundled `examples/harness-profiles/`
  workflow and `docs/dogfood/031/workflow.json` reference to use
  the wrapper so future runs avoid the FIFO hang.
- `docs/CLI_REFERENCE.md` and `docs/HOW_TO_HUMAN.md` now document
  the RFC 0028 V1 daemon/repo/dashboard verbs (`striatum daemon
  start/status/stop/sweep`, `striatumd` console script, `repo
  add/list/remove`, `--daemon` read routing, `dashboard --all`,
  bootstrap admin token semantics, audit shape, and the V1
  deferrals: no RPC server, no daemon-owned supervision, no
  mutation MCP tools, no Windows daemon support, no
  cross-repository workflows).

## 1.21.0 — 2026-05-11

### Added

- RFC 0026 V1 lane-liveness attestation: work-packet and publish-time
  bylines now downgrade unattested sessions to `author: operator`;
  attached supervised sessions regain lane/model bylines only when the
  pid identity and snapshot command binding match. Added
  `register-session --operator-label`, per-session attestation surfacing,
  and review-job `require_attested_lane: true` gates.
- RFC 0027 provenance-mode guardrails: workflows may declare
  `provenance_mode` (`advisory`, `attested_bylines`, `sealed_patch`).
  `sealed_patch` validates path policy but refuses to start until real
  source containment ships.
- RFC 0028 V1 registry-backed multi-repo acceptance slice: optional
  `striatumd` / `striatum daemon start` foreground sweep process, daemon
  registry, `repo add/list/remove`, explicit `--daemon` read routing,
  `dashboard --all`, resources-only daemon MCP, metadata-only hash-chained
  audit, and recovery sweeping across registered active runs. V1 does not
  ship a daemon RPC server; CLI and MCP clients open the owner-only
  registry SQLite directly under token/capability checks.
- Dogfood-031 revision round 2 hardens the daemon slice: unsupported
  forced-daemon verbs refuse instead of falling back to direct mode,
  `repo add` authorizes before repo-local access and requires `--init`
  for absent state databases, daemon MCP uses explicit tokens with
  repo-scope filtering, audit segment manifests are guarded and checked
  by doctor, and foreground sweeps write repo-local
  `daemon.recovery_sweep` events bylined `striatumd-<instance-id>`.
- Dogfood-031 revision round 3 removes `STRIATUM_DAEMON_TOKEN` plaintext
  env-var support, uses realpath/inode-based repository identity for new
  registrations, admin-gates manual `daemon sweep`, audits denied
  dashboard/MCP aggregate reads with client attribution on allowed reads,
  and documents RPC server, audit retention/rotation, HTTP transport, and
  full underlying recovery-byline propagation as follow-up RFC scope.

## 1.20.1 — 2026-05-10

### Added

- "Verdicts by posture" chips on `/run/<id>` are now clickable.
  New route `GET /run/<id>/posture/<posture>` lists every verdict
  recorded with that posture for the run: verdict value, review
  job, role/lane, session slug, finding artifact link, and
  rationale. Page also shows a one-paragraph "what does this
  posture mean?" explanation per RFC 0018's posture vocabulary
  (`devils_advocate`, `security`, `threat_model`, etc.).

## 1.20.0 — 2026-05-09

### Added

- RFC 0025 V1 Steps 2+3 (dogfood-029): `codex` and `gemini`
  plugin profiles, completing the V1 plugin scope.
  - **`codex` profile**: 14-file Codex plugin bundle under
    `.striatum/plugins/codex/` with `.codex-plugin/plugin.json`, 5
    skills (byte-shared with claude_code), 5 Markdown commands,
    `hooks/hooks.json`, `.mcp.json`, `README.md`, `.manifest.json`.
    User scope: `~/.codex/plugins/<namespace>/`.
  - **`gemini` profile** (promotes from RFC 0015 generic
    fallback): 14-file Gemini extension under
    `.striatum/plugins/gemini/` with `gemini-extension.json`,
    `GEMINI.md` context file, 5 skills (byte-shared), 5 TOML
    commands (bare top-level form per Gemini extension spec),
    `agents/striatum-recover.md` sub-agent definition,
    `README.md`, `.manifest.json`. User scope:
    `~/.gemini/extensions/<namespace>/`.
  - **`--profile all`** aggregates all three profiles into one
    install invocation. Result shape: `{"profile": "all",
    "results": [...]}`.
  - Marketplace fixture continues to be reentrant; gemini
    short-circuits with `{"skipped": True, "reason": "gemini has
    no marketplace concept"}` so JSON callers can detect the skip.
  - F1 byte-match test extended: skill template trees under all
    three profiles must match `skills/templates/claude_code/`
    byte-for-byte.

RFC 0025 status: **accepted (V1)** — three first-class profiles
shipped.

### Deferred to V2

- Cross-target install (one bundle into many target repos).
- Hosted marketplace.
- Codex `apps/` and `assets/`.
- Per-target git-repo extension format for gemini.

## 1.19.0 — 2026-05-09

### Added

- RFC 0025 V1 Step 1 (dogfood-028): `claude_code` plugin profile.
  - `striatum plugin install --profile claude_code` emits a
    14-file Claude Code plugin bundle under
    `.striatum/plugins/claude_code/`. Layout: `.claude-plugin/plugin.json`,
    `skills/striatum-{workflow,scaffold,claim-loop,supervise,recover}/SKILL.md`,
    `commands/{claim-next,status,why,dashboard,doctor}.md`,
    `hooks/hooks.json`, `.mcp.json`, `README.md`, `.manifest.json`.
  - `striatum plugin uninstall --profile claude_code` reads the
    bundle's manifest and deletes only manifest-tracked files;
    refuses to delete operator-edited files without `--force`.
  - `striatum init --with-plugins [profile]` mirrors
    `--with-skills`. Default profile is `claude_code`.
  - `--with-marketplace` (default on) writes
    `.striatum/plugins/marketplace.json` with a `local-striatum`
    fixture entry; reentrant — re-installs update in place.
  - Doctor checks `plugin_missing` and `plugin_outdated` walk every
    installed bundle's `.manifest.json` and surface the exact
    `striatum plugin install --profile <id>` invocation that fixes
    the drift.
  - Skill bodies are byte-shared with `skills/templates/claude_code/`
    via a CI test (`test_skill_templates_match_skills_module`)
    so future skill edits propagate to both surfaces.
  - URL-leak invariant: `test_claude_code_no_external_urls`
    forbids `https?://`, `git://`, `file://`, `ssh://`, `ftp://`
    in any rendered file.

### Deferred to V1 Step 2 / Step 3

- `codex` plugin profile (`.codex-plugin/plugin.json` + Codex
  commands).
- `gemini` profile promotion (split the current single-guide shape
  into the same five-skill structure used by claude_code).
- `--profile all` aggregation.
- Cross-target install (one bundle, many target repos).

## 1.18.0 — 2026-05-09

### Added

- RFC 0024 V4 (dogfood-027): pause/resume + per-job mutations.
  - **Migration v11** adds `runs.paused_at` and `runs.paused_reason`
    columns. Forward-only; idempotent against fresh DB whose
    schema baseline already includes them.
  - **`pause_run(conn, *, run_id, reason)`** sets the columns;
    idempotent on already-paused; refuses terminal states.
  - **`resume_run(conn, *, run_id)`** clears the columns;
    idempotent on not-paused; refuses terminal states (use
    `retry_job` to revive a canceled run).
  - **`claim_next` gate**: runs with `paused_at IS NOT NULL`
    return `{"status": "no_work", "paused": True}`. Active leases
    keep ticking; expire-leases at the top of `claim_next`
    handles paused-with-stale-leases.
  - **`retry_job(conn, *, run_id, job_id)`** resets a
    failed/canceled/blocked job to `queued`, increments
    `attempt`, marks prior `queue_messages` rows as `canceled`
    (preserving the partial unique index), re-enqueues, and
    revives canceled/failed runs to `running` with a loud
    `run.revived` event.
  - CLI: `striatum run pause/resume/retry-job`.
  - HTTP: `POST /run/<id>/pause`, `/run/<id>/resume`,
    `/run/<id>/job/<jid>/cancel`, `/run/<id>/job/<jid>/retry`. All
    mutation-gated.
  - UI: Pause/Resume buttons + paused status pill on the run
    detail page; Cancel/Retry buttons on the job detail page.
    Cancel confirm reads "Cancel this job AND its dependents…".
    All islands CSP-safe.

### Run-revival semantics (D078 follow-up)

Per RFC 0024 V4 design-review F1 (option C): when an operator
retries a job whose run is `canceled` or `failed`, the run
transitions back to `running` and a `run.revived` event is emitted
with `previous_run_state` payload. The terminal-state guarantee
softens for operator-triggered revival but stays loud (event +
documented). `retry_job` refuses to revive a `completed` run.

### Deferred to V5 if needed

- Pause-with-deadline (auto-resume at timestamp).
- Per-lane pause.
- Recovery integration of pause as escalation hook target.
- Consolidate `_read_json_body` / `_read_json_body_strict` helpers.

## 1.17.0 — 2026-05-09

### Added

- RFC 0024 V3 (dogfood-026): cancel-run mutation surface plus the
  dirty-tree visibility V2 deferred.
  - `cancel_run(conn, *, run_id, reason)` in `db.py` — top-down
    cancel that voids active leases, marks in-flight jobs (queued,
    running, blocked, ready, claimed) as canceled, transitions the
    run to `canceled`, emits `run.canceled`, and closes remaining
    sessions. Idempotent on already-canceled; refuses completed /
    failed via `InvalidTransitionError`.
  - `striatum run cancel --run-id <id> [--reason <text>]` — CLI.
  - `POST /run/<id>/cancel` — mutation-gated HTTP endpoint.
    Returns 200 on success (and on idempotent re-cancel); 405 / 404 /
    409 / 415 for the other paths.
  - Cancel button on the run-detail page when state is non-terminal
    (prepared / needs_branch_confirmation / ready / running). CSP-safe
    JS island in `/static/run_cancel.js`.
- Run-now dirty-tree visibility (closes V2 design-review F3):
  `POST /workflows/run/<path>` now returns 409 with
  `error.kind: "dirty_tree"` and `error.git_status` (first ~80
  lines of `git status --short`) when `git_create_or_checkout_branch`
  fails. Operators see the blocker without context-switching to a
  terminal.

### Deferred to V4

- Pause / resume runs.
- Auto-branch suffix (research showed multi-run-per-branch is
  by-design — the friction operators feel is dirty-tree, which V3
  fixes directly).
- Per-job mutation buttons (kill running job, retry).
- Programmatic re-run with parameter overrides.
- Recovery integration: cancel-run as an escalation hook target.

## 1.16.1 — 2026-05-09

### Added

- RFC 0024 V2.1: branch-confirm button on `/run/<id>` for runs in
  `needs_branch_confirmation` state. Operator no longer has to drop
  to the CLI when a `confirm`-mode workflow is started via the
  Run-now button. POST `/run/<id>/branch-confirm` with
  `{branch, create, use_current}` calls `branch_confirm` and
  `run_start` in one transaction; reload reveals the now-running
  run.

### Fixed

- SVG dependency graph rendered with explicit `width`/`height`
  attributes instead of relying solely on `viewBox` + CSS
  `max-width: 100%`. Small graphs no longer scale up to fill the
  full container width — boxes render at their natural pixel size
  and the SVG only shrinks for narrow viewports.

## 1.16.0 — 2026-05-09

### Added

- RFC 0024 V2 (dogfood-025): three editor additions on
  `/workflows/*` — run-now lifecycle, `If-Match` concurrency
  guard, and field-level validation errors.
  - `POST /workflows/run/<path>` — mutation-gated; calls
    `create_run + branch_confirm(create=True) + run_start`;
    returns `{run_id}` on 200; 409 on dirty-tree branch refusal;
    422 on validation failure with structured `errors[]`. When
    `branch.mode == "confirm"`, returns 200 with status
    `needs_branch_confirmation` so the operator can finish out of
    band.
  - `If-Match: <sha256>` precondition on `POST
    /workflows/edit/<path>`. GET stamps the disk sha into a hidden
    `<script id="workflow-sha256">` tag; editor JS echoes it on
    POST; on stale sha the server returns 412 with
    `current_sha256` so the editor can prompt for reload. Missing
    header → V1.5 opt-out (backward compatible).
  - `WorkflowError` extended with optional `field_path`. 8
    high-traffic raise sites tagged: `schema_version`, duplicate
    job id, unknown role, unknown lane, invalid artifact path,
    cycle references unknown job, cycle `max_iterations < 1`. The
    422 body now includes `error.errors: [{field_path, message}]`;
    editor highlights the offending form field via a
    `data-field-path` attribute. Untagged raise sites keep `None`
    and the editor falls back to the V1.5 top-of-form banner.
  - "Run this workflow now" button on `/workflows/<path>` (only
    rendered when the workflow is `valid`). On 200 navigates to
    `/run/<run_id>`. CSP-safe: behavior lives in
    `/static/workflow_run.js`.

### Workflow-trust model

V2 lets any operator with `--allow-mutations` launch a run from any
committed `workflow.json`. This matches the CLI surface (`striatum
run prepare --workflow <path>` from a shell). No new attack surface.

### Deferred to V3

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- AI-assisted scaffolding via chat tool that *writes*
  workflow.json (would require per-tool gating).
- Multi-error reporting (collect all errors, not just first).
- Field-path coverage for the remaining ~22 raise sites.
- `flock()` for hard concurrency guarantees.
- 409 body carrying `git status --short` output.

## 1.15.0 — 2026-05-09

### Added

- RFC 0024 V1.5 (dogfood-024): workflow visual builder.
  - `GET /workflows/edit/<path>` renders a form-driven editor
    for any repo-relative `workflow.json`. Existing files load
    their parsed JSON (even invalid — the editor opens so the
    operator can fix); non-existent paths render an empty
    scaffold with the workflow_id derived from the path stem.
  - `POST /workflows/edit/<path>` saves: validates the body
    via `validate_workflow`; on success atomically writes via
    `<path>.tmp` + rename; on validation failure returns 422
    with the WorkflowError message (file unchanged).
  - Mutation-gated (`--allow-mutations` required for POST).
  - Body capped at 1 MB; non-`application/json` content-types
    rejected with 415 (per design-review F1).
  - Path safety mirrors `/view/<path>`: `..`, leading `/`, null
    bytes, hidden dirs (`.git`, `.striatum`) refused.
  - JS island (`workflow_edit.js`) renders form sections from
    in-memory state: header, roles, lanes, jobs, edges, cycles.
    Add/remove buttons mutate state; save POSTs the full state
    as JSON; on success redirects to the detail page.
  - localStorage backup persists the in-progress draft so a
    browser-crash doesn't lose work; recovered with operator
    confirmation on reload.
  - "Edit" link added to the workflow detail page.

### Deferred to V2

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- "Run this workflow now" full lifecycle button.
- Field-level error highlighting (requires `validate_workflow`
  API change).
- `If-Match: <sha256>` precondition for safe concurrent edits.
- AI-assisted scaffolding via chat tool that *writes*
  workflow.json (would require per-tool gating).

## 1.14.0 — 2026-05-09

### Added

- RFC 0024 V1 (dogfood-023): workflow browser (read-only).
  - `GET /workflows` lists every `**/workflow.json` in the
    target repo with validation status, workflow_id,
    job/lane/role counts. Hidden dirs (`.git`, `.striatum`,
    `.venv`, `node_modules`, etc.) excluded from discovery.
  - `GET /workflows/<repo-path>` renders a detail page with
    the SVG dependency graph (reusing RFC 0022 V1's renderer)
    plus tables for jobs, lanes, roles, edges, and cycles.
    Invalid workflows render their `WorkflowError` message
    inline; the page never 500s.
  - Path safety mirrors `/view/<path>`.
  - New chat tool `list_workflows` extends RFC 0023 V1.5's
    closed read-only tool set; the model can answer "which
    workflow produced run X?". Capped at 100 entries.
- `Workflows` link in the top nav (between Runs and Chat).

### Deferred to V1.5

V1.5 (separate dogfood) ships the *visual builder*: form-driven
editor at `/workflows/edit/<path>`, save action with server-side
validation, per-job posture + required_review_postures widgets,
flash banner + redirect-after-save.

## 1.13.0 — 2026-05-09

### Added

- RFC 0023 V1.5 (dogfood-022): chat tool use + system-prompt briefing.
  Closes the V1.5 deferral from RFC 0023 V1.
  - **Six closed-set read-only tools** wired into the chat backend:
    `read_file(path)`, `list_dir(path)`, `striatum_status(run_id?)`,
    `striatum_why(target_id)`, `git_log(limit?)`,
    `git_diff(path?)`. The model decides when to call them; the
    backend executes server-side and feeds results back. Closed-
    set membership enforced in `execute_tool`; unknown tool names
    return error strings rather than executing. No tool that
    mutates state.
  - **Tool-call loop** in `_handle_chat_send`: up to 10 iterations
    of (request → assistant text + tool calls → execute → re-request
    with results). Loop terminates on a no-tool-calls response.
  - **System-prompt briefing** at chat-session creation: repo path,
    current branch, last 10 commits, top-level entries, AGENTS.md
    content (capped at 8 KB), active-run summary, tool-use
    guidance. The chat now has bearings on its first turn.
  - **Per-flavor tool wiring**: Anthropic Messages tool-use shape
    (content blocks with `type: "tool_use"` + `tool_result`) and
    OpenAI Chat tool-use shape (`tool_calls` + `role: "tool"`)
    both supported. Streaming tool calls are accumulated server-
    side and emitted as discrete events.
  - **JSONL transcript extensions**: new role values `tool_use`
    and `tool_result` persist tool calls + their wrapped results.
    Existing user/assistant/system roles unchanged.
  - **Prompt-injection defense**: tool results are wrapped in
    `<tool_result_begin name="..." args="..."> ... <tool_result_end>`
    delimiters. The system briefing instructs the model to treat
    content between the delimiters as data, not instructions
    (defense in depth; closes design-review F1).
- Chat history page now renders `tool_use` and `tool_result`
  entries as collapsed-by-default `<details>` blocks alongside
  user/assistant turns.

### Fixed

- **Graph-node click 404** (RFC 0022 V1 regression): SVG graph
  nodes link by *workflow* job id (e.g., `research_chat`) but
  the `/run/<id>/job/<id>` route handler queried by the *full*
  job id only. The handler now accepts either form.
- **Doctor page rendered no list**: the template referenced
  `doctor.checks` but the `doctor()` function returns
  `doctor.problems` (list[str]) and `doctor.problem_records`
  (list[dict]). Template rewritten to render the actual shape;
  CSS for the problem list added.
- **Chat double-render of user messages**: the JS island
  optimistically appended the user's message on form submit, then
  the SSE round-trip rendered the same message a second time
  (with timestamp). Optimistic append removed; the SSE stream is
  now the single source of truth for message rendering. ~250ms
  perceived latency before the user's own message appears, no
  duplication.

## 1.12.0 — 2026-05-09

### Added

- RFC 0023 V1 (dogfood-021): web chat surface +
  `/view/<path>` endpoint + inline Markdown rendering on
  artifact pages. Provider-neutral chat client streams HTTP
  to an operator-configured endpoint via four env vars
  (`STRIATUM_CHAT_API_BASE_URL`, `STRIATUM_CHAT_API_KEY`,
  `STRIATUM_CHAT_MODEL`, `STRIATUM_CHAT_API_FLAVOR`). Two
  flavors: `anthropic_messages` and `openai_chat` (covers
  OpenAI, OpenRouter, Ollama, vLLM, LiteLLM proxy, etc.).
  No default provider; operators opt in explicitly. URL
  scheme validation refuses non-loopback `http://`. Chat
  startup is `--allow-mutations`-gated.
- `/view/<path>` read-only file viewer: `.md` renders as
  HTML, text as `<pre>`, binaries as a metadata panel.
  Path traversal refused; `.git/` and `.striatum/` hidden
  by default. Directory listings deferred to V1.5.
- `/run/<id>/artifact/<id>` now renders `.md` artifact
  bodies inline (closes RFC 0022 V1.5 deferred).
- Chat transcripts in `.striatum/scratch/chat-<id>/transcript.jsonl`
  (gitignored). SQLite unchanged. No artifacts published.

### Dependency

- **`markdown-it-py` ≥ 4.0** is now a runtime dependency
  (the project's second after Jinja2). `html: False` at
  parse time; no separate sanitizer needed for V1.

### Boundary clarification (D074)

- AGENTS.md "no cloud APIs without explicit product
  decision" gets its first carve-out: outbound HTTP from
  striatum to an operator-configured endpoint is permitted
  for chat (and only chat). No hosted striatum service; no
  default endpoint; no telemetry. D028 (transcripts off)
  gets a parallel narrow carve-out: chat transcripts live
  in scratch JSONL only, never SQLite, never artifacts.

### Dogfood pattern (first 3-lane review)

- dogfood-021 declares three parallel design-review jobs
  (security, devils_advocate, threat_model) and three
  parallel build-review jobs (security, devils_advocate,
  ergonomics_dx) — first run to use RFC 0018 V1's
  `required_review_postures` reachability gate at full
  3-posture coverage.

## 1.11.1 — 2026-05-09

### Changed (docs only)

- Refresh the documentation set against the current state
  (RFCs 0001–0022, v1.11.0 features). Mention
  `--with-ddd-layout` (RFC 0021) + `--ddd-layout-force` /
  `--ddd-layout-dry-run` (V1.5) in `README.md`,
  `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`, and
  `docs/CLI_REFERENCE.md`. Update `README.md` "Status" section
  from `v1.1.0` to `v1.11.0` + add the PyPI `pip install
  striatum-orchestrator` instructions. Rewrite `docs/SPEC.md` §
  "Local Web UI" against the RFC 0022 V1 server-rendered shape
  (Jinja2 multi-page, SVG dependency graph, dark-mode CSS
  custom properties).
- Apply explicit "historical" banners to incubation-era
  documents per `docs/CONTEXT_HYGIENE.md` § "Failure modes" #1
  (mixed live/historical material with no label):
  `docs/INTERVIEW_LOG.md`, `docs/PRIOR_ART.md`,
  `docs/RFC_0014_DOGFOOD_FIX_SPEC.md`, and
  `docs/dogfood/HISTORICAL.md`. The `docs/INDEX.md` table-of-
  contents now lists these in a dedicated "Historical" section
  with a header-level callout, separating them from active
  reference material.
- `docs/dogfood/HISTORICAL.md` extended with a "current
  cadence" subsection listing recent runs (014–020) and what
  each shipped (RFC + tag + highlights), so a reader can find
  a recent canonical run instead of copying patterns from the
  incubation-era 001–013 directories.

No behavior change; no schema change; no new tests.

## 1.11.0 — 2026-05-09

### Added

- RFC 0022 V1 (dogfood-020): web UI redesign. Server-rendered
  Jinja2 multi-page UI replaces the hash-routed SPA. Five pages:
  `/`, `/run/<id>`, `/run/<id>/job/<id>`,
  `/run/<id>/artifact/<id>`, `/doctor`. Each page is real HTML
  that copy/pastes cleanly and works without JS. The JSON API
  (`/v1/*`) and SSE feed (`/events`) are unchanged.
- Refreshed visual palette: CSS custom properties for theme +
  status colors, `prefers-color-scheme: dark` media query for
  dark mode (no toggle button — OS preference wins), system
  font stack, 4px-grid spacing scale. New `base.css` replaces
  `app.css`.
- SVG dependency graph on `run_detail.html`: layered top-down
  layout (longest-path topological depth), state-colored nodes
  via custom-property `fill`, click-to-navigate to job detail,
  SVG `<title>` tooltip on hover for accessibility. Cycles
  (revision loops) are not rendered as edges — only the forward
  DAG from `workflow_graph_data().graph.edges`.
- Legacy hash-route redirect: a small JS island in `base.html`
  reads `window.location.hash` on load and rewrites
  `#/run/<id>` to `/run/<id>` so bookmarked SPA URLs still
  work.

### Dependency

- **Jinja2 ≥ 3.1** is now a runtime dependency (the project's
  first; previously zero-runtime-dep). Adds ~250 KB to the
  install size, pulls in `markupsafe` (~30 KB transitively).
  Trade-off taken for HTML correctness over hand-written
  string-format escaping.

### Removed

- `src/striatum/web/static/app.js`'s hash-router and the
  associated SPA mount. The mutation-button JS is preserved as
  a per-page island. The CSP header is byte-identical
  (`default-src 'self'; …` with no `unsafe-inline` / `unsafe-eval`).

### Deferred to V1.5

- Inline dogfood Markdown rendering on `/run/<id>/artifact/<id>`.
- SVG graph zoom / pan interactivity.

## 1.10.0 — 2026-05-09

### Added

- RFC 0021 V1.5 (dogfood-019): `--ddd-layout-force` and
  `--ddd-layout-dry-run` flags on `striatum init
  --with-ddd-layout`.
  - `--ddd-layout-force` overwrites existing regular-file
    targets with the template body. The envelope reports
    `status: "overwritten"` plus a `prior_sha256` field for
    audit. Non-regular-file targets (directories, broken
    symlinks) still surface as `status: "error"` regardless
    of force — the operator must resolve those manually.
  - `--ddd-layout-dry-run` reports what *would* happen without
    writing any files. The envelope's top-level `dry_run` flag
    is True; per-file statuses use a `would_*` vocabulary
    (`would_create`, `would_skip`, `would_overwrite`,
    `would_error`). Combine with `--ddd-layout-force` to
    preview a destructive overwrite.
  - Both flags without `--with-ddd-layout` are silent no-ops.
- `scaffold_ddd_layout(repo, *, force, dry_run)` public API
  signature is unchanged from V1; V1's `force=False,
  dry_run=False` defaults map to V1's behavior. Callers that
  pass either flag get the new V1.5 branches without
  deprecation work.

RFC 0021 status moves from `accepted (V1)` to
`accepted (V1+V1.5)`. V1.6 candidates (template parameter
substitution, multi-layout, `striatum scaffold sync`, doctor
check) remain deferred until operator evidence shows they're
wanted.

## 1.9.0 — 2026-05-09

### Added

- RFC 0018 step 3 V1.5 (dogfood-018): `verdicts.posture` column
  + introspection surfacing across six paths.
  - Migration v10 ALTERs `verdicts` to add a `posture TEXT NOT
    NULL DEFAULT 'neutral'` column and a covering index
    `idx_verdicts_posture`. Existing rows backfill to
    `'neutral'`. Forward-only; idempotent.
  - `record_review_verdict` reads the review job's posture from
    the workflow snapshot (defaulting to `'neutral'` when
    omitted) and writes it on INSERT. The `verdict.recorded`
    event payload now carries `posture` alongside `verdict`.
  - `status --json` adds a `verdicts_by_posture` dict alongside
    the existing verdict counts. Always emitted (empty dict
    when no verdicts) for stable shape.
  - `run summary` Markdown adds a `[posture: \`<name>\`]` suffix
    on each per-build verdict line *only* when at least one
    non-neutral posture exists in the run. Posture-omitting
    runs render byte-identically to v1.8.1.
  - `evidence export` JSON snapshot includes `posture` on every
    verdict row.
  - `run graph --format json` adds `posture` to each review
    node's `latest_verdict` block (when a verdict exists).
  - Dashboard verdicts panel renders a `Postures: <p1>=<n1>,
    <p2>=<n2>` summary line when at least one non-neutral
    posture exists. Sorted by count descending, then posture
    name ascending for deterministic ties; truncates to the
    top-3 with `+N more` overflow.
  - Web UI verdict list renders a posture chip alongside each
    verdict badge for non-neutral postures. New
    `.posture-chip` CSS class with `max-width: 12em` +
    `text-overflow: ellipsis` for long `custom:<name>` strings;
    full posture name shows on hover via `title` attribute.

### Changed (intentional)

- `evidence export` JSON snapshot's per-verdict block now
  includes a `posture` field. Downstream consumers parsing the
  redacted snapshot by key name (e.g. `verdict`,
  `findings_artifact_id`) tolerate the additive field; consumers
  that rely on a fixed shape may need an update.

### Tests

- `tests/test_review_postures_introspection.py` (15 cases)
  covering migration idempotency, submit-review backfill across
  declared/undeclared/custom postures, and each of the six
  introspection surfaces (including byte-identical zero-
  regression assertions for posture-omitting runs).

## 1.8.1 — 2026-05-09

### Changed

- PyPI distribution renamed from `striatum` (taken on PyPI by an
  unrelated project) to `striatum-orchestrator`. Module imports
  (`import striatum`) and the `striatum` console script are
  unchanged. Operators upgrading from a hypothetical earlier
  install would `pip uninstall striatum && pip install
  striatum-orchestrator`; in practice no one was on PyPI before
  this release.

## 1.8.0 — 2026-05-09

### Added

- RFC 0021 V1 (dogfood-017): `striatum init --with-ddd-layout`
  scaffolds the seven canonical human-facing DDD documents
  (`docs/SPEC.md`, `docs/PRD.md`, `docs/DECISION_LOG.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DDD.md`,
  `docs/rfcs/README.md`, `docs/rfcs/0001-template.md`) into the
  target repo. Mirrors RFC 0015's `--with-skills` for agent-
  facing files: opt-in (default off, plain `striatum init`
  unchanged), idempotent (existing files reported as `skipped`
  with `reason: "exists"`), composable (both flags can be
  combined; scaffold runs after skills install). New
  `src/striatum/scaffold/` package with seven `.md.tmpl`
  templates shipped via setuptools package-data; `scaffold_ddd_
  layout(repo, *, force, dry_run) -> dict` envelope shape:
  `{"layout": "ddd", "files": [...], "dry_run": bool}`.
- Per-file safety: a target that exists but is *not* a regular
  file (directory, broken symlink, etc.) returns
  `{"status": "error", "reason": "target exists but is not a
  regular file"}` rather than silently `skipped`. OSError during
  write surfaces per-file as `status: "error"` without aborting
  the rest of the scaffold.

### Dogfooded

The dogfood-017 workflow itself uses RFC 0018 V1 fields for the
first time end-to-end: both review jobs declare
`review_posture: "devils_advocate"`, the build job declares
`required_review_postures: ["devils_advocate"]`, and the
workflow validator's reachability gate accepts the run.

## 1.7.0 — 2026-05-09

### Added

- RFC 0018 V1 (dogfood-016): focused adversarial review postures.
  Workflow review jobs accept a new `review_posture` field
  (closed set of nine first-class values:
  `neutral | devils_advocate | security | threat_model |
  latency_performance | ergonomics_dx | accessibility |
  compliance_license | supply_chain`, plus a `custom:<name>`
  grammar for off-list flavors). Build jobs accept a new
  `required_review_postures: [...]` list declaring which postures
  must cover the build. The work-packet `review_policy` block
  exposes `posture` when declared and appends a deterministic
  posture-specific instruction sentence to `instruction` for
  first-class postures. The workflow validator walks the directed
  edge graph in both directions from each build with
  `required_review_postures` and refuses (exit code 8) when any
  required posture is not the `review_posture` of a reachable
  review job.

### Design note

The runtime build-completion gate as written in the original RFC
text deadlocks against striatum's lifecycle (a build's `complete`
mutation precedes its downstream review's verdict by
construction); D069 / V1_ACCEPTANCE record the re-cast to a
workflow-validation gate. Today's edge-verdict mechanism plus
existing run-completion semantics preserve runtime enforcement.
RFC 0018's text is patched to match.

### Deferred

RFC 0018 step 3 (`verdicts.posture` column + introspection
surfacing in `status`, `run summary`, `evidence export`,
`run graph --format json`, dashboard, web UI) remains deferred
to V1.5 per the RFC's own implementation path.

## 1.6.0 — 2026-05-09

### Added

- RFC 0020 step 3 (dogfood-015): `striatum recovery watch
  --run-id <id>` long-lived sweeper daemon. Wraps the existing
  `recovery auto` orchestrator in a sleep loop with single-
  instance pidfile (`.striatum/scratch/recovery-watch-<run_id>.pid`),
  `SIGTERM`/`SIGINT` signal-driven shutdown, JSONL emission per
  sweep + a final `watch_exit` envelope, exit-on-terminal default,
  `--max-sweeps` cap, and the same CLI overrides as `recovery
  auto`. Stale pidfiles (dead PIDs) are overwritten cleanly;
  active-PID collisions exit 4 with a clear message. New
  `src/striatum/recovery/watch.py`. Tests at
  `tests/test_recovery_watch.py` (8 cases, including a SIGTERM
  shutdown test that interrupts a long sleep). RFC 0020
  transitions to `accepted (V1)` — the "step 3 deferred"
  qualifier drops.

## 1.5.0 — 2026-05-09

### Added

- RFC 0019 (D067): `docs/DDD.md` documents striatum's domain-
  driven framing — bounded context, ubiquitous language,
  aggregate roots, value objects, domain events, the
  CLI-as-only-write-surface invariant, and an "Adding to the
  model" section that gives future RFCs a citation pattern.
  README `## What It Is For` cites it; `docs/INDEX.md` lists
  it; the RFC template gets an optional `## Domain Modeling`
  section. Documentation only.

- RFC 0020 V1 (dogfood-014): autonomous stalled-run recovery
  step 1+2. New `striatum recovery auto --run-id <id>` one-shot
  sweeper composable with cron / systemd timer; runs lazy lease
  expiry, optional process reconciliation, autonomous review-
  only requeue (D036-safe), human_checkpoint timeout escalation,
  and eligible-blocker doctor flagging. Returns a structured
  envelope `{run_id, swept_at, policy_source, dry_run, actions,
  escalations, still_stuck}`. New optional top-level
  `recovery_policy` workflow block with workflow-declared
  thresholds and an `escalation_hook` (kinds: `marker_file`,
  `webhook`, `shell`); validator rejects `.striatum/` marker
  paths, non-http(s) webhook URLs, and negative thresholds.
  Defaults preserve today's flow byte-for-byte
  (`autonomous_*` defaults are `false`; CLI flags
  `--autonomous-review-requeue` and
  `--autonomous-process-reconcile` opt in per sweep).
  Hook runners (`marker_file`, `webhook`, `shell`) emit a status
  dict that folds into the envelope's `escalations[]`; webhook
  failures continue the sweep without raising. New
  `src/striatum/recovery/` package (`auto.py`, `hooks.py`,
  `policy.py`). Tests at `tests/test_recovery_auto.py` (21
  cases). Step 3 (`recovery watch` daemon) deferred per RFC
  0020 § 4.

## 1.4.1 — 2026-05-09

### Added

- Web UI run-level artifact rollup. The run-detail view now
  shows every published artifact for the run as a table (kind,
  logical name, path, source job, byline, timestamp, sha256
  prefix). Clicking the logical name routes to the existing
  artifact viewer; clicking the source job routes to the
  job-detail view. New endpoint `GET /v1/runs/<id>/artifacts`
  wraps the existing read-only `striatum list artifacts
  --run-id <id>` verb. The change is purely additive — it
  closes the discoverability gap from RFC 0013 V1+step 7 where
  per-run Markdown (BUILD_HANDOFF, DESIGN_SYNTHESIS, RUN_SUMMARY,
  decisions, findings) was reachable only by drilling into the
  job that produced it. 3 new tests at `tests/test_web_ui.py`
  (16 total).

## 1.4.0 — 2026-05-08

### Added

- RFC 0013 step 7 (dogfood-013): web UI mutation buttons.
  `POST /v1/invoke` was already gated by `--allow-mutations`
  (RFC 0012); step 7 adds five click-driven buttons to the SPA
  that POST the same argv shapes:
  - **Continue blocker** / **Cancel blocker** on the job-detail
    view (when an open blocker is present); maps to
    `striatum checkpoint resolve --blocker-id <id> --action {continue, cancel}`.
  - **Record verdict** on review-job detail (when state =
    running); collects verdict + rationale + session/lease ids
    and maps to `striatum verdict ...`.
  - **Record decision** on the run-detail view (always
    available; no lease required); maps to
    `striatum decision record ...`.
  - **Requeue stale review** on stale-lease review-only jobs;
    maps to `striatum recovery requeue-stale ...`.
  Each button opens a confirmation modal showing the literal
  argv before firing; destructive actions (cancel job, reject
  verdict) get a red confirm button. `/v1/health` gains an
  `allow_mutations: bool` field the SPA caches once per page
  load to hide buttons when the gate is off; the runner-side
  gate stays authoritative as defence-in-depth. CSP unchanged
  (no external deps, no `eval`, no inline handlers).
  Tests at `tests/test_web_ui.py` (5 new cases, 13 total)
  cover health-flag both states, mutation refusal without the
  flag (HTTP 405 envelope), SPA wiring grep, and the
  no-external-URL invariant.

## 1.3.0 — 2026-05-08

### Added

- RFC 0016 step 3 (dogfood-012): Unicode `fancy` graph style +
  `--graph-orient {tb, lr}`. The dashboard graph panel and
  `striatum run graph --format ascii` now support box-drawn
  rendering with portable BMP characters (`┌`, `┐`, `└`, `┘`, `─`,
  `│`, `╌╌▶` for cycle back-edges) and a left-to-right layout
  that arranges layers as columns instead of rows. Both upgrades
  fall back deterministically: `fancy → layered` when per-slot
  width drops below 14, `lr → tb` when per-column width drops
  below 14. Color path unchanged; `_format_fancy_box` wraps the
  inner content (not the box frame) so the frame stays uniform
  across states. New flags on both `dashboard` and `run graph`:
  `--graph-orient {tb, lr}` (default `tb`) and the existing
  `--graph-style` choices now include `fancy` as a real renderer.
  8 new tests in `tests/test_dashboard.py` (23 total).

## 1.2.0 — 2026-05-08

### Added

- RFC 0015 step 3 (dogfood-011): codex + gemini skill profiles
  + `--profile all`. `striatum skills install --profile codex`
  writes five Markdown files at `.codex/agents/striatum-*.md`
  reusing the Claude Code skill bodies verbatim.
  `--profile gemini` writes a single
  `striatum-STRIATUM_GEMINI_GUIDE.md` (single-guide fallback per
  RFC 0015 § "Profile coverage" until Gemini CLI's skill
  convention stabilizes; the dedicated filename keeps
  `--profile all` collision-free with `generic`).
  `--profile all` fans out across the four first-class profiles
  (`claude_code, codex, gemini, generic`) in deterministic
  order, returning a `{"profile": "all", "results": [...]}`
  envelope. `striatum init --with-skills all` works the same
  way. Doctor's `skills_missing` / `skills_outdated` checks now
  cover every profile. Tests at `tests/test_skills_install.py`
  (10 new cases, 25 total) cover idempotent regeneration,
  manifest shape, edit detection, fan-out, and template-SHA
  parity for the new profiles.

## 1.1.0 — 2026-05-08

### Changed

- RFC 0017 V1 (dogfood-010): documentation reorganization. README
  trimmed from ~1,000 lines to 125 with seven canonical sections
  (Status, Install, Quick Start (Human Operator), Quick Start
  (Coding Agent), What It Is For, Documentation Map, License).
  Behavior model, sequential 1–11 usage walkthrough, dogfood-NNN
  history, per-RFC subsections, and command reference moved out
  of the README into `docs/GETTING_STARTED.md`,
  `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`,
  `docs/WRITING_WORKFLOWS.md`, `docs/CLI_REFERENCE.md`,
  `docs/INDEX.md`, and `docs/dogfood/HISTORICAL.md`. AGENTS.md
  slimmed (153 → 104 lines) to point at `docs/HOW_TO_AGENT.md`
  rather than reciting the verbs inline. Three new tests in
  `tests/test_doc_links.py` enforce relative-link integrity, the
  README line budget, and the human/agent quick-start heading
  split. Documentation only — no behavior change, no schema move.

## 1.0.0 — 2026-05-08

First stable release. Every RFC under `docs/rfcs/` is now in an
`accepted` (or `accepted (V1)`) state, and every V1 RFC has shipped
its implementation slice. The `0.x` line tracked individual RFC
landings on top of the V1 MVP baseline; `1.0.0` is the version the
runner exposes once the full V1 surface is on main.

### Highlights since 0.1.0

- **RFC 0006** — forward-only SQLite migration system (`PRAGMA
  user_version`); a database newer than the runner exits with
  code 9.
- **RFC 0007** — workflow visualization (`workflow graph` and
  `run graph` with Mermaid / JSON / Graphviz DOT / state-annotated
  ASCII output).
- **RFC 0008** — opt-in per-job git worktree isolation
  (`worktree create | release | list`) for parallel repo-write
  jobs.
- **RFC 0009** — long-lived process supervision
  (`supervise start | send | stop | status | list`) so an agent
  CLI can be held alive across multiple work packets.
- **RFC 0010 V1+V1.5+V2** — tool harness profiles surfaced on work
  packets, plus the reference Claude Code supervised wrapper at
  `.striatum/bin/claude-supervised-wrapper.sh`.
- **RFC 0011** — explicit session close + run-terminal auto-close
  (`session close`); doctor's `active_session_on_terminal_run`
  warning now clears by construction on clean-finish runs.
- **RFC 0012 V1** — local HTTP / Unix-socket service
  (`striatum serve`) with SSE for events and a mutation gate
  (`--allow-mutations`).
- **RFC 0013 V1** — local web UI: vanilla-JS SPA bundled at
  `src/striatum/web/static/` and served by `striatum serve --web`.
- **RFC 0014 V1** — process adapter completion guarantees
  (post-exit output validation, structured blocker payloads,
  `recovery process-reconcile`, doctor `process_*` checks). Closed
  [issue #1](https://github.com/halbritt/striatum/issues/1).
- **RFC 0015 V1** — self-contained agent skill bundles
  (`striatum skills install`, `init --with-skills`, doctor
  `skills_missing` / `skills_outdated`).
- **RFC 0016 V1** — live dependency graph panel in
  `striatum dashboard`; `run graph --format ascii` reuses the same
  pure renderer for one-shot snapshots.
- **Reviewer policy & artifact contracts** — RFCs 0002/0003/0004/0005
  shipped reviewer access scope + context policy fields, support
  ledgers, action-item ledgers, and harness improvement proposals
  with V1 front-matter schemas under `striatum.artifacts`.

### Tooling

- 50 source modules under `src/striatum/`, 260 tests under
  `tests/`, lint + mypy clean. The Makefile targets `install`,
  `lint`, `typecheck`, `test`, `smoke` are the supported entry
  points.
- `pyproject.toml`'s `[tool.setuptools.package-data]` ships the
  web SPA (`striatum.web.static`) and the agent skill templates
  (`striatum.skills.templates`) with the wheel.

### Notes for upgraders

- The `1.0.0` jump from `0.5.0` is purely a release-naming change;
  every behavior in `1.0.0` already shipped on main as part of the
  `0.2.0`–`0.5.0` line.
- The `striatum.workflow.v1`, `striatum.work-packet.v1`,
  `striatum.skills.manifest.v1`, and the per-kind front-matter
  schema versions remain V1; future schema changes will continue
  to use V1.x suffixes or new V2 schemas behind explicit RFCs.

## 0.5.0 — 2026-05-08

### Added

- RFC 0015 V1 (dogfood-009): self-contained agent skill bundles.
  New `striatum skills install [--profile {claude_code, generic}]
  [--scope {project, user}] [--namespace <prefix>] [--force]
  [--dry-run]` writes a Markdown bundle into the target tree that
  teaches a Striatum-aware agent how to drive the runner without
  reading the source repo. The Claude Code profile produces five
  skills (`striatum-workflow` router plus `striatum-scaffold`,
  `striatum-claim-loop`, `striatum-supervise`, `striatum-recover`)
  under `.claude/skills/<namespace>striatum-*/SKILL.md`; the
  generic profile produces a single
  `<namespace>STRIATUM_AGENT_GUIDE.md` for any agent CLI without a
  skill-discovery convention. Each install records a
  `striatum.skills.manifest.v1` JSON manifest with the rendered
  SHA256, the bundled-template SHA256, and the runner version per
  file. A re-install is byte-identical; an operator-edited file is
  `refused_modified` without `--force`; `--dry-run` writes nothing
  and prints the plan. New `striatum init [--with-skills [profile]]`
  flag runs the same install pipeline immediately after `init`.
  New doctor checks `skills_missing` (recorded file absent on disk)
  and `skills_outdated` (manifest version older than running
  install, or template SHA drift) surface the exact `skills install`
  invocation that would clear the condition; the runner never
  auto-regenerates. The bundle emits no external URLs (a unit test
  enforces no `http://` / `https://`) and ships inside the Python
  distribution via `[tool.setuptools.package-data]`. Tests at
  `tests/test_skills_install.py` (16 cases). `__version__` bumped
  to 0.5.0 (alongside the pyproject bump). The `codex` and
  `gemini` profiles plus `--profile all` and parser-walked verb
  tables are step 3 of the RFC's path and remain deferred.

## 0.4.0 — 2026-05-08

### Added

- RFC 0016 V1 (dogfood-008): live dependency graph panel in
  `striatum dashboard`. The frame now appends a layered ASCII view
  of the run's workflow graph annotated with current job state
  (`Q`/`R`/`C`/`B`/`H`/`F`/`P`/`X`/`S`) when the terminal is at
  least 100 columns wide and 30 lines tall and the workflow has at
  least one edge. Auto-detection can be overridden with `--graph` /
  `--no-graph`; `--graph-only` hides the rest of the frame for
  graph-first viewing; `--graph-style {auto,layered,list,fancy}`
  forces a layout (`fancy` falls back to `layered` in V1);
  `--graph-no-cycles` suppresses dashed `~~>` back-edges. ANSI 16
  colors quantize the existing Mermaid state palette and are gated
  on `isatty()` plus `NO_COLOR` (de-facto standard). New
  `striatum run graph --format ascii` reuses the same pure renderer
  for one-shot snapshots. Refactor: `compute_node_states(conn, *,
  run_id)` lifted from `cli/introspect.run_graph` to
  `striatum.workflow` so the dashboard and the existing graph CLI
  share one source of truth for "current state after a requeue."
  Tests at `tests/test_dashboard.py` (11 new cases covering
  layered/list/no-cycles/color/no-color/graph-only/ASCII format
  parity and an ANSI-table-vs-Mermaid-fills coverage guard).

## 0.3.0 — 2026-05-08

### Added

- RFC 0013 V1 (dogfood-007): local web UI. Bundled vanilla-JS SPA at
  `src/striatum/web/static/{index.html,app.js,app.css}` served by
  `striatum serve --web` (no-op flag in 0.2.0; now serves the real
  UI). Five views: run list, run detail with live SSE event log,
  job detail, artifact viewer with per-kind front-matter formatting
  (decision badge, finding verdict + severity chip,
  harness-improvement-proposal target chip, synthesis input list),
  and doctor. Tiny in-house Markdown renderer with HTML escaped at
  the input boundary; no external CDN imports; CSP header on every
  static and artifact-raw response. New endpoint
  `GET /v1/artifacts/<id>/raw` streams artifact bytes for the
  viewer. Static assets ship inside the wheel via
  `[tool.setuptools.package-data]`. Tests at
  `tests/test_web_ui.py` (8 cases). Mutation buttons (step 7 of
  the RFC) deferred.

### Fixed

- CI release-metadata check now sources the expected version from
  `pyproject.toml` instead of a hardcoded constant, so version
  bumps don't require touching the script.
- Test service-readiness window bumped to 30s so cold imports on
  macOS GitHub runners don't false-fail.
- Unix-socket service test uses a short `tempfile.mkdtemp` path so
  macOS's ~104-byte AF_UNIX limit doesn't trigger.

## 0.2.0 — 2026-05-08

First tagged release since the V1 scaffolding. The backlog of RFCs
landed before this point (run recovery / dogfood fixes, reviewer
independence policy, support ledgers + critique-to-action loops +
harness meta-optimization, SQLite migrations, workflow
visualization, worktree isolation, long-lived process supervision,
tool harness profiles V1+V1.5+V2, session close + auto-close,
process adapter completion guarantees) is treated as the `0.1.0`
baseline. `0.2.0` lands RFC 0012 V1 on top of that baseline as the
first explicitly versioned release. Subsequent RFCs bump the minor
version on landing.

### Added

- RFC 0012 V1 (dogfood-006): local HTTP / Unix-socket service. New
  `striatum serve` command runs a `ThreadingHTTPServer` on TCP
  loopback (default `127.0.0.1`) or a Unix-domain socket; refuses
  non-loopback hosts at startup with exit 8. Endpoints:
  `/v1/health`, `POST /v1/invoke`, `/v1/runs`, `/v1/runs/<id>`,
  `/v1/runs/<id>/why`, `/v1/runs/<id>/dashboard`,
  `/v1/runs/<id>/events` (SSE), `/v1/doctor`. Mutations gated
  behind `--allow-mutations` (whitelist of read verbs); auth via
  filesystem permissions on Unix sockets or optional `--token` on
  HTTP (length-safe constant-time compare). Single-instance via
  PID file; graceful shutdown on SIGTERM/SIGINT. New module
  `src/striatum/service.py`; tests at `tests/test_service.py` (16
  cases). Closes the long-standing D006 promise of an "optional
  Unix-socket / local HTTP API later for Slack, TUI, and web
  adapters" — the four V1 acceptance criteria all pass.
- RFC 0014 V1 / issue #1 (dogfood-005): process adapter completion
  guarantees. After every `striatum adapter run` exit (including
  timeout-fired SIGTERMs), the runner inspects required
  `expected_artifacts` and, for review jobs, the verdict table. When
  any required output is missing — or the child exited non-zero or
  hit the timeout — the job transitions from `running` to `blocked`,
  a blocker row is inserted with a structured `blocker_kind`
  (`process_outputs_missing`, `process_review_verdict_missing`,
  `process_exit_nonzero`, `process_timeout_exceeded`,
  `process_lost_with_outputs_missing`), and a privacy-safe diagnostic
  envelope is recorded as the new `blockers.payload_json` column.
  The envelope contains zero child stdout/stderr (D028 preserved); it
  carries `process_id`, `command`, `exit_code`, `duration_seconds`,
  `timeout_seconds`, `missing_artifact_paths`, `review_verdict_missing`,
  and operator-copyable `recovery_commands`. New CLI surface:
  `striatum adapter run --timeout-seconds <n>` (overrides
  `lanes.<id>.adapter_timeout_seconds`; capped at 86400) and
  `striatum recovery process-reconcile --run-id <id>` (mirrors the
  `recovery requeue-stale` lazy-on-CLI shape from D036). Two new
  doctor checks (`process_running_but_pid_gone`,
  `process_running_with_expired_lease`) and a `process_health`
  summary on `striatum status --run-id`. Migrations v8
  (`process_executions.state` enum + `'timed_out'` and `'lost'`) and
  v9 (`blockers.payload_json`); both idempotent against fresh DBs.
  Tests at `tests/test_process_adapter.py` (15 new cases). Closes
  [issue #1](https://github.com/halbritt/striatum/issues/1).
- `branch.mode` is now a closed enum (`"auto"` or `"confirm"`) and
  defaults to `"auto"` when omitted. In auto mode, `run prepare`
  atomically creates the suggested branch and transitions the run to
  `ready`, eliminating the separate `striatum branch confirm --create`
  step that was previously required. The response includes
  `branch_mode`, `branch`, `branch_created`, `current_git_branch`, and
  any warning. Workflows that explicitly want the manual gate can set
  `branch.mode: "confirm"`; behaviour there is unchanged. If git
  checkout fails during auto mode (dirty tree, conflicting branch),
  the run falls back to `needs_branch_confirmation` so the operator
  can resolve the issue and run `branch confirm` manually. Migrated
  the in-repo dogfood-001/-001-v2/-002/-003/-004 and the
  `examples/harness-profiles/` workflows to auto mode; remaining
  example fixtures keep `mode: "confirm"` for test-coverage symmetry.
  Five new tests in `tests/test_cli_mvp.py` cover the auto path,
  default-when-omitted, the still-functioning confirm path, unknown
  mode rejection, and the auto-without-suggested-name guard.
- RFC 0010 V2 / HARNESS-001 (dogfood-004): reference Claude Code
  supervised wrapper at `.striatum/bin/claude-supervised-wrapper.sh`.
  Bash `while IFS= read -r` loop that spawns a fresh `claude --print`
  per packet — each Striatum work packet is independent, so per-packet
  fresh-context matches the workflow's `fresh_session_required`
  defaults and avoids depending on Claude Code's undocumented
  multi-turn `--input-format stream-json` behaviour. Inner stdout
  and stderr go to `/dev/null` (RFC 0009 / D028); SIGTERM trap
  cleans up the in-flight inner process. Verification test at
  `tests/test_claude_supervised_wrapper.py` (4 cases, stub-claude on
  `$PATH` so it does not depend on the real binary). Closes
  `docs/dogfood/003/findings/HARNESS-001.md`.
- RFC 0010 V1.5 (HARNESS-001 follow-up): workflow-validate lint warning
  for missing repo-relative process-lane command paths. Fires when
  `lane.command[0]` looks like a repo-relative path (contains a slash
  or starts with `./`/`../`) and the file does not exist under the
  workflow's repo root. Surfaces under the `warnings` key in
  `workflow validate --json` and `workflow plan --json`. Non-blocking;
  bare binary names and absolute paths are not checked. Closes the V1.5
  step of `docs/dogfood/003/findings/HARNESS-001.md`.
- RFC 0010 V1 (dogfood-003): optional `harness_profiles` workflow map
  and per-lane `harness_profile_id` reference. When a lane references a
  declared profile, `claim-next` adds a `harness_profile` block to the
  work packet (passthrough projection of the profile body plus
  `profile_id`). Workflows that omit `harness_profiles` produce
  unchanged packets. Validation accepts the closed tool-family set
  `{generic, codex, claude_code, gemini_cli}`, requires `tool_family`
  and `strategy_version`, and enforces D021 accountability
  (`native_subagents = internal_to_parent_session`,
  `first_class_registration = not_supported`). Unknown sibling fields
  on profile bodies are accepted as lint warnings, surfaced under a
  `warnings` key in `striatum workflow validate --json` and
  `workflow plan --json`. Reference fixture lives at
  `examples/harness-profiles/workflow.json`. Tests in
  `tests/test_harness_profiles.py` cover validation, packet exposure,
  backwards compatibility, and fixture loading (including the
  dogfood-003 four-profile fixture).
- D055 follow-ups (post-RFC-0011): `recovery cancel-job --cascade`
  over a whole run now transitions `runs.state` to `'canceled'`
  (previously `'completed'`) when no job actually completed; auto-close
  fires under `source: "run_canceled"`, matching the source enum value
  RFC 0011 reserved. A new `test_run_failed_auto_closes_active_sessions`
  rounds out the source-enum matrix by exercising the reject-verdict
  path that drives a run to `'failed'`. Migration helper
  `striatum.migrations.rebuild_table()` extracts the FK-safe rebuild
  pattern (PRAGMA foreign_keys OFF + IF EXISTS partial-state recovery
  + DROP/RENAME) so future migrations against tables with
  self-referential FKs do not re-discover the requirement; v7 is
  retrofitted onto the helper. v5 remains untouched as immutable
  historical record.
- RFC 0011 (dogfood-002): explicit session close + run-terminal
  auto-close. New `striatum session close --session-id <id> --reason
  <text>` command transitions an `active` session to a new `closed`
  terminal state, recording `closed_at` and a non-empty
  `close_reason` and emitting a `session.closed` event with
  `source: "explicit"`. Idempotent against already-terminal sessions
  (returns the existing row plus a `note: "session was already
  <state>"`); refuses with exit 4 when the session holds an active
  lease (message points the operator at `striatum release`). When a
  run transitions to a terminal state, every still-active session on
  the run is auto-closed inside the same transaction with `source` of
  `"run_completed"`, `"run_failed"`, or `"run_canceled"` — eliminating
  the persistent `active_session_on_terminal_run` doctor warning that
  fired on every clean-finish run before this change. Migration
  version 7 adds the `closed` state value plus the `closed_at` and
  `close_reason` columns. `evidence export` and `run summary` carry a
  per-session block with the new fields; `RUN_SUMMARY.md` gains a
  `## Sessions` section.
- HARNESS-001 fixes (dogfood-001 v2): `docs/SPEC.md` "Supervised lane
  command contract" subsection making the three supervised-lane
  requirements explicit (alive across packets, NDJSON stdin, calls back
  via `striatum` CLI). New `doctor` problem record
  `supervisor_lost_with_held_lease` plus the stable `status` next-action
  `recover_orphan_supervisor` that fires when a supervisor row is
  `lost` while the session still owns an unexpired active lease.
  `striatum supervise stop` is idempotent against an already-`lost` or
  `stopped` supervisor: returns the existing terminal row plus a
  `note` describing the prior state instead of raising
  `InvalidTransitionError`.
- HARNESS-002 fixes (dogfood-001 v2): new `doctor` problem record
  `editable_install_outside_repo` warns when the running install is
  outside the repo argument and the repo is itself a Striatum source
  tree (suppressed when the repo is just a target, to avoid false
  positives). `striatum init` against a fresh DB now refuses with exit
  3 when the repo's source-tree `LATEST_VERSION` is higher than the
  running install's, with a clear message pointing at
  `pip install -e <repo>`. `Makefile install` resolves the install path
  via `$(MAKEFILE_DIR)` so `make install` from any cwd installs *this*
  Makefile's directory in editable mode (the previous `pip install -e
  .` was cwd-dependent and silently pinned to a Claude Code worktree).
- HARNESS-003 fixes (dogfood-001 v2): `docs/SPEC.md` "Reviewer
  Independence (advisory)" and "Byline Integrity" subsections making
  the runner's enforcement boundary explicit. New `doctor` problem
  record `reviewer_independence_unverified` flags two observable
  breaches — sessions that share a supervisor pid, or a reviewer
  session running unsupervised on a run whose author is supervised.
  `register-session --role reviewer` refuses when the workflow
  declares `reviewer_context_policy: fresh` and an active author
  session already exists, unless `--force-non-fresh --reason "..."` is
  passed; the reason is recorded in the new
  `sessions.non_fresh_reason` column. `publish-artifact` records the
  artifact file's actual `author:` line in the new
  `artifacts.author_line` column (NULL when the file omits it);
  evidence exports and run summaries read the actual column so a
  missing byline renders as `author: <missing>` rather than the
  workflow's declared expected. Migration version 6 adds both columns.
- HARNESS-004 fix (dogfood-001 v2): `docs/dogfood/001/roles/reviewer.md`
  now points reviewer harness proposals at
  `docs/dogfood/001/review/HARNESS-NNN.md` (inside the review job's
  `write_scope.allowed_paths`) instead of `docs/dogfood/001/findings/`
  (which is the author's path and is rejected by the publisher with
  exit 6). `tests/test_harness_v2_fixes.py::test_reviewer_role_doc_paths_match_write_scope`
  walks every dogfood reviewer role doc and asserts each
  `HARNESS-NNN.md` instruction path is contained in the corresponding
  review job's allowed paths.
- `striatum workflow graph --format dot <workflow.json>` emits a Graphviz
  `digraph striatum_workflow { ... }` alongside the existing Mermaid
  (default) and JSON outputs. Same nodes, dependency edges, parallel
  groups (rendered as `subgraph cluster_<group>` blocks), and bounded
  `needs_revision` cycle edges (rendered as dashed arrows with the
  `max_iterations` count). Pipe through `dot -Tsvg` to render.
- Three new artifact kinds and front-matter schemas (RFCs 0003/0004/0005,
  accepted): `support_ledger` (`striatum.support_ledger.v1`),
  `action_item_ledger` (`striatum.action_item_ledger.v1`), and
  `harness_improvement_proposal`
  (`striatum.harness_improvement_proposal.v1`). Migration version 5 drops the
  SQL `CHECK (artifact_kind IN (...))` on the `artifacts` table; allowed kinds
  now live in `striatum.artifacts.ALLOWED_ARTIFACT_KINDS` and are enforced by
  `publish-artifact` (`ArtifactError`, exit 6) and workflow validation
  (`WorkflowError`, exit 8). Reference fixture
  `examples/support-ledger-flow/` exercises the produce -> support ledger ->
  evidence audit -> final review pattern; "evidence audit" is a workflow
  convention name, not a new `job_type`.
- Reviewer independence policy fields on review jobs (RFC 0002, D051).
  `type: "review"` jobs may declare `reviewer_access_scope`
  (`document_only` | `artifact_augmented` | `repo_level`) and
  `reviewer_context_policy` (`fresh` | `cross_round`). The validator
  rejects unknown values, rejects the fields on non-review jobs, and
  rejects the explicit `reviewer_context_policy: "fresh"` +
  `fresh_session_required: false` conflict. Setting
  `reviewer_context_policy: "fresh"` without `fresh_session_required`
  silently stores the prepared job row with `fresh_session_required = 1`.
  Work packets gain a `review_policy` block (`access_scope`,
  `context_policy`, `instruction`) only when the workflow declares at
  least one of the fields; existing fixtures produce identical packets.
  The `examples/rfc-0014-operational-artifact-home/workflow.json` fixture
  now labels its three independent root reviews as `document_only` and
  `fresh`.
- `striatum run graph --run-id <id> [--format mermaid|json]` renders the
  workflow graph for an existing run with each node colored by current job
  state. Mermaid output appends a `classDef` palette plus per-node `class`
  assignments (completed/running/claimed/acked/blocked/stale_lease/
  waiting_human/failed/canceled/queued/pending); JSON output adds
  `current_state`, `attempt`, and a `latest_verdict` block on review nodes.
  The runner picks the highest-`attempt` row per `workflow_job_id` so
  requeued attempts show their latest state.
- `striatum list ...` subcommand group for read-only enumeration of runs,
  sessions, jobs, artifacts, and workflow snapshots. Each command returns a
  stable `{"items": [...], "count": N}` envelope shaped from existing SQLite
  state. `list runs` joins `workflow_snapshots` to surface `workflow_id`;
  `list sessions --run-id <id>` accepts `--state`, `--role`, `--lane`;
  `list jobs --run-id <id>` includes the latest verdict for review jobs and
  accepts `--state` and `--workflow-job-id`; `list artifacts --run-id <id>`
  embeds the structured author byline and accepts `--kind`; `list workflows`
  reports loaded snapshots with their `content_sha256`. Every run-scoped
  variant applies the lazy lease-expiry sweep before reading.
- `striatum checkpoint resolve --blocker-id <id> --action {continue|cancel}
  [--decision-id <id>]` resolves an open `human_checkpoint` blocker:
  `continue` re-queues the affected job and emits `checkpoint.resolved`;
  `cancel` marks the affected job `canceled` and emits
  `checkpoint.canceled`. Optional `--decision-id` validates a run-level
  decision artifact and records it on the resolution event payload.
- `striatum recovery cancel-job --run-id <id> --job-id <id> --reason <text>
  [--cascade]` is the explicit operator cancel for a non-terminal job.
  Refuses terminal-state jobs and refuses jobs with blocked dependents
  unless `--cascade` is set, in which case dependents are canceled
  transitively in the same transaction.
- Supervised-aware `claim-next`: when the claiming session has an
  `attached` supervisor, the runner writes the freshly built packet
  through the supervisor's stdin pipe inside the same transaction,
  refreshes `heartbeat_at`, and emits a `supervisor.packet_delivered`
  event. The CLI response gains an optional `supervisor_delivery` field.
  Pipe-missing or write-fail transitions the supervisor to `lost` while
  still committing and returning the packet so the caller can recover.
- Optional per-kind Markdown front-matter validation in `publish-artifact` for
  `decision` (`striatum.decision.v1`), `finding` (`striatum.finding.v1`),
  `findings_ledger` (`striatum.findings_ledger.v1`), and `synthesis`
  (`striatum.synthesis.v1`). Front matter is read with a minimal
  `key: <json-value>` parser, validated only when present, and never rewritten
  by the publisher. Other artifact kinds remain unschemaed.
- New example fixtures: `examples/human-checkpoint-flow/` (analyze -> review
  -> decide, where the decide job is a `human_checkpoint`-typed job whose
  session calls `block --severity human_checkpoint` to surface an operator
  checkpoint and the operator records the decision via
  `striatum decision record --outcome accepted`), and
  `examples/adapter-unavailable-flow/` (a process-lane workflow that requests
  `network=enforced` and is rejected at validation because the process adapter
  only provides `advisory_strict` for that constraint). Both are covered by
  end-to-end tests in `tests/test_cli_mvp.py`.
- `striatum dashboard` command: a compact, dependency-free terminal dashboard
  over the existing SQLite state that summarizes run state, job counts,
  verdicts, open blockers, claimable work, deterministic next actions, and
  the most recent events. Supports `--refresh` for live mode and `--once` for
  one-shot rendering in scripts and CI.
- Long-lived process supervision (RFC 0009). New `striatum supervise
  start | send | stop | status | list` commands hold an agent CLI alive
  across multiple work packets: `start` forks the lane command with
  `start_new_session=True` and a per-supervisor named pipe at
  `.striatum/scratch/<supervisor_id>/stdin.pipe`, `send` delivers a stored
  work packet as a newline-terminated JSON line through that pipe, `stop`
  sends `SIGTERM` (then `SIGKILL` after a five-second grace), `status` probes
  liveness and lazily transitions stuck rows to `lost`, and `list` reports
  supervisors for a run. The single-shot `striatum adapter run` command is
  unchanged — both flows coexist. Migration version 4 adds the new
  `process_supervisors` table with a partial unique index enforcing "at most
  one active supervisor per session". `expire_leases` marks supervised
  sessions `lost` without auto-killing the OS process, and `striatum doctor`
  flags supervisors whose pid is gone or whose stdin pipe is missing from
  disk. Stdout and stderr are sent to `DEVNULL`; the supervisor never
  captures transcripts or parses agent output for workflow state, preserving
  D028 and D037.
- `striatum workflow init [--style minimal|review|code-change] <path>` writes
  a starter workflow tree (`workflow.json` plus `roles/` and `prompts/`
  stubs) that validates cleanly with `workflow validate`. Refuses to
  overwrite an existing path. The `review` default mirrors the
  `examples/code-change-flow/` shape with placeholder paths; `minimal` skips
  review; `code-change` adds a one-shot `needs_revision` cycle.
- New example fixtures: `examples/code-change-flow/` (draft -> review -> apply
  with a one-shot needs_revision cycle) and
  `examples/failed-review-revision-cycle/` (single review whose second
  needs_revision opens a configured human checkpoint).
- Opt-in per-job git worktree isolation for parallel repo-write jobs
  (RFC 0008). Lanes declare `worktree_isolation: per_job` and the runner
  advertises `worktree_required: true` plus the `striatum worktree create`
  command on matching work packets without auto-creating anything. New CLI
  subcommands `worktree create | release | list` manage the worktrees,
  `publish-artifact` reads files from the active per-job worktree but
  records logical repo-relative paths so artifacts stay valid main-branch
  provenance, lease expiry marks worktrees `abandoned` for operator
  inspection, and `doctor` flags orphaned and missing-on-disk worktree rows.
  Migration version 2 adds the new `job_worktrees` table.
- Forward-only SQLite migration system. Schema version is tracked through
  `PRAGMA user_version`, the current schema is registered as
  `user_version = 1`, `striatum init` and every connect apply pending
  migrations inside a single `BEGIN IMMEDIATE` transaction, and a database
  newer than the runner supports is refused with the new exit code 9.
- Fourth adapter enforcement level `advisory_strict` (between `advisory` and
  `enforced`). The process adapter graduates `network=forbidden` and
  `repo_scope=local_only` to `advisory_strict`: proxy env vars are scrubbed
  from the child env when network is forbidden, and
  `STRIATUM_NETWORK_POLICY` / `STRIATUM_REPO_SCOPE` sentinels are set so
  cooperating agents can honor the policy.
- RFC 0009 (proposed) describing the V2 long-lived process supervisor for
  agent CLIs that span multiple work packets.

### Changed

- Split `striatum.cli` from a single ~3.5k-line module into a package
  (`src/striatum/cli/`) organized by concern: `parser`, `dispatch`,
  `mutations`, `introspect`, `evidence`, `run_summary`, `recovery`,
  `worktree`, `supervise`, and `workflow_init`. Public surface is preserved
  via re-exports in `striatum/cli/__init__.py`; the `striatum.cli:main`
  console entry point and `python -m striatum.cli` continue to work
  unchanged. Behavior is identical (pure refactor, all existing tests pass);
  cross-module helper calls that need to honor `monkeypatch.setattr` against
  `striatum.cli` use a lazy `from striatum import cli as _cli` lookup.
- `striatum doctor --verbose` now augments the historical string `problems`
  list with a `problem_records` list of structured rows. Each record carries
  a stable `check` name (e.g. `active_job_without_active_lease`,
  `stale_queue_message_claim`, `worktree_path_missing_on_disk`), the
  affected `id`, and a small `context` map. The string list is preserved
  verbatim so callers that already grep `problems` keep working.
- `striatum run summary` Markdown output now groups verdicts by review job
  with an attempt count and rolled-up prior verdicts, appends the structured
  author byline (`author: <role>-<model>-<ordinal>`) to each artifact line,
  surfaces the recorded branch alongside the current git branch with an
  explicit `(MISMATCH)` annotation when they differ, and prints a Timing
  block with `created_at`, `started_at`, `completed_at`, and wall-clock
  `duration`.
- Workflow validator now rejects cross-job expected-artifact path
  collisions, write-scope `allowed_paths` that overlap `forbidden_paths`,
  expected artifacts outside the job's write scope, unsound revision cycles
  whose target does not feed back into the cycle source through workflow
  edges, and parallel groups that mix `repo_write` with review-only jobs.
- Workflow validator emits a deprecation warning to stderr when jobs declare
  the legacy `needs` field; `edges` remains authoritative.
- Cycle resolution now redirects downstream dependencies to the new review
  attempt so jobs gated on the review verdict unblock once the new attempt
  accepts.
- MCP wrapper now speaks LSP-style `Content-Length` framing by default with
  automatic line-delimited fallback. Real MCP clients (Claude Desktop, IDE
  MCP integrations) can connect cleanly; existing line-delimited scripts and
  tests keep working unchanged. Added `python -m striatum.mcp --framing
  {auto,line,framed}` for operators that need to pin the wire shape.
- `striatum branch confirm` now honors the previously inert `--create` and
  `--use-current` flags and adds a new `--strict` flag. `--create` runs
  `git checkout -b <branch>` (with idempotent fallback to `git checkout`),
  `--use-current` records the actual current git branch, and `--strict`
  refuses to record unless the working tree already matches. Default
  behavior remains records-only, and the JSON response now includes `mode`
  and `created` fields.
- Replaced the evidence-export key-name blocklist with a default-deny policy
  registry. Any field not explicitly classified as `safe` in
  `EVIDENCE_POLICY` is redacted from exported Markdown, so future schema
  additions cannot silently leak agent or user prose.
- Pushed the `fresh_session_required` filter in `claim_next` into a single
  SQL query using a `NOT EXISTS` correlated subquery, replacing the
  per-candidate Python loop. Added covering index migration for
  `work_packets(run_id, session_id)`.

### Tooling

- (No tooling-only changes pending in this Unreleased window. Tooling work
  in this cycle is bundled with the feature commits above.)

## 0.1.0 - 2026-05-07

- Split Striatum from Engram with history preserved from the former
  `agent-runner/` incubation directory.
- Renamed the package, CLI, workflow schema, and repo-local state directory
  to `striatum`.
- Replaced the initial all-rights-reserved status with Apache-2.0 licensing.
- Added standalone project metadata, CI, and a fresh-clone smoke script.
- Added workflow planning, run-summary export, stale-lease recovery
  introspection, local API wrapper, and minimal process-adapter launch
  support.
- Added workflow graph export, bounded stale-work requeue, decision-artifact
  recording, a local MCP-like stdio wrapper, and explicit adapter
  enforcement validation.
- Added stricter release checks with `ruff`, `mypy`, wheel/sdist smoke, and
  installed package metadata validation.
