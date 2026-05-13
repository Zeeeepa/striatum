// Package db: Postgres-backed supervisor.PointerStore (RFC 0039 V1.6 F-store).
//
// The Python supervisor writes the same striatumd.process_supervisor_pointers
// table; this implementation matches column shapes byte-for-byte so the two
// supervisor cores can interoperate against one schema (RFC 0043 §3).

package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SupervisorPointerStore implements supervisor.PointerStore against an
// existing pgxpool.Pool. Wire it into cmd/striatumd/main.go's boot path:
//
//	store := db.NewSupervisorPointerStore(pool)
//	sup := supervisor.NewLiveness(cfg, store, supervisorID, pid)
//
// Returns a typed not-found error so liveness can distinguish "row gone"
// from a real DB outage.
type SupervisorPointerStore struct {
	pool *pgxpool.Pool
}

// ErrSupervisorNotFound is returned by Get when the supervisor_id has no
// row. Liveness treats this as a hard signal: the daemon has already
// reaped the supervisor record, so further heartbeats are dropped.
var ErrSupervisorNotFound = errors.New("supervisor pointer not found")

func NewSupervisorPointerStore(pool *pgxpool.Pool) *SupervisorPointerStore {
	return &SupervisorPointerStore{pool: pool}
}

// PointerRow is the locally-defined row shape; mirrors supervisor.PointerRow
// to keep this package free of the supervisor import (avoids cycle with
// go/pkg/supervisor importing go/pkg/db for the concrete store).
type PointerRow struct {
	SupervisorID    string
	RepositoryID    string
	SessionID       string
	PID             int
	StartedAt       time.Time
	LastHeartbeatAt time.Time
	StdinPipePath   string
	State           string
	LostReason      string
}

func (s *SupervisorPointerStore) UpsertSupervisorPointer(ctx context.Context, row PointerRow) error {
	if row.SupervisorID == "" {
		return fmt.Errorf("supervisor_pointer: empty supervisor_id")
	}
	if row.RepositoryID == "" {
		return fmt.Errorf("supervisor_pointer: empty repository_id")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers
		  (supervisor_id, repository_id, session_id, pid, started_at,
		   last_heartbeat_at, stdin_pipe_path, state, lost_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (supervisor_id) DO UPDATE SET
		  pid               = EXCLUDED.pid,
		  started_at        = EXCLUDED.started_at,
		  last_heartbeat_at = EXCLUDED.last_heartbeat_at,
		  stdin_pipe_path   = EXCLUDED.stdin_pipe_path,
		  state             = EXCLUDED.state,
		  lost_reason       = EXCLUDED.lost_reason
	`,
		row.SupervisorID,
		row.RepositoryID,
		nullable(row.SessionID),
		row.PID,
		row.StartedAt,
		row.LastHeartbeatAt,
		nullable(row.StdinPipePath),
		row.State,
		nullable(row.LostReason),
	)
	if err != nil {
		return fmt.Errorf("supervisor_pointer upsert: %w", err)
	}
	return nil
}

func (s *SupervisorPointerStore) MarkSupervisorLost(ctx context.Context, supervisorID string, reason string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET state = 'lost', lost_reason = $2
		 WHERE supervisor_id = $1
	`, supervisorID, reason)
	if err != nil {
		return fmt.Errorf("supervisor_pointer mark_lost: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSupervisorNotFound
	}
	return nil
}

func (s *SupervisorPointerStore) GetSupervisorPointer(ctx context.Context, supervisorID string) (PointerRow, error) {
	var row PointerRow
	var sessionID, stdinPipePath, lostReason *string
	err := s.pool.QueryRow(ctx, `
		SELECT supervisor_id, repository_id, session_id, pid, started_at,
		       last_heartbeat_at, stdin_pipe_path, state, lost_reason
		  FROM striatumd.process_supervisor_pointers
		 WHERE supervisor_id = $1
	`, supervisorID).Scan(
		&row.SupervisorID,
		&row.RepositoryID,
		&sessionID,
		&row.PID,
		&row.StartedAt,
		&row.LastHeartbeatAt,
		&stdinPipePath,
		&row.State,
		&lostReason,
	)
	if err != nil {
		return PointerRow{}, ErrSupervisorNotFound
	}
	if sessionID != nil {
		row.SessionID = *sessionID
	}
	if stdinPipePath != nil {
		row.StdinPipePath = *stdinPipePath
	}
	if lostReason != nil {
		row.LostReason = *lostReason
	}
	return row, nil
}

// nullable returns nil for the empty string, the value otherwise. Postgres
// columns for optional fields are nullable; this keeps NULL semantics out
// of Go's zero value.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
