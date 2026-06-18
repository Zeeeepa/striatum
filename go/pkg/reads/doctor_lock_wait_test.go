package reads

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type doctorLockWaitFakeRunner struct {
	doctorFakeRunner
	columnRows []map[string]any
	eventRows  []map[string]any
	auditRows  []map[string]any
	queries    []string
}

func (r *doctorLockWaitFakeRunner) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	r.queries = append(r.queries, sql)
	switch {
	case strings.Contains(sql, "information_schema.columns"):
		return dashboardAllRowsFromMaps(r.columnRows), nil
	case strings.Contains(sql, "tail_events"):
		return dashboardAllRowsFromMaps(r.eventRows), nil
	case strings.Contains(sql, "tail_audit"):
		return dashboardAllRowsFromMaps(r.auditRows), nil
	default:
		return dashboardAllRowsFromMaps(nil), nil
	}
}

func TestDoctorLockWaitSkipsOldSchemaWithoutLockWaitQueries(t *testing.T) {
	runner := &doctorLockWaitFakeRunner{}
	eventBlock, auditBlock, warnings, records := doctorLockWaitConvoys(
		context.Background(), runner, "repo_1", time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC))
	if len(warnings) != 0 || len(records) != 0 {
		t.Fatalf("old schema produced warnings=%#v records=%#v", warnings, records)
	}
	if eventBlock["skipped"] != doctorLockWaitSkippedMissingEvent {
		t.Fatalf("event block = %#v, want missing-column skip", eventBlock)
	}
	if auditBlock["skipped"] != doctorLockWaitSkippedMissingAudit {
		t.Fatalf("audit block = %#v, want missing-column skip", auditBlock)
	}
	if len(runner.queries) != 1 || !strings.Contains(runner.queries[0], "information_schema.columns") {
		t.Fatalf("queries = %#v, want only the column probe", runner.queries)
	}
}

func TestDoctorEventLockWaitConvoyUsesBoundedPerRunTail(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	runner := &doctorLockWaitFakeRunner{
		columnRows: []map[string]any{{"table_name": "events"}},
		eventRows: []map[string]any{{
			"run_id":           "run_hot",
			"samples":          int64(3),
			"max_lock_wait_us": int64(6_000_000),
			"p95_lock_wait_us": int64(5_500_000),
			"newest_sample_at": now,
		}},
	}
	eventBlock, auditBlock, warnings, records := doctorLockWaitConvoys(context.Background(), runner, "repo_1", now)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "event_chain_head_lock_convoy.run_hot") {
		t.Fatalf("warnings = %#v, want event convoy warning", warnings)
	}
	if len(records) != 1 || records[0]["check"] != "event_chain_head_lock_convoy" {
		t.Fatalf("records = %#v, want one event warning record", records)
	}
	if eventBlock["warning_count"] != 1 {
		t.Fatalf("event block = %#v, want one warning", eventBlock)
	}
	if auditBlock["skipped"] != doctorLockWaitSkippedMissingAudit {
		t.Fatalf("audit block = %#v, want audit missing-column skip", auditBlock)
	}
	eventSQL := strings.Join(runner.queries, "\n")
	for _, want := range []string{
		"candidate_runs AS MATERIALIZED",
		"CROSS JOIN LATERAL",
		"ORDER BY e.event_id DESC",
		"activity_at DESC",
		"LIMIT $4",
	} {
		if !strings.Contains(eventSQL, want) {
			t.Fatalf("event SQL missing %q:\n%s", want, eventSQL)
		}
	}
}

func TestHandleDoctorLockWaitConvoyWarningsDoNotRedDoctor(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	runner := &doctorLockWaitFakeRunner{
		columnRows: []map[string]any{{"table_name": "events"}},
		eventRows: []map[string]any{{
			"run_id":           "run_hot",
			"samples":          int64(1),
			"max_lock_wait_us": int64(6_000_000),
			"p95_lock_wait_us": int64(6_000_000),
			"newest_sample_at": now,
		}},
	}
	result, err := HandleDoctor(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "verbose": true},
	})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("doctor ok = %v, want true; problems=%#v", result["ok"], result["problems"])
	}
	if problems := strings.Join(result["problems"].([]string), "\n"); strings.Contains(problems, "lock_convoy") {
		t.Fatalf("lock convoy should not be a problem:\n%s", problems)
	}
	warnings := strings.Join(result["warnings"].([]string), "\n")
	if !strings.Contains(warnings, "event_chain_head_lock_convoy.run_hot") {
		t.Fatalf("warnings missing lock convoy:\n%s", warnings)
	}
	records := result["warning_records"].([]map[string]any)
	found := false
	for _, record := range records {
		if record["check"] == "event_chain_head_lock_convoy" && record["run_id"] == "run_hot" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warning_records missing event lock convoy: %#v", records)
	}
}

