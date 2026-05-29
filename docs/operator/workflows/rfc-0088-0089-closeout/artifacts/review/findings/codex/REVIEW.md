---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat-model", "tmux-liveness", "delivery-liveness", "supervise-rebridge", "false-health"]
---

# RFC 0089 Phase 1 Findings Review
author: reviewer-codex-gpt-5.5-xhigh-001
date: 2026-05-29
verdict: needs_revision
posture: threat_model
interrogation_id: intg_395637caadb787b99bd3a454d6cc9be4
interrogation_round_count: 3
stop_reason: confirmed_false_health_gap

## Summary

The implementation makes the right domain split between pane liveness,
delivery liveness, lane backend, and attestation, and the send path refuses
known-degraded delivery before writing a packet. The remaining blocker is that
helper attach-client-exit events are not consistently ingested before operator
read projections. That can make `status`, `dashboard`, and `supervise.status`
report `delivery_state=healthy` while the helper-owned delivery bridge has
already exited and the tmux pane is still `tmux_ok`.

Because Phase 1 explicitly exists to close false-health reporting before Phase
2 deletes the older wrapper path, this needs revision before the phase lands.

## Trust Boundaries And Attack Surfaces

- The authoritative lane state remains daemon PostgreSQL metadata, not tmux
  pane text or PTY output. The implementation keeps probe data to identity
  fields (`has-session`, `display-message`, `list-panes`) and typed failure
  records; I found no path that stores raw pane text in daemon state or
  artifacts.
- Tmux pane liveness and delivery liveness are separate trust boundaries. A
  live pane does not prove that the helper-owned attach bridge can deliver a
  packet.
- `supervise.rebridge` is a privileged repair mutation. It must not clear a
  delivery-degraded state unless the replacement bridge is actually usable or a
  later failure is immediately visible to operator reads.
- PID/start-token identity is the attestation boundary. The probe rejects
  pane id, pane pid, and numeric start-token mismatches; missing start-token
  evidence downgrades attestation rather than granting a model byline.

## Blocking Finding

### F1: Helper attach-exit can remain invisible to status/dashboard until a later mutation or sweep

`drainHelperEvents` is currently called from `supervise.send` and
`supervise.stop`, not from ordinary read projections. `HandleSuperviseStatus`
explicitly stays read-only and does not drain helper files before rendering the
DTO. The builder confirmed that the remaining ingestion paths are send/stop
mutations and the resident recovery sweep, whose default cadence is 60 seconds.

Impact: if the helper-owned attach client exits after normal start or after
`supervise.rebridge` returns, `ProbeLaneLiveness` can continue to report the
pane as `tmux_ok` while metadata still lacks `delivery_liveness.degraded`.
Until the next send/stop mutation or sweep drains the helper event, operator
read surfaces can show delivery as healthy. That is the false-health class the
Phase 1 findings were meant to eliminate.

This is not a silent packet-drop path: `supervise.send` re-probes liveness and
will either honor stored degradation or hit the missing FIFO reader path and
refuse delivery. The blocker is the operator/read surface: a degraded delivery
bridge can be displayed as healthy when the operator is deciding whether to
rebridge versus reclaim.

Minimum acceptable fix:

- Add daemon-owned helper-control-event ingestion before, or as part of, the
  operator liveness read projection, without importing pane text or PTY bytes.
- If a read method mutates control-event offsets or pointer metadata, update
  the authority model/docs/tests accordingly; otherwise add an explicit refresh
  path and make status/dashboard depend on it.
- Add a regression proving an `attach_client_exited` event emitted after
  start/rebridge is visible in `supervise.status` and dashboard before any
  subsequent `supervise.send`.

## Secondary Finding

### F2: `supervise.rebridge` can erase same-batch delivery degradation

When `launchRebridgeHelper` returns an initial event batch containing both
`agent_started` and immediate `attach_client_exited`, `HandleSuperviseRebridge`
records the events and then deletes `delivery_liveness` while merging launch
metadata. The builder confirmed this can clear the degradation that was just
recorded.

This should be fixed with F1 by preserving `launch.Metadata.tmux.delivery_liveness`
when the initial helper batch contains `attach_client_exited`, plus a focused
regression. This matters because immediate attach-client exit is not theoretical
in the current tmux-backed lanes.

## Non-Blocking Follow-Up

`supervise.rebridge` is registered through runtime metadata and a handwritten
route, while `contracts/daemon_methods.json`, generated route metadata, and
generated daemon method tables are not updated. The handoff calls this out as
out of the builder's write scope. Before broader release, reconcile the
generated contract so the method is not a runtime-only exception to the
single-contract source.

## Verification

Commands run:

```bash
cd go && go test ./pkg/supervisor ./pkg/mutations ./pkg/reads
cd go && go test ./pkg/cli/... ./pkg/mcp ./pkg/rpc
```

Both passed.

## Verdict

needs_revision
