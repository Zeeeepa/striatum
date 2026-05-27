# F44 supervised turn-driver design
author: operator
kind: handoff

## Problem framing

F42 made single-shot lanes run through the daemon-spawned turn-driver:
`supervise.start` wraps a process lane as `striatumd -agent-loop -turn-driver --
<lane command>`, and `CommandGenerator` later executes the lane command once per
conversation turn.

The live failure has two required fixes and one small liveness fix:

1. The daemon's systemd PATH is too narrow for operator-local agent binaries
   such as `gemini` and `codex`.
2. A content generation failure is currently reported through `OnFailure` and
   then still returned as a fatal `Loop.Run` error, so `RunTurnDriver` exits as
   failed instead of treating the floor as parked/escalated.
3. The direct pipe supervisor path starts a child with `cmd.Start` and never
   waits on it. On Linux that leaves zombies, and the read-side status probe
   uses signal 0 only, so a zombie can still project as `liveness: alive`.

The fix should stay generic to daemon-spawned process lanes and preserve the
D145 boundary: the generator may receive ordinary process environment such as
PATH and HOME, but no Striatum control state beyond the existing stripped
content-only prompt path.

## Chosen design

### 1. Supervised PATH, not argv0 resolution

Add operator-local bin directories to the supervised process environment in
`go/pkg/mutations/supervision_control.go`, centered on
`supervisedEnv`/`supervisedEnvEntries`.

Use the daemon process environment as the base, derive the operator home from
`HOME` with an `os.UserHomeDir` fallback, and append these path entries when the
home is absolute and safe:

- `$HOME/.local/bin`
- `$HOME/.npm-global/bin`

Append them after the daemon PATH rather than prepending. That keeps system
locations preferred when both contain the same binary, while still allowing
operator-local binaries to be found when the system PATH lacks them. Do not add
the repo root, current directory, relative paths, empty path segments, or any
hardcoded home directory.

This should be implemented as a small deterministic env builder, not ad hoc
string concatenation in launch code. The builder should de-duplicate PATH
entries and should produce one effective PATH value for tests. The PTY-helper
launch path should receive the same PATH entry through `supervisedEnvEntries`,
because helper child env is built by appending the spec env to `os.Environ`.

Why this over resolving `argv[0]` to an absolute path:

- It fixes the actual nested turn-driver case. The generator is executed later
  inside `CommandGenerator`, and `ContentOnlyEnv` preserves PATH while stripping
  `STRIATUM_*`.
- It benefits every daemon-spawned process lane, not just single-shot lanes.
- It keeps workflow command JSON stable and human-readable.
- It avoids pinning a binary path at `supervise.start` time based on one lookup
  policy that may not match the child process environment.

### 2. Generation failure becomes a reported non-fatal outcome

Change `go/pkg/turndriver/loop.go` so generation failure on `TurnOurTurn` has
these semantics:

- `generate` still retries according to `MaxGenerateAttempts`.
- If generation still fails and `OnFailure` is configured, call `OnFailure`
  with the turn, wrapped generation error, and attempt count.
- If `OnFailure` succeeds, return `nil` from `Loop.Run`.
- If `OnFailure` itself fails, return that reporting failure as fatal.
- If `OnFailure` is nil, preserve the current fatal generation error behavior.

This chooses "exit non-fatally after parking/escalation" rather than continuing
the loop. It avoids repeated generation/escalation spam while the floor is still
owned by the failed lane, and it makes `RunTurnDriver` exit cleanly after the
existing `ReportFailure` path records the escalation. A later feature can add an
explicit keep-alive policy if the conversation protocol grows a real parked
floor state that prevents immediate re-generation.

`go/pkg/agentloop/turn_driver.go` should not need new control-state plumbing.
`RunTurnDriver` already wires `Options.OnFailure` to
`conversation.ReportFailure`, and `CommandGenerator.BaseEnv` already flows
through `ContentOnlyEnv`, which strips `STRIATUM_*` but preserves PATH and HOME.

### 3. Land the narrow liveness fix

Land a small liveness slice in F44:

- In `go/pkg/mutations/supervision_control.go`, make the direct pipe launch
  path reap its child by waiting asynchronously after `cmd.Start`. Capture the
  start token before the waiter can reap an immediately-exiting child.
- In `go/pkg/reads/supervision.go`, make the status PID probe zombie-aware on
  Linux, matching the mutation-side `pidAliveLocal` behavior.
- Also make `HandleSuperviseStatus` treat a PID as alive only when the recorded
  `pid_start_time` is empty or matches the current process start token. This
  prevents PID reuse from projecting a stale supervisor as live.

Do not make F44 depend on a larger durable "unexpected child exit transitions
process_supervisors to stopped/lost" monitor if that grows. Reaping plus honest
status projection is the small correctness slice needed to stop reporting
zombies as alive. A later follow-up can persist terminal state immediately on
unexpected child exit.

## Exact implementation surface

- `go/pkg/mutations/supervision_control.go`
  - Add the supervised PATH builder used by `supervisedEnv` and
    `supervisedEnvEntries`.
  - Use safe absolute HOME-derived local bin dirs only.
  - Start a direct-child wait/reap goroutine in `launchPipeProcess`.

