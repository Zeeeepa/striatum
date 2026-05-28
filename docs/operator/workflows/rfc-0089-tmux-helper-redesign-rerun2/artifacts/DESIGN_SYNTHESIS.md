---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["design/claude_code/DESIGN.md", "design/codex/DESIGN.md", "design/agy/DESIGN.md", "docs/rfcs/0089-tmux-backed-lane-monitoring.md", "docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun2/TASK.md", "docs/operator/workflows/rfc-0089-tmux-helper-redesign/artifacts/DESIGN_SYNTHESIS.md"]
---

# RFC 0089 Phase 1 - Tmux Helper Redesign Synthesis (rerun2)
author: synthesizer-claude-opus-4.7-001
status: proposed
date: 2026-05-28

## 1. Reconciliation summary

The three rerun2 designs agree on the diagnosis (attach pid must not be the
supervised identity), on the probe vocabulary (the five `tmux_*` failure
classes plus `tmux_ok`), and on the read/mutation/recovery surfaces that must
consume the probe. They diverge on one load-bearing question and a handful of
smaller scoping choices. This synthesis resolves each one explicitly so the
implementer has a single buildable path.

| Question | Claude design | Codex design | AGY design | Synthesis choice |
| --- | --- | --- | --- | --- |
| How are packet bytes delivered to the pane? | replace `pty.Start(attachCmd)` with `tmux send-keys -l` via a new `TmuxByteWriter`; helper holds no ptmx | keep attach-PTY for delivery; fix daemon-side state interpretation | keep attach-PTY for delivery; helper distinguishes attach exit from agent exit | **keep attach-PTY for delivery in Phase 1; fix daemon-side state interpretation.** Defer `send-keys` delivery to a follow-on phase with explicit TUI-compatibility validation (see §3). |
| What turns "attach client exited but pane alive" from `state=detached` into a no-op? | helper never emits `attach_client_exited` because there is no attach client | `recordSuperviseReportEvent` consults `ProbeLaneLiveness` and skips the detached transition when tmux is OK | helper continues to emit `attach_client_exited` and exits with code 0, daemon treats it as observer event | **Codex's central fix: `recordSuperviseReportEvent` guards on `ProbeLaneLiveness`.** Defense in depth even after a later delivery-decoupling phase. |
| Metadata shape | extend existing `tmux` block; drop `attach_client_pid` | extend existing `tmux` block; keep `attach_client_pid` | extend existing `tmux` block; keep `attach_client_pid` | **extend existing `tmux` block; keep `attach_client_pid` as an optional diagnostic field.** No DDL, no rename window. |
| `remain-on-exit on` on the launched session | yes - keeps `tmux_pane_dead` distinct from `tmux_session_missing` | not mentioned | not mentioned | **adopt.** Pure diagnostic upgrade; classifies child exits cleanly. |
| Helper-level fix for attach exit on tmux-backed lanes | n/a (no attach client in Claude's design) | helper emits `attach_client_exited` only when probe is `tmux_ok`/`tmux_unavailable`; `agent_exited` with tmux cause otherwise | same as Codex | **adopt Codex/AGY's helper fix.** Pairs with the daemon-side `recordSuperviseReportEvent` guard. |
| `LaunchResult.AttachPID` field | delete | keep as metadata only | keep as metadata only | **keep**. Operator visibility into the bridge process during diagnostics. |
| Probe disable rollback | `STRIATUM_TMUX_PROBE_DISABLE=1` (already implemented) | same | same | **already implemented; keep**. Document it. |

Where any of the three designs contradicts the choices in this synthesis, the
choices below win.

## 2. Premise

`go/pkg/supervisor/pty.go::launchPTY` today already returns `PID:
identity.PanePID` (pane process pid, not the attach client pid) and records
tmux identity (`session_name`, `window_id`, `pane_id`, `pane_pid`,
`pane_start_token`, `attach_command`, `attach_client_pid`, `captured_at`) in
`LaunchResult.Metadata["tmux"]`. `go/pkg/supervisor/tmux_liveness.go`
implements `CaptureTmuxIdentity`, `ProbeTmuxLiveness`, and `ProbeLaneLiveness`
with the five failure classes plus `tmux_ok`. Reads
(`HandleSuperviseStatus`, `reattachStatusView`, `HandleDashboard`,
`dashboardAllStatus`, `doctor`) branch on `live.Backed == "tmux"` and project
the failure class. `reconcileSupervisorForDelivery` and `stopTmuxBackedLane`
already use the probe.

The bug RFC 0089 must close is concentrated in two places that still treat
"attach client exited" as a state transition:

1. **`go/pkg/mutations/supervision.go::recordSuperviseReportEvent`** converts
   `helper.attach_client_exited` into a supervisor `detached` row transition
   regardless of whether the underlying tmux pane is still alive. A live pane
   gets downgraded the moment any attach bridge exits.
2. **`go/pkg/supervisor/helper.go::RunHelper`** waits on `result.Cmd` (the
   attach client) and emits `attach_client_exited` even when the pane behind
   the attach is still alive and accepting bytes. The event payload is then
   consumed by the path in (1).

Phase 1 must close both. Per §3 it must not also redesign byte delivery; the
byte channel stays on the attach PTY.

## 3. Architectural decision: identity vs. delivery

The most consequential disagreement between the three designs is whether
Phase 1 must replace the attach-PTY byte channel.

Claude's design argues delivery must move to `tmux send-keys -l`: as long as
the helper holds `pty.Start(attachCmd)` as its ptmx, attach-client exit
breaks future writes (EIO), so "observer-only attach" is not actually
achieved. Codex and AGY's designs treat the attach PTY as bridge plumbing
that stays attached for the lifetime of the helper, and argue the bug is
daemon-side state interpretation, not the byte channel.

Both framings are technically correct about different processes:

- The **helper's own attach client** is internal plumbing. It is launched by
  `launchPTY`, dies with the helper, and is the byte pump. An operator never
  sees it directly. RFC 0089 does not require it to be observer-only - it
  requires *the operator's* tmux attach (a separate process the operator
  runs) to be observer-only.
