# Issue 248 Push Delivery Design Task

Design an RFC-quality plan for GitHub issue #248:
<https://github.com/halbritt/striatum/issues/248>

The problem is the recurring `work.await_packet` / `no_work` polling loop.
The issue reports mined evidence from 2,531 sessions showing the pattern as the
highest-payoff inefficiency, with 65 distinct sessions and repeated 15-66+
wasted-turn loops. The proposed direction is push-based work delivery or
wake-on-work so a lane can exit idle and be woken when new work is available.

Hard boundaries:

- Striatum remains local-first: no hosted queue, telemetry, cloud scheduler, or
  external persistence.
- Daemon-owned PostgreSQL remains authoritative live state.
- Repository files are durable provenance, not the message bus.
- Do not weaken capability tokens, session binding, lane attestation, or
  write-scope enforcement.
- Reconcile with D175 and issue #212 before proposing daemon-side spawn or wake
  behavior.

Current design facts to preserve:

- `work.await_packet` / `claim-next` atomically claim eligible work and can
  return structured `no_work` or `fresh_session_required` responses.
- Non-agent-loop `supervised_push` lanes already support a narrower one-packet
  auto-dispatch path from `supervise start`.
- `run drive` is the accepted operator-side reconcile loop that reduces manual
  start/stop churn without making the daemon an autonomous scheduler.
- D175 explicitly deferred daemon-side `supervision.auto_spawn` until:
  `run drive` is routinely daemonized in practice, poll cadence is a measured
  bottleneck, and a non-human scheduler principal model exists. Issue #248
  provides evidence for the measured-polling part; it does not by itself settle
  the scheduler principal or product-boundary decisions.

Consensus target:

- Produce a decision artifact that chooses one bounded design path and states
  what is deliberately deferred.
- If the design is implementation-ready, include the exact build slices,
  authoritative surfaces, tests, docs, and rollback/safety checks the build
  workflow should execute next.
- If the design is not implementation-ready, say so directly and name the human
  decision still blocking implementation.
