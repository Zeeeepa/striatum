package reads

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// #282: a non-accepting review verdict whose reviewed upstream was REVISED after
// the verdict (a build re-implemented to address a needs_revision, leaving this
// reviewer's verdict stale) silently blocks finalization with an
// otherwise-complete job graph. status now flags upstream_revised_after_verdict
// and hands the operator a precise recovery_action naming the review job, instead
// of a buried verdict row. A control review (no newer upstream) gets the ordinary
// non-accepting hint, so the flag is not vacuous.
func TestStatusFlagsStaleReviewVerdictAfterUpstreamRevision(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_stale_review_282"
	now := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	crSeedRepo(t, ctx, runner, repoID, now)
	runID := "run_stale_review_282"
	crSeedRun(t, ctx, runner, repoID, runID, "striatum/stale-review", now, false, nil)

	seedBuildJob := func(jobID string) {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
			  lane_selector_json, created_at
			) VALUES ($1,$2,$3,$4,2,'completed','implementer','Build','build',$5,
			         '{"mode":"repo_write","repo_write":true,"allowed_paths":["docs/"]}'::jsonb,
			         '[]'::jsonb,'{"lane_id":"codex"}'::jsonb,$6)`,
			repoID, jobID, runID, jobID, "idem_"+jobID, now); err != nil {
			t.Fatalf("seed build job %s: %v", jobID, err)
		}
	}
	dependOn := func(reviewJobID, buildJobID string) {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
			VALUES ($1,$2,$3)`, repoID, reviewJobID, buildJobID); err != nil {
			t.Fatalf("seed dependency %s->%s: %v", reviewJobID, buildJobID, err)
		}
	}
	seedBuildArtifact := func(buildJobID, artifactID string, at time.Time) {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.artifacts (
			  repository_id, artifact_id, run_id, job_id, logical_name, artifact_kind,
			  repo_path, content_sha256, size_bytes, publish_mode, created_at, attempt
			) VALUES ($1,$2,$3,$4,'handoff','handoff','docs/HANDOFF.md',$5,10,'create',$6,2)`,
			repoID, artifactID, runID, buildJobID, "sha_"+artifactID, at); err != nil {
			t.Fatalf("seed build artifact %s: %v", artifactID, err)
		}
	}

	// STALE: the build published a NEW artifact AFTER the review's verdict.
	seedBuildJob("job_build_stale")
	seedReviewJobWithVerdict(t, ctx, runner, statusVerdictFixture{
		repoID: repoID, runID: runID, jobID: "job_review_stale", workflowJobID: "review_build_codex",
		sessionID: "sess_review_stale", verdictID: "verdict_stale", verdict: "needs_revision", createdAt: now, ordinal: 1,
	})
	dependOn("job_review_stale", "job_build_stale")
	seedBuildArtifact("job_build_stale", "art_stale_revised", now.Add(time.Hour))

	// CONTROL: a non-accepting review with no newer upstream artifact.
	seedBuildJob("job_build_fresh")
	seedReviewJobWithVerdict(t, ctx, runner, statusVerdictFixture{
		repoID: repoID, runID: runID, jobID: "job_review_fresh", workflowJobID: "review_build_claude",
		sessionID: "sess_review_fresh", verdictID: "verdict_fresh", verdict: "needs_revision", createdAt: now.Add(2 * time.Hour), ordinal: 2,
	})
	dependOn("job_review_fresh", "job_build_fresh")
	seedBuildArtifact("job_build_fresh", "art_fresh", now) // older than the verdict

	result, err := HandleStatus(ctx, runner, rpc.Envelope{Params: map[string]any{"repository_id": repoID, "run_id": runID}})
	if err != nil {
		t.Fatalf("HandleStatus: %v", err)
	}
	rows, ok := result["latest_non_accepting_review_verdicts"].([]map[string]any)
	if !ok {
		t.Fatalf("latest_non_accepting_review_verdicts = %#v", result["latest_non_accepting_review_verdicts"])
	}
	byJob := map[string]map[string]any{}
	for _, row := range rows {
		byJob[fmt.Sprint(row["job_id"])] = row
	}

	stale := byJob["job_review_stale"]
	if stale == nil {
		t.Fatalf("stale review not surfaced; rows=%#v", rows)
	}
	if stale["upstream_revised_after_verdict"] != true {
		t.Fatalf("stale upstream_revised_after_verdict = %#v, want true", stale["upstream_revised_after_verdict"])
	}
	if action := fmt.Sprint(stale["recovery_action"]); !strings.Contains(action, "stale review") || !strings.Contains(action, "job_review_stale") {
		t.Fatalf("stale recovery_action = %q, want a stale-review rerun hint naming the job", action)
	}

	fresh := byJob["job_review_fresh"]
	if fresh == nil {
		t.Fatalf("control review not surfaced; rows=%#v", rows)
	}
	if fresh["upstream_revised_after_verdict"] != false {
		t.Fatalf("control upstream_revised_after_verdict = %#v, want false", fresh["upstream_revised_after_verdict"])
	}
	if action := fmt.Sprint(fresh["recovery_action"]); strings.Contains(action, "stale review") || !strings.Contains(action, "non-accepting review") {
		t.Fatalf("control recovery_action = %q, want the ordinary non-accepting hint", action)
	}
}
