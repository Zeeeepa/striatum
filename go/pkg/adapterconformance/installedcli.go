package adapterconformance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// RFC 0109 P3 — the installed-CLI adapter-conformance gate. RunLive (runner.go)
// drives an in-process testagent that reuses its session across turns by
// construction, so it is structurally immune to #95 ("the agy seat re-registers
// a new, unattested session on its second turn"). This runner drives the REAL
// lane CLI through the production agent loop (agentloop.RunContext) over the
// harness's unix socket + MCP endpoint, so #95 is reproducible in CI for the
// first time. It is the "cannot lie" backing for the seat support tier
// (workflowtemplates.SeatTierForAdapter): a seat may be `supported` only with a
// green fixture here (TestSupportedSeatsHaveInstalledCLIFixture enforces it).
//
// The gate is a two-turn claim → publish → claim cycle: one run with TWO queued
// jobs targeting a single role+lane, plus a pre-seeded, attested session (as the
// daemon supervisor creates before launching a lane). A viable seat claims,
// publishes, and completes BOTH jobs under that one session. A seat with #95
// either dies after turn one (turn-2 job never completes) or re-registers a
// duplicate session (the run shows more than the one seeded session, and/or
// turn 2 is driven by a different, unattested session).

// InstalledCLIOptions parameterizes one installed-CLI seat run.
type InstalledCLIOptions struct {
	// Command is the lane argv. Defaults per adapter when empty.
	Command []string
	// RunTimeout bounds the whole two-turn cycle. Defaults to 4m (the lane is a
	// real LLM CLI). Callers may shorten it for fast red iteration.
	RunTimeout time.Duration
	// AfterFirstWorkPacket runs once after work.await_packet returns the first
	// leased packet to the installed CLI. It is intended for harness-only chaos
	// cells such as restarting daemon-facing surfaces while the seat owns a lease.
	AfterFirstWorkPacket func(context.Context, InstalledCLIFirstWorkPacket) error
}

// InstalledCLIFirstWorkPacket is the observation passed to
// InstalledCLIOptions.AfterFirstWorkPacket.
type InstalledCLIFirstWorkPacket struct {
	Harness      *Harness
	RepositoryID string
	RunID        string
	JobID        string
	SessionID    string
}

// InstalledCLIReport is the observed outcome of a two-turn installed-CLI seat
// run. Failures is empty only when both turns completed under the single
// pre-seeded attested session.
type InstalledCLIReport struct {
	Adapter                       string
	Command                       []string
	SeatSession                   string
	SessionCount                  int
	SessionIDs                    []string
	Job1State                     string
	Job2State                     string
	Job1Owner                     string
	Job2Owner                     string
	Artifact1                     bool
	Artifact2                     bool
	RunErr                        error
	Output                        string
	AfterFirstWorkPacketTriggered bool
	AfterFirstWorkPacketErr       error
	Failures                      []string
}

// Passed reports whether the seat held across both turns (the #95 gate, plus the
// launch/discovery/claim defects #76/#139/#85 it depends on).
func (r InstalledCLIReport) Passed() bool { return len(r.Failures) == 0 }

func defaultInstalledCLICommand(adapter string) []string {
	switch adapter {
	case "agy":
		return []string{"agy", "--dangerously-skip-permissions"}
	case "codex":
		return []string{"codex", "--dangerously-bypass-approvals-and-sandbox"}
	case "claude":
		return []string{"claude", "--dangerously-skip-permissions"}
	default:
		return []string{adapter}
	}
}

