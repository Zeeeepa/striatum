# RFC 0095: Revision-Safe Workflow Lifecycle — coherent job attempts, leases, sessions, artifacts, and interrogation windows

Status: proposed
Date: 2026-05-30
author: proposer-claude-opus-4-8-001

Context:
- [#65](https://github.com/halbritt/striatum/issues/65) — the acute blocker: an
  iterated-interrogating-panel run that hits `needs_revision` on a repo-write
  build job **cannot be driven to a clean revision**. Four distinct faults,
  observed live in run `run_c6d66b69303872b0863b6c8faa0a7e69`.
- [#57](https://github.com/halbritt/striatum/issues/57) — the write-scope
  guardrail is bi-directionally strict (flags `dirty→clean` transitions and
  out-of-scope operator files), wedging `work.complete`.
- [#58](https://github.com/halbritt/striatum/issues/58) — `submit-review` raises
  a raw Postgres unique-constraint error when the finding was already published.
- [#60](https://github.com/halbritt/striatum/issues/60) — session lifetime is
  rigid: re-opened `fresh_session_required` jobs sit queued until the operator
  manually closes prior sessions and launches fresh ones.
- [RFC 0014](0014-process-adapter-completion-guarantees.md) — process-adapter
  completion + the recovery `auto_finalize`/`auto_publish_stale_artifacts`
  machinery (D057) that this RFC must make attempt-aware.
- [RFC 0045](0045-multi-phase-workflow-editor-and-schema.md) — phase gating +
  the `requires_verdict` cross-phase dependency the cycle router drives.
- [RFC 0082](0082-interrogation-sessions.md) / [RFC 0083](0083-iterated-panel-review-with-interrogation.md) /
  [RFC 0084](0084-interrogable-agent-loop-attestation-and-chat-ui.md) — the
  interrogation primitive, the iterated interrogating panel, and the
  `awaiting_interrogation` preserved-context window (D141) whose lifecycle
  Problem 1 breaks.
- [RFC 0093](0093-structured-live-collaboration-workflow-shapes.md) OQ4 / [RFC 0094](0094-deferred-collaboration-shapes-fog-of-war-and-synaptic-prune.md)
  `post_dialog_hook` — "keep participants live through a gate," the same
  liveness-window problem, here for review panels.
- [`docs/decisions/decision-log.md`](../decisions/decision-log.md) — D036
  (recovery refuses repo-write requeue), D057 (process-adapter completion
  guarantees), D141 (`awaiting_interrogation` targets), D094 (daemon-owned PG is
  the sole live-state authority).

## Problem

Striatum's workflow runtime is a set of individually-reasonable subsystems —
jobs, leases, queue messages, sessions, published artifacts, verdicts,
interrogation windows, the write-scope guard, and the recovery sweep — that **do
not compose under the two operations real runs depend on**: a `needs_revision`
re-open of a completed job, and a multi-reviewer interrogating panel against one
live author. The RFC 0094 design→build dogfood
(`run_c6d66b69303872b0863b6c8faa0a7e69`) drove the full pipeline and wedged at
the build-revision step, surfacing the incoherence as four concrete faults plus
three standing friction issues. They share one root cause.

### Observed failure modes (all reproduced live)

| # | Symptom | Issue |
|---|---|---|
| F-A | The first panel reviewer's `interrogation.close` transitions the **target** session to `closed` (`close_reason: interrogation_window_closed`); reviewers 2..N get `target_unavailable` and must vote `needs_revision` without interrogating. The design panel (claude-lane synth target) survived 3 interrogations; the build panel (codex-lane implementer target) collapsed after the first — so the window lifecycle is also **inconsistent/lane-dependent**. | #65 P1 |
| F-B | `run retry-job implement` (completed→queued) is **immediately re-completed** by the recovery sweep, which auto-publishes the *unchanged prior* `HANDOFF.md`. A `needs_revision` re-open is a no-op while the stale artifact exists; the fresh implementer can't even ack (`lease inactive/expired; await_packet returned no_work`). | #65 P2 |
| F-C | Removing the stale artifact stops auto-finalize, but the re-claim then fails `duplicate active job lease` — a released-but-dangling repo-write lease + `pending` message from the prior attempt blocks any new claim. | #65 P3 |
| F-D | No operator verb cleanly requeues a repo-write job for a declared revision: `requeue-stale` refuses repo-write (D036); `cancel-job` cancels; repeated `retry-job` stacks duplicate messages/leases (reached `attempt: 4`). | #65 P4 |
| F-E | The write-scope guard flags an operator-created out-of-scope file (`OPERATOR_REPORT.md`) and `dirty→clean` baseline transitions, wedging `work.complete` for sibling lanes in a shared worktree. | #57 |
| F-F | `submit-review` after a verify-time `publish-artifact` raises a raw Postgres unique-constraint crash instead of an idempotent no-op. | #58 |
| F-G | A re-opened `fresh_session_required` review sits `queued` until the operator manually closes the prior lane session and launches a fresh one. | #60 |

### Root cause

**There is no coherent, first-class notion of a *job attempt* that owns the
lease, the queue message, the published artifacts, the verdict, and (for
interrogable jobs) the interrogation window.** `attempt` exists today only as a
counter on the job row; everything else is scoped to the *job*, so a re-open
(new attempt) inherits the prior attempt's lease, message, artifact, and
sometimes window. The recovery sweep, the gate reader (`latestVerdict`,
`verifyRequiredArtifacts`), the write-scope baseline, and the session-per-lane
rule were each written for the **happy single-attempt path** and contradict each
other the moment a job is re-opened or a target is interrogated by more than
one reviewer.

Concretely:
- Recovery `auto_finalize` / `auto_publish_stale_artifacts` (RFC 0014/D057)
  completes a job whenever its expected artifact is present on disk — with **no
  notion of which attempt produced it** (F-B).
- A lease + queue message are bound to the *job*, and re-open neither releases
  the prior lease nor cancels the prior message atomically (F-C), and there's no
  requeue verb that respects the repo-write guard for a *declared* revision
  (F-D).
- The interrogation window is implicitly tied to the *first interrogation
  thread's* close rather than to the *panel* of declared interrogators (F-A).
- The write-scope baseline diff treats any deviation — including reverting a
  baseline-dirty file to clean, or an unrelated operator file — as a violation
  (F-E).
- `register-session` enforces one active session per lane with no replace path,
  and `fresh_session_required` re-opens are not auto-provisioned (F-G).
- `review.submit` re-publishes unconditionally (F-F).

This RFC makes the **attempt** first-class and reconciles every subsystem around
it, and makes the **interrogation window panel-owned**. It is a state-machine
correctness RFC, not a new feature: the goal is that `needs_revision` and
multi-reviewer panels *work*.

## Goals

1. **First-class job attempt.** Make `attempt` the unit that owns the active
   lease, the current queue message, the published artifacts, and the recorded
   verdict. Re-opening a job (via `needs_revision` cycle, human-checkpoint
   continue, or `retry-job`) starts a **new attempt** that is a clean slate.
2. **Attempt-scoped recovery.** Recovery `auto_finalize` /
   `auto_publish_stale_artifacts` only completes a job from artifacts produced by
   the **current** attempt; a re-opened attempt is never satisfied by a prior
   attempt's output (fixes F-B).
3. **Atomic, requeue-safe re-open.** Re-opening a job atomically releases the
   prior attempt's lease and cancels its pending message, and provides an
   operator/automatic requeue path for a *declared revision* of a repo-write job
   without tripping the D036 guard (fixes F-C, F-D).
4. **Panel-owned interrogation window.** The `awaiting_interrogation`
   preserved-context window is owned by the gate/panel, not by a single
   interrogation thread. `interrogation.close` ends a thread; the window stays
   open (the target re-arms) until all *declared* interrogators finish or a
   panel deadline fires, and the behavior is identical across lanes (fixes F-A).
5. **Session reuse/replacement.** `register-session --replace` and automatic
   fresh-session provisioning for re-opened `fresh_session_required` jobs, so a
   revision does not strand on manual session bookkeeping (fixes F-G, #60).
6. **Correct write-scope guard.** Flag only (a) files created during the current
   attempt outside `allowed_paths`, and (b) in-scope files mutated *away* from
   baseline; ignore `dirty→clean` transitions and pre-existing/operator files
   (fixes F-E, #57).
7. **Idempotent review publish.** `review.submit` treats an already-published
   identical finding as a no-op with a friendly notice, not a raw constraint
   crash (fixes F-F, #58).
8. **Revision context propagation.** A re-opened attempt's work packet carries
   the prior attempt's verdict + enumerated reviewer findings, so the lane knows
   what to revise without an out-of-band operator channel.
9. **Preserve the product boundary.** Daemon-owned PostgreSQL stays the sole
   live-state authority and sole writer (D094); no new external service; the
   changes are to internal state transitions, recovery policy, and read gates —
   no new live-dialog primitive and no vendor SDK.

## Non-Goals

- **No change to the workflow authoring schema's *shape*.** Cycles, phases, and
  `requires_verdict` gates stay as authored (RFC 0045); this RFC fixes how the
  runtime *executes* them under revision, not how they're declared.
- **No new interrogation/conversation method.** Panel-owned windows reuse the
  existing `interrogation.*` calls and the `awaiting_interrogation` state; the
  change is the *window lifecycle*, not the primitive.
- **No relaxation of attestation or review-diversity invariants** (RFC 0026/0064).
  Attempt-scoping must preserve "the reviewer differs from the author," lane
  attestation, and the substance-gate (RFC 0093) verdict agreement.
- **No auto-merge of revised work.** This RFC makes revision *possible and
  coherent*; landing decisions stay with the gate/operator.
- **Not the YAML multi-line front-matter parser bug (#59).** Orthogonal parser
  issue; out of scope.

## Proposal

### 1. First-class job attempt (the unifying abstraction)

Introduce an explicit **attempt** as the scoping key for runtime state. The job
row keeps `attempt` (already present) as the *current* attempt number; the
lease, queue message, published artifacts, and verdict each carry the
`(job_id, attempt)` they belong to.

- **Leases** carry `attempt`; a lease is only valid for the job's *current*
  attempt. Re-open increments `attempt`, which invalidates the prior lease by
  construction (no "duplicate active lease" — the prior lease is stale by
  attempt mismatch).
- **Queue messages** carry `attempt`; re-open cancels the prior attempt's
  message and enqueues a fresh one.
- **Published artifacts** record the `attempt` that produced them. The gate
  reader (`verifyRequiredArtifacts`, `dependenciesSatisfied`) and recovery only
  consider artifacts whose `attempt == jobs.attempt`. A prior attempt's artifact
  is retained for provenance but **does not satisfy** the new attempt.
- **Verdicts** record the `attempt` reviewed (they effectively already do via
  the cycle-scoped `cycle_<attempt>` naming from RFC 0093; this generalizes it).

This is the spine. F-B, F-C, F-D, and the gate-satisfaction bugs all dissolve
once "which attempt produced this?" is answerable. Migration: add `attempt`
columns (defaulting existing rows to the job's current attempt), owner-applied
per RFC 0079 §5 (the daemon migrates as a runtime role and must not crash-loop
on owner tables — see `project_daemon_migration_ownership`).

### 2. Attempt-scoped recovery (fixes F-B)

`recovery.auto_finalize` and `recovery.auto_publish_stale_artifacts` (RFC 0014)
gain an attempt guard: a job is auto-completable only when a **current-attempt**
artifact satisfies its `expected_artifacts`. A re-opened attempt with no new
artifact is **never** auto-finalized from the prior attempt's file; instead it
stays `queued`/`claimed` for the lane, and if the lane truly abandoned it the
recovery policy is `manual_inspection_required` (as today) — but it cannot
silently "complete" by re-publishing stale output. This closes the revision-gate
bypass: a `needs_revision` can only be cleared by a *new* attempt's artifact (or
an explicit operator override per D157 / RFC 0064).

### 3. Atomic, requeue-safe re-open (fixes F-C, F-D)

Define one internal `reopenJobForAttempt(job, reason)` used by every re-open path
(`needs_revision` cycle router, `checkpoint.resolve continue`, `run.retry_job`):
within a single transaction it (a) increments `attempt`, (b) releases the prior
lease (`release_reason: reopened`), (c) cancels the prior pending queue message,
(d) enqueues a fresh attempt message carrying revision context (§8), (e) resets
transitive downstream terminal jobs to `blocked` and clears their stale verdicts
(the RFC 0093/#63-F1 reset, now attempt-aware). Idempotent and atomic, so no
dangling lease/message can block the re-claim.

Add a recovery verb **`recovery.requeue_revision`** (or extend `retry_job` with
`--revision`) that requeues a repo-write job *for a declared revision* — the
D036 guard exists to stop blind requeue of abandoned repo-write work, not to
strand a legitimate revision, so the declared-revision path is exempt and routes
through `reopenJobForAttempt`.

### 4. Panel-owned interrogation window (fixes F-A)

Model the interrogable target's `awaiting_interrogation` window as a resource
owned by the **gate/panel**, not by an interrogation thread:

- When a job's downstream review panel is known (the set of `review` jobs that
  depend on the interrogable job), the target enters `awaiting_interrogation`
  with a **declared interrogator set** = those reviewers.
- `interrogation.close` ends *that thread only*. The target **re-arms** (stays in
  `awaiting_interrogation`) until every declared interrogator has opened+closed a
  thread (or explicitly waived), or a **panel deadline** elapses.
- The target session is closed (`interrogation_window_closed`) only when the
  panel is satisfied/expired — never by the first reviewer's close.
- Behavior is lane-independent: the daemon owns the window state, so the
  observed claude-vs-codex inconsistency (the agent loop re-arming or not after
  a close) is removed — the window is daemon state, not lane behavior.

This is the review-panel instance of RFC 0093 OQ4 / RFC 0094 `post_dialog_hook`
("keep participants live through a gate"); the mechanisms should share code.
Concurrency: serialized interrogation per target stays the policy (RFC 0093 §1,
D139 revisit) — re-arming is sequential across the declared set.

### 5. Session reuse/replacement (fixes F-G, #60)

- `register-session --replace` (and `--force`): atomically close any active
  session on the same `(run, lane)` and register the new one; the error path
  without `--replace` returns the exact remediation (which session to close).
- For a re-opened `fresh_session_required` job, the re-open (§3) optionally
  **auto-provisions** a fresh session registration stub for the required lane so
  the job does not sit `queued` waiting on manual operator bookkeeping; the
  operator (or a supervisor policy) still launches the supervised lane, but the
  registration friction is gone.

### 6. Correct write-scope guard (fixes F-E, #57)

Rework `write_scope_guard.go` baseline diffing so a violation is **only**:
1. a path **created during the current attempt** (not in the claim-time
   baseline) that lies outside `allowed_paths`; or
2. an **in-scope-or-tracked file mutated away from its baseline** content outside
   `allowed_paths`.

A baseline-dirty/untracked file transitioning to clean/committed, or an operator
file that exists but the attempt never touched, is **not** a violation. The
guard keys on *what this attempt did*, not on total worktree cleanliness. (This
also removes the operator-file footgun this RFC's own dogfood hit.)

### 7. Idempotent review publish (fixes F-F, #58)

`review.submit` (and `publish-artifact`) detect an already-published identical
artifact (same `repository_id, run_id, repo_path, content_sha256`) and treat it
as a **no-op success** with a structured notice, then proceed to record the
verdict — instead of surfacing the raw unique-constraint error. Publishing a
*different* content at the same path remains an error (real conflict).

### 8. Revision context propagation (supports §3)

A re-opened attempt's work packet includes a `revision_context` block: the prior
attempt's verdict, the enumerated reviewer findings, and the changed-artifact
refs. Today a `needs_revision` re-open via cycle attaches this; a manual
`retry-job` does not, so the lane re-does the job blind. Make it uniform via
`reopenJobForAttempt`. (Separately resolves the operator pain that
`supervise.send` only delivers packets, not freeform revision guidance — the
guidance rides the packet.)

## Unified lifecycle

```mermaid
stateDiagram-v2
  [*] --> queued
  queued --> claimed: claim (lease@attempt)
  claimed --> running: ack
  running --> published: publish artifact@attempt
  published --> awaiting_interrogation: interrogable (panel-owned window)
  awaiting_interrogation --> completed: panel done + verdict clears
  running --> completed: non-interrogable, verdict clears
  completed --> queued: reopenJobForAttempt (attempt++, lease released, msg canceled, downstream reset)
  awaiting_interrogation --> awaiting_interrogation: interrogation.close (re-arm until panel satisfied)
  running --> queued: stale lease (manual_inspection_required; NO auto-finalize from prior attempt)
  completed --> [*]
```

The single invariant: **every lease, message, artifact, and verdict is valid
only for `jobs.attempt`; recovery, gates, and the write-scope guard all read
through that lens.**

## Acceptance Criteria

1. **Re-open is a clean slate.** A `needs_revision` (or `retry-job`) on a
   completed repo-write job increments `attempt`, releases the prior lease,
   cancels the prior message, and the job is **not** auto-finalized from the
   prior attempt's artifact; a fresh lane claims+acks+holds an active lease and
   completes a new attempt. Live-PG test reproduces the #65 F-B/F-C scenario and
   asserts it now succeeds.
2. **No revision-gate bypass.** A re-opened attempt with no new artifact cannot
   reach `completed` via recovery; the gate (`dependenciesSatisfied`) only clears
   on a current-attempt clearing verdict (or D157 override). Fixture proves a
   `needs_revision` is not satisfied by re-publishing the identical prior artifact.
3. **Panel interrogation.** A 3-reviewer interrogating panel against one live
   interrogable target: **all three** open+close interrogation threads
   successfully; the target closes only after the third (or the panel deadline).
   Tested against both a claude-lane and a codex-lane target (lane-independent).
4. **No duplicate-active-lease wedge.** After re-open, `work.await_packet` from a
   fresh session succeeds (no `duplicate active job lease`); a guardrail test
   asserts at most one valid lease per `(job, current attempt)`.
5. **Requeue path.** `recovery.requeue_revision` (or `retry_job --revision`)
   requeues a repo-write job for a declared revision without the D036 refusal,
   routing through the atomic re-open.
6. **Session replace.** `register-session --replace` closes the prior
   `(run, lane)` session and registers the new one atomically; a re-opened
   `fresh_session_required` job no longer strands on manual close.
7. **Write-scope correctness.** A `dirty→clean` baseline transition and an
   out-of-scope operator file the attempt never touched do **not** raise a
   write-scope violation; an out-of-scope file *created by the attempt* still
   does. Test matrix over the four quadrants.
8. **Idempotent submit-review.** `publish-artifact` then `submit-review` on the
   same finding records the verdict with a friendly notice and no Postgres
   constraint error; a different-content same-path publish still errors.
9. **Revision context.** A re-opened attempt's packet carries the prior verdict +
   findings; a fixture asserts the lane receives them.
10. **Invariants preserved.** Attestation (RFC 0026), review diversity (RFC 0064),
    substance-gate verdict agreement (RFC 0093), and Go-only guardrails (RFC 0078)
    stay green; migrations are owner-applied (RFC 0079 §5) and do not crash-loop
    the daemon.

## Phased plan (smallest-blast-radius first)

1. **Phase 1 — stop the bleeding (no schema change).** §6 write-scope correctness
   (#57), §7 idempotent submit-review (#58), §5 `register-session --replace`
   (#60). These are local, high-value, and unblock day-to-day driving. Ship
   first.
2. **Phase 2 — attempt scoping core (§1 + §2 + §3).** Add `attempt` to leases /
   messages / artifacts; the `reopenJobForAttempt` helper; attempt-guard recovery
   auto-finalize; the requeue-revision verb. This is the #65 fix and the bulk of
   the work; owner-applied migration.
3. **Phase 3 — panel-owned interrogation window (§4).** Daemon-owned window
   lifecycle keyed on the declared interrogator set; share the liveness-bridge
   mechanism with RFC 0093 OQ4 / RFC 0094 `post_dialog_hook`.
4. **Phase 4 — revision context propagation (§8)** uniform across re-open paths.

Phases 1 and 3 are independently shippable; Phase 4 depends on Phase 2's
`reopenJobForAttempt`.

## Open Questions

1. **Attempt as a column vs a table.** Scope `attempt` as columns on
   leases/messages/artifacts/verdicts, or introduce a first-class `job_attempts`
   table that owns them by FK? Proposal: columns for V1 (smaller migration);
   revisit a table if attempt-level metadata grows.
2. **Panel deadline.** What bounds the panel-owned interrogation window when a
   reviewer never interrogates (crashes/stalls)? A per-panel deadline + a
   "reviewer waived interrogation" record? Proposal: panel deadline = max lease
   TTL of the declared reviewers; a missing reviewer's verdict is admissible only
   with an explicit `interrogation_waived` note (auditable).
3. **Auto-provision sessions.** Should the re-open auto-*launch* the supervised
   lane (not just register), or stop at registration? Proposal: register-only in
   V1 (launching is an operator/supervisor-policy decision; avoids the daemon
   spawning processes outside an explicit start).
4. **D036 boundary.** Does the declared-revision exemption need its own guard
   (e.g., only the cycle router / checkpoint override may declare a revision, not
   a bare operator requeue), to keep D036's "don't blind-requeue abandoned
   repo-write work" intact? Proposal: the exemption requires either a recorded
   `needs_revision` verdict or a D157 decision id.
5. **Backfill.** Existing in-flight runs predate `attempt` scoping; the migration
   defaults all extant leases/messages/artifacts to the job's current attempt.
   Confirm no live run is mid-revision at deploy (or gate the deploy on quiescent
   runs).

## Risks / migration

- **Owner-table migration risk.** Adding `attempt` to `leases`, `queue_messages`,
  `artifacts`, `verdicts` touches daemon-owned tables; per
  `project_daemon_migration_ownership` and RFC 0079 §5 these must be
  owner-applied to avoid crash-looping the daemon (it migrates as a runtime
  role). Stage as owner-applied DDL + a `schema_meta` bump.
- **Recovery behavior change.** Phase 2 makes recovery *stricter* (no
  stale-artifact auto-finalize of re-opened attempts). A genuinely-abandoned
  re-opened job now stalls `manual_inspection_required` instead of silently
  completing — which is the correct, auditable behavior, but operators must know
  the new failure mode (doc + dashboard surfacing).
- **Window deadline tuning.** Phase 3's panel deadline must not strand a run if a
  reviewer dies mid-window; OQ2's waiver mechanism is the safety valve.
