package db

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestAuditRecorderRaceLinearChain locks F4: concurrent goroutines calling
// RecordRPC against a real Postgres database must produce a linear hash
// chain. The test is opt-in on STRIATUM_PG_TEST_URL so `go test ./...`
// stays hermetic in CI environments without Postgres.
func TestAuditRecorderRaceLinearChain(t *testing.T) {
	url := os.Getenv("STRIATUM_PG_TEST_URL")
	if url == "" {
		t.Skip("STRIATUM_PG_TEST_URL not set; skipping pgx audit race test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, _, err := ConnectAndMigrate(ctx, url, "test-go-audit")
	if err != nil {
		t.Fatalf("connect/migrate: %v", err)
	}
	defer pool.Close()

	recorder := AuditRecorder{Runner: pool.Runner, DaemonVersion: "test-go-audit"}
	concurrency := 8
	perWorker := 4
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency*perWorker)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				env := rpc.Envelope{
					SchemaVersion: rpc.SupportedEnvelopeVersion,
					RequestID:     fmt.Sprintf("req_race_%d_%d", worker, j),
					Method:        "daemon.describe",
					Params:        map[string]any{},
				}
				auth := rpc.AuthContext{Decision: "allowed", Capability: rpc.CapabilityRead}
				resp := rpc.OKResponse(env.RequestID, map[string]any{"x": j}, "")
				if _, err := recorder.RecordRPC(ctx, env, auth, resp); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("RecordRPC failed under concurrency: %v", err)
	}

	rows, err := loadAuditChainForTest(ctx, pool.Runner)
	if err != nil {
		t.Fatalf("load audit chain: %v", err)
	}
	if len(rows) < concurrency*perWorker {
		t.Fatalf("expected at least %d audit rows, got %d", concurrency*perWorker, len(rows))
	}
	seenPrev := map[string]struct{}{}
	for i, row := range rows {
		if i > 0 && row.previousHash != rows[i-1].rowHash {
			t.Fatalf("hash chain broken at index %d: previous_hash=%q want %q", i, row.previousHash, rows[i-1].rowHash)
		}
		if row.previousHash != "" {
			if _, dup := seenPrev[row.previousHash]; dup {
				t.Fatalf("duplicate previous_hash %q at index %d", row.previousHash, i)
			}
			seenPrev[row.previousHash] = struct{}{}
		}
	}
}

type auditChainRow struct {
	auditID      int64
	rowHash      string
	previousHash string
}

func loadAuditChainForTest(ctx context.Context, runner Runner) ([]auditChainRow, error) {
	pr, ok := runner.(PgxRunner)
	if !ok {
		return nil, fmt.Errorf("runner is not PgxRunner")
	}
	queryRows, err := pr.Pool.Query(
		ctx,
		"SELECT audit_id, row_hash, COALESCE(previous_hash, '') FROM striatumd.audit_log ORDER BY audit_id",
	)
	if err != nil {
		return nil, err
	}
	defer queryRows.Close()
	result := []auditChainRow{}
	for queryRows.Next() {
		var row auditChainRow
		if err := queryRows.Scan(&row.auditID, &row.rowHash, &row.previousHash); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, queryRows.Err()
}