func TestDoctorAuditLockWaitConvoyUsesBoundedAuditTail(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	runner := &doctorLockWaitFakeRunner{
		columnRows: []map[string]any{{"table_name": "audit_log"}},
		auditRows: []map[string]any{{
			"repository_id":    "repo_1",
			"method":           "work.claim",
			"samples":          int64(2),
			"max_lock_wait_us": int64(7_000_000),
			"p95_lock_wait_us": int64(6_500_000),
			"newest_sample_at": now,
		}},
	}
	eventBlock, auditBlock, warnings, records := doctorLockWaitConvoys(context.Background(), runner, "repo_1", now)
	if eventBlock["skipped"] != doctorLockWaitSkippedMissingEvent {
		t.Fatalf("event block = %#v, want event missing-column skip", eventBlock)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "audit_chain_head_lock_convoy.repo_1.work.claim") {
		t.Fatalf("warnings = %#v, want audit convoy warning", warnings)
	}
	if len(records) != 1 || records[0]["check"] != "audit_chain_head_lock_convoy" {
		t.Fatalf("records = %#v, want one audit warning record", records)
	}
	if auditBlock["warning_count"] != 1 {
		t.Fatalf("audit block = %#v, want one warning", auditBlock)
	}
	auditSQL := strings.Join(runner.queries, "\n")
	for _, want := range []string{
		"tail_audit AS MATERIALIZED",
		"ORDER BY audit_id DESC",
		"LIMIT $1",
	} {
		if !strings.Contains(auditSQL, want) {
			t.Fatalf("audit SQL missing %q:\n%s", want, auditSQL)
		}
	}
}

func TestDoctorLockWaitOldSchemaSkipsAgainstLivePG(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	eventBlock, auditBlock, warnings, records := doctorLockWaitConvoys(
		ctx, runner, "repo_old_schema", time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC))
	if len(warnings) != 0 || len(records) != 0 {
		t.Fatalf("old live schema produced warnings=%#v records=%#v", warnings, records)
	}
	if eventBlock["skipped"] != doctorLockWaitSkippedMissingEvent {
		t.Fatalf("event block = %#v, want missing-column skip", eventBlock)
	}
	if auditBlock["skipped"] != doctorLockWaitSkippedMissingAudit {
		t.Fatalf("audit block = %#v, want missing-column skip", auditBlock)
	}
}

