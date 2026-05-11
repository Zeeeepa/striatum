# RFC 0030 + RFC 0031 Implementation Design (Claude Code)

author: designer-claude-opus-001

Status: design handoff
Date: 2026-05-11
Target: paired implementation plan for RFC 0030 (daemon RPC server +
version skew protocol) and RFC 0031 (daemon-owned supervision +
sealed-apply boundary).

## 0. How To Read This

RFC 0030 and RFC 0031 are designed together because they share one
trust boundary: the long-lived `striatumd` process. RFC 0030 builds
the boundary (wire protocol, capabilities, audit). RFC 0031 puts the
high-value workloads (supervised lane processes, signed apply
receipts) behind it. Splitting them implementation-wise lands two
half-features; this design ships them as one architectural unit, in
five landable PRs against the RFC 0033 substrate that dogfood-033
already merged.

The RFC 0033 Postgres substrate (`src/striatum/daemon_pg/`) is the
starting line. Repo-local `.striatum/state.sqlite3` keeps its V1
authority (D006/D007) for run/job/session state. The daemon DB
gains the RPC method registry, capability tokens, audit chain,
request log, daemon-owned `process_supervisors`, apply receipts,
and the signing key id pointer.

## 1. Trust Boundaries

Four explicit trust zones. Every interaction below the operator is
mediated by the daemon.

| Zone | Identity | Authority | Defense against |
|---|---|---|---|
| **Operator** | The OS user running `striatumd` and `striatum` | Grants capability tokens via `daemon.token.create`; owns the daemon's runtime directory and signing-key fallback file | Out of scope — see threat model |
| **Daemon process** | `striatumd` running as the operator's OS user | Mutates daemon DB, owns supervised child processes, owns signing key, mints apply receipts | Compromise == workflow-state compromise |
| **RPC client** (CLI / MCP / web / agent) | Holds one capability token bound to (client_id, scope) | Calls daemon methods via envelope-v1 over Unix socket, loopback HTTP, or MCP | Prompt-injected MCP client cannot escalate beyond its token |
| **Supervised lane process** | Child of the daemon, attested by (pid, pid_start_time) per RFC 0026 | Reads packets from FIFO, calls daemon RPC via its own token, never writes daemon DB or `.striatum/state.sqlite3` directly | Lane writing outside `write_scope.allowed_paths` is refused at publish time |

Threat model (RFC 0031 §Threat Model, copied here verbatim so the
implementer does not drift):

- **In scope**: an AI agent acting through documented interfaces who
  tries to exit its write scope, forge a byline, or call
  `apply.reviewed_patch` without a matching reviewer verdict.
  Daemon refuses; audit records the denial.
- **Out of scope**: an operator who reads
  `~/.local/state/striatum/signing_key`, runs `pg_dump` against the
  daemon DB and edits the audit chain, modifies
  `.striatum/state.sqlite3` directly, or has code execution as the
  daemon's OS user.

Sealed mode is an **AI guardrail**, not a cryptographic proof of
authorship and not a defense against the operator themselves.
SPEC.md and README.md updates in this dogfood must state both
sentences explicitly. Any doc string that says "tamper-evident
against the operator" is a bug.

Cannot be claimed even after this dogfood lands:

- **Model-token authorship proof.** D028 stays. No transcript
  capture, no cryptographic link from artifact bytes to model
  output.
- **Independent human decision provenance.** The operator (or a
  prompt-injected operator surrogate) still routes work.
- **Adversarial local-root resistance.** Local-root means the
  daemon's identity is captured.
- **Cross-repository transaction semantics.** Those belong to
  RFC 0032.

## 2. Module Layout

Two new top-level packages, plus surgical edits in existing files.

```text
src/striatum/
  daemon.py                 # existing V1 — keep V1 SQLite registry path; add
                            # RPC bootstrap + reattach hook
  daemon_pg/                # existing RFC 0033 — extend with new migrations
    migrations.py
    sql/
      0001_baseline.sql     # existing
      0002_rpc_method_registry.sql   # NEW
      0003_supervisors_and_receipts.sql  # NEW
  daemon_rpc/               # NEW — RFC 0030
    __init__.py
    envelope.py             # envelope-v1 dataclasses + JSON codec
    server.py               # Unix-socket + loopback-HTTP listener
    framing.py              # NDJSON framing; CBOR/MessagePack hook
    handshake.py            # daemon.hello / daemon.welcome
    registry.py             # method registry, methods_etag computation
    capability.py           # capability vocabulary, constant-time compare,
                            # keyring/0600 fallback, revoke/rotate
    audit.py                # RPC-side audit row writer using daemon_pg.audit
    request_log.py          # request-log rows, retention pruning
    client.py               # in-process Python client used by CLI dispatch
    transport_unix.py
    transport_http.py
  daemon_apply/             # NEW — RFC 0031 sealed apply
    __init__.py
    signing_key.py          # Ed25519 key custody, keyring/0600 fallback,
                            # rotate, public-key export
    patch_io.py             # load patch artifact, verify hash, validate
                            # write scope against patch paths
    receipt.py              # build, store, render receipt + evidence artifact
    worktree.py             # daemon-owned worktree under daemon runtime dir
    apply_service.py        # apply.reviewed_patch implementation
  supervisor.py             # existing — split: keep repo-local fallback path
                            # behind --no-daemon, route through daemon RPC by
                            # default
  daemon_supervisor/        # NEW — RFC 0031 supervision
    __init__.py
    spawn.py                # daemon-side fork + start_new_session=True,
                            # FIFO open as O_RDWR, fd inheritance
    lifecycle.py            # liveness probes, pid_start_time verify,
                            # SIGTERM-grace-SIGKILL on stop
    reattach.py             # daemon-startup reattach loop
    pointer.py              # repo-local process_supervisor_pointers writes
                            # (called by daemon, never by repo CLI)
  process_adapter.py        # existing — keep one-shot RFC 0014 path; no
                            # change to behavior
  mcp.py                    # existing — extend to surface daemon RPC method
                            # registry as MCP tools, filtered by token
                            # capabilities
  service.py                # existing — retire --allow-mutations, route
                            # mutations through daemon RPC client
  cli/
    daemon_cli.py           # existing — extend with `daemon token *`,
                            # `daemon describe`, `daemon migrate
                            # --supervisors`, `daemon key *`
    supervise_cli.py        # existing — route through daemon RPC by default
```

Existing tests stay green; new packages get isolated test modules
(`tests/test_daemon_rpc_*.py`, `tests/test_daemon_apply_*.py`,
`tests/test_daemon_supervisor_*.py`).

## 3. RFC 0030: RPC Layer

### 3.1 Wire framing

Envelope-v1 is one JSON object per request, one per response, on
every transport. The same shape carries `daemon.hello`,
`workflow.validate`, `supervise.start`, `apply.reviewed_patch`, and
every future route — there is no second wire format for "heavyweight"
calls.

```text
request_envelope_v1 := {
  "schema_version": 1,
  "request_id":     "<uuidv7>",
  "method":         "<dotted.route.name>",
  "params":         {<method-specific>},
  "capability_token": "<token-string>",
  "deadline_ms":    0
}

