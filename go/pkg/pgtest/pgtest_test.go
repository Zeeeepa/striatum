package pgtest

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPrivilegeRevocation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	privileged, unprivileged := Pools(t)

	// 1. Set up a record as superuser (privileged) to avoid Revoke rules on Insert
	now := time.Now().UTC()
	err := privileged.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
			repository_id, repo_identity, repo_root, state_db_path, display_name, registered_at, last_schema_version, state
		) VALUES ('repo_priv','ident_priv','/tmp/repo','/tmp/db','Repo', $1, 17, 'active')
	`, now)
	if err != nil {
		t.Fatalf("failed to insert repository: %v", err)
	}

	err = privileged.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
			repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_priv','snap_priv','wf_priv','sha','{}'::jsonb, $1)
	`, now)
	if err != nil {
		t.Fatalf("failed to insert snapshot: %v", err)
	}

	err = privileged.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
			repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ('repo_priv', 'run_priv', 'snap_priv', '/tmp/repo', 'running', $1)
	`, now)
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	err = privileged.Runner.Exec(ctx, `
		INSERT INTO striatumd.events (
			repository_id, run_id, event_type, payload_json, created_at
		) VALUES ('repo_priv', 'run_priv', 'test', '{}'::jsonb, $1)
	`, now)
	if err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	err = privileged.Runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
			repository_id, run_id, artifact_id, logical_name, artifact_kind, repo_path, content_sha256, size_bytes, publish_mode, created_at, author_line
		) VALUES ('repo_priv', 'run_priv', 'art_1', 'out', 'handoff', 'docs/out.md', 'sha', 100, 'create', $1, 'operator')
	`, now)
	if err != nil {
		t.Fatalf("failed to insert artifact: %v", err)
	}

	// 2. Try to update event under unprivileged pool
	err = unprivileged.Runner.Exec(ctx, `
		UPDATE striatumd.events SET event_type = 'hack'
	`)
	if err == nil {
		t.Fatal("expected UPDATE on events under unprivileged pool to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "42501") {
		t.Fatalf("expected insufficient privilege error (42501), got: %v", err)
	}

	// 3. Try to delete event under unprivileged pool
	err = unprivileged.Runner.Exec(ctx, `
		DELETE FROM striatumd.events
	`)
	if err == nil {
		t.Fatal("expected DELETE on events under unprivileged pool to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "42501") {
		t.Fatalf("expected insufficient privilege error (42501), got: %v", err)
	}

	// 4. Try to update artifact under unprivileged pool
	err = unprivileged.Runner.Exec(ctx, `
		UPDATE striatumd.artifacts SET logical_name = 'hack' WHERE artifact_id = 'art_1'
	`)
	if err == nil {
		t.Fatal("expected UPDATE on artifacts under unprivileged pool to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "42501") {
		t.Fatalf("expected insufficient privilege error (42501), got: %v", err)
	}

	// 5. Try to delete artifact under unprivileged pool
	err = unprivileged.Runner.Exec(ctx, `
		DELETE FROM striatumd.artifacts WHERE artifact_id = 'art_1'
	`)
	if err == nil {
		t.Fatal("expected DELETE on artifacts under unprivileged pool to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "42501") {
		t.Fatalf("expected insufficient privilege error (42501), got: %v", err)
	}
}
