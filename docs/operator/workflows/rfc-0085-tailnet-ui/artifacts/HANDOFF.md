---
title: RFC 0085 build handoff — tailnet-identity UI auth (minimal slice)
author: operator
kind: handoff
rfc: 0085
date: 2026-05-26
---

# RFC 0085 build handoff — tailnet-identity UI auth

Implements the RFC 0085 "Minimal landable slice" (D143): an opt-in, read-only,
allowlisted tailnet-identity auth mode on a dedicated `0600` unix socket fronted
by `tailscale serve`. The daemon's loopback HTTP bind and its bearer/MCP/CLI
path are untouched.

## What changed

### `go/pkg/webservice/`
- `service.go`: added identity-mode fields to `Config`
  (`IdentityAuth`, `IdentityAllowlist`, `AllowedHost`); `ServeHTTP` branches to
  `serveIdentity` when `IdentityAuth` is set and otherwise runs the unchanged
  loopback bearer path. Refactored `hostOnly` out of `isLoopbackHost` (shared).
- `identity.go` (new):
  - `IdentityReadRoutes()` — the **normative read-route allowlist** (5 GET
    routes: `/v1/health`, `/v1/runs`, `/v1/runs/{id}`,
    `/v1/runs/{id}/interrogations`, `/v1/runs/{id}/interrogations/{id}` incl.
    `?view=chat`).
  - `PermitIdentityRoute(method, path)` — the single authority the identity
    handler consults. Read-only is enforced by **membership in this allowlist,
    not by the HTTP verb** (design interrogation `intg_4b69c562…` rejected "GET
    means safe"). Sibling GET reads (`why`, `dashboard`, `artifacts`, `events`,
    `/v1/artifacts/.../raw`, templates, `/`, `/static`) are deliberately denied.
  - `serveIdentity` — Host allowlist (loopback or configured MagicDNS) →
    `Tailscale-User-Login` present (else 401) → login ∈ allowlist (else 403) →
    route ∈ allowlist (else 403) → dispatch to the shared GET router.

### `go/cmd/striatumd/`
- `main.go`: `--web-tailscale` flag (default from `STRIATUM_DAEMON_WEB_TAILSCALE`,
  off). When enabled, starts the UI socket after the MCP HTTP server; failure to
  start is fatal (cleans up listener/pidfile/MCP first).
- `web_service.go`:
  - `parseTailscaleUsers` — normalizes unset / empty / whitespace-only /
    empty-after-parse (`",,"`, `", ,"`) all to an empty set ⇒ **deny all (fail
    closed)**.
  - `resolveWebUIOptions` — identity-auth, read-only, runtime token as the
    downstream `CapabilityToken`, no `ServiceToken`; allowlist from
    `STRIATUM_DAEMON_WEB_TAILSCALE_USERS`, MagicDNS host from
    `STRIATUM_DAEMON_WEB_TAILSCALE_HOST`.
  - `startWebUISocket` — listens on `$STRIATUM_DAEMON_RUNTIME_DIR/web-ui.sock`,
    `chmod 0600`, serves the identity handler, removes the socket on shutdown.

## Operator command (documented target)

```
tailscale serve --bg unix:$STRIATUM_DAEMON_RUNTIME_DIR/web-ui.sock
```

Then browse `https://<node>.<tailnet>.ts.net/v1/runs/<id>/interrogations/<id>?view=chat`.
`serve` only — `funnel` (public) is out of boundary per AGENTS.md / RFC 0085.

## Tests

- `go/pkg/webservice/identity_test.go`:
  - allowed identity permitted GET → 200; not-in-allowlist → 403; no identity →
    401; `POST /v1/invoke` (read method **and** mutating method) → 403 (and the
    RPC is never dispatched); non-allowlisted GET (`/v1/runs/{id}/why`) → 403;
    empty allowlist denies a permitted route.
  - **`TestIdentityRouteAuditMatchesAllowlist`** (normative route-audit):
    enumerates the full GET/POST web surface and asserts `PermitIdentityRoute`
    permits **exactly** `IdentityReadRoutes()` and denies every other route,
    including all mutating ones (POST verbs, POST/DELETE on read paths).
- `go/cmd/striatumd/web_tailscale_test.go`:
  - the four empty-allowlist normalizations all deny; parse trims + dedupes;
    `resolveWebUIOptions` is read-only identity with the runtime token;
    **main loopback listener ignores `Tailscale-User-Login` and still gates on
    bearer** (identity header → 401; bearer → 200); identity socket end-to-end:
    permitted GET → 200, `POST /v1/invoke` → 403.

## Self-verify (green)

```
cd go && go build ./... \
  && go test ./cmd/striatumd/... ./pkg/webservice/... ./pkg/webguardrails/...
ok  github.com/halbritt/striatum/go/cmd/striatumd
ok  github.com/halbritt/striatum/go/pkg/webservice
ok  github.com/halbritt/striatum/go/pkg/webguardrails
```

`go vet` clean. The `webguardrails` "no `transcript` literal in webservice .go"
guardrail passes (identity code uses "interrogation", never "transcript").

## Out of scope (RFC follow-ups)

- `WhoIs` LocalAPI peer verification instead of header trust (the `0600` socket
  is a loopback-equivalent trust model: any same-owner process could connect and
  spoof identity — acceptable for this slice, stricter follow-up noted in RFC).
- Per-run repository resolution (TODO F38) for run-scoped routes over the tailnet.
- Any write access over the tailnet (would need its own decision).
