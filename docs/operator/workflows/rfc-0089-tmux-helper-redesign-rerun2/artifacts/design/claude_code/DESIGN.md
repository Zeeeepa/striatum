# RFC 0089 Phase 1 - Session/Pane Liveness Design (Claude Lane)

author: designer-claude-opus-4.7-001

## Premise

Phase 1 must move "is the lane alive?" from "is the transient `tmux
attach-session` client alive?" to "does the tmux session/pane still own a
running pane process whose identity matches what we launched?" The lane
process must outlive any attach client; an operator's `tmux attach`/`detach`
cycle is observer-only and must not mutate supervisor state. D028 holds: tmux
pane text and `.striatum/scratch/<supervisor_id>/pty.log` remain private
diagnostics and never become workflow state, provenance, byline input, verdict
input, or export content.

## Current State (Audit, Not Speculation)

A substantial part of the Phase 1 scaffolding is already on disk and must be
respected rather than rewritten. The honest design is: complete the residual
coupling, harden the metadata contract, and document the wire-up.

In place today:

- `go/pkg/supervisor/tmux_liveness.go` defines `TmuxIdentity`, the five
  failure-class constants (`tmux_session_missing`, `tmux_pane_missing`,
  `tmux_pane_dead`, `tmux_pane_pid_mismatch`, `tmux_unavailable`), the
  `ProbeTmuxLiveness` / `ProbeLaneLiveness` probe API, and
  `TmuxIdentityFromMetadata` for read paths.
- `go/pkg/supervisor/pty.go` `launchPTY` already returns `PID:
  identity.PanePID` (pane process pid, not the attach client) and records
  `tmuxBackedMetadata` (session_name, window_id, pane_id, pane_pid,
  pane_start_token, attach_command, attach_client_pid, captured_at).
- `go/pkg/supervisor/liveness.go` `Liveness.run` calls `ProbeLaneLiveness` per
  tick and marks the supervisor lost on a tmux failure class (with a three-tick
  grace for `tmux_unavailable`).
- `go/pkg/mutations/supervision_control.go` `reconcileSupervisorForDelivery`
  refuses delivery on any tmux failure class and surfaces the class verbatim.
  `stopTmuxBackedLane` uses `tmux kill-session` and falls back to SIGTERM on
  the pane pid only if tmux is uncontactable.
- `go/pkg/reads/supervision.go` (`HandleSuperviseStatus`, `reattachStatusView`),
  `go/pkg/reads/dashboard.go`, `dashboard_all.go`, `status.go`, and
  `go/pkg/reads/doctor.go` all branch on `live.Backed == "tmux"` and project
  the failure class.
- `go/pkg/mutations/recovery.go` consumes `ProbeLaneLiveness` for the sweep
  path.
- `go/pkg/supervisor/helper.go` distinguishes attach-client exit from agent
  exit via `attachClientExitPayload` and emits a separate
  `attach_client_exited` event when the tmux probe still reports
  `tmux_ok`/`tmux_unavailable`.

The residual gap, which Phase 1 must close:

- The PTY-helper byte channel still rides the `tmux attach-session` PTY.
  `launchPTY` does `pty.Start(attachCmd)` and the helper's `cmd.Wait()` watches
  the attach process. When the operator (or any client) exits attach, the
  helper recognises the case and emits `attach_client_exited`, but then
  returns from `RunHelper`, which closes the helper process. After that, the
  daemon's writes to the FIFO have no consumer driving the pane's pty, and the
  next `supervise.send` either blocks on the FIFO writer or breaks. So while
  the **liveness identity** is the pane pid, the **byte delivery path** is
  still attach-coupled. An "observer-only attach" is not actually achievable
  until delivery is decoupled.

That gap is the load-bearing change in this design. Everything else is wiring
hardening, tests, doc updates, and one rollback knob.

## Decision: Byte Delivery via `tmux send-keys`, Operator Attach Separate

Decouple lane bytes from attach. The helper owns the tmux session it created
and delivers packet frames to the pane via `tmux send-keys -t <pane_id> -l --
<frame>` (the `-l` literal flag plus `--` end-of-options sentinel, applied
once per FIFO line). Newline submit is a separate `send-keys -t <pane_id> C-m`
so the per-adapter PTY submit driver work owned by RFC 0088 stays the source
of truth for "what counts as Enter." Operator attach (`tmux attach-session -t
<session>`) is a parallel process the operator runs on demand; the helper does
not start, watch, or own it.

Consequences:

- `launchPTY` no longer calls `pty.Start(attachCmd)`. The helper does not need
  a ptmx at all; it has a `TmuxByteWriter` instead.
- The supervised "Cmd" the helper waits on becomes the **tmux pane process**,
  not the attach client. Waiting on the pane pid uses `os.FindProcess` +
  `Signal(syscall.Signal(0))` polling (we cannot `Wait()` a non-child), backed
  by the existing `ProbeTmuxLiveness` poll. The helper's `childDone` channel
  becomes a polling goroutine that fires when the probe goes from OK to a
  terminal failure class (`tmux_session_missing`, `tmux_pane_missing`,
  `tmux_pane_dead`, `tmux_pane_pid_mismatch`).
- `HelperEventAttachExited` is no longer emitted by the helper (there is no
  attach to exit). The event constant stays defined for one release for
  backward compatibility with any in-flight `helper-events.jsonl` files, but
  `attachClientExitPayload` and `tmuxExitCause` are deleted along with the
  `AttachPID` field on `LaunchResult`. Daemon-side handling that consumed
  `attach_client_exited` becomes dead and is removed.
- `pty.log` tee remains available: the helper opens a `tmux pipe-pane -t
  <pane_id> -o "cat >> .../pty.log"` when
  `STRIATUM_AGENT_LOOP_DEBUG_LOG` is set or defaulted, instead of teeing the
  ptmx. This keeps D151 (private diagnostics scratch) intact and removes
  another dependency on attach.

This is exactly what TASK.md item (1) asks for: "stop treating tmux
attach-session as the supervised lane identity." Identity AND delivery move off
attach.

## Exact Files and Functions to Change

`go/pkg/supervisor/pty.go`

- Rewrite `launchPTY` to:
  1. Resolve `runID`/`laneID`/`tmux` binary; on miss, route to
     `launchPlainPTY` (or fail closed if `RequireTmux`). Unchanged.
  2. Build the same `tmux new-session -d -s <name> -c <wd> [-e KEY=VAL]...
     -- <command>...` invocation. Unchanged. Append `--` before
     `spec.Command` to forbid argv-as-flag injection from lane command
     entries.
  3. Run `tmux set-option -t <name> status off` and a new
     `tmux set-option -t <name> remain-on-exit on`. `remain-on-exit on`
     keeps the pane (in `dead` state) after its child exits, so the probe
     classifies it as `tmux_pane_dead` rather than racing into
     `tmux_session_missing` (tmux destroys an empty session by default).
  4. Capture identity via `CaptureTmuxIdentity` (unchanged).
  5. **Do not** call `pty.Start(attachCmd)`. Instead construct a
     `TmuxByteWriter{Runner: spec.TmuxRunner, Pane: identity.PaneID}` and
     return it as `StdinWriter`. Return `Cmd: nil`, `AttachPID: 0`.
- Delete the `AttachPID` field from `LaunchResult` (after staged removal — see
  rollback).
- Add `TmuxByteWriter` (in a new `tmux_writer.go` if size warrants, otherwise
  same file) with `Write([]byte) (int, error)` and `Close() error`. `Write`
  splits at the last `\n` in the buffer, sends the literal segment with
  `send-keys -l -t <pane>`, sends `C-m` to submit, and buffers any trailing
  partial line. `Close` flushes any trailing partial line as literal without
  submit.

`go/pkg/supervisor/helper_protocol.go`

- Add `HelperEventPaneLost` (string `pane_lost`) carrying the terminal failure
  class. This replaces the load-bearing role of `attach_client_exited` for
  signalling the daemon that the pane disappeared.
- Mark `HelperEventAttachExited` deprecated in a comment with a removal target
  ("remove after one release; no longer emitted").

`go/pkg/supervisor/helper.go`

- `RunHelper`:
  - Drop the `pty.Start(attachCmd)` path. `result.StdinWriter` is now a
    `TmuxByteWriter` (PTY-backed lanes) or the existing plain-PTY ptmx
    (non-tmux fallback). Both implement `io.WriteCloser`.
  - Replace the `result.Cmd.Wait()` goroutine with a `paneWatcher` that polls
    `ProbeTmuxLiveness` for tmux-backed launches (or signal-0 with start-token
    comparison for plain-PTY) every `HelperOptions.LivenessPollInterval`
    (default 2 s, capped to 5 s) and fires `paneDone` on the first terminal
    class.
  - Stop pumping the PTY for "progress" bytes on tmux-backed lanes. Helper
    progress bytes were attach-screen escape sequences and not useful for
    state. For tmux-backed lanes the helper subscribes to `tmux pipe-pane`
    output if the diagnostic file is enabled, otherwise emits no progress
    events. `HelperEventProgress` continues to be emitted for plain-PTY
    lanes.
  - On `paneDone` fire `pane_lost` with the failure class instead of
    `agent_exited`. Plain-PTY lanes continue to emit `agent_exited`.
  - Delete `attachClientExitPayload` and `tmuxExitCause`; delete the
    `AttachPID > 0` branch in `terminateProcess`. The remaining
    `terminateProcess` only signals `result.PID` (the pane pid) when the
    helper is told to tear down via context cancellation.

`go/pkg/mutations/supervision_control.go`

- Drop the `attach_client_pid` SIGTERM in `HandleSuperviseStop` (the field will
  no longer be populated; the kill-session path is sufficient).
- `tmuxMetadataFromHelperEvents` continues to read tmux metadata from the
  helper's `agent_started` payload. No structural change; rename the
  consumed event type constant if `HelperEventPaneLost` is renamed.
- `reconcileSupervisorForDelivery` is unchanged in shape but gains one branch:
  when `live.Class == tmux_unavailable` and the pointer state is `attached`,
  treat the unavailability as soft (return a retry-suggesting RPC error)
  rather than marking lost. (`Liveness.run` already has the three-tick grace;
  delivery should not be the surface that drives lost transitions.)

`go/pkg/supervisor/liveness.go`

- `Liveness.run` continues to drive heartbeats and lost detection. No
  structural change. Add one assertion: if `row.Metadata` reports
  `tmux.state == "backed"` but `TmuxIdentityFromMetadata` returns
  `ok=false`, mark the supervisor lost with reason
  `tmux_metadata_corrupt`. This catches metadata schema drift early instead
  of falling through to a noisy `tmux_session_missing` cascade.

`go/pkg/reads/supervision.go`, `reads/dashboard.go`, `reads/status.go`,
`reads/doctor.go`, `reads/dashboard_all.go`

- These already consume `ProbeLaneLiveness` and project the failure class.
  The only addition is rendering an `attach_command` field on the
  `supervise.status` JSON whenever metadata reports
  `tmux.state == "backed"`, including when liveness is not OK, so the
  operator can run `tmux attach-session -t <session>` to inspect a stuck
  pane. The existing `tmuxMetadata` projection already extracts
  `attach_command`; ensure it is included in the OK branch of
  `HandleSuperviseStatus` as well as the unhealthy branch.

`go/pkg/mutations/recovery.go`

- Already calls `ProbeLaneLiveness`. Add explicit handling so a tmux failure
  class triggers `markSupervisorLostInTx` with the **class string** as the
  reason (the recovery sweep currently classifies but does not pass the class
  through cleanly). This keeps the lost reason matching the failure-class
  vocabulary `supervise.status` emits.

## Metadata Shape and Database/Read Projections

The metadata blob lives at `process_supervisor_pointers.metadata_json` under
key `tmux`. The shape Phase 1 commits to:

```json
{
  "tmux": {
    "state": "backed",
    "session_name": "striatum-<run>-<lane>-<sup>",
    "window_id": "@7",
    "pane_id": "%12",
    "pane_pid": 27345,
    "pane_start_token": "1716847182",
    "attach_command": "tmux attach-session -t <session_name>",
    "captured_at": "2026-05-28T17:10:01.123456Z"
  }
}
```

Removed in this phase:

- `attach_client_pid` — no longer recorded because there is no helper-owned
  attach client.

Unavailable case:

```json
{
  "tmux": {
    "state": "unavailable",
    "unavailable_reason": "tmux_not_found" | "missing_run_or_lane" |
                         "tmux_identity_capture_failed"
  }
}
```

Liveness is injected at read time by `attachTmuxLiveness` and is **not**
persisted. The persisted `tmux` object is launch-time identity only; the
liveness sub-object is a derived read-model value. This preserves the
"metadata is authoritative for identity, liveness is computed" invariant the
existing reads already follow.

No new SQL columns. No new tables. No new events except the
`HelperEventPaneLost` JSONL event in `helper-events.jsonl` (scratch file,
not durable state). The existing `supervisor.lost` event remains the durable
domain event; its payload gains the failure class verbatim under
`payload.reason`.

## Liveness Probe API

`ProbeLaneLiveness(ctx, runner, metadata, pid, expectedStartToken)
LaneLiveness` is the single entry point and is unchanged.

`LaneLiveness` fields:

- `Backed`: `"tmux"` or `"plain_pty"`.
- `Alive`: bool.
- `Class`: one of the five tmux classes (`tmux_ok`, `tmux_session_missing`,
  `tmux_pane_missing`, `tmux_pane_dead`, `tmux_pane_pid_mismatch`,
  `tmux_unavailable`) for tmux-backed lanes; `pid_ok`, `pid_gone`,
  `pid_identity_mismatch`, `pid_identity_unavailable` for plain-PTY.
- `Tmux`: pointer to `TmuxLiveness` when `Backed == "tmux"`.
- `ObservedPID`, `Detail`: probe context.

The probe is read-only. It must never mutate supervisor state. Mutations live
in the caller (`Liveness.run`, `reconcileSupervisorForDelivery`,
recovery sweep).

Probe sequence (tmux-backed):

1. `tmux has-session -t <session_name>` → session existence.
2. `tmux display-message -p -t <pane_id>
   "#{pane_id}|#{pane_pid}|#{pane_dead}|#{pane_start_time}"` →
   pane existence, pane pid, pane death, pane start time.
3. Compare observed `pane_id` against stored. Mismatch ⇒ `tmux_pane_missing`.
4. If `pane_dead == 1` ⇒ `tmux_pane_dead`.
5. Compare observed `pane_pid` against stored. Mismatch ⇒
   `tmux_pane_pid_mismatch`.
6. If a `pane_start_token` is stored AND observed, compare. Mismatch ⇒
   `tmux_pane_pid_mismatch` (same class — pid recycle and start token
   divergence are the same failure semantically).
7. Otherwise `tmux_ok`.

`tmux_unavailable` is returned only when the tmux binary itself is uncontact-
able (`exec.ErrNotFound`, probe timeout). It is a soft failure: the heartbeat
loop tolerates up to three consecutive ticks before marking lost; delivery
reconciliation returns a retry error rather than marking lost.

Probe timeout: 2 s default, overridable via `STRIATUM_TMUX_PROBE_TIMEOUT` (already
implemented in `tmuxProbeTimeout`).

## Liveness Failure Classes

The five terminal classes carry distinct operator semantics:

- `tmux_session_missing` — `has-session` failed. Most common cause: someone
  ran `tmux kill-session` out-of-band, or the daemon process crashed and the
  tmux server died with it. Operator recovery: investigate; if expected,
  workflow continues by acknowledging the lost supervisor.
- `tmux_pane_missing` — session exists but the pane id is gone. Cause:
  `tmux kill-pane` or a multi-pane layout change. Treat identically to
  `tmux_session_missing` for state transitions; the class is preserved for
  diagnostics.
- `tmux_pane_dead` — pane survives in `remain-on-exit on` state with no
  running child. This is the explicit "child exited" signal. The pane buffer
  can be inspected with `tmux capture-pane`, but per D028 that text is private
  diagnostic — not provenance.
- `tmux_pane_pid_mismatch` — pane pid changed, or stored vs observed
  `pane_start_time` diverged. Catches the rare case of a pane that respawned
  its child (e.g., a wrapper exec'd) before our heartbeat ticked. Treated as
  lost: identity drifted from launch attestation.
- `tmux_unavailable` — soft: tmux binary not contactable. Three-tick grace in
  the heartbeat loop; retry-coded RPC error on delivery; never auto-promotes
  to lost on a single tick.

## Where the Probe Is Called

| Surface | Function | Behavior on failure class |
| --- | --- | --- |
| Heartbeat tick | `supervisor.Liveness.run` | `MarkSupervisorLost(reason=class)` for terminal classes; three-tick grace for `tmux_unavailable` |
| Delivery (`supervise.send`) | `reconcileSupervisorForDelivery` | Terminal: `markSupervisorLostInTx(reason=class)` + `rpc.NewError("invalid_transition", ...)`. `tmux_unavailable`: retry-coded `invalid_transition` without state change |
| Read (`supervise.status`) | `HandleSuperviseStatus` | Project `liveness="gone"`, `lane_attestation="unattested"`, `lane_attestation_reason=class`. Include `attach_command` regardless |
| Read (`supervise.reattach_status`) | `reattachStatusView` | `reattach_state=lost_candidate`/`needs_verification`, `reattach_reason=class` |
| Read (`status`, `dashboard`, `dashboard_all`) | `attachSupervisorTmux` + `attachTmuxLivenessFromMetadata` | Pass-through projection; UI shows class and remediation hint |
| Read (`doctor --verbose`) | `reads/doctor.go` | Counted in supervisor diagnostics with class breakdown |
| Recovery sweep | `mutations/recovery.go` | `markSupervisorLostInTx(reason=class)` matching heartbeat semantics |
| Stop (`supervise.stop`) | `stopTmuxBackedLane` | Always runs `tmux kill-session -t <name>`; falls back to SIGTERM(pane_pid) only when tmux is uncontactable; ignores attach client entirely |

## How `supervise.stop` Terminates the Tmux Lane

The current `stopTmuxBackedLane` already does the right thing: send `tmux
kill-session -t <name>`, swallow "no server / session missing" errors as
success, and only fall back to `terminateProcess(panePID)` when tmux itself
is uncontactable. Phase 1 keeps that logic, deletes the
`attach_client_pid` SIGTERM line in `HandleSuperviseStop`, and asserts:

- After `kill-session` returns, the supervisor row transitions to `stopped`
  (existing `updateSupervisorState`) and emits `supervisor.stopped` with the
  stop reason and the (now-vestigial) `signal: "tmux_kill_session"` payload.
- Subsequent `supervise.status` returns `state=stopped` and projects the
  recorded tmux metadata (session/pane/attach command) verbatim for operator
  reference; liveness is not re-probed for terminal states.

## Tests

`go/pkg/supervisor/` package — unit tests with a stub `TmuxRunner`:

- `TestProbeTmuxLiveness_OK` — happy path, `pane_dead=0`, pids match.
  *Already exists; keep.*
- `TestProbeTmuxLiveness_SessionMissing` — `has-session` exits non-zero with
  "can't find session". Expect class `tmux_session_missing`. *Add.*
- `TestProbeTmuxLiveness_PaneMissing` — `has-session` OK but
  `display-message` returns a different `pane_id`. Expect
  `tmux_pane_missing`. *Add.*
- `TestProbeTmuxLiveness_PaneDead` — `display-message` returns
  `pane_dead=1`. Expect `tmux_pane_dead`. *Add.*
- `TestProbeTmuxLiveness_PIDMismatch` — observed `pane_pid` differs from
  stored. Expect `tmux_pane_pid_mismatch`. *Add.*
- `TestProbeTmuxLiveness_StartTokenMismatch` — observed `pane_start_time`
  differs from stored token. Expect `tmux_pane_pid_mismatch`. *Add.*
- `TestProbeTmuxLiveness_Unavailable_NotFound` — runner returns
  `exec.ErrNotFound`. Expect `tmux_unavailable`. *Already covered; keep.*
- `TestProbeTmuxLiveness_Unavailable_Timeout` — runner returns
  `context.DeadlineExceeded`. Expect `tmux_unavailable`. *Add.*
- `TestProbeLaneLiveness_DisableEnvFallsBackToPID` —
  `STRIATUM_TMUX_PROBE_DISABLE=1` forces plain-PTY path even when metadata is
  tmux-backed. *Add.*

`TmuxByteWriter` tests with a recording stub `TmuxRunner`:

- `TestTmuxByteWriter_SubmitsLineByLine` — writing
  `"hello\nworld\n"` produces two `send-keys -l "hello"; send-keys C-m;
  send-keys -l "world"; send-keys C-m` invocations against the recorded
  pane id.
- `TestTmuxByteWriter_BuffersPartialLine` — writing `"foo"` records only
  the literal; the submit happens on the next `\n`.
- `TestTmuxByteWriter_CloseFlushesPartial` — `Close()` after
  `"abc"` records the literal and **no** `C-m`.
- `TestTmuxByteWriter_HandlesSendKeysFailure` — runner returns an error;
  Write surfaces the error and does not corrupt internal state on the next
  call.

Helper-level tests in `go/pkg/supervisor/helper_test.go`:

- `TestHelperEmitsPaneLost_OnTmuxFailure` — start under a stub launch path
  whose `TmuxByteWriter` and probe runner together simulate the pane going
  dead. Expect the helper to emit `pane_lost` with class
  `tmux_pane_dead`, then exit.
- `TestHelperNeverEmitsAttachClientExited` — the helper has no attach
  client; scan the emitted events and assert no `attach_client_exited` is
  present. *Replaces the existing attach-exit tests.*
- `TestHelperOperatorAttachInvisibleToHelper` — simulate an operator opening
  a *separate* attach client and exiting; helper must remain running and
  continue to emit `packet_accepted` for subsequent FIFO frames. (This is
  the load-bearing regression test for TASK item 1.)

Integration tests in `tmux_liveness_integration_test.go` (gated behind
`-tags=tmux_integration` because they require a real tmux binary; CI must
run them in a job that installs tmux):

- `TestIntegration_AttachClientExitDoesNotMarkLost` — launch a tmux-backed
  lane, open `tmux attach-session` from the test, kill the attach client,
  wait two heartbeat intervals, assert `supervise.status` still reports
  `attached`/`alive`.
- `TestIntegration_KillPaneTransitionsLost` — launch, `tmux kill-pane -t
  <pane>`, assert next probe returns `tmux_pane_dead` or
  `tmux_pane_missing` (whichever wins the race; both are acceptable
  terminal classes) and that `supervisor.lost` is emitted with the same
  class as `reason`.
- `TestIntegration_KillSessionTransitionsLost` — launch, `tmux kill-session
  -t <name>`, assert `tmux_session_missing` and lost.
- `TestIntegration_SuperviseStopKillsSession` — launch, call
  `supervise.stop`, assert the named session is gone from `tmux
  list-sessions` and the row is `stopped`.

Read-projection tests in `go/pkg/reads/supervision_test.go`:

- `TestSuperviseStatus_ProjectsAttachCommandWhenAttached` — already partial;
  extend to assert `attach_command` is present in the OK branch.
- `TestSuperviseStatus_ProjectsFailureClass_PaneDead` — fake metadata + stub
  runner forcing pane_dead; expect `tmux.liveness.class = "tmux_pane_dead"`
  and `lane_attestation_reason = "tmux_pane_dead"`.
- `TestSuperviseStatus_TmuxUnavailable_KeepsAttachedNotLost` — stub returns
  unavailable; status reports `liveness="gone"` but the supervisor row state
  stays `attached` (status is read; lost transitions belong to mutation
  surfaces).

`go/pkg/mutations/supervision_control_test.go`:

- `TestReconcileSupervisorForDelivery_TmuxPaneDead_MarksLost` — feed a
  fake pointer/lease setup, runner reports `tmux_pane_dead`, expect lost
  transition and an `invalid_transition` RPC error carrying the class.
- `TestSuperviseStop_TmuxKillSession_NoAttachSIGTERM` — assert no
  `os.Process.Signal(SIGTERM)` is sent against any pid when tmux
  `kill-session` succeeds; the existing `terminateProcess` should not be
  invoked.
- `TestSuperviseStop_TmuxUnavailable_FallsBackToSIGTERM` — runner errors
  with `exec.ErrNotFound`; expect a single SIGTERM to the recorded pane pid
  and a `tmux_kill_fallback_reason="tmux_unavailable"` annotation in the
  event payload.

Authority-guardrail tests in
`go/pkg/mutations/write_scope_guard_test.go` and the
`docs/reference/command-authority-matrix.md` regression test:

- No new RPC routes are added. The matrix changes only by documenting the
  expanded JSON shape (see Docs section). Run the matrix guardrail test to
  confirm.

Total new tests: roughly 18 unit + 4 integration + 5 read projection + 3
mutation = 30 additions. Roughly 4 existing tests are deleted or replaced
(the attach-exit ones).

## Rollback Behavior

Phase 1 ships with two structurally-defined rollbacks:

1. **`STRIATUM_TMUX_PROBE_DISABLE=1`** (already implemented in
   `ProbeLaneLiveness`). When set, the probe ignores metadata and falls
   through to PID-based liveness using the recorded pane pid. This keeps the
   supervisor liveness signal alive even when tmux is misbehaving, at the
   cost of attestation precision (pane pid recycling becomes invisible).
   Useful for triage runs but never recommended for production.
2. **`supervision.require_tmux: false`** in workflow.json — when set, tmux
   unavailability at launch falls through to `launchPlainPTY` and the lane
   becomes plain-PTY backed. Metadata records
   `tmux.state="unavailable"` with the `unavailable_reason`. Liveness then
   uses the existing signal-0 + start-time path. This is the
   already-implemented non-tmux fallback.

There is intentionally no "use the old attach-as-liveness behavior" rollback
flag. The old behavior was incorrect (D080 byline regressions from attach
exit) and reintroducing it would compromise attestation. The two rollbacks
above cover the two real production risks (tmux flake, tmux missing).

## Non-Tmux Fallback Behavior

`launchPlainPTY` is the explicit fallback. It is invoked when:

- `tmux` binary is not on `PATH` (`exec.LookPath` fails), and
  `RequireTmux=false`.
- `STRIATUM_RUN_ID` or `STRIATUM_LANE_ID` is missing from the lane env, and
  `RequireTmux=false`.
- `CaptureTmuxIdentity` fails (rare), and `RequireTmux=false`.

When `RequireTmux=true` the launch fails closed with a structured error and
the supervisor row never reaches `attached`.

Plain-PTY metadata shape:

```json
{"tmux": {"state": "unavailable", "unavailable_reason": "<reason>"}}
```

Liveness for plain-PTY lanes uses `PIDLiveWithStartToken` (signal-0 plus
start-time comparison) — the existing path. Tests:
`TestProbeLaneLiveness_PlainPTY_OK`,
`TestProbeLaneLiveness_PlainPTY_PIDGone`,
`TestProbeLaneLiveness_PlainPTY_StartTokenMismatch` (all already exist).

## Command-Authority / Spec / Decision-Log Updates

`docs/reference/command-authority-matrix.md`:

- No new RPC methods. Add a paragraph under the existing
  `supervise.status` row noting that the response includes the `tmux` and
  `tmux.liveness` sub-objects when the lane is tmux-backed, and that the
  `lane_attestation_reason` field uses the five `tmux_*` failure-class
  strings as canonical values.
- Run the authority guardrail test after the doc edit.

`docs/reference/spec.md`:

- Update the supervisor section to record: "Tmux-backed agent-loop lanes
  treat the pane process pid + pane_start_time as the supervised identity.
  The `tmux attach-session` client is an observer only and never the
  supervised process." Cross-link to RFC 0089 §1 and the failure class list.

`docs/decisions/decision-log.md`:

- On acceptance: add the **D152** entry from RFC 0089 §"Proposed
  Decision-Log Entry" verbatim. The wording is already drafted in the RFC.

`docs/reference/ubiquitous-language.md`:

- Add an entry under "supervisor" for **pane liveness class** with the five
  values and short definitions; cross-reference RFC 0089.
- Mark the legacy `attach_client_pid` metadata key as retired.

`AGENTS.md` and `docs/how-to/how-to-agent.md`:

- No content changes needed. The agent workflow is unchanged; the supervisor
  internals are below the agent contract.

`CHANGELOG.md`:

- Under `Unreleased`, add a bullet: "RFC 0089 Phase 1: tmux-backed lanes track
  liveness via pane identity, not the transient attach client. Operator
  `tmux attach-session` is observer-only." Reference RFC 0089 and D152.

## Open Questions (None Blocking)

- **`tmux send-keys -l` quoting edge cases.** Packet frames are JSON lines;
  `send-keys -l` treats its argument as a literal but does not strip newlines
  or escapes. The `TmuxByteWriter` splits at `\n` before invoking `send-keys`,
  so embedded newlines in JSON string values (already escaped as `\n` literal
  characters in the wire form) are safe. Lane bootstrap shellouts that depend
  on bracketed-paste are RFC 0088 work, not Phase 1. If a per-adapter quirk
  surfaces in the build review, it lands in a Phase 1.1 follow-up patch, not
  a Phase 1 blocker.
- **Operator wraps `striatum supervise attach`.** Out of scope (Phase 2/3 per
  RFC 0089). The Phase 1 contract is: status returns a copyable `tmux
  attach-session` command string; the operator invokes it themselves.
- **`remain-on-exit` retention policy.** Phase 1 leaves the dead pane in the
  session for diagnostics; supervisor cleanup via `supervise.stop` runs
  `kill-session` and removes it. A future "auto-prune after N minutes" knob
  is not Phase 1.

## Can Live Tmux Monitoring Be Enabled By Config/Default Change Alone?

Yes — once Phase 1 lands. The Phase 1 implementation work is the foundation
RFC 0089 §"Phase 3" depends on. After Phase 1:

- Tmux-backed launch is already the `UsePTY=true` path; the supervisor never
  treats attach as identity; metadata is rich enough for Phase 2 read-surface
  work to lift `attach_command` into status/dashboard/web projections.
- Phase 3's "agent-loop lanes default to tmux-backed when tmux is installed"
  becomes a one-line default change in the lane-config branch that decides
  `UsePTY` / `RequireTmux`, plus a doc update. No new code paths.

The handoff statement for the implementation lane: **after Phase 1 lands,
live tmux monitoring is a configuration/default flip plus tests, not a code
redesign.** Phase 2 read-surface work and Phase 3 default flip are unblocked.
