# Claude Code Design: RFC 0028 V1 Daemon and Multi-Repository Control Plane

author: designer-claude-opus-001
date: 2026-05-11
status: design (fresh-lane round 1)
target: docs/rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md

## 0. Frame

RFC 0028 asks: should Striatum grow a long-running local control plane
whose clients are the CLI, MCP, web, and supervised agents — instead of
remaining a short-lived CLI over a per-repo SQLite file with a handful of
adapters bolted on the side?

The acceptance-criteria slice in §"Acceptance Criteria" is intentionally
narrow: register two repositories, surface a global read-only dashboard
across them, expose MCP resources spanning multiple repositories, route
read-only CLI calls through the daemon, audit every client request,
refuse mutation tools without explicit capability, run a recovery sweep
across all active runs, restart cleanly with an existing registry, and
keep direct CLI mode working unchanged. Nothing more.

The product trap RFC 0028 names — that a daemon concentrates authority
and can quietly weaken provenance guarantees — is the design's primary
constraint. RFC 0026 (lane attestation) shipped V1; RFC 0027 phase 1
(mode surfacing) shipped, and the rest is deferred until a real
containment mechanism exists. **The V1 daemon must not raise its
guarantee level above what those two RFCs already provide.** It can
host them more cleanly later; it cannot pretend they are stronger
because the daemon now exists.

This design treats the V1 slice as exactly four shippable phases
(§§ 5-8) inside RFC 0028 §8 step 1. Every later phase that RFC 0028
contemplates — supervision migration, MCP mutation, sealed-mode apply
authority, signing key custody — is explicitly deferred (§ 14).

## 1. What the V1 Daemon Cannot Prove (and Will Not Pretend To)

Before the design proper, write down the negative space. Every
acceptance test in this design exists to keep one of these honest:

1. **Model-token authorship.** D028 forbids broad transcript capture.
   The daemon may hold the supervised process's pid and may even own
   that process's lifecycle later, but it cannot prove the bytes of any
   artifact came from the model's token stream. The V1 daemon never
   describes itself as authoring evidence. Its byline contribution is
   exactly what RFC 0026 already mints: an attested supervisor binding
   produces a lane byline; an unattested CLI client publish stays
   `author: operator`. Hosting that machinery in a daemon does not
   strengthen it.

2. **Independent human decision provenance.** A `decision record`
   carries `owner: human`. The daemon has no privileged channel to
   distinguish a human typing into a CLI/web client from an LLM
   operator surrogate doing the same thing over the same socket.
   Capability tokens identify a *client*, not the entity behind it.

3. **Adversarial-local-root resistance.** An operator who can write the
   daemon registry, the per-repo `.striatum/state.sqlite3`, the daemon
   binary, or the daemon's own pidfile/socket can forge anything they
   like. The daemon is a trust boundary against good-faith mistakes and
   against unprivileged co-tenant clients on the same machine; it is
   not a defense against root.

4. **Sealed-patch enforcement.** RFC 0027 sealed mode is not in V1
   scope. The daemon must refuse to imply that registering a repo,
   running a sweep, or routing an apply call provides any sealed
   guarantee. Any V1 surface that touches `provenance_mode` reports it
   honestly; sealed runs continue to refuse to start until containment
   ships, **even when the daemon is what failed them**.

5. **Cross-repository isolation against root.** Daemon ACLs gate one
   client from another within the daemon's own surface. They do not
   stop a local user from opening another repo's `.striatum/` SQLite
   file directly. Per-repo write authority is still SPEC-bounded by the
   filesystem; the daemon adds defense in depth, not isolation.

Every external surface the daemon exposes — `daemon status`, the global
dashboard, MCP resources, audit log entries, error messages — must say
this in plain language when relevant, not bury it in this RFC.

## 2. Product Shape: What V1 Daemon Mode Actually Is

V1 daemon mode is **Step 1 of RFC 0028 §8**: a daemon registry plus a
read-only multi-repo dashboard plus a read-only multi-repo MCP surface
plus a resident recovery sweeper. It is not Step 2 (CLI client mode for
mutations), not Step 3 (recovery scheduling becomes default), not Step 4
(supervision moves into the daemon), not Step 5/6 (daemon-backed MCP
mutations). The CLI verb shape an operator types is unchanged in V1; the
daemon adds a parallel introspection lane that does not replace the
direct path.

Concretely, V1 ships:

- A new long-running process `striatumd` (Python; see §3).
- A new daemon registry stored at `~/.local/share/striatum/striatumd/`
  using SQLite, holding only daemon-owned state: registered
  repositories, capability tokens, audit log, recovery scheduler
  bookkeeping. **It does not duplicate per-repo run state.**
- A Unix-domain socket at `~/.local/share/striatum/striatumd/sock` and
  optional loopback HTTP for the web UI.
- A small set of new CLI verbs (`striatum daemon …`, `striatum repo …`,
  `striatum dashboard --all`) that talk to the socket; existing CLI
  verbs are unchanged and continue to operate directly on the per-repo
  SQLite when no daemon is running.
- A read-only MCP server that aggregates resources across registered
  repositories.
- A resident recovery sweeper that calls today's
  `striatum.recovery.run_auto_sweep` per registered run on a workflow-
  declared cadence, replacing the per-run `recovery watch` PID file
  with one daemon-owned schedule.

What the daemon does **not** do in V1:

- It does not mutate per-repo state. Mutating CLI verbs (`ack`,
  `claim-next`, `publish-artifact`, `verdict`, `complete`,
  `submit-review`, `decision record`, `recovery requeue-stale`) keep
  going through the existing CLI binary against the repo's SQLite,
  unchanged. The daemon's read-only routes use `striatum.api.invoke`
  with `--repo <registered-path>` under the hood and never broaden
  beyond today's read whitelist (§7.4).
- It does not own supervised processes. `striatum supervise start` is
  still the operator-driven RFC 0009 path. Daemon-owned supervision is
  RFC 0028 §8 step 4 and is out of scope.
- It does not expose mutation MCP tools. The MCP surface is
  `resources/list` + `resources/read` only, and the resources are
  read-only views (§7.5).
- It does not replace `striatum serve`. RFC 0012's per-repo HTTP/SSE
  service stays as is for users who only need one repo and prefer the
  existing flag set; the daemon is a different product point. They can
  coexist; §11 covers conflict avoidance.

The V1 daemon's value proposition is therefore intentionally modest:
"see all your runs at once, run recovery overnight without per-run
watcher processes, and let MCP clients ask read-only questions across
repositories." That is enough to test every load-bearing trust boundary
without expanding the runner's mutation surface.

## 3. Implementation Language and Process Shape

RFC 0028 leaves the language open. V1 stays in Python.

Reasons:

- The daemon's V1 jobs (registry CRUD, dispatching read calls into
  `striatum.api.invoke`, polling `events`, calling `run_auto_sweep`)
  are all already Python. A Go rewrite multiplies risk for the slice
  that benefits least.
- RFC 0026/0027 shipped recently; their semantics are the load-bearing
  trust boundaries. Re-implementing `session_lane_attestation`,
  `expected_author_line`, recovery policy, and the publisher's
  front-matter validator in a different language *for V1* is the
  highest-risk thing we could do.
- The protocol shape (§7) is HTTP+JSON over a Unix socket. A Go daemon
  can replace it later without breaking clients.

