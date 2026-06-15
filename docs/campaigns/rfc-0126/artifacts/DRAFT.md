# RFC 0126 P0 — build-owned review generation (DRAFT)

author: author-agent-001

Implements **Phase P0 of RFC 0126** (multi-reviewer revision coherence, accepted
**D194**; driven by GH #282). Scope is strictly P0: the `review_generation`
column, stamping in `applyVerdict`, the same-transaction bump in
`reopenJobForAttempt`, and stopping the verdict `DELETE`. **P1–P3 are not
implemented** (no work-packet generation stamp / write-boundary rejection, no
generation-scoped obligation gate, no retirement of the
`upstream_revised_after_verdict` legibility heuristic).

---

## 1. Design note

### Columns added

| Table | Column | Type |
| --- | --- | --- |
| `striatumd.jobs` | `review_generation` | `integer NOT NULL DEFAULT 1` |
| `striatumd.verdicts` | `review_generation` | `integer NOT NULL DEFAULT 1` |

`jobs.review_generation` is the build/synthesis job's **review epoch** (which
round of review covers the current build content). `verdicts.review_generation`
is the epoch a verdict was stamped against at record time. Both default to 1 so
pre-existing rows read as the first generation (mirroring `jobs.attempt`).

### Migration path: **owner bundle**, not a runtime migration (with ownership evidence)

`striatumd.jobs` and `striatumd.verdicts` are **owner-held** in the two-role
posture, so the column add is owner-table DDL and went into an **owner/admin
bundle** — `go/pkg/db/sql/owner/0009_review_generation.sql` — *not* a numbered
runtime migration. This is the OWNER-TABLE MIGRATION HAZARD the slice prompt
flagged (the RFC 0081 incident / D-log "Daemon migrates as runtime role").

**Evidence (decisive).** I first tried the runtime-migration route (a `0028`
ALTER) and the repo's own guard test rejected it:

```
TestFutureRuntimeMigrationsDoNotCarryOwnerDDL
  migration 28 carries owner-table DDL forbidden in regular runtime migrations:
  [ALTER TABLE striatumd.jobs ALTER TABLE striatumd.verdicts]
```

`futureRuntimeMigrationOwnerDDLFloor = 27`: any runtime migration with version
≥ 27 that `ALTER`/`DROP TABLE`s an existing `striatumd.*` table is forbidden,
because the daemon applies runtime migrations as `striatumd_rw`, which cannot
alter an owner-held table (the crash-loop). Migration `0024` *did* `ALTER`
`verdicts` as a runtime migration, but it predates the v27 floor and is
grandfathered. New owner-table DDL must ship as an owner bundle, applied
out-of-band as the database owner (`striatum daemon owner-ddl apply` /
`ApplyOwnerBundles` at daemon startup). `LatestOwnerBundleVersion` bumped 8 → 9.

### Test-harness adaptation (required, mirrors `artifact_write.go`)

The owner bundle creates a constraint the artifact-placement column already
solved: the default pgtest harness (`pgtest.Pool` → `db.ConnectAndMigrate`)
applies **only runtime migrations, not owner bundles**, so the
`review_generation` columns are **absent** in most package tests (verified: the
owner-bundle `artifacts.placement` column is likewise absent in that harness).
A static verdict `INSERT` naming `review_generation` would therefore break every
verdict-recording test.

So the verdict write path includes the column **adaptively**, gated on a single
`information_schema` column-presence check — `reviewGenerationEnabled(...)` —
exactly like `db.ArtifactPlacementColumnPresent` / `appendArtifactRowDirect`:

- **column present** (production; tests that apply owner bundles): stamp/bump.
- **column absent** (runtime-migrations-only harness): the historical INSERT,
  no bump — a no-op epoch, which is correct because nothing reads the column.

### The three edit sites

1. **`applyVerdict`** (`go/pkg/mutations/review.go`) — the single verdict-INSERT
   chokepoint for reviewer/adjudicator verdicts (`recordVerdict → applyVerdict`).
   Stamps the verdict with the **reviewed build's current `review_generation`**,
   resolved by `reviewedBuildGeneration(...)` via the review's revision-cycle
   target (`cycle.to`); a review with no `needs_revision` cycle defaults to
   generation 1.
