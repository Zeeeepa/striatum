package adapterconformance

import (
	"crypto/rand"
	"encoding/hex"
	"net/http/httptest"
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
// exposes it over an httptest loopback server with an ephemeral capability
// token. The fake agent (testagent/) drives this real stack over MCP; the
// DaemonObserver reads the same database to evaluate the live clauses.
//
// This is NOT the live daemon and NOT the live PostgreSQL: it is an ephemeral
// pgtest database + an in-process httptest server, torn down with the test. When
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
	// HTTP is the loopback httptest server serving MCP.
	HTTP *httptest.Server
	// Token is the ephemeral bearer capability token (id.secret form) granting
	// read/claim/write scoped to RepositoryID.
	Token string
	// RepositoryID is the repo the token is scoped to (set by the fixture).
	RepositoryID string

	authorizer *rpc.MemoryAuthorizer
}

// MCPEndpoint returns the MCP HTTP endpoint URL (httptest base + /mcp).
func (h *Harness) MCPEndpoint() string {
	return h.HTTP.URL + mcp.EndpointPath
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

	server := rpc.NewServer()
	server.Authorizer = authorizer
	// Production handler registration (mirrors registerHandlers in striatumd):
	// the read surface, the mutation surface, and the repository surface, all
	// over the real pgtest runner.
	reads.Register(server, runner, reads.Options{})
	mutations.Register(server, runner, mutations.Options{})
	repositories.Service{Runner: runner}.Register(server)

	mcpHandler := mcp.NewHTTPHandler(mcp.Service{
		RPC:              server,
		Authorizer:       authorizer,
		ActivityRecorder: sessionliveness.DBRecorder{Runner: runner},
	})
	httpServer := httptest.NewServer(mcpHandler)
	t.Cleanup(httpServer.Close)

	return &Harness{
		Runner:       runner,
		Server:       server,
		MCP:          mcpHandler,
		HTTP:         httpServer,
		Token:        token,
		RepositoryID: repositoryID,
		authorizer:   authorizer,
	}
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
