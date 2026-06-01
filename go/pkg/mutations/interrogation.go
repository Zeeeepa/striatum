package mutations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/lanehealth"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	"github.com/jackc/pgx/v5"
)

// RFC 0082: interrogation sessions. A bounded, peer-addressed, multi-turn Q&A
// bound to a live target worker session whose context is preserved. Lifecycle
// + correlation live in striatumd.interrogations (migration 0016); turns reuse
// the message bus (queue_messages, kind='agent_message', target_session_id set,
// payload_json.interrogation_id for correlation). The daemon is the sole
// writer and appends events for each transition.

const interrogateCapability = "interrogate"

// HandleInterrogationOpen opens an interrogation against a live target session.
// Only an interrogator-capable session may open; the target must be live
// (active + attested per D026) or the call fails target_unavailable.
func HandleInterrogationOpen(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	targetSessionID := stringParam(envelope, "target_session_id")
	topic := strings.TrimSpace(stringParam(envelope, "topic"))
	if sessionID == "" || targetSessionID == "" {
		return nil, rpc.NewError("schema_invalid", "interrogation.open requires session_id and target_session_id", nil)
	}
	if sessionID == targetSessionID {
		return nil, rpc.NewError("invalid_transition", "a session cannot interrogate itself", nil)
	}
	return withTxRetryOnDeadlock(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0101 Phase 0a (#137): serialize against the target's own await/claim
		// transaction on the same run BEFORE any FOR UPDATE so the {sessions, runs}
		// lock cycle cannot form. The interrogator's run_id is the per-run key.
		runID, err := sessionRunID(ctx, tx, repositoryID, sessionID)
		if err != nil {
			return nil, interrogationSessionError(err, "interrogator session not found")
		}
		if err := lockRunInterrogation(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		interrogator, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", sessionID, true)
		if err != nil {
			return nil, interrogationSessionError(err, "interrogator session not found")
		}
		if fmt.Sprint(interrogator["state"]) != "active" {
			return nil, rpc.NewError("invalid_transition", "interrogator session is not active", nil)
		}
		if !sessionHasCapability(interrogator, interrogateCapability) {
			return nil, rpc.NewError("capability_denied", "interrogator session lacks the 'interrogate' capability", nil)
		}
		if err := requireLiveTarget(ctx, tx, repositoryID, targetSessionID); err != nil {
			return nil, err
		}
		target, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", targetSessionID, false)
		if err != nil {
			return nil, interrogationSessionError(err, "target session not found")
		}
		runID = fmt.Sprint(interrogator["run_id"])
		if fmt.Sprint(target["run_id"]) != runID {
			return nil, rpc.NewError("invalid_transition", "interrogation is single-run scoped: interrogator and target must share a run", nil)
		}
		interrogationID, err := newID("intg")
		if err != nil {
			return nil, err
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.interrogations (
			  repository_id, run_id, interrogation_id, interrogator_session_id,
			  target_session_id, topic, state, turn_count, opened_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,'open',0,$7)`,
			repositoryID, runID, interrogationID, sessionID, targetSessionID,
			nullable(topic), now); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "interrogation.opened", sessionID, nil, nil, nil, nil, map[string]any{
			"interrogation_id":  interrogationID,
			"target_session_id": targetSessionID,
			"topic":             nullable(topic),
		}); err != nil {
			return nil, err
		}
		return map[string]any{
			"interrogation_id":  interrogationID,
			"run_id":            runID,
			"target_session_id": targetSessionID,
			"state":             "open",
		}, nil
	})
}

// HandleInterrogationAsk enqueues a question turn addressed to the target
// session's receive loop. Only the interrogator may ask, and only while the
// interrogation is open and the target is still live.
func HandleInterrogationAsk(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	interrogationID := stringParam(envelope, "interrogation_id")
	body := strings.TrimSpace(stringParam(envelope, "body"))
	if sessionID == "" || interrogationID == "" || body == "" {
		return nil, rpc.NewError("schema_invalid", "interrogation.ask requires session_id, interrogation_id, and body", nil)
	}
	return withTxRetryOnDeadlock(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0101 Phase 0a (#137): take the per-run interrogation lock first so
		// this critical section serializes against the target's await/claim on the
		// same run (see lockRunInterrogation).
		runID, err := interrogationRunID(ctx, tx, repositoryID, interrogationID)
		if err != nil {
			return nil, err
		}
		if err := lockRunInterrogation(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		interrogation, err := lockInterrogation(ctx, tx, repositoryID, interrogationID)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(interrogation["state"]) != "open" {
			return nil, rpc.NewError("invalid_transition", "interrogation is closed", nil)
		}
		if fmt.Sprint(interrogation["interrogator_session_id"]) != sessionID {
			return nil, rpc.NewError("capability_denied", "only the interrogator session may ask", nil)
		}
		interrogator, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", sessionID, false)
		if err != nil {
			return nil, interrogationSessionError(err, "interrogator session not found")
		}
		if !sessionHasCapability(interrogator, interrogateCapability) {
			return nil, rpc.NewError("capability_denied", "interrogator session lacks the 'interrogate' capability", nil)
		}
		targetSessionID := fmt.Sprint(interrogation["target_session_id"])
		if err := requireLiveTarget(ctx, tx, repositoryID, targetSessionID); err != nil {
			return nil, err
		}
		turnIndex := intValue(interrogation["turn_count"])
		messageID, err := interrogationTurnMessage(ctx, tx, repositoryID, interrogation, "interrogation_question", targetSessionID, body, turnIndex, "pending")
		if err != nil {
			return nil, err
		}
		if err := bumpInterrogationTurn(ctx, tx, repositoryID, interrogationID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, interrogation["run_id"], "interrogation.asked", sessionID, nil, messageID, nil, nil, map[string]any{
			"interrogation_id":  interrogationID,
			"target_session_id": targetSessionID,
			"turn_index":        turnIndex,
		}); err != nil {
			return nil, err
		}
		return map[string]any{
			"interrogation_id":  interrogationID,
			"message_id":        messageID,
			"turn_index":        turnIndex,
			"target_session_id": targetSessionID,
		}, nil
	})
}

// HandleInterrogationAnswer records the target session's reply, addressed back
// to the interrogator. Only the target session may answer.
func HandleInterrogationAnswer(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	interrogationID := stringParam(envelope, "interrogation_id")
	body := strings.TrimSpace(stringParam(envelope, "body"))
	if sessionID == "" || interrogationID == "" || body == "" {
		return nil, rpc.NewError("schema_invalid", "interrogation.answer requires session_id, interrogation_id, and body", nil)
	}
	return withTxRetryOnDeadlock(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0101 Phase 0a (#137): the answer path locks the target `sessions`
		// row (sessionliveness.Record) and was the half of the {sessions, runs}
		// cycle that aborted with 40P01. Take the per-run lock first so it
		// serializes against the target's concurrent await/claim.
		runID, err := interrogationRunID(ctx, tx, repositoryID, interrogationID)
		if err != nil {
			return nil, err
		}
		if err := lockRunInterrogation(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		interrogation, err := lockInterrogation(ctx, tx, repositoryID, interrogationID)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(interrogation["state"]) != "open" {
			return nil, rpc.NewError("invalid_transition", "interrogation is closed", nil)
		}
		if fmt.Sprint(interrogation["target_session_id"]) != sessionID {
			return nil, rpc.NewError("capability_denied", "only the target session may answer this interrogation", nil)
		}
		interrogatorSessionID := fmt.Sprint(interrogation["interrogator_session_id"])
		turnIndex := intValue(interrogation["turn_count"])
		messageID, err := interrogationTurnMessage(ctx, tx, repositoryID, interrogation, "interrogation_answer", interrogatorSessionID, body, turnIndex, "completed")
		if err != nil {
			return nil, err
		}
		// Mark the most recent outstanding question for this interrogation as
		// answered so the target's receive loop does not re-deliver it.
		if err := tx.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'completed', completed_at = $1, updated_at = $1
			 WHERE repository_id = $2 AND message_id = (
			   SELECT message_id FROM striatumd.queue_messages
			    WHERE repository_id = $2
			      AND kind = 'agent_message'
			      AND payload_json->>'interrogation_id' = $3
			      AND payload_json->>'turn' = 'question'
			      AND state IN ('pending','acked')
			    ORDER BY created_at DESC, message_id DESC
			    LIMIT 1)`,
			nowString(), repositoryID, interrogationID); err != nil {
			return nil, err
		}
		if err := bumpInterrogationTurn(ctx, tx, repositoryID, interrogationID); err != nil {
			return nil, err
		}
		if err := sessionliveness.Record(ctx, tx, repositoryID, sessionID, sessionliveness.LastSessionQuestionAt); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, interrogation["run_id"], "interrogation.answered", sessionID, nil, messageID, nil, nil, map[string]any{
			"interrogation_id": interrogationID,
			"turn_index":       turnIndex,
		}); err != nil {
			return nil, err
		}
		return map[string]any{
			"interrogation_id": interrogationID,
			"message_id":       messageID,
			"turn_index":       turnIndex,
		}, nil
	})
}

