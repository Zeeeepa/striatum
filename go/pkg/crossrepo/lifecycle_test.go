package crossrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCancelRunSkipsTerminalParticipants(t *testing.T) {
	ctx := context.Background()
	runner := newLifecycleFakeRunner("running", []Participant{
		{RepositoryAlias: "consumer", RepositoryID: "repo_consumer", LocalRunID: strPtr("run_consumer"), State: "completed"},
		{RepositoryAlias: "primary", RepositoryID: "repo_primary", LocalRunID: strPtr("run_primary"), State: "running"},
	})
	local := &lifecycleFakeLocalRunner{}

	result, err := CancelRun(ctx, runner, "xrun_1", "operator request", local)
	if err != nil {
		t.Fatal(err)
	}
	if result["state"] != "canceled" {
		t.Fatalf("state = %v, want canceled", result["state"])
	}
	assertStringSlice(t, result["skipped_terminal"], []string{"consumer"})
	assertStringSlice(t, result["canceled"], []string{"primary"})
	if got := local.canceledAliases(); strings.Join(got, ",") != "repo_primary/run_primary" {
		t.Fatalf("local cancellations = %v", got)
	}
	if runner.participants["consumer"].State != "completed" {
		t.Fatalf("terminal participant state changed: %s", runner.participants["consumer"].State)
	}
	if runner.participants["primary"].State != "canceled" {
		t.Fatalf("primary state = %s, want canceled", runner.participants["primary"].State)
	}
	if runner.lastReconcileError != "" {
		t.Fatalf("lastReconcileError = %q, want empty", runner.lastReconcileError)
	}
}

func TestCancelRunSkipsPreparingParticipantWithoutLocalRun(t *testing.T) {
	ctx := context.Background()
	runner := newLifecycleFakeRunner("preparing", []Participant{
		{RepositoryAlias: "consumer", RepositoryID: "repo_consumer", LocalRunID: nil, State: "preparing"},
		{RepositoryAlias: "primary", RepositoryID: "repo_primary", LocalRunID: strPtr("run_primary"), State: "prepared"},
	})
	local := &lifecycleFakeLocalRunner{}

	result, err := CancelRun(ctx, runner, "xrun_1", "operator request", local)
	if err != nil {
		t.Fatal(err)
	}
	if result["state"] != "canceled" {
		t.Fatalf("state = %v, want canceled", result["state"])
	}
	assertStringSlice(t, result["skipped_no_local_run"], []string{"consumer"})
	assertStringSlice(t, result["canceled"], []string{"primary"})
	if runner.participants["consumer"].State != "canceled" {
		t.Fatalf("consumer state = %s, want canceled", runner.participants["consumer"].State)
	}
	if runner.participants["primary"].State != "canceled" {
		t.Fatalf("primary state = %s, want canceled", runner.participants["primary"].State)
	}
}

func TestCancelRunRecordsBlockedErrorDetails(t *testing.T) {
	ctx := context.Background()
	runner := newLifecycleFakeRunner("running", []Participant{
		{RepositoryAlias: "consumer", RepositoryID: "repo_consumer", LocalRunID: strPtr("run_consumer"), State: "running"},
		{RepositoryAlias: "primary", RepositoryID: "repo_primary", LocalRunID: strPtr("run_primary"), State: "running"},
	})
	local := &lifecycleFakeLocalRunner{failRepositoryID: "repo_consumer"}

	result, err := CancelRun(ctx, runner, "xrun_1", "operator request", local)
	if err != nil {
		t.Fatal(err)
	}
	if result["state"] != "blocked" {
		t.Fatalf("state = %v, want blocked", result["state"])
	}
	assertStringSlice(t, result["blocked"], []string{"consumer"})
	blockedErrors, ok := result["blocked_errors"].(map[string]string)
	if !ok {
		t.Fatalf("blocked_errors = %#v, want map[string]string", result["blocked_errors"])
	}
	if blockedErrors["consumer"] != "cancel failed for consumer" {
		t.Fatalf("blocked error = %q", blockedErrors["consumer"])
	}
	if runner.lastReconcileError != "consumer: cancel failed for consumer" {
		t.Fatalf("lastReconcileError = %q", runner.lastReconcileError)
	}
	if runner.participants["consumer"].State != "blocked" {
		t.Fatalf("consumer state = %s, want blocked", runner.participants["consumer"].State)
	}
	if runner.participants["primary"].State != "canceled" {
		t.Fatalf("primary state = %s, want canceled", runner.participants["primary"].State)
	}
}

