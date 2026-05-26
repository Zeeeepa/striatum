# TASK — F44: daemon-spawned single-shot turn-driver lanes must find their generator and fail gracefully

Reference: `docs/TODO.md` F44; F42 (`docs/operator/workflows/f42-conversation-turn-driver/`,
v2.6.0, D145); memory `project_f42_turn_driver`, `feedback_supervisor_path_npmglobal`.

## Problem (found during F42 live verification, 2026-05-26)

F42 shipped the turn-driver: a lane declaring `adapter_capabilities.single_shot:
true` is run by `supervise.start` as `striatumd -agent-loop -turn-driver -- <lane
command>`. The driver invokes the lane command once per turn as a content
generator (`go/pkg/agentloop/turn_driver.go` `CommandGenerator`, env built by
`ContentOnlyEnv`).

Live verification on the real daemon exposed three defects:

1. **Generator not found on the daemon's PATH.** The daemon-spawned turn-driver
   inherits the daemon's systemd `PATH` (`/usr/local/sbin:…:/snap/bin`), which
   lacks `~/.local/bin` / `~/.npm-global/bin` where `gemini` (and `codex`) live.
   Result, captured from stderr:
   ```
   agent-loop failed: turn-driver generator failed after 2 attempt(s):
   content generator failed: exec: "gemini": executable file not found in $PATH
   ```
   This is the recurring `feedback_supervisor_path_npmglobal` issue, now hitting
   the turn-driver. (`supervisedEnvEntries` in
   `go/pkg/mutations/supervision_control.go` sets no PATH;
   `ContentOnlyEnv`/`CommandGenerator.BaseEnv` pass the daemon PATH through.)

2. **Generator-not-found crashes the whole driver instead of parking the floor.**
   `RunTurnDriver` → `turndriver.Loop.Run` returns the generation error, the
   process exits, and (because `cmd.Start` is called without `cmd.Wait` —
   `supervision_control.go:866`) it becomes a **zombie**. The F42 design intended
   repeated generation failure to *park the floor + emit an escalation report*
   (`OnFailure`/`ReportFailure`), not exit the process. An exec-not-found (or any
   single generation failure) should be handled the same graceful way.

3. **Stale liveness.** After the child exits/zombies, `supervise.status` still
   reports `liveness: alive` with a frozen heartbeat. The daemon does not reap
   exited supervised children and does not transition their liveness.

A `striatumd.service.d/path.conf` systemd drop-in (operator-local, not in the
repo) currently works around #1. F44 makes the product correct without that.

## Goal

A daemon-spawned single-shot turn-driver lane finds its generator binary without
operator systemd surgery, fails a turn gracefully (park + escalate) instead of
crashing, and never shows as `alive` after it has exited.

## Scope (smallest correct fix)

1. **PATH for supervised lanes.** In `supervise.start`'s supervised-process env
   (`supervisedEnv`/`supervisedEnvEntries`, `supervision_control.go`), ensure the
   operator's local bin directories (`$HOME/.local/bin`, `$HOME/.npm-global/bin`)
   are on `PATH` for the spawned lane — OR resolve the lane command's argv[0] to
   an absolute path at launch. Prefer a deterministic, testable approach; do not
   hardcode a single user's home. Keep it generic (this helps every
   daemon-spawned lane, not just the turn-driver). Decide and justify which of
   the two approaches; a unit test must pin the resulting env/command.
2. **Graceful generator failure.** A generation failure — including
   exec-not-found — must route through the existing `OnFailure`/`ReportFailure`
   path (park the floor, emit `session.report`/escalation) and let the driver
   keep looping or exit non-fatally, NOT crash `RunTurnDriver`. Add a
   `turndriver` test for the exec-not-found / generator-error path proving the
   loop does not propagate a fatal error and the failure is reported.
3. **Liveness honesty (smaller, optional within this slice if it grows):** the
   daemon should reap exited supervised children and stop reporting stale
   `alive`. If this expands the diff materially, the synthesis may defer #3 to a
   follow-up and say so explicitly — but #1 and #2 are required.

## Constraints

- Daemon is the sole writer; no new hosted services/telemetry. Keep the
  spoon-feeding boundary from D145 intact (the generator still receives only
  topic+transcript; do not add `STRIATUM_*` or control state to the child env —
  adding PATH dirs is fine and is not control state).
- Generic by capability/role, not by model name.

## Definition of done

- A daemon-spawned single-shot lane finds its generator with no operator PATH
  drop-in (verifiable: with the `path.conf` drop-in removed, a supervised gemini
  turn-driver still finds `gemini`).
- Generator failure parks the floor + escalates; the driver does not crash/zombie
  on it. Covered by a `turndriver` test.
- `go test ./...` green; `gofmt` clean.
- HANDOFF with the exact verification commands; DECISION_LOG note (and whether
  liveness #3 landed or deferred).

Defer: the operator-facing `striatum conversation drive` debug command, deeper
child token non-discovery, and chat-UI conversation rendering (F43).
