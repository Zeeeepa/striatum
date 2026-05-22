package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

const (
	defaultAutoFinalizeMtimeGraceSeconds = 30
	autoFinalizeSummary                  = "auto-finalized from stable expected artifact files"
	noProcessAutoFinalizeRationale       = "auto-finalized from stable expected artifact front matter; no terminal output or transcript was used as evidence"
)

func HandleRecoveryAutoFinalize(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "recovery.auto_finalize requires run_id", nil)
	}
	dryRun, err := autoFinalizeDryRun(envelope)
	if err != nil {
		return nil, err
	}
	mtimeGraceSeconds, err := autoFinalizeMtimeGraceSeconds(envelope)
	if err != nil {
		return nil, err
	}
	policy, err := readAutoFinalizePolicy(ctx, runner, repositoryID, runID, envelope, mtimeGraceSeconds)
	if err != nil {
		return nil, err
	}
	if !dryRun && policy["live_allowed"] != true {
		return nil, rpc.NewError("invalid_transition", "recovery.auto_finalize live mode requires workflow recovery.auto_finalize.enabled=true or --force", nil)
	}

	if dryRun {
		return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
			run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
			if err != nil {
				return nil, err
			}
			if fmt.Sprint(run["state"]) != "running" {
				return autoFinalizeEmptyResult(runID, true, fmt.Sprintf("run is not running: %s", run["state"]), policy), nil
			}
			rows, err := autoFinalizeCandidateRows(ctx, tx, repositoryID, runID, stringParam(envelope, "job_id"))
			if err != nil {
				return nil, err
			}
			return evaluateAutoFinalizeRows(ctx, tx, repositoryID, fmt.Sprint(run["repo_root"]), runID, rows, mtimeGraceSeconds, boolParam(envelope, "allow_no_process_execution"), policy)
		})
	}

	rows, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(run["state"]) != "running" {
			return autoFinalizeEmptyResult(runID, false, fmt.Sprintf("run is not running: %s", run["state"]), policy), nil
		}
		candidates, err := autoFinalizeCandidateRows(ctx, tx, repositoryID, runID, stringParam(envelope, "job_id"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"repo_root": run["repo_root"], "rows": candidates}, nil
	})
	if err != nil {
		return nil, err
	}
	if _, ok := rows["eligible_count"]; ok {
		return rows, nil
	}

	repoRoot := fmt.Sprint(rows["repo_root"])
	finalized := []map[string]any{}
	skipped := []map[string]any{}
	for _, item := range asList(rows["rows"]) {
		row := asMap(item)
		jobID := fmt.Sprint(row["job_id"])
		result, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
			fresh, err := autoFinalizeCandidateRows(ctx, tx, repositoryID, runID, jobID)
			if err != nil {
				return nil, err
			}
			if len(fresh) == 0 {
				return map[string]any{
					"eligible": false,
					"skip": map[string]any{
						"workflow_job_id": row["workflow_job_id"],
						"job_id":          jobID,
						"reason":          "candidate is no longer auto-finalizable",
					},
				}, nil
			}
			evaluation := evaluateAutoFinalizeCandidate(ctx, tx, repositoryID, repoRoot, fresh[0], mtimeGraceSeconds, boolParam(envelope, "allow_no_process_execution"))
			if evaluation["eligible"] != true {
				return evaluation, nil
			}
			finalizedCandidate, err := finalizeAutoFinalizeCandidate(ctx, tx, repositoryID, evaluation["candidate"].(map[string]any), boolParam(envelope, "allow_no_process_execution"))
			if err != nil {
				return nil, err
			}
			return map[string]any{"eligible": true, "finalized": finalizedCandidate}, nil
		})
		if err != nil {
			skipped = append(skipped, map[string]any{
				"workflow_job_id": row["workflow_job_id"],
				"job_id":          jobID,
				"reason":          "live auto-finalize failed: " + err.Error(),
			})
			continue
		}
		if result["eligible"] == true {
			finalized = append(finalized, result["finalized"].(map[string]any))
		} else {
			skipped = append(skipped, asMap(result["skip"]))
		}
	}
	return map[string]any{
		"run_id":                    runID,
		"dry_run":                   false,
		"policy":                    policy,
		"lane_finalization_summary": autoFinalizeLaneFinalizationSummary(len(finalized), 0, 0),
		"eligible_count":            0,
		"eligible":                  []map[string]any{},
		"finalized_count":           len(finalized),
		"finalized":                 finalized,
		"skipped_count":             len(skipped),
		"skipped":                   skipped,
	}, nil
}