func TestCancelRunBlocksNonPreparingParticipantWithoutLocalRun(t *testing.T) {
	ctx := context.Background()
	runner := newLifecycleFakeRunner("running", []Participant{
		{RepositoryAlias: "consumer", RepositoryID: "repo_consumer", LocalRunID: nil, State: "running"},
		{RepositoryAlias: "primary", RepositoryID: "repo_primary", LocalRunID: strPtr("run_primary"), State: "running"},
	})
	local := &lifecycleFakeLocalRunner{}

	result, err := CancelRun(ctx, runner, "xrun_1", "operator request", local)
	if err != nil {
		t.Fatal(err)
	}
	if result["state"] != "blocked" {
		t.Fatalf("state = %v, want blocked", result["state"])
	}
	assertStringSlice(t, result["blocked"], []string{"consumer"})
	blockedErrors := result["blocked_errors"].(map[string]string)
	if blockedErrors["consumer"] != "participant consumer has no local_run_id" {
		t.Fatalf("blocked error = %q", blockedErrors["consumer"])
	}
	if runner.lastReconcileError != "consumer: participant consumer has no local_run_id" {
		t.Fatalf("lastReconcileError = %q", runner.lastReconcileError)
	}
}

func TestDescribeRunReturnsTypedNotFound(t *testing.T) {
	ctx := context.Background()
	runner := newLifecycleFakeRunner("", nil)
	runner.runMissing = true

	_, err := DescribeRun(ctx, runner, "xrun_missing")
	if err == nil {
		t.Fatal("expected error")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "not_found" {
		t.Fatalf("err = %#v, want not_found RPC error", err)
	}
}

type lifecycleFakeRunner struct {
	runState             string
	runMissing           bool
	participants         map[string]Participant
	lastReconcileError   string
	cancelingTransition  bool
	canceledAtTransition bool
}

func newLifecycleFakeRunner(state string, participants []Participant) *lifecycleFakeRunner {
	items := map[string]Participant{}
	for _, participant := range participants {
		items[participant.RepositoryAlias] = participant
	}
	return &lifecycleFakeRunner{runState: state, participants: items}
}

func (r *lifecycleFakeRunner) Exec(_ context.Context, sql string, args ...any) error {
	normalized := strings.Join(strings.Fields(sql), " ")
	switch {
	case strings.Contains(normalized, "UPDATE striatumd.cross_repo_runs SET state = 'canceling'"):
		r.runState = "canceling"
		r.cancelingTransition = true
	case strings.Contains(normalized, "UPDATE striatumd.cross_repo_run_repositories SET state = $1"):
		state, _ := args[0].(string)
		alias, _ := args[3].(string)
		participant := r.participants[alias]
		participant.State = state
		r.participants[alias] = participant
	case strings.Contains(normalized, "UPDATE striatumd.cross_repo_runs SET state = $1"):
		state, _ := args[0].(string)
		r.runState = state
		if state == "canceled" {
			r.canceledAtTransition = true
		}
		if len(args) > 3 && args[3] != nil {
			r.lastReconcileError = fmt.Sprint(args[3])
		} else {
			r.lastReconcileError = ""
		}
	default:
		return fmt.Errorf("unexpected exec: %s", normalized)
	}
	return nil
}

func (r *lifecycleFakeRunner) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	if strings.Contains(sql, "SELECT workflow_id, workflow_version, primary_repository_id, state") {
		if r.runMissing {
			return lifecycleFakeRow{err: pgx.ErrNoRows}
		}
		version := "2026-05-18"
		return lifecycleFakeRow{values: []any{"workflow", &version, "repo_primary", r.runState}}
	}
	return lifecycleFakeRow{err: fmt.Errorf("unexpected query row: %s", sql)}
}

