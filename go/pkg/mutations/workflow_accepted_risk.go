package mutations

import (
	"context"
	"errors"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func HandleWorkflowAcceptRisk(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	decisionRef := stringParam(envelope, "decision_artifact_ref")
	if decisionRef == "" {
		decisionRef = stringParam(envelope, "accepted_risk_decision_id")
	}
	if decisionRef == "" {
		return nil, rpc.NewError("schema_invalid", "decision_artifact_ref must be a non-empty string", map[string]any{"field_path": "decision_artifact_ref"})
	}
	rationale := stringParam(envelope, "rationale")
	if rationale == "" {
		rationale = stringParam(envelope, "override_rationale")
	}
	if rationale == "" {
		return nil, rpc.NewError("schema_invalid", "rationale must be a non-empty string", map[string]any{"field_path": "rationale"})
	}
	requested := stringSliceParam(envelope, "finding_fingerprints", "finding_fingerprint_sha256")
	if len(requested) == 0 {
		return nil, rpc.NewError("schema_invalid", "finding_fingerprints must include at least one lint finding fingerprint", map[string]any{"field_path": "finding_fingerprints"})
	}
	acceptedBy := stringParam(envelope, "accepted_by")
	if acceptedBy == "" {
		acceptedBy = stringParam(envelope, "session_id")
	}
	if acceptedBy == "" {
		acceptedBy = "daemon"
	}

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		target, err := workflowAcceptanceTarget(ctx, tx, repositoryID, envelope)
		if err != nil {
			return nil, err
		}
		lintPayload, err := workflowauthoring.Lint(target.workflow)
		if err != nil {
			return nil, err
		}
		fingerprint, err := workflowauthoring.WorkflowFingerprint(target.workflow)
		if err != nil {
			return nil, err
		}
		target.workflowFingerprint = fingerprint
		findings := lintFindingsByFingerprint(lintPayload)
		accepted := []map[string]any{}
		for _, requestedFingerprint := range requested {
			finding, ok := findings[requestedFingerprint]
			if !ok {
				return nil, rpc.NewError("invalid_transition", "accepted-risk fingerprint is not present in current lint payload", map[string]any{"finding_fingerprint_sha256": requestedFingerprint})
			}
			acceptedRiskID, err := newID("war")
			if err != nil {
				return nil, err
			}
			if err := insertWorkflowAcceptedRisk(ctx, tx, repositoryID, acceptedRiskID, target, finding, decisionRef, acceptedBy, rationale); err != nil {
				return nil, err
			}
			accepted = append(accepted, map[string]any{
				"accepted_risk_id":            acceptedRiskID,
				"workflow_snapshot_id":        nullable(target.workflowSnapshotID),
				"workflow_fingerprint_sha256": nullable(target.bindingFingerprint()),
				"workflow_id":                 stringValueAny(target.workflow["workflow_id"]),
				"workflow_version":            nullable(target.workflow["workflow_version"]),
				"source_path":                 nullable(target.sourcePath),
				"lint_rule":                   finding["rule"],
				"finding_fingerprint_sha256":  finding["fingerprint"],
				"decision_artifact_ref":       decisionRef,
				"accepted_by":                 acceptedBy,
				"rationale":                   rationale,
			})
		}
		return map[string]any{
			"accepted_risks":              accepted,
			"count":                       len(accepted),
			"workflow_snapshot_id":        nullable(target.workflowSnapshotID),
			"workflow_fingerprint_sha256": target.workflowFingerprint,
			"accepted_risk_binding":       target.bindingMode(),
			"accepted_risk_decision_ref":  decisionRef,
			"accepted_risk_warning_count": lintPayload["warning_count"],
		}, nil
	})
}

type workflowAcceptanceTargetData struct {
	workflow            map[string]any
	sourcePath          string
	workflowSnapshotID  string
	workflowFingerprint string
}

func (t workflowAcceptanceTargetData) bindingMode() string {
	if t.workflowSnapshotID != "" {
		return "workflow_snapshot"
	}
	return "workflow_fingerprint"
}

func (t workflowAcceptanceTargetData) bindingFingerprint() string {
	if t.workflowSnapshotID != "" {
		return ""
	}
	return t.workflowFingerprint
}

