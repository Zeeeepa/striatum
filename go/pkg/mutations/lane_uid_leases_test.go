package mutations

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestClassifyProcStateBlocksStoppedTracedLiveAndUnknown(t *testing.T) {
	for _, state := range []byte{'R', 'S', 'D', 'T', 't', 'I', 'P'} {
		got, err := classifyProcState(state)
		if err != nil {
			t.Fatalf("classifyProcState(%q): %v", state, err)
		}
		if got != laneUIDTaskLive {
			t.Fatalf("classifyProcState(%q) = %q, want live", state, got)
		}
	}
	for _, state := range []byte{'Z', 'X', 'x'} {
		got, err := classifyProcState(state)
		if err != nil {
			t.Fatalf("classifyProcState(%q): %v", state, err)
		}
		if got != laneUIDTaskReaped {
			t.Fatalf("classifyProcState(%q) = %q, want reaped", state, got)
		}
	}
	if _, err := classifyProcState('W'); err == nil {
		t.Fatalf("classifyProcState(W) succeeded; unknown proc states must fail closed")
	}
}

func TestLaneUIDProcessProofRecordsStateEvidenceAndBlocksUnknown(t *testing.T) {
	origReadDir := laneUIDReadDir
	origReadFile := laneUIDReadFile
	t.Cleanup(func() {
		laneUIDReadDir = origReadDir
		laneUIDReadFile = origReadFile
	})

	laneUIDReadDir = func(path string) ([]os.DirEntry, error) {
		if path != "/proc" {
			t.Fatalf("unexpected readdir path %q", path)
		}
		return []os.DirEntry{
			fakeDirEntry{name: "101", dir: true},
			fakeDirEntry{name: "102", dir: true},
			fakeDirEntry{name: "103", dir: true},
			fakeDirEntry{name: "self", dir: true},
		}, nil
	}
	laneUIDReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/proc/101/status", "/proc/102/status", "/proc/103/status":
			return []byte("Name:\ttest\nUid:\t65001\t65001\t65001\t65001\n"), nil
		case "/proc/101/stat":
			return []byte("101 (sleep) S 1 1 1 0 -1 0 0 0 0 0"), nil
		case "/proc/102/stat":
			return []byte("102 (gone) Z 1 1 1 0 -1 0 0 0 0 0"), nil
		case "/proc/103/stat":
			return []byte("103 (future) W 1 1 1 0 -1 0 0 0 0 0"), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	proof := baseLaneUIDScrubProof(65001, "lane-pool-1", "sup_1")
	err := appendLaneUIDScrubProof(context.Background(), nil, "repo_1", "luid_1", 65001, "lane-pool-1", "sup_1", "/repo", proof)
	if err == nil || !strings.Contains(err.Error(), "non-reaped or unknown") {
		t.Fatalf("appendLaneUIDScrubProof err = %v, want blocking P1 failure", err)
	}
	checks := proof["checks"].(map[string]any)
	p1 := checks["p1_processes_for_uid"].(map[string]any)
	if p1["observed_count"] != 3 || p1["blocking_count"] != 2 {
		t.Fatalf("p1 proof = %#v, want 3 observed / 2 blocking", p1)
	}
	observations := p1["observations"].([]map[string]any)
	if observations[0]["state"] != "S" || observations[0]["classification"] != laneUIDTaskLive {
		t.Fatalf("live observation = %#v", observations[0])
	}
	if observations[1]["state"] != "Z" || observations[1]["classification"] != laneUIDTaskReaped {
		t.Fatalf("reaped observation = %#v", observations[1])
	}
	if observations[2]["state"] != "W" || observations[2]["classification"] != laneUIDTaskUnknown || observations[2]["error"] == nil {
		t.Fatalf("unknown observation = %#v", observations[2])
	}
}

type fakeDirEntry struct {
	name string
	dir  bool
}

func (e fakeDirEntry) Name() string {
	return e.name
}

func (e fakeDirEntry) IsDir() bool {
	return e.dir
}

func (e fakeDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}

