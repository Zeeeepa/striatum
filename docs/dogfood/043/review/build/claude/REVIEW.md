---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0045", "multi-phase-workflow", "build"]
---

author: reviewer-unknown-model-003

# RFC 0045 Build Review — claude / ergonomics_dx

Posture: developer-ergonomics. Evaluating both the Python core handoff
(`docs/dogfood/043/build/python/HANDOFF.md`) and the frontend handoff
(`docs/dogfood/043/build/frontend/HANDOFF.md`) from a first-time-operator
perspective: are the multi-phase affordances discoverable, are validator
and drag-drop errors actionable, are `workflow upgrade --add-phases` and
the React Flow band rendering surface obvious without prior context.

Bottom line: **accept_with_findings**. The two surfaces compose into a
coherent first-contact experience for the v1 → v1.1 transition. Three
ergonomics gaps are worth surfacing before this lands as the canonical
multi-phase shape: the React Flow editor has no phase-creation
affordance, the Python handoff does not cite literal validator error
strings (so the "operator-actionable" claim is not directly verifiable
from the handoff alone), and the `phase_synthesis` job-type backed by a
review-lifecycle row in SQLite is a leak worth a CHANGELOG note.

## Required-checks summary

| Required check (all lanes)                                  | Status | Evidence |
| ----------------------------------------------------------- | ------ | -------- |
| v1 workflows still validate and execute unchanged           | met    | python HANDOFF cites `tests/test_cli_mvp.py::test_v1_workflow_fixtures_validate_without_phase_progress`; frontend HANDOFF cites preserved `jobsToNodes` / `workflowToEdges` / `syncWorkflowEdges` / `syncWorkflowJobs` / `newJobFromBlock` / `PALETTE_BLOCKS` tests, plus a new `jobsToNodes (v1 path)` assertion that v1 keeps the `cols = ceil(sqrt(n))`, 220×140 grid. |
| RFC 0045 AC bullets → implementation site                   | met (with gap) | See AC mapping table below. The handoff implies coverage; one AC (validator error naming) is asserted but the error strings themselves are not cited verbatim. |
| Validator output is operator-actionable                     | partially met | Python HANDOFF asserts the validator "rejects invalid cross-phase edges" and "requires `phase_id` on phased jobs", and a focused suite `tests/test_workflow_phases.py` exists; no literal error string is reproduced. Frontend drag refusal error is well-described (see below). |
| React Flow band rendering does not break v1 graph rendering | met    | Frontend HANDOFF: "gated entirely on `(workflow.phases ?? []).length > 0`. v1 workflows keep the original square-grid layout, thin grey edges, and job-only inspector — no visual changes for them." Reinforced by the `jobsToNodes (v1 path)` test and `workflowToEdges (v1.1 cross-phase tagging) … v1 workflows never receive cross-phase styling`. |

## AC bullet → implementation site (RFC 0045 §"Acceptance Criteria")

| AC bullet                                                         | Implementation site (per handoffs)                                  |
| ----------------------------------------------------------------- | ------------------------------------------------------------------- |
| 1. v1.1 well-formed accepted; ill-formed refused with named errors | `src/striatum/workflow.py` (validator) + `tests/test_workflow_phases.py`; named-error claim asserted, strings not cited (see Finding F1). |
| 2. v1 continues to validate and run unchanged                     | `tests/test_cli_mvp.py::test_v1_workflow_fixtures_validate_without_phase_progress` (python HANDOFF Verification). |
| 3. `workflow validate` reports phase structure on v1.1            | python HANDOFF: "`status --json` derives `phases` and `current_phase_id` from the workflow snapshot"; introspection edits cited in `src/striatum/cli/introspect.py`. |
| 4. `workflow generate --shape multi_phase` produces valid v1.1    | python HANDOFF: "generator supports `shape: 'multi_phase'` and emits v1.1 workflows with phased track jobs and synthesis gates" (`src/striatum/workflow_generator/{core,catalog}.py`, `tests/test_workflow_generator.py`). |
| 5. `workflow upgrade --add-phases` infers phases + writes upgrade  | python HANDOFF: "`striatum workflow upgrade --add-phases` previews by default and writes with `--apply`" (`tests/test_workflow_upgrade.py`). Preview-by-default is exactly the safe-default ergonomics RFC §6 calls for. |
| 6. React Flow editor renders phase color bands + distinct edges    | frontend HANDOFF: `<ViewportPortal>` band overlay; `workflowToEdges` adds `className: "cross-phase-edge"`, `style: { stroke: "#000", strokeWidth: 3 }`, and `data: { crossPhase, sourcePhase, targetPhase }`. Tests in `__tests__/workflow-graph-editor.test.ts`. |
| 7. `status --json` includes `phases` block for v1.1 runs           | python HANDOFF: "`status --json` derives `phases` and `current_phase_id` from the workflow snapshot plus latest job attempts." Dashboard + service surfaces also receive it (`src/striatum/dashboard.py`, `src/striatum/service.py`). |
| 8. `tests/fixtures/multi_phase_workflow.json` exercises lifecycle  | python HANDOFF cites the fixture and `tests/test_cli_mvp.py::test_multi_phase_workflow_lifecycle_and_phase_progress`. |

