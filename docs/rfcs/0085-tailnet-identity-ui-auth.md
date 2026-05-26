# RFC 0085: Tailnet-identity UI authentication (loopback-preserving)

Status: proposed
Date: 2026-05-26
Author: proposer-claude-opus-4-7-001
Context: builds on [`RFC 0084`](0084-interrogable-agent-loop-attestation-and-chat-ui.md)
D1 (the Go web service is now mounted on the daemon listener). Goal: let an
operator view the web UI (run history, interrogation chat) from another device
on their **private tailnet**, while the daemon's HTTP listener stays
**loopback-only**. Surfaced 2026-05-26 ("can I view the dialog with the
tailscale URL?").

## Problem

The daemon HTTP listener binds `127.0.0.1`, refuses non-loopback bind, rejects
non-loopback `Host`, and authenticates with a bearer token only. A browser on
another tailnet device cannot satisfy any of these: it can't reach a loopback
bind, its `Host` is the node's MagicDNS name, and it cannot set an
`Authorization` header on navigation. There is no cookie/session/identity auth.

We want **no change to the loopback bind or the bearer/MCP/CLI path**, plus a
browser-reachable, tailnet-private UI.

## Approach

`tailscale serve` runs on the daemon host and dials a local target, so the
daemon stays loopback-only — Tailscale terminates a tailnet-private HTTPS
endpoint (`https://<node>.<tailnet>.ts.net/`) and proxies to a local address.
Tailscale injects authoritative identity headers (`Tailscale-User-Login`,
`Tailscale-User-Name`) for the requesting peer and strips client-spoofed copies.
We make the daemon's web service trust that identity instead of a pasted token.

**`serve` only, never `funnel`.** Funnel exposes the service to the public
internet and would violate the local-first / no-hosted-services boundary
(AGENTS.md). This RFC authorizes tailnet-private `serve` exposure only.

## Four pieces (this is what was missing)

1. **A browser-compatible authorization mechanism (core).** A request to the
   web routes is authenticated when it carries a Tailscale identity whose login
   is in an explicit allowlist — no bearer token required. The bearer path is
   unchanged for direct loopback / MCP / CLI.
2. **Host allowlist (not bind change).** The loopback *bind* stays `127.0.0.1`.
   A configured allowed-Host (the node's MagicDNS name) is added so the web
   handler accepts the `Host` `tailscale serve` forwards. Bind ≠ Host.
3. **Authoritative identity (anti-spoofing).** Identity headers are trusted only
   on a **dedicated UI listener** that only `tailscale serve` is pointed at — a
   unix socket at `$STRIATUM_DAEMON_RUNTIME_DIR/web-ui.sock`, mode `0600`
   (owner-only). The main loopback port never trusts identity headers. (Hardening
   follow-up: verify each peer via the Tailscale LocalAPI `WhoIs` instead of
   trusting headers.)
4. **A stable UI endpoint for `serve` to target.** The daemon's HTTP port is
   ephemeral (changes per restart); `serve` needs a stable target. The unix
   socket in (3) is that stable target and avoids any port.

## Scope and defaults (conservative)

- **Off by default.** Opt-in via `--web-tailscale` (or
  `STRIATUM_DAEMON_WEB_TAILSCALE=1`). When off, nothing changes.
- **Explicit allowlist.** `STRIATUM_DAEMON_WEB_TAILSCALE_USERS` (comma-separated
  tailnet logins). Empty allowlist while enabled = **deny all** (fail closed).
- **Read-only via an explicit route allowlist (not the HTTP verb).** Design
  interrogation `intg_4b69c562…` rejected "GET means safe": the read-only
  invariant must be an **audited allowlist of permitted read routes**, not a
  `reject non-GET` check. Tailnet-identity requests are permitted only for an
  explicit set — `GET /v1/health`, `GET /v1/runs`, `GET /v1/runs/{id}`,
  `GET /v1/runs/{id}/interrogations`, `GET /v1/runs/{id}/interrogations/{id}`
  (incl. `?view=chat`) — and **everything else is denied by default**, including
  `POST /v1/invoke` (even read methods), workflow generation, and any future
  route. Mutations remain bearer + loopback. This set is enough to "view the
  dialog in the UI." A **normative route-audit test** asserts the permitted set
  is read-only and that no mutating route is reachable over the identity socket.

## Minimal landable slice

- `--web-tailscale` flag + the two env knobs; default off.
- A unix-socket UI listener (`web-ui.sock`, 0600) serving the existing web
  handler in a "tailscale identity" auth mode: authenticate via
  `Tailscale-User-Login` ∈ allowlist; permit only the **read-route allowlist**
  above (deny everything else by default); accept the configured MagicDNS `Host`.
- Allowlist normalization: unset, empty, whitespace-only, and empty-after-parse
  all resolve to an empty set ⇒ deny every identity request (fail closed).
- Documented operator command: `tailscale serve --bg unix:$RUNTIME/web-ui.sock`.
- Tests: identity-in-allowlist permitted-route GET → 200; identity-not-in-allowlist
  → 403; `POST /v1/invoke` (mutating AND read method) under identity → 403; a
  not-allowlisted GET route under identity → 403; no identity → 401; bearer path
  on the main listener unchanged; the four empty-allowlist normalizations deny.
- **Route-audit test (normative):** enumerate the identity-socket permitted set
  and assert it is exactly the read-route allowlist and that no mutating handler
  is reachable over the identity socket.

## Drawbacks / follow-ups

- Header-trust-on-a-0600-socket is a loopback trust model (any owner process
  could connect to the socket and spoof identity); the `WhoIs` LocalAPI
  verification is the stricter follow-up.
- Per-run repository resolution (TODO F38) still applies for run-scoped routes
  over the tailnet.
- No write access over the tailnet in this slice by design; a future scoped
  mutation grant would need its own decision.

## Decision

- **D143** — accept tailnet-identity UI authentication: loopback bind preserved;
  opt-in, read-only, allowlisted tailnet-identity auth on a dedicated 0600 unix
  socket fronted by `tailscale serve`; `funnel` prohibited.
