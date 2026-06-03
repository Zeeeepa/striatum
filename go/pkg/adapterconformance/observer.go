package adapterconformance

import (
	"context"
	"fmt"
	"time"

	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	"github.com/jackc/pgx/v5"
)

// queryer is the read surface the DaemonObserver needs over the daemon
// database. The pgtest db.PgxRunner satisfies it.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// DaemonObserver evaluates conformance clauses by reading daemon state — the
// same database the production stack uses (DESIGN §1.3 observer.go). It reads
// the session protocol columns (via sessionliveness.ProjectionFromRow), the
// active lease row, job state, and the artifacts row for C8. It reads ONLY
// daemon state; it never reads PTY text (D028).
type DaemonObserver struct {
	Runner queryer
}

// SessionSnapshot is the observer's read of a session's protocol-liveness
// columns and its derived liveness projection/classification. The *time fields
// are nil when the corresponding protocol event has not yet fired.
type SessionSnapshot struct {
	SessionID           string
	State               string
	LastToolsListAt     *time.Time
	LastAwaitPacketAt   *time.Time
	LastAckAt           *time.Time
	LastWorkHeartbeatAt *time.Time
	LastWorkCompleteAt  *time.Time
	// Projection is sessionliveness.ProjectionFromRow over the session row +
	// active-lease columns (the same projection production liveness uses).
	Projection map[string]any
	// Classification is sessionliveness.Classify over the same activity.
	Classification sessionliveness.Result
}

