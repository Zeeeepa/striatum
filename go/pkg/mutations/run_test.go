package mutations

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// #123: gitBranchExists must correctly report whether a local branch exists.
func TestGitBranchExists(t *testing.T) {
	// Create a real git repo in a temp dir so we can assert on branch presence.
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")
	run("commit", "--allow-empty", "-m", "init")

	if !gitBranchExists(dir, "main") {
		t.Error("gitBranchExists should return true for existing branch 'main'")
	}
	if gitBranchExists(dir, "non-existent-branch") {
		t.Error("gitBranchExists should return false for branch that does not exist")
	}

	// Create another branch and confirm it is detected.
	run("branch", "feature-x")
	if !gitBranchExists(dir, "feature-x") {
		t.Error("gitBranchExists should return true for newly created branch 'feature-x'")
	}
}

// TestStoredJobTypeMapsAuthoringTypesToDBEnum proves run.prepare projects the
// workflow-authoring job `type` onto the jobs_job_type_check storage enum:
// phase_synthesis -> review and the RFC 0094 §2 work-packet `proposal` type ->
// build (the fog_of_war_review withheld deliverable), an empty type -> generic,
// and every already-valid type passes through. The DB CHECK only accepts
// draft/review/ledger/synthesis/build/test/human_checkpoint/generic, so an
// unmapped `proposal` would fail the jobs INSERT at prepare.
func TestStoredJobTypeMapsAuthoringTypesToDBEnum(t *testing.T) {
	cases := map[string]string{
		"":                 "generic",
		"phase_synthesis":  "review",
		"proposal":         "build",
		"build":            "build",
		"review":           "review",
		"synthesis":        "synthesis",
		"draft":            "draft",
		"human_checkpoint": "human_checkpoint",
	}
	dbEnum := map[string]bool{
		"draft": true, "review": true, "ledger": true, "synthesis": true,
		"build": true, "test": true, "human_checkpoint": true, "generic": true,
	}
	for in, want := range cases {
		if got := storedJobType(in); got != want {
			t.Errorf("storedJobType(%q) = %q, want %q", in, got, want)
		}
		if !dbEnum[storedJobType(in)] {
			t.Errorf("storedJobType(%q) = %q is not in jobs_job_type_check", in, storedJobType(in))
		}
	}
}

// TestAutoConfirmBranchEnsuresRefWithoutCheckout exercises the git helper used
// by branch.confirm/run.prepare: when the suggested branch does not exist, the
// daemon creates the ref without moving the operator's primary checkout.
func TestAutoConfirmBranchEnsuresRefWithoutCheckout(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	currentBranch := func() string {
		t.Helper()
		cmd := exec.Command("git", "branch", "--show-current")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git branch --show-current: %v\n%s", err, out)
		}
		return string(out)
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")
	run("commit", "--allow-empty", "-m", "init")

	// Suggested branch does not exist yet: create it.
	suggested := "striatum/auto-test"
	if gitBranchExists(dir, suggested) {
		t.Fatalf("branch %q must not exist before test", suggested)
	}
	branch, created, err := gitEnsureBranchRef(dir, suggested, "")
	if err != nil {
		t.Fatalf("gitEnsureBranchRef: %v", err)
	}
	if branch != suggested {
		t.Errorf("branch = %q, want %q", branch, suggested)
	}
	if !created {
		t.Error("expected created=true for a new branch")
	}
	if !gitBranchExists(dir, suggested) {
		t.Error("branch must exist after creation")
	}
	if got := strings.TrimSpace(currentBranch()); got != "main" {
		t.Fatalf("current branch moved to %q, want main", got)
	}

	// Second call to the same branch: ref already exists, still no checkout.
	_, created2, err := gitEnsureBranchRef(dir, suggested, "")
	if err != nil {
		t.Fatalf("gitEnsureBranchRef (second call): %v", err)
	}
	if created2 {
		t.Error("second call must not re-create; expected created=false")
	}
	if got := strings.TrimSpace(currentBranch()); got != "main" {
		t.Fatalf("second call moved current branch to %q, want main", got)
	}
}

