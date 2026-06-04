package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/agentloop"
	daemonapply "github.com/halbritt/striatum/go/pkg/apply"
	"github.com/halbritt/striatum/go/pkg/blob"
	"github.com/halbritt/striatum/go/pkg/crossrepo"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mcp"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/reads"
	recoverypkg "github.com/halbritt/striatum/go/pkg/recovery"
	"github.com/halbritt/striatum/go/pkg/repositories"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
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

var (
	daemonVersion    = "go-dev"
	buildGitSHA      = "unknown"
	buildDirty       = "unknown"
	globalSocketPath string
)

func main() {
	var socketPath string
	var postgresURL string
	var migrate bool
	var describe bool
	var migrationsSHASource string
	var sweepIntervalSeconds float64
	var maxSweeps optionalIntFlag
	var agentLoop bool
	var mcpHTTPAddr string
	var webTailscale bool
	var auditHashFormat string
	flag.StringVar(&socketPath, "socket", defaultSocketPath(), "Unix socket path")
	flag.StringVar(&auditHashFormat, "audit-hash-format", envOr("STRIATUM_AUDIT_HASH_FORMAT", "v2"), "RFC 0110 §5.2: audit row hash format (v2|v3); default v2. Flipping to v3 is the forward-only cutover and requires the owner bundle applied + a restart.")
	flag.BoolVar(&webTailscale, "web-tailscale", envBool("STRIATUM_DAEMON_WEB_TAILSCALE"), "RFC 0085: serve a read-only tailnet-identity UI on a dedicated 0600 unix socket ($STRIATUM_DAEMON_RUNTIME_DIR/web-ui.sock) for `tailscale serve`; default off; loopback bind + bearer path unchanged")
	flag.StringVar(&postgresURL, "postgres-url", "", "PostgreSQL connection URL")
	flag.StringVar(&mcpHTTPAddr, "mcp-http-addr", defaultMCPHTTPAddr(), "loopback HTTP/SSE MCP listen address; use 'off' to disable")
	flag.BoolVar(&migrate, "migrate", true, "apply daemon PostgreSQL migrations before serving when a URL is configured")
	flag.BoolVar(&describe, "describe", false, "print daemon metadata and exit")
	flag.BoolVar(&agentLoop, "agent-loop", false, "run as the interactive MCP agent loop instead of a daemon server")
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
			"core=go envelope=%d framing=%s supported_schema=%d methods_etag=%s daemon_version=%s git_sha=%s build_dirty=%s migration_count=%d\n",
			rpc.SupportedEnvelopeVersion,
			rpc.DefaultFraming,
			db.LatestDaemonDBVersion,
			rpc.MethodsETag(),
			daemonVersion,
			buildGitSHA,
			buildDirty,
			len(migrationSHAs),
		)
		return
	}

	if agentLoop {
		repoRoot := os.Getenv("STRIATUM_REPO")
		runID := os.Getenv("STRIATUM_RUN_ID")
		sessionID := os.Getenv("STRIATUM_SESSION_ID")

		if err := agentloop.Run(socketPath, repoRoot, runID, sessionID, flag.Args()); err != nil {
			log.Fatalf("agent-loop failed: %v", err)
		}
		os.Exit(0)
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
	var webServiceToken string
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
		// The runtime pool may be reconnected below if L0 rotates the password,
		// so the deferred close binds to the variable, not the initial pool.
		defer func() { pool.Close() }()
		runner = pool.Runner
		// RFC 0110 §8.2 (C-DEPLOY-CAPABILITY-PARITY): verify the binary and the
		// live schema agree on authority-bearing capabilities before serving
		// mutations. Inert when no owner bundle has stamped a capability; once
		// the N+1 owner bundle is applied, a binary that does not support a
		// stamped capability fails closed here (old-binary / authority-schema).
		if err := db.VerifyCapabilityParity(ctx, runner,
			db.RequiredAuthorityCapabilities(), db.SupportedAuthorityCapabilities()); err != nil {
			log.Fatalf("daemon capability parity check failed: %v", err)
		}
		// RFC 0110 §9 (L0 credential bootstrap): over an owner connection,
		// generate + register this instance's authority secret and (two-role
		// only) rotate the runtime password. Inert when the owner bundle is not
		// applied (authority schema absent) — so a daemon without the bundle is
		// unaffected. Fail-closed on any owner-connection failure (§9.2).
		authResult, err := db.BootstrapAuthority(ctx, db.BootstrapConfig{
			RuntimeURL:    config.URL,
			OwnerURL:      strings.TrimSpace(os.Getenv("STRIATUM_OWNER_DB_URL")),
			RuntimeRole:   runtimeRoleFromURL(config.URL),
			InstanceID:    daemonInstanceID(),
			DaemonVersion: daemonVersion,
		})
		if err != nil {
			log.Fatalf("daemon authority bootstrap failed: %v", err)
		}
		if authResult.NewRuntimeURL != "" {
			// L0 rotated the runtime password: reconnect the pool with it before
			// serving (RFC 0110 §9.1). The old pool's open connections are closed.
			pool.Close()
			rotatedPool, err := db.Connect(ctx, authResult.NewRuntimeURL, daemonVersion)
			if err != nil {
				log.Fatalf("daemon reconnect after credential rotation failed: %v", err)
			}
			pool = rotatedPool
			runner = pool.Runner
		}
		db.SetAuthorityRuntime(authResult.Secret, auditHashFormat, authResult.Posture, authResult.RotatorCollision)
		log.Printf("daemon authority: posture=%s audit_hash_format=%s registered=%t rotator_collision=%t",
			authResult.Posture, auditHashFormat, authResult.Registered, authResult.RotatorCollision)
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
		// The runtime client token (bootstrapped above or pre-existing) is the
		// same bearer the MCP listener and CLI present. The mounted web service
		// reuses it to gate HTTP access and author its downstream RPC calls.
		webServiceToken = readRuntimeTokenFile(tokenPath)
		if webServiceToken == "" {
			// RFC 0084 D1 build-review finding: the runtime token is the web
			// service's bearer, and the web handler authenticates open when its
			// ServiceToken is empty. If the token file is unreadable, fail
			// CLOSED — inject an unguessable deny token so mounted /v1 routes
			// reject every request (401) while MCP keeps its own auth — rather
			// than mounting the web surface without authentication.
			webServiceToken = randomDenyToken()
			log.Printf("warning: daemon runtime token %s unreadable; web service locked with a deny token (/v1 will 401 until the token is restored)", tokenPath)
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
	server.AuditRecorder = recorder
	var shutdownOnce sync.Once
	shutdownHook := func(context.Context) error {
		go func() {
			// Let the RPC response flush before closing the listener.
			time.Sleep(50 * time.Millisecond)
			shutdownOnce.Do(cancel)
		}()
		return nil
	}
	blobClient, err := loadBlobClient()
	if err != nil {
		log.Fatalf("blob client setup: %v", err)
	}
	if blobClient != nil {
		log.Printf("blob storage configured")
	}
	registerHandlers(server, runner, handlerOptions{
		ShutdownHook:  shutdownHook,
		KeyRotateHook: daemonapply.RotateFallbackSigningKey,
		BlobClient:    blobClient,
	})

	globalSocketPath = socketPath
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		log.Fatalf("create socket directory: %v", err)
	}
	pidPath := daemonPidfilePath(socketPath)
	if err := writeDaemonPidfile(pidPath, os.Getpid()); err != nil {
		log.Fatalf("write daemon pidfile: %v", err)
	}
	defer func() {
		if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove daemon pidfile %s: %v", pidPath, err)
		}
	}()
	listener, err := rpc.ListenUnix(socketPath)
	if err != nil {
		log.Fatalf("listen on %s: %v", socketPath, err)
	}
	log.Printf("striatumd-go listening on %s", socketPath)
	stopMCPHTTP, err := startMCPHTTPServer(ctx, cancel, mcpHTTPAddr, server, authorizer, runner, resolveWebServiceOptions(webServiceToken))
	if err != nil {
		_ = listener.Close()
		_ = os.Remove(pidPath)
		log.Fatalf("start MCP HTTP/SSE server: %v", err)
	}
	if stopMCPHTTP != nil {
		defer stopMCPHTTP()
	}
	if webTailscale {
		stopWebUI, err := startWebUISocket(ctx, server, resolveWebUIOptions(webServiceToken))
		if err != nil {
			_ = listener.Close()
			_ = os.Remove(pidPath)
			if stopMCPHTTP != nil {
				stopMCPHTTP()
			}
			log.Fatalf("start tailnet-identity UI socket: %v", err)
		}
		if stopWebUI != nil {
			defer stopWebUI()
		}
	}
	schedulerErr := startRecoveryScheduler(ctx, cancel, runner, sweepIntervalSeconds, maxSweeps)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	if err := server.Serve(ctx, listener); err != nil && ctx.Err() == nil {
		cancel()
		if stopMCPHTTP != nil {
			stopMCPHTTP()
		}
		log.Fatalf("serve: %v", err)
	}
	cancel()
	if schedulerErr != nil {
		err := <-schedulerErr
		if err != nil && !errors.Is(err, context.Canceled) {
			if stopMCPHTTP != nil {
				stopMCPHTTP()
			}
			log.Fatalf("recovery scheduler: %v", err)
		}
	}
}

