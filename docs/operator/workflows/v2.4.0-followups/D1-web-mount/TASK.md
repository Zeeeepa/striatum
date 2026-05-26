# D1 task — Mount the Go web service in the daemon

## Goal

Make the Go web service (`/v1/...`, including the interrogation chat route added
in RFC 0084) actually reachable from the running daemon. Today
`newWebServiceHandler` has no non-test caller; `startMCPHTTPServer`
(`go/cmd/striatumd/main.go:288`) mounts `mcp.NewHTTPHandler(...)` as the **sole**
HTTP handler, so the entire `/v1` surface is built and httptest-verified but not
served live.

## Exact integration point

`go/cmd/striatumd/main.go:306` — the `httpServer.Handler` is `mcp.NewHTTPHandler`.
Replace it with a multiplexer:

- Requests whose path is `/mcp`, `/mcp/sse`, or `/mcp/messages` (prefix
  `/mcp`) → the existing `mcp.NewHTTPHandler(mcp.Service{...})`.
- All other paths (`/v1/...`, `/run`, `/`, `/static/...`, `/workflow-templates`,
  `/workflows/...`) → `newWebServiceHandler(rpcServer, webServiceOptions{...})`.

`newWebServiceHandler` (`go/cmd/striatumd/web_service.go:18`) takes the
`*rpc.Server` and `webServiceOptions{RepositoryID, AllowMutations, WebEnabled,
... }`. The web service enforces its own loopback-host + bearer-token auth, so
the same token works. Pass `WebEnabled: true` so `/` and `/static` render. Wire
through the daemon's existing repository id + allow-mutations config (mirror how
those are resolved elsewhere in main.go); do not weaken auth or bind non-loopback.

## Constraints

- Same listener/port as MCP (do not open a second port unless trivially cleaner;
  prefer the mux). Loopback-only, bearer auth preserved.
- Do not regress the MCP path: `/mcp` POST/GET must behave exactly as before.
- No new product surface beyond mounting the existing handler.

## Definition of done

- `GET /v1/health` returns 200 JSON on the daemon's HTTP listener.
- `GET /v1/runs/{runID}/interrogations/{id}?view=chat` returns the chat HTML
  (200, `text/html`, `Cache-Control: no-store`) for a real interrogation.
- `POST /mcp` `tools/list` still works (no MCP regression).
- A Go test asserts the mux routes `/mcp` to the MCP handler and `/v1/health`
  to the web handler on one listener.
- `go build ./... && go test ./cmd/striatumd/... ./pkg/webservice/...` green.
