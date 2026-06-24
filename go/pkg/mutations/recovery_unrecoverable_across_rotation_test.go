package mutations

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

// TestReservedExitCodeStaysInLockstepWithAgentloop pins the daemon-side mirror of
// the reserved RFC 0143 Slice A floor exit code (97) to the agentloop authority, so
// the two cannot silently drift (the daemon reads the wrapper's exit code).
func TestReservedExitCodeStaysInLockstepWithAgentloop(t *testing.T) {
	if ExitUnrecoverableAcrossRotation != agentloop.ExitUnrecoverableAcrossRotation {
		t.Fatalf("mutations.ExitUnrecoverableAcrossRotation=%d != agentloop.ExitUnrecoverableAcrossRotation=%d",
			ExitUnrecoverableAcrossRotation, agentloop.ExitUnrecoverableAcrossRotation)
	}
	if ExitUnrecoverableAcrossRotation != 97 {
		t.Fatalf("reserved floor exit code = %d, want 97", ExitUnrecoverableAcrossRotation)
	}
}

// TestRecoverySweepClassifiesDaemonObservedStaleEpochAsUnrecoverableAcrossRotation is
// acceptance criterion (a): a confirmed-dead unsealed lane WITH a LIVE
// daemon.stale_epoch_rotation observation for its session classifies the typed
// session_unrecoverable_across_rotation floor (not the generic agent_exited_unsealed).
func TestRecoverySweepClassifiesDaemonObservedStaleEpochAsUnrecoverableAcrossRotation(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_rot_floor_fires"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoID, runID, sessionID, true)
	stubDeadProbe(t)

	// T1: the daemon observed (and recorded) a stale-epoch rejection for this session.
	if err := RecordStaleEpochRejection(ctx, runner, repoID, runID, sessionID); err != nil {
		t.Fatalf("record stale-epoch observation: %v", err)
	}

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassSessionUnrecoverableAcrossRotation {
		t.Fatalf("last_stall_class = %q, want %q (T1 fires the floor)", got, stallClassSessionUnrecoverableAcrossRotation)
	}
}

// TestRecoverySweepClassifiesDirectExit97AsUnrecoverableAcrossRotation is the
// T2-direct carrier (acceptance (a)/(e) trusted-direct half): the wrapper's OWN
// supervisor.agent_exited exit_code==97 fires the floor even without a T1 observation.
func TestRecoverySweepClassifiesDirectExit97AsUnrecoverableAcrossRotation(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_rot_floor_direct97"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoID, runID, sessionID, true)
	stubDeadProbe(t)

	// The wrapper's own reap recorded exit 97 on the owning session.
	if _, err := appendEvent(ctx, runner, repoID, runID, "supervisor.agent_exited", sessionID, jobID, nil, nil, nil,
		map[string]any{"exit_code": agentloop.ExitUnrecoverableAcrossRotation}); err != nil {
		t.Fatalf("record agent_exited exit 97: %v", err)
	}

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassSessionUnrecoverableAcrossRotation {
		t.Fatalf("last_stall_class = %q, want %q (T2-direct exit 97 fires the floor)", got, stallClassSessionUnrecoverableAcrossRotation)
	}
}

// TestOrdinaryUnsealedExitWithNoObservationStaysAgentExitedUnsealed is acceptance
// criterion (c) baseline: an ordinary unsealed dead lane with NO observation and NO
// direct exit 97 stays agent_exited_unsealed — the floor does not over-fire.
func TestOrdinaryUnsealedExitWithNoObservationStaysAgentExitedUnsealed(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_rot_floor_baseline"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoID, runID, sessionID, true)
	stubDeadProbe(t)

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassAgentExitedUnsealed {
		t.Fatalf("last_stall_class = %q, want %q (no observation -> generic class)", got, stallClassAgentExitedUnsealed)
	}
}

