---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: high
tags:
  - reliability
  - recovery
  - leases
  - worktree
  - conformance
author: reliability-auditor-codex-via-live-striatum-mcp-001
run_id: run_8489e7d2df3b56e1ed7fdb49ff5c8ba7
---

# Reliability Spine Audit
author: reliability-auditor-codex-via-live-striatum-mcp-001
date: 2026-06-16
verdict: accept_with_findings

## Verdict

Executive verdict: simplify-before-growth. The core architecture is still viable:
daemon-owned Postgres is the right authority boundary, `doctor` is again capable
of being a stop-and-fix signal, and recent branch/worktree anchoring work closed
real data-loss classes. The reset should pause growth until the recovery spine,
completion-redrive locking, and conformance fixtures prove the live failure modes
that escaped `divergent_ideation`.

Scope note: this was a fresh document-scoped review. I used the packet-listed
context documents and packet-listed source/test files, plus the packet prompt and
its command list. I did not broaden into unlisted mutation or read source files.

Command summary:

| Command | Result |
|---|---|
| `git status --short --branch` | Started on `main...origin/main`; later bootstrap saw unrelated untracked review artifacts already present outside this review path. |
| `striatum operator bootstrap --markdown` | Daemon reachable, but CLI authorization failed; bootstrap reported VERSION/latest tag `2.33.0`, active runs 0, claimable jobs 0, open blockers 0. |
| `striatum status --json` | Failed: `daemon RPC authorization failed`. |
| `striatum doctor --verbose --json` | Failed: `daemon RPC authorization failed`; I could not independently verify doctor from the CLI in this lane. |
| `striatum workflow validate docs/operator/workflows/striatum-reliability-reset-2026-06-16/workflow.json --json` | Passed: `valid: true`. |
| `make lint` | Passed: `golangci-lint` reported `0 issues`. |
| `make typecheck` / `make test` | Both failed in `go/pkg/mutations`: `TestSpawnRunAsSpecResolvesLaneUser` got `mode:"daemon_user", run_as_user:""` where the test wanted `lane_user striatum-lane`. |

## Top Reliability Failure Modes

| ID | Severity | Failure mode | Current guardrail | Why it failed / remains weak | Disposition |
|---|---:|---|---|---|---|
| F1 | P0 | Confirmed-dead, artifact-reconstructable lanes still wait on lease-time recovery. | `recovery.sweep`, `recovery.complete-stalled`, doctor/liveness projections. | Recent live runs repeatedly hit `agent_exited_unsealed -> recovery_exhausted`, and `complete-stalled` refused until a renewed lease timed out. The operator had to finalize green artifacts manually through daemon recovery. | Fix #308/#309/#302 before new reliability features. Completion eligibility should be based on proven dead session plus durable/reconstructable outputs, not on waiting out an otherwise-known-dead lease. |
| F2 | P0 | One completion redrive path is still documented as outside the per-run `lockRun` invariant. | RFC 0104 says every per-run mutation takes `lockRun` first and a guard test asserts that. | The current brief still names `escalation_resolve.go:156` as re-driving completion without `lockRun`, and the next actions still call out escalation-redrive lock coverage. | Route escalation resolution through the same locked completion helper and add guard coverage for that handler. |
| F3 | P0 | The `divergent_ideation` supported-tier fixture does not exercise the artifact, worktree, supervisor, or run-drive surfaces that failed in live runs. | RFC 0105 reliability fixture and supported-tier catalog claim. | The fixture proves double fan-out/join scheduling over direct handlers and no required artifacts. Recent live divergent runs wedged at final artifact/completion recovery, so the green fixture did not represent the escaped defect class. | Demote or mark conditionally supported until the fixture includes real artifacts, run-drive/supervisor lifecycle, and final-job unsealed-exit recovery. |
| F4 | P0 | The current local test gate is red in a mutation package tied to run-as/spawn semantics. | `make typecheck` and `make test`. | Both prompt-level test commands failed on `TestSpawnRunAsSpecResolvesLaneUser`. Even if this is environment-specific, the release gate is not green in the active lane. | Fix or explicitly quarantine the test before treating v2.34 work as releasable. |

## Evidence Table