func workflowAcceptanceTarget(ctx context.Context, runner any, repositoryID string, envelope rpc.Envelope) (workflowAcceptanceTargetData, error) {
	if snapshotID := stringParam(envelope, "workflow_snapshot_id"); snapshotID != "" {
		row, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", snapshotID, false)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return workflowAcceptanceTargetData{}, rpc.NewError("workflow_snapshot_not_found", "workflow_snapshot_id was not found", nil)
			}
			return workflowAcceptanceTargetData{}, err
		}
		return workflowAcceptanceTargetData{
			workflow:           asMap(row["workflow_json"]),
			sourcePath:         stringValueAny(row["source_path"]),
			workflowSnapshotID: snapshotID,
		}, nil
	}
	if runID := stringParam(envelope, "run_id"); runID != "" {
		rows, err := queryRows(ctx, runner, `
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
			return workflowAcceptanceTargetData{}, err
		}
		if len(rows) == 0 {
			return workflowAcceptanceTargetData{}, rpc.NewError("run_not_found", "run_id was not found", nil)
		}
		return workflowAcceptanceTargetData{
			workflow:           asMap(rows[0]["workflow_json"]),
			sourcePath:         stringValueAny(rows[0]["source_path"]),
			workflowSnapshotID: stringValueAny(rows[0]["workflow_snapshot_id"]),
		}, nil
	}
	if body, ok := envelope.Params["workflow_json"].(map[string]any); ok {
		if err := workflowauthoring.Validate(body); err != nil {
			return workflowAcceptanceTargetData{}, rpc.NewError("workflow_error", err.Error(), nil)
		}
		return workflowAcceptanceTargetData{workflow: body, sourcePath: stringParam(envelope, "source_path")}, nil
	}
	workflowPath := stringParam(envelope, "workflow_path")
	if workflowPath == "" {
		workflowPath = stringParam(envelope, "path")
	}
	if workflowPath == "" {
		return workflowAcceptanceTargetData{}, rpc.NewError("schema_invalid", "workflow_path, workflow_json, workflow_snapshot_id, or run_id is required", map[string]any{"field_path": "workflow_path"})
	}
	repo, err := rowByID(ctx, runner, repositoryID, "repositories", "repository_id", repositoryID, false)
	if err != nil {
		return workflowAcceptanceTargetData{}, err
	}
	workflow, sourcePath, err := workflowauthoring.LoadFile(fmt.Sprint(repo["repo_root"]), workflowPath)
	if err != nil {
		return workflowAcceptanceTargetData{}, rpc.NewError("workflow_error", err.Error(), nil)
	}
	return workflowAcceptanceTargetData{workflow: workflow, sourcePath: sourcePath}, nil
}

func lintFindingsByFingerprint(payload map[string]any) map[string]map[string]any {
	result := map[string]map[string]any{}
	if warnings, ok := payload["warnings"].([]map[string]any); ok {
		for _, warning := range warnings {
			fingerprint := stringValueAny(warning["fingerprint"])
			if fingerprint != "" {
				result[fingerprint] = warning
			}
		}
	}
	return result
}

func insertWorkflowAcceptedRisk(ctx context.Context, runner any, repositoryID string, acceptedRiskID string, target workflowAcceptanceTargetData, finding map[string]any, decisionRef string, acceptedBy string, rationale string) error {
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}
	findingArg, err := db.JSONBArg(runner, finding)
	if err != nil {
		return err
	}
	err = exec.Exec(ctx, `
		INSERT INTO striatumd.workflow_accepted_risks (
		  repository_id, accepted_risk_id, workflow_snapshot_id,
		  workflow_fingerprint_sha256, workflow_id, workflow_version, source_path,
		  lint_rule, finding_fingerprint_sha256, finding_json,
		  decision_artifact_ref, accepted_by, rationale
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12,$13)`,
		repositoryID,
		acceptedRiskID,
		nullable(target.workflowSnapshotID),
		nullable(target.bindingFingerprint()),
		stringValueAny(target.workflow["workflow_id"]),
		nullable(target.workflow["workflow_version"]),
		nullable(target.sourcePath),
		stringValueAny(finding["rule"]),
		stringValueAny(finding["fingerprint"]),
		findingArg,
		decisionRef,
		acceptedBy,
		rationale,
	)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return rpc.NewError("invalid_transition", "accepted-risk record already exists for this workflow binding and lint finding", map[string]any{
			"lint_rule":                  finding["rule"],
			"finding_fingerprint_sha256": finding["fingerprint"],
		})
	}
	return err
}

func stringValueAny(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
