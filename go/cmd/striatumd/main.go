package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	daemonapply "github.com/halbritt/striatum/go/pkg/apply"
	"github.com/halbritt/striatum/go/pkg/crossrepo"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/reads"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/supervisor"
)

// supervisorPointerStore is the daemon's boot-time handle on the
// Postgres-backed supervisor.PointerStore implementation. Construction is
// gated on the Postgres pool being present; consumers (e.g. supervise.list
// + supervise.status handlers — RFC 0048 Phase B follow-up) read it via
// the Server's free-form attachments. V1.7 ships the wire-up; the
// not_implemented handlers stay until RFC 0048 Phase B lands the real
// handler ports.
var supervisorPointerStore *db.SupervisorPointerStore
var _ supervisor.PointerStore = (*supervisorPointerStoreAdapter)(nil)

// supervisorPointerStoreAdapter bridges db.PointerRow ↔ supervisor.PointerRow.
// They are the same wire shape; the adapter exists only so the supervisor
// package interface compiles against the db-side concrete store without
// pulling the supervisor import into the db package (would create an
// import cycle).
type supervisorPointerStoreAdapter struct {
	store *db.SupervisorPointerStore
}

func (a *supervisorPointerStoreAdapter) UpsertSupervisorPointer(ctx context.Context, row supervisor.PointerRow) error {
	return a.store.UpsertSupervisorPointer(ctx, db.PointerRow{
		SupervisorID:    row.SupervisorID,
		RepositoryID:    row.RepositoryID,
		SessionID:       row.SessionID,
		PID:             row.PID,
		StartedAt:       row.StartedAt,
		LastHeartbeatAt: row.LastHeartbeatAt,
		StdinPipePath:   row.StdinPipePath,
		State:           row.State,
		LostReason:      row.LostReason,
	})
}

func (a *supervisorPointerStoreAdapter) MarkSupervisorLost(ctx context.Context, supervisorID string, reason string) error {
	return a.store.MarkSupervisorLost(ctx, supervisorID, reason)
}

func (a *supervisorPointerStoreAdapter) GetSupervisorPointer(ctx context.Context, supervisorID string) (supervisor.PointerRow, error) {
	row, err := a.store.GetSupervisorPointer(ctx, supervisorID)
	if err != nil {
		return supervisor.PointerRow{}, err
	}
	return supervisor.PointerRow{
		SupervisorID:    row.SupervisorID,
		RepositoryID:    row.RepositoryID,
		SessionID:       row.SessionID,
		PID:             row.PID,
		StartedAt:       row.StartedAt,
		LastHeartbeatAt: row.LastHeartbeatAt,
		StdinPipePath:   row.StdinPipePath,
		State:           row.State,
		LostReason:      row.LostReason,
	}, nil
}

const daemonVersion = "go-dev"

