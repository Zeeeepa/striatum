# D1 operator notes — what live verification + interrogation surfaced

The lane built the `/mcp` vs `/v1` multiplexer (`newDaemonHTTPHandler`) and a
routing test. Three issues were then caught and fixed before landing — two by
the genuine build-review interrogation and live HTTP verification:

1. **Fail-open web auth (build-review finding).** `authenticate` treats an empty
   `ServiceToken` as "allow all"; main.go mounted the web service with the
   runtime token and only *logged* a warning when it was unreadable. Fix:
   `randomDenyToken()` substitutes an unguessable bearer when the runtime token
   is empty, so `/v1` fails closed (401) while MCP keeps its own auth. Test:
   `TestDaemonWebFailsClosedWithDenyToken`.

2. **Turns silently dropped (live verification).** `interrogation.show` returns
   `turns` as `[]map[string]any`, but the chat handler asserted `.([]any)` →
   the assertion failed live and every turn vanished ("No turns recorded"),
   even though the httptest passed (its fake used `[]any`). Fix: the handler
   accepts both types; the test fake now uses `[]map[string]any` to mirror the
   real read.

3. **Multi-repo scoping (live verification).** The daemon is multi-repo, but the
   mounted web service injects `repository_id` only from
   `STRIATUM_DAEMON_WEB_REPOSITORY_ID`. Unset → every run-scoped `/v1` route
   returns `repo_not_registered`. **Current limitation:** set that env to the
   target repository_id (a systemd drop-in does this locally). **Follow-up:**
   resolve `repository_id` per-run from the `runID` path segment so the mounted
   web service works across all registered repos without per-deploy config.

## Verified live

- `POST /mcp` `tools/list` → 102 tools (MCP unregressed).
- `GET /v1/health` → 200 with bearer, 401 without.
- `GET /v1/runs/{runID}/interrogations/{id}?view=chat` → 200 `text/html`,
  `Cache-Control: no-store`, 6 turns rendered, attacker bodies escaped via
  `html/template` (`live-chat-served.html`). Wrong-run path → 404 (IDOR guard).
