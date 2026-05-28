---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["design/claude_code/DESIGN.md", "design/codex/DESIGN.md", "design/agy/DESIGN.md", "docs/rfcs/0089-tmux-backed-lane-monitoring.md", "docs/operator/workflows/rfc-0089-tmux-helper-redesign/TASK.md"]
---

# RFC 0089 Phase 1 - Tmux Helper Redesign Synthesis
author: synthesizer-claude-opus-4.7-001
status: proposed
date: 2026-05-28

## 1. Reconciliation summary

Three independent designs agree on the diagnosis and on the shape of the
liveness probe. They diverge on three load-bearing questions, which this
synthesis resolves explicitly so the implementer has one path:

| Question                                | Claude design           | Codex design                                 | AGY design                       | Synthesis choice                                                                                                                                            |
| --------------------------------------- | ----------------------- | -------------------------------------------- | -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| How are packet bytes delivered to pane? | keep attach-session PTY | drop attach; use `tmux load-buffer/paste-buffer` | keep attach-session PTY        | **keep attach-session PTY as packet transport, observer for liveness**. Buffer-paste delivery deferred; TUI input compatibility is a bigger Phase-2/3 risk. |
| What happens when attach client exits?  | emit `attach_client_exited`, exit, daemon moves pointer to `detached` | helper waiter is the pane probe; attach exit ignored | helper self-heals: relaunch attach | **Claude's `detached` state distinction**. No self-heal in Phase 1; reattach is Phase 2.                                                                    |
| Metadata block name                     | new `tmux_lane`         | extend existing `tmux`                       | extend `tmux`                    | **extend existing `tmux` block** with `state: "backed"` + safe identity fields. Preserves existing read projections; no rename window.                      |

This synthesis is a buildable specification, not a menu. Where any design
contradicts the choices below, the choices below win.

## 2. Premise

`go/pkg/supervisor/pty.go::launchPTY` creates a detached tmux session and
then spawns `tmux attach-session -t <session>` under a creack/pty PTY. The
attach client's pid becomes `LaunchResult.PID` and threads through every
downstream liveness check:

- `mutations/supervision_control.go::HandleSuperviseStart` persists it as
  `striatumd.process_supervisors.pid`.
- `mutations/supervision_control.go::reconcileSupervisorForDelivery` calls
  `pidAliveLocal(supervisor.PID)` before each `supervise.send`.
- `reads/supervision.go::HandleSuperviseStatus` calls
  `pidLiveWithStartToken(pid, ...)` on the attach pid.
- `supervisor/liveness.go::Liveness.run` polls
  `processAliveAtStartTime(l.pid, row.StartedAt)` every 5 s and marks the
  supervisor lost on failure.
- `supervisor/helper.go::RunHelper` waits on `result.Cmd` (the attach
  client) and emits `helper_event=agent_exited` when it returns.
- `mutations/supervision_control.go::HandleSuperviseStop` SIGTERMs
  `supervisor.PID` (the attach client) and never touches the tmux session.
- `mutations/recovery.go::HandleRecoveryProcessReconcile` flips
  `process_executions.state` to `lost` when `pidAlive(pid)` is false.

A `tmux attach-session` client can exit (operator `C-b d`, terminal hangup,
helper SIGTERM, EIO on PTY master) while the underlying tmux session and
the real lane process keep running. Today that exit cascades as
`agent_exited` → `supervisor_lost` → `lane_attestation = unattested` →
`recovery.process_reconcile` flipping the process to `lost`, and any later
`supervise.send` is refused with `pid_gone`. RFC 0089 acceptance test #1
("attach client exit must not mark the lane lost") is impossible without
the redesign in this synthesis.

Phase 1 splits **lane identity** (the pane process) from
**attach-client lifecycle** (the transient PTY observer), routes every
liveness check through a tmux-aware probe when the supervisor is
tmux-backed, and reroutes `supervise.stop` to terminate the tmux session
rather than the attach client. No new RPC verbs are added. No schema
migration is required.

## 3. Launch metadata shape

### 3.1 Extend the existing `tmux` block

The existing `process_supervisor_pointers.metadata_json.tmux` block is
extended with the safe identity fields RFC 0089 §1 requires. The `state`
discriminator replaces the implicit "is `unavailable_reason` set?" check:

```jsonc
"tmux": {
  "state":             "backed",          // "backed" | "unavailable"
  "session_name":      "striatum-run_1-lane_1-sup_1",
  "window_id":         "@3",              // tmux #{window_id}
  "pane_id":           "%4",              // tmux #{pane_id}
  "pane_pid":          48211,             // tmux #{pane_pid} at launch
  "pane_start_token":  "1748452211",      // see §3.3
  "attach_command":    "tmux attach-session -t striatum-run_1-lane_1-sup_1",
  "attach_client_pid": 48309,             // diagnostic only; never the supervised identity
  "captured_at":       "2026-05-28T15:42:09Z"
}
```

Plain-PTY fallback keeps the existing shape:

```jsonc
"tmux": {
  "state":              "unavailable",
  "unavailable_reason": "tmux_not_found"
}
```

