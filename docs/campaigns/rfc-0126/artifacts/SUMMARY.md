---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs:
  - draft
  - review
---

# RFC 0126 P0 — build-owned review generation (APPLY / SUMMARY)

author: author-agent-002

Applies the **accepted** review of RFC 0126 Phase P0 (multi-reviewer revision
coherence, accepted **D194**; GH #282). The reviewer's verdict was **accept**
with no P0 defects — only two advisory notes that are explicitly *not* P0
defects and are correctly deferred to P1/P2. No code change was therefore
required to satisfy the review; this apply step **materializes the exact
reviewed change set onto the run's feature branch**
(`striatum/rfc-0126-p0-review-generation`) and re-verifies build + the P0
pgtest live, so the operator can review and integrate from one branch.

The reviewed `go/` edits were authored uncommitted in the draft job's per-job
worktree (the run branch carried only the `DRAFT.md` git-publication). This
apply job reproduced those edits **byte-for-byte** (all 11 files verified
identical to the reviewed source) into the apply worktree and committed them to
the run branch.

---

## 1. Final set of edits

### Columns added

| Table | Column | Type |
| --- | --- | --- |
| `striatumd.jobs` | `review_generation` | `integer NOT NULL DEFAULT 1` (build review epoch) |
| `striatumd.verdicts` | `review_generation` | `integer NOT NULL DEFAULT 1` (epoch stamped at record time) |

### Migration path actually taken: **owner bundle**, with ownership evidence

`striatumd.jobs` and `striatumd.verdicts` are **owner-held** in the two-role
posture, so the column-add is owner-table DDL and went into an **owner/admin
bundle** — `go/pkg/db/sql/owner/0009_review_generation.sql` — **not** a numbered
runtime migration. The daemon applies runtime migrations as `striatumd_rw`,
which cannot `ALTER` an owner-held table; doing so would crash-loop the daemon
(the RFC 0081 incident / D-log "Daemon migrates as runtime role").

**Ownership evidence (decisive, in-repo guard):** the runtime route was tried
first (a `0028` ALTER) and the repo's own guard rejected it —
`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` with
`futureRuntimeMigrationOwnerDDLFloor = 27` forbids any runtime migration with
version ≥ 27 that `ALTER`/`DROP`s an existing `striatumd.*` table. (Migration
`0024` did `ALTER verdicts` as a runtime migration, but it predates the v27
floor and is grandfathered.) So the column lives in the owner bundle and the
runtime guard stays green.

- `LatestOwnerBundleVersion` bumped **8 → 9** (`go/pkg/db/owner.go`).
- `LatestDaemonDBVersion` stays **27** (`go/pkg/db/migrations.go` unchanged — the
  runtime-migration attempt was fully reverted).
- No new GRANT / SECURITY DEFINER function: the table-level grant to
  `striatumd_rw` (migration 0005) covers new columns and `verdicts` INSERT is not
  revoked from the runtime role.

### The three required edit sites (+ supporting edits)

1. **Stamp — `applyVerdict`** (`go/pkg/mutations/review.go`): the single
   verdict-INSERT chokepoint for reviewer/adjudicator verdicts. Branches on a
   `reviewGenerationEnabled(...)` `information_schema` column-presence check
   (mirroring `db.ArtifactPlacementColumnPresent`): **present** → 15-column
   stamped INSERT, value from `reviewedBuildGeneration(...)` (resolved via the
   review's revision-cycle `to` target; a review with no `needs_revision` cycle
   defaults to generation 1); **absent** (runtime-migrations-only test harness)
   → the historical 14-column INSERT. `HandleOverrideVerdict`'s separate operator
   INSERT is left unstamped (DEFAULT 1) by design, with a comment recording the
   P0 boundary.
2. **Same-transaction bump — `reopenJobForAttempt`**
   (`go/pkg/mutations/revision_routing.go`): adds `bumpReviewGeneration(...)`
   after the target reset, in the **same transaction** as the attempt bump and
   before `enqueueJob`, so a verdict can never be stamped against a half-updated
   generation. New helpers `bumpReviewGeneration`, `reviewGenerationEnabled`,
   `reviewedBuildGeneration`.
3. **Stop the DELETE — `resetJobToBlockedCore`**
   (`go/pkg/mutations/revision_routing.go`): removed `DELETE FROM
   striatumd.verdicts`; verdict history is now append-only, staleness is a
   generation non-match. Removed the now-vestigial `preserveVerdicts` parameter
   and the `resetJobToBlockedPreservingVerdicts` variant; its sole caller
   (`recovery_invalidate_job.go`) now uses `resetJobToBlockedWithReason` (it
   still supersedes its verdict rows as the durable receipt first).

Supporting/test edits: `claim.go` and `fresh_review_byte_identical_test.go`
stale-comment corrections; `revision_routing_test.go`
(`TestRevisionCycleReReviewsAndUnblocksDownstream` now asserts the prior verdict
is preserved, count 1 not 0, and ages the preserved round to model real
minutes-apart timing); `verdict_provenance_stamp_test.go`
(`TestVerdictWriteSurfacesAreClassified` INSERT count 2 → 3 for the adaptive
present/absent branches); new `owner_test.go`
`TestOwnerBundleNineAddsReviewGeneration`; new pgtest
`review_generation_test.go`.

**Files (11):** `go/pkg/db/sql/owner/0009_review_generation.sql` (new),
`go/pkg/mutations/review_generation_test.go` (new), `go/pkg/db/owner.go`,
`go/pkg/db/owner_test.go`, `go/pkg/mutations/review.go`,
`go/pkg/mutations/revision_routing.go`,
`go/pkg/mutations/recovery_invalidate_job.go`, `go/pkg/mutations/claim.go`,
`go/pkg/mutations/fresh_review_byte_identical_test.go`,
`go/pkg/mutations/revision_routing_test.go`,
`go/pkg/mutations/verdict_provenance_stamp_test.go` — `+237 / -70` over the
tracked files plus the two new files.

---

## 2. How each review finding was resolved

The verdict was **accept**; the reviewer recorded **zero** required-change
findings. The five review criteria (scope, owner-table hazard, append-only
history, atomicity, build+tests) all passed and the reviewer verified build and
tests **live** against a throwaway PostgreSQL 16 cluster. Resolution: the
reviewed change set is carried forward verbatim — nothing to fix.

The two **advisory notes** are explicitly *not P0 defects* and were correctly
deferred at draft time; they are recorded here and carried to the later runs (no
P0 action):

- **A1 — read-side recency reliance until P2.** With the DELETE gone, a
  re-reviewed build's review job can hold multiple non-superseded verdicts, and
  the gate / `latestVerdict` still order by `created_at` (second-truncated). This
  is fine in production (rounds are minutes apart) and is disclosed in the DRAFT
  transitional note; the one compressed-loop fixture ages the preserved round to
  model the real gap. **→ Deferred to P2**, which makes the generation comparison
  unconditional (the proper home for the read-side change).
- **A2 — `HandleOverrideVerdict` unstamped (DEFAULT 1).** The operator-override
  INSERT is a second verdict write path not routed through `applyVerdict`; P0
  leaves it at the column default (documented in-code). Harmless now (nothing
  reads the column at the gate yet). **→ P1/P2** must stamp it before the
  generation-scoped gate consults the column.

---

## 3. Build & test results (live)

Re-verified in the apply worktree (run branch tip `d3fd8ea3`) against a private,
throwaway **PostgreSQL 16** cluster (`initdb` + `pg_ctl`, trust auth, port 5434,
`STRIATUM_PG_TEST_URL` pointed at it, fully torn down afterward — the live daemon
DB was untouched).

- `make -C go build` — **PASS** (all three binaries: `striatum`, `striatumd`,
  `striatum-supervisor-helper`; also confirms no broken callers from the removed
  `resetJobToBlockedPreservingVerdicts` variant).
- `go vet ./pkg/mutations ./pkg/db` — **PASS** (0 issues).
- **P0 fence** `TestRevisionBumpsBuildGenerationAndPreservesVerdicts`
  (`pkg/mutations`) — **PASS**; asserts the full P0 contract: verdict stamped at
  record time against build gen 1; re-open bumps the build to gen 2 in the same
  tx; the prior-round verdict row **survives** (count 1, no DELETE) and is
  **non-current** (verdict gen 1 < build gen 2); reviewer genuinely re-blocked.
  Stress-run **8×, deterministic**.
- `pkg/mutations` neighbours — **PASS**:
  `TestRevisionCycleReReviewsAndUnblocksDownstream`,
  `TestVerdictWriteSurfacesAreClassified` (INSERT count 2→3),
  `TestFreshReviewLineage*` (5), `TestRecoveryInvalidateJob*` (3),
  `TestOverrideVerdictForcesOverridePostureAndStamp`.
- `pkg/db` owner-bundle + migration guards — **PASS**:
  `TestOwnerBundleNineAddsReviewGeneration`, `TestMigrationsAreOrdered`,
  `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`,
  `TestOwnerBundleAppliesAndIsIdempotent`,
  `TestLiveMigrationsInstallCurrentSchemaInvariants` (version 27).

Pre-existing environmental failures the DRAFT disclosed
(`TestSpawnRunAsSpecResolvesLaneUser`,
`TestBootstrapTwoRoleWithoutOwnerURLActionable` — both depend on host env / the
system PostgreSQL on :5432 and fail identically on unmodified `main`) were not
re-exercised here; they are unrelated to this change set, which touches none of
their code paths.

---

## 4. What remains (P1–P3, separate runs)

P0 lands only the column + stamp + same-tx bump + append-only history; **P1**
(work-packet generation stamping / write-boundary rejection), **P2** (the
generation-scoped obligation/completion gate that makes the generation
comparison unconditional and stamps the override path), and **P3** (retiring the
`upstream_revised_after_verdict` legibility heuristic) remain as their own runs.

---

*Integration note:* this change is left on the run feature branch
`striatum/rfc-0126-p0-review-generation` for operator review and integration —
**not** merged to `main`.
