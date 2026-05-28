# RFC 0089 Phase 1 Design - Tmux Session/Pane Liveness

Date: 2026-05-28
Status: design handoff
Target: RFC 0089 Phase 1
author: designer-codex-gpt-5.5-xhigh-001

## Summary

Phase 1 should make tmux-backed lanes live or lost by the tmux session and pane
process identity, not by the lifetime of any `tmux attach-session` client. The
smallest coherent slice is:

- keep `process_supervisors.pid`, `process_supervisor_pointers.pid`, and
  `daemon_supervisors.pid` as the tmux pane pid for tmux-backed launches;
- keep helper pid and attach-client pid in metadata only;
- use one tmux liveness probe everywhere a state transition or read projection
  decides whether the lane is alive;
- make `attach_client_exited` a diagnostic event for tmux-backed lanes when the
  pane is still live, not a supervisor state transition;
- make `supervise.stop` kill the tmux session and then clean up helper/attach
  bridge processes.

This keeps D028 intact: tmux pane text, alternate-screen content, and PTY logs
remain private diagnostics only. The probe and all durable events use process
metadata, byte counts, and liveness classes only.

## Current Shape

The current tree already contains most of the raw probe substrate:

- `go/pkg/supervisor/pty.go` creates a detached tmux session, captures
  `window_id`, `pane_id`, `pane_pid`, `pane_start_time`, returns `PID` as the
  pane pid, and stores `attach_client_pid` separately.
- `go/pkg/supervisor/tmux_liveness.go` implements
  `CaptureTmuxIdentity`, `ProbeTmuxLiveness`, and `ProbeLaneLiveness`.
- `go/pkg/mutations/supervision_control.go` already uses
  `ProbeLaneLiveness` in `reconcileSupervisorForDelivery` and kills the tmux
  session in `HandleSuperviseStop`.
- `go/pkg/reads/supervision.go` can attach tmux metadata and liveness details
  to `supervise.status`.

The remaining hazard is consistency. `supervise.report` currently turns
`attach_client_exited` into `state=detached`, and dashboard/status-style reads
do not all use the same lane-attestation decision. That is enough to recreate
the bug RFC 0089 is trying to close: a live pane can be downgraded because an
observer or I/O bridge exited.

## Metadata Contract

Use the existing JSON metadata path; no database migration is needed for
Phase 1.

`process_supervisor_pointers.metadata_json.tmux` should be the durable metadata
object for tmux-backed lanes:

```json
{
  "state": "backed",
  "session_name": "striatum-run-lane-sup",
  "window_id": "@1",
  "pane_id": "%2",
  "pane_pid": 48211,
  "pane_start_token": "1748452211",
  "attach_command": "tmux attach-session -t striatum-run-lane-sup",
  "attach_client_pid": 48220,
  "captured_at": "2026-05-28T00:00:00Z"
}
```

Additional diagnostic fields may be merged after launch:

```json
{
  "attach_client_last_exit": {
    "pid": 48220,
    "exit_code": 0,
    "observed_at": "2026-05-28T00:00:10Z",
    "tmux_liveness": "tmux_ok"
  }
}
```

The pointer/root supervisor pid columns stay the pane pid. The helper process
pid remains under top-level metadata (`helper_pid`, `helper_pid_start_time`).
Attach pid never becomes the supervised lane identity.

For optional tmux fallback, keep the existing metadata shape:

```json
{"tmux": {"state": "unavailable", "unavailable_reason": "tmux_not_found"}}
```

`supervision.require_tmux=true` continues to fail closed instead of falling
back.

## Liveness API

Keep `go/pkg/supervisor/tmux_liveness.go` as the single liveness API:

```go
func ProbeLaneLiveness(ctx context.Context, r TmuxRunner, metadata map[string]any, pid int, expectedStartToken string) LaneLiveness
```

For tmux-backed metadata, it must ignore the passed process pid except as a
fallback detail and derive health from:

1. `tmux has-session -t <session_name>`;
2. `tmux display-message -p -t <pane_id> "#{pane_id}|#{pane_pid}|#{pane_dead}|#{pane_start_time}"`;
3. pane id equality;
4. pane pid equality;
5. pane start-token equality when both stored and observed tokens are present.

Failure classes:

- `tmux_ok`: session exists, pane exists, pane is not dead, pid matches, start
  token matches when available.
- `tmux_session_missing`: `has-session` fails because the session is gone.
- `tmux_pane_missing`: the pane id is absent or no longer resolves to the
  recorded pane.
- `tmux_pane_dead`: tmux reports `pane_dead=1`.
- `tmux_pane_pid_mismatch`: pane pid or pane start token differs.
- `tmux_unavailable`: tmux cannot be executed or the probe times out.

