package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func TestSuperviseStartInsertsAndAttachesProcessSupervisor(t *testing.T) {
	origMkfifo := supervisionMkfifo
	origLaunch := supervisionLaunch
	defer func() {
		supervisionMkfifo = origMkfifo
		supervisionLaunch = origLaunch
	}()
	supervisionMkfifo = func(path string) error {
		return os.WriteFile(path, nil, 0o600)
	}
	supervisionLaunch = func(context.Context, supervisionStartConfig, string, string, string, string) (supervisionLaunchResult, error) {
		return supervisionLaunchResult{
			PID:          os.Getpid(),
			PIDStartTime: "start-token",
			Metadata: map[string]any{
				"tmux": map[string]any{
					"session_name":   "striatum-run_1-lane_1-sup_1",
					"attach_command": "tmux attach-session -t striatum-run_1-lane_1-sup_1",
				},
			},
		}, nil
	}

	repoRoot := t.TempDir()
	tx1 := &superviseControlFakeTx{}
	tx2 := &superviseControlFakeTx{}
	runner := &superviseControlFakeRunner{
		repoRoot: repoRoot,
		txs:      []*superviseControlFakeTx{tx1, tx2},
	}
	result, err := HandleSuperviseStart(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_start",
		Method:        "supervise.start",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStart: %v", err)
	}
	if result["state"] != "attached" || result["session_id"] != "sess_1" || result["run_id"] != "run_1" {
		t.Fatalf("start result = %#v", result)
	}
	if result["lane_attestation"] != "attested" {
		t.Fatalf("lane_attestation = %v", result["lane_attestation"])
	}
	tmux := result["tmux"].(map[string]any)
	if tmux["attach_command"] != "tmux attach-session -t striatum-run_1-lane_1-sup_1" {
		t.Fatalf("tmux metadata = %#v", tmux)
	}
	if !tx1.sawExec("INSERT INTO striatumd.process_supervisors") {
		t.Fatalf("missing process_supervisors insert: %#v", tx1.execs)
	}
	if !tx1.sawExec("pg_advisory_xact_lock") {
		t.Fatalf("missing supervise-start advisory lock: %#v", tx1.execs)
	}
	if !tx1.sawExec("INSERT INTO striatumd.daemon_supervisors") {
		t.Fatalf("missing daemon_supervisors insert: %#v", tx1.execs)
	}
	if !tx2.sawExec("UPDATE striatumd.process_supervisors", "state = $1") {
		t.Fatalf("missing attached process update: %#v", tx2.execs)
	}
	if len(tx1.eventInserts()) != 1 || len(tx2.eventInserts()) != 1 {
		t.Fatalf("event inserts tx1/tx2 = %d/%d", len(tx1.eventInserts()), len(tx2.eventInserts()))
	}
	payload := tx2.lastEventInsert().args[9].(map[string]any)
	eventTmux := payload["tmux"].(map[string]any)
	if eventTmux["session_name"] != "striatum-run_1-lane_1-sup_1" {
		t.Fatalf("started event tmux metadata = %#v", eventTmux)
	}
}

func TestSuperviseStartPropagatesRequireTmuxFromLaneSupervision(t *testing.T) {
	origMkfifo := supervisionMkfifo
	origLaunch := supervisionLaunch
	defer func() {
		supervisionMkfifo = origMkfifo
		supervisionLaunch = origLaunch
	}()
	supervisionMkfifo = func(path string) error {
		return os.WriteFile(path, nil, 0o600)
	}
	var launchedConfig supervisionStartConfig
	supervisionLaunch = func(_ context.Context, config supervisionStartConfig, _ string, _ string, _ string, _ string) (supervisionLaunchResult, error) {
		launchedConfig = config
		return supervisionLaunchResult{PID: os.Getpid(), PIDStartTime: "start-token"}, nil
	}

	runner := &superviseControlFakeRunner{
		repoRoot: t.TempDir(),
		workflowSupervision: map[string]any{
			"transport":    supervisionTransportPTYHelper,
			"require_tmux": true,
		},
		txs: []*superviseControlFakeTx{{}, {}},
	}
	result, err := HandleSuperviseStart(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_start_require_tmux",
		Method:        "supervise.start",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStart: %v", err)
	}
	if !launchedConfig.RequireTmux || launchedConfig.Transport != supervisionTransportPTYHelper {
		t.Fatalf("launched config = %#v", launchedConfig)
	}
	if result["require_tmux"] != true {
		t.Fatalf("result require_tmux = %#v", result["require_tmux"])
	}
}