- The **operator's tmux attach client** is a separate `tmux attach-session`
  invocation run from the operator's own terminal. It already coexists with
  the helper's attach client (tmux supports multiple clients per session)
  and detaching it does nothing to the helper or the pane.

The operator-side observer-only property is already achieved by the current
launch shape *if* the daemon stops reading the helper's `attach_client_exited`
event as a `detached` state transition. That makes the daemon-side fix the
load-bearing change.

Replacing the delivery channel with `tmux send-keys -l` is the right
long-term direction. It cleans up helper lifecycle (helper can crash and
restart without losing the lane), removes the residual coupling between
bridge process exit and byte delivery, and simplifies the supervisor model.
But it carries non-trivial TUI compatibility risk:

- `claude` TUI relies on PTY-style stdin and bracketed-paste semantics.
- `codex` agent-loop currently consumes bootstrap via argv but live packet
  delivery rides the same PTY.
- `send-keys -l` translates Enter to `C-m`, which goes through tmux's
  pseudo-terminal handling. Per-adapter regression risk is real and is RFC
  0088 territory.

Phase 1 commits to the daemon-side fix, leaves byte delivery on the attach
PTY, and explicitly defers send-keys delivery to a follow-on phase that can
land with adapter-by-adapter compatibility tests. Section 12 lists this as
out of scope.

## 4. Exact files and functions to change

### 4.1 `go/pkg/supervisor/pty.go`

- Add a single line in `launchPTY` after the `set-option status off` call:
  `_ = exec.Command("tmux", "set-option", "-t", sessionName, "remain-on-exit",
  "on").Run()`. This keeps the pane in `dead` state after its child exits so
  the probe reports `tmux_pane_dead` instead of racing into
  `tmux_session_missing` (tmux destroys empty sessions by default).
- Append `--` before `spec.Command` in the `new-session` argv to forbid
  argv-as-flag injection from lane command entries. (Defensive; no current
  exploit, but tightens the boundary.)
- Comment update on `LaunchResult.Cmd`: clarify that for tmux-backed
  launches this is the attach bridge process, not the supervised lane.
  `Cmd.Process.Pid` must not be treated as lane identity downstream.
- Keep `LaunchResult.AttachPID` and `LaunchResult.PID` (= pane pid) as-is.
- Keep `tmuxBackedMetadata` shape as-is.

### 4.2 `go/pkg/supervisor/helper.go`

- In the `case err := <-childDone:` branch, change the dispatch:
  - Compute `live := ProbeLaneLiveness(ctx, opts.TmuxRunner,
    result.Metadata, result.PID, panePidStartToken(result.Metadata))`.
  - If `result.Metadata["tmux"].state == "backed"` and `live.Class` is
    `tmux_ok` or `tmux_unavailable`, emit `HelperEventAttachExited` with
    payload `{attach_pid, exit_code, observed_at, tmux_liveness:
    live.Class}` and **return nil without tearing down the supervisor**.
    The pane is still alive; the helper exit is then a graceful detach
    that the daemon will treat as bridge-exited-but-pane-alive (see §4.3).
  - Otherwise (terminal class or non-tmux), emit `HelperEventAgentExited`
    with `cause: live.Class` for tmux-backed lanes.
