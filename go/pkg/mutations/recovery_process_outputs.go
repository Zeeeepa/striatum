package mutations

import (
	"context"
	"fmt"
	"github.com/halbritt/striatum/go/pkg/db"
	"syscall"
)

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func evaluateAndBlockLostProcess(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID string, processID string, command any) (any, error) {
	missingPaths, verdictMissing, err := validateProcessOutputs(ctx, runner, repositoryID, job)
	if err != nil {
		return nil, err
	}
	if len(missingPaths) == 0 && !verdictMissing {
		return nil, nil
	}
	existing, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.blockers
		 WHERE repository_id = $1
		   AND job_id = $2
		   AND state = 'open'
		 LIMIT 1`, repositoryID, job["job_id"])
	if err != nil {
		return nil, err
	}
	if existing {
		return nil, nil
	}
	blockerID, err := newID("blk")
	if err != nil {
		return nil, err
	}
	now := nowString()
	blockerKind := "process_lost_with_outputs_missing"
	description := fmt.Sprintf("process %s was lost (external kill or runner exit); required outputs missing: %d artifact(s), verdict missing=%v", processID, len(missingPaths), verdictMissing)
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	blockerPayload := map[string]any{
		"process_id":             processID,
		"command":                command,
		"missing_artifact_paths": missingPaths,
		"review_verdict_missing": verdictMissing,
	}
	blockerPayloadArg, err := db.JSONBArg(runner, blockerPayload)
	if err != nil {
		return nil, err
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id,
		  severity, blocker_kind, description, state, created_at, payload_json
		)
		VALUES ($1,$2,$3,$4,$5,'blocked',$6,$7,'open',$8,$9::jsonb)`,
		repositoryID,
		blockerID,
		job["run_id"],
		job["job_id"],
		sessionID,
		blockerKind,
		description,
		now,
		blockerPayloadArg,
	); err != nil {
		return nil, err
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'blocked'
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, job["job_id"]); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.blocked", sessionID, job["job_id"], nil, nil, nil, map[string]any{
		"blocker_id":   blockerID,
		"blocker_kind": blockerKind,
	}); err != nil {
		return nil, err
	}
	return blockerKind, nil
}

func validateProcessOutputs(ctx context.Context, runner any, repositoryID string, job map[string]any) ([]string, bool, error) {
	requiredPaths := []string{}
	for _, item := range asList(job["expected_artifacts_json"]) {
		expected := asMap(item)
		if expected["required"] == false {
			continue
		}
		path, _ := expected["path"].(string)
		if path != "" {
			requiredPaths = append(requiredPaths, path)
		}
	}
	published := map[string]bool{}
	if len(requiredPaths) > 0 {
		rows, err := queryRows(ctx, runner, `
			SELECT repo_path FROM striatumd.artifacts
			 WHERE repository_id = $1 AND job_id = $2`, repositoryID, job["job_id"])
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			published[fmt.Sprint(row["repo_path"])] = true
		}
	}
	missing := []string{}
	for _, path := range requiredPaths {
		if !published[path] {
			missing = append(missing, path)
		}
	}
	verdictMissing := false
	if fmt.Sprint(job["job_type"]) == "review" {
		found, err := existsRow(ctx, runner, `
			SELECT 1 FROM striatumd.verdicts
			 WHERE repository_id = $1 AND job_id = $2
			 LIMIT 1`, repositoryID, job["job_id"])
		if err != nil {
			return nil, false, err
		}
		verdictMissing = !found
	}
	return missing, verdictMissing, nil
}