func TestSuperviseStartWrapsAgentLoopLaneInAgentLoop(t *testing.T) {
	origMkfifo := supervisionMkfifo
	origLaunch := supervisionLaunch
	defer func() {
		supervisionMkfifo = origMkfifo
		supervisionLaunch = origLaunch
	}()
	t.Setenv("STRIATUM_AGENT_LOOP_BINARY", "/bin/striatumd")
	supervisionMkfifo = func(path string) error {
		return os.WriteFile(path, nil, 0o600)
	}
	var launchedConfig supervisionStartConfig
	supervisionLaunch = func(_ context.Context, config supervisionStartConfig, _ string, _ string, _ string, _ string) (supervisionLaunchResult, error) {
		launchedConfig = config
		return supervisionLaunchResult{PID: os.Getpid(), PIDStartTime: "start-token"}, nil
	}

	runner := &superviseControlFakeRunner{
		repoRoot: t.TempDir(),
		workflowLane: map[string]any{
			"adapter_capabilities": map[string]any{"agent_loop": true},
		},
		txs: []*superviseControlFakeTx{{}, {}},
	}
	result, err := HandleSuperviseStart(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_start_agent_loop",
		Method:        "supervise.start",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStart: %v", err)
	}
	want := []string{"/bin/striatumd", "-agent-loop", "--", "/bin/cat"}
	if strings.Join(launchedConfig.Command, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("launched command = %#v, want %#v", launchedConfig.Command, want)
	}
	if launchedConfig.AgentLoopMode != agentLoopModeSelfDriving || result["agent_loop_mode"] != agentLoopModeSelfDriving {
		t.Fatalf("self-driving agent-loop mode not surfaced: config=%#v result=%#v", launchedConfig.AgentLoopMode, result)
	}
	if strings.Join(launchedConfig.OriginalCommand, "\x00") != "/bin/cat" {
		t.Fatalf("original command = %#v", launchedConfig.OriginalCommand)
	}
}

func TestResolveSupervisedCommandBinaryResolvesOnAugmentedPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "faketool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STRIATUM_SUPERVISED_PATH_DIRS", dir)

	got := resolveSupervisedCommandBinary([]string{"faketool", "arg"})
	want := []string{bin, "arg"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("resolved = %#v, want %#v", got, want)
	}

	// An absolute / path-bearing argv0 is left untouched.
	abs := resolveSupervisedCommandBinary([]string{"/bin/cat", "-"})
	if strings.Join(abs, "\x00") != strings.Join([]string{"/bin/cat", "-"}, "\x00") {
		t.Fatalf("absolute argv0 rewritten: %#v", abs)
	}
}

func TestSupervisedEnvAddsOperatorLocalBinsToPath(t *testing.T) {
	home := t.TempDir()
	localBin := filepath.Join(home, ".local", "bin")
	npmBin := filepath.Join(home, ".npm-global", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatalf("mkdir local bin: %v", err)
	}
	if err := os.MkdirAll(npmBin, 0o755); err != nil {
		t.Fatalf("mkdir npm bin: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", strings.Join([]string{"/usr/local/bin", "/usr/bin"}, string(os.PathListSeparator)))
	t.Setenv("STRIATUM_SUPERVISED_PATH_DIRS", "")

	entries := supervisedEnvEntries("/repo", "repo_1", "run_1", "sess_1", "sup_1", "lane_1")
	path := envValue(t, entries, "PATH")
	want := strings.Join([]string{"/usr/local/bin", "/usr/bin", localBin, npmBin}, string(os.PathListSeparator))
	if path != want {
		t.Fatalf("PATH = %q, want %q", path, want)
	}
	if countEnv(entries, "PATH") != 1 {
		t.Fatalf("supervisedEnvEntries PATH count = %d, want 1", countEnv(entries, "PATH"))
	}
	if countEnv(supervisedEnv("/repo", "repo_1", "run_1", "sess_1", "sup_1", "lane_1"), "PATH") != 1 {
		t.Fatalf("supervisedEnv should emit exactly one effective PATH")
	}
	for _, key := range []string{
		"STRIATUM_REPOSITORY_ID",
		"STRIATUM_RUN_ID",
		"STRIATUM_SESSION_ID",
		"STRIATUM_SUPERVISOR_ID",
		"STRIATUM_REPO",
		"STRIATUM_LANE_ID",
	} {
		if envValue(t, entries, key) == "" {
			t.Fatalf("%s missing from supervised env entries: %#v", key, entries)
		}
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry, "STRIATUM_SUPERVISED_PATH_DIRS=") {
			t.Fatalf("override trust-boundary env leaked through supervisedEnvEntries: %#v", entries)
		}
	}
}

