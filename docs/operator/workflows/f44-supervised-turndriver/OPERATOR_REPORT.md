# Operator Report — F44 supervised turn-driver hardening

Run: `run_8e1f89659a527746786c71800e725072` · branch `striatum/f44-supervised-turndriver`
Workflow: iterated-interrogating-panel (design + build), 2 model lanes (claude_code + codex).
Shipped v2.7.0, D146.

## Outcome

Fixed the F44 bug F42 live verification surfaced. The codex implementer landed:
- **PATH**: supervised lanes build one deduped effective `PATH` = inherited
  system PATH + existing operator-local dirs (`$HOME/.local/bin`,
  `$HOME/.npm-global/bin`, or `STRIATUM_SUPERVISED_PATH_DIRS`). No hardcoded home.
- **Graceful failure**: `turndriver.Loop` routes exhausted generator failures
  (incl. exec-not-found) through `OnFailure` → `session.report` escalation +
  parked floor, instead of crashing `RunTurnDriver`.
- **Honest liveness**: pipe supervisors reap via async `cmd.Wait`; read-side
  liveness is zombie/start-token-aware; `supervise.status` reports `gone` not
  stale `alive`.

## Panel

All four verdicts `accept_with_findings`, each via a genuine live interrogation
(design: codex threat_model 2 rounds, claude ergonomics_dx 2 rounds; build:
codex threat_model 2 rounds, claude ergonomics_dx 3 rounds). Both review panels
were driven concurrently so the interrogable target's window stayed open for both
— the F42 window-closure friction did not recur. `go test ./...` green.

## Live verification (PASSED, in isolation)

Rebuilt + installed `striatumd` 2.7.0. To isolate the PATH fix, forced the daemon
to a minimal `PATH` (via a temporary `path.conf`) on which `gemini` is NOT
findable — the exact F42 failure condition. Then `supervise.start` a gemini
`single_shot` lane:

- The supervised turn-driver's `PATH` was augmented to include `~/.local/bin`
  (F44 #1) — the generator **executed** (pre-F44: `exec: "gemini": not found`
  crash).
- `supervise.status` reported **`liveness: gone`** after exit, with **no zombie**
  (`defunct` count 0) — F44 #3. Pre-F44 this was a stale `alive` zombie.
- Graceful-failure (#2) is unit-tested (missing-binary generator) + reviewed; the
  clean reap with no crash corroborates it live.

(gemini-cli did not complete a turn in the smoke window — a known gemini runtime
slowness, orthogonal to F44; the F44-relevant behaviors all verified.)

The temporary minimal `path.conf` was removed after verification; the daemon now
runs on its natural PATH and **needs no operator `path.conf` workaround** — F44
makes supervised lanes self-sufficient.

## Friction (low; the F42 lessons held)

- One `review.submit` from a reviewer initially failed until it included
  `logical_name`+`kind` (F39 ergonomics — the single-call contract should accept
  path+verdict alone). Recorded for a future F39 polish.