func (e fakeDirEntry) Info() (fs.FileInfo, error) {
	return nil, nil
}

func TestScrubLaneUIDArtifactsFailsClosedOnKillFailure(t *testing.T) {
	origExec := laneUIDExec
	origRemoveAll := laneUIDRemoveAll
	origHome := laneOSUserHome
	t.Cleanup(func() {
		laneUIDExec = origExec
		laneUIDRemoveAll = origRemoveAll
		laneOSUserHome = origHome
	})

	home := t.TempDir()
	repo := t.TempDir()
	laneUIDExec = func(command string, args ...string) error {
		return errors.New("kill refused")
	}
	laneUIDRemoveAll = func(path string) error {
		return nil
	}
	laneOSUserHome = func(name string) string {
		if name == "lane-pool-1" {
			return home
		}
		return ""
	}

	proof := baseLaneUIDScrubProof(1234, "lane-pool-1", "sup_1")
	err := scrubLaneUIDArtifacts("lane-pool-1", "sup_1", repo, proof)
	if err == nil || !strings.Contains(err.Error(), "kill lane uid process domain") {
		t.Fatalf("err = %v, want fail-closed kill error", err)
	}
	checks := proof["checks"].(map[string]any)
	if got := checks["s1_kill_all"]; !strings.Contains(got.(string), "failed") {
		t.Fatalf("s1_kill_all = %#v, want failed diagnostic", got)
	}
	if _, ok := checks["p4_acl_worktree_cleanup"]; ok {
		t.Fatalf("P4 must not be a deferred placeholder: %#v", checks)
	}
	if _, ok := checks["p5_proof_recorded"]; ok {
		t.Fatalf("P5 must not be unconditional: %#v", checks)
	}
}

func TestLaneUIDHomeCleanupProofFailsWhenCredentialStoreRemains(t *testing.T) {
	origHome := laneOSUserHome
	t.Cleanup(func() { laneOSUserHome = origHome })

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir residue: %v", err)
	}
	laneOSUserHome = func(name string) string {
		if name == "lane-pool-1" {
			return home
		}
		return ""
	}

	checks := map[string]any{}
	err := proveLaneUIDHomeCleanup("lane-pool-1", checks)
	if err == nil || !strings.Contains(err.Error(), ".claude") {
		t.Fatalf("err = %v, want credential residue proof failure", err)
	}
	if checks["p3_p4_home_cleanup"] == nil {
		t.Fatalf("proof failure should be recorded in checks: %#v", checks)
	}
}

func TestLaneUIDWorkspaceCleanupProofBlocksActiveOrAbandonedWork(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  string
		state string
	}{
		{name: "active_worktree", kind: "worktree", state: "active"},
		{name: "abandoned_worktree", kind: "worktree", state: "abandoned"},
		{name: "active_workspace", kind: "workspace", state: "active"},
		{name: "abandoned_workspace", kind: "workspace", state: "abandoned"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			runner := pgtest.Pool(t).Runner
			stubCleanLaneUIDScrubSideEffects(t)
			fx := seedLaneUIDLeaseFixture(t, ctx, runner, tc.name, laneUIDLeaseStateActive)
			if tc.kind == "worktree" {
				insertLaneUIDLeaseWorktree(t, ctx, runner, fx, tc.state)
			} else {
				insertLaneUIDLeaseWorkspace(t, ctx, runner, fx, tc.state)
			}

			result := scrubLaneUIDLeasesForSession(ctx, runner, fx.repoID, fx.sessionID, "test: p5 dirty")
			if result["scrubbed_count"] != 1 {
				t.Fatalf("scrubbed_count = %#v, want 1; result=%#v", result["scrubbed_count"], result)
			}
			row := laneUIDLeaseRow(t, ctx, runner, fx)
			if row["state"] != laneUIDLeaseStateQuarantined || row["scrub_status"] != "failed" {
				t.Fatalf("lease row = %#v, want quarantined failed", row)
			}
			if failure := rowString(row, "scrub_failure"); !strings.Contains(failure, "active or abandoned worktrees/workspaces") {
				t.Fatalf("scrub_failure = %q, want P5 worktree/workspace failure", failure)
			}
			proof := asMap(row["scrub_proof"])
			checks := asMap(proof["checks"])
			p5 := asMap(checks["p5_workspace_acl_cleanup"])
			if n, ok := intValueOptional(p5["unclean_count"]); !ok || n != 1 {
				t.Fatalf("p5 cleanup proof = %#v, want one unclean row", p5)
			}
			if _, ok := checks["p5_complete_proof"]; ok {
				t.Fatalf("P5 complete proof must not be recorded on dirty work: %#v", checks)
			}
		})
	}
}

