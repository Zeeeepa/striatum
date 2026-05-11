# RFC 0030 + RFC 0031 Implementation Design

author: designer-codex-gpt-5.5-001

Status: design handoff
Date: 2026-05-11
Target: RFC 0030 daemon RPC server and RFC 0031 daemon-owned supervision plus sealed-apply boundary

## Design Position

RFC 0030 and RFC 0031 should ship as one daemon V2 slice. RPC is the
authority boundary; daemon-owned supervision, lane attestation, sealed apply,
signing custody, and apply receipts all depend on that boundary being real.
The RFC 0033 Postgres substrate is the landing pad for daemon-global state,
but it is not itself the control plane. Clients call the daemon. They do not
open the daemon DB directly.

The implementation should preserve the current product split:
repo-local `.striatum/state.sqlite3` remains the authoritative workflow/run
state store, while the daemon DB owns registry, clients, capabilities, audit,
request/session observability, scheduler cursors, supervisor ownership, signing
metadata, and apply receipts. Mutations that affect repo-local workflow state
still run through Striatum's existing parser/dispatcher semantics, but in
daemon mode the daemon invokes those semantics after capability and version
checks instead of letting every client open SQLite directly.

This design intentionally does not include cross-repository workflows, MCP
mutation expansion beyond routing through the same RPC/capability gate, the
Python-to-Go rewrite, bundled Postgres, or retirement of direct repo-local CLI
mode.

## Implementation Stages

1. Land the daemon RPC server and client library over the RFC 0033 substrate.
   This includes wire envelope parsing, transports, `daemon.hello`,
   `daemon.describe`, method registry, capability checks, audit append, request
   logging, and CLI routing defaults.

2. Move supervision ownership behind daemon RPC. Add daemon DB
   `process_supervisors`, add repo-local `process_supervisor_pointers`, and
   make `supervise.*` commands default to daemon calls while preserving
   `--no-daemon` for one transition release unless the workflow declares
   `require_daemon: true`.

3. Add sealed apply as daemon authority. Add signing key custody, key metadata,
   apply receipt rows, the `apply.reviewed_patch` route, sealed-mode `run
   start` gating, and receipt evidence artifact publication.

4. Harden with live Postgres integration tests, restart/reattach tests,
   version-skew tests, and refusal tests for every sealed-apply mismatch.

## RPC Envelope And Transports

The daemon should expose one canonical envelope across all transports:

```json
{
  "schema_version": 1,
  "request_id": "uuidv7",
  "method": "dotted.route.name",
  "params": {},
  "capability_token": "secret-token",
  "deadline_ms": 0
}
```

Responses use:

```json
{
  "schema_version": 1,
  "request_id": "uuidv7",
  "ok": true,
  "data": {},
  "audit_id": 123
}
```

Errors replace `data` with an `error` object containing `code`, `message`, and
optional `details`. Error codes must be stable strings; CLI exit-code mapping
stays at the client boundary. Duplicate `request_id` values are refused within
the request-log retention window to protect idempotency.

JSON is the required framing for V2. The envelope is deliberately
language-agnostic: primitive JSON fields, dotted route names, RFC3339
timestamps only in payloads, canonical JSON hashing rules, and no Python class
names or exception strings in the protocol. A future Go daemon should be able
to implement the same contract with only the method registry and canonical
hashing rules as shared fixtures.

Transports:

- Unix-domain socket is the default at `${XDG_RUNTIME_DIR}/striatum/daemon.sock`
  with owner-only runtime directory permissions and `0600` socket permissions.
  The socket carries newline-delimited JSON envelopes.
- Loopback HTTP is opt-in via daemon startup config. It refuses any non-loopback
  bind address. The body carries the envelope; `Authorization: Bearer ...` may
  carry the token for HTTP clients, but the daemon normalizes it into
  `capability_token` before authorization.
- MCP over socket uses the same daemon authorization and method registry. This
  dogfood should not expand `tools/list`; MCP mutation defaults belong to RFC
  0032. Any MCP call that does invoke a mutating route must pass through the
  same RPC path, audit append, and capability checks.

## Version Handshake

