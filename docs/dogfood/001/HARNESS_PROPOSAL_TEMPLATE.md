---
schema_version: "striatum.harness_improvement_proposal.v1"
artifact_kind: "harness_improvement_proposal"
target: "workflow"
expected_benefit: "Replace this with the concrete benefit you expect."
risk: "Replace with the risk of making the change. Optional."
rollback: "Replace with a rollback or deferral path. Optional."
---

# HARNESS-NNN — short title

Status: proposed
Run: dogfood-001
Reporter: {role}-{model}-{ordinal}
Surface: {claim-next | supervise | dashboard | publish-artifact | ...}

## Observed friction

What happened? Be specific. Include the exact command, the actual output,
and what you expected instead.

## Supporting runner evidence

- run_id:
- job_id:
- packet_id (if relevant):
- supervisor_id (if relevant):
- relevant event types from `striatum why <id>`:

## Proposed change

What should change in the runner, the workflow contract, the prompts, the
defaults, or the docs to make this surface less surprising next time?
Concrete enough that someone could write an RFC from it.

## Risk

What breaks if this change ships? Who has to migrate?

## Rollback

If we ship this and it turns out wrong, what's the unwind path?

## Notes

Anything else worth capturing before the context fades.