- The existing `attachClientExitPayload` helper stays but its decision is
  now driven by `ProbeLaneLiveness` rather than a tmux runner shortcut.
  The single source of truth for "pane is alive" is the probe.
- `terminateProcess(result)` continues to be called only for forced
  teardown (packet forward error, context cancel, agent exit with terminal
  class).
- No `tmux send-keys` writer is added. `forwardPacketStream` continues to
  write to `result.StdinWriter` (the attach PTY ptmx) unchanged.

### 4.3 `go/pkg/mutations/supervision.go`

This file holds the central daemon-side fix.

- Add a metadata lookup in `findReportSupervisor` so
  `recordSuperviseReportEvent` has access to `process_supervisor_pointers.
  metadata_json` (specifically the `tmux` sub-object).
- In `recordSuperviseReportEvent` for `HelperEventAttachExited`:
  - Load the pointer's tmux metadata.
  - If `metadata.tmux.state == "backed"` **and** the report payload says
    `tmux_liveness ∈ {tmux_ok, tmux_unavailable}`:
    - Refresh heartbeat (`updateReportSupervisorHeartbeat`).
    - Append a curated daemon event `supervisor.attach_client_exited`
      with payload `{attach_pid, exit_code, observed_at, tmux_liveness}`.
    - Merge `attach_client_last_exit: {pid, exit_code, observed_at,
      tmux_liveness}` into pointer metadata.
    - Leave `state=attached` (do not call
      `updateReportSupervisorDetached`).
  - Otherwise (legacy non-tmux pointer, missing payload, or any terminal
    class): fall through to the existing `updateReportSupervisorDetached`
    path so legacy attach-only behavior is preserved.
- Backward compatibility: pointer rows without a `tmux` metadata block are
  treated as non-tmux (legacy) and continue to detach on attach exit. This
  prevents older supervisor rows from getting stuck `attached` after a
  helper exit.

### 4.4 `go/pkg/mutations/supervision_control.go`

- `reconcileSupervisorForDelivery` is already routed through
  `ProbeLaneLiveness`. One clarification: when `live.Class ==
  tmux_unavailable` and pointer state is `attached`, return a retry-coded
  `invalid_transition` RPC error and **do not mark lost**. The heartbeat
  loop already has a three-tick grace; delivery should not be the surface
  that drives lost transitions.
- `HandleSuperviseStop` already calls `stopTmuxBackedLane`. Keep that path.
  Drop the unconditional `attach_client_pid` SIGTERM that follows
  `kill-session`: it is redundant when the session is gone, and harmful if
  the pid has been recycled. The fallback `terminateProcess(panePID)` on
  `tmux_unavailable` already covers the no-tmux path.
- `supervisor.lost` event payloads emitted from this file must include
  `tmux_liveness: <class>` when the cause is a tmux failure class.

### 4.5 `go/pkg/supervisor/liveness.go`

- No structural change. `Liveness.run` already uses `ProbeLaneLiveness` and
  marks lost on terminal classes with a three-tick grace for
  `tmux_unavailable`.
- Add one assertion: if `row.Metadata.tmux.state == "backed"` but
  `TmuxIdentityFromMetadata(row.Metadata)` returns `ok=false`, mark the
  supervisor lost with reason `tmux_metadata_corrupt`. This catches
  metadata schema drift early instead of silently falling through.

### 4.6 `go/pkg/reads/supervision.go` and read projections

- `HandleSuperviseStatus`, `statusSessions`, `HandleDashboard`,
  `dashboardAllStatus`, and `reattachStatusView` already consume
  `ProbeLaneLiveness`. Two small additions:
  1. Ensure `attach_command` is projected in the OK branch of
     `HandleSuperviseStatus` (not only in the unhealthy branches), so the
     operator can copy the attach command for any live tmux-backed lane.
  2. Introduce a shared helper that computes `lane_attestation` from
     `(supervisor.state, ProbeLaneLiveness, pointer/daemon-row consistency)`.
     Use it from all four projection sites. The rule:
     - `state=attached` + `tmux_ok` + pointer/daemon consistency ⇒
       `lane_attestation=attested`.
     - Terminal tmux class ⇒ `unattested` with that class as
       `lane_attestation_reason`.
     - `tmux_unavailable` ⇒ `unattested` or `needs_verification`
       (projection-dependent), never `lost`.
     - Plain-PTY keeps existing pid/start-token behavior.
- Dashboard queries currently omit `ps.pid` and `ps.pid_start_time` in some
  paths and mark `state='attached'` as attested. Backfill the missing
  columns and route through the shared helper.

