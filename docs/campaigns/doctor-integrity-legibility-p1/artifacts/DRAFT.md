# Doctor integrity legibility P1 — design note (reach `doctor ok=True`)

author: author-agent-001

## Goal

D204 (landed) reclassified the un-actionable doctor integrity findings to
`warnings`, taking `striatum doctor` from ~278 `problems` down to **42**. All 42
are old historical-loss artifacts. This P1 slice adds the two preservation
signals D204 deliberately left out, plus a curated baseline for
genuinely-lost-but-immaterial content, taking doctor from `ok=false` (42
actionable-but-historical problems) to **`ok=true`** — while keeping ANY
uncatalogued real integrity gap red and visible. A *green* doctor must mean
"trust it," and that is the load-bearing property behind the `AGENTS.md` "Do not
paste over a broken runner" guardrail.

This is a read-only diagnostic change in `go/pkg/reads/`. **No schema change, no
migration, no new RPC.**

## Edit sites

- `go/pkg/reads/doctor_artifact_anchor.go` (extended)
  - New warning codes `artifact_superseded_on_default_branch`,
    `artifact_acknowledged_loss`; new `defaultRefHistoryRevisionCap = 200`.
  - New per-pass cache bundle `artifactAnchorPass` (default-ref / history /
    ack-baseline caches), threaded through `checkArtifactAnchor` and
    `checkBlobExhaustArtifact` so a pass that scans hundreds of rows does bounded
    work (one default-ref resolve per repo root, memoized history hits, one
    baseline load per repo root).
  - New helpers `artifactContentInDefaultRefHistory` /
    `computeContentInDefaultRefHistory` (Rule A) and `pathExistsOnRef` (Rule B).
  - `artifactContentPreserved` now also consults default-branch *history* (so the
    empty-`blob_key` legacy path recognizes history-preserved content).
  - `artifactWarning` switch extended with the two new codes;
    `acknowledgedLossWarning` carries `reason` + `acknowledged_by` into the record.
  - `doctorArtifactAnchorIntegrity` surfaces the additive block field
    `acknowledged_loss_status` (`absent` | `loaded` | `parse_error`).
- `go/pkg/reads/doctor_acknowledged_loss.go` (new)
  - Schema `striatum.doctor.acknowledged_loss.v1` reader for the curated baseline
    at `docs/operator/doctor-acknowledged-loss.json`, with sha-bound `honor`.
