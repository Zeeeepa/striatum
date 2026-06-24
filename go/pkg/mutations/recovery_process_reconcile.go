package mutations

import (
	"context"
	"fmt"
	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

func HandleRecoveryProcessReconcile(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.process_reconcile requires run_id", nil)
	}
	// #355: pre-probe every running process row's liveness OUTSIDE the
	// transaction, exactly as HandleRecoveryAuto was hoisted for #198. The in-tx
	// probe in the loop below shells out to `tmux list-panes` (wrapped as
	// `sudo -n -u <user> -- env -i tmux …` under STRIATUM_LANE_OS_USER). Doing
	// that inside this lock-holding, event-appending transaction holds the per-repo
	// `repo_event_chain_heads FOR UPDATE` (the global append serialization point)
	// across slow subprocess IO; under a multi-run supervise storm every other
	// append on the repo — including HandleRunPrepare's `run.created` — queued
	// behind it and died at statement_timeout as `append_event_row (sd): 57014`.
	// This was the one in-tx subprocess path the #198 fix missed; #355 mirrors that
	// fix here. The probe is a pure read whose result only gates the in-tx writes,
	// so snapshotting it first and injecting the oracle preserves the decision
	// logic exactly. The oracle is built from the SAME JOINed row shape / probePID
	// selection the loop uses (#355 Caveat B), or the cache key would miss and the
	// loop would silently fall back to the in-tx probe, re-creating the convoy.
	ctx = withLivenessOracle(ctx, buildReconcileLivenessOracle(ctx, runner, repositoryID, runID))
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// #355: mark every call below as executing inside the reconcile transaction
		// (the #198 observability seam, shared with the sweep). Liveness probes must
		// NOT run while this is set — they were pre-probed above and read from the
		// injected oracle. The regression asserts no probe runs with inSweepTx=true.
		ctx := withinSweepTx(ctx)
		// RFC 0104: per-run advisory lock first.
		if err := lockRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		// #355 defense-in-depth: cap how long this lock-holding, event-appending
		// transaction can run. The daemon role baseline is statement_timeout=600s
		// with no lock_timeout, so before the pre-tx hoist above a wedged probe held
		// the chain-head lock for the full window. A short SET LOCAL means any
		// residual slow statement here fails its OWN transaction fast rather than
		// starving every other append on the repo. SET LOCAL is scoped to this tx and
		// reverts on commit/rollback; it never leaks to the pooled connection.
		if err := tx.Exec(ctx, `SET LOCAL statement_timeout = '15s'`); err != nil {
			return nil, err
		}
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		repoRoot := fmt.Sprint(run["repo_root"])
		rows, err := queryRows(ctx, tx, `
			SELECT pe.*,
			       p.metadata_json AS supervisor_metadata_json,
			       p.pid_start_time AS supervisor_pid_start_time,
			       p.pid AS supervisor_pid,
			       p.supervisor_id,
			       p.daemon_supervisor_id
			  FROM striatumd.process_executions pe
			  LEFT JOIN LATERAL (
			    SELECT ptr.metadata_json, ptr.pid_start_time, ptr.pid, ptr.supervisor_id, ptr.daemon_supervisor_id
			      FROM striatumd.process_supervisor_pointers ptr
			     WHERE ptr.repository_id = pe.repository_id
			       AND ptr.run_id = pe.run_id
			       AND ptr.session_id = pe.session_id
			     ORDER BY ptr.updated_at DESC, ptr.supervisor_id DESC
			     LIMIT 1
			  ) p ON true
			 WHERE pe.repository_id = $1
			   AND pe.run_id = $2
			   AND pe.state = 'running'
			 ORDER BY pe.started_at
			 FOR UPDATE OF pe`, repositoryID, runID)
		if err != nil {
			return nil, err
		}
		stillRunning := []map[string]any{}
		lost := []map[string]any{}
		now := nowString()
		for _, row := range rows {
			pid := intValue(row["pid"])
			// #355 Caveat A + B: select (metadata, probePID, expectedStart) with the
			// SAME helper the pre-tx oracle uses, then resolve liveness through
			// probeLaneLivenessCached. With the oracle injected above this reads the
			// pre-tx snapshot (no subprocess IO inside the lock window); without an
			// oracle (operator-direct RPC / unit tests) it falls back to a live probe,
			// preserving prior behavior. Calling ProbeLaneLiveness directly here would
			// make the oracle a silent no-op and re-create the convoy.
			metadata, probePID, expectedStart := reconcileProbeSelection(row)
			live := probeLaneLivenessCached(ctx, metadata, probePID, expectedStart)
			alive := live.Alive
			if live.Backed != "tmux" && pid > 0 {
				alive = pidAlive(pid)
			}
			if live.Class == string(gosupervisor.TmuxLivenessUnavailable) {
				stillRunning = append(stillRunning, map[string]any{
					"process_id": row["process_id"],
					"job_id":     row["job_id"],
					"pid":        pid,
					"started_at": row["started_at"],
					"liveness":   live.Class,
				})
				continue
			}
			if alive {
				stillRunning = append(stillRunning, map[string]any{
					"process_id": row["process_id"],
					"job_id":     row["job_id"],
					"pid":        pid,
					"started_at": row["started_at"],
					"liveness":   live.Class,
				})
				continue
			}
			processID := fmt.Sprint(row["process_id"])
			if err := tx.Exec(ctx, `
				UPDATE striatumd.process_executions
				   SET state = 'lost', ended_at = $1
				 WHERE repository_id = $2 AND process_id = $3`, now, repositoryID, processID); err != nil {
				return nil, err
			}
			var supervisorID string
			if s, ok := row["supervisor_id"].(string); ok {
				supervisorID = s
			}
			var daemonSupervisorID string
			if ds, ok := row["daemon_supervisor_id"].(string); ok {
				daemonSupervisorID = ds
			}
			if supervisorID != "" {
				stopReason := "unexpected child exit (lost)"
				if err := updateSupervisorState(ctx, tx, repositoryID, supervisorID, daemonSupervisorID, "stopped", now, 0, "", "", &now, &stopReason); err != nil {
					return nil, err
				}
				if err := markActiveSessionTerminal(ctx, tx, activeSessionTerminalUpdate{
					RepositoryID: repositoryID,
					SessionID:    fmt.Sprint(row["session_id"]),
					State:        "lost",
					Reason:       "process lost: " + stopReason,
				}); err != nil {
					return nil, err
				}
				agentloop.CleanupGeminiSettings(repoRoot, supervisorID)
				agentloop.CleanupClaudeScheduledTasksLock(repoRoot)
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "process.lost", row["session_id"], row["job_id"], nil, nil, row["lease_id"], map[string]any{
				"process_id": processID,
				"pid":        row["pid"],
				"reason":     live.Class,
			}); err != nil {
				return nil, err
			}
			job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", fmt.Sprint(row["job_id"]), true)
			if err != nil {
				return nil, err
			}
			blockerKind, err := evaluateAndBlockLostProcess(ctx, tx, repositoryID, job, fmt.Sprint(row["session_id"]), processID, row["command_json"])
			if err != nil {
				return nil, err
			}
			lost = append(lost, map[string]any{
				"process_id":   processID,
				"job_id":       row["job_id"],
				"pid":          row["pid"],
				"blocker_kind": blockerKind,
			})
		}
		return map[string]any{
			"run_id":               runID,
			"checked_count":        len(rows),
			"still_running":        stillRunning,
			"transitioned_to_lost": lost,
			"next_actions":         []string{"inspect_process_blockers", "resume_or_requeue_affected_work"},
		}, nil
	})
}
