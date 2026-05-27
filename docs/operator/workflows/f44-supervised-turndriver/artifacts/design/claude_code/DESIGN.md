---
author: operator
kind: handoff
logical_name: claude_design
---

# F44 — Supervised turn-driver PATH + graceful failure (claude_code design lane)

## 1. Problem framing

F42 (v2.6.0, D145) shipped the single-shot turn-driver: a lane declaring
`adapter_capabilities.single_shot: true` is launched by `supervise.start` as
`striatumd -agent-loop -turn-driver -- <lane command>`
(`turnDriverAgentLoopCommand`, `supervision_control.go:1329`). The wrapped
process runs `RunTurnDriver` (`turn_driver.go:31`), which builds a
`turndriver.Loop` whose `Generator` is a `CommandGenerator` that invokes the
*real* lane binary (`gemini`, `codex`, …) once per turn as a pure content
generator (`turn_driver.go:98`), feeding it only topic+transcript and stripping
every `STRIATUM_*` var via `ContentOnlyEnv` (`turn_driver.go:125`) — the D145
spoon-feeding boundary.

Live verification on the real daemon (2026-05-26) exposed three layered defects.
They are not independent: #1 *triggers* #2, and #2 *triggers* #3. Understanding
the trigger chain is what makes the fix minimal.

1. **Generator not found on the daemon's PATH.** `launchPipeProcess`
   (`supervision_control.go:840`) sets `cmd.Env = supervisedEnv(...)`, which is
   `append(os.Environ(), supervisedEnvEntries(...)...)` (`:1466`).
   `supervisedEnvEntries` (`:1470`) adds only six `STRIATUM_*` entries and **no
   PATH**, so the supervised process inherits the daemon's systemd `PATH`
   (`/usr/local/sbin:…:/snap/bin`). That PATH lacks `~/.local/bin` and
   `~/.npm-global/bin`, where `gemini`/`codex` are installed. The agent-loop
   wrapper binary itself resolves fine (`agentLoopExecutable()` uses
   `os.Executable()`, absolute — `:1368`), but inside the loop the generator
   shells out to `gemini` by **bare name** (`exec.CommandContext(g.Command[0])`,
   `turn_driver.go:105`) with `ContentOnlyEnv(os.Environ())` — which faithfully
   passes the daemon's deficient PATH straight through. Captured stderr:

   ```
   content generator failed: exec: "gemini": executable file not found in $PATH
   ```

   This is the recurring `feedback_supervisor_path_npmglobal` failure mode, now
   reaching the turn-driver instead of a directly-supervised lane.

2. **A generation failure crashes the whole driver instead of parking the
   floor.** `Loop.generate` retries `MaxGenerateAttempts` (default 2) and returns
   a wrapped `ErrGenerationFailed` (`loop.go:149-175`). `Loop.Run` then calls
   `OnFailure` **and immediately `return err`** (`loop.go:112-117`). That fatal
   return propagates out of `RunTurnDriver`, the wrapper process exits, and —
   because `launchPipeProcess` calls `cmd.Start` with **no `cmd.Wait`**
   (`:866`) — the exited child becomes a zombie reparented to nobody (it is a
   direct daemon child, so it sits as `Z` until reaped). The F42 design intent
   (encoded in `ReportFailure`/`OnFailure`, `turn_driver.go:211`) was that a
   generation failure *parks the floor and escalates* via `session.report`, then
   the driver keeps living. An exec-not-found is just a generation failure that
   happens on attempt 1; it should be handled the same graceful way, not be a
   one-way crash.

3. **Stale liveness after the crash.** `supervise.status`'s liveness derivation
   calls `pidAlive` (`reads/supervision.go:660`), which is **signal-0 only** and
   has no zombie check. A zombie is still signalable, so `pidAlive` returns true
   and status reports `liveness: "alive"` with a frozen heartbeat forever. Note
   the asymmetry: the *mutations*-side probe `pidAliveLocal`
   (`supervision_control.go:1541`) already calls `processZombieLocal` and would
   correctly report dead — only the *reads* path is blind.

The current operator workaround for #1 is a `striatumd.service.d/path.conf`
systemd drop-in (local, not in the repo). F44 must make the product correct so
that drop-in can be removed.

## 2. Chosen fixes

### 2.1 PATH for supervised lanes — **augment the supervised env (append local bin dirs)**

**Decision: extend the PATH in `supervisedEnvEntries`, appended (not prepended),
deriving directories from the operator's `$HOME` (with an env override). Reject
the "resolve argv[0] to absolute at launch" alternative.**

Mechanism. `supervisedEnvEntries` gains a computed `PATH=` entry built from:

1. the inherited `PATH` (`os.Getenv("PATH")`), kept verbatim and **first**;
2. then appended, the local bin dirs that exist and are not already present —
   default set `$HOME/.local/bin` and `$HOME/.npm-global/bin`, resolved from
   `os.UserHomeDir()` (falls back to `$HOME`); overridable wholesale by a new
   `STRIATUM_SUPERVISED_PATH_DIRS` (`:`-joined) for non-standard installs.

Because `supervisedEnv = append(os.Environ(), entries...)` and Go's `exec` uses
the **last** value for a duplicated key, appending our computed `PATH=` entry
deterministically wins over the inherited one. `PATH` is a non-`STRIATUM_` key,
so it survives `ContentOnlyEnv` and reaches the generator child — which is
exactly the channel we need, and is *not* control state, so D145 is preserved
(see §5). The same augmented PATH also benefits every directly-supervised lane,
not just turn-drivers, which is why it belongs in the shared `supervisedEnvEntries`
seam rather than in turn-driver-specific code.

Why **append**, not prepend: prepending `$HOME/.local/bin` lets a stray
user-local binary shadow a system tool the daemon expects (the PATH-injection /
wrong-binary risk in §6). Appending makes the local dirs a *fallback* — system
locations resolve first, and we only add reach for binaries that the system PATH
genuinely lacks. This matches the failure we are fixing (gemini is *only* in
`~/.npm-global/bin`) without changing resolution of anything already findable.

Why **not** "resolve argv[0] to absolute at launch":
- It is not actually simpler. To resolve `gemini` to an absolute path the daemon
  must run `exec.LookPath` *against the augmented PATH anyway* — the daemon's own
  PATH can't find gemini either — so this alternative still needs the same
  `$HOME` dir list. It adds work without removing the root cause.
- It bakes a snapshot path at launch; if the binary is reinstalled/moved
  mid-session (npm upgrade) the frozen absolute path breaks, whereas a PATH entry
  re-resolves each turn.
- argv[0] of the *supervised* process is `striatumd` (the wrapper), not `gemini`;
  the bare-name lookup we must fix happens one level down inside
  `CommandGenerator.Generate`. Rewriting that argv[0] would mean threading an
  absolute path through `turnDriverAgentLoopCommand` → `RunTurnDriver` → the
  command slice, touching more surface than a single env helper.
- It is less generic: it fixes only the generator exec, not the many other bare
  binaries a supervised lane may invoke.

No hardcoded home: directories come from `os.UserHomeDir()`/`$HOME` at runtime,
or the explicit override env. Nothing user-specific is committed.

### 2.2 Graceful generator failure — route through `OnFailure`, do not return fatal

**Decision: in `Loop.Run`, a generation failure that has an `OnFailure` handler
is reported and then the loop *continues* rather than returning the error;
classify permanent misconfiguration (exec-not-found) so it escalates once and
the driver exits cleanly instead of hot-looping.**

The change is localized to `loop.go:112-117`. Today:

- `OnFailure` is invoked, then `return err` (fatal) → process exit → zombie.

Proposed control flow when `generate` fails:

- Always call `OnFailure` (park floor + `session.report` escalate) as today.
- If `OnFailure` is **nil**, preserve current behavior (`return err`) so the
  pure-library contract is unchanged for callers that want fatal semantics.
- If `OnFailure` is set:
  - **Transient failure** (model timeout `context.DeadlineExceeded`, non-zero
    exit, empty output): `Sleep(opts.PollInterval)` then `continue` the loop.
    The floor was not advanced by us (we never called `Say`), so the next
    `AwaitTurn` re-derives the same `our_turn`; we get another attempt after the
    operator/peer state changes, exactly matching the crash-safe floor model.
  - **Permanent misconfiguration** — `exec-not-found`, detected with
    `errors.Is(err, exec.ErrNotFound)` (threaded through the
    `ErrGenerationFailed` wrap so it remains inspectable): escalate **once**, then
    `return nil`. Returning nil exits the wrapper cleanly (no non-zero crash, the
    operator sees the escalation), and — combined with the §2.1 PATH fix — this
    path should not normally be reachable in production; it is the belt to §2.1's
    suspenders. Exiting rather than hot-looping avoids re-escalating an
    unrecoverable condition every poll interval.

To keep `exec.ErrNotFound` inspectable, `CommandGenerator.Generate`
(`turn_driver.go:112-121`) must wrap with `%w` (it already does) and
`Loop.generate` must wrap `lastErr` with `%w` rather than `%v`
(`loop.go:174` currently uses `%v`, which *flattens* the chain) — a one-verb
change so `errors.Is` works end to end.