## Strengths (ergonomics_dx)

### Discoverability of the multi-phase shape

- **`shape: "multi_phase"` in the generator catalog** is the right place
  for a first-time operator to land. The CLI surface from RFC 0034 V1
  (`striatum workflow templates list` / `workflow generate --shape …`)
  is unchanged in shape, so an operator who already knows about
  `code_change` discovers `multi_phase` through the same verb.
- **`workflow upgrade --add-phases` previews by default**
  (`src/striatum/cli/workflow.py`, per HANDOFF) is the textbook
  safe-default UX for a destructive operation: an operator who runs
  the verb without `--apply` sees the proposed diff and cannot
  accidentally rewrite the workflow on first contact. This matches
  RFC 0045 §6's "refuse to write unless `--apply` is also passed."
- **`status --json` and dashboard pick up `phases`** without an opt-in
  flag — a first-time operator runs the usual `striatum status --json`
  on a v1.1 run and immediately sees `current_phase_id`. No new verb
  to memorize.

### Validator + runtime ergonomics

- **`phase_synthesis` reuses the review lifecycle** at runtime
  (python HANDOFF Deviation #2). Operationally this means a
  first-time operator does not need a new mental model — claim →
  publish → verdict applies. Good ergonomic reuse, with a caveat (see
  Finding F3).
- **Focused phase suite is named clearly:**
  `tests/test_workflow_phases.py` is the obvious place a new
  contributor will look. Good discoverability of test surface.

### React Flow editor

- **v1 path is byte-identical** to today's experience: bands are
  gated on `(workflow.phases ?? []).length > 0` (frontend HANDOFF
  "Scope shipped"). For a first-time operator opening a legacy
  dogfood-NNN workflow there is *no* new visual noise. This is
  exactly the right backwards-compat ergonomics — and the dedicated
  `jobsToNodes (v1 path)` test plus the preserved baseline test list
  in "Test coverage added" gives confidence the regression surface
  is covered.
- **Band-header click opens `PhaseInspector`** with editable
  `title` / `description` and a job list, and clicking a job row jumps
  back to the job inspector. That round-trip discoverability
  ("everything in the band is one click away") is the most important
  first-contact affordance for a phased workflow.
- **Drag refusal across bands** ships `role="alert"` inline error
  referencing the job id and the declared phase (frontend HANDOFF
  "Drag-drop refusal across bands" + "Suggested reviewer focus"). That
  satisfies the "validator error messages name field, rule, suggested
  fix" rubric for the editor-side error path: an operator dragging
  into the wrong band sees *what* refused (this job), *which rule*
  (phase boundary), and the implicit fix (release into the correct
  band, or change `phase` via the inspector field).
- **`syncWorkflowEdges` strips derived `crossPhase` / `sourcePhase`
  / `targetPhase` keys** before serialization. From a first-time
  operator's perspective this means a v1.1 round-trip through the
  editor does not pollute `workflow.json` with editor-internal
  state — the JSON they inspect matches what the validator sees.
- **Phase field rendered only when `workflow.phases?.length > 0`**
  in the job inspector is correct progressive disclosure: v1
  authors don't see a phantom "phase" field, v1.1 authors do.

## Findings

### F1 — Validator error strings not cited verbatim (low)

**Posture:** ergonomics_dx — first-time operator gets the validator
text, not the test name.

**Evidence:** Python HANDOFF asserts validation "rejects invalid
cross-phase edges" and "requires `phase_id` on phased jobs", and the
RFC 0045 AC #1 demands "named errors." But the handoff does not
reproduce the actual error strings the operator will see. The
RFC §2 promises errors that "name the field, the rule, and a
suggested fix." The reviewer cannot, from the handoff alone,
confirm that the messages distinguish between (a) missing `phase_id`
on a phased job, (b) duplicate `phases[].id`, (c) cross-phase edge
bypassing `synthesis_job_id`, and (d) `phase_synthesis` job missing
its phase. From an ergonomics perspective this is the difference
between a first-time operator self-correcting and reaching out to a
collaborator.

**Suggested follow-up (non-blocking):** add a `tests/test_workflow_phases.py`
assertion that pins each error message's content (e.g.
`assert "phase_id" in err and "must reference an existing phase"
in err`), or include the literal strings in the next handoff /
the eventual CHANGELOG entry. Reviewers in a future
`fresh_context` pass would then have a verifiable surface.

### F2 — No phase-creation UI in the React Flow editor (low)

**Posture:** ergonomics_dx — first-time operator wanting to create
their first phased workflow inside the editor must drop to JSON or
to the CLI generator.

