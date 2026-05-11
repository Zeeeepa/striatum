---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/034/design/codex/DESIGN.md", "docs/dogfood/034/design/claude_code/DESIGN.md", "docs/dogfood/034/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# Design Synthesis: RFC 0030 + RFC 0031

Status: implementation plan
Date: 2026-05-11
Target: RFC 0030 daemon RPC server and RFC 0031 daemon-owned supervision plus sealed apply

## Design Decision

Implement RFC 0030 and RFC 0031 as one daemon V2 line of work, staged behind
the RFC 0033 PostgreSQL substrate. The concrete path is the Codex/Claude
convergence path: build a language-neutral JSON RPC boundary first, route CLI
and MCP surfaces through it, then move supervision and sealed apply authority
behind that same boundary. Gemini's service-manager installer and Windows
service work are deferred because they are operational packaging, not an
acceptance criterion for the paired RFCs.

The live workflow state boundary does not move. Repository-local
`.striatum/state.sqlite3` remains the authoritative run/job/session/artifact
state store for one target repository. The daemon PostgreSQL database owns
daemon-global state only: registered repositories, clients, capabilities,
audit, request logs, method registry metadata, supervisor ownership, signing
key metadata, and apply receipts. Daemon-mediated workflow mutations call the
same runner command-service semantics after version, capability, audit, and
request-log checks; clients no longer open daemon state directly.

Sealed apply is an AI guardrail. It proves that a daemon-accepted apply receipt
matches a reviewed patch digest and a daemon-owned apply operation. It does
not prove model-token authorship, independent human judgment, resistance to a
malicious local operator, or resistance to code execution as the daemon OS user.

## Accepted Implementation Scope

RFC 0030 acceptance criteria map to the following concrete implementation
work:

| RFC 0030 acceptance criterion | Code plan | Test owner |
|---|---|---|
| Python `striatumd` accepts envelope-v1 JSON over a Unix-domain socket with `0600` permissions and refuses non-loopback HTTP bindings. | Add `src/striatum/daemon_rpc/envelope.py`, `framing.py`, `server.py`, `transport_unix.py`, and `transport_http.py`; wire startup from `src/striatum/daemon.py`; keep JSON NDJSON as V2.0 framing. | `tests/test_daemon_rpc_transport.py` |
| `daemon.hello` / `daemon.welcome` works for matching CLI and refuses a higher envelope version. | Add `src/striatum/daemon_rpc/handshake.py` and `src/striatum/daemon_rpc/client.py`; map incompatible envelope/framing to exit code 10. | `tests/test_daemon_rpc_handshake.py` |
| Every method declares a required capability and denied calls record audit rows. | Add `src/striatum/daemon_rpc/registry.py` and `capability.py`; extend `src/striatum/daemon_pg/audit.py` with canonical params hashing and denial vocabulary. | `tests/test_daemon_rpc_capabilities.py`, `tests/test_daemon_pg.py` |
| `daemon.describe` returns method list with stable `methods_etag`. | Make generated registry the code source of truth; optionally mirror rows in PG migration `0002_rpc_methods.sql` for introspection; implement `daemon.describe` in `registry.py`. | `tests/test_daemon_rpc_registry.py` |
| Request log row exists for every request and default retention applies. | Reuse RFC 0033 `striatumd.rpc_request_log`; add `src/striatum/daemon_rpc/request_log.py` with 7-day / bounded-row pruning and duplicate request-id refusal. | `tests/test_daemon_rpc_request_log.py` |
| Read verbs route through daemon by default; `--no-daemon` falls back to repo-local SQLite for reads only. | Add routing layer in `src/striatum/cli/dispatch.py`; map read command params to daemon routes and keep direct read fallback. | `tests/test_daemon_rpc_cli_routing.py` |
| Mutating workflow verbs route through daemon; transition `--no-daemon` works with warning. | Route `workflow.validate`, `run.prepare`, `run.start`, `session.register`, `claim_next`, `ack`, `heartbeat`, `publish_artifact`, `verdict`, `submit_review`, `complete`, `release`, and `block` through daemon by default; direct path remains only when workflow does not require daemon. | `tests/test_daemon_rpc_cli_routing.py`, `tests/test_cli_mvp.py` |
| Admin verbs refuse `--no-daemon`. | Bind `repo.*`, `daemon.token.*`, `daemon.key.rotate`, `daemon.migrate`, and `daemon.shutdown` as daemon-only in `dispatch.py`. | `tests/test_daemon_rpc_cli_routing.py` |
| `dashboard --all` works end-to-end over RPC. | Repoint global dashboard reads from direct registry access to `dashboard.all` RPC while preserving existing `src/striatum/dashboard.py` rendering. | `tests/test_daemon_rpc_dashboard_all.py`, `tests/test_dashboard.py` |
| Docs name RPC routes, exit code 10, and version-skew rules. | Update `docs/SPEC.md`, `docs/MCP.md`, `docs/CLI_REFERENCE.md`, `docs/HOW_TO_HUMAN.md`, and RFC 0030 status notes. | `tests/test_doc_links.py` plus doc review |

