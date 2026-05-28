# RFC 0089 Phase 1 — tmux helper redesign (claude_code design)

author: designer-claude-opus-4.7-001

## 1. Premise

`go/pkg/supervisor/pty.go::launchPTY` creates a detached tmux session that
runs the real lane command, then spawns a *second* process,
`tmux attach-session -t <session>`, under a creack/pty PTY. The attach
process gives the daemon a PTY master to write packets through. Today the
attach process pid is what `launchPTY` returns as `LaunchResult.PID`, and that
attach pid is what the rest of the codebase treats as the supervised lane
identity:

- `mutations/supervision_control.go::HandleSuperviseStart` writes the attach
  pid into `striatumd.process_supervisors.pid` and into the pointer row.
- `mutations/supervision_control.go::reconcileSupervisorForDelivery` calls
  `pidAliveLocal(supervisor.PID)` before every `supervise.send`.
- `reads/supervision.go::HandleSuperviseStatus` calls
  `pidLiveWithStartToken(pid, …)`, which signals-0 the attach pid.
- `supervisor/liveness.go::Liveness.run` polls
  `processAliveAtStartTime(l.pid, row.StartedAt)` every 5 s and marks the
  supervisor `lost` on failure.
- `supervisor/helper.go::RunHelper` waits on `result.Cmd` (the attach
  client) and emits `helper_event=agent_exited` when it returns.
- `mutations/supervision_control.go::HandleSuperviseStop` SIGTERMs
  `supervisor.PID` — i.e. the attach client — and never touches the tmux
  session.
- `mutations/recovery.go::HandleRecoveryProcessReconcile` flips
  `process_executions.state` to `lost` when `pidAlive(pid)` is false.

A `tmux attach-session` client can exit (operator hits `Ctrl-b d`, terminal
hangup, helper SIGTERM, EIO on PTY master) while the underlying tmux
session and the real lane process keep running. Under the current code that
attach exit propagates as `agent_exited`, `supervisor_lost`,
`lane_attestation = unattested`, `recovery.process_reconcile` flipping the
process to `lost`, etc., and any later `supervise.send` is refused with
`pid_gone`. RFC 0089 acceptance test #1 ("attach client exit must not mark
the lane lost") is therefore impossible without the redesign below.

Phase 1's job is to split *lane identity* from *attach-client lifecycle*
and to route every existing supervisor liveness check through a tmux-aware
probe when the supervisor is tmux-backed. No new RPC verbs are added,
no schema migration is needed, and pane text never enters
state/provenance/exports (D028 / D151 unchanged).

## 2. Data captured at launch

### 2.1 New supervisor metadata block

Replace the current `tmux_attach_metadata` shape (`session_name`,
`window_id`, `pane_id`, `attach_command`) with an explicit `tmux_lane`
block on the pointer's `metadata_json`:

```jsonc
"tmux_lane": {
  "transport_kind":   "tmux",              // "tmux" | "plain_pty" | "pipe"
  "session_name":     "striatum-…",
  "window_id":        "@3",                // tmux #{window_id}
  "pane_id":          "%4",                // tmux #{pane_id}
  "pane_pid":         48211,               // tmux #{pane_pid} at launch
  "pane_start_token": "1748452211",        // tmux #{pane_start_time} OR processStartToken(pane_pid); whichever lands first
  "attach_command":   "tmux attach-session -t striatum-…",
  "attach_client_pid": 48309,              // diagnostic only; never the supervised identity
  "captured_at":      "2026-05-28T15:42:09Z"
}
```

`unavailable_reason` stays where it is, but moves under `tmux_lane` (no
sibling `tmux: {}` block). The read projection alias `tmux:` in
`reads/supervision.go::attachSupervisorTmux` keeps existing JSON shape
backwards compatible by reading from `tmux_lane` first and falling back to
the legacy `tmux` key during the migration window.

The `transport_kind` field is the discriminator every downstream probe
keys on. `tmux` ⇒ tmux probe; `plain_pty` ⇒ pid+start-token probe (today's
behaviour); `pipe` ⇒ pid probe.

### 2.2 Supervised pid swap

`launchPTY` is reshaped to return the **pane pid** as `LaunchResult.PID`,
not the attach client pid. Concretely, the new shape is:

