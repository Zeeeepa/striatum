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
  mode: authenticate when `Tailscale-User-Login` ∈ allowlist; reject non-GET
  (read-only, 403); accept the configured MagicDNS `Host`; no identity → 401.
- The main loopback listener and its bearer auth are unchanged and never trust
  identity headers.
- Go tests: allowed-identity GET → 200; not-in-allowlist → 403; POST/mutation
  under identity → 403; no identity → 401; bearer path on the main listener
  still works.

Self-verify: `cd go && go build ./... && go test ./cmd/striatumd/... ./pkg/webservice/...`.
Stay live after completing for the build-review interrogation. Write a HANDOFF.

## Out of scope (RFC follow-ups)

`WhoIs` LocalAPI verification (vs header trust), per-run repo resolution (F38),
any write access over the tailnet.
