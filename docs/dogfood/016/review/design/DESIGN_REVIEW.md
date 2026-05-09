# Design review: RFC 0018 V1 (steps 1+2)

author: reviewer-claude-opus-001
date: 2026-05-09
verdict: accept_with_findings

Adversarial review of `docs/dogfood/016/DESIGN_SYNTHESIS.md`
against RFC 0018 V1 acceptance criteria, the existing workflow
validator / packet builder shape from
`research/POSTURE_SHAPE.md`, and the lifecycle constraints in
`src/striatum/db.py`.

## Verdict

**accept_with_findings** — V1 is implementable as written. Two
findings carried forward to V1.5; one acceptance-blocking item
addressed below by recording the design decision explicitly.

## Sweep matrix

| Concern | Synthesis treatment | Verdict |
| --- | --- | --- |
| **V1 posture vocabulary right-sized** | Nine first-class postures from RFC 0018 verbatim plus `custom:<name>`. Coverage matches the RFC's stated cases (security, threat_model, devils_advocate, etc.). | OK |
| **`custom:<name>` grammar unambiguous** | `custom:<empty>` rejected; whitespace-only suffix rejected; case sensitivity preserved (no normalisation). `custom:foo:bar` accepted as the literal string `custom:foo:bar` — the inner colon is part of the name. | OK; the inner-colon behavior is worth a one-line test. |
| **Acceptance gate composes with today's "downstream reviews must accept"** | Synthesis re-casts the gate from "build-completion runtime gate" (RFC text) to "workflow-validation gate" (research § "Lifecycle ambiguity"). Today's edge-verdict gate is preserved untouched. | OK; see Finding 1. |
| **Build re-run / attempt handling** | Workflow-validation gate is static; per-attempt verdict gating remains the existing edge mechanism. New attempts get fresh verdicts naturally. | OK |
| **Test plan completeness** | 24 cases cover RFC 0018's V1 acceptance criteria minus step-3-only items. Includes both forward-edge and reverse-edge reachability cases, both first-class and custom: postures, and the zero-regression contract. | OK |
| **Workflow-validator interactions** | The new validator runs at workflow validation AND at every workflow load (so `run prepare` catches it before a run starts). Refusal raises `WorkflowError` (exit code 8) — same as existing review-field rejections. | OK |
| **Step-3-deferred safety** | Synthesis confirms no `verdicts.posture` column is read or written. Posture lives on the review *job* in V1, not the verdict row. The workflow-validation gate walks job declarations only. | OK |

## Findings

### Finding 1 — RFC text divergence (accept_with_findings)

The RFC's "step 2" describes a runtime build-completion gate that
deadlocks against striatum's lifecycle (a build's complete
mutation precedes the downstream review's verdict). The
synthesis re-casts the gate as a workflow-validation gate. This
delivers the operator value the RFC intended (catches mis-wired
workflows where `required_review_postures` cannot be satisfied)
without the deadlock.

**Recommendation:** the V1 acceptance decision (D069) records
this re-cast explicitly, and the RFC text is updated when V1
lands so future readers see the validation-time framing rather
than the runtime framing. Alternative patch text for the RFC's
"Proposal § Step 2 § Runtime acceptance rule" is suggested in
Finding 1A below.

#### Finding 1A — Proposed RFC patch text

Replace RFC 0018's "Runtime acceptance rule" with:

> **Workflow-validation acceptance rule:** the workflow validator
> walks the directed edge graph in both directions from each
> build job declaring `required_review_postures`. For each
> required posture P, it requires at least one *reachable*
> `type: "review"` job (forward or reverse from the build) whose
> `review_posture == P`. Failure raises `WorkflowError` (exit
> code 8) at `striatum workflow validate` and `run prepare`,
> naming the build, the missing posture, and the postures
> available across reachable reviews.
>
> Runtime enforcement is preserved by the existing edge-verdict
> gate (a downstream-of-review job stays blocked until the
> review accepts) and the existing run-completion semantics (a
> run cannot terminate while jobs remain non-terminal). No new
> runtime gate is added in V1; V1.5 may add a run-completion
> "blocked on missing posture" surfacing if dogfood evidence
> shows operators want explicit signaling.

This is suggested only; V1 implementation can proceed against
the synthesis as written and the RFC patch can land in the same
PR.

### Finding 2 — `custom:<name>` inner-colon test (accept_with_findings)

Test plan case 7 (`test_review_posture_accepts_custom_named_value`)
should explicitly cover `custom:security:strict` as the literal
posture name to lock the "no normalisation, no inner-colon
splitting" behavior. Currently implicit; one extra assertion in
that case is enough.

### Finding 3 — Validator placement of `_validate_required_review_postures` (note)

The synthesis calls this a "new helper" without specifying where
it slots in. Recommend it lives next to
`_validate_review_job_fields` and is called from `_validate_jobs`
on the same per-job iteration. Implementation choice; not a
contract issue.

## Risks reviewed and rejected

- **Custom-posture proliferation.** A workflow could declare a
  hundred different `custom:<name>` postures; the validator
  accepts each. Acceptable — the operator burden is on the
  workflow author. V1.5 may add a config knob; not blocking.
- **Forward-and-reverse reachability over-permissive.** A
  workflow with `implement → review_design` followed by
  `review_design → final_check` would let a `final_check`
  review satisfy `implement.required_review_postures` even
  though it's two hops away. Acceptable — the workflow author
  declared the chain; the gate is "is the posture reachable in
  the dependency graph," which is what the operator authored.
  Tighter "direct neighbor only" semantics could land in V1.5
  if dogfood evidence shows authors want it.
- **Posture sentence wording leakage.** The
  `POSTURE_INSTRUCTIONS` sentences are deterministic, lowercase
  English, and embedded in the work packet. Translation /
  localization is out of scope. Acceptable.

## Decision

Accept V1 with the three findings above. Findings 2 and 3 are
implementation refinements; Finding 1 is the design re-cast that
the V1 acceptance decision (D069) must record explicitly. The
implementer may proceed against the synthesis as written.
