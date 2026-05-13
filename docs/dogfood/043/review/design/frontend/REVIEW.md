---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0045", "frontend", "design"]
---

author: reviewer-unknown-model-001

# Frontend Synthesis Review (RFC 0045 React Flow Editor)

Posture: `ergonomics_dx`. Lens: first-time-operator discoverability of the
proposed V1 frontend slice in
`docs/dogfood/043/DESIGN_SYNTHESIS_frontend.md`.

Verdict intent: **accept_with_findings**. The synthesis answers both
targeted questions in the affirmative — color-banding does help operators
locate phases without occluding node interactions, and the cross-phase edge
treatment is visually distinct enough to be discoverable on the default
theme — but several surface-level ergonomics gaps should be addressed
during implementation or recorded as follow-ups. None of them block the V1
slice.

## What works for first-time operators

- The band overlay sits *under* the React Flow node layer because
  `PhaseBands` is rendered as the first child of `<ReactFlow>` before
  `<Background />` (synthesis §"Phase Color Bands", lines 60–68). Combined
  with the very low band opacity (`rgba(*, 0.10)`, lines 99–102) this
  satisfies the explicit non-obstruction check in the prompt — nodes are
  not visually occluded and click targets remain on top of the bands.
- Phase header strip uses existing design tokens (`var(--bg-overlay)`,
  `var(--fg-muted)`, `var(--border)`, lines 101–103). Visual consistency
  with the rest of the editor is a non-trivial ergonomics win — a
  first-time operator will recognize the header as belonging to the same
  surface as the inspector.
- The `hasExplicitPhases` predicate (line 194) cleanly preserves v1
  rendering: bands return `null`, no `cross-phase-edge` styling, no phase
  selector. This is unambiguous and matches the prompt's "v1 single-phase
  rendering is preserved" check.
- Cross-phase edges are unmistakably distinct from intra-phase ones — 3px
  black stroke vs. the existing thin grey default (lines 110–120). A
  first-time operator viewing a v1.1 workflow will not mistake them for
  intra-phase edges.
- Drag-refused error message (lines 183–185) is operator-actionable: it
  names the job id, the target phase id, and the corrective action ("Use
  the inspector phase field to move it"). This is good guidance language.
- Serialization hygiene is called out explicitly: `syncWorkflowEdges` must
  drop `crossPhase`, `sourcePhase`, `targetPhase` (lines 134–136). This
  protects operators from accidentally polluting `workflow.json` with
  derived render metadata.

## Findings

### F1 — Cross-phase edge `#000` and band `rgba()` palette are not theme-aware (severity: low)

Synthesis lines 114–120 specify `stroke: "#000"` for cross-phase edges and
lines 99–102 specify a hardcoded `rgba(102, 187, 255, 0.10) / …` palette
for bands. The rest of the editor uses CSS custom properties via
`shared/theme.css`. A first-time operator on a dark theme will see a flat
black stroke that does not respect the prevailing palette — the "visually
distinct without being jarring" property may degrade outside the default
theme. Suggested fix: introduce a `--cross-phase-edge` token and per-band
custom properties (e.g. `--phase-band-0…3`) and reference them from both
the inline `style` and the CSS backstop rule on line 124–128. The inline
style override is fine to keep as the primary path, but it should read
from a CSS variable so theme switching keeps the affordance legible.

### F2 — Phase-band header click affordance is not signaled (severity: low)

The header strip is described as "clickable" (line 71–72) and the
`onSelectPhase` handler is wired up (line 65–67), but the synthesis does
not specify a hover/focus state or a cursor change. A first-time operator
will see a styled bar at the top of each band but has no signal it is
interactive until they click. Suggested fix: in `shared/theme.css`, add
`cursor: pointer` and a hover background tint to the phase header class,
and either an explicit `role="button"` plus `aria-pressed` or a `<button>`
inside the header div for keyboard reachability. This is part of the
"side panel mount" affordance check from the prompt — selecting a phase
is the only way to mount `PhaseInspector`, so its discoverability gates
the inspector's discoverability.

### F3 — `PhaseInspector` dismissal is not specified (severity: low)

Synthesis §"Phase Metadata Panel" defines the discriminated
`GraphSelection` union (lines 144–148) and says `PhaseInspector` mounts
"in the existing right-hand inspector slot" (line 165–166), but it never
specifies the dismissal path. A first-time operator who clicks a phase
header and lands on the phase inspector has no documented way to return
to "no selection" other than (presumably) clicking a job, which silently
swaps the inspector. Suggested fix: state that clicking the same phase
header toggles selection off, or that clicking blank canvas sets
selection to `null` (and confirm that the existing job inspector follows
the same convention so the two surfaces stay consistent).

### F4 — Drag-drop policy is only discoverable by failing (severity: low)

The refusal flow (lines 178–188) is well-specified but is a pure failure
path: a first-time operator learns the policy only by attempting a
cross-band drag and being snapped back with the inline error. There is
no upfront affordance (e.g. node "locked-to-phase" badge, cursor change
when crossing a band boundary mid-drag) suggesting the constraint
exists. Suggested fix during implementation: as the drag crosses a band
boundary, render a `not-allowed` cursor or a band overlay tint so the
operator gets predictive feedback. This is consistent with the prompt's
"drag-drop policy is consistent and discoverable" check — the current
plan covers *consistent* but only weakly covers *discoverable*.

### F5 — Silent fallback for unknown `phase` value (severity: low)

Line 86–87: "place unknown or missing phase ids in the first explicit
phase". For a first-time operator who fat-fingers a `phase` value in
`workflow.json`, the editor will silently render their job in `phases[0]`
with no warning. The validator catches this at workflow load time, but
the editor surface should signal it too. Suggested fix: when a job's
`phase` does not match any `phases[].id`, render the node with a small
warning badge in its corner (reusing the existing `cycle-edge`-style
warning vocabulary if one exists), and/or surface the unmatched id in
the job inspector. This is low severity because the validator is the
authoritative gate, but the editor's silent fallback is an ergonomics
hazard for anyone hand-editing workflows.

### F6 — `phase` vs `phase_id` dual reading without a deprecation marker (severity: low)

Synthesis lines 44–48 describe `jobPhaseId(job) = String(job.phase ??
job.phase_id ?? "")` and explain `phase_id` is "read-compatible only".
This is fine for the V1 slice, but a first-time operator reading the
code will see two field names and have no in-file signal that `phase_id`
is the historical/discouraged form. Suggested fix: a one-line code
comment at the read site noting `phase_id` is legacy-compat and the
canonical field is `phase`. Lightweight; informs future readers without
adding behavioral complexity.

## Non-findings (explicitly checked, no issue)

- v1 single-phase rendering preservation: clean and well-bounded by the
  `hasExplicitPhases` predicate (line 194). No regressions for v1
  workflows.
- Phase coordinates not persisted: lines 92–93 are explicit that
  coordinates are UI-only and do not write back to `workflow.json`. Good
  invariant; respects the existing editor contract.
- `parallel_group` layout within a band (lines 88–91): uses an
  intuitive group-then-row layout that matches the current square-grid
  convention as closely as the band geometry allows. No discoverability
  concern.
- `PhaseInspector` props contract (lines 154–163) is clean and matches
  the shape of the existing job inspector — a reader of one will follow
  the other.

## Summary

The synthesis is implementation-ready and answers the targeted ergonomics
questions positively. The findings above are surface-level polish items
that should be addressed during implementation or noted as follow-ups,
not blockers. Recommend **accept_with_findings**.