// RunInstalledCLI drives the real lane CLI for adapter through a two-turn
// claim → publish → claim cycle against the harness. It SKIPS when the CLI is
// absent (mirroring pgtest's skip without a database), so the gate is a no-op on
// a machine without the CLI but blocks where present.
func RunInstalledCLI(t *testing.T, h *Harness, adapter string, opts InstalledCLIOptions) InstalledCLIReport {
	t.Helper()

	command := opts.Command
	if len(command) == 0 {
		command = defaultInstalledCLICommand(adapter)
	}
	if _, err := exec.LookPath(command[0]); err != nil {
		t.Skipf("installed CLI %q not on PATH; skipping RFC 0109 P3 installed-CLI seat gate: %v", command[0], err)
	}
	timeout := opts.RunTimeout
	if timeout == 0 {
		timeout = 4 * time.Minute
	}

	bg := context.Background()
	fx := seedInstalledCLISeatFixture(t, bg, h.Runner, h.RepositoryID, adapter)

	// Resolve the lane env exactly as a supervised lane receives it. RunContext
	// reads the MCP endpoint, repository_id, and token from the environment; the
	// unix socket is passed explicitly. Blank any inherited daemon endpoint/token
	// pointers so the lane cannot escape to the live daemon.
	t.Setenv(agentloop.EnvMCPURL, h.MCPEndpoint())
	t.Setenv(agentloop.EnvRepositoryID, h.RepositoryID)
	t.Setenv(agentloop.EnvMCPToken, h.Token)
	t.Setenv(agentloop.EnvMCPTokenFile, "")
	t.Setenv(agentloop.EnvDaemonMCPHTTPURL, "")
	t.Setenv(agentloop.EnvDaemonMCPHTTPEndpointFile, "")

	report := InstalledCLIReport{Adapter: adapter, Command: command, SeatSession: fx.SeatSession}
	var hook *installedCLIHook
	if opts.AfterFirstWorkPacket != nil {
		hook = installAfterFirstWorkPacketHook(t, h, fx, opts.AfterFirstWorkPacket)
	}

	runCtx, cancel := context.WithCancel(bg)
	defer cancel()
	out := &syncBuffer{}
	runDone := make(chan error, 1)
	go func() {
		runDone <- agentloop.RunContext(runCtx, h.SocketPath, fx.RepoRoot, fx.RunID, fx.SeatSession, command, out, out)
	}()

	obs := h.Observer()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeoutCh := time.After(timeout)
loop:
	for {
		select {
		case err := <-runDone:
			report.RunErr = err
			report.Failures = append(report.Failures, fmt.Sprintf("lane process exited before both turns completed: %v", err))
			break loop
		case <-timeoutCh:
			report.Failures = append(report.Failures, fmt.Sprintf("two-turn cycle did not complete within %s (the seat stalled at launch/discovery/claim or wedged after turn one)", timeout))
			break loop
		case <-ticker.C:
			j1, _ := obs.JobState(bg, h.RepositoryID, fx.Job1ID)
			j2, _ := obs.JobState(bg, h.RepositoryID, fx.Job2ID)
			if j1 == "completed" && j2 == "completed" {
				break loop
			}
		}
	}

	// Stop the lane and drain.
	cancel()
	select {
	case <-runDone:
	case <-time.After(20 * time.Second):
	}

	// Collect the observed state regardless of outcome (diagnostics + gate).
	report.Job1State, _ = obs.JobState(bg, h.RepositoryID, fx.Job1ID)
	report.Job2State, _ = obs.JobState(bg, h.RepositoryID, fx.Job2ID)
	report.Job1Owner, _ = obs.JobLeaseOwner(bg, h.RepositoryID, fx.Job1ID)
	report.Job2Owner, _ = obs.JobLeaseOwner(bg, h.RepositoryID, fx.Job2ID)
	if a1, _ := obs.Artifact(bg, h.RepositoryID, fx.Job1ID, fx.Artifact1); a1 != nil {
		report.Artifact1 = true
	}
	if a2, _ := obs.Artifact(bg, h.RepositoryID, fx.Job2ID, fx.Artifact2); a2 != nil {
		report.Artifact2 = true
	}
	sessions, _ := obs.SessionsForRun(bg, h.RepositoryID, fx.RunID)
	report.SessionCount = len(sessions)
	for _, s := range sessions {
		report.SessionIDs = append(report.SessionIDs, fmt.Sprint(s["session_id"]))
	}
	report.Output = out.String()

	// Reset Failures and assert the gate from the collected state, so a clean
	// run that broke the poll loop on completion is judged solely on final state.
	report.Failures = report.Failures[:0]
	if hook != nil {
		report.AfterFirstWorkPacketTriggered, report.AfterFirstWorkPacketErr = hook.result(15 * time.Second)
		if !report.AfterFirstWorkPacketTriggered {
			report.Failures = append(report.Failures, "after-first-work-packet hook did not run; the restart-while-leased cell never observed the first leased packet")
		}
		if report.AfterFirstWorkPacketErr != nil {
			report.Failures = append(report.Failures, fmt.Sprintf("after-first-work-packet hook failed: %v", report.AfterFirstWorkPacketErr))
		}
	}
	if report.Job1State != "completed" {
		report.Failures = append(report.Failures, fmt.Sprintf(
			"turn 1 job state=%q (want completed): the seat never claimed+published+completed its first packet — trust/feedback prompt (#76/#139) or MCP-discovery stall (#85) likely blocked launch/claim", report.Job1State))
	}
	if report.Job2State != "completed" {
		report.Failures = append(report.Failures, fmt.Sprintf(
			"turn 2 job state=%q (want completed): the seat did not drive a second turn — the session that produced turn 1 is gone (#95)", report.Job2State))
	}
	if report.SessionCount != 1 {
		report.Failures = append(report.Failures, fmt.Sprintf(
			"saw %d session rows for the run %v (want exactly 1): the seat re-registered a duplicate, unattested session across turns (#95)", report.SessionCount, report.SessionIDs))
	}
	if report.Job2Owner != "" && report.Job2Owner != fx.SeatSession {
		report.Failures = append(report.Failures, fmt.Sprintf(
			"turn 2 was driven by session %q, not the pre-seeded attested seat %q (#95)", report.Job2Owner, fx.SeatSession))
	}
	return report
}