// Session reads the session row plus its active-lease columns and derives the
// liveness projection + classification. now anchors the deadline arithmetic so
// callers can pin a deterministic clock.
func (o DaemonObserver) Session(ctx context.Context, repositoryID, sessionID string, now time.Time) (SessionSnapshot, error) {
	row, err := o.oneRow(ctx, `
		SELECT s.session_id,
		       s.state,
		       s.registered_at,
		       s.last_mcp_request_at,
		       s.last_tools_list_at,
		       s.last_await_packet_at,
		       s.last_packet_delivered_at,
		       s.last_ack_at,
		       s.last_work_block_at,
		       s.last_work_release_at,
		       s.last_work_complete_at,
		       s.last_work_heartbeat_at,
		       s.last_session_ready_at,
		       s.last_session_heartbeat_at,
		       s.last_session_question_at,
		       s.last_session_escalate_at,
		       s.last_pty_activity_at,
		       s.last_tool_call_started_at,
		       s.last_tool_call_finished_at,
		       s.liveness_stall_class,
		       s.liveness_stall_since,
		       active_lease.lease_id          AS active_lease_id,
		       active_lease.acquired_at       AS active_lease_acquired_at,
		       active_lease.expires_at        AS active_lease_expires_at,
		       active_lease.last_heartbeat_at AS active_lease_last_heartbeat_at
		  FROM striatumd.sessions s
		  LEFT JOIN LATERAL (
		     SELECT l.lease_id, l.acquired_at, l.expires_at, l.last_heartbeat_at
		       FROM striatumd.leases l
		      WHERE l.repository_id = s.repository_id
		        AND l.owner_session_id = s.session_id
		        AND l.state = 'active'
		      ORDER BY l.acquired_at DESC, l.lease_id DESC
		      LIMIT 1
		  ) active_lease ON true
		 WHERE s.repository_id = $1 AND s.session_id = $2
		 LIMIT 1`, repositoryID, sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	activity := sessionliveness.ActivityFromRow(row)
	snap := SessionSnapshot{
		SessionID:           sessionID,
		State:               fmt.Sprint(row["state"]),
		LastToolsListAt:     activity.LastToolsListAt,
		LastAwaitPacketAt:   activity.LastAwaitPacketAt,
		LastAckAt:           activity.LastAckAt,
		LastWorkHeartbeatAt: activity.LastWorkHeartbeatAt,
		LastWorkCompleteAt:  activity.LastWorkCompleteAt,
		Projection:          sessionliveness.ProjectionFromRow(row, now),
		Classification:      sessionliveness.Classify(activity, sessionliveness.DefaultPolicy(), now),
	}
	return snap, nil
}

// ActiveLease reads the active lease row owned by the session for the job, or
// returns (nil, nil) when none is active.
func (o DaemonObserver) ActiveLease(ctx context.Context, repositoryID, sessionID, jobID string) (map[string]any, error) {
	row, err := o.oneRow(ctx, `
		SELECT lease_id, resource_id, owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND owner_session_id = $2 AND resource_id = $3 AND state = 'active'
		 ORDER BY acquired_at DESC, lease_id DESC
		 LIMIT 1`, repositoryID, sessionID, jobID)
	if err == errNoObserverRow {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

// JobState reads the jobs.state value for a job.
func (o DaemonObserver) JobState(ctx context.Context, repositoryID, jobID string) (string, error) {
	row, err := o.oneRow(ctx, `
		SELECT state FROM striatumd.jobs WHERE repository_id = $1 AND job_id = $2 LIMIT 1`,
		repositoryID, jobID)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(row["state"]), nil
}

// Artifact reads the artifacts row produced by a job for a logical_name (the C8
// signal). Returns (nil, nil) when none exists.
func (o DaemonObserver) Artifact(ctx context.Context, repositoryID, jobID, logicalName string) (map[string]any, error) {
	row, err := o.oneRow(ctx, `
		SELECT artifact_id, artifact_kind, logical_name, repo_path, content_sha256, attempt
		  FROM striatumd.artifacts
		 WHERE repository_id = $1 AND job_id = $2 AND logical_name = $3
		 ORDER BY attempt DESC
		 LIMIT 1`, repositoryID, jobID, logicalName)
	if err == errNoObserverRow {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

// SessionsForRun returns every session row for a run (session_id, state,
// registered_at), oldest first. The RFC 0109 P3 installed-CLI gate uses the
// count as the #95 signal: a seat that re-registers a duplicate session on a
// follow-up turn produces more than the one pre-seeded, attested session.
func (o DaemonObserver) SessionsForRun(ctx context.Context, repositoryID, runID string) ([]map[string]any, error) {
	return o.rows(ctx, `
		SELECT session_id, state, registered_at
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND run_id = $2
		 ORDER BY registered_at, session_id`, repositoryID, runID)
}

// JobLeaseOwner returns the owner_session_id of the most recent lease (any
// state) on a job resource, or "" when no lease ever bound. It identifies which
// session actually drove a job — the per-turn session the #95 gate compares.
func (o DaemonObserver) JobLeaseOwner(ctx context.Context, repositoryID, jobID string) (string, error) {
	row, err := o.oneRow(ctx, `
		SELECT owner_session_id
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND resource_id = $2
		 ORDER BY acquired_at DESC, lease_id DESC
		 LIMIT 1`, repositoryID, jobID)
	if err == errNoObserverRow {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprint(row["owner_session_id"]), nil
}

// InterrogationAnswered reports whether the interrogation has at least one
// completed answer turn (the C7 round-trip signal).
func (o DaemonObserver) InterrogationAnswered(ctx context.Context, repositoryID, interrogationID string) (bool, error) {
	rows, err := o.rows(ctx, `
		SELECT 1
		  FROM striatumd.queue_messages
		 WHERE repository_id = $1
		   AND kind = 'agent_message'
		   AND state = 'completed'
		   AND payload_json->>'interrogation_id' = $2
		   AND payload_json->>'turn' = 'answer'
		 LIMIT 1`, repositoryID, interrogationID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

var errNoObserverRow = pgx.ErrNoRows

func (o DaemonObserver) oneRow(ctx context.Context, sql string, args ...any) (map[string]any, error) {
	rows, err := o.rows(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errNoObserverRow
	}
	return rows[0], nil
}

func (o DaemonObserver) rows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := o.Runner.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToMap)
}
