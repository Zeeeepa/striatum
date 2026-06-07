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