RFC 0031 acceptance criteria map to the following concrete implementation
work:

| RFC 0031 acceptance criterion | Code plan | Test owner |
|---|---|---|
| `supervise.start` over RPC spawns a lane process as the daemon child and records pid plus `pid_start_time`. | Add `src/striatum/daemon_supervisor/spawn.py`, `lifecycle.py`, `reattach.py`, and `pointer.py`; route `src/striatum/cli/supervise.py` through daemon by default. | `tests/test_daemon_supervisor.py` |
| `supervise.send` writes a packet to FIFO and records `supervisor.packet_delivered`. | Implement FIFO write and heartbeat update in `daemon_supervisor/lifecycle.py`; write metadata event in daemon DB and repo-local pointer update. | `tests/test_daemon_supervisor.py` |
| Daemon restart reattaches one alive child and verifies `pid_start_time`. | Add daemon startup reattach hook in `src/striatum/daemon.py`; use `/proc/<pid>/stat` on Linux and `psutil.Process(pid).create_time()` only as optional portability helper. | `tests/test_daemon_supervisor_restart.py` |
| `run start` for `sealed_patch` succeeds with signing key and `apply` token; refuses otherwise. | Add sealed preflight in `src/striatum/daemon_apply/apply_service.py` and route-aware `run.start`; workflow validator accepts `require_daemon`, `apply_gate`, and `sealed_patch_provider`. | `tests/test_daemon_apply_startup.py`, `tests/test_workflow_field_errors.py` |
| `apply.reviewed_patch` happy path records apply receipt and Markdown evidence artifact bylined `author: striatumd-<instance-id>`. | Add `src/striatum/daemon_apply/signing_key.py`, `patch_io.py`, `worktree.py`, `receipt.py`, and `apply_service.py`; add PG migration `0003_supervisors_receipts.sql`. | `tests/test_daemon_apply.py`, `tests/test_artifact_schemas.py` |
| Base-tree drift, digest mismatch, and missing verdict each refuse with documented denial vocabulary. | Implement ordered refusal checks in `apply_service.py` before mutating the daemon-owned worktree; audit all denials. | `tests/test_daemon_apply_adversarial.py` |
| Docs name daemon-mediated supervision, sealed apply, and apply-gate field. | Update `docs/SPEC.md`, `docs/MCP.md`, RFC 0027 status block, `docs/CLI_REFERENCE.md`, and `docs/HOW_TO_HUMAN.md`. | `tests/test_doc_links.py` plus doc review |

## Deferred Scope

| Deferred item | Why deferred | Landing place |
|---|---|---|
| Cross-repo workflows | The paired slice keeps repo-local run state and single-repo mutation semantics. Cross-repo scheduling needs its own schema and crash reconciliation. | RFC 0032 |
| MCP mutation capability expansion | This slice routes any daemon MCP mutation through the same RPC/capability/audit path, but does not broaden the tool set beyond the paired RFC needs. | RFC 0032 |
| Python to Go core port | The wire contract, canonical JSON hashing, and SQL migrations must be language-neutral now; replacing the Python daemon is a separate implementation program. | D084 follow-up RFC |
| Bundled or Dockerized PostgreSQL distribution | RFC 0033 chose system Postgres for V2. Packaging Postgres changes installer and lifecycle responsibility. | RFC 0033 follow-up |
| Retirement of direct repo-local CLI mode | V2 needs a compatibility window and real operator experience before removing direct mode. | Future daemon-first retirement RFC |
| Service-manager installer | Useful for daemon persistence, but not required for RPC, supervision ownership, or sealed apply acceptance. | Future service-manager RFC |

