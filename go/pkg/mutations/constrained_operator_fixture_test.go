package mutations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestConstrainedOperatorMediatedSurfaceFixture(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	seedConstrainedOperatorRun(t, ctx, runner, repoRoot, map[string]any{"process_execution": true})

	writeResult, err := HandleRepoWrite(ctx, runner, constrainedEnvelope("repo.write", map[string]any{
		"path":    "docs/out.txt",
		"content": "mediated\n",
	}))
	if err != nil {
		t.Fatalf("repo.write in scope: %v", err)
	}
	if writeResult["status"] != "written" {
		t.Fatalf("repo.write result = %#v", writeResult)
	}
	if body, err := os.ReadFile(filepath.Join(repoRoot, "docs", "out.txt")); err != nil || string(body) != "mediated\n" {
		t.Fatalf("written body = %q err=%v", body, err)
	}

	_, err = HandleRepoWrite(ctx, runner, constrainedEnvelope("repo.write", map[string]any{
		"path":    "src/out.txt",
		"content": "outside\n",
	}))
	assertRepoWriteRPCCode(t, err, "path_outside_scope")
	if _, statErr := os.Stat(filepath.Join(repoRoot, "src", "out.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("out-of-scope write landed: %v", statErr)
	}

	patch := repoPatchText("docs/out.txt", "mediated", "patched")
	patchPreview, err := HandleRepoPatchPreview(ctx, runner, constrainedEnvelope("repo.patch_preview", map[string]any{"patch": patch}))
	if err != nil {
		t.Fatalf("repo.patch-preview: %v", err)
	}
	if patchPreview["status"] != "previewed" {
		t.Fatalf("patch preview = %#v", patchPreview)
	}
	if body, err := os.ReadFile(filepath.Join(repoRoot, "docs", "out.txt")); err != nil || string(body) != "mediated\n" {
		t.Fatalf("patch preview mutated body = %q err=%v", body, err)
	}
	patchApply, err := HandleRepoPatchApply(ctx, runner, constrainedEnvelope("repo.patch_apply", map[string]any{"patch": patch}))
	if err != nil {
		t.Fatalf("repo.patch-apply: %v", err)
	}
	if patchApply["status"] != "applied" {
		t.Fatalf("patch apply = %#v", patchApply)
	}
	if body, err := os.ReadFile(filepath.Join(repoRoot, "docs", "out.txt")); err != nil || string(body) != "patched\n" {
		t.Fatalf("patch apply body = %q err=%v", body, err)
	}

	processResult, err := HandleProcessRun(ctx, runner, constrainedEnvelope("process.run", map[string]any{
		"process_id": "proc_allowed",
		"args":       []any{"sh", "-c", "printf ok"},
	}))
	if err != nil {
		t.Fatalf("process.run with job opt-in: %v", err)
	}
	if processResult["status"] != "exited" || processResult["stdout_bytes"] != 2 {
		t.Fatalf("process result = %#v", processResult)
	}

	reqArg, err := db.JSONBArg(runner, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET capability_requirements_json = $1::jsonb
		 WHERE repository_id = 'repo_constrained_fixture'
		   AND job_id = 'job_constrained_fixture'`, reqArg); err != nil {
		t.Fatalf("clear process opt-in: %v", err)
	}

	escapedCommand := []any{"sh", "-c", "printf escaped"}
	_, err = HandleProcessRun(ctx, runner, constrainedEnvelope("process.run", map[string]any{
		"process_id": "proc_refused",
		"args":       escapedCommand,
	}))
	assertProcessRunRPCCode(t, err, "invalid_transition")

	if _, err := HandleDecisionRecord(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_escape_decision",
		Method:        "decision.record",
		Params: map[string]any{
			"repository_id":  "repo_constrained_fixture",
			"run_id":         "run_constrained_fixture",
			"path":           "docs/decisions/PROCESS_ESCAPE.md",
			"outcome":        "accepted",
			"title":          "Permit one process escape",
			"decision_id":    "escape_process_fixture",
			"rationale":      "Exercise the constrained operator escape gate.",
			"escape_surface": "process.run",
			"escape_action":  "sha256:" + commandHash([]string{"sh", "-c", "printf escaped"}),
		},
	}); err != nil {
		t.Fatalf("record escape decision: %v", err)
	}

	escapedResult, err := HandleProcessRun(ctx, runner, constrainedEnvelope("process.run", map[string]any{
		"process_id": "proc_escaped",
		"args":       escapedCommand,
	}))
	if err != nil {
		t.Fatalf("process.run with matching escape: %v", err)
	}
	if escapedResult["status"] != "exited" || escapedResult["stdout_bytes"] != 7 {
		t.Fatalf("escaped process result = %#v", escapedResult)
	}

	var processRows int64
	if err := runner.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM striatumd.process_executions
		 WHERE repository_id = 'repo_constrained_fixture'
		   AND state = 'exited'`).Scan(&processRows); err != nil {
		t.Fatalf("count process evidence: %v", err)
	}
	if processRows != 2 {
		t.Fatalf("exited process evidence rows = %d, want 2", processRows)
	}
}

func seedConstrainedOperatorRun(t *testing.T, ctx context.Context, runner db.Runner, repoRoot string, requirements map[string]any) {
	t.Helper()
	now := time.Now().UTC()
	workflowArg, err := db.JSONBArg(runner, map[string]any{
		"workflow_id":   "wf_constrained_fixture",
		"operator_mode": "constrained",
		"lanes":         map[string]any{"codex": map[string]any{"display_model": "Codex"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "codex"})
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":            "repo_write",
		"repo_write":      true,
		"allowed_paths":   []string{"docs/"},
		"forbidden_paths": []string{".striatum/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	requirementsArg, err := db.JSONBArg(runner, requirements)
	if err != nil {
		t.Fatal(err)
	}
	packetArg, err := db.JSONBArg(runner, map[string]any{"packet_id": "wp_constrained_fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_constrained_fixture','ident_constrained_fixture',$1,$2,'repo',$3,23,'active')`,
		repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_constrained_fixture','snap_constrained_fixture','wf_constrained_fixture','sha',$1::jsonb,$2)`,
		workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ('repo_constrained_fixture','run_constrained_fixture','snap_constrained_fixture',$1,'running',$2)`,
		repoRoot, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, registered_at
		) VALUES ('repo_constrained_fixture','sess_constrained_fixture','run_constrained_fixture','implementer','codex','implementer-codex-1',1,'["write"]'::jsonb,'active',$1)`,
		now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  lane_selector_json, capability_requirements_json, state, write_scope_json,
		  idempotency_key, created_at, current_lease_id
		) VALUES ('repo_constrained_fixture','job_constrained_fixture','run_constrained_fixture','build','Build','build','implementer',
		  $1::jsonb,$2::jsonb,'running',$3::jsonb,'idem_constrained_fixture',$4,'lease_constrained_fixture')`,
		laneSelectorArg, requirementsArg, writeScopeArg, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, target_session_id,
		  payload_json, created_at, updated_at, claimed_at, current_lease_id
		) VALUES ('repo_constrained_fixture','msg_constrained_fixture','run_constrained_fixture','job_constrained_fixture','work','claimed','sess_constrained_fixture',
		  '{}'::jsonb,$1,$1,$1,'lease_constrained_fixture')`, now); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at, last_heartbeat_at
		) VALUES ('repo_constrained_fixture','lease_constrained_fixture','run_constrained_fixture','job','job_constrained_fixture','sess_constrained_fixture',
		  'active',$1,$2,$1)`, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.work_packets (
		  repository_id, packet_id, run_id, job_id, message_id, lease_id,
		  session_id, packet_json, packet_sha256, created_at
		) VALUES ('repo_constrained_fixture','wp_constrained_fixture','run_constrained_fixture','job_constrained_fixture','msg_constrained_fixture','lease_constrained_fixture',
		  'sess_constrained_fixture',$1::jsonb,'sha',$2)`, packetArg, now); err != nil {
		t.Fatalf("insert work packet: %v", err)
	}
}

func constrainedEnvelope(method string, overrides map[string]any) rpc.Envelope {
	params := map[string]any{
		"repository_id": "repo_constrained_fixture",
		"session_id":    "sess_constrained_fixture",
		"job_id":        "job_constrained_fixture",
		"lease_id":      "lease_constrained_fixture",
	}
	for key, value := range overrides {
		params[key] = value
	}
	return rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_constrained_fixture",
		Method:        method,
		Params:        params,
	}
}
