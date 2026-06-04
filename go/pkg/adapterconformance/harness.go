package adapterconformance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mcp"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/reads"
	"github.com/halbritt/striatum/go/pkg/repositories"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

// Harness is the RFC 0101 Layer 2 in-process daemon test harness (DESIGN §1.3
// runner.go step 1-2). It provisions an isolated, migrated PostgreSQL database
// via pgtest and assembles the SAME production RPC + MCP stack the real
// striatumd boots (see go/cmd/striatumd/main.go lines ~346, ~628-648), then
// exposes it over restartable loopback HTTP + unix-socket surfaces with an
// ephemeral capability token. The fake agent (testagent/) drives this real stack
// over MCP; the DaemonObserver reads the same database to evaluate the live
// clauses.
//
// This is NOT the live daemon and NOT the live PostgreSQL: it is an ephemeral
// pgtest database + in-process daemon surfaces, torn down with the test. When
// STRIATUM_PG_TEST_URL is unset, pgtest.Pool SKIPs the test cleanly.
type Harness struct {
	// Runner is the migrated pgtest database runner the production handlers and
	// the DaemonObserver both read/write.
	Runner db.Runner
	// Server is the production RPC server with the real mutation + read +
	// repository handlers registered.
	Server *rpc.Server
	// MCP is the production MCP HTTP handler wrapping Server, with the
	// session-activity recorder wired (so tools/list stamps last_tools_list_at).
	MCP *mcp.HTTPHandler
	// HTTPURL is the stable loopback base URL serving MCP. Restartable harnesses
	// rebind this same address so a launched lane keeps the endpoint it received
	// at startup.
	HTTPURL string
	// Token is the ephemeral bearer capability token (id.secret form) granting
	// read/claim/write scoped to RepositoryID.
	Token string
	// RepositoryID is the repo the token is scoped to (set by the fixture).
	RepositoryID string
	// SocketPath is a unix-domain socket serving the SAME production rpc.Server
	// over the newline-framed JSON protocol the daemon uses (rpc.Server.Serve).
	// The MCP HTTP endpoint above serves a lane CLI's tools/call traffic;
	// this socket serves the agent-loop receive loop (startDaemonReceiverLoop ->
	// rpcclient.Client over cfg.SocketPath), which is the surface the
	// installed-CLI conformance runner drives. Empty for harnesses that only need
	// the in-process testagent (MCP-only).
	SocketPath string

	authorizer *rpc.MemoryAuthorizer

	mu         sync.Mutex
	httpAddr   string
	httpServer *trackedHTTPServer
	socket     *trackedSocketServer
}

// MCPEndpoint returns the MCP HTTP endpoint URL.
func (h *Harness) MCPEndpoint() string {
	return h.HTTPURL + mcp.EndpointPath
}

// Observer returns a DaemonObserver reading the harness database. The pgtest
// runner is a db.PgxRunner, which exposes the Query method the observer needs;
// a runner without Query yields a nil-query observer (its reads error rather
// than panic).
func (h *Harness) Observer() DaemonObserver {
	q, _ := h.Runner.(queryer)
	return DaemonObserver{Runner: q}
}

// NewHarness provisions the in-process daemon stack over an isolated pgtest
// database. It registers the production mutation, read, and repository handlers
// — the REAL handlers, not a fake service — so the fake agent exercises the
// genuine bootstrap/turn/ack/heartbeat/publish/complete state machine. The
// capability token grants read+claim+write so the agent can register a session,
// await/ack/heartbeat work, answer interrogations, and publish+complete.
//
// The caller supplies repositoryID up front (the fixture seeds it); the token is
// scoped to it. All resources register t.Cleanup so the test tears them down.
func NewHarness(t *testing.T, repositoryID string) *Harness {
	t.Helper()

	runner := pgtest.Pool(t).Runner

	authorizer := rpc.NewMemoryAuthorizer()
	token := mintToken(t)
	expiry := time.Now().Add(time.Hour)
	grant := rpc.CapabilityGrant{RepositoryID: repositoryID, ExpiresAt: expiry}
	authorizer.AddToken(token, "conformance-fake-agent", map[rpc.Capability]rpc.CapabilityGrant{
		rpc.CapabilityRead:  grant,
		rpc.CapabilityClaim: grant,
		rpc.CapabilityWrite: grant,
	}, expiry)

	h := &Harness{
		Runner:       runner,
		Token:        token,
		RepositoryID: repositoryID,
		authorizer:   authorizer,
	}
	h.rebuildDaemonStackLocked()
	if err := h.startHTTPLocked(""); err != nil {
		t.Fatalf("start harness MCP HTTP: %v", err)
	}

	// Unix-socket surface for the agent-loop receive loop. A short temp dir
	// keeps the socket path under the OS sun_path limit (~108 bytes on Linux); a
	// long t.TempDir() subtest path can overflow it. The listener serves the SAME
	// server over rpc.Server.Serve (the production daemon path); Serve returns
	// when the listener closes on ctx cancel.
	sockDir, err := os.MkdirTemp("", "stsock")
	if err != nil {
		t.Fatalf("mkdir socket dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	h.SocketPath = filepath.Join(sockDir, "d.sock")
	if err := h.startSocketLocked(); err != nil {
		t.Fatalf("start harness unix socket: %v", err)
	}
	t.Cleanup(func() {
		h.Close()
	})

	return h
}

// RestartDaemonSurfaces simulates a daemon restart for conformance tests: it
// drops the harness's externally visible HTTP and unix-socket listeners,
// rebuilds the in-process production RPC/MCP stack, then rebinds the same
// addresses. The pgtest database stays alive, matching the daemon-owned
// PostgreSQL boundary; launched lanes keep using the endpoint/socket they were
// handed at startup.
func (h *Harness) RestartDaemonSurfaces() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr := h.httpAddr
	h.stopHTTPLocked()
	h.stopSocketLocked()
	h.rebuildDaemonStackLocked()
	if err := h.startHTTPLocked(addr); err != nil {
		return err
	}
	if err := h.startSocketLocked(); err != nil {
		h.stopHTTPLocked()
		return err
	}
	return nil
}

// Close stops the harness's externally visible daemon surfaces. It does not
// close Runner; pgtest owns that lifecycle.
func (h *Harness) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopHTTPLocked()
	h.stopSocketLocked()
}