```go
type LaunchResult struct {
    PID         int             // pane_pid (the real lane process)
    StdinWriter io.WriteCloser  // attach PTY master, unchanged
    Cmd         *exec.Cmd       // attach client *exec.Cmd, only used to write stdin / close PTY
    AttachPID   int             // attach client pid; metadata only
    Metadata    map[string]any  // includes the "tmux_lane" object above for tmux-backed launches
}
```

The supervisor helper (`supervisor/helper.go::RunHelper`) emits the
`agent_started` event with `pid = pane_pid`, and the daemon records that
pane_pid in `striatumd.process_supervisors.pid` and the pointer row. The
existing `pid_start_time` column is filled from the tmux `pane_start_time`
when available, otherwise from `processStartToken(pane_pid)`, matching the
D080 attestation contract: an owned long-lived process pid + a start-time
binding. No schema change is required — the columns already exist.

### 2.3 How pane identity is captured

In `launchPTY`, immediately after the existing `tmux new-session -d …`
succeeds and before the attach is spawned:

```go
out, err := exec.Command("tmux", "display-message", "-p",
    "-t", sessionName,
    "#{window_id}|#{pane_id}|#{pane_pid}|#{pane_start_time}",
).Output()
```

Parse the four pipe-delimited fields and capture them in a new
`tmuxIdentity` struct (see §3). If `display-message` fails or any of the
three required fields is empty (`window_id`, `pane_id`, `pane_pid`), the
launch returns `tmux_identity_capture_failed` — i.e. we *do not* fall
through to "treat the attach pid as identity"; we tear down the half-built
session, then `RequireTmux ? fail-closed : retry-as-plain-PTY`.

`pane_start_time` is best-effort: some tmux builds (≤ 2.9) lack it. When
empty, fall back to `processStartToken(pane_pid)` from
`supervisor/start_time_linux.go`. The downstream probe treats an empty
start token as `pid_identity_unverified`, matching today's
`pid_identity_unavailable` semantics in `reads/supervision.go`.

## 3. Liveness probe API

New file `go/pkg/supervisor/tmux_liveness.go` (no other package
depends on tmux execv today — keep it isolated from `pty.go` so it is
trivially unit-testable with a fake `tmuxRunner`).

```go
type TmuxIdentity struct {
    SessionName     string
    WindowID        string
    PaneID          string
    PanePID         int
    PaneStartToken  string  // "" ⇒ best-effort verify only
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
    Class             TmuxLivenessClass
    Healthy           bool   // true iff Class == TmuxLivenessOK
    ObservedPanePID   int    // 0 when unknown
    ObservedStartTok  string // "" when unknown
    Detail            string // short non-secret diagnostic; never pane text
}

type TmuxRunner interface {
    // Run executes one tmux command with the given args and returns its
    // stdout. The implementation MUST capture stderr separately and surface
    // it via err.Error() — pane text never reaches a TmuxRunner caller.
    Run(ctx context.Context, args ...string) (string, error)
}

func ProbeTmuxLiveness(ctx context.Context, r TmuxRunner, id TmuxIdentity) TmuxLiveness
```

`ProbeTmuxLiveness` is purely a metadata probe; the only tmux commands it
runs are `has-session`, `display-message -p`, and `list-panes` (all return
ID/integer/format-token-only output). The implementation is:

1. `tmux has-session -t <SessionName>` → exit 0 ⇒ continue; non-zero
   ⇒ `tmux_session_missing`. tmux not on PATH or invocation error
   ⇒ `tmux_unavailable` (`Detail: "tmux: command not found"` or
   exec error message).
2. `tmux display-message -p -t <PaneID> '#{pane_id}|#{pane_pid}|#{pane_dead}|#{pane_start_time}'`
   →
   - command failure ⇒ `tmux_pane_missing`;
   - returned `pane_id` differs from `id.PaneID` ⇒ `tmux_pane_missing` (tmux
     reused the slot under a different pane);
   - `pane_dead == "1"` ⇒ `tmux_pane_dead`;
   - `pane_pid != id.PanePID` ⇒ `tmux_pane_pid_mismatch` (lane process exited
     and tmux re-ran something else under the pane; equivalent to D080
     "pid identity mismatch");
   - `id.PaneStartToken != ""` and `pane_start_time != id.PaneStartToken`
     ⇒ `tmux_pane_pid_mismatch` (same pid was reused with a new start time).
