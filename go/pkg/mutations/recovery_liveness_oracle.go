package mutations

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

// recovery_liveness_oracle.go (#198) moves the process/filesystem/tmux liveness
// probe OUT of the lock-holding recovery-sweep transaction.
//
// Why this matters: ProbeLaneLiveness shells out to `tmux list-panes -a` (an
// external subprocess) or reads /proc, which under multi-run load takes
// hundreds of milliseconds to seconds per call. The 60s daemon sweep
// (HandleRecoveryAuto) used to call it once per stuck job from inside the
// transaction that already holds the per-run advisory lock (lockRun) and
// FOR UPDATE row locks on jobs/sessions. Across N supervised lanes × M runs the
// subprocess calls serialized inside the lock window and stacked into the
// minutes-long convoy in #198, behind which every append_event_row / session
// UPDATE / claim queued until the statement timeout (global SQLSTATE 57014).
//
// The probe is a PURE READ of OS/tmux state: its result only GATES whether the
// subsequent in-transaction writes happen, and the value does not depend on
// holding any DB lock. So pre-probing every candidate supervisor BEFORE opening
// the transaction and injecting the results via context preserves the decision
// logic exactly while removing all subprocess/filesystem IO from the lock
// window. The DB writes that act on the decision stay inside the transaction and
// keep their atomicity.
//
// Fallback: when no oracle is present in the context (the operator-invoked
// recovery RPCs, and every direct unit test of the decision tree), the probe is
// performed live exactly as before. So this is a pure latency/lock-footprint
// optimization for the periodic sweep with no behavior change anywhere else.

// probeLaneLiveness is the indirection through which both the pre-tx snapshot
// pass and the live fallback reach the OS/tmux probe. It is a package var so a
// regression test can wrap it to count invocations and assert none occur while a
// sweep transaction is open (#198). Production always uses ProbeLaneLiveness.
var probeLaneLiveness = func(ctx context.Context, metadata map[string]any, pid int, expectedStart string) gosupervisor.LaneLiveness {
	return gosupervisor.ProbeLaneLiveness(ctx, tmuxRunnerForSupervisorMetadata(metadata), metadata, pid, expectedStart)
}

// inSweepTxKey marks a context that is executing INSIDE the recovery-sweep
// transaction (after lockRun + the FOR UPDATE row locks). The #198 regression
// reads it through the probe seam to prove no liveness probe runs while the
// sweep transaction holds its locks; production code never branches on it.
type inSweepTxKey struct{}

func withinSweepTx(ctx context.Context) context.Context {
	return context.WithValue(ctx, inSweepTxKey{}, true)
}

// InSweepTx reports whether ctx is inside the recovery-sweep transaction. It is
// exported-to-package for the #198 regression's probe recorder.
func inSweepTx(ctx context.Context) bool {
	v, _ := ctx.Value(inSweepTxKey{}).(bool)
	return v
}

type livenessOracleKey struct{}

// livenessOracle resolves a supervisor pointer's liveness from a snapshot taken
// before the transaction opened. found=false means the snapshot has no entry
// (caller falls back to a live probe).
type livenessOracle struct {
	byKey map[string]gosupervisor.LaneLiveness
}

// supervisorLivenessProbeKey is the stable identity of a supervised agent for
// liveness caching: the recorded PID + its start token plus the tmux pane/session
// identity when the lane is tmux-backed. Two rows that resolve to the same agent
// share a key, so the agent is probed at most once per sweep.
func supervisorLivenessProbeKey(metadata map[string]any, pid int, expectedStart string) string {
	tmuxKey := ""
	if id, ok := gosupervisor.TmuxIdentityFromMetadata(metadata); ok {
		tmuxKey = fmt.Sprintf("tmux:%s/%s/%d/%s", id.SessionName, id.PaneID, id.PanePID, id.PaneStartToken)
	}
	return fmt.Sprintf("%s|pid:%d|start:%s", tmuxKey, pid, expectedStart)
}

func (o *livenessOracle) lookup(metadata map[string]any, pid int, expectedStart string) (gosupervisor.LaneLiveness, bool) {
	if o == nil || o.byKey == nil {
		return gosupervisor.LaneLiveness{}, false
	}
	live, ok := o.byKey[supervisorLivenessProbeKey(metadata, pid, expectedStart)]
	return live, ok
}

func withLivenessOracle(ctx context.Context, oracle *livenessOracle) context.Context {
	return context.WithValue(ctx, livenessOracleKey{}, oracle)
}

func livenessOracleFromContext(ctx context.Context) *livenessOracle {
	oracle, _ := ctx.Value(livenessOracleKey{}).(*livenessOracle)
	return oracle
}

// probeLaneLivenessCached returns the pre-probed liveness for the supervised
// agent when an oracle is present in ctx (the periodic sweep path), otherwise it
// performs the live probe — preserving the operator-RPC and unit-test behavior.
func probeLaneLivenessCached(ctx context.Context, metadata map[string]any, pid int, expectedStart string) gosupervisor.LaneLiveness {
	if live, ok := livenessOracleFromContext(ctx).lookup(metadata, pid, expectedStart); ok {
		return live
	}
	return probeLaneLiveness(ctx, metadata, pid, expectedStart)
}

