package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5/pgconn"
)

// sqlStateCheckViolation is SQLSTATE 23514, raised by append_event_row when a
// payload trips the durable-event transcript exclusion (RFC 0110 §12,
// C-EVENT-NO-TRANSCRIPTS). It is distinct from the events FK code (23503), so the
// daemon can tell a rejected payload from a missing parent row.
const sqlStateCheckViolation = "23514"

// EventRow is the content of one striatumd.events append routed through the
// owner-owned SECURITY DEFINER write path (RFC 0110 §7 P2). The nullable foreign
// keys are typed any so callers pass their already null-coerced values straight
// through, keeping the SD path's stored row identical to the historical direct
// INSERT. event_id, created_at, previous_hash, and row_hash are assigned in-DB by
// the SD function (it owns the chain), so they are not fields here.
type EventRow struct {
	RepositoryID   string
	RunID          any
	EventType      string
	ActorSessionID any
	JobID          any
	MessageID      any
	ArtifactID     any
	LeaseID        any
	Payload        map[string]any
}

// AppendEventRowSD is the P2 (PhaseFull) event-append path: it routes through the
// owner-owned SECURITY DEFINER striatumd.append_event_row, which asserts daemon
// authority, enforces durable-event transcript exclusion, locks the per-repository
// chain head, computes the v3 chain hash in-DB, appends the event, and advances
// the chain head — all atomic with the caller's mutation transaction (event
// appends ride the mutation tx, so the event commits or rolls back with the state
// transition that emitted it). It returns the event_id the function assigned.
//
// The event chain hash is computed entirely in-DB with no Go counterpart: unlike
// the audit chain, nothing in Go recomputes an event row hash, so an in-DB-only
// v3 hash satisfies G1 (the only write path computes the hash and holds the chain
// lock in-DB) with no Go/PL-pgSQL porting hazard. The caller passes already
// null-coerced foreign keys so the stored row matches the pre-P2 direct INSERT.
//
// runner is the caller's transaction (the production *PgxTxRunner supports
// QueryRow). A lost/again-rotated authority surfaces as a structured
// daemon_auth_lost (RFC 0110 §4.5); a transcript-shaped payload surfaces as
// event_payload_rejected (§12) — both before any row lands.
func AppendEventRowSD(ctx context.Context, runner any, row EventRow) (int64, error) {
	rower, ok := runner.(interface {
		QueryRow(context.Context, string, ...any) Row
	})
	if !ok {
		return 0, fmt.Errorf("runner does not support query row")
	}
	payload := row.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payloadArg, err := JSONBArg(runner, payload)
	if err != nil {
		return 0, err
	}
	var eventID int64
	err = rower.QueryRow(ctx,
		`SELECT striatumd.append_event_row($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`,
		row.RepositoryID, row.RunID, row.EventType, row.ActorSessionID, row.JobID,
		row.MessageID, row.ArtifactID, row.LeaseID, payloadArg,
	).Scan(&eventID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case sqlStateInvalidAuthorization:
				return 0, rpc.NewError(
					"daemon_auth_lost",
					"daemon authority is no longer valid (registry row missing/superseded — restart the daemon to re-bootstrap, or check for a concurrent rotator)",
					map[string]any{"sqlstate": sqlStateInvalidAuthorization},
				)
			case sqlStateCheckViolation:
				return 0, rpc.NewError(
					"event_payload_rejected",
					pgErr.Message,
					map[string]any{"sqlstate": sqlStateCheckViolation},
				)
			}
		}
		return 0, fmt.Errorf("append_event_row (sd): %w", err)
	}
	return eventID, nil
}