func startMCPHTTPServer(ctx context.Context, cancel context.CancelFunc, addr string, rpcServer *rpc.Server, authorizer rpc.Authorizer, runner db.Runner, webOpts webServiceOptions) (func(), error) {
	value := strings.TrimSpace(addr)
	if value == "" {
		value = "127.0.0.1:0"
	}
	if strings.EqualFold(value, "off") || strings.EqualFold(value, "disabled") || strings.EqualFold(value, "none") {
		return nil, nil
	}
	listener, err := listenMCPHTTP(value)
	if err != nil {
		return nil, err
	}
	endpoint := mcpEndpointURL(listener.Addr())
	endpointPath, err := writeMCPEndpointFile(endpoint)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	discoveryPath, err := writeDaemonDiscoveryFile(endpoint, webOpts, listener)
	if err != nil {
		_ = listener.Close()
		_ = os.Remove(endpointPath)
		return nil, err
	}
	mcpHandler := mcp.NewHTTPHandler(mcp.Service{
		RPC:              rpcServer,
		Authorizer:       authorizer,
		ActivityRecorder: sessionliveness.DBRecorder{Runner: runner},
	})
	webHandler := newWebServiceHandler(rpcServer, webOpts)
	httpServer := &http.Server{
		Handler:           newDaemonHTTPHandler(mcpHandler, webHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("striatumd-go MCP HTTP/SSE listening on %s (web service mounted at /v1)", endpoint)
	log.Printf("striatumd-go MCP endpoint file %s", endpointPath)
	log.Printf("striatumd-go discovery file %s", discoveryPath)
	go func() {
		err := httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("MCP HTTP/SSE server stopped: %v", err)
			cancel()
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	return func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
		_ = listener.Close()
		if err := os.Remove(endpointPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove MCP endpoint file %s: %v", endpointPath, err)
		}
		if err := os.Remove(discoveryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove discovery file %s: %v", discoveryPath, err)
		}
	}, nil
}

func writeDaemonDiscoveryFile(endpoint string, webOpts webServiceOptions, listener net.Listener) (string, error) {
	runtimeDir, err := admin.RuntimeDir()
	if err != nil {
		return "", err
	}

	_, portStr, err := net.SplitHostPort(listener.Addr().String())
	var portInt int
	if err == nil {
		portInt, _ = strconv.Atoi(portStr)
	}

	data := map[string]any{
		"pid":           os.Getpid(),
		"socket_path":   globalSocketPath,
		"mcp_http_url":  endpoint,
		"mcp_http_port": portInt,
		"client_token":  webOpts.ServiceToken,
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	discoveryPath := filepath.Join(runtimeDir, "discovery.json")
	if err := writeOwnerOnlyJSONFile(discoveryPath, encoded); err != nil {
		return "", err
	}

	return discoveryPath, nil
}

func writeOwnerOnlyJSONFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, "discovery-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmpFile.Write(content); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func listenMCPHTTP(addr string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP HTTP listen address %q: %w", addr, err)
	}
	if host == "" {
		return nil, fmt.Errorf("MCP HTTP listen address %q must bind an explicit loopback host", addr)
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("MCP HTTP listen address %q must bind loopback only", addr)
		}
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok && !tcpAddr.IP.IsLoopback() {
		_ = listener.Close()
		return nil, fmt.Errorf("MCP HTTP listen address %q resolved to non-loopback address %s", addr, tcpAddr.IP.String())
	}
	return listener, nil
}

func mcpEndpointURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String() + mcp.EndpointPath
	}
	return "http://" + net.JoinHostPort(host, port) + mcp.EndpointPath
}

func writeMCPEndpointFile(endpoint string) (string, error) {
	path, err := admin.RuntimeMCPEndpointPath()
	if err != nil {
		return "", err
	}
	if err := writeOwnerOnlyTextFile(path, endpoint+"\n"); err != nil {
		return "", fmt.Errorf("write MCP endpoint file %s: %w", path, err)
	}
	return path, nil
}

// readRuntimeTokenFile reads the daemon runtime client token, returning "" when
// the file is absent or unreadable. An empty token must not be passed to the
// mounted web service as-is (its authenticate() treats an empty ServiceToken as
// open); callers substitute randomDenyToken to fail closed.
func readRuntimeTokenFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// randomDenyToken returns an unguessable bearer the web service requires but
// nobody holds, so /v1 requests fail closed (401) when the runtime token is
// unreadable. On RNG failure it still returns a non-empty constant so the web
// service never authenticates open.
func randomDenyToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "deny-web-token-rng-unavailable"
	}
	return "deny-" + hex.EncodeToString(buf)
}

// runtimeRoleFromURL extracts the runtime role from the runtime DSN (the role
// whose password L0 rotates and whose authority secret is registered). Defaults
// to striatumd_rw when the URL carries no username.
func runtimeRoleFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil || parsed.User.Username() == "" {
		return "striatumd_rw"
	}
	return parsed.User.Username()
}

