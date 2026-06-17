package mutations

import (
	"context"
	"os/exec"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

// TestSweep317ReopensFreshAttemptForStalePublishedArtifact reproduces #317: a
// dead-lane job whose required artifact ROW was already published THIS attempt
// but whose body is NOT durable (the prior session's worktree commit never
// integrated to the run branch). #308 finalize cannot fire (non-durable), and a
// same-attempt requeue would trap the re-run on artifact_immutable_byline_mismatch
// (the (logical_name, attempt) row is immutable and append-only, so the re-run
// can neither republish nor complete). The sweep must REOPEN ON A FRESH ATTEMPT
// instead, so the re-run publishes into a clean namespace.
func TestSweep317ReopensFreshAttemptForStalePublishedArtifact(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "stale_artifact_317", true)

	// Drive the #308 dead-unsealed shape (declares the required artifact,
	// max_attempts=1, dead-but-active session, expired lease).
	seedUnsealedPublishedFinalJob308(t, ctx, runner, ids)
	// Seed the published artifact ROW (publish happened this attempt) but DO NOT
	// make its body durable: no run-branch commit and no copy at repo_root. So
	// verifyRequiredArtifacts passes (row exists) while the reconstructability
	// probe fails (body not durable) — the exact #317 trap.
	seedPublishedArtifact(t, ctx, runner, ids, "art_stale_317", "out", "docs/out.txt", []byte("non-durable body\n"), nil)
	// Anchor an attempt pin (as a real publish does via pinWorktreeCommitStack) at
	// a commit that does NOT carry docs/out.txt, so the reconstructability probe
	// returns positive loss (missing_from_git_anchor) rather than a "pending,
	// warn" status — which is what makes #308 finalize refuse and the same-attempt
	// requeue trap form. This mirrors the real #317: the worktree commit was
	// pinned but never integrated to the run branch.
	pinAt := gitRevParse(t, repoRoot, "refs/heads/"+ids.runBranch)
	gitRun(t, repoRoot, "update-ref", attemptPinRef(ids.runID, ids.jobID, 1), pinAt)

	restore := probeLaneLiveness
	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		return gosupervisor.LaneLiveness{Backed: "tmux", Alive: false, Class: string(gosupervisor.TmuxLivenessPaneDead), ObservedPID: 8888}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	result, err := SweepRun(ctx, runner, ids.repoID, ids.runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The job must NOT auto-finalize (body is non-durable) and must NOT be left
	// blocked/wedged: it is reopened on a fresh attempt and is reclaimable again.
	if got := jobState(t, ctx, runner, ids.repoID, ids.jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued (reopened on a fresh attempt); recovery_actions=%#v", got, result["recovery_actions"])
	}
	// attempt bumped 1 -> 2, and max_attempts bumped in lockstep so the re-claim is
	// within budget (this is recovery clearing its own partial work, not a retry).
	attempt := scalarInt(t, ctx, runner, `SELECT attempt FROM striatumd.jobs WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID)
	maxAttempts := scalarInt(t, ctx, runner, `SELECT max_attempts FROM striatumd.jobs WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID)
	if attempt != 2 {
		t.Fatalf("attempt = %d, want 2 (reopened on a fresh attempt)", attempt)
	}
	if maxAttempts < 2 {
		t.Fatalf("max_attempts = %d, want >= 2 (bumped in lockstep so the re-claim is not exhausted)", maxAttempts)
	}

	// The decision tree reports the distinct action and a provenance event.
	summary := recoveryActionsFromSweep(t, result)
	acts, _ := summary["actions"].([]map[string]any)
	found := false
	for _, a := range acts {
		if a["action"] == "reopen_fresh_attempt_stale_artifact" && a["acted"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an acted reopen_fresh_attempt_stale_artifact action; got %#v", summary["actions"])
	}
	events := scalarInt(t, ctx, runner, `
		SELECT count(*) FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3
		   AND event_type = 'recovery.reopened_fresh_attempt'`, ids.repoID, ids.runID, ids.jobID)
	if events != 1 {
		t.Fatalf("recovery.reopened_fresh_attempt events = %d, want 1", events)
	}

	// The prior attempt's append-only artifact row is RETAINED (provenance intact).
	rows := scalarInt(t, ctx, runner, `
		SELECT count(*) FROM striatumd.artifacts
		 WHERE repository_id = $1 AND artifact_id = $2`, ids.repoID, "art_stale_317")
	if rows != 1 {
		t.Fatalf("prior artifact row count = %d, want 1 (append-only row must be retained)", rows)
	}
}
