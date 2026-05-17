package reads

import (
	"context"
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
