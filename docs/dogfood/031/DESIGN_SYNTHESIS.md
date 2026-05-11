---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/031/design/codex/DESIGN.md", "docs/dogfood/031/design/claude_code/DESIGN.md", "docs/dogfood/031/design/gemini/DESIGN.md", "docs/dogfood/031/review/design/devils/REVIEW.md", "docs/dogfood/031/review/design/security/REVIEW.md", "docs/dogfood/031/review/design/threat/REVIEW.md"]
---

# RFC 0028 V1 Daemon Implementation Plan (revision 3)

author: operator

Status: revised synthesis
Date: 2026-05-11
Target: RFC 0028 V1 acceptance-criteria slice

## Revision 3 Response

Revision 3 preserves the revision-2 scope and the round-1 blocker closures:
V1 remains an optional local daemon for multi-repository read visibility,
daemon registry state, metadata-only audit, capability-gated daemon endpoints,
and one daemon-managed recovery sweep loop. It still does not move ordinary
workflow mutation authority into the daemon.

This revision adds concrete answers for the round-2 blockers and tightens the
material non-blocking findings without expanding V1 into daemon-owned
supervision, MCP mutation, sealed apply, hosted service semantics, or operator
tenancy.

| Finding | Concrete answer in revision 3 |
|---|---|
| B1: Hash-chain continuity across audit segment rotation is unspecified. | V1 uses one continuous hash chain across segments. The first row in segment N+1 has `previous_hash` equal to segment N's `last_hash`; segment manifest rows are append-only through daemon APIs; `daemon doctor` walks every retained manifest and retained row range end-to-end. Retention tombstones preserve continuity metadata but explicitly reduce row-level verification for purged segments. |
| B2: `repo remove` cascade semantics for capabilities and audit are unspecified. | `repo remove` is idempotent. It disables the repository record, revokes all live repo-scoped capability rows for that `repository_id`, audits each revocation, cascades scheduler cursors to terminal removed state, preserves audit rows, and never reuses `repository_id`. In-flight requests whose authorization started before revocation may finish, but every later authorization observes revocation. |
| B3: MCP exposure of daemon audit reopens the MCP threat surface. | `striatum://daemon/audit` is removed from V1 MCP. Audit is available only through the admin-gated CLI registry surface in shipped V1; daemon RPC/socket and loopback HTTP admin endpoints are deferred. MCP remains resources-only and excludes cross-repository audit metadata. |
| NB1: Daemon-emitted repo-local event byline. | Daemon recovery events use a fixed daemon byline format, `author: striatumd-<instance-id>`, distinct from operator, role, lane, and model bylines. |
| NB2: Cross-platform packaging gap. | V1 daemon support is Linux and macOS. Windows daemon support is deferred. Linux uses XDG runtime/state locations; macOS uses Keychain and owner-only directories under `~/Library/Caches/striatum/runtime/` and `~/Library/Application Support/striatum/`. CI must cover each claimed branch. |
| NB3: Reserved future capability strings. | Removed. V1 capability vocabulary is only `read` and `admin`; future slices add future strings when earned. |
| NB4: `repo rebind` is referenced but unspecified. | Removed from V1. Operators use `repo remove` then `repo add`; the removal path revokes repo-scoped capabilities and new registration allocates a fresh `repository_id`. |
| NB5: `recovery` capability has no client-facing endpoints. | Removed. The sweep loop uses daemon-internal authority. V1 has no client-facing recovery control endpoints beyond admin lifecycle/configuration. |
| NB6: Crash during sweep cursor semantics. | Scheduler cursors advance only after a sweep completes and repo-local events/audit are durably written. A daemon killed mid-sweep retries that run on restart instead of skipping it. |
| NB7: Health-probe audit summarization. | Every health probe that reaches daemon routing writes a metadata-only audit row with `command="health"` and `repository_id=null`; there is no unspecified summarization policy in V1. |
| NB8: Forced-daemon exit-code classes. | Forced daemon failures use stable classes: 9 version/schema skew, 10 daemon unreachable, 11 token missing/invalid/expired/revoked, 12 capability denied or repo out of scope, 13 registry/protocol/internal daemon error. |

## Accepted Implementation Scope

V1 implements a narrow optional daemon mode:

- a local `striatum daemon start/status/stop` foreground sweep process, with
  service installer support only if explicitly included in the implementation
  slice;
- a daemon registry for repository records, clients, capabilities, audit rows,
  scheduler cursors, and daemon metadata;
- `striatum repo add/list/remove` as daemon administration commands;
- explicit daemon-routed read surfaces for status, doctor, why, and dashboard;
- a global read-only dashboard across registered repositories;
- a daemon MCP resource surface spanning registered repositories but excluding
  daemon audit in V1;
