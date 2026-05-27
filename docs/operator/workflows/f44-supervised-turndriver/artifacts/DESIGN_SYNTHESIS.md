---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/workflows/f44-supervised-turndriver/artifacts/design/claude_code/DESIGN.md", "docs/operator/workflows/f44-supervised-turndriver/artifacts/design/codex/DESIGN.md"]
---

# F44 — Supervised turn-driver PATH + graceful failure — design synthesis

author: operator

## 0. What this reconciles

Two independent designs (`design/claude_code/DESIGN.md`, `design/codex/DESIGN.md`)
attack the same three-defect chain from F42 live verification (2026-05-26):
a daemon-spawned single-shot turn-driver (1) can't find its generator binary on
the daemon's narrow systemd PATH, (2) crashes the whole driver on that generation
failure instead of parking+escalating, and (3) then projects a frozen
`liveness: alive` because the exited child is a never-`Wait`ed zombie.

The two designs **agree completely on #1** and on the framing of #2/#3. They
**diverge on two buildable decisions**:

- **#2 failure semantics:** claude wants to *classify* transient vs. permanent
  failures and `continue` the loop for transient ones (return `nil` only for
  exec-not-found); codex wants the simpler *exit-non-fatal-after-escalate* with
  no `continue` and no classification.
- **#3 liveness scope:** claude wants to land *only* the zombie-aware read probe
  and defer the reaper; codex wants to also land a per-child `cmd.Wait` reaper
  and a start-token identity check now.

This synthesis resolves both, picks the smaller-correct buildable path for each,
and is the single buildable spec for the implementer.

## 1. PATH for supervised lanes — augment `supervisedEnvEntries` (both agree)

**Decision (unanimous): extend `PATH` in the shared supervised-process env
builder; do NOT resolve `argv[0]` to an absolute path at launch.**

Mechanism, in `go/pkg/mutations/supervision_control.go`:

- Add a small deterministic, unexported env builder (not ad-hoc string
  concatenation in launch code) that produces **one** effective `PATH=` value:
  1. the inherited daemon `PATH` (`os.Getenv("PATH")`) kept verbatim and **first**;
  2. then **appended** (never prepended): the operator-local bin dirs that are
     **absolute, exist, and are not already present** — default set
     `$HOME/.local/bin` and `$HOME/.npm-global/bin`, with `$HOME` resolved from
     `os.UserHomeDir()` (fall back to `os.Getenv("HOME")`);
  3. overridable wholesale by a new `STRIATUM_SUPERVISED_PATH_DIRS` (`:`-joined)
     env for non-standard installs.
