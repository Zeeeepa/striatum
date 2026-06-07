package mutations

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
	"github.com/jackc/pgx/v5"
)

// TestSweepProbesLivenessOutsideTransaction is the #198 regression.
//
// The masked-dead-lane probe (supervisedAgentConfirmedDead -> ProbeLaneLiveness)
// shells out to `tmux list-panes` / reads /proc. Before the fix it ran from
// INSIDE the recovery-sweep transaction that already holds the per-run advisory
// lock and FOR UPDATE row locks, so under multi-run load the subprocess calls
// stacked inside the lock window into the minutes-long convoy that starved every
// other write (global SQLSTATE 57014).
//
// This test seeds a job whose owning session still looks alive by lease/
// heartbeat (the `default` decision-tree branch, where the masked-dead probe is
// the ONLY signal that can act) and whose supervised agent is reported dead by
// the probe. It overrides the package probe seam with a recorder that captures,
// for every probe call, whether the liveness oracle is already present in the
// context.
//
// The probe seam records, for every call, whether the context is marked as
// executing inside the sweep transaction (inSweepTx — set right after withTx
// opens, after lockRun and the FOR UPDATE row locks). Before the fix the probe
// ran from inside that transaction (inSweepTx=true); after it, the snapshot pass
// runs before withTx (inSweepTx=false) and the in-tx code reads the injected
// oracle without probing. Asserting every probe call had inSweepTx=false proves
// NO liveness probe ran while the sweep transaction held its locks — while the
// requeue still fires, proving the decision logic is preserved.
func TestSweepProbesLivenessOutsideTransaction(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_probe_outside_tx"

	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)

	// Make the owning session look ALIVE by protocol/heartbeat so the decision
	// tree's CASE 1/CASE 2 do not fire and the masked-dead probe in the `default`
	// branch is the sole signal that can requeue this job (#147 Symptom B).
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET registered_at = $3, last_mcp_request_at = $3,
		       last_session_heartbeat_at = $3, last_work_heartbeat_at = $3,
		       last_pty_activity_at = $3
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID, now); err != nil {
		t.Fatalf("set fresh activity: %v", err)
	}

	// Record a supervisor pointer with a PID so supervisedAgentConfirmedDead
	// reaches the probe (plain PTY lane: no tmux metadata, PID-based identity).
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,4242,'','attached',$6,'{}'::jsonb)`,
		repoID, supID, "dsup_"+sessionID, runID, sessionID, now); err != nil {
		t.Fatalf("seed supervisor pointer: %v", err)
	}

	type probeCall struct{ inTx bool }
	var (
		mu    sync.Mutex
		calls []probeCall
	)
	restore := probeLaneLiveness
	probeLaneLiveness = func(pctx context.Context, metadata map[string]any, pid int, expectedStart string) gosupervisor.LaneLiveness {
		mu.Lock()
		calls = append(calls, probeCall{inTx: inSweepTx(pctx)})
		mu.Unlock()
		// Report the supervised agent as positively dead (plain PTY lane).
		return gosupervisor.LaneLiveness{Backed: "plain_pty", Alive: false, Class: "pid_gone", ObservedPID: pid}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// Behavior preserved: the masked-dead lane was requeued on the same attempt.
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 1 {
		t.Fatalf("acted_count = %v, want 1 (masked-dead lane must be requeued); %#v", summary["acted_count"], summary)
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued (requeued)", got)
	}

	// Lock-window proof: the probe ran, and every probe call happened OUTSIDE the
	// sweep transaction (the pre-tx snapshot pass), never while it held its locks.
	mu.Lock()
	defer mu.Unlock()
	if len(calls) == 0 {
		t.Fatalf("expected at least one liveness probe; got none (probe seam not exercised)")
	}
	for i, c := range calls {
		if c.inTx {
			t.Fatalf("liveness probe #%d ran INSIDE the sweep transaction (#198 regression): probes must be pre-computed outside the lock window", i)
		}
	}
}

// TestBuildRunLivenessOracleDedupesAndDegrades documents two oracle invariants:
// the pre-tx snapshot probes each distinct supervised agent at most once, and a
// query failure degrades to an empty oracle (forcing the in-tx live fallback)
// rather than failing the sweep.
func TestBuildRunLivenessOracleDedupesAndDegrades(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_oracle_dedupe"
	runID := "run_" + repoID
	sessionID := "sess_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "implement", "type": "build", "role_id": "implementer"}},
	})
	sessionB := sessionID + "_b"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, sessionID, "implementer", "claude", []string{"write"}, "active", 1)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, sessionB, "implementer", "claude", []string{"write"}, "active", 2)

	now := time.Now().UTC()
	// Two pointer rows (distinct sessions, satisfying the one-active-pointer-per-
	// session constraint) that describe the SAME plain-PTY agent: same pid, same
	// empty start token. The oracle keys on agent identity, so it must probe once.
	pointers := []struct{ supID, sessionID string }{
		{"sup_a_" + repoID, sessionID},
		{"sup_b_" + repoID, sessionB},
	}
	for _, p := range pointers {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.process_supervisor_pointers (
			  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
			  pid, pid_start_time, state, updated_at, metadata_json
			) VALUES ($1,$2,$3,$4,$5,4242,'','attached',$6,'{}'::jsonb)`,
			repoID, p.supID, "d"+p.supID, runID, p.sessionID, now); err != nil {
			t.Fatalf("seed pointer %s: %v", p.supID, err)
		}
	}

	var probes int
	restore := probeLaneLiveness
	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		probes++
		return gosupervisor.LaneLiveness{Backed: "plain_pty", Alive: true}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	oracle := buildRunLivenessOracle(ctx, runner, repoID, runID)
	if probes != 1 {
		t.Fatalf("probe count = %d, want 1 (the same agent across two pointers must be probed once)", probes)
	}
	if _, ok := oracle.lookup(map[string]any{}, 4242, ""); !ok {
		t.Fatalf("oracle should resolve the probed agent")
	}

	// A nil-runner snapshot degrades to an empty oracle (no panic, no error path).
	probes = 0
	empty := buildRunLivenessOracle(ctx, degradedRunner{}, repoID, runID)
	if _, ok := empty.lookup(map[string]any{}, 4242, ""); ok {
		t.Fatalf("degraded oracle must be empty (forcing in-tx live fallback)")
	}
}