Every connection begins with `daemon.hello`. The initial hello is the only
method that may be accepted before route authorization, and it still records a
metadata-only audit row for malformed or incompatible clients.

Client hello:

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

Daemon welcome:

```json
{
  "ok": true,
  "data": {
    "daemon_version": "1.22.0",
    "envelope": 1,
    "framing": "json",
    "substrate": "postgres",
    "substrate_schema": 1,
    "methods_etag": "sha256:...",
    "sealed_apply": {"supported": true, "key_loaded": true}
  }
}
```

Refuse and downgrade rules:

- If no shared envelope version or framing exists, respond with
  `version_incompatible`; the CLI exits 10.
- If the daemon is older than the CLI, the daemon publishes only its actual
  methods. The CLI must refuse to call routes missing from `daemon.describe`.
- No silent fallback to direct mode after a daemon version mismatch. Operators
  must upgrade/downgrade or pass `--no-daemon` on a verb that still supports
  direct mode.
- `methods_etag` is a canonical hash of route name, params schema hash,
  required capabilities, minimum envelope, and deprecation state. Any registry
  change invalidates it.

`daemon.describe` returns method metadata when the caller has `read`
capability. It accepts an optional `if_methods_etag`; when unchanged, the
daemon returns `not_modified` with the current etag and no method body.

## Method Registry And Capabilities

The daemon owns a generated method registry, not scattered per-command checks.
Each entry has:

```text
route_name
params_schema_version
params_schema_hash
required_capabilities
repository_scope: none | required | optional
handler
audit_class: read | mutation | admin | apply
min_envelope
```

Capability vocabulary for this paired slice is:

```text
read, write, review, claim, apply, admin
```

`apply` belongs here, not later, because sealed mode cannot ship without an
explicit apply token. Tokens may carry multiple capabilities and may be scoped
to one `repository_id` or daemon-global admin scope. Token secrets are never
stored or audited; token hashes and token ids are.

Initial route bindings:

| Route family | Capability | Notes |
|---|---|---|
| `daemon.hello` | none | Version negotiation only; malformed/incompatible attempts audited. |
| `daemon.describe`, `status`, `why`, `doctor`, `dashboard.*` | `read` | Repo-scoped unless explicitly daemon-global. |
| `workflow.validate`, `run.prepare`, `run.start`, `session.register`, `ack`, `block`, `heartbeat`, `publish_artifact`, `complete`, `release` | `write` | Transitional `--no-daemon` allowed unless `require_daemon`. |
| `claim_next` | `claim` | Separate from generic writes so agents can receive work without broad mutation authority. |
| `verdict`, `submit_review` | `review` | Review authority is narrower than general write. |
| `supervise.start`, `supervise.send`, `supervise.stop` | `write` | Daemon becomes process parent. |
| `supervise.status`, `supervise.list` | `read` | Reads daemon DB authoritative rows. |
| `apply.reviewed_patch` | `apply` | Always daemon-only. |
| `repo.add`, `repo.remove`, `daemon.token.*`, `daemon.key.rotate`, `daemon.shutdown`, `daemon.migrate` | `admin` | No direct fallback. |

`run.start` for sealed workflows requires both `write` for the start mutation
and an available repo-scoped token with `apply`; the caller does not need to
hold the apply token personally if the workflow is being started by an admin
that provisions it, but the daemon must verify that such a grant exists.

## Audit And Request Log

Every request receives one metadata-only audit row at authorization decision
time, allowed or denied. The row shape should map directly onto the RFC 0033
`striatumd.audit_log` table:

```text
ts
schema_version
hash_format_version
daemon_version
client_id
repository_id
method
decision
denial_reason
transport
request_id
exit_code
params_sha256
previous_hash
row_hash
segment_id
```

`params_sha256` is the SHA-256 of canonical JSON params. It is never the raw
body. The audit chain excludes transcripts, stdout/stderr, artifact contents,
token secrets, request bodies, response bodies, tracebacks, and free-form model
prose. Denial reasons must be a small vocabulary:
`capability_missing`, `token_revoked`, `token_expired`,
`repo_not_registered`, `method_unknown`, `version_incompatible`,
`deadline_exceeded`, `schema_invalid`, `sealed_key_missing`,
`base_tree_mismatch`, `patch_digest_mismatch`, `review_verdict_missing`, and
`apply_gate_unsatisfied`.

