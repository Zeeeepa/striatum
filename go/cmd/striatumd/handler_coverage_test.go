package main

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

var errCoverageRunner = errors.New("coverage runner does not execute SQL")

type coverageRunner struct{}

func (coverageRunner) Exec(context.Context, string, ...any) error {
	return errCoverageRunner
}

func (coverageRunner) QueryRow(context.Context, string, ...any) db.Row {
	return coverageRow{}
}

func (coverageRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errCoverageRunner
}

func (coverageRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errCoverageRunner
}

type coverageRow struct{}

func (coverageRow) Scan(...any) error {
	return errCoverageRunner
}

func TestGoDaemonMethodCoverageIsExplicit(t *testing.T) {
	server := rpc.NewServer()
	registerHandlers(server, coverageRunner{})

	var missingHandlers []string
	var notImplementedHandlers []string
	for _, entry := range rpc.SortedMethods() {
		if entry.Deprecated || entry.Method == "daemon.hello" || entry.Method == "daemon.describe" {
			continue
		}
		handler, ok := server.Handlers[entry.Method]
		if !ok {
			missingHandlers = append(missingHandlers, entry.Method)
			continue
		}
		_, err := handler(context.Background(), rpc.Envelope{
			SchemaVersion: rpc.SupportedEnvelopeVersion,
			RequestID:     "coverage-" + entry.Method,
			Method:        entry.Method,
			Params:        coverageParams(),
		})
		var rpcErr *rpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == "not_implemented" {
			notImplementedHandlers = append(notImplementedHandlers, entry.Method)
		}
	}

	assertSameStrings(t, "missing Go daemon handlers", missingHandlers, nil)
	assertSameStrings(t, "Go daemon not_implemented handlers", notImplementedHandlers, []string{
		"daemon.key.rotate",
		"daemon.migrate_repo_local",
		"daemon.shutdown",
		"daemon.token.create",
		"daemon.token.revoke",
		"daemon.token.rotate",
		"dogfood.publish_on_behalf",
		"dogfood.surgical_recovery",
		"repo.init",
		"supervise.send",
		"supervise.start",
		"supervise.stop",
		"workflow.generate",
		"workflow.generate.preview",
		"workflow.graph",
		"workflow.init",
		"workflow.plan",
		"workflow.upgrade",
		"workflow.validate",
	})
}

func coverageParams() map[string]any {
	return map[string]any{
		"apply_receipt_id":     "receipt_1",
		"artifact_id":          "artifact_1",
		"blocker_id":           "blocker_1",
		"branch":               "main",
		"branch_name":          "main",
		"checkpoint_id":        "checkpoint_1",
		"cross_repo_run_id":    "xrun_1",
		"decision_id":          "decision_1",
		"job_id":               "job_1",
		"lease_id":             "lease_1",
		"message_id":           "msg_1",
		"path":                 "docs/out.md",
		"receipt_id":           "receipt_1",
		"repository_id":        "repo_1",
		"review_id":            "review_1",
		"run_id":               "run_1",
		"session_id":           "sess_1",
		"supervisor_id":        "sup_1",
		"target_id":            "run_1",
		"worktree_id":          "worktree_1",
		"workflow_path":        "workflow.json",
		"workflow_template_id": "default",
	}
}

func assertSameStrings(t *testing.T, label string, got []string, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s changed:\n got: %v\nwant: %v", label, got, want)
	}
}
