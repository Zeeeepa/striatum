# RFC 0110: Daemon → PostgreSQL authentication and the database-enforced write boundary

Status: accepted (D164)
Date: 2026-06-03
author: proposer-claude-opus-4-8-001
Context: RFC 0033 (PostgreSQL as sole substrate; §3 append-only audit invariant), RFC 0043 (daemon-required runtime), RFC 0079 §5 (owner-applied migrations), RFC 0096 (supervised-lane trust boundary; session-bound capability tokens), RFC 0107 (multi-principal trust model), GH #87 (lane PG-reachability). Touchpoints: `go/pkg/db/connection.go` (`Connect`/`ConnectAndMigrate`/`ResolveConfig`, pgx v5 + pgxpool, simple protocol), `go/pkg/db/audit.go` (`AuditRecorder`, `V2RowHash`, `VerifyRows`), `go/pkg/reads/doctor_lane_sandbox.go` (the advisory `lane_pg_reachable` warning), `go/pkg/mutations/supervision_control.go` (lane env allowlist), `docs/how-to/postgres-transition.md` (the documented `striatumd_rw` / owner two-role posture), migrations `0001/0005/0006/0013` (existing `REVOKE UPDATE/DELETE` on append-only tables).

## Problem

`striatumd` is a long-lived local process that owns the authoritative PostgreSQL
state. How it **authenticates to that database** is currently underspecified and
sits in one of two postures, each with a distinct hole:

- **Unix-socket PEER auth as the daemon's OS user, no password** (the live
  deployment, e.g. `postgres://halbritt@/postgres?host=/var/run/postgresql`).
  Convenient and secret-free, but it conflates the daemon's identity with its
  OS user. Supervised AI agent **lanes** (untrusted coding agents) are spawned
  as that same OS user, so a lane can open PostgreSQL **directly** over the
  socket and bypass the artifact/RPC API and the hash-chained audit log
  entirely — the **#87** trust hole. The env allowlist
  (`supervision_control.go`) scrubs `PG*`/`*DSN*`/`DATABASE_URL`, but PEER needs
  no secret, so scrubbing does not help. The only mitigation today is an
  **opt-in, advisory-only** dedicated PG-less lane OS user
  (`STRIATUM_LANE_OS_USER`), surfaced as a `lane_pg_reachable` warning in
  `daemon doctor` — never enforced.

- **Password auth as a least-privilege `striatumd_rw` role**
  (`docs/how-to/postgres-transition.md`). This separates the daemon's PG
  identity, but introduces a **standing secret in a config file** — a leak
  surface that survives restarts and never rotates — and still does nothing
  about a same-host lane that obtains it.

Three further forces are unaddressed:

1. **Migrations need a higher-privileged owner role** the runtime role lacks
   (`--as-owner`, RFC 0079 §5) — a second credential, needed at the worst time
   (upgrade).
2. **The future is multi-principal** (RFC 0107: several humans + AI operators
   sharing one `striatumd` + PG across repos) and possibly **PG on another
   host** (PEER is same-host-only).
3. **The audit log knows the daemon wrote a row, not which RPC/principal caused
   it** — an attribution gap RFC 0107 needs closed.

The unifying observation: today the "artifact/RPC API is the sole write path"
invariant is enforced **only in the daemon process**. Anything that authenticates
to PG as the runtime role — including a lane that scraped a credential — can
write the authoritative tables directly. Authentication should not be the only
thing standing between an untrusted same-host process and the audit chain.

## Proposal (design RFC; phased implementation follows)

A **layered** model. Each layer is independently shippable and independently
valuable; together they make the write contract true in the database, not just
in the process. The design rule throughout: **make a leaked runtime credential
uninteresting** rather than relying on perfect secrecy.

### L0 — Credential: ephemeral, owner-bootstrapped, RAM-only

At startup `striatumd` opens a short-lived **owner** connection over the
existing unix-socket PEER path (the same path `--as-owner` migrations already
use), runs `ALTER ROLE striatumd_rw PASSWORD '<crypto/rand>'`, closes the owner
connection, then opens its runtime `pgxpool` as `striatumd_rw` with the password
held **only in process memory** (zeroed after `pgxpool.ParseConfig` consumes
it). The password is never written to disk, env, or `daemon.toml`. Every
`systemctl --user restart striatumd` re-rotates, so any password a lane scraped
from a previous run's core dump or swap is already dead.

- **Remote-PG escape hatch:** when PG is on another host there is no owner-PEER
  socket. Add an explicit `STRIATUM_OWNER_DB_URL` (parallel to the existing
  `STRIATUM_DAEMON_DB_URL`) for the bootstrap connection; when unset, fall
  through to PEER. For the at-rest secret in this case, prefer a **systemd
  encrypted credential** (`LoadCredentialEncrypted=` + `sd_get_credentials()`,
  TPM/host-key encrypted, never in `/proc/<pid>/environ`) over a plaintext file
  — near-zero daemon code, and the lowest-friction floor for operators who
  cannot use owner-PEER bootstrap.
