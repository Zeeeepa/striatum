package mutations

import (
	"context"
	"fmt"
	"os"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// RFC 0135 P1/P2 (#354, D246/D269) — the fan-in barrier is live by default for
// confirmed fan-in runs. Completion stages each declared sibling and pins its exact
// stack; the downstream gate assembles the ready barrier before the join job queues.
// STRIATUM_BARRIER_FANIN=0 is the recoverable kill switch back to the shipped D206
// per-completion merge (fanInIntegrateRunBranch in worktree.go).

// barrierFaninAssemblyEnabled reports whether the RFC 0135 P1/P2 fan-in
// staging-at-completion + assembly path is live. It is ON by default; setting
// STRIATUM_BARRIER_FANIN=0 restores the legacy D206 per-completion merge path.
func barrierFaninAssemblyEnabled() bool {
	return os.Getenv("STRIATUM_BARRIER_FANIN") != "0"
}

// stageFaninContributionAtCompletion is the staging-at-completion hook (RFC 0135
// P1). When the fan-in fold is enabled AND the completing seat is a declared
// in-edge of a recorded fan-in freeze point in its run, it stages the seat's
// completion as an attempt-addressed contribution (stageFaninContribution), so the
// barrier can later fire and assemble. The live completion path calls this before
// the legacy merge path, so declared fan-in siblings stage-and-pin instead of
// per-completion merging. With STRIATUM_BARRIER_FANIN=0, or when the seat belongs to
// no recorded fan-in freeze point, it returns ("", nil) without touching git or PG.
//
// commitSHA is the seat's completed worktree HEAD (the same head the per-completion
// merge folds). attempt is the seat's live attempt (the seal). It returns the
// staging ref written (empty when it no-ops).
func stageFaninContributionAtCompletion(ctx context.Context, runner db.TxRunner, repoRoot, repositoryID, runID, workflowJobID, jobID, commitSHA string, attempt int) (string, error) {
	if !barrierFaninAssemblyEnabled() {
		return "", nil
	}
	barrierID, found, err := faninBarrierForSeat(ctx, runner, repositoryID, runID, workflowJobID)
	if err != nil {
		return "", err
	}
	if !found {
		// The seat is not a declared fan-in in-edge: no freeze point waits on it. The
		// default per-completion path is the only thing that runs (this hook no-ops).
		return "", nil
	}
	return stageFaninContribution(ctx, runner, repoRoot, repositoryID, barrierID, runID, workflowJobID, jobID, commitSHA, attempt)
}

// faninBarrierForSeat resolves the fan-in barrier (freeze record) a completing seat
// is a DECLARED in-edge of, scoped to the seat's run. A seat appears in at most one
// fan-in barrier per run (the freeze record declares its sibling set once at
// fan-out). It returns ("", false, nil) when the seat is not a declared fan-in
// in-edge.
func faninBarrierForSeat(ctx context.Context, runner db.TxRunner, repositoryID, runID, workflowJobID string) (string, bool, error) {
	// Match the seat against each freeze record's declared sibling set via
	// jsonb_array_elements_text — the same expansion the barrier evaluator
	// (barrier_fanin.go) and the doctor use, rather than the jsonb `?` containment
	// operator (which collides with the pgx parameter placeholder).
	rows, err := queryRows(ctx, runner, `
		SELECT fp.barrier_id
		  FROM striatumd.fanin_freeze_points fp
		 WHERE fp.repository_id = $1 AND fp.run_id = $2
		   AND EXISTS (
		     SELECT 1
		       FROM jsonb_array_elements_text(fp.declared_sibling_job_ids) AS sib(value)
		      WHERE sib.value = $3
		   )
		 ORDER BY fp.barrier_id
		 LIMIT 1`,
		repositoryID, runID, workflowJobID)
	if err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return fmt.Sprint(rows[0]["barrier_id"]), true, nil
}

// DispatchBarrierAssembly is the barrier_assembly job DISPATCHER (RFC 0135 P2): the
// compatibility entry point that runs the recoverable, two-phase-journaled assembly
// (runBarrierAssembly) for a fired fan-in barrier, advancing the run branch to the
// deterministic assembled commit. The default scheduler path assembles inline at
// the downstream fan-in gate; this dispatcher remains the explicit assembly surface
// for deployments that drive a barrier_assembly job.
//
// CUTOVER DISCIPLINE. It refuses when the fan-in fold is killed
// (STRIATUM_BARRIER_FANIN=0) or owner bundle 0013 has not widened the jobs job_type
// CHECK to permit 'barrier_assembly' (jobBarrierAssemblyTypePermitted). With the
// kill switch on it returns a clear invalid_transition rather than silently advancing
// the run branch by a disabled path.
//
// It runs inside one transaction holding the per-run advisory lock (RFC 0104 fire
// serialization), exactly as runBarrierAssembly requires. runRef is the run-branch
// ref the assembly CAS-advances.
func DispatchBarrierAssembly(ctx context.Context, runner db.Runner, repositoryID, runID, barrierID, runRef, assemblyWorkflowJobID, assemblyJobID string) (barrierAssemblyResult, error) {
	if !barrierFaninAssemblyEnabled() {
		return barrierAssemblyResult{}, rpc.NewError("invalid_transition",
			"barrier_assembly dispatch is disabled (STRIATUM_BARRIER_FANIN=0); the shipped per-completion run-branch merge (D206) is active",
			map[string]any{"barrier_id": barrierID, "run_id": runID})
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return barrierAssemblyResult{}, err
	}
	var out barrierAssemblyResult
	_, err = withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// Owner bundle 0013 must permit the barrier_assembly job_type, or the job that
		// drives this dispatch could not have been persisted; probe defensively so a
		// behind-deployment refuses loudly rather than advancing the run branch off a
		// path the deployment cannot record.
		permitted, perr := jobBarrierAssemblyTypePermitted(ctx, tx)
		if perr != nil {
			return nil, perr
		}
		if !permitted {
			return nil, rpc.NewError("invalid_transition",
				"barrier_assembly job_type is not permitted by the live jobs CHECK; apply owner bundle 0013 before dispatching the assembly",
				map[string]any{"barrier_id": barrierID, "run_id": runID})
		}
		// Serialize fire per run (RFC 0104): the assembly CAS-advances the run branch,
		// so it must not interleave with another fire/integration on the same run.
		if lerr := lockRun(ctx, tx, repositoryID, runID); lerr != nil {
			return nil, lerr
		}
		// The barrier must actually be ready (every declared in-edge live-staged or a
		// terminal gap) before we assemble — never assemble a partial join.
		ready, rerr := faninBarrierReady(ctx, tx, repositoryID, barrierID)
		if rerr != nil {
			return nil, rerr
		}
		if !ready {
			return nil, rpc.NewError("invalid_transition",
				"fan-in barrier is not ready: a declared in-edge is not live-staged and is not a terminal gap; do not assemble a partial join",
				map[string]any{"barrier_id": barrierID, "run_id": runID})
		}
		res, aerr := runBarrierAssembly(ctx, tx, repoRoot, runRef, repositoryID, barrierID, assemblyWorkflowJobID, assemblyJobID)
		if aerr != nil {
			return nil, aerr
		}
		out = res
		return map[string]any{"status": "assembled"}, nil
	})
	if err != nil {
		return barrierAssemblyResult{}, err
	}
	return out, nil
}