- central capability authorization that default-denies every non-health request
  without an explicit token;
- metadata-only, hash-chained audit rows for every daemon request, including
  denied requests and health probes that reach routing;
- a daemon sweep loop that runs existing recovery policy across active
  registered runs without one `recovery watch` process per run;
- tests proving daemon restart with a pre-existing registry and at least one
  registered repo-local state store.

Revision-round-3 implementation note: the shipped V1 does not include a daemon
RPC server. CLI and MCP clients open the owner-only registry SQLite directly
under token/capability checks, and `striatumd` is functionally a foreground
sweep loop plus lifecycle marker. The socket/HTTP/protocol-routing design below
is retained as the accepted future direction, but it is deferred to a follow-up
RFC unless a section explicitly says "implemented in V1".

V1 does not introduce operator tenancy. The operator trust boundary is the
current OS user plus local file permissions. The modeled identities are
repository records and client tokens.

The acceptance criteria map to this implementation:

| RFC 0028 acceptance criterion | V1 commitment |
|---|---|
| Register at least two target repositories. | `repo add` records canonical roots plus repo-local identity, schema version, and state. Registration refuses path re-occupation with a different identity until the old repository is removed; re-add allocates a fresh `repository_id`. |
| Global read-only dashboard lists runs, blockers, claimable jobs, and stale leases. | `dashboard --all` fans out over repo-local state, degrades individual bad repos, and never treats registry copies as live run truth. |
| Daemon exposes MCP resource list spanning multiple repositories. | V1 MCP exposes resources only. `tools/list` is empty, daemon audit is not exposed over MCP, and there is no raw daemon invoke endpoint. |
| CLI can call daemon for read-only status, doctor, dashboard operations. | `--daemon` or `STRIATUM_DAEMON=1` routes supported read commands to the daemon. Direct mode never auto-probes the daemon. |
| Direct CLI mode still works. | Without explicit daemon mode, existing commands keep the repo-local path and current semantics. |
| Daemon records every client request. | Audit rows are appended for allowed, denied, malformed, expired, revoked, out-of-scope, unknown, and health requests that reach daemon routing. |
| Daemon refuses mutation tools unless explicitly permitted. | V1 has no generic mutation surface. Admin repo/capability commands require `admin`; sweep work uses daemon-internal authority. Unknown or absent endpoints are not promoted to capability-checkable mutation doors. |
| Daemon can run a recovery sweep across all active runs. | Foreground daemon sweep loops over active registered runs using existing recovery policy. If service-manager support is not shipped, docs call this foreground daemon sweep, not installed resident recovery. |
| Tests cover restart with registry and registered repo-local state. | Integration tests use temp registry/runtime paths and temp initialized repos; restart preserves repo rows, schema metadata, capabilities, audit chain continuity, segment manifests, and scheduler cursor state. |
| Docs state local-first optional mode and provenance limits. | Docs must say daemon mode does not strengthen lane attestation, does not implement sealed apply, and does not create hosted service semantics. |

## Deferred Scope

Cross-repository workflows are deferred. V1 is multi-repository introspection
and recovery scheduling only.

General daemon-backed workflow mutations are deferred. Existing direct CLI
mutations remain authoritative for job claims, publishing, verdicts, session
heartbeats, decision records, run lifecycle, worktrees, and evidence export.

Daemon-owned supervision is deferred. V1 does not create
`daemon_supervisors` or otherwise claim process ownership. Existing repo-local
`process_supervisors` remains the RFC 0026 compatibility source.

MCP mutation tools are deferred. V1 daemon MCP is resources-only.

Sealed-mode apply, signing keys, protected apply services, and containment are
deferred. `sealed_patch` remains unsupported/unstartable until a future
accepted containment design lands.

Remote serving, daemon RPC routing, and hosted semantics are deferred. The V1
foreground process binds an owner-only Unix socket as a lifecycle marker only;
it does not accept RPC requests. Optional HTTP transport and its loopback
refusal rules are future scope.

Full Go or TypeScript rewrite is deferred. V1 stays in Python.

Local multi-user operator tenancy is deferred. V1 is one daemon trust zone per
OS user; tokens identify clients, not humans.

Windows daemon support is deferred. V1 daemon support is Linux and macOS only.

Installed service-manager integration is optional for this implementation
slice. If `daemon install` for at least one platform does not land, product
language must say "foreground daemon sweep" rather than "resident installed
service".

## Supported Platforms

V1 daemon support is claimed only for Linux and macOS.

Linux uses `${XDG_STATE_HOME:-~/.local/state}/striatum/striatumd.sqlite3` for
the registry and `${XDG_RUNTIME_DIR}` for sockets and runtime-file token
fallbacks. The runtime directory must be owner-only. Registry files must be
`0600`; runtime directories must be `0700`; token fallback files must be
`0600`.

