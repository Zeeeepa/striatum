package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

type terminalCleanupExec struct {
	sql  string
	args []any
}

type terminalCleanupTx struct {
	execs                  []terminalCleanupExec
	nextEvent              int64
	queries                []string
	committed              bool
	rolledBack             bool
	suppressActiveSessions bool             // #417: model a run with no active sessions left
	orphanSessionIDs       []string         // #417: sessions still owning a live supervisor
	openBlockers           []map[string]any // #420: blockers still open on the terminal run
}

func (tx *terminalCleanupTx) Exec(ctx context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, terminalCleanupExec{sql: sql, args: append([]any{}, args...)})
	return nil
}

func (tx *terminalCleanupTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.queries = append(tx.queries, sql)
	switch {
	case strings.Contains(sql, "FROM striatumd.sessions") && strings.Contains(sql, "state = 'active'"):
		if tx.suppressActiveSessions {
			return runPrepareRowsFromMaps(nil), nil
		}
		return runPrepareRowsFromMaps([]map[string]any{{
			"session_id": "sess_1",
			"role_id":    "implementer",
			"lane_id":    "claude",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.leases"):
		return runPrepareRowsFromMaps(nil), nil
	case strings.Contains(sql, "SELECT DISTINCT") && strings.Contains(sql, "FROM striatumd.process_supervisors"):
		// #417 backstop: sessions (regardless of their own state) that still own a
		// live supervisor for this terminal run.
		rows := []map[string]any{}
		for _, sid := range tx.orphanSessionIDs {
			rows = append(rows, map[string]any{"session_id": sid})
		}
		return runPrepareRowsFromMaps(rows), nil
	case strings.Contains(sql, "UPDATE striatumd.blockers") && strings.Contains(sql, "RETURNING"):
		// #420: the still-open blockers this terminal run resolves.
		return runPrepareRowsFromMaps(tx.openBlockers), nil
	case strings.Contains(sql, "FROM striatumd.process_supervisors ps") && strings.Contains(sql, "FOR UPDATE OF ps"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"supervisor_id":         "sup_1",
			"pid":                   nil,
			"pid_start_time":        "",
			"stdin_pipe_path":       "",
			"daemon_supervisor_id":  "dsup_1",
			"pointer_metadata_json": map[string]any{},
		}}), nil
	default:
		return runPrepareRowsFromMaps(nil), nil
	}
}

func (tx *terminalCleanupTx) QueryRow(ctx context.Context, sql string, args ...any) db.Row {
	switch {
	case strings.Contains(sql, "repo_event_chain_heads"):
		return terminalCleanupRow{}
	case strings.Contains(sql, "nextval"):
		tx.nextEvent++
		return terminalCleanupRow{eventID: tx.nextEvent}
	case strings.Contains(sql, "ps.cwd, ps.scratch_path"):
		return terminalCleanupRow{cwd: "", scratchPath: "", runAsUser: ""}
	default:
		return terminalCleanupRow{err: pgx.ErrNoRows}
	}
}

func (tx *terminalCleanupTx) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	return "", nil
}

func (tx *terminalCleanupTx) Commit(ctx context.Context) error {
	tx.committed = true
	return nil
}

func (tx *terminalCleanupTx) Rollback(ctx context.Context) error {
	tx.rolledBack = true
	return nil
}

type terminalCleanupRow struct {
	eventID     int64
	cwd         string
	scratchPath string
	runAsUser   string
	err         error
}