This is the smallest change that satisfies "fail a turn gracefully (park +
escalate) instead of crashing." It deliberately keeps the classification in the
generic `turndriver` package (by error sentinel, not by model name), honoring
the "generic by capability/role" constraint.

### 2.3 Liveness honesty — **land the small half, defer the reaper**

**Decision: land the zombie-aware read probe now (small, safe, directly fixes
the reported "shows alive" symptom); defer the active SIGCHLD/`cmd.Wait` reaper
to a follow-up and say so explicitly.**

- **Land now:** make `reads/supervision.go pidAlive` (`:660`) zombie-aware by
  mirroring the existing `processZombieLocal` logic already living in
  `mutations/supervision_control.go:1555` (lift it to a shared helper, or add a
  small linux `/proc/<pid>/stat` state read in the reads package). A zombie then
  reports `liveness: "gone"`/`"stalled"`, which is the honest answer and removes
  the frozen-`alive` symptom from the DoD's verification. This is ~10 lines + a
  table test and carries no process-lifecycle risk.
- **Defer (call out in synthesis):** actually *reaping* the zombie so it leaves
  the process table. The clean fix is a daemon-level child reaper (a SIGCHLD
  handler or a `cmd.Wait` goroutine per supervised child). That touches daemon
  startup/process-management wiring and supervisor-pointer state transitions —
  materially larger than #1/#2 and orthogonal to the turn-driver bug. With §2.1
  and §2.2 in place the driver no longer exits on the common failure, so zombies
  become rare; the cosmetic `gone` report covers the remaining honesty gap. A
  follow-up issue should wire the reaper (likely opportunistic
  `reapIfChild(pid)` already exists at `:1576` and could be called from the
  status read or a periodic sweep — but wiring it correctly without racing the
  liveness probe deserves its own slice).

## 3. Exact files + tests

