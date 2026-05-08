# Review Wrapper Design

Review `docs/dogfood/004/DESIGN_SYNTHESIS.md`, the prior pipe-behavior
research, RFC 0009, and the supervisor-side code under
`src/striatum/cli/supervise.py`. Use fresh context. Do not rely on
prior conversation memory.

Assess:

- whether the chosen `claude` invocation form is justified by the
  research, not picked from memory;
- whether the wrapper shape (bash `exec` vs Python coproc loop) is
  the smallest one that satisfies RFC 0009's supervised-lane
  contract;
- whether stdout/stderr stay off per D028 / RFC 0009;
- whether the verification test would actually fail if the wrapper
  used a wrong invocation form (or, if the test is empirical, that
  the failure mode is well-defined);
- whether the design respects the `.striatum/` boundary — the only
  permitted `.striatum/` write is `.striatum/bin/` for the wrapper
  itself;
- whether transcript capture, hosted services, and remote A2A
  surfaces stay out of scope;
- whether the deferrals list is honest about what V2 does not do;
- what tests, docs, and RFC updates the build slice must touch
  before acceptance.

Write `docs/dogfood/004/review/design/DESIGN_REVIEW.md` as a
`finding` artifact:

```text
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["dogfood-004", "harness-001-v2"]
---
```

Submit a structured verdict via `submit-review` (or `verdict` after
`publish-artifact`). Use `accept` or `accept_with_findings` only if
a human could reasonably record an acceptance decision and let
implementation proceed.
