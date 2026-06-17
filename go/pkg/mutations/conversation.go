package mutations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

// RFC 0086: multi-party conversation. A symmetric N-party live conversation —
// N participant sessions, a topic, a round-robin floor, and a turn-ordered
// shared transcript. The 1->1 asymmetric special case is RFC 0082 interrogation.
//
// Lifecycle + floor state live in striatumd.conversations (migration 0017).
// Turns reuse the message bus (queue_messages, kind='agent_message',
// payload_json.conversation_id), exactly like interrogation. Two payload roles:
//   - role='turn': a completed transcript entry (state 'acked'); author_session_id
//     + body + turn_index. These ARE the transcript.
//   - role='your_turn': a pending delivery addressed to the current floor-holder
//     (target_session_id set, state 'pending'); the floor-holder's await loop
//     receives it as a `conversation_message` envelope. Acked on delivery.
// The daemon is the sole writer and appends events for each transition.

// HandleConversationOpen opens an N-party conversation among active participant
// sessions sharing a run. Round-robin floor starts at participants[0], which
// receives the opening turn.
func HandleConversationOpen(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	participants := stringSliceParam(envelope, "participant_session_ids", "participants")
	topic := strings.TrimSpace(stringParam(envelope, "topic"))
	maxRounds := intParam(envelope, "max_rounds", 3)
	if len(participants) < 2 {
		return nil, rpc.NewError("schema_invalid", "conversation.open requires participant_session_ids with at least 2 sessions", nil)
	}
	if maxRounds < 1 {
		return nil, rpc.NewError("schema_invalid", "conversation.open requires max_rounds >= 1", nil)
	}
	if dup := firstDuplicate(participants); dup != "" {
		return nil, rpc.NewError("invalid_transition", fmt.Sprintf("participant %s listed more than once", dup), nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		var runID string
		for _, sid := range participants {
			s, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", sid, true)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, rpc.NewError("not_found", fmt.Sprintf("participant session %s not found", sid), nil)
			}
			if err != nil {
				return nil, err
			}
			if fmt.Sprint(s["state"]) != "active" {
				return nil, rpc.NewError("invalid_transition", fmt.Sprintf("participant session %s is not active", sid), nil)
			}
			r := fmt.Sprint(s["run_id"])
			if runID == "" {
				runID = r
			} else if r != runID {
				return nil, rpc.NewError("invalid_transition", "conversation is single-run scoped: all participants must share a run", nil)
			}
		}
		conversationID, err := newID("conv")
		if err != nil {
			return nil, err
		}
		now := nowString()
		participantsArg, err := db.JSONBArg(tx, participants)
		if err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.conversations (
			  repository_id, run_id, conversation_id, topic, participants_json,
			  floor_index, round_count, max_rounds, turn_count, state, opened_at
			)
			VALUES ($1,$2,$3,$4,$5::jsonb,0,0,$6,0,'open',$7)`,
			repositoryID, runID, conversationID, nullable(topic), participantsArg,
			maxRounds, now); err != nil {
			return nil, err
		}
		// No delivery message: participants[0] holds the floor, so its await
		// loop derives its turn from floor_index (deliverPendingConversationTurn).
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "conversation.opened", participants[0], nil, nil, nil, nil, map[string]any{
			"conversation_id": conversationID,
			"participants":    participants,
			"topic":           nullable(topic),
			"max_rounds":      maxRounds,
		}); err != nil {
			return nil, err
		}
		recordWake(tx, WakeEvent{
			RepositoryID:   repositoryID,
			RunID:          runID,
			Kind:           "conversation_turn_available",
			ConversationID: conversationID,
		})
		return map[string]any{
			"conversation_id": conversationID,
			"run_id":          runID,
			"participants":    participants,
			"floor":           participants[0],
			"state":           "open",
		}, nil
	})
}

// HandleConversationSay records the floor-holder's turn, advances the round-robin
// floor, and delivers the turn to the next participant. Auto-closes at max_rounds.
func HandleConversationSay(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	conversationID := stringParam(envelope, "conversation_id")
	body := strings.TrimSpace(stringParam(envelope, "body"))
	if sessionID == "" || conversationID == "" || body == "" {
		return nil, rpc.NewError("schema_invalid", "conversation.say requires session_id, conversation_id, and body", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		conv, err := lockConversation(ctx, tx, repositoryID, conversationID)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(conv["state"]) != "open" {
			return nil, rpc.NewError("invalid_transition", "conversation is closed", nil)
		}
		participants := conversationParticipants(conv)
		floor := intValue(conv["floor_index"])
		if floor < 0 || floor >= len(participants) {
			return nil, rpc.NewError("invalid_transition", "conversation floor is out of range", nil)
		}
		if participants[floor] != sessionID {
			return nil, rpc.NewError("capability_denied", fmt.Sprintf("not your turn: the floor belongs to %s", participants[floor]), nil)
		}
		runID := fmt.Sprint(conv["run_id"])
		turnIndex := intValue(conv["turn_count"])
		// Record the completed turn (transcript entry).
		if _, err := conversationTurnRecord(ctx, tx, repositoryID, runID, conversationID, sessionID, body, turnIndex); err != nil {
			return nil, err
		}
		// Advance the round-robin floor.
		nextFloor := (floor + 1) % len(participants)
		roundCount := intValue(conv["round_count"])
		if nextFloor == 0 {
			roundCount++
		}
		maxRounds := intValue(conv["max_rounds"])
		closing := roundCount >= maxRounds
		now := nowString()
		if closing {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.conversations
				   SET turn_count = turn_count + 1, round_count = $1, state = 'closed', closed_at = $2
				 WHERE repository_id = $3 AND conversation_id = $4`,
				roundCount, now, repositoryID, conversationID); err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, tx, repositoryID, runID, "conversation.closed", sessionID, nil, nil, nil, nil, map[string]any{
				"conversation_id": conversationID, "reason": "max_rounds_reached", "rounds": roundCount,
			}); err != nil {
				return nil, err
			}
		} else {
			// Advance the floor; the next participant's await loop derives its
			// turn from floor_index (no delivery message needed).
			if err := tx.Exec(ctx, `
				UPDATE striatumd.conversations
				   SET turn_count = turn_count + 1, floor_index = $1, round_count = $2
				 WHERE repository_id = $3 AND conversation_id = $4`,
				nextFloor, roundCount, repositoryID, conversationID); err != nil {
				return nil, err
			}
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "conversation.said", sessionID, nil, nil, nil, nil, map[string]any{
			"conversation_id": conversationID, "turn_index": turnIndex,
		}); err != nil {
			return nil, err
		}
		if !closing {
			recordWake(tx, WakeEvent{
				RepositoryID:   repositoryID,
				RunID:          runID,
				Kind:           "conversation_turn_available",
				ConversationID: conversationID,
			})
		}
		result := map[string]any{
			"conversation_id": conversationID,
			"turn_index":      turnIndex,
			"round_count":     roundCount,
			"state":           map[bool]string{true: "closed", false: "open"}[closing],
		}
		if !closing {
			result["next_floor"] = participants[nextFloor]
		}
		return result, nil
	})
}

