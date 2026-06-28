package mutations

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestResolvePublishPlacementUsesExpectedArtifactPlacement(t *testing.T) {
	job := map[string]any{
		"expected_artifacts_json": []any{map[string]any{
			"logical_name": "summary",
			"kind":         "synthesis",
			"path":         "docs/SUMMARY.md",
			"placement":    artifactcontracts.PlacementGitPublication,
			"required":     true,
		}},
	}
	got := resolvePublishPlacement(job, "synthesis", "summary", "docs/SUMMARY.md", 1)
	if got != artifactcontracts.PlacementGitPublication {
		t.Fatalf("placement = %q, want git publication", got)
	}
}

func TestResolvePublishPlacementFallsBackToLegacyKindDefault(t *testing.T) {
	job := map[string]any{"expected_artifacts_json": []any{}}
	if got := resolvePublishPlacement(job, "finding", "review", "docs/REVIEW.md", 1); got != artifactcontracts.PlacementBlobExhaust {
		t.Fatalf("finding placement = %q, want blob default", got)
	}
}

func TestPublishBlobRequiredPlacementRefusesWithoutBlobClient(t *testing.T) {
	if !haveGit(t) {
		return
	}
	previous := packageBlobClient
	packageBlobClient = nil
	t.Cleanup(func() { packageBlobClient = previous })

	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "placement_blob_required", true)
	setExpectedArtifactPlacement(t, ctx, runner, ids, artifactcontracts.PlacementBlobExhaust)
	setWorkflowBlobRequiredPosture(t, ctx, runner, ids)
	writeWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", "blob-required body\n")

	_, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"kind":         "handoff",
		"logical_name": "out",
		"path":         "docs/out.txt",
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "blob_disabled" {
		t.Fatalf("publish error = %v, want blob_disabled", err)
	}
	count, err := runner.QueryScalar(ctx, `
		SELECT count(*) FROM striatumd.artifacts
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("count artifact rows: %v", err)
	}
	if fmt.Sprint(count) != "0" {
		t.Fatalf("artifact rows after refused publish = %v, want 0", count)
	}
}

func TestPublishGitPlacementsIgnoreBlobRequiredPostureWithoutBlobClient(t *testing.T) {
	if !haveGit(t) {
		return
	}
	previous := packageBlobClient
	packageBlobClient = nil
	t.Cleanup(func() { packageBlobClient = previous })

	for _, placement := range []string{
		artifactcontracts.PlacementGitPublication,
		artifactcontracts.PlacementGitPointerManifest,
	} {
		t.Run(placement, func(t *testing.T) {
			ctx := context.Background()
			runner := pgtest.Pool(t).Runner
			repoRoot := t.TempDir()
			ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "placement_"+placement, true)
			setExpectedArtifactPlacement(t, ctx, runner, ids, placement)
			setWorkflowBlobRequiredPosture(t, ctx, runner, ids)
			writeWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", "git placement body\n")

			result, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
				"session_id":   ids.sessionID,
				"job_id":       ids.jobID,
				"lease_id":     ids.leaseID,
				"kind":         "handoff",
				"logical_name": "out",
				"path":         "docs/out.txt",
			}))
			if err != nil {
				t.Fatalf("publish git placement: %v", err)
			}
			if result["placement"] != placement {
				t.Fatalf("result placement = %v, want %s", result["placement"], placement)
			}
			placementColumnPresent := db.ArtifactPlacementColumnPresent(ctx, runner)
			query := `
				SELECT blob_key FROM striatumd.artifacts
				 WHERE repository_id = $1 AND job_id = $2 AND logical_name = 'out'
				 LIMIT 1`
			if placementColumnPresent {
				query = `
					SELECT placement, blob_key FROM striatumd.artifacts
					 WHERE repository_id = $1 AND job_id = $2 AND logical_name = 'out'
					 LIMIT 1`
			}
			row, err := oneRow(ctx, runner, query, ids.repoID, ids.jobID)
			if err != nil {
				t.Fatalf("read artifact row: %v", err)
			}
			if placementColumnPresent && row["placement"] != placement {
				t.Fatalf("row placement = %v, want %s", row["placement"], placement)
			}
			if row["blob_key"] != nil {
				t.Fatalf("git placement must not record blob_key, got %#v", row["blob_key"])
			}
		})
	}
}

func setExpectedArtifactPlacement(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs, placement string) {
	t.Helper()
	expectedArg, err := db.JSONBArg(runner, []any{map[string]any{
		"logical_name": "out",
		"kind":         "handoff",
		"path":         "docs/out.txt",
		"placement":    placement,
		"required":     true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET expected_artifacts_json = $3::jsonb
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID, expectedArg); err != nil {
		t.Fatalf("set expected artifact placement: %v", err)
	}
}

func setWorkflowBlobRequiredPosture(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs) {
	t.Helper()
	snapshotID := snapshotIDForRun(t, ctx, runner, ids.repoID, ids.runID)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.workflow_snapshots
		   SET workflow_json = jsonb_set(workflow_json, '{artifact_placement_posture}', to_jsonb($3::text), true)
		 WHERE repository_id = $1 AND workflow_snapshot_id = $2`,
		ids.repoID, snapshotID, artifactcontracts.BlobRequiredPosture); err != nil {
		t.Fatalf("set workflow blob-required posture: %v", err)
	}
}
