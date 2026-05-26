# Operator Report — F42 autonomous conversation turn-driver

Run: `run_63a8ffa4a77edebfd25620876fe9e7ce` · branch `striatum/f42-conversation-turn-driver`
Workflow: iterated-interrogating-panel (design + build), 2 model lanes (claude_code + codex).
Driver: operator drove headless lanes manually (PTY launcher unused per
`project_agentloop_headless_vs_pty`); models pinned (`claude --model opus`, `codex exec`).

## Outcome (what shipped)

The codex implementer landed the generic turn-driver per the design synthesis:

- `go/pkg/turndriver/` — pure, fake-tested loop (`Conversation` + `Generator`
  seams; `ConversationContext` carries only `Topic`+`Transcript`; output
  sanitization; bounded retries; not-our-floor wait; closed-conversation exit).
- `go/pkg/agentloop/turn_driver.go` — `striatumd -agent-loop -turn-driver`
  runtime wiring. The **driver** is the MCP client (holds the token, calls
  `conversation.say`); the child agent is invoked once per turn as a content
  generator. `ContentOnlyEnv` strips all `STRIATUM_*` before exec.
- `supervise.start` selects driven mode from **lane capability metadata**
  (`adapter_capabilities.single_shot` / `self_driving`), **not model name** —
  satisfies the genericity requirement.
- D145 in `docs/DECISION_LOG.md`; conversation-3way recipe + gemini guide
  templates updated to the turn-driver path.
- `go test ./...` green, `gofmt -l` clean.

The spoon-feeding-hazard tension (TASK.md) was resolved with an enforceable
seam: a reflection test pins `ConversationContext` to topic+transcript only, and
the env scrub keeps control/credential state out of the child. The design panel
interrogated exactly this boundary (see below).

## Genuine interrogation signal

- **codex design review (threat_model)** ran a real 2-round interrogation of the
  live synthesizer, pressing on the spoon-feeding boundary ("what concrete guard
  prevents enabling the driver for an MCP-capable agent as a packet proxy?"). The
  synthesizer separated Hazard A (feeding control state — structurally
  unreachable) from Hazard B (child credential self-discovery — accepted v1
  residual risk). Verdict: `accept_with_findings`.

## Friction findings (real product signal from driving the run)

1. **Parallel-gate write-scope deadlock (recurring; `project_concurrent_gate_writescope_deadlock`).**
   Two parallel `draft` gates with disjoint per-lane write scopes in a *shared
   worktree*: the second completer is rejected because the first sibling's path
   is dirty and outside the second's scope. claude completed first (codex path
   still clean) → passed; codex then failed `work.complete`. Recovery: commit
   between gates + `run.retry_job`. **Fix candidate:** worktree isolation for
   parallel gate groups, or scope the dirty-tree guard to the job's own paths.

2. **Interrogator answer channel is `interrogation.show`, not `await_packet`.**
   Answers are stored addressed to the interrogator in state `completed`;
   `work.await_packet` only delivers pending *questions*. A reviewer that polls
   `await_packet` for answers hangs. Should be documented in the reviewer guide.

3. **Reviewer session needs the `interrogate` capability at registration.**
   `session.register` stores whatever `capabilities` are passed (no lane
   cross-check). A reviewer registered without `interrogate` gets
   `capability_denied` on `interrogation.open`. The agent-loop/supervise path
   must grant `interrogate` to reviewer sessions on interrogable upstreams, or
   the workflow reviewer role should imply it.

4. **`review.submit` is a single call (acks + publishes + records verdict).**
   The codex reviewer called `artifact.publish` separately first, so
   `review.submit`'s internal publish hit a duplicate-content constraint and
   failed; it recovered via `work.ack` + `review.verdict`. The friction-free
   recipe is ONE `review.submit{session_id,job_id,lease_id,path,verdict}` with
   no separate publish/ack. (F39: the opaque errors here are exactly the
   `review.verdict`/`artifact.publish` friction.)

5. **awaiting_interrogation window closes after the FIRST interrogation, breaking
   multi-reviewer panels.** `maybeCloseInterrogationTarget` closes the target
   when it has no open interrogations AND no active lease. After the codex design
   review closed its interrogation, the synthesizer (whose `claude -p` had
   exited, releasing its lease) was closed `interrogation_window_closed` — so the
   second design reviewer (claude, ergonomics_dx) could not interrogate and
   honestly recorded **0 rounds**; re-attaching an answerer process could not
   reopen the daemon-side window. **Fix candidate (proposed by the reviewer):**
   keep the window open / re-openable until all panel reviewers have
   interrogated, or lease-extend the target while reviews are pending. Mitigation
   used for the build panel: launch both reviewers concurrently and open
   interrogations early so windows overlap.

6. **Stale-lease on agent relaunch.** Reusing a killed agent's session leaves its
   job `claimed` with a held lease (lazy expiry); the relaunch gets `no_work`.
   Recovery: `recovery.cancel-job` (requires a non-empty `reason`) →
   `run.retry_job` → fresh session.

7. **Interrogable answerer exit condition.** The synthesizer prompt exited after
   "one interrogation close", but a 2-reviewer panel needs the target alive
   across all reviewers. The implementer prompt was corrected to stay alive until
   ~15 consecutive `no_work` polls.

## Verdict ledger

- design: threat_model `accept_with_findings` (2-round interrogation),
  ergonomics_dx `accept_with_findings` (0 rounds — window-closure bug #5).
- build: threat_model `accept_with_findings` (3-round interrogation),
  ergonomics_dx `accept_with_findings` (2-round interrogation).

Both build interrogations were genuine and ran concurrently against the live
implementer — launching both reviewers at once kept the implementer's
`awaiting_interrogation` window open for both (the workaround for finding #5).
Run `run_63a8ffa4a77edebfd25620876fe9e7ce` reached state `completed`; all four
verdicts are non-blocking (`accept_with_findings`, low/medium severity).

## Live verification (DoD #5) — post-merge, pending

The running daemon is still the pre-F42 binary. Live proof that a gemini lane
participates autonomously via the turn-driver (not a shell script) requires
rebuilding + restarting `striatumd` with this code, then running a conversation
with a gemini lane declared `adapter_capabilities.single_shot: true` under
`supervise.start` (`agent_loop_mode=turn_driver`). Tracked as the immediate
post-merge action.