macOS uses Keychain Services for token storage. Registry state lives under
`~/Library/Application Support/striatum/`; runtime socket and token fallback
files live under `~/Library/Caches/striatum/runtime/` with owner-only `0700`
directory permissions and `0600` token fallback files. Because this fallback is
not memory-backed and can survive reboot, macOS runtime-file fallback emits a
degraded-trust warning and should use shorter token expiry guidance than
keyring-backed storage.

Windows daemon support, Windows named-pipe transport, and Windows Credential
Manager integration are follow-up work. The direct CLI remains supported on
platforms where it is already supported.

## Registry Storage Decision

Choose Option C: hybrid registry plus per-repository run stores.

`.striatum/state.sqlite3` in each target repository remains authoritative for
runs, jobs, sessions, leases, artifacts, verdicts, blockers, worktrees,
process supervisors, and repo-local events. The daemon registry stores only
global daemon concerns: registered repositories, clients, capability grants,
audit chain and segment manifests, scheduler cursors, daemon lifecycle
metadata, and registry migrations.

This preserves the core repo-local live-state contract but does not preserve
fresh-clone visibility for daemon request audit. Daemon audit is per-machine
daemon evidence unless an operator exports it separately. Run-affecting daemon
actions, especially recovery requeues and degraded sweep marks, must still
write repo-local events so the repository's run timeline remains inspectable
from the repo-local state store.

Initial registry paths use the platform rules above. Tests must override state
and runtime paths.

Initial registry tables:

- `daemon_meta(key, value)`.
- `repositories(repository_id, repo_identity, repo_root, state_db_path,
  display_name, registered_at, removed_at, last_seen_at, last_schema_version,
  state, settings_json)`, with uniqueness on `repo_identity` and on active
  canonical `repo_root`.
- `clients(client_id, client_kind, display_name, token_id, token_hash,
  token_salt, created_at, expires_at, revoked_at, last_used_at)`.
- `client_capabilities(capability_id, client_id, repository_id nullable,
  capability, granted_at, expires_at, revoked_at, revoked_reason)`.
- `audit_log(audit_id integer primary key autoincrement, timestamp,
  client_id nullable, repository_id nullable, command,
  authorization_result, denial_reason nullable, transport, request_id,
  exit_code, payload_sha256, previous_hash, row_hash, segment_id)`.
- `audit_segments(segment_id, opened_at, closed_at, first_audit_id,
  last_audit_id, first_hash, last_hash, previous_segment_id,
  previous_segment_last_hash, next_segment_first_previous_hash,
  retention_state, state)`.
- `scheduler_cursors(repository_id, run_id, cursor_kind, last_sweep_at,
  next_sweep_after, last_result_json, state)`.

Do not create future-only supervisor registry tables in V1.

Registry migrations are separate from repo-local migrations but use the same
discipline: `PRAGMA user_version`, WAL, foreign keys, forward-only migration
list, `BEGIN IMMEDIATE`, and refusal when registry version is newer than the
runner supports.

Existing-state registration plan:

1. `repo add <path>` canonicalizes the repo root and refuses symlink or
   traversal ambiguity.
2. It verifies `.striatum/state.sqlite3` exists or requires explicit `--init`.
3. By default it connects through the existing repo-local path and applies
   pending migrations. With `--no-migrate`, it first checks whether migrations
   would be needed and refuses rather than mutating repo-local state.
4. It reads or creates a repo-local daemon registration UUID. This UUID lives
   in repo-local state, not in files under `.striatum/` outside the database.
5. The registry records the UUID, canonical path, state DB path, schema
   version, and active/missing/disabled state.
6. If a registered path disappears, reads mark it `missing`.
7. If a different repository later occupies the same active path, the identity
   mismatch is refused. V1 has no `repo rebind` command.

Repository removal semantics:

- `repo remove <repository_id-or-path>` is a daemon admin operation.
- Removal is idempotent. Repeating removal of an already removed repository
  returns success with no duplicate revocation rows.
- Removal marks the repository row `removed` and sets `removed_at`; V1 does
  not hard-delete repository rows through daemon APIs.
- Removal revokes every live `client_capabilities` row with that
  `repository_id` by setting `revoked_at` and `revoked_reason="repo_removed"`.
- Removal writes one audit row for the remove command and one audit row per
  revoked capability with `authorization_result="revoked"` and the originating
  `repository_id`.
- In-flight requests whose authorization already completed before revocation
  may finish and are audited with their original authorization result. Any
  request that authorizes after revocation must fail as repo removed or
  capability revoked.
- Scheduler cursors for the repository move to terminal removed state and are
  excluded from future sweep enumeration.