// HandleConversationClose ends a conversation early. Any participant may close.
func HandleConversationClose(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	conversationID := stringParam(envelope, "conversation_id")
	if sessionID == "" || conversationID == "" {
		return nil, rpc.NewError("schema_invalid", "conversation.close requires session_id and conversation_id", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		conv, err := lockConversation(ctx, tx, repositoryID, conversationID)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(conv["state"]) == "closed" {
			return nil, rpc.NewError("invalid_transition", "conversation is already closed", nil)
		}
		if !containsString(conversationParticipants(conv), sessionID) {
			return nil, rpc.NewError("capability_denied", "only a participant may close the conversation", nil)
		}
		now := nowString()
		if err := tx.Exec(ctx, `
			UPDATE striatumd.conversations
			   SET state = 'closed', closed_at = $1
			 WHERE repository_id = $2 AND conversation_id = $3`,
			now, repositoryID, conversationID); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, conv["run_id"], "conversation.closed", sessionID, nil, nil, nil, nil, map[string]any{
			"conversation_id": conversationID, "reason": "closed_by_participant",
		}); err != nil {
			return nil, err
		}
		return map[string]any{"conversation_id": conversationID, "state": "closed"}, nil
	})
}

// conversationTurnRecord writes a completed transcript turn onto the message bus
// (D028: authored body + correlation only). state 'acked' = historical record.
func conversationTurnRecord(ctx context.Context, tx db.TxRunner, repositoryID, runID, conversationID, authorSessionID, body string, turnIndex int) (string, error) {
	messageID, err := newID("msg")
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"kind":              "conversation_turn",
		"role":              "turn",
		"conversation_id":   conversationID,
		"author_session_id": authorSessionID,
		"body":              body,
		"turn_index":        turnIndex,
	}
	payloadArg, err := db.JSONBArg(tx, payload)
	if err != nil {
		return "", err
	}
	now := nowString()
	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, kind, state, target_session_id,
		  payload_json, created_at, updated_at, acked_at
		)
		VALUES ($1,$2,$3,'agent_message','acked',$4,$5::jsonb,$6,$6,$6)`,
		repositoryID, messageID, runID, authorSessionID, payloadArg, now); err != nil {
		return "", err
	}
	return messageID, nil
}

// deliverPendingConversationTurn returns a conversation_message for the awaiting
// session iff it currently holds the floor of an open conversation. The signal
// is derived from durable floor state (conversations.floor_index), not a
// consumable message — so it is IDEMPOTENT and crash-safe: a floor-holder that
// errors or restarts before calling conversation.say simply sees its turn again,
// rather than the turn being lost and the round-robin stalling. Read-only.
func deliverPendingConversationTurn(ctx context.Context, runner db.Runner, repositoryID, sessionID string) (map[string]any, error) {
	// Hot-path gate: this runs on every await-claim poll, but an open
	// conversation is rare. A single EXISTS probe (index-only on
	// idx_conversations_state, short-circuits at the first row) lets the
	// overwhelmingly common "no open conversation" case skip the row scan +
	// participant-slice work below.
	var hasOpen bool
	if err := runner.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM striatumd.conversations
		   WHERE repository_id = $1 AND state = 'open'
		)`, repositoryID).Scan(&hasOpen); err != nil {
		return nil, err
	}
	if !hasOpen {
		return nil, nil
	}

	rows, err := queryRows(ctx, runner, `
		SELECT conversation_id, topic, participants_json, floor_index, turn_count
		  FROM striatumd.conversations
		 WHERE repository_id = $1 AND state = 'open'
		 ORDER BY opened_at ASC`, repositoryID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		participants := asStringSlice(row["participants_json"])
		floor := intValue(row["floor_index"])
		if floor < 0 || floor >= len(participants) || participants[floor] != sessionID {
			continue
		}
		conversationID := fmt.Sprint(row["conversation_id"])
		transcript, err := conversationTranscript(ctx, runner, repositoryID, conversationID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type":            "conversation_message",
			"status":          "conversation_message",
			"conversation_id": conversationID,
			"topic":           row["topic"],
			"participants":    participants,
			"your_turn":       true,
			"turn_index":      intValue(row["turn_count"]),
			"transcript":      transcript,
		}, nil
	}
	return nil, nil
}

