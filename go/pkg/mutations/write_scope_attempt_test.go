package mutations

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestArtifactPublishUsesFrozenAttemptScopeAfterJobScopeWidened(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "frozen_publish", true)
	stampFrozenPacketScope(t, ctx, runner, ids, []any{"docs/"})
	widenJobScope(t, ctx, runner, ids, []any{"docs/", "other/"})
	target := filepath.Join(ids.worktreeRoot, "other", "out.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("outside frozen scope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"kind":         "handoff",
		"logical_name": "outside",
		"path":         "other/out.txt",
	}))
	assertWriteScopeDrift(t, err)
}

func TestCompleteUsesFrozenAttemptScopeAfterJobScopeWidened(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "frozen_complete", true)
	stampFrozenPacketScope(t, ctx, runner, ids, []any{"docs/"})
	widenJobScope(t, ctx, runner, ids, []any{"docs/", "other/"})
	target := filepath.Join(ids.worktreeRoot, "other", "out.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("outside frozen scope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	assertWriteScopeDrift(t, err)
}

func TestSameAttemptReclaimCarriesFrozenScope(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "frozen_reclaim", false)
	stampFrozenPacketScope(t, ctx, runner, ids, []any{"docs/"})
	widenJobScope(t, ctx, runner, ids, []any{"docs/", "other/"})
	resetJobForSameAttemptReclaim(t, ctx, runner, ids)
	freshSession := "sess_fresh_" + ids.repoID
	intgSeedSessionOrdinal(t, ctx, runner, ids.repoID, ids.runID, freshSession, "author", "codex", []string{"write"}, "active", 2)
	intgAttest(t, ctx, runner, ids.repoID, ids.runID, freshSession, "codex")

	claim, err := HandleClaimNext(ctx, runner, intgEnv(ids.repoID, map[string]any{"session_id": freshSession}))
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	packet := asMap(claim["packet"])
	scope := asMap(packet["write_scope"])
	allowed := stringListFromAny(scope["allowed_paths"])
	if len(allowed) != 1 || allowed[0] != "docs/" {
		t.Fatalf("claimed packet allowed_paths = %#v, want frozen [docs/]; packet=%#v", allowed, packet)
	}
}

func stampFrozenPacketScope(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs, allowed []any) {
	t.Helper()
	packetArg, err := db.JSONBArg(runner, map[string]any{
		"packet_id": ids.messageID,
		"job":       map[string]any{"attempt": 1},
		"write_scope": map[string]any{
			"mode":            "repo_write",
			"repo_write":      true,
			"allowed_paths":   allowed,
			"forbidden_paths": []any{".striatum/"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.work_packets
		   SET packet_json = $1::jsonb
		 WHERE repository_id = $2 AND job_id = $3 AND lease_id = $4`,
		packetArg, ids.repoID, ids.jobID, ids.leaseID); err != nil {
		t.Fatalf("stamp packet scope: %v", err)
	}
}

func widenJobScope(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs, allowed []any) {
	t.Helper()
	scopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":            "repo_write",
		"repo_write":      true,
		"allowed_paths":   allowed,
		"forbidden_paths": []any{".striatum/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET write_scope_json = $1::jsonb
		 WHERE repository_id = $2 AND job_id = $3`,
		scopeArg, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("widen job scope: %v", err)
	}
}

func resetJobForSameAttemptReclaim(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'test_requeue'
		 WHERE repository_id = $2 AND lease_id = $3`,
		now, ids.repoID, ids.leaseID); err != nil {
		t.Fatalf("release lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.queue_messages
		   SET state = 'pending', current_lease_id = NULL, updated_at = $1
		 WHERE repository_id = $2 AND message_id = $3`,
		now, ids.repoID, ids.messageID); err != nil {
		t.Fatalf("reset queue message: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'queued', current_lease_id = NULL
		 WHERE repository_id = $1 AND job_id = $2`,
		ids.repoID, ids.jobID); err != nil {
		t.Fatalf("reset job: %v", err)
	}
}

func assertWriteScopeDrift(t *testing.T, err error) {
	t.Helper()
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "write_scope_drift" {
		t.Fatalf("error = %v, want write_scope_drift", err)
	}
	if !strings.Contains(rpcErr.Message, "frozen write_scope") || !strings.Contains(rpcErr.Message, "recovery resume") {
		t.Fatalf("message = %q, want frozen-scope recovery guidance", rpcErr.Message)
	}
}
