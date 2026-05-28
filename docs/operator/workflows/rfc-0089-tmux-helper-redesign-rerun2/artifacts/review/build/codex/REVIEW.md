---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: needs_revision
---

# Build review codex threat model

author: reviewer-codex-gpt-5.5-xhigh-001

Verdict: needs_revision

## Finding: helper bridge exit can create false healthy delivery state

The implementation separates tmux pane liveness from the transient
`tmux attach-session` client, which addresses the core attach-as-liveness bug.
However, the builder confirmed a remaining Phase 1 trust-boundary gap: if the
helper-owned PTY bridge used for FIFO-to-pane delivery exits, the pane may still
probe as `tmux_ok` and status/dashboard may continue to report the lane as
attached or attested, while future prompt delivery through `supervise.send` can
block or fail.

That makes pane liveness look authoritative for the whole lane, even though it
only proves the tmux pane exists. The missing boundary is between
`pane_liveness=tmux_ok` and `delivery_liveness=healthy`. A live autonomous MCP
agent may continue to complete work, but any path still relying on the helper
delivery handle can become a packet-delivery blackhole.

RFC 0089 Phase 1 says attach-client exit must leave the lane attached/attested,
and also says live RFC 0088 agent-loop lanes should keep completing work through
MCP while monitored. The current behavior satisfies the operator-observer half
of that requirement, but it risks false health when the daemon delivery handle
is gone.

Recommended fix for this phase: either keep or rebuild the helper forwarding
bridge when the helper-owned attach client exits, or mark helper-owned bridge
exit as delivery-degraded/detached/lost until rebridge or send-keys delivery is
implemented. For the current scope, explicitly surfacing delivery-degraded is
the smaller safe change because it prevents false health without expanding into
the deferred delivery redesign.

## Checked Trust Boundaries

- Raw tmux pane text and PTY logs remain private diagnostics only; the builder
  reported liveness uses `has-session` and tmux format fields, not pane capture.
- Attach-client lifetime is no longer the lane identity for ordinary operator
  attach/detach.
- Pane pid reuse is mitigated by comparing the stored pane pid start token when
  available.
- Missing session, missing pane, dead pane, pid mismatch, unavailable tmux, and
  corrupt tmux metadata are intended to become structured liveness classes.
- Read projections need an explicit delivery-health distinction so pane
  liveness cannot overstate workflow-control health.

## Interrogation Evidence

Curated interrogation: `intg_5c90667022e4c7e258f3bd2116a41059`.

The builder identified changed code paths and tests, then confirmed the bridge
continuity caveat should be treated as blocking unless Phase 1 is narrowed or
the status model distinguishes pane liveness from delivery liveness.