// #77: run.prepare gates review/phase_synthesis edges on a clearing verdict,
// EXCEPT edges into an adjudicator (phase_synthesis), which stay ungated so the
// adjudicator can absorb a reviewer's needs_revision dissent.
func TestEdgeRequiresClearingVerdictExemptsAdjudicatorInbound(t *testing.T) {
	wf := map[string]any{"jobs": []any{
		map[string]any{"id": "cross_exam", "type": "review"},
		map[string]any{"id": "adjudicate", "type": "phase_synthesis"},
		map[string]any{"id": "implement", "type": "build"},
		map[string]any{"id": "proposal", "type": "draft"},
	}}
	cases := []struct {
		from, to string
		want     bool
	}{
		{"cross_exam", "adjudicate", false}, // review -> adjudicator: ungated (absorbs)
		{"cross_exam", "implement", true},   // review -> build: gated
		{"adjudicate", "implement", true},   // phase_synthesis -> build: gated
		{"adjudicate", "adjudicate", false}, // phase_synthesis -> phase_synthesis: ungated
		{"proposal", "cross_exam", false},   // draft -> review: not verdict-capable source
	}
	for _, tc := range cases {
		if got := edgeRequiresClearingVerdict(wf, tc.from, tc.to); got != tc.want {
			t.Errorf("edgeRequiresClearingVerdict(%s->%s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestDecisionRecordCanMarkCompletedRunCompromised(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	repoID := "repo_compromise_decision"
	runID := "run_compromise_decision"
	seedCompletedRunForDecision(t, ctx, runner, repoID, runID, repoRoot)

	result, err := HandleDecisionRecord(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id":               runID,
		"path":                 "docs/decisions/compromised.md",
		"outcome":              "accepted",
		"title":                "Invalidate compromised provenance",
		"decision_id":          "dec_compromised",
		"rationale":            "Review provenance was compromised; replacement run required.",
		"mark_run_compromised": true,
	}))
	if err != nil {
		t.Fatalf("decision.record: %v", err)
	}
	transition := asMap(result["run_state_transition"])
	if transition["from"] != "completed" || transition["to"] != "compromised" {
		t.Fatalf("run_state_transition = %#v, want completed -> compromised", result["run_state_transition"])
	}
	run, err := oneRow(ctx, runner, `
		SELECT state, stop_reason
		  FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	if run["state"] != "compromised" || run["stop_reason"] != "provenance_compromised" {
		t.Fatalf("run row = %#v, want compromised provenance stop", run)
	}
	if _, err := oneRow(ctx, runner, `
		SELECT event_id
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.compromised'`, repoID, runID); err != nil {
		t.Fatalf("run.compromised event not recorded: %v", err)
	}
}

func TestCompletedReviewCompromiseIsReplacementRunOnly(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	repoID := "repo_completed_review_compromise"
	runID := "run_completed_review_compromise"
	reviewJobID := "job_completed_review_compromise"
	seedCompletedRunForDecision(t, ctx, runner, repoID, runID, repoRoot)
	seedCompletedReviewJob(t, ctx, runner, repoID, runID, reviewJobID)

	_, err := HandleRunRetryJob(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"job_id": reviewJobID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" || !strings.Contains(rpcErr.Message, "replacement run") {
		t.Fatalf("retry completed review error = %v, want replacement-run guidance", err)
	}

	result, err := HandleDecisionRecord(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id":               runID,
		"path":                 "docs/decisions/compromised-review.md",
		"outcome":              "accepted",
		"title":                "Invalidate compromised completed review",
		"decision_id":          "dec_compromised_review",
		"rationale":            "Completed review provenance was compromised; replacement run required.",
		"mark_run_compromised": true,
	}))
	if err != nil {
		t.Fatalf("decision.record: %v", err)
	}
	transition := asMap(result["run_state_transition"])
	if transition["from"] != "completed" || transition["to"] != "compromised" {
		t.Fatalf("run_state_transition = %#v, want completed -> compromised", result["run_state_transition"])
	}
	if got := jobState(t, ctx, runner, repoID, reviewJobID); got != "completed" {
		t.Fatalf("completed review job state = %q, want completed (replacement-run-only path does not revive jobs)", got)
	}
	if _, err := oneRow(ctx, runner, `
		SELECT event_id
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.compromised'
		   AND payload_json->>'stop_reason' = 'provenance_compromised'`, repoID, runID); err != nil {
		t.Fatalf("run.compromised evidence missing: %v", err)
	}
}

func TestRunResumeCompletedRunGivesCompromiseGuidance(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	repoID := "repo_resume_completed_guidance"
	runID := "run_resume_completed_guidance"
	seedCompletedRunForDecision(t, ctx, runner, repoID, runID, repoRoot)

	_, err := HandleRunResume(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("run.resume error = %v, want invalid_transition", err)
	}
	if !strings.Contains(rpcErr.Message, "--mark-run-compromised") {
		t.Fatalf("run.resume message = %q, want compromised-run guidance", rpcErr.Message)
	}
	if strings.Contains(rpcErr.Message, "retry_job") {
		t.Fatalf("run.resume message = %q, must not point completed runs at retry_job", rpcErr.Message)
	}
}