func (r *lifecycleFakeRunner) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if !strings.Contains(sql, "SELECT repository_alias, repository_id, local_run_id, state") {
		return nil, fmt.Errorf("unexpected query: %s", sql)
	}
	rows := make([][]any, 0, len(r.participants))
	for _, alias := range []string{"consumer", "primary"} {
		if participant, ok := r.participants[alias]; ok {
			rows = append(rows, []any{
				participant.RepositoryAlias,
				participant.RepositoryID,
				participant.LocalRunID,
				participant.State,
			})
		}
	}
	return &lifecycleFakeRows{rows: rows, index: -1}, nil
}

func (r *lifecycleFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("not implemented")
}

func (r *lifecycleFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("not implemented")
}

type lifecycleFakeRow struct {
	values []any
	err    error
}

func (r lifecycleFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanValues(r.values, dest...)
}

type lifecycleFakeRows struct {
	rows  [][]any
	index int
	err   error
}

func (r *lifecycleFakeRows) Close()                        {}
func (r *lifecycleFakeRows) Err() error                    { return r.err }
func (r *lifecycleFakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *lifecycleFakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *lifecycleFakeRows) Next() bool {
	r.index++
	return r.index < len(r.rows)
}
func (r *lifecycleFakeRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.rows) {
		return errors.New("scan without current row")
	}
	return scanValues(r.rows[r.index], dest...)
}
func (r *lifecycleFakeRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.rows) {
		return nil, errors.New("values without current row")
	}
	return r.rows[r.index], nil
}
func (r *lifecycleFakeRows) RawValues() [][]byte { return nil }
func (r *lifecycleFakeRows) Conn() *pgx.Conn     { return nil }

func scanValues(values []any, dest ...any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("scan got %d destinations for %d values", len(dest), len(values))
	}
	for i, value := range values {
		switch target := dest[i].(type) {
		case *string:
			if value == nil {
				*target = ""
				continue
			}
			*target = value.(string)
		case **string:
			if value == nil {
				*target = nil
				continue
			}
			switch typed := value.(type) {
			case *string:
				*target = typed
			case string:
				copied := typed
				*target = &copied
			default:
				return fmt.Errorf("unsupported nullable string value %T", value)
			}
		default:
			return fmt.Errorf("unsupported scan target %T", target)
		}
	}
	return nil
}

type lifecycleFakeLocalRunner struct {
	failRepositoryID string
	canceled         []string
}

func (l *lifecycleFakeLocalRunner) Prepare(context.Context, string, string, string) (string, error) {
	return "", errors.New("not implemented")
}
func (l *lifecycleFakeLocalRunner) Start(context.Context, string, string) error {
	return errors.New("not implemented")
}
func (l *lifecycleFakeLocalRunner) Cancel(_ context.Context, repositoryID string, localRunID string, _ string) error {
	if repositoryID == l.failRepositoryID {
		return errors.New("cancel failed for consumer")
	}
	l.canceled = append(l.canceled, repositoryID+"/"+localRunID)
	return nil
}
func (l *lifecycleFakeLocalRunner) ParticipantIntact(context.Context, string, *string) bool {
	return false
}
func (l *lifecycleFakeLocalRunner) HumanCheckpoint(context.Context, string, *string, string) error {
	return nil
}
func (l *lifecycleFakeLocalRunner) canceledAliases() []string {
	return append([]string{}, l.canceled...)
}

func assertStringSlice(t *testing.T, got any, want []string) {
	t.Helper()
	typed, ok := got.([]string)
	if !ok {
		t.Fatalf("got %T (%#v), want []string", got, got)
	}
	if len(typed) != len(want) {
		t.Fatalf("got %v, want %v", typed, want)
	}
	for i := range want {
		if typed[i] != want[i] {
			t.Fatalf("got %v, want %v", typed, want)
		}
	}
}

func strPtr(value string) *string {
	return &value
}

var _ db.Runner = (*lifecycleFakeRunner)(nil)
var _ LocalRunner = (*lifecycleFakeLocalRunner)(nil)
var _ pgx.Rows = (*lifecycleFakeRows)(nil)