Rationale for choosing this over Claude's `tmux_lane` rename: every
existing read projection (`supervise.status`, `dashboard`,
`dashboard_all`, `doctor`, `reattach_status`) already passes through
`reads/supervision.go::attachSupervisorTmux`. Extending the existing block
means no parallel read-side accessor, no two-step migration window, and no
operator scripts to update on the existing `tmux.session_name` /
`tmux.attach_command` keys.

### 3.2 Supervised pid swap

`LaunchResult` gains an `AttachPID` field, and `PID` is rewritten to mean
the **pane pid** (the real lane process) for tmux-backed launches:

```go
type LaunchResult struct {
    PID         int             // pane_pid for tmux-backed; child pid for plain PTY
    StdinWriter io.WriteCloser  // attach PTY master, unchanged
    Cmd         *exec.Cmd       // attach client *exec.Cmd, used for stdin / PTY close
    AttachPID   int             // attach client pid; diagnostic only for tmux-backed
    Metadata    map[string]any  // includes the "tmux" object above for tmux-backed
}
```

The helper emits `agent_started` with `pid = pane_pid` and
`attach_client_pid` in the metadata payload (§7.2). The daemon writes the
**pane pid** into `striatumd.process_supervisors.pid`, the pointer row,
and `daemon_supervisors.pid`. `pid_start_time` is filled from the pane
start token. No schema change is required — the columns already exist.

### 3.3 Capturing pane identity

In `launchPTY`, immediately after the existing `tmux new-session -d ...`
succeeds and before `pty.Start(attachCmd)`:

```go
out, err := exec.Command("tmux", "display-message", "-p",
    "-t", sessionName,
    "#{window_id}|#{pane_id}|#{pane_pid}|#{pane_start_time}",
).Output()
```

Parse the four pipe-delimited fields. If `display-message` fails or any
of `window_id`, `pane_id`, `pane_pid` is empty, the launch returns
`tmux_identity_capture_failed`: tear down the half-built session, then
`RequireTmux ? fail-closed : fall back to plain PTY`. We never silently
accept "treat the attach pid as identity" as a degraded mode.

