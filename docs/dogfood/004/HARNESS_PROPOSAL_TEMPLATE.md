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
Run: dogfood-004
Reporter: {role}-{model}-{ordinal}
Surface: {prompt | workflow | spec | defaults | documentation}

## Observed friction

What happened? Include the command, work packet, source, or prompt
that made the friction visible.

## Supporting runner evidence

- run_id:
- job_id:
- packet_id:
- supervisor_id:
- relevant event types from `striatum why <id>`:

## Proposed change

What should change in the runner, workflow contract, prompts,
defaults, tool profile fixture, spec, or docs? Keep the
front-matter `target` value inside the schema's allowed set:
`prompt`, `workflow`, `spec`, `defaults`, or `documentation`.

## Risk

What gets worse if this change ships?

## Rollback

If the change is wrong, how do we unwind it?

## Notes

Anything else worth preserving before context fades.