func autoFinalizeDryRun(envelope rpc.Envelope) (bool, error) {
	value, exists := envelope.Params["dry_run"]
	if !exists || value == nil {
		return true, nil
	}
	typed, ok := value.(bool)
	if !ok {
		return false, rpc.NewError("schema_invalid", "dry_run must be a boolean when present", nil)
	}
	return typed, nil
}

func autoFinalizeMtimeGraceSeconds(envelope rpc.Envelope) (int, error) {
	value := intParam(envelope, "mtime_grace_seconds", defaultAutoFinalizeMtimeGraceSeconds)
	if value < 0 {
		return 0, rpc.NewError("invalid_transition", "mtime_grace_seconds must be non-negative", nil)
	}
	return value, nil
}

func readAutoFinalizePolicy(ctx context.Context, runner db.Runner, repositoryID, runID string, envelope rpc.Envelope, mtimeGraceSeconds int) (map[string]any, error) {
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, false)
		if err != nil {
			return nil, err
		}
		snapshot, err := rowByID(ctx, tx, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
		if err != nil {
			return nil, err
		}
		workflow := asMap(snapshot["workflow_json"])
		enabled := false
		policy := any(nil)
		if recovery := asMap(workflow["recovery"]); len(recovery) > 0 {
			policy = recovery["auto_finalize"]
		}
		if policy == nil {
			policy = workflow["auto_finalize"]
		}
		switch typed := policy.(type) {
		case bool:
			enabled = typed
		case map[string]any:
			enabled = typed["enabled"] == true
		}
		force := boolParam(envelope, "force")
		return map[string]any{
			"workflow_enabled":           enabled,
			"force":                      force,
			"live_allowed":               enabled || force,
			"mtime_grace_seconds":        mtimeGraceSeconds,
			"allow_no_process_execution": boolParam(envelope, "allow_no_process_execution"),
		}, nil
	})
}

func autoFinalizeEmptyResult(runID string, dryRun bool, reason string, policy map[string]any) map[string]any {
	return map[string]any{
		"run_id":                    runID,
		"dry_run":                   dryRun,
		"policy":                    policy,
		"lane_finalization_summary": autoFinalizeLaneFinalizationSummary(0, 0, 0),
		"eligible_count":            0,
		"eligible":                  []map[string]any{},
		"finalized_count":           0,
		"finalized":                 []map[string]any{},
		"skipped_count":             1,
		"skipped":                   []map[string]any{{"reason": reason}},
	}
}

func autoFinalizeCandidateRows(ctx context.Context, runner any, repositoryID, runID, jobID string) ([]map[string]any, error) {
	return queryRows(ctx, runner, `
		SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at,
		       l.state AS lease_state, s.state AS session_state,
		       qm.message_id, qm.state AS message_state
		  FROM striatumd.jobs j
		  JOIN striatumd.leases l
		    ON l.repository_id = j.repository_id
		   AND l.lease_id = j.current_lease_id
		  JOIN striatumd.sessions s
		    ON s.repository_id = j.repository_id
		   AND s.session_id = l.owner_session_id
		  LEFT JOIN striatumd.queue_messages qm
		    ON qm.repository_id = j.repository_id
		   AND qm.message_id = j.current_message_id
		 WHERE j.repository_id = $1
		   AND j.run_id = $2
		   AND j.state IN ('claimed', 'running')
		   AND l.state = 'active'
		   AND l.expires_at >= $3::timestamptz
		   AND s.state = 'active'
		   AND (qm.message_id IS NULL OR qm.state IN ('claimed', 'acked'))
		   AND ($4 = '' OR j.job_id = $4)
		 ORDER BY j.workflow_job_id`, repositoryID, runID, nowString(), jobID)
}

func evaluateAutoFinalizeRows(ctx context.Context, runner any, repositoryID, repoRoot, runID string, rows []map[string]any, mtimeGraceSeconds int, allowNoProcessExecution bool, policy map[string]any) (map[string]any, error) {
	eligible := []map[string]any{}
	skipped := []map[string]any{}
	for _, row := range rows {
		evaluation := evaluateAutoFinalizeCandidate(ctx, runner, repositoryID, repoRoot, row, mtimeGraceSeconds, allowNoProcessExecution)
		if evaluation["eligible"] == true {
			eligible = append(eligible, evaluation["candidate"].(map[string]any))
		} else {
			skipped = append(skipped, asMap(evaluation["skip"]))
		}
	}
	return map[string]any{
		"run_id":                    runID,
		"dry_run":                   true,
		"policy":                    policy,
		"lane_finalization_summary": autoFinalizeLaneFinalizationSummary(len(eligible), 0, 0),
		"eligible_count":            len(eligible),
		"eligible":                  eligible,
		"finalized_count":           0,
		"finalized":                 []map[string]any{},
		"skipped_count":             len(skipped),
		"skipped":                   skipped,
	}, nil
}

