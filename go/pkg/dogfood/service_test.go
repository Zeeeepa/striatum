package dogfood

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func TestPublishOnBehalfRequiresReasonBeforeRetirement(t *testing.T) {
	_, err := (Service{}).HandlePublishOnBehalf(context.Background(), envelope(map[string]any{
		"repository_id": "repo_1",
		"session_id":    "sess_1",
		"artifact_path": "docs/result.md",
		"artifact_kind": "finding",
		"logical_name":  "result",
		"reason":        "   ",
	}))
	assertRPCError(t, err, "reason_required")
}

func TestPublishOnBehalfValidShapeFailsClosedWithPreciseCode(t *testing.T) {
	_, err := (Service{}).HandlePublishOnBehalf(context.Background(), envelope(map[string]any{
		"repository_id": "repo_1",
		"session_id":    "sess_1",
		"artifact_path": "docs/result.md",
		"artifact_kind": "finding",
		"logical_name":  "result",
		"reason":        "operator inspected the stalled session",
		"verdict":       "accept",
	}))
	rpcErr := assertRPCError(t, err, "dogfood_publish_on_behalf_retired")
	if rpcErr.Details["blocker"] != "legacy_python_composite_uses_repo_local_sqlite_connection" {
		t.Fatalf("blocker detail = %#v", rpcErr.Details)
	}
	if !strings.Contains(rpcErr.Message, "work.ack") {
		t.Fatalf("message does not name daemon-native remediation: %q", rpcErr.Message)
	}
}

func TestPublishOnBehalfRejectsInvalidVerdictBeforeRetirement(t *testing.T) {
	_, err := (Service{}).HandlePublishOnBehalf(context.Background(), envelope(map[string]any{
		"repository_id": "repo_1",
		"session_id":    "sess_1",
		"artifact_path": "docs/result.md",
		"artifact_kind": "finding",
		"logical_name":  "result",
		"reason":        "operator inspected the stalled session",
		"verdict":       "approve",
	}))
	assertRPCError(t, err, "invalid_verdict")
}

func TestSurgicalRecoveryRequiresConfirmWrite(t *testing.T) {
	_, err := (Service{}).HandleSurgicalRecovery(context.Background(), envelope(map[string]any{
		"repository_id":        "repo_1",
		"job_id":               "job_1",
		"reason":               "operator inspected the worktree and process",
		"extend_lease_seconds": 900,
	}))
	assertRPCError(t, err, "confirm_write_required")
}

func TestSurgicalRecoveryValidShapeFailsClosedWithPreciseCode(t *testing.T) {
	_, err := (Service{}).HandleSurgicalRecovery(context.Background(), envelope(map[string]any{
		"repository_id":        "repo_1",
		"job_id":               "job_1",
		"reason":               "operator inspected the worktree and process",
		"extend_lease_seconds": 900,
		"confirm_write":        true,
	}))
	rpcErr := assertRPCError(t, err, "dogfood_surgical_recovery_retired")
	if rpcErr.Details["blocker"] != "legacy_python_composite_uses_repo_local_sqlite_connection_and_progress_lock_policy" {
		t.Fatalf("blocker detail = %#v", rpcErr.Details)
	}
	if rpcErr.Details["extend_lease_seconds"] != 900 {
		t.Fatalf("extend detail = %#v", rpcErr.Details)
	}
}

func TestSurgicalRecoveryBoundsLeaseExtensionBeforeRetirement(t *testing.T) {
	_, err := (Service{}).HandleSurgicalRecovery(context.Background(), envelope(map[string]any{
		"repository_id":        "repo_1",
		"job_id":               "job_1",
		"reason":               "operator inspected the worktree and process",
		"extend_lease_seconds": 59,
		"confirm_write":        true,
	}))
	assertRPCError(t, err, "invalid_lease_extension")
}

func TestServiceRegistersDogfoodMethods(t *testing.T) {
	server := rpc.NewServer()
	Service{Runner: nonNilRunner{}}.Register(server)
	for _, method := range []string{"dogfood.publish_on_behalf", "dogfood.surgical_recovery"} {
		if server.Handlers[method] == nil {
			t.Fatalf("%s was not registered", method)
		}
	}
}

func envelope(params map[string]any) rpc.Envelope {
	return rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_test",
		Method:        "dogfood.test",
		Params:        params,
	}
}

func assertRPCError(t *testing.T, err error, code string) *rpc.Error {
	t.Helper()
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %v, want rpc error %q", err, code)
	}
	if rpcErr.Code != code {
		t.Fatalf("error code = %q, want %q (message %q)", rpcErr.Code, code, rpcErr.Message)
	}
	return rpcErr
}

type nonNilRunner struct{}

func (nonNilRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected exec")
}

func (nonNilRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeDogfoodRow{}
}

func (nonNilRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query")
}

func (nonNilRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("unexpected tx")
}

type fakeDogfoodRow struct{}

func (fakeDogfoodRow) Scan(...any) error {
	return pgx.ErrNoRows
}
