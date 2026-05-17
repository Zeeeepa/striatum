package mutations

import (
	"context"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowgenerate"
)

func HandleWorkflowGenerate(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	if envelope.Params["confirm_write"] != true {
		return nil, rpc.NewError("schema_invalid", "confirm_write must be true", map[string]any{"field_path": "confirm_write"})
	}
	spec, err := generatorSpecParam(envelope)
	if err != nil {
		return nil, err
	}
	generated, err := workflowgenerate.GenerateFromMap(spec)
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	result, err := workflowgenerate.Write(repoRoot, generated)
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	return result, nil
}

func HandleWorkflowInit(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	target := firstStringParam(envelope, "path", "target", "scaffold_root")
	if target == "" {
		return nil, rpc.NewError("schema_invalid", "workflow.init requires path", map[string]any{"field_path": "path"})
	}
	target, err = workflowgenerate.SafeRelativePath(target, "path")
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	style := stringParam(envelope, "style")
	if style == "" {
		style = "review"
	}
	shape := style
	if style == "code-change" {
		shape = "code_change"
	}
	if style != "minimal" && style != "review" && style != "code-change" {
		return nil, rpc.NewError("workflow_error", fmt.Sprintf("unknown workflow style: %q", style), map[string]any{"field_path": "style"})
	}
	slug := strings.Trim(strings.TrimSuffix(target, "/"), "/")
	if idx := strings.LastIndex(slug, "/"); idx >= 0 {
		slug = slug[idx+1:]
	}
	if slug == "" {
		slug = "starter-workflow"
	}
	spec, err := workflowgenerate.DefaultSpec(
		target,
		"docs/workflows/"+slug,
		shape,
		"local",
		map[string]map[string]any{},
		map[string]any{},
	)
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	spec.WorkflowID = slug + "-starter"
	spec.Name = fmt.Sprintf("%s starter (%s)", slug, style)
	spec.Branch["suggested_name"] = "striatum/" + slug
	generated, err := workflowgenerate.Generate(spec)
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	if _, err := workflowgenerate.SafeTarget(repoRoot, target); err != nil {
		return nil, workflowGenerateError(err)
	}
	if _, err := workflowgenerate.Write(repoRoot, generated); err != nil {
		return nil, workflowGenerateError(err)
	}
	return map[string]any{
		"status":        "created",
		"path":          target,
		"workflow_path": target + "/workflow.json",
		"style":         style,
		"files":         legacyInitFileList(style),
	}, nil
}

func HandleWorkflowUpgrade(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	path := firstStringParam(envelope, "path", "workflow", "workflow_path")
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	queryer, ok := runner.(workflowgenerate.RunQueryer)
	if !ok {
		return nil, workflowGenerateError(&workflowgenerate.Error{Message: "workflow upgrade cannot verify non-terminal runs because daemon PostgreSQL query access is unavailable", FieldPath: "path"})
	}
	result, err := workflowgenerate.Upgrade(ctx, queryer, repositoryID, repoRoot, workflowgenerate.UpgradeOptions{
		Path:      path,
		Force:     boolParam(envelope, "force"),
		DryRun:    boolParam(envelope, "dry_run"),
		AddPhases: boolParam(envelope, "add_phases"),
		Apply:     boolParam(envelope, "apply"),
	})
	if err != nil {
		return nil, workflowGenerateError(err)
	}
	return result, nil
}

func generatorSpecParam(envelope rpc.Envelope) (map[string]any, error) {
	value, ok := envelope.Params["spec"].(map[string]any)
	if !ok || value == nil {
		return nil, rpc.NewError("schema_invalid", "missing spec object", map[string]any{"field_path": "spec"})
	}
	return value, nil
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

func firstStringParam(envelope rpc.Envelope, keys ...string) string {
	for _, key := range keys {
		if value, ok := envelope.Params[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func legacyInitFileList(style string) []string {
	if style == "minimal" {
		return []string{"roles/author.md", "prompts/draft.md", "workflow.json"}
	}
	return []string{
		"roles/author.md",
		"roles/reviewer.md",
		"prompts/draft.md",
		"prompts/review.md",
		"prompts/apply.md",
		"workflow.json",
	}
}
