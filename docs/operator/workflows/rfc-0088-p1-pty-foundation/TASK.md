# TASK — RFC 0088 P1: daemon-owned interactive PTY lane + owned-PTY byline attestation (claude baseline)

Reference: `docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md`
(Decisions 1 and 2; Phasing P1). Prior art: D140 (conditional `--print`
deprecation + the unsubmitted-prompt blocker), D080/D141 (attestation +
agent-loop interrogation), memory `project_agentloop_headless_vs_pty`.

## Goal

Make **claude** runnable as a daemon-owned, long-lived **interactive PTY
session** (its native interactive mode — no `-p`/`--print`) that:

1. receives its bootstrap prompt and every subsequent turn by **stdin-submit
   through the PTY master** the supervisor already owns, and
2. earns a **first-class lane byline** (`author: <role>-<model>-<ordinal>`)
   via the owned pid + launch-command-snapshot binding — *not* `author:
   operator`.

This is the foundation the rest of RFC 0088 (agy migration, codex cutover,
`--print` wrapper + turn-driver deletion) builds on. P1 proves it on claude,
the known-good baseline. Do **not** delete the `--print` wrapper or the
turn-driver in P1 — that is P3, after all adapters are proven.

## The two defects to close

### 1. The PTY launcher never submits the prompt (D140 Phase A blocker)

`striatumd -agent-loop` in `self_driving` mode launches the lane, but with an
interactive TUI agent the bootstrap prompt is **buffered unsubmitted** (claude
waits for an Enter/submit it never receives). `go/pkg/supervisor/pty.go`
already allocates a PTY (`UsePTY: true`) and threads the **PTY master back as
the daemon's stdin handle** (`LaunchResult.StdinWriter`), so the channel
exists — what's missing is sending the correct **submit key-sequence**
(newline/`\r`, bracketed-paste where needed) after the agent is ready, and
delivering each later turn the same way instead of re-invoking the binary.

Anchors: `go/pkg/supervisor/pty.go` (`Launch`, `UsePTY`, `StdinWriter`);
`go/pkg/mutations/supervision_control.go` (`agentLoopModeSelfDriving` ~line 30,
launch/`cmd.Start`, `supervisedEnv`); the agent-loop executor in
`go/pkg/agentloop/` (`loop.go`, `bootstrap.go`, `endpoint.go`).

### 2. Owned-PTY persistent sessions don't earn a lane byline

Today only the supervised `--print` *wrapper* is attested; agent-loop /
persistent sessions publish `author: operator` (D141 left bylines unchanged).
The owned-PTY session has exactly the pid-start-time + snapshot-command binding
attestation needs, so it must derive the lane byline the same way the wrapper
does — just for a long-lived process.

Anchors: `go/pkg/mutations/mutations.go:647 sessionLaneAttestation`;
`go/pkg/mutations/claim.go:705 artifactAuthorIdentity(... attested bool ...)`;
`go/pkg/mutations/supervision_control.go:1680 laneAttestation(pidStartTime)`;
liveness in `go/pkg/reads/supervision.go pidLiveWithStartToken`. Attestation is
**anti-fabrication friction, not non-repudiation** (D080) — the byline holds
while the owned pid identity holds and the launch command matched the snapshot;
turn-to-turn context drift does not weaken it.

## Deliverables

- An interactive-PTY launch path (a new agent-loop submit mode or an extension
  of `self_driving`) that launches claude in interactive mode over a PTY and
  **submits** the bootstrap + per-turn prompts via the PTY master.
- Byline attestation extended so an owned-PTY persistent session earns
  `author: <role>-<model>-<ordinal>` (pid + command-snapshot match), without
  widening the forgery surface vs the wrapper (no byline for a session whose
  pid identity changed or whose command does not match the snapshot).
- Tests: a PTY submit-driver test using a **fake/echo TUI binary** (no live
  model); an attestation-derivation test proving an owned-PTY session yields a
  lane byline and a mismatched pid/command still yields `author: operator`.