The request log is observability, not integrity. It should use the existing
`striatumd.rpc_request_log` table, extended only if needed with
`response_hash`, `started_at`, `finished_at`, `expires_at`, daemon version,
envelope version, schema version, decision, and `audit_id`. If a request body
needs debugging, store only canonical hashes. Default retention is seven days
or a bounded row count, whichever expires first. `daemon doctor` reports
oldest retained request, row count, retention health, and any duplicate
request-id refusals.

## CLI Routing Migration

V2 routing should use a daemon client library below the CLI dispatcher. The
parser stays familiar; the routing layer decides direct mode versus daemon
mode before executing command semantics.

- Read verbs default to daemon RPC and allow `--no-daemon` fallback.
- Mutating workflow verbs default to daemon RPC and allow `--no-daemon` only
  during the transition. If a workflow declares `require_daemon: true`,
  direct mode is refused.
- Admin verbs are daemon-only.
- Supervision verbs default to daemon RPC. Direct mode is allowed for one
  minor release only for non-`require_daemon` workflows.
- Sealed apply has no direct mode.

The daemon handler may call existing repo-local command modules internally, but
that internal call must be after authorization, audit decision, and
request-log creation. The daemon should not shell out to `striatum`; it should
call Python module functions in the Python implementation and the equivalent
command-service layer in a future Go implementation.

Migration from V1 direct registry reads:

1. RFC 0033 cutover migrates the daemon registry to Postgres.
2. `striatumd` starts the RPC server instead of exposing only a lifecycle
   marker socket.
3. V1 `--daemon` read paths become RPC reads.
4. Direct registry-opening code remains only for cutover/tombstone inspection
   and is not used by normal clients after cutover.

## Daemon-Owned Supervision

Add RFC 0031 tables in a new daemon DB migration rather than modifying the RFC
0033 baseline retroactively.

Daemon DB:

```text
process_supervisors(
  supervisor_id primary key,
  session_id,
  run_id,
  repository_id,
  adapter,
  command_json,
  cwd,
  scratch_path,
  stdin_pipe_path,
  pid,
  pid_start_time,
  state,
  started_at,
  attached_at,
  heartbeat_at,
  ended_at,
  stop_reason
)
```

`state` is `starting | attached | detached | stopped | lost`. Use text with
`CHECK`, matching RFC 0033 style. Add indexes on `(repository_id, run_id,
state)` and a partial unique index for one active supervisor per
`session_id`.

Repo-local SQLite gets only:

```text
process_supervisor_pointers(
  session_id primary key,
  supervisor_id,
  daemon_substrate_version,
  last_known_state,
  last_observed_at
)
```

The pointer table is observability for run summaries and byline derivation
when repo-local code needs a quick answer. The daemon is the only writer. If a
pointer is stale or the daemon is unavailable, lane attestation is not
upgraded; bylines must downgrade rather than infer.

`supervise.start` validates the session, lane command, adapter constraints, and
repository registration; creates a scratch directory and FIFO; inserts a
daemon DB row in `starting`; spawns the configured lane command as a daemon
child with stdout/stderr to `DEVNULL`; records `pid` and `pid_start_time`; then
transitions to `attached` and updates the repo-local pointer. The daemon never
parses terminal output.

`supervise.send` writes the stored work packet JSON plus newline to the
supervisor FIFO, refreshes heartbeat, and records `supervisor.packet_delivered`
metadata. `supervise.stop` sends SIGTERM, waits five seconds, then SIGKILL,
updates state, and releases or marks related leases through normal runner
semantics. `supervise.status` and `supervise.list` read daemon rows and probe
current pid identity.

On daemon restart, the daemon scans `attached` rows, verifies `pid` plus
`pid_start_time` using the platform process-start token, and either reattaches
or marks `lost`. Mismatches are not recoverable by optimism; the row becomes
`lost`, the pointer is updated, and normal stale-lease recovery takes over.

## Sealed Apply

`provenance_mode: sealed_patch` becomes startable only under daemon authority.
The new workflow fields are:

```json
{
  "require_daemon": true,
  "apply_gate": true,
  "sealed_patch_provider": "daemon"
}
```

`sealed_patch_provider: "refuse"` remains a debugging aid to preserve the V1
refuse-to-start behavior. If `provenance_mode` is `sealed_patch`, the runner
should imply `require_daemon: true`; an explicit `require_daemon: false` is a
validation error.

`run.start` refuses a sealed workflow unless:

- the caller is connected to a daemon;
- `daemon.welcome` advertises sealed apply support;
- the daemon has loaded a signing key;
- the repository has a repo-scoped `apply` token grant;
- the workflow declares protected/operator-writable path policy that passes
  existing RFC 0027 validation;
- direct `--no-daemon` mode is not in use.

`apply.reviewed_patch` accepts:

```json
{
  "repository_id": "repo_...",
  "run_id": "run_...",
  "patch_artifact_id": "art_...",
  "reviewer_verdict_id": "verdict_...",
  "expected_patch_sha256": "hex",
  "expected_base_tree": "git-tree",
  "deadline_ms": 300000
}
```

The daemon verifies, in order:

1. Caller has `apply` for the repository.
2. Run is `sealed_patch`, unpaused, unblocked for apply, and daemon-required.
3. Patch artifact exists, is immutable, and its recorded digest matches bytes.
4. Patch touched paths are inside write scope and outside forbidden paths.
5. Reviewer verdict exists, is accepting, and binds to the same patch digest
   and base/result tree.
6. The current protected repository base tree equals the patch base tree.
7. Required verification jobs, if configured, passed against the candidate.
8. Daemon-owned worktree applies the patch cleanly.
9. Post-apply tree hash equals the computed candidate result.

Any mismatch refuses before modifying protected source and records an audit
denial. The daemon applies into a daemon-owned worktree, not the operator's
editable checkout. This slice should emit patch plus receipt evidence, not
commit, push, merge, amend, or rebase git history.

## Signing Key Custody And Rotation

The daemon owns an Ed25519 keypair. Private key lookup order:

1. OS keyring, keyed by daemon instance id.
2. `0600` runtime fallback file with a degraded-trust warning.

Sealed-mode runs refuse to start if no private key can be loaded. Public key
metadata is published through `daemon.describe` and receipt verification
surfaces. Store key metadata in daemon DB:

```text
signing_keys(
  key_id primary key,
  public_key,
  algorithm,
  custody,
  created_at,
  activated_at,
  retired_at,
  state
)
```

`daemon.key.rotate` is admin-only. Rotation creates a new key, marks it active,
and retires the old key for signing while preserving old public keys for
receipt verification. Losing the private key blocks new sealed applies but must
not invalidate old receipts.

## Apply Receipts

Add append-only daemon DB receipt rows:

```text
apply_receipts(
  receipt_id primary key,
  repository_id,
  run_id,
  patch_artifact_id,
  patch_sha256,
  base_tree_hash,
  post_apply_tree_hash,
  reviewer_verdict_id,
  signing_key_id,
  daemon_version,
  substrate_version,
  applied_at,
  receipt_sha256,
  signature,
  audit_id
)
```

Receipt payload version 1:

```json
{
  "receipt_version": "striatum.apply_receipt.v1",
  "repository_id": "repo_...",
  "run_id": "run_...",
  "patch_artifact_id": "art_...",
  "patch_sha256": "hex",
  "base_tree_hash": "git-tree",
  "post_apply_tree_hash": "git-tree",
  "reviewer_verdict_id": "verdict_...",
  "signing_key_id": "key_...",
  "daemon_version": "1.22.0",
  "substrate_version": 2,
  "applied_at": "2026-05-11T20:00:00Z"
}
```

Canonicalize this payload, sign it, store it append-only in Postgres, and
publish a Markdown evidence artifact under the run's evidence path with:

```text
author: striatumd-<instance-id>
```

The Markdown artifact should include receipt id, patch digest, base tree,
post-apply tree, verdict id, key id, timestamp, daemon version, substrate
version, and signature. It is durable provenance; the daemon DB receipt is the
authoritative sealed-apply record.

## Data Migration

