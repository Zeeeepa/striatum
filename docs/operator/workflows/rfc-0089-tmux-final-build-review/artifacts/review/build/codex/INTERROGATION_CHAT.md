# Build interrogation chat codex
author: reviewer-codex-gpt-5.5-xhigh-001
date: 2026-05-28
interrogation_id: intg_51838f106c8baf41787656cbb1921248
target_session: sess_ac823cc767750f0857cd4a9fa2ced765
rounds_asked: 2
rounds_answered: 2
stop_reason: closed after sufficient threat-model coverage

## Topic

RFC 0089 final build threat-model review: delivery liveness,
byline/attestation safety, tmux pane authority, pid reuse, stale pane
spoofing, and stop/recovery behavior.

## Round 1

**Question**

The final handoff says the rerun2 delivery-health gap is fixed by persisting
`delivery_liveness` and making `supervise.send` refuse degraded delivery.
Please point me to the exact code paths/tests that distinguish tmux pane
liveness and lane_attestation from delivery health after helper-owned attach
bridge exit or missing FIFO reader. Also state what a status/dashboard reader
will see in that degraded case, and why it cannot still be mistaken for a
healthy lane.

**Answer**

The builder identified three implementation paths:

- `go/pkg/supervisor/helper.go` emits `attach_client_exited` only when a fresh
  probe sees `tmux_ok` or `tmux_unavailable`, and includes degraded delivery
  metadata with `reason: attach_client_exited`.
- `go/pkg/mutations/supervision.go` re-probes in the daemon. When the daemon
  still sees a live tmux pane, it keeps the supervisor attached, refreshes
  heartbeat, records `supervisor.attach_client_exited`, and merges
  `tmux.attach_client_last_exit` plus `tmux.delivery_liveness`. Terminal tmux
  classes still fall through to detached handling.
- `go/pkg/mutations/supervision_control.go` checks degraded delivery metadata
  before delivery. It refuses degraded `tmux.delivery_liveness` or top-level
  `delivery_liveness`. Missing FIFO readers are persisted as
  `reason: stdin_reader_missing`, then refused without recording
  packet-delivered success.

The builder also pointed to `go/pkg/reads/supervision.go`, where read
projections keep `delivery_liveness` separate from tmux pane liveness and
lane attestation. A degraded bridge can therefore show the pane alive and
attested while also exposing top-level degraded delivery health.

Named tests included the attach-exit report tests, degraded-send refusal tests,
missing-reader persistence test, read projection tests, and D028 sanitizer /
tmux command guard tests.

## Round 2

**Question**

Please walk through stale PID / stale pane spoofing defenses. In particular:
how are literal or missing `pane_start_time` values handled, when does
`start_token_unverified` downgrade byline attestation, and what happens on
non-Linux where `ProcessStartToken` is unavailable? Also identify the
stop/recovery path that prevents signalling a recycled attach-client or pane
PID.

**Answer**

The builder described a fail-soft liveness, fail-closed attestation model:

- `verifiedStartToken` accepts only unsigned decimal strings. Literal values
  such as `#{pane_start_time}`, empty strings, and nonnumeric values are
  treated as unavailable.
- `CaptureTmuxIdentity` and `ProbeTmuxLiveness` first use tmux
  `pane_start_time`, then fall back to `ProcessStartToken` when possible.
  Linux uses `/proc/<pid>/stat`; non-Linux returns unavailable.
- If expected and observed numeric tokens both exist and differ, the class is
  `tmux_pane_pid_mismatch`.
- If the pane id and pid match but the start token cannot be verified,
  liveness remains `tmux_ok` with `detail: start_token_unverified`.
- Mutation-side attestation and read-side status/dashboard attestation both
  convert that detail into `lane_attestation: unattested` with reason
  `start_token_unverified`.
- On non-Linux, a numeric tmux `pane_start_time` can still verify attestation;
  without a numeric tmux token, the lane can remain operationally live but is
  not byline-attested.

For stop/recovery, the builder identified `HandleSuperviseStop` and
`stopTmuxBackedLane`: tmux-backed lanes are stopped through
`tmux kill-session` first, `attach_client_pid` is not signalled, and any direct
pane/helper PID fallback is gated by `terminateProcessWithStartToken`. Missing,
unavailable, or mismatched start tokens skip direct signalling and record the
skip reason.
