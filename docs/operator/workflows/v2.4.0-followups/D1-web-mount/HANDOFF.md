# D1 handoff — Go web service mounted in the daemon

author: operator

## What changed

The daemon's single loopback HTTP listener now serves **both** the MCP
JSON-RPC/SSE surface and the Go web service (`/v1/...`, `/run`, `/`,
`/static/...`, `/workflow-templates`, `/workflows/...`) — including the
RFC 0084 interrogation chat route. Previously `newWebServiceHandler` had no
non-test caller and `/v1/...` was httptest-only.

### `go/cmd/striatumd/main.go`

- `startMCPHTTPServer` now takes a `webServiceOptions` argument. Instead of
  setting `httpServer.Handler = mcp.NewHTTPHandler(...)` as the sole handler,
  it builds both the MCP handler and `newWebServiceHandler(rpcServer, webOpts)`
  and sets the handler to `newDaemonHTTPHandler(mcpHandler, webHandler)`.
- The daemon resolves the runtime client token (the same bearer the MCP
  listener and CLI use) after bootstrap via `readRuntimeTokenFile(tokenPath)`
  and passes `resolveWebServiceOptions(webServiceToken)` to the server. If the
  token file is unreadable the daemon logs a warning and continues (loopback
  host auth still applies); MCP behavior is unchanged.
- Same listener/port — no second port opened. Loopback-only binding
  (`listenMCPHTTP`) and bearer auth are preserved unchanged.

### `go/cmd/striatumd/web_service.go`

- Added `newDaemonHTTPHandler(mcpHandler, webHandler)`: a path multiplexer.
  Requests where the path is `/mcp` or under `/mcp/` (`/mcp/sse`,
  `/mcp/messages`) route to the MCP handler **byte-for-byte unchanged**;
  everything else routes to the web service.
- Added `resolveWebServiceOptions(token)`: wires the web service from the
  runtime token plus optional env overrides:
  - `STRIATUM_DAEMON_WEB_REPOSITORY_ID` — pin a repository scope (default
    unset = multi-repo; requests carry `repository_id`).
  - `STRIATUM_DAEMON_WEB_ALLOW_MUTATIONS` — read-only by default; mutations
    require this to be explicitly truthy.
  - `WebEnabled: true` so `/` and `/static` render.
  The runtime token is used as both the HTTP bearer gate (`ServiceToken`) and
  the downstream RPC `CapabilityToken`.

### Tests

- `go/cmd/striatumd/web_mux_test.go` — `TestDaemonHTTPHandlerRoutesMCPAndWeb`
  stands up the mux on one `httptest.Server` and asserts `POST /mcp`
  (`tools/list`) routes to the MCP handler (JSON-RPC result with a `tools`
  list) and `GET /v1/health` routes to the web service (`"mode":"go"`).

## How to verify against a live daemon

The daemon writes its HTTP endpoint to
`~/.cache/striatum/runtime/mcp-http-endpoint` (or `$XDG_RUNTIME_DIR/striatum/`,
or macOS `~/Library/Caches/striatum/runtime/`) as `http://127.0.0.1:<port>/mcp`.
The bearer token lives next to it at `client-token`.

```sh
RUNTIME=~/.cache/striatum/runtime          # adjust per platform / XDG_RUNTIME_DIR
BASE=$(sed 's#/mcp$##' "$RUNTIME/mcp-http-endpoint")
TOKEN=$(cat "$RUNTIME/client-token")

# 1. Web service health (web handler):
curl -s "$BASE/v1/health" -H "Authorization: Bearer $TOKEN" -H "Host: 127.0.0.1"
# => {"ok":true,"data":{"started_at":"...","mode":"go","allow_mutations":false}}

# 2. Interrogation chat route (RFC 0084), for a real run+interrogation:
curl -s "$BASE/v1/runs/<RUN_ID>/interrogations/<INTERROGATION_ID>?view=chat" \
  -H "Authorization: Bearer $TOKEN" -i
# => 200, Content-Type: text/html; charset=utf-8, Cache-Control: no-store

# 3. MCP unchanged (no regression):
curl -s "$BASE/mcp" -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":"l","method":"tools/list","params":{}}'
# => JSON-RPC result with the MCP tools list
```

Both surfaces refuse non-loopback `Host` headers and require the bearer token.

## Test results

```
$ cd go && go build ./... && go test ./cmd/striatumd/... ./pkg/webservice/...
ok  	github.com/halbritt/striatum/go/cmd/striatumd	0.017s
ok  	github.com/halbritt/striatum/go/pkg/webservice	0.004s
```

## Notes / follow-ups

- The web service is **read-only by default**. Enable mutations only by
  setting `STRIATUM_DAEMON_WEB_ALLOW_MUTATIONS=1` on the daemon process;
  mutation methods are still gated at the RPC layer by the token's
  capabilities (defense in depth).
- Repository scope is left unpinned so the multi-repo daemon serves all
  registered repos; routes that need scoping carry `repository_id` /`run_id`.
  Pin `STRIATUM_DAEMON_WEB_REPOSITORY_ID` if a single-repo deployment wants the
  web service to default-inject one repository.
