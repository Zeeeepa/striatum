// Package mutations is the Go-core port of the daemon-side PostgreSQL
// workflow mutation handlers.
package mutations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type handlerFn func(context.Context, db.Runner, rpc.Envelope) (map[string]any, error)

func Register(server *rpc.Server, runner db.Runner) {
	if runner == nil {
		return
	}
	server.Register("session.register", makeHandler(runner, HandleRegisterSession))
	server.Register("session.close", makeHandler(runner, HandleCloseSession))
	server.Register("work.claim_next", makeHandler(runner, HandleClaimNext))
	server.Register("claim_next", makeHandler(runner, HandleClaimNext))
	server.Register("work.ack", makeHandler(runner, HandleAckWork))
	server.Register("ack", makeHandler(runner, HandleAckWork))
	server.Register("work.heartbeat", makeHandler(runner, HandleHeartbeat))
	server.Register("heartbeat", makeHandler(runner, HandleHeartbeat))
	server.Register("work.release", makeHandler(runner, HandleReleaseWork))
	server.Register("release", makeHandler(runner, HandleReleaseWork))
	server.Register("work.block", makeHandler(runner, HandleBlockWork))
	server.Register("block", makeHandler(runner, HandleBlockWork))
	server.Register("work.complete", makeHandler(runner, HandleCompleteWork))
	server.Register("complete", makeHandler(runner, HandleCompleteWork))
	server.Register("artifact.publish", makeHandler(runner, HandlePublishArtifact))
	server.Register("publish_artifact", makeHandler(runner, HandlePublishArtifact))
	server.Register("review.verdict", makeHandler(runner, HandleRecordVerdict))
	server.Register("verdict", makeHandler(runner, HandleRecordVerdict))
	server.Register("review.submit", makeHandler(runner, HandleSubmitReview))
	server.Register("submit_review", makeHandler(runner, HandleSubmitReview))
	server.Register("review.override", makeHandler(runner, HandleOverrideVerdict))
	server.Register("run.prepare", makeHandler(runner, HandleRunPrepare))
	server.Register("run.start", makeHandler(runner, HandleRunStart))
	server.Register("run.pause", makeHandler(runner, HandleRunPause))
	server.Register("run.resume", makeHandler(runner, HandleRunResume))
	server.Register("run.cancel", makeHandler(runner, HandleRunCancel))
	server.Register("run.retry_job", makeHandler(runner, HandleRunRetryJob))
	server.Register("branch.confirm", makeHandler(runner, HandleBranchConfirm))
	server.Register("decision.record", makeHandler(runner, HandleDecisionRecord))
	server.Register("checkpoint.resolve", makeHandler(runner, HandleCheckpointResolve))
	server.Register("recovery.stale_leases", makeHandler(runner, HandleRecoveryStaleLeases))
	server.Register("recovery.requeue_stale", makeHandler(runner, HandleRecoveryRequeueStale))
	server.Register("recovery.cancel_job", makeHandler(runner, HandleRecoveryCancelJob))
	server.Register("recovery.process_reconcile", makeHandler(runner, HandleRecoveryProcessReconcile))
	server.Register("recovery.resume", makeHandler(runner, HandleRecoveryResume))
	server.Register("recovery.auto", makeHandler(runner, HandleRecoveryAuto))
}

func makeHandler(runner db.Runner, fn handlerFn) rpc.Handler {
	return func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		return fn(ctx, runner, envelope)
	}
}

func requireRepositoryID(envelope rpc.Envelope) (string, error) {
	value, _ := envelope.Params["repository_id"].(string)
	if value == "" {
		return "", rpc.NewError("repo_not_registered", "daemon RPC route requires repository_id", nil)
	}
	return value, nil
}

func stringParam(envelope rpc.Envelope, key string) string {
	if value, ok := envelope.Params[key].(string); ok {
		return value
	}
	return ""
}

func boolParam(envelope rpc.Envelope, key string) bool {
	if value, ok := envelope.Params[key].(bool); ok {
		return value
	}
	return false
}