func main() {
	var socketPath string
	var postgresURL string
	var migrate bool
	var describe bool
	var migrationsSHASource string
	flag.StringVar(&socketPath, "socket", defaultSocketPath(), "Unix socket path")
	flag.StringVar(&postgresURL, "postgres-url", "", "PostgreSQL connection URL")
	flag.BoolVar(&migrate, "migrate", true, "apply daemon PostgreSQL migrations before serving when a URL is configured")
	flag.BoolVar(&describe, "describe", false, "print daemon metadata and exit")
	flag.StringVar(&migrationsSHASource, "migrations-sha-source", "", "verify embedded migration SHAs against SQL files at this path before serving")
	flag.Parse()

	if describe {
		fmt.Printf("core=go envelope=%d framing=%s methods_etag=%s\n", rpc.SupportedEnvelopeVersion, rpc.DefaultFraming, rpc.MethodsETag())
		return
	}

	if migrationsSHASource != "" {
		if err := db.VerifyMigrationsSHASource(migrationsSHASource); err != nil {
			log.Fatalf("migrations sha source check failed: %v", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	substrateSchema := 0
	var recorder *db.AuditRecorder
	var runner db.Runner
	var authorizer rpc.Authorizer
	config := db.ResolveConfig(postgresURL)
	// RFC 0039 V1.6 F2 (dogfood-047 codex finding): the daemon must
	// refuse to bind a socket without a configured Postgres URL.
	// Production has no use for an unauthenticated, no-audit daemon;
	// installing AllowAllAuthorizer{} as a default was a security
	// regression that let `daemon describe` run without recording an
	// audit row. Refuse here; AllowAllAuthorizer{} remains available
	// for unit tests that explicitly construct a server.
	if config.URL == "" {
		log.Fatalf("striatumd refuses to start without a Postgres URL; pass --postgres-url or set STRIATUM_DAEMON_DB_URL")
	}
	{
		var pool *db.Pool
		var err error
		if migrate {
			var version int
			pool, version, err = db.ConnectAndMigrate(ctx, config.URL, daemonVersion)
			if err != nil {
				log.Fatalf("daemon db connect/migrate failed: %v", err)
			}
			substrateSchema = version
		} else {
			pool, err = db.Connect(ctx, config.URL, daemonVersion)
			if err != nil {
				log.Fatalf("daemon db connect failed: %v", err)
			}
		}
		defer pool.Close()
		runner = pool.Runner
		recorder = &db.AuditRecorder{Runner: pool.Runner, DaemonVersion: daemonVersion}
		authorizer = &rpc.PostgresAuthorizer{Runner: pool.Runner, Clock: time.Now}
		if pool.RawPool != nil {
			supervisorPointerStore = db.NewSupervisorPointerStore(pool.RawPool)
		}
	}
	_ = supervisorPointerStore // RFC 0048 Phase B will wire supervise.* handlers

	server := rpc.NewServer()
	server.DaemonVersion = daemonVersion
	server.SubstrateSchema = substrateSchema
	server.Authorizer = authorizer
	if recorder != nil {
		server.AuditRecorder = recorder
	}
	registerHandlers(server, runner)

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		log.Fatalf("create socket directory: %v", err)
	}
	listener, err := rpc.ListenUnix(socketPath)
	if err != nil {
		log.Fatalf("listen on %s: %v", socketPath, err)
	}
	log.Printf("striatumd-go listening on %s", socketPath)
	if err := server.Serve(ctx, listener); err != nil && ctx.Err() == nil {
		log.Fatalf("serve: %v", err)
	}
}

func registerHandlers(server *rpc.Server, runner db.Runner) {
	daemonapply.Service{Runner: runner}.Register(server)
	registerCrossRepoHandlers(server, runner)
	// RFC 0048 Phase B: register the Go-core read-surface handlers
	// before the not-implemented stub loop so the loop's existence-check
	// skips them. Mirrors src/striatum/daemon_pg/handlers/reads/ in
	// Python; same response shapes.
	reads.Register(server, runner)
	mutations.Register(server, runner)
	for _, method := range []string{
		"status", "why", "doctor", "dashboard", "dashboard.all",
		"evidence.export", "corpus.export", "run.summary", "run.graph",
		"workflow.validate", "workflow.plan", "workflow.graph",
		"workflow.templates.list", "workflow.templates.show",
		"workflow.generate.preview", "list.runs", "list.sessions",
		"list.jobs", "list.artifacts", "list.workflows", "worktree.list",
		"repo.list", "session.register", "session.close", "work.claim_next",
		"work.ack", "work.heartbeat", "work.release", "supervise.start",
		"supervise.send", "supervise.stop", "supervise.status",
		"supervise.list", "supervise.reattach_status", "work.send_message",
		"work.block", "work.complete", "artifact.publish", "worktree.create",
		"worktree.release", "workflow.init", "workflow.generate",
		"workflow.upgrade", "dogfood.publish_on_behalf", "review.submit",
		"review.verdict", "review.override", "decision.record",
		"checkpoint.resolve", "branch.confirm", "run.prepare", "run.start",
		"run.pause", "run.resume", "run.cancel", "run.retry_job", "repo.init",
		"recovery.stale_leases", "recovery.requeue_stale",
		"recovery.cancel_job", "recovery.process_reconcile", "recovery.resume",
		"recovery.auto", "dogfood.surgical_recovery",
		"repo.add", "repo.remove", "daemon.token.create", "daemon.token.revoke",
		"daemon.token.rotate", "daemon.key.rotate", "daemon.shutdown",
		"daemon.migrate", "daemon.migrate_repo_local", "ack", "heartbeat",
		"release", "block", "complete", "publish_artifact", "claim_next",
		"verdict", "submit_review",
	} {
		if _, exists := server.Handlers[method]; exists {
			continue
		}
		server.Register(method, notImplementedHandler(method))
	}
}

func registerCrossRepoHandlers(server *rpc.Server, runner db.Runner) {
	server.Register("cross_repo.list", func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		if runner == nil {
			return nil, rpc.NewError("daemon_db_missing", "cross-repo routes require daemon PostgreSQL", nil)
		}
		return crossrepo.ListRuns(ctx, runner)
	})
	server.Register("cross_repo.describe", func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		if runner == nil {
			return nil, rpc.NewError("daemon_db_missing", "cross-repo routes require daemon PostgreSQL", nil)
		}
		runID := param(envelope.Params, "cross_repo_run_id")
		if runID == "" {
			runID = param(envelope.Params, "run_id")
		}
		if runID == "" {
			return nil, rpc.NewError("schema_invalid", "cross-repo route requires cross_repo_run_id", nil)
		}
		return crossrepo.DescribeRun(ctx, runner, runID)
	})
	server.Register("cross_repo.why", func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		if runner == nil {
			return nil, rpc.NewError("daemon_db_missing", "cross-repo routes require daemon PostgreSQL", nil)
		}
		runID := param(envelope.Params, "cross_repo_run_id")
		if runID == "" {
			runID = param(envelope.Params, "run_id")
		}
		if runID == "" {
			return nil, rpc.NewError("schema_invalid", "cross-repo route requires cross_repo_run_id", nil)
		}
		return crossrepo.Why(ctx, runner, runID)
	})
	server.Register("cross_repo.cancel", func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		return nil, rpc.NewError("not_implemented", "cross-repo cancel requires the daemon lifecycle service", nil)
	})
}

func notImplementedHandler(method string) rpc.Handler {
	return func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		return nil, rpc.NewError("not_implemented", fmt.Sprintf("%s is registered but not implemented in the Go daemon yet", method), nil)
	}
}

func param(params map[string]any, key string) string {
	if value, ok := params[key].(string); ok {
		return value
	}
	return ""
}

func defaultSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	return filepath.Join(runtimeDir, "striatum", "daemon-go.sock")
}