### 4.7 `go/pkg/reads/doctor.go`

- Already calls `ProbeLaneLiveness` through `reattachStatusRows`. Keep
  read-only. Add the failure-class vocabulary mapping:
  - `tmux_ok` → non-problem.
  - `tmux_unavailable` → warning (verification needed).
  - `tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`,
    `tmux_pane_pid_mismatch` → problem; emit remediation hint
    (`investigate the operator's tmux state`, `acknowledge stale lane and
    restart`).

### 4.8 `go/pkg/mutations/recovery.go`

- `HandleRecoveryProcessReconcile` already uses `ProbeLaneLiveness`.
  Tighten so the failure class is passed through verbatim as
  `process.lost.reason` (today the class is computed and then dropped in
  favor of a generic reason).
- If a separate supervisor sweep path exists that does not yet use
  `ProbeLaneLiveness`, route it through the shared helper from §4.6.

### 4.9 No new RPC routes

This synthesis intentionally adds no new daemon RPC methods. The
`command-authority-matrix.md` requires only a paragraph update under
`supervise.status` (see §10). The authority guardrail test should pass
unchanged.

## 5. Metadata shape and database/read projections

Single source of truth: `process_supervisor_pointers.metadata_json.tmux`.
No new SQL columns, no new tables, no migration.

Backed launch shape (set at launch time):

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
    "attach_client_pid": 27420,
    "captured_at": "2026-05-28T17:10:01.123456Z"
  }
}
```

Unavailable launch shape (plain-PTY fallback):

```json
{
  "tmux": {
    "state": "unavailable",
    "unavailable_reason": "tmux_not_found" | "missing_run_or_lane" | "tmux_identity_capture_failed"
  }
}
```

Diagnostic fields merged after launch when an attach-bridge exit is
observed against a live pane:

```json
{
  "tmux": {
    "state": "backed",
    "...": "...",
    "attach_client_last_exit": {
      "pid": 27420,
      "exit_code": 0,
      "observed_at": "2026-05-28T17:11:14.512Z",
      "tmux_liveness": "tmux_ok"
    }
  }
}
```

Liveness is computed at read time by `ProbeLaneLiveness` /
`attachTmuxLivenessFromMetadata` and **not** persisted. The persisted `tmux`
object is launch identity plus diagnostic merges only; the per-read
`tmux.liveness` sub-object is a derived projection. This preserves the
existing "metadata is authoritative for identity, liveness is computed"
invariant.

`PointerRow` does not gain new struct fields. All extended tmux properties
stay nested under `Metadata["tmux"]`. This keeps the rollback path trivial
(an older binary reads `pid` from the same column and ignores the metadata
extensions).

## 6. Liveness probe API and failure classes

API stays single-entry through `ProbeLaneLiveness`:

```go
func ProbeLaneLiveness(
    ctx context.Context,
    r TmuxRunner,
    metadata map[string]any,
    pid int,
    expectedStartToken string,
) LaneLiveness
```

`LaneLiveness` fields:

- `Backed`: `"tmux"` or `"plain_pty"`.
- `Alive`: bool.
- `Class`: one of `tmux_ok`, `tmux_session_missing`, `tmux_pane_missing`,
  `tmux_pane_dead`, `tmux_pane_pid_mismatch`, `tmux_unavailable` for tmux;
  `pid_ok`, `pid_gone`, `pid_identity_mismatch`,
  `pid_identity_unavailable` for plain-PTY.
- `Tmux`: pointer to `TmuxLiveness` when `Backed == "tmux"` (probe detail).
- `ObservedPID`, `Detail`: probe context for surfaces that want to render
  diagnostics.

Tmux probe sequence:

1. `tmux has-session -t <session_name>` → existence.
2. `tmux display-message -p -t <pane_id> "#{pane_id}|#{pane_pid}|
   #{pane_dead}|#{pane_start_time}"` → pane existence, pane pid, dead bit,
   start time.
3. Observed `pane_id` must equal stored. Mismatch ⇒ `tmux_pane_missing`.
4. `pane_dead == 1` ⇒ `tmux_pane_dead`.
5. Observed `pane_pid` must equal stored. Mismatch ⇒
   `tmux_pane_pid_mismatch`.
6. If `pane_start_token` is stored AND observed, they must match. Mismatch
   ⇒ `tmux_pane_pid_mismatch` (pid recycle and start-time drift are
   semantically the same failure).
7. Otherwise `tmux_ok`.

