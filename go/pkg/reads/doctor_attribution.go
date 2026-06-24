package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
)

// doctorAttributionUnknown is the RFC 0167 P0 D7 advisory rule: non-terminal runs
// whose created_by_principal_id is NULL/unresolvable. It surfaces the load-bearing
// misattribution risk as visible, fixable debt — kept ADVISORY (a warning, never a
// hard problem) so it never blocks a dogfood, matching the doctor-as-provenance-
// guard culture. The scan rides the runs_missing_origin SECURITY DEFINER projection
// (created_by_principal_id is SELECT-revoked for the runtime role, Route 2), so a
// secretless / pre-0022 daemon skips cleanly rather than 42501-ing.
func doctorAttributionUnknown(ctx context.Context, runner db.Runner, repositoryID string) (map[string]any, []string, []map[string]any) {
	block := map[string]any{"checked": false}
	if repositoryID == "" {
		block["skipped"] = "repository_id required"
		return block, nil, nil
	}
	secret := db.AuthorityFromContext(ctx).Secret
	if secret == "" {
		block["skipped"] = "daemon authority secret unavailable"
		return block, nil, nil
	}
	rows, err := collectRows(ctx, runner,
		`SELECT run_id, state FROM striatumd.runs_missing_origin($1, $2)`,
		secret, repositoryID,
	)
	if err != nil {
		// Pre-0022 / un-adopted database: the projection is absent. Advisory only —
		// skip cleanly, never red.
		block["checked"] = true
		block["skipped"] = "runs_missing_origin projection unavailable"
		return block, nil, nil
	}
	runIDs := make([]string, 0, len(rows))
	records := make([]map[string]any, 0, len(rows))
	warnings := make([]string, 0, len(rows))
	for _, row := range rows {
		runID := stringValue(row["run_id"])
		if runID == "" {
			continue
		}
		runIDs = append(runIDs, runID)
		records = append(records, map[string]any{
			"check":  "attribution_unknown",
			"run_id": runID,
			"state":  stringValue(row["state"]),
		})
		warnings = append(warnings, "attribution_unknown."+runID+": non-terminal run has no resolvable origin operator (created_by_principal_id is NULL)")
	}
	block["checked"] = true
	block["skipped"] = nil
	block["count"] = len(runIDs)
	block["runs"] = runIDs
	return block, warnings, records
}
