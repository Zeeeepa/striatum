package reads

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
)

func HandleWorkflowValidate(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repoRoot, workflowPath, err := workflowAuthoringInputs(ctx, runner, envelope)
	if err != nil {
		return nil, err
	}
	workflow, _, err := workflowauthoring.LoadFile(repoRoot, workflowPath)
	if err != nil {
		return nil, workflowAuthoringError(err)
	}
	return map[string]any{
		"workflow_id": workflow["workflow_id"],
		"valid":       true,
	}, nil
}

func HandleWorkflowLint(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	workflow, sourcePath, snapshotID, err := workflowLintTarget(ctx, runner, repositoryID, envelope)
	if err != nil {
		return nil, err
	}
	payload, err := workflowauthoring.Lint(workflow)
	if err != nil {
		return nil, workflowAuthoringError(err)
	}
	fingerprint, err := workflowauthoring.WorkflowFingerprint(workflow)
	if err != nil {
		return nil, err
	}
	acceptedRisks, err := workflowAcceptedRiskRows(ctx, runner, repositoryID, snapshotID, fingerprint)
	if err != nil {
		return nil, err
	}
	payload["workflow_fingerprint_sha256"] = fingerprint
	payload["workflow_snapshot_id"] = nullableStringValue(snapshotID)
	payload["source_path"] = nullableStringValue(sourcePath)
	payload["accepted_risks"] = acceptedRisks
	annotateAcceptedRiskFindings(payload, acceptedRisks)
	return payload, nil
}

func HandleWorkflowPlan(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repoRoot, workflowPath, err := workflowAuthoringInputs(ctx, runner, envelope)
	if err != nil {
		return nil, err
	}
	workflow, _, err := workflowauthoring.LoadFile(repoRoot, workflowPath)
	if err != nil {
		return nil, workflowAuthoringError(err)
	}
	plan, err := workflowauthoring.Plan(workflow)
	if err != nil {
		return nil, workflowAuthoringError(err)
	}
	return plan, nil
}

func HandleWorkflowAcceptedRisksList(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	snapshotID := stringParam(envelope, "workflow_snapshot_id")
	fingerprint := stringParam(envelope, "workflow_fingerprint_sha256")
	rows, err := collectRows(ctx, runner, `
		SELECT accepted_risk_id, workflow_snapshot_id, workflow_fingerprint_sha256,
		       workflow_id, workflow_version, source_path, lint_rule,
		       finding_fingerprint_sha256, finding_json, decision_artifact_ref,
		       accepted_by, accepted_at, expires_at, rationale
		  FROM striatumd.workflow_accepted_risks
		 WHERE repository_id = $1
		   AND ($2 = '' OR workflow_snapshot_id = $2)
		   AND ($3 = '' OR workflow_fingerprint_sha256 = $3)
		 ORDER BY accepted_at DESC, accepted_risk_id DESC`,
		repositoryID,
		snapshotID,
		fingerprint,
	)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, workflowAcceptedRiskRow(row))
	}
	return map[string]any{"accepted_risks": items, "count": len(items)}, nil
}

func HandleWorkflowGraph(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repoRoot, workflowPath, err := workflowAuthoringInputs(ctx, runner, envelope)
	if err != nil {
		return nil, err
	}
	format, err := graphChoice(envelope, "format", "mermaid", map[string]bool{
		"json": true, "mermaid": true, "dot": true,
	})
	if err != nil {
		return nil, err
	}
	workflow, _, err := workflowauthoring.LoadFile(repoRoot, workflowPath)
	if err != nil {
		return nil, workflowAuthoringError(err)
	}
	graph, err := workflowauthoring.Graph(workflow, format)
	if err != nil {
		return nil, workflowAuthoringError(err)
	}
	return graph, nil
}

func workflowAuthoringInputs(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (string, string, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return "", "", err
	}
	workflowPath := stringParam(envelope, "workflow_path")
	if workflowPath == "" {
		workflowPath = stringParam(envelope, "path")
	}
	if workflowPath == "" {
		return "", "", rpc.NewError("schema_invalid", "workflow_path must be a non-empty string", map[string]any{"field_path": "workflow_path"})
	}
	repoRoot, err := workflowAuthoringRepoRoot(ctx, runner, repositoryID)
	if err != nil {
		return "", "", err
	}
	return repoRoot, workflowPath, nil
}

func workflowLintTarget(ctx context.Context, runner db.Runner, repositoryID string, envelope rpc.Envelope) (map[string]any, string, string, error) {
	if snapshotID := stringParam(envelope, "workflow_snapshot_id"); snapshotID != "" {
		row, err := workflowSnapshotRow(ctx, runner, repositoryID, snapshotID)
		if err != nil {
			return nil, "", "", err
		}
		return jsonMap(row["workflow_json"]), stringFrom(row, "source_path"), snapshotID, nil
	}
	if runID := stringParam(envelope, "run_id"); runID != "" {
		rows, err := collectRows(ctx, runner, `
			SELECT w.workflow_snapshot_id, w.source_path, w.workflow_json
			  FROM striatumd.runs r
			  JOIN striatumd.workflow_snapshots w
			    ON w.repository_id = r.repository_id
			   AND w.workflow_snapshot_id = r.workflow_snapshot_id
			 WHERE r.repository_id = $1 AND r.run_id = $2
			 LIMIT 1`,
			repositoryID,
			runID,
		)
		if err != nil {
			return nil, "", "", err
		}
		if len(rows) == 0 {
			return nil, "", "", rpc.NewError("run_not_found", "run_id was not found", nil)
		}
		return jsonMap(rows[0]["workflow_json"]), stringFrom(rows[0], "source_path"), stringFrom(rows[0], "workflow_snapshot_id"), nil
	}
	if body, ok := envelope.Params["workflow_json"].(map[string]any); ok {
		if err := workflowauthoring.Validate(body); err != nil {
			return nil, "", "", workflowAuthoringError(err)
		}
		return body, stringParam(envelope, "source_path"), "", nil
	}
	repoRoot, workflowPath, err := workflowAuthoringInputs(ctx, runner, envelope)
	if err != nil {
		return nil, "", "", err
	}
	workflow, sourcePath, err := workflowauthoring.LoadFile(repoRoot, workflowPath)
	if err != nil {
		return nil, "", "", workflowAuthoringError(err)
	}
	return workflow, sourcePath, "", nil
}

