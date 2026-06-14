# RFC 0126: Multi-reviewer revision coherence — a build-owned review generation

Status: accepted (D194)
Date: 2026-06-14
author: proposer-claude-opus-4-8-001
Context: RFC 0095 (job attempt first-class / revision-safe lifecycle), RFC 0118 / D-pending#240
(run-completion provenance gate), RFC 0125 / D192 (durable gate artifact provenance —
the #282 *legibility* fix shipped here). Driven by GH #282 (revision cycles leave stale
non-accepting review verdicts blocking finalization) and the long-running
"design-panel revision re-open wedge" (`project_design_panel_revision_reopen_wedge`).
Grounded reads at `main`: `go/pkg/mutations/revision_routing.go`
(`reopenJobForAttempt:280`, `resetDownstreamForRevision:422`, `resetJobToBlockedCore`),
`go/pkg/mutations/review.go` (`applyVerdict`, `recordVerdict`, the verdicts INSERT
chokepoint), `go/pkg/mutations/run_completion_gate.go` (`verifyRunCompletionProvenance:37`
— reads "latest non-superseded verdict per review job"), `go/pkg/mutations/mutations.go`
(`maybeCompleteRun`, `verifyRequiredArtifacts`), `go/pkg/reads/status.go`
(`statusLatestNonAccepting` — now flags `upstream_revised_after_verdict`),
`go/pkg/db/sql/0005_repo_local_workflow_state.sql` (verdicts table),
`go/pkg/db/sql/0007_decision_propagation.sql` (`superseded_by_decision_id`).

## Summary

When a build/synthesis job has N independent reviewer jobs and one returns
`needs_revision`, the build reopens (attempt bump), the downstream reviewers are
recursively reset, the build re-runs, and the reviewers re-review. The reset works
by **clearing** each reviewer's attempt-scoped verdict rows — and that clear races
the reviewers' independent lifecycles. A reviewer that fails to start, is slow, or
records on the wrong side of the reset can leave a **stale** prior-attempt verdict as
the latest non-superseded verdict for that gate, silently blocking finalization while
every job in the graph shows `completed`. The operator had to hand-diagnose a buried
stale verdict and manually `recovery invalidate-job` it (the Hippo S12 codex review).

This RFC replaces the **DELETE-on-revision** model with a **build-owned monotonic
review generation**. Every verdict is stamped with the generation at write time,
enforced at the write boundary; the finalization gate is a serializable assertion
that **every required reviewer obligation has a current-generation accepting
non-superseded verdict**. Stale verdicts become structurally invisible (non-matching
generation) — no clear, no race, no manual invalidation. The reframe (from the
`/adhd` divergent pass): *staleness should be a non-match, not a flag someone has to
remember to clear.*

## Problem

`reopenJobForAttempt` → `resetDownstreamForRevision` already does a recursive walk of
all transitive downstream jobs and `resetJobToBlocked`s each terminal one, which
**clears** its attempt-scoped verdicts (`DELETE FROM striatumd.verdicts`). The
incoherence is not in the *walk* — it covers every reviewer — but in the **timing**
and the **clear**:

1. **The clear races verdict recording.** A reviewer lane that is mid-flight when the
   reset fires (or that re-claims and records against a frozen snapshot) can land a
   verdict that the reset already passed, or whose attempt scoping doesn't line up
   with the build's new attempt. The verdict survives as "latest non-superseded".
2. **A reviewer that never re-ran is indistinguishable from one that re-affirmed.**
   The gate reads "latest non-superseded verdict per review job". If a reviewer's
   attempt-2 lane fails to start (e.g. the #279 scratch-ACL bug), its attempt-1
   `needs_revision` is the latest non-superseded verdict — a *stale block* that looks
   identical to a genuine open finding.
3. **Finalization reads verdict recency, not build-attempt scope.** The gate has no
   positive proof that every required reviewer judged the *current* build attempt; it
   infers it from per-job latest-verdict rows that the reset mutated.
4. **DELETE destroys the audit trail.** Clearing verdicts on revision means a
   retrospective cannot reconstruct what each reviewer said in each round.

GH #282 shipped the *legibility* half in RFC 0125 (status now flags
`upstream_revised_after_verdict` and emits a precise `recovery_action`). This RFC is
the *coherence* half: make the stale block impossible, not merely legible.

## Goals

- After a build revision, finalization can proceed **only if** every required
  reviewer has a non-superseded **accepting** verdict stamped with the build's
  **current** generation. No stale verdict can silently block; no reviewer can be
  silently skipped (a missing current-generation verdict is an explicit unsatisfied
  obligation, not a vacuous pass).
- Staleness is enforced at the **write boundary** — a verdict for a superseded
  generation physically cannot be recorded, so the race window is closed at the
  source rather than filtered at the gate.
- **No DELETE of verdict history** on revision — every round's verdicts persist,
  stamped by generation, for offline reconstruction.
- Reviewer independence is preserved (reviewers remain separate jobs on separate
  lanes); the coherence is in the daemon, not in lane coordination.
- The gate refusal is **self-explaining** (names the reviewer + generation), building
  on the #282 legibility surface.

## Non-goals

- No new graduated artifact shape (RFC 0106 freeze holds) — the generation is a
  column on existing rows, not a new artifact kind.
- No change to reviewer **independence policy** (RFC 0002) or to same-model pairing
  rules.
- Not a redesign of the attempt model (RFC 0095) — the review generation is a
  build-job-scoped counter that rides alongside the existing `attempt`.