func TestSupervisedEnvPathOverrideAppendsExistingAbsoluteDirs(t *testing.T) {
	baseDir := t.TempDir()
	customOne := filepath.Join(baseDir, "custom-one")
	customTwo := filepath.Join(baseDir, "custom-two")
	if err := os.MkdirAll(customOne, 0o755); err != nil {
		t.Fatalf("mkdir custom one: %v", err)
	}
	if err := os.MkdirAll(customTwo, 0o755); err != nil {
		t.Fatalf("mkdir custom two: %v", err)
	}
	missing := filepath.Join(baseDir, "missing")
	t.Setenv("PATH", strings.Join([]string{"/usr/bin", customOne}, string(os.PathListSeparator)))
	t.Setenv("STRIATUM_SUPERVISED_PATH_DIRS", strings.Join([]string{
		customOne,
		missing,
		"relative/bin",
		customTwo,
	}, string(os.PathListSeparator)))

	entries := supervisedEnvEntries("/repo", "repo_1", "run_1", "sess_1", "sup_1", "lane_1")
	path := envValue(t, entries, "PATH")
	want := strings.Join([]string{"/usr/bin", customOne, customTwo}, string(os.PathListSeparator))
	if path != want {
		t.Fatalf("PATH = %q, want %q", path, want)
	}
}

func TestSuperviseSendDeliversPacketUnacknowledged(t *testing.T) {
	dir := t.TempDir()
	pipePath := dir + "/stdin.pipe"
	if err := syscall.Mkfifo(pipePath, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	readDone := make(chan []byte, 1)
	go func() {
		data, _ := os.ReadFile(pipePath)
		readDone <- data
	}()

	tx := &superviseControlFakeTx{pipePath: pipePath, pid: os.Getpid()}
	runner := &superviseControlFakeRunner{txs: []*superviseControlFakeTx{tx}}
	result, err := HandleSuperviseSend(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_send",
		Method:        "supervise.send",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"packet_id":     "packet_1",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseSend: %v", err)
	}
	if result["delivery_state"] != "delivered_unacknowledged" || result["control_ack_expected"] != true {
		t.Fatalf("send result = %#v", result)
	}
	body := <-readDone
	var packet map[string]any
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatalf("packet json = %q: %v", string(body), err)
	}
	if packet["packet"] != "body" {
		t.Fatalf("delivered packet = %#v", packet)
	}
	if !tx.sawExec("UPDATE striatumd.process_supervisors", "heartbeat_at") {
		t.Fatalf("missing heartbeat update: %#v", tx.execs)
	}
	event := tx.lastEventInsert()
	if event == nil || event.args[3] != "supervisor.packet_delivered" {
		t.Fatalf("event insert = %#v", event)
	}
	payload := event.args[9].(map[string]any)
	if payload["packet_id"] != "packet_1" || payload["stdin_delivery"] != stdinDeliveryPersistentFIFO {
		t.Fatalf("event payload = %#v", payload)
	}
}

