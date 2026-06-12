# RFC 0120: Await-Packet Idle Exit and Wake Boundary

Status: accepted (D180)
Date: 2026-06-12
author: operator-codex-gpt-5
Context: GH #248; RFC 0116 / D175 (`run drive`, daemon auto-spawn deferred);
RFC 0107 (principal boundary); RFC 0096 (supervised-lane trust boundary);
`go/pkg/mutations/claim.go`; `go/pkg/agentloop/bootstrap.go`;
`go/pkg/agentloop/loop.go`.

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
loop unless the lane is also told to exit. It is not enough by itself.

Verdict: useful follow-up after idle semantics are corrected.

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

Accept Lane C now.

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

Defer Lane B as the next compatible increment: enqueue-side wake hints and a
wake-aware wait path that reduces daemon-side polling while preserving
`claim_next` as the source of truth. Defer Lane A to #212 / a scheduler-principal
RFC.

## Acceptance

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

## Follow-Up

- Add a notify-only wake bus around enqueue/requeue/conversation/interrogation
  messages, likely with PostgreSQL `LISTEN`/`NOTIFY` or an in-process wake
  broker plus polling fallback.
- Revisit daemon-side spawn only through #212 with the D175 evidence trigger:
  `run drive` daemonized in practice, poll cadence measured as bottleneck, and a
  non-human scheduler principal model in hand.

## Domain Modeling

This RFC is a boundary clarification. `claim_next` remains the authoritative
work-delivery state transition. `idle_behavior` is an advisory value on the
`work.await_packet` envelope that tells the lane process how to behave after the
authoritative state transition says no work is available.