Process model:

- Single Python process. `asyncio` is overkill for V1 traffic; the
  existing service.py uses `ThreadingHTTPServer` and that is the V1
  template. One thread per accepted connection, a small thread pool
  for the resident recovery sweeper, a single goroutine-equivalent
  thread for SSE fan-out.
- One per-repo `sqlite3.Connection` is opened lazily on first use and
  cached in a small LRU keyed by repo id; SQLite WAL is already on
  (`db.connect`), so concurrent reads are safe. Writes (only for
  daemon-owned read paths that emit events? — none do; read-only)
  remain on the CLI side.
- The daemon's own SQLite (registry / capabilities / audit) is opened
  with `journal_mode=WAL`, `busy_timeout=5000`, owner-only file
  permissions (0600).

Distribution: the daemon is shipped as a CLI subcommand of the existing
`striatum` package (`striatum daemon start`). No separate binary, no
new install path. The daemon's working directory is
`~/.local/share/striatum/striatumd/` (XDG_DATA_HOME-aware).

## 4. Tenancy: What "Multi-Tenant" Means in V1

RFC 0028 §2 lists three candidate tenants. V1 commits to two:

- **Repository tenants.** Each registered repo is named by a stable
  daemon-side `repo_id` and points at a filesystem path. The daemon
  owns the mapping from `repo_id` → repo root; per-repo run state
  stays in `<repo_root>/.striatum/state.sqlite3`. This is **Option C**
  from RFC 0028 §6: hybrid registry + per-repo run stores. Cross-repo
  joins fan out across per-repo connections; queries that don't need
  per-repo data (capability lookup, registry list) are served from the
  central registry.

- **Client tenants.** Each connected CLI/MCP/web client identifies
  itself by an opaque capability token (§6). The token is bound to a
  set of capabilities and an optional repository scope. The token
  identifies the *connection* (and therefore the client), not the
  human or LLM behind the client.

V1 does **not** model "operator tenants" as a separate identity. There
is no OS-user mapping, no per-human ACL, no capability revocation
broadcast. The single-laptop assumption from RFC 0028 §10 holds; the
shared-workstation case is deferred. This is recorded as deferred scope
(§14) so we do not pretend the daemon enforces multi-user isolation it
does not provide.