type installedCLIHook struct {
	triggered chan struct{}
	done      chan error
}

func installAfterFirstWorkPacketHook(t *testing.T, h *Harness, fx *installedCLISeatFixture, hook func(context.Context, InstalledCLIFirstWorkPacket) error) *installedCLIHook {
	t.Helper()
	original, ok := h.Server.Handlers["work.await_packet"]
	if !ok {
		t.Fatalf("work.await_packet handler is not registered")
	}
	state := &installedCLIHook{
		triggered: make(chan struct{}, 1),
		done:      make(chan error, 1),
	}
	var once sync.Once
	h.Server.Register("work.await_packet", func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		res, err := original(ctx, envelope)
		if err == nil && workPacketForJob(res, fx.Job1ID) {
			once.Do(func() {
				state.triggered <- struct{}{}
				go func() {
					state.done <- hook(context.Background(), InstalledCLIFirstWorkPacket{
						Harness:      h,
						RepositoryID: fx.RepositoryID,
						RunID:        fx.RunID,
						JobID:        fx.Job1ID,
						SessionID:    fx.SeatSession,
					})
				}()
			})
		}
		return res, err
	})
	return state
}

func workPacketForJob(res map[string]any, jobID string) bool {
	if fmt.Sprint(res["type"]) != "work_packet" {
		return false
	}
	packet, ok := res["packet"].(map[string]any)
	if !ok {
		return false
	}
	job, ok := packet["job"].(map[string]any)
	if !ok {
		return false
	}
	return fmt.Sprint(job["job_id"]) == jobID
}

func (h *installedCLIHook) result(timeout time.Duration) (bool, error) {
	select {
	case <-h.triggered:
	default:
		return false, nil
	}
	select {
	case err := <-h.done:
		return true, err
	case <-time.After(timeout):
		return true, fmt.Errorf("hook did not finish within %s", timeout)
	}
}

// installedCLISeatFixture is the two-turn, single-attested-session fixture.
type installedCLISeatFixture struct {
	RepositoryID string
	RunID        string
	Role         string
	Lane         string
	SeatSession  string
	RepoRoot     string
	Job1ID       string
	Job2ID       string
	Artifact1    string // logical_name
	Artifact2    string // logical_name
}