response_envelope_v1 := {
  "schema_version": 1,
  "request_id":     "<same uuidv7>",
  "ok":             true | false,
  "data":           {<method-specific>}    // when ok
  "error":          {"reason": "<vocab>", "message": "<human>",
                     "exit_code": <int>}    // when not ok
  "audit_id":       <bigint>
}
```

Constraints baked into `daemon_rpc/envelope.py`:

- `schema_version` is an integer. Versions are not silently widened;
  every backward-incompatible change increments it and old daemons
  refuse.
- `request_id` is a UUIDv7 string supplied by the client. Daemon
  refuses duplicates within a 60-second window using `rpc_request_log`
  primary key.
- `deadline_ms == 0` means client supplies no bound. Daemon applies
  a method-specific server bound (default 300_000ms).
- `capability_token` lives in the envelope, not the URL or HTTP
  header for the Unix-socket transport. For loopback HTTP transport
  it duplicates into the `Authorization: Bearer` header; the
  envelope value is canonical and the header is a transport
  convenience.

Framing is NDJSON in V2.0. `daemon_rpc/framing.py` exposes a
`Framing` abstract base class so a future CBOR/MessagePack framing
(announced via `daemon.welcome`) drops in without changing
`envelope.py`. Negotiation lives in §3.3.

### 3.2 Transports

`daemon_rpc/transport_unix.py`:

- Binds `${XDG_RUNTIME_DIR}/striatum/daemon.sock` with mode `0600`.
- Refuses to start if the parent directory is not owner-only.
- One connection serves many requests; the daemon accepts
  `daemon.hello` once per connection.
- Uses `asyncio.start_unix_server`. Each connection runs in its own
  task; routes dispatch on a thread pool for blocking DB work.

`daemon_rpc/transport_http.py`:

- Off by default. Opt-in via `daemon start --http 127.0.0.1:<port>`.
- Refuses to bind any non-loopback address at startup. The bind
  check runs before any socket call and the daemon exits with code
  10 (`DaemonUnreachableError`-compatible code) if violated.
- `POST /rpc/v1` with the envelope as the JSON body. Constant-time
  compare on the bearer token mirrors envelope-token compare.

MCP transport is described in §6.

### 3.3 Version handshake

First message on every connection MUST be `daemon.hello`:

```json
{
  "schema_version": 1,
  "method": "daemon.hello",
  "params": {
    "client": {
      "name": "striatum-cli",
      "version": "1.22.1",
      "supported_envelope": [1],
      "supported_framings": ["json"]
    }
  },
  "request_id": "0190..."
}
```

Daemon answers with `daemon.welcome`:

```json
{
  "request_id": "0190...",
  "ok": true,
  "data": {
    "daemon_version": "1.22.1",
    "envelope": 1,
    "framing": "json",
    "substrate": "pg-16",
    "substrate_schema": 3,
    "methods_etag": "sha256:abc...",
    "sealed_apply": {
      "supported": true,
      "signing_key_id": "ed25519:<fingerprint>",
      "public_key": "<base64>"
    }
  }
}
```

Refuse / downgrade rules (`daemon_rpc/handshake.py`):

| Condition | Daemon action | CLI behavior |
|---|---|---|
| Client `supported_envelope` does not include the daemon's current `envelope` | Reply `ok: false`, `error.reason = version_incompatible`, `exit_code = 10` | CLI exits 10; message says `upgrade daemon or downgrade CLI` |
| Client framing not in daemon set | Pick highest mutually supported framing; otherwise same as above | — |
| Daemon older than CLI by minor version | Daemon publishes only methods it knows; `methods_etag` reflects narrowed registry | CLI refuses to call missing methods locally; never falls back to direct repo-local mode |
| `methods_etag` matches client cache | Daemon skips full registry payload | CLI re-uses cached registry under `${XDG_CACHE_HOME}/striatum/daemon/methods-<etag>.json` |
| `sealed_apply.supported == false` | — | `run start` for a `sealed_patch` workflow refuses with code 14 |

No silent downgrade. `--no-daemon` is honored only for verbs in the
transitional set (§3.7); a daemon-version refusal never becomes a
direct-mode fallback.

### 3.4 Capability vocabulary and token lifecycle

V2 capabilities, fixed:

```text
read         introspection (status, why, doctor, dashboard, evidence)
claim        claim work packets
write        ordinary mutating verbs (ack, publish-artifact, complete,
             release, block, heartbeat, session.register)
review       record verdicts, submit reviews
apply        sealed-apply authority (RFC 0031 apply.reviewed_patch)
admin        repo add/remove, daemon token *, daemon shutdown,
             recovery cancel_job, key rotate
recovery     run recovery sweeps and resume blockers
```

Tokens carry the union of capabilities; the daemon checks
membership.

The RFC 0033 baseline migration already accepts these seven values
in `client_capabilities.capability`. Migration 0002 in this dogfood
adds the method registry and binding table; the capability values
themselves do not move.

Token issuance (`daemon_rpc/capability.py`):

```text
daemon.token.create      admin    creates a (client_id, token_id, secret) tuple
daemon.token.list        admin    returns tokens (token_id, scope, caps, expiry)
daemon.token.revoke      admin    sets revoked_at on the client row
daemon.token.rotate      admin    same client_id, new (token_id, secret),
                                  old marked revoked atomically