The term "tenant" never appears in user-facing strings ("repository",
"client", "token" are clearer and don't imply hosted persistence).

## 5. The Daemon Registry (Phase 1 of V1)

The daemon's central state lives in `striatumd.sqlite3` under
`~/.local/share/striatum/striatumd/`. Schema (V1):

```text
repos(
  repo_id TEXT PRIMARY KEY,         -- "rep_<hex>"
  display_name TEXT NOT NULL,
  repo_root TEXT NOT NULL UNIQUE,   -- absolute, canonicalized
  added_at TEXT NOT NULL,
  added_by_token_id TEXT,
  state TEXT NOT NULL DEFAULT 'active',  -- 'active' | 'detached'
  last_seen_state_db_inode INTEGER,      -- detect repo moves
  last_seen_state_db_size INTEGER
)

capabilities(
  token_id TEXT PRIMARY KEY,        -- "tok_<hex>" (NOT the secret)
  token_hash TEXT NOT NULL UNIQUE,  -- HMAC-SHA256 of the secret + per-token salt
  token_salt TEXT NOT NULL,
  display_name TEXT NOT NULL,
  capability_set TEXT NOT NULL,     -- JSON array of capability strings
  repo_scope TEXT NOT NULL,         -- '*' or JSON array of repo_id
  created_at TEXT NOT NULL,
  expires_at TEXT,                  -- nullable; NULL = until-revoked
  revoked_at TEXT,
  last_used_at TEXT
)

audit(
  event_id INTEGER PRIMARY KEY AUTOINCREMENT,
  occurred_at TEXT NOT NULL,
  token_id TEXT,                    -- nullable; NULL for unauthenticated probes
  client_label TEXT,                -- "cli@v1.20.1" / "mcp@claude-desktop"
  repo_id TEXT,                     -- nullable; daemon-level calls have none
  command TEXT NOT NULL,            -- "status" / "doctor" / "dashboard"
  argv_hash TEXT NOT NULL,          -- sha256 of canonicalized argv
  authorization_result TEXT NOT NULL, -- 'allowed' | 'denied' | 'no_token'
  denial_reason TEXT
)

recovery_schedule(
  repo_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  next_sweep_at TEXT NOT NULL,
  last_sweep_at TEXT,
  last_envelope_json TEXT,
  PRIMARY KEY (repo_id, run_id)
)
```

Migrations follow the same forward-only `PRAGMA user_version` pattern
the per-repo DB uses (`migrations.py`). The registry has its own
migration registry — touching the per-repo migrations module would
couple the daemon's schema lifecycle to the runner's, which we want to
avoid.

`striatum repo add /path/to/repo` is the operator UX:

```text
$ striatum repo add ~/git/engram-codeword
[striatumd] inspecting ~/git/engram-codeword
[striatumd] found .striatum/state.sqlite3 (schema version 18)
[striatumd] 3 active runs, 1 stale lease, 0 unresolved blockers
[striatumd] register? [Y/n] y
[striatumd] registered as rep_a4f9c2 (display: engram-codeword)
[striatumd] note: existing CLI invocations against this repo continue
            to work directly. The daemon adds a global view; it does
            not move state.
```

The "existing CLI invocations continue to work" hint is load-bearing:
operators must not believe `repo add` migrates state. **It does not.**
The per-repo `.striatum/` is unchanged, so anyone with no daemon, an
older runner, or a different client can keep operating on it. If they
do, the daemon's caches go stale, but read endpoints reconcile by
re-reading the per-repo SQLite on each call (no caching of run state
itself; only registry metadata is cached).

`striatum repo list` enumerates registered repos with their last-known
state. `striatum repo remove <repo_id>` removes the registry entry and
detaches the daemon from that repo; it does not touch
`<repo_root>/.striatum/`.

## 6. Capabilities and the Local Authorization Model

The daemon must refuse mutation calls unless explicit capability is
granted. Capabilities are issued as opaque secrets bound to a
named role:

- `read` — list repos, read run state, read artifacts (text + raw),
  read events, run `doctor`, read the dashboard.
- `claim` — register sessions, ack, heartbeat, claim-next, release.
  (Not in V1; reserved keyword for forward-compat.)
- `review` — `submit-review`, `verdict`. (Not in V1.)
- `apply` — operate sealed-mode apply. (Not in V1; reserved.)
- `recovery` — invoke `recovery requeue-stale` /
  `process-reconcile` / `cancel-job` etc. (Not in V1; the resident
  sweeper is daemon-internal and uses a daemon-internal capability not
  exposed to clients.)
- `admin` — register/remove repos, mint/revoke capabilities, restart
  the daemon. **The bootstrap token (§6.1) is the only `admin`
  token until an operator mints another.**

V1 implements `read`, `recovery`, `admin` in the schema and in the
authorization gate. `claim`, `review`, `apply` are **declared in the
capability vocabulary but not exposed via any V1 endpoint** so the
audit log and refusal envelope are stable when later phases add them.
Refusal carries the missing capability name, which is also the value
operators add to a token's `capability_set` to fix it (§9).

### 6.1. Token issuance and storage

Tokens are minted locally without a hosted directory:

- **Bootstrap token.** First `striatum daemon start` mints a single
  `admin` token with `capability_set: ["admin", "read", "recovery"]`,
  `repo_scope: "*"`, no expiry. The secret is printed once on stderr,
  written to `~/.local/share/striatum/striatumd/bootstrap.token` mode
  0400, and never logged again. The first thing the operator should
  do is `striatum capability mint --capability read --repo '*' --name
  cli-default` to get a non-admin token for daily CLI use, then
  `chmod 0` the bootstrap file. Documented in the daemon onboarding
  doc; the daemon also prints the suggestion in its first-run banner.
- **Operator-minted tokens.** `striatum capability mint
  --capability <c> [--capability <c2> …] [--repo <repo_id>|*]
  [--expires-in <duration>] [--name <label>]` calls the registry,
  generates a 32-byte URL-safe secret, hashes it with HMAC-SHA256
  using a per-token random salt, stores `(token_id, token_hash,
  token_salt, capability_set, repo_scope, expires_at)`, and prints
  the secret once. The secret is never persisted in plaintext, never
  re-derivable. Operators paste it into `~/.config/striatum/clients.toml`
  or set `STRIATUM_TOKEN=<secret>`.
- **Token expiry.** Optional `expires_at` is checked at every request.
  Expired tokens look like denial with reason `token_expired`.
- **Token revocation.** `striatum capability revoke --token-id <id>`
  sets `revoked_at`. There is no broadcast — a connected client whose
  token was revoked sees its next request denied with reason
  `token_revoked` and disconnects.

Constant-time compare: the request handler computes the candidate
HMAC over `(secret, stored_salt)` for the looked-up `token_id` and
compares with `hmac.compare_digest(stored_hash, computed_hash)`. The
`token_id` itself is sent in the request header (`X-Striatum-Token-Id`)
so the lookup is O(1) against an indexed column; the secret arrives in
`Authorization: Bearer <secret>`. This avoids walking the full token
table on every request and matches RFC 0012 V1 design-review F1.

### 6.2. Repository scope

Each token's `repo_scope` is either the literal string `"*"` or a JSON
array of `repo_id` values. The dashboard, MCP resource list, and read
endpoints filter to the token's scope. A request that names a
`repo_id` outside scope is denied with reason `repo_out_of_scope`.

V1 does not support negative scopes ("everything except X"), regex
scopes, or path-based scopes. Tokens are bound to ids minted by the
registry, not to repository content.

### 6.3. Capability defaults

The bootstrap token is `admin`. Every client connection that omits a
token is denied with `no_token` (no anonymous reads). The daemon does
not auto-issue a `read` token from `admin`; the operator does that
explicitly so they understand they are loosening the surface. This is
deliberately more conservative than `striatum serve` today, which
treats Unix-socket presence as authentication.

Two operator-friendly affordances keep this from being annoying:

- The bootstrap script the daemon prints on first start contains the
  exact `mint` command for a `read` token and an export line.
- `striatum daemon status` reports "no client tokens minted yet — only
  bootstrap admin remains active" until at least one non-admin token
  exists.

## 7. Client Surfaces

### 7.1. Transport

V1 binds:

- A Unix-domain socket at `~/.local/share/striatum/striatumd/sock`
  with mode 0600. This is the default for CLI and MCP.
- An optional loopback HTTP listener (`--http 127.0.0.1:<port>`) for
  the web UI when the operator runs `striatum daemon start --web`.
  Non-loopback hosts are refused at startup with exit code 8 (matches
  service.py).

WebSockets are deferred. SSE (already proven in service.py) is the
only push channel V1 ships, and only for the global dashboard event
stream (§7.4).

### 7.2. Wire shape

HTTP+JSON over both transports. The Unix transport speaks bare HTTP/1.1
the same way `service.py`'s `_UnixHTTPServer` does — no novel framing.
Every endpoint returns `{"ok": bool, "data"|"error": ...}` to match
`api.invoke`'s envelope.

Versioning: the path prefix `/v1d/` distinguishes daemon endpoints from
RFC 0012's `/v1/` so a client (or a copy-pasted curl) cannot hit the
wrong server by accident if both happen to be running. The `d` is for
"daemon"; later phases that promote endpoints to daemon-required can
add `/v2d/`.

### 7.3. Daemon-level endpoints

```text
GET  /v1d/health                     — liveness; cheap; no auth required
GET  /v1d/version                    — daemon version, pid, uptime
GET  /v1d/capabilities/self          — describe the calling token
GET  /v1d/repos                      — list repos in token scope
POST /v1d/repos                      — register; needs admin
DELETE /v1d/repos/<repo_id>          — detach; needs admin
GET  /v1d/dashboard                  — global dashboard JSON; needs read
GET  /v1d/dashboard/events           — SSE: cross-repo event firehose; needs read
GET  /v1d/recovery/schedule          — list scheduled sweeps; needs read
POST /v1d/recovery/sweep-now         — daemon-internal only (signed by daemon)
```

`/v1d/health` is the only unauthenticated endpoint. It returns
`{"ok": true, "data": {"status": "ok", "started_at": "...",
"version": "..."}}`. It does not reveal repository ids, token counts,
or anything an unauthenticated probe should not see.

### 7.4. Per-repository read endpoints

These mirror `/v1/runs/...` from RFC 0012 but are scoped through the
daemon and routed via `api.invoke(argv, repo=registry[repo_id])` so the
existing CLI dispatcher handles all read semantics:

```text
GET  /v1d/repos/<repo_id>/status                — striatum status
GET  /v1d/repos/<repo_id>/doctor                — striatum doctor
GET  /v1d/repos/<repo_id>/runs                  — list runs
GET  /v1d/repos/<repo_id>/runs/<run_id>         — single run snapshot
GET  /v1d/repos/<repo_id>/runs/<run_id>/why?id= — striatum why
GET  /v1d/repos/<repo_id>/runs/<run_id>/dashboard
GET  /v1d/repos/<repo_id>/runs/<run_id>/events  — SSE per-repo
GET  /v1d/repos/<repo_id>/artifacts/<aid>/raw   — artifact bytes (read)
```

The router's mutation gate is **stricter than `striatum serve` today**:
`/v1d/` exposes no `/invoke` catch-all. Every endpoint is either an
explicitly read-shape route or it is absent. There is no V1 path for
"daemon, run this argv for me." Mutating verbs continue to require the
operator to invoke the local CLI directly against the repo (or to use
`striatum serve --allow-mutations` with the per-repo HTTP service if
they want web-mediated mutation).

The "is this verb a read?" check that `service.py` already has
(`is_read_command`) is the model. V1's daemon imports the same
allowlist module and refuses anything not in it with HTTP 405 and
reason `not_a_read_endpoint`. We **do not** widen the allowlist for
the daemon.

### 7.5. MCP surface

A new module `src/striatum/daemon_mcp.py` wraps the Unix socket as an
MCP resource server. It is **distinct from** `src/striatum/mcp.py`,
which remains the per-repo stdio adapter for clients that want
single-repo access without daemon involvement.

V1 MCP exposes:

- `resources/list`:
  - `striatum://daemon/repos`
  - `striatum://daemon/dashboard`
  - `striatum://repo/<repo_id>/status`
  - `striatum://repo/<repo_id>/doctor`
  - `striatum://repo/<repo_id>/runs`
  - `striatum://repo/<repo_id>/run/<run_id>`
  - `striatum://repo/<repo_id>/run/<run_id>/why?id=<id>`
  - `striatum://repo/<repo_id>/blockers`
  - `striatum://repo/<repo_id>/stale-leases`
  - `striatum://daemon/audit?since=<event_id>` (admin only)
- `resources/read`: returns the JSON body the corresponding HTTP GET
  would return.
- `tools/list`: **empty in V1.** No mutation tools, no read tools.
  Resources cover the read surface; adding tools at all would make
  `tools/call` look like a mutation surface. When V1 ships the
  `mcp_call_count_v1d_tools_call` metric is hard-coded to 0; an
  operator wanting an MCP `repo_add` tool installs `striatum`
  upstream-of-daemon and uses the local MCP wrapper directly until
  RFC 0028 §8 step 5 arrives.
- `prompts/list`: same operator-boundary, claim-loop, reviewer-posture
  prompts the existing wrapper exposes, scoped to the active
  repository when the request includes one. These are read-only docs;
  exposing them does not widen the mutation surface.

This explicitly answers the prompt's question about MCP authorization
defaults: **read-only is the only V1 default, and there is no mutation
promotion path in V1**. The promotion path is RFC 0028 §8 step 5,
which is deferred (§14). Today's `striatum serve --allow-mutations`
flag does not migrate to the daemon; the daemon never had it. If a
client needs the today-style mutation surface, they continue to use
`striatum serve --allow-mutations` per-repo.

The MCP token model: an MCP client connects to the socket the same way
a CLI client does, presenting `X-Striatum-Token-Id` /
`Authorization: Bearer` over a small JSON-RPC `initialize` envelope.
Without a token the connection is dropped after sending an MCP
`initialize` failure with `code: -32001` and message `no_token`.

### 7.6. CLI client mode

`striatum --daemon ...` (or env `STRIATUM_DAEMON=1`) routes the
existing read verbs through the daemon. The verb set V1 exposes via
the daemon:

- `striatum --daemon dashboard --all`
- `striatum --daemon repo list`
- `striatum --daemon repo add <path>` (admin)
- `striatum --daemon doctor [--repo <id>|--all]`
- `striatum --daemon status [--repo <id>|--all]`
- `striatum --daemon why --repo <id> --id <art>`

Without `--daemon` (or with no daemon running), every existing CLI
verb works exactly as today. The daemon never silently intercepts CLI
calls: explicit opt-in is required so operators always know which path
they are on. This is the answer to RFC 0028 §8 "direct CLI mode still
works": V1 makes daemon-routed mode opt-in per-call so direct mode is
literally the same code path it was before V1.

`STRIATUM_DAEMON_SOCKET=/path/to/sock` overrides the default; useful
for tests and for non-XDG layouts.

## 8. Resident Recovery Sweeper

RFC 0028 §4 lists "periodic stale-lease sweeps across all active runs"
as one of the daemon's jobs. V1 ships this minimum:

- The daemon enumerates active runs across all registered repos at
  startup and every time `repo add` / `repo remove` fires.
- For each `(repo_id, run_id)` it inserts/updates `recovery_schedule`
  with `next_sweep_at = now + interval`. Interval is per-repo: read
  the workflow snapshot's `recovery_policy` block via the per-repo
  SQLite; absent block uses the daemon default (60s for the resident
  sweeper, matching `recovery watch` today).
- A single daemon-owned background thread polls the schedule and
  invokes `striatum.recovery.run_auto_sweep(...)` against the per-repo
  connection. The sweep itself is unchanged; the daemon contributes
  scheduling and the bookkeeping of last-envelope-per-run.
- Each sweep emits a `daemon.recovery.swept` row in the audit log
  with `repo_id`, `run_id`, action counts, and a digest of the
  envelope. The full envelope is held in `recovery_schedule.last_envelope_json`
  and exposed via `GET /v1d/recovery/schedule`.

This **replaces per-run `recovery watch` PID files** for daemon users.
Operators running `recovery watch --run-id <id>` against a repo also
registered with the daemon get a doctor warning ("daemon recovery
schedule already manages this run") but the watcher is not refused;
two sweepers running against the same run are safe because the
underlying CLI verbs are idempotent. Doctor calling this out is
defense in depth so operators do not unknowingly double-schedule.

The sweeper does **not** auto-cancel jobs, does **not** auto-resolve
checkpoints, and does **not** mutate per-repo state outside what
`run_auto_sweep` already does (which is the same set of mutations the
operator-driven `recovery auto` does, with the same D036 boundaries
intact).

The escalation hook surface from RFC 0020 remains workflow-declared:
the daemon's sweeper invokes the same hook code path. It does not
broaden the hook vocabulary; if a workflow has no hook declared, the
daemon emits `daemon.recovery.escalation_eligible` to its audit log
(visible via `striatum daemon audit`) and does nothing else.

## 9. Error Model

Every refusal carries a structured envelope so clients can branch on
machine-readable reasons. The HTTP and MCP envelopes share a vocab:

```json
{
  "ok": false,
  "error": {
    "code": "capability_denied",
    "message": "this client does not have the 'read' capability",
    "missing_capability": "read",
    "token_id": "tok_a4f9c2",
    "request_id": "req_91f2"
  }
}
```

V1 codes:

- `no_token` (HTTP 401 / MCP -32001)
- `token_revoked` (401 / -32001)
- `token_expired` (401 / -32001)
- `capability_denied` (403 / -32002, includes `missing_capability`)
- `repo_not_registered` (404 / -32003)
- `repo_out_of_scope` (403 / -32002)
- `not_a_read_endpoint` (405 / -32004) — V1 mutation refusal; the
  body says "the daemon does not expose this verb in V1; use the
  direct CLI against `<repo_root>`"
- `version_skew` (409 / -32005) — daemon API version older than the
  client expects, or vice versa; carries the daemon's version and
  the client's reported version
- `repo_state_db_unreachable` (502 / -32006) — the per-repo SQLite is
  missing or schema-newer than the daemon's runner; carries the
  underlying file path and the missing-or-conflicting version
- `daemon_internal` (500 / -32603)

`request_id` is generated per-request and copied to the audit log so
operators can grep `daemon audit | grep req_91f2` and find the
authorization decision.

The CLI client (`striatum --daemon ...`) maps these to existing CLI
exit codes where possible (`capability_denied` → 5,
`repo_not_registered` → 2 with a clear message, `version_skew` → 9,
`repo_state_db_unreachable` → 9). Operators get the same exit-code
semantics they're used to in direct CLI mode.

## 10. Audit Log

Every client request, including denials, becomes one row in
`audit`. The audit table is the V1 implementation of RFC 0028's
"daemon records every client request with client id, repository id,
command, authorization result, and timestamp."

What goes in:

- `occurred_at` (RFC3339, UTC)
- `token_id` (or NULL for `no_token` rejections)
- `client_label` (HTTP `User-Agent` for HTTP clients;
  `clientInfo.name@version` for MCP)
- `repo_id` (NULL for daemon-level commands)
- `command` (canonical verb name, e.g. `dashboard`, `status`, `why`,
  `repo:add`)
- `argv_hash` (canonicalized argv hashed; full argv is **not**
  recorded — hashing avoids accidentally persisting any path or run-id
  the client typed, which is consistent with D028's no-broad-capture
  posture; `striatum daemon audit --verbose` can re-derive a
  human-readable summary from the canonical command + repo + result
  without storing the original argv)
- `authorization_result`: `allowed` | `denied` | `no_token`
- `denial_reason` (the error code; NULL on allow)

What stays out:

- Request bodies, response bodies, exception tracebacks.
- Token secrets, token hashes, token salts.
- Repository contents, artifact bodies, work packets.
- Per-event SSE payloads.

`striatum daemon audit [--since <id>] [--token <id>] [--repo <id>]
[--verbose]` is read-only and requires `admin`. The audit log is
append-only and rotated when it crosses 500 MB by renaming to
`audit-<date>.sqlite3` and starting a fresh table; no record is ever
deleted from a live audit table by the daemon.

This is intentionally the *only* persistent log the daemon keeps.
There is no stdout transcript, no per-request log file, no SSE
recording. D028 stays intact.

## 11. Compatibility With `striatum serve` and `python -m striatum.mcp`

Three local server processes can in principle exist for the same repo:

1. The new `striatumd` Unix socket.
2. RFC 0012's `striatum serve` per-repo HTTP/Unix.
3. RFC 0009-derived ad-hoc tooling.
4. The per-repo `python -m striatum.mcp` stdio wrapper.

V1 keeps them coexisting:

- `striatum serve` and `striatumd` use distinct paths and ports (the
  daemon's default socket is under `~/.local/share/striatum/`; serve's
  is wherever the operator passed `--unix`). They do not stomp on
  each other's PID files.
- `python -m striatum.mcp` continues to work per-repo; clients that
  want only one repo and don't want to mint a daemon token can use
  the existing wrapper unchanged.
- `striatum daemon doctor` reports if it detects a `striatum serve`
  on the same repo with `--allow-mutations`, since that flag has the
  same authority as a future daemon `claim`/`review`/`apply`
  capability would. Not a refusal — the operator chose it — just a
  visibility line.

V1 does not deprecate any of these surfaces. Deprecation belongs to
RFC 0028 §8 step 7 ("decide whether direct SQLite CLI mode remains a
supported fallback or becomes a compatibility mode") which is out of
scope.

## 12. Interplay with RFC 0026 (Lane Attestation)

RFC 0026 V1 derives lane attestation from `process_supervisors`: a
session is attested iff there is an `attached`-state supervisor row
whose pid is alive *and* whose `pid_start_time` matches the recorded
token *and* whose command matches the snapshot. Today the supervisor
is started by an operator running `striatum supervise start`.

The V1 daemon does not own supervised processes. Therefore:

- A CLI client connecting to the daemon and calling `register_session`
  through any future mutation path would still produce an unattested
  session. The daemon does not magically attest sessions just by
  hosting the call. (V1 does not expose `register_session` through
  the daemon at all, so this is hypothetical, but the semantics must
  be designed in now so step 5 doesn't accidentally claim attestation
  on behalf of a daemon-routed CLI.)
- A supervised session that is attested today (operator ran
  `supervise start` against the per-repo state) remains attested
  whether or not the daemon is registered against that repo. The
  daemon's per-repo `connect()` reads the same `process_supervisors`
  rows the operator-driven CLI does; `session_lane_attestation`
  returns the same answer.
- The daemon's read surfaces that render bylines (`/v1d/repos/<id>/status`
  with verdict bylines, `/v1d/repos/<id>/runs/<rid>/dashboard`,
  artifact viewer in the web UI) call `artifact_author_identity`
  through the existing CLI dispatcher; the daemon never reconstructs
  bylines from `sessions.lane_id` directly. This invariant is checked
  by a unit test that mocks the per-repo connection and verifies the
  daemon serializes back the byline computed by `identity.py`, not
  one it built itself.

When RFC 0028 §8 step 4 moves supervision into the daemon, the daemon
becomes the spawner of `lanes.<lane_id>.command`, populates
`process_supervisors`, and updates the recorded pid/start-time/command
the same way the existing `supervise.py` does. **Attestation is still
derived from the same row shape**, so RFC 0026's invariant
("attestation = a runner-spawned process from this lane's command is
alive on the recorded pid for this session") holds. The daemon does
not invent an "I attest because I am the daemon" path. Daemon-level
identity (the daemon spawned it under capability X) is recorded in the
audit log; it does not fold into the session's attestation derivation.

Downgrade behavior: when a CLI client connects to the daemon without
holding a supervised session, every read surface that names a session
the client is operating from labels its bylines exactly as RFC 0026
would: `author: operator` for unattested, lane-typed for attested. The
daemon never sees an unattested register call in V1, so this section
is forward-looking design, but the conventions need to be locked in:

- **Daemon never widens identity.** A CLI client connecting through
  the daemon does not gain attestation. It does not lose attestation
  either: if the same operator separately runs `supervise start`
  against the per-repo state, the supervisor row exists and the
  daemon-routed read calls reflect attestation when they render
  bylines.
- **Capability tokens are not attestation.** A token labeled
  `name: claude-opus-driver` in the registry says nothing about
  whether any supervised process is alive. The daemon must never
  display the token name in an artifact author position. Token
  display goes in the audit log only.

## 13. Interplay with RFC 0027 (Sealed Patch Provenance)

RFC 0027 has shipped only phase 1 (mode surfacing). The V1 daemon
inherits that posture verbatim:

- `provenance_mode` is rendered honestly in `/v1d/repos/<id>/status`
  and in the global dashboard ("repo X: 3 runs in advisory, 1 run in
  attested_bylines"). The daemon does not relabel modes.
- Sealed-mode runs continue to refuse to start. The daemon does not
  paper over the unsupported-platform error; the per-repo CLI returns
  the same exit code 9 and the daemon surfaces it as
  `repo_state_db_unreachable` if the run ID was claimed by the daemon
  at registration time but the per-repo CLI now refuses to operate
  on it.
- Patch artifacts (when phase 3 ships) are read by the daemon the
  same way it reads any other artifact: through `striatum.api.invoke`
  against the per-repo state. No daemon-side patch validation, no
  daemon-side digest computation, no daemon-side apply gate.

The interesting forward-looking question is what RFC 0027 surfaces
might **eventually** want to live in the daemon. The candidates that
make sense to migrate inward are those whose authority the operator
should not hold:

- **Signing key custody.** Phase 4-5 of RFC 0027 introduces an
  Ed25519 runner key. The daemon is the natural home for that key:
  it stores it under `~/.local/share/striatum/striatumd/keys/` mode
  0600, never exposes it over any endpoint, and the apply gate
  becomes a daemon-only capability (`apply`). This is exactly the
  promotion path RFC 0028 §9 contemplates.
- **Apply service.** When sealed mode ships, the only process
  authorized to write to the protected tree is the apply service.
  Hosting that in the daemon (with `apply` capability) makes the
  write-authority story enforceable by Unix permissions: only the
  daemon process holds the write-side mount/ACL.
- **Containment supervision.** When per-job lane scratch is
  containment-bounded (bwrap, separate Unix users, etc.), the daemon
  spawns the lane process inside the container and the operator
  cannot. RFC 0028 §8 step 4 (supervision migration) is a hard
  prerequisite for this and it is not in V1.

What V1 must **not** do:

- The daemon must not advertise "sealed" anywhere. Capability strings
  reserve `apply` so step 5's daemon can refuse mutations from non-`apply`
  clients, but no V1 endpoint accepts `apply` and no V1 surface
  describes any operation as sealed.
- The daemon must not store keys. V1 has no signing material on disk.
  A `keys/` directory does not exist; `striatum keys init` does not
  call the daemon.
- The daemon must not own the apply gate. There is no `/v1d/apply`,
  no `/v1d/keys`, no `/v1d/receipts/issue`. Read-only receipt and
  patch-artifact viewing is fine because it goes through the same
  per-repo CLI; emission is not.
- The daemon must not implicitly upgrade `provenance_mode`. Reading
  an `advisory` run through the daemon does not mean it is anything
  more than `advisory`. Receipt verification, when it ships, runs in
  the per-repo CLI for V1 and stays there until containment ships.

This is the answer to the prompt's question "which V1 capabilities
must avoid implying sealed guarantees the daemon does not yet
provide": **all of them.** V1 ships `read`, the daemon-internal
sweeper, and `admin`. None imply patch provenance, none imply sealed
authority, none imply containment.

## 14. Phased Delivery Inside V1

The V1 slice itself has four shippable steps. Each is independently
useful and independently reversible.

### Phase 1 — Daemon process and registry

- New `striatum daemon start|stop|status|reload` CLI verbs in
  `src/striatum/cli/daemon.py`.
- New `src/striatum/daemon/__init__.py`, `src/striatum/daemon/server.py`
  (the threaded HTTP/Unix server, modeled on `service.py`), and
  `src/striatum/daemon/registry.py` (the central SQLite registry
  schema and CRUD).
- New `src/striatum/daemon/auth.py` (token mint/verify/revoke,
  capability matrix).
- New `striatum repo add|list|remove` and `striatum capability
  mint|list|revoke` CLI verbs in `src/striatum/cli/daemon.py`.
- Bootstrap token printed once on first start.
- Audit table populated for every request including health and
  capabilities/self.

Milestone: `striatum daemon start` boots, `striatum capability mint
--capability read --repo '*'` issues a token, `curl --unix-socket
.../sock -H "X-Striatum-Token-Id: tok_…" -H "Authorization: Bearer
…" http://localhost/v1d/repos` returns the registered list, and
`striatum daemon audit --verbose` shows the curl call.

### Phase 2 — Read-only multi-repo dashboard

- `src/striatum/daemon/dashboard.py` produces the global dashboard
  JSON: per-repo claimable jobs, blockers, stale leases, running
  sessions, recent verdicts.
- `/v1d/dashboard`, `/v1d/dashboard/events` SSE.
- `striatum --daemon dashboard --all` CLI.
- `striatum --daemon doctor --all` aggregates per-repo doctor checks.
- `striatum --daemon status [--repo|--all]` proxies to per-repo
  `status` via `api.invoke`.
- Web UI gets a `/daemon/` mount: the existing SPA learns a
  global-mode flag that, when active, talks to `/v1d/` instead of
  `/v1/` and renders the global dashboard above the per-repo view.

Milestone: a single browser tab opened on `daemon serve --web` shows
all registered repos' runs, blockers, and doctor warnings, refreshing
live via SSE.

### Phase 3 — MCP read surface

- `src/striatum/daemon/mcp.py` exposes the resource list from §7.5
  over the daemon Unix socket using the same JSON-RPC framing
  `mcp.py` uses today.
- `tools/list` returns empty.
- `prompts/list` returns the existing operator-boundary prompts,
  scoped to the active repo when a request includes one.
- An MCP client connecting without a token sees an `initialize`
  failure.

Milestone: a Claude Desktop-style client points at the daemon socket
via stdio bridge and lists `striatum://daemon/repos`,
`striatum://repo/<id>/status`, etc. Reads return the same JSON the
HTTP endpoints would.

### Phase 4 — Resident recovery sweeper

- `src/striatum/daemon/recovery.py` enumerates active runs from each
  registered repo's SQLite, schedules sweeps per the workflow's
  `recovery_policy`, and calls `striatum.recovery.run_auto_sweep`.
- `recovery_schedule` table populated.
- `/v1d/recovery/schedule` exposes the schedule + last envelope.
- `striatum daemon doctor` warns when a running `recovery watch`
  PID file exists for a registered repo.

Milestone: an operator with two registered repos shuts down all
`recovery watch` processes, leaves the daemon running overnight, and
finds in the morning that stale review-only leases were requeued and
human-checkpoint escalations fired according to each workflow's
`recovery_policy`.

## 15. Touch Points in `src/striatum/`

Concrete file-by-file enumeration of what V1 changes:

- `src/striatum/api.py` — **unchanged.** The daemon imports and uses
  `invoke()` as today. Critical that this stays the chokepoint so
  daemon-routed reads have identical semantics to direct-CLI reads.
- `src/striatum/service.py` — **unchanged behavior;** small
  refactor to extract the `is_read_command` allowlist into
  `src/striatum/service_verbs.py` so the daemon imports the same
  list. No new flags.
- `src/striatum/mcp.py` — **unchanged.** The per-repo stdio wrapper
  remains a separate adapter. The new daemon MCP module does not
  import this one; both happen to share JSON-RPC framing helpers
  which can be hoisted into `src/striatum/mcp_framing.py` as part of
  Phase 3.
- `src/striatum/supervisor.py` — **unchanged.** Supervision is
  operator-driven in V1.
- `src/striatum/recovery/__init__.py` — exports `run_auto_sweep`
  unchanged. The daemon's resident sweeper imports it directly.
  `run_watch` and `pidfile_path` are unchanged.
- `src/striatum/recovery/auto.py` — **unchanged.**
- `src/striatum/web/__init__.py` — gains a `daemon_mode` flag that
  the SPA reads via a new `/v1d/health` probe; the static assets
  learn one extra view (the global dashboard). Per-repo views are
  unchanged.
- `src/striatum/dashboard.py` — gains a `multi_repo_dashboard(...)`
  helper that aggregates the existing per-repo dashboard data; this
  is pure read code, no schema change. The TUI dashboard
  (`striatum dashboard --run-id`) is unchanged.
- `src/striatum/cli/__init__.py` — registers two new verb groups
  (`daemon`, `repo`, `capability`).
- `src/striatum/cli/daemon.py` — **new.** Thin wrappers that talk
  to the daemon socket; for `daemon start` they spawn the daemon
  process.
- `src/striatum/cli/parser.py` — adds the new verb group definitions
  and a global `--daemon` flag.
- `src/striatum/cli/dispatch.py` — adds dispatching for the new
  verb group; existing verbs are unchanged.
- `src/striatum/db.py` — **unchanged.** The daemon never opens
  per-repo connections directly; it always goes through `api.invoke`,
  so per-repo schema changes (RFC 0026/0027 etc.) reach the daemon
  for free.
- `src/striatum/migrations.py` — **unchanged.** Daemon registry
  migrations live in a new `src/striatum/daemon/migrations.py` so
  they have an independent `user_version` lineage. Per-repo and
  daemon DBs do not share schema.

New files (V1):

- `src/striatum/daemon/__init__.py`
- `src/striatum/daemon/server.py`
- `src/striatum/daemon/registry.py`
- `src/striatum/daemon/auth.py`
- `src/striatum/daemon/migrations.py`
- `src/striatum/daemon/recovery.py`
- `src/striatum/daemon/dashboard.py`
- `src/striatum/daemon/mcp.py`
- `src/striatum/daemon/audit.py`
- `src/striatum/cli/daemon.py`
- `src/striatum/service_verbs.py` (extracted shared allowlist)

## 16. State Machines and Trust Boundaries

### Connection lifecycle

```
client TCP/Unix accept
  └─ HTTP request arrives
       ├─ no token? → 401 no_token (audit row, NULL token_id)
       ├─ token unknown? → 401 no_token (audit row, NULL token_id)
       ├─ token revoked? → 401 token_revoked
       ├─ token expired? → 401 token_expired
       ├─ capability missing? → 403 capability_denied
       ├─ repo out of scope? → 403 repo_out_of_scope
       ├─ verb not in V1 read allowlist? → 405 not_a_read_endpoint
       ├─ repo state DB version mismatch? → 502 repo_state_db_unreachable
       ├─ api.invoke raises StriatumError? → 4xx/5xx with that error
       └─ ok → 200 with envelope (audit row, allowed)
```

Every branch ends in an audit row. The audit row is written **before**
the response is sent; if the audit insert fails, the daemon serves a
500 daemon_internal and refuses the original request, because an
unaudited request is worse than a refused one.

### Daemon process lifecycle

```
striatum daemon start
  ├─ check pidfile (~/.local/share/striatum/striatumd/pid)
  │    ├─ exists, pid alive? → exit 7 (single-instance)
  │    └─ stale → overwrite
  ├─ open registry SQLite, run migrations
  ├─ if registry has zero tokens: mint bootstrap admin, print secret
  ├─ enumerate registered repos, probe each .striatum/state.sqlite3
  │    ├─ missing? → mark repo state=detached, log audit row
  │    ├─ schema newer than this runner supports? → mark detached
  │    └─ ok → seed recovery_schedule for active runs
  ├─ start HTTP/Unix listener
  ├─ start recovery sweeper thread
  └─ wait for SIGTERM/SIGINT
       └─ on signal: stop accepting, drain in-flight, close, remove pidfile
```

`striatum daemon reload` rereads the registry without dropping live
connections; useful after `repo add` from a separate shell so the
sweeper picks up the new repo. (Otherwise the sweeper sees it next
poll cycle anyway; reload is just a faster path.)

### Restart consistency

The acceptance criterion "tests cover daemon restart with a
pre-existing registry and at least one registered repo-local state
store" maps to a startup-restore step:

- Read every `repos` row.
- For each, call `striatum.api.invoke(["status", "--json"], repo=…)`
  and seed `recovery_schedule` from the response.
- Detached repos (state DB missing or schema-newer) are skipped with
  one audit row each; the daemon does not refuse to start because of
  one missing repo.

A daemon that crashes mid-sweep restarts with `recovery_schedule.last_sweep_at`
intact and reschedules from there. No work is lost because the
underlying `run_auto_sweep` already records its own audit/event trail
in the per-repo `events` table; the daemon's bookkeeping is only for
"when do I sweep next," not for "what did I do."

## 17. Migrating Existing `.striatum/state.sqlite3` Repos

`striatum repo add <path>` against a repository that already has live
runs is the common case. The daemon must **not** rewrite per-repo
state. The migration is purely additive:

1. The daemon canonicalizes the path (`Path.resolve(strict=True)`).
2. It opens a read-only `sqlite3.connect(...)` to
   `<path>/.striatum/state.sqlite3` and reads `PRAGMA user_version`.
3. If the version is unknown / newer than this daemon supports, the
   add is refused with `version_skew` and a clear message naming the
   per-repo schema version and the daemon's supported maximum. The
   per-repo state is not touched.
4. If the version is older, the daemon does **not** auto-migrate.
   Auto-migration happens lazily on the first per-repo `connect()`
   from `db.py` (existing behavior). The operator gets a one-line
   stderr hint: "this repo's state DB is at vN; the runner supports
   vM. The next CLI call against this repo will migrate it. To
   migrate now, run `striatum status --repo <path>`."
5. If the version matches, the daemon inserts a `repos` row and
   responds with `repo_id`.

For repos that already had `recovery watch` running, the daemon does
not steal the lock. Both can coexist; the resident sweeper is
idempotent. The operator can stop the watcher when convenient.

Backwards path: `striatum repo remove <repo_id>` removes the registry
row, removes the recovery schedule, and writes one audit row. It does
not touch `<path>/.striatum/`. A removed repo can be re-added with
`striatum repo add <path>` and gets a fresh `repo_id`.

## 18. Test Plan

Tests live under `tests/test_daemon_*.py`. The matrix below covers
the V1 acceptance criteria:

### Capability default-deny

- `tests/test_daemon_auth.py`
  - Bootstrap token mints on first start, exists exactly once,
    `bootstrap.token` file is mode 0400.
  - HTTP request with no token returns 401 with code `no_token` and
    audit row (token_id NULL).
  - Token with capability `["read"]` calling `/v1d/repos` succeeds;
    same token calling `POST /v1d/repos` returns 403 with
    `missing_capability: admin`.
  - Repo-out-of-scope: token with `repo_scope: ["rep_a"]` requesting
    `/v1d/repos/rep_b/status` returns 403 with `repo_out_of_scope`.
  - Token revocation: `revoke` then next request returns 401
    `token_revoked`.
  - Token expiry: insert `expires_at` in the past; next request
    returns 401 `token_expired`.
  - Constant-time compare: timing attack on token_hash is bounded
    (smoke test, not a real timing assertion).

### Audit log completeness

- `tests/test_daemon_audit.py`
  - Every refusal path above produces an audit row.
  - Every success produces an audit row.
  - Audit row never contains the secret bearer token, the token_hash,
    or the request body.
  - `argv_hash` is stable for identical arguments and differs for
    different ones.
  - `daemon audit --verbose` is admin-only (denied for `read`-only).

### Daemon restart

- `tests/test_daemon_restart.py`
  - Start daemon, register two repos (one with active runs), stop
    the daemon, start it again. The new daemon's `repos` table has
    both, `recovery_schedule` is re-seeded for the run-bearing repo,
    and the sweeper begins ticking.
  - Restart with a registered repo whose state DB has been moved or
    deleted. The repo is marked `detached`, an audit row is written,
    and the daemon starts cleanly.
  - Restart with a registered repo whose state DB schema is newer
    than the daemon supports. Repo is marked `detached`, audit row
    has reason `state_db_too_new`, daemon starts.
  - Restart with the bootstrap token already minted. No new
    bootstrap token is issued.

### Multi-repo dashboard

- `tests/test_daemon_dashboard.py`
  - Two registered repos, each with one active run, return one row
    each in `/v1d/dashboard.repos[]`.
  - Per-repo claimable jobs, stale leases, and unresolved blockers
    aggregate correctly.
  - Token with `repo_scope: ["rep_a"]` only sees `rep_a` in the
    dashboard.
  - SSE stream emits one `repo:rep_a` event when an event is
    inserted into `rep_a`'s per-repo `events` table; same for
    `rep_b`. No cross-repo leakage.

### MCP read-only resources across repositories

- `tests/test_daemon_mcp.py`
  - `resources/list` returns the V1 resource set scoped to the
    token's repos.
  - `resources/read` for `striatum://repo/<id>/status` returns the
    same JSON the HTTP endpoint returns.
  - `tools/list` returns an empty array. `tools/call` of any name
    returns `method_not_found`.
  - MCP `initialize` without a token fails with `-32001 no_token`.
  - Admin-only `striatum://daemon/audit` resource is denied to
    read-only tokens.

### Supervised-agent re-attach

- `tests/test_daemon_attestation_passthrough.py`
  - Per-repo session is registered via the existing CLI; supervisor
    is started; daemon-routed `/v1d/repos/<id>/status` shows the
    session as attested.
  - Kill the supervised process; daemon-routed read shows the
    session as unattested (pid_alive false), exactly matching what
    the per-repo CLI would say.
  - Re-attach via `supervise start` on the per-repo CLI; daemon-
    routed read flips back to attested.
  - The daemon never reconstructs an attested byline by itself: a
    test that fakes `process_supervisors` rows on the per-repo state
    DB while the daemon's connection is open verifies the daemon
    re-reads on each call (or uses a connection that sees committed
    rows; SQLite WAL semantics make this trivial as long as the
    daemon doesn't cache).

### Direct CLI mode regression

- `tests/test_daemon_direct_cli_unaffected.py`
  - With the daemon NOT running: every existing CLI verb behaves
    byte-for-byte as today. (Pin a subset of representative
    `--json` outputs as fixtures.)
  - With the daemon running but the CLI invoked WITHOUT `--daemon`:
    same as above. The daemon's existence does not change direct
    CLI behavior.
  - `STRIATUM_DAEMON=1 striatum status` against a repo that is NOT
    registered with the running daemon returns
    `repo_not_registered` with a hint to run `striatum repo add`.

### Resident recovery sweeper

- `tests/test_daemon_recovery.py`
  - Registered repo with one stale review-only lease: after one
    sweep cycle, the lease is requeued, audit row written, last
    envelope stored.
  - Workflow with `recovery_policy.checkpoint_timeout_seconds: 0`:
    open `human_checkpoint` blocker triggers escalation hook each
    sweep until resolved.
  - Sweep against a repo whose state DB went missing returns the
    repo to `detached` and stops scheduling sweeps for it; audit
    row written.
  - Repo with `recovery watch` running concurrently: doctor surfaces
    a warning; sweeps still run; no double-mutation issues.

### Trust-boundary regression tests

- `tests/test_daemon_provenance_invariants.py`
  - `provenance_mode: sealed_patch` run continues to refuse to start
    when accessed via the daemon. The daemon does not paper over
    `Unsupported containment` errors.
  - `attested_bylines` workflows: bylines rendered through the daemon
    match those rendered through direct CLI for the same repo state.
  - Daemon does not expose `apply`, `keys`, or `receipts/issue` in
    V1; tests assert `404 not_a_read_endpoint` for any such path.

### Smoke / load

- `tests/test_daemon_smoke.py`
  - 200 concurrent read requests against `/v1d/repos/<id>/status`
    do not deadlock.
  - SSE clients can subscribe / unsubscribe without leaking threads.
  - Daemon shuts down cleanly within 5s of SIGTERM.

## 19. Decision Points That Need Operator Input

Things V1 commits to but that the operator should explicitly accept
(record in `docs/DECISION_LOG.md` before landing):

1. **Hybrid storage (Option C).** Per-repo SQLite is unchanged;
   daemon registry is separate. Going to Option B (central daemon
   SQLite) is a one-way door; V1 picks C for revertibility.
2. **Python core for V1.** Go rewrite stays an open RFC question;
   V1 prototypes in Python so RFC 0026/0027 semantics aren't
   re-implemented under V1.
3. **No mutation surface in V1 daemon.** Every mutating verb stays
   on the direct CLI (and on `striatum serve --allow-mutations` for
   web users). Operators who want web-mediated mutation continue to
   use the per-repo service.
4. **Bootstrap admin token is the only `admin`-by-default token.**
   The operator must explicitly mint a `read` token before connecting
   any client. Scripts that auto-add `STRIATUM_TOKEN=$(cat …)` should
   read a non-bootstrap token.
5. **Audit log hashes argv rather than storing it.** This is more
   conservative than RFC 0028 §10 strictly requires; the synthesis
   should weigh whether full argv (with possible secrets in user-
   supplied paths or message bodies) is acceptable. V1 chooses hash;
   operators can opt into full argv via a future flag.

## 20. Deferred Scope (Explicit)

Out of scope for V1, recorded so the synthesis does not silently
absorb them:

- Daemon-owned supervised processes (RFC 0028 §8 step 4).
- Daemon-mediated mutation MCP tools (§8 step 5).
- Daemon mutation capabilities `claim`, `review`, `apply` exposed
  to clients (§8 step 6). Reserved in vocabulary; not enforceable
  beyond capability lookup until endpoints exist.
- Sealed-mode authority migration into daemon (§9 of RFC 0028).
- Cross-repository workflows and cross-repo runs.
- Multi-user / OS-user identity mapping.
- Per-client session expiry, lockout, compromised-client recovery
  (RFC 0028 §10 future work).
- WebSocket / bidirectional control channels.
- Long-poll fallback for SSE (the daemon is local; corporate proxies
  don't apply).
- A Go core (RFC 0028 §7).
- macOS / Windows daemon packaging beyond "this is a Python
  subprocess." launchd / systemd unit files belong in a follow-up
  RFC.
- Replacing direct CLI mode (RFC 0028 §8 step 7).
- Daemon log files (only the audit table is durable; no stdout log
  capture).
- Receipt issuance, key custody, apply gate.
- Web mutation buttons surfaced through the daemon (per-repo
  `striatum serve --allow-mutations` continues to be the supported
  path for those).
- Token rotation / per-token rate limits.
- Outbound calls of any kind. The daemon never phones home; D020
  remains intact.

## 21. Summary

V1 daemon mode is RFC 0028 §8 step 1, no more. It registers
repositories, exposes read-only views (HTTP + MCP + CLI + web) across
them, runs recovery overnight without per-run watcher PIDs, and
audits every client request. It deliberately does not gain authority
that its current tools cannot defend: no mutation surface, no
supervision migration, no sealed authority, no signing keys, no
cross-repo workflows, no multi-user model, no transcript capture.

RFC 0026's lane attestation invariant survives unchanged because the
daemon never spawns supervised processes in V1. RFC 0027's
provenance-mode honesty survives unchanged because the daemon never
relabels modes and exposes no apply or signing endpoints. Direct CLI
mode survives unchanged because the daemon-routed path is opt-in per
call, never silent.

The phases and tests above are deliberately conservative so that the
synthesizer and reviewers can both push V1 *smaller* (drop a phase,
defer a capability) without breaking the trust boundaries laid out in
§§ 1, 12, and 13. Pushing V1 *bigger* — adding a mutation tool,
moving keys in early, claiming sealed authority — is the change that
needs the most scrutiny, because the daemon's authority is exactly
what the RFC 0026/0027 work set out to constrain.
