package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type processRunRunner struct {
	repoRoot     string
	requirements map[string]any
	escape       bool
	execs        []processRunExec
	commits      int
}

type processRunExec struct {
	sql  string
	args []any
}

func (r *processRunRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec")
}

func (r *processRunRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (r *processRunRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (r *processRunRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return &processRunTx{runner: r}, nil
}

type processRunTx struct {
	runner *processRunRunner
}

func (tx *processRunTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.runner.execs = append(tx.runner.execs, processRunExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *processRunTx) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	switch {
	case strings.Contains(sql, "repo_event_chain_heads"):
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		return decisionRecordRow{values: []any{int64(1)}}
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

func (tx *processRunTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (tx *processRunTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":                       "job_1",
			"run_id":                       "run_1",
			"state":                        "claimed",
			"current_lease_id":             "lease_1",
			"capability_requirements_json": tx.runner.requirements,
			"lane_selector_json":           map[string]any{"lane_id": "lane_1"},
		}}), nil
	case strings.Contains(sql, "FROM striatumd.leases"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"lease_id":         "lease_1",
			"state":            "active",
			"owner_session_id": "session_1",
			"resource_id":      "job_1",
			"expires_at":       time.Now().Add(time.Hour),
		}}), nil
	case strings.Contains(sql, "FROM striatumd.runs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"run_id":    "run_1",
			"repo_root": tx.runner.repoRoot,
		}}), nil
	case strings.Contains(sql, "FROM striatumd.events"):
		if tx.runner.escape {
			return runPrepareRowsFromMaps([]map[string]any{{"?column?": 1}}), nil
		}
		return runPrepareRowsFromMaps(nil), nil
	case strings.Contains(sql, "FROM striatumd.work_packets"):
		return runPrepareRowsFromMaps([]map[string]any{{"packet_id": "wp_1"}}), nil
	case strings.Contains(sql, "FROM striatumd.job_worktrees"):
		return runPrepareRowsFromMaps(nil), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func (tx *processRunTx) Commit(context.Context) error {
	tx.runner.commits++
	return nil
}

func (tx *processRunTx) Rollback(context.Context) error {
	return nil
}

func TestProcessRunRecordsExecutionEvidence(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &processRunRunner{
		repoRoot:     repoRoot,
		requirements: map[string]any{"process_execution": true},
	}

	result, err := HandleProcessRun(context.Background(), runner, processRunEnvelope(map[string]any{
		"process_id": "proc_1",
		"args":       []any{"sh", "-c", "printf ok"},
	}))
	if err != nil {
		t.Fatalf("process.run failed: %v", err)
	}
	if result["status"] != "exited" || result["exit_code"] != 0 {
		t.Fatalf("result = %#v", result)
	}
	if result["stdout_bytes"] != 2 {
		t.Fatalf("stdout bytes = %#v", result["stdout_bytes"])
	}
	sum := sha256.Sum256([]byte("ok"))
	if result["stdout_sha256"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("stdout hash = %#v", result["stdout_sha256"])
	}
	if _, leaked := result["stdout"]; leaked {
		t.Fatalf("result should not include stdout transcript: %#v", result)
	}
	if !processRunSawExec(runner, "INSERT INTO striatumd.process_executions") {
		t.Fatal("process_executions insert not recorded")
	}
	if !processRunSawExec(runner, "UPDATE striatumd.process_executions") {
		t.Fatal("process_executions update not recorded")
	}
	payload := processRunCompletedPayload(t, runner)
	if payload["state"] != "exited" || payload["stdout_bytes"] != 2 {
		t.Fatalf("completed payload = %#v", payload)
	}
}

func TestProcessRunRequiresExplicitJobCapabilityBeforeExecution(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &processRunRunner{repoRoot: repoRoot, requirements: map[string]any{}}
	target := filepath.Join(repoRoot, "should-not-exist")

	_, err := HandleProcessRun(context.Background(), runner, processRunEnvelope(map[string]any{
		"process_id": "proc_1",
		"args":       []any{"sh", "-c", "touch should-not-exist"},
	}))
	assertProcessRunRPCCode(t, err, "invalid_transition")
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("command ran before capability refusal: %v", statErr)
	}
	if processRunSawExec(runner, "INSERT INTO striatumd.process_executions") {
		t.Fatal("process evidence row was inserted for refused execution")
	}
}

func TestProcessRunAllowsMatchingEscapeDecision(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &processRunRunner{repoRoot: repoRoot, requirements: map[string]any{}, escape: true}

	result, err := HandleProcessRun(context.Background(), runner, processRunEnvelope(map[string]any{
		"process_id": "proc_escape",
		"args":       []any{"sh", "-c", "printf escaped"},
	}))
	if err != nil {
		t.Fatalf("process.run with escape failed: %v", err)
	}
	if result["status"] != "exited" || result["stdout_bytes"] != 7 {
		t.Fatalf("result = %#v", result)
	}
	if !processRunSawExec(runner, "INSERT INTO striatumd.process_executions") {
		t.Fatal("process evidence row was not inserted for escaped execution")
	}
}

func processRunEnvelope(overrides map[string]any) rpc.Envelope {
	params := map[string]any{
		"repository_id": "repo_1",
		"session_id":    "session_1",
		"job_id":        "job_1",
		"lease_id":      "lease_1",
	}
	for key, value := range overrides {
		params[key] = value
	}
	return rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_process_run",
		Method:        "process.run",
		Params:        params,
	}
}

func processRunSawExec(runner *processRunRunner, fragment string) bool {
	for _, exec := range runner.execs {
		if strings.Contains(exec.sql, fragment) {
			return true
		}
	}
	return false
}

func processRunCompletedPayload(t *testing.T, runner *processRunRunner) map[string]any {
	t.Helper()
	for _, exec := range runner.execs {
		if !strings.Contains(exec.sql, "INSERT INTO striatumd.events") || len(exec.args) < 10 {
			continue
		}
		if exec.args[3] != "process.completed" {
			continue
		}
		payload, ok := exec.args[9].(map[string]any)
		if !ok {
			t.Fatalf("event payload = %#v", exec.args[9])
		}
		return payload
	}
	t.Fatalf("process.completed event not found in %#v", runner.execs)
	return nil
}

func assertProcessRunRPCCode(t *testing.T, err error, code string) {
	t.Helper()
	rpcErr := &rpc.Error{}
	if !errors.As(err, &rpcErr) || rpcErr.Code != code {
		t.Fatalf("error = %v, want rpc code %s", err, code)
	}
}