The probe never invokes `capture-pane`, `pipe-pane`, `save-buffer`,
`show-buffer`, `copy-mode`, or `select-pane -P`. D028 must be enforced by a
guard test (§9.6).

Probe timeout: 2 s default; `STRIATUM_TMUX_PROBE_TIMEOUT` overrides.
Already implemented; keep.

Class semantics:

- `tmux_session_missing` — session was killed out-of-band or tmux server
  died. Terminal.
- `tmux_pane_missing` — session exists, pane id no longer resolves.
  Terminal.
- `tmux_pane_dead` — pane survives in `remain-on-exit` mode with no
  running child. Terminal. `tmux capture-pane` can inspect the buffer for
  the operator but per D028 that text is private diagnostics, not
  provenance.
- `tmux_pane_pid_mismatch` — pid changed or start-time drifted. Terminal.
- `tmux_unavailable` — tmux binary uncontactable or probe timeout. Soft:
  three-tick grace in heartbeat; retry-coded RPC error on delivery; never
  auto-lost on a single tick.

## 7. Where the probe is called

| Surface | Function | Behavior on failure class |
| --- | --- | --- |
| Heartbeat tick | `supervisor.Liveness.run` | `MarkSupervisorLost(reason=class)` for terminal classes; three-tick grace for `tmux_unavailable`; immediate lost on `tmux_metadata_corrupt`. |
| Delivery (`supervise.send`) | `reconcileSupervisorForDelivery` | Terminal: `markSupervisorLostInTx(reason=class)` + `invalid_transition` RPC error carrying the class. `tmux_unavailable`: retry-coded `invalid_transition`, no state change. |
| Helper child-exit branch | `RunHelper`'s `case <-childDone` | `tmux_ok`/`tmux_unavailable` ⇒ emit `attach_client_exited` and return nil. Terminal ⇒ emit `agent_exited` with `cause=class` and tear down. |
| Daemon report (`supervise.report`) | `recordSuperviseReportEvent` for `HelperEventAttachExited` | tmux-backed + payload says `tmux_ok`/`tmux_unavailable` ⇒ refresh heartbeat, append `supervisor.attach_client_exited`, merge metadata, **keep `state=attached`**. Otherwise legacy detached path. |
| Read (`supervise.status`) | `HandleSuperviseStatus` | Project `tmux.liveness.class` and `lane_attestation_reason=class`. Include `attach_command` regardless. |
| Read (`supervise.reattach_status`) | `reattachStatusView` | `reattach_state=lost_candidate`/`needs_verification`, `reattach_reason=class`. |
| Read (`status`, `dashboard`, `dashboard_all`) | shared `lane_attestation` helper from §4.6 | Pass-through projection; UI shows class and remediation hint. |
| Read (`doctor --verbose`) | `reads/doctor.go` | Counted in supervisor diagnostics with class breakdown and remediation hints. |
| Recovery sweep | `mutations/recovery.go` | `markSupervisorLostInTx(reason=class)` matching heartbeat semantics; class also flows into `process.lost.reason`. |
| Stop (`supervise.stop`) | `stopTmuxBackedLane` | Always runs `tmux kill-session -t <name>`; falls back to SIGTERM(pane_pid) only when tmux is uncontactable; no SIGTERM on attach_client_pid. |

## 8. `supervise.stop` semantics

The existing `stopTmuxBackedLane` does the right thing today: send `tmux
kill-session -t <name>`, swallow "no server / session missing" as success,
fall back to `terminateProcess(panePID)` only when tmux is uncontactable.
Phase 1 keeps that logic and additionally:

- Removes the unconditional `attach_client_pid` SIGTERM that follows
  `kill-session` (redundant; harmful if pid recycled).
- Asserts that after `kill-session` returns, the row transitions to
  `stopped` (via the existing `updateSupervisorState`) and emits
  `supervisor.stopped` with `{reason, signal: "tmux_kill_session"}`.
- Subsequent `supervise.status` returns `state=stopped` and projects the
  recorded tmux metadata verbatim for operator reference; liveness is not
  re-probed for terminal states.

## 9. Tests and fixtures

All test files placed alongside the code they exercise. Total: roughly 18
unit + 4 integration (gated on a real tmux) + 7 read + 6 mutation + 2
guardrail.

### 9.1 Probe unit tests (`go/pkg/supervisor/tmux_liveness_test.go`)

- `TestProbeTmuxLiveness_OK` — happy path; matching pids, `pane_dead=0`.
  *Already exists; keep.*
- `TestProbeTmuxLiveness_SessionMissing` — stub runner returns non-zero on
  `has-session`. Expect `tmux_session_missing`.