- Audit rows for the removed `repository_id` are preserved. They remain
  append-only daemon evidence and may reference repository ids no longer active
  in `repositories`.
- `daemon doctor` reports the count of removed repository ids still referenced
  by retained audit rows so operators understand those rows no longer join to
  active repositories.
- `repository_id` is never reused. New registration always allocates a fresh
  id, even for the same canonical path after removal.

## Implementation Language Decision

Choose Python for V1.

The intended daemon shape performs registry CRUD, local HTTP/Unix-socket
request handling, read fan-out, audit logging, and recovery scheduling.
Revision-round-3 V1 implements the registry CRUD/read fan-out/audit/recovery
sweep subset without RPC request handling. Existing
workflow validation, repo-local migrations, CLI dispatch, service API, MCP
framing, recovery policy, artifact validation, and lane-attestation helpers
are Python. Reimplementing them in another language for this slice would add
risk at the trust boundary.

The protocol should still be clean enough for a later Go daemon to replace the
Python daemon without changing CLI or MCP clients. TypeScript is not chosen
for V1 because the daemon's core work is local process/state orchestration,
not web UI velocity.

## Phased Plan Aligned To RFC 0028 Section 8

This dogfood lands RFC 0028 section 8 step 1 plus the narrow part of recovery
scheduling needed for the acceptance criterion. Other steps remain follow-ups.

| RFC 0028 step | V1 stance | Testable milestone |
|---|---|---|
| 1. Daemon registry and read-only multi-repo dashboard. | Land now. | Foreground daemon starts, two repos register, global dashboard reports run state, restart preserves registry. |
| 2. Daemon-backed CLI client mode. | Mitigate via registry-backed read mode; daemon RPC deferred. | `--daemon status/doctor/why/dashboard` reports `mode: daemon`; no explicit daemon flag uses direct mode. |
| 3. Recovery scheduling and global doctor. | Land daemon sweep loop and daemon doctor registry checks. | Sweep uses existing policy, emits repo-local events for run-affecting actions, and surfaces degraded repos. |
| 4. Move supervision into daemon. | Deferred. | Later RFC slice defines supervisor schema and reconciles pid/start-time. |
| 5. Daemon-backed MCP. | Land resources only, without daemon audit. | MCP lists/read resources; `tools/list` is empty; daemon audit URI is absent. |
| 6. Mutation capabilities for trusted clients. | Deferred except daemon admin commands. | Later capabilities route through explicit endpoints after authorization and audit. |
| 7. Decide direct SQLite CLI future. | Deferred. | V1 guarantees direct mode fallback as an explicit operator choice, not as silent daemon downgrade. |

## Schema And Migration

Use a new daemon registry package with path helpers, migration registration,
connection setup, file-permission checks, and test override environment
variables. Keep repo-local schema changes minimal: the only acceptable V1
repo-local schema addition is the repository identity needed to prevent
path-based capability transfer. If the existing schema already has a suitable
stable identity, reuse it.

Capability vocabulary:

- `read`
- `admin`

There are no reserved-but-not-exposed capability strings in V1. Future slices
add future capability names when they define concrete endpoints, storage rules,
audit semantics, and tests for those endpoints.

Authorization must classify every endpoint before dispatch. New daemon
endpoints default to absent or denied until explicitly classified.

Token lifecycle:

- Every non-health request requires `token_id` plus a 32-byte or stronger
  URL-safe secret.
- The daemon stores salted HMAC/SHA-256 or stronger password-token hashes and
  uses `hmac.compare_digest` for verification.
- Bootstrap creates one admin token, displays the secret once, and stores the
  secret only through the configured client-secret storage mechanism.
- Tokens support expiry, revocation, rotation, display names, and repository
  scope.
- Errors and audit rows include token id/client id only when safe; never token
  secrets or salts.
- OS keyring is preferred for CLI and MCP secret storage. Linux uses the
  configured keyring provider; macOS uses Keychain Services.
- If keyring storage is unavailable, the fallback is an owner-only runtime file
  under the platform runtime directory with mode `0600`, a short expiry
  recommendation, and a degraded-trust warning.
- Config files may store daemon socket path and token id. They must not store
  plaintext secrets. Secrets must not be passed in argv or recommended through
  environment variables.

Audit integrity and retention:

- Daemon APIs have no endpoint to update or delete audit rows or segment
  manifests.
- Every audit row includes `previous_hash` and `row_hash`, where `row_hash`
  hashes normalized row fields plus the previous hash.
- The audit chain is continuous across segment rotation. If segment N closes
  with `last_hash=H`, the first row in segment N+1 must have
  `previous_hash=H`. The segment N+1 manifest records
  `previous_segment_last_hash=H`.