func (h *Harness) rebuildDaemonStackLocked() {
	server := rpc.NewServer()
	server.Authorizer = h.authorizer
	// Production handler registration (mirrors registerHandlers in striatumd):
	// the read surface, the mutation surface, and the repository surface, all
	// over the real pgtest runner.
	reads.Register(server, h.Runner, reads.Options{})
	mutations.Register(server, h.Runner, mutations.Options{})
	repositories.Service{Runner: h.Runner}.Register(server)

	h.Server = server
	h.MCP = mcp.NewHTTPHandler(mcp.Service{
		RPC:              server,
		Authorizer:       h.authorizer,
		ActivityRecorder: sessionliveness.DBRecorder{Runner: h.Runner},
	})
}

func (h *Harness) startHTTPLocked(addr string) error {
	listener, err := listenTCPRetry(addr)
	if err != nil {
		return err
	}
	h.httpAddr = listener.Addr().String()
	h.HTTPURL = "http://" + h.httpAddr
	h.httpServer = newTrackedHTTPServer(listener, h.MCP)
	h.httpServer.start()
	return nil
}

func (h *Harness) stopHTTPLocked() {
	if h.httpServer == nil {
		return
	}
	h.httpServer.close()
	h.httpServer = nil
}

func (h *Harness) startSocketLocked() error {
	if strings.TrimSpace(h.SocketPath) == "" {
		return nil
	}
	listener, err := rpc.ListenUnix(h.SocketPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	server := h.Server
	go func() {
		defer close(done)
		_ = server.Serve(ctx, listener)
	}()
	h.socket = &trackedSocketServer{listener: listener, cancel: cancel, done: done}
	return nil
}

func (h *Harness) stopSocketLocked() {
	if h.socket == nil {
		return
	}
	h.socket.close()
	h.socket = nil
}

func listenTCPRetry(addr string) (net.Listener, error) {
	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:0"
	}
	deadline := time.Now().Add(2 * time.Second)
	var last error
	for {
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return listener, nil
		}
		last = err
		if strings.HasSuffix(addr, ":0") || time.Now().After(deadline) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	return nil, fmt.Errorf("listen tcp %s: %w", addr, last)
}

type trackedHTTPServer struct {
	server   *http.Server
	listener net.Listener
	done     chan struct{}

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func newTrackedHTTPServer(listener net.Listener, handler http.Handler) *trackedHTTPServer {
	t := &trackedHTTPServer{
		listener: listener,
		done:     make(chan struct{}),
		conns:    map[net.Conn]struct{}{},
	}
	t.server = &http.Server{Handler: handler}
	t.server.ConnState = func(conn net.Conn, state http.ConnState) {
		t.mu.Lock()
		defer t.mu.Unlock()
		switch state {
		case http.StateNew, http.StateActive, http.StateIdle:
			t.conns[conn] = struct{}{}
		case http.StateHijacked, http.StateClosed:
			delete(t.conns, conn)
		}
	}
	return t
}

func (t *trackedHTTPServer) start() {
	go func() {
		defer close(t.done)
		_ = t.server.Serve(t.listener)
	}()
}

func (t *trackedHTTPServer) close() {
	_ = t.server.Close()
	t.mu.Lock()
	for conn := range t.conns {
		_ = conn.Close()
	}
	t.mu.Unlock()
	<-t.done
}

type trackedSocketServer struct {
	listener net.Listener
	cancel   context.CancelFunc
	done     chan struct{}
}

func (t *trackedSocketServer) close() {
	t.cancel()
	_ = t.listener.Close()
	<-t.done
}

// mintToken generates an ephemeral id.secret capability token.
func mintToken(t *testing.T) string {
	t.Helper()
	idBytes := make([]byte, 8)
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(idBytes); err != nil {
		t.Fatalf("mint token id: %v", err)
	}
	if _, err := rand.Read(secretBytes); err != nil {
		t.Fatalf("mint token secret: %v", err)
	}
	return "conf_" + hex.EncodeToString(idBytes) + "." + hex.EncodeToString(secretBytes)
}
