---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0050", "design-review", "ergonomics_dx", "ui-rework", "v1-scope"]
---

author: reviewer-unknown-model-001

# Design Review — RFC 0050 V1 synthesis ↔ UI_REWORK.md handoff

Posture: `ergonomics_dx`. Fresh-context, document-only read of
`docs/dogfood/054/DESIGN_SYNTHESIS.md` against the canonical handoff
`docs/design/UI_REWORK.md` and the phase split declared in
`docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md`.
Evaluates whether a first-time implementer can pick up the synthesis,
walk top-to-bottom, and land V1 without re-deriving the design or
sliding into V1.5 / V2 work.

## Verdict

`accept_with_findings`. The synthesis cites `UI_REWORK.md` as the
canonical input rather than re-deriving it, the component / partial /
service / dashboard / token / test scope matches RFC 0050 V1, the
three named regressions are exactly the ones RFC 0050 V1 pins, and the
§9 acceptance hooks point at the right V1-applicable rows (§9.3, §9.4,
§9.9, §9.10). Two ordering / scope ambiguities should be tightened so
the build round does not drift into V1.5 territory.

## Canonical-input citation

The synthesis opens with "This synthesis treats `docs/design/UI_REWORK.md`
as the canonical design input. It does not re-derive the design
ceremony." That is the load-bearing claim for skipping the standard
3-designer ceremony per RFC 0050's "Skipped ceremony" section, and the
synthesis honors it: every Decisions bullet either cites a specific
UI_REWORK.md section (§5, §5.8, §7.3, §8.1, §8.5, §8.9) or names the
RFC 0050 V1 constraint that scopes the work. Pass.

## V1 scope coverage

Cross-checked each of the six RFC 0050 V1 deliverables against the
synthesis Decisions block:

| RFC 0050 V1 deliverable | Synthesis decision | Status |
| --- | --- | --- |
| Shared component library (RunStatePill / JobStatePill / VerdictChip / LaneAttestationChip / PostureChip / BylineLine / LaneEvidenceChip / ExpectedArtifactsTable) | "Shared component library" decision lists all 8 components at the §5 paths | matches |
| Jinja2 macro partial `_components.html` | "Jinja2 component partial" decision names the macros (run_state_pill, job_state_pill, verdict_chip, lane_attestation_chip, posture_chip, byline_line) | matches §5.10 |
| `service.py` payload shaping for `run_list` / `run_detail` / `job_detail` | "Service payload shaping" decision enumerates the three page shapers and the canonical chip payload fields | matches |
| `dashboard.py` text-mode parity | "Dashboard parity" decision names `RUN_STATE_ORDER`, `ATTESTATION_REASON_ORDER`, attestation chips, verdict-override suffixes, blocker kind lines, expected-artifact rows, process-envelope blocks, shared `next_actions` | matches §8.9 |
| CSS semantic tokens | "CSS semantic tokens" decision extends `base.css` with status / attestation / override / muted-evidence tokens, no literal colors in component CSS | matches §7.3 |
| Three V1 regression tests | `test_dashboard_web_parity` / `test_byline_regression` / `test_override_rationale_regression` | matches RFC 0050 V1 acceptance |

LaneEvidenceChip V1 form is correctly muted-only (`not_yet_correlated`)
per §6 truthfulness table and §10 OQ-2; the synthesis explicitly
defers the green `lane_evidence_present` state to GH #5 / V1.7.

## V1.5 / V2 drift check

RFC 0050 V1.5 lists three template extensions (`run_detail.html`
restructure, `job_detail.html` extend with override modal,
`artifact_view.html` extend with byline integrity surface) and V2
lists islands (`recovery-panel`, `override_verdict.js`,
`copy_on_click.js`, workflow-graph-editor attestation field, SSE live
region). Checked each against the synthesis:

- No `recovery-panel` island. Pass.
- No `override_verdict.js` modal. Pass — the synthesis never mentions
  the modal; `service.py` payload shaping is described as "shape work
  only; RFC 0050 says V1 does not add a new mutation path outside the
  existing service gate."
- No `copy_on_click.js`. Pass.
- No workflow-graph-editor changes. Pass.
- No SSE. Pass.
- No `run_detail.html` restructure (next-actions banner + recovery
  panel + sessions strip). Pass — the synthesis caveats "the larger
  V1.5 screen restructures stay out of this phase."