- `go/pkg/reads/doctor.go` — **unchanged**. The existing assembly already appends
  the artifact check's `(warnings, warningRecords)` channel and `ok =
  len(problems)==0`; the new rules only move rows from `problems` to `warnings`,
  so no assembly change was needed.
- `docs/decisions/decision-log.md` — new **D205** entry (references D204 + the
  `AGENTS.md` guardrail; records that the live 16-entry baseline is
  operator-curated provenance committed separately).
- Tests: `go/pkg/reads/doctor_integrity_legibility_p1_test.go` (new) and two
  fixture updates in `go/pkg/reads/doctor_artifact_anchor_test.go`.

## The three rules

D204 split the 42 residual problems into three operator-verified classes; this
slice clears each:

| Class | Count | What is true | This slice does |
|---|---|---|---|
| **Recoverable** | 14 | `content_sha256` matches a revision of `repo_path` in default-branch **history** | **Rule A** → fully clean |
| **Superseded** | 12 | `repo_path` is **live on the default-branch tip** with *different* content | **Rule B** → `artifact_superseded_on_default_branch` warning |
| **Genuine loss** | 16 | `repo_path` is **absent from the default branch** and content is on no durable ref | **Rule C** → `artifact_acknowledged_loss` warning *iff* curated; else stays a `problem` |

### Rule A — default-branch *history* awareness

`artifactContentInDefaultRefHistory(ctx, repoRoot, defaultRef, repoPath,
expectedSHA, cache)`: content is durably preserved if `content_sha256` equals the
sha256 of `repo_path`'s blob at **any** reachable revision of the default branch,
not only its tip. Implementation: `git log <defaultRef> --max-count=200
--format=%H -- <repoPath>` lists only the commits that touched that path; for each
we read the blob and compare. **Bounded** (`--max-count=200`), `ctx`-cancellable,
**memoized** per `repoRoot|defaultRef|repoPath|expectedSHA`, and **safe-degrades
to false** on any git error. For a git-anchor artifact a history match is fully
clean (like a tip match); for an empty-`blob_key` legacy artifact it becomes an
`artifact_legacy_unverifiable` warning (via `artifactContentPreserved`).

### Rule B — superseded-on-default-branch

When content is unverifiable on every ref/tip/history but `repo_path` still exists
on the default-branch tip (any content), the deliverable landed and only the
recorded *draft* sha is unverifiable. `pathExistsOnRef` (`git cat-file -e
<commit>:<repoPath>`, `ctx`-cancellable, safe-degrade false) gates a new
`artifact_superseded_on_default_branch` **warning** — not a problem.

### Rule C — curated acknowledged-loss baseline

For a genuine loss the daemon cannot prove preservation, but the operator can
record that a specific, reviewed loss is **acknowledged** (immaterial superseded
dogfood content). New reader `doctor_acknowledged_loss.go` consults a tracked JSON
file:

```json
{
  "schema": "striatum.doctor.acknowledged_loss.v1",
  "entries": [
    {
      "artifact_id": "art_...",
      "repo_path": "docs/issues/22/build/HANDOFF.md",
      "content_sha256": "<hex>",
      "reason": "Superseded dogfood draft; never merged to main. Immaterial historical loss.",
      "acknowledged_by": "operator-<role>-<model>-<ordinal>",
      "acknowledged_at": "2026-06-16"
    }
  ]
}
```

- **Loaded once per repo root** (memoized), returning a lookup keyed by
  `artifact_id`. Status is recorded as the additive block field
  `acknowledged_loss_status` (`absent` | `loaded` | `parse_error`).
- **Safe-degrade:** a missing file → `absent` empty set (no downgrades); an
  unreadable/malformed file or wrong schema → `parse_error` empty set (still no
  downgrades). A broken/missing baseline can only **fail to downgrade** a problem,
  never mask one.
- **Sha-bound:** an entry is honored ONLY if `entry.content_sha256` equals the
  row's `content_sha256` (case-insensitive hex, both trimmed, both non-empty). An
  id-match with a mismatched/empty sha is **not** honored — the loss stays a
  problem. This binds the acknowledgment to the exact lost content so a
  stale/wrong entry can never mask a *different* future problem at the same id.
- **The live baseline file is NOT authored in this slice.** The operator curates
  and commits `docs/operator/doctor-acknowledged-loss.json` (the 16 reviewed
  entries) separately after verifying the real residual against deployed code.
  This slice ships only the reader + schema + fixtures + tests; the reader
  safe-degrades while the file is absent.

## Check ordering (both `checkArtifactAnchor` and the blob genuine-loss branch)

1. Preserved at default-branch **tip** (exact content) → clean. *(existing)*
2. **Rule A**: preserved in default-branch **history** (exact content) → clean.
3. Terminal-debris run (`canceled`/`failed`) → `artifact_debris_terminal_run`
   warning. *(existing)*
4. **Rule B**: `repo_path` exists on default-branch tip →
   `artifact_superseded_on_default_branch` warning.
5. **Rule C**: `artifact_id` in the sha-bound ack baseline →
   `artifact_acknowledged_loss` warning.
6. else → genuine-loss `problem` (`artifact_anchor_hash_mismatch` /
   `artifact_anchor_missing_file` / `artifact_blob_metadata_missing`). *(existing)*

## Genuine-loss detection preserved (load-bearing)

An artifact whose content is on no ref, not in default-branch history, whose path
is absent from the default branch, and which is NOT in the ack baseline (or is
present only with a mismatched sha) MUST stay a `problem`. This is proven by the
`...SafetyStillReds` test (both `not_in_baseline` and `sha_mismatch` subtests) and
by the unchanged `TestDoctorArtifactGenuineLossStillReds` /
`TestDoctorArtifactLegacyBlobKeyGenuineLossStillReds`.

## Honest build + test results

Run from `go/` inside the per-job worktree.

- **Build:** `make -C go build` → **OK** (exit 0; `striatum`, `striatumd`,
  `striatum-supervisor-helper` all built). `go vet ./pkg/reads/` → **OK** (exit 0).
- **Targeted tests** (`STRIATUM_PG_TEST_URL=postgres://halbritt@/postgres?host=/var/run/postgresql
  go test ./pkg/reads/ -run <names> -p 1 -timeout 900s -v`) — **all PASS**:
  - `TestDoctorArtifactAnchorPreservedInDefaultBranchHistoryIsClean` (Rule A) — PASS
  - `TestDoctorArtifactSupersededOnDefaultBranchWarns` (Rule B) — PASS
  - `TestDoctorArtifactAcknowledgedLossInBaselineWarns` (Rule C accept) — PASS
  - `TestDoctorArtifactAcknowledgedLossSafetyStillReds/not_in_baseline` — PASS
  - `TestDoctorArtifactAcknowledgedLossSafetyStillReds/sha_mismatch` — PASS
  - `TestLoadAcknowledgedLossSetSafeDegrades` (reader safe-degrade) — PASS
  - Updated fixtures `TestDoctorArtifactAnchorIntegrityReportsRunBranchMismatch`,
    `TestDoctorArtifactAnchorIntegrityReportsJobPinMismatch` — PASS
  - Regression: existing D204 legibility + anchor tests
    (`...PreservedOnDefaultBranchIsClean`, `...LegacyBlobKeyPreservedOnDefaultBranchWarns`,
    `...GenuineLossStillReds`, `...LegacyBlobKeyGenuineLossStillReds`,
    `...AcceptsRunBranchMatch`, `...GitChecksExplicitGitPublicationSynthesis`,
    `TestReadGitDefaultBranchRefDegradesSafely`) — PASS
- **Why two existing tests were updated (not a regression):** their fixtures
  committed the artifact to `main`, so under the new Rule B (path live on the
  default-branch tip with different content) they are *correctly* reclassified
  from a `hash_mismatch` problem to an `artifact_superseded_on_default_branch`
  warning. To keep them covering the genuine `hash_mismatch` **problem** branch +
  anchor-kind classification, the fixtures now place the artifact on a ref-only
  path (run branch / `refs/striatum` pin) that is **absent** from the default
  branch, so Rule B does not fire and the mismatch stays a problem.
- **Full-package run** (`go test ./pkg/reads/ -p 1 -timeout 900s`): the non-PG
  unit tests pass; **15 tests FAIL on PostgreSQL connection only** — `create
  pgtest database: ... FATAL: Peer authentication failed for user "halbritt"
  (SQLSTATE 28000)`. These fail at connect time before any test logic, are
  PG-infra-gated (this lane OS user cannot peer-auth as `halbritt`), reference no
  doctor/artifact code, and are unrelated to this change. No panics, no logic
  failures.

## Scope / guardrail compliance

- Stayed within `write_scope.allowed_paths` (`go/`, `docs/decisions/`,
  `docs/campaigns/doctor-integrity-legibility-p1/`).
- Code written onto the run branch via the daemon (`publish_source_changes`
  enabled); **not** merged to `main`. The live acknowledged-loss baseline file is
  left for the operator to curate and commit separately.
