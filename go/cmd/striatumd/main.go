package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/admin"
	daemonapply "github.com/halbritt/striatum/go/pkg/apply"
	"github.com/halbritt/striatum/go/pkg/crossrepo"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/reads"
	recoverypkg "github.com/halbritt/striatum/go/pkg/recovery"
	"github.com/halbritt/striatum/go/pkg/repositories"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/supervisor"
)

// supervisorPointerStore is the daemon's boot-time handle on the
// Postgres-backed supervisor.PointerStore implementation. Construction is
// gated on the Postgres pool being present; supervise handlers read it via the
// Server's free-form attachments.
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
	var sweepIntervalSeconds float64
	var maxSweeps optionalIntFlag
	flag.StringVar(&socketPath, "socket", defaultSocketPath(), "Unix socket path")
	flag.StringVar(&postgresURL, "postgres-url", "", "PostgreSQL connection URL")
	flag.BoolVar(&migrate, "migrate", true, "apply daemon PostgreSQL migrations before serving when a URL is configured")
	flag.BoolVar(&describe, "describe", false, "print daemon metadata and exit")
	flag.StringVar(&migrationsSHASource, "migrations-sha-source", "", "verify embedded migration SHAs against SQL files at this path before serving")
	flag.Float64Var(&sweepIntervalSeconds, "sweep-interval-seconds", 60.0, "seconds between resident recovery sweeps")
	flag.Var(&maxSweeps, "max-sweeps", "maximum resident recovery sweeps before exiting; when set to 0, one startup sweep still runs")
	flag.Parse()

	if describe {
		migrationSHAs, err := db.MigrationSHASet()
		if err != nil {
			log.Fatalf("load migration sha set: %v", err)
		}
		fmt.Printf(
			"core=go envelope=%d framing=%s supported_schema=%d methods_etag=%s daemon_version=%s migration_count=%d\n",
			rpc.SupportedEnvelopeVersion,
			rpc.DefaultFraming,
			db.LatestDaemonDBVersion,
			rpc.MethodsETag(),
			daemonVersion,
			len(migrationSHAs),
		)
		return
	}

	if migrationsSHASource != "" {
		if err := db.VerifyMigrationsSHASource(migrationsSHASource); err != nil {
			log.Fatalf("migrations sha source check failed: %v", err)
		}
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

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
		tokenPath, err := admin.RuntimeTokenPath()
		if err != nil {
			log.Fatalf("resolve daemon runtime token path: %v", err)
		}
		bootstrap, err := admin.BootstrapRuntimeTokenIfNeeded(ctx, runner, tokenPath)
		if err != nil {
			log.Fatalf("bootstrap daemon admin token failed: %v", err)
		}
		if bootstrap != nil {
			log.Printf("bootstrapped daemon admin client %s and wrote runtime token %s", bootstrap["client_id"], tokenPath)
		}
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
	server.SealedApplyFunc = daemonapply.FallbackSigningKeyStatus
	server.Authorizer = authorizer
	if recorder != nil {
		server.AuditRecorder = recorder
	}
	var shutdownOnce sync.Once
	shutdownHook := func(context.Context) error {
		go func() {
			// Let the RPC response flush before closing the listener.
			time.Sleep(50 * time.Millisecond)
			shutdownOnce.Do(cancel)
		}()
		return nil
	}
	registerHandlers(server, runner, handlerOptions{
		ShutdownHook:  shutdownHook,
		KeyRotateHook: daemonapply.RotateFallbackSigningKey,
	})

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		log.Fatalf("create socket directory: %v", err)
	}
	listener, err := rpc.ListenUnix(socketPath)
	if err != nil {
		log.Fatalf("listen on %s: %v", socketPath, err)
	}
	log.Printf("striatumd-go listening on %s", socketPath)
	schedulerErr := startRecoveryScheduler(ctx, cancel, runner, sweepIntervalSeconds, maxSweeps)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	if err := server.Serve(ctx, listener); err != nil && ctx.Err() == nil {
		cancel()
		log.Fatalf("serve: %v", err)
	}
	cancel()
	if schedulerErr != nil {
		err := <-schedulerErr
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("recovery scheduler: %v", err)
		}
	}
}