- **Single-role dev guard:** if the resolved owner role *is* the runtime role
  (common in dev), skip rotation with a `WARN` rather than rotating the password
  out from under the bootstrapping connection.
- **Diagnosability:** `daemon doctor` gains a `db-credential-posture` probe that
  asserts a password is set (`SELECT rolpassword IS NOT NULL FROM pg_authid …`)
  **without ever reading or logging the value**; the daemon emits a single
  structured `db_credential_rotated` log line on success.

### L1 — Enforcement: PostgreSQL guards the write contract

Make `striatumd_rw` unable to bypass the audit/artifact contract **at the
database level**. Migrations `0001/0005/0006/0013` already `REVOKE UPDATE/DELETE`
on `audit_log`/`events`/`artifacts` (RFC 0033 §3 append-only). The next tier
also revokes `INSERT` and exposes writes only through `SECURITY DEFINER`
PL/pgSQL functions owned by the owner role, which enforce the invariants
currently checked in Go (hash-chain append locked atomically with the mutation;
attempt-scope artifact immutability). `striatumd_rw` keeps only `EXECUTE`.
Result: an attacker who obtains the runtime DSN **cannot forge artifacts or
tamper with the audit chain** — the role literally lacks table-level DML.

- **Phasing:** Phase 0 `audit_log` (highest value, single call site
  `AuditRecorder.RecordRPC`) → Phase 1 `artifacts` (attempt-scope immutability
  in-DB) → Phase 2 `events`. Each phase is provable green before the next.
- **RLS as a second tier, not the primary:** enable Row-Level Security on
  per-session tables (`leases`, `sessions`) keyed on
  `current_setting('app.session_id')`, set per transaction. Functions own the
  *write path*; RLS limits *which rows* a valid call may touch.
- **CI gate:** the `pgtest` harness already runs mutations through a per-test
  `striatumd_rw_<db>` role. Add a negative-path test asserting a direct
  `INSERT` from that role fails with `42501` (insufficient_privilege), so the
  REVOKE invariant is machine-verified on every migration forward.

### L2 — Isolation: lanes cannot reach PG out-of-band, by default

Promote the dedicated PG-less lane OS user from opt-in advisory to the
**hardened default**, and back it with a filesystem boundary: move PG's socket
into a `0700` directory (`unix_socket_directories`) owned by a daemon identity
**distinct from the lane identity**, so a lane cannot traverse the directory to
reach the socket even sharing nothing else. `doctor_lane_sandbox.go` escalates
`lane_pg_reachable` from a warning to a **startup-blocking** error when no
distinct lane user is configured — gated behind a config flag
(`security.pg_socket_hardened`, default-false on upgrade, flipping to default-on
a minor version later) so existing installs are not stranded on upgrade day.
Also scrub `PGHOST` from the lane env (defense-in-depth against a lane handing a
guessed socket path to a libpq tool).

### L3 — Attribution: every mutation names its RPC and principal

Add a `pgxpool.BeforeAcquire`/`AfterRelease` hook pair in `connection.go` that
issues `SET LOCAL striatum.rpc_id = '<id>'` and `SET LOCAL
striatum.principal_id = '<principal>'` at the start of every transaction
(cleared on release so provenance never bleeds across pooled checkouts). The
hash-chained audit log then attributes every row mutation to the originating
RPC call **and principal**, not merely "the daemon process" — directly closing
the RFC 0107 attribution gap. This layer is useful **standalone, today**, before
any other layer lands. Record auth posture and transitions in a daemon-owned
`daemon_auth_log` table that `daemon doctor` can read **via the owner
connection** even when the runtime credential is broken — turning the
"we rotated something and forgot to tell the daemon" 3am page into a one-command
diagnosis.

### Future — cross-host identity

For the multi-host path, issue a **client TLS certificate from a local
self-signed CA** generated at `daemon init` (no external CA, no ACME, no cloud),
with `daemon doctor` warning 30 days before expiry. Purely additive: the
same-host default stays owner-PEER bootstrap + L1/L2.

## Acceptance

- L0: a fresh `striatumd` start rotates the `striatumd_rw` password with no
  on-disk secret in the default local-PEER posture; a captured runtime DSN from
  before a restart fails after it; `daemon doctor` reports credential posture
  without leaking the secret. Remote-PG operators have a documented
  `STRIATUM_OWNER_DB_URL` / systemd-credential path.
- L1: `striatumd_rw` cannot directly `INSERT` into `audit_log` (then `artifacts`,
  `events`) — proven by a `pgtest` negative-path test asserting SQLSTATE
  `42501`; the hash-chain `VerifyRows` invariant still holds end-to-end after
  the write path moves behind `SECURITY DEFINER`.