| Claim | Evidence | Confidence |
|---|---|---|
| The runner still hits `agent_exited_unsealed` followed by recovery exhaustion and delayed `complete-stalled`. | `docs/operator/BRIEF.md:40-48` says the doctor P1 dogfood hit #289 again and #292 recurred because `complete-stalled` refused until the apply lease expired after about 31 minutes. | High |
| Two recent `divergent_ideation` runs wedged at final job and needed `recovery complete-stalled`. | `docs/operator/BRIEF.md:49-61` names #290 and #296, says both were finalized by `complete-stalled`, and files #308 for autonomous sweep to auto-finalize reconstructable `agent_exited_unsealed` jobs. | High |
| The currently tracked recovery cluster is #308/#309/#302 and is held for v2.34.0. | `docs/operator/BRIEF.md:89-104`. | High |
| Sweep handles stale attached supervisors, but the decisive blocker path is still lease-expiry shaped. | `docs/reference/spec.md:2557-2578` describes stale heartbeat surfacing and the `heartbeat_stall_lease_expired` blocker after the active lease has expired. | Medium-high |
| `recovery.complete_stalled` is the explicit recovery tool for recovery-exhausted durable jobs, not an ordinary happy path. | `docs/reference/command-authority-matrix.md:190`. | High |
| The `lockRun` invariant is absolute by contract. | `docs/reference/spec.md:758-778` requires every run-scoped mutation to take `lockRun` first and says surfaced `40P01` is a should-never-happen signal. | High |
| A documented open P0 says escalation redrive still misses `lockRun`. | `docs/operator/BRIEF.md:173-176` and `docs/operator/BRIEF.md:231-234`. | Medium-high, because this review did not open the named source file. |
| `divergent_ideation` is advertised as supported on a green unattended fixture. | `docs/reference/workflow-catalog.md:105-112`; `docs/reference/workflow-types.md:537-541`. | High |
| The generated shape is artifact-bearing and multi-stage. | `go/pkg/workflowgenerate/shapes_divergent.go:52-138` emits problem, branch, convergence, deepen, and final synthesis artifacts; `go/pkg/workflowgenerate/shapes_divergent.go:140-154` wires both fan-outs and joins. | High |
| The conformance fixture intentionally bypasses the artifact contract. | `go/pkg/adapterconformance/divergent_ideation_test.go:86-92` says nodes are document-only with no required artifact; `go/pkg/adapterconformance/divergent_ideation_test.go:184-203` drives `work.complete` and `session.close` directly. | High |
| The current test gate is red in run-as spawn semantics. | Command output from `make typecheck` and `make test`: `go/pkg/mutations` fails `TestSpawnRunAsSpecResolvesLaneUser`. | High |

## Gaps In Tests Or Conformance Fixtures

1. Heartbeat and lease behavior remain under-proven at exactly the edge where
   operators feel pain: active-but-dead supervised lanes, renewed leases, and
   reconstructable artifacts should be one integrated recovery test, not separate
   unit checks.
2. The supported-tier `divergent_ideation` fixture proves DAG scheduling, but not
   the live product contract: no `artifact.publish`, no byline/front-matter
   validation, no `worktree.create`, no supervisor packet delivery, no
   `work.await_packet`, and no `run drive`.
3. Completion provenance and escalation resolution need an adversarial concurrency
   test: one path resolving an escalation while another verdict/complete path is
   evaluating `maybe_complete_run`.
4. The current red `spawn_grant_test.go` result means the suite cannot be used as
   the release oracle until the run-as expectation is fixed or the environment
   contract is made explicit.

## P0 Fixes With Exact File/Test Targets

| Fix | Target files/tests |
|---|---|
| Make reconstructable `agent_exited_unsealed` jobs auto-finalize or become immediately operator-actionable without waiting for lease expiry. | Recovery and completion mutation code behind `recovery.sweep` / `recovery.complete_stalled`; add a pgtest covering a dead supervised session, active renewed lease, required artifact present, and final run completion. |
| Close the `lockRun` hole in escalation-driven completion. | `go/pkg/mutations/escalation_resolve.go` path named by the brief; add a guard test that includes escalation resolution in the RFC 0104 lock-first assertion. |
| Raise the `divergent_ideation` graduation gate to cover the escaped live defect. | Extend `go/pkg/adapterconformance/divergent_ideation_test.go` with required artifacts and a final-synthesis unsealed-exit recovery cell; keep `go/pkg/workflowgenerate/shapes_divergent_test.go` for generator shape coverage. |
| Restore green local gates. | Fix `go/pkg/mutations/spawn_grant_test.go::TestSpawnRunAsSpecResolvesLaneUser` or the run-as resolver it covers; then rerun `make typecheck` and `make test`. |

## What To Freeze Until This Is Fixed

- Freeze new supported-tier workflow shape graduations. Experimental shapes can
  remain experimental, but no shape should claim `supported` without exercising
  artifact publication, completion, and recovery paths that match its real graph.
- Freeze daemon auto-spawn expansion. The brief already marks #212 parked; the
  current recovery and run-as test state makes more autonomous spawn authority too
  expensive.