var errQueryFailed = errors.New("query failed")

// degradedRunner is a db.Runner whose multi-row Query fails, exercising the
// build-oracle degrade-to-empty path (a pre-tx snapshot failure must not fail
// the sweep — it falls back to in-tx live probes).
type degradedRunner struct{}

func (degradedRunner) Exec(context.Context, string, ...any) error { return errQueryFailed }
func (degradedRunner) QueryRow(context.Context, string, ...any) db.Row {
	return nil
}
func (degradedRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errQueryFailed
}
func (degradedRunner) BeginTx(context.Context) (db.TxRunner, error) { return nil, errQueryFailed }
func (degradedRunner) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errQueryFailed
}

// TestConfirmedDeadTreatsIdentityUnavailableAsUnknown guards the review finding
// on the #198 oracle hoist: PIDLivenessIdentityUnavailable means kill(pid,0)
// SUCCEEDED (the agent is alive/signalable) but /proc/<pid>/stat was
// momentarily unreadable. Treating that as confirmed-dead force-expires a LIVE
// agent's lease (the #145/#147 false-requeue class) — and with the pre-tx
// oracle the transient failure would be cached for the whole sweep window. A
// definitive not-alive verdict (PIDLivenessGone) must still confirm death.
func TestConfirmedDeadTreatsIdentityUnavailableAsUnknown(t *testing.T) {
	prev := probeLaneLiveness
	defer func() { probeLaneLiveness = prev }()

	row := map[string]any{
		"supervisor_pointer_state":          "attached",
		"supervisor_pointer_pid":            4242,
		"supervisor_pointer_pid_start_time": "1234567",
	}

	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		return gosupervisor.LaneLiveness{Alive: false, Class: gosupervisor.PIDLivenessIdentityUnavailable}
	}
	if supervisedAgentConfirmedDead(context.Background(), row) {
		t.Fatal("pid_identity_unavailable (signalable but /proc unreadable) must NOT confirm death — false requeue of a live agent")
	}

	probeLaneLiveness = func(context.Context, map[string]any, int, string) gosupervisor.LaneLiveness {
		return gosupervisor.LaneLiveness{Alive: false, Class: gosupervisor.PIDLivenessGone}
	}
	if !supervisedAgentConfirmedDead(context.Background(), row) {
		t.Fatal("a definitively gone agent must still confirm death")
	}
}