- `go/pkg/turndriver/loop.go`
  - Change the generation-failure branch in `Loop.Run` so successful
    `OnFailure` makes the loop return nil rather than propagating
    `ErrGenerationFailed`.
  - Preserve fatal return when there is no `OnFailure` or when failure reporting
    fails.

- `go/pkg/agentloop/turn_driver.go`
  - No behavior change expected beyond existing `ReportFailure` use; only touch
    if a test exposes that `RunTurnDriver` needs to wrap/report a new error
    shape.

- `go/pkg/reads/supervision.go`
  - Make read-side PID liveness zombie-aware and start-token-aware.

- `go/pkg/reads/supervision_process_linux.go`
  - Add or share the Linux `/proc/<pid>/stat` zombie-state parser used by
    read-side liveness.

## Tests

- `go/pkg/mutations/supervision_control_test.go`
  - Add `TestSupervisedEnvAddsOperatorLocalBinsToPath`.
  - Set `HOME` to a temp directory and `PATH` to a daemon-like value.
  - Assert the effective supervised PATH preserves the original entries,
    appends `$HOME/.local/bin` and `$HOME/.npm-global/bin`, and does not
    duplicate existing entries.
  - Assert the normal `STRIATUM_REPOSITORY_ID`, `STRIATUM_RUN_ID`,
    `STRIATUM_SESSION_ID`, `STRIATUM_SUPERVISOR_ID`, `STRIATUM_REPO`, and
    `STRIATUM_LANE_ID` entries are still present.

- `go/pkg/turndriver/loop_test.go`
  - Replace the current expectation in
    `TestLoopGeneratorFailureAndEmptyOutputDoNotSay`, or add a narrower
    `TestLoopGeneratorFailureReportsAndExitsCleanly`.
  - Use a fake generator that returns an exec-not-found-shaped error for every
    attempt.
  - Assert `Loop.Run` returns nil when `OnFailure` succeeds, no `Say` occurs,
    `OnFailure` is called once, and the failure contains
    `ErrGenerationFailed` plus the configured attempt count.
  - Add `TestLoopGeneratorFailureReturnsReportFailure` so an `OnFailure` error
    still propagates.

- `go/pkg/agentloop/turn_driver_test.go`
  - Existing `TestContentOnlyEnvStripsAllStriatumVariables` already pins that
    PATH and HOME survive while `STRIATUM_*` is stripped. No new test is
    required unless implementation changes this helper.

- `go/pkg/reads/supervision_test.go`
  - Add a Linux-stat parser test equivalent to the mutation-side zombie test.
  - Add a status projection test using the fake runner where recorded
    `pid_start_time` does not match the current start token; assert
    `liveness: gone` and unattested lane state.

## Alternatives considered

1. Resolve every lane command `argv[0]` to an absolute path during
   `supervise.start`.
   Rejected because it does not help arbitrary child subprocesses, is awkward
   for the turn-driver wrapper boundary, and risks pinning the wrong binary at
   launch time.

2. Require an operator systemd PATH drop-in.
   Rejected because it keeps local installation knowledge outside the product
   and already failed live verification without manual surgery.

3. Continue looping after `OnFailure`.
   Deferred. It could keep the lane process resident, but without a daemon-level
   parked-floor state it risks immediately regenerating and re-escalating the
   same failing turn.

4. Implement full durable child-exit monitoring in F44.
   Deferred if it expands. Reaping and honest status projection should land now;
   persisting an unexpected-exit terminal state can follow once the daemon owns a
   clear child watcher abstraction.

## Risks and mitigations

- PATH injection: only add absolute HOME-derived directories; never add the
  repository, `.`, relative entries, or empty segments.
- Wrong binary: append local bins after the daemon PATH so existing system
  binaries keep precedence. Workflows that need a specific binary can still use
  an absolute command path.
- D145 boundary regression: do not pass any new `STRIATUM_*`, lease, packet, or
  MCP token state to the content generator. The existing `ContentOnlyEnv` test
  should remain green.
- Silent failure: if `OnFailure` cannot report escalation, keep returning a
  fatal error. Only a successfully reported failure becomes non-fatal.
- Liveness race: capture `pid_start_time` before launching the reaper goroutine,
  and make status check PID identity so PID reuse does not appear live.

## Rollout and verification

Implementation should land as one small F44 slice. Update
`docs/DECISION_LOG.md` only if the implementer chooses a materially different
policy, especially if full durable child-exit state transitions are deferred.
The handoff should say explicitly whether the liveness slice landed fully or
only the reaper/status projection landed.

Verification commands:

- `cd go && gofmt -l .`
- `cd go && go test ./pkg/mutations ./pkg/turndriver ./pkg/agentloop ./pkg/reads`
- `cd go && go test ./...`
- Live smoke after removing the local systemd PATH drop-in: start a supervised
  single-shot lane whose command is `gemini`; confirm the turn-driver finds the
  binary, and then use an intentionally missing generator to confirm
  `session.report` escalation is emitted and the supervisor no longer projects
  `liveness: alive` after exit.