func startRecoveryScheduler(ctx context.Context, cancel context.CancelFunc, runner db.Runner, sweepIntervalSeconds float64, maxSweepsFlag optionalIntFlag) <-chan error {
	done := make(chan error, 1)
	var maxSweeps *int
	if maxSweepsFlag.Provided {
		value := maxSweepsFlag.Value
		maxSweeps = &value
	}
	interval := time.Duration(sweepIntervalSeconds * float64(time.Second))
	go func() {
		result, err := recoverypkg.RunScheduler(ctx, recoverypkg.SchedulerOptions{
			Interval:  interval,
			MaxSweeps: maxSweeps,
			SweepOnce: recoverypkg.ActiveRunSweep{
				Runner: runner,
				Author: "striatumd-go",
			}.SweepOnce,
		})
		if err == nil {
			log.Printf("recovery scheduler stopped after %d sweep(s): %s", result.Sweeps, result.Reason)
			if result.Reason == recoverypkg.ReasonMaxSweepsReached {
				cancel()
			}
		} else if !errors.Is(err, context.Canceled) {
			cancel()
		}
		done <- err
	}()
	return done
}

type optionalIntFlag struct {
	Value    int
	Provided bool
}

func (f *optionalIntFlag) Set(value string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	f.Value = parsed
	f.Provided = true
	return nil
}

func (f *optionalIntFlag) String() string {
	if f == nil || !f.Provided {
		return ""
	}
	return strconv.Itoa(f.Value)
}

type handlerOptions struct {
	ShutdownHook  admin.ShutdownFunc
	KeyRotateHook admin.KeyRotateFunc
}

func registerHandlers(server *rpc.Server, runner db.Runner, opts ...handlerOptions) {
	options := handlerOptions{}
	if len(opts) > 0 {
		options = opts[0]
	}
	admin.Service{
		Runner:        runner,
		DaemonVersion: daemonVersion,
		ShutdownHook:  options.ShutdownHook,
		KeyRotateHook: options.KeyRotateHook,
	}.Register(server)
	daemonapply.Service{Runner: runner}.Register(server)
	registerCrossRepoHandlers(server, runner)
	// RFC 0048 Phase B: register the Go-core read-surface handlers
	// before the not-implemented stub loop so the loop's existence-check
	// skips them. Mirrors src/striatum/daemon_pg/handlers/reads/ in
	// Python; same response shapes.
	reads.Register(server, runner)
	mutations.Register(server, runner)
	repositories.Service{Runner: runner}.Register(server)
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
		"workflow.upgrade", "review.submit",
		"review.verdict", "review.override", "decision.record",
		"checkpoint.resolve", "branch.confirm", "run.prepare", "run.start",
		"run.pause", "run.resume", "run.cancel", "run.retry_job", "repo.init",
		"recovery.stale_leases", "recovery.requeue_stale",
		"recovery.cancel_job", "recovery.process_reconcile", "recovery.resume",
		"recovery.auto",
		"repo.add", "repo.remove", "daemon.token.create", "daemon.token.revoke",
		"daemon.token.rotate", "daemon.key.rotate", "daemon.shutdown",
		"daemon.migrate", "ack", "heartbeat",
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
	local := localWorkflowRunner{runner: runner}
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
		reason := param(envelope.Params, "reason")
		if reason == "" {
			reason = "operator canceled cross-repo run"
		}
		return crossrepo.CancelRun(ctx, runner, runID, reason, local)
	})
}

type localWorkflowRunner struct {
	runner db.Runner
}

func (l localWorkflowRunner) Prepare(ctx context.Context, repositoryID string, alias string, crossRepoRunID string) (string, error) {
	return "", fmt.Errorf("cross-repo local prepare is not wired in the Go daemon")
}

func (l localWorkflowRunner) Start(ctx context.Context, repositoryID string, localRunID string) error {
	_, err := mutations.HandleRunStart(ctx, l.runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "cross_repo_start_" + localRunID,
		Method:        "run.start",
		Params: map[string]any{
			"repository_id": repositoryID,
			"run_id":        localRunID,
		},
	})
	return err
}

func (l localWorkflowRunner) Cancel(ctx context.Context, repositoryID string, localRunID string, reason string) error {
	active, err := l.runner.QueryScalar(ctx, "SELECT repo_root FROM striatumd.repositories WHERE repository_id = $1 AND state = 'active'", repositoryID)
	if err != nil {
		return err
	}
	if active == "" {
		return fmt.Errorf("active repository not found: %s", repositoryID)
	}
	_, err = mutations.HandleRunCancel(ctx, l.runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "cross_repo_cancel_" + localRunID,
		Method:        "run.cancel",
		Params: map[string]any{
			"repository_id": repositoryID,
			"run_id":        localRunID,
			"reason":        reason,
		},
	})
	return err
}

func (l localWorkflowRunner) ParticipantIntact(ctx context.Context, repositoryID string, localRunID *string) bool {
	return false
}

func (l localWorkflowRunner) HumanCheckpoint(ctx context.Context, repositoryID string, localRunID *string, reason string) error {
	return nil
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