`pane_start_time` is best-effort: tmux ≤ 2.9 lacks it. When empty, fall
back to `processStartToken(pane_pid)` (see §3.4). When **both** sources
are empty, the probe records the captured token as `""` and downstream
classifies an empty token as `start_token_unverified` (probe still OK;
the lane is not lost because the platform can't expose start time).

### 3.4 Shared process-start-token helper

`processStartToken` / `pidLiveWithStartToken` live today in
`go/pkg/mutations/supervision_process_*.go` and
`go/pkg/reads/supervision_process_*.go` as duplicated helpers. Codex's
design correctly notes this duplication. As part of Phase 1, extract into
`go/pkg/supervisor/process_identity.go` (single home, no new package).
Both probe paths (tmux `pane_start_time` fallback, plain-PTY pid probe)
read identical token semantics from the same helper.

## 4. Tmux liveness probe API

New file `go/pkg/supervisor/tmux_liveness.go`. No other package depends
on tmux execv today; this isolation makes the probe unit-testable with a
fake runner.

```go
type TmuxIdentity struct {
    SessionName    string
    WindowID       string
    PaneID         string
    PanePID        int
    PaneStartToken string  // "" ⇒ best-effort verify only
}

type TmuxLivenessClass string

const (
    TmuxLivenessOK              TmuxLivenessClass = "tmux_ok"
    TmuxLivenessSessionMissing  TmuxLivenessClass = "tmux_session_missing"
    TmuxLivenessPaneMissing     TmuxLivenessClass = "tmux_pane_missing"
    TmuxLivenessPaneDead        TmuxLivenessClass = "tmux_pane_dead"
    TmuxLivenessPanePIDMismatch TmuxLivenessClass = "tmux_pane_pid_mismatch"
    TmuxLivenessUnavailable     TmuxLivenessClass = "tmux_unavailable"
)

type TmuxLiveness struct {
    Class            TmuxLivenessClass
    Healthy          bool   // true iff Class == TmuxLivenessOK
    ObservedPanePID  int    // 0 when unknown
    ObservedStartTok string // "" when unknown
    Detail           string // short non-secret diagnostic; never pane text
}

type TmuxRunner interface {
    // Run executes one tmux command and returns stdout.
    // Implementations MUST capture stderr separately and surface it via
    // err.Error(). Pane text never reaches a TmuxRunner caller.
    Run(ctx context.Context, args ...string) (string, error)
}

func ProbeTmuxLiveness(ctx context.Context, r TmuxRunner, id TmuxIdentity) TmuxLiveness
```

### 4.1 Probe sequence

`ProbeTmuxLiveness` runs only these tmux commands: `has-session`,
`display-message -p`. It never reads pane content (no `capture-pane`,
`pipe-pane`, `save-buffer`, `show-buffer`, `copy-mode`, `select-pane -P`).
This is a structural D028 guarantee, asserted in tests (§7.1).

1. `tmux has-session -t <SessionName>`. Exit 0 ⇒ continue. Non-zero
   ⇒ `tmux_session_missing`. Tmux not on PATH, exec error, or timeout
   ⇒ `tmux_unavailable` (`Detail` = `"tmux: command not found"`, the
   exec error message, or `"probe_timeout"`).
2. `tmux display-message -p -t <PaneID> '#{pane_id}|#{pane_pid}|#{pane_dead}|#{pane_start_time}'`.
   - command failure ⇒ `tmux_pane_missing`;
   - returned `pane_id` differs from `id.PaneID` ⇒ `tmux_pane_missing`
     (tmux reused the slot under a different pane);
   - `pane_dead == "1"` ⇒ `tmux_pane_dead`;
   - `pane_pid != id.PanePID` ⇒ `tmux_pane_pid_mismatch` (pane process
     exited and tmux re-ran something else under the pane; equivalent to
     D080 "pid identity mismatch");
   - `id.PaneStartToken != ""` and `pane_start_time != id.PaneStartToken`
     ⇒ `tmux_pane_pid_mismatch` (same pid reused with a new start time).
3. Otherwise ⇒ `TmuxLivenessOK`, with `ObservedPanePID` and
   `ObservedStartTok` populated for the caller's read projection.

`Lost()` returns true for every class except `TmuxLivenessOK` and
`TmuxLivenessUnavailable`. The unavailable case is a probe outage, not a
lane outage; callers handle it explicitly (§5).

### 4.2 Unified lane probe

Five callsites share a `ProbeLaneLiveness` helper, also in
`tmux_liveness.go`, so callers do not branch on `tmux.state` themselves:

```go
type LaneLiveness struct {
    Backed      string         // "tmux" | "plain_pty"
    Alive       bool
    Class       string         // tmux class OR pid class
    Tmux        *TmuxLiveness  // nil when Backed != "tmux"
    ObservedPID int            // 0 unless we observed one
    Detail      string
}

func ProbeLaneLiveness(ctx context.Context, r TmuxRunner,
    metadata map[string]any, pid int, expectedStartToken string) LaneLiveness
```

`ProbeLaneLiveness` extracts `metadata["tmux"]["state"]`. `"backed"` ⇒
build `TmuxIdentity` from metadata, call `ProbeTmuxLiveness`, translate.
Anything else (missing block, `"unavailable"`, no `state` key) ⇒ fall
through to today's `pidLiveWithStartToken(pid, expectedStartToken)` with
the canonical pid classes (`pid_ok`, `pid_gone`, `pid_identity_mismatch`,
`pid_identity_unavailable`).

`STRIATUM_TMUX_PROBE_DISABLE=1` at the top of `ProbeLaneLiveness` forces
the pid-probe branch even for tmux-backed lanes. See §9.1.

### 4.3 Default runner and timeout

`execTmuxRunner{}` wraps `exec.CommandContext("tmux", args...)` with a
2-second per-call timeout (override via
`STRIATUM_TMUX_PROBE_TIMEOUT`). Context cancellation maps to
`tmux_unavailable` with detail `probe_timeout`.

## 5. Where the probe is called

Five callsites switch from "signal-0 the supervisor pid" to
`ProbeLaneLiveness(...)`. All five route through the same helper, so the
"is this tmux-backed?" branch lives in exactly one place.

### 5.1 `supervisor/liveness.go::Liveness.run`

Today: every 5 s, `processAliveAtStartTime(l.pid, row.StartedAt)`; on
`false`, `MarkSupervisorLost(reason="process_exited")`. Replace with
`ProbeLaneLiveness`, sourcing pointer metadata from the existing
`l.store.GetSupervisorPointer` call. The `PointerRow` shape gains a
`Metadata map[string]any` field; the store reads it from `metadata_json`.

- `Class == "tmux_ok"` or `"pid_ok"` ⇒ `row.State = "running"`.
- `Class == "tmux_unavailable"` ⇒ leave state alone; bump a
  `tmux_probe_skipped_at` timestamp on the row. After **three consecutive
  unavailable ticks (~15 s)** mark
  `lost(reason="tmux_unavailable_persistent")`. This protects a
  partially-rotting daemon environment without flapping on a single
  transient miss.
- Any other non-OK class ⇒ `MarkSupervisorLost(reason=string(Class))`.

### 5.2 `mutations/supervision_control.go::reconcileSupervisorForDelivery`

Today: `pidAliveLocal(supervisor.PID)` then
`processStartToken(supervisor.PID) != supervisor.PIDStartTime`. Replace
with one `ProbeLaneLiveness` call. `markSupervisorLostInTx` takes the
class string as the lost reason; the event payload carries
`reason: "tmux_pane_pid_mismatch"` (etc.). Pointer-reconciliation and
lease-error branches are unchanged.

`tmux_unavailable` at delivery time is **not** retried silently. The send
returns `invalid_transition` with detail
`"tmux probe unavailable; cannot verify lane"` plus the diagnostic detail
(e.g. `tmux: command not found`). The operator fixes the daemon
environment rather than re-issuing blindly.

### 5.3 `reads/supervision.go::HandleSuperviseStatus`

The `tmux` block in the read projection gains a `liveness` sub-block:

```jsonc
"tmux": {
  "state":            "backed",
  "session_name":     "striatum-...",
  "pane_id":          "%4",
  "attach_command":   "tmux attach-session -t striatum-...",
  "liveness": {
    "class":              "tmux_ok",
    "healthy":            true,
    "observed_pane_pid":  48211,
    "observed_pane_start": "1748452211",
    "detail":             null
  },
  "remediation": null
}
```

`liveness == "alive"` (top-level field) iff `LaneLiveness.Alive` (works
for both tmux-backed and plain-PTY lanes). The top-level `liveness` field
keeps its `"alive" | "stalled" | "gone"` shape for back-compat with
operator scripts. `lane_attestation` keeps its current derivation
(`attested` iff `attached`, `alive`, not flagged for reattach repair), so
RFC 0088 byline rules do not change shape.

`reads.SetTmuxRunnerForTest` is the testing seam (matches the existing
pattern for injecting other runners into read paths).

### 5.4 `mutations/recovery.go::HandleRecoveryProcessReconcile`

`process_executions` is the per-packet process record. The probe needs
supervisor metadata, so the recovery sweep joins
`process_executions` to `process_supervisor_pointers` on
`supervisor_id` (the column already exists), pulls `metadata_json`, and
runs `ProbeLaneLiveness`. `tmux_pane_pid_mismatch`, `tmux_pane_dead`,
`tmux_session_missing`, and `tmux_pane_missing` all classify the row as
`lost` with the class as the reason. The legacy path-only check survives
for rows whose supervisor metadata lacks `tmux.state == "backed"`.

### 5.5 `cmd/striatum/doctor.go` and `reads/doctor.go`

Doctor renders `tmux.liveness.class` per-supervisor when `--verbose` is
set, plus a static remediation hint when the probe returned
`tmux_unavailable` (e.g. "install tmux on the daemon host; see
`docs/how-to/daemon-runbook.md`"). Doctor stays a read; it never calls
`MarkSupervisorLost`.

`dashboard`, `dashboard_all`, and `status` pick the new `tmux.liveness`
block up automatically once `attachSupervisorTmux` is extended (§5.3).

## 6. `supervise.stop` rework

`HandleSuperviseStop` (`mutations/supervision_control.go`) is rewired so
the **lane** (the tmux session and its pane process) is terminated, not
the attach client.

```text
1. Resolve tmux identity from pointer metadata (tmuxLaneFromMetadata).
2. Drain helper events (unchanged).
3. If tmux-backed (metadata.tmux.state == "backed"):
     signaled = tmuxKillSession(tmux.SessionName)
     // wraps `tmux kill-session -t <name>` with a 2 s timeout.
     // "session already gone" exit codes are treated as success.
4. As idempotent cleanup, terminateProcess(attach_client_pid) and
   terminateProcess(helper_pid). With the session already gone, both are
   already exiting; the call is safe.
5. If tmux is unavailable mid-stop:
     // fall back to SIGTERMing pane_pid directly.
     // Record the fallback reason in the supervisor.stopped event
     // payload (`tmux_kill_fallback_reason: "tmux_unavailable"`).
6. Unchanged: os.Remove(StdinPipePath) for FIFO cleanup.
7. Mark all supervisor rows `stopped`.
```

The returned `signal` field in the JSON response gains a new symbolic
value `tmux_kill_session`, distinct from `SIGTERM` / `SIGKILL`, so
operators and scripts can tell which path was taken. `supervise.list` and
`dashboard` need no change — they render `stop_reason` verbatim.

If the tmux session is already missing during an explicit stop, treat
stop as idempotent: clean up helper/FIFO state, mark stopped, and include
the failure class as a note rather than refusing.

## 7. Helper-side mechanics

`supervisor/helper.go::RunHelper` and `pty.go::launchPTY` are reshaped
together. The helper learns pane identity at launch and learns to
distinguish attach-client exit from real lane exit.

### 7.1 `agent_started` payload

`agentPIDFromEvents` (in `mutations/supervision_control.go`) already
reads `payload.pid`, so this slot continues to hold the **supervised**
pid; attach is metadata-only. `tmuxMetadataFromHelperEvents` is extended
(not renamed; backwards-compat in §9.3) to read pane fields directly.

```jsonc
{
  "schema_version": "striatum.supervisor_helper.event.v1",
  "event_type":     "agent_started",
  "supervisor_id":  "sup_...",
  "payload": {
    "pid":               48211,         // pane_pid (the lane)
    "attach_client_pid": 48309,         // diagnostic
    "metadata": {
      "tmux": {
        "state":            "backed",
        "session_name":     "...",
        "window_id":        "@3",
        "pane_id":          "%4",
        "pane_pid":         48211,
        "pane_start_token": "1748452211",
        "attach_command":   "tmux attach-session -t ...",
        "captured_at":      "2026-05-28T15:42:09Z"
      }
    }
  }
}
```

### 7.2 Attach-client exit: `attached → detached`

When `result.Cmd.Wait()` returns (attach client exited), the helper
probes tmux (using the same `ProbeTmuxLiveness` codepath against the
captured identity):

- **Pane alive (`tmux_ok`)**: emit a new event
  `attach_client_exited` carrying `attach_client_pid` and `pane_pid`,
  then exit cleanly with code 0. The daemon recognises this event and
  moves the pointer state from `attached` to `detached` (the state
  already exists in `supervisorActiveStatesRead`). `supervise.send` then
  rejects with `needs_reattach` (not `pid_gone`). The supervisor row is
  **not** marked `lost`. The lane keeps running inside tmux; the operator
  can still `tmux attach-session` to watch it.
- **Pane dead/missing/mismatched**: emit `agent_exited` with the pane
  class (`tmux_pane_dead`, `tmux_session_missing`, etc.) as the cause.
  This is the real "the lane finished" path.

**No self-healing in Phase 1.** AGY's design proposed automatically
relaunching `tmux attach-session` from inside the helper when the
observer exits. That is rejected here because it (a) creates an opaque
new attach process whose stdout/stderr the daemon does not own, (b)
opens a TOCTOU window between probe and relaunch, and (c) makes
`stdin_pipe_path` cleanup hairy. Re-establishing the daemon's PTY master
against a still-live tmux session is properly the job of a future
`striatum supervise reattach` verb (RFC 0089 Phase 2, §9.2 below).

### 7.3 `LaunchResult` and packet delivery

`forwardPacketStream` writes to `StdinWriter` (the attach PTY master) as
today. That path is unchanged in Phase 1; the attach client is still
the byte sink for packet bytes. The byte path becomes unusable when
`StdinWriter` is closed (attach exit); that is exactly the
`needs_reattach` condition surfaced in §7.2.

Codex's design proposed switching to `tmux load-buffer` /
`tmux paste-buffer` for packet delivery so the attach client is never
needed for byte transport. That is rejected for Phase 1 because:

1. Buffer-paste into a TUI's input editor is provider-specific. The
   Claude TUI and Codex TUI handle large pastes differently; agy
   does not run in tmux at all in this workflow. A buffer-paste
   regression would land an RFC 0089 blocker on top of an RFC 0088
   blocker.
2. The current PTY master path already works for delivery. The bug we
   are fixing is liveness, not byte transport.
3. Phase 2's reattach verb naturally supplies a fresh PTY master against
   the live tmux session, restoring the existing delivery path without
   a new substrate.

Buffer-paste delivery may return as part of RFC 0089 Phase 2 or a
follow-up RFC; it is explicitly **out of scope** for Phase 1.

## 8. Failure classes

Exactly the set RFC 0089 §2 specifies, with one internal lost-reason
sub-flavour for persistent unavailability:

| Class                         | Cause                                                                  | Treatment                                                                                                                                                  |
| ----------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `tmux_ok`                     | session alive, pane alive, pid + start token match                     | `running` / `alive` / `attested`                                                                                                                           |
| `tmux_session_missing`        | `tmux has-session` non-zero                                            | `lost(reason="tmux_session_missing")`                                                                                                                      |
| `tmux_pane_missing`           | `display-message` failed or pane_id no longer matches                  | `lost(reason="tmux_pane_missing")`                                                                                                                         |
| `tmux_pane_dead`              | pane exists, `pane_dead == 1`                                          | `lost(reason="tmux_pane_dead")`                                                                                                                            |
| `tmux_pane_pid_mismatch`      | pane alive but pid or start token differs                              | `lost(reason="tmux_pane_pid_mismatch")` (mirrors D080)                                                                                                     |
| `tmux_unavailable`            | tmux not on PATH, exec failed, probe timeout                           | heartbeat: skip + bump skip counter; delivery: refuse `invalid_transition`; status: `tmux.liveness.class = "tmux_unavailable"`                              |
| `tmux_unavailable_persistent` | derived: 3 consecutive unavailable heartbeat ticks                     | heartbeat: `lost(reason="tmux_unavailable_persistent")`. Surfaced as the `lost_reason` string only; not a first-class probe class.                          |

Constants live in `supervisor.TmuxLiveness*` and are re-exported from the
read package so the read projection does not re-stringify them.

## 9. Rollback, fallback, opt-out

### 9.1 `STRIATUM_TMUX_PROBE_DISABLE=1`

A daemon-process env var read by `ProbeLaneLiveness`. When set, the
probe always falls through to the pid+start-token path even on
tmux-backed lanes — production collapses to RFC 0089 P0. Documented in
`docs/how-to/daemon-runbook.md`. Operators can flip it from a systemd
override without code changes if the new probe misbehaves.

This is **not** a workflow knob. Lane authors must not gate product
behaviour at runtime.

### 9.2 Non-tmux fallback

`launchPlainPTY` is untouched. Its `Metadata.tmux` is set to
`{"state": "unavailable", "unavailable_reason": "<reason>"}` so the read
projection still includes a `tmux` block carrying the existing
remediation strings. `ProbeLaneLiveness` sees a non-`backed` state and
falls through to `pidLiveWithStartToken`.

Workflows with `supervision.require_tmux: true` (the codex agent-loop
lane) keep failing closed via the existing `tmuxRequiredError` check;
that codepath is unchanged.

### 9.3 Backwards compatibility

- The read-side accessor `reads/supervision.go::tmuxMetadata` (and its
  callers) read the existing `tmux.session_name` and
  `tmux.attach_command` keys unchanged. The new fields
  (`tmux.state`, `tmux.window_id`, `tmux.pane_id`, `tmux.pane_pid`,
  `tmux.pane_start_token`, `tmux.attach_client_pid`, `tmux.liveness`)
  are additive. Until the daemon writes the new fields, the read
  projection looks indistinguishable from today (no `tmux.liveness`;
  top-level `liveness` driven by the pid probe).
- `process_supervisors` and `process_supervisor_pointers` columns are
  unchanged; only the `metadata_json` payload grows. No migration.
- `striatum supervise status` JSON gains the optional `tmux.liveness`
  object. Existing top-level fields keep their meanings. No CLI breaking
  change.
- Helper event `attach_client_exited` is a new event type. Older daemon
  builds that do not recognise it will fall back to the existing
  `agent_exited` handling — i.e. they regress to today's "attach exit
  marks the lane lost". The daemon must land before any lane is started
  by a helper built to emit the new event. The implementer documents
  this ordering in the handoff.

## 10. Tests

All tests live next to the code they cover and gate on
`exec.LookPath("tmux")` so CI without tmux skips with a clear reason
rather than silently mis-passing.

### 10.1 `go/pkg/supervisor/tmux_liveness_test.go` (unit, fake runner)

- `TestProbeOK` — runner returns expected `has-session` ok +
  matching `display-message`; expect `TmuxLivenessOK`.
- `TestProbeSessionMissing` — `has-session` non-zero; expect
  `TmuxLivenessSessionMissing`.
- `TestProbePaneMissing` — `has-session` ok; `display-message` returns
  wrong pane_id; expect `TmuxLivenessPaneMissing`.
- `TestProbePaneDead` — `display-message` returns `pane_dead=1`; expect
  `TmuxLivenessPaneDead`.
- `TestProbePanePIDMismatch` — `display-message` returns a different
  `pane_pid`; expect `TmuxLivenessPanePIDMismatch`.
- `TestProbePaneStartTokenMismatch` — pane_pid matches but
  `pane_start_time` differs from the captured token; expect
  `TmuxLivenessPanePIDMismatch`.
- `TestProbeStartTokenEmptyIsUnverifiedNotLost` — captured token empty,
  observed token present; expect `TmuxLivenessOK` with detail
  `start_token_unverified`.
- `TestProbeTmuxNotFound` — runner returns `exec.ErrNotFound`; expect
  `TmuxLivenessUnavailable` with detail `tmux: command not found`.
- `TestProbeTimeout` — runner returns `context.DeadlineExceeded`; expect
  `TmuxLivenessUnavailable` with detail `probe_timeout`.
- `TestProbeNeverReadsPaneText` — runner records every command issued;
  asserts no `capture-pane`, `pipe-pane`, `save-buffer`,
  `show-buffer`, `copy-mode`, or `select-pane -P` shows up. Structural
  D028 guard.

### 10.2 `go/pkg/supervisor/tmux_liveness_integration_test.go`

Skipped when `exec.LookPath("tmux")` fails.

- `TestIntegrationAttachClientExitDoesNotKillProbe` — start a real
  detached tmux session running `sleep 600`; capture the identity;
  start and immediately kill an attach client; assert
  `ProbeTmuxLiveness` still returns `TmuxLivenessOK`.
- `TestIntegrationSessionKillIsSessionMissing` — start a session,
  capture identity, `tmux kill-session`, assert
  `TmuxLivenessSessionMissing`.
- `TestIntegrationShortSleepIsPaneDeadOrPidMismatch` — start a session
  whose entry command is `sh -c "sleep 0.1; exec sleep 600"`; capture
  identity at `sh` pid; after 1 s expect either
  `TmuxLivenessPaneDead` (if pane is configured to die on entry exit)
  or `TmuxLivenessPanePIDMismatch` (the more likely outcome under
  default remain-on-exit settings). The test asserts the lost set
  rather than a specific class so it is robust across tmux configs.

### 10.3 `go/pkg/supervisor/helper_test.go` (extension)

- `TestRunHelperAttachClientExitWithLivePaneIsNotLost` — fake `Launch`
  returns `Cmd` for an attach surrogate that exits cleanly, and a
  pane-pid surrogate (sleep-forever subprocess) that stays alive.
  Helper emits `attach_client_exited` with `attach_client_pid` and
  `pane_pid`, then exits cleanly. Helper must NOT emit `agent_exited`.
- `TestRunHelperRecordsPanePIDNotAttachPID` — initial `agent_started`
  payload `pid` equals the pane pid; `attach_client_pid` carries the
  attach pid.
- `TestRunHelperPaneDeadEmitsAgentExited` — pane surrogate dies first;
  helper probes, sees pane dead, emits `agent_exited` with
  `cause: "tmux_pane_dead"`.

### 10.4 `go/pkg/supervisor/liveness_test.go` (extension)

- `TestLivenessSurvivesAttachClientExit` — fake `PointerStore` returns
  `metadata.tmux.state = "backed"`; tmux runner stub returns OK for a
  still-alive session. The 5 s tick keeps the row `running` even
  though the old "attach pid" is dead.
- `TestLivenessMarksLostOnPaneDead` — runner returns
  `tmux_pane_dead`; expect `MarkSupervisorLost(reason="tmux_pane_dead")`.
- `TestLivenessTransientTmuxUnavailableDoesNotMarkLost` — runner
  returns `tmux_unavailable` for two consecutive ticks then OK; row
  stays `running`. After three consecutive misses, mark
  `lost(reason="tmux_unavailable_persistent")`.

### 10.5 `go/pkg/mutations/supervision_control_test.go` (extension)

- `TestReconcileForDeliveryUsesTmuxProbeForTmuxBackedLane` — metadata
  `tmux.state = "backed"`; injected probe returns `tmux_pane_missing`.
  `supervise.send` returns `invalid_transition` with detail
  `tmux_pane_missing`; supervisor row goes to `lost`; event payload
  carries `reason: "tmux_pane_missing"`.
- `TestReconcileForDeliveryFallsThroughForPlainPTY` — metadata
  `tmux.state = "unavailable"`; legacy `pidAliveLocal` path is
  exercised unchanged.
- `TestReconcileRefusesOnTmuxUnavailable` — `tmux_unavailable`;
  delivery rejected `invalid_transition: tmux probe unavailable`;
  supervisor row NOT marked lost; event NOT emitted.
- `TestSuperviseStopUsesTmuxKillSession` — metadata `tmux.state =
  "backed"`; mocked tmux runner records `kill-session -t <name>` was
  issued **before** any pid SIGTERM. Response `signal` is
  `tmux_kill_session`.
- `TestSuperviseStopFallsBackToSIGTERMWhenTmuxUnavailable` — runner
  returns `tmux: command not found`. Response `signal` is `SIGTERM`;
  event payload carries `tmux_kill_fallback_reason: "tmux_unavailable"`.
- `TestSuperviseStopIdempotentWhenSessionAlreadyGone` — runner returns
  session-already-gone on `kill-session`; stop succeeds, row marked
  `stopped`, response includes `note: "tmux_session_missing"`.

### 10.6 `go/pkg/reads/supervision_test.go` (extension)

- `TestSuperviseStatusReportsTmuxLivenessClass` — metadata
  `tmux.state = "backed"`; tmux runner injected via
  `reads.SetTmuxRunnerForTest`. Verify read projection includes
  `tmux.liveness.class`, top-level `liveness` is derived correctly, and
  `lane_attestation` is `attested` only when probe is OK.
- `TestSuperviseStatusNeverIncludesPaneText` — assertion that the
  status payload never contains any key matching `capture`, `buffer`,
  `output`, `stdout`, `stderr`, `transcript`. Same shape as the
  existing `TestTmuxMetadataIsAllowlistedForOperatorStatus`, extended
  over the live-probe path.
- `TestReattachStatusViewClassifiesByProbe` — `tmux_ok` ⇒
  `reattachable`; `tmux_session_missing`/`tmux_pane_missing`/
  `tmux_pane_dead`/`tmux_pane_pid_mismatch` ⇒ `lost_candidate`;
  `tmux_unavailable` ⇒ `needs_verification`.

### 10.7 `go/pkg/mutations/recovery_test.go` (extension)

- `TestRecoveryProcessReconcileUsesTmuxProbeForTmuxBackedRows` —
  `process_executions` rows joined to a tmux-backed supervisor. Pane is
  alive in tmux but the old recovery sweep used to flip the row to
  `lost` based on the attach pid being gone. With the redesign the row
  stays `running` and the sweep result reports `still_running`.
- `TestRecoveryProcessReconcileMarksLostOnPaneDead` — runner returns
  `tmux_pane_dead`; sweep flips the row to `lost` with reason
  `tmux_pane_dead`.

### 10.8 D028 guards (extension of existing tests)

Extend `TestTmuxMetadataIsAllowlistedForOperatorStatus` (or whichever
existing test is the structural allowlist guard for the `tmux` block)
to cover the new fields. The allowlist contains exactly:
`state`, `session_name`, `window_id`, `pane_id`, `pane_pid`,
`pane_start_token`, `attach_command`, `attach_client_pid`,
`captured_at`, `liveness` (sub-keys: `class`, `healthy`,
`observed_pane_pid`, `observed_pane_start`, `detail`), `remediation`,
`unavailable_reason`. Anything else in the metadata is dropped before
emission.

### 10.9 Verification commands

Per TASK.md:

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

## 11. Fixtures

The fake `TmuxRunner` lives in `go/pkg/supervisor/tmux_liveness_test.go`
as `fakeTmuxRunner`:

- records every `(args, ctx)` pair;
- returns a programmed `(stdout, err)` per matching `args` prefix;
- defaults to "command not found" if no programmed response matches.

Probe-output fixtures are inlined in tests, not on disk; the
`display-message` output is a five-byte-or-so pipe-delimited string and
does not warrant a fixture file.

The integration tests in §10.2 use `t.TempDir()` for any working
directory and a unique `sessionName` per test (UUID prefix) so parallel
runs do not collide. They `t.Cleanup` `tmux kill-session` to avoid
leaking sessions on failure.

## 12. Doc updates

| File                                                | Change                                                                                                                                                                                                |
| --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/reference/spec.md` (Process Supervision)      | Define the extended `tmux` metadata block (`state`, the seven new fields, `liveness` sub-block) and the six `TmuxLiveness*` classes (plus the derived `tmux_unavailable_persistent` lost-reason).      |
| `docs/decisions/decision-log.md`                    | Add `D152 — RFC 0089 Phase 1 lands tmux session/pane liveness…` paragraph (text per RFC 0089 §"Proposed Decision-Log Entry"), status `accepted` when the implementation lands.                        |
| `docs/reference/ubiquitous-language.md`             | New entry `tmux lane` (the pane-pid-identified supervised process) distinct from `attach client` (the transient `tmux attach-session` PTY writer that is now observer-only for liveness purposes).    |
| `docs/how-to/daemon-runbook.md`                     | New "Tmux liveness probe" section: the seven classes, what `tmux_probe_unavailable` means in status, the `STRIATUM_TMUX_PROBE_DISABLE=1` rollback knob, and which logs to consult.                     |
| `docs/reference/command-authority-matrix.md`        | No new RPC methods. Annotate `supervise.status`, `supervise.stop`, `recovery.process_reconcile`, and `supervise.reattach_status` rows with "consults tmux liveness probe (RFC 0089 P1)".               |
| `CHANGELOG.md` (Unreleased)                         | "RFC 0089 P1: tmux-backed lanes track liveness from tmux session/pane identity, not the transient `tmux attach-session` client."                                                                       |
| `docs/rfcs/0089-tmux-backed-lane-monitoring.md`     | Status `proposed → accepted` after the workflow accepts the design. Flagged for the synthesizer / coordinator; not a synthesis-time edit.                                                              |

No glossary deletions are required.

## 13. Out of scope for Phase 1

These are explicitly deferred and the implementer should not land them in
the Phase 1 slice:

- A `striatum supervise reattach` verb that re-establishes the daemon's
  PTY master against a still-running tmux session without re-running the
  lane command. RFC 0089 Phase 2.
- Self-healing attach inside `RunHelper` (AGY's design proposal).
- `tmux load-buffer` / `tmux paste-buffer` packet delivery (Codex's
  design proposal). Possibly a future RFC.
- Flipping agent-loop lanes to tmux-backed **by default** when tmux is
  installed. RFC 0089 Phase 3.
- Single tmux server per run vs server per supervisor consolidation.
  RFC 0089 §"Open Questions"; non-blocking.
- Any redaction or export pipeline change. PTY logs under
  `.striatum/scratch/<supervisor_id>/pty.log` remain D028 / D151 private
  diagnostics.
- Any new RPC verb. Phase 1 is implemented by extending existing
  handlers; no `command-authority-matrix.md` row is added.

## 14. Acceptance hookup

Mapping back to RFC 0089 §"Acceptance Criteria":

| RFC line                                                                  | Where it lands                                                                                                                  |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| attach exit ⇒ lane stays attached/attested                                 | §5.1 + §5.2 + §7.2; tests §10.3 `TestRunHelperAttachClientExit…` + §10.4 `TestLivenessSurvivesAttachClientExit`                  |
| killing session/pane ⇒ structured lost state before next delivery          | §5.1 + §5.2; tests §10.4 `TestLivenessMarksLostOnPaneDead`, §10.5 `TestReconcileForDeliveryUsesTmuxProbe…`                       |
| `supervise.status --json` exposes session/pane/pid/attach/class            | §3.1 + §5.3; test §10.6 `TestSuperviseStatusReportsTmuxLivenessClass`                                                            |
| doctor / dashboard / recovery report same classes without pane text        | §5.4 + §5.5; tests §10.1 `TestProbeNeverReadsPaneText`, §10.6 `TestSuperviseStatusNeverIncludesPaneText`                         |
| `supervise.stop` terminates the tmux lane via the session                  | §6; tests §10.5 `TestSuperviseStopUsesTmuxKillSession` + `TestSuperviseStopFallsBackToSIGTERMWhenTmuxUnavailable`                |
| live agent-loop lane can be `tmux attach`'d while still working via MCP    | structural — already true once attach exits do not cascade; documented via the implementer's live-attach run                    |
| regression: attach-client exit does not mark supervisor lost                | §10.4 `TestLivenessSurvivesAttachClientExit`                                                                                    |
| regression: missing session/pane prevents delivery with structured failure  | §10.5 `TestReconcileForDeliveryUsesTmuxProbeForTmuxBackedLane`                                                                  |
| D028 guard: pane text never enters state / exports                          | §10.1 `TestProbeNeverReadsPaneText`, §10.6 `TestSuperviseStatusNeverIncludesPaneText`, §10.8 allowlist guard                    |

The implementer's handoff statement at the end of the slice should be:
*"After RFC 0089 Phase 1, RFC 0088 agent-loop lanes can be flipped to
tmux-backed by default with a workflow.json / daemon-config change
alone; no further code change is required."*
