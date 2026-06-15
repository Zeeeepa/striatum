---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# Review — RFC 0126 P0 (build-owned review generation)

author: reviewer-agent-001

**Verdict: accept.** P0 is correctly scoped, the owner-table migration hazard
(the single highest-risk point) is handled decisively with evidence, verdict
history is genuinely append-only, the generation bump is atomic with the attempt
bump, and `make -C go build` plus a real P0 pgtest pass. I verified build and
tests **live** against an isolated throwaway PostgreSQL 16 cluster (torn down
afterward), not just from the DRAFT's reported results. Reviewed: RFC 0126 P0,
the published DRAFT.md, and the actual worktree changes (the `go/` edits are
uncommitted in the draft job's per-job worktree; the run branch carries only the
DRAFT.md git-publication — expected for this workflow).

## Five criteria

1. **Scope — PASS (exactly P0, no leakage).** Owner bundle
   `go/pkg/db/sql/owner/0009_review_generation.sql` adds
   `review_generation int NOT NULL DEFAULT 1` to `striatumd.jobs` (build epoch)
   and `striatumd.verdicts` (stamp). Stamp in `applyVerdict` (review.go:653, the
   confirmed chokepoint) via `reviewedBuildGeneration` (resolves the cycle `to`
   target). Same-tx bump in `reopenJobForAttempt` (revision_routing.go:341) via
   `bumpReviewGeneration`. DELETE removed from `resetJobToBlockedCore`. No P1
   (work-packet stamp / write-boundary reject), no P2 (obligation gate —
   `verifyRunCompletionProvenance` untouched, still latest-non-superseded), no P3
   (heuristic retirement). Confirmed by grep + reading the gate.

2. **Owner-table hazard — PASS.** `jobs`/`verdicts` are owner-held; a runtime
   ALTER would crash-loop the daemon. Ownership verified *empirically*: a runtime
   ALTER (v>=27) is rejected by `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`
   (floor v27), so the add went to **owner bundle 0009**.
   `LatestOwnerBundleVersion` 8->9 (owner.go:20); `LatestDaemonDBVersion` stays
   27 (migrations.go:17); no runtime/`sql/` change. The runtime-migrations-only
   harness is handled by the adaptive `reviewGenerationEnabled` catalog check,
   mirroring `db.ArtifactPlacementColumnPresent`.

3. **Append-only history — PASS.** DELETE gone; vestigial `preserveVerdicts` and
   `resetJobToBlockedPreservingVerdicts` removed, sole caller
   (recovery_invalidate_job.go) switched to `resetJobToBlockedWithReason` (still
   supersedes its rows as the durable receipt first). Proven live: prior-round
   verdict survives (count 1) and is non-current (gen 1 < gen 2).

4. **Atomicity — PASS.** `bumpReviewGeneration` runs inside `reopenJobForAttempt`
   on the same `runner`/tx as the attempt bump, before `enqueueJob` — a verdict
   can't be stamped against a half-updated generation.

5. **Build + tests — PASS (verified live).** `make -C go build` PASS (3
   binaries; also confirms no broken callers from the removed reset variant).
   `go vet ./pkg/mutations ./pkg/db` PASS. Live pgtests (throwaway PG 16, trust
   auth, port 5434, removed after; live daemon DB untouched):
   `TestRevisionBumpsBuildGenerationAndPreservesVerdicts` PASS (the P0 fence:
   bump + history survival + non-currency; 8x deterministic);
   `TestRevisionCycleReReviewsAndUnblocksDownstream`,
   `TestVerdictWriteSurfacesAreClassified` (count 2->3),
   `TestFreshReviewLineage*`, `TestRecoveryInvalidateJob*`,
   `TestOverrideVerdictForcesOverridePostureAndStamp` all PASS;
   pkg/db `TestOwnerBundleNineAddsReviewGeneration` + migration/owner-bundle
   guards PASS.

## Advisory notes (not P0 defects — disclosed and correctly deferred)

1. **Read-side recency reliance until P2.** Without the P2 generation-aware read,
   a re-reviewed build's review job now holds multiple non-superseded verdicts and
   the gate/`latestVerdict` still order by `created_at` (second-truncated). Fine
   in production (rounds are minutes apart) but it now leans on wall-clock
   recency until P2; the one compressed-loop fixture aged the preserved round 1h
   to model the real gap. P2 should make the generation comparison unconditional.

2. **`HandleOverrideVerdict` unstamped (DEFAULT 1).** The operator-override INSERT
   is a second verdict write path not routed through `applyVerdict`; P0 leaves it
   at the column default (documented). Harmless now (nothing reads the column at
   the gate yet), but P1/P2 must stamp it before the generation-scoped gate
   consults the column, else an override reads as gen 1 and could look stale.

## Conclusion

P0 delivers exactly the foundation the Phasing table describes, with the
owner-table hazard correctly navigated and proven. **accept.**
