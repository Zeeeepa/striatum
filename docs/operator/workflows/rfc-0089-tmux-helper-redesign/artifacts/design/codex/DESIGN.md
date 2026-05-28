# RFC 0089 Phase 1 Design - Tmux Session/Pane Liveness
author: designer-codex-gpt-5.5-xhigh-001
date: 2026-05-28
status: draft

## Objective

Make `tmux attach-session` an observer only. A tmux-backed supervisor is live
when its recorded tmux session and pane process are live, not when a transient
attach client is still connected.

Phase 1 should not flip defaults and should not add transcript capture. Tmux
pane text, `capture-pane` output, and `pty.log` bytes remain private operator
diagnostics under D028. They must not become daemon state, artifacts, exports,
byline evidence, verdict input, or recovery authority.

## Current Failure

The failure starts in `go/pkg/supervisor/pty.go:launchPTY`:

- it creates a detached tmux session;
- it starts `tmux attach-session -t <session>` under a PTY;
- it returns the attach client's PID and `*exec.Cmd` as the supervised lane
  identity.

That value flows through:

- `go/pkg/supervisor/helper.go:RunHelper`, which waits on `result.Cmd.Wait()`
  and emits `agent_exited`;
- `go/pkg/mutations/supervision_control.go:launchPTYHelper`, which persists
  the helper-reported PID as the supervisor PID;
- `HandleSuperviseSend`, `reconcileSupervisorForDelivery`,
  `HandleSuperviseStop`, `sessionLaneAttestation`, `HandleSuperviseStatus`,
  `reattachStatusView`, `statusSessions`, `HandleDashboard`, and
  `dashboardAll*`, which still reason from process PID/start-token liveness.

There is a second, easy-to-miss coupling: the attach client's PTY master is
also the helper's packet-input path. If the attach client exits while the tmux
pane continues, treating the pane as live is not enough; any helper-side packet
input must target the pane itself, not the dead attach client's PTY.

## Design Summary

For tmux-backed launches:

- do not spawn `tmux attach-session` as part of supervision;
- record the tmux session, window, pane, pane PID, and pane PID start token;
- store the pane PID in the existing supervisor PID columns;
- expose `tmux attach-session -t <session>` only as copyable metadata;
- use tmux session/pane probes for delivery reconciliation, status, doctor,
  dashboard, recovery, and attestation;
- write any helper-delivered bytes to the recorded pane by tmux target, never
  through an attach-client PTY;
- stop by killing the tmux session, then clean up helper processes.

No schema migration is required for Phase 1. The existing PID columns can hold
the pane PID, and `process_supervisor_pointers.metadata_json` already carries
safe tmux metadata.

## Launch Metadata

Store this shape under `process_supervisor_pointers.metadata_json.tmux` and in
the `supervisor.started` payload:

```json
{
  "state": "backed",
  "session_name": "striatum-run_1-lane_1-sup_1",
  "window_id": "@1",
  "pane_id": "%2",
  "pane_pid": 12345,
  "pane_pid_start_token": "987654",
  "attach_command": "tmux attach-session -t striatum-run_1-lane_1-sup_1"
}
```

Rules:

- `process_supervisors.pid`, `process_supervisor_pointers.pid`, and
  `daemon_supervisors.pid` store `pane_pid` for tmux-backed supervisors.
- The matching `pid_start_time` columns store `pane_pid_start_token` when the
  platform can provide one.
- No attach-client PID is persisted as supervisor identity. If a later debug
  path records an attach client, it must live under a metadata-only
  `attach_client.role: "observer"` block and must never affect liveness,
  delivery, stop, or attestation.
- Optional fallback keeps the existing unavailable shape:

```json
{
  "tmux": {
    "unavailable_reason": "tmux_not_found"
  }
}
```

Read projections must allowlist only safe fields: session/window/pane ids,
pane PID/start token, attach command, liveness class, unavailable reason, and
static remediation. Do not expose pane text, captured output, arbitrary tmux
stderr, PTY log paths, or transcript bytes.

## Tmux Probe API

Add a tmux probe in `go/pkg/supervisor` with an injectable command runner:

```go
type TmuxLaneIdentity struct {
    SessionName       string
    PaneID            string
    PanePID           int
    PanePIDStartToken string
}

type TmuxProbeResult struct {
    Live                 bool
    FailureClass         string
    SessionName          string
    PaneID               string
    CurrentWindowID      string
    CurrentPanePID       int
    CurrentPaneDead      bool
    CurrentPIDStartToken string
}

func ProbeTmuxLane(ctx context.Context, runner TmuxCommandRunner, id TmuxLaneIdentity) TmuxProbeResult
```

Probe sequence:

1. Resolve `tmux`; failure is `tmux_unavailable`.
2. Run `tmux has-session -t <session>`; failure is
   `tmux_session_missing`.
3. Run `tmux display-message -p -t <pane_id>
   "#{window_id}\t#{pane_id}\t#{pane_pid}\t#{pane_dead}"`; failure after the
   session exists is `tmux_pane_missing`.
4. If `pane_dead` is `1`, return `tmux_pane_dead`.
5. If current `pane_pid` differs from stored `pane_pid`, return
   `tmux_pane_pid_mismatch`.
6. If a stored start token exists and the current token can be read, compare
   it; mismatch is also `tmux_pane_pid_mismatch`.
7. Otherwise return `Live: true`.

Use exactly these Phase 1 failure classes:

- `tmux_unavailable`
- `tmux_session_missing`
- `tmux_pane_missing`
- `tmux_pane_dead`
- `tmux_pane_pid_mismatch`

Extract the duplicated process-start-token helpers from
`go/pkg/mutations/supervision_process_*.go` and
`go/pkg/reads/supervision_process_*.go` into a shared helper, either exported
from `go/pkg/supervisor` or moved to a tiny `go/pkg/processidentity` package.
The probe, mutation paths, and read paths should compare the same token format.

## Supervisor Helper Changes

Change `go/pkg/supervisor/pty.go` first.

For tmux-backed `launchPTY`:

- create the detached tmux session as today, including the existing `-e KEY=VAL`
  environment propagation;
- do not call `pty.Start(tmux attach-session ...)`;
- query `window_id`, `pane_id`, `pane_pid`, and `pane_dead` immediately after
  `new-session`;
- capture the pane PID start token when available;
- return `LaunchResult.PID` as the pane PID;
- return metadata with `tmux.state: "backed"` and the safe identity fields;
- provide a wait function that polls `ProbeTmuxLane` and returns only when the
  pane is missing, dead, mismatched, or the context is canceled;
- provide a terminate function that runs `tmux kill-session -t <session>`.

For packet input, add a tmux pane writer rather than using an attach-client
PTY. A practical implementation is:

- `tmux load-buffer -b <supervisor-buffer-name> -` with the bytes on stdin;
- `tmux paste-buffer -d -b <supervisor-buffer-name> -t <pane_id>`;
- probe before writing and return a structured error on any tmux failure class.

This keeps long packet prompts out of shell arguments, avoids quoting problems,
and does not read pane text. The helper can still emit `packet_accepted` after
successful writes. It should not emit progress events based on tmux pane output
unless a later decision explicitly accepts a metadata-only progress source.

Change `go/pkg/supervisor/helper.go:RunHelper`:

- stop requiring `LaunchResult.StdinWriter` to also be an `io.Reader`;
- use a separate optional progress reader for non-tmux plain PTY launches;
- wait on the launch result's lane waiter, not `Cmd.Wait()` unconditionally;
- do not emit `agent_exited` because an attach client disconnected;
- if the tmux lane waiter returns a failure class outside an explicit stop,
  emit an event payload that lets the daemon classify the supervisor as lost,
  not as a clean operator stop;
- make `terminateProcess(result)` call the launch result's terminate function
  for tmux-backed lanes, then clean up helper-local processes.

Plain PTY fallback keeps the current `pty.Start(cmd)` behavior, `Cmd.Wait()`,
and SIGTERM/SIGKILL termination.

## Mutation Paths

### Start

Update `go/pkg/mutations/supervision_control.go:launchPTYHelper` and
`HandleSuperviseStart`:

- read helper `agent_started.payload.pid` as the lane PID, now the tmux pane
  PID for tmux-backed launches;
- persist the pane PID and pane PID start token in `process_supervisors`,
  `process_supervisor_pointers`, and `daemon_supervisors`;
- merge full safe tmux metadata into pointer metadata;
- include the same safe tmux metadata in `supervisor.started`;
- keep the current `pidAliveLocal(launch.PID)` startup check, now applied to
  the pane PID for tmux-backed launches.

### Delivery Reconciliation

Update `reconcileSupervisorForDelivery`:

