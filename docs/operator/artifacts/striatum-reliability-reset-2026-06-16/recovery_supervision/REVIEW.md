---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: high
tags:
  - recovery
  - supervision
  - liveness
  - run-drive
author: recovery-auditor-codex-via-live-striatum-mcp-001
run_id: run_8489e7d2df3b56e1ed7fdb49ff5c8ba7
session_id: sess_535d20f5382daaf0fd6a9664ebc65efd
title: Recovery And Supervision Audit
---

# Recovery And Supervision Audit

## Most painful recovery/supervision failure modes

1. **Confirmed-dead lanes with durable artifacts still fall into operator ceremony.**
   The current operator brief records repeated `agent_exited_unsealed` /
   `recovery_exhausted` incidents that were finalized by `recovery
   complete-stalled`, including a recurrence where the renewed lease forced about
   a 31-minute wait before `complete-stalled` would accept the job
   (`docs/operator/BRIEF.md:40-62`). That is the clearest reliability reset
   target: once the daemon can prove the lane is dead, the expected artifact is
   durable, front matter/byline validate, and provenance evidence is
   reconstructable, recovery should finalize or requeue from those facts rather
   than wait for lease time.

2. **The no-work exit boundary is correct, but it makes run-drive correctness
   load-bearing.** `work.await_packet` deliberately returns `idle_behavior:
   "exit_session"` on terminal idle, and later work is delegated to `run drive`
   or a scheduler (`docs/reference/spec.md:853-868`). `run drive` is only a
   CLI-local loop over daemon primitives, not a daemon method
   (`docs/reference/spec.md:1585-1594`; `docs/reference/command-authority-matrix.md:241-242`).
   Any missed wake, stale spawn authorization, driver crash, or stale local
   operator loop therefore becomes an operational wedge even when the database
   state is internally consistent.

3. **Supervision has too many "visible later" failure paths.** The spec still
   admits lane commands where `supervise start` happens but the run silently
   fails to advance until `doctor` reports `supervisor_lost_with_held_lease`
   (`docs/reference/spec.md:2626-2633`). Delivery degradation is also recorded
   and causes later sends to fail closed until rebridge or restart
   (`docs/reference/spec.md:2440-2453`, `docs/reference/spec.md:2588-2594`),
   but the automatic repair path is narrower than the status/projection path.
   A local-first runner can tolerate explicit failure; it should not require the
   operator to notice a silent non-advancing pane.

## Timing/liveness/lease hazards

- **Lease expiration remains too central to recovery decisions.** The glossary
  correctly defines leases as a safety mechanism, not liveness proof
  (`docs/reference/ubiquitous-language.md:124-128`). The spec says lazy expiry
  and `recovery.sweep` apply stale-lease policy (`docs/reference/spec.md:1677-1685`,
  `docs/reference/spec.md:1712-1722`), while supervisor status separately knows
  pane/process identity, stale progress, and delivery state
  (`docs/reference/spec.md:2512-2536`). The #292 pattern shows these signals are
  not yet composed strongly enough: a renewed unexpired lease can block recovery
  even after the lane is proven dead and outputs are present.

- **Leaseless hung sessions are recognized, but the remedy is only partially
  automatic.** The spec now says `recovery.sweep` scans queued jobs bound to a
  hung supervised session, closes the hung session when it is honestly stalled
  or dead, and surfaces `leaseless_count` in dashboard stalls
  (`docs/reference/spec.md:2557-2570`). That closes a major blind spot, but it
  must be tested through `run drive`: the next driver iteration has to spawn or
  adopt the replacement session exactly once, with no duplicate claims and no
  lost queued work.

- **Wake hints are deliberately non-authoritative, so missed-notification
  fallback is part of correctness.** The wake bus only hints that committed
  state may have changed and must never authorize transitions
  (`docs/reference/spec.md:863-868`; `docs/reference/command-authority-matrix.md:120-123`).
  That is the right boundary, but the driver must prove it re-reads state after
  every wake and after timeout fallback, otherwise no-work exit simply moves
  polling bugs from lane agents into the operator driver.