**Code**
- `go/pkg/mutations/supervision_control.go` — `supervisedEnvEntries`: compute and
  append the augmented `PATH=`; add an unexported helper
  `supervisedPATH(base string, homeDirs []string) string` and a
  `supervisedLocalBinDirs() []string` reader for `$HOME`/override. (PATH fix #1.)
- `go/pkg/turndriver/loop.go` — `Loop.Run` failure branch: `continue`/`return nil`
  classification; `Loop.generate`: wrap `lastErr` with `%w`. (Graceful #2.)
- `go/pkg/agentloop/turn_driver.go` — no behavior change required; confirm
  `Generate` already wraps `%w` (it does). Optionally surface `exec.ErrNotFound`
  unambiguously. (Supports #2.)
- `go/pkg/reads/supervision.go` — `pidAlive` zombie-aware. (Liveness #3, landed
  half.)

**Tests**
- `supervision_control_test.go` (new/extended): unit-pin `supervisedEnvEntries`
  output — assert exactly one `PATH=` entry, that it ends with the local bin dirs
  in order, that system PATH stays first, that an absent dir is skipped, and that
  `STRIATUM_SUPERVISED_PATH_DIRS` overrides. Use a temp `$HOME`; **no hardcoded
  home**. This is the "unit test must pin the resulting env/command" the TASK
  requires.
- `turndriver/loop_test.go`: add `TestLoopExecNotFoundEscalatesAndExitsCleanly`
  (fake generator returns an error wrapping `exec.ErrNotFound`; assert `OnFailure`
  fired once, no `Say`, and `Run` returns **nil**) and
  `TestLoopTransientGenerateFailureReportsAndContinues` (fail once then succeed;
  assert escalate-then-say, `Run` nil). **Update existing
  `TestLoopGeneratorFailureAndEmptyOutputDoNotSay`** — it currently asserts
  `errors.Is(err, ErrGenerationFailed)`; under the new contract with `OnFailure`
  set it must assert continue/clean-exit semantics instead. Keep one case with
  `OnFailure == nil` asserting the legacy fatal return.
- `reads/supervision_test.go`: a zombie-state `/proc` stat fixture (or injected
  state reader) asserting `pidAlive` → false / liveness `gone`.

**Verification (for HANDOFF)**
- `make test` / `go test ./...` green; `gofmt -l go/` empty.
- Live: with the `striatumd.service.d/path.conf` drop-in **removed** and daemon
  restarted, start a supervised `gemini` single-shot turn-driver lane and confirm
  `supervise.status` shows it generating (not exec-not-found), the conversation
  advances, and after the conversation closes `supervise.status` reports `gone`
  not a frozen `alive`.

## 4. Alternatives considered

1. **Resolve argv[0] to absolute at launch** (the prompt's second option).
   Rejected — see §2.1; still needs the `$HOME` dir list, freezes a path that can
   move, targets the wrong argv layer, and is less generic.
2. **Prepend local bin dirs to PATH.** Rejected — introduces a real
   wrong-binary/shadowing risk (§6) for no benefit over appending in our failure
   case.
3. **Fix PATH only at the turn-driver layer** (augment in `RunTurnDriver`/
   `CommandGenerator.BaseEnv`). Rejected — narrower than the bug class; the
   PATH deficiency hits every daemon-spawned lane, so the shared
   `supervisedEnvEntries` seam is the correct home. (Could be a fallback if a
   reviewer objects to changing env for non-turn-driver lanes, but the broader
   fix is strictly more correct and is what `feedback_supervisor_path_npmglobal`
   has been asking for.)
4. **For #2: always `continue` on any failure** (no exec-not-found
   classification). Simpler, but hot-loops + re-escalates forever on a permanent
   misconfiguration. The classification costs one `errors.Is` and is worth it.
5. **For #2: always `return nil` after escalate** (never continue). Loses the
   retry-next-turn behavior for genuinely transient model timeouts, which is the
   exact case `OnFailure`/park-floor was designed for. Rejected.

## 5. Risks

- **PATH-injection / wrong-binary safety (primary).** Adding directories to a
  child's PATH can change which binary a bare name resolves to. Mitigated by
  **appending** (system dirs win; we only add fallback reach) and by skipping
  non-existent dirs. The override env is operator-controlled, same trust domain
  as the daemon. Residual risk: an operator who *intends* a system tool but has a
  stale copy in `~/.local/bin` is unaffected (append → system wins); the reverse
  (relying on a local override of a system tool) is intentionally **not**
  supported, which is the safe default. Document this ordering in the DECISION_LOG
  note.
- **D145 boundary.** The constraint is that the generator child receives only
  topic+transcript and no `STRIATUM_*`/control state (`ContentOnlyEnv`). Adding
  `PATH` is a non-`STRIATUM_` *environment-resolution* var, not workflow control
  state, and TASK explicitly blesses "adding PATH dirs is fine and is not control
  state." `ContentOnlyEnv` already keeps non-`STRIATUM_` vars, so the augmented
  PATH flows through unchanged and no other state leaks. A `turndriver` guard test
  (`TestConversationContextCarriesNoControlState` already exists) keeps the
  spoon-feeding shape honest; we add an assertion that the generator env contains
  no `STRIATUM_` keys but does contain the augmented PATH.
- **Duplicate PATH entries.** `append(os.Environ(), "PATH=…")` yields two PATH
  entries; Go exec last-wins makes this correct, but it is mildly surprising. Low
  risk; the unit test pins "exactly one effective PATH" by checking the
  last-wins value. Optionally dedup in the helper for cleanliness.
- **#2 contract change is observable to library callers.** `Loop.Run` no longer
  returns a fatal error when `OnFailure` is set. The only in-tree caller
  (`RunTurnDriver`) wants exactly this. The nil-`OnFailure` legacy path is
  preserved and tested, so external/library use is unaffected. Flagged because it
  changes an existing test's expectation.
- **#3 deferral.** Shipping the cosmetic `gone` without the reaper leaves
  zombies in the table (rare post-#1/#2). Acceptable for this slice; the
  follow-up issue is the mitigation. Synthesis must state the deferral.

## 6. Rollout

1. Land #1 (PATH) + #2 (graceful) together — they are the required DoD and the
   trigger pair; #1 alone leaves the crash latent, #2 alone leaves gemini
   unfound.
2. Land the #3 read-probe half in the same PR (small, removes the reported
   symptom); open a follow-up issue for the active reaper and reference it in the
   DECISION_LOG note.
3. DECISION_LOG entry: record the append-not-prepend PATH policy, the
   `STRIATUM_SUPERVISED_PATH_DIRS` override, the `Loop.Run` graceful-continue
   contract (and the nil-`OnFailure` legacy carve-out), and the #3 land/defer
   split.
4. After merge, on the operator host: remove the `striatumd.service.d/path.conf`
   drop-in, restart the daemon, and run the live verification in §3. Removing the
   drop-in is the real-world proof the product is now self-sufficient.
5. No version-bump opinion here (operator policy: each landed RFC/feature bumps
   minor + CHANGELOG promote + tag at merge); F44 is a defect-fix slice on the
   F42 feature line and the synthesis/build lanes will decide the bump.

## 7. Definition-of-done mapping

- *Generator found with no drop-in* → §2.1, live verification §3.
- *Generator failure parks + escalates, no crash/zombie, `turndriver` test* →
  §2.2, new loop tests §3.
- *`go test ./...` green, `gofmt` clean* → §3 verification.
- *HANDOFF with exact commands + DECISION_LOG note (liveness landed/deferred)* →
  §3, §6 (#3 = read-probe landed, reaper deferred).