func intParam(envelope rpc.Envelope, key string, fallback int) int {
	switch value := envelope.Params[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func stringSliceParam(envelope rpc.Envelope, keys ...string) []string {
	for _, key := range keys {
		value, ok := envelope.Params[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []any:
			result := []string{}
			for _, item := range typed {
				result = append(result, fmt.Sprint(item))
			}
			return result
		case string:
			if typed != "" {
				return []string{typed}
			}
		}
	}
	return []string{}
}

func withTx(ctx context.Context, runner db.Runner, fn func(db.TxRunner) (map[string]any, error)) (map[string]any, error) {
	tx, err := runner.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	result, err := fn(tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	committed = true
	return result, nil
}

func rowByID(ctx context.Context, runner any, repositoryID, table, column, value string, forUpdate bool) (map[string]any, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	return oneRow(ctx, runner, fmt.Sprintf(
		"SELECT * FROM striatumd.%s WHERE repository_id = $1 AND %s = $2%s",
		table,
		column,
		suffix,
	), repositoryID, value)
}

func oneRow(ctx context.Context, runner any, sql string, args ...any) (map[string]any, error) {
	q, ok := runner.(queryer)
	if !ok {
		return nil, fmt.Errorf("runner does not support row queries")
	}
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, pgx.ErrNoRows
	}
	return items[0], nil
}

func queryRows(ctx context.Context, runner any, sql string, args ...any) ([]map[string]any, error) {
	q, ok := runner.(queryer)
	if !ok {
		return nil, fmt.Errorf("runner does not support row queries")
	}
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToMap)
}

func nowString() string {
	return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
}

func expiresAfter(seconds int) string {
	return time.Now().UTC().Add(time.Duration(seconds) * time.Second).Truncate(time.Second).Format(time.RFC3339)
}

func newID(prefix string) (string, error) {
	var body [16]byte
	if _, err := rand.Read(body[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(body[:]), nil
}

func activeLeaseFor(ctx context.Context, runner any, repositoryID, leaseID, sessionID, jobID string) (map[string]any, error) {
	lease, err := rowByID(ctx, runner, repositoryID, "leases", "lease_id", leaseID, true)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(lease["state"]) != "active" {
		return nil, rpc.NewError("lease_error", "lease is not active", nil)
	}
	if fmt.Sprint(lease["owner_session_id"]) != sessionID {
		return nil, rpc.NewError("lease_error", "lease is owned by another session", nil)
	}
	if jobID != "" && fmt.Sprint(lease["resource_id"]) != jobID {
		return nil, rpc.NewError("lease_error", "lease does not belong to the job", nil)
	}
	if expires, ok := asTime(lease["expires_at"]); ok && expires.UTC().Before(time.Now().UTC()) {
		return nil, rpc.NewError("lease_error", "lease is expired", nil)
	}
	return lease, nil
}

func isRepoWrite(job map[string]any) bool {
	scope := asMap(job["write_scope_json"])
	return scope["repo_write"] == true || scope["mode"] == "repo_write"
}

func verifyRequiredArtifacts(ctx context.Context, runner any, repositoryID, jobID string) error {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, false)
	if err != nil {
		return err
	}
	expected := asList(job["expected_artifacts_json"])
	if len(expected) == 0 {
		return nil
	}
	for _, item := range expected {
		artifact := asMap(item)
		if artifact["required"] != true {
			continue
		}
		_, err := oneRow(ctx, runner, `
			SELECT 1 FROM striatumd.artifacts
			 WHERE repository_id = $1 AND job_id = $2 AND logical_name = $3
			   AND artifact_kind = $4 AND repo_path = $5
			 LIMIT 1`,
			repositoryID,
			jobID,
			artifact["logical_name"],
			artifact["kind"],
			artifact["path"],
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return rpc.NewError("invalid_transition", fmt.Sprintf(
				"required artifact is missing: logical_name=%q, kind=%q, path=%q",
				artifact["logical_name"],
				artifact["kind"],
				artifact["path"],
			), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func maybeEnqueueDownstream(ctx context.Context, runner any, repositoryID, completedJobID string) error {
	rows, err := queryRows(ctx, runner, `
		SELECT job_id FROM striatumd.job_dependencies
		 WHERE repository_id = $1 AND depends_on_job_id = $2`, repositoryID, completedJobID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		jobID := fmt.Sprint(row["job_id"])
		job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return err
		}
		if fmt.Sprint(job["state"]) != "blocked" {
			continue
		}
		satisfied, err := dependenciesSatisfied(ctx, runner, repositoryID, jobID)
		if err != nil {
			return err
		}
		if satisfied {
			if _, err := enqueueJob(ctx, runner, repositoryID, jobID); err != nil {
				return err
			}
		}
	}
	return nil
}

func dependenciesSatisfied(ctx context.Context, runner any, repositoryID, jobID string) (bool, error) {
	deps, err := queryRows(ctx, runner, `
		SELECT * FROM striatumd.job_dependencies
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, jobID)
	if err != nil {
		return false, err
	}
	for _, dep := range deps {
		upstream, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", fmt.Sprint(dep["depends_on_job_id"]), false)
		if err != nil {
			return false, err
		}
		if fmt.Sprint(upstream["state"]) != "completed" {
			return false, nil
		}
		required := requiredVerdicts(asMap(dep["gate_json"])["requires_verdict"])
		if len(required) == 0 {
			continue
		}
		latest, err := latestVerdict(ctx, runner, repositoryID, fmt.Sprint(upstream["job_id"]))
		if err != nil {
			return false, err
		}
		if !required[latest] {
			return false, nil
		}
	}
	return true, nil
}

func requiredVerdicts(value any) map[string]bool {
	result := map[string]bool{}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			result[fmt.Sprint(item)] = true
		}
	case []string:
		for _, item := range typed {
			result[item] = true
		}
	}
	return result
}

func asList(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	case []byte:
		var result []any
		_ = json.Unmarshal(typed, &result)
		return result
	case string:
		var result []any
		_ = json.Unmarshal([]byte(typed), &result)
		return result
	}
	return nil
}

func latestVerdict(ctx context.Context, runner any, repositoryID, jobID string) (string, error) {
	row, err := oneRow(ctx, runner, `
		SELECT verdict FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2
		 ORDER BY created_at DESC, verdict_id DESC
		 LIMIT 1`, repositoryID, jobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprint(row["verdict"]), nil
}

func enqueueJob(ctx context.Context, runner any, repositoryID, jobID string) (string, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return "", err
	}
	state := fmt.Sprint(job["state"])
	if state != "blocked" && state != "queued" {
		return "", rpc.NewError("invalid_transition", "job is not enqueueable", nil)
	}
	messageID, err := newID("msg")
	if err != nil {
		return "", err
	}
	now := nowString()
	selector := asMap(job["lane_selector_json"])
	targetLane, _ := selector["lane_id"].(string)
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return "", fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, payload_json, claim_count,
		  max_claims, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,0,$8,$9,$10)`,
		repositoryID,
		messageID,
		job["run_id"],
		jobID,
		job["role_id"],
		nullable(targetLane),
		map[string]any{},
		job["max_attempts"],
		now,
		now,
	); err != nil {
		return "", err
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'queued', current_message_id = $1, ready_at = $2
		 WHERE repository_id = $3 AND job_id = $4`, messageID, now, repositoryID, jobID); err != nil {
		return "", err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "queue.message_enqueued", nil, jobID, messageID, nil, nil, map[string]any{
		"workflow_job_id": job["workflow_job_id"],
	}); err != nil {
		return "", err
	}
	return messageID, nil
}

func maybeCompleteRun(ctx context.Context, runner any, repositoryID, runID string) error {
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, true)
	if err != nil {
		return err
	}
	failed, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2 AND state = 'failed'
		 LIMIT 1`, repositoryID, runID)
	if err != nil {
		return err
	}
	remaining, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2
		   AND state NOT IN ('completed','skipped','canceled')
		 LIMIT 1`, repositoryID, runID)
	if err != nil {
		return err
	}
	now := nowString()
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	if failed && fmt.Sprint(run["state"]) == "running" {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.runs
			   SET state = 'failed', completed_at = $1, stop_reason = 'job_failed'
			 WHERE repository_id = $2 AND run_id = $3`, now, repositoryID, runID); err != nil {
			return err
		}
		if _, err := appendEvent(ctx, runner, repositoryID, runID, "run.failed", nil, nil, nil, nil, nil, map[string]any{"reason": "job_failed"}); err != nil {
			return err
		}
		return closeRemainingSessions(ctx, runner, repositoryID, runID, "run_failed", "run_failed")
	}
	if remaining || fmt.Sprint(run["state"]) != "running" {
		return nil
	}
	hasCompleted, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2 AND state = 'completed'
		 LIMIT 1`, repositoryID, runID)
	if err != nil {
		return err
	}
	state := "canceled"
	eventType := "run.canceled"
	source := "run_canceled"
	reason := "run_canceled"
	var payload map[string]any = map[string]any{"reason": "all_jobs_canceled"}
	if hasCompleted {
		state = "completed"
		eventType = "run.completed"
		source = "run_completed"
		reason = "run_completed"
		payload = nil
	}
	if state == "completed" {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.runs
			   SET state = $1, completed_at = $2
			 WHERE repository_id = $3 AND run_id = $4`, state, now, repositoryID, runID); err != nil {
			return err
		}
	} else if err := exec.Exec(ctx, `
		UPDATE striatumd.runs
		   SET state = $1, completed_at = $2, stop_reason = 'all_jobs_canceled'
		 WHERE repository_id = $3 AND run_id = $4`, state, now, repositoryID, runID); err != nil {
		return err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, runID, eventType, nil, nil, nil, nil, nil, payload); err != nil {
		return err
	}
	return closeRemainingSessions(ctx, runner, repositoryID, runID, source, reason)
}

func existsRow(ctx context.Context, runner any, sql string, args ...any) (bool, error) {
	_, err := oneRow(ctx, runner, sql, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func closeRemainingSessions(ctx context.Context, runner any, repositoryID, runID, source, reason string) error {
	rows, err := queryRows(ctx, runner, `
		SELECT * FROM striatumd.sessions
		 WHERE repository_id = $1 AND run_id = $2 AND state = 'active'
		 ORDER BY registered_at
		 FOR UPDATE`, repositoryID, runID)
	if err != nil {
		return err
	}
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	now := nowString()
	for _, row := range rows {
		sessionID := fmt.Sprint(row["session_id"])
		activeLease, err := existsRow(ctx, runner, `
			SELECT 1 FROM striatumd.leases
			 WHERE repository_id = $1 AND owner_session_id = $2 AND state = 'active'
			 LIMIT 1`, repositoryID, sessionID)
		if err != nil {
			return err
		}
		if activeLease {
			continue
		}
		if err := exec.Exec(ctx, `
			UPDATE striatumd.sessions
			   SET state = 'closed', closed_at = $1, close_reason = $2
			 WHERE repository_id = $3 AND session_id = $4`, now, reason, repositoryID, sessionID); err != nil {
			return err
		}
		if _, err := appendEvent(ctx, runner, repositoryID, runID, "session.closed", sessionID, nil, nil, nil, nil, map[string]any{
			"session_id": sessionID,
			"role_id":    row["role_id"],
			"lane_id":    row["lane_id"],
			"reason":     reason,
			"source":     source,
		}); err != nil {
			return err
		}
	}
	return nil
}

func sessionLaneAttestation(ctx context.Context, runner any, repositoryID, sessionID string) map[string]any {
	row, err := oneRow(ctx, runner, `
		SELECT supervisor_id, pid
		  FROM striatumd.process_supervisors
		 WHERE repository_id = $1 AND session_id = $2 AND state = 'attached'
		 ORDER BY started_at DESC
		 LIMIT 1`, repositoryID, sessionID)
	if err != nil {
		return map[string]any{
			"attested":      false,
			"state":         "unattested",
			"supervisor_id": nil,
			"pid":           nil,
			"reason":        "no_attached_supervisor",
		}
	}
	return map[string]any{
		"attested":      true,
		"state":         "attested",
		"supervisor_id": row["supervisor_id"],
		"pid":           row["pid"],
		"reason":        nil,
	}
}

func appendEvent(
	ctx context.Context,
	runner any,
	repositoryID string,
	runID any,
	eventType string,
	actorSessionID any,
	jobID any,
	messageID any,
	artifactID any,
	leaseID any,
	payload map[string]any,
) (int64, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	previous, err := previousEvent(ctx, runner, repositoryID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	var previousHash any
	if previous != nil {
		hash, err := eventRowHash(previous)
		if err != nil {
			return 0, err
		}
		previousHash = hash
	}
	eventID, err := nextEventID(ctx, runner)
	if err != nil {
		return 0, err
	}
	createdAt := nowString()
	rowMaterial := map[string]any{
		"repository_id":    repositoryID,
		"event_id":         eventID,
		"run_id":           nullable(runID),
		"event_type":       eventType,
		"actor_session_id": nullable(actorSessionID),
		"job_id":           nullable(jobID),
		"message_id":       nullable(messageID),
		"artifact_id":      nullable(artifactID),
		"lease_id":         nullable(leaseID),
		"payload_json":     payload,
		"created_at":       createdAt,
	}
	rowHash, err := canonicalEventHash(rowMaterial, previousHash)
	if err != nil {
		return 0, err
	}
	storedPayload := map[string]any{}
	for key, value := range payload {
		storedPayload[key] = value
	}
	storedPayload["_event_chain"] = map[string]any{
		"algorithm":     "sha256-json-v1",
		"previous_hash": nullable(previousHash),
		"row_hash":      rowHash,
	}
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return 0, fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.events (
		  repository_id, event_id, run_id, event_type, actor_session_id, job_id,
		  message_id, artifact_id, lease_id, payload_json, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		repositoryID,
		eventID,
		nullable(runID),
		eventType,
		nullable(actorSessionID),
		nullable(jobID),
		nullable(messageID),
		nullable(artifactID),
		nullable(leaseID),
		storedPayload,
		createdAt,
	); err != nil {
		return 0, err
	}
	return eventID, nil
}

func previousEvent(ctx context.Context, runner any, repositoryID string) (map[string]any, error) {
	return oneRow(ctx, runner, `
		SELECT repository_id, event_id, run_id, event_type, actor_session_id,
		       job_id, message_id, artifact_id, lease_id, payload_json, created_at
		  FROM striatumd.events
		 WHERE repository_id = $1
		 ORDER BY event_id DESC
		 LIMIT 1
		 FOR UPDATE`, repositoryID)
}

func nextEventID(ctx context.Context, runner any) (int64, error) {
	rower, ok := runner.(interface {
		QueryRow(context.Context, string, ...any) db.Row
	})
	if !ok {
		return 0, fmt.Errorf("runner does not support query row")
	}
	var eventID int64
	err := rower.QueryRow(ctx, "SELECT nextval(pg_get_serial_sequence('striatumd.events', 'event_id'))").Scan(&eventID)
	return eventID, err
}

func canonicalEventHash(row map[string]any, previousHash any) (string, error) {
	payload := map[string]any{
		"previous_hash":    nullable(previousHash),
		"repository_id":    row["repository_id"],
		"event_id":         row["event_id"],
		"run_id":           nullable(row["run_id"]),
		"event_type":       row["event_type"],
		"actor_session_id": nullable(row["actor_session_id"]),
		"job_id":           nullable(row["job_id"]),
		"message_id":       nullable(row["message_id"]),
		"artifact_id":      nullable(row["artifact_id"]),
		"lease_id":         nullable(row["lease_id"]),
		"payload_json":     eventPayload(row["payload_json"]),
		"created_at":       timestampString(row["created_at"]),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func eventRowHash(row map[string]any) (string, error) {
	payload := asMap(row["payload_json"])
	if chain := asMap(payload["_event_chain"]); chain != nil {
		if hash, ok := chain["row_hash"].(string); ok && hash != "" {
			return hash, nil
		}
		previous := chain["previous_hash"]
		return canonicalEventHash(row, previous)
	}
	return canonicalEventHash(row, nil)
}

func eventPayload(value any) map[string]any {
	payload := asMap(value)
	result := map[string]any{}
	for key, item := range payload {
		if key == "_event_chain" {
			continue
		}
		result[key] = item
	}
	return result
}

func asMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case []byte:
		var result map[string]any
		_ = json.Unmarshal(typed, &result)
		if result != nil {
			return result
		}
	case string:
		var result map[string]any
		_ = json.Unmarshal([]byte(typed), &result)
		if result != nil {
			return result
		}
	}
	return map[string]any{}
}

func nullable(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
	case *string:
		if typed == nil || *typed == "" {
			return nil
		}
		return *typed
	}
	return value
}

func asTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, true
	case string:
		parsed, err := time.Parse(time.RFC3339, typed)
		return parsed, err == nil
	case []byte:
		parsed, err := time.Parse(time.RFC3339, string(typed))
		return parsed, err == nil
	}
	return time.Time{}, false
}

func timestampString(value any) string {
	if ts, ok := asTime(value); ok {
		return ts.UTC().Truncate(time.Second).Format(time.RFC3339)
	}
	return fmt.Sprint(value)
}

var operatorLabelPattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)

func validateOperatorLabel(label string, workflow map[string]any) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(label))
	if !operatorLabelPattern.MatchString(cleaned) {
		return "", rpc.NewError("invalid_transition", "operator label must match ^[a-z0-9._-]{1,64}$", nil)
	}
	reserved := map[string]struct{}{
		"operator": {}, "attested": {}, "unattested": {}, "unknown": {},
	}
	if lanes := asMap(workflow["lanes"]); lanes != nil {
		for lane := range lanes {
			reserved[strings.ToLower(lane)] = struct{}{}
		}
	}
	if _, found := reserved[cleaned]; found {
		return "", rpc.NewError("invalid_transition", fmt.Sprintf("operator label %q is reserved", cleaned), nil)
	}
	return cleaned, nil
}

func workflowDeclaresFreshReviewer(workflow map[string]any) bool {
	jobs := asList(workflow["jobs"])
	if len(jobs) == 0 {
		return false
	}
	for _, item := range jobs {
		job := asMap(item)
		if job["type"] != "review" {
			continue
		}
		if job["reviewer_context_policy"] == "fresh" || job["fresh_session_required"] == true {
			return true
		}
	}
	return false
}