## Silent-degrade hazards

- **Doctor/status can know about a stuck run while the run still appears
  structurally valid.** Doctor surfaces `supervisor_lost_with_held_lease`,
  non-healthy reattach states, stale supervisor heartbeat, and
  lease-expired heartbeat stalls (`docs/reference/spec.md:2603-2624`), while
  status reports next actions (`docs/reference/spec.md:1596-1606`). Those are
  useful diagnostics, but they are not sufficient recovery. A run should not
  require a human or AI operator to independently correlate `doctor`, `status`,
  `supervise.status`, and `recovery complete-stalled` before it moves again.

- **Operator-local PTY logs are useful but intentionally non-authoritative.**
  The docs are clear that `.striatum/scratch/<supervisor_id>/pty.log` is private
  diagnostics and is not parsed for workflow state, provenance, completion, or
  recovery (`docs/reference/spec.md:2662-2674`). Recovery fixes must therefore
  be based on helper events, supervisor rows, leases, artifacts, and frozen
  provenance stamps, not on terminal text or manual transcript inspection.

- **Experimental workflow shapes still imply operator supervision.** The
  catalog labels conversation, custom, iterated interrogating panel, and
  multi-phase shapes as experimental and says to expect supervision
  (`docs/reference/workflow-catalog.md:55-63`,
  `docs/reference/workflow-catalog.md:96-103`,
  `docs/reference/workflow-catalog.md:216-223`,
  `docs/reference/workflow-catalog.md:274-281`). Divergent ideation has a real
  reliability fixture proving double fan-out/join recovery
  (`docs/reference/workflow-types.md:534-541`;
  `go/pkg/adapterconformance/divergent_ideation_test.go:18-31`), but that
  fixture exercises dead-branch requeue, not the final unsealed-artifact
  auto-finalization failure mode now showing up in live runs.

## Places where recovery should become automatic, louder, or simpler

1. **Automatic:** make `recovery.sweep` auto-finalize a confirmed-dead,
   unsealed job when all declared artifacts are durable, schema-valid,
   byline-valid, and provenance-reconstructable. This is the #308/#309 shape
   described by the operator brief (`docs/operator/BRIEF.md:88-104`) and should
   happen before lease expiry when session/process liveness is conclusive.

2. **Automatic:** have `run drive` run or consume the same sweep/stall decision
   before deciding it has no work to launch. The driver already reconciles
   sessions and supervisors (`docs/reference/spec.md:1585-1594`); it should not
   leave an obvious `recover_orphan_supervisor`, `heartbeat_stall_lease_expired`,
   or leaseless hung-session state for a separate ceremony unless the recovery
   decision is unsafe.

3. **Louder:** convert supervision command-contract violations into start-time
   refusal where possible. The spec says missing lane requirements can lead to a
   silent non-advancing run (`docs/reference/spec.md:2626-2633`); workflow
   validation, run preparation, or `supervise.start` should fail before a lane is
   launched if the command is known to be one-shot, lacks agent-loop capability,
   or cannot use the configured MCP control path.

4. **Simpler:** collapse common operator recovery guidance into one structured
   next action per stuck run. If the correct path is "sweep will auto-finalize",
   "rebridge then resend", "register a fresh session", or "human checkpoint",
   status/dashboard/why should name exactly that. Avoid asking operators to infer
   it from several partially overlapping projections.

## P0/P1 fixes with exact tests

### P0

