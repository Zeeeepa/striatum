# Implementation (interrogable)

Task: {{TASK}}

Build the smallest-scope item from
`docs/operator/workflows/rfc-0094-deferred-collaboration-shapes/artifacts/DESIGN_SYNTHESIS.md`,
incorporating any required revisions from the design panel. Default target is
RFC 0094 slice 1 (`post_dialog_hook` + `synaptic_prune`); pull in slice 2
(Check-B + `collaboration_ledger` v1.1 + anti-theater corpus) only if it stays
reviewable. Defer slices 3-4 to a future run and record the deferral.

Scope:

- Land the change within the job's allowed write scope.
- Keep the change reviewable. Resist scope creep; defer follow-on items to
  a future run.
- Keep the `collaboration_ledger` schema change **additive** (RFC 0093 V1
  ledgers must still validate). Add no new daemon method or route — the
  command-authority matrix + guardrail tests must stay green.
- Update or add tests for behavior you change.

Hand off:

- Write `docs/operator/workflows/rfc-0094-deferred-collaboration-shapes/artifacts/build/HANDOFF.md`
  summarizing what landed, what was deferred, and the exact verification
  commands the reviewers should run.

This node is **interrogable**. After you publish the handoff, your session
stays live so the build-review panel can interrogate your preserved
reasoning. Keep the decisions behind the diff in your working memory; you
will answer the panel from your own context.

If a build reviewer returns `needs_revision`, you will re-enter this job
(up to two iterations per reviewer) to land the enumerated revisions.

When the handoff is complete, emit the `submit-handoff` packet from the
runner's work packet.