// conversationTranscript returns the ordered turn records for a conversation.
func conversationTranscript(ctx context.Context, runner any, repositoryID, conversationID string) ([]map[string]any, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT payload_json
		  FROM striatumd.queue_messages
		 WHERE repository_id = $1
		   AND kind = 'agent_message'
		   AND payload_json->>'role' = 'turn'
		   AND payload_json->>'conversation_id' = $2
		 ORDER BY (payload_json->>'turn_index')::int ASC`,
		repositoryID, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		p := asMap(row["payload_json"])
		out = append(out, map[string]any{
			"turn_index":        p["turn_index"],
			"author_session_id": p["author_session_id"],
			"body":              p["body"],
		})
	}
	return out, nil
}

// HandleConversationShow returns a conversation's metadata + ordered transcript.
func HandleConversationShow(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	conversationID := stringParam(envelope, "conversation_id")
	if conversationID == "" {
		return nil, rpc.NewError("schema_invalid", "conversation.show requires conversation_id", nil)
	}
	conv, err := rowByID(ctx, runner, repositoryID, "conversations", "conversation_id", conversationID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rpc.NewError("not_found", "unknown conversation_id", nil)
	}
	if err != nil {
		return nil, err
	}
	transcript, err := conversationTranscript(ctx, runner, repositoryID, conversationID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"conversation": map[string]any{
			"conversation_id": conversationID,
			"run_id":          conv["run_id"],
			"topic":           conv["topic"],
			"participants":    conversationParticipants(conv),
			"state":           conv["state"],
			"floor_index":     conv["floor_index"],
			"round_count":     conv["round_count"],
			"max_rounds":      conv["max_rounds"],
			"turn_count":      conv["turn_count"],
			"opened_at":       conv["opened_at"],
			"closed_at":       conv["closed_at"],
		},
		"turns":      transcript,
		"turn_count": len(transcript),
	}, nil
}

// HandleConversationList lists conversations for a run.
func HandleConversationList(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "conversation.list requires run_id", nil)
	}
	rows, err := queryRows(ctx, runner, `
		SELECT conversation_id, topic, participants_json, state, round_count,
		       max_rounds, turn_count, opened_at, closed_at
		  FROM striatumd.conversations
		 WHERE repository_id = $1 AND run_id = $2
		 ORDER BY opened_at ASC`,
		repositoryID, runID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"conversation_id": row["conversation_id"],
			"topic":           row["topic"],
			"participants":    asStringSlice(row["participants_json"]),
			"state":           row["state"],
			"round_count":     row["round_count"],
			"max_rounds":      row["max_rounds"],
			"turn_count":      row["turn_count"],
			"opened_at":       row["opened_at"],
			"closed_at":       row["closed_at"],
		})
	}
	return map[string]any{"run_id": runID, "count": len(items), "items": items}, nil
}

func lockConversation(ctx context.Context, runner any, repositoryID, conversationID string) (map[string]any, error) {
	row, err := rowByID(ctx, runner, repositoryID, "conversations", "conversation_id", conversationID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rpc.NewError("not_found", "unknown conversation_id", nil)
	}
	return row, err
}

func conversationParticipants(conv map[string]any) []string {
	return asStringSlice(conv["participants_json"])
}

// asStringSlice coerces a jsonb array (or []any) into a []string.
func asStringSlice(value any) []string {
	out := []string{}
	for _, item := range asList(value) {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func firstDuplicate(items []string) string {
	seen := map[string]struct{}{}
	for _, it := range items {
		if _, ok := seen[it]; ok {
			return it
		}
		seen[it] = struct{}{}
	}
	return ""
}

func containsString(items []string, target string) bool {
	for _, it := range items {
		if it == target {
			return true
		}
	}
	return false
}
