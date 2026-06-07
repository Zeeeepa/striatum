package mutations

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestSweepDrainsHelperEventsOutsideTransaction is the #198 regression for the
// helper-event drain.
//
// drainRunHelperEvents used to run INSIDE HandleRecoveryAuto's per-run
// advisory-lock transaction. Per supervisor it takes `FOR UPDATE OF ps` plus
// three heartbeat UPDATEs (the `process_supervisor_pointers SET updated_at`
// holder observed open for 2m36s in #198), so under N supervisors the supervisor
// row locks stacked behind the advisory lock for the whole sweep and starved
// every other write (SQLSTATE 57014).
//
// The drain now runs before the main sweep transaction, per supervisor, in its
// own short transaction. This wraps the recoveryDrainHelperEvents seam to record,
// for each invocation, whether the context is marked as inside the sweep
// transaction (inSweepTx). Asserting every drain ran with inSweepTx=false proves
// the per-supervisor `FOR UPDATE OF ps` + heartbeat writes never execute while
// the sweep's per-run advisory lock is held.
func TestSweepDrainsHelperEventsOutsideTransaction(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_drain_outside_tx"
	runID := "run_" + repoID
	sessionID := "sess_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "implement", "type": "build", "role_id": "implementer"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "implementer", "claude", []string{"write"}, "active")

	now := time.Now().UTC()
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,4242,'','attached',$6,'{}'::jsonb)`,
		repoID, supID, "dsup_"+sessionID, runID, sessionID, now); err != nil {
		t.Fatalf("seed supervisor pointer: %v", err)
	}

	type drainCall struct {
		supervisorID string
		inTx         bool
	}
	var (
		mu    sync.Mutex
		calls []drainCall
	)
	restore := recoveryDrainHelperEvents
	recoveryDrainHelperEvents = func(dctx context.Context, _ db.TxRunner, _ string, supervisorID string, _ time.Duration) error {
		mu.Lock()
		calls = append(calls, drainCall{supervisorID: supervisorID, inTx: inSweepTx(dctx)})
		mu.Unlock()
		return nil
	}
	t.Cleanup(func() { recoveryDrainHelperEvents = restore })

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) == 0 {
		t.Fatalf("expected the active supervisor to be drained; got no drain calls")
	}
	for i, c := range calls {
		if c.inTx {
			t.Fatalf("helper-event drain #%d (supervisor %s) ran INSIDE the sweep transaction (#198 regression): the per-supervisor FOR UPDATE OF ps + heartbeat writes must drain outside the advisory-lock window", i, c.supervisorID)
		}
	}
}
