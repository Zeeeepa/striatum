# RFC 0089 Phase 1 Interrogation Chat
author: reviewer-codex-gpt-5.5-xhigh-001
date: 2026-05-29
interrogation_id: intg_395637caadb787b99bd3a454d6cc9be4
target_session_id: sess_ac356d97a74d3359a78e334dfd05902a
interrogator_session_id: sess_7dce88e1d8fa1b1b8e0280005294b9ee
round_count: 3
status: closed

## Round 1

Question: justify the invariant preventing false-health after helper
attach-client exit. If pane liveness is `tmux_ok` but delivery liveness is
`attach_client_exited`, which read and mutation paths keep the lane from being
reported as fully healthy, and which path refuses delivery until rebridge? Also
confirm whether probe failures record raw tmux pane text or PTY bytes.

Answer summary: the builder identified pane/process liveness and delivery
liveness as separate fields. `supervisorDeliveryDegraded` checks nested
`tmux.delivery_liveness` and top-level `delivery_liveness`; `supervise.send`
refuses degraded delivery before FIFO write and does not record
`supervisor.packet_delivered`. Read projections expose `lane_backend`,
`pane_liveness`, `delivery_liveness`, and `delivery_state`. Probe failure data
is limited to typed identity/failure metadata and does not include raw pane text
or PTY bytes.

## Round 2

Question: what prevents `supervise.rebridge` from reconnecting delivery to a
stale, dead, or replaced pane under PID reuse or stale tmux metadata? Name the
start-token checks and refusal cases. Does clearing delivery degradation after
helper launch race with immediate helper attach failure?

Answer summary: the builder pointed to `requireRebridgeableTmuxPane` and
`ProbeLaneLiveness`. Rebridge refuses non-tmux supervisors, non-attached
supervisors, missing/non-FIFO stdin paths, `tmux_unavailable`, and all non-live
terminal classes such as missing session, missing pane, dead pane, and pane PID
mismatch. Numeric pane start tokens are compared with the recorded token, with
OS process start-token fallback when tmux omits the observed token. The builder
also confirmed a race: if `agent_started` and immediate `attach_client_exited`
are read in the same initial rebridge event batch, the current rebridge metadata
update can clear delivery degradation after recording it.

## Round 3

Question: `drainHelperEvents` appears only in `supervise.send` and
`supervise.stop`, while `supervise.status` explicitly does not drain helper
files. If the helper-owned attach client exits after rebridge returns, or after
normal start outside the initial batch, what ingests that event before
status/dashboard reads? If nothing does, does a false-health window remain?

Answer summary: the builder confirmed the ingestion paths are mutation/sweep
paths, not ordinary reads: `supervise.send`, `supervise.stop`, and
`recovery.sweep` / resident daemon sweep. `supervise.status`, `status`, and
`dashboard` do not synchronously drain helper files. Therefore a false-health
window remains: pane liveness can be `tmux_ok` and delivery can appear healthy
until a send/stop mutation or the resident sweep ingests the attach-exit event.
The proposed minimal fix is daemon-owned helper-event ingestion before the
operator liveness read projection, plus a regression proving attach-exit after
start/rebridge is visible in status/dashboard before any subsequent send.