- if pointer metadata says `tmux.state == "backed"` or has both
  `session_name` and `pane_id`, call `ProbeTmuxLane`;
- if the probe is live, continue to pointer and daemon row consistency checks;
- if the probe fails, mark the supervisor lost in the same transaction with
  `markSupervisorLostInTx`, include `liveness_class` plus safe tmux identity
  fields in the event payload, and return `invalid_transition`;
- do not check or care whether an operator attach client exists.

Keep the current PID/start-token path for non-tmux supervisors and for tmux
fallback metadata with `unavailable_reason`.

`writeSupervisorPayload` can keep writing to the FIFO. The helper behind that
FIFO must perform the pane write described above, so the daemon mutation path
does not need to learn terminal byte mechanics.

### Supervise Report

Update `go/pkg/mutations/supervision.go:recordSuperviseReportEvent` so a
tmux-backed helper event that carries one of the tmux failure classes does not
blindly mark the supervisor `stopped`. Clean `supervise.stop` marks stopped;
unexpected missing/dead/mismatched tmux lanes should become `lost` with the
failure class in `supervisor.lost`.

### Stop

Update `HandleSuperviseStop`:

- if the active supervisor is tmux-backed, run
  `tmux kill-session -t <session_name>` as the primary termination action;
- then terminate the helper PID and any recorded observer process as cleanup;
- remove the FIFO as today;
- mark all supervisor rows `stopped`;
- record `supervisor.stopped` with `signal: "tmux_kill_session"` and safe tmux
  identity fields.

If tmux is unavailable at stop time, fall back to signaling the pane PID and
helper PID, but include a `tmux_unavailable` note in the response. If the tmux
session is already missing during an explicit stop, treat stop as idempotent:
clean up helper/FIFO state, mark stopped, and include the failure class as a
note rather than refusing.

### Attestation And Bylines

Update `sessionLaneAttestation` in `go/pkg/mutations/mutations.go`:

- for tmux-backed attached supervisors, attestation is true only when
  `ProbeTmuxLane` is live;
- for non-tmux supervisors, keep PID/start-token attestation;
- if the tmux probe fails, return `attested: false` and `reason` equal to the
  failure class.

This keeps `claim` packet bylines, `artifact.publish`, review verdict gates,
and interrogation live-target checks aligned with the real lane process rather
than a transient attach client.

## Read Paths

Create a shared read helper used by:

- `go/pkg/reads/supervision.go:HandleSuperviseStatus`
- `go/pkg/reads/supervision.go:reattachStatusView`
- `go/pkg/reads/status.go:statusSessions`
- `go/pkg/reads/dashboard.go:HandleDashboard`
- `go/pkg/reads/dashboard_all.go`
- `go/pkg/reads/doctor.go`

For tmux-backed rows:

- enrich `tmux` with `liveness: "alive" | "gone" | "unknown"`;
- include `failure_class` when not live;
- include current pane PID/start token when safe;
- set top-level `liveness` to `alive` only when the tmux probe is live;
- apply the existing stall overlay only after tmux liveness is live;
- set `lane_attestation` from the same probe result.

For `supervise.reattach_status`:

- `reattachable`: tmux probe is live and pointer/daemon rows are coherent;
- `lost_candidate`: `tmux_session_missing`, `tmux_pane_missing`,
  `tmux_pane_dead`, or `tmux_pane_pid_mismatch`;
- `needs_verification`: `tmux_unavailable`;
- `needs_repair`: pointer/daemon row mismatch after a live probe;
- `terminal`: existing terminal states.

Keep reads read-only. `supervise.status` should report the failure class; the
mutating paths that persist `lost` are delivery reconciliation, helper report
handling, explicit recovery, and recovery sweep.

Doctor is currently minimal in Go. Phase 1 should extend it enough to surface
active tmux-backed supervisor failures as stable problem strings and verbose
metadata-only records. It must not read pane text.

## Recovery Sweep

Add a supervisor liveness step to `go/pkg/mutations/recovery.go` before normal
lease expiry in `HandleRecoveryAuto` / `SweepRun`:

1. Query active `process_supervisors` for the run, joined to pointer metadata
   and active job leases.
2. For each tmux-backed supervisor, call `ProbeTmuxLane`.
3. On live probes, no state change.
4. On failure in dry-run mode, report the supervisor id, session id, job id,
   lease id, and failure class.
