# Build review: RFC 0018 V1 (steps 1+2)

author: reviewer-claude-opus-002
date: 2026-05-09
verdict: accept

Fresh-context review of the V1 build against the V1_ACCEPTANCE
re-cast (workflow-validation gate, not runtime build-completion
gate). Walked the source diff (`workflow.py`, `db.py`,
`tests/test_review_postures.py`) and the doc patches.

## Verdict

**accept** — V1 acceptance gate satisfied; no blocking findings.

## Sweep matrix

| Acceptance gate | How V1 satisfies it | Verified |
| --- | --- | --- |
| **Validator: posture on non-review job rejected** | `_validate_review_posture` raises `WorkflowError` with `non-review job <id> cannot declare review_posture` for any non-review type. | `tests/test_review_postures.py::test_review_posture_rejected_on_non_review_job` |
| **Validator: unknown posture rejected** | Closed-set check against `ALLOWED_POSTURES`; `custom:` prefix required for off-list. | `test_review_posture_rejects_unknown_value` |
| **Validator: empty/bare-prefix invalid values rejected** | Empty string + bare `custom:` + whitespace-only custom name all raise. | `test_review_posture_rejects_empty_string`, `test_review_posture_rejects_bare_custom_prefix`, `test_review_posture_rejects_whitespace_only_custom_name` |
| **Validator: required-postures on non-build job rejected** | `_validate_required_review_postures` raises with `non-build job <id> cannot declare required_review_postures`. | `test_required_review_postures_rejected_on_non_review_job` |
| **Validator: empty/non-list/unknown entries rejected** | Three separate cases raise with deterministic messages. | `test_required_review_postures_rejects_empty_list`, `test_required_review_postures_rejects_non_list`, `test_required_review_postures_rejects_unknown_entry` |
| **Packet exposure (declared)** | `_build_review_policy` includes `posture` key when declared; `instruction` includes the deterministic posture sentence for first-class postures. | `test_packet_exposes_posture_when_declared` (security; "security-focused review" sentence asserted), `test_packet_instruction_appends_posture_sentence_for_first_class` (threat_model) |
| **Packet exposure (undeclared)** | When posture is not declared, the `posture` key is absent and no posture sentence appears in `instruction`. | `test_packet_omits_posture_when_undeclared` |
| **Packet exposure (neutral)** | Declared `neutral` exposes the key but appends no sentence (the table maps `neutral` → empty string). | `test_packet_instruction_unchanged_for_neutral_posture` |
| **Packet exposure (custom:)** | Custom postures expose the literal string and get no auto-sentence. | `test_packet_instruction_unchanged_for_custom_posture` |
| **Reachability gate (forward edge)** | `_validate_required_postures_reachable` walks forward edges from the build; matching review posture passes. | `test_workflow_validates_when_required_posture_is_reachable_via_forward_edge` |
| **Reachability gate (reverse edge)** | Walks reverse edges so design-review-before-build patterns also satisfy. | `test_workflow_validates_when_required_posture_is_reachable_via_reverse_edge` |
| **Reachability gate (unreachable)** | Refuses with `build job <id> requires review posture <P> but no reachable review job declares it`. | `test_workflow_rejects_unreachable_required_posture` |
| **Reachability gate (disconnected)** | Right-posture review exists but no edge connects → still refused. | `test_workflow_rejects_when_review_with_required_posture_exists_but_disconnected` |
| **Custom postures end-to-end** | A `custom:my_thing` posture validates, exposes on the packet, and satisfies the reachability gate. | `test_required_review_postures_accepts_custom_entries`, `test_review_posture_accepts_custom_named_value` (includes `custom:security:strict` lock per Finding 2) |
| **Zero regression for posture-omitting workflow** | A workflow that declares neither field validates exactly as pre-V1 and produces a packet with no `review_policy` block. | `test_zero_regression_for_posture_omitting_workflow_validator`, `test_zero_regression_for_posture_omitting_workflow_packet` |
| **Build re-run / attempt handling** | Workflow-validation gate is static; per-attempt verdict gating inherits the existing edge-verdict mechanism (no new code on the run-time path). The "stale verdict from attempt N satisfies attempt N+1" risk does not apply because the V1 gate is at validation time, not at verdict-record time. | Source review of `_validate_required_postures_reachable` and the unchanged `record_review_verdict` path. |
| **Suite health** | `tests/test_review_postures.py`: 24/24 pass. `make lint` clean. `make typecheck` clean (59 source files, no issues). Full suite: 318 baseline + 24 posture = 341 passed (one transient `test_doc_links` failure on the first full run did not reproduce in isolation; will re-confirm on the second full pass before tagging). | Direct run output. |

## Quality observations (non-blocking)

1. The implementer wired the new validator helpers next to
   `_validate_reviewer_policy` per Finding 3 — placement matches
   the design recommendation.
2. The `custom:security:strict` test case is present in
   `test_review_posture_accepts_custom_named_value` per
   Finding 2.
3. The RFC text patch (Finding 1A) landed in
   `docs/rfcs/0018-focused-adversarial-review-postures.md` —
   "Step 2" prose now describes the workflow-validation gate
   explicitly with an "Implementation note" explaining the
   lifecycle re-cast.
4. The `_validate_required_postures_reachable` walker treats an
   undeclared `review_posture` on a reachable review job as
   `"neutral"` for the gate. This matches the synthesis ("Each
   review's posture is its `review_posture` (or `"neutral"` when
   omitted)") and is documented in the docstring.
5. The forward-and-reverse reachability semantics are
   over-permissive in the abstract (a multi-hop chain through
   intermediate non-review jobs still counts) but correctly
   over-permissive: the workflow author declared the chain, so
   the runner respects it. Tighter "direct neighbor only"
   semantics could land in V1.5 if dogfood evidence shows
   authors want it.

## Risks reviewed and rejected

- **Lifecycle re-cast loses operator value.** No — the
  validation-time gate catches the same mis-wired-workflow
  condition the RFC's runtime gate intended to catch, just
  earlier (at workflow validate / run prepare instead of at
  build complete). The error message is more useful at this
  altitude because it can list the available postures across
  the graph.
- **Reverse-edge reachability lets an upstream-of-build
  review satisfy a downstream-of-build required posture.**
  This is intentional. dogfood-016's own workflow uses a
  design-review-before-implement pattern (review reachable via
  the reverse edge); the V1 design synthesis explicitly
  enumerates this case.
- **Posture instruction sentence wording leaks into review
  prompts that don't want it.** Workflow authors who don't want
  the auto-appended sentence omit `review_posture` entirely (or
  declare `custom:<name>` to suppress the auto-sentence). V1
  preserves the existing escape hatch.

## Decision

Accept V1. Land the change, bump to 1.7.0, transition RFC 0018
to `accepted (V1; step 3 deferred)`. Step 3 (`verdicts.posture`
column + introspection surfaces) waits for V1.5 per the RFC's
own implementation path.