3. Otherwise ⇒ `TmuxLivenessOK`, with `ObservedPanePID` and
   `ObservedStartTok` populated for the caller's read projection.

Two cheap derived helpers:

- `func (l TmuxLiveness) Lost() bool` — true for everything except
  `TmuxLivenessOK` and `TmuxLivenessUnavailable`; the latter is a probe
  outage, not a lane outage, and is handled by the caller (§4) without
  marking the supervisor lost.
- `func (l TmuxLiveness) AsRead() map[string]any` — returns the read
  projection block used in `supervise.status` and `dashboard` (`class`,
  `healthy`, `observed_pane_pid`, `observed_pane_start_token`, `detail`).

Implementation notes:

- The default `TmuxRunner` is `execTmuxRunner{}` calling
  `exec.CommandContext("tmux", args...)` with a 2 s timeout per call
  (configurable via `STRIATUM_TMUX_PROBE_TIMEOUT`). Probe failures from
  ctx cancellation map to `tmux_unavailable` with detail `probe_timeout`.
- The probe never reads pane content (`capture-pane`, `pipe-pane`, etc.).
  This is a structural D028 guarantee, asserted in tests (§6.5).
- For unit tests, `tmuxRunner` is injectable. Stdlib `*exec.Cmd` is wrapped
  by an internal `defaultTmuxRunner` so production code remains a thin
  shim around `exec.Command` exactly the way the current
  `tmuxAttachMetadata` is.

## 4. Where the probe is called

Five callsites switch from "signal-0 the supervisor pid" to "if
tmux-backed, run the tmux probe; otherwise keep the existing pid probe".
All five share one helper, added to
`go/pkg/supervisor/tmux_liveness.go`:

```go
// LaneLiveness routes a liveness check through tmux when the supervisor's
// pointer metadata says transport_kind == "tmux", and through the existing
// pid+start-time probe otherwise. It returns a uniform LaneLiveness value
// so callers do not branch on transport_kind themselves.
type LaneLiveness struct {
    Backed      string         // "tmux" | "plain_pty" | "pipe"
    Alive       bool
    Class       string         // tmux class OR pid class ("pid_ok" / "pid_gone" / "pid_identity_mismatch" / "pid_identity_unavailable")
    Tmux        *TmuxLiveness  // nil when Backed != "tmux"
    ObservedPID int            // 0 unless we observed one
    Detail      string
}

func ProbeLaneLiveness(ctx context.Context, r TmuxRunner,
    metadata map[string]any, pid int, expectedStartToken string) LaneLiveness
```

`ProbeLaneLiveness` extracts `transport_kind` from
`metadata["tmux_lane"]`. The five callsites are:

### 4.1 `supervisor/liveness.go::Liveness.run`

Today: ticks every 5 s, calls `processAliveAtStartTime(l.pid, row.StartedAt)`,
on `false` calls `MarkSupervisorLost(reason="process_exited")`. Replace
with `ProbeLaneLiveness`, passing the pointer metadata loaded from the
existing `l.store.GetSupervisorPointer` call. The store interface gains
`Metadata() map[string]any` (today the PointerRow shape does not carry
the metadata blob; add a `Metadata map[string]any` field).

- `Class == TmuxLivenessOK` or `pid_ok` ⇒ `row.State = "running"` (today's
  branch).
- `Class == TmuxLivenessUnavailable` (tmux not runnable) ⇒ leave state
  alone, do **not** mark lost. Bump a `tmux_probe_skipped_at` timestamp on
  the row so the operator can see the probe outage. After three
  consecutive unavailable ticks (i.e. ~15 s) we *do* mark
  `lost(reason="tmux_unavailable_persistent")` — that protects us from a
  partially-rotting daemon environment without flapping on a single
  transient miss.
- Any other non-OK class ⇒ `MarkSupervisorLost(reason=string(Class))`.

### 4.2 `mutations/supervision_control.go::reconcileSupervisorForDelivery`

Today: calls `pidAliveLocal(supervisor.PID)` and then
`processStartToken(supervisor.PID) != supervisor.PIDStartTime`. Replace
with one `ProbeLaneLiveness` call. The existing `markSupervisorLostInTx`
takes the tmux class as the reason (so the event payload carries
`reason: "tmux_pane_pid_mismatch"` etc.). The lease-error and
pointer-reconciliation branches are unchanged.

