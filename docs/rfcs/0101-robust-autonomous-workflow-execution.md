# RFC 0101: Robust Autonomous Workflow Execution — survive lane failure, run to completion or fail loudly, without a human babysitter

Status: proposed
Date: 2026-05-31
author: proposer-claude-opus-4-8-001

Context:
- A striatum workflow run is a **distributed system whose workers are unreliable
  LLM CLIs driven through PTYs**. The daemon's state machine is the only reliable
  component; every lane is an external process that can hang, exit early, buffer
  its bootstrap, block on a survey prompt, or silently stop making progress.
- Across two days of dogfooding (this session alone closed ~14 issues and
  triaged ~22), the failure modes cluster into a small set of recurring shapes,
  and almost every one **wedged a live run that then required a human operator to
  diagnose and hand-drive recovery** — often with attempt-bumping workarounds
  (`run retry-job`) that pollute run history. Representative evidence:
  - **Silent lane-boundary failures.** Claude Code v2.1.x buffered the bootstrap
    prompt in its TUI and never submitted it, so two `claude_code` lanes sat idle
    while `supervise.list` read `attached`/`healthy`
    ([#101](https://github.com/halbritt/striatum/issues/101)); `agy` exits after
    its one `--prompt-interactive` turn and re-registers an unattested duplicate,
    stalling any multi-turn seat ([#95](https://github.com/halbritt/striatum/issues/95));
    `agy` spawns a background MCP-discovery probe and idles past the deadline
    ([#85](https://github.com/halbritt/striatum/issues/85)); a gemini usage survey
    blocks a live review lane until an operator types `0`
    ([#76](https://github.com/halbritt/striatum/issues/76)).
  - **Dishonest liveness.** A lane doing long *local* artifact repair between MCP
    calls is classified `stalled` while actively working
    ([#80](https://github.com/halbritt/striatum/issues/80)); a lane blocked inside
    a hidden MCP call holds a live lease with no operator-visible timeout
    ([#83](https://github.com/halbritt/striatum/issues/83)); `supervise rebridge`
    reported success while delivery stayed degraded
    ([#67](https://github.com/halbritt/striatum/issues/67), fixed).
  - **No autonomous recovery.** Transferring a live repo-write claim to a fresh
    session required choreography that bumped the attempt
    ([#82](https://github.com/halbritt/striatum/issues/82),
    [#108](https://github.com/halbritt/striatum/issues/108)); a re-opened revision
    job auto-published the unchanged prior artifact or stranded on a duplicate
    lease ([#65](https://github.com/halbritt/striatum/issues/65)); a re-publish
    collided on a fixed `logical_name`
    ([#84](https://github.com/halbritt/striatum/issues/84)). Each needed an
    operator at the keyboard.
  - **Silent stalls instead of loud failures.** A spent session polled `no_work`
    forever while a same-role `fresh_session_required` job sat queued, with no
    explanation ([#107](https://github.com/halbritt/striatum/issues/107)); a
    leaked interrogation window left reviewers 2..N `target_unavailable`
    ([#65 P1](https://github.com/halbritt/striatum/issues/65)).
  - **Contract/scope friction that converts good work into a failed completion.**
    Opaque artifact-contract errors ([#74](https://github.com/halbritt/striatum/issues/74)/[#79](https://github.com/halbritt/striatum/issues/79)/[#110](https://github.com/halbritt/striatum/issues/110)),
    byline rejection ([#106](https://github.com/halbritt/striatum/issues/106)),
    sibling/parallel write-scope false positives ([#93](https://github.com/halbritt/striatum/issues/93)/[#102](https://github.com/halbritt/striatum/issues/102)/[#109](https://github.com/halbritt/striatum/issues/109)),
    parallel-claim deadlocks ([#103](https://github.com/halbritt/striatum/issues/103)),
    parallel same-`(role,lane)` registration refusal ([#100](https://github.com/halbritt/striatum/issues/100)).
- The deep, still-open robustness tail is exactly the part that **can only be
  validated against a live lane + a daemon restart** and therefore was deferred
  through every prior pass: #80/#83 (liveness fusion), #85/#95 (turn-driver),
  #70 (lane sandbox), and the systems work behind them. They keep being deferred
  because there is no *framework* that makes them safe to build and prove.
- Prior RFCs each own a slice of this surface but none owns **robustness as a
  property of the whole run**: [RFC 0091] (lane health), [RFC 0095]
  (attempt-scoped lifecycle — the state-machine substrate), [RFC 0096]
  (lane trust boundary — the safety substrate), [RFC 0097] (run orchestration —
  automation that *amplifies* every robustness gap, since an autonomous
  orchestrator wedges faster than a human driver). RFC 0097 explicitly names
  RFC 0095/0096 as hard prerequisites "because automation amplifies their bugs."
  This RFC is the missing connective tissue between them.
- [`docs/decisions/decision-log.md`](../decisions/decision-log.md): **D094/D005**
  (the daemon is authoritative; lanes are daemon-mediated), **D028** (curated
  artifacts, not raw transcripts, are workflow state), **D026/D080** (attestation
  governs byline provenance). Robustness must be built *within* these — the
  daemon stays the single source of truth; lanes stay scratch.

## Problem

The runner can author and validate beautiful workflow *shapes*, but it cannot yet
**run one to completion unattended**. A single lane failure anywhere in a
multi-phase, multi-lane run — a CLI that won't submit its bootstrap, a lane that
exits after one turn, a tool call that hangs, a heartbeat missed during honest
local work — silently wedges the run. The control surface then frequently lies
(`attached`/`healthy` over an idle pane), so the operator does not even know it is
wedged until they poll; and when they do, recovery is a manual diagnosis followed
by hand-driven verbs, several of which bump the attempt and make a clean
operational correction look like a content failure in run history.

Concretely, today a run's robustness *is* the operator. Every dogfood in this
project's history has needed a human to babysit it, and the runner-fixes "can't
be dogfooded through the broken runner" (RFC 0097) precisely because the runner
is not robust enough to carry its own development. That is the loop we have to
break.

Three structural deficits cause this:

1. **The lane boundary is unguarded and unverified.** Each adapter
   (claude/codex/agy) fails in its own silent way, there is no contract every
   adapter must satisfy, and a new CLI version can regress bootstrap or turn
   behavior with no test catching it — it is discovered *live*, mid-run.
2. **Liveness is inferred from protocol timestamps alone.** The daemon cannot see
   whether the child PTY is actually producing output or touching files, so it
   cannot distinguish "working locally," "stuck in a tool call," and "dead" — and
   so it both false-alarms healthy lanes and fails to alarm dead ones.
3. **There is no autonomous recovery loop and no loud-failure contract.** Stuck
   states are detected (sometimes) but not *acted on* without an operator, and a
   run that cannot proceed stays `running` forever instead of escalating with an
   actionable diagnosis.

## Goals

- **G1 — Survive single lane failure.** Any one lane hanging, exiting, or
  silently stalling must convert to a recoverable job state and be recovered
  (re-spawn / transfer / requeue, same attempt) without an operator, within a
  bounded budget.
- **G2 — Honest liveness.** The control surface must never report a lane healthy
  when its work is not progressing, and never report it stalled while it is
  demonstrably producing output. Every reported state is derivable from a real
  progress signal.
- **G3 — Run to completion or fail loudly.** A run either advances, pauses at a
  genuine human-decision gate, self-recovers, or transitions to an explicit
  `needs_operator` state with a structured, actionable escalation. It never
  silently wedges.
- **G4 — Verified robustness.** Robustness regressions are caught by a
  fault-injection suite + an adapter-conformance suite in CI, not by a live
  dogfood.
- **G5 — Within the existing invariants.** The daemon stays authoritative (D094);
  lanes stay scratch (D005); attempts stay first-class (RFC 0095); the lane
  sandbox stays enforced (RFC 0096). No new cloud/hosted/external dependency.

## Non-goals

- Not a new workflow *shape* or collaboration primitive (that is RFC 0093/0098).
- Not the orchestration driver itself (RFC 0097) — this is the substrate that
  makes orchestration safe; RFC 0097 is the consumer.
- Not making LLM agents themselves reliable — we treat them as Byzantine and
  contain them. Robustness is a property of the *harness*, not the worker.
- Not eliminating human-decision gates — accept/override/unrecoverable decisions
  are *meant* to pause. The goal is to remove the *unintended* babysitting, not
  the intended approvals.

## Robustness model

A run is modeled as a set of jobs, each progressing through attempts (RFC 0095),
each attempt worked by one supervised lane. We add two invariants the daemon
checks continuously:

- **Lane lifecycle invariant.** A lane is in exactly one of:
  `starting → live(working) → {completing, idle} → terminal`. Every transition
  has a bounded deadline and a defined failure edge; there is no state in which a
  lane can sit indefinitely without either making progress, being recovered, or
  being declared terminal.
- **Run-health invariant.** A run is *healthy* iff at least one of: (a) some job
  is making progress (a fresh progress signal within its window), (b) the run is
  cleanly paused at a human-decision gate, (c) the run is terminal. A run that is
  none of these for longer than a bounded grace period is **stuck** and must be
  driven (recovery → escalation). "Stuck but `running`" is the bug class this RFC
  eliminates.

Defense in depth — five layers, each a phase:

### Layer 1 — Honest liveness (Phase 1)

Fuse three independent progress signals instead of relying on protocol
timestamps alone:

1. **Protocol** — MCP heartbeats, leases, await/ack (existing,
   `pkg/sessionliveness`).
2. **PTY activity** — the supervisor-helper records the timestamp of the last
   byte the child wrote to the PTY, and the boundaries of an in-flight tool call
   (`agent_started`/`progress`/`agent_exited` already exist;
   `pkg/supervisor/helper.go`). This is the signal the daemon is blind to today.
3. **Filesystem progress** — the most-recent mtime under the job's
   `write_scope.allowed_paths`.

The classifier reports a *precise* state, not a binary healthy/stalled:
`working_protocol` · `working_local` (PTY/FS active, protocol quiet — the #80
case) · `working_tool` (inside an MCP/tool call, with a visible `since` + a
deadline — the #83 case) · `quiet` (no signal, pre-deadline) · `stalled` (quiet
past deadline) · `dead` (pane gone). The helper **auto-heartbeats the lease while
the child is producing output**, so honest local work never trips the
lease-heartbeat stall. Delivery state is reported honestly (the #67 principle,
generalized: a benign observer exit is not a degradation). Closes #80/#83;
subsumes the RFC 0091 liveness tail.

### Layer 2 — Lane-failure containment + adapter conformance (Phase 2)

Make the lane boundary a *contract* every adapter must satisfy, verified in CI:

- **Persistent turn-driver.** For one-shot CLIs (agy/gemini), the supervisor-helper
  detects process exit while the job is unfinished and re-spawns the CLI with a
  continue-prompt, owning turn boundaries so a single-shot lane self-drives a
  multi-turn job. Generalizes F42/F44; closes #95.
- **Bootstrap-delivery contract.** Every TUI-buffering adapter delivers its
  bootstrap via argv (not type-then-CR), proven per adapter (the #101 fix for
  claude, already landed, becomes a contract clause).
- **Lane-env hardening.** Disable surveys/telemetry, force foreground MCP (no
  background discovery probe), allowlist env, keep operational material out of the
  work tree (RFC 0096; closes #76/#85/#70 deterministically rather than by prompt
  steering).
- **Adapter conformance harness.** A fixture run each adapter must pass end to
  end against the *actually installed* CLI — bootstrap submits → `tools/list` →
  claim → a multi-turn job with an interrogation → publish → complete → loop —
  run in CI. A CLI version bump that breaks bootstrap or the turn loop then fails
  CI, converting "discover the regression in a live run" into "catch it before
  merge." This is the single highest-leverage robustness mechanism for the lane
  boundary.

### Layer 3 — Bounded autonomous self-recovery (Phase 3)

A daemon-side **recovery supervisor** (policy loop, crash-safe — state in PG)
continuously evaluates every active run against stuck-state detectors and applies
bounded recovery without an operator:

| Stuck state | Recovery action | Budget |
|---|---|---|
| dead lane, unfinished job | re-spawn the lane (Layer 2) | N respawns/job |
| stale lease past deadline | transfer + requeue to a fresh session, **same attempt** (#82/#108 generalized) | N/job |
| no-progress window (Layer 1) | nudge → re-bridge (#67) → re-spawn | bounded |
| leaked interrogation window | close the panel-owned window (#65 P1) | — |
| orphaned blocker / duplicate lease | reconcile (RFC 0095 `reopenJobForAttempt`) | — |

Every action is idempotent and re-entrant. Recovery preserves the attempt
wherever the failure was operational (a dead lane is not a content failure); it
bumps the attempt only when the *work* must be redone. Only when a per-job /
per-run budget exhausts does it hand off to Layer 4. This is the heart of "no
human babysitter."

### Layer 4 — Loud structured escalation (Phase 4)

When recovery is exhausted, the run transitions to an explicit `needs_operator`
state (not `running`) and emits a structured **escalation artifact** naming the
exact stuck job, the failure class, the recovery attempts made and why they
failed, and the operator's concrete options (re-prepare / transfer / cancel).
The run-health invariant is checked continuously; a `no_work` that is structural
(a same-role `fresh_session_required` job a spent session can never claim, #107,
already landed) is reported with its reason rather than polled silently. The
operator UI surfaces stuck runs. A run never silently stalls — it self-heals or
stops *loudly* with a diagnosis a human (or RFC 0099 constrained operator) can
act on in one step.

### Layer 5 — Fault-injection robustness suite (Phase 5)

A chaos harness runs a representative workflow against the daemon with injected
faults — kill a lane mid-turn, drop an MCP call, inject a 40P01 deadlock, make a
CLI one-shot-exit, stall a lane silently, leak an interrogation window, hand a
bad packet path — and asserts the run **self-recovers (Layer 3) or escalates
loudly (Layer 4) within bounded time, never silently wedging.** Together with the
Layer 2 conformance harness, this is the robustness regression gate: a change
that reintroduces a silent wedge fails the suite. This is how robustness stops
being a property we re-discover by dogfood and becomes a property we *test*.

## Phasing

- **Phase 1 — Honest liveness.** Layer 1: supervisor-helper PTY-activity + tool-call
  boundary recording, classifier fusion, lease auto-heartbeat. Closes #80/#83;
  generalizes #67. *(Restart-gated; the long-deferred RFC 0091 tail finally gets
  a frame.)*
- **Phase 2 — Adapter conformance + turn-driver.** Layer 2. Closes #95/#85/#76/#70
  deterministically; promotes the #101 fix to a contract clause.
- **Phase 3 — Autonomous recovery.** Layer 3: the recovery supervisor + budgets +
  the recovery-action library. Generalizes #82/#108/#65/#84.
- **Phase 4 — Loud escalation.** Layer 4: run-health invariant, escalation
  artifact, `needs_operator` state, operator-UI surfacing.
- **Phase 5 — Chaos suite.** Layer 5: the fault-injection + conformance gate.

Phases 1–2 are the prerequisite substrate (and remove the lane-boundary class
that has blocked every prior pass). Phase 3 is the payoff (unattended recovery).
Phases 4–5 make it durable and self-proving. Each phase lands behind its own
tests; the chaos suite (Phase 5) is what lets later changes be made *without* a
live dogfood — closing the loop RFC 0097 named.

## Relationship to existing RFCs

This is the **umbrella** that frames the robustness slices and supplies the
connective tissue they lack:

- **RFC 0091 (lane health)** → Layer 1 is its completion (the PTY-activity signal
  it lacked).
- **RFC 0095 (attempt-scoped lifecycle)** → the state-machine substrate Layer 3
  recovers *on*; `reopenJobForAttempt` / attempt-scoped artifacts are the
  primitives recovery composes. (Hard dependency.)
- **RFC 0096 (lane trust boundary)** → the safety substrate Layer 2 hardens within;
  a robust lane is also a contained lane. (Hard dependency for Phase 2.)
- **RFC 0097 (run orchestration)** → the **consumer**. Autonomous orchestration is
  only viable on a robust substrate; RFC 0097 already declares it needs this. This
  RFC supplies the missing "what happens when a lane fails mid-orchestration"
  answer, and the chaos suite that lets RFC 0097 be built by an orchestrated
  dogfood (the self-hosting milestone).
- **RFC 0099 (constrained operator)** → consumes Layer 4: a structured escalation
  is exactly what a constrained AI operator acts on.

## Open questions

- **OQ1.** PTY-activity recording (Layer 1) must distinguish *meaningful* output
  from spinner/redraw noise (a TUI repaints constantly). Threshold on byte-delta?
  On distinct frames? Needs measurement against the real CLIs.
- **OQ2.** Recovery budgets (Layer 3): per-job respawn/transfer counts, the
  no-progress window length, and the escalation grace period are policy knobs.
  Defaults vs. workflow-declared overrides?
- **OQ3.** The turn-driver continue-prompt (Layer 2) for one-shot CLIs: re-spawn
  with a "call work.await_packet again" nudge vs. the adapter's own
  session-resume? Measure which the installed agy/gemini actually honors.
- **OQ4.** Does the recovery supervisor run inside the daemon (a policy loop) or
  as a separate supervised process? In-daemon is simpler and crash-couples to the
  authoritative state; a separate process is more isolated. Lean in-daemon (D094).
- **OQ5.** Chaos-suite fault injection (Layer 5) needs hooks to kill lanes / drop
  MCP calls deterministically. A test-only fault-injection seam in the
  supervisor-helper + MCP server, gated behind a build tag or env, never compiled
  into the shipped daemon.

## Acceptance — how we will know the run is robust

The bar is behavioral, not feature-count:

1. **Conformance (Phase 2):** each adapter passes the end-to-end conformance
   fixture in CI against the installed CLI; a CLI bump that breaks bootstrap fails
   CI.
2. **Unattended recovery (Phase 3):** the chaos suite kills a lane in every job
   type of a representative run and the run completes with **no operator action**,
   attempts bumped only where work was actually redone.
3. **Honest liveness (Phase 1):** for every injected fault, `supervise status`
   reports the true state within one classification window — never `healthy` over
   a dead pane, never `stalled` over an actively-producing one.
4. **Loud failure (Phase 4):** an *unrecoverable* injected fault drives the run to
   `needs_operator` with a structured escalation within the grace period — never a
   silent `running`.
5. **Self-hosting (the milestone):** a striatum runner-fix is developed by an
   orchestrated dogfood *through* the runner — the loop RFC 0097 named as blocked
   "because the runner is not robust enough to carry its own development" is
   closed.

[RFC 0091]: 0091-lane-health-module.md
[RFC 0095]: 0095-revision-safe-workflow-lifecycle.md
[RFC 0096]: 0096-supervised-lane-trust-boundary.md
[RFC 0097]: 0097-full-workflow-run-orchestration.md
