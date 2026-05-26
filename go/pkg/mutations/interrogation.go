package mutations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
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
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
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
		runID := fmt.Sprint(interrogator["run_id"])
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
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
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
			"interrogation_id": interrogationID,
			"message_id":       messageID,
			"turn_index":       turnIndex,
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
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
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
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
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

// maybeCloseInterrogationTarget closes a target session that finished its
// interrogable work and has no remaining open interrogations or active lease.
func maybeCloseInterrogationTarget(ctx context.Context, tx db.TxRunner, repositoryID, runID, targetSessionID string) (bool, error) {
	target, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", targetSessionID, true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if fmt.Sprint(target["state"]) != "active" {
		return false, nil
	}
	openOthers, err := existsRow(ctx, tx, `
		SELECT 1 FROM striatumd.interrogations
		 WHERE repository_id = $1 AND target_session_id = $2 AND state = 'open'
		 LIMIT 1`, repositoryID, targetSessionID)
	if err != nil {
		return false, err
	}
	if openOthers {
		return false, nil
	}
	activeLease, err := existsRow(ctx, tx, `
		SELECT 1 FROM striatumd.leases
		 WHERE repository_id = $1 AND owner_session_id = $2 AND state = 'active'
		 LIMIT 1`, repositoryID, targetSessionID)
	if err != nil {
		return false, err
	}
	if activeLease {
		return false, nil
	}
	now := nowString()
	reason := "interrogation_window_closed"
	if err := tx.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET state = 'closed', closed_at = $1, close_reason = $2
		 WHERE repository_id = $3 AND session_id = $4`,
		now, reason, repositoryID, targetSessionID); err != nil {
		return false, err
	}
	if _, err := appendEvent(ctx, tx, repositoryID, runID, "session.closed", targetSessionID, nil, nil, nil, nil, map[string]any{
		"session_id": targetSessionID,
		"role_id":    target["role_id"],
		"lane_id":    target["lane_id"],
		"reason":     reason,
		"source":     "interrogation_close",
	}); err != nil {
		return false, err
	}
	return true, nil
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
	attestation := sessionLaneAttestation(ctx, runner, repositoryID, targetSessionID)
	if attested, _ := attestation["attested"].(bool); attested {
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
