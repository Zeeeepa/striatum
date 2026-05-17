package reads

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestDetailReadHandlersValidateBeforeQuery(t *testing.T) {
	tests := []struct {
		name    string
		handler handlerFn
		params  map[string]any
		code    string
	}{
		{
			name:    "run events since cursor",
			handler: HandleRunEvents,
			params:  map[string]any{"repository_id": "repo_1", "run_id": "run_1", "since_event_id": -1},
			code:    "schema_invalid",
		},
		{
			name:    "posture path characters",
			handler: HandleRunPostureVerdicts,
			params:  map[string]any{"repository_id": "repo_1", "run_id": "run_1", "posture": "../bad"},
			code:    "schema_invalid",
		},
		{
			name:    "escalation state",
			handler: HandleEscalationList,
			params:  map[string]any{"repository_id": "repo_1", "state": "bad"},
			code:    "schema_invalid",
		},
		{
			name:    "artifact id",
			handler: HandleArtifactShow,
			params:  map[string]any{"repository_id": "repo_1"},
			code:    "schema_invalid",
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