5. On failure in live mode, mark supervisor/pointer/daemon rows `lost`, record
   `supervisor.lost`, and surface the failure class in the sweep result.

Do not kill or erase the tmux session from recovery. If an active repo-write
lease is involved, preserve the existing operator-inspection posture: report
the lost supervisor and let the existing stale-lease/recovery policy decide
whether the job can be requeued or must be inspected. If the lease has already
expired, ordinary `expireLeases` handling can run after the supervisor is
classified.

## Rollback And Fallback

- If a lane records `tmux.unavailable_reason`, all paths keep the current
  non-tmux PID/start-token behavior.
- If a tmux-backed row lacks complete pane metadata, treat it as
  `tmux_unavailable` for attestation and delivery but keep read surfaces
  inspectable.
- No workflow default changes are part of Phase 1. Operators can continue
  using plain PTY fallback by leaving `supervision.require_tmux` false.
- A code rollback does not need a schema rollback because Phase 1 uses existing
  row columns and JSON metadata.

## Tests

Add focused tests without requiring tmux for the full suite:

- `go/pkg/supervisor`: fake-runner unit tests for `ProbeTmuxLane` covering
  live, `tmux_unavailable`, `tmux_session_missing`, `tmux_pane_missing`,
  `tmux_pane_dead`, PID mismatch, and start-token mismatch.
- `go/pkg/supervisor`: tmux integration tests skipped when `tmux` is absent:
  launch a `sleep` pane, attach and detach an observer, and assert the probe is
  still live; kill the session and assert `tmux_session_missing`.
- `go/pkg/supervisor/helper_test.go`: prove attach-client exit does not emit
  `agent_exited` and does not close the helper's lane waiter while the pane
  probe is live.
- `go/pkg/supervisor/helper_test.go`: prove tmux-backed packet input uses the
  pane writer and still emits `packet_accepted` without reading pane text.
- `go/pkg/mutations/supervision_control_test.go`: start stores pane PID and
  tmux metadata; send rejects and marks lost on each failure class; attach PID
  absence is ignored; stop calls tmux session termination for tmux-backed rows.
- `go/pkg/mutations/artifact*` and `go/pkg/mutations/review*`: expected byline
  derivation and `require_attested_lane` use the tmux probe, not attach-client
  PID.
- `go/pkg/reads/supervision_test.go`, `status_test.go`,
  `dashboard_test.go`, and `dashboard_all_test.go`: read projections show live
  tmux liveness after observer detach and show each failure class without raw
  terminal text.
- `go/pkg/reads/doctor*_test.go`: doctor reports active tmux-backed supervisor
  failures with stable classes and metadata-only details.
- `go/pkg/mutations/recovery*_test.go`: recovery sweep marks tmux-backed
  missing/dead/mismatched supervisors unhealthy before ordinary stale-lease
  handling.
- D028 guard: extend tmux metadata allowlist and redaction/archive/export tests
  so `pane_text`, `capture-pane` output, `pty.log`, and arbitrary tmux command
  stderr never enter artifacts, trajectory/evidence/corpus/archive exports,
  doctor details, or events.

Required implementation verification:

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

## Documentation Updates

- `docs/reference/spec.md`: update Process Supervision to say tmux-backed
  supervisors use session/pane liveness; document failure classes, pane-writer
  delivery, read-only status behavior, and stop semantics.
- `docs/reference/command-authority-matrix.md`: update notes for
  `supervise.stop`, `supervise.status`, `supervise.reattach_status`, and
  `recovery.sweep`. No new RPC method is required for Phase 1.
- `docs/reference/ubiquitous-language.md`: add or refine a term for
  tmux-backed supervisor liveness as a derived read-model value.
- `docs/decisions/decision-log.md`: add the RFC 0089/D152 decision only when
  the operator accepts the RFC. Phase 1 implementation should not silently mark
  a proposed RFC accepted.

## Handoff

Start with `go/pkg/supervisor/pty.go` and `go/pkg/supervisor/helper.go`. The
critical first correction is not only "which PID is live" but "which terminal
receives helper-delivered bytes": both must use the recorded tmux pane.

Then thread the same probe through mutation guards, attestation, read
projections, doctor, and recovery. Live tmux monitoring can be enabled by
configuration or default change after this Phase 1 slice only if the
observer-detach, missing-session, missing-pane, pane-dead, and
PID/start-token-mismatch tests pass.
