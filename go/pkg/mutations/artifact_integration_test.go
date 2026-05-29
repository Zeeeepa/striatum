package mutations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestPublishArtifactUsesLaneAttestedAuthorLine(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(repoRoot, "docs", "out.md")
	if err := os.WriteFile(artifactPath, []byte("# Output\nauthor: implementer-codex-gpt-5-001\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	workflow := map[string]any{
		"workflow_id": "wf_test",
		"lanes": map[string]any{
			"codex": map[string]any{"display_model": "Codex GPT-5"},
		},
	}
	workflowArg, err := db.JSONBArg(runner, workflow)
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":          "repo_write",
		"repo_write":    true,
		"allowed_paths": []string{"docs/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_artifact','ident_artifact',$1,$2,'repo',$3,14,'active')`,
		repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_artifact','snap_1','wf_test','sha',$1::jsonb,$2)`, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ('repo_artifact','run_1','snap_1',$1,'running',$2)`, repoRoot, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at
		) VALUES ('repo_artifact','sess_1','run_1','implementer','codex','implementer-codex-1',1,'active',$1)`, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  lane_selector_json, state, write_scope_json, idempotency_key, created_at
		) VALUES ('repo_artifact','job_1','run_1','build','Build','build','implementer',
		  $1::jsonb,'running',$2::jsonb,'idem_1',$3)`, laneSelectorArg, writeScopeArg, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ('repo_artifact','lease_1','run_1','job','job_1','sess_1','active',$1,$2)`,
		now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	pid := os.Getpid()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, state, started_at
		) VALUES ('repo_artifact','sup_1','run_1','sess_1','codex','[]'::jsonb,$1,$2,$3,'attached',$4)`,
		repoRoot, filepath.Join(repoRoot, ".striatum", "scratch"), pid, now); err != nil {
		t.Fatalf("insert supervisor: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ('repo_artifact', 'sup_1', 'dsup_1', 'run_1', 'sess_1', $1, '', 'attached', $2, '{}'::jsonb)`,
		pid, now,
	); err != nil {
		t.Fatalf("insert pointer: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_supervisors (
		  daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
		  daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
		  pid_start_time, state, started_at, heartbeat_at
		) VALUES ('dsup_1', 'repo_artifact', 'run_1', 'sess_1', 'sup_1', 'inst', 'codex', '[]'::jsonb, 'sha', '/tmp', $1, '', 'attached', $2, $2)`,
		pid, now,
	); err != nil {
		t.Fatalf("insert daemon supervisor: %v", err)
	}
	result, err := HandlePublishArtifact(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_publish",
		Method:        "artifact.publish",
		Params: map[string]any{
			"repository_id": "repo_artifact",
			"session_id":    "sess_1",
			"job_id":        "job_1",
			"lease_id":      "lease_1",
			"kind":          "handoff",
			"logical_name":  "out",
			"path":          "docs/out.md",
		},
	})
	if err != nil {
		t.Fatalf("publish artifact: %v", err)
	}
	if result["status"] != "published" {
		t.Fatalf("publish result = %#v", result)
	}
}

func TestValidateSandboxJail(t *testing.T) {
	repoRoot := t.TempDir()
	outsideDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Valid path inside repo root
	res, err := ValidateSandboxJail(repoRoot, "docs/file.md")
	if err != nil {
		t.Fatalf("expected valid path inside repo, got err: %v", err)
	}
	expected := filepath.Clean(filepath.Join(repoRoot, "docs/file.md"))
	if res != expected {
		t.Fatalf("expected %v, got %v", expected, res)
	}

	// 2. Absolute target escapes
	_, err = ValidateSandboxJail(repoRoot, "/etc/passwd")
	if err == nil {
		t.Fatal("expected absolute path to fail, but succeeded")
	}

	// 3. Symlink escaping repo root
	outsideLink := filepath.Join(repoRoot, "outside_link")
	err = os.Symlink(outsideDir, outsideLink)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ValidateSandboxJail(repoRoot, "outside_link/some_file.md")
	if err == nil {
		t.Fatal("expected symlink escape to fail, but succeeded")
	}

	// 4. Symlink staying inside repo root
	internalDir := filepath.Join(repoRoot, "internal_dir")
	err = os.MkdirAll(internalDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	internalLink := filepath.Join(repoRoot, "docs/internal_link")
	err = os.Symlink(internalDir, internalLink)
	if err != nil {
		t.Fatal(err)
	}

	res, err = ValidateSandboxJail(repoRoot, "docs/internal_link/some_file.md")
	if err != nil {
		t.Fatalf("expected internal symlink to succeed, got: %v", err)
	}
	expected = filepath.Clean(filepath.Join(internalDir, "some_file.md"))
	if res != expected {
		t.Fatalf("expected %v, got %v", expected, res)
	}
}

