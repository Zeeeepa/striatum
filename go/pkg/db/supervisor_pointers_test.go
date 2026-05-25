package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestSupervisorPointerStoreRoundtrip locks F-store against a real Postgres
// instance.
func TestSupervisorPointerStoreRoundtrip(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_test_pointers','ident_pointers','/tmp/repo','/tmp/repo/.striatum','repo',$1,14,'active')`,
		now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_test_pointers','snap_pointers','wf','sha','{}'::jsonb,$1)`, now); err != nil {
		t.Fatalf("insert workflow snapshot: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ('repo_test_pointers','run_pointers','snap_pointers','/tmp/repo','running',$1)`, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at
		) VALUES ('repo_test_pointers','sess_test_pointers','run_pointers','implementer','codex','slug',1,'active',$1)`, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	runner, ok := pool.Runner.(db.PgxRunner)
	if !ok {
		t.Fatalf("expected PgxRunner backing the pool, got %T", pool.Runner)
	}
	store := db.NewSupervisorPointerStore(runner.Pool)
	supID := "sup_test_pointers_001"
	row := db.PointerRow{
		SupervisorID:    supID,
		RepositoryID:    "repo_test_pointers",
		SessionID:       "sess_test_pointers",
		PID:             4242,
		StartedAt:       now,
		LastHeartbeatAt: now,
		StdinPipePath:   "/tmp/striatum-test/fifo",
		State:           "starting",
	}

	t.Run("upsert_inserts", func(t *testing.T) {
		if err := store.UpsertSupervisorPointer(ctx, row); err != nil {
			t.Fatalf("upsert insert: %v", err)
		}
		got, err := store.GetSupervisorPointer(ctx, supID)
		if err != nil {
			t.Fatalf("get after insert: %v", err)
		}
		if got.SupervisorID != row.SupervisorID || got.RepositoryID != row.RepositoryID {
			t.Fatalf("mismatch on identity: got %+v want %+v", got, row)
		}
		if got.PID != row.PID {
			t.Fatalf("pid: got %d want %d", got.PID, row.PID)
		}
		if got.State != "starting" {
			t.Fatalf("state: got %q want starting", got.State)
		}
	})

	t.Run("upsert_updates_existing", func(t *testing.T) {
		row.State = "attached"
		row.LastHeartbeatAt = time.Now().UTC().Truncate(time.Microsecond)
		if err := store.UpsertSupervisorPointer(ctx, row); err != nil {
			t.Fatalf("upsert update: %v", err)
		}
		got, err := store.GetSupervisorPointer(ctx, supID)
		if err != nil {
			t.Fatalf("get after update: %v", err)
		}
		if got.State != "attached" {
			t.Fatalf("state after update: got %q want attached", got.State)
		}
	})

	t.Run("mark_lost_sets_state_and_reason", func(t *testing.T) {
		if err := store.MarkSupervisorLost(ctx, supID, "process_exited"); err != nil {
			t.Fatalf("mark lost: %v", err)
		}
		got, err := store.GetSupervisorPointer(ctx, supID)
		if err != nil {
			t.Fatalf("get after lost: %v", err)
		}
		if got.State != "lost" {
			t.Fatalf("state: got %q want lost", got.State)
		}
		if got.LostReason != "process_exited" {
			t.Fatalf("lost_reason: got %q want process_exited", got.LostReason)
		}
	})

	t.Run("get_missing_returns_typed_not_found", func(t *testing.T) {
		_, err := store.GetSupervisorPointer(ctx, "sup_does_not_exist_ever")
		if !errors.Is(err, db.ErrSupervisorNotFound) {
			t.Fatalf("expected ErrSupervisorNotFound, got %v", err)
		}
	})

	t.Run("mark_lost_missing_returns_typed_not_found", func(t *testing.T) {
		err := store.MarkSupervisorLost(ctx, "sup_does_not_exist_ever", "noop")
		if !errors.Is(err, db.ErrSupervisorNotFound) {
			t.Fatalf("expected ErrSupervisorNotFound, got %v", err)
		}
	})

	t.Run("upsert_rejects_empty_supervisor_id", func(t *testing.T) {
		err := store.UpsertSupervisorPointer(ctx, db.PointerRow{RepositoryID: "x"})
		if err == nil {
			t.Fatal("expected error for empty supervisor_id")
		}
	})

	t.Run("upsert_rejects_empty_repository_id", func(t *testing.T) {
		err := store.UpsertSupervisorPointer(ctx, db.PointerRow{SupervisorID: "x"})
		if err == nil {
			t.Fatal("expected error for empty repository_id")
		}
	})
}