- `audit_segments.last_hash[N]` must equal the first row
  `previous_hash` of segment N+1. `daemon doctor` verifies this by walking
  segment manifests and retained rows, not only the active segment.
- `audit_segments` rows are append-only through daemon APIs. Closing a segment
  fills its closing metadata; no daemon API may rewrite an earlier segment's
  identity, row range, or hash anchors.
- Production retention/rotation is deferred from the shipped V1. The design
  target remains 90 days queryable in the active registry, with rotation that
  closes old segments into manifest rows and optional compressed sidecar
  exports.
- Deleting row data from a rotated segment is an explicit admin retention
  action. It leaves a manifest/tombstone with the segment id, row id range,
  first hash, last hash, previous segment link, next segment link when known,
  retention action timestamp, and `retention_state="purged"`.
- Tombstoned segments preserve cross-segment continuity evidence but remove
  row-level verification inside the purged segment. `daemon doctor` reports
  this as a retention gap, not as proof that the purged rows were intact.
- `daemon doctor` verifies chain continuity, segment continuity, registry file
  permissions, socket directory permissions, token fallback permissions, newer
  `user_version` refusal, malformed rows, missing active segment, missing
  manifests, and rotated segment metadata.
- The audit log records daemon request metadata. It is not tamper-resistant
  against an attacker with local filesystem write access.

## CLI Client And Direct Mode

V1 daemon mode is explicit and deterministic:

- `--daemon` or `STRIATUM_DAEMON=1` requires a live compatible daemon, valid
  token, and sufficient capability. Missing daemon, protocol skew, missing
  token, expired/revoked token, and denied capability fail clearly and never
  fall back to direct mode.
- `--no-daemon` forces direct mode.
- Without explicit daemon mode, existing single-repo commands use direct mode
  and do not probe the daemon.
- `dashboard --all`, `daemon status`, `daemon stop`, `repo add/list/remove`,
  and capability administration are daemon-only and have no direct fallback.

Forced-daemon failure exit-code classes:

| Code | Class |
|---|---|
| 9 | Registry schema skew or repo-local schema skew in V1; CLI/daemon protocol skew is deferred with the RPC server. |
| 10 | Foreground process lifecycle conflicts in V1; daemon-unreachable/client-transport failures are deferred with the RPC server. |
| 11 | Token missing, malformed, invalid, expired, or revoked. |
| 12 | Capability denied, repository out of scope, or repository removed. |
| 13 | Registry corruption, protocol framing error, malformed daemon response, or internal daemon error. |

Daemon JSON responses include `mode: "daemon"`, daemon version, protocol
version, `repository_id` when scoped, and repo-local schema version where
applicable. Direct-mode JSON remains stable except for documented additive
fields.

## MCP Surface And Capability Defaults

V1 daemon MCP exposes resources only:

- `striatum://daemon/repos`
- `striatum://daemon/dashboard`
- `striatum://repo/<repository_id>/status`
- `striatum://repo/<repository_id>/doctor`
- `striatum://repo/<repository_id>/runs`
- `striatum://repo/<repository_id>/run/<run_id>`
- `striatum://repo/<repository_id>/run/<run_id>/why?id=<id>`
- `striatum://repo/<repository_id>/blockers`
- `striatum://repo/<repository_id>/stale-leases`

`striatum://daemon/audit` is not a V1 MCP resource. MCP host processes that
need audit visibility must ask the operator to run `striatum daemon audit` or
use a non-MCP daemon admin surface. This avoids exposing cross-repository audit
metadata to prompt-injectable MCP clients.

`tools/list` is empty in V1. There is no daemon MCP mutation endpoint and no
raw daemon `/invoke`. Unknown MCP methods return the standard MCP
unknown-method error. Prompt-injected mutation-shaped calls therefore have no
V1 route to authorize.

Every MCP resource read requires an explicit token except initial health or
initialize failure reporting. Read tokens are scoped to repository ids and
filter resource lists accordingly.

## Resident Recovery Scheduler

The scheduler reuses existing recovery behavior rather than inventing a new
policy:

- enumerate active runs across registered active repositories;
- use round-robin scheduling over `(repository_id, run_id)`;
- apply a per-run sweep timeout, initially 30 seconds unless tests show this
  is too low;
- cap work per tick so slow repositories do not starve others;
- mark timed-out or repeatedly failing sweeps as `sweep_degraded` in
  `scheduler_cursors` and surface them in `daemon doctor`;
- read each workflow's `recovery_policy` and preserve retry budgets;
- requeue only policy-eligible review-only stale work;
- leave repo-write stale leases stale or blocked for operator inspection;
- never auto-resolve human checkpoints;
- store bounded summaries and hashes, not transcript-like text.

