package reads

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type recallFakeRunner struct {
	query string
	args  []any
	rows  []map[string]any
	err   error
}

func (r *recallFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("recall must be read-only")
}

func (r *recallFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return dashboardAllFakeRow{}
}

func (r *recallFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "2026-06-14 00:00:00+00", nil
}

func (r *recallFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("recall must not open a transaction")
}

func (r *recallFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.query = sql
	r.args = args
	if r.err != nil {
		return nil, r.err
	}
	return dashboardAllRowsFromMaps(r.rows), nil
}

func TestRecallMemoryBuildsMetadataOnlyFTSQuery(t *testing.T) {
	runner := &recallFakeRunner{rows: []map[string]any{
		{
			"artifact_id":    "art_1",
			"run_id":         "run_1",
			"artifact_kind":  "decision",
			"logical_name":   "D179",
			"repo_path":      "docs/decisions/decision-log.md",
			"author_line":    "author: operator",
			"content_sha256": "abc",
			"created_at":     "2026-06-14 00:00:00+00",
			"score":          float64(0.9),
		},
	}}

	hits, meta, err := RecallMemory(context.Background(), runner, "repo_1", "RFC 0119", 999)
	if err != nil {
		t.Fatalf("RecallMemory: %v", err)
	}
	if len(hits) != 1 || hits[0].ArtifactID != "art_1" || hits[0].Score != 0.9 {
		t.Fatalf("hits = %#v", hits)
	}
	if meta.Limit != RecallLimitCap || meta.HitCount != 1 || meta.RankingMethod == "" || meta.SourceSurface == "" {
		t.Fatalf("meta = %#v", meta)
	}
	for _, needle := range []string{"websearch_to_tsquery('english'", "ts_rank(search_vector", "ORDER BY score DESC, created_at DESC, artifact_id ASC"} {
		if !strings.Contains(runner.query, needle) {
			t.Fatalf("query missing %q:\n%s", needle, runner.query)
		}
	}
	if len(runner.args) != 3 || runner.args[0] != "repo_1" || runner.args[1] != "RFC 0119" || runner.args[2] != RecallLimitCap {
		t.Fatalf("args = %#v", runner.args)
	}
}

func TestRecallMemoryEmptyQueryDegradesSoftly(t *testing.T) {
	runner := &recallFakeRunner{}
	hits, meta, err := RecallMemory(context.Background(), runner, "repo_1", " \t", 0)
	if err != nil {
		t.Fatalf("RecallMemory: %v", err)
	}
	if len(hits) != 0 || meta.DegradedReason != "empty_query" || runner.query != "" {
		t.Fatalf("hits=%#v meta=%#v query=%q", hits, meta, runner.query)
	}
}

func TestHandleRecallSearchRequiresQueryKey(t *testing.T) {
	_, err := HandleRecallSearch(context.Background(), &recallFakeRunner{}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1"},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %v, want rpc error", err)
	}
	if rpcErr.Code != "schema_invalid" {
		t.Fatalf("code = %q", rpcErr.Code)
	}
}