```

CLI surface mirrors the methods:

```bash
striatum daemon token create --client-kind cli --capability admin --capability read
striatum daemon token list --json
striatum daemon token revoke <token_id> --reason "rotated 2026-05-12"
striatum daemon token rotate <token_id>
```

Token shape `tok_id.tok_secret` (`tok_` short prefix avoids
confusion with V1 `dtok_` daemon-registry tokens; cutover code maps
V1 `dtok_` to V2 `tok_` during migration). Scope is one of:

- `daemon` — repo_id NULL in `client_capabilities`; the token covers
  every active repo for that capability.
- `repo:<repository_id>` — `repository_id` set; per-repo capability
  evaluated by the existing `_authorize` helper rewritten to query
  `daemon_pg.client_capabilities` instead of the V1 SQLite registry.

Storage:

- **Server side**: SHA-256 `token_hash` plus `token_salt` per row.
  The secret never lands on disk in cleartext. Constant-time
  compare via `hmac.compare_digest` (already present in
  `daemon.py:_authorize`; ported into
  `daemon_rpc/capability.py:verify_token`).
- **Client side**: OS keyring (`keyring` Python library, optional
  install extra `striatum-orchestrator[keyring]`). If keyring is
  unavailable, fallback is `${XDG_RUNTIME_DIR}/striatum/tokens/<token_id>`
  mode `0600`. The fallback path emits a doctor record
  `daemon_token_fallback_unprotected` if the file mode drifts above
  `0600`. Both paths refuse to write if the parent directory is not
  owner-only.

Refusal vocabulary (`daemon_rpc/envelope.py:ErrorReason`):

```text
capability_missing      token does not carry required capability
capability_expired      capability row has expires_at in the past
token_missing           envelope had empty or absent capability_token
token_malformed         token_id.secret split failed
token_invalid           token_id unknown or secret mismatch (constant-time)
token_revoked           clients.revoked_at is set
token_expired           clients.expires_at is past
repo_not_registered     repo_id does not resolve to active row
method_unknown          method not in registry
version_incompatible    envelope schema or framing mismatch
deadline_exceeded       method exceeded deadline_ms or server bound
duplicate_request       request_id seen within 60-second window
```

Exit codes (`docs/CLI_REFERENCE.md` update):

```text
10  DaemonUnreachableError + version_incompatible + http_bind_non_loopback
11  DaemonAuthError (token_*)
12  DaemonCapabilityError (capability_*)
13  DaemonRegistryError (repo_not_registered, registry IO)
14  SealedApplyRefused (apply gate refused; see §4)
15  SupervisorRefused (supervisor invariant violated; see §5)
```

The audit row records the denial vocabulary verbatim. Refused
requests still write a `decision = 'denied'` row before the
connection closes.

### 3.5 Method registry

Bound to capabilities at the table level. Schema lives in
migration 0002:

```sql
CREATE TABLE striatumd.rpc_methods (
  method            text PRIMARY KEY,
  capability        text NOT NULL CHECK (capability IN (
                      'read','write','review','claim','apply','admin','recovery'
                    )),
  scope_kind        text NOT NULL CHECK (scope_kind IN ('daemon','repo')),
  params_schema_id  text NOT NULL,
  daemon_min_version text NOT NULL,
  description       text NOT NULL
);
```

A small seed insert (also part of migration 0002) populates the
registry from a single Python-side source of truth
(`daemon_rpc/registry.py:METHOD_REGISTRY`). The daemon at boot
asserts that every Python-registered method has a row in
`rpc_methods` and refuses to start if they diverge — startup self-
check, not a runtime cost.

`methods_etag` is `sha256(canonical-json(METHOD_REGISTRY))`. CLI
caches against the etag (§3.3).

`daemon.describe` is `read`-scoped (recommended in RFC 0030 OQ;
this design accepts the recommendation). It returns method names,
capabilities, params schemas, and `daemon_min_version`. MCP
`tools/list` reuses the same registry filtered by capability
(§6).

Initial route table (only the routes new to this dogfood; existing
V1 read routes keep their command strings):

```text
# RFC 0030 admin / handshake
daemon.hello              read       daemon
daemon.welcome            read       daemon     (response only)
daemon.describe           read       daemon
daemon.token.create       admin      daemon
daemon.token.list         admin      daemon
daemon.token.revoke       admin      daemon
daemon.token.rotate       admin      daemon
daemon.shutdown           admin      daemon
daemon.key.rotate         admin      daemon
daemon.audit.tail         admin      daemon

# RFC 0030 read-routed verbs (replace V1 direct-SQLite reads)
status                    read       repo
why                       read       repo
doctor                    read       repo
dashboard                 read       repo
dashboard.all             read       daemon
evidence.export           read       repo

# RFC 0030 mutating workflow verbs
workflow.validate         write      repo
run.prepare               write      repo
run.start                 write      repo
session.register          write      repo
claim_next                claim      repo
ack                       write      repo
heartbeat                 write      repo
publish_artifact          write      repo
verdict                   review     repo
submit_review             review     repo
complete                  write      repo
release                   write      repo
block                     write      repo

# RFC 0031 supervision
supervise.start           write      repo
supervise.send            write      repo
supervise.stop            write      repo
supervise.status          read       repo
supervise.list            read       repo
supervise.reattach_status read       daemon

# RFC 0031 sealed apply
apply.reviewed_patch      apply      repo
apply.receipt.show        read       repo
apply.receipt.verify      read       repo
```

Routes that already exist in V1 keep their command-string identity
for audit continuity; the row in `rpc_methods` simply ratifies the
binding so audit and method registry agree.

### 3.6 Audit and request log

Two distinct tables, one purpose each.

**Audit** (`striatumd.audit_log`, schema already from RFC 0033):

- One row per authorization decision (allowed or denied).
- Body: never present. `params_sha256` is canonical-JSON hash of
  the envelope params (stable canonicalization in
  `daemon_pg.audit.canonical_params`).
- Append-only enforced by Postgres trigger + role privilege
  (RFC 0033 §5).
- Hash chain unchanged from RFC 0033 V1: row_hash = sha256(
  previous_hash || canonical-payload), preserved across
  segments.
- `daemon_version` and `schema_version` columns from the RFC 0033
  baseline ensure audit rows remain interpretable across daemon
  upgrades.

**Request log** (`striatumd.rpc_request_log`, schema already from
RFC 0033):

- One row per RPC call (regardless of allowed/denied).
- Stores `request_id` (primary key — also the deduplication
  surface), `client_id`, `repository_id`, `method`, `params_hash`,
  `response_hash`, `decision`, `audit_id` (FK back to audit_log),
  `envelope_version`, `daemon_version`, `schema_version`,
  `started_at`, `finished_at`, `expires_at`.
- Default retention: 7 days, capped at 100k rows; whichever is
  hit first triggers pruning by `expires_at`.
- `daemon doctor` reports request-log size, oldest row, and
  rotation state. No operator query API in V2.0 (RFC 0030 OQ
  recommendation; this design accepts).

### 3.7 Routing CLI → daemon

`daemon_rpc/client.py` is an in-process Python client that the
existing CLI dispatcher delegates to. It is **not** a separate
client binary; the CLI parser stays untouched.

Dispatch precedence in `striatum/cli/dispatch.py`:

```text
1. If --no-daemon and verb is in DIRECT_ALLOWED:    use V1 repo-local path.
2. If --no-daemon and verb is in DIRECT_FORBIDDEN:  refuse with code 12.
3. If daemon reachable (socket present, hello succeeds):
       client.call(method, params, token=read_token())
4. Otherwise:
       a. read verbs: fall back to repo-local with one-line warning.
       b. mutating verbs: refuse with code 10.
       c. admin verbs: refuse with code 10.
```

`DIRECT_ALLOWED` includes V1 read verbs and (for the V2 transition
window only) the mutating workflow verbs the work-packet
`commands` block emits today. `DIRECT_FORBIDDEN` is admin
(repo.*, daemon.token.*, daemon.shutdown, daemon.key.*) and
sealed-apply.

`require_daemon: true` is a new workflow-level field accepted by
the validator. When set, the dispatcher refuses every mutating
verb for that run in direct mode regardless of CLI flags.

### 3.8 Test infrastructure

- `tests/daemon_rpc_harness.py` spins a daemon process bound to a
  per-test runtime directory and per-test Postgres database (the
  existing `tests/pg_harness.py` from dogfood-033).
- A `--daemon-url unix:<path>` CLI plumbing override lets CLI
  tests dispatch into the per-test daemon via the in-process
  client.
- Version-skew tests inject `supported_envelope: [99]` and
  assert the refuse path emits exit code 10 and a denied audit
  row.
- Method-registry-drift test asserts startup refuses when the
  Python registry and the SQL `rpc_methods` table disagree.

## 4. RFC 0031: Sealed Apply

### 4.1 Signing key custody

`daemon_apply/signing_key.py` owns one Ed25519 keypair. Public
key is published via `daemon.welcome.data.sealed_apply.public_key`
and `daemon describe`.

Custody order at daemon start:

```python
def load_signing_key():
    # 1. OS keyring under service="striatum-daemon", username=<instance_id>
    if keyring_available():
        secret = keyring.get_password("striatum-daemon", instance_id)
        if secret is not None:
            return Ed25519PrivateKey.from_seed(unhex(secret))
    # 2. Fallback file
    path = runtime_dir() / "signing_key"
    if path.exists():
        mode = path.stat().st_mode & 0o777
        if mode & 0o077:
            raise DegradedTrust("signing key fallback file not 0600")
        return Ed25519PrivateKey.from_seed(path.read_bytes())
    # 3. Bootstrap (operator-visible warning)
    seed = secrets.token_bytes(32)
    if keyring_available():
        keyring.set_password("striatum-daemon", instance_id, seed.hex())
    else:
        path.write_bytes(seed); os.chmod(path, 0o600)
        log_warning("signing key on disk in 0600 fallback")
    return Ed25519PrivateKey.from_seed(seed)
