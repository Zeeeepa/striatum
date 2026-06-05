---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs:
  - "PROBLEM_BRIEF.md"
  - "TRADEOFF_LEDGER.md"
  - "proposals/option_a/PROPOSAL_A.md"
  - "proposals/option_b/PROPOSAL_B.md"
  - "proposals/option_c/PROPOSAL_C.md"
  - "scorecards/SCORECARD_A.md"
  - "scorecards/SCORECARD_B.md"
  - "scorecards/SCORECARD_C.md"
---

# RFC 0112 Arbitrator Synthesis
author: arbitrator-codex-gpt-5.5-xhigh-001
date: 2026-06-05
status: implementation-plan-selected

## Decision

Select Proposal B's lifecycle-first plan, amended by Proposal A's
attempt-aware packet projection and Proposal C's fixture-rigor requirements.
This is the only plan that fixes both halves of the failure:

- early close: the pending-consumer predicate sees direct dependents plus
  explicit consumers declared in the frozen workflow snapshot;
- late close or leak: every terminal consumer transition reaches one
  lifecycle choke point that invokes the generalized release hook.

No new daemon RPC family, no fake dependency edges, no durable
`interrogation_consumers` table, no transcript capture, no ACE support-tier
flip, and no schema migration are part of this slice. The implementation must
land in a separately scoped follow-up because it touches `go/pkg/*` and
`docs/reference/*`, outside this panel's write scope.

## Explicit Answers

1. **Field name and shape.** Use `interrogation_targets` on the consumer job:
   `{ "workflow_job_id": "<interrogable upstream workflow job>", "required": true|false }`.
   Unknown entry fields are V1 lint warnings, not hard errors. Hard validation:
   target exists, target is not the declaring job, target is `interrogable:
   true`, the declaring job is reachable downstream from the target through
   ordinary workflow edges, duplicates are rejected, and declaring
   `interrogation_targets` on the same target job is rejected. Lint warnings:
   redundant direct dependency, unknown entry fields, and `required: true`
   where the lane lacks `interrogate`.

2. **`required: true`.** Advisory in V1, not a completion or verdict gate.
   It strengthens packet instructions and produces curated evidence at
   lifecycle moments. Packet projection writes nothing. An actual
   `interrogation.open` refusal appends `interrogation.unavailable_signaled`;
   terminalizing a consumer that skipped a required target appends
   `interrogation.required_skipped`. A future hard gate can be a predicate over
   interrogation rows plus these events.

3. **Multiple targets.** Allow `N >= 1` in V1. The machinery is already
   set-based: predicate, hook, packet state, and evidence events are per
   `(consumer, target)` entry. Mixed states are represented entry-by-entry.
   Add a lint warning above three targets so authors see the complexity without
   blocking real future shapes.

4. **Terminal release hook paths.** Introduce
   `markJobTerminal(...)` in `go/pkg/mutations` and route terminal job writes
   through it. It updates the job, appends the caller's event, and calls
   `releaseInterrogationTargetsForTerminalConsumer(...)`.

   Covered paths: `work.complete`; `work.block` to `waiting_human`;
   `review.verdict` / `submit-review` accept, accept-with-findings,
   absorbed needs-revision, reject, and human-checkpoint paths;
   `override-verdict`; checkpoint / escalation resolve completing or canceling
   a waiting review; recovery auto-publish / validated-output completion /
   auto-finalize; `recovery.cancel_job` including cascades. `run.cancel` and
   run-failure finalization may be allowlisted only with the structural
   rationale that `closeRemainingSessions` closes all remaining windows.
   Add an AST/static guard so future terminal-state job updates either use the
   helper or carry an explicit reviewed allowlist entry. State and test the
   RFC 0104 per-run lock assumption so concurrent consumer terminalization
   cannot double-miss closure.