**Evidence:** frontend HANDOFF "Known V1 limitations":
*"No phase creation / reordering UI — operators edit `phases[]` via
JSON or the Python `workflow_generator` `multi_phase` shape."* The
PhaseInspector can *edit* `title` / `description` but not *create*
a new phase or reorder phases. For a returning operator this is
fine — they already understand the multi-phase shape and reach for
`workflow generate --shape multi_phase` or `workflow upgrade --add-phases`.
For a first-time operator browsing into the editor with a v1
workflow, the path to "make this multi-phase" is not visible from
the canvas. The closest signal is the existence of the `phase`
field on the job inspector — but it only renders for v1.1.

**Suggested follow-up (non-blocking):** either (a) call this out in
docs / contextual tooltip ("To convert this workflow to multi-phase,
run `striatum workflow upgrade --add-phases <path>`"), or (b)
defer to a follow-up RFC explicitly. Either is fine; the current
state is *consistent* (the React Flow editor mirrors RFC 0045 §5's
explicit scope of "drag-drop respects phase boundaries", which
implies phases pre-exist). Calling it out in CHANGELOG suffices for
V1.

### F3 — `phase_synthesis` runtime row is a `review` row (low)

**Posture:** ergonomics_dx — operator inspecting SQLite or audit
chain sees a different `job_type` than the workflow snapshot
declares.

**Evidence:** Python HANDOFF "Deviations": *"`phase_synthesis`
runtime rows are stored as review-lifecycle rows while the workflow
snapshot retains the authored `phase_synthesis` type."* For an
operator running `striatum status --json` this is invisible (phase
progress is derived from the snapshot). For an operator debugging
with `sqlite3 .striatum/state.sqlite3 "SELECT id, job_type FROM
jobs WHERE ..."` this looks like the `phase_synthesis` type was
silently dropped. The deviation is *correct* — the DB migration is
out of write scope — but a first-time operator may file this as a
bug.

**Suggested follow-up (non-blocking):** add a one-line note to the
RFC 0045 status section, the CHANGELOG, or `docs/SPEC.md` under the
Workflow Config section: "until the DB migration lands, runtime
rows for `phase_synthesis` jobs have `job_type = 'review'`; the
authored type is preserved on the workflow snapshot." This protects
the next first-time operator from a 30-minute debugging session.

### F4 — Frontend bundle not built + UI tests not run on this turn (informational)

**Posture:** ergonomics_dx — verification surface for the reviewer.

**Evidence:** Frontend HANDOFF "Verification status": *"`striatum
ack` was denied … `make ui-test` / `npm test` were also denied. I
shipped the code and the new unit suites without running `npm test`
or `npm run build` locally. The reviewer should expect to run
`make ui-test` and `make ui-build` against this branch as part of
the build-review job."* This is **not** an ergonomics defect — the
handoff is explicit and the prompt itself permits proceeding when
`striatum ack` is denied. Flagged so the next-stage reviewer who
controls a less-restricted lane runs the build + test before
merge. The bundle-hash CI gate from RFC 0038 §3 will catch a
mismatch independently.

## Per-lane angle (claude / ergonomics_dx)

The packet's per-lane angle calls out four targets explicitly. Each
one mapped to a verdict-relevant observation:

1. **First-time-operator discoverability of multi-phase shape** —
   covered by `workflow generate --shape multi_phase` (CLI) and
   `workflow upgrade --add-phases` (preview-by-default conversion).
   Editor discoverability has a gap (F2). Net: **discoverable through
   CLI; partial through editor.**

2. **Validator error messages operator-actionable** — handoff
   asserts named errors and a dedicated `tests/test_workflow_phases.py`
   suite. Literal strings not reproduced (F1). Net: **claimed-actionable
   pending verbatim citation; safe enough to accept.**

3. **CLI `workflow upgrade --add-phases` UX clear** — preview-by-default
   with explicit `--apply` is the right shape. Heuristic is documented
   in RFC §6 ("Cluster jobs by `parallel_group` prefix") and the
   handoff cites `tests/test_workflow_upgrade.py`. Net: **clear.**

4. **React Flow affordances obvious** — band-header click → inspector,
   cross-phase edges visually distinct, drag-refusal with `role="alert"`
   error, v1 path unchanged. Net: **obvious for editing existing v1.1
   workflows; phase-creation path is not on the canvas (F2).**

## Verdict

`accept_with_findings` with severity `low`. None of F1–F4 is a
blocker for landing RFC 0045 V1. F1 and F3 are cheap documentation
fixes. F2 is a deliberate non-goal carried from synthesis but worth
either a tooltip or a follow-up RFC reference in CHANGELOG. F4 is
informational and downstream reviewers should run `make ui-test` /
`make ui-build` to close the loop.

The first-time-operator experience the RFC promised — opt-in v1.1
schema, validator that refuses cross-phase shortcuts, generator and
upgrade verbs that scaffold the phased shape, React Flow editor that
visually communicates phase boundaries without disturbing v1 — is
present and consistent across the two implementations.
