package reads

import (
	"context"
	"fmt"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
)

const doctorLaneUIDScrubbingStuckAfter = 10 * time.Minute

func doctorLaneUIDLeases(ctx context.Context, runner db.Runner, repositoryID string, now time.Time) (map[string]any, []string, []map[string]any) {
	block := map[string]any{"checked": false, "skipped": "repository_id required"}
	if repositoryID == "" {
		return block, nil, nil
	}
	rows, err := collectRows(ctx, runner, `
		SELECT lease_id, pool_uid, pool_user, generation, run_id, session_id,
		       supervisor_id, state, scrub_status, scrub_failure,
		       leased_at, scrub_started_at, returned_at
		  FROM striatumd.lane_uid_leases
		 WHERE repository_id = $1
		   AND state IN ('active','scrubbing','quarantined')
		 ORDER BY leased_at, lease_id`,
		repositoryID,
	)
	block["checked"] = true
	block["skipped"] = nil
	if err != nil {
		block["error"] = err.Error()
		return block, []string{"lane_uid_leases.read_failed: " + err.Error()}, []map[string]any{{
			"check": "lane_uid_leases_read_failed",
			"error": err.Error(),
		}}
	}
	held := []map[string]any{}
	problems := []string{}
	records := []map[string]any{}
	stuckBefore := now.UTC().Add(-doctorLaneUIDScrubbingStuckAfter)
	for _, row := range rows {
		item := map[string]any{
			"lease_id":      row["lease_id"],
			"pool_uid":      row["pool_uid"],
			"pool_user":     row["pool_user"],
			"generation":    row["generation"],
			"run_id":        row["run_id"],
			"session_id":    row["session_id"],
			"supervisor_id": row["supervisor_id"],
			"state":         row["state"],
			"scrub_status":  row["scrub_status"],
			"scrub_failure": row["scrub_failure"],
			"leased_at":     row["leased_at"],
		}
		if row["scrub_started_at"] != nil {
			item["scrub_started_at"] = row["scrub_started_at"]
		}
		held = append(held, item)
		state := stringFrom(row, "state")
		switch state {
		case "quarantined":
			record := map[string]any{"check": "lane_uid_quarantined", "lease": item}
			records = append(records, record)
			problems = append(problems, fmt.Sprintf("lane_uid_quarantined.%s: uid %v scrub failed", stringFrom(row, "lease_id"), row["pool_uid"]))
		case "scrubbing":
			if started, ok := parseTimeValue(row["scrub_started_at"]); !ok || started.Before(stuckBefore) {
				record := map[string]any{"check": "lane_uid_scrubbing_stuck", "lease": item}
				records = append(records, record)
				problems = append(problems, fmt.Sprintf("lane_uid_scrubbing_stuck.%s: uid %v has not returned", stringFrom(row, "lease_id"), row["pool_uid"]))
			}
		}
	}
	block["held_count"] = len(held)
	block["held"] = held
	block["quarantined_count"] = countLaneUIDState(held, "quarantined")
	block["scrubbing_count"] = countLaneUIDState(held, "scrubbing")
	return block, problems, records
}

func countLaneUIDState(rows []map[string]any, state string) int {
	count := 0
	for _, row := range rows {
		if stringFrom(row, "state") == state {
			count++
		}
	}
	return count
}
