# Review V1 Design

Use fresh context. Do not rely on prior conversation memory.

Read:

- `docs/dogfood/005/DESIGN_SYNTHESIS.md` (target);
- `docs/dogfood/005/research/CURRENT_ADAPTER.md`;
- `docs/rfcs/0014-process-adapter-completion-guarantees.md`.

Assess:

- whether the diagnostic envelope contains zero child stdout/stderr
  (D028 hard constraint);
- whether the envelope is recorded in `payload_json` of an existing
  table (no new SQLite tables);
- whether the blocker-reason vocabulary covers all four V1 failure
  modes plus the post-reconcile case, with no overlap or ambiguity;
- whether `--timeout-seconds` and `recovery process-reconcile`
  follow existing CLI patterns and don't break callers who omit
  them;
- whether the test plan includes the externally-killed
  reconciliation path and the issue #1 reproduction;
- whether schema impact (`process_executions.state` new values) is
  honestly accounted for — needs migration or doesn't;
- whether deferrals are explicit and don't smuggle scope through
  the back door.

Write `docs/dogfood/005/review/design/DESIGN_REVIEW.md` as a
`finding` artifact with front matter:

```text
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["dogfood-005", "rfc-0014"]
---
```

Submit via `verdict` (or `submit-review`). Use `accept` /
`accept_with_findings` only if a human could reasonably approve and
let implementation proceed.