2. **`reopenJobForAttempt`** (`go/pkg/mutations/revision_routing.go`) — bumps the
   reopened build's `review_generation` in the **same transaction** as the
   attempt bump (`bumpReviewGeneration`), so a verdict can never be stamped
   against a half-updated generation. Only the directly-reopened target advances.
3. **`resetJobToBlockedCore`** (`go/pkg/mutations/revision_routing.go`) — removed
   `DELETE FROM striatumd.verdicts`. Verdict history is now append-only; a
   revision renders prior-round verdicts non-current by **generation mismatch**,
   not by a clear.

**Chokepoint caveat (reported, not the single INSERT).** `applyVerdict` is the
chokepoint for all lane/adjudicator verdicts, but `HandleOverrideVerdict` has a
**separate** operator INSERT. P0 stamps `applyVerdict`; the override INSERT is
left unchanged and `review_generation` **DEFAULTs to 1** in production. Stamping
the override is deferred to P1/P2 (when the generation-scoped gate actually
consults the column); leaving it unstamped keeps both the runtime-only harness
and production correct without an adaptive branch on a rare, non-hot path.

---

## 2. Edits applied (file:line description)

New files:
- **`go/pkg/db/sql/owner/0009_review_generation.sql`** — owner bundle: the two
  `ADD COLUMN IF NOT EXISTS review_generation integer NOT NULL DEFAULT 1`
  statements + rationale (owner-held tables; no grant/SD-function needed because
  the table-level grant covers new columns and `verdicts` INSERT is not revoked
  from `striatumd_rw`).
- **`go/pkg/mutations/review_generation_test.go`** — the P0 pgtest (see §3).

Modified:
- **`go/pkg/db/owner.go`** — `LatestOwnerBundleVersion 8 → 9` + label for
  bundle 9.
- **`go/pkg/db/owner_test.go`** — `TestOwnerBundleNineAddsReviewGeneration`
  (asserts the bundle carries both ALTERs).
- **`go/pkg/mutations/review.go`**
  - `applyVerdict`: branch on `reviewGenerationEnabled` — present → 15-column
    stamped INSERT (`reviewedBuildGeneration` computes the value); absent →
    historical 14-column INSERT.
  - `HandleOverrideVerdict`: unchanged INSERT + a comment recording the
    DEFAULT-1 P0 boundary.
- **`go/pkg/mutations/revision_routing.go`**
  - `reopenJobForAttempt`: added the `bumpReviewGeneration(...)` call after the
    target reset, in the same transaction.
  - `bumpReviewGeneration` (new), `reviewGenerationEnabled` (new),
    `reviewedBuildGeneration` (new) helpers.
  - `resetJobToBlockedCore`: removed the `DELETE` + the now-vestigial
    `preserveVerdicts` parameter; removed `resetJobToBlockedPreservingVerdicts`
    (its single caller now uses `resetJobToBlockedWithReason`); refreshed the
    doc comments to the append-only contract.
- **`go/pkg/mutations/recovery_invalidate_job.go`** — caller switched from
  `resetJobToBlockedPreservingVerdicts` → `resetJobToBlockedWithReason` (the
  reset is append-only now; the invalidate path still supersedes its verdict
  rows as the durable receipt first).
- **`go/pkg/mutations/claim.go`** — corrected a stale comment that claimed the
  prior verdict row is DELETEd on re-open.
- **`go/pkg/mutations/fresh_review_byte_identical_test.go`** — corrected a stale
  fixture comment (production no longer DELETEs; the fixture's manual delete is
  same-session convenience).
- **`go/pkg/mutations/revision_routing_test.go`** —
  `TestRevisionCycleReReviewsAndUnblocksDownstream`: assert the prior verdict is
  **preserved** (count 1, not 0); age the preserved round to model the real
  (minutes-apart) round timing so the latest-verdict read is deterministic (see
  §4 note).
- **`go/pkg/mutations/verdict_provenance_stamp_test.go`** —
  `TestVerdictWriteSurfacesAreClassified`: `review.go` verdict-INSERT count
  `2 → 3` (the adaptive present/absent branches) + classification comment.