- `docs/rfcs/0088-*.md` status/notes updated if behavior diverges from the RFC;
  `docs/DECISION_LOG.md` D148/D149 may be promoted on landing; `docs/TODO.md`
  P1 entry.

## Adjacent defect to fix in P1 (discovered 2026-05-27 while driving this run)

`launchPipeProcess` (`supervision_control.go:847`) and the PTY helper
(`supervisor/pty.go:67,171`) call `exec.CommandContext(ctx, command[0], …)`,
which resolves argv0 against the **launching process's own PATH at call time**;
setting `cmd.Env` to F44's `supervisedPath()` afterward (line 849) is too late
and only affects the child's subprocesses. The helper is also spawned with **no
`cmd.Env`** (line 907), so it inherits the daemon's systemd PATH. Net effect:
F44 retiring the `path.conf` drop-in **regressed supervised lane launch** — any
lane binary not on the daemon's systemd PATH (codex, claude in `~/.local/bin`)
fails with `exec: "<bin>": executable file not found in $PATH`. An operator
`path.conf` drop-in was restored to unblock this run, but the real fix belongs
here: resolve argv0 to an absolute path using `supervisedPath()` (or pass the
augmented env to the helper) before `exec.Command`, then delete the drop-in
again. Add a regression test that launches a lane binary present only on the
augmented PATH, not the daemon PATH.

## Out of scope for P1

agy lane, gemini removal, codex cutover, deleting the `--print` wrapper or the
turn-driver/`single_shot` capability. Per-adapter submit sequences for
codex/agy are P2/P3 — P1 may structure the submit driver to be per-adapter, but
only claude must be proven.

## Verification log (2026-05-27)

**PROVEN end-to-end:** a daemon-owned claude interactive PTY agent-loop lane
(`adapter_capabilities.agent_loop: true`, command `claude --model opus
--dangerously-skip-permissions`, no `-p`) received its bootstrap, **submitted**
it, connected to the striatum MCP via the ephemeral `--mcp-config
--strict-mcp-config`, and autonomously `await_packet → wrote VERIFY.md →
publish → complete` (job `completed`). The published artifact carried a **lane
byline `author: writer-claude-opus-004`, not `author: operator`** — confirming
the owned-PTY session is attested and earns the byline via the existing D080
supervised-session attestation path (`claim.go` `artifactAuthorIdentity(…
attested …)`). **D149's goal is met without new attestation code**, because
`supervise.start` attaches a supervisor (pid + command-snapshot) to the
agent-loop session. Harness: `docs/operator/workflows/rfc-0088-p1-verify/`.

The submit needed a **separate keystroke after a delay** (concatenating CR to
the prompt is absorbed into the multi-line input); see `agentLoopSubmitDelay`
(default 750ms, `STRIATUM_AGENT_LOOP_SUBMIT_DELAY_MS`).

**Open P1 follow-ups found during verification:**
1. **claude bypass-consent dialog.** claude's interactive startup can show a
   one-time "Bypass Permissions mode" consent dialog (`1. No, exit / 2. Yes`);
   the bootstrap Enter then selects "No, exit" and claude exits. Robust
   interactive lanes must pre-accept this (seed claude config / lane prep) or
   the submit driver must answer it. Adapter-specific lane-prep surface.
2. **path.conf drop-in removal pending.** Fix C (argv0 resolution) is unit-
   tested, but the end-to-end "remove the drop-in" check was confounded by
   finding #1 (PATH-independent). Re-validate drop-in removal after #1.

## Verification

`cd go && gofmt -l . && go vet ./... && go test ./...`. Live proof
(reviewers run it): a daemon-spawned claude interactive PTY session takes a
bootstrap prompt, performs `await_packet → write → artifact.publish → ack →
work.complete`, the published artifact carries a `author: <role>-claude-…`
byline (not `operator`), and the session is interrogable while live.