func autoFinalizeLaneFinalizationSummary(autoFromArtifact, manualPublish, pending int) map[string]any {
	return map[string]any{
		"auto_from_artifact": autoFromArtifact,
		"manual_publish":     manualPublish,
		"pending":            pending,
	}
}

func evaluateAutoFinalizeCandidate(ctx context.Context, runner any, repositoryID, repoRoot string, row map[string]any, mtimeGraceSeconds int, allowNoProcessExecution bool) map[string]any {
	jobID := fmt.Sprint(row["job_id"])
	sessionID := fmt.Sprint(row["owner_session_id"])
	leaseID := fmt.Sprint(row["lease_id"])
	messageID := ""
	if nullable(row["message_id"]) != nil {
		messageID = fmt.Sprint(row["message_id"])
	}
	base := map[string]any{
		"workflow_job_id": row["workflow_job_id"],
		"job_id":          jobID,
		"session_id":      sessionID,
		"lease_id":        leaseID,
		"message_id":      nullable(messageID),
	}
	expected := []map[string]any{}
	for _, item := range asList(row["expected_artifacts_json"]) {
		declared := asMap(item)
		if declared["required"] == false {
			continue
		}
		expected = append(expected, declared)
	}
	if len(expected) == 0 {
		return autoFinalizeNotEligible(base, "no required expected_artifacts declared")
	}
	if messageID == "" {
		return autoFinalizeNotEligible(base, "no active queue message is attached")
	}
	expectedByline, err := expectedAuthorLine(ctx, runner, repositoryID, row, sessionID)
	if err != nil {
		return autoFinalizeNotEligible(base, "could not derive expected byline: "+err.Error())
	}

	artifacts := []map[string]any{}
	refusals := []map[string]any{}
	for _, declared := range expected {
		result := evaluateAutoFinalizeArtifact(ctx, runner, repositoryID, repoRoot, row, sessionID, expectedByline, declared, mtimeGraceSeconds, allowNoProcessExecution)
		if result["ok"] == true {
			artifacts = append(artifacts, result["artifact"].(map[string]any))
		} else {
			refusals = append(refusals, asMap(result["refusal"]))
		}
	}
	if len(refusals) > 0 {
		skip := cloneMap(base)
		skip["reason"] = "expected_artifact validation refused"
		skip["artifacts"] = refusals
		return map[string]any{"eligible": false, "skip": skip}
	}
	candidate := cloneMap(base)
	candidate["lane_finalization"] = "auto_from_artifact"
	candidate["message_state"] = row["message_state"]
	candidate["job_state"] = row["state"]
	candidate["job_type"] = row["job_type"]
	candidate["expected_byline"] = expectedByline
	candidate["artifacts"] = artifacts
	return map[string]any{"eligible": true, "candidate": candidate}
}

