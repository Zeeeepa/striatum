package mutations

import (
	"context"
	"errors"
	"fmt"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
	"strings"
)

func HandleRecoveryStaleLeases(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.stale_leases requires run_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first.
		if err := lockRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		if _, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true); err != nil {
			return nil, err
		}
		if _, err := expireLeases(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		// #179: the expired-lease disjunct must only surface leases that still
		// represent an UNRESOLVED stale condition. A lease that recovery already
		// transferred or requeued away is historical provenance, not an actionable
		// stale lease — its job has since moved to a fresh lease/attempt — yet the
		// bare `l.state = 'expired'` join kept matching it, so every later
		// recovery.stale_leases call re-reported the same already-released lease as
		// stale. expireLeases (run just above) stamps a genuinely stale lease
		// state='expired', released_at=now, release_reason='expired', so released_at
		// alone cannot discriminate; the recovery transfer/requeue release reasons
		// are what mark a lease as already handled. Exclude those.
		rows, err := queryRows(ctx, tx, `
			SELECT j.job_id, j.workflow_job_id, j.state AS job_state,
			       j.write_scope_json,
			       l.lease_id, l.owner_session_id, l.acquired_at,
			       l.expires_at, l.released_at, l.release_reason,
			       l.state AS lease_state,
			       qm.message_id, qm.state AS message_state
			  FROM striatumd.jobs j
			  LEFT JOIN striatumd.leases l
			    ON l.repository_id = j.repository_id
			   AND (l.lease_id = j.current_lease_id
			        OR (l.resource_id = j.job_id
			            AND l.state = 'expired'
			            AND `+db.ExpiredLeaseStillStalePredicate+`))
			  LEFT JOIN striatumd.queue_messages qm
			    ON qm.repository_id = j.repository_id
			   AND qm.message_id = j.current_message_id
			 WHERE j.repository_id = $1
			   AND j.run_id = $2
			   AND (j.state = 'stale_lease'
			        OR (l.state = 'expired'
			            AND `+db.ExpiredLeaseStillStalePredicate+`))
			 ORDER BY j.workflow_job_id, l.expires_at`, repositoryID, runID)
		if err != nil {
			return nil, err
		}
		entries := []map[string]any{}
		seen := map[string]bool{}
		for _, row := range rows {
			key := fmt.Sprintf("%s/%v", row["job_id"], row["lease_id"])
			if seen[key] {
				continue
			}
			seen[key] = true
			repoWrite := isRepoWrite(row)
			policy := "safe_to_reclaim_when_pending"
			actions := []string{"register_or_select_session", "claim_available_work"}
			if repoWrite {
				policy = "manual_inspection_required"
				actions = []string{"inspect_worktree_and_artifacts", "decide_requeue_or_cancel"}
			}
			entries = append(entries, map[string]any{
				"job_id":           row["job_id"],
				"workflow_job_id":  row["workflow_job_id"],
				"job_state":        row["job_state"],
				"lease_id":         row["lease_id"],
				"owner_session_id": row["owner_session_id"],
				"expires_at":       row["expires_at"],
				"released_at":      row["released_at"],
				"release_reason":   row["release_reason"],
				"message_id":       row["message_id"],
				"message_state":    row["message_state"],
				"repo_write":       repoWrite,
				"recovery_policy":  policy,
				"next_actions":     actions,
			})
		}
		nextActions := []string{}
		if len(entries) > 0 {
			nextActions = []string{"inspect_worktree_and_artifacts", "decide_requeue_or_cancel"}
		}
		return map[string]any{
			"run_id":       runID,
			"stale_count":  len(entries),
			"stale_leases": entries,
			"next_actions": nextActions,
		}, nil
	})
}

// requeueSameAttemptOptions carries the provenance recorded on the
// recovery.requeued_same_attempt event for a same-attempt requeue.
type requeueSameAttemptOptions struct {
	operatorOverride bool
	justification    string
	author           string
}

