---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
---

# Review GH #12-#13 Ergonomics

author: reviewer-unknown-model-003

## Verdict

accept_with_findings

## Findings

### F1 - Low - #12 needs an explicit negative affordance for disallowed copy targets

The #12 mitigation correctly narrows copy-on-click from global
`[data-copy]` handling to allowlisted containers such as `.recipe-list`,
`.code-recipe`, and `.copyable-token`. From a first-time-operator
perspective, the ergonomics contract is still underspecified: the docs say to
refuse `data-copy` outside those containers, but do not state whether refusal
is silent, logged, surfaced in dev tooling, or visible in the UI.

Silent refusal is secure enough, but it makes future UI work harder to debug:
an operator or contributor can add a visible copy affordance outside an
allowlisted container and get no obvious reason that clicking does nothing.
The polish pass should define one consistent behavior for disallowed
`data-copy` elements, preferably no clipboard write plus a developer-visible
warning or testable inert state, while keeping the operator-facing UI free of
surprise toasts for markup bugs.

### F2 - Low - #13 is discoverable after purge, but should define the inspector label transition

The #13 mitigation is directionally correct: when a job changes from `review`
to `generic`, `require_attested_lane` should be purged from internal state and
the serialized workflow. That closes the ghost-field mismatch where the node
body still shows a value the inspector hides.

For UI consistency, the planned fix should also pin the operator-facing
transition: after changing the type, the node label and inspector should agree
immediately, without requiring save/reload or another selection change. The
requested Vitest coverage should assert the serialized output, and it would be
stronger if it also covered the visible node-label state after the type change.

## Notes

The listed specs and roadmap are sufficient to proceed with the ergonomics
polish. No blocking issue is visible from the document-only review surface.