func evaluateAutoFinalizeArtifact(ctx context.Context, runner any, repositoryID, repoRoot string, job map[string]any, sessionID, expectedByline string, declared map[string]any, mtimeGraceSeconds int, allowNoProcessExecution bool) map[string]any {
	pathText, _ := declared["path"].(string)
	kind, _ := declared["kind"].(string)
	logicalName, _ := declared["logical_name"].(string)
	if pathText == "" {
		return autoFinalizeArtifactRefusal(declared, "expected_artifact path is missing", nil)
	}
	if kind == "" || !allowedArtifactKinds[kind] {
		return autoFinalizeArtifactRefusal(declared, "expected_artifact kind is invalid", nil)
	}
	if kind == "transcript" {
		return autoFinalizeArtifactRefusal(declared, "transcript artifacts are not auto-finalized", nil)
	}
	if logicalName == "" {
		return autoFinalizeArtifactRefusal(declared, "expected_artifact logical_name is missing", nil)
	}
	if !pathAllowed(repoRoot, pathText, asMap(job["write_scope_json"])) {
		return autoFinalizeArtifactRefusal(declared, "artifact path is outside the job write scope", nil)
	}
	path, err := repoRelativePath(repoRoot, pathText, false)
	if err != nil {
		return autoFinalizeArtifactRefusal(declared, err.Error(), nil)
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return autoFinalizeArtifactRefusal(declared, "expected artifact file does not exist", nil)
	}
	stable := autoFinalizeMtimeStable(info, mtimeGraceSeconds)
	if stable["stable"] != true {
		return autoFinalizeArtifactRefusal(declared, "expected artifact file mtime is inside the grace period", map[string]any{"age_seconds": stable["age_seconds"]})
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return autoFinalizeArtifactRefusal(declared, "could not read expected artifact: "+err.Error(), nil)
	}
	frontMatter, err := autoFinalizeRequiredFrontMatter(kind, path, payload)
	if err != nil {
		return autoFinalizeArtifactRefusal(declared, err.Error(), nil)
	}
	if err := validateArtifactFrontMatter(kind, path, payload); err != nil {
		return autoFinalizeArtifactRefusal(declared, err.Error(), nil)
	}
	if err := validateAutoFinalizeRequiredByline(payload, expectedByline); err != nil {
		return autoFinalizeArtifactRefusal(declared, err.Error(), nil)
	}
	if reason, err := autoFinalizeLaneEvidenceRefusal(ctx, runner, repositoryID, sessionID, expectedByline, allowNoProcessExecution); err != nil {
		return autoFinalizeArtifactRefusal(declared, err.Error(), nil)
	} else if reason != "" {
		return autoFinalizeArtifactRefusal(declared, reason, nil)
	}
	existing, err := autoFinalizeExistingArtifactStatus(ctx, runner, repositoryID, fmt.Sprint(job["run_id"]), fmt.Sprint(job["job_id"]), logicalName, pathText, payload)
	if err != nil {
		return autoFinalizeArtifactRefusal(declared, err.Error(), nil)
	}
	if existing["status"] == "conflict" {
		return autoFinalizeArtifactRefusal(declared, fmt.Sprint(existing["reason"]), nil)
	}
	return map[string]any{
		"ok": true,
		"artifact": map[string]any{
			"path":              pathText,
			"disk_path":         path,
			"kind":              kind,
			"logical_name":      logicalName,
			"sha256":            sha256Hex(payload),
			"size_bytes":        len(payload),
			"mtime_age_seconds": stable["age_seconds"],
			"publish_status":    existing["status"],
			"artifact_id":       existing["artifact_id"],
			"front_matter":      frontMatter,
		},
	}
}

func autoFinalizeNotEligible(base map[string]any, reason string) map[string]any {
	skip := cloneMap(base)
	skip["reason"] = reason
	return map[string]any{"eligible": false, "skip": skip}
}

func autoFinalizeArtifactRefusal(declared map[string]any, reason string, extra map[string]any) map[string]any {
	refusal := map[string]any{
		"path":         declared["path"],
		"kind":         declared["kind"],
		"logical_name": declared["logical_name"],
		"reason":       reason,
	}
	for key, value := range extra {
		refusal[key] = value
	}
	return map[string]any{"ok": false, "refusal": refusal}
}

func autoFinalizeMtimeStable(info os.FileInfo, graceSeconds int) map[string]any {
	age := time.Since(info.ModTime()).Seconds()
	if age < 0 {
		age = 0
	}
	return map[string]any{
		"stable":      age >= float64(graceSeconds),
		"age_seconds": float64(int(age*1000)) / 1000,
	}
}

func validateAutoFinalizeRequiredByline(payload []byte, expectedByline string) error {
	if !utf8.Valid(payload) {
		return fmt.Errorf("markdown artifact must be UTF-8")
	}
	lines := markdownTitleBlockAuthorLines(string(payload))
	if len(lines) == 0 {
		return fmt.Errorf("markdown artifact author line is required for auto-finalize")
	}
	for _, line := range lines {
		if canonicalBylineForm(line) == expectedByline {
			return nil
		}
	}
	return fmt.Errorf("markdown artifact author line must match expected work packet author line")
}