// TestRunResumeReportsDaemonAutoSpawnDrive (#383 item 1): a paused auto_spawn run
// resumed under an enabled daemon scheduler reports drive=daemon_auto_spawn and no
// next_action — the scheduler re-adopts it, so no manual `run drive` is required.
func TestRunResumeReportsDaemonAutoSpawnDrive(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, runID := "repo_resume_drive_daemon", "run_resume_drive_daemon"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_resume_daemon")
	t.Setenv("STRIATUM_AUTO_SPAWN_SCHEDULER", "1")
	if _, err := HandleRunPause(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID})); err != nil {
		t.Fatalf("run.pause: %v", err)
	}

	result, err := HandleRunResume(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID}))
	if err != nil {
		t.Fatalf("run.resume: %v", err)
	}
	if result["status"] != "resumed" {
		t.Fatalf("status = %v, want resumed", result["status"])
	}
	if result["drive"] != "daemon_auto_spawn" {
		t.Fatalf("drive = %v, want daemon_auto_spawn", result["drive"])
	}
	if result["next_action"] != nil {
		t.Fatalf("next_action = %v, want absent when the daemon re-drives", result["next_action"])
	}
	if msg, _ := result["drive_message"].(string); !strings.Contains(msg, "scheduler will re-adopt") {
		t.Fatalf("drive_message = %q, want scheduler re-adoption note", msg)
	}
}

// TestRunResumeReportsSchedulerDisabledDrive (#383 item 1): a paused auto_spawn run
// resumed while the daemon scheduler is OFF reports
// drive=auto_spawn_scheduler_disabled with a `run drive` next_action — the run has
// a grant but nothing will re-drive it until the scheduler is enabled.
func TestRunResumeReportsSchedulerDisabledDrive(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, runID := "repo_resume_drive_disabled", "run_resume_drive_disabled"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_resume_disabled")
	t.Setenv("STRIATUM_AUTO_SPAWN_SCHEDULER", "")
	if _, err := HandleRunPause(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID})); err != nil {
		t.Fatalf("run.pause: %v", err)
	}

	result, err := HandleRunResume(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID}))
	if err != nil {
		t.Fatalf("run.resume: %v", err)
	}
	if result["drive"] != "auto_spawn_scheduler_disabled" {
		t.Fatalf("drive = %v, want auto_spawn_scheduler_disabled", result["drive"])
	}
	next, _ := result["next_action"].(string)
	if !strings.Contains(next, "run drive --run-id "+runID) {
		t.Fatalf("next_action = %q, want a run drive re-invoke for %s", next, runID)
	}
}

// TestRunResumeReportsOperatorRunDrive (#383 item 1): a paused run with NO
// auto_spawn grant resumed reports drive=operator_run_drive with a `run drive`
// next_action — the scheduler never adopts a grant-less run, so the operator-side
// driver is the only home and resume says so explicitly.
func TestRunResumeReportsOperatorRunDrive(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, runID := "repo_resume_drive_operator", "run_resume_drive_operator"
	t.Setenv("STRIATUM_AUTO_SPAWN_SCHEDULER", "1") // even enabled, a grant-less run is operator-driven
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"author": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
		"jobs":        []any{map[string]any{"id": "author_draft", "type": "draft", "role_id": "author"}},
	})
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state='running', paused_at=now(), paused_reason='maint'
		 WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatalf("seed paused running run: %v", err)
	}

	result, err := HandleRunResume(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID}))
	if err != nil {
		t.Fatalf("run.resume: %v", err)
	}
	if result["drive"] != "operator_run_drive" {
		t.Fatalf("drive = %v, want operator_run_drive", result["drive"])
	}
	next, _ := result["next_action"].(string)
	if !strings.Contains(next, "run drive --run-id "+runID) {
		t.Fatalf("next_action = %q, want a run drive re-invoke", next)
	}
}

// TestSchedulerReAdoptsResumedRun (#383 item 1): the auto_spawn scheduler re-drives
// a run that was paused and then resumed. After resume clears paused_at, the
// scheduler's reconcile pass spawns the queued lane again with no manual
// `run drive` — proving the daemon-auto_spawn re-arm path the resume result
// advertises actually makes the run progress.
func TestSchedulerReAdoptsResumedRun(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, runID := "repo_sched_resume_readopt", "run_sched_resume_readopt"
	startAutoSpawnRun(t, ctx, runner, repoID, runID, "client_owner_readopt")

	// Pause holds: the scheduler launches nothing while paused.
	if _, err := HandleRunPause(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID})); err != nil {
		t.Fatalf("run.pause: %v", err)
	}
	pausedFake := &fakeSpawnExecutor{}
	pausedResult, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", pausedFake)
	if err != nil {
		t.Fatalf("schedulerSpawnOnce (paused): %v", err)
	}
	if pausedResult["status"] != "paused" || len(pausedFake.calls) != 0 {
		t.Fatalf("paused sweep = %#v / %d spawns, want paused / 0", pausedResult, len(pausedFake.calls))
	}

	// Resume lifts the hold; the next sweep re-adopts and spawns the queued lane.
	if _, err := HandleRunResume(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID})); err != nil {
		t.Fatalf("run.resume: %v", err)
	}
	resumedFake := &fakeSpawnExecutor{}
	resumedResult, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", resumedFake)
	if err != nil {
		t.Fatalf("schedulerSpawnOnce (resumed): %v", err)
	}
	if resumedResult["status"] != "active" {
		t.Fatalf("resumed sweep status = %v, want active", resumedResult["status"])
	}
	if len(resumedFake.calls) != 1 {
		t.Fatalf("resumed sweep spawned %d lanes, want 1 (the scheduler re-drives a resumed run)", len(resumedFake.calls))
	}
	if resumedFake.calls[0].role != "author" || resumedFake.calls[0].lane != "codex" {
		t.Fatalf("resumed spawn = %s/%s, want author/codex", resumedFake.calls[0].role, resumedFake.calls[0].lane)
	}
}

func TestRunCancelClosesActiveLeasedSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_run_cancel_closes_session"
	runID := "run_cancel_closes_session"
	sessionID := "sess_run_cancel_closes_session"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"author": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "author", "codex", []string{"write"}, "active")
	seedAuthorActiveWork(t, ctx, runner, repoID, runID, sessionID, "acked")

	result, err := HandleRunCancel(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"reason": "operator canceled run",
	}))
	if err != nil {
		t.Fatalf("run.cancel: %v", err)
	}
	if result["state"] != "canceled" || result["status"] != "canceled" {
		t.Fatalf("run.cancel result = %#v, want canceled", result)
	}

	session, err := oneRow(ctx, runner, `
		SELECT state, close_reason
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if session["state"] != "closed" || session["close_reason"] != "run_canceled" {
		t.Fatalf("session row = %#v, want closed/run_canceled", session)
	}
	lease, err := oneRow(ctx, runner, `
		SELECT state, release_reason
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND lease_id = $2`, repoID, "lease_work_"+sessionID)
	if err != nil {
		t.Fatalf("read lease: %v", err)
	}
	if lease["state"] != "released" || lease["release_reason"] != "run_canceled" {
		t.Fatalf("lease row = %#v, want released/run_canceled", lease)
	}
	if got := jobState(t, ctx, runner, repoID, "job_work_"+sessionID); got != "canceled" {
		t.Fatalf("job state = %q, want canceled", got)
	}
}

func TestRunCancelAlreadyCanceledClosesAnomalousActiveSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_run_cancel_already_canceled_cleanup"
	runID := "run_cancel_already_canceled_cleanup"
	sessionID := "sess_already_canceled_cleanup"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"author": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs
		   SET state = 'canceled', completed_at = NOW(), stop_reason = 'operator_canceled'
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err != nil {
		t.Fatalf("mark run canceled: %v", err)
	}
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "author", "codex", []string{"write"}, "active")

	result, err := HandleRunCancel(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
	}))
	if err != nil {
		t.Fatalf("run.cancel already-canceled cleanup: %v", err)
	}
	if result["status"] != "already_canceled" {
		t.Fatalf("run.cancel result = %#v, want already_canceled", result)
	}
	session, err := oneRow(ctx, runner, `
		SELECT state, close_reason
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if session["state"] != "closed" || session["close_reason"] != "run_canceled" {
		t.Fatalf("session row = %#v, want closed/run_canceled", session)
	}
}

func seedCompletedReviewJob(t *testing.T, ctx context.Context, runner any, repoID, runID, jobID string) {
	t.Helper()
	now := time.Now().UTC()
	execer, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		t.Fatalf("runner does not support Exec")
	}
	if err := execer.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, expected_artifacts_json, idempotency_key, created_at,
		  completed_at
		) VALUES ($1,$2,$3,'final_review',1,'completed','reviewer',
		  'Final Review','review','[]'::jsonb,'idem_'||$2,$4,$4)`,
		repoID, jobID, runID, now); err != nil {
		t.Fatalf("insert completed review job: %v", err)
	}
}

func seedCompletedRunForDecision(t *testing.T, ctx context.Context, runner any, repoID, runID, repoRoot string) {
	t.Helper()
	now := time.Now().UTC()
	execer, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		t.Fatalf("runner does not support Exec")
	}
	if err := execer.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,16,'active')`,
		repoID, "ident_"+repoID, repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := execer.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha','{}'::jsonb,$3)`, repoID, "snap_"+runID, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := execer.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at, completed_at
		) VALUES ($1,$2,$3,$4,'completed',$5,$5)`,
		repoID, runID, "snap_"+runID, repoRoot, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
}
