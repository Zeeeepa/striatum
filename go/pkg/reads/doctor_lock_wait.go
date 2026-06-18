package reads

import (
	"context"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
)

const (
	doctorLockWaitWindow              = 15 * time.Minute
	doctorLockWaitCandidateRunLimit   = 8
	doctorLockWaitEventsPerRunLimit   = 200
	doctorLockWaitAuditTailLimit      = 2000
	doctorLockWaitOutputLimit         = 20
	doctorLockWaitWarningThresholdUS  = 5_000_000
	doctorLockWaitSkippedMissingEvent = "events.lock_wait_us column not present; apply owner bundle 0014 to enable"
	doctorLockWaitSkippedMissingAudit = "audit_log.lock_wait_us column not present; apply owner bundle 0014 to enable"
)

func doctorLockWaitConvoys(ctx context.Context, runner db.Runner, repositoryID string, now time.Time) (map[string]any, map[string]any, []string, []map[string]any) {
	eventBlock := doctorLockWaitBlock("event_chain_head_lock_convoy")
	auditBlock := doctorLockWaitBlock("audit_chain_head_lock_convoy")
	columns, err := doctorLockWaitColumns(ctx, runner)
	if err != nil {
		warning := "lock_wait_convoy.column_probe_failed: " + err.Error()
		eventBlock["error"] = err.Error()
		auditBlock["error"] = err.Error()
		return eventBlock, auditBlock, []string{warning}, []map[string]any{{
			"check": "lock_wait_convoy_column_probe_failed",
			"error": err.Error(),
		}}
	}

	warnings := []string{}
	warningRecords := []map[string]any{}
	if columns["events"] {
		eventWarnings, eventRecords := doctorEventLockWaitConvoys(ctx, runner, repositoryID, now, eventBlock)
		warnings = append(warnings, eventWarnings...)
		warningRecords = append(warningRecords, eventRecords...)
	} else {
		eventBlock["checked"] = true
		eventBlock["skipped"] = doctorLockWaitSkippedMissingEvent
	}
	if columns["audit_log"] {
		auditWarnings, auditRecords := doctorAuditLockWaitConvoys(ctx, runner, repositoryID, now, auditBlock)
		warnings = append(warnings, auditWarnings...)
		warningRecords = append(warningRecords, auditRecords...)
	} else {
		auditBlock["checked"] = true
		auditBlock["skipped"] = doctorLockWaitSkippedMissingAudit
	}
	return eventBlock, auditBlock, warnings, warningRecords
}

func doctorLockWaitBlock(check string) map[string]any {
	return map[string]any{
		"checked":       false,
		"check":         check,
		"skipped":       "lock_wait_us column probe pending",
		"threshold_us":  doctorLockWaitWarningThresholdUS,
		"window_secs":   int(doctorLockWaitWindow.Seconds()),
		"warnings":      []map[string]any{},
		"warning_count": 0,
	}
}

func doctorLockWaitColumns(ctx context.Context, runner db.Runner) (map[string]bool, error) {
	rows, err := collectRows(ctx, runner, `
		SELECT table_name
		  FROM information_schema.columns
		 WHERE table_schema = 'striatumd'
		   AND table_name IN ('events', 'audit_log')
		   AND column_name = 'lock_wait_us'`)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, row := range rows {
		table := strings.TrimSpace(stringFrom(row, "table_name"))
		if table != "" {
			result[table] = true
		}
	}
	return result, nil
}

func doctorEventLockWaitConvoys(ctx context.Context, runner db.Runner, repositoryID string, now time.Time, block map[string]any) ([]string, []map[string]any) {
	if repositoryID == "" {
		block["skipped"] = "repository_id required"
		return nil, nil
	}
	block["checked"] = true
	block["skipped"] = nil
	block["candidate_run_limit"] = doctorLockWaitCandidateRunLimit
	block["events_per_run_limit"] = doctorLockWaitEventsPerRunLimit
	cutoff := now.UTC().Add(-doctorLockWaitWindow)
	rows, err := collectRows(ctx, runner, `
		WITH candidate_runs AS MATERIALIZED (
			SELECT r.run_id,
			       r.state,
			       COALESCE(r.completed_at, r.started_at, r.created_at) AS activity_at
			  FROM striatumd.runs r
			 WHERE r.repository_id = $1
			   AND (
			     r.state NOT IN ('completed','failed','canceled')
			     OR COALESCE(r.completed_at, r.started_at, r.created_at) >= $2::timestamptz
			)
			 ORDER BY CASE WHEN r.state NOT IN ('completed','failed','canceled') THEN 0 ELSE 1 END,
			          activity_at DESC,
			          r.run_id
			 LIMIT $3
		),
		tail_events AS MATERIALIZED (
			SELECT cr.run_id, e.event_id, e.event_type, e.created_at, e.lock_wait_us
			  FROM candidate_runs cr
			  CROSS JOIN LATERAL (
			    SELECT e.event_id, e.event_type, e.created_at, e.lock_wait_us
			      FROM striatumd.events e
			     WHERE e.repository_id = $1
			       AND e.run_id = cr.run_id
			     ORDER BY e.event_id DESC
			     LIMIT $4
			  ) e
		)
		SELECT run_id,
		       COUNT(*)::bigint AS samples,
		       MAX(lock_wait_us)::bigint AS max_lock_wait_us,
		       percentile_disc(0.95) WITHIN GROUP (ORDER BY lock_wait_us)::bigint AS p95_lock_wait_us,
		       MAX(created_at) AS newest_sample_at
		  FROM tail_events
		 WHERE created_at >= $2::timestamptz
		   AND lock_wait_us IS NOT NULL
		 GROUP BY run_id
		HAVING MAX(lock_wait_us) >= $5
		 ORDER BY MAX(lock_wait_us) DESC, run_id
		 LIMIT $6`,
		repositoryID, cutoff, doctorLockWaitCandidateRunLimit, doctorLockWaitEventsPerRunLimit,
		doctorLockWaitWarningThresholdUS, doctorLockWaitOutputLimit,
	)
	if err != nil {
		block["error"] = err.Error()
		return []string{"event_chain_head_lock_convoy.read_failed: " + err.Error()}, []map[string]any{{
			"check": "event_chain_head_lock_convoy_read_failed",
			"error": err.Error(),
		}}
	}
	warnings := []string{}
	records := []map[string]any{}
	for _, row := range rows {
		runID := stringFrom(row, "run_id")
		record := map[string]any{
			"check":                "event_chain_head_lock_convoy",
			"repository_id":        repositoryID,
			"run_id":               runID,
			"samples":              intFrom(row, "samples"),
			"max_lock_wait_us":     intFrom(row, "max_lock_wait_us"),
			"p95_lock_wait_us":     intFrom(row, "p95_lock_wait_us"),
			"newest_sample_at":     timestampValue(row["newest_sample_at"]),
			"threshold_us":         doctorLockWaitWarningThresholdUS,
			"window_secs":          int(doctorLockWaitWindow.Seconds()),
			"events_per_run_limit": doctorLockWaitEventsPerRunLimit,
		}
		records = append(records, record)
		warnings = append(warnings, "event_chain_head_lock_convoy."+runID+": max_lock_wait_us="+intPlaceholder(intFrom(row, "max_lock_wait_us"))+" p95_lock_wait_us="+intPlaceholder(intFrom(row, "p95_lock_wait_us")))
	}
	block["warnings"] = records
	block["warning_count"] = len(records)
	return warnings, records
}

