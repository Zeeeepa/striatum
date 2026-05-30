# RFC 0097: Full Workflow Run Orchestration — autonomous end-to-end run execution

Status: proposed
Date: 2026-05-30
author: proposer-claude-opus-4-8-001

Context:
- This session drove the RFC 0094 design→build dogfood
  (`run_c6d66b69303872b0863b6c8faa0a7e69`) by hand: ~15 `register-session` +
  `supervise start` calls across phases, manual phase-boundary watching, manual
  lane retirement between phases, manual revision-cycle recovery. A single
  3-lane design→build run is hours of operator bookkeeping — the *freestyling*
  the runner (RFC 0034) was meant to end, reintroduced at the run-driving layer.
- [RFC 0095](0095-revision-safe-workflow-lifecycle.md) — **hard prerequisite.**
  Autonomous orchestration multiplies whatever the lifecycle does; until the job
  **attempt** is first-class (coherent re-open, closed-session claim guard,
  attempt-scoped recovery, session auto-provision, panel-owned interrogation
  window), a self-driving run just hits the #65/#75/#81/#82/#84 faults faster.
- [RFC 0096](0096-supervised-lane-trust-boundary.md) — **hard prerequisite.**
  Auto-launching many lanes amplifies the trust-boundary exposure (#87/#70/#86);
  the env allowlist + work-tree hygiene must land before the orchestrator spawns
  lanes unattended.
- [RFC 0083](0083-iterated-panel-review-with-interrogation.md) / [RFC 0084](0084-interrogable-agent-loop-attestation-and-chat-ui.md)
  — the iterated interrogating panel + `awaiting_interrogation` the orchestrator
  must sequence (design panel → synth → build panel).
- [`docs/decisions/decision-log.md`](../decisions/decision-log.md) — D094
  (daemon-owned PG is the sole authority; lanes are daemon-owned), D005 (the
  coordinator owns next-action selection, work assignment, and invoking workflow
  commands — this RFC is that responsibility made concrete and automatic).

## Problem

A prepared+started run is a DAG of jobs across phases with declared roles, lanes,
parallelism, and revision cycles. Executing it today requires the **operator** to
manually, for every job in every phase: register a session (correct role + lane +
capabilities + `--fresh`), `supervise start` it, watch the dashboard for the
phase boundary, retire the finished lanes, register + launch the next phase's
sessions, detect and recover revision cycles, and resolve checkpoints. The
runner knows the entire graph — which jobs are ready, which role/lane each needs,
when a phase completes — yet none of that drives lane provisioning. The operator
is a manual scheduler for a graph the daemon already holds.

Two costs follow:

1. **It doesn't scale and is error-prone.** ~15 manual session launches for one
   design→build run; a mis-typed capability or a missed `--fresh` strands a job;
   a missed phase boundary stalls the run. The operator's attention goes to
   bookkeeping, not to the decisions only a human should make.
2. **It hides the lifecycle bugs behind operator heroics.** The Engram forum run
   only *completed* because the operator hand-applied the workarounds in
   #75–#87. Manual driving masks how broken the runner is; it also means every
   run's success depends on operator skill, not the runner.

### The dogfood paradox (answering "can they be dogfooded?")

The natural Striatum way to build a feature is a dogfood — a full workflow run.
But **full workflow runs are the broken thing**: #65/#75/#81/#82/#84 are *runner*
faults, and the RFC 0094 dogfood wedged on them. **You cannot reliably dogfood
the fixes-to-the-runner through the broken runner.** So the RFC 0095/0096 Phase 1
fixes are **bootstrapped via subagents** (operator-driven, worktree-isolated,
integrated + reviewed + merged), not via a dogfood. Once the lifecycle is
coherent (RFC 0095) and lanes are sandboxed (RFC 0096), full runs become
reliable, and the delivery model **flips**: this RFC's orchestrator is the layer
that makes a full run the default vehicle, and the **self-hosting milestone** is
when RFC 0097 itself is built and validated by an orchestrated full run. The
bootstrap order is load-bearing, not incidental.

## Goals

1. **One command drives a whole run.** `striatum run execute --run-id <id>` (or a
   daemon-side run-driver) takes a prepared+started run from first phase to
   terminal state, auto-provisioning and auto-launching the lanes each ready
   phase needs, advancing on completion, and retiring finished lanes — with **no**
   manual `register-session`/`supervise start` per job.
2. **The operator handles decisions, not bookkeeping.** The orchestrator pauses
   and surfaces **only genuine human-decision gates** — `human_checkpoint`
   blockers, accept/override decisions, unrecoverable failures — to the operator
   (dashboard + a decision queue). Everything mechanical (session lifecycle,
   phase advance, interrogation sequencing, bounded revision cycles) is automatic.
3. **Correct by construction on the fixed lifecycle.** The orchestrator is built
   *on* RFC 0095: it auto-provisions fresh sessions for `fresh_session_required`
   re-opens, respects the closed-session claim guard, sequences the panel-owned
   interrogation window, and drives bounded `needs_revision` cycles without
   stranding.
4. **Crash-safe and resumable.** The run's authoritative state is the daemon's PG
   (D094); the orchestrator is a stateless driver that can be killed and resumed
   — it reads the run graph + job states and continues. Re-running `run execute`
   on a partially-driven run is idempotent.
5. **Respect the product boundary.** Lane launching stays daemon-mediated (the
   existing supervisor path); the orchestrator automates the operator's
   `supervise start` calls, it does not introduce a new process-spawning
   authority. No hosted service; local-first.

## Non-Goals

- **Not removing human-decision gates.** Accept/override, risk acceptance, and
  `human_checkpoint` resolution stay human (D005/D157). The orchestrator routes
  them to the operator; it does not auto-decide them.
- **No auto-merge / auto-land.** The orchestrator drives the *run*; landing the
  run's output to `main` stays a separate, gated step.
- **No scheduling beyond the declared graph.** Round count, parallelism, and
  revision budgets are as authored (RFC 0045); the orchestrator executes the
  declared shape, it does not invent new parallelism (D015 deferred AI build
  parallelization stays deferred).
- **No new lane-launch primitive.** Reuses `supervise start` / the agent-loop;
  this RFC is orchestration *policy* over existing methods.

## Proposal (sketch — design to be sharpened by the first orchestrated dogfood)

### 1. The run orchestrator

A driver — `striatum run execute --run-id <id>` (operator-side) and/or a
daemon-side `run.execute` policy — loops:

1. Read the run graph + job states (the daemon already exposes this).
2. For each **ready** job (deps satisfied, queued) not yet provisioned: register
   the session for its `(role, lane)` with the declared capabilities + `--fresh`
   when required (RFC 0095 §5 auto-provision + parallel distinct sessions), then
   `supervise start` it (RFC 0096-sandboxed env).
3. Monitor completion; on a phase's completion, retire its lanes and advance.
4. For an interrogable job, hold the panel-owned window (RFC 0095 §4) until the
   declared reviewer set finishes, then sequence the gate.
5. On `needs_revision` within budget, drive the bounded cycle (RFC 0095 §3
   `reopenJobForAttempt` + auto-provision the fresh revision session).
6. On a `human_checkpoint` / unrecoverable failure, **pause** and surface a
   decision to the operator; resume on resolution (incl. the D157 `override`).
7. Terminate when the run reaches a terminal state; emit a run report.

### 2. Concurrency + lane-launch policy

Honor the workflow's declared `parallelism.max_active_jobs` and disjoint
write-scopes; cap concurrent supervised lanes (resource limit, OQ). Model pinning
+ lane commands come from the workflow `lanes` block (as today).

### 3. Operator decision surface

A compact, pollable **decision queue** (extends `striatum dashboard`): the open
checkpoints / pending decisions / failures the orchestrator is blocked on, each
with the safe actions (continue / override+decision-id / cancel). The operator
works the queue; the orchestrator does the rest.

### 4. Observability

Each orchestrator action emits an event (already the model); `dashboard
--run-id` shows the live phase, active lanes, and pending decisions. The run
report summarizes phases, verdicts, cycles, and operator decisions.

## Acceptance Criteria

1. **End-to-end with no manual session ops.** `run execute` drives the 3-lane
   design→build dogfood (the RFC 0094 shape) from start to terminal with the
   operator issuing **zero** `register-session`/`supervise start` calls and only
   resolving genuine checkpoints. Reproduces this session's run without the manual
   labor.
2. **Lifecycle-correct.** A `needs_revision` cycle within budget is driven
   automatically (fresh session auto-provisioned, closed sessions never reclaim,
   panel window held for all reviewers) — i.e., RFC 0095's ACs hold under
   automation.
3. **Pauses only at human gates.** A `human_checkpoint` (e.g. budget-exhausted
   revision) pauses the orchestrator and surfaces a decision; resolving it
   (incl. `override --decision-id`) resumes the run.
4. **Crash-safe/resumable.** Killing and re-running `run execute` mid-run resumes
   without double-provisioning or stranding; idempotent against the daemon state.
5. **Sandboxed.** Auto-launched lanes use the RFC 0096 allowlist env + out-of-tree
   operational material; no DSN/secret reaches a lane, no work-tree pollution.
6. **Self-hosting milestone.** Once RFC 0095/0096 Phase 1–2 land, RFC 0097 is
   itself built and validated by an **orchestrated** full dogfood run (the
   delivery model has flipped from subagent-bootstrap to dogfood).
7. **Boundary preserved.** Lane launching stays daemon-mediated; no new
   process-spawn authority; D094/D005 intact; Go-only guardrails (RFC 0078) green.

## Open Questions

1. **Operator-CLI driver vs daemon run-driver.** Is `run execute` a foreground
   operator process (simple, but dies with the terminal) or a daemon-owned policy
   (survives, but the daemon spawning lanes unattended is a bigger authority
   step)? Proposal: ship the operator-CLI driver first (it just automates the
   calls the operator makes today, crash-safe via daemon state); promote to a
   daemon policy once proven.
2. **Decision-queue surface.** Extend `dashboard` with a decision queue, or a new
   `run decisions` read + a notify hook? Proposal: a `dashboard` panel + a
   `run decisions --run-id` read for scripts.
3. **Resource limits.** Beyond `max_active_jobs`, do we cap total concurrent
   supervised lanes / model spend per run? Proposal: a configurable concurrent-
   lane cap, default = `max_active_jobs`.
4. **Partial autonomy.** Should the operator be able to run *some* phases
   manually and hand others to the orchestrator (e.g. drive design by hand,
   auto-run the build panel)? Proposal: yes — `run execute` is idempotent against
   daemon state, so it can pick up a partially-driven run.
5. **Failure policy.** On a lane that fails (not `needs_revision`), does the
   orchestrator retry within a budget, or always escalate to the operator?
   Proposal: escalate by default; a per-job declared retry budget is a follow-up.
