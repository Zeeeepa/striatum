# RFC 0082: Interrogation Sessions

Status: accepted
Date: 2026-05-25
Accepted: 2026-05-25 (D138; implemented in the rfc-0082-interrogation run. All 9 Required Tests pass under live PostgreSQL, including the end-to-end intention test: a reviewer interrogates a builder's preserved context and receives a context-aware answer.)
author: proposer-claude-opus-4-7-001
Context:
[`RFC 0002`](0002-reviewer-independence-policy.md),
[`RFC 0009`](0009-long-lived-process-supervision.md),
[`RFC 0068`](0068-go-production-daemon-port.md) (MCP agent-loop),
[`RFC 0077`](0077-mcp-activity-liveness-deadlines.md),
[`RFC 0081`](0081-conversation-trajectories.md),
[`docs/DECISION_LOG.md`](../decisions/decision-log.md) (D028 no-transcript-capture, D026 lane attestation),
the 2026-05-25 event-bus robustness assessment.

## Problem

The original intent for the message/event bus was: a worker *subscribes* to
receive work, *preserves its context window*, and is available for *iterative
interrogation by other workers* on design and build review. The current
implementation does not realize the third pillar, and undercuts the second:

- **Review is artifact-mediated, not live interrogation.** A reviewer job
  claims after the builder's job completes, reads the *published artifact* on a
  cold context, and records a verdict. It never queries the builder's live
  reasoning.
- **The message bus cannot address a worker.** `work.send_message` is
  run-scoped broadcast with no `target_session_id`, and `work.await_packet` /
  `work.claim_next` only deliver rows with `kind='work'`
  (`go/pkg/mutations/claim.go`). A message aimed at a peer is never delivered to
  that peer's receive loop.
- **Questions go up, not sideways.** `session.report report_kind=question`
  routes to the operator/inbox, not to a peer worker.
- **Context is reset between jobs.** `fresh_session_required: true` forces a new
  session (cold context) per job, so the design→build→review arc deliberately
  discards the very context an interrogation would query.

There is no construct for "worker B asks worker A a question, and A answers from
its preserved context." This RFC adds one.

## Goals

- A first-class **interrogation session**: a bounded, multi-turn, peer-addressed
  Q&A bound to a *live target worker session* whose context is preserved.
- Targeted, session-addressed delivery: a question reaches the target worker's
  receive loop (not just the operator, not a broadcast).
- Context preservation for the interrogation window: a worker can remain
  available to answer from its preserved context after finishing its work.
- Make review *interrogative*: a reviewer can iteratively question the builder's
  live context in addition to reading its artifact.
- Strict provenance within D028: interrogation turns are curated, structured
  records (question/answer text the agents deliberately authored), never raw
  provider stdout/stderr. Surfaced through the RFC 0081 `dialogue` trajectory.
- **Tests that validate the intention**, not just the plumbing: an end-to-end
  test in which a reviewer interrogates a builder's *preserved context* and
  receives a context-aware answer is the acceptance bar.

## Non-Goals

- Replacing artifact-based review or verdicts; interrogation augments them.
- Capturing model transcripts or token streams (D028 stands).
- Cross-run or cross-repository interrogation (single run scope for V1).
- A new runtime language or substrate; daemon-owned PostgreSQL remains the only
  live state and the daemon remains the sole writer.

## Proposal

### 1. Domain concept

An **interrogation** is a bounded, ordered, multi-turn exchange between an
**interrogator session** (e.g. a reviewer) and a **target session** (e.g. the
builder) within one run. It has lifecycle `open → (ask/answer)* → closed`. Turns
are addressed to the target session and delivered to its receive loop; answers
come from the target's preserved context. The target must be *live* (an attached,
attested session per D026) for the duration; an interrogation against a closed
session fails `target_unavailable`.

### 2. Data model (ownership-safe, daemon-owned)

A new daemon-owned table records lifecycle + correlation; turns reuse the
existing message bus with targeting. Per RFC 0079 §5 the migration is applied by
the owner and `GRANT`s the runtime role; it adds no `ALTER` to existing tables.

