---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0045", "python", "design"]
---

author: reviewer-unknown-model-002

# Track A (Python core) — Ergonomics/DX Review

## Verdict

`accept_with_findings`. The synthesis lands a coherent, narrow V1 surface that
keeps phase behavior expressible through ordinary jobs, dependency rows, and
verdict gates, and it correctly preserves v1 workflows by gating every new
behavior behind `striatum.workflow.v1.1`. From a first-time-operator lens the
schema/validator surface is mostly discoverable, but several affordances around
error messaging, command surface, and inferred behavior leave the operator
guessing. Findings are addressable in implementation and do not require a
re-spin.

## What Works (Ergonomics/DX)

- **Schema gate is unambiguous.** `ACCEPTED_WORKFLOW_SCHEMA_VERSIONS` plus the
  explicit v1-forbids / v1.1-optional / v1.1-non-empty matrix is reachable from
  a single read of the "Workflow Schema" section (lines 18–66). An operator can
  tell at a glance which schema string admits which fields. ✓
- **v1 invariants are explicit.** The "Backwards Compatibility Matrix" section
  (lines 333–344) enumerates the exact externally-visible surfaces — fixtures,
  `plan_workflow()`, `workflow_graph_data()`, `create_run()`, `status --json`,
  `dashboard --once`, review lifecycle — that must remain unchanged. A
  first-time operator running a v1 workflow has a written guarantee, not a
  hope. ✓
- **Phase status JSON shape is concrete.** The example payload (lines 184–204)
  plus the derivation rules (lines 206–214) gives consumers (dashboard,
  service, future TUIs) a complete contract without forcing them to read
  `phase_progress_for_run()`. ✓
- **Materialization defaults are operator-safe.** Making
  `edge_dependency_pairs(include_phase_materialized=True)` the default while
  validation paths opt out (lines 122–141) means existing callers — Mermaid,
  DOT, `create_run()`, graph data — see the executable graph without any
  caller edits. ✓

## Findings (Ergonomics/DX gaps)

### F1 — Validator error messages are unspecified

The work packet's acceptance criterion explicitly asks whether "validator error
messages are operator-actionable (point at exact field path, name the rule,
suggest a fix)". The "Validator Plan" section (lines 67–120) enumerates *what*
is rejected — "Reject duplicate phase ids", "Reject ordinary cross-phase edge",
"Reject phase skip", "Reject `phase_synthesis` without `phase_id`" — but
nowhere specifies what the operator sees. A first-time operator authoring a
v1.1 workflow with `phases[1].id == phases[0].id` cannot tell from the design
whether the validator will emit:

- `duplicate phase id 'phase_1_design' at phases[1].id (already used at phases[0].id); choose a unique id`, or
- a generic `ValueError("invalid phases")`.

The two outcomes differ by an order of magnitude in time-to-fix. The synthesis
should either (a) include a one-line message contract — field path, rule name,
suggested remediation — bound to each `_validate_phases()` rule, or (b)
explicitly defer the contract to a follow-up with a named owner. As written,
the rules section is a list of refusals, not a usability contract.