5. **RFC 0105 fixture for ACE graduation.** Add
   `go/pkg/adapterconformance/ace_interrogation_test.go`, seeded from the
   production ACE generator output, not a hand-built graph. Required cells:
   happy path, revision reopen with fresh target session, dead lane during
   revision re-cascade, and compatibility with no `interrogation_targets`.
   The happy and revision cells must seed a convener-only fact and assert the
   interrogation answer uses preserved context, matching RFC 0082's intention
   test. The fault cell must prove same-attempt requeue, join remains blocked,
   target window stays open, and the run completes or escalates loudly within
   budget. Add a `waiting_human` block/resolve assertion, direct-dependent
   suites unchanged, `TestNoTargetsPacketUnchanged`, no active
   `awaiting_interrogation` session at cell end, and validation negative tests.

6. **Work-packet namespace.** Use `context.interrogation_targets[]`. Fields:
   `workflow_job_id`, `required`, `state`, `target_session_id`,
   `target_attempt`, `reason`, and daemon-authored `instruction`.
   `available` means the target's current attempt has a live awaiting session.
   `unavailable` means an awaiting session existed for the relevant attempt but
   is retired; include the retired session id for evidence linkage and use the
   session close reason, such as `interrogation_window_closed`. `not_ready`
   means there is no awaiting session for the target's current attempt; after
   reopen this includes `reason: revision_reopened` rather than falsely
   exposing a stale retired session. Add `attempt` to future
   `session.awaiting_interrogation` event payloads; legacy events default to
   attempt 1.

## Implementation Sequence

1. Add `ValidateInterrogationTargets` and lint rules in workflow authoring.
2. Update the ACE generator so every cross-examiner declares
   `convener_draft` as a required interrogation target; keep the graph
   unchanged.
3. Add one snapshot-derived resolver helper for declared consumers and target
   projection. Reuse it from the pending predicate, release hook, and packet
   builder; do not materialize the relation.
4. Generalize the release hook and introduce the terminal-state choke point
   plus AST guard. Correct Proposal B by not relying on the recovery sweep as
   the all-consumers-terminal leak backstop; the guard and fixture invariant are
   the real backstop.
5. Add V1 evidence events for unavailable and skipped required targets.
6. Add attempt-aware `context.interrogation_targets[]` projection in
   `buildPacket`.
7. Land the adapterconformance and workflowauthoring tests before any tier
   graduation discussion.

## Test Sequence

Run focused validation and mutation tests first: target missing, self,
non-interrogable, unreachable, duplicate, absent arrays, malformed entries,
direct-plus-explicit dedupe, multiple targets with mixed states, packet
projection states, required-skip events, unavailable events, and the
terminal-release guard.

Then run the RFC 0105 ACE cells through production handlers with the fake
agent: happy path, revision reopen, dead-lane re-cascade, waiting-human
block/resolve, compatibility/no-targets. Existing direct-dependent
interrogation tests must remain unchanged.

Finally run `make test`, `make typecheck`, `make lint`, and the PG-gated
`make check`/`make smoke` path expected for this change set.

## Docs Sequence

Before acceptance, revise RFC 0112 to incorporate the decisions above:
multi-target allowed, advisory `required` with V1 evidence events, terminal
choke point and exact paths, attempt-aware packet projection, and the fixture
hardening requirements. After implementation, update `docs/reference/spec.md`
for the workflow field and packet contract, and
`docs/reference/ubiquitous-language.md` for "interrogation consumer." Do not
update the decision log or RFC status in this panel.

## Risks And Rollback

Main risks are a missed terminalizer, stale-session projection after revision,
shallow fixture evidence, and snapshot-query maintainability. The selected plan
mitigates them with the choke point, static guard, attempt-aware projection, and
preserved-context assertions. Query cost should be profiled but is expected to
be small for Striatum run sizes.

Rollback is straightforward because there is no schema table: remove the ACE
generator field, remove validation/projection/predicate extensions, and the
system reverts to direct-dependent behavior. In-flight ACE runs prepared before
this field remains absent in their frozen snapshots; they must be re-prepared
after implementation rather than silently rewritten.