```
striatumd.interrogations(
  repository_id text, run_id text, interrogation_id text,
  interrogator_session_id text, target_session_id text,
  topic text, state text CHECK (state IN ('open','closed')),
  opened_at timestamptz, closed_at timestamptz,
  PRIMARY KEY (repository_id, interrogation_id))
```

Turns are `striatumd.queue_messages` rows with `kind IN
('interrogation_question','interrogation_answer')`, `target_session_id` set,
and `interrogation_id` carried in `payload_json` for correlation/threading.

### 3. Daemon methods

- `interrogation.open(target_session_id, topic)` → `interrogation_id`
  (interrogator-capability gated; fails if target session is not live).
- `interrogation.ask(interrogation_id, body)` → enqueues a question turn
  addressed to the target session.
- `interrogation.answer(interrogation_id, body)` → target session replies
  (only the target session may answer).
- `interrogation.close(interrogation_id)` → terminal.
- `interrogation.list(run_id)` / `interrogation.show(interrogation_id)` → reads.

All mutations are daemon-written and append `events` rows
(`interrogation.opened|asked|answered|closed`).

### 4. Delivery: unify the worker receive loop

Extend `work.await_packet` to return a typed envelope so a worker's single
subscribe loop receives *either* work *or* a pending interrogation question
addressed to its session:

```
{ "type": "work_packet",            "packet": {...} }
{ "type": "interrogation_question", "interrogation_id": "...", "body": "...", "lease_id": "..." }
{ "type": "none" }
```

This preserves backward compatibility (existing callers that expect a work
packet check `type`), and realizes "subscribe to the bus to receive work" as
"await the next actionable item." Delivery prefers a pending interrogation
question for the session over new work so interrogations are answered promptly.
(Push via PG `LISTEN/NOTIFY` or the existing MCP SSE channel is an allowed
optimization but not required for V1; the bounded long-poll is acceptable.)

### 5. Context preservation window

Add a session phase `awaiting_interrogation`. After `work.complete`, a worker
whose job declares `interrogable: true` does NOT close; it re-enters its
`await_packet` loop and remains available to answer interrogations from its
preserved context until `interrogation.close` (by the interrogator) or a bounded
idle timeout, after which `session.close` runs. This is the mechanism that keeps
the context window alive for review. Requires the MCP agent-loop executor
(`go/pkg/agentloop`); the fresh-per-packet supervised wrapper cannot preserve
context and is not interrogable.

### 6. Review integration

A review job (or a new lightweight `interrogation` job role) gains
`reviewer_access_scope: live_session`: it opens an interrogation against the
builder's live target session, asks design/build questions, and folds the
answers into its verdict alongside the published artifact. `fresh_session_required`
is relaxed for the builder when `interrogable: true` so its context survives into
the review.

### 7. CLI / MCP surface

- `striatum interrogation open --run-id --target-session <id> --topic "..."`
- `striatum interrogation ask --interrogation-id <id> --body "..."`
- `striatum interrogation answer --interrogation-id <id> --body "..."`
- `striatum interrogation list --run-id <id>` / `show --interrogation-id <id>`
- Same as MCP tools (`interrogation.*`), exposed to lane agents via the bundle.

### 8. Provenance (D028)

Interrogation turns are curated records — question/answer text the agents
deliberately authored, session/role/lane identifiers, timestamps, and the
`interrogation_id` — never provider stdout/stderr. They surface in the RFC 0081
`dialogue` trajectory profile and `provenance` stream; a guard test asserts no
raw-output field is ever projected.

## Required Tests (acceptance-defining)

Implementation is incomplete without all of these, using the RFC 0080
`go/pkg/pgtest` harness for live-PG coverage:

1. **Lifecycle unit tests** — `open → ask → answer → close` state machine;
   double-close, answer-after-close, and ask-on-unknown-id are rejected.
2. **Targeting tests** — an `interrogation.ask` is delivered to the target
   session's `await_packet` and to **no other** session; a non-target session's
   `await_packet` never returns it.
