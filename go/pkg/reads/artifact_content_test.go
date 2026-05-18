package reads

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestArtifactGetContentValidatesParams(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		code   string
	}{
		{
			name:   "missing repository_id",
			params: map[string]any{},
			code:   "repo_not_registered",
		},
		{
			name:   "missing artifact_id",
			params: map[string]any{"repository_id": "repo_1"},
			code:   "schema_invalid",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := HandleArtifactGetContent(context.Background(), nil, rpc.Envelope{Params: c.params})
			if err == nil {
				t.Fatalf("expected error code %q, got nil", c.code)
			}
			rpcErr, ok := err.(*rpc.Error)
			if !ok {
				t.Fatalf("expected *rpc.Error, got %T: %v", err, err)
			}
			if rpcErr.Code != c.code {
				t.Fatalf("expected code %q, got %q", c.code, rpcErr.Code)
			}
		})
	}
}

func TestArtifactListForRunValidatesParams(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		code   string
	}{
		{
			name:   "missing repository_id",
			params: map[string]any{},
			code:   "repo_not_registered",
		},
		{
			name:   "missing run_id",
			params: map[string]any{"repository_id": "repo_1"},
			code:   "schema_invalid",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := HandleArtifactListForRun(context.Background(), nil, rpc.Envelope{Params: c.params})
			if err == nil {
				t.Fatalf("expected error code %q, got nil", c.code)
			}
			rpcErr, ok := err.(*rpc.Error)
			if !ok {
				t.Fatalf("expected *rpc.Error, got %T: %v", err, err)
			}
			if rpcErr.Code != c.code {
				t.Fatalf("expected code %q, got %q", c.code, rpcErr.Code)
			}
		})
	}
}
