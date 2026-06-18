package reads

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestStatusExcludesTerminalRunBlockersRepoWide is the #417 follow-on: a repo-wide
// status read must not surface open blockers / human_checkpoints on TERMINAL runs
// as pending operator work. They are not actionable — a canceled/completed run can
// never grow new work, so resolving the blocker changes nothing. The #193
// terminal-run scoping already excludes such runs from claimable_jobs /
// blocked_downstream_jobs; statusBlockers was missed, so a dead run's stale
// blockers (e.g. a 21-day-old revision_routing checkpoint on a canceled run)
// surfaced forever in the frontier. A run_id-scoped call is an explicit ask for one
// run and still shows its own blockers; the blocker rows are preserved as
// provenance either way.
func TestStatusExcludesTerminalRunBlockersRepoWide(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_terminal_blockers_417"
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	crSeedRepo(t, ctx, runner, repoID, now)

	liveRun := "run_live_417"
	termRun := "run_canceled_417"
	crSeedRunState(t, ctx, runner, repoID, liveRun, "striatum/live", now, "running")
	crSeedRunState(t, ctx, runner, repoID, termRun, "striatum/term", now, "canceled")

	seedBlocker := func(blockerID, runID, severity, kind string) {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.blockers
			  (repository_id, blocker_id, run_id, severity, blocker_kind, description, state, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,'open',$7)`,
			repoID, blockerID, runID, severity, kind, "desc "+blockerID, now); err != nil {
			t.Fatalf("seed blocker %s: %v", blockerID, err)
		}
	}
	seedBlocker("blk_live", liveRun, "blocked", "missing_input")
	seedBlocker("blk_term", termRun, "blocked", "missing_input")
	seedBlocker("chk_live", liveRun, "human_checkpoint", "revision_routing")
	seedBlocker("chk_term", termRun, "human_checkpoint", "revision_routing")

	ids := func(v any) map[string]bool {
		out := map[string]bool{}
		rows, _ := v.([]map[string]any)
		for _, r := range rows {
			out[fmt.Sprint(r["blocker_id"])] = true
		}
		return out
	}

	// Repo-wide: terminal-run blockers + checkpoints are excluded; live ones remain.
	repoWide, err := HandleStatus(ctx, runner, rpc.Envelope{Params: map[string]any{"repository_id": repoID}})
	if err != nil {
		t.Fatalf("HandleStatus repo-wide: %v", err)
	}
	ob := ids(repoWide["open_blockers"])
	cp := ids(repoWide["human_checkpoints"])
	if !ob["blk_live"] || !ob["chk_live"] {
		t.Fatalf("live-run blockers missing from open_blockers: %#v", repoWide["open_blockers"])
	}
	if !cp["chk_live"] {
		t.Fatalf("live-run checkpoint missing from human_checkpoints: %#v", repoWide["human_checkpoints"])
	}
	if ob["blk_term"] || ob["chk_term"] {
		t.Fatalf("terminal-run blocker surfaced in repo-wide open_blockers: %#v", repoWide["open_blockers"])
	}
	if cp["chk_term"] {
		t.Fatalf("terminal-run checkpoint surfaced in repo-wide human_checkpoints: %#v", repoWide["human_checkpoints"])
	}

	// Run-scoped to the terminal run: its own blockers ARE shown (explicit ask).
	scoped, err := HandleStatus(ctx, runner, rpc.Envelope{Params: map[string]any{"repository_id": repoID, "run_id": termRun}})
	if err != nil {
		t.Fatalf("HandleStatus run-scoped: %v", err)
	}
	if sb := ids(scoped["open_blockers"]); !sb["blk_term"] {
		t.Fatalf("run-scoped status should show the terminal run's own blocker: %#v", scoped["open_blockers"])
	}
}
