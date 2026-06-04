package reads

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
)

// repoConcurrentRuns returns the RFC 0108 Phase 5 repo-scoped concurrent-runs
// view: every run currently `running` on the repository (the parallel fan-out),
// each with its branch, the repo-write paths it intends to touch, its lane
// sessions (operator/role/lane/state), and the OTHER active runs it collides with
// — shares a branch with, or whose repo-write scope prefix-overlaps. It is the
// read-side complement to the Phase 2/3 run.start enforcement: where those refuse
// or warn at start, this lets an operator or maintainer see the whole parallel
// surface and every live collision on one screen (the RFC 0102 attention
// principle). SELECT-only, like the rest of the dashboard/status projections.
func repoConcurrentRuns(ctx context.Context, runner db.Runner, repositoryID string) ([]map[string]any, error) {
	runRows, err := collectRows(ctx, runner, `
		SELECT r.run_id, r.branch_name, r.state, r.paused_at,
		       COALESCE(
		         jsonb_agg(j.write_scope_json) FILTER (WHERE j.job_id IS NOT NULL),
		         '[]'::jsonb) AS write_scopes
		  FROM striatumd.runs r
		  LEFT JOIN striatumd.jobs j
		    ON j.repository_id = r.repository_id AND j.run_id = r.run_id
		 WHERE r.repository_id = $1 AND r.state = 'running'
		 GROUP BY r.run_id, r.branch_name, r.state, r.paused_at, r.created_at
		 ORDER BY r.created_at, r.run_id`, repositoryID)
	if err != nil {
		return nil, err
	}
	if len(runRows) == 0 {
		return []map[string]any{}, nil
	}
	sessionRows, err := collectRows(ctx, runner, `
		SELECT s.run_id, s.role_id, s.lane_id, s.slug, s.state, s.operator_label
		  FROM striatumd.sessions s
		  JOIN striatumd.runs r
		    ON r.repository_id = s.repository_id AND r.run_id = s.run_id
		 WHERE s.repository_id = $1 AND r.state = 'running'
		 ORDER BY s.run_id, s.ordinal, s.session_id`, repositoryID)
	if err != nil {
		return nil, err
	}
	lanesByRun := map[string][]map[string]any{}
	for _, s := range sessionRows {
		runID := stringFrom(s, "run_id")
		lanesByRun[runID] = append(lanesByRun[runID], map[string]any{
			"role_id":        s["role_id"],
			"lane_id":        s["lane_id"],
			"slug":           s["slug"],
			"state":          s["state"],
			"operator_label": s["operator_label"],
		})
	}

	runIDs := make([]string, len(runRows))
	branches := make([]string, len(runRows))
	pathSets := make([][]string, len(runRows))
	out := make([]map[string]any, 0, len(runRows))
	for i, r := range runRows {
		runID := stringFrom(r, "run_id")
		branch := stringFrom(r, "branch_name")
		paths := repoWritePathsFromScopes(r["write_scopes"])
		runIDs[i] = runID
		branches[i] = branch
		pathSets[i] = paths
		lanes := lanesByRun[runID]
		if lanes == nil {
			lanes = []map[string]any{}
		}
		out = append(out, map[string]any{
			"run_id":             runID,
			"branch":             branch,
			"state":              r["state"],
			"paused":             r["paused_at"] != nil,
			"write_scope_paths":  paths,
			"lanes":              lanes,
			"lane_count":         len(lanes),
			"integration_status": "in_flight", // Phase 4 will populate real integration state
		})
	}

	// Collisions across the active runs: a shared branch is a definite collision
	// (Phase 3 refuses it at start); a repo-write scope prefix-overlap is the
	// potential merge conflict (Phase 3 warns). Surfacing both here is the read
	// view of exactly what run.start enforces.
	for i := range out {
		collisions := make([]map[string]any, 0)
		for j := range runIDs {
			if i == j {
				continue
			}
			if branches[i] != "" && branches[i] == branches[j] {
				collisions = append(collisions, map[string]any{
					"run_id": runIDs[j], "kind": "branch", "detail": branches[i],
				})
				continue
			}
			if path := firstScopeOverlap(pathSets[i], pathSets[j]); path != "" {
				collisions = append(collisions, map[string]any{
					"run_id": runIDs[j], "kind": "write_scope", "detail": path,
				})
			}
		}
		out[i]["collisions"] = collisions
		out[i]["collides"] = len(collisions) > 0
	}
	return out, nil
}

// repoWritePathsFromScopes extracts the deduped, normalized union of allowed_paths
// from a run's aggregated write_scope_json values, counting only repo-write scopes.
func repoWritePathsFromScopes(value any) []string {
	set := map[string]bool{}
	for _, item := range decodeJSONArray(value) {
		if !isRepoWriteScope(item) {
			continue
		}
		scope := objectFromJSONish(item)
		for _, p := range decodeJSONArray(scope["allowed_paths"]) {
			if text, ok := p.(string); ok {
				if clean := normalizeReadScopePath(text); clean != "" {
					set[clean] = true
				}
			}
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// decodeJSONArray returns value as a []any, json-decoding it first when it is a
// raw jsonb scalar (pgx returns jsonb columns as []byte/string).
func decodeJSONArray(value any) []any {
	switch item := value.(type) {
	case []any:
		return item
	case []byte:
		var decoded []any
		_ = json.Unmarshal(item, &decoded)
		return decoded
	case string:
		var decoded []any
		_ = json.Unmarshal([]byte(item), &decoded)
		return decoded
	default:
		return nil
	}
}

// normalizeReadScopePath cleans a declared scope path for prefix comparison
// (repo-relative, no leading "./", no trailing "/"); returns "" for an unusable
// path (absolute or escaping).
func normalizeReadScopePath(p string) string {
	p = strings.TrimSpace(filepath.ToSlash(p))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/")
	if p == "" || p == "." || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return ""
	}
	return p
}

// firstScopeOverlap returns the first path in a that prefix-overlaps any path in
// b (equal, or one contains the other), or "" when disjoint.
func firstScopeOverlap(a, b []string) string {
	for _, pa := range a {
		for _, pb := range b {
			if scopePrefixOverlap(pa, pb) {
				return pa
			}
		}
	}
	return ""
}

func scopePrefixOverlap(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
