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

type corpusExportFakeRunner struct {
	query string
	args  []any
}

func (r *corpusExportFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("corpus export must be read-only")
}

func (r *corpusExportFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return dashboardAllFakeRow{}
}

func (r *corpusExportFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r *corpusExportFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("corpus export must not open a transaction")
}

func (r *corpusExportFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.query = sql
	r.args = args
	if !strings.Contains(sql, "artifact_kind AS kind") ||
		!strings.Contains(sql, "repo_path AS path") ||
		!strings.Contains(sql, "author_line AS byline") ||
		!strings.Contains(sql, "LIMIT 2") {
		return nil, errors.New("corpus export did not project current archive-safe artifact columns")
	}
	return dashboardAllRowsFromMaps([]map[string]any{
		{
			"artifact_id":    "art_2",
			"run_id":         "run_2",
			"kind":           "finding",
			"logical_name":   "review",
			"path":           "docs/reviews/REVIEW.md",
			"content_sha256": "def",
			"byline":         "author: reviewer",
			"published_at":   "2026-05-25T00:00:00Z",
		},
	}), nil
}

func TestHandleCorpusExportReturnsVersionedArtifactRows(t *testing.T) {
	runner := &corpusExportFakeRunner{}
	result, err := HandleCorpusExport(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_a", "limit": float64(2), "redaction_tier": "curated"},
	})
	if err != nil {
		t.Fatalf("HandleCorpusExport: %v", err)
	}
	if result["corpus_contract_version"] != 2 || result["repository_id"] != "repo_a" || result["redaction_tier"] != "curated" {
		t.Fatalf("contract header = %#v", result)
	}
	if result["row_count"] != 1 || result["limit"] != 2 {
		t.Fatalf("counts = %#v", result)
	}
	if len(runner.args) != 1 || runner.args[0] != "repo_a" {
		t.Fatalf("query args = %#v", runner.args)
	}
	rows := result["rows"].([]map[string]any)
	if rows[0]["kind"] != "finding" || rows[0]["path"] != "docs/reviews/REVIEW.md" {
		t.Fatalf("row = %#v", rows[0])
	}
}
