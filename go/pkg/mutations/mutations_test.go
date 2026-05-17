package mutations

import (
	"context"
	"errors"
	"os"
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
		"work.send_message",
		"work.block",
		"block",
		"work.complete",
		"complete",
		"artifact.publish",
		"publish_artifact",
		"worktree.create",
		"worktree.release",
		"supervise.start",
		"supervise.send",
		"supervise.stop",
		"workflow.init",
		"workflow.generate",
		"workflow.upgrade",
		"dogfood.publish_on_behalf",
		"dogfood.surgical_recovery",
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
		"recovery.auto_publish_stale_artifacts",
		"recovery.auto",
		"recovery.auto_finalize",
		"supervise.report",
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

func TestSendMessageBodyDefaultsAndValidatesObjects(t *testing.T) {
	body, err := sendMessageBody(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 0 {
		t.Fatalf("default body = %#v", body)
	}

	body, err = sendMessageBody(map[string]any{"body_json": `{"summary":"working"}`})
	if err != nil {
		t.Fatal(err)
	}
	if body["summary"] != "working" {
		t.Fatalf("body summary = %#v", body["summary"])
	}

	_, err = sendMessageBody(map[string]any{"body_json": `[]`})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("non-object body error = %v, want invalid_transition", err)
	}

	_, err = sendMessageBody(map[string]any{"body_json": `{`})
	rpcErr = nil
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("invalid JSON body error = %v, want schema_invalid", err)
	}
}

func TestRecoveryAutoFinalizeRequiresRunID(t *testing.T) {
	server := rpc.NewServer()
	Register(server, inertRunner{})
	handler := server.Handlers["recovery.auto_finalize"]
	if handler == nil {
		t.Fatal("recovery.auto_finalize was not registered")
	}
	_, err := handler(context.Background(), rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_recovery.auto_finalize",
		Method:        "recovery.auto_finalize",
		Params:        map[string]any{"repository_id": "repo_1"},
	})
	rpcErr := &rpc.Error{}
	if !errors.As(err, &rpcErr) {
		t.Fatalf("returned non-rpc error: %v", err)
	}
	if rpcErr.Code != "schema_invalid" {
		t.Fatalf("error code = %q", rpcErr.Code)
	}
	if rpcErr.Message != "recovery.auto_finalize requires run_id" {
		t.Fatalf("error message = %q", rpcErr.Message)
	}
}

func TestAutoFinalizeDryRunDefaultsToProjectionMode(t *testing.T) {
	dryRun, err := autoFinalizeDryRun(rpc.Envelope{Params: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !dryRun {
		t.Fatal("missing dry_run should default to true")
	}
	dryRun, err = autoFinalizeDryRun(rpc.Envelope{Params: map[string]any{"dry_run": false}})
	if err != nil {
		t.Fatal(err)
	}
	if dryRun {
		t.Fatal("explicit dry_run=false should request live mode")
	}
	_, err = autoFinalizeDryRun(rpc.Envelope{Params: map[string]any{"dry_run": "false"}})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("non-boolean dry_run error = %v, want schema_invalid", err)
	}
}

func TestAutoFinalizeFindingRequiresVerdictIntentFrontMatter(t *testing.T) {
	payload := []byte("---\nschema_version: \"striatum.finding.v1\"\nartifact_kind: \"finding\"\n---\n\nauthor: operator\n")
	_, err := autoFinalizeRequiredFrontMatter("finding", "finding.md", payload)
	if err == nil {
		t.Fatal("finding without verdict_intent should be refused")
	}
	if got := err.Error(); got != "finding artifact front matter missing required field 'verdict_intent'" {
		t.Fatalf("error = %q", got)
	}
}

func TestRegisterWithNilRunnerLeavesServerAlone(t *testing.T) {
	server := rpc.NewServer()
	Register(server, nil)
	if len(server.Handlers) != 0 {
		t.Fatalf("nil runner registered handlers: %v", server.Handlers)
	}
}

func TestRecoveryAutoPublishCanonicalAndDeprecatedAliasRequireRunID(t *testing.T) {
	server := rpc.NewServer()
	Register(server, inertRunner{})

	for _, method := range []string{"recovery.auto_publish_stale_artifacts", "recovery.auto"} {
		handler := server.Handlers[method]
		if handler == nil {
			t.Fatalf("%s was not registered", method)
		}
		_, err := handler(context.Background(), rpc.Envelope{
			SchemaVersion: rpc.SupportedEnvelopeVersion,
			RequestID:     "req_" + method,
			Method:        method,
			Params:        map[string]any{"repository_id": "repo_1"},
		})
		rpcErr := &rpc.Error{}
		if !errors.As(err, &rpcErr) {
			t.Fatalf("%s returned non-rpc error: %v", method, err)
		}
		if rpcErr.Code != "schema_invalid" {
			t.Fatalf("%s error code = %q", method, rpcErr.Code)
		}
		if rpcErr.Message != "recovery.auto_publish_stale_artifacts requires run_id" {
			t.Fatalf("%s error message = %q", method, rpcErr.Message)
		}
	}
}

func TestAutoPublishableArtifactsRequireMatchingBylineForEveryFile(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.WriteFile(repoRoot+"/notes.txt", []byte("author: worker-codex-001\n\nresult\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoRoot+"/wrong.md", []byte("author: reviewer-codex-001\n\nresult\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	job := map[string]any{
		"expected_artifacts_json": []map[string]any{
			{"path": "notes.txt", "kind": "other", "logical_name": "notes", "required": true},
			{"path": "wrong.md", "kind": "other", "logical_name": "wrong", "required": true},
		},
	}
	got, err := autoPublishableArtifacts(context.Background(), nil, "repo_1", repoRoot, job, "sess_1", "author: worker-codex-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["path"] != "notes.txt" {
		t.Fatalf("publishable artifacts = %#v", got)
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
