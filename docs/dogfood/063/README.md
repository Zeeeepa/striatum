# Dogfood 063 — RFC 0053 Phase B schema rename (workflow.v1.2)

**Closes:** [RFC 0053 Phase B deferred items](../../rfcs/0053-human-principal-and-terminology-truing.md),
[TODO #44 schema rename + state rename + prompt sweep + escalation
artifact-kind](../TODO.md), [ROADMAP §5.8 RFC 0053 Phase B](../ROADMAP.md).

**Why:** RFC 0053 (accepted as D103) names the AI operator as the
default driver and the human principal as escalation-only. The
runtime vocabulary still uses `human_checkpoint` (blocker severity +
workflow field) and `waiting_human` (run state) — both pre-RFC-0053
naming. The semantic shift is done; the schema rename was deferred
to a workflow-schema bump.

**Scope:**

- Bump workflow schema: `striatum.workflow.v1` → `striatum.workflow.v1.2`.
- Rename `human_checkpoint` blocker-severity / workflow field →
  `escalation_checkpoint`. Both names accepted by the validator with
  a soft-deprecation warning on the old name; the workflow generator
  emits the new name.
- Rename `waiting_human` run state → `waiting_escalation`. Both
  names accepted on read; `runs.state` writes use the new name; a
  migration row maps `waiting_human` → `waiting_escalation` in
  existing PG state.
- `workflow upgrade <path>` rule that rewrites v1 workflows to v1.2
  with the rename applied.
- Escalation artifact-kind scope was deferred from this rename slice and
  landed in the follow-up Phase 5 work (`striatum.escalation.v1`,
  daemon escalation projection routes, and `striatum inbox`). Remaining
  escalation decisions are artifact-only creation policy and stricter blocker
  payload/table shape.
- CLI prompt-string sweep: every verb whose stderr / prompt text
  says "human confirmation required" or similar says "operator
  confirmation" or "escalation required" instead.

**Shape:** standard 8-job dogfood. Schema-bump-class — codex
`threat_model` design review is load-bearing (migration safety +
back-compat).

**Branch:** `striatum/dogfood-063-rfc-0053-phase-b-schema-rename`.

**Post-landing:** v1.x.0 bump (decide minor or major — this is a
breaking workflow-schema change for consumers that hard-code the
field name; minor with deprecation warning is the proposed path).
ROADMAP §5.8 RFC 0053 Phase B → ✅ shipped. RFC 0053 Status updated.
TODO #44 closed.
