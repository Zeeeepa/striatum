---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0085 Design Defense — tailnet-identity UI auth

Thesis: this slice lets a tailnet device view the UI **without weakening the
loopback security model**. Every relaxation is additive, opt-in, read-only, and
fail-closed. Below, each claim is stated so it can be falsified by a test or a
concrete attack.

## 1. Anti-spoofing trust model (the 0600 socket)

Claim: trusting `Tailscale-User-Login` is safe **only** on a dedicated unix
socket at `$STRIATUM_DAEMON_RUNTIME_DIR/web-ui.sock`, mode `0600`, that only
`tailscale serve` is pointed at. The main loopback TCP port never reads identity
headers.

Why it holds:
- `tailscale serve` runs *on the daemon host as the daemon's user*, terminates a
  tailnet-private HTTPS endpoint, injects authoritative `Tailscale-User-*`
  headers for the verified peer, and **strips client-supplied copies**. A remote
  browser therefore cannot set its own identity header — Tailscale overwrites it.
- The socket is `0600`, owner-only. The only principals who can open it are the
  daemon user and root. A *remote* tailnet peer cannot open the socket at all;
  it can only arrive via `serve`, i.e. with Tailscale-asserted identity.

What a same-host attacker could/couldn't do (be precise):
- **Could:** a process *already running as the daemon's user (or root)* can
  `connect()` to the socket and send a forged `Tailscale-User-Login: admin@...`.
  That yields read-only UI access.
- **Couldn't:** a different local user cannot open the socket (0600). A remote
  attacker cannot reach it without Tailscale identity. No one gains write access
  (see §2). No one reaches it via the loopback TCP port (headers ignored there).
- **Falsifiable boundary:** this is *exactly the loopback trust model* — anyone
  already executing as the trusted user has already won; they could read the
  bearer token from the runtime dir just as easily. We add **no** privilege a
  same-uid process didn't already have. We deliberately do **not** claim defense
  against a compromised daemon-user account.

Why `WhoIs` is the stricter follow-up, not a blocker: the Tailscale LocalAPI
`WhoIs` lets the daemon *independently* verify the peer instead of trusting the
forwarded header, closing the same-uid header-forgery gap. It is out of scope
here because (a) it adds a LocalAPI dependency and per-request call, and (b) the
0600 socket already reduces the exposure to the pre-existing loopback trust
boundary. Shipping header-trust now is honest about its threat model; `WhoIs`
upgrades it without re-architecting.

## 2. Read-only enforcement

Claim: a tailnet-identity request can only call **GET** routes; everything else
is 403. Mutations stay bearer + loopback.

How it's guaranteed (not by convention):
- The tailscale-identity auth mode is a property of the **dedicated socket
  listener**. In that mode the handler rejects any method != GET with 403 before
  dispatch — it is a gate on the request, not a per-route opt-in, so a new POST
  route cannot accidentally become reachable.
- The main loopback listener is untouched: bearer-authenticated, all methods,
  and it never enters identity mode. Mutating `/v1/invoke` methods and workflow
  generation are only reachable there.
- **Falsifiable:** tests assert identity GET → 200, identity POST/mutation → 403,
  and that the bearer path on the main listener still serves all methods. If a
  POST under tailnet identity ever returns 2xx, the design is violated.

## 3. Fail-closed posture

Claim: the feature defaults to off and denies by default when on.
- **Off by default:** absent `--web-tailscale` / `STRIATUM_DAEMON_WEB_TAILSCALE`,
  no socket listener is created and nothing about today's behavior changes.
- **Empty allowlist while enabled = deny all:** `..._WEB_TAILSCALE_USERS` empty
  means *no login matches*, so every identity request is 403. Enabling the
  feature without naming users grants nothing — you must opt in twice (enable +
  list users).
- No identity header at all → 401. Identity present but not in allowlist → 403.
- **Falsifiable:** enabled + empty allowlist must 403 every request; this is a
  required test.

## 4. Loopback invariant (bind ≠ Host)

Claim: the TCP bind stays `127.0.0.1`. We add only (a) an allowed-`Host` entry
and (b) a separate unix socket.
- `tailscale serve` dials a **local** target (the unix socket) — it does not
  require the daemon to bind a routable interface. The daemon's TCP listener
  remains loopback-only and still refuses non-loopback binds.
- The browser's `Host` is the node's MagicDNS name. That is a *Host header*
  allowlist entry on the web handler, **not** a bind change. Accepting a Host
  string does not open a network interface. Bind is where we listen; Host is what
  we're willing to answer for — orthogonal.
- **Falsifiable:** a port scan of non-loopback interfaces shows no new daemon
  TCP port; the only new endpoint is a filesystem socket.

## 5. `serve`, never `funnel`

- `tailscale serve` exposes the endpoint **only to the operator's private
  tailnet** — authenticated devices the operator already controls. That is
  in-boundary: local-first, no hosted service, no public surface.
- `tailscale funnel` would publish the service to the **public internet**,
  directly violating the AGENTS.md no-hosted-services / local-first boundary and
  exposing identity-trusting routes to anyone. This RFC authorizes `serve` only;
  `funnel` is prohibited by D143.
- **Falsifiable:** documented operator command is `tailscale serve` against the
  unix socket; no `funnel` command is documented or supported.

## Conceded honestly
- Same-uid header forgery is possible until `WhoIs` lands (§1) — accepted because
  it equals the existing loopback trust boundary, not a new one.
- Per-run repository resolution (F38) still gates run-scoped routes over the
  tailnet; this slice does not solve it.
- No tailnet write path exists by design; granting one needs its own decision.