## RPC Envelope and Framing Decision

Use one canonical envelope on every V2.0 transport:

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
  "audit_id": "aud_..."
}
```

Errors set `ok: false` and replace `data` with:

```json
{
  "code": "stable_reason",
  "message": "human readable summary",
  "details": {}
}
```

`schema_version` is an integer envelope version, not the daemon version. The
daemon refuses incompatible envelopes with exit code 10. `request_id` is
client-supplied UUIDv7; duplicates are refused within request-log retention.
`params` must be a JSON object. `deadline_ms: 0` means no client deadline, but
the server may still apply method-specific bounds.

V2.0 framing is newline-delimited JSON over Unix sockets and one JSON body per
HTTP request. The envelope is deliberately Go-portable: primitive JSON types,
dotted method names, canonical JSON hashing for params, no Python exception
classes on the wire, and generated method-registry fixtures that a future Go
daemon can consume.

## Transport Decision

Unix-domain socket is the default transport:

| Platform | Default socket discovery |
|---|---|
| Linux | `${XDG_RUNTIME_DIR}/striatum/daemon.sock`; fallback refused if the runtime dir is not owner-only |
| macOS | `${TMPDIR}/striatum-${UID}/daemon.sock` unless `STRIATUM_DAEMON_RUNTIME_DIR` is set |
| Windows | Deferred. Named pipe `\\.\pipe\striatumd` is the likely future shape, but Windows daemon support is not in this slice |

The Unix socket and parent runtime directory must be owner-only; socket mode is
`0600`. Loopback HTTP is opt-in via daemon startup config, for example
`--http 127.0.0.1:<port>`. Any non-loopback bind address is refused at startup
with exit code 10. HTTP carries the same envelope in `POST /rpc/v1`; the
bearer header is allowed as a transport convenience but normalized into the
same token verification path.

MCP uses the daemon socket as a sub-surface over the same authorization model.
It does not get a separate trust boundary.

## Version Handshake Rules

Every connection starts with `daemon.hello`; no ordinary route may run before
the handshake.

Client hello params:

```json
{
  "client": {
    "name": "striatum-cli",
    "version": "1.22.0",
    "supported_envelope": [1],
    "supported_framings": ["json"]
  }
}
```

Daemon welcome data:

```json
{
  "daemon_version": "1.22.0",
  "envelope": 1,
  "framing": "json",
  "substrate": "postgres",
  "substrate_schema": 1,
  "methods_etag": "sha256:...",
  "sealed_apply": {
    "supported": true,
    "key_loaded": true,
    "public_key": "base64..."
  }
}
```

Rules:

- If there is no shared envelope version, return `version_incompatible`; the
  CLI exits 10.
- If there is no shared framing, return `version_incompatible`; the CLI exits
  10.
- If the daemon is older than the CLI, the daemon publishes only methods it
  supports. The CLI refuses unsupported routes locally after `daemon.describe`.
- No silent downgrade to direct mode after version-skew refusal. The operator
  must upgrade, downgrade, or explicitly use `--no-daemon` on a verb that still
  permits direct mode.
- `daemon.describe` is `read`-capability protected and returns
  `methods_etag`. CLI cache lives under `${XDG_CACHE_HOME}/striatum/daemon/`.

## Capability Vocabulary and Method Registry

Use this closed vocabulary for the paired implementation:

```text
read    introspection and describe routes
write   ordinary workflow mutations
review  review verdict and submit-review mutations
claim   work-packet claim authority
apply   sealed apply authority
admin   repository, token, daemon, migration, and key administration
```

`recovery` remains a future capability candidate from RFC 0032. In this slice,
recovery cancellation and daemon recovery administration stay under `admin`.

The method registry lives in `src/striatum/daemon_rpc/registry.py` as the code
source of truth. A PG mirror table may be seeded for doctor/introspection, but
daemon boot must assert the Python registry and SQL mirror agree. Each entry
declares route name, params schema version/hash, required capability,
repository scope, handler, audit class, minimum envelope, and deprecation
state.

Initial mapping:

| Method | Required capability |
|---|---|
| `daemon.hello` | none before handshake; malformed attempts still audited |
| `daemon.describe` | `read` |
| `status`, `why`, `doctor`, `dashboard`, `dashboard.all`, `evidence.export` | `read` |
| `workflow.validate`, `run.prepare`, `run.start`, `session.register`, `ack`, `block`, `heartbeat`, `publish_artifact`, `complete`, `release` | `write` |
| `claim_next` | `claim` |
| `verdict`, `submit_review` | `review` |
| `supervise.start`, `supervise.send`, `supervise.stop` | `write` |
| `supervise.status`, `supervise.list`, `supervise.reattach_status` | `read` |
| `apply.reviewed_patch` | `apply` |
| `apply.receipt.show`, `apply.receipt.verify` | `read` |
| `repo.add`, `repo.remove`, `daemon.token.*`, `daemon.key.rotate`, `daemon.shutdown`, `daemon.migrate` | `admin` |

Tokens may be daemon-global or scoped to one `repository_id`. Token secrets are
never logged. Store salted token hashes in the daemon DB and use
constant-time comparison.

## Audit + Request Log Mapping

Every RPC request records both an authorization audit row and a request-log row
on the RFC 0033 PostgreSQL substrate.

Audit row shape:

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

`params_sha256` is SHA-256 of canonical JSON params. Audit never stores token
secrets, request bodies, response bodies, stdout/stderr, transcripts, artifact
contents, tracebacks, or model prose. Denials use a closed vocabulary:
`capability_missing`, `token_missing`, `token_malformed`, `token_invalid`,
`token_revoked`, `token_expired`, `repo_not_registered`, `method_unknown`,
`version_incompatible`, `deadline_exceeded`, `schema_invalid`,
`duplicate_request`, `sealed_key_missing`, `base_tree_mismatch`,
`patch_digest_mismatch`, `review_verdict_missing`, `verdict_digest_mismatch`,
`write_scope_violation`, and `apply_gate_unsatisfied`.

Request log shape reuses `striatumd.rpc_request_log`: `request_id`,
`client_id`, `repository_id`, `method`, `params_hash`, `response_hash`,
`decision`, `audit_id`, `envelope_version`, `daemon_version`,
`schema_version`, `started_at`, `finished_at`, and `expires_at`. Retention is
7 days or a bounded row count, whichever expires first. `daemon doctor` reports
request-log size, oldest retained row, and pruning health. There is no operator
query API in V2.0.

## Supervisor Ownership Migration

Add daemon DB table `striatumd.process_supervisors` in a new migration:

```text
supervisor_id PRIMARY KEY
session_id
run_id
repository_id
adapter
command_json
cwd
scratch_path
stdin_pipe_path
pid
pid_start_time
state           starting | attached | detached | stopped | lost
started_at
attached_at
heartbeat_at
ended_at
stop_reason
```

Add indexes on `(repository_id, run_id, state)` and a partial unique index for
one active supervisor per `session_id`.

Repo-local SQLite receives only:

```text
process_supervisor_pointers(
  session_id PRIMARY KEY,
  supervisor_id,
  daemon_substrate_version,
  last_known_state,
  last_observed_at
)
```

The daemon is the only writer of both the authoritative daemon row and the
repo-local pointer. If the daemon is unavailable or the pointer is stale,
attestation downgrades; repo-local code must not infer a lane-typed byline.

Supervision RPC methods:

| Method | Behavior |
|---|---|
| `supervise.start` | Validate session, lane command, adapter constraints, and repo registration; create scratch/FIFO; spawn lane as daemon child with stdout/stderr to `DEVNULL`; record pid and `pid_start_time`; update pointer |
| `supervise.send` | Write stored work packet JSON plus newline to FIFO; refresh heartbeat; record `supervisor.packet_delivered` |
| `supervise.stop` | SIGTERM, 5-second grace, SIGKILL if needed; update state; release or mark related leases through existing semantics |
| `supervise.status` | Read daemon row and verify pid identity |
| `supervise.list` | List daemon-owned supervisors by repo/run/session filters |

Daemon restart reattaches by scanning `attached` rows and verifying current
process identity with pid plus `pid_start_time`. Linux uses `/proc/<pid>/stat`
field 22. A future portability helper may use `psutil`, but lack of a verified
start-time token marks the supervisor `lost`; there is no optimistic reattach.

## Sealed-Apply Boundary

`apply.reviewed_patch` is the only apply authority in this slice. Params:

```json
{
  "repository_id": "repo_...",
  "run_id": "run_...",
  "patch_artifact_id": "art_...",
  "reviewer_verdict_id": "verdict_...",
  "expected_patch_sha256": "sha256:...",
  "expected_base_tree": "git-tree"
}
```

Flow:

1. Verify caller has `apply` for the repository.
2. Verify the run is `provenance_mode: sealed_patch`, not paused, not canceled,
   not blocked for apply, and daemon-required.
3. Load the patch or patch-summary artifact and recompute its digest.
4. Verify touched paths are inside the producer job write scope and outside
   forbidden paths.
5. Load reviewer verdict; require accepting verdict over the same run, patch
   digest, base tree, and result tree.
6. Verify current daemon-owned worktree base tree equals the patch base tree.
7. Apply with `git apply --check`, then `git apply --index`, in a
   daemon-owned worktree under the daemon runtime directory.
8. Verify post-apply tree equals the expected result tree.
9. Insert append-only receipt row in daemon DB and sign the canonical payload.
10. Publish a Markdown evidence artifact with `author: striatumd-<instance-id>`.

Refuse-on-mismatch happens before any apply operation when possible. If a crash
occurs during the git worktree operation, recovery marks the apply operation
failed unless a receipt row already exists and verifies cleanly. The operator's
editable checkout is not modified. The daemon does not push, merge, rebase, or
amend history. A local signed commit remains deferred unless a later RFC 0027
phase explicitly enables it.

## Signing Key Custody

The daemon owns one Ed25519 signing key pair. Private key lookup order:

1. OS keyring, keyed by daemon instance id.
2. `0600` runtime fallback file with a degraded-trust warning.

Sealed-mode runs refuse to start if no signing key can be loaded. The public
key and active `signing_key_id` are advertised through `daemon.welcome` and
`daemon.describe`.

Add daemon DB table:

```text
signing_keys(
  key_id PRIMARY KEY,
  public_key,
  algorithm,
  custody,
  created_at,
  activated_at,
  retired_at,
  state
)
```

`daemon.key.rotate` is admin-only. Rotation creates a new active key and
retires the old key for signing while preserving old public keys for receipt
verification. Losing the private key blocks new sealed applies but does not
invalidate old receipts.

The documentation must state the guardrail boundary plainly: the signing key is
local and operator-readable by design. The signature proves this daemon signed
this receipt payload; it is not cryptographic non-repudiation against the local
operator.

## Apply Receipt Format

Append-only daemon DB row:

```text
apply_receipts(
  receipt_id PRIMARY KEY,
  repository_id,
  run_id,
  patch_artifact_id,
  patch_sha256,
  base_tree_hash,
  post_apply_tree_hash,
  reviewer_verdict_id,
  producer_job_id,
  producer_session_id,
  producer_supervisor_id,
  signing_key_id,
  daemon_version,
  substrate_version,
  applied_at,
  receipt_sha256,
  signature,
  audit_id
)
```

Canonical payload:

```json
{
  "receipt_version": "striatum.apply_receipt.v1",
  "repository_id": "repo_...",
  "run_id": "run_...",
  "patch_artifact_id": "art_...",
  "patch_sha256": "sha256:...",
  "base_tree_hash": "git-tree",
  "post_apply_tree_hash": "git-tree",
  "reviewer_verdict_id": "verdict_...",
  "producer_job_id": "job_...",
  "producer_session_id": "sess_...",
  "producer_supervisor_id": "sup_...",
  "signing_key_id": "ed25519:...",
  "daemon_version": "1.22.0",
  "substrate_version": 3,
  "applied_at": "2026-05-11T00:00:00Z"
}
```

Markdown evidence artifact front matter uses
`schema_version: "striatum.apply_receipt.v1"` and
`artifact_kind: "apply_receipt"` once the artifact kind is registered. Until
that schema exists, the receipt Markdown is published as a generic durable
evidence artifact only if the workflow declares that artifact kind. It includes
receipt id, patch digest, base tree, post-apply tree, verdict id, key id,
daemon version, substrate version, timestamp, and signature.

## Workflow Schema Additions

Add workflow-level:

```json
{
  "require_daemon": true,
  "sealed_patch_provider": "daemon"
}
```

`require_daemon: true` refuses direct mode for all mutations in the run.
`provenance_mode: sealed_patch` implies `require_daemon: true`; an explicit
`require_daemon: false` with sealed mode is a validation error.

Add job-level:

```json
{
  "apply_gate": true
}
```

`apply_gate: true` is valid only on a build/handoff job whose expected artifacts
include a patch or patch-summary artifact with a digest. Completion is refused
until an accepting downstream verdict references the same digest and
`apply.reviewed_patch` has produced a receipt.

`sealed_patch_provider: "refuse"` remains as a debugging aid to preserve the V1
refuse-to-start behavior. It is allowed only with explicit operator intent and
must be surfaced in `doctor`.

## Routing Plan

| CLI category | V2 default | Direct fallback |
|---|---|---|
| Read verbs: `status`, `why`, `doctor`, `dashboard`, `evidence` | Daemon RPC | `--no-daemon` allowed |
| Workflow mutations: `workflow validate`, `run prepare`, `run start`, `session register`, `claim-next`, `ack`, `heartbeat`, `publish-artifact`, `verdict`, `submit-review`, `complete`, `release`, `block` | Daemon RPC | Transition-only `--no-daemon` warning unless `require_daemon` |
| Supervision: `supervise start/send/stop/status/list` | Daemon RPC | Transition-only fallback for non-`require_daemon` runs |
| Admin: `repo add/remove/list`, `daemon token *`, `daemon key *`, `daemon migrate`, `daemon shutdown` | Daemon-only | Refuse `--no-daemon` |
| Sealed apply: `apply reviewed-patch`, `apply receipt show/verify` | Daemon RPC | No fallback |

If the daemon is unreachable, read verbs may fall back with a warning. Mutating
verbs refuse rather than silently writing direct state. Work-packet commands
remain stable during the transition, but the dispatcher decides daemon routing
underneath.

## MCP Surface Update

Daemon MCP uses capability tokens as the only access path. There is no global
`--allow-mutations` flag in daemon V2.

`tools/list` filters by the caller token's capabilities. A read-only token sees
read tools and resources. A token without `apply` does not see
`apply.reviewed_patch`. `tools/call` repeats capability enforcement even if a
tool was hidden from `tools/list`; hidden tools are not security. Every MCP
tool call records the same audit and request-log rows as the equivalent RPC.

This slice should not claim RFC 0032's full MCP mutation expansion. It only
establishes that daemon MCP, once mutating tools exist, goes through the same
method registry, token capability checks, audit chain, and request log.

## Test Matrix

| Case | Expected result |
|---|---|
| Missing capability calls `publish_artifact` | RPC refused, exit code 12, audit `denied/capability_missing` |
| Missing `apply` calls `apply.reviewed_patch` | RPC refused, exit code 14, audit `denied/capability_missing` |
| Revoked token races with in-flight request | Authorization decision uses transaction-time token state; request either allowed before revoke audit or denied after revoke audit, never unaudited |
| CLI supports envelope `[2]`, daemon supports `[1]` | `daemon.welcome ok:false`, CLI exits 10, no direct fallback |
| HTTP bind `0.0.0.0:8080` | Daemon refuses startup with exit code 10 |
| Duplicate `request_id` within retention | Refused with `duplicate_request`, original request log preserved |
| Patch bytes digest differs from artifact digest | `apply.reviewed_patch` refuses `patch_digest_mismatch` |
| Verdict references different patch digest | Refuses `verdict_digest_mismatch` |
| Verdict belongs to different run | Refuses `verdict_run_mismatch` |
| Patch base tree differs from daemon-owned worktree | Refuses `base_tree_mismatch` |
| Patch touches forbidden path | Refuses `write_scope_violation` |
| Non-accepting verdict | Refuses `verdict_not_accepting` |
| Missing signing key | `sealed_patch` `run.start` refuses `sealed_key_missing` |
| No repo-scoped `apply` token exists | `sealed_patch` `run.start` refuses `apply_token_missing` |
| `sealed_patch_provider: refuse` | `run.start` refuses with documented debugging reason |
| Supervisor restart with matching pid and start time | Reattaches and keeps `attached` state |
| Supervisor restart with pid reused and start-time mismatch | Marks `lost`, updates pointer, releases or expires lease through normal path |
| FIFO missing on `supervise.send` | Marks supervisor `lost`, records event, refuses delivery |
| MCP `tools/list` with read-only token | Mutating tools omitted |
| MCP `tools/call` directly invokes hidden mutating method | Refused with `capability_missing` and audited |
| Request log retention pruning | Old rows pruned by `expires_at`; audit rows remain append-only |
| Key rotation | Old receipts verify with old public key; new receipts use new key |

## Documentation Deltas

- `docs/SPEC.md`: daemon RPC as default route, repo-local run-state boundary,
  exit code 10 version-skew refusal, daemon-owned supervision, sealed apply
  startability, apply gate, and guardrail-not-non-repudiation threat framing.
- `docs/MCP.md`: daemon MCP routes through RPC/capability checks; tokens are
  mandatory; no global `--allow-mutations`; full mutation expansion remains
  RFC 0032.
- `docs/UBIQUITOUS_LANGUAGE.md`: add RPC envelope, method registry, version
  handshake, capability scope, request log, daemon-owned supervisor,
  supervisor pointer, sealed-apply authority, apply receipt, apply gate, and
  daemon signing key.
- `docs/CLI_REFERENCE.md`: add daemon routing flags, exit codes 10-14,
  `daemon.describe`, `daemon token *`, `daemon key rotate`, and
  `apply reviewed-patch`.
- `docs/HOW_TO_HUMAN.md`: daemon setup for required-daemon workflows,
  version-skew recovery, token creation/revocation, lost supervisor recovery,
  and sealed apply operator workflow.
- RFC 0030 status: update to accepted/implemented after build review.
- RFC 0031 status: update to accepted/implemented after build review.
- `CHANGELOG.md`: daemon V2 RPC, daemon-owned supervision, and sealed-apply
  release note with explicit compatibility warnings.

## Staging Plan

This dogfood should hand implementation a staged plan, not require one giant
merge.

1. RPC foundation: add `daemon_rpc`, socket/HTTP transports, handshake,
   method registry, capability checks, audit mapping, request logging, and CLI
   read routing. Verify `dashboard --all` over RPC.
2. Workflow mutation routing: route mutating workflow verbs through daemon by
   default, keep transition `--no-daemon` warning, add `require_daemon`.
3. Daemon-owned supervision: add daemon `process_supervisors`, repo-local
   pointers, `supervise.*` RPCs, restart reattach, and attestation downgrade on
   stale pointers.
4. Sealed apply: add signing key custody, startup gating, `apply.reviewed_patch`,
   daemon-owned worktree, receipt rows, receipt evidence artifact, and
   adversarial refuse tests.
5. Documentation and status updates: update product docs after behavior lands.

Deferred beyond this dogfood: RFC 0032 cross-repo workflows, full MCP mutation
tool expansion, Go daemon port, bundled Postgres, direct-mode retirement, and
service-manager installers.

## Human-Decision Questions

1. RFC 0030 version-skew window: choose the support window for daemon to CLI
   minor versions. Recommendation: support N-2 minor for read routes and refuse
   N-3 with exit code 10.
2. RFC 0030 request-log query API: keep it internal to `daemon doctor` for
   V2.0. Expose a query command only after operators have concrete incidents
   where doctor output is insufficient.
3. RFC 0031 apply worktree integration: this synthesis chooses a daemon-owned
   worktree and leaves the operator checkout untouched. The operator still
   merges or commits through normal git workflow unless a later RFC authorizes
   daemon-created local commits.
4. RFC 0031 signing key bootstrap: choose whether daemon startup may generate a
   key automatically. Recommendation: yes, but sealed-mode `run.start` must
   refuse if the generated or loaded key cannot be stored in OS keyring or a
   verified `0600` fallback.
5. RFC 0031 leaked apply token response: use `daemon.token.revoke` plus an
   operator decision artifact. Do not claim the system can remediate code
   execution as the daemon OS user.
