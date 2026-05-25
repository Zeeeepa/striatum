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
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, state, started_at
		) VALUES ('repo_artifact','sup_1','run_1','sess_1','codex','[]'::jsonb,$1,$2,4242,'attached',$3)`,
		repoRoot, filepath.Join(repoRoot, ".striatum", "scratch"), now); err != nil {
		t.Fatalf("insert supervisor: %v", err)
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