- Freeze doctor warning downgrades except for sha-bound, tested cases. `doctor`
  only stays trustworthy if new warnings are rare, typed, and backed by tests that
  prove unlisted real loss stays red.
- Freeze non-critical feature work until `make test` is green in the active
  environment and the #308/#309/#302 recovery cluster is resolved.

## Doctor Signal Review

`doctor` is moving in the right direction. The brief says D204/D205 moved
historical artifact problems out of `ok=false` while preserving red status for
uncatalogued real loss, and the live doctor was green after #290/#296 cleanup.
That is the right posture: preserved-on-default-branch and sha-acknowledged
historical loss can be warnings; `artifact_anchor_missing_file`,
`artifact_anchor_hash_mismatch`, `artifact_blob_metadata_missing` without a safe
classification, `worktree_head_unreachable`, and
`job_completed_without_anchor` must stay red.

The remaining risk is normalization of warnings. A green doctor plus a pile of
routine warnings trains operators to ignore the surface again. The reset plan
should set a warning budget and make every warning either actionable, explicitly
accepted, or scheduled for deletion.

## Divergent Ideation Postmortem

`divergent_ideation` revealed both a narrow bug and an envelope problem. The
narrow bug is final-job recovery for reconstructable unsealed work. The envelope
problem is that the shape graduated on a conformance fixture that validates graph
readiness but not the real failure plane. A shape whose value comes from serial
fan-out/fan-in, multiple fresh sessions, artifact-heavy synthesis, and
autonomous recovery needs a fixture that drives those surfaces together.

Keep the shape available, but treat `supported` as conditional until the fixture
matches live operation. Minimum re-graduation gate: generated workflow with
required artifacts, real `work.await_packet` or run-drive path, supervised lane
failure after artifact write but before seal, recovery sweep, final completion,
and a doctor check proving no stranded anchors or artifact gaps.

## Architecture Corner Check

The main corner is not "too much Postgres" or "too many daemon methods"; it is
too many nearly-correct recovery paths that encode slightly different authority
tests. `work.complete`, `review.submit`, `recovery.sweep`,
`recovery.complete_stalled`, escalation resolution, supervisor liveness, and
doctor projections all reason about the same fact: whether a job's output is
durable and whether its lane still has legitimate authority. The reset should
collapse those decisions behind fewer shared predicates and prove every caller
uses them under the same run lock.

## Delete / Freeze / Fix Plan

Delete or demote:

- Demote any supported-tier claim whose fixture omits artifacts, supervisors, or
  run-drive behavior that the shape depends on.
- Delete stale compatibility aliases and obsolete service surfaces only after the
  recovery P0s are green; deletion should reduce recovery paths, not create one
  more migration campaign.

Freeze:

- New workflow shapes.
- Auto-spawn.
- Warning-only doctor reclassifications that are not sha-bound and safety-tested.

Fix:

- P0: #308/#309/#302 recovery cluster.
- P0: escalation completion redrive under `lockRun`.
- P0: red `go/pkg/mutations` run-as spawn test.
- P0: divergent ideation fixture widened to artifact + supervisor + recovery.
- P1: warning budget and truth-surface guardrails for README/docs index/authority
  matrix freshness.

## Two-Week Reset Plan

1. Days 1-2: restore green `make test`; fix or quarantine the run-as spawn
   failure with an explicit environment contract.
2. Days 3-5: land #308/#309/#302 as one recovery slice, including the
   active-lease-but-dead-lane reconstructable-artifact test.
3. Days 6-7: close the escalation `lockRun` hole and add the guard test.
4. Days 8-10: upgrade the divergent-ideation reliability fixture to use real
   artifacts and recovery; temporarily mark the support tier conditional if the
   fixture cannot land cleanly.
5. Days 11-12: run `doctor --verbose` from an authorized operator context and
   classify every warning into action, explicit acceptance, or deletion.
6. Days 13-14: cut a release only if `make lint`, `make typecheck`, `make test`,
   workflow validation, and doctor are all green or have documented,
   non-normalized exceptions.

## Definition Of Done

- `make lint`, `make typecheck`, and `make test` pass in the active lane
  environment.
- `striatum doctor --verbose --json` is green from an authorized operator context
  and warning count is intentionally bounded.
- A live-style recovery fixture proves `agent_exited_unsealed` plus durable
  artifact reaches completion without waiting out a renewed lease.
- The RFC 0104 guard covers escalation resolution and every other run-scoped
  mutation that can re-drive completion.
- `divergent_ideation` support tier is backed by a fixture that exercises its
  actual artifact, supervisor, worktree, and final recovery contract.
- No new shapes, auto-spawn authority, or doctor-warning downgrades land before
  the above are true.