func TestSuperviseSendWrongKindPacketIDPointsAtClaimNextPacketID(t *testing.T) {
	tx := &superviseControlFakeTx{}
	runner := &superviseControlFakeRunner{txs: []*superviseControlFakeTx{tx}}
	_, err := HandleSuperviseSend(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_send_wrong_id",
		Method:        "supervise.send",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"packet_id":     "msg_123",
		},
	})
	if err == nil {
		t.Fatalf("expected supervise send to reject wrong-kind packet id")
	}
	rpcErr, ok := err.(*rpc.Error)
	if !ok || rpcErr.Code != "not_found" {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(rpcErr.Message, "msg_123 is a message id, not a work packet id") ||
		!strings.Contains(rpcErr.Message, "data.packet_id") ||
		!strings.Contains(rpcErr.Message, "data.packet.packet_id") {
		t.Fatalf("message = %q", rpcErr.Message)
	}
}

func TestSuperviseStopMarksSupervisorStoppedAndUnlinksPipe(t *testing.T) {
	dir := t.TempDir()
	pipePath := dir + "/stdin.pipe"
	if err := os.WriteFile(pipePath, nil, 0o600); err != nil {
		t.Fatalf("write pipe placeholder: %v", err)
	}
	tx := &superviseControlFakeTx{pipePath: pipePath}
	runner := &superviseControlFakeRunner{txs: []*superviseControlFakeTx{tx}, pipePath: pipePath}
	result, err := HandleSuperviseStop(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_stop",
		Method:        "supervise.stop",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"reason":        "operator_requested",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStop: %v", err)
	}
	if result["state"] != "stopped" || result["stop_reason"] != "operator_requested" {
		t.Fatalf("stop result = %#v", result)
	}
	if _, err := os.Stat(pipePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pipe still exists or unexpected stat err: %v", err)
	}
	if !tx.sawExec("UPDATE striatumd.process_supervisors", "ended_at") {
		t.Fatalf("missing stopped update: %#v", tx.execs)
	}
	event := tx.lastEventInsert()
	if event == nil || event.args[3] != "supervisor.stopped" {
		t.Fatalf("event insert = %#v", event)
	}
}

func TestLinuxProcStatZombieDetectsDefunctProcessState(t *testing.T) {
	if !linuxProcStatZombie([]byte("1234 (agent command) Z 1 2 3")) {
		t.Fatal("expected Z process state to be treated as zombie")
	}
	if linuxProcStatZombie([]byte("1234 (agent command) S 1 2 3")) {
		t.Fatal("sleeping process state should not be treated as zombie")
	}
}

type superviseControlFakeRunner struct {
	mu                  sync.Mutex
	repoRoot            string
	pipePath            string
	workflowSupervision map[string]any
	workflowLane        map[string]any
	txs                 []*superviseControlFakeTx
}

func (r *superviseControlFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *superviseControlFakeRunner) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	return r.fakeRow(sql, args...)
}

func (r *superviseControlFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (r *superviseControlFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.txs) == 0 {
		return nil, errors.New("unexpected BeginTx")
	}
	tx := r.txs[0]
	r.txs = r.txs[1:]
	if tx.pipePath == "" {
		tx.pipePath = r.pipePath
	}
	return tx, nil
}

func (r *superviseControlFakeRunner) fakeRow(sql string, args ...any) db.Row {
	switch {
	case strings.Contains(sql, "SELECT s.session_id, s.run_id"):
		return superviseControlFakeRow{values: []any{"sess_1", "run_1", "lane_1", "active", "snap_1", r.repoRoot}}
	case strings.Contains(sql, "SELECT state FROM striatumd.sessions"):
		return superviseControlFakeRow{values: []any{"active"}}
	case strings.Contains(sql, "SELECT workflow_json"):
		lane := map[string]any{
			"adapter": "process",
			"command": []any{"/bin/cat"},
		}
		if r.workflowSupervision != nil {
			lane["supervision"] = r.workflowSupervision
		}
		for key, value := range r.workflowLane {
			lane[key] = value
		}
		return superviseControlFakeRow{values: []any{map[string]any{
			"lanes": map[string]any{
				"lane_1": lane,
			},
		}}}
	case strings.Contains(sql, "SELECT supervisor_id, state") && strings.Contains(sql, "state = ANY"):
		return superviseControlFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "SELECT session_id") && strings.Contains(sql, "FROM striatumd.sessions"):
		return superviseControlFakeRow{values: []any{"sess_1"}}
	case strings.Contains(sql, "SELECT ps.supervisor_id"):
		return superviseControlFakeRow{values: []any{"sup_1", "run_1", "sess_1", "attached", r.pipePath, nil, "", "dsup_1", map[string]any{"stdin_delivery": stdinDeliveryPersistentFIFO}}}
	default:
		return superviseControlFakeRow{err: errors.New("unexpected runner query: " + sql)}
	}
}

