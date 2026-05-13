author: operator-claude-opus-1

# Dogfood-043 Phase 1 Operator Notes — RFC 0045 Multi-Phase Workflow

Run: `run_648f79036ed441ed81073254207389a0`
Branch: `striatum/dogfood-043-rfc-0045`
Workflow: `docs/dogfood/043/workflow.json` — 15 jobs, 2 parallel tracks
(Track A Python core, Track B React Flow editor); 3-way design + 3-way
build review postures (codex threat_model, claude ergonomics_dx,
gemini adversarial).

## What shipped

### Track A — Python core (codex implementer)

RFC 0045's schema half landed end-to-end. `striatum.workflow.v1.1`
introduces a top-level `phases` array and a new `phase_synthesis` job
type that gates each phase's transition. Validator refuses cross-phase
dependencies that do not route through the phase's `phase_synthesis`
gate. Runtime materialization adds the phase-synthesis fan-in edges
automatically while keeping `needs` comparison on authored edges only,
so v1 workflows continue to run unchanged. `status --json` derives
`phases` and `current_phase_id` from the workflow snapshot plus the
latest job attempts, and the dashboard + service run-detail surfaces
display the phase block when the workflow has one.

Authoring surfaces caught up: the workflow generator gains
`shape: "multi_phase"` and `striatum workflow upgrade --add-phases`
inspects existing parallel_group clusters, prints a preview by default,
and writes with `--apply`. No migration was required — phase synthesis
runtime rows ride on the existing review lifecycle, and phase progress
is derived rather than persisted in `jobs.phase_id`.

`make lint`, `make typecheck`, and `make smoke` all passed; the focused
phase suite passes; `make test` ran 734 tests with 702 passed, 31
skipped, and 1 failure that is outside this packet's write scope
(`test_decision_log_rows_under_word_budget` against an over-budget
`docs/DECISION_LOG.md` row from a prior dogfood).

### Track B — React Flow editor (claude implementer)

RFC 0045's editor half landed in the
`/workflows/edit/<path>` React Flow island from RFC 0038. Phases
render as color-banded horizontal lanes through a `<ViewportPortal>`
so the bands pan and zoom with the nodes (synthesis-literal placement
would have pinned them to the viewport). Cross-phase edges get a
distinct visual style (thick black stroke + `cross-phase-edge`
class + `crossPhase`/`sourcePhase`/`targetPhase` data); intra-phase
edges keep the default thin grey. A new `PhaseInspector` swaps into
the right-hand inspector slot when a band header is clicked, letting
the operator edit `title`/`description` and see the phase's
`synthesis_job_id` plus its job roster. Drag-drop refuses cross-band
moves with a snap-back and an inline `role="alert"` error.
`syncWorkflowEdges` strips the derived cross-phase keys before save,
so the round-trip is clean. v1 workflows render with the unchanged
square-grid layout, thin grey edges, and job-only inspector.

`striatum ack` and the follow-up `make ui-test` / `npm test` were
denied at the supervised turn (permission gate); the implementer
shipped code + unit suites per the "if ack is denied, write HANDOFF
and exit normally" rule. The reviewer is expected to run
`make ui-test` and `make ui-build` against this branch.

## Third instance of the codex/codex anti-pattern

Build review verdicts:

- codex (threat_model) — `needs_revision`, high severity.
- claude (ergonomics_dx) — `accept_with_findings`, low severity.
- gemini (adversarial) — `accept`, low severity.

D097 (`dec_2c5fbf49e91441aca3562a66919ea8c1`) overrides the codex
`needs_revision` verdict on the same grounds as D095 and D096: when
the codex implementer and a codex reviewer work on the same lane,
the reviewer's findings cluster around the implementer's own blind
spots. The other two cross-lane reviewers accept. With dogfood-043
we now have three independent recurrences across two runs
(dogfood-042 Track A → D095; dogfood-042 Track C → D096;
dogfood-043 Python build → D097). The anti-pattern is now
well-characterized, not anecdotal.