// HandleInterrogationClose terminates an interrogation. If the target session
// is in the awaiting_interrogation window (its interrogable job completed, it
// holds no active lease, and no other interrogations remain open against it),
// the target session is closed — this is the bounded close of the
// context-preservation window described in RFC 0082 §5.
func HandleInterrogationClose(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	interrogationID := stringParam(envelope, "interrogation_id")
	if sessionID == "" || interrogationID == "" {
		return nil, rpc.NewError("schema_invalid", "interrogation.close requires session_id and interrogation_id", nil)
	}
	return withTxRetryOnDeadlock(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// RFC 0101 Phase 0a (#137): close also closes the target session (window
		// teardown), touching `sessions`; take the per-run lock first for a uniform
		// lock order across all interrogation mutations.
		lockRunID, err := interrogationRunID(ctx, tx, repositoryID, interrogationID)
		if err != nil {
			return nil, err
		}
		if err := lockRunInterrogation(ctx, tx, repositoryID, lockRunID); err != nil {
			return nil, err
		}
		interrogation, err := lockInterrogation(ctx, tx, repositoryID, interrogationID)
		if err != nil {
			return nil, err
		}
		runID := fmt.Sprint(interrogation["run_id"])
		targetSessionID := fmt.Sprint(interrogation["target_session_id"])
		if fmt.Sprint(interrogation["state"]) == "closed" {
			return nil, rpc.NewError("invalid_transition", "interrogation is already closed", nil)
		}
		if fmt.Sprint(interrogation["interrogator_session_id"]) != sessionID {
			return nil, rpc.NewError("capability_denied", "only the interrogator session may close this interrogation", nil)
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.interrogations
			   SET state = 'closed', closed_at = $1
			 WHERE repository_id = $2 AND interrogation_id = $3`,
			now, repositoryID, interrogationID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "interrogation.closed", sessionID, nil, nil, nil, nil, map[string]any{
			"interrogation_id":  interrogationID,
			"target_session_id": targetSessionID,
		}); err != nil {
			return nil, err
		}
		targetClosed, err := maybeCloseInterrogationTarget(ctx, tx, repositoryID, runID, targetSessionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"interrogation_id":      interrogationID,
			"state":                 "closed",
			"target_session_id":     targetSessionID,
			"target_session_closed": targetClosed,
		}, nil
	})
}

// terminalInterrogationConsumerStates are the job states in which a reviewer
// dependent of an interrogable target has finished and will not interrogate the
// live target again. A reviewer in any other (pre-verdict working) state —
// including stale_lease, which recovery may re-run — is a live panel consumer
// that still holds the interrogation window open.
const terminalInterrogationConsumerStates = `('completed','failed','canceled','skipped','waiting_human')`

// maybeCloseInterrogationTarget closes a target session that finished its
// interrogable work, once the whole review panel that consumes it has finished.
//
// #65 P1: the awaiting_interrogation window is owned by the panel/gate (the set
// of reviewer jobs that depend on the interrogable job), not by an individual
// interrogation thread. The first reviewer's interrogation.close must NOT tear
// down the target while reviewers 2..N still have to interrogate it; otherwise
// they get target_unavailable and are forced to vote without interrogating —
// exactly what an interrogating panel exists to prevent. The target therefore
// stays live until no reviewer dependent remains in a pre-verdict working state
// (and no interrogation is open against it, and it holds no active lease).
func maybeCloseInterrogationTarget(ctx context.Context, runner any, repositoryID, runID, targetSessionID string) (bool, error) {
	target, err := rowByID(ctx, runner, repositoryID, "sessions", "session_id", targetSessionID, true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if fmt.Sprint(target["state"]) != "active" {
		return false, nil
	}
	openOthers, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.interrogations
		 WHERE repository_id = $1 AND target_session_id = $2 AND state = 'open'
		 LIMIT 1`, repositoryID, targetSessionID)
	if err != nil {
		return false, err
	}
	if openOthers {
		return false, nil
	}
	activeLease, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.leases
		 WHERE repository_id = $1 AND owner_session_id = $2 AND state = 'active'
		 LIMIT 1`, repositoryID, targetSessionID)
	if err != nil {
		return false, err
	}
	if activeLease {
		return false, nil
	}
	// Panel-owned window: keep the target live while any reviewer that depends on
	// the interrogable job is still in a pre-verdict working state. A target that
	// never entered a panel window (interrogableJobID == "") falls through to the
	// legacy single-thread close below.
	interrogableJobID, err := interrogableJobForTargetSession(ctx, runner, repositoryID, targetSessionID)
	if err != nil {
		return false, err
	}
	if interrogableJobID != "" {
		pending, err := interrogationConsumersPending(ctx, runner, repositoryID, interrogableJobID)
		if err != nil {
			return false, err
		}
		if pending {
			return false, nil
		}
	}
	return closeInterrogationTargetSession(ctx, runner, repositoryID, runID, target, "interrogation_window_closed", "interrogation_close")
}

// closeInterrogationTargetSession transitions an interrogable target session to
// closed and records the session.closed event. Callers own the window guards
// (active state, no open interrogations, no active lease, no pending panel
// consumers); this is the shared write.
func closeInterrogationTargetSession(ctx context.Context, runner any, repositoryID, runID string, target map[string]any, reason, source string) (bool, error) {
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return false, fmt.Errorf("runner does not support exec")
	}
	now := nowString()
	targetSessionID := fmt.Sprint(target["session_id"])
	if err := exec.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET state = 'closed', closed_at = $1, close_reason = $2
		 WHERE repository_id = $3 AND session_id = $4`,
		now, reason, repositoryID, targetSessionID); err != nil {
		return false, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, runID, "session.closed", targetSessionID, nil, nil, nil, nil, map[string]any{
		"session_id": targetSessionID,
		"role_id":    target["role_id"],
		"lane_id":    target["lane_id"],
		"reason":     reason,
		"source":     source,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// interrogableJobForTargetSession returns the interrogable job whose completion
// put targetSessionID into the awaiting_interrogation window, or "" if the
// session never entered such a window (e.g. an ad-hoc, non-panel interrogation
// of a wrapper-attested session). The window is identified by the
// session.awaiting_interrogation event (lifecycle.go), keyed by actor session.
func interrogableJobForTargetSession(ctx context.Context, runner any, repositoryID, targetSessionID string) (string, error) {
	row, err := oneRow(ctx, runner, `
		SELECT job_id FROM striatumd.events
		 WHERE repository_id = $1 AND actor_session_id = $2
		   AND event_type = 'session.awaiting_interrogation'
		   AND job_id IS NOT NULL
		 ORDER BY event_id DESC
		 LIMIT 1`, repositoryID, targetSessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return fmt.Sprint(row["job_id"]), nil
}

// interrogableTargetSessionForJob returns the session that entered the
// awaiting_interrogation window when interrogableJobID completed, or "" if the
// job is not interrogable / never opened a window.
func interrogableTargetSessionForJob(ctx context.Context, runner any, repositoryID, interrogableJobID string) (string, error) {
	row, err := oneRow(ctx, runner, `
		SELECT actor_session_id FROM striatumd.events
		 WHERE repository_id = $1 AND job_id = $2
		   AND event_type = 'session.awaiting_interrogation'
		   AND actor_session_id IS NOT NULL
		 ORDER BY event_id DESC
		 LIMIT 1`, repositoryID, interrogableJobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return fmt.Sprint(row["actor_session_id"]), nil
}

// interrogationConsumersPending reports whether any reviewer job that depends on
// the interrogable job is still in a pre-verdict working state — i.e. could yet
// interrogate the live target. Direct dependents of an interrogable job are
// exactly the review panel (the next phase depends on the reviewers, not on the
// interrogable job), so this is a precise panel-scoped predicate.
func interrogationConsumersPending(ctx context.Context, runner any, repositoryID, interrogableJobID string) (bool, error) {
	return existsRow(ctx, runner, `
		SELECT 1
		  FROM striatumd.job_dependencies dep
		  JOIN striatumd.jobs j
		    ON j.repository_id = dep.repository_id AND j.job_id = dep.job_id
		 WHERE dep.repository_id = $1 AND dep.depends_on_job_id = $2
		   AND j.state NOT IN `+terminalInterrogationConsumerStates+`
		 LIMIT 1`, repositoryID, interrogableJobID)
}

// releaseInterrogationTargetForCompletedReview closes the interrogable target
// session(s) upstream of a just-terminated reviewer job once every reviewer in
// the panel has finished. This is the authoritative panel-window closer: the
// last reviewer's interrogation.close cannot close the target (the closing
// reviewer's own job is still active at that moment), so the window is retired
// here when the final reviewer job reaches a terminal state. Safe to call after
// any reviewer job completes; it no-ops while consumers are still pending.
func releaseInterrogationTargetForCompletedReview(ctx context.Context, runner any, repositoryID, runID, reviewJobID string) error {
	upstreams, err := queryRows(ctx, runner, `
		SELECT depends_on_job_id
		  FROM striatumd.job_dependencies
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, reviewJobID)
	if err != nil {
		return err
	}
	for _, up := range upstreams {
		interrogableJobID := fmt.Sprint(up["depends_on_job_id"])
		if interrogableJobID == "" || interrogableJobID == "<nil>" {
			continue
		}
		targetSessionID, err := interrogableTargetSessionForJob(ctx, runner, repositoryID, interrogableJobID)
		if err != nil {
			return err
		}
		if targetSessionID == "" {
			continue
		}
		if _, err := maybeCloseInterrogationTarget(ctx, runner, repositoryID, runID, targetSessionID); err != nil {
			return err
		}
	}
	return nil
}

// closeInterrogationTargetForReopen retires the prior-attempt interrogable
// target session when its job is re-opened for a revision attempt. With the
// panel-owned window (#65 P1) the target stays live through the whole panel, so
// a needs_revision re-open must explicitly close the superseded session (and any
// interrogation still open against it) — the fresh attempt spawns a new session.
func closeInterrogationTargetForReopen(ctx context.Context, runner any, repositoryID, runID, interrogableJobID string) error {
	targetSessionID, err := interrogableTargetSessionForJob(ctx, runner, repositoryID, interrogableJobID)
	if err != nil {
		return err
	}
	if targetSessionID == "" {
		return nil
	}
	target, err := rowByID(ctx, runner, repositoryID, "sessions", "session_id", targetSessionID, true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if fmt.Sprint(target["state"]) != "active" {
		return nil
	}
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	now := nowString()
	// Retire any interrogation still open against the superseded target so the
	// revision boundary leaves no orphaned-open interrogation rows.
	if err := exec.Exec(ctx, `
		UPDATE striatumd.interrogations
		   SET state = 'closed', closed_at = $1
		 WHERE repository_id = $2 AND target_session_id = $3 AND state = 'open'`,
		now, repositoryID, targetSessionID); err != nil {
		return err
	}
	_, err = closeInterrogationTargetSession(ctx, runner, repositoryID, runID, target, "revision_reopened", "revision_reopen")
	return err
}

// interrogationTurnMessage writes a turn onto the message bus. Turns are curated
// records (D028): the authored body text plus correlation identifiers, never
// raw provider output.
func interrogationTurnMessage(ctx context.Context, tx db.TxRunner, repositoryID string, interrogation map[string]any, kind, targetSessionID, body string, turnIndex int, state string) (string, error) {
	messageID, err := newID("msg")
	if err != nil {
		return "", err
	}
	interrogationID := fmt.Sprint(interrogation["interrogation_id"])
	turn := "answer"
	if kind == "interrogation_question" {
		turn = "question"
	}
	payload := map[string]any{
		"kind":             kind,
		"body":             body,
		"interrogation_id": interrogationID,
		"turn":             turn,
		"turn_index":       turnIndex,
	}
	payloadArg, err := db.JSONBArg(tx, payload)
	if err != nil {
		return "", err
	}
	now := nowString()
	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, kind, state, target_session_id,
		  payload_json, created_at, updated_at
		)
		VALUES ($1,$2,$3,'agent_message',$4,$5,$6::jsonb,$7,$7)`,
		repositoryID, messageID, interrogation["run_id"], state, targetSessionID,
		payloadArg, now); err != nil {
		return "", err
	}
	return messageID, nil
}

func bumpInterrogationTurn(ctx context.Context, tx db.TxRunner, repositoryID, interrogationID string) error {
	return tx.Exec(ctx, `
		UPDATE striatumd.interrogations
		   SET turn_count = turn_count + 1
		 WHERE repository_id = $1 AND interrogation_id = $2`,
		repositoryID, interrogationID)
}

// interrogationRunID resolves the run_id for an interrogation without taking a
// row lock, so the per-run advisory lock (lockRunInterrogation) can be acquired
// as the FIRST statement in the transaction (RFC 0101 Phase 0a). An unknown
// interrogation_id surfaces the same not_found error lockInterrogation would.
func interrogationRunID(ctx context.Context, runner any, repositoryID, interrogationID string) (string, error) {
	row, err := oneRow(ctx, runner, `
		SELECT run_id FROM striatumd.interrogations
		 WHERE repository_id = $1 AND interrogation_id = $2`, repositoryID, interrogationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", rpc.NewError("not_found", "unknown interrogation_id", nil)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprint(row["run_id"]), nil
}

// sessionRunID resolves the run_id for a session without a row lock, for the
// interrogation.open per-run advisory lock acquisition order.
func sessionRunID(ctx context.Context, runner any, repositoryID, sessionID string) (string, error) {
	row, err := oneRow(ctx, runner, `
		SELECT run_id FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`, repositoryID, sessionID)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(row["run_id"]), nil
}

func lockInterrogation(ctx context.Context, runner any, repositoryID, interrogationID string) (map[string]any, error) {
	row, err := rowByID(ctx, runner, repositoryID, "interrogations", "interrogation_id", interrogationID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rpc.NewError("not_found", "unknown interrogation_id", nil)
	}
	return row, err
}

// requireLiveTarget enforces RFC 0082's live-target requirement: the target
// must be an active, attested session (D026). A closed or unattached session
// fails target_unavailable.
func requireLiveTarget(ctx context.Context, runner any, repositoryID, targetSessionID string) error {
	target, err := rowByID(ctx, runner, repositoryID, "sessions", "session_id", targetSessionID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return rpc.NewError("target_unavailable", "target session does not exist", nil)
	}
	if err != nil {
		return err
	}
	if fmt.Sprint(target["state"]) != "active" {
		return rpc.NewError("target_unavailable", "target session is not live (must be active)", nil)
	}
	checker := lanehealth.Checker{
		Probe: lanehealth.ProdProbe{Runner: supervisionTmuxRunner},
	}
	health, err := checker.Check(ctx, runner, repositoryID, targetSessionID)
	if err == nil && health.LiveTarget() {
		return nil
	}
	// RFC 0084: a wrapper-attested target satisfies D026, but an interrogable
	// agent-loop session that has entered the awaiting_interrogation window
	// (it completed its interrogable job and is still active) is equally a
	// live, identified interrogation target — and it is the only target with
	// the preserved context RFC 0082 interrogation exists to query. Wrapper
	// attestation (D080) drives artifact byline provenance and is intentionally
	// NOT granted here; this widens only interrogation liveness.
	awaiting, err := targetInAwaitingInterrogation(ctx, runner, repositoryID, targetSessionID)
	if err != nil {
		return err
	}
	if awaiting {
		return nil
	}
	return rpc.NewError("target_unavailable", "target session is not attested and is not in the awaiting_interrogation window; interrogation requires a live, attested session or a live interrogable agent-loop target", nil)
}

// targetInAwaitingInterrogation reports whether the target session has entered
// the RFC 0082 §5 awaiting_interrogation context-preservation window (it
// completed an interrogable job). Combined with an active session state in
// requireLiveTarget, this admits live agent-loop targets that are not wrapper
// attested without granting them byline attestation (D080).
func targetInAwaitingInterrogation(ctx context.Context, runner any, repositoryID, targetSessionID string) (bool, error) {
	return existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.events
		 WHERE repository_id = $1 AND actor_session_id = $2
		   AND event_type = 'session.awaiting_interrogation'
		 LIMIT 1`, repositoryID, targetSessionID)
}

func sessionHasCapability(session map[string]any, capability string) bool {
	for _, item := range asList(session["capabilities_json"]) {
		if value, ok := item.(string); ok && value == capability {
			return true
		}
	}
	return false
}

func interrogationSessionError(err error, message string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return rpc.NewError("not_found", message, nil)
	}
	return err
}
