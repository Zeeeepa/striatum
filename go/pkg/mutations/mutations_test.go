package mutations

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type inertRunner struct{}

func (inertRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected exec")
}

func (inertRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (inertRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (inertRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("unexpected tx")
}

type fakeRow struct {
	err error
}

func (r fakeRow) Scan(...any) error {
	return r.err
}

func TestRegisterInstallsInitialMutationHandlers(t *testing.T) {
	server := rpc.NewServer()
	Register(server, inertRunner{})

	for _, method := range []string{
		"session.register",
		"session.close",
		"work.claim_next",
		"claim_next",
		"work.ack",
		"ack",
		"work.heartbeat",
		"heartbeat",
		"work.release",
		"release",
		"work.block",
		"block",
		"work.complete",
		"complete",
		"artifact.publish",
		"publish_artifact",
		"review.verdict",
		"verdict",
		"review.submit",
		"submit_review",
		"review.override",
		"run.prepare",
		"run.start",
		"run.pause",
		"run.resume",
		"run.cancel",
		"run.retry_job",
		"branch.confirm",
		"decision.record",
		"checkpoint.resolve",
		"recovery.stale_leases",
		"recovery.requeue_stale",
		"recovery.cancel_job",
		"recovery.process_reconcile",
		"recovery.resume",
		"recovery.sweep",
		"recovery.auto",
	} {
		handler, ok := server.Handlers[method]
		if !ok {
			t.Fatalf("%s was not registered", method)
		}
		_, err := handler(context.Background(), rpc.Envelope{
			SchemaVersion: rpc.SupportedEnvelopeVersion,
			RequestID:     "req_" + method,
			Method:        method,
			Params:        map[string]any{},
		})
		rpcErr := &rpc.Error{}
		if !errors.As(err, &rpcErr) || rpcErr.Code != "repo_not_registered" {
			t.Fatalf("%s handler did not run expected repo-scope validation: %v", method, err)
		}
	}
}

func TestRegisterWithNilRunnerLeavesServerAlone(t *testing.T) {
	server := rpc.NewServer()
	Register(server, nil)
	if len(server.Handlers) != 0 {
		t.Fatalf("nil runner registered handlers: %v", server.Handlers)
	}
}

func TestWorkflowDeclaresFreshReviewer(t *testing.T) {
	if workflowDeclaresFreshReviewer(map[string]any{"jobs": []any{map[string]any{"type": "draft"}}}) {
		t.Fatal("draft job should not trigger fresh reviewer policy")
	}
	if !workflowDeclaresFreshReviewer(map[string]any{"jobs": []any{map[string]any{"type": "review", "reviewer_context_policy": "fresh"}}}) {
		t.Fatal("fresh review policy was not detected")
	}
	if !workflowDeclaresFreshReviewer(map[string]any{"jobs": []any{map[string]any{"type": "review", "fresh_session_required": true}}}) {
		t.Fatal("fresh_session_required review was not detected")
	}
}

func TestValidateOperatorLabelRejectsReservedLane(t *testing.T) {
	_, err := validateOperatorLabel("codex", map[string]any{"lanes": map[string]any{"codex": map[string]any{}}})
	if err == nil {
		t.Fatal("expected reserved lane label to be rejected")
	}
	cleaned, err := validateOperatorLabel("  local.operator_1 ", map[string]any{"lanes": map[string]any{"codex": map[string]any{}}})
	if err != nil {
		t.Fatalf("valid label rejected: %v", err)
	}
	if cleaned != "local.operator_1" {
		t.Fatalf("cleaned label = %q", cleaned)
	}
}

func TestWorkflowPhaseEdgesMaterializeSynthesisFanIn(t *testing.T) {
	workflow := map[string]any{
		"schema_version": "striatum.workflow.v1.1",
		"workflow_id":    "wf-phases",
		"branch":         map[string]any{"mode": "confirm", "suggested_name": "wf/phases"},
		"roles":          map[string]any{"worker": map[string]any{}},
		"lanes":          map[string]any{"lane_a": map[string]any{}},
		"phases": []any{
			map[string]any{"id": "phase_1_design", "name": "Design"},
			map[string]any{"id": "phase_2_build", "name": "Build"},
		},
		"jobs": []any{
			phaseJob("design_a", "phase_1_design", "handoff"),
			phaseJob("synthesize_design", "phase_1_design", "phase_synthesis"),
			phaseJob("build_a", "phase_2_build", "handoff"),
			phaseJob("synthesize_build", "phase_2_build", "phase_synthesis"),
		},
		"edges": []any{
			map[string]any{"from": "synthesize_design", "to": "build_a", "on": "completed"},
		},
	}
	index, err := validateWorkflowForPrepare(workflow)
	if err != nil {
		t.Fatalf("workflow validation failed: %v", err)
	}
	pairs := edgeDependencyPairs(workflow, index, true)
	got := map[string]bool{}
	for _, pair := range pairs {
		got[pair.fromID+"->"+pair.toID] = true
	}
	for _, expected := range []string{
		"design_a->synthesize_design",
		"synthesize_design->build_a",
		"build_a->synthesize_build",
	} {
		if !got[expected] {
			t.Fatalf("missing materialized edge %s in %#v", expected, got)
		}
	}
}

func phaseJob(id string, phaseID string, jobType string) map[string]any {
	return map[string]any{
		"id":       id,
		"type":     jobType,
		"role_id":  "worker",
		"lane_id":  "lane_a",
		"phase_id": phaseID,
		"write_scope": map[string]any{
			"allowed_paths": []any{"docs/"},
		},
		"expected_artifacts": []any{},
	}
}