`tmux_unavailable` is verification failure, not proof that the lane is lost.
Delivery should fail closed until a later probe succeeds or operator recovery
handles it. Read projections can show remediation.

The probe must never call text-bearing tmux commands such as `capture-pane`,
`pipe-pane`, `save-buffer`, `show-buffer`, `copy-mode`, or `select-pane`.

## Implementation Changes

### `go/pkg/supervisor/pty.go`

Preserve the current launch direction:

- `LaunchResult.PID` is the tmux pane pid.
- `LaunchResult.AttachPID` is the attach client pid.
- `LaunchResult.Metadata["tmux"]` carries the identity object.
- `tmuxBackedMetadata` includes `pane_start_token` when available.

Tighten the comments and tests so callers do not treat `Cmd.Process.Pid` as
the lane identity for tmux-backed launches. `Cmd` is the attach/I/O bridge
process today; it is not the supervised lane.

On start, replace the final `pidAliveLocal(launch.PID)` check in
`HandleSuperviseStart` with `ProbeLaneLiveness` when tmux metadata is present.
This proves the row is backed by the pane identity that was just recorded. A
`tmux_unavailable` result during a required-tmux launch should fail the start;
for optional fallback, start should have already fallen back to plain PTY.

### `go/pkg/supervisor/helper.go`

Keep `attachClientExitPayload` as the helper-side distinction between "attach
bridge exited but pane is still live" and "the tmux lane is lost." It should
emit `attach_client_exited` only when `ProbeLaneLiveness` returns `tmux_ok` or
`tmux_unavailable`; missing/dead/mismatched pane remains `agent_exited` with a
tmux cause.

Do not include pane text in helper events. `progress` remains byte counts only.

### `go/pkg/mutations/supervision.go`

Change `recordSuperviseReportEvent` for `HelperEventAttachExited`:

- load pointer metadata in `findReportSupervisor`;
- if metadata is tmux-backed and the report payload says `tmux_liveness` is
  `tmux_ok` or `tmux_unavailable`, do not call
  `updateReportSupervisorDetached`;
- refresh heartbeat, append the `supervisor.attach_client_exited` event, and
  merge `attach_client_last_exit` into pointer metadata;
- keep `state=attached` so byline attestation and status remain tied to the
  pane;
- use the old detached transition only for non-tmux rows or legacy payloads
  where the lane identity truly cannot be verified.

This is the central fix for attach-as-liveness.

### `go/pkg/mutations/supervision_control.go`

`reconcileSupervisorForDelivery` is already the right boundary. Complete it
with tests and one small payload refinement:

- `tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`, and
  `tmux_pane_pid_mismatch` mark the supervisor lost and refuse delivery;
- `tmux_unavailable` refuses delivery without marking lost;
- successful tmux liveness proceeds to pointer and daemon-supervisor consistency
  checks;
- `supervisor.lost` event payload includes `tmux_liveness` for tmux failures.

`HandleSuperviseStop` should remain tmux-first:

- if tmux-backed, run `tmux kill-session -t <session_name>`;
- treat an already-missing session as a successful idempotent stop with note
  `tmux_session_missing`;
- only fall back to terminating pane pid when tmux is unavailable;
- then terminate helper and attach-client pids when present and remove the FIFO.

### `go/pkg/reads/supervision.go`

Make this file the read helper for all supervisor/tmux projections:

- keep `attachTmuxLiveness` and `attachTmuxLivenessFromMetadata`;
- add a shared helper that computes `lane_attestation` from
  `ProbeLaneLiveness`, supervisor state, and reattach consistency;
- use that helper from `HandleSuperviseStatus`, `statusSessions`,
  `HandleDashboard`, and `dashboardAllStatus`.

The rule should be:

- `state=attached` plus `tmux_ok` plus pointer/daemon consistency means
  `lane_attestation=attested`;
- tmux lost classes mean `unattested` with the same class as reason;
- `tmux_unavailable` means `unattested` or `needs_verification`, depending on
  the projection, but never `lost`;
- plain PTY keeps the existing pid/start-token behavior.

Dashboard queries currently omit `ps.pid` and `ps.pid_start_time` in some
paths and mark any attached supervisor as attested. Add the pid/start-token
columns and reuse the shared helper.

### `go/pkg/reads/doctor.go`

The current path through `reattachStatusRows` is good. Keep doctor read-only,
and ensure it reports tmux classes in the same vocabulary:

- non-problem: `tmux_ok`;
- warning/verification: `tmux_unavailable`;
- problem: `tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`,
  `tmux_pane_pid_mismatch`.