- `TestProbeTmuxLiveness_PaneMissing` — `has-session` ok; `display-message`
  returns a different `pane_id`. Expect `tmux_pane_missing`.
- `TestProbeTmuxLiveness_PaneDead` — `display-message` returns
  `pane_dead=1`. Expect `tmux_pane_dead`.
- `TestProbeTmuxLiveness_PIDMismatch` — observed `pane_pid` differs from
  stored. Expect `tmux_pane_pid_mismatch`.
- `TestProbeTmuxLiveness_StartTokenMismatch` — observed `pane_start_time`
  differs from stored token. Expect `tmux_pane_pid_mismatch`.
- `TestProbeTmuxLiveness_StartTokenEmptyIsOK` — stored token empty; probe
  returns `tmux_ok` with `detail="start_token_unverified"`.
- `TestProbeTmuxLiveness_Unavailable_NotFound` — runner returns
  `exec.ErrNotFound`. Expect `tmux_unavailable`. *Already covered; keep.*
- `TestProbeTmuxLiveness_Unavailable_Timeout` — runner returns
  `context.DeadlineExceeded`. Expect `tmux_unavailable`.
- `TestProbeLaneLiveness_DisableEnvFallsBackToPID` —
  `STRIATUM_TMUX_PROBE_DISABLE=1` forces plain-PTY path even with
  tmux-backed metadata.

### 9.2 Helper tests (`go/pkg/supervisor/helper_test.go`)

- `TestHelperEmitsAttachClientExited_WhenPaneAlive` — stub
  `helperLaunch` returns a tmux-backed `LaunchResult`; stub `TmuxRunner`
  reports `tmux_ok`; simulate `result.Cmd.Wait()` returning (attach
  client exited). Assert helper emits `HelperEventAttachExited` with
  `tmux_liveness="tmux_ok"`, **returns nil**, and does not call
  `terminateProcess`.
- `TestHelperEmitsAgentExited_WhenPaneDead` — same shape but stub
  `TmuxRunner` reports `tmux_pane_dead`. Assert helper emits
  `HelperEventAgentExited` with `cause="tmux_pane_dead"` and tears down.
- `TestHelperEmitsAgentExited_OnTmuxUnavailableRespects3TickGrace` —
  ensure `tmux_unavailable` at child-exit time still emits
  `attach_client_exited` rather than `agent_exited` (matches the
  heartbeat-loop policy).
- *Replace* the existing `TestHelper_AttachExitTreatedAsAgentExit` (if
  present) with the above three.

### 9.3 Helper-integration tests (`go/pkg/supervisor/helper_test.go`)

- `TestHelperOperatorAttachInvisibleToHelper` — launch a tmux-backed lane
  through `helperLaunch`, simulate an external `tmux attach-session`
  process exiting (without touching the helper-owned attach client), then
  verify the helper continues to forward subsequent FIFO frames and emits
  `HelperEventPacketAccepted`.

### 9.4 Tmux integration tests (`go/pkg/supervisor/tmux_liveness_integration_test.go`)

Gated behind `-tags=tmux_integration` (must skip when `exec.LookPath("tmux")`
fails). CI must add a job that installs tmux.

- `TestIntegration_AttachClientExitDoesNotMarkLost` — launch a tmux-backed
  lane, open `tmux attach-session` from the test, kill the attach client,
  wait two heartbeat intervals, assert `supervise.status` still reports
  `attached`/`alive`. **Load-bearing regression for TASK item 1.**
- `TestIntegration_KillPaneTransitionsLost` — launch, `tmux kill-pane -t
  <pane>`, assert the next probe returns `tmux_pane_dead` or
  `tmux_pane_missing` (race-acceptable) and `supervisor.lost` carries the
  same class.
- `TestIntegration_KillSessionTransitionsLost` — launch, `tmux
  kill-session -t <name>`, assert `tmux_session_missing` and lost.
- `TestIntegration_SuperviseStopKillsSession` — launch, call
  `supervise.stop`, assert the named session is gone from `tmux
  list-sessions` and the supervisor row is `stopped`.

### 9.5 Mutation/report tests (`go/pkg/mutations/supervision_test.go` and `go/pkg/mutations/supervision_control_test.go`)

- `TestRecordSuperviseReport_AttachExit_TmuxOK_KeepsAttached` — fake
  pointer with `tmux.state=backed`; report payload with
  `tmux_liveness="tmux_ok"`. Assert: no `state=detached` transition;
  heartbeat refreshed; `supervisor.attach_client_exited` event appended;
  `attach_client_last_exit` merged into metadata.
