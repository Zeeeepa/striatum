package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/webservice"
)

type webServiceOptions struct {
	RepositoryID    string
	CapabilityToken string
	ServiceToken    string
	AllowMutations  bool
	WebEnabled      bool
}

func newWebServiceHandler(rpcServer *rpc.Server, opts webServiceOptions) http.Handler {
	return webservice.New(webservice.Config{
		RPC:             rpcServer,
		RepositoryID:    opts.RepositoryID,
		CapabilityToken: opts.CapabilityToken,
		ServiceToken:    opts.ServiceToken,
		AllowMutations:  opts.AllowMutations,
		WebEnabled:      opts.WebEnabled,
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

func envBool(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