### `go/pkg/mutations/recovery.go`

`HandleRecoveryProcessReconcile` already uses `ProbeLaneLiveness`. Add tests
so tmux-backed process reconciliation:

- keeps a running process running for `tmux_ok`;
- keeps it running for `tmux_unavailable`;
- transitions to lost for missing/dead/mismatched pane classes with the class
  in `process.lost.reason`.

If there is a separate recovery sweep path for supervisors, route it through
the same `ProbeLaneLiveness` helper rather than pid-only signal checks.

## Tests

Supervisor probe tests:

- extend `go/pkg/supervisor/tmux_liveness_test.go` for direct tmux command
  errors that represent missing panes, pane-dead, pid mismatch, and start-token
  mismatch;
- keep `TestProbeTmuxLivenessNeverReadsPaneText`;
- keep/extend the integration test proving attach-client exit does not make
  the tmux probe unhealthy.

Helper/report mutation tests:

- replace `TestSuperviseReportRecordsAttachClientExitAsDetached` with a tmux
  case proving `attach_client_exited` keeps `state=attached`, updates heartbeat,
  records metadata, and emits `supervisor.attach_client_exited`;
- add a legacy/non-tmux case if the detached transition is retained;
- keep `agent_exited` as the only helper event that stops the supervisor.

Delivery tests:

- `TestReconcileSupervisorForDeliveryTmuxOKAllowsDelivery`;
- `TestReconcileSupervisorForDeliveryTmuxUnavailableRefusesWithoutLost`;
- `TestReconcileSupervisorForDeliveryTmuxSessionMissingMarksLost`;
- `TestReconcileSupervisorForDeliveryTmuxPaneMissingMarksLost`;
- `TestReconcileSupervisorForDeliveryTmuxPaneDeadMarksLost`;
- `TestReconcileSupervisorForDeliveryTmuxPanePIDMismatchMarksLost`.

Stop tests:

- keep `TestHandleSuperviseStopKillsTmuxSession`;
- add already-missing session idempotence;
- add fallback-to-pane-termination on `tmux_unavailable`;
- assert helper and attach-client pids are terminated as cleanup, not as the
  primary lane stop.

Read-path tests:

- `supervise.status` shows `tmux.liveness.class` and lane attestation from the
  tmux probe;
- `status`, `dashboard`, and `dashboard.all` no longer mark a session attested
  merely because `ps.state='attached'`;
- doctor reports the same tmux failure classes and stays read-only.

D028 guard tests:

- keep probe-level forbidden command checks;
- add a read-metadata sanitizer test with fake `pane_text`, `stdout`, `stderr`,
  `transcript`, `capture`, and `buffer` fields in metadata;
- add export/archive guard coverage or a static guardrail proving tmux pane text
  commands and PTY log paths are not inputs to artifacts, trajectory export,
  evidence export, corpus export, or run archive content.

Verification for the implementation branch:

```bash
cd go && gofmt -l .
cd go && go vet ./...
cd go && go test ./...
```

## Rollback and Fallback

Rollback is localized: disable tmux probing with
`STRIATUM_TMUX_PROBE_DISABLE=1` to fall back to pid/start-token probing while
keeping tmux metadata visible. This should be treated as a temporary operator
escape hatch, not a default.

Non-tmux fallback remains:

- `supervision.require_tmux=true`: fail closed when tmux is missing or identity
  capture fails.
- `supervision.require_tmux=false`: fall back to plain PTY and publish
  `tmux.state=unavailable` metadata with a remediation string in read
  projections.

## Documentation and Authority

No new daemon RPC methods are required, so
`docs/reference/command-authority-matrix.md` should not need new rows. Update it
only if implementation changes the authority behavior of existing
`supervise.*`, `status`, `doctor`, or `recovery.*` routes.

`docs/reference/spec.md` and `docs/reference/ubiquitous-language.md` should be
checked after implementation for wording that still implies a process-supervisor
pid is an attach client. If present, correct it to "pane process identity" for
tmux-backed lanes.

`docs/decisions/decision-log.md` already contains D152 accepting RFC 0089
Phase 1. The implementation should only update it if the final behavior
deviates from D152 or adds a new product decision.

## Builder Notes

The highest-risk edge is conflating three different processes:

- lane process: tmux pane pid; authoritative for liveness and byline
  attestation;
- helper process: packet/PTY bridge; operational plumbing;
- attach client: operator or bridge observer; diagnostic only.

Keep those identities separate in names, metadata, events, and tests. Once
Phase 1 lands, enabling universal tmux monitoring should be a configuration or
default change, not another liveness redesign.
