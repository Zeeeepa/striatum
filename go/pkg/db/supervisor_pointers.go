// Package db: Postgres-backed supervisor.PointerStore (RFC 0039 V1.6 F-store).
//
// The Python supervisor writes the same striatumd.process_supervisor_pointers
// table; this implementation matches column shapes byte-for-byte so the two
// supervisor cores can interoperate against one schema (RFC 0043 §3).

package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
	if row.SessionID == "" {
		return fmt.Errorf("supervisor_pointer: empty session_id")
	}
	var runID string
	err := s.pool.QueryRow(ctx, `
		SELECT run_id FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`,
		row.RepositoryID, row.SessionID,
	).Scan(&runID)
	if err != nil {
		return fmt.Errorf("supervisor_pointer lookup session run: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"started_at":        row.StartedAt.UTC().Format(time.RFC3339Nano),
		"last_heartbeat_at": row.LastHeartbeatAt.UTC().Format(time.RFC3339Nano),
		"stdin_pipe_path":   row.StdinPipePath,
		"lost_reason":       row.LostReason,
	})
	if err != nil {
		return fmt.Errorf("supervisor_pointer metadata: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers
		  (repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		   pid, state, updated_at, metadata_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		ON CONFLICT (repository_id, supervisor_id) DO UPDATE SET
		  daemon_supervisor_id = EXCLUDED.daemon_supervisor_id,
		  run_id               = EXCLUDED.run_id,
		  session_id           = EXCLUDED.session_id,
		  pid                  = EXCLUDED.pid,
		  state                = EXCLUDED.state,
		  updated_at           = EXCLUDED.updated_at,
		  metadata_json        = EXCLUDED.metadata_json
	`,
		row.RepositoryID,
		row.SupervisorID,
		row.SupervisorID,
		runID,
		row.SessionID,
		row.PID,
		row.State,
		row.LastHeartbeatAt,
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("supervisor_pointer upsert: %w", err)
	}
	return nil
}

func (s *SupervisorPointerStore) MarkSupervisorLost(ctx context.Context, supervisorID string, reason string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET state = 'lost',
		       updated_at = now(),
		       metadata_json = metadata_json || jsonb_build_object('lost_reason', $2)
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
	var metadataRaw []byte
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT supervisor_id, repository_id, session_id, COALESCE(pid, 0),
		       updated_at, state, metadata_json
		  FROM striatumd.process_supervisor_pointers
		 WHERE supervisor_id = $1
	`, supervisorID).Scan(
		&row.SupervisorID,
		&row.RepositoryID,
		&row.SessionID,
		&row.PID,
		&updatedAt,
		&row.State,
		&metadataRaw,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PointerRow{}, ErrSupervisorNotFound
		}
		return PointerRow{}, ErrSupervisorNotFound
	}
	row.LastHeartbeatAt = updatedAt
	metadata := map[string]any{}
	_ = json.Unmarshal(metadataRaw, &metadata)
	if startedAt, ok := metadata["started_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, startedAt); err == nil {
			row.StartedAt = parsed
		}
	}
	if heartbeat, ok := metadata["last_heartbeat_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, heartbeat); err == nil {
			row.LastHeartbeatAt = parsed
		}
	}
	if stdinPipePath, ok := metadata["stdin_pipe_path"].(string); ok {
		row.StdinPipePath = stdinPipePath
	}
	if lostReason, ok := metadata["lost_reason"].(string); ok {
		row.LostReason = lostReason
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
