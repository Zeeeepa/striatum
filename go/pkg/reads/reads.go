// Package reads contains the Go RFC 0048 Phase B transition handlers for
// read-surface CLI verbs. Each exported Handle* function mirrors a Python
// read handler in src/striatum/daemon_pg/handlers/reads/ and returns the same
// top-level JSON shape so compatibility fixtures can compare the two paths.
//
// Scope (this file holds the shared helpers; per-method files in this
// package hold the handlers):
//   - status, dashboard, doctor, why — core operator reads.
//   - list.runs, list.sessions, list.jobs, list.artifacts,
//     list.workflows — listing reads.
//   - run.summary, evidence.export, corpus.export — reporting reads.
//
// Every handler scopes by repository_id and runs SELECT-only SQL.
package reads

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

// Queryer narrows db.Runner to the multi-row Query method used by the
// read package; crossrepo uses the same shape internally.
type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func queryer(runner db.Runner) (Queryer, error) {
	q, ok := runner.(Queryer)
	if !ok {
		return nil, fmt.Errorf("runner does not support row queries")
	}
	return q, nil
}

func requireRepositoryID(envelope rpc.Envelope) (string, error) {
	value, _ := envelope.Params["repository_id"].(string)
	if value == "" {
		return "", rpc.NewError("repo_not_registered", "daemon RPC route requires repository_id", nil)
	}
	return value, nil
}

func stringParam(envelope rpc.Envelope, key string) string {
	if value, ok := envelope.Params[key].(string); ok {
		return value
	}
	return ""
}

func collectRows(ctx context.Context, runner db.Runner, sql string, args ...any) ([]map[string]any, error) {
	q, err := queryer(runner)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToMap)
}

// Register wires every read handler in this package onto the rpc.Server.
// Call from cmd/striatumd/main.go after the registry-side handlers land.
func Register(server *rpc.Server, runner db.Runner) {
	if runner == nil {
		return
	}
	server.Register("status", makeHandler(runner, HandleStatus))
	server.Register("dashboard", makeHandler(runner, HandleDashboard))
	server.Register("doctor", makeHandler(runner, HandleDoctor))
	server.Register("why", makeHandler(runner, HandleWhy))
	server.Register("list.runs", makeHandler(runner, HandleListRuns))
	server.Register("list.sessions", makeHandler(runner, HandleListSessions))
	server.Register("list.jobs", makeHandler(runner, HandleListJobs))
	server.Register("list.artifacts", makeHandler(runner, HandleListArtifacts))
	server.Register("list.workflows", makeHandler(runner, HandleListWorkflows))
	server.Register("run.summary", makeHandler(runner, HandleRunSummary))
	server.Register("run.detail", makeHandler(runner, HandleRunDetail))
	server.Register("job.detail", makeHandler(runner, HandleJobDetail))
	server.Register("run.events", makeHandler(runner, HandleRunEvents))
	server.Register("run.posture_verdicts", makeHandler(runner, HandleRunPostureVerdicts))
	server.Register("artifact.show", makeHandler(runner, HandleArtifactShow))
	server.Register("evidence.export", makeHandler(runner, HandleEvidenceExport))
	server.Register("corpus.export", makeHandler(runner, HandleCorpusExport))
	server.Register("escalation.list", makeHandler(runner, HandleEscalationList))
	server.Register("escalation.show", makeHandler(runner, HandleEscalationShow))
}

// handlerFn is the per-method signature: (ctx, runner, envelope) → response.
type handlerFn func(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error)

func makeHandler(runner db.Runner, fn handlerFn) rpc.Handler {
	return func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		return fn(ctx, runner, envelope)
	}
}
