package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestSupervisorPointerStoreRoundtrip locks F-store against a real Postgres
// instance. Skips when STRIATUM_PG_TEST_URL is unset so `go test ./...`
// stays hermetic in CI environments without Postgres (matches the existing
// TestAuditRecorderRaceLinearChain opt-in pattern).
func TestSupervisorPointerStoreRoundtrip(t *testing.T) {
	url := os.Getenv("STRIATUM_PG_TEST_URL")
	if url == "" {
		t.Skip("STRIATUM_PG_TEST_URL not set; skipping supervisor pointers PG test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, _, err := ConnectAndMigrate(ctx, url, "test-go-supervisor-pointers")
	if err != nil {
		t.Fatalf("connect/migrate: %v", err)
	}
	defer pool.Close()

	runner, ok := pool.Runner.(PgxRunner)
	if !ok {
		t.Fatalf("expected PgxRunner backing the pool, got %T", pool.Runner)
	}
	store := NewSupervisorPointerStore(runner.Pool)
	supID := "sup_test_pointers_001"
	row := PointerRow{
		SupervisorID:    supID,
		RepositoryID:    "repo_test_pointers",
		SessionID:       "sess_test_pointers",
		PID:             4242,
		StartedAt:       time.Now().UTC().Truncate(time.Microsecond),
		LastHeartbeatAt: time.Now().UTC().Truncate(time.Microsecond),
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
		row.State = "running"
		row.LastHeartbeatAt = time.Now().UTC().Truncate(time.Microsecond)
		if err := store.UpsertSupervisorPointer(ctx, row); err != nil {
			t.Fatalf("upsert update: %v", err)
		}
		got, err := store.GetSupervisorPointer(ctx, supID)
		if err != nil {
			t.Fatalf("get after update: %v", err)
		}
		if got.State != "running" {
			t.Fatalf("state after update: got %q want running", got.State)
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
		if !errors.Is(err, ErrSupervisorNotFound) {
			t.Fatalf("expected ErrSupervisorNotFound, got %v", err)
		}
	})

	t.Run("mark_lost_missing_returns_typed_not_found", func(t *testing.T) {
		err := store.MarkSupervisorLost(ctx, "sup_does_not_exist_ever", "noop")
		if !errors.Is(err, ErrSupervisorNotFound) {
			t.Fatalf("expected ErrSupervisorNotFound, got %v", err)
		}
	})

	t.Run("upsert_rejects_empty_supervisor_id", func(t *testing.T) {
		err := store.UpsertSupervisorPointer(ctx, PointerRow{RepositoryID: "x"})
		if err == nil {
			t.Fatal("expected error for empty supervisor_id")
		}
	})

	t.Run("upsert_rejects_empty_repository_id", func(t *testing.T) {
		err := store.UpsertSupervisorPointer(ctx, PointerRow{SupervisorID: "x"})
		if err == nil {
			t.Fatal("expected error for empty repository_id")
		}
	})
}