3. **Authorization tests** — only an interrogator-capable session may
   `open`/`ask`; only the target session may `answer`; capability matrix guard.
4. **Live-target requirement** — `interrogation.open` against a closed/unattached
   session fails `target_unavailable`.
5. **await_packet envelope tests** — discriminator correctly returns
   `work_packet` vs `interrogation_question` vs `none`; a pending question is
   preferred over new work.
6. **Multi-turn live-PG integration** — seed a run with a live target session,
   open an interrogation, run ≥2 ask/answer rounds, assert each question is
   received by the target and each answer is recorded and correlated by
   `interrogation_id` in order.
7. **End-to-end intention test (the bar)** — a builder session completes a job
   with `interrogable: true`, enters `awaiting_interrogation`; a reviewer opens
   an interrogation, asks a question, the builder answers **from its preserved
   context** (assert the answer references prior in-context work, e.g. a
   builder-only fact seeded into the build packet and never re-published), the
   interrogation closes, and the builder session then closes. `trajectory export
   --profile dialogue` reproduces the Q&A thread.
8. **D028 guard** — no interrogation projection or record contains provider
   stdout/stderr; only curated authored fields.
9. **Migration ownership safety** — the `interrogations` migration creates a new
   table + grants the runtime role and does not `ALTER` owner tables (regression
   guard for the RFC 0081 incident).

## Acceptance Criteria

- A reviewer can iteratively interrogate a builder's **preserved context** and
  receive context-aware answers, proven by test (7).
- `interrogation.{open,ask,answer,close,list,show}` exist as daemon methods, CLI
  verbs, and MCP tools, capability-gated, daemon-written, with `events` rows.
- `work.await_packet` returns the typed envelope and delivers session-addressed
  questions; existing work delivery is unchanged for non-interrogable runs.
- Interrogation turns are curated (D028) and appear in the RFC 0081 `dialogue`
  trajectory.
- All Required Tests pass under `go test ./...` and `-race`, with live-PG tests
  running in CI (RFC 0080); smoke green.
- Daemon-owned PostgreSQL remains the only live substrate; the migration is
  owner-safe (test 9).

## Implementation Plan

1. Migration: `interrogations` table (owner-applied, runtime grant) +
   `target_session_id`/`interrogation_id` handling on `queue_messages`.
2. Daemon mutations + reads: `interrogation.*`; register in
   `contracts/daemon_methods.json` and regenerate via `routergen`.
3. Extend `HandleAwaitPacket` to the typed envelope + question-preference; add
   the `awaiting_interrogation` session phase + `interrogable` job flag +
   `fresh_session_required` relaxation.
4. CLI verbs + MCP tool exposure + skill-bundle docs.
5. Trajectory (`dialogue`) surfacing of interrogation turns.
6. All Required Tests, incl. the e2e intention test on the agent-loop executor.
7. Docs: `docs/WORKFLOW_TYPES.md` review-by-interrogation, daemon runbook.

## Risks

- Keeping a builder context alive widens the resource/lease window; bound it
  with an idle timeout + recovery sweep (RFC 0020/0077).
- The agent-loop executor must be the path for interrogable runs; the
  fresh-per-packet wrapper cannot satisfy the intent (assessment finding).
- Targeting + delivery changes to `await_packet` must not regress plain work
  delivery — covered by tests (5) and existing claim tests.

## Open Questions

- Should interrogation answers be allowed to attach references to artifacts
  (richer than text), and if so how curated?
- Should `interrogation.open` support a non-live target by replaying its
  trajectory (degraded, artifact-only) rather than failing? (V1: fail clean.)
- Push delivery (LISTEN/NOTIFY vs MCP SSE) now or as a follow-up?

## Domain Modeling

Introduces *interrogation* as a bounded conversation aggregate bound to a live
session, addressed via the existing message bus with targeting, and observable
through the RFC 0081 trajectory read-model. It does not add a second authority
over run state; the daemon remains the single writer. It completes the original
bus intention — subscribe, preserve context, interrogate — that the 2026-05-25
assessment found unrealized.