`tmux_unavailable` here is **not** silently retried (unlike §4.1): a
delivery requires us to know the lane is live. Treat
`tmux_unavailable` at delivery time as `invalid_transition: tmux probe
unavailable; cannot verify lane`, and surface the diagnostic detail (e.g.
`tmux: command not found`) so the operator can fix the daemon environment
rather than re-issue blindly.

### 4.3 `reads/supervision.go::HandleSuperviseStatus`

Today: `pidLiveWithStartToken(pid, expected)` → `liveness = "alive" | "gone"`
and the `lane_attestation` derivation. Add a `tmux` block to the read
projection:

```jsonc
"tmux": {
  "transport_kind":   "tmux",
  "session_name":     "striatum-…",
  "pane_id":          "%4",
  "attach_command":   "tmux attach-session -t striatum-…",
  "state":            "attachable",       // "attachable" | "unavailable"
  "liveness": {
    "class":               "tmux_ok",
    "healthy":             true,
    "observed_pane_pid":   48211,
    "observed_pane_start": "1748452211",
    "detail":              null
  },
  "remediation": null
}
```

`liveness == "alive"` iff `LaneLiveness.Alive` (works for both
tmux-backed and plain-PTY lanes). The `tmux.liveness` block is the
*structured* surface; the top-level `liveness` field stays
`"alive" | "stalled" | "gone"` for back-compat with operator scripts.
`lane_attestation` keeps its current derivation (attested iff `attached`,
`alive`, and not flagged for reattach repair), so RFC 0088 byline rules
do not change shape.

### 4.4 `mutations/recovery.go::HandleRecoveryProcessReconcile`

Today: `pidAlive(pid)` over `process_executions` rows. The
`process_executions` table is the *per-packet* process record (separate
from `process_supervisors`), so the probe needs the supervisor metadata.
Join `process_executions` to `process_supervisor_pointers` on
`supervisor_id` (the column already exists), pull `metadata_json`, run
`ProbeLaneLiveness`, treat `Class == "tmux_pane_pid_mismatch"` and
`Class == "tmux_pane_dead"` as `lost`. Keep the existing path-only check
for the non-tmux rows.

### 4.5 `cmd/striatum/doctor.go` (`doctor --verbose`)

Today's doctor output, per `reads/doctor.go`, surfaces supervisor rows
verbatim. Extend the per-supervisor section to render
`tmux.liveness.class` and the static remediation hint when the probe
returned `tmux_unavailable`. Doctor stays a read; it does not call
`markSupervisorLost`.

Dashboard/`dashboard_all`/`status` already pass `tmux` through via
`attachSupervisorTmux`; once the read projection adds `tmux.liveness`
they pick it up automatically.

## 5. `supervise.stop` rework

`HandleSuperviseStop` (`mutations/supervision_control.go:320`) is rewired
so the *lane*, not the *attach client*, is terminated. New sequence:

```go
// 1. Resolve tmux identity from pointer metadata.
tmux := tmuxLaneFromMetadata(supervisor.Metadata)

// 2. Drain helper events (unchanged).

// 3a. If tmux-backed: kill the tmux session FIRST. That terminates the
//     pane process (the real lane) and incidentally tears down any
//     attach clients still hanging on the PTY.
if tmux != nil && tmux.TransportKind == "tmux" {
    signaled = tmuxKillSession(tmux.SessionName) // "tmux_kill_session" string sentinel
}

// 3b. If tmux is unavailable mid-stop (operator removed it, etc.),
//     fall back to SIGTERMing pane_pid directly. Record the fallback
//     reason in the supervisor.stopped event payload so the operator
//     can see we did not kill via tmux.

// 4. Always also terminateProcess(attach_client_pid) and helper_pid as
//    a belt-and-suspenders cleanup. With the session gone they will
//    already be exiting, but the call is idempotent.

// 5. The stdin pipe path cleanup (os.Remove(StdinPipePath)) is unchanged.
```

`tmuxKillSession(name)` is a wrapper around
`tmux kill-session -t <name>` with a 2 s timeout. A non-zero exit because
the session is already gone is treated as success.

The returned `signal` field in the JSON response gains a new symbolic
value `tmux_kill_session` distinct from `SIGTERM` / `SIGKILL` so
operators/scripts can tell which path was taken. `supervise.list` and
`dashboard` need no change here — they already render the `stop_reason`
string verbatim.

