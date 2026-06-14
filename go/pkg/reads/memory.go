package reads

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	RecallDefaultLimit = 8
	RecallLimitCap     = 20

	recallRankingMethod = "websearch_to_tsquery + ts_rank + recency + provenance"
	recallSourceSurface = "striatum artifact-metadata FTS"
)

var recallProvenanceWeights = map[string]float64{
	"decision":                     1.00,
	"finding":                      0.95,
	"synthesis":                    0.95,
	"handoff":                      0.90,
	"findings_ledger":              0.88,
	"operator_brief":               0.86,
	"work_plan":                    0.84,
	"collaboration_ledger":         0.82,
	"support_ledger":               0.78,
	"action_item_ledger":           0.76,
	"harness_improvement_proposal": 0.70,
	"escalation":                   0.68,
	"operator_report":              0.45,
	"test_report":                  0.42,
	"auto_finalize_gate_evidence":  0.40,
	"commit_request":               0.36,
	"pr_request":                   0.36,
	"progress_note":                0.20,
}

type RecallHit struct {
	ArtifactID    string  `json:"artifact_id"`
	RunID         string  `json:"run_id"`
	ArtifactKind  string  `json:"artifact_kind"`
	LogicalName   string  `json:"logical_name"`
	RepoPath      string  `json:"repo_path"`
	AuthorLine    string  `json:"author_line"`
	ContentSHA256 string  `json:"content_sha256"`
	CreatedAt     string  `json:"created_at"`
	Score         float64 `json:"score"`
}

type RecallMeta struct {
	Query               string `json:"query"`
	RankingMethod       string `json:"ranking_method"`
	Limit               int    `json:"limit"`
	SourceSurface       string `json:"source_surface"`
	MaxIndexedCreatedAt string `json:"max_indexed_created_at,omitempty"`
	HitCount            int    `json:"hit_count"`
	DegradedReason      string `json:"degraded_reason,omitempty"`
}

func RecallMemory(ctx context.Context, runner db.Runner, repositoryID, query string, limit int) ([]RecallHit, RecallMeta, error) {
	query = strings.TrimSpace(query)
	limit = normalizeRecallLimit(limit)
	meta := RecallMeta{
		Query:         query,
		RankingMethod: recallRankingMethod,
		Limit:         limit,
		SourceSurface: recallSourceSurface,
	}
	maxIndexed, err := recallMaxIndexedCreatedAt(ctx, runner, repositoryID)
	if err == nil {
		meta.MaxIndexedCreatedAt = maxIndexed
	}
	if query == "" {
		meta.DegradedReason = "empty_query"
		return nil, meta, nil
	}

	rows, err := collectRows(ctx, runner, `
WITH q AS (
  SELECT websearch_to_tsquery('english', $2) AS tsq
)
SELECT artifact_id,
       run_id,
       artifact_kind,
       logical_name,
       repo_path,
       author_line,
       content_sha256,
       created_at::text AS created_at,
       (
         ts_rank(search_vector, q.tsq) * 0.70 +
         (1.0 / (1.0 + GREATEST(EXTRACT(EPOCH FROM (now() - created_at)), 0) / 2592000.0)) * 0.20 +
         (`+recallProvenanceWeightSQL()+`) * 0.10
       ) AS score
  FROM striatumd.artifacts, q
 WHERE repository_id = $1
   AND search_vector @@ q.tsq
 ORDER BY score DESC, created_at DESC, artifact_id ASC
 LIMIT $3`,
		repositoryID,
		query,
		limit,
	)
	if err != nil {
		if isTSQueryParseError(err) {
			meta.DegradedReason = "query_parse_error"
			return nil, meta, nil
		}
		meta.DegradedReason = "recall_failed"
		return nil, meta, err
	}

	hits := make([]RecallHit, 0, len(rows))
	for _, row := range rows {
		hits = append(hits, RecallHit{
			ArtifactID:    fmt.Sprint(row["artifact_id"]),
			RunID:         fmt.Sprint(row["run_id"]),
			ArtifactKind:  fmt.Sprint(row["artifact_kind"]),
			LogicalName:   fmt.Sprint(row["logical_name"]),
			RepoPath:      fmt.Sprint(row["repo_path"]),
			AuthorLine:    fmt.Sprint(row["author_line"]),
			ContentSHA256: fmt.Sprint(row["content_sha256"]),
			CreatedAt:     fmt.Sprint(row["created_at"]),
			Score:         floatFrom(row["score"]),
		})
	}
	meta.HitCount = len(hits)
	return hits, meta, nil
}

func HandleRecallSearch(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	if _, ok := envelope.Params["query"]; !ok {
		return nil, rpc.NewError("schema_invalid", "recall.search requires query", nil)
	}
	query := stringParam(envelope, "query")
	hits, meta, err := RecallMemory(ctx, runner, repositoryID, query, intFrom(envelope.Params, "limit"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"rows":  hits,
		"count": len(hits),
		"meta":  meta,
	}, nil
}

func normalizeRecallLimit(limit int) int {
	if limit <= 0 {
		return RecallDefaultLimit
	}
	if limit > RecallLimitCap {
		return RecallLimitCap
	}
	return limit
}

func recallMaxIndexedCreatedAt(ctx context.Context, runner db.Runner, repositoryID string) (string, error) {
	value, err := runner.QueryScalar(ctx, `
SELECT COALESCE(MAX(created_at)::text, '')
  FROM striatumd.artifacts
 WHERE repository_id = $1`, repositoryID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func recallProvenanceWeightSQL() string {
	kinds := make([]string, 0, len(recallProvenanceWeights))
	for kind := range recallProvenanceWeights {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	var b strings.Builder
	b.WriteString("CASE artifact_kind")
	for _, kind := range kinds {
		_, _ = fmt.Fprintf(&b, " WHEN %q THEN %.2f", kind, recallProvenanceWeights[kind])
	}
	b.WriteString(" ELSE 0.50 END")
	return strings.ReplaceAll(b.String(), `"`, `'`)
}

func isTSQueryParseError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42601" || strings.Contains(pgErr.Message, "tsquery")
	}
	return strings.Contains(strings.ToLower(err.Error()), "tsquery")
}

func floatFrom(value any) float64 {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	case float32:
		return floatFrom(float64(v))
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		var out float64
		if _, err := fmt.Sscanf(v, "%f", &out); err == nil {
			return out
		}
	}
	return 0
}