// TestRecoveredSessionClearsStaleEpochObservationThenOrdinaryDeathStaysUnsealed is
// the BINDING Correction 1 (acceptance (b)): a session that recorded a stale-epoch
// observation, then RECONNECTED (superseding it), then dies ORDINARILY must classify
// agent_exited_unsealed — NOT the typed floor. The supersede is what prevents the
// over-fire the v2 falsifier_1 established.
func TestRecoveredSessionClearsStaleEpochObservationThenOrdinaryDeathStaysUnsealed(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_rot_floor_recovered"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoID, runID, sessionID, true)
	stubDeadProbe(t)

	// 1. The daemon observed a stale-epoch rejection for this session.
	if err := RecordStaleEpochRejection(ctx, runner, repoID, runID, sessionID); err != nil {
		t.Fatalf("record stale-epoch observation: %v", err)
	}
	// Sanity: it is LIVE before the recovery.
	if live, err := hasLiveStaleEpochObservation(ctx, runner, repoID, sessionID); err != nil || !live {
		t.Fatalf("expected a LIVE observation before recovery; live=%v err=%v", live, err)
	}
	// 2. The session reconnected across the rotation (presented the current epoch),
	//    superseding the observation (Correction 1).
	if err := RecordStaleEpochRecovered(ctx, runner, repoID, runID, sessionID); err != nil {
		t.Fatalf("record stale-epoch recovery: %v", err)
	}
	if live, err := hasLiveStaleEpochObservation(ctx, runner, repoID, sessionID); err != nil || live {
		t.Fatalf("observation must NOT be live after recovery; live=%v err=%v", live, err)
	}

	// 3. The session later dies ORDINARILY (unsealed) — it must classify generically.
	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassAgentExitedUnsealed {
		t.Fatalf("last_stall_class = %q, want %q (Correction 1: a RECOVERED session's later ordinary death is NOT the floor)", got, stallClassAgentExitedUnsealed)
	}
}

// TestTmuxPaneDeadStatus97WithoutDaemonObservationStaysUnsealed is the A5 / GD-2
// negative (Correction 2): a same-uid `respawn-pane … exit 97` drives only the
// forgeable #{pane_dead_status}==97 — with NO T1 observation and NO direct-path
// exit 97 — and must NOT record the typed class. It stays agent_exited_unsealed.
func TestTmuxPaneDeadStatus97WithoutDaemonObservationStaysUnsealed(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_rot_floor_tmux_forge"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoID, runID, sessionID, true)

	// The pane is dead with #{pane_dead_status}==97 — the forgeable tmux carrier — but
	// there is NO daemon observation and NO wrapper exit 97. Stub the probe to return
	// a dead tmux pane carrying PaneDeadStatus=97.
	forged := agentloop.ExitUnrecoverableAcrossRotation
	restore := probeLaneLiveness
	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		tmux := gosupervisor.TmuxLiveness{
			Class:           gosupervisor.TmuxLivenessPaneDead,
			State:           "lost",
			Healthy:         false,
			ObservedPanePID: 8888,
			PaneDeadStatus:  &forged,
		}
		return gosupervisor.LaneLiveness{Backed: "tmux", Alive: false, Class: string(gosupervisor.TmuxLivenessPaneDead), Tmux: &tmux, ObservedPID: 8888}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassAgentExitedUnsealed {
		t.Fatalf("last_stall_class = %q, want %q (Correction 2: a bare/forged tmux 97 with no T1 is NOT the floor)", got, stallClassAgentExitedUnsealed)
	}
}

