package reads

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowgenerate"
)

func HandleWorkflowGeneratePreview(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	spec, err := generatorSpecParam(envelope)
	if err != nil {
		return nil, err
	}
	generated, err := workflowgenerate.GenerateFromMap(spec)
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	repoRoot, err := workflowGenerateRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	planned, err := workflowgenerate.Preview(repoRoot, generated)
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	result := generated.Map()
	result["planned_writes"] = planned
	return result, nil
}

func generatorSpecParam(envelope rpc.Envelope) (map[string]any, error) {
	value, ok := envelope.Params["spec"].(map[string]any)
	if !ok || value == nil {
		return nil, rpc.NewError("schema_invalid", "missing spec object", map[string]any{"field_path": "spec"})
	}
	return value, nil
}

func workflowGenerateRepositoryRoot(ctx context.Context, runner db.Runner, repositoryID string) (string, error) {
	rows, err := collectRows(ctx, runner, `
		SELECT repo_root
		  FROM striatumd.repositories
		 WHERE repository_id = $1
		   AND state != 'removed'
		 LIMIT 1`, repositoryID)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", rpc.NewError("repo_not_registered", "unknown registered repository: "+repositoryID, nil)
	}
	root := fmt.Sprint(rows[0]["repo_root"])
	if root == "" {
		return "", rpc.NewError("repo_not_registered", "registered repository has no repo_root: "+repositoryID, nil)
	}
	return root, nil
}

func workflowGenerateError(err error) error {
	if genErr, ok := err.(*workflowgenerate.Error); ok {
		details := map[string]any{}
		if genErr.FieldPath != "" {
			details["field_path"] = genErr.FieldPath
		}
		if genErr.Hint != "" {
			details["hint"] = genErr.Hint
		}
		if genErr.Ref != "" {
			details["ref"] = genErr.Ref
		}
		return rpc.NewError("workflow_error", genErr.Message, details)
	}
	return err
}
