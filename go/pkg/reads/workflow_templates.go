package reads

import (
	"context"
	"errors"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

func HandleWorkflowTemplatesList(_ context.Context, _ db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	if _, err := requireRepositoryID(envelope); err != nil {
		return nil, err
	}
	kind, err := optionalTemplateKind(envelope)
	if err != nil {
		return nil, err
	}
	catalog, err := workflowtemplates.Load()
	if err != nil {
		return nil, workflowTemplateError(err)
	}
	templates, err := catalog.List(kind)
	if err != nil {
		return nil, workflowTemplateError(err)
	}
	return map[string]any{"templates": templates}, nil
}

func HandleWorkflowTemplatesShow(_ context.Context, _ db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	if _, err := requireRepositoryID(envelope); err != nil {
		return nil, err
	}
	templateID, err := workflowTemplateID(envelope)
	if err != nil {
		return nil, err
	}
	catalog, err := workflowtemplates.Load()
	if err != nil {
		return nil, workflowTemplateError(err)
	}
	template, err := catalog.Get(templateID)
	if err != nil {
		return nil, workflowTemplateError(err)
	}
	return template, nil
}

func optionalTemplateKind(envelope rpc.Envelope) (string, error) {
	raw, ok := envelope.Params["kind"]
	if !ok || raw == nil {
		return "", nil
	}
	kind, ok := raw.(string)
	if !ok {
		return "", rpc.NewError("schema_invalid", "kind must be shape, lane_set, role_pack, or adversary_pack", map[string]any{"field_path": "kind"})
	}
	return kind, nil
}

func workflowTemplateID(envelope rpc.Envelope) (string, error) {
	for _, key := range []string{"template_id", "workflow_template_id"} {
		value, ok := envelope.Params[key].(string)
		if ok && value != "" {
			return value, nil
		}
	}
	return "", rpc.NewError("schema_invalid", "workflow.templates.show requires template_id", map[string]any{"field_path": "template_id"})
}

func workflowTemplateError(err error) error {
	var catalogErr *workflowtemplates.Error
	if !errors.As(err, &catalogErr) {
		return err
	}
	details := map[string]any{}
	if catalogErr.FieldPath != "" {
		details["field_path"] = catalogErr.FieldPath
	}
	if catalogErr.Hint != "" {
		details["hint"] = catalogErr.Hint
	}
	return rpc.NewError("workflow_error", catalogErr.Message, details)
}
