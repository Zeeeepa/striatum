package mutations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// attemptScopeFixture seeds a repo+run+session+job+lease with a real on-disk
// repo root so HandlePublishArtifact runs end-to-end. The job uses a non-
// markdown ("handoff", .txt) artifact so the author-line/front-matter/lane-
// attestation checks are skipped — the test isolates RFC 0095 §1 / #84
// attempt-scoped publish + gate behavior.
func attemptScopeFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, jobID, sessionID, leaseID, repoRoot string) {
	t.Helper()
	repoRoot = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	runID = "run_" + repoID
	jobID = "job_" + repoID
	sessionID = "sess_" + repoID
	leaseID = "lease_" + repoID

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,18,'active')`,
		repoID, "ident_"+repoID, repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	workflowArg, err := db.JSONBArg(runner, map[string]any{"workflow_id": "wf", "lanes": map[string]any{"claude": map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha',$3::jsonb,$4)`,
		repoID, "snap_"+runID, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,$2,$3,$4,'running',$5)`,
		repoID, runID, "snap_"+runID, repoRoot, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at
		) VALUES ($1,$2,$3,'implementer','claude',$4,1,'active',$5)`,
		repoID, sessionID, runID, sessionID+"-slug", now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode": "repo_write", "repo_write": true, "allowed_paths": []string{"docs/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, title, job_type, role_id,
		  lane_selector_json, state, write_scope_json, idempotency_key, created_at
		) VALUES ($1,$2,$3,'build',1,'Build','build','implementer',
		  $4::jsonb,'running',$5::jsonb,'idem_'||$1,$6)`,
		repoID, jobID, runID, laneSelectorArg, writeScopeArg, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	return runID, jobID, sessionID, leaseID, repoRoot
}

func bumpJobAttempt(t *testing.T, ctx context.Context, runner db.Runner, repoID, jobID string, attempt int) {
	t.Helper()
	if err := runner.Exec(ctx, `UPDATE striatumd.jobs SET attempt = $3 WHERE repository_id = $1 AND job_id = $2`,
		repoID, jobID, attempt); err != nil {
		t.Fatalf("bump attempt: %v", err)
	}
}

func writeArtifactFile(t *testing.T, repoRoot, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoRoot, rel), []byte(body), 0o644); err != nil {
		t.Fatalf("write artifact file %s: %v", rel, err)
	}
}

func publishViaHandler(t *testing.T, ctx context.Context, runner db.Runner, repoID, sessionID, jobID, leaseID, logicalName, rel string) (map[string]any, error) {
	t.Helper()
	return HandlePublishArtifact(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":   sessionID,
		"job_id":       jobID,
		"lease_id":     leaseID,
		"kind":         "handoff",
		"logical_name": logicalName,
		"path":         rel,
	}))
}

func artifactRowsForLogical(t *testing.T, ctx context.Context, runner db.Runner, repoID, jobID, logicalName string) []map[string]any {
	t.Helper()
	rows, err := queryRows(ctx, runner, `
		SELECT artifact_id, attempt, content_sha256 FROM striatumd.artifacts
		 WHERE repository_id = $1 AND job_id = $2 AND logical_name = $3
		 ORDER BY attempt`, repoID, jobID, logicalName)
	if err != nil {
		t.Fatalf("select artifact rows: %v", err)
	}
	return rows
}

// TestAttemptScopedRepublishSameLogicalNameSucceeds is the load-bearing RFC 0095
// §1 / #84 fix: a re-opened job (attempt bumped) republishing the SAME fixed
// logical_name with DIFFERENT content succeeds — no `already exists` error — and
// both attempts' rows coexist with distinct `attempt` values.
func TestAttemptScopedRepublishSameLogicalNameSucceeds(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_attempt_republish"
	runID, jobID, sessionID, leaseID, repoRoot := attemptScopeFixture(t, ctx, runner, repoID)
	_ = runID

	rel := "docs/build.txt"
	logicalName := "build_output"

	// Attempt 1 publish.
	writeArtifactFile(t, repoRoot, rel, "attempt-1 content")
	res1, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel)
	if err != nil {
		t.Fatalf("attempt 1 publish: %v", err)
	}
	if fmt.Sprint(res1["status"]) != "published" {
		t.Fatalf("attempt 1 status = %v, want published", res1["status"])
	}

	// Re-open the job to attempt 2 and republish the SAME logical_name with
	// DIFFERENT content at the SAME path. Pre-#84 this raised the
	// `already exists with different content` conflict on the
	// (run_id, job_id, logical_name) unique key.
	bumpJobAttempt(t, ctx, runner, repoID, jobID, 2)
	writeArtifactFile(t, repoRoot, rel, "attempt-2 REVISED content")
	res2, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel)
	if err != nil {
		t.Fatalf("attempt 2 republish must succeed (the #84 fix), got: %v", err)
	}
	if fmt.Sprint(res2["status"]) != "published" {
		t.Fatalf("attempt 2 status = %v, want published; res=%#v", res2["status"], res2)
	}
	if res1["artifact_id"] == res2["artifact_id"] {
		t.Fatalf("attempt 2 must be a fresh row, got the same artifact_id %v", res2["artifact_id"])
	}

	// Both rows exist with distinct attempts.
	rows := artifactRowsForLogical(t, ctx, runner, repoID, jobID, logicalName)
	if len(rows) != 2 {
		t.Fatalf("expected 2 attempt-scoped rows, got %d: %#v", len(rows), rows)
	}
	if intValue(rows[0]["attempt"]) != 1 || intValue(rows[1]["attempt"]) != 2 {
		t.Fatalf("expected attempts [1,2], got [%v,%v]", rows[0]["attempt"], rows[1]["attempt"])
	}
	if rows[0]["content_sha256"] == rows[1]["content_sha256"] {
		t.Fatalf("the two attempts should carry different content digests")
	}
}

// TestVerifyRequiredArtifactsHonorsCurrentAttempt is RFC 0095 §1: the gate reader
// is satisfied ONLY by an artifact at the job's current attempt. A job at attempt
// 2 is NOT satisfied by its attempt-1 artifact, and IS satisfied once the
// attempt-2 artifact is published.
func TestVerifyRequiredArtifactsHonorsCurrentAttempt(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_attempt_gate"
	runID, jobID, sessionID, leaseID, repoRoot := attemptScopeFixture(t, ctx, runner, repoID)

	rel := "docs/build.txt"
	logicalName := "build_output"
	// Declare a fixed (non-${cycle}) required expected artifact.
	expectedArg, err := db.JSONBArg(runner, []any{
		map[string]any{"required": true, "kind": "handoff", "logical_name": logicalName, "path": rel},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET expected_artifacts_json = $3::jsonb
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID, expectedArg); err != nil {
		t.Fatalf("set expected artifacts: %v", err)
	}

	// Attempt 1: publish and confirm the gate is satisfied.
	writeArtifactFile(t, repoRoot, rel, "attempt-1 content")
	if _, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel); err != nil {
		t.Fatalf("attempt 1 publish: %v", err)
	}
	if err := verifyRequiredArtifacts(ctx, runner, repoID, jobID); err != nil {
		t.Fatalf("attempt 1 gate should be satisfied, got: %v", err)
	}

	// Re-open to attempt 2: the attempt-1 artifact must NOT satisfy the gate.
	bumpJobAttempt(t, ctx, runner, repoID, jobID, 2)
	if err := verifyRequiredArtifacts(ctx, runner, repoID, jobID); err == nil {
		t.Fatalf("attempt 2 gate must NOT be satisfied by the attempt-1 artifact, but it was")
	}

	// Publish the attempt-2 artifact: the gate is satisfied again.
	writeArtifactFile(t, repoRoot, rel, "attempt-2 content")
	if _, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel); err != nil {
		t.Fatalf("attempt 2 publish: %v", err)
	}
	if err := verifyRequiredArtifacts(ctx, runner, repoID, jobID); err != nil {
		t.Fatalf("attempt 2 gate should be satisfied after the attempt-2 publish, got: %v", err)
	}
	_ = runID
}

// TestSameAttemptCollisionStillRejected is the preserved #94 invariant: within a
// SINGLE attempt, republishing the same logical_name with DIFFERENT content is
// still rejected as an in-attempt immutability conflict.
func TestSameAttemptCollisionStillRejected(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_attempt_same_collision"
	_, jobID, sessionID, leaseID, repoRoot := attemptScopeFixture(t, ctx, runner, repoID)

	rel := "docs/build.txt"
	logicalName := "build_output"
	writeArtifactFile(t, repoRoot, rel, "first content")
	if _, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	// Same attempt (still 1), same logical_name, DIFFERENT content -> rejected.
	writeArtifactFile(t, repoRoot, rel, "different content same attempt")
	_, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel)
	if err == nil {
		t.Fatalf("same-attempt republish with different content must be rejected, but it succeeded")
	}
}

// TestSameAttemptIdempotentRepublishNoOp is the preserved #58 invariant: the same
// logical_name + identical content at the SAME attempt is an idempotent no-op
// success reusing the existing artifact_id.
func TestSameAttemptIdempotentRepublishNoOp(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_attempt_idempotent"
	_, jobID, sessionID, leaseID, repoRoot := attemptScopeFixture(t, ctx, runner, repoID)

	rel := "docs/build.txt"
	logicalName := "build_output"
	writeArtifactFile(t, repoRoot, rel, "stable content")
	res1, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	// Same attempt, same logical_name, IDENTICAL content -> idempotent no-op.
	res2, err := publishViaHandler(t, ctx, runner, repoID, sessionID, jobID, leaseID, logicalName, rel)
	if err != nil {
		t.Fatalf("idempotent republish must succeed, got: %v", err)
	}
	if fmt.Sprint(res2["status"]) != "already_published" {
		t.Fatalf("idempotent republish status = %v, want already_published", res2["status"])
	}
	if fmt.Sprint(res1["artifact_id"]) != fmt.Sprint(res2["artifact_id"]) {
		t.Fatalf("idempotent republish must reuse the artifact_id, got %v vs %v", res1["artifact_id"], res2["artifact_id"])
	}
	// Exactly one row exists for this attempt.
	rows := artifactRowsForLogical(t, ctx, runner, repoID, jobID, logicalName)
	if len(rows) != 1 {
		t.Fatalf("idempotent no-op must leave exactly 1 row, got %d", len(rows))
	}
}