Recommend: add a "Validator Error Contract" subsection with one example per
rule. Three or four examples plus a stated convention ("field path, rule name,
remediation hint") is sufficient.

### F2 — `striatum workflow upgrade --add-phases` is a hidden mode on a dual-purpose verb

The "Upgrade CLI" section (lines 286–331) keeps `striatum workflow upgrade
<path>` as the existing harness-profile upgrader and folds phase rewriting in
as a flag on the same verb. From a first-time-operator perspective:

- `striatum workflow upgrade --help` will list two unrelated behaviors that
  share a single command name.
- An operator who reads `docs/TODO.md` or the RFC and learns "there is a way
  to add phases to a v1 workflow" will not naturally type `workflow upgrade`;
  they will try `workflow add-phases`, `workflow migrate`, or similar.
- The synthesis's stated rationale ("keeping that convention avoids a second
  write contract for the same verb") is a maintainer-side argument, not a
  user-side one. The user-side cost is real discoverability loss.

Recommend: either expose `striatum workflow add-phases` as a thin alias that
calls the same code path, or write a one-line `workflow upgrade --help` epilog
explicitly naming both modes. The synthesis is silent on either path.

### F3 — `phase_synthesis` verdicts ride the `submit-review` verb

The "Runtime Changes" section (lines 165–167) keeps `submit-review` as the
verdict-recording verb for both `review` and `phase_synthesis`, justified by
"V1 compatibility" and the packet-level `verdict` command shape. From an
ergonomics_dx perspective this is a discoverability mismatch:

- A first-time operator running `striatum --help` or `striatum submit-review
  --help` will see help text scoped to "reviews". Nothing in the verb name
  surfaces that phase-synthesis verdicts are accepted.
- The work-packet `commands.verdict` field papers over this for agents inside
  the runner, but humans reading dashboards, transcripts, or running
  `striatum verdict --help` from outside a session get no signal.

The synthesis acknowledges this softly ("update help/error text to 'verdict-
capable jobs' where possible") but does not commit. For ergonomics_dx, the
help-text update is not optional — the verb name is the only surface the
operator sees.

Recommend: bind the help-text rename to the same PR and add a
`striatum verdict` top-level alias if scope allows. At minimum, the synthesis
should mark the help-text change as required, not aspirational.

### F4 — Inferred phase boundaries from `parallel_group` prefixes are magical

The upgrade path inference (line 308) splits `parallel_group` on "the first
`_`, `-`, or `:`". This is heuristic in the unfriendly sense:

- Workflows that already use `parallel_group: design_a` / `build_a` accidentally
  pass; workflows that use `parallel_group: design-a` / `build-a` also pass;
  but `parallel_group: design_python_a` produces a phase named `design`, not
  `design_python`, by silent string splitting.
- The synthesis does not specify what happens when inference produces zero
  phases (every group is one token), one phase (every group shares the same
  prefix), or N phases the operator did not intend.
- The return envelope (lines 318–329) reports `phases_added`, but a first-time
  operator running `--dry-run` cannot easily verify the inference matched
  intent without manually cross-referencing `parallel_group` values.

Recommend: in `--dry-run` output, surface a `inference_source` per phase
(e.g. `"inferred_from": "parallel_group prefix 'design'"`) so the operator can
audit the boundaries without re-deriving them. And specify the
zero-phase / one-phase failure behavior (refuse with a pointer to
`--phase-map`?) in the synthesis itself, not the implementer's discretion.

### F5 — `phase_synthesis` author contract under-specified for `expected_artifacts`

The "Generator Catalog Shape" section (line 278) states that the generator
emits "a required synthesis artifact at `{artifact_root}/{phase_id}/
SYNTHESIS.md`", but the validator section (lines 99–108) does not state
whether author-written `phase_synthesis` jobs are required to declare
`expected_artifacts`. A first-time operator hand-authoring a v1.1 workflow
sees:

- `phase_synthesis` "must not declare `reviewer_access_scope`,
  `reviewer_context_policy`, `review_posture`, or `required_review_postures`"
  (lines 107–108), explicitly forbidding review-only fields.
- No statement about which fields *must* be present beyond `phase_id`.

Without an explicit author contract, hand-authored phase_synthesis jobs may
ship without an artifact requirement and silently complete on `verdict accept`
with no synthesis document, undermining the cross-phase gate's intent.

Recommend: state in `_validate_phases()` whether `phase_synthesis` requires at
least one `expected_artifacts[]` entry with `required: true`, or note
explicitly that artifact discipline is delegated to the publisher / verdict
gate elsewhere.

### F6 — RFC ↔ synthesis field-name divergence is not flagged for operators

The synthesis chooses `phase_id` over the RFC's `phase`, drops the RFC's
`phases[].title` in favor of `name`, and drops `phases[].synthesis_job_id` in
favor of deriving from the unique `phase_synthesis` job (lines 60–64). The
rationale is sound. But a first-time operator reading RFC 0045 first
(documented at `docs/rfcs/0045-...`) and then the workflow examples will hit
three field-name mismatches without any hint that the RFC's spec is no longer
authoritative for V1.

Recommend: the synthesis should explicitly call out in one sentence that RFC
0045 needs an addendum updating its field shapes to match V1, and that the
upgrade CLI handles legacy authors who guessed RFC field names (or, if it
does not, that hand-typing `phase:` instead of `phase_id:` produces the
"unknown field" validator path).

## Citations

All section/line citations refer to `docs/dogfood/043/DESIGN_SYNTHESIS_python.md`
at the head of `striatum/dogfood-043-rfc-0045`. RFC field divergence cross-
references `docs/rfcs/0045-multi-phase-workflow-editor-and-schema.md` lines
77–100.

## Summary

The Python-core synthesis chooses a defensible, minimum-surface V1: phases as
validation + materialization, no schema/runtime migration, verdict gates that
ride existing rails. v1 workflows are protected. The first-time-operator
experience is good on schema + status shape and weak on three surfaces:
validator error messages (F1), upgrade CLI discoverability (F2, F3), and
inferred behavior in the upgrade path (F4). F5 and F6 are smaller gaps in the
author contract. None are blockers; all should be resolved before the
implementation order in lines 388–397 closes out steps 7–8.