The scheduler uses daemon-internal authority. V1 has no client-facing recovery
capability and no client-facing recovery mutation endpoint.

Scheduler cursor advancement is completion-based. A sweep records an
in-progress marker if needed for diagnostics, but `last_sweep_at`,
`next_sweep_after`, and terminal result state advance only after the sweep's
repo-local event writes and daemon audit rows are durably committed. If the
daemon is killed mid-sweep, restart must select the interrupted run as an
eligible next target rather than treating it as already swept. A crash alone
must not mark `sweep_degraded`.

For every run-affecting action, including review-only requeue,
process-reconcile outcome, cancel/degraded mark, or refusal that changes run
visibility, the daemon writes a repo-local event through the existing
repo-local mutation/event path. These events use a fixed daemon byline,
`author: striatumd-<instance-id>`, distinct from any operator, role, lane, or
model byline. Registry audit rows supplement that timeline; they are not the
only record of run-affecting recovery.

If `recovery watch` is also running for a registered repo, `daemon doctor`
warns about duplicate scheduling. Idempotency protects state correctness, but
operators should use one scheduler per run for a clean timeline.

If service-manager support does not land, docs and UI call this a foreground
daemon sweep. If `daemon install` lands for at least one platform, docs may say
installed resident recovery for that platform only.

## Audit Log Shape

Every daemon request that reaches daemon routing appends an audit row after
authorization is computed. This includes successful reads, denied reads, admin
actions, malformed tokens, expired/revoked tokens, repo-out-of-scope requests,
unknown endpoint requests, and health probes. V1 does not summarize health
probes; each health request writes a row with `command="health"` and
`repository_id=null`.

Required fields are timestamp, client id or null for unauthenticated health
probes, repository id or null, normalized command/endpoint, authorization
result, denial reason when safe, transport, request id when supplied, exit code
when dispatch completed, payload/argv hash, previous hash, row hash, and
segment id.

The audit log deliberately omits request bodies, response bodies, artifact
contents, model rationales, blocker prose, terminal output, token secrets, hash
salts, and exception tracebacks. `striatum daemon audit` requires `admin`.
Daemon audit is not exposed through MCP in V1.

Audit rows are per-machine daemon evidence. They are not workflow authorship
evidence, transcript evidence, human identity proof, model-token authorship
proof, source-byte provenance, or sealed provenance evidence.

## Doctor, Status, Dashboard, Web, And Evidence Surfaces

`daemon doctor` reports:

- registry schema/user-version status;
- registry file mode and owner;
- runtime directory, socket, and token fallback file permissions;
- audit hash-chain continuity and rotation segment state, including
  cross-segment hash links and retention tombstone gaps;
- token/capability problems, including expired admin-only setups;
- removed repository ids referenced by retained audit rows;
- missing, disabled, schema-skewed, removed, or identity-mismatched
  repositories;
- duplicate recovery schedulers;
- sweep-degraded runs;
- registered repos whose `.striatum/state.sqlite3` is unreachable.

`daemon doctor` detects internal inconsistency and known identity/path
mismatches. It does not claim full detection of offline local filesystem tamper
or deleted registry rows.

`dashboard --all` groups by repository, then run. It must show provenance mode
honestly as repo-local state reports it, but V1 UI labels only shipped modes:
`advisory` and `attested_bylines`. If `sealed_patch` appears, the dashboard
renders it as unsupported/unstartable text, not a shipped guarantee.

The web UI can consume daemon read endpoints later, but V1 avoids a full web
redesign. Optional loopback HTTP and Unix-socket request transport are deferred
with the daemon RPC server.

Evidence export remains repo-local. Daemon audit export is separate
client-access metadata and not workflow authorship evidence, transcript
evidence, human identity proof, or sealed provenance evidence.

## Compatibility And Upgrade Risks

Direct CLI mode remains a V1 guarantee. Existing workflows must be operable
with no daemon installed, no daemon running, and no registry file.

Version skew rules:

- Forced daemon mode never falls back to direct mode.
- Newer CLI with older daemon fails with `version_skew` unless the protocol
  explicitly declares the request compatible.
- Older CLI with newer daemon is refused when protocol major differs.
- Daemon with newer repo-local DB than supported refuses that repo with exit
  code class 9/version-skew semantics.
- Registry newer than daemon refuses startup and leaves files untouched.

Split-brain risks:

- Direct CLI can mutate repo-local state while daemon is running. V1 accepts
  this; daemon read surfaces re-query repo-local DBs rather than caching run
  state as truth.
- Registry rows can reference moved/deleted/removed repositories. They are
  surfaced as missing, removed, or identity-mismatched.
- Daemon audit is per-machine. Fresh clones do not include daemon request
  history unless exported separately.

