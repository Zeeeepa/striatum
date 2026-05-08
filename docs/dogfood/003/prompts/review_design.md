# Review Tool Harness Profile Design

Review `docs/dogfood/003/DESIGN_SYNTHESIS.md`, RFC 0010, and the research
artifacts. Use fresh context. Do not rely on prior conversation memory.

Assess:

- whether the design preserves Striatum's generic product boundary;
- whether native subagents remain internal to parent sessions by default;
- whether validation and work-packet exposure are small enough for the first
  build slice and do not accidentally require provider wrappers;
- whether the design accounts for RFC 0010's concrete profile examples and
  extended fields without overfitting core scheduling to any one provider;
- whether the design is backed by current official tool docs and the refreshed
  dogfood research;
- whether the fixture profiles can test generic, Codex, Claude Code, and
  Gemini CLI behavior while keeping defaults portable;
- what tests and docs must change before acceptance.

Write `docs/dogfood/003/review/DESIGN_REVIEW.md` as a `finding` artifact.
Use front matter when practical:

```text
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["dogfood-003", "rfc-0010"]
---
```

Submit the review with `submit-review`. Use `accept` or
`accept_with_findings` only if a human could reasonably record an acceptance
decision and allow implementation to proceed.