// TestTypedFloorGrantsNoAutoSealAuthority is the BINDING Correction 2 observability-
// only invariant (acceptance (d)): the typed floor routes the IDENTICAL
// finalize-or-escalate path as agent_exited_unsealed and seals NOTHING on its own
// strength. With NO durable published artifact, a rotation-locked lane ESCALATES for
// an operator requeue (the job is not auto-completed) — exactly as the generic class
// would, just with a legible rotation reason. The floor never auto-seals.
func TestTypedFloorGrantsNoAutoSealAuthority(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_rot_floor_no_autoseal"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoID, runID, sessionID, true)
	stubDeadProbe(t)

	// Drive the floor (T1) AND exhaust the unsealed budget so the sweep must choose
	// the escalate path (no durable artifact was published for this fixture job).
	if err := RecordStaleEpochRejection(ctx, runner, repoID, runID, sessionID); err != nil {
		t.Fatalf("record stale-epoch observation: %v", err)
	}
	preseedRequeueBudget(t, ctx, runner, repoID, runID, jobID, 1)

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The typed class was recorded (legible reason)...
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassSessionUnrecoverableAcrossRotation {
		t.Fatalf("last_stall_class = %q, want %q", got, stallClassSessionUnrecoverableAcrossRotation)
	}
	// ...but the job was NOT auto-sealed/completed by the floor's strength — it
	// escalated for the operator requeue, identical to the agent_exited_unsealed path.
	if got := jobState(t, ctx, runner, repoID, jobID); got == "completed" {
		t.Fatalf("job state = completed: the typed floor must NOT auto-seal (observability-only); it must escalate for an operator requeue")
	}
	_, _, escalation := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if !escalation {
		t.Fatalf("escalation_pending = false: the rotation-locked lane with no durable artifact must escalate (no auto-seal)")
	}

	// The escalation reason is legibly the rotation class (acceptance (a) legibility).
	row, err := oneRow(ctx, runner, `
		SELECT payload_json::text AS payload FROM striatumd.escalation_inbox
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read escalation payload: %v", err)
	}
	payload := fmt.Sprint(row["payload"])
	if !strings.Contains(payload, stallClassSessionUnrecoverableAcrossRotation) {
		t.Fatalf("escalation payload missing the typed rotation class: %s", payload)
	}
}

// TestRotationLockedLaneWithDurableArtifactAutoFinalizesTaggedTyped is the finalize
// half of acceptance (a)/(d): a rotation-locked lane WITH an already-published,
// body-reconstructable durable artifact takes the SAME #308 auto-finalize path as
// agent_exited_unsealed (the job completes from the durable artifact — NOT a new seal
// minted by the floor), only TAGGED with the typed class. This proves the typed floor
// adds no new finalize path and is no-more-privileged (observability-only).
func TestRotationLockedLaneWithDurableArtifactAutoFinalizesTaggedTyped(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "rot_finalize", true)

	payload := []byte("durable rotation deliverable\n")
	sha := commitWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", string(payload))
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+ids.runBranch, sha)
	seedPublishedArtifact(t, ctx, runner, ids, "art_rot_finalize", "out", "docs/out.txt", payload, nil)
	seedUnsealedPublishedFinalJob308(t, ctx, runner, ids)

	// The daemon observed a stale-epoch rejection for this owning session (T1).
	if err := RecordStaleEpochRejection(ctx, runner, ids.repoID, ids.runID, ids.sessionID); err != nil {
		t.Fatalf("record stale-epoch observation: %v", err)
	}

	restore := probeLaneLiveness
	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		return gosupervisor.LaneLiveness{Backed: "tmux", Alive: false, Class: string(gosupervisor.TmuxLivenessPaneDead), ObservedPID: 8888}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	result, err := SweepRun(ctx, runner, ids.repoID, ids.runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// Same finalize path as agent_exited_unsealed: the job completes from the durable
	// artifact (NOT a floor-minted seal), and requeue_count does not climb.
	if got := jobState(t, ctx, runner, ids.repoID, ids.jobID); got != "completed" {
		t.Fatalf("job state = %q, want completed (auto-finalized from durable artifact)", got)
	}
	requeue, _, _ := jobRecoveryCounts(t, ctx, runner, ids.repoID, ids.jobID)
	if requeue != 0 {
		t.Fatalf("requeue_count = %d, want 0", requeue)
	}
	// ...but the action is TAGGED with the typed rotation class (legible reason).
	summary := recoveryActionsFromSweep(t, result)
	acts, _ := summary["actions"].([]map[string]any)
	foundTyped := false
	for _, a := range acts {
		if a["action"] == "auto_finalize_unsealed" && a["acted"] == true && a["stall_class"] == stallClassSessionUnrecoverableAcrossRotation {
			foundTyped = true
		}
	}
	if !foundTyped {
		t.Fatalf("auto_finalize_unsealed action not tagged with the typed rotation class; actions=%#v", acts)
	}
}
