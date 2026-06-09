package mutations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestRecoveryResumeWriteScopeBlockerRequeuesAfterDirtyPathCleared(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "recover_write_scope", true)
	dirtyPath := filepath.Join(ids.worktreeRoot, "outside.txt")
	if err := os.WriteFile(dirtyPath, []byte("operator dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked, err := HandleBlockWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":  ids.sessionID,
		"job_id":      ids.jobID,
		"lease_id":    ids.leaseID,
		"severity":    "blocked",
		"kind":        "write_scope.out_of_scope_dirty",
		"description": "out-of-scope dirty path blocked completion",
	}))
	if err != nil {
		t.Fatalf("work.block: %v", err)
	}
	if err := os.Remove(dirtyPath); err != nil {
		t.Fatalf("clear dirty path: %v", err)
	}

	result, err := HandleRecoveryResume(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"blocker_id": fmt.Sprint(blocked["blocker_id"]),
		"session_id": ids.sessionID,
		"complete":   true,
		"summary":    "cleared out-of-scope dirty path",
	}))
	if err != nil {
		t.Fatalf("recovery.resume: %v", err)
	}
	if result["status"] != "resumed_requeued" || result["completed_inline"] != false {
		t.Fatalf("resume result = %#v, want same-attempt requeue without inline completion", result)
	}
	job, err := oneRow(ctx, runner, `
		SELECT state, attempt
		  FROM striatumd.jobs
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	if job["state"] != "queued" || intValue(job["attempt"]) != 1 {
		t.Fatalf("job row = %#v, want queued same attempt", job)
	}
	message, err := oneRow(ctx, runner, `
		SELECT state
		  FROM striatumd.queue_messages
		 WHERE repository_id = $1 AND message_id = $2`, ids.repoID, ids.messageID)
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if message["state"] != "pending" {
		t.Fatalf("message state = %#v, want pending", message["state"])
	}
	blocker, err := oneRow(ctx, runner, `
		SELECT state
		  FROM striatumd.blockers
		 WHERE repository_id = $1 AND blocker_id = $2`, ids.repoID, blocked["blocker_id"])
	if err != nil {
		t.Fatalf("read blocker: %v", err)
	}
	if blocker["state"] != "resolved" {
		t.Fatalf("blocker state = %#v, want resolved", blocker["state"])
	}
}