func seedInstalledCLISeatFixture(t *testing.T, ctx context.Context, runner db.Runner, repositoryID, adapter string) *installedCLISeatFixture {
	t.Helper()

	repoRoot := t.TempDir()
	role := "author"
	lane := adapter
	runID := "run_" + repositoryID
	seat := "sess_seat_" + repositoryID

	docDir := filepath.Join(repoRoot, "docs")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	path1 := "docs/turn-one.md"
	path2 := "docs/turn-two.md"
	for _, p := range []string{path1, path2} {
		if err := os.WriteFile(filepath.Join(repoRoot, p), []byte(fixtureArtifactBody()), 0o644); err != nil {
			t.Fatalf("write artifact %s: %v", p, err)
		}
	}

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'conformance',$5,16,'active')`,
		repositoryID, fixtureRepoIdentity, repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("seed repository: %v", err)
	}

	workflow := map[string]any{
		"workflow_id": fixtureWorkflowID,
		"roles": map[string]any{
			role: map[string]any{"summary": "draft two small documents across two turns"},
		},
		"lanes": map[string]any{
			lane: map[string]any{
				"display_model": adapter,
				"capabilities":  []any{"claim", "write"},
			},
		},
		"jobs": []any{
			map[string]any{"id": "turn_one", "interrogable": false},
			map[string]any{"id": "turn_two", "interrogable": false},
		},
	}
	seedFixtureRun(t, ctx, runner, repositoryID, runID, workflow)

	writeScope := map[string]any{"mode": "document_only", "allowed_paths": []any{"docs"}}
	job1 := "job1_" + repositoryID
	job2 := "job2_" + repositoryID
	expected1 := []any{map[string]any{"logical_name": "turn_one_note", "kind": "progress_note", "path": path1, "required": true}}
	expected2 := []any{map[string]any{"logical_name": "turn_two_note", "kind": "progress_note", "path": path2, "required": true}}
	// Two distinct workflow jobs so the (repo, run, workflow_job_id, attempt)
	// uniqueness holds; both target the same role+lane so the one seat claims
	// each turn in sequence. The task_prompt is deliberately mechanical so the
	// gate measures session continuity (#95), NOT the lane's free reasoning: the
	// artifact already exists on disk; the lane only publishes it and completes.
	req1 := seatTaskRequirements(1, "turn_one_note", path1)
	req2 := seatTaskRequirements(2, "turn_two_note", path2)
	seedSeatJob(t, ctx, runner, repositoryID, runID, job1, "turn_one", role, lane, writeScope, expected1, req1, now)
	seedSeatJob(t, ctx, runner, repositoryID, runID, job2, "turn_two", role, lane, writeScope, expected2, req2, now.Add(time.Millisecond))

	// The seat session the supervisor would create before launching the lane —
	// active + attested via the current process pid (lanehealth treats it live).
	seedFixtureSession(t, ctx, runner, repositoryID, runID, seat, role, lane, []string{"claim", "write"}, 1)
	attestFixtureSession(t, ctx, runner, repositoryID, runID, seat, lane)

	return &installedCLISeatFixture{
		RepositoryID: repositoryID,
		RunID:        runID,
		Role:         role,
		Lane:         lane,
		SeatSession:  seat,
		RepoRoot:     repoRoot,
		Job1ID:       job1,
		Job2ID:       job2,
		Artifact1:    "turn_one_note",
		Artifact2:    "turn_two_note",
	}
}

// seatTaskRequirements builds a mechanical, exploration-forbidding task_prompt +
// objective for one turn. The gate measures session continuity across turns
// (#95), not the lane's reasoning, so the instruction is the narrowest possible:
// publish a file that already exists, then complete. This keeps a real (slow,
// exploratory) LLM CLI on a deterministic two-call path.
func seatTaskRequirements(turn int, logicalName, repoPath string) map[string]any {
	directive := fmt.Sprintf(
		"This is turn %d of a 2-turn conformance check. The artifact already exists on disk at %q — do NOT write, edit, list, read, or explore any files, and do NOT run shell commands. Do EXACTLY two MCP tool calls, in order: (1) artifact.publish with this packet's job_id and lease_id, kind=\"progress_note\", logical_name=%q, path=%q; (2) work.complete with the same job_id and lease_id and a one-line summary. Then call work.await_packet again and handle the next packet the same way. Take no other action.",
		turn, repoPath, logicalName, repoPath)
	return map[string]any{
		"objective":   fmt.Sprintf("Publish the existing artifact %s and complete turn %d. Do nothing else.", logicalName, turn),
		"task_prompt": map[string]any{"inline": directive},
	}
}

// seedSeatJob seeds one queued job + its pending work queue_message for the
// installed-CLI seat fixture. It mirrors seedFixtureJob but parameterizes the
// workflow_job_id (so one run carries two distinct turns) and the
// capability_requirements_json (so the lane gets a crisp task_prompt). The job
// is a draft (work.complete finalizes it; no verdict needed).
func seedSeatJob(t *testing.T, ctx context.Context, runner db.Runner, repositoryID, runID, jobID, workflowJobID, role, lane string, writeScope map[string]any, expectedArtifacts []any, requirements map[string]any, now time.Time) {
	t.Helper()
	writeScopeArg, err := db.JSONBArg(runner, writeScope)
	if err != nil {
		t.Fatalf("encode write_scope: %v", err)
	}
	expectedArg, err := db.JSONBArg(runner, expectedArtifacts)
	if err != nil {
		t.Fatalf("encode expected_artifacts: %v", err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": lane})
	if err != nil {
		t.Fatalf("encode lane_selector: %v", err)
	}
	requirementsArg, err := db.JSONBArg(runner, requirements)
	if err != nil {
		t.Fatalf("encode capability_requirements: %v", err)
	}
	seedExec(t, ctx, runner, "seed seat job "+jobID, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, capability_requirements_json, created_at
		) VALUES ($1,$2,$3,$4,1,'queued',$5,'Conformance seat draft','draft',$6,$7::jsonb,$8::jsonb,$9::jsonb,$10::jsonb,$11)`,
		repositoryID, jobID, runID, workflowJobID, role, "idem_"+jobID,
		writeScopeArg, expectedArg, laneSelectorArg, requirementsArg, now)
	seedExec(t, ctx, runner, "seed seat queue message "+jobID, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,$7)`,
		repositoryID, "msg_"+jobID, runID, jobID, role, lane, now)
}

// syncBuffer is a goroutine-safe writer: the agent-loop tees PTY stdout and the
// receive-loop stderr from separate goroutines into the same sink.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
