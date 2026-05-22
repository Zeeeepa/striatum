# Review The Arbitration

Review the arbitrator synthesis as a dissent reviewer. Write
`examples/implementation-panel-flow/artifacts/DISSENT_REVIEW.md` with
`striatum.finding.v1` front matter:

```yaml
---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept
severity: medium
tags: ["implementation-panel", "dissent"]
---
```

Try to falsify the arbitration:

- Identify hidden costs or maintenance traps.
- Check whether a rejected option is stronger under the declared criteria.
- Check whether the synthesis ignores scorecard evidence.
- Call out any missing rollback, testing, or migration evidence.

Use `needs_revision` only when the arbitration has a material flaw that must be
resolved before a decision is recorded.