The codex findings are real — they describe useful tightenings, not
implementation gaps — and they are absorbed into RFC 0045 V1.5 as
TODO item 27:

1. Cycle phase-jump validator gap (a cycle edge inside a phase
   shouldn't be allowed to silently cross out of phase).
2. Strict phase-skip restriction (`phase_n → phase_n+2` should
   refuse, not warn).
3. Strict-on-v1 `phase_id` check (carrying a `phase_id` on a v1
   workflow with no declared `phases` should refuse rather than be
   ignored).
4. Drag-drop dropdown bypass (the inspector's `phase` selector
   lets an operator change a job's phase without re-validating the
   edges out of that job).
5. Malformed v1.1 tolerance (partial v1.1 blocks should fail-closed
   rather than degrade toward v1 behavior).

Toward the harness side of this, TODO item 26 was partly shipped: a
prep commit during dogfood-043 added a soft warning when the
workflow validator detects a same-model implementer↔reviewer pair on
the same lane. The full refuse-by-default behavior (validator-level
rejection with an explicit override knob) is deferred. After three
instances, the case for the hard rule is now strong; until it lands,
operators should pattern-match the override during build review.

## Manual consolidate, dogfood-042 lesson applied

Dogfood-043's workflow intentionally did not include a `consolidate`
job. In dogfood-042 the `consolidate_phase_1` job was cascaded into
cancellation because the operator had already done the cross-track
synthesis to drive the override decisions, and an in-workflow
consolidate job sitting on the dependency graph was the wrong locus
for that work. Dogfood-043 applies that lesson directly: the
operator writes the consolidate artifacts out-of-band as a normal
edit pass — this `PHASE_1_OPERATOR_NOTES.md`, `BUILD_HANDOFF.md`,
the changelog promotion, the RFC index status bump, and the TODO
follow-ups. The runner remains the source of truth for what
happened (`run_summary`, `OPERATOR_REPORT.md`, `D097`); the operator
is the right surface for the prose synthesis on top.

This works for dogfood-043 because the operator was already
synthesizing the override decision and the cross-track delta. A
future RFC could promote a dedicated "operator consolidate"
workflow shape — but until that lands, "no consolidate job; operator
writes the artifacts manually" is the cleaner pattern.

## Follow-ups absorbed into RFC 0045 V1.5

The codex findings above land via a future dogfood. Specifically:
the cycle phase-jump and strict phase-skip restrictions tighten the
Python validator; the strict-on-v1 `phase_id` check refuses
v1 workflows that smuggle phase metadata; the drag-drop dropdown
bypass requires a frontend revalidation hook on the inspector's
`phase` field; and the malformed v1.1 tolerance is a fail-closed
sweep across both the validator and the frontend's
`hasExplicitPhases` gate. None of these block V1 — V1 ships a
working multi-phase shape and editor — and all five are scoped for
V1.5.

In parallel, TODO item 26's partial ship (soft warning in the
validator) should graduate to refuse-by-default in a future dogfood,
ideally one that exercises a mixed-model implementer/reviewer
fixture to validate the rule doesn't false-positive.

## Pointers

- `docs/dogfood/043/BUILD_HANDOFF.md` — combined cross-track handoff.
- `docs/dogfood/043/build/python/HANDOFF.md` — Track A per-track.
- `docs/dogfood/043/build/frontend/HANDOFF.md` — Track B per-track.
- `docs/dogfood/043/DESIGN_SYNTHESIS_python.md` and
  `docs/dogfood/043/DESIGN_SYNTHESIS_frontend.md` — synthesis inputs.
- `docs/dogfood/043/decisions/D097_cycle_exhaustion_python_build.md`
  — override decision artifact.
- `docs/dogfood/043/OPERATOR_REPORT.md` — per-intervention narrative
  authored during the run.
- `CHANGELOG.md` v1.32.0 — promotion entry.
- `docs/TODO.md` items 26 (partial-shipped) and 27 (V1.5 follow-up).
- `docs/rfcs/README.md` RFC 0045 row — status bumped to
  `accepted (V1 implemented; codex findings folded into V1.5)`.