func autoFinalizeRequiredFrontMatter(kind, path string, payload []byte) (map[string]any, error) {
	if _, ok := frontMatterSchemas[kind]; !ok {
		return nil, nil
	}
	if !isMarkdownPath(path) {
		return nil, fmt.Errorf("%s artifact must be Markdown to auto-finalize front matter", kind)
	}
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("%s artifact must be UTF-8 to validate front matter", kind)
	}
	block, ok := frontMatterBlock(string(payload))
	if !ok {
		return nil, fmt.Errorf("%s artifact front matter is required for auto-finalize", kind)
	}
	parsed, err := parseFrontMatterBlock(block)
	if err != nil {
		return nil, err
	}
	if kind == "finding" {
		if _, ok := parsed["verdict_intent"].(string); !ok {
			return nil, fmt.Errorf("finding artifact front matter missing required field 'verdict_intent'")
		}
	}
	return parsed, nil
}

func autoFinalizeLaneEvidenceRefusal(ctx context.Context, runner any, repositoryID, sessionID, expectedByline string, allowNoProcessExecution bool) (string, error) {
	if strings.HasPrefix(expectedByline, "author: operator") || allowNoProcessExecution {
		return "", nil
	}
	found, err := existsRow(ctx, runner, `
		SELECT 1
		  FROM striatumd.process_executions
		 WHERE repository_id = $1
		   AND session_id = $2
		   AND state = 'exited'
		   AND exit_code = 0
		 LIMIT 1`, repositoryID, sessionID)
	if err != nil {
		return "", err
	}
	if !found {
		return "lane_evidence_missing: no clean process_executions row for the session", nil
	}
	return "", nil
}

func autoFinalizeExistingArtifactStatus(ctx context.Context, runner any, repositoryID, runID, jobID, logicalName, pathText string, payload []byte) (map[string]any, error) {
	digest := sha256Hex(payload)
	row, err := oneRow(ctx, runner, `
		SELECT artifact_id, repo_path, content_sha256
		  FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3
		   AND logical_name = $4
		 LIMIT 1`, repositoryID, runID, jobID, logicalName)
	if err == nil {
		if fmt.Sprint(row["repo_path"]) == pathText && fmt.Sprint(row["content_sha256"]) == digest {
			return map[string]any{"status": "already_published", "artifact_id": row["artifact_id"]}, nil
		}
		return map[string]any{"status": "conflict", "reason": "artifact logical name already exists with different content"}, nil
	}
	if err == pgx.ErrNoRows {
		return map[string]any{"status": "would_publish", "artifact_id": nil}, nil
	}
	return nil, err
}

func finalizeAutoFinalizeCandidate(ctx context.Context, runner any, repositoryID string, candidate map[string]any, allowNoProcessExecution bool) (map[string]any, error) {
	sessionID := fmt.Sprint(candidate["session_id"])
	jobID := fmt.Sprint(candidate["job_id"])
	leaseID := fmt.Sprint(candidate["lease_id"])
	messageID := fmt.Sprint(candidate["message_id"])
	if candidate["message_state"] == "claimed" {
		if err := ackInline(ctx, runner, repositoryID, sessionID, messageID, leaseID); err != nil {
			return nil, err
		}
	}

	published := []map[string]any{}
	for _, item := range asList(candidate["artifacts"]) {
		artifact := asMap(item)
		result, err := publishArtifact(ctx, runner, repositoryID, sessionID, jobID, leaseID, fmt.Sprint(artifact["kind"]), fmt.Sprint(artifact["logical_name"]), fmt.Sprint(artifact["path"]))
		if err != nil {
			return nil, err
		}
		artifactID := fmt.Sprint(result["artifact_id"])
		runID, err := autoFinalizeRunID(ctx, runner, repositoryID, jobID)
		if err != nil {
			return nil, err
		}
		payload := map[string]any{
			"path":         artifact["path"],
			"kind":         artifact["kind"],
			"logical_name": artifact["logical_name"],
			"sha256":       valueOr(result["sha256"], artifact["sha256"]),
			"validation": map[string]any{
				"byline":            candidate["expected_byline"],
				"mtime_age_seconds": artifact["mtime_age_seconds"],
				"source":            "expected_artifact_file",
			},
			"rationale": autoFinalizeSummary,
		}
		if allowNoProcessExecution {
			payload["override_rationale"] = noProcessAutoFinalizeRationale
		}
		if _, err := appendEvent(ctx, runner, repositoryID, runID, "artifact.auto_finalized", sessionID, jobID, nil, artifactID, leaseID, payload); err != nil {
			return nil, err
		}
		published = append(published, map[string]any{
			"path":         artifact["path"],
			"kind":         artifact["kind"],
			"logical_name": artifact["logical_name"],
			"artifact_id":  artifactID,
			"status":       result["status"],
		})
	}

	runID, err := autoFinalizeRunID(ctx, runner, repositoryID, jobID)
	if err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, runID, "job.auto_finalized", sessionID, jobID, nullable(messageID), nil, leaseID, map[string]any{
		"artifact_count":    len(published),
		"rationale":         autoFinalizeSummary,
		"lane_finalization": "auto_from_artifact",
		"validation": map[string]any{
			"byline": candidate["expected_byline"],
			"source": "expected_artifact_file",
		},
	}); err != nil {
		return nil, err
	}

	var completeResult map[string]any
	switch fmt.Sprint(candidate["job_type"]) {
	case "review", "phase_synthesis":
		verdictArtifact, err := autoFinalizeVerdictArtifact(asList(candidate["artifacts"]))
		if err != nil {
			return nil, err
		}
		verdict := fmt.Sprint(asMap(verdictArtifact["front_matter"])["verdict_intent"])
		artifactID, err := autoFinalizePublishedArtifactID(published, fmt.Sprint(verdictArtifact["logical_name"]))
		if err != nil {
			return nil, err
		}
		completeResult, err = recordVerdict(ctx, runner, repositoryID, sessionID, jobID, leaseID, verdict, artifactID, autoFinalizeSummary)
		if err != nil {
			return nil, err
		}
	default:
		var err error
		completeResult, err = completeAutoFinalizedJob(ctx, runner, repositoryID, jobID, sessionID, leaseID, messageID)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"workflow_job_id":   candidate["workflow_job_id"],
		"job_id":            jobID,
		"session_id":        sessionID,
		"lane_finalization": "auto_from_artifact",
		"summary":           autoFinalizeSummary,
		"artifacts":         published,
		"complete":          completeResult,
	}, nil
}

