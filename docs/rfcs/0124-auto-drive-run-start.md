# RFC 0124: Auto-drive on `run start` — background the reconcile loop off the operator

Status: accepted (D191)
Date: 2026-06-14
author: proposer-claude-opus-4-8-001
Context: #212; RFC 0116 / D175 (`run drive`); RFC 0122 / D189 (daemon-side scheduler principal — **this RFC is the operator-side shim toward that end-state**); RFC 0120 / D180 (notify-only wake bus); RFC 0105 / D161 (unattended-reliability yolo gate); RFC 0103 W7 (operator as bounded processor). Code already on `main`: `go/cmd/striatum/run_start.go`, the `run start` interceptor in `go/cmd/striatum/main.go`, `go/pkg/cli/rundrive`, and the `--no-drive` opt-out in `scripts/dod/driver.py`.

## Summary

Make `striatum run start` launch a detached `run drive` for the run it just
started, **on by default**, so the run reconciles itself to a terminal state
with no operator process — and no operator-model tokens — in the loop. This RFC
specifies the behavior, the **default-on decision**, the lifecycle and security
posture of the background driver, its opt-outs, and its contracts.

The implementation **shipped provisionally on `main`** alongside RFC 0122's
acceptance (D189, #212). This RFC exists to give that operator-facing behavior
change its own design record and ratification, rather than leaving it as a rider
on the scheduler-principal RFC. Acceptance ratifies the shipped behavior; the
maintainer may instead **amend the default** (opt-in) before ratifying.

## Why this is its own RFC (not part of RFC 0122)

RFC 0122 designs a **daemon trust primitive** — a scheduler that spawns under the
run owner's pre-authorization. This RFC is an **operator CLI UX change** — a core
verb (`run start`) gains a default side effect (a background process). They are
different subjects with different lifecycles (0122 is design-only and phased;
this is shipped code), and folding the second into the first (as D189 did) hid
operator-facing decisions — default-on, background-process security, co-driving,
pause interaction — behind a "do-now wiring" label. Separating them lets 0122
stay a clean trust-model RFC and lets the decisions below be adjudicated
explicitly.

This RFC is the **cheap, shippable-today step**: it removes the operator *model*
from the mechanical loop while spawn authority still rides the operator
principal. RFC 0122's scheduler is the **end state**: it removes the standing
operator *credential* by moving spawn authority into the daemon. 0124 is a
deliberate intermediate, not a competitor.

## Problem

`run drive` (RFC 0116) is a deterministic Go loop — it spends zero
operator-model tokens. But something must *launch and own* it. When the operator
is an expensive frontier model, having it hand-drive lanes (or babysit a
foreground `run drive`) burns tokens on mechanical orchestration the model adds
no judgment to. Daemonizing `run drive` by hand (the original
`docs/how-to/daemonize-run-drive.md`) fixes this but is a manual per-run step an
operator must remember. The product goal (#212) is zero operator touch; the
ergonomic gap is that `run start` does not, itself, ensure the run gets driven.

Waiting for RFC 0122's daemon scheduler is the "correct" end state but is
design-only and phased (weeks). Auto-drive is hours and removes the dominant
cost (operator-model tokens) now.

## Behavior

`run start` is intercepted (mirroring the existing `run drive` interceptor in
`main.go`): it runs the start **verbatim** through the daemon route, then, on a
zero exit and unless opted out, launches a detached driver for the started run.

- **What is launched:** `run drive --run-id <id>` for the run, carrying the
  operator's `--repo` (resolved to cwd if unset) and, if set, `--daemon-socket`
  / `--capability-token-file`.
- **How it is detached:** a transient `systemd-run --user` unit named
  `striatum-drive-<run-id>`. It survives the `run start` process, is inspectable
  via `journalctl --user -u striatum-drive-<run-id>`, and garbage-collects
  itself (`--collect`) at exit.
- **Idempotent:** the unit name is the run id, so a second `run start` (or a
  manual `run drive`) for the same run hits systemd's "unit already exists" and
  is treated as *already driving* (success). The driver itself re-derives state
  from daemon reads and checks for a live session before spawning, so it never
  double-spawns a slot.
- **Best-effort:** if `systemd-run` is unavailable (no user systemd session,
  some containers) the start still succeeds and prints a one-line manual-drive
  hint. Auto-drive never changes the start's exit code.
- **Non-invasive:** all auto-drive notices go to **stderr**, so `--json`
  consumers of `run start` are unaffected.

## Design decisions

### Default-on vs opt-in — the decision for the maintainer

**Recommendation: default-on, with two opt-outs.** The product goal is
zero-operator-touch (#212); a default-off `--drive` flag would reintroduce the
"remember to drive it" step auto-drive exists to remove. The self-driving cases
that must *not* get a second driver are covered by the opt-outs:

- `run start --no-drive` — per-invocation (a harness that drives the run itself);
- `STRIATUM_RUN_DRIVE_AUTO=0` (also `false`/`no`/`off`) — global (CI, or an
  operator who always drives by hand).

`scripts/dod/driver.py` (the unattended-DoD harness) already passes `--no-drive`.

**Alternative considered: default-off + `--drive`.** Safer-by-surprise (no
verb gains a hidden side effect), but it fails the zero-touch goal and pushes the
cost back onto the operator. *This is the explicit open question for the
maintainer; the shipped code is default-on.*

### Transient unit vs installed template

Chose a **transient `systemd-run` unit** over an installed
`striatum-run-drive@.service` template because it is self-contained (no
`daemon install` dependency, no per-run env file), is journaled, and
auto-collects. The cost is no pre-inspectable unit file; `journalctl --user -u
striatum-drive-<id>` and `systemctl --user status` cover inspection.

### Lifecycle

The driver exits when the run reaches a terminal state (`completed`/`failed`/
`canceled`) and the unit is collected. There is **no `Restart`**: a loud refusal
(RFC 0105) should surface in the journal, not be masked by a respawn loop;
re-`run start` (idempotent) or an explicit `run drive` resumes driving.

### Co-driving with explicit `run drive`

Because the driver is idempotent, the background driver composes safely with any
explicit `run drive` (the refactoring-campaign skill, a debugging operator): both
re-derive from daemon reads and check for live sessions before spawning. The
explicit driver then serves as a foreground terminal-state waiter. Harnesses that
want exactly one driver use `--no-drive`.

### Security posture

`run start` gains a side effect, and the background driver holds the operator's
runtime capability (discovered from `XDG_RUNTIME_DIR`) for the run's lifetime — a
**standing credential**. Threat model: the driver exercises only the authority
`run start` already uses (`register-session` + `supervise.start` under the
operator principal); it mints no new capability and crosses no new boundary —
it changes spawn *cadence*, not *authority* (the same argument D175 made for
`run drive`). No secret appears on the systemd command line: the token is
discovered from the runtime dir, and only a non-default `--daemon-socket` is
passed via `--setenv`. The standing-credential residual is real and is precisely
what RFC 0122's daemon scheduler removes.

### Mid-run pause / cancel (contract + inherited risk)

The background driver outlives `run start`, so it must respect a *mid-run*
`run pause`/`run cancel`. The driver exits on terminal states; **pause is
non-terminal**, so the loop continues, and whether it spawns for a paused run
depends on whether pausing gates job readiness. This behavior is **inherited from
`run drive`** (a foreground `run drive` on a paused run behaves identically), not
introduced by auto-drive — but auto-drive makes it reachable without an operator
consciously starting a driver, so it is called out here. **C5 below makes
"respect mid-run pause/cancel" an acceptance gate to verify against `run drive`'s
reconcile, fixing it in `run drive` (one home) if it does not already hold.**

## Contracts

- **C1 — idempotent:** a second start (or manual `run drive`) for a driven run
  never double-spawns a slot; unit-name collision is treated as already-driving.
- **C2 — opt-outs honored:** `--no-drive` is stripped before the start mutation
  and suppresses the launch; `STRIATUM_RUN_DRIVE_AUTO` falsey suppresses globally.
- **C3 — degrade cleanly:** no `systemd-run` ⇒ start still succeeds, prints a
  manual-drive hint, exit code unchanged.
- **C4 — non-invasive:** notices are stderr-only; `run start --json` stdout is
  byte-identical to the no-auto-drive output; the start exit code passes through.
- **C5 — respect mid-run pause/cancel:** the driver must not spawn lanes for a
  paused run. **Met:** `run drive`'s reconcile now holds a paused run
  (`paused_at` set) — it does cleanup but launches no new lanes, stays
  non-terminal so resume re-drives, and announces the hold once
  (`isPausedRun` + the pause branch in `ReconcileOnce`,
  `TestRunDriveHoldsPausedRun`). Fixed in `run drive` (one home), so a foreground
  driver gets the same guarantee. Canceled is terminal and already exits.
- **C6 — no standing secret exposure:** no capability token on a process command
  line or in a unit's recorded args; socket override passed via env only.

C1–C6 are met by the shipped code: `run_start_test.go` covers C1–C4 launch
decisions and the arg/flag handling; `TestRunDriveHoldsPausedRun` covers C5.

## Relationship to RFC 0122

When RFC 0122 Phase 3 lands (the daemon scheduler spawns under the captured
run-owner principal), auto-drive is either (a) **retargeted** so `run start` flips
the run's `auto_spawn` grant and the daemon drives — deleting the standing
operator credential — or (b) **kept** as the non-daemon fallback for deployments
that do not enable the scheduler. That choice is deferred to 0122 Phase 3; this
RFC does not pre-commit it.

## Non-Goals

- **No all-runs supervisor.** Auto-drive only ever drives a run an operator just
  *deliberately started*, so it cannot drive a paused or held run *at start* (the
  hazard a blind active-runs sweeper would hit, and the same one RFC 0122
  §"crash/restart" guards). Driving every active run is the daemon scheduler's
  job (0122), not a CLI loop's.
- **No new RPC, no hosted service, no daemon-side spawn.** `run start` and the
  driver are CLI clients of existing audited mutations; daemon-initiated spawn is
  RFC 0122.

## Decision ask

**Accepted (D191).** The shipped default-on auto-drive is ratified: `run start`
backgrounds a `run drive` by default, with the `--no-drive` / `STRIATUM_RUN_DRIVE_AUTO=0`
opt-outs. D189 was split so it covers only the RFC 0122 scheduler design. The C5
acceptance gate was discharged before flipping to accepted — `run drive` now
holds a paused run (above). The default-on-vs-opt-in choice landed on **on**,
because the product goal is zero-operator-touch and the self-driving harness
cases are covered by the opt-outs.
