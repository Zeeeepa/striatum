# RFC 0120: Await-Packet Idle Exit and Wake Boundary

Status: accepted (D180; Phase 1 implemented, Phase 2 implemented)
Date: 2026-06-12
author: operator-codex-gpt-5
Context: GH #248; RFC 0116 / D175 (`run drive`, daemon auto-spawn deferred);
RFC 0107 (principal boundary); RFC 0096 (supervised-lane trust boundary);
`go/pkg/mutations/claim.go`; `go/pkg/agentloop/bootstrap.go`;
`go/pkg/agentloop/loop.go`; `go/pkg/cli/rundrive`; `enqueueJob`
(`go/pkg/mutations/mutations.go`); RFC 0082/RFC 0086 agent-message delivery.

## Problem

GH #248 captures a real architectural inefficiency: lanes stay resident and
repeatedly call `work.await_packet` after `no_work`, because the current
bootstrap prompt tells them to do exactly that. The mined evidence shows wasted
model turns and fragile long-lived sessions (`helper_process_gone`,
`tmux_session_missing`, stale-session polling).

`work.await_packet` is already a daemon-side 30s long poll over
`work.claim_next`. The waste that GH #248 ranks highest is not only PostgreSQL
polling inside the daemon. It is also model-side polling after a terminal idle
answer.

## Prior Art

- Temporal matching service: resident workers long-poll task queues. Good fit
  when workers are meant to stay alive, not when model turns are expensive.
- Kubernetes watch: clients resume from authoritative state using a change
  stream; notifications are hints, durable state remains authoritative.
- PostgreSQL `LISTEN` / `NOTIFY`: useful local wake hint, not a durable queue.
- NATS JetStream and RabbitMQ consumers: queue systems pair delivery with
  acknowledgement, backpressure, and redelivery semantics.
- systemd socket activation: wake/spawn is a separate principal and lifecycle
  decision, not just a notification mechanism.
- GitHub Actions self-hosted runners: resident-runner polling is acceptable
  when the runner is cheap and deliberately resident.

## Three Lanes

### Lane A: True Daemon Auto-Spawn

The daemon subscribes to work availability and spawns lanes whenever packets are
ready.

Pros: closest to the literal "exit and get woken" request.

Cons: crosses D175's deferred boundary. The daemon becomes an autonomous actor
initiating work without contemporaneous operator authorization. It needs a
scheduler principal, audit attribution, run-as policy, restart safety, and a
fresh RFC tied to #212.

Verdict: defer.

### Lane B: Notify-Only Event Bus

Add a wake bus so waiters sleep on events rather than fixed intervals, while
`claim_next` remains authoritative and all delivery stays at-least-once.

Pros: aligns with Kubernetes/Postgres lessons. Compatible with existing state.

Cons: helps daemon query churn, but it does not stop the model-side `no_work`
loop unless the lane is also told to exit. It is the right follow-up after
Lane C, not a substitute for it.

Verdict: accept as Phase 2 of this RFC.

### Lane C: Idle-Exit Protocol with Operator-Side Wake

Keep D175's accepted boundary: `run drive` is the operator-authorized wake
surface. Change terminal idle semantics so `work.await_packet` returns an
explicit `idle_behavior=exit_session`, and change the agent-loop bootstrap and
daemon receiver to treat that as a normal clean exit rather than another poll.

Pros: smallest slice that directly eliminates the mined wasted model turns.
Keeps `claim_next` authoritative. Does not introduce a scheduler principal or
daemon auto-spawn. Composes with future notify-only wake and #212 auto-spawn
work.

Cons: it is not full push delivery. `run drive` still polls the run frontier
until a later event-driven driver wake lands.

Verdict: accept as the first build slice.

## Decision

Accept Lane C as Phase 1 and Lane B as Phase 2 of this RFC.

`work.await_packet` terminal idle envelopes include:

```json
{
  "type": "none",
  "status": "no_work",
  "idle_behavior": "exit_session"
}
```

Lane bootstrap instructions must say: on `no_work`, do not poll; end the lane
process cleanly. The PTY daemon receiver must also stop/terminate the lane when
it receives this explicit idle behavior, so hidden receiver goroutines do not
continue polling.

Phase 2 adds enqueue-side wake hints and a wake-aware wait path that reduces
daemon-side polling while preserving `claim_next` as the source of truth.
Notifications wake a driver or waiter so it can re-read authoritative daemon /
PostgreSQL state; notifications do not claim, lease, complete, verdict, or
otherwise mutate workflow state by themselves.

Implementation: Phase 2 uses an in-process daemon wake broker plus the
read-shaped `wake.wait` RPC. Mutation transactions collect wake hints and
publish them only after commit. `run drive` waits on `wake.wait` between
reconcile passes with its configured interval as the timeout; unsupported or
missed notifications degrade to the existing bounded polling behavior.

Defer only Lane A to #212 / a scheduler-principal RFC.

## Acceptance

### Phase 1: Idle Exit

- `awaitNoneEnvelope()` carries `idle_behavior=exit_session` and a stop-lane
  hint.
- `BuildBootstrapPrompt` no longer instructs agents to keep polling after
  `no_work`.
- The PTY daemon receiver exits/terminates the lane when `no_work` explicitly
  carries `idle_behavior=exit_session`.
- Existing `work.claim_next`, lease, ack, complete, review, and
  `supervised_push` semantics remain unchanged.
- No new hosted service, external queue, telemetry, or daemon auto-spawn path is
  introduced.

### Phase 2: Notify-Only Wake Bus

- Add a local wake abstraction with a PostgreSQL `LISTEN` / `NOTIFY`
  implementation or an in-process broker plus polling fallback. The abstraction
  is an optimization around existing durable state, not a new authoritative
  queue.
- Emit wake hints after durable work becomes observable, including ordinary work
  enqueue/requeue paths, interrogation agent-message availability, and
  conversation floor-turn availability.
- Make `run drive` able to block on wake hints between reconcile passes instead
  of sleeping for a fixed interval when no action is available. A wake only
  shortens the next reconcile; it does not decide which job to launch.
- Keep wake payloads small and non-sensitive: repository id, run id when known,
  event kind, and an optional durable identifier such as `message_id` or
  `conversation_id`. Raw prompts, artifacts, PTY output, and transcripts are not
  wake payloads.
- Treat notification loss as acceptable. Every woken path must relist or resume
  from daemon/PostgreSQL state and must retain a bounded polling fallback so a
  missed notification cannot wedge a run.
- Add tests proving that enqueue/requeue/conversation/interrogation transitions
  emit wake hints after commit, that `run drive` wakes without waiting for the
  full interval, and that dropping a notification still converges through the
  fallback path.
- Do not introduce daemon-side lane spawn, external queue infrastructure,
  hosted services, telemetry, or a new scheduler principal in Phase 2.

## Deferred

- Revisit daemon-side spawn only through #212 with the D175 evidence trigger:
  `run drive` daemonized in practice, poll cadence measured as bottleneck, and a
  non-human scheduler principal model in hand.

## Domain Modeling

This RFC is a boundary clarification plus a wake-hint design. `claim_next`
remains the authoritative work-delivery state transition. `idle_behavior` is an
advisory value on the `work.await_packet` envelope that tells the lane process
how to behave after the authoritative state transition says no work is
available. Phase 2 wake events are domain events over already-committed daemon
state; they are hints for when to reconcile, not commands to perform workflow
state transitions.