```

A sealed-mode `run start` refuses if the daemon could not load a
signing key. The error path is exit code 14
(`SealedApplyRefused`) with reason `signing_key_unavailable`.

**Provenance limit (must appear in SPEC.md and README.md
verbatim):** The signing key is operator-readable by design. An
apply receipt signature proves "this daemon, on this machine,
signed this receipt at this time"; it does not prove "the
operator did not write the patch bytes themselves and then make
the daemon sign over them." That is what sealed mode does not
claim.

`daemon.key.rotate` is admin-only. Old keypair is retained in
`striatumd.signing_keys` (new table in migration 0003) so
historical receipts remain verifiable. Receipts carry the key
fingerprint, not the active key id; verification looks up by
fingerprint.

### 4.2 Patch artifacts (existing RFC 0027 surface)

This dogfood does **not** redesign patch capture. RFC 0027's V1
implementation already lands `kind: patch_summary` artifacts with
a `patch_digest`, `base_tree`, `result_tree`, `paths`, and
producer (job, session, supervisor). The daemon-apply path
consumes those rows; it does not re-capture.

`daemon_apply/patch_io.py` is read-only against patch artifacts:

- Loads patch bytes from the repo-local artifacts directory
  (path is recorded in the artifact row).
- Recomputes SHA-256 over the canonical patch bytes; refuses
  with `patch_digest_mismatch` if the recomputation disagrees
  with the artifact row.
- Validates that every touched path is within the producer job's
  `write_scope.allowed_paths` and outside `forbidden_paths`; the
  refusal here is `write_scope_violation` even though scope was
  already checked at capture — defense-in-depth.

### 4.3 Apply receipt

Stored in two places. The daemon DB row is the integrity record;
the Markdown artifact is the operator-visible evidence.

Migration 0003 (`daemon_pg/sql/0003_supervisors_and_receipts.sql`)
adds:

```sql
CREATE TABLE striatumd.apply_receipts (
  receipt_id          text PRIMARY KEY,
  receipt_version     integer NOT NULL,         -- starts at 1
  run_id              text NOT NULL,
  repository_id       text NOT NULL REFERENCES striatumd.repositories(repository_id),
  cross_repo_run_id   text,                     -- NULL in V2; RFC 0032 hook
  patch_artifact_id   text NOT NULL,
  patch_digest        text NOT NULL,
  base_tree           text NOT NULL,
  post_apply_tree     text NOT NULL,
  reviewer_verdict_id text NOT NULL,
  reviewer_byline     text NOT NULL,            -- RFC 0026 attested or operator
  producer_job_id     text NOT NULL,
  producer_session_id text NOT NULL,
  producer_supervisor_id text,                  -- NULL when no supervisor
  signing_key_id      text NOT NULL REFERENCES striatumd.signing_keys(key_id),
  signature           text NOT NULL,            -- base64(Ed25519 sig over canonical body)
  daemon_version      text NOT NULL,
  substrate_version   integer NOT NULL,
  applied_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_apply_receipts_run ON striatumd.apply_receipts(run_id);
CREATE INDEX idx_apply_receipts_repo ON striatumd.apply_receipts(repository_id, applied_at DESC);

CREATE TABLE striatumd.signing_keys (
  key_id            text PRIMARY KEY,           -- "ed25519:<fingerprint>"
  public_key        text NOT NULL,              -- base64
  created_at        timestamptz NOT NULL DEFAULT now(),
  rotated_out_at    timestamptz,
  rotated_reason    text
);
```

Markdown evidence shape (written into the run's evidence path with
daemon byline `author: striatumd-<instance_id>` — RFC 0026
attestation rules still apply, the daemon is its own attested
identity):

```markdown
---
schema_version: "striatum.apply_receipt.v1"
artifact_kind: "apply_receipt"
receipt_id: "rcpt_<hex>"
patch_artifact_id: "art_<hex>"
patch_digest: "sha256:..."
signing_key_id: "ed25519:<fingerprint>"
signature: "<base64>"
---

# Apply Receipt rcpt_<hex>

author: striatumd-<instance_id>

This receipt records that the striatum daemon applied patch
artifact `art_<hex>` against repository `<repo_id>` after
reviewer verdict `vrd_<hex>` accepted it. The signature is local
evidence and is not non-repudiation against a malicious local
operator (see RFC 0031 §Threat Model).
```

Verification (`apply.receipt.verify`):

1. Recompute the canonical body hash.
2. Look up `signing_keys.public_key` by `signing_key_id`.
3. Verify Ed25519 signature.
4. Verify `patch_digest` against the recomputed patch bytes (if
   the patch artifact still exists).
5. Verify `base_tree` and `post_apply_tree` are reachable from the
   target repo's git history.
6. Verify `reviewer_verdict_id` exists and references the same
   `patch_digest`.

Any mismatch returns `ok: false` with the documented reason. The
verification never claims more than "the daemon signed this and the
referenced objects are still consistent"; SPEC.md update says so.

### 4.4 Apply gate (workflow opt-in)

`apply_gate: true` is a new workflow-level field on build/handoff
jobs. The validator accepts it on any job whose `expected_artifacts`
includes a `kind: patch_summary` entry; rejects it otherwise. The
gate refuses `complete` on the job unless:

1. The job's `expected_artifacts` includes a published
   `patch_summary` with a `patch_digest`.
2. A downstream reviewer verdict exists that references the same
   `patch_digest`.
3. A successful `apply.reviewed_patch` call exists for the
   `(run_id, patch_artifact_id)` pair — checked by joining
   `apply_receipts` against the local run row.

The refusal pathway returns exit code 14 with reasons:

```text
apply_gate_missing_patch_summary
apply_gate_no_reviewer_verdict
apply_gate_verdict_digest_mismatch
apply_gate_not_applied
```

The gate is opt-in; V1-shaped workflows without `apply_gate`
complete via the existing `complete` semantics unchanged.

### 4.5 `apply.reviewed_patch` flow

Single RPC, capability `apply`, scope `repo`. Params:

```json
{
  "run_id": "run_<hex>",
  "repository_id": "<repo>",
  "patch_artifact_id": "art_<hex>",
  "reviewer_verdict_id": "vrd_<hex>"
}
```

`daemon_apply/apply_service.py:apply_reviewed_patch`:

```python
def apply_reviewed_patch(repo, params, auth):
    # 0. Workflow precondition
    require workflow.provenance_mode == "sealed_patch"     or refuse(14, "sealed_apply_disabled")

    # 1. Load patch
    patch = patch_io.load_artifact(repo, params.patch_artifact_id)
    if sha256(patch.bytes) != patch.row.patch_digest:
        refuse(14, "patch_digest_mismatch")

    # 2. Load verdict
    verdict = repo.find_verdict(params.reviewer_verdict_id)
    require verdict.run_id == params.run_id                or refuse(14, "verdict_run_mismatch")
    require verdict.outcome in ("accept", "accept_with_findings")
                                                            or refuse(14, "verdict_not_accepting")
    require verdict.patch_digest_hash == patch.row.patch_digest
                                                            or refuse(14, "verdict_digest_mismatch")

    # 3. Verify base tree
    target_head = git.head_tree(daemon_worktree(repo))
    require target_head == patch.row.base_tree              or refuse(14, "base_tree_drift")

    # 4. Apply to daemon-owned worktree (NOT operator worktree)
    worktree.apply_patch(daemon_worktree(repo), patch.bytes)
    post_tree = git.head_tree(daemon_worktree(repo))
    require post_tree == patch.row.result_tree              or refuse(14, "result_tree_mismatch")

    # 5. Mint receipt BEFORE writing operator-visible Markdown so a crash
    #    during step 6 produces missing-evidence + clean worktree, not
    #    half-state.
    receipt = receipt.build(run_id=params.run_id, repository_id=repo.id,
                            patch_artifact_id=params.patch_artifact_id,
                            patch_digest=patch.row.patch_digest,
                            base_tree=patch.row.base_tree,
                            post_apply_tree=post_tree,
                            reviewer_verdict_id=verdict.id,
                            reviewer_byline=verdict.byline,
                            producer_job_id=patch.row.producer_job_id,
                            producer_session_id=patch.row.producer_session_id,
                            producer_supervisor_id=patch.row.producer_supervisor_id,
                            signing_key_id=signing_key.id,
                            daemon_version=DAEMON_VERSION,
                            substrate_version=SUBSTRATE_VERSION)
    receipt.signature = signing_key.sign(receipt.canonical_body())
    receipt.store_in_daemon_db()           # transactional insert

    # 6. Write Markdown evidence artifact via the existing
    #    publish_artifact path (daemon-attested byline).
    receipt.publish_evidence_artifact(repo)

    return {"receipt_id": receipt.id, "post_apply_tree": post_tree}
```

The daemon-owned worktree is a private bare-clone scratch under
`<runtime_dir>/worktrees/<repo_id>/`. Operators are not expected to
read or modify it; the operator's editable checkout remains
untouched. The daemon does **not** push, commit-amend, or rewrite
git history — RFC 0027 invariant preserved. The patch is applied
with `git apply --check` then `git apply --index`, then the daemon
creates one local signed commit if and only if `sealed_patch_commit:
true` is set on the workflow (RFC 0027 phase 6); otherwise the
worktree is left with the patch applied and the patch bytes are the
durable form.

### 4.6 Sealed-mode `run start` preconditions

`run.start` for a workflow declaring `provenance_mode: sealed_patch`
refuses unless:

```text
- daemon RPC connection established (no --no-daemon)
- daemon.welcome.data.sealed_apply.supported == true
- signing key loadable at daemon start
- at least one capability token bound to (repository_id, apply) exists
- require_daemon: true is implicit (the validator rewrites the workflow
  with a documented warning if combined with require_daemon: false)
```

Refusal exit code 14, reasons matching the precondition that
failed.

## 5. RFC 0031: Daemon-Owned Supervision

### 5.1 Schema migration (V1 → V2)

Migration 0003 also creates the daemon-side `process_supervisors`
table; repo-local SQLite migration v10 creates the pointer table.

```sql
-- daemon DB
CREATE TABLE striatumd.process_supervisors (
  supervisor_id     text PRIMARY KEY,
  repository_id     text NOT NULL REFERENCES striatumd.repositories(repository_id),
  run_id            text NOT NULL,
  session_id        text NOT NULL,
  adapter           text NOT NULL,
  command_json      jsonb NOT NULL,
  cwd               text NOT NULL,
  scratch_path      text NOT NULL,
  stdin_pipe_path   text NOT NULL,
  pid               integer,
  pid_start_time    text,                      -- RFC 0026 token
  state             text NOT NULL CHECK (state IN
                      ('starting','attached','detached','stopped','lost')),
  started_at        timestamptz NOT NULL,
  attached_at       timestamptz,
  heartbeat_at      timestamptz,
  ended_at          timestamptz,
  stop_reason       text
);
CREATE UNIQUE INDEX uq_active_supervisor_per_session
  ON striatumd.process_supervisors(session_id)
  WHERE state IN ('starting','attached','detached');
CREATE INDEX idx_process_supervisors_repo_state
  ON striatumd.process_supervisors(repository_id, state);
```

```sql
-- repo-local migration v10
CREATE TABLE process_supervisor_pointers (
  session_id                 TEXT PRIMARY KEY REFERENCES sessions(session_id),
  supervisor_id              TEXT NOT NULL,
  daemon_substrate_version   INTEGER NOT NULL,
  last_known_state           TEXT NOT NULL,
  last_observed_at           TEXT NOT NULL
);
CREATE INDEX idx_supervisor_pointers_state
  ON process_supervisor_pointers(last_known_state);
```

The repo-local `process_supervisors` table from RFC 0009 stays in
place for historical rows (run-summary reproduction continues to
work). The migration does **not** rewrite existing rows. New
supervisors create only the pointer. Existing dogfood runs whose
supervisors were spawned under V1 are recovered as one-time
"pre-V2 supervisor" rows surfaced by `doctor` so operators can
stop them or let them age out via lease expiry.

Cutover command:

```bash
striatum daemon migrate --supervisors --dry-run
striatum daemon migrate --supervisors
```

The migrate command:

1. Reads all repo-local `process_supervisors` rows where
   `state IN ('starting','attached','detached')`.
2. For each, attempts `pid_start_time` reattach (§5.4).
3. On success, writes the daemon DB row and the repo-local
   pointer in one daemon transaction; the original repo-local
   row is left alone (provenance trail).
4. On reattach failure, marks the original row `lost` (existing
   transition) and emits a `supervisor.lost` event with
   `phase: pre_v2_migration`.

`--supervisors` is additive on top of dogfood-033's
`--from sqlite --to pg` registry migrate. Both can run in either
order; the supervisor migrate is idempotent.

### 5.2 Supervision RPC methods

```text
supervise.start     write     (per repo)
supervise.send      write     (per repo)
supervise.stop      write     (per repo)
supervise.status    read      (per repo)
supervise.list      read      (per repo)
supervise.reattach_status read    daemon
```

`supervise.start` is the major behavior change. Under V1 the CLI
process forked the child; under V2 the daemon forks the child and
becomes its parent. CLI invocations of `striatum supervise start`
become RPC calls; the in-process Python `daemon_rpc/client.py` is
the only mediator.

`daemon_supervisor/spawn.py` runs inside the daemon's event loop on
a dedicated thread (subprocess.Popen is blocking). The child
inherits no daemon-private file descriptors:

```python
def spawn_supervised(packet):
    scratch = daemon_runtime() / "supervisors" / supervisor_id
    scratch.mkdir(parents=True)
    pipe = scratch / "stdin.pipe"
    os.mkfifo(pipe, 0o600)
    # Open O_RDWR in the parent so the child does not see premature EOF.
    # (Same pattern as existing repo-local supervisor.)
    fd = os.open(pipe, os.O_RDWR | os.O_NONBLOCK)
    fcntl.fcntl(fd, fcntl.F_SETFL, fcntl.fcntl(fd, fcntl.F_GETFL) & ~os.O_NONBLOCK)
    child = subprocess.Popen(
        command,
        stdin=fd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        start_new_session=True,
        cwd=cwd,
        env=lane_env,
        close_fds=True,
    )
    pid_start = process_start_time(child.pid)   # /proc/<pid>/stat field 22
    insert daemon row state='starting'
    while not pid_alive_with_start(child.pid, pid_start):
        if elapsed > 5s: refuse(15, "supervisor_failed_to_start")
        time.sleep(0.05)
    update daemon row state='attached', pid, pid_start_time
    write pointer row in repo-local SQLite (via repo-local DB connection
    that the daemon holds for this repo)
    return supervisor_id
```

`supervise.send` writes the packet bytes to the FIFO inside a daemon
transaction (FIFO write + heartbeat_at update + `supervisor.packet_delivered`
event recorded in repo-local events table via the same daemon-held
repo connection). The audit row in `striatumd.audit_log` records the
RPC decision; the repo-local event is the durable workflow trace.

`supervise.stop` SIGTERMs the recorded pid (pid_start_time guard
prevents stopping a pid that has already been reused), waits 5
seconds, then SIGKILL. Transition to `stopped`. The pointer row is
updated by the daemon to `last_known_state = 'stopped'`.

### 5.3 Daemon lifecycle loop

`daemon_supervisor/lifecycle.py` runs as an asyncio task inside the
daemon. Tick interval default 5 seconds (configurable via
`STRIATUM_SUPERVISOR_TICK_SECONDS`).

Per tick:

```text
for row in striatumd.process_supervisors where state in ('attached','starting'):
    if not pid_alive(row.pid):                        -> mark 'lost'; event
    elif process_start_time(row.pid) != row.pid_start_time:  -> mark 'lost'; event
    else:                                              -> heartbeat_at = now()
        emit pointer-row update (repo-local) every 30s, not every tick
```

Stop on `daemon stop`: SIGTERM every `attached` child with 5-second
grace then SIGKILL. The graceful-stop step is best-effort because
the daemon may also be terminated; `start_new_session=True` ensures
children survive a daemon crash but they will be reattached at
restart (§5.4) or aged out via lease expiry.

### 5.4 Reattach across daemon restart

The critical invariant: a daemon restart MUST NOT mark a
still-alive supervised child as `lost`.

`daemon_supervisor/reattach.py` runs once at daemon start, before
the RPC server accepts connections:

```python
def reattach_at_startup():
    rows = daemon_db.select_supervisors_in_state(('attached',))
    for row in rows:
        if not pid_alive(row.pid):
            transition(row, 'lost', reason='daemon_restart_pid_gone')
            continue
        current_start = process_start_time(row.pid)
        if current_start != row.pid_start_time:
            transition(row, 'lost', reason='daemon_restart_pid_reused')
            continue
        # Re-open the FIFO O_RDWR in the new daemon so subsequent
        # supervise.send writes do not see premature EOF.
        fd = os.open(row.stdin_pipe_path, os.O_RDWR | os.O_NONBLOCK)
        fcntl.fcntl(fd, fcntl.F_SETFL,
                    fcntl.fcntl(fd, fcntl.F_GETFL) & ~os.O_NONBLOCK)
        register_fd(supervisor_id=row.id, fd=fd)
        # daemon row stays 'attached'; lifecycle ticks pick it up
```

Reattach failures emit `supervisor.lost` events into the
repo-local event log so `striatum status` and `striatum supervise
list` surface them. `supervise.list` reports an explicit
`reattach_state` field on every row so operators can distinguish
"never lost across this restart" from "transitioned to lost at
restart."

**Required test (RFC 0031 acceptance criterion + this dogfood's
explicit ask):** spawn a daemon, start a supervised child, kill
the daemon (`SIGKILL` so cleanup hooks do not run), confirm child
still alive, spawn a fresh daemon against the same DB and runtime
dir, assert the reattach loop transitions the row through verify
→ still `attached`, assert `supervise.send` to that supervisor
delivers a packet without the child seeing EOF, assert
`supervise.status` reports `liveness: alive` with the matching
`pid_start_time`. Then kill the child outside the daemon, run
another tick, assert the row transitions to `lost` with reason
`pid_gone`.

A second test exercises the negative path: between daemon SIGKILL
and restart, kill the child and start an unrelated process with
the same pid (test injects the pid_start_time via the test
seam). Reattach must mark `lost` with reason
`daemon_restart_pid_reused`.

### 5.5 Compatibility: `--no-daemon` and V1 supervisors

For one minor release window:

- `striatum supervise start --no-daemon` falls back to the V1
  repo-local path (`src/striatum/supervisor.py:supervise_start`).
  The fallback writes only the repo-local `process_supervisors`
  row; no pointer, no daemon DB row.
- `striatum supervise start` (default) routes through the daemon
  RPC. The repo-local row is no longer written; only the pointer.
- `doctor` adds a check `daemon_v2_v1_supervisor_mixed` that
  surfaces when both shapes exist for the same run.
- Workflows declaring `require_daemon: true` refuse `--no-daemon`
  with exit code 12.

V1 dogfood workflows (`docs/dogfood/030`, `031`, `032`, `033`)
do not need re-running. V2 supervisors and V1 supervisors coexist
during the transition; only the daemon owns the V2 rows.

## 6. MCP Behavior

V2 MCP gains tool routes via the RFC 0030 method registry. No
expansion of `tools/list` beyond what RPC routes already exist.
RFC 0032 owns wider MCP mutation policy (cross-repo,
mutation-defaults debate); this design ships the bare
"MCP-as-RPC-transport" surface only.

`src/striatum/mcp.py` changes:

```text
Existing:
  - JSON-RPC over stdio (Content-Length framed or line)
  - tools/list returns a hardcoded list of read-only tools
  - resources/list reads via daemon registry SQLite directly

V2:
  - MCP server opens a daemon RPC client at startup (envelope-v1
    over Unix socket).
  - tools/list returns the daemon method registry filtered by the
    client's token capabilities. Methods are 1:1 with RPC routes:
    "tools" namespace == RPC method name.
  - tools/call wraps the inbound MCP params into envelope-v1
    params, calls the daemon, returns the response.
  - resources/list and resources/read continue to work; they now
    issue the same daemon RPC calls instead of opening the V1
    registry SQLite.
```

Capability gating:

- Every MCP client connection carries one capability token (env
  var `STRIATUM_MCP_TOKEN` or `--token <token>`).
- `tools/list` returns the intersection of the daemon method
  registry with the token's capabilities.
- `tools/call` for a method the token cannot exercise refuses
  with the standard envelope `error.reason: capability_missing`;
  the audit row records the denial.
- There is no `--allow-mutations` flag; that V1 ergonomic is
  removed by V2 (RFC 0032 §8 also says so).

The MCP server itself is read-only on every code path it owns;
mutations always go through the daemon RPC.

## 7. Concrete Touch Points

Per the prompt, the implementer needs a complete file-by-file
plan.

| File / package | Change |
|---|---|
| `src/striatum/daemon.py` | Add `start_rpc_server` hook called from `run_daemon_foreground` after `pg_doctor` succeeds. Keep V1 SQLite registry helpers behind a deprecated-mode flag for read fallback during the transition. Hand off to `daemon_rpc.server.serve_forever` |
| `src/striatum/daemon_pg/migrations.py` | Append migrations 0002 + 0003 to `MIGRATIONS` tuple; bump `LATEST_DAEMON_DB_VERSION = 3`. |
| `src/striatum/daemon_pg/sql/0002_rpc_method_registry.sql` | NEW. Create `striatumd.rpc_methods`. Insert seed rows from the registry. Add columns to `audit_log` if envelope-related fields need annotation (recommended: none — request_id already there). |
| `src/striatum/daemon_pg/sql/0003_supervisors_and_receipts.sql` | NEW. Create `process_supervisors`, `signing_keys`, `apply_receipts`. Add `striatumd.refuse_signing_key_secret` constraint (no `private_key` column ever). |
| `src/striatum/daemon_pg/audit.py` | Extend `append_audit` signature to accept envelope-style fields (`envelope_version`, `daemon_min_version_at_write`). Keep V1 hash format compatibility (RFC 0033 hash_format_version 2 is current). |
| `src/striatum/daemon_pg/connection.py` | Add `connect_with_role(role)` so RPC paths run as the `striatumd_app` role and audit append uses the `striatumd_audit_writer` role; doctor reports both. |
| `src/striatum/daemon_rpc/` | NEW package (see §2). |
| `src/striatum/daemon_apply/` | NEW package (see §2). |
| `src/striatum/daemon_supervisor/` | NEW package (see §2). |
| `src/striatum/supervisor.py` | Keep V1 in place; wrap `supervise_start` with a "routes via daemon RPC when reachable" shim. Module-level deprecation banner. |
| `src/striatum/process_adapter.py` | No change in this dogfood. RFC 0014 one-shot path is independent of daemon-owned supervision. |
| `src/striatum/api.py` | Route every CLI-invocable verb through `daemon_rpc.client.call` when daemon reachable. Direct-mode fallbacks unchanged. |
| `src/striatum/service.py` | Retire `--allow-mutations`. Convert to thin client over `daemon_rpc.client`. Loopback HTTP server uses `daemon_rpc.transport_http` for the actual wire. |
| `src/striatum/mcp.py` | Replace direct registry SQLite reads with `daemon_rpc.client` calls. Token plumbing (`STRIATUM_MCP_TOKEN`). |
| `src/striatum/cli/dispatch.py` | Implement the precedence table from §3.7. Reject mutating verbs in direct mode when `require_daemon` is set on the workflow. |
| `src/striatum/cli/daemon_cli.py` | New subcommands: `daemon token create/list/revoke/rotate`, `daemon describe`, `daemon migrate --supervisors`, `daemon key rotate/export-public`, `daemon audit tail --limit N`. |
| `src/striatum/cli/supervise_cli.py` | Route through daemon RPC by default; honor `--no-daemon` per §5.5. |
| `src/striatum/migrations.py` | Append repo-local migration v10 for `process_supervisor_pointers`. |
| `docs/SPEC.md` | Add §RPC trust boundary, §Sealed apply receipts (with the explicit "not non-repudiation" sentence), §Daemon-owned supervision. |
| `docs/MCP.md` | Update to "MCP is a transport over the daemon RPC method registry; tools are RPC routes filtered by capability." Document `STRIATUM_MCP_TOKEN`. |
| `docs/UBIQUITOUS_LANGUAGE.md` | Add the 12 new terms from RFC 0030 + RFC 0031 Domain Modeling sections. |
| `docs/CLI_REFERENCE.md` | Document exit codes 10–15, every `daemon *` subcommand, every `--no-daemon` semantic. |
| `docs/HOW_TO_HUMAN.md` | First-time setup: `daemon start`, bootstrap admin token, `daemon token create --capability apply` for a sealed-mode run, `daemon migrate --supervisors`. |
| `docs/HOW_TO_AGENT.md` | One section: "Your work packet's commands block now routes through the daemon by default. You do not need to know that; do not hand-edit envelope JSON." |
| `docs/rfcs/0030-...md`, `0031-...md` | Update status from `proposed` to `accepted (V2)` once both ship. |
| `CHANGELOG.md` | Two bullets: RFC 0030 daemon RPC server, RFC 0031 daemon-owned supervision + sealed-apply boundary. |

## 8. Test Matrix

| Scenario | Module | Expected |
|---|---|---|
| Hello mismatch envelope version | `daemon_rpc/handshake` | Exit 10, `version_incompatible` audit |
| Bind non-loopback HTTP | `daemon_rpc/transport_http` | Daemon refuses to start, code 10 |
| Token with `read` only calls `claim_next` | `daemon_rpc/capability` | `capability_missing` denied audit |
| Token revoked mid-session | `daemon_rpc/capability` | Next call: `token_revoked` |
| Token rotated then old token used | `daemon_rpc/capability` | Old token: `token_revoked`; new: allowed |
| Method registry drift (Python vs SQL) | `daemon_rpc/registry` | Daemon refuses to start; clear error |
| `methods_etag` cache hit | `daemon_rpc/client` | No registry payload re-sent |
| `daemon.describe` without `read` | `daemon_rpc/capability` | `capability_missing` |
| Duplicate request_id within 60s | `daemon_rpc/request_log` | `duplicate_request` |
| Sealed-mode `run start` without signing key | `daemon_apply/signing_key` | Exit 14, `signing_key_unavailable` |
| `apply.reviewed_patch` patch hash mismatch | `daemon_apply/patch_io` | Exit 14, `patch_digest_mismatch` |
| Verdict digest != patch digest | `daemon_apply/apply_service` | Exit 14, `verdict_digest_mismatch` |
| Base tree drift | `daemon_apply/apply_service` | Exit 14, `base_tree_drift` |
| Apply happy path | `daemon_apply/apply_service` | Receipt row inserted; Markdown evidence with daemon byline |
| Receipt verification with rotated key | `daemon_apply/receipt` | Old key still verifies historical receipts |
| Apply gate refusing complete without verdict | `daemon_apply/apply_service` | Exit 14, `apply_gate_no_reviewer_verdict` |
| Daemon SIGKILL with one live supervised child | `daemon_supervisor/reattach` | Fresh daemon reattaches, child stays attached |
| Daemon SIGKILL, child dies, pid reused | `daemon_supervisor/reattach` | Reattach marks `lost` with `daemon_restart_pid_reused` |
| `supervise.send` after daemon restart | `daemon_supervisor/spawn` | FIFO write succeeds, packet delivered |
| `supervise.stop` after pid_start drift | `daemon_supervisor/lifecycle` | Refuses to SIGTERM; marks `lost` |
| V1 supervisor pre-migration | `daemon_supervisor/reattach` | Migrate path adopts or marks `lost` honestly |
| MCP `tools/list` with `read`-only token | `mcp` | Returns only `read`-scoped methods |
| MCP `tools/call apply.reviewed_patch` without `apply` | `mcp` | Refused; audit `capability_missing` |
| Cross-repo `apply.reviewed_patch` | `daemon_apply/apply_service` | Refused (V2 scope), pointer to RFC 0032 |

## 9. PR Sequencing

Five PRs, each independently testable and releasable. Branch
`striatum/dogfood-034-rfc-0030-0031-rpc-supervision` is the
integration branch.

1. **PR 1 — RFC 0030 substrate + registry** (migrations 0002,
   `daemon_rpc/registry.py`, capability vocabulary update,
   `daemon_pg/audit.py` extension). No wire server yet; just the
   tables, capability machinery, and the registry. Tests assert
   migration applies forward and method-registry drift detection
   refuses startup.
2. **PR 2 — RFC 0030 wire server** (`daemon_rpc/envelope.py`,
   `transport_unix.py`, `transport_http.py`, `handshake.py`,
   `client.py`). CLI dispatcher routes the read verbs through the
   new client when daemon is reachable. Direct mode still default
   for mutating verbs.
3. **PR 3 — RFC 0030 mutating verb routing** (CLI dispatcher
   change, MCP wrapper rewrite, V2 token CLI subcommands, audit
   chain continuity test across V1→V2 cutover). `--allow-mutations`
   deleted; `service.py` rewritten.
4. **PR 4 — RFC 0031 daemon-owned supervision** (migration 0003
   supervisor side, `daemon_supervisor/` package, pointer table
   migration v10, `daemon migrate --supervisors`, reattach test
   suite). `supervise.*` RPCs land.
5. **PR 5 — RFC 0031 sealed apply** (migration 0003 receipts +
   signing keys, `daemon_apply/` package, `apply.reviewed_patch`
   RPC, apply receipt format, `apply_gate` workflow field, sealed-
   mode `run start` preconditions, SPEC.md + README.md updates
   asserting the non-claims).

Each PR carries its own CHANGELOG entry under `Unreleased`. The
RFC status flip from `proposed` to `accepted (V2)` belongs to
PR 5.

## 10. Risk Register

| Risk | Mitigation |
|---|---|
| Daemon now single point of failure for mutations | V1 direct-mode fallback for read verbs preserved; `--no-daemon` honored during transition for mutating verbs; admin verbs daemon-only by design (the operator can always re-run `daemon start`). |
| Wire-protocol version bumps become production incidents | `daemon.hello` refuses fail-closed with documented exit code; CLI/daemon version-skew matrix tested in CI. |
| Python long-running process GIL / asyncio behavior | Subprocess.Popen and DB writes happen on a thread pool; the event loop only owns sockets and the lifecycle ticker. Profiling/load tests deferred to a follow-up RFC. |
| Operator-readable signing key is misread as cryptographic non-repudiation | SPEC.md + README.md updates in PR 5 carry the explicit "AI guardrail, not non-repudiation" language. The threat model section is copied verbatim into both docs. |
| Receipt format becomes load-bearing for downstream consumers we did not anticipate | `receipt_version` integer in the row; bump to 2 if/when we need to widen. Historical receipts verifiable forever (signing_keys table preserves rotated-out keys). |
| Daemon crash during `apply.reviewed_patch` between worktree write and receipt mint | Receipt is recorded BEFORE the operator-visible Markdown artifact (see §4.5 step 5 vs 6). A crash mid-flow leaves: receipt row present + missing evidence artifact (operator can re-publish via verify path) OR worktree dirty + no receipt (operator clears worktree manually; refuses to apply again until clean). |
| V2 MCP exposes `apply` capability that prompt-injected agents could exercise if granted | Token default carries `read` only; operators must explicitly grant `apply`; RFC 0032 tightens further. |
| V1 supervisor mixed with V2 supervisor during the transition window | `doctor` surfaces; CLI documents `--supervisors` migrate as a one-time step; release notes name a sunset window of one minor release. |

## 11. Open Questions To Resolve In Synthesis

These are questions where the RFCs already publish a
recommendation; I list them so the synthesizer can ratify or
push back, not to re-litigate the RFCs.

- Supported daemon ↔ CLI version-skew window. RFC 0030 OQ
  recommends N-2 minor. I accept that.
- `daemon.describe` capability. RFC 0030 OQ recommends `read`.
  I accept; alternative `none` is a fingerprinting risk.
- Whether `methods_etag` cache crosses processes. RFC 0030 OQ
  recommends `yes`. I accept; cache file under
  `${XDG_CACHE_HOME}/striatum/daemon/`.
- Operator query API for request log. RFC 0030 OQ recommends
  internal to `daemon doctor`. I accept.
- `apply` token leak handling. RFC 0031 OQ recommends
  `daemon.token.revoke` plus operator-recorded decision. I
  accept.
- Daemon-applied worktree vs operator's checkout. RFC 0031 OQ
  recommends daemon-owned worktree. I accept; this design uses
  `<runtime_dir>/worktrees/<repo_id>/`.
- `sealed_patch` + `require_daemon: false` combination. RFC 0031
  OQ recommends validator rewrites with warning. I accept.

## 12. Out-Of-Scope For This Dogfood

Stated explicitly so synthesis can refuse to widen scope:

- Cross-repository workflow schema, cross-repo coordinator,
  `cross_repo_runs` table. → RFC 0032.
- MCP mutation defaults / capability-defaulting beyond
  `tools/list filters by token`. → RFC 0032.
- Wider audit retention/rotation policy. RFC 0033 covers append-
  only; retention window beyond 7 days is a follow-up.
- Bundled / Dockerized Postgres distribution. RFC 0033 deferred.
- Python → Go core port. D084 / future RFC.
- Sealed-mode signed local commit. RFC 0027 phase 6; this design
  ships up to phase 5 (receipts + apply gate) and leaves the
  local-commit pathway behind a workflow flag
  (`sealed_patch_commit: true`) wired but not enabled in any
  dogfood workflow yet.
- Service-manager installation (`systemd`, `launchd`, Windows
  Service). The daemon stays a foreground process started by
  `striatum daemon start` (or `striatumd`); platform integration
  is its own follow-up.
- Windows daemon support generally. The pid_start_time path uses
  `/proc/<pid>/stat`; macOS uses `proc_pidinfo`; Windows needs
  a separate RFC.

## 13. Acceptance Self-Check

This design satisfies the dogfood-034 prompt and the underlying
RFCs as follows.

| Prompt ask | Section |
|---|---|
| Trust boundaries (daemon / RPC clients / lanes / operator) | §1 |
| Capability vocabulary + token lifecycle + revocation | §3.4 |
| Sealed-apply gate semantics including all refuse paths | §4.4, §4.5 |
| Apply receipt format + provenance limits stated explicitly | §4.1, §4.3 |
| Supervisor reattach across daemon restart with tests | §5.4 |
| V1 → V2 supervisor row migration story | §5.1, §5.5 |
| MCP behavior with capability-filtered `tools/list` and no expansion | §6 |
| Concrete touch points in `src/striatum/` plus new modules | §2, §7 |
| Explicit non-claims | §1 (closing list) |

The byline `author: designer-claude-opus-001` appears once, plain,
in the title block (line 3). No bold, italics, heading prefix, or
trailing punctuation, per the prompt's hard constraint.
