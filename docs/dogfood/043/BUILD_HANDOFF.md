author: implementer-codex-1

# Dogfood-043 Build Handoff — RFC 0045 Multi-Phase Workflow

Run: `run_648f79036ed441ed81073254207389a0`
Branch: `striatum/dogfood-043-rfc-0045`
Workflow: `docs/dogfood/043/workflow.json` (15 jobs, 2 parallel tracks)

This handoff consolidates the two per-track HANDOFFs into a single
cross-track summary. The per-track files remain authoritative for
file-level detail:

- Track A (Python core): `docs/dogfood/043/build/python/HANDOFF.md`
- Track B (React Flow editor): `docs/dogfood/043/build/frontend/HANDOFF.md`

## What shipped

### Track A — Python core (`implement_python`, codex)

- `striatum.workflow.v1.1` schema accepted. Validation enforces declared
  `phases`, requires `phase_id` on phased jobs, requires exactly one
  `phase_synthesis` job per phase, and rejects invalid cross-phase edges
  (cross-phase deps must route through the phase's `phase_synthesis` gate).
- Workflow graph materialization adds phase synthesis fan-in edges while
  keeping `needs` comparison on authored edges only.
- Run preparation materializes `phase_synthesis` jobs onto the existing
  review lifecycle so the SQLite `jobs.job_type` constraint does not
  require an out-of-scope migration.
- `status --json` derives `phases` and `current_phase_id` from the
  workflow snapshot plus latest job attempts; dashboard + service
  run-detail surfaces consume that block.
- Workflow generator: `shape: "multi_phase"` emits v1.1 workflows with
  phased track jobs and synthesis gates.
- `striatum workflow upgrade --add-phases` previews by default and
  writes with `--apply`.
- Files: `src/striatum/workflow.py`,
  `src/striatum/cli/{introspect,mutations,parser,dispatch,workflow}.py`,
  `src/striatum/{dashboard,service}.py`,
  `src/striatum/workflow_generator/{core,catalog}.py`, fixtures, and
  tests under `tests/test_workflow_phases.py`,
  `tests/test_workflow_generator.py`, `tests/test_workflow_upgrade.py`,
  `tests/test_cli_mvp.py` (two scoped cases),
  `tests/test_dashboard.py`, `tests/test_service.py`.
- Verification: `make lint`, `make typecheck`, `make smoke` all passed;
  734 tests with 702 passed / 31 skipped / 1 failed
  (`test_doc_links.py::test_decision_log_rows_under_word_budget`,
  outside this packet's write scope — `docs/DECISION_LOG.md` row D094
  exceeds the word budget).

### Track B — React Flow editor (`implement_frontend`, claude)

- Phase color bands implemented via `<ViewportPortal>` (so bands
  pan/zoom with nodes); four-entry deterministic palette;
  `phases[].color` read but not yet plumbed to band fill (V1
  optional).
- Cross-phase edges receive `className: "cross-phase-edge"`, inline
  `style: { stroke: "#000", strokeWidth: 3 }`, and
  `data: { crossPhase, sourcePhase, targetPhase }`. CSS backstop rule
  added beside `.cycle-edge`.
- `PhaseInspector` swaps into the right-hand inspector slot when a
  band header is clicked (edit `title`/`description`, show
  `synthesis_job_id`, list jobs in phase).
- Drag-drop refuses cross-band moves: `onNodesChange` snaps position
  back and renders an inline `role="alert"` error; intra-band drags
  pass through unchanged.
- Job inspector gains a `phase` selector when
  `workflow.phases?.length > 0`; canonical key is `phase`, with
  read-compat `phase_id` accepted by `jobPhaseId`.
- `syncWorkflowEdges` strips derived `crossPhase`/`sourcePhase`/
  `targetPhase` keys before serialization (round-trip-safe).
- `selectedJobId` replaced by a `GraphSelection` discriminated union
  (`{kind:"job"|"phase", id}`); `onPaneClick` clears it.
- v1 codepaths preserved: square-grid layout, thin grey edges,
  job-only inspector — no visual change for v1 workflows.
- Files: `src/striatum/web/frontend/src/shared/types.ts`,
  `src/striatum/web/frontend/src/shared/theme.css`,
  `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`,
  and added unit suites in
  `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`.
- Verification: `striatum ack` and follow-up `make ui-test` / `npm test`
  were denied at supervised turn (permission gate); code + unit suites
  shipped per the "if ack is denied, write HANDOFF and exit normally"
  rule. Reviewer is expected to run `make ui-test` and `make ui-build`
  against this branch.

## Build review verdicts

Three-way build review with distinct postures:

| Reviewer | Posture | Verdict | Severity |
|----------|---------|---------|----------|
| codex | threat_model | needs_revision | high |
| claude | ergonomics_dx | accept_with_findings | low |
| gemini | adversarial | accept | low |

**Codex `needs_revision` overridden via D097**
(`dec_2c5fbf49e91441aca3562a66919ea8c1`, `accepted_with_follow_up`).
The override applies the same logic as D095 (dogfood-042 Track A) and
D096 (dogfood-042 Track C): the codex implementer + codex reviewer
pairing produces convergent-blind-spot findings that 2-of-3 cross-lane
majority overrides. Dogfood-043 is the **third** independent recurrence
of the same anti-pattern, which now is well-characterized.

Codex findings absorbed into RFC 0045 V1.5 follow-up (TODO item 27):

1. Cycle phase-jump validator gap (cross-phase edge inside a declared
   cycle bypasses the phase-synthesis gate).
2. Strict phase-skip restriction (`phase_n → phase_n+2` jumping a
   phase entirely should be rejected, not merely warned).
3. `phase_id` strict-on-v1 check (a `phase_id` on a v1 workflow that
   declares no `phases` should refuse rather than silently ignore).
4. Drag-drop dropdown bypass (the inspector `phase` selector lets an
   operator move a job across phases without re-validating the
   resulting edge set).
5. Malformed v1.1 tolerance (validator should fail-closed on partial
   `phases`/`phase_synthesis` blocks rather than degrading toward v1
   behavior).

## Deviations recorded

### Track A
- No `phases` runtime tables, no `jobs.phase_id` column, no migration.
  Phase progress derived from snapshots as selected by the synthesis
  (avoids touching `src/striatum/db.py`, which is outside Track A's
  write scope). `phase_synthesis` runtime rows store as review-lifecycle
  rows while the workflow snapshot retains the authored
  `phase_synthesis` type.
- No frontend work (out of Track A's Python scope).

### Track B
- Band overlay mounted via `<ViewportPortal>` rather than as the first
  child of `<ReactFlow>` (synthesis-literal placement would pin bands
  to the viewport in React Flow v11; the portal preserves the
  functional intent that bands occupy world coordinates).
- `phases[].color` is read but not plumbed to the band fill (V1
  optional per synthesis; deterministic palette retained).
- `PhaseInspector` uses native `<input>`/`<textarea>` for title and
  description editing rather than the structured widgets in the job
  `Inspector` (synthesis specifies the contract, not the widget shape).

## Known V1 limitations carried forward (per synthesis non-goals)

- No phase creation / reordering UI — operators edit `phases[]` via
  JSON or the Python `multi_phase` generator shape.
- No automatic edge rewrite when an operator changes a job's `phase`
  via the inspector. Drags refuse; phase-via-field accepts but the
  validator (Track A) flags invalid cross-phase deps on save.
- Coordinates remain UI-only and are not persisted back to
  `workflow.json`.
- Phase progress / run status is not rendered in `PhaseInspector` yet;
  the props contract accepts an optional `status` prop for the Track A
  `status --json` phases block in a follow-up.

## Suggested reviewer focus (forwarded from the per-track HANDOFFs)

- v1 backwards-compat: open an existing dogfood-NNN workflow without
  `phases` and confirm no visual or runtime change.
- v1.1 fixture: a two-phase / two-track workflow with one explicit
  cross-phase edge through the `phase_synthesis` gate; intra-phase
  edges stay default thin grey, cross-phase render thick black.
- `syncWorkflowEdges` round-trip: load v1.1 → click Save → re-read
  posted JSON; ensure no `crossPhase`/`sourcePhase`/`targetPhase`
  keys appear in `edges[]`.
- Drag refusal: drag any job vertically into a different band; expect
  a snap-back plus the inline error referencing both the job id and
  its declared phase.
- `workflow upgrade --add-phases` preview vs. `--apply` against an
  existing fixture that already clusters work into parallel groups.

## Pointers

- Decision: `docs/dogfood/043/decisions/D097_cycle_exhaustion_python_build.md`
- Operator notes: `docs/dogfood/043/PHASE_1_OPERATOR_NOTES.md`
- Per-track HANDOFFs:
  `docs/dogfood/043/build/python/HANDOFF.md` and
  `docs/dogfood/043/build/frontend/HANDOFF.md`
- Per-track DESIGN_SYNTHESIS:
  `docs/dogfood/043/DESIGN_SYNTHESIS_python.md` and
  `docs/dogfood/043/DESIGN_SYNTHESIS_frontend.md`
- Operator report (per-intervention narrative):
  `docs/dogfood/043/OPERATOR_REPORT.md`