## Design

### The review generation (the spine)

Add a monotonic `review_generation` (integer) owned by the **build/synthesis** job —
the job whose revision invalidates its reviewers. It starts at 1 and is incremented
**in the same transaction** as `reopenJobForAttempt` (the attempt bump). The
generation is the build attempt's *review epoch*: "which round of review covers the
current build content".

**Verdicts are stamped, not cleared.** Add `review_generation` to the `verdicts`
table. `applyVerdict` (the single INSERT chokepoint all verdict writes funnel through)
stamps the verdict with the **reviewed build job's current generation** at record
time. `resetDownstreamForRevision` stops issuing `DELETE FROM striatumd.verdicts`;
instead the generation bump alone makes prior-round verdicts non-current. Existing
verdict rows are preserved as durable provenance (a retrospective sees every round).

### Write-boundary enforcement (close the race at the source)

Stamp the reviewed build's generation into the reviewer's **work packet** at dispatch
and validate it at `work.complete` / `review.submit`: a verdict whose stamped
generation does not equal the build job's **current** `review_generation` is
**rejected** (`invalid_transition: verdict for a superseded review generation`), and
the reviewer is re-queued for the current generation. A slow or stale lane therefore
*cannot* record an out-of-epoch verdict — the daemon refuses it. (This mirrors the
RFC 0125 body-durability gate's "frozen at record time" discipline and RFC 0118's
single-chokepoint stamping.)

### The obligation gate (positive proof, serializable)

At each generation bump, the daemon records the **required-reviewer obligation set**
for that generation (derived from the workflow snapshot's declared reviewers for the
build — the same set the workflow already knows). The run-completion gate
(`verifyRunCompletionProvenance`) gains a per-build assertion, evaluated under the
existing per-run advisory lock (RFC 0104):

> For each build job, for every required reviewer obligation at the build's current
> `review_generation`, there exists a non-superseded **accepting** verdict stamped
> with that generation.

Implemented as a set-difference (`required_reviewers MINUS
reviewers_with_current_generation_accepting_verdict`); a non-empty difference is a
gate failure routed through the existing `failing[] → escalateProvenanceGateFailure`
path with key `review_generation_incomplete` and the offending reviewer/generation
list (so `why <finalizer>` is never empty). This is **orthogonal** to RFC 0118's
attestation stamp and RFC 0125's body-reconstructability — it is the *coverage* axis.

### What this removes

- The `DELETE FROM striatumd.verdicts` in `resetJobToBlockedCore` (verdict history
  becomes append-only; staleness is a generation non-match).
- The need for an operator to `recovery invalidate-job` a stale verdict in the common
  case — a stale verdict is automatically non-current. (The verb stays for genuine
  operator overrides, now recorded with `posture=override` per RFC 0118.)
- The "latest non-superseded verdict per job" read as the gate's coherence source —
  replaced by the generation-scoped obligation set.

## Phasing

| Phase | Scope |
| --- | --- |
| **P0** | `verdicts.review_generation` column + build-job `review_generation`; stamp in `applyVerdict`; bump in `reopenJobForAttempt` (same tx); stop the verdict DELETE. |
| **P1** | Work-packet generation stamp + `work.complete`/`review.submit` write-boundary rejection of off-generation verdicts. |
| **P2** | Obligation set per generation + the generation-scoped completion-gate assertion + structured refusal; pgtest for the #282 shape (revised build, one reviewer never re-runs ⇒ gate refuses with the named obligation, not a clean complete). |
| **P3** | Retire the legibility-only `upstream_revised_after_verdict` heuristic in `statusLatestNonAccepting` in favor of the authoritative generation comparison (keep the `recovery_action`). |

## Test obligations

1. **#282 regression fence (pgtest):** revised build (generation 2), reviewer A accepts
   at gen 2, reviewer B's gen-1 `needs_revision` remains ⇒ the gate refuses with
   `review_generation_incomplete` naming B, and the run does NOT complete.
2. **Write-boundary (pgtest):** a verdict submitted with a superseded generation is
   rejected and the reviewer re-queued.
3. **No-regression (pgtest):** a single-reviewer build that accepts at the current
   generation completes cleanly; a revised-then-fully-re-reviewed build completes.
4. **History preserved (pgtest):** prior-generation verdict rows survive a revision
   (no DELETE) and are reconstructable.

## Anti-bets / open questions

- **Reviewer silence ≠ re-affirmation.** The `/adhd` "persistent reviewer whose silence
  re-affirms its prior verdict" idea is **rejected** — it reintroduces exactly the
  silent-skip bug (a reviewer that fails to start would "re-affirm" a stale verdict).
  A missing current-generation verdict MUST be an unsatisfied obligation.
- **Generation vs content-hash.** Stamping verdicts with a build *content hash* (so a
  byte-identical re-build reuses verdicts) was considered and deferred: a reviewer
  judges more than the build artifact bytes (it re-runs tests, reads context), and
  defining "same content" across a multi-file build is fragile. The monotonic
  generation is simpler and correct; content-hash reuse is a possible later
  optimization.
- **Obligation-set source.** Derive the required-reviewer set from the workflow
  snapshot at each bump (authoritative, drift-proof) vs. from live downstream edges —
  recommend the snapshot, matching RFC 0118's snapshot-derived required set.
- Interaction with RFC 0108 multi-run isolation and the RFC 0104 per-run lock must be
  pinned before P2 lands (the obligation assertion runs under the run lock).
