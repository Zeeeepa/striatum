package mutations

import (
	"context"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestRevisionBumpsBuildGenerationAndPreservesVerdicts is the RFC 0126 P0
// (D194 / GH #282) regression fence. A reviewer's needs_revision verdict routes
// the revision cycle and re-opens the build (synth) job. The P0 contract:
//
//  1. the verdict is STAMPED at record time with the build's current
//     review_generation (1);
//  2. re-opening the build BUMPS the build's review_generation (to 2) in the
//     same transaction as the attempt bump;
//  3. the prior-round verdict row SURVIVES the re-open (no DELETE — verdict
//     history is append-only); and
//  4. it is NON-CURRENT: its stamped generation no longer matches the build's,
//     so a stale verdict can no longer silently win the latest-row tiebreak.
func TestRevisionBumpsBuildGenerationAndPreservesVerdicts(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	// review_generation is owner-bundle DDL (owner bundle 0009: owner-held jobs +
	// verdicts), so the default runtime-migrations-only harness does not have the
	// columns. Apply owner bundles so the adaptive verdict path stamps and bumps
	// the generation — this is the production posture (daemon applies owner bundles
	// at startup) the P0 contract is defined against.
	if _, _, err := db.ApplyOwnerBundles(ctx, runner, "test"); err != nil {
		t.Fatalf("apply owner bundles: %v", err)
	}
	repoID := "repo_review_generation"
	cycles := []any{
		map[string]any{
			"from":            "review_design_codex",
			"to":              "synth",
			"on_verdict":      "needs_revision",
			"max_iterations":  float64(2),
			"allow_same_lane": true,
		},
	}
	_, reviewJobID, synthJobID, sessionID, leaseID := revisionFixture(t, ctx, runner, repoID, cycles)

	// Wire the reviewer as a downstream dependent of the build it reviews, so the
	// revision re-open resets the reviewer through resetDownstreamForRevision —
	// the exact path that used to DELETE its verdict.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`, repoID, reviewJobID, synthJobID); err != nil {
		t.Fatalf("wire reviewer dependency on build: %v", err)
	}

	// Sanity: the build starts at generation 1 (migration default), and recording
	// the verdict will stamp that value before the bump.
	if got := buildReviewGeneration(t, ctx, runner, repoID, synthJobID); got != 1 {
		t.Fatalf("build review_generation before revision = %d, want 1", got)
	}

	result, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     reviewJobID,
		"lease_id":   leaseID,
		"verdict":    "needs_revision",
	}))
	if err != nil {
		t.Fatalf("record verdict: %v", err)
	}
	if fmt.Sprint(result["status"]) != "revision_routed" {
		t.Fatalf("status = %v, want revision_routed; result=%#v", result["status"], result)
	}

	// (2) The build's generation advanced in the same transaction as the re-open.
	buildGen := buildReviewGeneration(t, ctx, runner, repoID, synthJobID)
	if buildGen != 2 {
		t.Fatalf("build review_generation after revision = %d, want 2 (bumped on re-open)", buildGen)
	}

	// (3) The prior-round verdict row survived the re-open: history is append-only.
	verdictRows, err := queryRows(ctx, runner, `
		SELECT verdict, review_generation FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2`, repoID, reviewJobID)
	if err != nil {
		t.Fatalf("load verdicts: %v", err)
	}
	if len(verdictRows) != 1 {
		t.Fatalf("expected 1 surviving verdict row for the reviewer (no DELETE on revision), got %d", len(verdictRows))
	}
	if v := fmt.Sprint(verdictRows[0]["verdict"]); v != "needs_revision" {
		t.Fatalf("surviving verdict = %q, want needs_revision", v)
	}

	// (1) + (4) The surviving verdict was stamped against build generation 1 at
	// record time and is now non-current: its generation is strictly less than the
	// build's current generation.
	verdictGen := intValue(verdictRows[0]["review_generation"])
	if verdictGen != 1 {
		t.Fatalf("verdict review_generation = %d, want 1 (stamped at record time against build gen 1)", verdictGen)
	}
	if verdictGen >= buildGen {
		t.Fatalf("verdict generation %d must be < build generation %d to be non-current", verdictGen, buildGen)
	}

	// The reviewer was genuinely reset by the revision (it is downstream of the
	// build), proving the verdict survived a real reset rather than an untouched
	// job — the reset path is exactly where the DELETE used to fire.
	if got := jobState(t, ctx, runner, repoID, reviewJobID); got != "blocked" {
		t.Fatalf("reviewer job state = %q, want blocked (reset by resetDownstreamForRevision)", got)
	}
}

func buildReviewGeneration(t *testing.T, ctx context.Context, runner any, repoID, jobID string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT review_generation FROM striatumd.jobs
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("load build generation for %s: %v", jobID, err)
	}
	return intValue(row["review_generation"])
}
