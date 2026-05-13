---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0046", "v1", "build"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0046 V1 (claude, ergonomics_dx)

Operator-composed (recurring claude-no-publish anti-pattern; this
dogfood self-validated the new guard — operator publish-on-behalf
required --allow-no-process-execution + rationale, exactly as
RFC 0046 V1 intended).

## Summary

Build shipped F-schema (v15 migration adds attestation_override_rationale
column), F-guard (publish_artifact refuses model-byline publish without
matching process_executions row), F-override (--allow-no-process-execution
+ --override-rationale CLI flags), F-event (provenance event emitted),
F-test (6/6 unit tests pass).

## Findings

- F-1 (low): V1 ships the weaker "any clean exit-0 process_executions
  row" guarantee rather than path-specific check, because the current
  schema doesn't capture observed output paths. V1.7 follow-up tracked
  in RFC 0046 §Open question 2 + the V1.7 backlog.
- F-2 (low): Web UI `LaneEvidenceChip` and dashboard `evid:` column
  per CLAUDE_DESIGN_UI_REWORK_PROMPT.md are deferred to the design pass.

## Verdict

`accept_with_findings` (low). The dogfood itself validated the guard;
the override rationale recorded on the impl HANDOFF demonstrates the
audit trail working as designed.