var daemonInstanceIDOnce struct {
	once  sync.Once
	value string
}

// daemonInstanceID returns this process's stable instance id (RFC 0110 §9.1
// registry key), generated once per daemon.
func daemonInstanceID() string {
	daemonInstanceIDOnce.once.Do(func() {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			daemonInstanceIDOnce.value = "inst-unknown"
			return
		}
		daemonInstanceIDOnce.value = "inst-" + hex.EncodeToString(buf)
	})
	return daemonInstanceIDOnce.value
}

func writeOwnerOnlyTextFile(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
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
	BlobClient    *blob.Client
}

// loadBlobClient builds the daemon's S3 client from environment
// configuration. Returns (nil, nil) when blob storage is opt-out
// (STRIATUM_BLOB_ENDPOINT unset); returns a non-nil error for
// half-configured or malformed configuration. The daemon refuses to
// start when configuration is malformed but happily runs without blob
// storage when it is simply absent (RFC 0072 is opt-in for V1).
func loadBlobClient() (*blob.Client, error) {
	cfg, err := blob.LoadConfig()
	if blob.IsDisabled(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return blob.New(cfg)
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
		BlobClient:    options.BlobClient,
	}.Register(server)
	daemonapply.Service{Runner: runner}.Register(server)
	registerCrossRepoHandlers(server, runner)
	// RFC 0048 Phase B: register the Go-core read-surface handlers
	// before the not-implemented stub loop so the loop's existence-check
	// skips them. Mirrors src/striatum/daemon_pg/handlers/reads/ in
	// Python; same response shapes.
	reads.Register(server, runner, reads.Options{BlobClient: options.BlobClient})
	mutations.Register(server, runner, mutations.Options{BlobClient: options.BlobClient})
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

func defaultMCPHTTPAddr() string {
	if value := os.Getenv("STRIATUM_DAEMON_MCP_HTTP_ADDR"); strings.TrimSpace(value) != "" {
		return value
	}
	return "127.0.0.1:0"
}