func (r terminalCleanupRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	stringIndex := 0
	for _, target := range dest {
		switch typed := target.(type) {
		case *int64:
			*typed = r.eventID
		case *string:
			switch stringIndex {
			case 0:
				*typed = r.cwd
			case 1:
				*typed = r.scratchPath
			default:
				*typed = r.runAsUser
			}
			stringIndex++
		case **string:
			*typed = nil
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func TestCloseRemainingSessionsStopsAttachedSupervisors(t *testing.T) {
	tx := &terminalCleanupTx{}
	if err := closeRemainingSessions(context.Background(), tx, "repo_1", "run_1", "run_completed", "run_completed"); err != nil {
		t.Fatalf("closeRemainingSessions: %v", err)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.process_supervisors", "stopped") {
		t.Fatalf("expected process_supervisors to be stopped, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.process_supervisor_pointers", "stopped") {
		t.Fatalf("expected process_supervisor_pointers to be stopped, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.daemon_supervisors", "stopped") {
		t.Fatalf("expected daemon_supervisors to be stopped, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.sessions", "run_completed") {
		t.Fatalf("expected session close reason, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "INSERT INTO striatumd.events", "supervisor.stopped") {
		t.Fatalf("expected supervisor.stopped event, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "INSERT INTO striatumd.events", "session.closed") {
		t.Fatalf("expected session.closed event, execs=%#v", tx.execs)
	}
}

// TestCloseRemainingSessionsReapsOrphanSupervisorOfClosedSession is the #417
// regression: 511 of 562 stranded supervisors in the incident belonged to a
// session that was ALREADY closed when the run terminated, so the active-session
// loop never reached them and they lingered 'attached', storming the status
// sudo+tmux probe path. Here there are no active sessions at all, yet a supervisor
// is still live for the terminal run; the run-scoped backstop must reap it.
func TestCloseRemainingSessionsReapsOrphanSupervisorOfClosedSession(t *testing.T) {
	tx := &terminalCleanupTx{
		suppressActiveSessions: true,
		orphanSessionIDs:       []string{"sess_orphan"},
	}
	if err := closeRemainingSessions(context.Background(), tx, "repo_1", "run_1", "run_completed", "run_completed"); err != nil {
		t.Fatalf("closeRemainingSessions: %v", err)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.process_supervisors", "stopped") {
		t.Fatalf("expected orphan process_supervisors reaped to stopped, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.process_supervisor_pointers", "stopped") {
		t.Fatalf("expected orphan process_supervisor_pointers reaped to stopped, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.daemon_supervisors", "stopped") {
		t.Fatalf("expected orphan daemon_supervisors reaped to stopped, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "INSERT INTO striatumd.events", "supervisor.stopped") {
		t.Fatalf("expected supervisor.stopped event for orphan, execs=%#v", tx.execs)
	}
}

// TestCloseRemainingSessionsResolvesTerminalRunOpenBlockers is the #420 forward
// fix: when a run reaches a terminal state its still-open blockers are resolved —
// ALL kinds, including human_checkpoint / escalation-class (whose adjudication
// obligation is moot once the run is dead, unlike the LIVE-run recovery.resolve_blocker
// path that refuses them) — with the escalation_inbox mirror kept consistent.
func TestCloseRemainingSessionsResolvesTerminalRunOpenBlockers(t *testing.T) {
	tx := &terminalCleanupTx{
		suppressActiveSessions: true, // no sessions/supervisors to reap; isolate the blocker path
		openBlockers: []map[string]any{
			{"blocker_id": "blk_chk", "severity": "human_checkpoint", "blocker_kind": "revision_routing", "job_id": nil},
			{"blocker_id": "blk_plain", "severity": "blocked", "blocker_kind": "missing_input", "job_id": "job_1"},
		},
	}
	if err := closeRemainingSessions(context.Background(), tx, "repo_1", "run_1", "run_canceled", "run_canceled"); err != nil {
		t.Fatalf("closeRemainingSessions: %v", err)
	}
	if !sawExecWithArg(tx.execs, "UPDATE striatumd.escalation_inbox", "blk_chk") {
		t.Fatalf("expected escalation_inbox mirror resolved for terminal-run blockers, execs=%#v", tx.execs)
	}
	if !sawExecWithArg(tx.execs, "INSERT INTO striatumd.events", "blocker.resolved") {
		t.Fatalf("expected a blocker.resolved provenance event, execs=%#v", tx.execs)
	}
	// The blockers UPDATE ... RETURNING ran (it drives the loop above).
	sawBlockerUpdate := false
	for _, q := range tx.queries {
		if strings.Contains(q, "UPDATE striatumd.blockers") && strings.Contains(q, "state = 'resolved'") {
			sawBlockerUpdate = true
		}
	}
	if !sawBlockerUpdate {
		t.Fatalf("expected the run-scoped blockers resolve UPDATE, queries=%#v", tx.queries)
	}
}

func sawExecWithArg(execs []terminalCleanupExec, sqlPart string, arg any) bool {
	for _, exec := range execs {
		if !strings.Contains(exec.sql, sqlPart) {
			continue
		}
		for _, item := range exec.args {
			if item == arg {
				return true
			}
		}
	}
	return false
}