func autoFinalizeRunID(ctx context.Context, runner any, repositoryID, jobID string) (any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, false)
	if err != nil {
		return nil, err
	}
	return job["run_id"], nil
}

func autoFinalizeVerdictArtifact(artifacts []any) (map[string]any, error) {
	for _, item := range artifacts {
		artifact := asMap(item)
		if artifact["kind"] != "finding" {
			continue
		}
		frontMatter := asMap(artifact["front_matter"])
		if _, ok := frontMatter["verdict_intent"].(string); ok {
			return artifact, nil
		}
	}
	return nil, rpc.NewError("invalid_transition", "review auto-finalize requires a finding expected_artifact with verdict_intent front matter", nil)
}

func autoFinalizePublishedArtifactID(published []map[string]any, logicalName string) (any, error) {
	for _, artifact := range published {
		if fmt.Sprint(artifact["logical_name"]) == logicalName {
			return artifact["artifact_id"], nil
		}
	}
	return nil, rpc.NewError("invalid_transition", "review auto-finalize could not resolve findings artifact id", nil)
}

func completeAutoFinalizedJob(ctx context.Context, runner any, repositoryID, jobID, sessionID, leaseID, messageID string) (map[string]any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(job["state"]) != "running" {
		return nil, rpc.NewError("invalid_transition", "job must be running before auto-finalize completion", nil)
	}
	if err := verifyRequiredArtifacts(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	now := nowString()
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, repositoryID, jobID); err != nil {
		return nil, err
	}
	if messageID != "" && messageID != "<nil>" {
		if err := exec.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'completed', completed_at = $1, updated_at = $2,
			       current_lease_id = NULL
			 WHERE repository_id = $3 AND message_id = $4`, now, now, repositoryID, messageID); err != nil {
			return nil, err
		}
	}
	if err := exec.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = COALESCE(released_at, $1),
		       release_reason = COALESCE(release_reason, 'auto_finalized')
		 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, leaseID); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "job.completed", sessionID, jobID, nullable(messageID), nil, leaseID, map[string]any{
		"summary": autoFinalizeSummary,
	}); err != nil {
		return nil, err
	}
	if err := maybeEnqueueDownstream(ctx, runner, repositoryID, jobID); err != nil {
		return nil, err
	}
	if err := maybeCompleteRun(ctx, runner, repositoryID, fmt.Sprint(job["run_id"])); err != nil {
		return nil, err
	}
	return map[string]any{"status": "completed", "job_id": jobID}, nil
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func cloneMap(input map[string]any) map[string]any {
	output := map[string]any{}
	for key, value := range input {
		output[key] = value
	}
	return output
}
