package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

func TestDrainRunHelperEventsDrainsActivePointers(t *testing.T) {
	ctx := context.Background()
	tx := &recoveryDrainFakeTx{
		rows: []map[string]any{
			{"supervisor_id": "sup_1"},
			{"supervisor_id": "sup_2"},
		},
	}
	orig := recoveryDrainHelperEvents
	defer func() { recoveryDrainHelperEvents = orig }()
	calls := []string{}
	recoveryDrainHelperEvents = func(_ context.Context, gotTx db.TxRunner, repositoryID, supervisorID string, wait time.Duration) error {
		if gotTx != tx {
			t.Fatalf("drain tx = %#v, want test tx", gotTx)
		}
		if repositoryID != "repo_1" {
			t.Fatalf("repositoryID = %q", repositoryID)
		}
		if wait != 0 {
			t.Fatalf("wait = %s, want 0", wait)
		}
		calls = append(calls, supervisorID)
		return nil
	}

	result, err := drainRunHelperEvents(ctx, tx, "repo_1", "run_1", false)
	if err != nil {
		t.Fatalf("drainRunHelperEvents: %v", err)
	}
	if !strings.Contains(tx.query, "FROM striatumd.process_supervisor_pointers") ||
		!strings.Contains(tx.query, "state IN ('starting','attached')") {
		t.Fatalf("unexpected query: %s", tx.query)
	}
	if result["checked_count"] != 2 || result["drained_count"] != 2 {
		t.Fatalf("result counts = %#v", result)
	}
	if strings.Join(calls, ",") != "sup_1,sup_2" {
		t.Fatalf("drain calls = %#v", calls)
	}
}

func TestDrainRunHelperEventsDryRunDoesNotDrain(t *testing.T) {
	ctx := context.Background()
	tx := &recoveryDrainFakeTx{
		rows: []map[string]any{
			{"supervisor_id": "sup_1"},
		},
	}
	orig := recoveryDrainHelperEvents
	defer func() { recoveryDrainHelperEvents = orig }()
	recoveryDrainHelperEvents = func(context.Context, db.TxRunner, string, string, time.Duration) error {
		t.Fatal("dry run must not drain helper events")
		return nil
	}

	result, err := drainRunHelperEvents(ctx, tx, "repo_1", "run_1", true)
	if err != nil {
		t.Fatalf("drainRunHelperEvents: %v", err)
	}
	if result["checked_count"] != 1 || result["drained_count"] != 0 || result["dry_run"] != true {
		t.Fatalf("result = %#v", result)
	}
}

type recoveryDrainFakeTx struct {
	rows  []map[string]any
	query string
}

func (tx *recoveryDrainFakeTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	tx.query = sql
	return runPrepareRowsFromMaps(tx.rows), nil
}

func (tx *recoveryDrainFakeTx) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected exec")
}

func (tx *recoveryDrainFakeTx) QueryRow(context.Context, string, ...any) db.Row {
	return superviseReportFakeRow{err: errors.New("unexpected query row")}
}

func (tx *recoveryDrainFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *recoveryDrainFakeTx) Commit(context.Context) error {
	return nil
}

func (tx *recoveryDrainFakeTx) Rollback(context.Context) error {
	return nil
}