- No `byline integrity surface` on `artifact_view.html` (V1.5 work).
  Pass — the synthesis stops at the muted provenance-evidence chip,
  which is V1 per RFC 0050's "Render the GH #5 provenance-evidence
  chip as `not_yet_correlated` (muted)" goal.

See findings F1 and F2 below for the two grey-area scope calls that
do bleed slightly toward V1.5 and should be pinned now.

## Findings

### F1 — Implementation order contradicts its own rationale (severity: medium)

Step 3 in the Implementation order reads:

> 3. Add the semantic tokens and compact table styles in `base.css`,
>    **because the macros and TS components should reference tokens
>    from their first commit.**

But step 1 already lands "shared type definitions and component
renderers from `docs/design/UI_REWORK.md §5`, including
`LaneEvidenceChip` in muted-only V1 form" and step 2 already lands the
Jinja2 macros that consume `--status-*` / `--attestation-*` /
`--override-marker` tokens. If steps 1–2 ship before step 3, the
components reference tokens that do not yet exist in `base.css` —
which is the exact regression the rationale claims to prevent.

UI_REWORK.md §5 is explicit: "Apply CSS via semantic tokens (§7); no
literal hex codes in component CSS." Components without their tokens
in place either fall back to undefined-variable defaults (transparent
chips) or the implementer back-fills literal hex while waiting for
step 3 — both are V1 acceptance failures (§7.3 forbids literal hex in
component CSS; the snapshot tests under §9.1 will catch the wrong
chip color).

**Recommend:** reorder so step 1 becomes "Add CSS semantic tokens and
compact table styles in `base.css` (§7.3)" and step 3 becomes the
component-renderers step. Alternatively, pin tokens as part of step 1's
first commit and re-label the step accordingly — the rationale text
the synthesis already wrote describes that intent; only the ordinal
needs to flip.

### F2 — `_expected_artifacts_table.html` partial straddles V1 / V1.5 (severity: low)

RFC 0050 V1 lists the Jinja partial as `templates/_components.html`
(singular). RFC 0050 V1.5 lists "`job_detail.html` extend:
`ExpectedArtifactsTable` partial + process-evidence section + override
modal" — the "extend" phrasing covers both the partial and its
injection. The synthesis adds `_expected_artifacts_table.html` as a
V1 deliverable.

Reading the synthesis charitably, this is defensible because:

- §5.8 lists `ExpectedArtifactsTable` as a V1 component.
- §9.1 browser smoke on `job_detail.html` asserts
  `ExpectedArtifactsTable` rows — the component needs a render
  surface for that assertion to evaluate at all.
- The synthesis explicitly stops short of "the larger V1.5 screen
  restructures."

But a first-time implementer reading the synthesis cannot tell
whether the partial is wired into `job_detail.html` in V1 or staged
on a fixture-only render path until V1.5. The Decisions text says
the partial "renders the work packet's declared `expected_artifacts`
next to matching published artifact rows" without naming the host
template.

**Recommend:** add a sentence to the "Expected artifacts partial"
decision naming the V1 host template (the `job_detail.html` inject-
point from §8.4) or naming the test-fixture-only path. Either is
defensible; not picking forces the build round to choose by guess.

### F3 — "Process evidence stub" scope on `job_detail.html` is implicit (severity: low)

The synthesis says: "render a muted provenance-evidence value on
`artifact_view.html` and parsed blocker envelopes on `job_detail.html`
only to the extent needed for V1 tests."

Three things are ambiguous here:

- "Parsed blocker envelopes on `job_detail.html`" — UI_REWORK.md §8.5
  ("`ProcessExecutionEvidence` on `job_detail.html` and
  `artifact_view.html`") is the binding section, but RFC 0050 V1.5
  lists "process-evidence section" on `job_detail.html` as V1.5
  work. The synthesis pulls this slightly forward to V1 because §8.9
  requires the dashboard text-mode envelope block in V1 and the named
  V1 regression `test_dashboard_web_parity` needs the web side to
  render the same envelope.

- "Only to the extent needed for V1 tests" leaves the bar undefined —
  the named V1 regression set is `test_dashboard_web_parity`,
  `test_byline_regression`, `test_override_rationale_regression`. Of
  those, only the parity test exercises the envelope, and only for
  chip-vocabulary parity (UI_REWORK.md §9.9 enumerates the four
  strings: `unattested:no_attached_supervisor`,
  `process_outputs_missing`, `accept_with_findings`,
  `accept_with_findings (override)`). The full envelope block
  rendering (process_id / command / exit_code / duration / timeout /
  missing_artifact_paths / review_verdict_missing / recovery_commands
  from §5.9) is not required by the three V1 named tests.

- `artifact_view.html` muted chip rendering is V1 per the RFC 0050 V1
  goal ("Render the GH #5 provenance-evidence chip as
  `not_yet_correlated` (muted)"), but RFC 0050 V1.5 separately lists
  `artifact_view.html` byline integrity surface + provenance stub.
  The synthesis correctly takes only the minimal muted chip; that
  bound is fine but should be named so the build round does not
  expand it.

**Recommend:** pin the V1 bar explicitly — e.g. "On `job_detail.html`,
render the chip-vocabulary strings required by §9.9
(`process_outputs_missing` and the override suffix). The full §5.9
envelope block is V1.5." Same for `artifact_view.html`: "render the
muted `provenance evidence: not yet correlated` chip; the byline
integrity surface is V1.5." A first-time implementer reading "only to
the extent needed for V1 tests" without that pin will either
over-render (V1.5 drift) or under-render (parity test fails).

### F4 — Shared-enum exports are not enumerated (severity: low)

UI_REWORK.md §8.1 names the exact TypeScript exports for V1:
`RunState`, `JobState`, `VerdictProvenance`, `AttestationReason`,
`BlockerKind`, `ProcessAdapterEnvelope`. The synthesis says only
"shared enums in `src/striatum/web/frontend/src/shared/types.ts`."
A first-time implementer would need to open §8.1 to find the list,
which is fine for one missing list but the synthesis enumerates the
component file names, the macro names, the dashboard constants, the
CSS token families, and the regression test names — the enum list is
the one symbol set left unspelled.

**Recommend:** inline the six type names in the "Shared component
library" decision (one line), to match the level of specificity of
the rest of the document.

## What's working

- Canonical-input discipline. The synthesis never tries to redo the
  six operator flows, the screen specs, the closed enums, or the
  visual system — it points at sections in UI_REWORK.md instead. This
  is exactly what RFC 0050's "Skipped ceremony" section authorized.

- Component list is exhaustively spelled out. All eight V1 components
  from §5 are named with their file paths, not just the famous five
  chips. `ExpectedArtifactsTable` and `LaneEvidenceChip` (the two
  components a casual reader might assume are V1.5) are explicitly
  pulled into V1 with the correct constraints (RFC 0050 V1
  expected-artifacts payload; muted-only `not_yet_correlated` chip).

- Dashboard parity decision matches §8.9 line-for-line:
  `RUN_STATE_ORDER`, `ATTESTATION_REASON_ORDER`, attestation chips,
  verdict override suffixes, blocker kind lines, expected-artifact
  rows when width permits, process-envelope blocks, shared
  `next_actions`. The "consume the same `next_actions` source as the
  web view" line correctly identifies the §9.10 parity requirement.

- Three V1 regression names are exact and consistent with RFC 0050's
  V1 acceptance and UI_REWORK.md §9.3 / §9.4. Reusing the existing
  `tests/test_next_actions_v141_burndown.py` from v1.45.0 as the
  backend-hook prerequisite is correct — that test is the OQ-4
  source-of-truth pinned by CHANGELOG.md v1.45.0.

- Acceptance section maps §9 rows to RFC 0050 V1 obligations cleanly
  (§9.3 → byline truthfulness, §9.4 → override rationale, §9.9 →
  dashboard / web parity, §9.10 → shared next-actions). The §9 row
  pins make the build-round acceptance unambiguous.

- "V1.5 template restructuring and V2 islands remain follow-up
  dogfoods under RFC 0050" is the right closing sentence — it names
  the two follow-on phases and confirms the build round should not
  reach for them.

## Recommended build-round preconditions

Before the implement round begins, the synthesis (or a one-paragraph
addendum to it) should pin:

1. Implementation order F1: tokens land before / with the first
   commit that references them. Either reorder steps 1 and 3 or
   relabel step 1 to bundle the tokens.
2. `_expected_artifacts_table.html` host F2: name the V1 host template
   (or name the V1 test-fixture-only path).
3. Job-detail / artifact-view process-evidence bar F3: name the
   minimum V1 render bar (chip vocabulary only vs. full envelope vs.
   byline integrity surface).
4. Shared-enum exports F4: enumerate the six TypeScript type names
   inline.

None of F1–F4 block acceptance of the design. F1 is the only one that
risks a build-round regression if left implicit; F2–F4 are
ergonomics-of-handoff fixes that keep the build implementer from
guessing.