- **De-duplicate** PATH entries and emit a single `PATH=` so tests can pin one
  value (do not rely solely on Go exec's last-wins; emit a clean single entry).
- Never add the repo root, `.`, relative entries, empty segments, or any
  hardcoded home directory.
- This belongs in the **shared** `supervisedEnvEntries` seam (consumed by both
  `supervisedEnv` and the PTY-helper launch path, which appends the spec env to
  `os.Environ()`), so every daemon-spawned lane benefits — not just turn-drivers.

**Why append, not prepend** (claude's risk analysis, adopted): prepending lets a
stray user-local binary shadow a system tool the daemon expects. Appending makes
the local dirs a *fallback* — system locations resolve first; we only add reach
for binaries the system PATH genuinely lacks (exactly the gemini-only-in-npm
case). A workflow that needs a specific binary may still use an absolute command.

**Why not argv[0]-absolute** (both reject): it still needs the same `$HOME` dir
list (the daemon's own PATH can't `LookPath` gemini either), freezes a snapshot
path that breaks on reinstall, targets the wrong argv layer (the supervised
process is `striatumd`; the bare-name lookup is one level down in
`CommandGenerator.Generate`), and fixes only the generator exec rather than every
bare binary a supervised lane invokes.

**D145 boundary preserved.** `PATH` is a non-`STRIATUM_` environment-resolution
var, not workflow control state; TASK explicitly blesses adding PATH dirs.
`ContentOnlyEnv` already strips every `STRIATUM_*` while preserving `PATH`/`HOME`,
so the augmented PATH flows through to the generator child unchanged and no
control state leaks.

## 2. Graceful generator failure — exit non-fatally after a reported escalation

**Conflict:** claude proposes classify-then-`continue` for transient failures and
`return nil` only for `exec.ErrNotFound`; codex proposes the simpler
exit-non-fatal-after-escalate with no `continue`.

**Decision: adopt codex's exit-non-fatal-after-escalate as the F44 contract; do
NOT `continue` and do NOT branch control flow on error classification in this
slice. Keep claude's `%w` wrapping hygiene fix.**

Rationale for choosing codex's semantics over claude's `continue`: there is **no
daemon-level parked-floor protocol state** today that suppresses immediate
re-generation. claude's own analysis admits the next `AwaitTurn` would re-derive
the same `our_turn` — which means a transient `continue` would regenerate after
one `PollInterval`, re-fail, and re-escalate on a tight loop. Without a real
parked state to gate on, `continue` is escalation spam, not graceful retry.
Exiting cleanly after one recorded escalation is the honest, non-spammy behavior,
and — combined with §1 — the common cause (exec-not-found) no longer occurs.

Exact semantics in `go/pkg/turndriver/loop.go`, `Loop.Run` generation-failure
branch (after `generate` exhausts `MaxGenerateAttempts`):

- If `OnFailure != nil`: call `OnFailure(turn, wrappedErr, attempts)` (the
  existing park-floor + `session.report` escalation path).
  - If `OnFailure` returns `nil` (escalation recorded): **`return nil`** — the
    wrapper exits cleanly; the operator sees the escalation, no non-zero crash,
    no zombie loop.
  - If `OnFailure` itself returns an error: **`return that error`** (fatal) — a
    failure we could not even report must stay loud.
- If `OnFailure == nil`: preserve today's fatal `return err` so the pure-library
  contract is unchanged for callers that want fatal semantics.
- **exec-not-found is treated identically** to any other generation failure — no
  special `errors.Is(err, exec.ErrNotFound)` control branch. The §1 PATH fix is
  what removes its cause; §2 only guarantees that *if* generation fails for any
  reason, it parks+escalates+exits cleanly instead of crashing.

**Keep from claude (cheap hygiene, adopted):** in `Loop.generate`, wrap `lastErr`
with `%w` instead of `%v` (currently `%v` flattens the chain) so the wrapped
`ErrGenerationFailed` and any underlying `exec.ErrNotFound` remain inspectable in
the escalation report and for a future classifier. This is a one-verb change with
no control-flow dependency.

`go/pkg/agentloop/turn_driver.go` needs **no behavior change**: `RunTurnDriver`
already wires `Options.OnFailure` to `conversation.ReportFailure`, and
`CommandGenerator.BaseEnv` already flows through `ContentOnlyEnv`. Touch it only
if a test exposes a new error shape that must be wrapped.

**Deferred (call out):** a real keep-alive "stay resident and retry next turn"
policy belongs to a follow-up that first introduces a daemon-level parked-floor
state the loop can gate on. F44 deliberately does not add it.

## 3. Liveness honesty — land the per-child reaper AND the honest read probe

**Conflict:** claude lands only the zombie-aware read probe and defers the reaper;
codex lands the reaper + zombie-aware probe + start-token identity check.

**Decision: adopt codex's fuller slice — it is still small and *actually fixes*
the zombie rather than merely relabeling it.** claude's read-probe-only leaves the
zombie in the process table and only renames its status to `gone`; codex's
per-child `cmd.Wait` goroutine reaps it. The reaper here is **local to
`launchPipeProcess`** (one goroutine per supervised child), not a global SIGCHLD
handler, so it does not touch daemon startup wiring — which was claude's stated
reason for deferral and does not apply to this narrower shape.

Land all three, in this order of safety:

1. **Reaper** — `go/pkg/mutations/supervision_control.go` `launchPipeProcess`:
   after `cmd.Start`, **capture `pid_start_time` (the start token) first**, then
   start an async goroutine that `cmd.Wait()`s the child so it cannot become a
   zombie. Capturing the token before the waiter can reap an immediately-exiting
   child is the race mitigation (addresses claude's "racing the liveness probe"
   concern).
2. **Zombie-aware read probe** — `go/pkg/reads/supervision.go`: make the
   status PID probe zombie-aware on Linux, mirroring the mutation-side
   `pidAliveLocal`/`processZombieLocal` logic (lift to a shared helper or a
   `go/pkg/reads/supervision_process_linux.go` `/proc/<pid>/stat` state reader).
   A zombie/dead PID then reports `gone`/`stalled`, never frozen `alive`.
3. **Start-token identity** — `HandleSuperviseStatus`: treat a PID as alive only
   when the recorded `pid_start_time` is empty or matches the current process
   start token, so PID reuse cannot project a stale supervisor as live.

**Escape hatch (explicit):** if the reaper goroutine proves to race supervisor
state transitions or balloons the diff in practice, fall back to claude's
position — land items 2 and 3 (both small, zero process-lifecycle risk) and
defer item 1 (the reaper) to a follow-up, saying so in the HANDOFF.

**Deferred by both (keep deferred):** a durable monitor that immediately
transitions `process_supervisors` to `stopped`/`lost` on unexpected child exit.
Reaping + honest projection is the F44 correctness slice; durable terminal-state
persistence is a later follow-up once the daemon owns a clear child-watcher
abstraction.

## 4. Exact implementation surface (merged)

**Code**
- `go/pkg/mutations/supervision_control.go`
  - New unexported PATH builder + local-bin-dirs reader (`$HOME`/override),
    appended-and-deduped, used by `supervisedEnv`/`supervisedEnvEntries`. (#1)
  - `launchPipeProcess`: capture `pid_start_time`, then start an async
    `cmd.Wait` reaper goroutine. (#3.1)
- `go/pkg/turndriver/loop.go`
  - `Loop.Run` failure branch: `OnFailure` success → `return nil`; `OnFailure`
    error → fatal; `OnFailure == nil` → legacy fatal. No `continue`, no
    classification. (#2)
  - `Loop.generate`: wrap `lastErr` with `%w` (was `%v`). (#2 hygiene)
- `go/pkg/agentloop/turn_driver.go` — no behavior change expected. (supports #2)
- `go/pkg/reads/supervision.go` — zombie-aware + start-token-aware PID
  liveness; `HandleSuperviseStatus` start-token identity check. (#3.2, #3.3)
- `go/pkg/reads/supervision_process_linux.go` — add/share the Linux
  `/proc/<pid>/stat` zombie-state parser used by the read-side probe. (#3.2)

**Tests**
- `go/pkg/mutations/supervision_control_test.go`
  - `TestSupervisedEnvAddsOperatorLocalBinsToPath`: temp `$HOME`, daemon-like
    `PATH`; assert exactly **one** `PATH=` entry, original entries preserved and
    first, `$HOME/.local/bin` then `$HOME/.npm-global/bin` appended in order,
    absent dirs skipped, no duplicates, and `STRIATUM_SUPERVISED_PATH_DIRS`
    overrides. Assert the six `STRIATUM_*` entries
    (`STRIATUM_REPOSITORY_ID`/`RUN_ID`/`SESSION_ID`/`SUPERVISOR_ID`/`REPO`/`LANE_ID`)
    are still present. **No hardcoded home.** (satisfies TASK's "unit test must
    pin the resulting env/command")
- `go/pkg/turndriver/loop_test.go`
  - Add `TestLoopGeneratorFailureReportsAndExitsCleanly`: fake generator returns
    an exec-not-found-shaped error every attempt; assert `Run` returns **nil**,
    no `Say`, `OnFailure` called **once**, and the reported error wraps
    `ErrGenerationFailed` + carries the attempt count.
  - Add `TestLoopGeneratorFailureReturnsReportFailure`: `OnFailure` returns an
    error → `Run` propagates it (fatal).
  - **Update** the existing `TestLoopGeneratorFailureAndEmptyOutputDoNotSay`:
    under the new contract with `OnFailure` set it must assert clean-exit (`nil`)
    semantics; keep one case with `OnFailure == nil` asserting the legacy fatal
    return (`errors.Is(err, ErrGenerationFailed)`).
- `go/pkg/agentloop/turn_driver_test.go`
  - Keep existing `TestContentOnlyEnvStripsAllStriatumVariables`
    green (PATH/HOME survive, `STRIATUM_*` stripped). No new test required unless
    the helper changes.
- `go/pkg/reads/supervision_test.go`
  - Linux `/proc` zombie-state stat parser test → `pidAlive` false / liveness
    `gone`, mirroring the mutation-side zombie test.
  - Status-projection test where recorded `pid_start_time` ≠ current start token
    → `liveness: gone` and unattested lane state.

## 5. Verification (for HANDOFF)

- `cd go && gofmt -l .` → empty.
- `cd go && go test ./pkg/mutations ./pkg/turndriver ./pkg/agentloop ./pkg/reads`
- `cd go && go test ./...` → green.
- **Live** (the real-world proof): remove the operator's
  `striatumd.service.d/path.conf` drop-in, restart the daemon, then start a
  supervised single-shot `gemini` turn-driver lane. Confirm: (a) it finds
  `gemini` and the conversation advances (no exec-not-found); (b) with an
  intentionally missing generator, `session.report` escalation is emitted and the
  driver exits cleanly (no fatal crash); (c) after exit, `supervise.status`
  reports `gone`, not a frozen `alive`.

## 6. DECISION_LOG note (for the implementer to record)

Record, as a defect-fix slice on the F42/D145 feature line:

1. **PATH policy:** supervised lanes get `$HOME/.local/bin` + `$HOME/.npm-global/bin`
   **appended** (system PATH wins; local is fallback), derived from
   `os.UserHomeDir()`/`$HOME`, overridable via `STRIATUM_SUPERVISED_PATH_DIRS`,
   deduped to one effective `PATH`. Removes the need for the
   `striatumd.service.d/path.conf` drop-in.
2. **Graceful-failure contract:** `Loop.Run` returns `nil` after a successful
   `OnFailure` escalation (exit non-fatally, no `continue`); stays fatal when
   `OnFailure` is nil or itself errors. exec-not-found is treated as an ordinary
   generation failure. A keep-alive/retry-next-turn policy is **deferred** pending
   a real parked-floor state. `Loop.generate` now wraps with `%w`.
3. **Liveness:** per-child `cmd.Wait` reaper + zombie-aware read probe +
   start-token identity check all land in F44; durable unexpected-exit
   terminal-state persistence is **deferred**. (If the reaper is dropped under
   the §3 escape hatch, the HANDOFF must say so.)

## 7. Definition-of-done mapping

- *Generator found with no drop-in* → §1; live verification §5(a).
- *Generator failure parks + escalates, no crash/zombie, `turndriver` test* →
  §2; new loop tests §4.
- *No frozen `alive` after exit* → §3; read-side tests §4, live §5(c).
- *`go test ./...` green, `gofmt` clean* → §5.
- *HANDOFF with exact commands + DECISION_LOG note (liveness landed/deferred)* →
  §5, §6.
</content>
</invoke>
