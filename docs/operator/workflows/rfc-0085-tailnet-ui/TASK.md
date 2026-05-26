# RFC 0085 task — tailnet-identity UI auth, run through interrogation

Authoritative design: `docs/rfcs/0085-tailnet-identity-ui-auth.md`. This dogfood
runs that RFC through the interrogation process at the design stage AND the
build stage.

## Design ratification (interrogable)

Read RFC 0085 and write a concise **design defense** (artifacts/DESIGN_DEFENSE.md)
that an adversarial reviewer will interrogate. It must take a position on, and be
ready to answer interrogation about:
- The anti-spoofing trust model: why a `0600` unix socket that only
  `tailscale serve` targets is acceptable for trusting `Tailscale-User-Login`,
  and exactly what an attacker on the same host could/couldn't do.
- Read-only enforcement: how GET-only is guaranteed for tailnet-identity
  requests, and that the bearer/loopback path is unchanged.
- Fail-closed posture: empty allowlist while enabled = deny all; off by default.
- The loopback invariant: the bind stays `127.0.0.1`; only the Host allowlist
  + a dedicated socket are added.
- Why `serve` (tailnet-private) is in-boundary but `funnel` (public) is not.

Stay live after completing so the design reviewer can interrogate this reasoning.

## Build (interrogable) — minimal landable slice

Implement RFC 0085's "Minimal landable slice" in `go/cmd/striatumd/` +
`go/pkg/webservice/`:
- `--web-tailscale` flag (+ `STRIATUM_DAEMON_WEB_TAILSCALE` env), default off;
  `STRIATUM_DAEMON_WEB_TAILSCALE_USERS` allowlist (empty + enabled = deny all).
- A unix-socket UI listener at `$STRIATUM_DAEMON_RUNTIME_DIR/web-ui.sock`
  (mode `0600`) serving the existing web handler in a tailscale-identity auth
  mode: authenticate when `Tailscale-User-Login` ∈ allowlist; accept the
  configured MagicDNS `Host`; no identity → 401.
- **Read-only via an explicit route allowlist, NOT the HTTP verb** (design
  interrogation `intg_4b69c562…` rejected "GET means safe"). Permit only:
  `GET /v1/health`, `GET /v1/runs`, `GET /v1/runs/{id}`,
  `GET /v1/runs/{id}/interrogations`, `GET /v1/runs/{id}/interrogations/{id}`
  (incl. `?view=chat`). Deny everything else by default — `POST /v1/invoke`
  (even read methods), workflow generation, any other/future route → 403.
- Allowlist normalization: unset / empty / whitespace-only / empty-after-parse
  → empty set → deny every identity request (fail closed).
- The main loopback listener and its bearer auth are unchanged and never trust
  identity headers.
- Go tests: allowed-identity permitted-route GET → 200; identity not in allowlist
  → 403; `POST /v1/invoke` (mutating AND read method) under identity → 403; a
  non-allowlisted GET under identity → 403; no identity → 401; bearer path on the
  main listener still works; the four empty-allowlist normalizations all deny.
- **Route-audit test (normative):** enumerate the identity-socket permitted set
  and assert it equals the read-route allowlist and that no mutating handler is
  reachable over the identity socket.

Self-verify: `cd go && go build ./... && go test ./cmd/striatumd/... ./pkg/webservice/...`.
Stay live after completing for the build-review interrogation. Write a HANDOFF.

## Out of scope (RFC follow-ups)

`WhoIs` LocalAPI verification (vs header trust), per-run repo resolution (F38),
any write access over the tailnet.