func TestDoctorLockWaitConvoysQueryLivePGRows(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	if _, _, err := db.ApplyOwnerBundles(ctx, runner, "test"); err != nil {
		t.Fatalf("apply owner bundles: %v", err)
	}
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	const repoID = "repo_doctor_lock_wait"
	seedDoctorLockWaitRepo(t, ctx, runner, repoID, now)

	// This run started long ago but completed most recently. The test catches the
	// candidate ordering regression where terminal runs were ordered by started_at
	// rather than the same activity_at expression used for admission.
	seedDoctorLockWaitRun(t, ctx, runner, repoID, "run_hot", "completed", now.Add(-2*time.Hour), now.Add(-time.Minute))
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.events (
		  repository_id, run_id, event_type, payload_json, created_at,
		  previous_hash, row_hash, lock_wait_us
		) VALUES (
		  $1, 'run_hot', 'job.completed', '{}'::jsonb, $2,
		  NULL, 'event_hash_hot', $3
		)`, repoID, now.Add(-time.Minute), doctorLockWaitWarningThresholdUS+1000); err != nil {
		t.Fatalf("insert hot event: %v", err)
	}

	for i := 0; i < doctorLockWaitCandidateRunLimit; i++ {
		runID := "run_other_" + intPlaceholder(i)
		seedDoctorLockWaitRun(t, ctx, runner, repoID, runID, "completed", now.Add(-time.Hour+time.Duration(i)*time.Minute), now.Add(-14*time.Minute+time.Duration(i)*time.Second))
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.events (
			  repository_id, run_id, event_type, payload_json, created_at,
			  previous_hash, row_hash, lock_wait_us
			) VALUES (
			  $1, $2, 'job.completed', '{}'::jsonb, $3,
			  NULL, $4, $5
			)`, repoID, runID, now.Add(-14*time.Minute+time.Duration(i)*time.Second), "event_hash_other_"+intPlaceholder(i), doctorLockWaitWarningThresholdUS-1); err != nil {
			t.Fatalf("insert other event %s: %v", runID, err)
		}
	}

	segmentID := doctorLockWaitAuditSegment(t, ctx, runner)
	insertDoctorLockWaitAudit(t, ctx, runner, segmentID, repoID, "work.claim", now.Add(-2*time.Minute), doctorLockWaitWarningThresholdUS+2000, "audit_hash_hot")
	insertDoctorLockWaitAudit(t, ctx, runner, segmentID, repoID, "work.low", now.Add(-2*time.Minute), doctorLockWaitWarningThresholdUS-1, "audit_hash_low")
	insertDoctorLockWaitAudit(t, ctx, runner, segmentID, repoID, "work.old", now.Add(-30*time.Minute), doctorLockWaitWarningThresholdUS+5000, "audit_hash_old")
	insertDoctorLockWaitAudit(t, ctx, runner, segmentID, "repo_other", "work.other", now.Add(-2*time.Minute), doctorLockWaitWarningThresholdUS+5000, "audit_hash_other")

	eventBlock, auditBlock, warnings, records := doctorLockWaitConvoys(ctx, runner, repoID, now)
	warningText := strings.Join(warnings, "\n")
	if !strings.Contains(warningText, "event_chain_head_lock_convoy.run_hot") {
		t.Fatalf("warnings missing hot event run:\n%s", warningText)
	}
	if strings.Contains(warningText, "run_other_") {
		t.Fatalf("warnings included below-threshold terminal runs:\n%s", warningText)
	}
	if !strings.Contains(warningText, "audit_chain_head_lock_convoy."+repoID+".work.claim") {
		t.Fatalf("warnings missing hot audit method:\n%s", warningText)
	}
	for _, notWant := range []string{"work.low", "work.old", "repo_other"} {
		if strings.Contains(warningText, notWant) {
			t.Fatalf("warnings included filtered audit row %q:\n%s", notWant, warningText)
		}
	}
	if eventBlock["warning_count"] != 1 {
		t.Fatalf("event block = %#v, want one event warning", eventBlock)
	}
	if auditBlock["warning_count"] != 1 {
		t.Fatalf("audit block = %#v, want one audit warning", auditBlock)
	}
	checks := map[string]map[string]any{}
	for _, record := range records {
		checks[stringFrom(record, "check")] = record
	}
	eventRecord := checks["event_chain_head_lock_convoy"]
	if eventRecord == nil || eventRecord["repository_id"] != repoID || eventRecord["run_id"] != "run_hot" ||
		intFrom(eventRecord, "samples") != 1 || intFrom(eventRecord, "threshold_us") != doctorLockWaitWarningThresholdUS ||
		intFrom(eventRecord, "events_per_run_limit") != doctorLockWaitEventsPerRunLimit {
		t.Fatalf("event warning record = %#v", eventRecord)
	}
	auditRecord := checks["audit_chain_head_lock_convoy"]
	if auditRecord == nil || auditRecord["repository_id"] != repoID || auditRecord["method"] != "work.claim" ||
		intFrom(auditRecord, "samples") != 1 || intFrom(auditRecord, "threshold_us") != doctorLockWaitWarningThresholdUS ||
		intFrom(auditRecord, "audit_tail_limit") != doctorLockWaitAuditTailLimit {
		t.Fatalf("audit warning record = %#v", auditRecord)
	}
}

func seedDoctorLockWaitRepo(t *testing.T, ctx context.Context, runner db.Runner, repoID string, now time.Time) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,28,'active')`,
		repoID, "ident_"+repoID, "/tmp/"+repoID, "/tmp/"+repoID+"/.striatum", now); err != nil {
		t.Fatalf("insert repository %s: %v", repoID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha','{}'::jsonb,$3)`, repoID, "snap_"+repoID, now); err != nil {
		t.Fatalf("insert workflow snapshot %s: %v", repoID, err)
	}
}

func seedDoctorLockWaitRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, state string, startedAt, completedAt time.Time) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at,
		  started_at, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$6,$7)`,
		repoID, runID, "snap_"+repoID, "/tmp/"+repoID, state, startedAt, completedAt); err != nil {
		t.Fatalf("insert run %s: %v", runID, err)
	}
}

func doctorLockWaitAuditSegment(t *testing.T, ctx context.Context, runner db.Runner) int64 {
	t.Helper()
	var segmentID int64
	if err := runner.QueryRow(ctx,
		"SELECT segment_id FROM striatumd.audit_segments WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1").Scan(&segmentID); err != nil {
		t.Fatalf("read audit segment: %v", err)
	}
	return segmentID
}

func insertDoctorLockWaitAudit(t *testing.T, ctx context.Context, runner db.Runner, segmentID int64, repositoryID, method string, ts time.Time, lockWaitUS int, rowHash string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.audit_log (
		  ts, schema_version, hash_format_version, daemon_version,
		  client_id, repository_id, method, decision, denial_reason,
		  transport, request_id, exit_code, params_sha256, previous_hash,
		  row_hash, segment_id, lock_wait_us
		) VALUES (
		  $1, 1, 3, 'test', 'client', $2, $3, 'allowed', NULL,
		  'rpc', $4, NULL, 'params', NULL,
		  $5, $6, $7
		)`, ts, repositoryID, method, "req_"+rowHash, rowHash, segmentID, lockWaitUS); err != nil {
		t.Fatalf("insert audit row %s: %v", rowHash, err)
	}
}