- L2: with the hardened default configured, a lane process cannot open the
  daemon's PostgreSQL socket; `daemon doctor` blocks (not just warns) when no
  distinct lane identity is configured under the enforcement flag.
- L3: every authoritative mutation carries an attributable `rpc_id` +
  `principal_id` readable from the audit log; provenance resets across pool
  checkouts (test-pinned). `daemon doctor` can diagnose auth failures via the
  owner connection when the runtime credential is dead.
- `docs/reference/spec.md` carries the daemon→PG authentication model and the
  database-enforced write-boundary invariant; `docs/decisions/decision-log.md`
  records the decision on acceptance.

## Non-goals

- **Not SaaS / hosted identity.** No external IdP/SSO, no hosted control plane,
  no telemetry, no new hosted services (reaffirms RFC 0107). The local
  self-signed CA and systemd credentials are entirely operator-owned.
- **Not a PostgreSQL C extension or background worker.** In-database
  enforcement uses only stock PL/pgSQL + RLS + GRANT/REVOKE — no compiled `.so`,
  no PG-version-fragile native code.
- **Not a from-scratch PG wire proxy.** The L2 boundary is a `0700` socket
  directory + distinct OS identity, not a new in-process protocol server (that
  alternative is recorded as rejected below).
- Does not, by itself, deliver the RFC 0107 multi-principal model — it supplies
  the credential, enforcement, isolation, and attribution substrate RFC 0107
  builds the principal layer on.

## Sequencing

L3 (attribution) and the L0 `daemon doctor` posture probe are the cheapest and
land first (no behavior change, immediate RFC 0107 value). L0 credential
rotation and L1 Phase 0 (`audit_log`) follow as the security core. L2 ships
behind the default-false enforcement flag so upgrades are non-breaking; the flip
to default-on is a separate, announced minor version. Cross-host certs are
deferred until a real multi-host deployment exists. This RFC is sequenced
**after** the RFC 0104/0105 reliability foundation and does not block RFC 0103's
remaining work.

## Relationship to prior RFCs

- **RFC 0033 §3** (append-only audit invariant) is the seed L1 generalizes: from
  "revoke UPDATE/DELETE" to "revoke all direct DML; writes only via vetted
  owner-owned functions."
- **RFC 0079 §5** (owner-applied migrations) supplies both the L0 bootstrap path
  (owner-PEER `ALTER ROLE`) and the L1 function DDL delivery (`--as-owner`).
- **RFC 0096** (lane trust boundary, env allowlist, session-bound tokens) is the
  per-session substrate; L2 hardens its #87 residue from advisory to default,
  and L3's `app.session_id` RLS reuses its session binding.
- **RFC 0107** (multi-principal) consumes L3 attribution (per-principal audit
  rows) and L1 isolation (cross-principal write confinement) directly.
- **GH #87** (`doctor_lane_sandbox.go`) is the open item L2 closes structurally.

## Appendix — rejected alternatives (ideation trap log)

Recorded so the design record carries *why* these were declined:

- **PG C background worker / hook checking a daemon-signed token per DML** —
  shipping and maintaining a compiled C extension for a Go local-first tool;
  fragile across PG versions and a heavy install burden. L1's PL/pgSQL functions
  achieve in-engine enforcement with stock SQL.
- **Run PostgreSQL inside a Linux user namespace** so the daemon's effective UID
  differs from lanes' — changes the deployment model and is fragile under
  `systemd --user`.
- **`seccomp`-block `socket()` in the daemon after the pool opens** — also
  blocks the daemon's own reconnect on PG restart / pool churn; it would page
  on-call rather than protect.
- **Custom PAM module verifying the connecting process's ppid chain** —
  non-portable, and ppid is spoofable once the daemon exits.
- **Quorum N-of-N PL/pgSQL gate that unlocks the role** — security theater; a
  compromised subsystem is already inside the quorum.
- **`pg_notify` nonce consumed at connect / systemd socket-activation fd handed
  to a *client*** — infeasible as stated: authentication precedes any SQL, and
  PostgreSQL owns its own listen socket (a client cannot be handed it via
  `SD_LISTEN_FDS`). The salvageable "access path constructed before lanes exist"
  insight survives only inside a daemon-owned forwarder.
- **"PostgreSQL listens on a Linux abstract-namespace socket"** — PG binds only
  filesystem sockets; an abstract socket is reachable solely in front of a
  daemon-owned forwarder, which L2 deliberately avoids in favor of the simpler
  `0700` filesystem-socket directory.

*Provenance: this RFC's option space was generated by the `adhd` divergent-ideation
skill (5 cognitive frames × 6 ideas → score/cluster/trap → deepen top 3); the
trap log above is its critic phase, preserved.*
