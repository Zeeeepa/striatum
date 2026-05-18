# RFC 0030: Daemon RPC Server and Version Skew Protocol

Status: accepted (V2)
Date: 2026-05-11
Supersession note: D094 / RFC 0043 retired `--no-daemon` and made
daemon-owned PostgreSQL mandatory for workflow-state verbs. Any older
sections below that mention direct repo-local fallback or optional daemon
mode are historical context; current behavior is exit 11 for unreachable
daemon and exit 12 for unregistered repositories or repositories with legacy
SQLite state.
Context:
[`RFC 0028`](0028-long-running-daemon-and-multi-repository-control-plane.md),
[`RFC 0033`](0033-storage-substrate-rewrite-for-daemon-v2.md) (accepted V2),
[`RFC 0012`](0012-local-service-api.md),
[`RFC 0013`](0013-local-web-ui.md),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D082, D083, D084, D086, D006, D009),
[`docs/SPEC.md`](../SPEC.md) § "Product Boundary" and § "State Store",
`src/striatum/daemon.py`,
`src/striatum/api.py`,
`src/striatum/service.py`

RFC 0030 is the spine of daemon V2. It is gated on RFC 0033 (substrate)
landing first because the wire protocol, audit framing, and request log
key off the substrate choice.

Implementation status: dogfood-034 lands the envelope-v1 codec, JSON
framing helpers, owner-local transport guards, `daemon.hello` /
`daemon.welcome`, `daemon.describe`, the capability-bound method registry,
and PostgreSQL request/audit helper wiring. Later RFC 0048/D094 work
completed daemon accept-loop CLI routing and retired production
repo-local direct mode.

## Problem

RFC 0028 V1 shipped what dogfood-031 round 2 named "no daemon RPC server."
The Unix socket bound by `striatumd` is a lifecycle marker; CLI and MCP
clients open the daemon registry SQLite directly under capability
checks. D082 ratifies the long-term product as "daemon with CLI client"
— daemon-mediated routing becomes primary and direct repo-local CLI mode
becomes a compatibility shim. D094/RFC 0043 later retire that shim for
production and make the paired test-harness escape the only direct-mode
path.

Closing finding A1 from dogfood-031 means designing a real RPC layer
that:

- routes mutating requests through the daemon process (so audit, capability
  checks, and serialization are mediated, not advisory);
- supports the Python daemon now (V2.0) and a future Go daemon (D084)
  without re-litigating the wire format;