func doctorAuditLockWaitConvoys(ctx context.Context, runner db.Runner, repositoryID string, now time.Time, block map[string]any) ([]string, []map[string]any) {
	block["checked"] = true
	block["skipped"] = nil
	block["audit_tail_limit"] = doctorLockWaitAuditTailLimit
	cutoff := now.UTC().Add(-doctorLockWaitWindow)
	rows, err := collectRows(ctx, runner, `
		WITH tail_audit AS MATERIALIZED (
			SELECT audit_id, ts, repository_id, method, lock_wait_us
			  FROM striatumd.audit_log
			 ORDER BY audit_id DESC
			 LIMIT $1
		)
		SELECT COALESCE(repository_id, '') AS repository_id,
		       method,
		       COUNT(*)::bigint AS samples,
		       MAX(lock_wait_us)::bigint AS max_lock_wait_us,
		       percentile_disc(0.95) WITHIN GROUP (ORDER BY lock_wait_us)::bigint AS p95_lock_wait_us,
		       MAX(ts) AS newest_sample_at
		  FROM tail_audit
		 WHERE ts >= $2::timestamptz
		   AND lock_wait_us IS NOT NULL
		   AND ($3 = '' OR repository_id = $3)
		 GROUP BY COALESCE(repository_id, ''), method
		HAVING MAX(lock_wait_us) >= $4
		 ORDER BY MAX(lock_wait_us) DESC, method
		 LIMIT $5`,
		doctorLockWaitAuditTailLimit, cutoff, repositoryID, doctorLockWaitWarningThresholdUS, doctorLockWaitOutputLimit,
	)
	if err != nil {
		block["error"] = err.Error()
		return []string{"audit_chain_head_lock_convoy.read_failed: " + err.Error()}, []map[string]any{{
			"check": "audit_chain_head_lock_convoy_read_failed",
			"error": err.Error(),
		}}
	}
	warnings := []string{}
	records := []map[string]any{}
	for _, row := range rows {
		rowRepoID := stringFrom(row, "repository_id")
		method := stringFrom(row, "method")
		record := map[string]any{
			"check":            "audit_chain_head_lock_convoy",
			"repository_id":    rowRepoID,
			"method":           method,
			"samples":          intFrom(row, "samples"),
			"max_lock_wait_us": intFrom(row, "max_lock_wait_us"),
			"p95_lock_wait_us": intFrom(row, "p95_lock_wait_us"),
			"newest_sample_at": timestampValue(row["newest_sample_at"]),
			"threshold_us":     doctorLockWaitWarningThresholdUS,
			"window_secs":      int(doctorLockWaitWindow.Seconds()),
			"audit_tail_limit": doctorLockWaitAuditTailLimit,
		}
		records = append(records, record)
		warnings = append(warnings, "audit_chain_head_lock_convoy."+doctorAuditLockWaitWarningScope(rowRepoID, method)+": max_lock_wait_us="+intPlaceholder(intFrom(row, "max_lock_wait_us"))+" p95_lock_wait_us="+intPlaceholder(intFrom(row, "p95_lock_wait_us")))
	}
	block["warnings"] = records
	block["warning_count"] = len(records)
	return warnings, records
}

func doctorAuditLockWaitWarningScope(repositoryID, method string) string {
	scope := strings.TrimSpace(repositoryID)
	if scope == "" {
		scope = "global"
	}
	method = strings.TrimSpace(method)
	if method == "" {
		method = "unknown"
	}
	return scope + "." + strings.NewReplacer(" ", "_", "/", "_", ":", "_").Replace(method)
}