## 6. Tests

All tests live next to the code they cover, follow the existing
`go test ./go/...` pattern, and gate on `exec.LookPath("tmux")` so CI
without tmux skips with a clear reason (`t.Skip("tmux not installed")`)
rather than silently mis-passing.

### 6.1 `go/pkg/supervisor/tmux_liveness_test.go`

Pure unit tests against the injected `TmuxRunner`. No real tmux.

- `TestProbeOK` — runner returns expected has-session ok + matching
  display-message; expect `TmuxLivenessOK`.
- `TestProbeSessionMissing` — runner returns non-zero on `has-session`;
  expect `TmuxLivenessSessionMissing`.
- `TestProbePaneMissing` — has-session ok; display-message returns the
  wrong pane_id; expect `TmuxLivenessPaneMissing`.
- `TestProbePaneDead` — display-message returns `pane_dead=1`; expect
  `TmuxLivenessPaneDead`.
- `TestProbePanePIDMismatch` — display-message returns a different
  pane_pid; expect `TmuxLivenessPanePIDMismatch`.
- `TestProbePaneStartTokenMismatch` — pane_pid matches but
  pane_start_time differs from the captured token; expect
  `TmuxLivenessPanePIDMismatch`.
- `TestProbeStartTokenEmptyIsUnverifiedNotLost` — captured token empty,
  observed token present; expect `TmuxLivenessOK` with detail
  `start_token_unverified` (the lane is not lost just because tmux
  build doesn't expose `pane_start_time`).
- `TestProbeTmuxNotFound` — runner returns exec.ErrNotFound; expect
  `TmuxLivenessUnavailable` with detail `tmux: command not found`.
- `TestProbeNeverReadsPaneText` — runner records every command issued;
  asserts no `capture-pane`, `pipe-pane`, `save-buffer`, `show-buffer`,
  `copy-mode`, or `select-pane -P` shows up. This is the D028 structural
  guard.

### 6.2 `go/pkg/supervisor/helper_test.go` (extension)

- `TestRunHelperAttachClientExitWithLivePaneIsNotLost` — fake `Launch`
  returns `Cmd` for an attach surrogate that exits cleanly, and a
  pane-pid surrogate (a sleep-forever subprocess) that stays alive.
  Helper must NOT emit `agent_exited`; it must instead keep emitting
  probe-OK ticks until the pane surrogate dies. Then it emits
  `agent_exited` exactly once with the pane pid in the payload, not the
  attach pid. (See §7 for the helper-side restart behaviour.)
- `TestRunHelperRecordsPanePIDNotAttachPID` — the initial
  `agent_started` event payload `pid` field equals the pane pid; an
  `attach_client_pid` metadata field carries the attach pid.

### 6.3 `go/pkg/supervisor/supervisor_test.go` (extension)

- `TestLivenessSurvivesAttachClientExit` — using the fake
  `PointerStore`, set `metadata = {"tmux_lane":{"transport_kind":"tmux",…}}`,
  point the tmux runner stub at a still-alive session. Even when the
  attach client (the row's old pid) is killed, the heartbeat goroutine
  keeps the row `running` and never calls `MarkSupervisorLost`.
- `TestLivenessMarksLostOnPaneDead` — same setup, runner now returns
  `tmux_pane_dead`; expect `MarkSupervisorLost` with reason
  `tmux_pane_dead`.
- `TestLivenessTransientTmuxUnavailableDoesNotMarkLost` — runner
  returns `tmux_unavailable` for two consecutive ticks then OK; row stays
  `running`. After three consecutive misses, mark lost with reason
  `tmux_unavailable_persistent`.

### 6.4 `go/pkg/mutations/supervision_control_test.go` (extension)

- `TestReconcileForDeliveryUsesTmuxProbeForTmuxBackedLane` — pointer
  metadata `transport_kind = "tmux"`; tmux probe injected. Probe says
  `tmux_pane_missing` → `supervise.send` returns
  `invalid_transition: supervisor cannot accept delivery: tmux_pane_missing`,
  the supervisor row goes to `lost`, and an event with payload
  `reason: "tmux_pane_missing"` is appended.
- `TestReconcileForDeliveryFallsThroughForPlainPTY` — metadata says
  `transport_kind = "plain_pty"`; verifies the legacy
  `pidAliveLocal` path is still exercised, unchanged.
- `TestSuperviseStopUsesTmuxKillSession` — metadata says tmux; mocked
  tmux runner records `kill-session -t <name>` was issued *before* any
  pid SIGTERM. `signal` field in the response is `tmux_kill_session`.
- `TestSuperviseStopFallsBackToSIGTERMWhenTmuxUnavailable` — tmux runner
  returns `tmux: command not found`. Response `signal` is `SIGTERM`,
  event payload carries `tmux_kill_fallback_reason: "tmux_unavailable"`.

### 6.5 `go/pkg/reads/supervision_test.go` (extension)

- `TestSuperviseStatusReportsTmuxLivenessClass` — pointer metadata
  `transport_kind = "tmux"`, tmux runner injected (via an exported
  package-level hook `reads.SetTmuxRunnerForTest`). Verify the read
  projection includes `tmux.liveness.class`, top-level `liveness` is
  derived correctly, and `lane_attestation` is `attested` only when
  probe is OK.
- `TestSuperviseStatusNeverIncludesPaneText` — assertion that the
  status payload never contains any key matching `capture`, `buffer`,
  `output`, `stdout`, `stderr`, `transcript`, etc. — same shape as the
  existing `TestTmuxMetadataIsAllowlistedForOperatorStatus` test, just
  extended over the live-probe path.

### 6.6 `go/pkg/mutations/recovery_test.go` (extension)

- `TestRecoveryProcessReconcileUsesTmuxProbeForTmuxBackedRows` — running
  `process_executions` rows joined to a tmux-backed supervisor. The
  pane process is still alive in tmux but the recovery sweep used to
  flip the row to `lost` based on the attach pid being gone. With the
  redesign the row stays `running` and the sweep result reports
  `still_running` for it.

## 7. Helper-side mechanics

`supervisor/helper.go::RunHelper` and `pty.go::launchPTY` are reshaped
together because the helper has to know the pane identity in order to
report `pid = pane_pid` and decide whether to re-establish a fresh
attach client when the existing one exits.

### 7.1 Attach client restart

`forwardPacketStream` writes to the PTY master returned by
`pty.Start(attachCmd)`. When the attach client exits but the pane is
still alive, the existing PTY master is closed (EIO on next write), so
the next `supervise.send` would lose bytes. RFC 0089 Phase 1 keeps the
existing behaviour for now (the helper exits, the daemon restarts
the supervisor via the reattach pathway), with the **important caveat**
that exit no longer cascades to `supervisor.lost`. Concretely:

- The helper's `result.Cmd.Wait()` goroutine, on attach client exit,
  probes tmux. If pane is alive, the helper emits a new event
  `attach_client_exited` (carrying `attach_client_pid` and `pane_pid`)
  and then exits cleanly with code 0. The daemon distinguishes this from
  `agent_exited` and from helper crash, and treats it as "supervisor
  detached — needs reattach", not "supervisor lost". The pointer row's
  state moves from `attached` to `detached` (the state already exists
  in `supervisorActiveStatesRead`), and `supervise.send` rejects with
  `needs_reattach` rather than `pid_gone`.
- If pane is dead/missing/mismatched on attach exit, the helper emits
  `agent_exited` with the pane class as the cause, exactly as today.

A future RFC 0089 follow-up (Phase 2 read-surface work) can ship a
`striatum supervise reattach` verb that re-attaches the daemon's PTY
master to the still-live tmux session without re-running the lane
command. Phase 1 only needs to make the *correct* `detached` vs `lost`
distinction.

### 7.2 Pane pid capture in the helper

`Launch` returns the new `LaunchResult` shape (§2.2). The helper
encodes both pids into the `agent_started` payload:

```jsonc
{
  "schema_version": "striatum.supervisor_helper.event.v1",
  "event_type": "agent_started",
  "supervisor_id": "sup_…",
  "payload": {
    "pid": 48211,                  // pane_pid
    "attach_client_pid": 48309,
    "metadata": { "tmux_lane": { … } }
  }
}
```

`mutations/supervision_control.go::agentPIDFromEvents` already reads
`payload.pid`, so this slot continues to hold the *supervised* pid;
attach is metadata-only. `tmuxMetadataFromHelperEvents` is renamed
`tmuxLaneFromHelperEvents` and reads `payload.metadata.tmux_lane`.

## 8. Failure classes

The structured failure classes surfaced upward (event payloads,
`supervise.status`, doctor, recovery sweep) are exactly the set RFC 0089
§2 specifies, with no additions:

| Class                       | Cause                                                                  | Treatment in §4 callsites                                                  |
| --------------------------- | ---------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| `tmux_ok`                   | session alive, pane alive, pane_pid + start_token match                | `running`/`alive`/`attested`                                               |
| `tmux_session_missing`      | `tmux has-session` failed                                              | `lost` with reason `tmux_session_missing`                                  |
| `tmux_pane_missing`         | `display-message` failed or pane_id no longer matches                  | `lost` with reason `tmux_pane_missing`                                     |
| `tmux_pane_dead`            | pane exists, `pane_dead == 1`                                          | `lost` with reason `tmux_pane_dead`                                        |
| `tmux_pane_pid_mismatch`    | pane alive but pid or start_token differs from captured identity       | `lost` with reason `tmux_pane_pid_mismatch` (mirrors D080 pid mismatch)    |
| `tmux_unavailable`          | tmux not on PATH, exec failed, probe timeout                           | heartbeat: transient (3-tick threshold); delivery: refuse with invalid_transition; status: `lane_attestation_reason = "tmux_probe_unavailable"` |
| `tmux_unavailable_persistent` | derived after 3 consecutive `tmux_unavailable` ticks                  | heartbeat: `lost`; delivery/status: unreachable (already lost)             |

These are exposed as string constants under
`supervisor.TmuxLiveness*` and re-exported from the read package so the
read projection does not re-stringify them.

## 9. Rollback, fallback, opt-out

### 9.1 Phase-1 rollback knob

Add `STRIATUM_TMUX_PROBE_DISABLE=1` (read by `ProbeLaneLiveness` at
top of function). When set, the probe always falls through to the
current pid+start-token path even on tmux-backed lanes — i.e. the
production behavior collapses back to RFC 0089 P0. The env var is
documented in `docs/how-to/daemon-runbook.md` and named explicitly so
ops can flip it from a systemd override without code changes if the
new probe misbehaves.

This is *not* a workflow knob (we don't want lane authors gating product
behaviour at runtime); it is a daemon-process knob.

### 9.2 Non-tmux fallback

`launchPlainPTY` is untouched. Its `Metadata` is set to
`{"tmux_lane": {"transport_kind": "plain_pty",
"unavailable_reason": "<reason>"}}` so the read projection still
includes a `tmux` block carrying `state: "unavailable"` and the existing
remediation strings. `ProbeLaneLiveness` sees
`transport_kind = "plain_pty"` and falls through to
`pidLiveWithStartToken`.

For lanes whose workflow.json sets `supervision.require_tmux: true`
(the codex agent-loop lane is the present consumer), `launchPlainPTY`
is never reached because `launchPTY` already returns
`tmuxRequiredError` before the fallback (`pty.go:127`). That codepath
is unchanged.

### 9.3 Backwards compatibility

- `reads/supervision.go::tmuxMetadata` keeps reading from the legacy
  `tmux` key for one release. Until the daemon writes the new
  `tmux_lane` block, the read projection looks indistinguishable from
  today (no `tmux.liveness`, top-level `liveness` is still
  signal-0-driven). After the daemon rolls forward, existing operator
  scripts that grep on `tmux.session_name` and `tmux.attach_command`
  keep working unchanged.
- The columns in `process_supervisors` and `process_supervisor_pointers`
  are unchanged; only the `metadata_json` payload grows. No migration
  is needed.
- `striatum supervise status` JSON gains the optional
  `tmux.liveness` object. Existing top-level fields keep their
  meanings. No CLI breaking change.

## 10. Doc updates

| File                                                          | Change                                                                                                                                                                              |
| ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/reference/spec.md` (supervisor section)                 | Define `tmux_lane` metadata block (the seven fields above) and the seven `TmuxLiveness*` classes; note that `supervise.status` may include a `tmux.liveness` object.                 |
| `docs/decisions/decision-log.md`                              | Add `D152 — RFC 0089 Phase 1 lands tmux session/pane liveness…` paragraph (text per RFC 0089 §"Proposed Decision-Log Entry"). Status `accepted`.                                    |
| `docs/reference/ubiquitous-language.md`                       | New entry `tmux lane` (pane-pid–identified, session/pane-probed) distinct from `attach client` (the transient `tmux attach-session` PTY-master writer).                            |
| `docs/how-to/daemon-runbook.md`                               | New "Tmux liveness probe" section: how to read the seven classes, what `tmux_probe_unavailable` means, how to flip `STRIATUM_TMUX_PROBE_DISABLE=1`, and which logs to consult.       |
| `docs/reference/command-authority-matrix.md`                  | No new RPC methods, so no new rows. Annotate `supervise.status`, `supervise.stop`, `recovery.process_reconcile` rows with "consults tmux liveness probe (RFC 0089 P1)".              |
| `CHANGELOG.md` (Unreleased)                                   | "RFC 0089 P1: tmux-backed lanes track liveness from tmux session/pane identity, not the transient `tmux attach-session` client."                                                    |
| `docs/rfcs/0089-tmux-backed-lane-monitoring.md`               | Status `proposed → accepted` after the cycle completes (touch out of scope here, but flagged for the synthesizer / coordinator).                                                    |

There are no glossary deletions; the existing `attach command` /
`tmux session` / `tmux pane` entries are correct.

## 11. Out of scope for Phase 1

For clarity these are *not* in this slice and the implementer should
defer them to the RFC 0089 Phase 2/3 follow-ups:

- A `striatum supervise attach` convenience verb (Phase 2 read-surface
  work, RFC 0089 §4).
- Flipping agent-loop lanes to tmux-backed *by default* when tmux is
  installed (Phase 3, RFC 0089 §"Phase 3").
- Reattach without restart (live re-attach of the daemon's PTY master to
  a still-running tmux session). Phase 1 reuses the existing
  `attached → detached → reattach via supervise.start` shape.
- Single tmux server-per-run vs server-per-supervisor consolidation
  (RFC 0089 §"Open Questions" — non-blocking).
- Any redaction or export pipeline change. PTY logs under
  `.striatum/scratch/<supervisor_id>/pty.log` stay D028 / D151 private
  diagnostics, untouched.

## 12. Acceptance hookup

Mapping back to RFC 0089 §"Acceptance Criteria" so the implementer can
confirm coverage on the way out:

| RFC line                                                                  | Where it lands                                                                                                |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| attach exit ⇒ lane stays attached/attested                                 | §4.1 + §4.2 + §7.1; test §6.3 `TestLivenessSurvivesAttachClientExit` + §6.2 `TestRunHelperAttachClientExit…`  |
| killing session/pane ⇒ structured lost state before next delivery          | §4.1 + §4.2; tests §6.3 `TestLivenessMarksLostOnPaneDead`, §6.4 `TestReconcileForDeliveryUsesTmuxProbe…`      |
| `supervise.status --json` exposes session/pane/pid/attach/class            | §2.1 + §4.3 + read projection in `tmux.liveness`; test §6.5 `TestSuperviseStatusReportsTmuxLivenessClass`     |
| doctor / dashboard / recovery report same classes without pane text        | §4.4 + §4.5; tests §6.1 `TestProbeNeverReadsPaneText`, §6.5 `TestSuperviseStatusNeverIncludesPaneText`        |
| `supervise.stop` terminates the tmux lane via the session                   | §5; tests §6.4 `TestSuperviseStopUsesTmuxKillSession` + `TestSuperviseStopFallsBackToSIGTERMWhenTmuxUnavailable` |
| live agent-loop lane can be `tmux attach`'d while still working via MCP    | structural — already true once attach exits do not cascade; doc-tested via the implementer's live-attach run  |
| regression: attach-client exit does not mark supervisor lost                | §6.3 `TestLivenessSurvivesAttachClientExit`                                                                   |
| regression: missing session/pane prevents delivery with structured failure  | §6.4 `TestReconcileForDeliveryUsesTmuxProbeForTmuxBackedLane`                                                 |
| D028 guard: pane text never enters state / exports                          | §6.1 `TestProbeNeverReadsPaneText`, §6.5 `TestSuperviseStatusNeverIncludesPaneText`                           |

The verification command at the end of the implementation work is
exactly the TASK.md list: targeted Go tests, `gofmt -l .`, `go vet ./...`,
`go test ./...`. The handoff statement at the end of the implementation
slice is: *"after Phase 1, RFC 0088 agent-loop lanes can be flipped to
tmux-backed by default with a workflow.json / daemon-config change
alone; no further code change is required."*