`go/pkg/db/migrations.go` is **unchanged** (the runtime-migration attempt was
fully reverted; `LatestDaemonDBVersion` stays 27).

---

## 3. Test obligations

**New — `TestRevisionBumpsBuildGenerationAndPreservesVerdicts`**
(`review_generation_test.go`). Applies owner bundles (production posture), wires
a reviewer as a downstream dependent of its build (`synth`), then records a
`needs_revision` verdict that routes the revision cycle and reopens the build.
Asserts the full P0 contract:

1. the verdict is **stamped at record time** with the build's generation (1);
2. reopening the build **bumps** its `review_generation` to 2 (same tx as the
   attempt bump);
3. the prior-round verdict row **survives** the reset (count 1 — no DELETE);
4. it is **non-current**: `verdict.review_generation (1) < build (2)`;
5. the reviewer was genuinely re-blocked, proving the row survived a *real*
   reset (the path that used to DELETE it). Stress-run 15× — deterministic.

This satisfies the slice's minimum P0 obligation (revision bumps the build's
generation; prior-round verdict rows survive but are non-current). The
generation-scoped completion-gate fence (RFC 0126 test obligation #1, the #282
shape) is **P2** and intentionally out of scope.

---

## 4. Build & test results

Environment note: this host has no `STRIATUM_PG_TEST_URL` configured and no
usable credentials for the live daemon DB, so I ran the PG-gated pgtests against
a **private throwaway PostgreSQL 16 cluster** (`initdb`/`pg_ctl`, trust auth,
port 5433) — fully isolated, nothing persistent, the live daemon DB untouched.

- `make -C go build` — **PASS** (all three binaries).
- `make -C go lint` (`golangci-lint`: govet/staticcheck/errcheck/ineffassign) —
  **0 issues**.
- `pkg/db` (named): `TestMigrationsAreOrdered` (count == 27),
  `TestLiveMigrationsInstallCurrentSchemaInvariants` (version 27),
  `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`,
  `TestOwnerBundleAppliesAndIsIdempotent` (applied == version == 9),
  `TestOwnerBundleNineAddsReviewGeneration` — **PASS**.
- `pkg/mutations` — **full package green except one pre-existing environmental
  failure** (below). New P0 test 15/15 deterministic; the verdict / revision /
  reopen / invalidate / fresh-review / provenance / run-completion neighbours
  all pass.
- `pkg/reads` — **PASS**.

**Pre-existing environmental failures (NOT caused by this change; each fails
identically on unmodified `main`):**
- `TestSpawnRunAsSpecResolvesLaneUser` (`pkg/mutations`) — lane-user resolution
  expects `striatum-lane`; environment-dependent.
- `TestBootstrapTwoRoleWithoutOwnerURLActionable` (`pkg/db`) — derives a
  negative-path owner DSN with no port and hits the host's *system* PostgreSQL
  on `:5432`, returning a SASL-auth error instead of the expected
  "name STRIATUM_OWNER_DB_URL" message.

### Transitional note (P0 → P2)

Stopping the DELETE makes a re-reviewed build's review job carry **multiple
non-superseded verdicts** (round-1 + round-2). The completion gate and
`latestVerdict` still read "latest non-superseded verdict per job"
(`ORDER BY created_at DESC`) — they do **not** consult `review_generation` until
P2. `nowString()` truncates `created_at` to the second, so two rounds recorded
in the *same wall-clock second* tie and fall back to the random `verdict_id`
tiebreak. In production this is moot — a revision round spans the minutes the
build takes to re-run and the reviewer to re-review, so the fresh round is
unambiguously newer. It only surfaces in a fixture that compresses the whole
loop into one second; the one affected test ages the preserved round to model
the real gap. **P2's generation-scoped gate makes this unconditional** and is
the proper home for the read-side change. Reconstructability gate (#285) is
untouched — `verifyRunCompletionProvenance` uses `SELECT *`, so the new column
does not affect it.

No new decision is required: D194 already accepts RFC 0126; this is its P0
implementation, and the owner-bundle-vs-runtime-migration choice follows the
existing D187 owner-table-DDL precedent.
