package mutations

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

// TestReconcileProbesLivenessOutsideTransaction is the #355 regression — an
// uncovered isomorph of the #198 convoy (TestSweepProbesLivenessOutsideTransaction).
//
// HandleRecoveryProcessReconcile opens withTx, takes lockRun, and used to call
// the tmux/proc liveness probe (ProbeLaneLiveness — wrapped as a `sudo -n -u …
// tmux list-panes` subprocess under STRIATUM_LANE_OS_USER) for every running
// process row INSIDE that transaction, while also appending `process.lost`. The
// process.lost append takes the per-repo `repo_event_chain_heads FOR UPDATE`
// (the global event-append serialization point); holding it across a slow
// subprocess starved every other append on the repo — including
// HandleRunPrepare's `run.created` — until statement_timeout fired as
// `append_event_row (sd): 57014`. The #198 fix only hoisted HandleRecoveryAuto's
// probe out of its tx; this path was missed.
//
// This test seeds a running process_executions row whose owning lane is reported
// DEAD by the probe seam, and a supervisor pointer so the reconcile loop reaches
// the probe. The probe seam records, for every call, whether the context is
// marked as executing inside the sweep/reconcile transaction (inSweepTx — set
// right after withTx opens, after lockRun). Before the fix the probe ran with
// inSweepTx=true; after the pre-tx hoist the snapshot pass runs before withTx
// (inSweepTx=false) and the in-tx loop reads the injected oracle without probing.
// Asserting every probe call had inSweepTx=false proves NO liveness probe ran
// while the reconcile transaction held its locks — while the process is still
// transitioned to lost, proving the decision logic is preserved.
func TestReconcileProbesLivenessOutsideTransaction(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_reconcile_probe_outside_tx"

	runID, jobID, leaseID, msgID, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)

	now := time.Now().UTC()
	// The process_executions row FK-references a work_packets row, which in turn
	// references the run/job/message/lease/session the seed helper created.
	packetID := "pkt_reconcile_" + repoID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.work_packets (
		  repository_id, packet_id, run_id, job_id, message_id, lease_id, session_id,
		  packet_json, packet_sha256, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'{}'::jsonb,'sha',$8)`,
		repoID, packetID, runID, jobID, msgID, leaseID, sessionID, now); err != nil {
		t.Fatalf("seed work_packet: %v", err)
	}
	// A running process_executions row owned by the (stalled) session/lease, with
	// a PID the probe will report dead. plain-PTY lane (empty metadata): the
	// reconcile loop falls back to pidAlive for non-tmux, so we must report a PID
	// that is NOT alive — use a PID that does not exist on this host.
	deadPID := 2147480000
	procID := "proc_reconcile_" + repoID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_executions (
		  repository_id, process_id, run_id, job_id, session_id, lease_id,
		  packet_id, adapter, command_json, cwd, scratch_path, stdin_mode,
		  stdio_mode, state, started_at, pid
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'pty','["sh"]'::jsonb,'/tmp','/tmp/scratch','none','suppressed','running',$8,$9)`,
		repoID, procID, runID, jobID, sessionID, leaseID, packetID, now, deadPID); err != nil {
		t.Fatalf("seed running process_executions: %v", err)
	}

	// A supervisor pointer for the session so the reconcile JOIN resolves the
	// supervisor row (and so buildReconcileLivenessOracle / the in-tx loop key on
	// the SAME (metadata, pid, start) triple — #355 Caveat B). pid 0 on the pointer
	// so probePID falls back to pe.pid (the dead PID), mirroring the loop.
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,0,'','attached',$6,'{}'::jsonb)`,
		repoID, supID, "dsup_"+sessionID, runID, sessionID, now); err != nil {
		t.Fatalf("seed supervisor pointer: %v", err)
	}

	type probeCall struct {
		inTx          bool
		pid           int
		expectedStart string
	}
	var (
		mu    sync.Mutex
		calls []probeCall
	)
	restore := probeLaneLiveness
	probeLaneLiveness = func(pctx context.Context, metadata map[string]any, pid int, expectedStart string) gosupervisor.LaneLiveness {
		mu.Lock()
		calls = append(calls, probeCall{inTx: inSweepTx(pctx), pid: pid, expectedStart: expectedStart})
		mu.Unlock()
		// plain-PTY lane: report not-tmux so the loop falls back to pidAlive(pid),
		// which will be false for the non-existent PID — confirming the process lost.
		return gosupervisor.LaneLiveness{Backed: "plain_pty", Alive: false, Class: "pid_gone", ObservedPID: pid}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })

	result, err := HandleRecoveryProcessReconcile(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "reconcile_" + runID,
		Method:        "recovery.process_reconcile",
		Params: map[string]any{
			"repository_id": repoID,
			"run_id":        runID,
		},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Behavior preserved: the dead process was transitioned to lost.
	transitioned, _ := result["transitioned_to_lost"].([]map[string]any)
	if len(transitioned) != 1 {
		t.Fatalf("transitioned_to_lost = %v, want 1 (the dead process must be lost); %#v", len(transitioned), result)
	}
	if got := processExecutionState(t, ctx, runner, repoID, procID); got != "lost" {
		t.Fatalf("process_executions state = %q, want lost", got)
	}

	// Lock-window proof (#355): the probe ran, and EVERY probe call happened
	// OUTSIDE the reconcile transaction (the pre-tx snapshot pass), never while it
	// held lockRun + the event-append chain-head lock.
	mu.Lock()
	defer mu.Unlock()
	if len(calls) == 0 {
		t.Fatalf("expected at least one liveness probe; got none (probe seam not exercised — Caveat A regressed if the in-tx loop calls ProbeLaneLiveness directly)")
	}
	for i, c := range calls {
		if c.inTx {
			t.Fatalf("liveness probe #%d ran INSIDE the reconcile transaction (#355 regression): the tmux/proc subprocess must be pre-computed outside the chain-head-lock window", i)
		}
	}
}

func processExecutionState(t *testing.T, ctx context.Context, runner any, repoID, processID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT state FROM striatumd.process_executions
		 WHERE repository_id = $1 AND process_id = $2`, repoID, processID)
	if err != nil {
		t.Fatalf("read process_executions state: %v", err)
	}
	return fmt.Sprint(row["state"])
}