Security risks:

- A hostile local client with a valid read token can inspect workflow metadata
  across that token's repo scope. Scope, expiry, revocation, and storage must
  be tested.
- A local user with filesystem write access can tamper with registry or
  repo-local SQLite. V1 protects against accidental and daemon-client misuse,
  not adversarial local filesystem control.
- MCP prompt injection risk is reduced by resources-only MCP, absent mutation
  tools, and the absence of daemon audit resources over MCP.

## Test Matrix

| Area | Required tests |
|---|---|
| Registry lifecycle | Create temp registry/runtime dirs, start foreground daemon, register two initialized repos, stop, restart, assert rows, schema versions, audit chain continuity, segment manifests, and scheduler cursors survive. |
| Supported platforms | Exercise Linux XDG paths and macOS Application Support/Cache/Keychain branches in CI for the platforms claimed; assert Windows daemon support is not advertised. |
| Existing repo registration | Register repo with existing `.striatum/state.sqlite3`; assert run/artifact/event rows and file hash are not rewritten except normal migrations when default migration is allowed. |
| No-migrate registration | `repo add --no-migrate` refuses when repo-local migrations would run and succeeds read-only when schema is already supported. |
| Repo identity/remove/re-add | Delete or replace a repo at a previously registered canonical path; assert identity mismatch is refused while active, `repo remove` revokes capabilities, re-add allocates a fresh `repository_id`, and old repo-scoped capabilities do not transfer. |
| Repo remove idempotency | Repeated `repo remove` succeeds without duplicate revocation rows; retained audit rows for removed repository ids are preserved and counted by doctor. |
| Symlink/path traversal | Refuse symlink loops, `..` traversal, non-repo paths, state DB symlink escapes, and duplicate canonical roots through different spellings. |
| Token lifecycle | No-token non-health reads fail; minted token has id plus one-time secret; expired, revoked, malformed, wrong-scope, and rotated tokens fail with structured reasons and no secret leakage. |
| Token storage | Runtime-file fallback is `0600`; group/world-readable fallback files are refused or warned as degraded according to policy; env/argv secret paths are not used. OS keyring storage is deferred. |
| Capability scoping | Read token scoped to repo A cannot read repo B; admin-only audit endpoint refuses read token; removed repository scope fails after removal. |
| Audit privacy | Allowed and denied requests record metadata and hashes but not request/response bodies, artifact text, blocker prose, token secrets, salts, tracebacks, or terminal output. |
| Audit integrity/retention | Hash chain verifies; altered row, missing segment, newer registry version, unsafe file permissions, and retention tombstone gaps surface through doctor or startup refusal. |
| Audit chain across rotation | Rotate segments, delete or forge data in a closed segment, and assert doctor surfaces a chain break by comparing `audit_segments.last_hash` to the next segment's first `previous_hash`; purged segments report retention gaps rather than row-level proof. |
| Health audit | Every health probe reaching daemon routing writes a metadata-only audit row with `command="health"` and `repository_id=null`. |
| Global dashboard correctness | Two repos with active runs, blockers, claimable jobs, and stale leases render in one dashboard; missing/bad repo appears as degraded row; one bad repo does not abort all. |
| CLI daemon read mode | `--daemon status/doctor/why/dashboard` reports `mode: daemon`; forced daemon with no daemon/token/version match fails with the documented exit code; no explicit daemon flag stays direct. |
| Direct CLI preservation | Current direct-mode workflow tests pass without daemon; mutating workflow verbs do not route through daemon. |
| MCP resource surface | Default daemon MCP lists and reads scoped resources; `tools/list` is empty; unknown methods return standard MCP unknown-method error; daemon audit URI is absent. |
| MCP mutation absence | Calls to raw invoke, publish, verdict, claim, repo mutation tools, or recovery mutation tools have no V1 MCP route. |
| Hostile local clients | Oversized payloads, unknown endpoints, malformed token ids, replayed request ids, and command-classification bypass attempts are denied without tracebacks where the registry-backed V1 routes expose those shapes; loopback HTTP auth failures are deferred with HTTP transport. |
| Unix socket and HTTP bind | Runtime directory is `0700`, registry is `0600`, socket is owner-only as a lifecycle marker; HTTP bind behavior is deferred with HTTP transport. |
| Resident/foreground recovery | Sweep across active runs requeues only eligible review-only stale work, preserves repo-write stale leases, emits repo-local events with daemon byline for run-affecting actions, and marks timed-out completed sweeps degraded. |
| Crash during sweep | Kill daemon mid-sweep; on restart the interrupted run is the next eligible sweep target, not skipped, and `sweep_degraded` is not falsely set. |
| Duplicate recovery scheduling | Concurrent `recovery watch` produces a doctor warning and idempotent state. |
| Version and auth failures | CLI/daemon protocol mismatch, repo-local schema mismatch, registry schema mismatch, daemon unreachable, invalid token, and capability denial produce documented failures and do not mutate state. |
| Provenance honesty | Daemon-rendered status/dashboard reports `advisory` and `attested_bylines` without changing author lines; `sealed_patch` is shown only as unsupported/unstartable if encountered. |