func TestRecoverLaneUIDLeasesRetriesQuarantinedOnlyAfterCleanP5Proof(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	stubCleanLaneUIDScrubSideEffects(t)
	fx := seedLaneUIDLeaseFixture(t, ctx, runner, "retry_quarantined", laneUIDLeaseStateQuarantined)
	insertLaneUIDLeaseWorktree(t, ctx, runner, fx, "active")

	preview := recoverLaneUIDLeases(ctx, runner, fx.repoID, fx.runID, false, false)
	quarantined := preview["quarantined"].([]map[string]any)
	if len(quarantined) != 1 || quarantined[0]["action"] != "operator_retry_required" {
		t.Fatalf("preview quarantined = %#v, want operator retry hint", quarantined)
	}

	dirtyRetry := recoverLaneUIDLeases(ctx, runner, fx.repoID, fx.runID, false, true)
	quarantined = dirtyRetry["quarantined"].([]map[string]any)
	if len(quarantined) != 1 || quarantined[0]["state"] != laneUIDLeaseStateQuarantined {
		t.Fatalf("dirty retry quarantined = %#v, want still quarantined", quarantined)
	}
	if row := laneUIDLeaseRow(t, ctx, runner, fx); row["state"] != laneUIDLeaseStateQuarantined || row["scrub_status"] != "failed" {
		t.Fatalf("dirty retry row = %#v, want quarantined failed", row)
	}

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.job_worktrees
		   SET state = 'released', released_at = $3
		 WHERE repository_id = $1 AND worktree_id = $2`,
		fx.repoID, "wt_"+fx.suffix, now); err != nil {
		t.Fatalf("release worktree: %v", err)
	}

	cleanRetry := recoverLaneUIDLeases(ctx, runner, fx.repoID, fx.runID, false, true)
	quarantined = cleanRetry["quarantined"].([]map[string]any)
	if len(quarantined) != 1 || quarantined[0]["state"] != laneUIDLeaseStateReturned {
		t.Fatalf("clean retry quarantined = %#v, want returned item", quarantined)
	}
	row := laneUIDLeaseRow(t, ctx, runner, fx)
	if row["state"] != laneUIDLeaseStateReturned || row["scrub_status"] != "clean" {
		t.Fatalf("clean retry row = %#v, want returned clean", row)
	}
	proof := asMap(row["scrub_proof"])
	checks := asMap(proof["checks"])
	if checks["p5_complete_proof"] != true {
		t.Fatalf("checks = %#v, want complete P5 proof after clean retry", checks)
	}
}

type laneUIDLeaseFixture struct {
	repoID       string
	runID        string
	sessionID    string
	jobID        string
	workLeaseID  string
	uidLeaseID   string
	supervisorID string
	suffix       string
}

func seedLaneUIDLeaseFixture(t *testing.T, ctx context.Context, runner db.Runner, suffix, state string) laneUIDLeaseFixture {
	t.Helper()
	fx := laneUIDLeaseFixture{
		repoID:       "repo_luid_" + suffix,
		runID:        "run_luid_" + suffix,
		sessionID:    "sess_luid_" + suffix,
		jobID:        "job_luid_" + suffix,
		workLeaseID:  "lease_work_" + suffix,
		uidLeaseID:   "luid_" + suffix,
		supervisorID: "sup_luid_" + suffix,
		suffix:       suffix,
	}
	now := time.Now().UTC()
	intgSeedRepo(t, ctx, runner, fx.repoID)
	intgSeedRun(t, ctx, runner, fx.repoID, fx.runID, map[string]any{
		"workflow_id": "wf_luid",
		"roles":       map[string]any{"author": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})
	intgSeedSession(t, ctx, runner, fx.repoID, fx.runID, fx.sessionID, "author", "codex", []string{"write"}, "closed")
	intgSeedClaimableWork(t, ctx, runner, fx.repoID, fx.runID, fx.jobID, "draft", "author", "codex")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',$6,$7,$6,'test')`,
		fx.repoID, fx.workLeaseID, fx.runID, fx.jobID, fx.sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert job lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.lane_uid_leases (
		  repository_id, lease_id, pool_uid, pool_user, generation,
		  run_id, session_id, supervisor_id, state, leased_at
		) VALUES ($1,$2,65001,'lane-pool-1',1,$3,$4,$5,$6,$7)`,
		fx.repoID, fx.uidLeaseID, fx.runID, fx.sessionID, fx.supervisorID, state, now); err != nil {
		t.Fatalf("insert lane uid lease: %v", err)
	}
	return fx
}

func insertLaneUIDLeaseWorktree(t *testing.T, ctx context.Context, runner db.Runner, fx laneUIDLeaseFixture, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_worktrees (
		  repository_id, worktree_id, run_id, job_id, lease_id,
		  base_branch, worktree_path, state, created_at
		) VALUES ($1,$2,$3,$4,$5,'main',$6,$7,$8)`,
		fx.repoID, "wt_"+fx.suffix, fx.runID, fx.jobID, fx.workLeaseID,
		".striatum/worktrees/wt_"+fx.suffix, state, time.Now().UTC()); err != nil {
		t.Fatalf("insert job worktree: %v", err)
	}
}