Postgres migration 2 should add method-registry support tables only if the
registry must be introspected from SQL. The source of truth can remain generated
code plus an etag. Existing RFC 0033 `rpc_request_log` and `client_sessions`
are reused.

Postgres migration 3 should add `process_supervisors`,
`process_supervisor_events` if needed for restart diagnostics, `signing_keys`,
and `apply_receipts`.

Repo-local migration adds `process_supervisor_pointers`. During first daemon
start after upgrade, active V1 repo-local `process_supervisors` rows are
imported into daemon DB when their pid plus `pid_start_time` verifies. Failed
verification marks them `lost`; terminated historical rows stay repo-local for
run summary reproduction.

The daemon must never hold Postgres locks while doing long git apply work. Use
short transactions for authorization/audit, worktree reservation, receipt
insert, and pointer updates. Long work happens between transactions with a
lease-like apply operation id so crash recovery can mark in-progress applies
as failed before any protected source update is claimed.

## Tests

Add a daemon harness that starts per-test Postgres when available or uses a
unique schema/database under `STRIATUM_DAEMON_DB_URL`. The harness starts the
daemon on a per-test runtime directory and injects client tokens without using
global operator state.

Required coverage:

- envelope parsing rejects missing fields, bad schema versions, duplicate
  request ids, non-object params, and invalid deadlines;
- Unix socket permissions are owner-only; loopback HTTP refuses non-loopback;
- hello/welcome accepts compatible clients and exits 10 on incompatible
  envelope/framing;
- methods etag changes when route capability or params schema changes;
- every route in the registry has a capability test for allow and deny;
- audit rows are written for allowed and denied requests and never contain
  token secrets or bodies;
- request logs rotate by age/row bound and remain separate from audit;
- CLI defaults route through daemon, while supported `--no-daemon` paths still
  work during the transition;
- supervisor start/send/stop/list routes mutate daemon DB rows and repo-local
  pointers correctly;
- daemon restart reattaches only when pid plus start-time token matches;
- lost supervisor updates downgrade lane attestation;
- sealed `run.start` refuses without daemon, without key, without apply token,
  or with `sealed_patch_provider: refuse`;
- `apply.reviewed_patch` refuses digest mismatch, missing review verdict,
  non-accepting verdict, base-tree drift, out-of-scope paths, missing apply
  capability, and unsupported key custody;
- happy-path apply writes one receipt row, one audit row, and one Markdown
  evidence artifact with matching hashes and signature;
- key rotation preserves old receipt verification and uses the new key for
  subsequent receipts.

## Documentation Deltas

After implementation, update:

- `docs/SPEC.md` for daemon RPC as default route, exit code 10 version skew,
  daemon-owned supervision, sealed apply startability, and the continued
  repo-local state boundary.
- `docs/MCP.md` to say daemon MCP requests route through RPC/capability checks
  while mutation tool expansion remains RFC 0032.
- `docs/UBIQUITOUS_LANGUAGE.md` for daemon RPC, method registry, request log,
  apply gate, apply receipt, signing key custody, and supervisor pointer.
- `docs/CLI_REFERENCE.md` for daemon routing flags, `daemon.describe`,
  `daemon.key.rotate`, and `apply reviewed-patch`.
- `docs/HOW_TO_HUMAN.md` and `docs/HOW_TO_AGENT.md` for daemon-required sealed
  workflows and stale/lost supervisor recovery.
- RFC 0030 and RFC 0031 status blocks and implementation notes.

## Deferred Scope

RFC 0032 owns cross-repository workflows and broad MCP mutation capability
expansion. This slice only ensures that any MCP mutation that exists routes
through the same RPC, capability, audit, and request-log path.

The Python-to-Go daemon port is deferred. This design keeps the wire contract,
canonical hashes, route registry, and SQL migrations language-neutral so the Go
port has a stable target.

Bundled, embedded, and Dockerized Postgres distribution is deferred from RFC
0033. This design assumes the system Postgres substrate already exists.

Retirement of direct repo-local CLI mode is deferred to a future RFC. This
slice may warn on direct mutating fallback, but it must not remove it except
where sealed mode or admin-only daemon operations explicitly require daemon
authority.