// buildRunLivenessOracle pre-probes the liveness of every recorded supervisor
// pointer for the run OUTSIDE any transaction and returns an oracle keyed by
// supervisor agent identity. It is read-only (a plain SELECT with no FOR UPDATE)
// so it never serializes against concurrent writers, and the (possibly slow)
// tmux/proc probes run with no DB lock held. A query error degrades to an empty
// oracle so the sweep transaction simply falls back to live probes — never
// failing the sweep because the pre-probe could not run.
func buildRunLivenessOracle(ctx context.Context, runner db.Runner, repositoryID, runID string) *livenessOracle {
	oracle := &livenessOracle{byKey: map[string]gosupervisor.LaneLiveness{}}
	rows, err := queryRows(ctx, runner, `
		SELECT pid, pid_start_time, metadata_json
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND run_id = $2`,
		repositoryID, runID)
	if err != nil {
		return oracle
	}
	for _, row := range rows {
		pid := intValue(row["pid"])
		if pid <= 0 {
			continue
		}
		metadata := asMap(row["metadata_json"])
		expectedStart, _ := row["pid_start_time"].(string)
		key := supervisorLivenessProbeKey(metadata, pid, expectedStart)
		if _, seen := oracle.byKey[key]; seen {
			continue
		}
		oracle.byKey[key] = probeLaneLiveness(ctx, metadata, pid, expectedStart)
	}
	return oracle
}

// buildReconcileLivenessOracle is the #355 pre-tx oracle for
// HandleRecoveryProcessReconcile. It is an isomorph of buildRunLivenessOracle —
// the #198 hoist that HandleRecoveryProcessReconcile never received — but it
// MUST mirror the reconcile loop's own row shape exactly or the cache misses and
// silently falls back to the in-tx probe, re-creating the convoy (#355 Caveat
// B). Specifically the reconcile loop (recovery.go) reads each running
// process_executions row JOINed to its latest process_supervisor_pointers row
// and:
//
//   - probePID prefers ptr.pid (supervisor_pid) over pe.pid when > 0,
//   - expectedStart is ptr.pid_start_time (supervisor_pid_start_time),
//   - metadata is ptr.metadata_json (supervisor_metadata_json).
//
// supervisorLivenessProbeKey is computed from (metadata, probePID, expectedStart),
// so the oracle key MUST be built from the SAME JOINed selection. This query is
// the SELECT shape from the reconcile loop with `FOR UPDATE OF pe` stripped (a
// pure read), so the slow tmux/proc probe runs with no DB lock held and the
// cache key matches the in-tx lookup byte-for-byte. A query error degrades to an
// empty oracle (in-tx live fallback) rather than failing the reconcile.
func buildReconcileLivenessOracle(ctx context.Context, runner db.Runner, repositoryID, runID string) *livenessOracle {
	oracle := &livenessOracle{byKey: map[string]gosupervisor.LaneLiveness{}}
	rows, err := queryRows(ctx, runner, `
		SELECT pe.pid,
		       p.metadata_json AS supervisor_metadata_json,
		       p.pid_start_time AS supervisor_pid_start_time,
		       p.pid AS supervisor_pid
		  FROM striatumd.process_executions pe
		  LEFT JOIN LATERAL (
		    SELECT ptr.metadata_json, ptr.pid_start_time, ptr.pid
		      FROM striatumd.process_supervisor_pointers ptr
		     WHERE ptr.repository_id = pe.repository_id
		       AND ptr.run_id = pe.run_id
		       AND ptr.session_id = pe.session_id
		     ORDER BY ptr.updated_at DESC, ptr.supervisor_id DESC
		     LIMIT 1
		  ) p ON true
		 WHERE pe.repository_id = $1
		   AND pe.run_id = $2
		   AND pe.state = 'running'`,
		repositoryID, runID)
	if err != nil {
		return oracle
	}
	for _, row := range rows {
		metadata, probePID, expectedStart := reconcileProbeSelection(row)
		if probePID <= 0 {
			continue
		}
		key := supervisorLivenessProbeKey(metadata, probePID, expectedStart)
		if _, seen := oracle.byKey[key]; seen {
			continue
		}
		oracle.byKey[key] = probeLaneLiveness(ctx, metadata, probePID, expectedStart)
	}
	return oracle
}

// reconcileProbeSelection extracts the (metadata, probePID, expectedStart) triple
// that the reconcile loop probes from a row, in the SAME way the in-tx loop does
// (#355 Caveat B). It is shared between the pre-tx oracle builder and the in-tx
// loop so the cache key cannot drift between the two call sites.
func reconcileProbeSelection(row map[string]any) (map[string]any, int, string) {
	metadata := asMap(row["supervisor_metadata_json"])
	probePID := intValue(row["pid"])
	if supervisorPID := intValue(row["supervisor_pid"]); supervisorPID > 0 {
		probePID = supervisorPID
	}
	expectedStart, _ := row["supervisor_pid_start_time"].(string)
	return metadata, probePID, expectedStart
}