func workflowSnapshotRow(ctx context.Context, runner db.Runner, repositoryID string, snapshotID string) (map[string]any, error) {
	rows, err := collectRows(ctx, runner, `
		SELECT workflow_snapshot_id, source_path, workflow_json
		  FROM striatumd.workflow_snapshots
		 WHERE repository_id = $1 AND workflow_snapshot_id = $2
		 LIMIT 1`,
		repositoryID,
		snapshotID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, rpc.NewError("workflow_snapshot_not_found", "workflow_snapshot_id was not found", nil)
	}
	return rows[0], nil
}

func workflowAcceptedRiskRows(ctx context.Context, runner db.Runner, repositoryID string, snapshotID string, fingerprint string) ([]map[string]any, error) {
	rows, err := collectRows(ctx, runner, `
		SELECT accepted_risk_id, workflow_snapshot_id, workflow_fingerprint_sha256,
		       workflow_id, workflow_version, source_path, lint_rule,
		       finding_fingerprint_sha256, finding_json, decision_artifact_ref,
		       accepted_by, accepted_at, expires_at, rationale
		  FROM striatumd.workflow_accepted_risks
		 WHERE repository_id = $1
		   AND (
		     ($2 <> '' AND workflow_snapshot_id = $2)
		     OR
		     ($3 <> '' AND workflow_fingerprint_sha256 = $3)
		   )
		 ORDER BY accepted_at DESC, accepted_risk_id DESC`,
		repositoryID,
		snapshotID,
		fingerprint,
	)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, workflowAcceptedRiskRow(row))
	}
	return items, nil
}

func workflowAcceptedRiskRow(row map[string]any) map[string]any {
	return map[string]any{
		"accepted_risk_id":            row["accepted_risk_id"],
		"workflow_snapshot_id":        row["workflow_snapshot_id"],
		"workflow_fingerprint_sha256": row["workflow_fingerprint_sha256"],
		"workflow_id":                 row["workflow_id"],
		"workflow_version":            row["workflow_version"],
		"source_path":                 row["source_path"],
		"lint_rule":                   row["lint_rule"],
		"finding_fingerprint_sha256":  row["finding_fingerprint_sha256"],
		"finding":                     jsonMap(row["finding_json"]),
		"decision_artifact_ref":       row["decision_artifact_ref"],
		"accepted_by":                 row["accepted_by"],
		"accepted_at":                 row["accepted_at"],
		"expires_at":                  row["expires_at"],
		"rationale":                   row["rationale"],
	}
}

func annotateAcceptedRiskFindings(payload map[string]any, acceptedRisks []map[string]any) {
	byFingerprint := map[string][]any{}
	for _, risk := range acceptedRisks {
		fingerprint, _ := risk["finding_fingerprint_sha256"].(string)
		if fingerprint == "" {
			continue
		}
		byFingerprint[fingerprint] = append(byFingerprint[fingerprint], risk["accepted_risk_id"])
	}
	warnings, ok := payload["warnings"].([]map[string]any)
	if !ok {
		return
	}
	for _, warning := range warnings {
		fingerprint, _ := warning["fingerprint"].(string)
		ids := byFingerprint[fingerprint]
		if len(ids) == 0 {
			warning["accepted"] = false
			continue
		}
		warning["accepted"] = true
		warning["accepted_risk_ids"] = ids
	}
}

func jsonMap(value any) map[string]any {
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

func workflowAuthoringRepoRoot(ctx context.Context, runner db.Runner, repositoryID string) (string, error) {
	if runner == nil {
		return "", rpc.NewError("daemon_db_missing", "workflow authoring handlers require repository metadata", nil)
	}
	rows, err := collectRows(ctx, runner,
		`SELECT repo_root
		   FROM striatumd.repositories
		  WHERE repository_id = $1 AND state <> 'removed'
		  LIMIT 1`,
		repositoryID,
	)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", rpc.NewError("repo_not_registered", "daemon RPC route requires repository_id", nil)
	}
	repoRoot := stringFrom(rows[0], "repo_root")
	if repoRoot == "" {
		return "", rpc.NewError("repo_not_registered", "daemon repository has no repo_root", nil)
	}
	return repoRoot, nil
}

func workflowAuthoringError(err error) error {
	var workflowErr *workflowauthoring.Error
	if !errors.As(err, &workflowErr) {
		return err
	}
	details := map[string]any{}
	if workflowErr.FieldPath != "" {
		details["field_path"] = workflowErr.FieldPath
	}
	return rpc.NewError("workflow_error", workflowErr.Message, details)
}