- **P0-1: finalize by liveness, not lease timeout, for reconstructable
  `agent_exited_unsealed` jobs.** Add
  `go/pkg/mutations/recovery_sweep_supervision_test.go::TestSweepAutoFinalizesAgentExitedUnsealedWithDurableArtifactBeforeLeaseExpiry`.
  Seed a running job with an active unexpired lease, an attached supervisor that
  has emitted `agent_exited`, a durable expected artifact with valid
  `striatum.finding.v1` or `striatum.synthesis.v1` front matter, matching
  byline, and lane evidence. Run `mutations.SweepRun`. Expect
  `artifact.auto_finalized` and `job.auto_finalized`, downstream readiness or run
  completion, no `recovery_exhausted`, and no requirement to wait for lease
  expiry.

- **P0-2: make leaseless hung-session recovery drive a replacement lane.** Add
  `go/pkg/cli/rundrive/drive_supervision_test.go::TestRunDriveReplacesLeaselessHungBoundSessionOnce`.
  Seed a queued job that is uniquely bound to an active supervised session whose
  heartbeat is past the idle deadline and whose pane/process is dead, with no
  active lease. Run one `run drive --once` equivalent over the daemon RPC
  handlers. Expect the stale session closed/lost, exactly one fresh eligible
  session started, the queued message still pending or claimed by that session,
  and no duplicate supervisor rows.

- **P0-3: fail closed on non-viable supervised lane commands before launch.** Add
  `go/pkg/workflowvalidate/supervised_lane_contract_test.go::TestSupervisedAgentLoopRejectsOneShotCommandWithoutOverride`
  and a `supervise.start` handler test with the same fixture. A process lane with
  `adapter_capabilities.agent_loop: true` but a known one-shot command shape
  should fail validation/start with a structured suggestion, not launch and rely
  on `doctor` to discover `supervisor_lost_with_held_lease` later.

### P1

- **P1-1: prove wake hints cannot strand run-drive.** Add
  `go/pkg/cli/rundrive/drive_wake_test.go::TestRunDriveRereadsCommittedStateAfterWakeTimeoutAndDuplicateHint`.
  Exercise missed wake, duplicate wake, and timeout fallback. The driver must
  re-read run/session/job state before spawning and must not double-start lanes.

- **P1-2: make degraded delivery repairable in the driver loop.** Add
  `go/pkg/cli/rundrive/drive_supervision_test.go::TestRunDriveRebridgesLivePaneWithDegradedDelivery`.
  Seed an attached tmux-backed supervisor with live pane identity and
  `delivery_liveness.class=degraded` from `attach_client_exited`. Expect the
  driver to call the same handler as `supervise.rebridge` or report a single
  structured restart action; it must not leave the run idle with only a status
  hint.

- **P1-3: extend the divergent-ideation reliability fixture to final unsealed
  artifacts.** Add
  `go/pkg/adapterconformance/divergent_ideation_test.go::TestDivergentIdeationFinalSynthesisAgentExitedUnsealedAutoFinalizes`.
  Reuse the existing double fan-out/join fixture, drive through both fan-outs,
  create the final artifact, then inject `agent_exited_unsealed` on
  `final_synthesis` before completion. The sweep should finalize or restore the
  run without operator intervention and without violating the final join.

## What not to build until this is stable

- Do not promote daemon auto-spawn to a broad default. The command matrix says
  it depends on run-scoped spawn authorization grants and refuses missing or
  expired grants (`docs/reference/command-authority-matrix.md:241-242`); it
  should stay opt-in until run-drive plus recovery sweep handle the current
  no-work, liveness, and lease edges without operator rescue.

- Do not expand durable transcript capture, memory-driven state transitions, or
  terminal-log-based recovery. The product boundary says provider output and PTY
  logs are diagnostics, not workflow truth (`README.md:19-22`;
  `docs/reference/spec.md:2667-2674`).

- Do not add more experimental multi-agent shapes as default recommendations
  until the recovery spine is green. The next reliability work should deepen the
  liveness/auto-finalization/driver fixtures, not add more DAG complexity for
  operators to supervise.

- Do not make UI convenience the main fix. A better dashboard button for
  `complete-stalled` is useful after the fact, but the P0 is that reconstructable
  dead-lane jobs should not need that button.