- `TestRecordSuperviseReport_AttachExit_TmuxPaneDead_FallsThroughToDetached` —
  same setup but payload says `tmux_liveness="tmux_pane_dead"`. Assert
  legacy detached transition.
- `TestRecordSuperviseReport_AttachExit_LegacyNonTmux_DetachesAsToday` —
  pointer has no `tmux` metadata. Assert legacy detached transition.
- `TestReconcileSupervisorForDelivery_TmuxPaneDead_MarksLost` — feed
  pointer/lease with tmux-backed metadata; runner reports
  `tmux_pane_dead`. Assert lost transition + `invalid_transition` RPC
  error carrying the class.
- `TestReconcileSupervisorForDelivery_TmuxUnavailable_RetryNotLost` —
  runner errors with `exec.ErrNotFound`. Assert no lost transition,
  retry-coded RPC error.
- `TestSuperviseStop_TmuxKillSession_NoAttachSIGTERM` — runner reports
  successful `kill-session`. Assert no SIGTERM is sent against any
  `attach_client_pid` or pane pid.
- `TestSuperviseStop_TmuxUnavailable_FallsBackToSIGTERM` — runner errors
  with `exec.ErrNotFound`. Assert single SIGTERM to recorded pane pid and
  `tmux_kill_fallback_reason="tmux_unavailable"` annotation on the
  emitted `supervisor.stopped` event.

### 9.6 Read-projection tests (`go/pkg/reads/supervision_test.go`)

- `TestSuperviseStatus_ProjectsAttachCommand_WhenAttached` — assert
  `attach_command` is present in the OK branch.
- `TestSuperviseStatus_ProjectsFailureClass_PaneDead` — fake metadata +
  stub runner forcing `pane_dead`. Assert `tmux.liveness.class =
  "tmux_pane_dead"` and `lane_attestation_reason = "tmux_pane_dead"`.
- `TestSuperviseStatus_TmuxUnavailable_KeepsAttachedNotLost` — stub
  returns unavailable. Status reports `liveness="gone"` but row state
  stays `attached`.
- `TestSuperviseStatus_LaneAttestationHelperConsistentAcrossSurfaces` —
  feed the same pointer to `HandleSuperviseStatus`, `statusSessions`,
  `HandleDashboard`, and `dashboardAllStatus`. Assert all four return the
  same `lane_attestation` for the same probe class.

### 9.7 D028 guard tests (new file
`go/pkg/supervisor/d028_guard_test.go` plus extend export/archive tests)

- `TestProbeTmuxLivenessNeverReadsPaneText` — runner records every command;
  assert zero invocations of `capture-pane`, `pipe-pane`, `save-buffer`,
  `show-buffer`, `copy-mode`, `select-pane -P`. *Promote to a unit test
  that fails the build on regression.*
- `TestSuperviseStatus_MetadataSanitizer` — feed pointer metadata with
  hostile `pane_text`, `stdout`, `stderr`, `transcript`, `capture`,
  `buffer` fields. Assert none of these surface in the
  `supervise.status` JSON.
- `TestExports_NoTmuxPaneText` — invoke trajectory export, evidence
  export, corpus export, and run archive against a tmux-backed fixture
  and assert no `pty.log` path or `tmux capture-pane` content appears in
  any output payload.

### 9.8 Authority-guardrail tests

- No new RPC routes; the matrix gains only a documentation paragraph (§10).
  Run `go test ./go/...` with the existing authority-guardrail test to
  confirm the matrix and route map still align.

## 10. Documentation updates

### 10.1 `docs/reference/command-authority-matrix.md`

Append a paragraph under the existing `supervise.status` row noting that
the response now includes `tmux` and `tmux.liveness` sub-objects when the
lane is tmux-backed, and that `lane_attestation_reason` uses the canonical
`tmux_*` failure-class strings (`tmux_session_missing`, `tmux_pane_missing`,
`tmux_pane_dead`, `tmux_pane_pid_mismatch`, `tmux_unavailable`). No new
rows; no new methods. Run the authority guardrail test after the edit.

### 10.2 `docs/reference/spec.md`

Update the supervisor section: "For tmux-backed agent-loop lanes, the
supervised identity is the pane process pid plus pane_start_time. The
`tmux attach-session` client is bridge plumbing and is not the supervised
process. Liveness is derived from a tmux probe (session/pane/pane_dead/
pid/start-token), not from the attach client lifetime." Cross-link to RFC
0089 §1 and the failure-class list.

### 10.3 `docs/reference/ubiquitous-language.md`

- New entry under "supervisor": **pane liveness class** with the five
  terminal values and `tmux_ok`; cross-reference RFC 0089.