- handles version skew between CLI and daemon explicitly, including
  refuse/downgrade semantics (RFC 0028 OQ#7);
- binds capabilities to RPC routes, not to ad-hoc "is the operator
  allowed" checks in each command;
- preserves the V1 read surfaces (`status`, `doctor`, `why`,
  `dashboard --all`) without operator-visible behavior change.

## Goals

- Define a language-agnostic wire protocol for daemon RPC.
- Specify the daemon transport layer: Unix-domain socket default, loopback
  HTTP optional, MCP over the same authorization model.
- Specify the version handshake and refuse/downgrade rules.
- Specify the capability-to-route binding and audit shape for every RPC
  call.
- Specify the migration path from V1 direct-registry-reads to
  daemon-mediated reads.
- Specify the request log persistence model on the RFC 0033 substrate.

## Non-Goals

- Defining sealed apply or supervised-process ownership. That lives in
  RFC 0031.
- Defining cross-repo transaction semantics. That lives in RFC 0032.
- Removing direct repo-local CLI mode. That retirement is its own
  follow-up RFC.
- Multi-user / multi-OS-user access (deferred by D083).
- Picking a specific storage substrate. That is RFC 0033's job.
- Hosted or networked transports beyond local loopback.

## Proposal

### 1. Wire framing

The daemon speaks one canonical request/response framing on every
transport:

```text
request  := envelope_v1
envelope_v1 := { schema_version, request_id, method, params, capability_token, deadline_ms }
response := { schema_version, request_id, ok, data | error, audit_id }
```

- `schema_version` is the integer version of the envelope shape.
  Bumped only on backward-incompatible changes; never silently widened.
- `request_id` is a UUIDv7 string supplied by the client. The daemon
  refuses duplicate request_ids within a configurable window.
- `method` is a dotted route name (`workflow.validate`,
  `run.prepare`, `session.register`, `repo.add`, ...). Methods are
  capability-bound (§4).
- `params` is a JSON object; method-specific schema lives in a generated
  registry the daemon publishes via `daemon.describe`.
- `capability_token` is the token string. The daemon never logs the
  secret; the audit row records the hash.
- `deadline_ms` lets the client bound long operations. Zero means no
  deadline; the daemon may apply its own server-side bound.

The wire is JSON over the transport. The daemon may later offer a
binary framing (CBOR, MessagePack) without changing the envelope shape;
clients negotiate framing during handshake (§3).

### 2. Transports

- **Unix-domain socket** (default): `${XDG_RUNTIME_DIR}/striatum/daemon.sock`
  with `0600` permissions. Newline-delimited JSON request/response.
- **Loopback HTTP**: optional, opt-in via `daemon start --http
  127.0.0.1:<port>`. One method per POST; envelope in the body. Auth via
  `Authorization: Bearer <token>` header (constant-time compare).
- **MCP over socket**: the daemon's MCP server speaks MCP over the same
  Unix socket on a documented sub-path; capability gating is shared with
  the JSON-RPC path.

WebSocket / SSE event streams are documented as future work and are not
in V2 scope. Streaming for the web UI uses the existing `LISTEN`/`NOTIFY`
plumbing in the substrate (RFC 0033) with a thin daemon-side fan-out.

No remote transports. Refused at startup if the bind address is not
loopback.

### 3. Version handshake

Every client opens a connection with `daemon.hello`:

```json
{
  "schema_version": 1,
  "client": {
    "name": "striatum-cli",
    "version": "1.22.0",
    "supported_envelope": [1],
    "supported_framings": ["json"]
  }
}
```

The daemon responds with `daemon.welcome`:

```json
{
  "ok": true,
  "data": {
    "daemon_version": "1.22.0",
    "envelope": 1,
    "framing": "json",
    "substrate": "pg-16",
    "substrate_schema": 7,
    "methods_etag": "sha256:..."
  }
}
```

Refuse / downgrade rules:

- If the daemon's `envelope` is newer than any version in `supported_envelope`,
  the daemon responds with `daemon.welcome` carrying `ok: false` and
  `error: version_incompatible`. The CLI exits with documented code 10.
- If the client's framing is unsupported, the daemon picks the highest
  framing both support; if none, refuse as above.
- If the daemon is older than the CLI by a minor version, the daemon
  publishes only the methods it knows. The CLI MAY refuse methods the
  daemon does not advertise (the CLI is responsible for not calling
  unsupported routes).
- The `methods_etag` is a stable hash of the published method list. CLIs
  cache `daemon.describe` against this etag.

No silent downgrade: the CLI never falls back to direct repo-local mode
on a daemon version mismatch. The operator must upgrade the daemon or
downgrade the CLI.

### 4. Capability binding

Every RPC method declares its required capability set in a registry
shipped with the daemon binary. Example:

```text
workflow.validate     read
status                read
why                   read
doctor                read
dashboard.all         read
repo.add              admin
repo.remove           admin
session.register      write
claim_next            claim
ack                   write
publish_artifact      write
verdict               review
complete              write
recovery.cancel_job   admin
daemon.shutdown       admin
```

Capability vocabulary in V2: `read`, `write`, `review`, `claim`,
`admin`. Mutation MCP-specific capabilities (`apply`, etc.) come in
RFC 0032. Capability tokens may carry multiple capabilities; the daemon
checks the union.

Capability tokens are issued by `daemon.token.create` (admin-only) and
revoked by `daemon.token.revoke` (admin-only). Tokens may be scoped to
a single repository_id; daemon-global tokens are explicit (`scope:
"daemon"`).

### 5. Audit shape

Every RPC call records one audit row at decision time (allowed or
denied):

```text
audit_row := {
  ts, request_id, client_id, token_hash, repo_id?, method,
  params_hash, decision, denial_reason?, exit_code?, transport,
  previous_hash, row_hash
}
```

- `params_hash` is SHA-256 of the canonicalized params; the row never
  carries the request body.
- `decision` is `allowed` or `denied`.
- `denial_reason` is one of a small documented vocabulary
  (`capability_missing`, `token_revoked`, `token_expired`,
  `repo_not_registered`, `method_unknown`, `version_incompatible`, ...).
- Hash chain rules follow RFC 0033 §5.

### 6. Request log

Distinct from audit. Stored on the substrate (RFC 0033). Retains:

- request envelope (without `capability_token` secret),
- response (or terminal error),
- timing,
- daemon version, substrate schema, request_id.

Default retention 7 days. Bounded by row count to cap disk use.
`daemon doctor` reports request-log size, oldest row, and rotation
state.

### 7. Routing through the daemon

V2 verbs route as follows:

- **Read verbs** (`status`, `why`, `doctor`, `dashboard`, `list`,
  `evidence`): route through daemon RPC in production. Unreachable
  daemon refuses with exit code 11; unregistered repositories and
  repositories with legacy SQLite state refuse with exit code 12.
- **Mutating workflow verbs** (`workflow validate`, `run prepare`,
  `run start`, `session register`, `claim-next`, `ack`,
  `publish-artifact`, `verdict`, `submit-review`, `complete`,
  `release`, `block`, `heartbeat`): route through daemon RPC in
  production. `--no-daemon` is retired; legacy SQLite compatibility is
  test-harness/migration-only.
- **Admin verbs** (`repo add/list/remove`, `daemon start/stop/status`,
  `daemon token *`, `daemon migrate`, `daemon shutdown`): daemon-only.
  No direct fallback.
- **Adapter / supervisor verbs**: deferred to RFC 0031.

CLI shape stays mostly unchanged. The daemon client library transparently
turns each CLI invocation into one or more RPC calls.

### 8. Compatibility with V1

- V1 daemon registry SQLite is migrated to the new substrate via RFC
  0033 §4 before RFC 0030 ships.
- V1 `--daemon` read routing semantics were preserved during the
  transition. Current production behavior requires the daemon for all
  Striatum verbs.
- Direct repo-local CLI mode was transition-only. Current production
  uses daemon-owned PostgreSQL; legacy SQLite compatibility is limited
  to migration and explicit test fixtures.
- Existing dogfood workflows (`docs/dogfood/030`, `docs/dogfood/031`)
  do not need re-running. New runs may be configured to require
  daemon-mediated mode via a workflow-level `require_daemon: true`
  field defined in RFC 0031.

### 9. Test infrastructure

- A daemon test harness spins a daemon process bound to a
  per-test runtime directory and a per-test substrate (RFC 0033 §7).
- CLI tests gain a `--daemon-url` plumbing that the harness sets so
  RPC traffic in tests goes to the per-test daemon.
- Legacy direct-mode tests are fixture-only; production behavior asserts
  daemon-required refusal.
- Version-skew tests inject mismatched envelope / methods to assert
  the refuse/downgrade rules.

### 10. Provenance and trust implications

- The daemon now mediates every mutation. Daemon compromise is
  workflow-state compromise; the V1 trust boundary tightens around the
  daemon process and its substrate.
- Capability tokens are the only access path for non-health requests.
  No env-var token fallback (per V1 dogfood-031 round-3 finding); the
  daemon runtime token lives under the daemon runtime directory as
  `client-token` with `0600` permissions and degraded-trust warning.
- The daemon never parses agent stdout/stderr; structured RPC calls are
  the only mutation surface. Agent processes call the daemon via the
  same RPC, not via direct SQLite writes.
- RFC 0031 builds sealed-apply authority on top of this RPC trust
  boundary.

## Compatibility and Migration

- RFC 0033 substrate migration runs first (operator UX).
- Then `striatum daemon start` opens the RPC server; CLI clients begin
  routing through the daemon by default.
- Direct-mode mutation verbs were transition-only. D094/RFC 0043 and
  D104 retired them for production use.
- Existing CLI scripts that pass `--repo /path` keep working because
  the daemon resolves the repo against its registry.

## Downsides and risks

- The daemon process is now a single point of failure for the orchestration
  surface. D094/RFC 0043 removed direct production CLI fallback; an
  unreachable daemon is exit 11 and an unregistered or legacy-SQLite
  repository is exit 12.
- Wire-protocol versioning errors are now production incidents.
- The Go daemon is the production authority after D111; Python daemon text in
  older sections is retained as historical design context.
- MCP semantics now have a real mutation surface; prompt-injected MCP
  clients can exercise `write` capabilities if they hold a token. RFC
  0032 tightens the MCP mutation defaults.
- A future Go port (D084) may discover wire choices made in Python
  V2.0 that don't survive. We accept that the V2.0 Python wire is the
  reference and the Go port may need a partner RFC.

## Benefits

- One canonical mutation surface.
- Capability tokens are real, audited, route-bound, and revocable.
- Version skew is explicit and refused, not silently degraded.
- Multi-repository read and recovery flows route through a single
  audited point.
- Sealed-apply (RFC 0031) and cross-repo (RFC 0032) inherit a real
  trust boundary instead of "the daemon happened to be running."

## Acceptance Criteria

- A Python `striatumd` process accepts envelope-v1 JSON requests over
  a Unix-domain socket with `0600` permissions and refuses non-loopback
  HTTP bindings at startup.
- `daemon.hello` / `daemon.welcome` handshake works against a matching
  CLI and refuses a CLI with a higher envelope version. Test asserts
  both sides.
- Every method in the registry declares a required capability and the
  daemon refuses calls without the right token, recording an audit
  `denied` row with the documented vocabulary.
- `daemon.describe` returns the method list with a stable `methods_etag`
  the CLI can cache.
- A request log row exists on the substrate (RFC 0033) for every
  request; default retention applies.
- Read verbs route through the daemon in production; unreachable daemon
  refuses instead of falling back to repo-local SQLite.
- Mutating workflow verbs route through the daemon in production; the
  transition-era `--no-daemon` path is retired.
- Admin verbs also require the daemon boundary.
- `dashboard --all` continues to work end-to-end against the new RPC
  routing.
- Documentation in `docs/SPEC.md`, `docs/MCP.md`, `docs/CLI_REFERENCE.md`,
  and `docs/HOW_TO_HUMAN.md` is updated to name the RPC routes,
  exit code 10, and the version-skew rules.

## Open Questions

- What is the supported daemon ↔ CLI version skew window? At least
  one minor release; recommendation: support N-2 minor, refuse N-3.
- Should `daemon.describe` be free (no capability) or `read`?
  Recommendation: `read`. Reviewers should push back.
- Should the CLI cache `methods_etag` across processes? Recommendation:
  yes, under `${XDG_CACHE_HOME}/striatum/daemon/`.
- Should the request log expose a query API for operators, or stay
  internal to `daemon doctor`? Recommendation: internal in V2; expose
  in a follow-up if real demand appears.
- How should clients identify themselves beyond `client.name`? Token
  scope already binds client identity; do we need a separate
  `client_id` claim? Defer to RFC 0032 when MCP mutation needs richer
  client distinction.

## Domain Modeling

Terms to add to `docs/UBIQUITOUS_LANGUAGE.md` after acceptance:

- **RPC envelope** — the canonical request/response shape with
  `schema_version`, `request_id`, `method`, `params`, and
  `capability_token`. Versioned via the envelope schema integer.
- **Method registry** — the daemon's published list of RPC methods,
  required capabilities, params schemas, and a stable etag.
- **Version handshake** — the `daemon.hello` / `daemon.welcome`
  exchange that negotiates envelope version, framing, and method
  registry hash before any other RPC.
- **Capability scope** — the binding of a capability token to a
  repository_id or daemon-global scope; methods refuse when scope and
  method-required scope do not match.
- **Request log** — the daemon-side persistent record of every RPC
  request and response. Distinct from audit (which records authorization
  decisions and capability events); request log is for operator
  observability.
