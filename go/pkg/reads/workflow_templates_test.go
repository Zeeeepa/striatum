package reads

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestHandleWorkflowTemplatesListReturnsPythonShape(t *testing.T) {
	result, err := HandleWorkflowTemplatesList(context.Background(), nil, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1"},
	})
	if err != nil {
		t.Fatalf("HandleWorkflowTemplatesList: %v", err)
	}
	templates := result["templates"].([]map[string]any)
	ids := map[string]string{}
	for _, entry := range templates {
		ids[stringFrom(entry, "template_id")] = stringFrom(entry, "kind")
	}
	if ids["multi_phase"] != "shape" || ids["author_reviewer"] != "lane_set" {
		t.Fatalf("unexpected template IDs: %#v", ids)
	}
}

func TestHandleWorkflowTemplatesListKindFilter(t *testing.T) {
	result, err := HandleWorkflowTemplatesList(context.Background(), nil, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "kind": "shape"},
	})
	if err != nil {
		t.Fatalf("HandleWorkflowTemplatesList: %v", err)
	}
	for _, entry := range result["templates"].([]map[string]any) {
		if stringFrom(entry, "kind") != "shape" {
			t.Fatalf("kind filter returned %#v", entry)
		}
	}
}

func TestHandleWorkflowTemplatesShowSupportsCoverageParamName(t *testing.T) {
	result, err := HandleWorkflowTemplatesShow(context.Background(), nil, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "workflow_template_id": "review"},
	})
	if err != nil {
		t.Fatalf("HandleWorkflowTemplatesShow: %v", err)
	}
	if result["template_id"] != "review" || result["kind"] != "shape" {
		t.Fatalf("result = %#v", result)
	}
}

func TestWorkflowTemplateHandlersValidateParams(t *testing.T) {
	tests := []struct {
		name    string
		handler handlerFn
		params  map[string]any
		code    string
	}{
		{
			name:    "invalid kind",
			handler: HandleWorkflowTemplatesList,
			params:  map[string]any{"repository_id": "repo_1", "kind": "bad"},
			code:    "workflow_error",
		},
		{
			name:    "missing template id",
			handler: HandleWorkflowTemplatesShow,
			params:  map[string]any{"repository_id": "repo_1"},
			code:    "schema_invalid",
		},
		{
			name:    "unknown template",
			handler: HandleWorkflowTemplatesShow,
			params:  map[string]any{"repository_id": "repo_1", "template_id": "missing"},
			code:    "workflow_error",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.handler(context.Background(), nil, rpc.Envelope{Params: tc.params})
			var rpcErr *rpc.Error
			if !errors.As(err, &rpcErr) || rpcErr.Code != tc.code {
				t.Fatalf("error = %v, want rpc code %s", err, tc.code)
			}
		})
	}
}
