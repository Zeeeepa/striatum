# RFC 0106: Workflow-shape support tiers and graduation — govern the catalog, don't prune it

Status: accepted (D162, 2026-06-03)
Date: 2026-06-02
author: proposer-claude-opus-4-8-001
Context: RFC 0034 (workflow generator + template catalog), RFC 0074 (shape/adversary-pack expansion), RFC 0083/0086/0093/0094/0098 (collaboration shapes), RFC 0105 (reliability harness); `go/pkg/workflowgenerate/generate.go`, `go/pkg/workflowtemplates`, `go/pkg/workflowauthoring` (lint), `docs/reference/workflow-catalog.md`.

## Problem

The workflow-shape catalog — ~14 generator shapes in
`go/pkg/workflowgenerate/generate.go` (incl. `falsification_gate`,
`cross_examination`, `adjudicated_constraint_extraction` with its ~16 `ace_*`
role templates, `implementation_panel`, `multi_phase`, N-party `conversation`) —
is **the product's anti-hallucination / anti-falsifiability mechanism**, and is
therefore core value, not over-generality. The architecture review's "prune to
three shapes" recommendation was wrong under the real mission.

But there is a real defect in the *process*: **new shapes keep landing while
existing ones still wedge** under the multi-lane revision lifecycle. The catalog
front-runs its own foundation, so the operator faces a menu of choreographies of
which an unknown subset actually survive an unattended run. Under yolo, choosing
an unreliable shape silently is a trap.

The fix is **governance, not subtraction**: tell the truth about which shapes are
proven to run unattended, and stop widening until the catalog catches up.

## Proposal

1. **Support tiers.** Add a `support_tier: supported | experimental` field to each
   shape in the catalog (`workflowtemplates` registry + generated
   `docs/reference/workflow-catalog.md`). `supported` = has a green RFC 0105
   per-shape reliability fixture. `experimental` = exists, may be valuable, not yet
   proven unattended.
2. **Honest lint, not a block.** `workflow.lint` / `workflow.validate`
   (`workflowauthoring`) emits a **warning** (not an error) when a run uses an
   `experimental` shape: "this shape has no unattended-reliability gate; expect to
   supervise it." Yolo can opt in knowingly; the default run path surfaces the risk.
3. **Graduation gate.** A shape moves `experimental → supported` **only** when it
   has a passing RFC 0105 fixture across the fault matrix. A guard test fails if a
   shape is marked `supported` without a registered green fixture — so the tier
   cannot lie. D169 adds one narrow amendment: a provably-isomorphic generated
   shape may share another shape's fixture when a separate graph-isomorphism guard
   fails on structural drift while ignoring role/artifact/prose naming.
4. **Freeze policy (decision-log, not code).** Record a decision: **no new shapes
   are authored until the existing catalog has graduated** (or been explicitly
   marked permanently-experimental). This is a discipline commitment, reversible by
   a later decision, that redirects velocity from breadth to depth.

No shape is removed. Every choreography stays available; the catalog simply stops
pretending unproven shapes are production-ready.

## Acceptance

- `workflow.lint` warns on `experimental`-shape runs and is silent on `supported`
  ones; tested in `workflowauthoring`.
- A guard test rejects `support_tier: supported` without a green RFC 0105 fixture.
- Any shared-fixture co-graduation has an explicit graph-isomorphism guard and
  names the source fixture it depends on.
- `docs/reference/workflow-catalog.md` renders the tier per shape.
- An initial honest classification lands: the shapes proven in dogfoods
  (minimal, review+synthesis, code-change) start `supported`; the
  collaboration/interrogation choreographies start `experimental` until their
  RFC 0105 fixtures are green.

## Non-goals

- **Not removing or hiding any shape** — explicitly the opposite of the review's
  "prune" call.
- Not blocking experimental shapes — yolo may opt in; the lint informs, it does
  not gate.
- Not designing new shapes — this RFC freezes new-shape authoring, it does not add.

## D169 Amendment: Isomorphic Co-Graduation

`cross_examination` is a narrow co-graduation case. With equal challenger-chain
length, its generated graph is structurally the same as `falsification_gate`:
source artifact → linear challenger chain → optional scribe → adjudicator
collaboration-ledger gate → commit → final, with a `needs_revision` cycle back
to the first challenger. The differences are role, job, artifact, and prompt
names.

Duplicating `falsification_gate_test.go` under renamed jobs would add no new
unattended-reliability coverage. Instead, `cross_examination` may use the
`falsification_gate` reliability fixture while
`go/pkg/workflowgenerate/generate_test.go::TestCrossExaminationIsStructurallyIsomorphicToFalsificationGate`
continues to prove the generated graph is isomorphic. If the guard fails, either
restore equivalence, demote `cross_examination`, or add a genuinely distinct RFC
0105 fixture for the new structure.

## Relationship to prior RFCs

- **RFC 0034/0074** built and expanded the catalog; this RFC adds a maturity model
  over it.
- **RFC 0105** supplies the reliability fixture that defines `supported`; this RFC
  is its consumer and the reason the per-shape fixture exists.
- **RFC 0093/0094/0098** contributed the collaboration shapes that carry the
  anti-hallucination value; this RFC protects that value by making each shape earn
  a `supported` badge rather than deleting them.
