package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func TestReleaseRequeueRefusesRepoWriteWithoutMutating(t *testing.T) {
	tx := &releaseFakeTx{}
	runner := &releaseFakeRunner{tx: tx}
	_, err := HandleReleaseWork(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_release",
		Method:        "work.release",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"message_id":    "msg_1",
			"lease_id":      "lease_1",
			"reason":        "operator retry",
			"requeue":       true,
		},
	})
	if err == nil {
		t.Fatalf("expected release refusal")
	}
	rpcErr, ok := err.(*rpc.Error)
	if !ok || rpcErr.Code != "invalid_transition" {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(rpcErr.Message, "release --requeue is not supported for repo_write jobs") ||
		!strings.Contains(rpcErr.Message, "striatum recovery requeue-stale") {
		t.Fatalf("message = %q", rpcErr.Message)
	}
	if len(tx.execs) != 0 {
		t.Fatalf("unexpected mutations: %#v", tx.execs)
	}
	if tx.committed {
		t.Fatalf("transaction committed")
	}
	if !tx.rolledBack {
		t.Fatalf("transaction did not roll back")
	}
}

func TestCompleteWorkRefusesReviewJobs(t *testing.T) {
	tx := &completeReviewFakeTx{}
	runner := &completeReviewFakeRunner{tx: tx}

	_, err := HandleCompleteWork(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_complete_review",
		Method:        "work.complete",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"job_id":        "job_review",
			"lease_id":      "lease_1",
			"summary":       "done",
		},
	})
	if err == nil {
		t.Fatalf("expected review completion refusal")
	}
	rpcErr, ok := err.(*rpc.Error)
	if !ok || rpcErr.Code != "invalid_transition" {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(rpcErr.Message, "review jobs must use submit-review") {
		t.Fatalf("message = %q", rpcErr.Message)
	}
	if len(tx.execs) != 0 {
		t.Fatalf("unexpected mutations: %#v", tx.execs)
	}
	if tx.committed {
		t.Fatalf("transaction committed")
	}
	if !tx.rolledBack {
		t.Fatalf("transaction did not roll back")
	}
}

type releaseFakeRunner struct {
	tx *releaseFakeTx
}

func (r *releaseFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *releaseFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: errors.New("unexpected runner query outside tx")}
}

func (r *releaseFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected runner scalar query outside tx")
}

func (r *releaseFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}

type releaseFakeTx struct {
	execs      []string
	committed  bool
	rolledBack bool
}

func (tx *releaseFakeTx) Exec(_ context.Context, sql string, _ ...any) error {
	tx.execs = append(tx.execs, sql)
	return nil
}

func (tx *releaseFakeTx) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: errors.New("unexpected row query")}
}

func (tx *releaseFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (tx *releaseFakeTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.queue_messages"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"message_id": "msg_1",
			"job_id":     "job_1",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":           "job_1",
			"run_id":           "run_1",
			"write_scope_json": map[string]any{"mode": "repo_write"},
		}}), nil
	case strings.Contains(sql, "FROM striatumd.leases"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"lease_id":         "lease_1",
			"state":            "active",
			"owner_session_id": "sess_1",
			"resource_id":      "job_1",
			"expires_at":       time.Now().UTC().Add(time.Hour),
		}}), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func (tx *releaseFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *releaseFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

type completeReviewFakeRunner struct {
	tx *completeReviewFakeTx
}

func (r *completeReviewFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *completeReviewFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: errors.New("unexpected runner query outside tx")}
}

func (r *completeReviewFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected runner scalar query outside tx")
}

func (r *completeReviewFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}

type completeReviewFakeTx struct {
	execs      []string
	committed  bool
	rolledBack bool
}

func (tx *completeReviewFakeTx) Exec(_ context.Context, sql string, _ ...any) error {
	tx.execs = append(tx.execs, sql)
	return nil
}

func (tx *completeReviewFakeTx) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: errors.New("unexpected row query")}
}

func (tx *completeReviewFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (tx *completeReviewFakeTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":             "job_review",
			"run_id":             "run_1",
			"job_type":           "review",
			"state":              "running",
			"current_message_id": "msg_1",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.leases"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"lease_id":         "lease_1",
			"state":            "active",
			"owner_session_id": "sess_1",
			"resource_id":      "job_review",
			"expires_at":       time.Now().UTC().Add(time.Hour),
		}}), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func (tx *completeReviewFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *completeReviewFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}