- New entry: **attach bridge process** for `attach_client_pid` (helper's
  observer of the pane; not the supervised identity).
- Note that `attach_client_pid` remains an optional diagnostic field, not
  a state authority.

### 10.4 `docs/decisions/decision-log.md`

The log already contains D152 accepting RFC 0089. Implementation should
update D152 only if final behavior deviates. The synthesis does not
deviate, so no log edit is required at synthesis time. If the implementer
adopts the deferred send-keys delivery later, that becomes a new D
entry.

### 10.5 `docs/how-to/how-to-agent.md` and `AGENTS.md`

No agent-contract changes. The supervisor internals are below the agent
boundary.

### 10.6 `CHANGELOG.md`

Under `Unreleased`, add: "RFC 0089 Phase 1: tmux-backed lanes use the pane
process pid + start-time as supervised identity, not the attach client.
`supervise.report` no longer downgrades a live pane to `detached` when the
attach bridge exits. Operator `tmux attach-session` works in parallel with
the helper bridge as observer-only." Reference RFC 0089 and D152.

## 11. Rollback and fallback

Two structurally-defined rollbacks ship with Phase 1:

1. **`STRIATUM_TMUX_PROBE_DISABLE=1`** (already implemented in
   `ProbeLaneLiveness`): falls through to PID-based liveness using the
   recorded pane pid. Liveness signal stays alive at the cost of
   attestation precision (pid recycling becomes invisible). Operator
   escape hatch for tmux flake; never the default.
2. **`supervision.require_tmux: false`** in workflow.json: tmux
   unavailability at launch falls through to `launchPlainPTY`. Metadata
   records `tmux.state="unavailable"` with the reason. Liveness then uses
   the existing signal-0 + start-time path.

There is intentionally no "use the old attach-as-liveness behavior"
rollback flag. The old behavior was incorrect (D080 byline regressions
from attach exit) and re-enabling it would re-open the bug.

Non-tmux fallback path (`launchPlainPTY`) is unchanged. It is invoked
when:

- `tmux` binary is missing from `PATH` and `RequireTmux=false`;
- `STRIATUM_RUN_ID` or `STRIATUM_LANE_ID` is missing and
  `RequireTmux=false`;
- `CaptureTmuxIdentity` fails and `RequireTmux=false`.

When `RequireTmux=true`, launch fails closed with a structured error and
the supervisor row never reaches `attached`.

## 12. Out of scope for Phase 1

The following are explicitly deferred to follow-on phases. Naming them
here so the implementer does not accidentally widen scope:

- **`tmux send-keys`-based byte delivery.** Per §3, replacing the
  attach-PTY byte channel with `tmux send-keys -l` is the right long-term
  direction but carries per-adapter TUI compatibility risk that is not
  Phase 1 work. Track as a follow-on with explicit per-adapter regression
  fixtures (claude TUI, codex agent-loop, agy `--print`).
- **Helper crash + pane survival reattach.** If the helper itself dies
  while the pane is alive, Phase 1 marks the supervisor lost. A future
  phase can introduce a helper reattach path that re-spawns the byte
  bridge and reuses the existing pane. Not Phase 1.
- **`striatum supervise attach`** wrapper. Status returns a copyable
  `tmux attach-session` command. The operator runs it themselves. RFC
  0089 Phase 2/3.
- **Auto-flip agent-loop lanes to tmux-backed by default.** Per RFC 0089
  Phase 3. After Phase 1 + Phase 2 read-surface work, this is a one-line
  default change plus tests.
- **`remain-on-exit` auto-prune.** Dead panes are left in the session for
  diagnostics until `supervise.stop` runs `kill-session`. An auto-prune
  knob is a separate decision.
- **Capturing pane text into PostgreSQL or artifacts.** D028 holds.
  Permanently out of scope.

## 13. Handoff statement

**After Phase 1 lands, enabling live tmux monitoring on every RFC 0088
agent-loop lane is a configuration/default flip plus tests, not a code
redesign.**

The Phase 1 substrate (identity capture, probe, projection, report-event
fix, stop semantics) is sufficient for Phase 2 read-surface work (lifting
`attach_command` into status/dashboard/web) and Phase 3 default flip
(switching `UsePTY=true` + `RequireTmux=false` on the agent-loop branch).
No new code paths are required for the flip itself.

The one residual architectural risk worth naming: byte delivery still
rides the attach PTY. Until the deferred send-keys delivery phase lands,
helper crashes lose the byte bridge and the lane is marked lost even
though the pane survives. That risk is bounded - helpers are short-lived
relative to lanes only in pathological cases - and is the explicit
trade-off this synthesis takes to keep TUI compatibility intact.