## Staging Plan For Provenance Honesty

V1 must avoid overclaiming RFC 0026 or RFC 0027 guarantees:

- In `advisory` mode, daemon mode remains advisory.
- In `attested_bylines`, the daemon reports lane-attestation results derived
  from existing repo-local helpers only.
- Token labels, client ids, socket connections, and audit rows never become
  artifact bylines.
- Daemon recovery event bylines identify daemon actions only and never imply a
  human operator, role lane, or model authored the artifact.
- `sealed_patch` runs continue to refuse start until containment and apply
  authority exist. The daemon does not add apply endpoints, key storage,
  signing, or receipt issuing.
- Daemon audit records client request metadata. The hash chain can detect some
  local tamper and corruption in retained manifests and rows, but it is not
  transcript evidence, source-byte provenance, human identity proof,
  model-token authorship proof, or resistance to a local filesystem writer.
- Documentation and UI labels should use "daemon mode", "global read view",
  "foreground daemon sweep", and, only where service installation lands,
  "resident installed daemon". Avoid "sealed", "trusted apply", or "proof" for
  V1 daemon behavior.

## Human-Decision Questions

These questions should be answered before implementation proceeds or before
the corresponding deferred step is scheduled:

| Question | Synthesis recommendation | Human decision needed |
|---|---|---|
| Is V1 required to ship service-manager installation? | Not required for acceptance if wording says foreground daemon sweep. | Decide whether to add `daemon install` now for at least Linux user systemd, or defer installed residency. |
| Should daemon mode become required for sealed provenance? | Probably yes eventually, but not in V1. | Defer until RFC 0027 containment/apply design is accepted. |
| Is Option C enough? | Yes for V1, with explicit per-machine daemon audit caveat. | Revisit only if fan-out or audit export pain appears. |
| Should cross-repository workflows be in scope? | No. | Later RFC after daemon read mode is proven. |
| Which MCP capabilities are safe by default? | Resources-only reads with explicit tokens and no daemon audit resource. | Confirm this remains acceptable for MCP clients. |
| Does local multi-user mode matter? | Not for V1. | Decide before any shared-workstation or hosted-adjacent mode. |
| What is the upgrade story when CLI and daemon versions differ? | Forced daemon fails closed; direct mode is explicit. | Confirm no silent fallback is acceptable UX. |
| Is Python acceptable for a long-running control plane? | Yes for V1. | Revisit before daemon-owned supervision becomes primary. |
| How should token fallback behave without OS keyring? | Runtime `0600` file with degraded warning and short expiry guidance. | Decide whether to hard-require OS keyring for non-admin MCP clients. |
| How should daemon audit be exported? | Keep per-machine registry audit; offer explicit export later. Do not expose audit over MCP in V1. | Decide whether audit export belongs in a follow-up. |

## Implementation Landing Order

1. Registry path helpers, platform path selection, file-permission checks,
   registry migrations, audit hash-chain helpers, segment manifest helpers, and
   test overrides.
2. Repo identity support, `repo add/list/remove`, `--no-migrate`, path
   re-occupation refusal, removal idempotency, capability revocation, and
   repository-id non-reuse.
3. Foreground sweep process, Unix socket lifecycle marker, lifecycle lock,
   health command, registry-backed read routing, forced-daemon refusal classes,
   and append-only audit. RPC version handshake and deterministic socket/HTTP
   request routing are deferred.
4. Token/capability management for `read` and `admin`, OS keyring/runtime
   fallback client storage, expiry/revocation/rotation, and no-token denial.
5. Explicit daemon client mode for read-only `status`, `doctor`, `why`, and
   `dashboard --all`; direct-mode preservation tests.
6. Daemon MCP read resources with empty `tools/list` and no audit URI.
7. Recovery scheduler using round-robin time-bounded sweeps,
   completion-based cursor advancement, and repo-local event emission with
   daemon byline for run-affecting actions.
8. Documentation updates and fixture tests for two-repo dashboard, restart,
   MCP absence of tools/audit, audit privacy/integrity, token lifecycle, repo
   removal, crash during sweep, platform path branches, and provenance honesty.

This order lands RFC 0028 V1 without weakening the repo-local control plane or
claiming that daemon mode has solved sealed provenance, apply authority,
operator tenancy, or model-authorship proof.