// requeueSameAttemptResult reports what the helper did so callers can shape
// their RPC result and detect the idempotent no-op path.
type requeueSameAttemptResult struct {
	messageID          any
	alreadyReclaimable bool
	leaseID            any
}

// terminalMessageStates are the queue_message states that are NOT live — a job
// whose current message is in one of these (or NULL) needs a fresh pending one
// to become claimable. Mirrors slotHasUnclaimedParallelWork's "live" set
// (pending/claimed/acked) by complement.
var terminalMessageStates = map[string]bool{
	"completed": true,
	"canceled":  true,
	"failed":    true,
	"expired":   true,
	"dead":      true,
}

// requeueJobSameAttempt returns a dead-lane unfinished job to claimable WITHOUT
// bumping the attempt and WITHOUT resetting downstream. It is the RFC 0101
// Phase 3 Slice 1 primitive for the "running-limbo" failure: a supervised lane
// died (operator close, dead pane, missed heartbeat) leaving jobs.state in
// claimed/running/stale_lease, current_lease_id NULL, the lease released, and
// zero artifacts — so neither the auto-publish recovery nor the expired-lease
// requeue path (HandleRecoveryRequeueStale's JOIN to an expired lease) can
// reclaim it.
//
// Unlike reopenJobForAttempt (revision_routing.go) this is an OPERATIONAL, not
// content, recovery: attempt/max_attempts and downstream jobs are untouched.
//
// It is idempotent: an already-queued job whose current message is pending is a
// no-op success (result.alreadyReclaimable=true), mirroring the
// already_reclaimable pattern in HandleRecoveryRequeueStale.
//
// The job map must carry at least job_id, run_id, state, current_lease_id,
// current_message_id, and the columns insertPendingMessageForJob needs
// (role_id, lane_selector_json or target lane, max_attempts).
func requeueJobSameAttempt(ctx context.Context, tx db.TxRunner, repositoryID string, job map[string]any, opts requeueSameAttemptOptions) (requeueSameAttemptResult, error) {
	jobID := fmt.Sprint(job["job_id"])
	runID := job["run_id"]
	now := nowString()

	// Force-expire any residual ACTIVE lease still pinned to the job so a fresh
	// claim cannot trip the uq_active_resource_lease partial unique index. A
	// lease already in 'released'/'expired' is harmless and left as-is.
	if err := tx.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'expired', released_at = COALESCE(released_at, $1),
		       release_reason = 'recovery_requeue'
		 WHERE repository_id = $2 AND resource_id = $3 AND state = 'active'`,
		now, repositoryID, jobID); err != nil {
		return requeueSameAttemptResult{}, err
	}

	// Resolve the job's current message (if any) and whether it is still live.
	messageID := nullable(job["current_message_id"])
	currentMessageState := ""
	currentMessageLive := false
	if messageID != nil {
		row, err := oneRow(ctx, tx, `
			SELECT state FROM striatumd.queue_messages
			 WHERE repository_id = $1 AND message_id = $2`, repositoryID, messageID)
		if errors.Is(err, pgx.ErrNoRows) {
			messageID = nil
		} else if err != nil {
			return requeueSameAttemptResult{}, err
		} else {
			currentMessageState = fmt.Sprint(row["state"])
			currentMessageLive = !terminalMessageStates[currentMessageState]
		}
	}
	// RFC 0101 Phase 5: the live foreground claim path (HandleClaimNext) binds the
	// work message to a lease but does NOT stamp jobs.current_message_id, so a
	// genuinely live-claimed job that then dies arrives here with
	// current_message_id NULL while its work message is still in a non-terminal
	// (claimed/acked) state. Minting a fresh pending message in that case would
	// trip the uq_active_work_message_per_job partial unique index (one
	// non-terminal work message per job). Resolve the job's still-live work
	// message directly so we REUSE it rather than duplicate it. This is keyed on
	// the same (pending/claimed/acked) set the unique index covers, so it finds
	// exactly the message the index would collide with. (Surfaced by the
	// fault-injection chaos suite, which drives the REAL claim path rather than a
	// hand-seeded current_message_id.)
	if messageID == nil {
		row, err := oneRow(ctx, tx, `
			SELECT message_id, state FROM striatumd.queue_messages
			 WHERE repository_id = $1 AND job_id = $2 AND kind = 'work'
			   AND state IN ('pending','claimed','acked')
			 ORDER BY created_at DESC, message_id DESC
			 LIMIT 1`, repositoryID, jobID)
		if errors.Is(err, pgx.ErrNoRows) {
			messageID = nil
		} else if err != nil {
			return requeueSameAttemptResult{}, err
		} else {
			messageID = row["message_id"]
			currentMessageState = fmt.Sprint(row["state"])
			currentMessageLive = !terminalMessageStates[currentMessageState]
		}
	}
	leaseID := nullable(job["current_lease_id"])

	// Idempotency: an already-queued job whose live message is already pending is
	// reclaimable; report a no-op success without mutating the job/message state.
	if currentMessageLive && fmt.Sprint(job["state"]) == "queued" && currentMessageState == "pending" {
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.requeued_same_attempt", nil, jobID, messageID, nil, leaseID, requeueSameAttemptEventPayload(opts, true)); err != nil {
			return requeueSameAttemptResult{}, err
		}
		recordWake(tx, WakeEvent{
			RepositoryID: repositoryID,
			RunID:        fmt.Sprint(runID),
			Kind:         "work_available",
			MessageID:    fmt.Sprint(messageID),
		})
		return requeueSameAttemptResult{messageID: messageID, alreadyReclaimable: true, leaseID: leaseID}, nil
	}

	if !currentMessageLive {
		// No live message (NULL or terminal): mint a fresh pending one. This also
		// flips the job to queued + current_lease_id NULL + current_message_id.
		created, err := insertPendingMessageForJob(ctx, tx, repositoryID, job, now)
		if err != nil {
			return requeueSameAttemptResult{}, err
		}
		messageID = created
	} else {
		// Reuse the live message: flip the job to queued and the message back to
		// pending so a fresh session can claim the SAME attempt.
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = 'queued', current_lease_id = NULL
			 WHERE repository_id = $1 AND job_id = $2`, repositoryID, jobID); err != nil {
			return requeueSameAttemptResult{}, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'pending', current_lease_id = NULL, updated_at = $1
			 WHERE repository_id = $2 AND message_id = $3`, now, repositoryID, messageID); err != nil {
			return requeueSameAttemptResult{}, err
		}
	}

	if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.requeued_same_attempt", nil, jobID, messageID, nil, leaseID, requeueSameAttemptEventPayload(opts, false)); err != nil {
		return requeueSameAttemptResult{}, err
	}
	recordWake(tx, WakeEvent{
		RepositoryID: repositoryID,
		RunID:        fmt.Sprint(runID),
		Kind:         "work_available",
		MessageID:    fmt.Sprint(messageID),
	})
	return requeueSameAttemptResult{messageID: messageID, leaseID: leaseID}, nil
}

func requeueSameAttemptEventPayload(opts requeueSameAttemptOptions, alreadyReclaimable bool) map[string]any {
	payload := map[string]any{
		"already_reclaimable": alreadyReclaimable,
	}
	if opts.operatorOverride {
		payload["operator_override"] = true
		payload["repo_write"] = true
	}
	if opts.justification != "" {
		payload["justification"] = opts.justification
	}
	if opts.author != "" {
		payload["author"] = opts.author
	}
	return payload
}

func HandleRecoveryRequeueStale(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	jobID := stringParam(envelope, "job_id")
	if runID == "" || jobID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.requeue_stale requires run_id and job_id", nil)
	}
	force := boolParam(envelope, "force")
	justification := strings.TrimSpace(stringParam(envelope, "justification"))
	if force && justification == "" {
		return nil, rpc.NewError("invalid_transition", "--force requeue requires --justification", nil)
	}
	recoveryAuthor := stringParam(envelope, "recovery_author")

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0104: per-run advisory lock first.
		if err := lockRun(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		if _, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true); err != nil {
			return nil, err
		}
		if _, err := expireLeases(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		now := nowString()
		// #82: an operator-inspected transfer (`--force`) of a live-but-wrong
		// repo-write claim. Force-expire the job's still-active lease and mark a
		// claimed/running job stale so the same attempt + queue message can be
		// requeued to a fresh session below — a lease-ownership correction that,
		// unlike run.retry_job, does NOT bump the attempt counter or reset
		// downstream. `--force` already requires `--justification`.
		if force {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.leases
				   SET state = 'expired', released_at = $1, release_reason = 'operator_transfer'
				 WHERE repository_id = $2 AND resource_id = $3 AND state = 'active'`,
				now, repositoryID, jobID); err != nil {
				return nil, err
			}
			if err := tx.Exec(ctx, `
				UPDATE striatumd.jobs
				   SET state = 'stale_lease'
				 WHERE repository_id = $1 AND job_id = $2 AND state IN ('claimed', 'running')`,
				repositoryID, jobID); err != nil {
				return nil, err
			}
		}
		rows, err := queryRows(ctx, tx, `
			SELECT j.job_id, j.run_id, j.workflow_job_id, j.state,
			       j.role_id, j.lane_selector_json, j.max_attempts,
			       j.write_scope_json, j.current_message_id, j.current_lease_id,
			       l.lease_id, l.owner_session_id, l.expires_at,
			       qm.message_id, qm.state AS message_state
			  FROM striatumd.jobs j
			  JOIN striatumd.leases l
			    ON l.repository_id = j.repository_id
			   AND l.resource_id = j.job_id
			   AND l.state = 'expired'
			  LEFT JOIN striatumd.queue_messages qm
			    ON qm.repository_id = j.repository_id
			   AND qm.message_id = j.current_message_id
			 WHERE j.repository_id = $1
			   AND j.run_id = $2
			   AND j.job_id = $3
			   AND j.state IN ('queued', 'blocked', 'stale_lease')
			 ORDER BY l.expires_at DESC
			 LIMIT 1
			 FOR UPDATE OF j, l`, repositoryID, runID, jobID)
		if err != nil {
			return nil, err
		}
		var row map[string]any
		if len(rows) > 0 {
			row = rows[0]
		} else {
			// #82: when the job is held by a LIVE claimant (active lease) rather
			// than a stale/expired one, guide the operator to the transfer path
			// instead of the bare "no stale lease" error.
			hasActive, lerr := existsRow(ctx, tx, `
				SELECT 1 FROM striatumd.leases
				 WHERE repository_id = $1 AND resource_id = $2 AND state = 'active' LIMIT 1`,
				repositoryID, jobID)
			if lerr != nil {
				return nil, lerr
			}
			if hasActive {
				return nil, rpc.NewError("invalid_transition", "job is held by a live claimant (active lease); after stopping the wrong session and inspecting, transfer it with `--force --justification \"<reason>\"` (preserves the attempt; does not retry the job)", nil)
			}
			// RFC 0101 Phase 3 Slice 1 (#121): a dead-lane repo-write job is left in
			// "running-limbo" — jobs.state in claimed/running/stale_lease,
			// current_lease_id NULL, the lease already released (NOT expired), and
			// zero artifacts. There is no expired-lease row for the JOIN above to
			// find, so today this errored "no stale expired lease". Reclaim it on the
			// SAME attempt via requeueJobSameAttempt (no attempt bump, no downstream
			// reset). The D036 repo-write inspection gate is preserved: repo-write
			// still requires --force --justification. 'queued' is included so a
			// repeated requeue of an already-reclaimed job is an idempotent no-op
			// (the helper detects already_reclaimable); 'blocked' is excluded — a
			// blocked job with no lease is legitimately waiting on dependencies.
			limbo, lerr := queryRows(ctx, tx, `
				SELECT j.job_id, j.run_id, j.workflow_job_id, j.state,
				       j.role_id, j.lane_selector_json, j.max_attempts,
				       j.write_scope_json, j.current_message_id, j.current_lease_id
				  FROM striatumd.jobs j
				 WHERE j.repository_id = $1
				   AND j.run_id = $2
				   AND j.job_id = $3
				   AND j.state IN ('claimed', 'running', 'stale_lease', 'queued')
				 LIMIT 1
				 FOR UPDATE OF j`, repositoryID, runID, jobID)
			if lerr != nil {
				return nil, lerr
			}
			if len(limbo) == 0 {
				return nil, rpc.NewError("invalid_transition", "job has no stale expired lease to requeue", nil)
			}
			row = limbo[0]
		}

		repoWrite := isRepoWrite(row)
		if repoWrite && !force {
			return nil, rpc.NewError("invalid_transition", "repo-write stale jobs require manual inspection; rerun with `--force --justification \"<reason>\"` to override after inspection", nil)
		}
		opts := requeueSameAttemptOptions{author: recoveryAuthor}
		if force && repoWrite {
			opts.operatorOverride = true
			opts.justification = justification
		}
		result, err := requeueJobSameAttempt(ctx, tx, repositoryID, row, opts)
		if err != nil {
			return nil, err
		}
		// Preserve the verb's legacy `recovery.stale_requeued` audit event (the
		// helper also appends the canonical `recovery.requeued_same_attempt`); keep
		// the original payload shape so existing audit consumers are unchanged.
		legacyPayload := map[string]any{
			"already_reclaimable": result.alreadyReclaimable,
			"repo_write":          repoWrite,
		}
		if recoveryAuthor != "" {
			legacyPayload["author"] = recoveryAuthor
		}
		if force && repoWrite {
			legacyPayload["operator_override"] = true
			legacyPayload["justification"] = justification
		}
		// On the expired-lease JOIN path row["lease_id"] is the expired lease that
		// was found; on the dead-lane limbo path there is no such row, so it is nil.
		responseLeaseID := nullable(row["lease_id"])
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "recovery.stale_requeued", nil, jobID, result.messageID, nil, responseLeaseID, legacyPayload); err != nil {
			return nil, err
		}
		status := "requeued"
		if result.alreadyReclaimable {
			status = "already_reclaimable"
		}
		return map[string]any{
			"status":            status,
			"run_id":            runID,
			"job_id":            jobID,
			"workflow_job_id":   row["workflow_job_id"],
			"lease_id":          responseLeaseID,
			"message_id":        result.messageID,
			"repo_write":        repoWrite,
			"operator_override": force && repoWrite,
			"next_actions":      []string{"register_or_select_session", "claim_available_work"},
		}, nil
	})
}

func insertPendingMessageForJob(ctx context.Context, runner any, repositoryID string, job map[string]any, now string) (string, error) {
	messageID, err := newID("msg")
	if err != nil {
		return "", err
	}
	selector := asMap(job["lane_selector_json"])
	targetLane, _ := selector["lane_id"].(string)
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return "", fmt.Errorf("runner does not support exec")
	}
	payloadArg, err := db.JSONBArg(runner, map[string]any{})
	if err != nil {
		return "", err
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, payload_json, claim_count,
		  max_claims, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7::jsonb,0,$8,$9,$10)`,
		repositoryID,
		messageID,
		job["run_id"],
		job["job_id"],
		job["role_id"],
		nullable(targetLane),
		payloadArg,
		job["max_attempts"],
		now,
		now,
	); err != nil {
		return "", err
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'queued', current_lease_id = NULL, current_message_id = $1
		 WHERE repository_id = $2 AND job_id = $3`, messageID, repositoryID, job["job_id"]); err != nil {
		return "", err
	}
	return messageID, nil
}
