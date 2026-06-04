package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/webservice"
)

type webServiceOptions struct {
	RepositoryID    string
	CapabilityToken string
	ServiceToken    string
	AllowMutations  bool
	WebEnabled      bool

	// RFC 0085 tailnet-identity UI listener (web-ui.sock). Set only by
	// resolveWebUIOptions for the dedicated identity socket; the main loopback
	// listener leaves these zero-valued and keeps its bearer auth.
	IdentityAuth      bool
	IdentityAllowlist map[string]struct{}
	AllowedHost       string
}

func newWebServiceHandler(rpcServer *rpc.Server, opts webServiceOptions) http.Handler {
	return webservice.New(webservice.Config{
		RPC:               rpcServer,
		RepositoryID:      opts.RepositoryID,
		CapabilityToken:   opts.CapabilityToken,
		ServiceToken:      opts.ServiceToken,
		AllowMutations:    opts.AllowMutations,
		WebEnabled:        opts.WebEnabled,
		IdentityAuth:      opts.IdentityAuth,
		IdentityAllowlist: opts.IdentityAllowlist,
		AllowedHost:       opts.AllowedHost,
	})
}

// newDaemonHTTPHandler multiplexes the daemon's single loopback HTTP listener
// between the MCP JSON-RPC/SSE handler and the Go web service. Requests whose
// path is /mcp or under /mcp/ (the MCP endpoint, its SSE stream, and the
// message channel) go to the MCP handler unchanged; everything else (/v1/...,
// /run, /, /static/..., /workflow-templates, /workflows/...) goes to the web
// service. The MCP path is left byte-for-byte identical so there is no MCP
// behavior change; both handlers enforce their own loopback-host + bearer auth.
func newDaemonHTTPHandler(mcpHandler http.Handler, webHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMCPPath(r.URL.Path) {
			mcpHandler.ServeHTTP(w, r)
			return
		}
		webHandler.ServeHTTP(w, r)
	})
}

func isMCPPath(path string) bool {
	return path == "/mcp" || strings.HasPrefix(path, "/mcp/")
}

// resolveWebServiceOptions wires the daemon's live web service from the
// runtime token plus optional environment overrides. The runtime client token
// (the same bearer the MCP listener and CLI use) gates HTTP access and authors
// the downstream RPC calls. The web service is read-only by default; mutations
// require STRIATUM_DAEMON_WEB_ALLOW_MUTATIONS to be explicitly enabled. The
// repository scope is left unset (multi-repo) unless pinned via
// STRIATUM_DAEMON_WEB_REPOSITORY_ID, mirroring how the daemon resolves other
// HTTP knobs (see defaultMCPHTTPAddr).
func resolveWebServiceOptions(token string) webServiceOptions {
	return webServiceOptions{
		RepositoryID:    strings.TrimSpace(os.Getenv("STRIATUM_DAEMON_WEB_REPOSITORY_ID")),
		CapabilityToken: token,
		ServiceToken:    token,
		AllowMutations:  envBool("STRIATUM_DAEMON_WEB_ALLOW_MUTATIONS"),
		WebEnabled:      true,
	}
}

// resolveWebUIOptions builds the options for the RFC 0085 tailnet-identity UI
// listener served over web-ui.sock. It authenticates by Tailscale identity (not
// bearer), is read-only by route allowlist, and authors its downstream RPC
// calls with the runtime capability token. The MagicDNS host that
// `tailscale serve` forwards is read from STRIATUM_DAEMON_WEB_TAILSCALE_HOST.
func resolveWebUIOptions(token string) webServiceOptions {
	return webServiceOptions{
		RepositoryID:      strings.TrimSpace(os.Getenv("STRIATUM_DAEMON_WEB_REPOSITORY_ID")),
		CapabilityToken:   token,
		AllowMutations:    false,
		WebEnabled:        true,
		IdentityAuth:      true,
		IdentityAllowlist: parseTailscaleUsers(os.Getenv("STRIATUM_DAEMON_WEB_TAILSCALE_USERS")),
		AllowedHost:       strings.TrimSpace(os.Getenv("STRIATUM_DAEMON_WEB_TAILSCALE_HOST")),
	}
}

// parseTailscaleUsers parses the comma-separated STRIATUM_DAEMON_WEB_TAILSCALE_USERS
// allowlist into a set of logins. Unset, empty, whitespace-only, and
// empty-after-parse (e.g. ",," or ", ,") all normalize to an empty set, which —
// while tailnet auth is enabled — denies every identity request (fail closed).
func parseTailscaleUsers(raw string) map[string]struct{} {
	users := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		login := strings.TrimSpace(part)
		if login != "" {
			users[login] = struct{}{}
		}
	}
	return users
}

// startWebUISocket starts the RFC 0085 tailnet-identity UI listener on a
// dedicated unix socket at $STRIATUM_DAEMON_RUNTIME_DIR/web-ui.sock, mode 0600
// (owner-only). It serves the web handler in identity-auth mode — the stable,
// portless target an operator points `tailscale serve --bg unix:<sock>` at.
// The daemon's loopback HTTP bind and its bearer/MCP path are untouched.
func startWebUISocket(ctx context.Context, rpcServer *rpc.Server, opts webServiceOptions) (func(), error) {
	runtimeDir, err := admin.RuntimeDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("create runtime dir %s: %w", runtimeDir, err)
	}
	sockPath := filepath.Join(runtimeDir, "web-ui.sock")
	if err := os.Remove(sockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove stale web-ui socket %s: %w", sockPath, err)
	}
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen on web-ui socket %s: %w", sockPath, err)
	}
	// Owner-only: only processes running as the daemon owner (the same trust
	// boundary as loopback) can connect, which is what makes header-trust on
	// this socket acceptable per RFC 0085.
	if err := os.Chmod(sockPath, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(sockPath)
		return nil, fmt.Errorf("chmod web-ui socket %s to 0600: %w", sockPath, err)
	}
	httpServer := &http.Server{
		Handler:           newWebServiceHandler(rpcServer, opts),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("striatumd-go tailnet-identity UI socket listening on %s (mode 0600; point `tailscale serve --bg unix:%s` here)", sockPath, sockPath)
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("web-ui socket server stopped: %v", err)
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
		if err := os.Remove(sockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove web-ui socket %s: %v", sockPath, err)
		}
	}, nil
}

func envBool(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

// envOr returns the trimmed environment value for name, or fallback when unset.
func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
