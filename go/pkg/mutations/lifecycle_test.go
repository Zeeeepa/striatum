package mutations

import (
	"context"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestRegisterSessionDefaultsToWorkflowLaneCapabilities(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_caps"
	runID := "run_lifecycle_caps"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles": map[string]any{
			"reviewer": map[string]any{},
		},
		"lanes": map[string]any{
			"claude_code": map[string]any{
				"capabilities": []any{"write", "interrogate", "write"},
			},
		},
	})

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"role":   "reviewer",
		"lane":   "claude_code",
	}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	got := registeredSessionCapabilities(t, ctx, runner, repoID, fmt.Sprint(result["session_id"]))
	want := []string{"write", "interrogate"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
}

func TestRegisterSessionExplicitCapabilitiesOverrideLaneDefault(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_lifecycle_caps_explicit"
	runID := "run_lifecycle_caps_explicit"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles": map[string]any{
			"reviewer": map[string]any{},
		},
		"lanes": map[string]any{
			"codex": map[string]any{
				"capabilities": []any{"write", "interrogate"},
			},
		},
	})

	result, err := HandleRegisterSession(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id":     runID,
		"role":       "reviewer",
		"lane":       "codex",
		"capability": []any{"review", "review", " "},
	}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	got := registeredSessionCapabilities(t, ctx, runner, repoID, fmt.Sprint(result["session_id"]))
	want := []string{"review"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
}

func registeredSessionCapabilities(t *testing.T, ctx context.Context, runner any, repoID, sessionID string) []string {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT capabilities_json
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID)
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	result := []string{}
	for _, item := range asList(row["capabilities_json"]) {
		result = append(result, fmt.Sprint(item))
	}
	return result
}