type superviseControlFakeTx struct {
	pipePath   string
	pid        int
	nextEvent  int64
	execs      []superviseControlExec
	committed  bool
	rolledBack bool
}

type superviseControlExec struct {
	sql  string
	args []any
}

func (tx *superviseControlFakeTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, superviseControlExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *superviseControlFakeTx) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	switch {
	case strings.Contains(sql, "SELECT supervisor_id, state") && strings.Contains(sql, "state = ANY"):
		return superviseControlFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "SELECT ps.supervisor_id"):
		var pid any
		if tx.pid > 0 {
			pid = tx.pid
		}
		return superviseControlFakeRow{values: []any{"sup_1", "run_1", "sess_1", "attached", tx.pipePath, pid, "", "dsup_1", map[string]any{"stdin_delivery": stdinDeliveryPersistentFIFO}}}
	case strings.Contains(sql, "FROM striatumd.work_packets"):
		return superviseControlFakeRow{values: []any{"packet_1", "run_1", "job_1", "lease_1", "sess_1", map[string]any{"packet": "body"}}}
	case strings.Contains(sql, "FROM striatumd.leases"):
		return superviseControlFakeRow{values: []any{"active", "sess_1", "job_1", "2999-01-01T00:00:00Z"}}
	case strings.Contains(sql, "SELECT state, daemon_supervisor_id"):
		dsup := "dsup_1"
		return superviseControlFakeRow{values: []any{"attached", &dsup}}
	case strings.Contains(sql, "FROM striatumd.daemon_supervisors") && strings.Contains(sql, "SELECT state"):
		return superviseControlFakeRow{values: []any{"attached"}}
	case strings.Contains(sql, "SELECT metadata_json"):
		return superviseControlFakeRow{values: []any{map[string]any{"stdin_delivery": stdinDeliveryPersistentFIFO}}}
	case strings.Contains(sql, "repo_event_chain_heads"):
		return superviseControlFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		tx.nextEvent++
		return superviseControlFakeRow{values: []any{tx.nextEvent}}
	default:
		return superviseControlFakeRow{err: errors.New("unexpected tx query: " + sql)}
	}
}

func (tx *superviseControlFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *superviseControlFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *superviseControlFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *superviseControlFakeTx) sawExec(parts ...string) bool {
	for _, exec := range tx.execs {
		ok := true
		for _, part := range parts {
			if !strings.Contains(exec.sql, part) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func (tx *superviseControlFakeTx) eventInserts() []superviseControlExec {
	var events []superviseControlExec
	for _, exec := range tx.execs {
		if strings.Contains(exec.sql, "INSERT INTO striatumd.events") {
			events = append(events, exec)
		}
	}
	return events
}

func (tx *superviseControlFakeTx) lastEventInsert() *superviseControlExec {
	events := tx.eventInserts()
	if len(events) == 0 {
		return nil
	}
	return &events[len(events)-1]
}

func envValue(t *testing.T, env []string, key string) string {
	t.Helper()
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	t.Fatalf("missing env key %s in %#v", key, env)
	return ""
}

func countEnv(env []string, key string) int {
	prefix := key + "="
	count := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			count++
		}
	}
	return count
}

type superviseControlFakeRow struct {
	values []any
	err    error
}

func (r superviseControlFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > len(r.values) {
		return errors.New("not enough fake row values")
	}
	for i, value := range r.values[:len(dest)] {
		switch target := dest[i].(type) {
		case *string:
			if value == nil {
				*target = ""
			} else {
				*target = value.(string)
			}
		case **string:
			if value == nil {
				*target = nil
			} else if ptr, ok := value.(*string); ok {
				*target = ptr
			} else {
				text := value.(string)
				*target = &text
			}
		case **int:
			if value == nil {
				*target = nil
			} else {
				typed := value.(int)
				*target = &typed
			}
		case *int64:
			*target = value.(int64)
		case *any:
			*target = value
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
