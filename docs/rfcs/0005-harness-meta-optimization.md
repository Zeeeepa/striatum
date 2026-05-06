# RFC 0005: Harness Meta-Optimization

Status: proposed
Date: 2026-05-06
Context: ARIS paper review, `agent-runner/docs/SPEC.md`,
`agent-runner/docs/DECISION_LOG.md`

## Problem

`agent_runner` will accumulate structured evidence about its own workflow
friction: repeated blockers, manual command plumbing, failed gates, unclear
work packets, adapter limitations, and review cycles that plateau. Today those
observations become manual validation notes or ad hoc follow-up specs.

The runner should be able to propose improvements to its own prompts,
workflows, defaults, and docs from that event trail. It must not auto-apply
those changes, because the harness is the safety boundary.

## Goals

- Use existing runner events, artifacts, blockers, and validation notes to
  propose targeted harness improvements.
- Keep proposals review-gated and human-approved.
- Preserve local-first behavior and transcript-off defaults.
- Avoid optimizing toward reviewer preference without evidence of workflow
  benefit.

## Non-Goals

- Do not train models or fine-tune anything.
- Do not send event logs to external services.
- Do not auto-edit workflows or prompts.
- Do not use private corpus content in meta-optimization artifacts.

## Proposal

Add an optional maintenance workflow pattern:

```text
evidence export + validation notes -> meta analysis -> patch proposal ->
independent review -> human disposition
```

The meta-analysis job may inspect:

- runner event summaries;
- blockers and human checkpoints;
- verdict distributions;
- repeated status/why/doctor issues;
- validation notes;
- workflow files and prompt artifacts.

It emits a `harness_improvement_proposal` artifact containing:

- observed friction;
- supporting runner evidence;
- proposed prompt/workflow/spec changes;
- expected benefit;
- risk;
- rollback or deferral path.

The proposal is advisory. A reviewer must approve it before implementation, and
the human retains final disposition.

## Acceptance Criteria

- Workflow validation accepts `harness_improvement_proposal` as an artifact
  kind.
- A sample maintenance workflow can analyze committed evidence exports without
  reading `.agent_runner/` directly.
- Proposed harness changes require a review job before any implementation job
  becomes claimable.
- Documentation states that meta-optimization never auto-applies patches.

## Open Questions

- Should meta-analysis be periodic, manually invoked, or attached only to
  blocked validation runs?
- What threshold of repeated friction should trigger a proposal rather than a
  validation note?
