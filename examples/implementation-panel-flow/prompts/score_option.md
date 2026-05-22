# Score An Implementation Option

Review only the problem brief and the proposal assigned to your job. Write the
assigned scorecard artifact with `striatum.finding.v1` front matter:

```yaml
---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: medium
tags: ["implementation-panel", "scorecard"]
---
```

Score the proposal from 1 to 5 for:

- correctness
- simplicity
- migration_risk
- testability
- operator_ergonomics
- cost
- performance
- reversibility
- security_privacy
- maintainability

For each score, add a short evidence note. Use `needs_revision` only when the
proposal cannot be compared without a concrete correction.