func insertLaneUIDLeaseWorkspace(t *testing.T, ctx context.Context, runner db.Runner, fx laneUIDLeaseFixture, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_workspaces (
		  repository_id, workspace_id, run_id, job_id, lease_id,
		  workspace_kind, base_branch, base_tree_sha, workspace_path, state, created_at
		) VALUES ($1,$2,$3,$4,$5,'plain_dir','main','tree',$6,$7,$8)`,
		fx.repoID, "ws_"+fx.suffix, fx.runID, fx.jobID, fx.workLeaseID,
		".striatum/workspaces/ws_"+fx.suffix, state, time.Now().UTC()); err != nil {
		t.Fatalf("insert job workspace: %v", err)
	}
}

func laneUIDLeaseRow(t *testing.T, ctx context.Context, runner db.Runner, fx laneUIDLeaseFixture) map[string]any {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT state, scrub_status, scrub_failure, scrub_proof
		  FROM striatumd.lane_uid_leases
		 WHERE repository_id = $1 AND lease_id = $2`,
		fx.repoID, fx.uidLeaseID)
	if err != nil {
		t.Fatalf("read lane uid lease: %v", err)
	}
	return row
}

func stubCleanLaneUIDScrubSideEffects(t *testing.T) {
	t.Helper()
	origExec := laneUIDExec
	origRemoveAll := laneUIDRemoveAll
	origReadDir := laneUIDReadDir
	origStat := laneUIDStat
	origHome := laneOSUserHome
	home := t.TempDir()
	t.Cleanup(func() {
		laneUIDExec = origExec
		laneUIDRemoveAll = origRemoveAll
		laneUIDReadDir = origReadDir
		laneUIDStat = origStat
		laneOSUserHome = origHome
	})
	laneUIDExec = func(command string, args ...string) error { return nil }
	laneUIDRemoveAll = func(path string) error { return nil }
	laneUIDReadDir = func(path string) ([]os.DirEntry, error) { return nil, nil }
	laneUIDStat = func(path string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	laneOSUserHome = func(name string) string {
		if name == "lane-pool-1" {
			return home
		}
		return ""
	}
}
