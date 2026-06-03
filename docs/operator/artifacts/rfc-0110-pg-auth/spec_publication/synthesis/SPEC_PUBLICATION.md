---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: spec-author-claude-opus-4.8-001
workflow: rfc-0110-pg-auth-panel
phase: spec_publication
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 1
title: "RFC 0110 — Published spec: daemon→PostgreSQL authentication and the database-enforced write boundary"
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/spec_publication/draft/SPEC.md
  - docs/operator/artifacts/rfc-0110-pg-auth/revision_synthesis/synthesis/REVISION_SYNTHESIS_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/constraint_discharge_review/synthesis/DISCHARGE_SYNTHESIS_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/constraint_discharge_review/review/DISCHARGE_REVIEW_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/adjudicator/COLLABORATION_LEDGER_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/DECISION_cycle2_proceed.md
---

# RFC 0110 — Published Spec: Daemon→PostgreSQL Authentication and the Database-Enforced Write Boundary
author: spec-author-claude-opus-4.8-001

## Publication gate (this artifact's standing)

This is the **published, authoritative** RFC 0110 spec, gated on the latest
cleared constraint ledger — authored from adjudicated constraints, not from
the original proposal. The gate chain it clears:

1. `COLLABORATION_LEDGER_cycle_2.md` — the cycle-2 adjudicated constraint
   extraction: verdict `needs_revision`, **12 binding constraints**
   (1 critical + 11 high) + 3 medium follow-ups.
2. `DECISION_cycle2_proceed.md` (`dec_b95396ff8f414eae5dbcced5476313ad`,
   `accepted_with_follow_up`) — operator direction past the exhausted
   revision budget: accept the converged design, carry the cycle-2 findings
   as **binding implementation constraints in the published spec**.
3. `REVISION_SYNTHESIS_cycle_1.md` — the cleared revision discharging all
   twelve constraints (fold-in / answer / accept-as-risk, each with a named
   falsifiable gate) and folding the three mediums.
4. `DISCHARGE_REVIEW_cycle_1.md` — `accept_with_findings`: all twelve
   discharges confirmed; carry-forward findings preserved below (§14, §15
   step 0, §17).
5. `DISCHARGE_SYNTHESIS_cycle_1.md` — names the revision synthesis as the
   binding ledger for this publication and the mandatory carry-forward gates,
   all of which appear in §14–§16 as normative acceptance checks.

The working draft (`../draft/SPEC.md`) carries identical normative content;
**this synthesis is the authoritative copy**. Where this spec is silent,
`REVISION_SYNTHESIS_cycle_1.md` is normative; where both are silent,
`SYNTHESIS_cycle_2.md` is. Every binding constraint lands below as testable
text or a named gate; none is waived.

Current-source anchors were re-verified at revision time and spot-checked by
the discharge review: `go/pkg/db/connection.go:207` (pool forces
`pgx.QueryExecModeSimpleProtocol`), `go/pkg/db/connection.go:82` (generic
`BeginTx` on the shared `Runner`), `go/pkg/rpc/server.go:119-130` (audit
append fail-open today), `go/pkg/db/audit.go:82` (`RecordRPCTransport` opens
its own transaction), `go/pkg/pgtest/pgtest.go:75-86` (imperative Go
`GRANT`/`REVOKE` role layout).

---

## 1. Problem and threat model

Striatum's authoritative live state is the daemon-owned PostgreSQL instance
(RFC 0033 + D094 / RFC 0043). Today the runtime role `striatumd_rw` holds
direct DML on every durable surface, the runtime credential is a static DSN,
attribution of writes to RPCs/principals is application-side convention, and
the audit chain hash is computed only in Go (`V2RowHash`). Consequences:

- a leaked runtime DSN string is a **durable, unattributed write capability**:
  the holder can insert hash-correct audit rows, forge artifacts, and rewrite
  events without any daemon involvement;
- lanes (agent subprocesses) on the same host can reach the PostgreSQL socket
  (#87, today advisory-only);
- nothing in the database distinguishes a daemon-issued write from any other
  write performed with the same role.

RFC 0110 closes this with four layers:

- **L0 — ephemeral runtime credential.** At every daemon start, an
  owner-PEER-bootstrapped rotation gives `striatumd_rw` a fresh RAM-only
  password. A captured DSN string dies at the next restart.
- **L1 — DB-enforced write contract.** Direct DML on protected durable
  surfaces is revoked; writes route through owner-owned `SECURITY DEFINER`
  functions that compute the chain hash in-DB, enforce append-only/attempt
  invariants, and verify **daemon authority** (§4) before mutating. Phased:
  `audit_log` → `artifacts` → `events` (§7).
- **L2 — lane PG-unreachability.** A dedicated PG-less lane OS user and a
  `0700` socket directory make the database unreachable from lanes as the
  hardened default posture (#87; status language in §11).
- **L3 — per-RPC attribution.** Every mutating transaction carries
  `rpc_id`/`principal_id`/`session_id` labels set in an in-transaction
  prelude — **labels only, never authority** (§4.6). The original
  `BeforeAcquire` placement is corrected: `BeforeAcquire` fires before the
  mutation transaction, so a `SET LOCAL` invariant cannot hold there.

No native code, no wire proxy, no hosted identity (`C-NO-NATIVE`). Sequenced
after the RFC 0104/0105 reliability foundation.

---

## 2. Product claims (normative, narrowed)

Two separable guarantees, each tied to the phase where its gate is live:

- **G1 — invariant integrity (DB-enforced).** A leaked runtime credential
  cannot violate the append-only / hash-chain / attempt-scope / segment
  invariants of a protected surface: the only write path computes the hash and
  holds the chain lock in-DB.
- **G2 — daemon issuance.** A write to a protected surface succeeds only if
  the caller presents the per-process daemon-authority secret (§4). A process
  holding only the DSN *string* cannot mutate.

**Read-scope posture (binding, `C-DSN-READ-SCOPE`).** RFC 0110 prevents
**unauthorized mutation** and **hash/attempt-invariant violation** by a
runtime-credential holder. It does **not** claim read confidentiality against
a live runtime credential in this phase: `striatumd_rw` retains broad `SELECT`
over daemon-owned tables (artifacts, events, sessions, queue payloads,
principals, blockers). Read exposure is *bounded* — a leaked DSN **string**
dies at the next restart (L0), and a sandboxed lane cannot reach the socket at
all (L2) — but a live credential reads what the daemon reads. The earlier
"a leaked DSN is uninteresting" sentence is **retracted** as overbroad. A
*read-scope least privilege* successor issue is filed before the first
behavior-changing PR merges (§19); until it lands, no Striatum doc may claim
private-read denial.

**Claim phasing (binding, `C-PHASED-WRITE-CLOSURE`).** The sentence "the
daemon's durable write paths are DB-enforced" (the sole-durable-write-path
claim) is **reserved** to phase P2 `full` (§7). Every operator-facing artifact
(this spec, D164, doctor text, release notes, README-level claims) keys its
claim to the doctor `pg_write_boundary` posture string so claims and reality
cannot drift apart.

**G2 residual (explicit).** G2 does not survive a full live-process memory
scrape of a running daemon (the scrape yields DSN *and* secret). Bounded by L0
rotation-on-restart and L2 lane isolation; not eliminated (§17).

---

## 3. Architecture overview

```
            ┌────────────────────────────── striatumd (Go) ──────────────────────────────┐
            │  L0 bootstrap (owner PEER conn):                                           │
            │    rotate striatumd_rw password (RAM-only)                                 │
            │    generate daemon_auth secret (crypto/rand, RAM-only)                     │
            │    UPSERT digest → daemon_auth_registry (owner-owned, role_name column)    │
            │                                                                            │
            │  per mutating RPC:                                                         │
            │    BeginAuthorizedMutation(ctx, attr) ──► MutationTx                       │
            │      prelude via ExecBound (extended protocol, never simple):              │
            │        set_config('striatum.daemon_auth',  secret, true)   ── authority    │
            │        set_config('striatum.rpc_id',       …, true)        ── label        │
            │        set_config('striatum.principal_id', …, true)        ── label        │
            │        set_config('app.session_id',        …, true)        ── label        │
            │      handler SQL … final write = audit append (same tx, atomic)            │
            └────────────────────────────────────────────────────────────────────────────┘
                                              │ UNIX socket (0700 dir under L2)
            ┌────────────────────────────── PostgreSQL ─────────────────────────────────┐
            │  SECURITY DEFINER append_* fns (owner-owned, hardened):                    │
            │    assert_daemon_authority()  ── sha256(presented‖salt) vs registry digest │
            │    in-DB v3 hash, chain lock, append-only enforcement                      │
            │  direct DML on protected surfaces: REVOKEd from striatumd_rw (per phase)   │
            │  daemon_auth_registry / daemon_auth_log / schema_authority: owner-only     │
            └────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. The daemon-authority gate (L1 core)

### 4.1 Mechanism

At bootstrap, over the **owner** PEER connection, `striatumd`:

1. generates `auth_secret` via `crypto/rand` — RAM-only, never disk/env/DSN;
2. computes `digest = sha256(auth_secret || per-instance salt)` and UPSERTs
   `(instance_id, role_name, digest, rotated_at)` into the **owner-owned**
   `striatumd.daemon_auth_registry`, on which `striatumd_rw` holds **no
   privilege** (`T-REGISTRY-ACL`: `has_table_privilege('striatumd_rw',
   'striatumd.daemon_auth_registry', 'SELECT')` is false);
3. holds `auth_secret` in process memory for the pool's lifetime
   (`C-SECRET-HONEST`: RAM-only + rotation-on-restart, not zeroization).

Every owner-owned `SECURITY DEFINER` write function begins with
`striatumd.assert_daemon_authority()`: reads
`current_setting('striatum.daemon_auth', true)`, compares
`sha256(presented || salt)` to the registry digest for the current instance,
and `RAISE`s SQLSTATE `28000` on miss. A `striatumd_rw` caller cannot forge
the secret, read the digest, or learn it from the function.

### 4.2 Transport: the prelude never rides simple protocol (binding, critical — `C-EXTENDED-AUTH-PRELUDE`)

The pool default is `pgx.QueryExecModeSimpleProtocol`
(`go/pkg/db/connection.go:207`), under which bound parameters interpolate into
SQL text observable in `pg_stat_activity.query` by any same-role session. The
prelude therefore **must not** execute through simple protocol or any other
path that serializes the secret into query text.

**Resolution:** per-call extended-protocol override; pool default unchanged.
`TxRunner` gains one dedicated method, **`ExecBound(ctx, sql, args...)`**,
which always passes `pgx.QueryExecModeExec` (Parse/Bind/Execute — bound
values travel in Bind messages; `pg_stat_activity.query` shows the literal
`$1`). `ExecBound` is the **only** API the prelude uses, and all four GUCs
ride it (the labels are not secrets, but keeping principal/session ids out of
query text is free and privacy-positive). The migration-DDL rationale for the
simple-protocol pool default is untouched.

**Gates.**
- **T-PRELUDE-OBSERVER** (PG-gated): a second `striatumd_rw` session polls
  `pg_stat_activity` (and `pg_stat_statements` where installed) while the
  daemon performs authorized mutations under load; the secret value never
  appears in any observed query text.
- **G-PRELUDE-MODE** (unit guard): fails a prelude whose exec mode resolves to
  `QueryExecModeSimpleProtocol`; asserts `ExecBound` is the sole prelude entry
  point.

### 4.3 The authorized mutation wrapper (binding — `C-AUTH-TX-WRAPPER`)

Authority is a **constructor, not a convention**. A single constructor in
`go/pkg/db`:

```
BeginAuthorizedMutation(ctx, attr AuthorityContext) (MutationTx, error)
```

begins the transaction, immediately issues the §4.2 prelude (authority + the
three attribution labels, `is_local = true`), and returns a `MutationTx`.
Enforcement is layered so the wrapper is unavoidable:

1. **Type-level:** write-surface helpers (audit append, artifact append, event
   append, and their SD-function callers as phases land) accept only
   `MutationTx`, never `TxRunner`.
2. **G-MUTATION-TX** (guard): enumerates mutating RPC handlers and fails any
   that reach `Runner.BeginTx`/`withTx` directly. Read-only paths, migrations,
   and recovery sweeps live on an explicit allowlist with rationale strings.
   The guard run includes a **doc-presence check** for the spec sentences it
   enforces (§15 step 0).
3. **T-SQL-ORDER** (regression): a recording runner captures a compliant
   handler's statement stream and asserts
   `set_config('striatum.daemon_auth', …)` is statement 1, before the first
   DML or write-function call; a deliberately non-compliant fake handler
   fails.

L3 attribution and the L1 authority secret enter PostgreSQL through the same
prelude, so "attribution set but authority forgotten" (or vice versa) is
structurally impossible. The wrapper lands in release N (§15) **before** any
schema authority exists, so every mutating handler migrates while the change
is behavior-neutral.

### 4.4 Audit append: atomic with its mutation, fail-closed (binding — `C-AUDIT-AUTH-PRELUDE`)

Today audit append is fail-open (`go/pkg/rpc/server.go:119-130` consults
`auditErr` only to attach `AuditID`; `go/pkg/db/audit.go:82` opens its own
transaction). The contract becomes:

- **Mutation-coupled (the binding case).** For mutating RPCs the audit row is
  appended **inside the same `MutationTx`**, as its final write (via
  `append_audit_row` once P0 lands; via direct insert before that). Mutation
  and audit row commit or roll back **atomically** — success-without-audit-row
  is impossible by construction, and the append inherits the same
  authority/attribution prelude as the mutation it records.
- **Standalone (reads, denials, transport errors).** Non-mutating audit rows
  append in their own small authorized transaction. On append failure the RPC
  response is **converted to an error** (`audit_append_failed`), a structured
  diagnostic is emitted, and a doctor counter increments. The
  ignore-`auditErr` path is removed; a nil `AuditRecorder` becomes a
  test-harness-only configuration, refused in production wiring.

**Accept-as-risk (scoped):** fail-closed couples RPC availability to audit
appendability. Accepted deliberately: the audit chain lives in the same
PostgreSQL as every durable surface, so "cannot append audit row" almost
always means "cannot do durable work at all"; a daemon answering RPCs it
cannot record would violate the provenance posture this RFC establishes.

**Gate.** **T-AUDIT-FAILCLOSED:** force `append_audit_row` (and the pre-P0
direct insert) to reject inside a mutating RPC ⇒ the mutation rolls back and
the RPC errors; force a standalone append failure ⇒ the response is an error.
Never success with a missing row.

### 4.5 Authority lifecycle: lifetime-of-instance, loud lapses (binding — `C-AUTH-WINDOW-LIVENESS`)

There is **no freshness window and no periodic refresh** — the self-wedge
failure mode is structurally absent, not handled:

- **Validity = lifetime of instance.** `assert_daemon_authority` matches the
  presented secret against the registry digest with no TTL check in the hot
  path. A correct secret never ages out while its daemon lives.
- `rotated_at` is **diagnostic only** — consumed by `daemon doctor` and the
  rotator probe (§9.4), never by the assert path.
- **Authority lapse is loud.** If the registry row is missing, wiped, or
  superseded (a same-role rotator UPSERTed over it), write functions raise
  `28000`; the daemon maps that SQLSTATE on the authorized path to a
  structured, owner-attributable **`daemon_auth_lost`** error naming the
  probable cause ("registry row for instance `<id>` missing/superseded —
  restart the daemon to re-bootstrap, or check for a concurrent rotator") plus
  a doctor finding. Never a bare/silent `28000` write failure. Runtime
  self-heal via a live owner connection is **rejected** for 0110 (owner
  connectivity stays a bootstrap dependency, not a steady-state one); restart
  is the re-bootstrap path.

**Gate.** **T-AUTH-LIVENESS:** (a) age `rotated_at` arbitrarily ⇒ authorized
writes keep succeeding; (b) delete/overwrite the registry row ⇒ the next
authorized write fails as structured `daemon_auth_lost` with the diagnostic
and doctor finding, and no mutation lands.

### 4.6 GUCs are labels; the secret is authority

`striatum.rpc_id`, `striatum.principal_id`, `app.session_id` are **attribution
labels only** — read solely to store provenance, never to authorize. The only
authority-bearing GUC is `striatum.daemon_auth`. RLS on `leases`/`sessions`
keyed on `current_setting('app.session_id', true)` is defense-in-depth
row-scoping under an already daemon-authorized session, **never** the trust
boundary; it lands last (§7). Gates: **T-GUC-LABEL** (fake labels without the
secret still fail; with the daemon path they succeed and are stored verbatim),
**T-EXEC-AUTH** (§6).

### 4.7 Reset robustness with bounded discard (folds `OPS-12`; carries `C-ATTR-RESET-FAIL`)

Transaction-local GUCs vanish on commit/rollback by PostgreSQL semantics; the
`AfterRelease` `DISCARD ALL` backstop stays. Connection **destroy** is
narrowed to the cases where session state is ambiguous: error/cancel/timeout
**mid-prelude**, handler panic, or unknown transaction outcome. A clean
`ROLLBACK` returns the connection to the pool normally. Blast radius is
bounded: jittered backoff on re-establishment, plus a destroy-rate ceiling
that raises a doctor finding past N destroys/minute — a mass-cancel or
PG-stress event becomes a visible posture signal, not a reconnect storm
colliding with the rotated password.

**Gate.** **T-ATTR-RESET:** all five cases (commit, rollback, cancel, timeout,
panic) leave the next checkout with no prior `rpc_id`/`principal_id`/
`app.session_id` and no `daemon_auth`.

---

## 5. The audit chain: v3 hash contract

### 5.1 The load-bearing risk, resolved by format — not by porting

The audit chain hash is computed today in Go (`V2RowHash`) over JSON with
Go-specific canonicalization (alphabetical key order, `encoding/json`
escaping). Reproducing those bytes in PL/pgSQL is the load-bearing L1 risk:
any divergence (HTML escaping, key order, U+2028/U+2029, timezone rendering)
breaks `VerifyRows` for every chain. **Resolution: do not port — supersede.**
New SQL-written rows use an explicit **v3 length-prefixed `bytea` canonical**
that is escaping-free and key-order-free, byte-identical in Go and PL/pgSQL by
construction:

- Same 15 fields, fixed declared order: `ts, schema_version,
  hash_format_version, daemon_version, client_id, repository_id, method,
  decision, denial_reason, transport, request_id, exit_code, params_sha256,
  previous_hash, segment_id`.
- `row_hash = lower_hex(sha256(concat over fields of Enc(field)))` where:
  string `s` → `dec(octet_length(convert_to(s,'UTF8'))) || ':' ||
  utf8_bytes(s)`; integer `n` → decimal ASCII, length-prefixed like a string;
  null → the 3 bytes `-1:`. No implicit casts.
- `ts` renders as the RFC3339 second-truncated UTC string via
  `to_char((ts AT TIME ZONE 'UTC'), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')` — the
  explicit `AT TIME ZONE 'UTC'` makes it independent of the session
  `TimeZone` GUC (**T-TS**: `TimeZone` ∈ {UTC, EST, Asia/Kolkata,
  Australia/Lord_Howe}).
- Hashing uses **`pgcrypto`'s `digest(bytea,'sha256')`** (stock contrib —
  `C-NO-NATIVE` holds); the owner bundle installs it and startup asserts it
  present before any `REVOKE`.
- `V2RowHash` is preserved **permanently** as the only reader of pre-cutover
  rows. `VerifyRows` (`go/pkg/db/audit.go:213`) gains an explicit dispatch on
  `hash_format_version`: `2→V2RowHash`, `3→V3RowHash`, **unknown → verifier
  failure** (never silent v2 fallback); chain continuity is checked across the
  v2→v3 boundary.
- An owner-only, **test-only** raw-row writer (bypassing the SD fn) seeds
  malformed/mixed/tampered chains so verifier paths are testable without a
  production bypass. It patches no privileges (§10).

**Gates.** **T-HASH-PARITY** (Go vs SQL byte-for-byte over ASCII, multibyte
UTF-8, `<`/`>`/`&`, U+2028/U+2029, empty, null, integers, ts edges), **T-TS**,
**T-VERIFY-MIXED** (v2-only, v3-only, mixed-with-continuity, unknown-format
failure, per-format tamper).

### 5.2 The v3 cutover is one release gate (binding — `C-AUDIT-FORMAT-CUTOVER`)

The cutover ships as a single named release gate **R-V3** — all eight items in
one release, none earlier:

1. SQL `append_audit_row` (the only SQL writer);
2. SQL v3 `bytea` hash builder;
3. Go `V3RowHash` (byte-identical builder);
4. `VerifyRows` v2/v3 dispatch;
5. unknown-format ⇒ verifier failure;
6. operator-committed `audit.hash_format` flag, **default v2**;
7. mixed-format tests (T-HASH-PARITY, T-TS, T-VERIFY-MIXED);
8. rollback/skew runbook (skew = every post-cutover row fails identically;
   tamper = an isolated row fails).

With the flag at its default, **no SQL append can emit a v3 row**
(**T-ROLLBACK-POSTURE**: flag off ⇒ no v3 row producible and a v2-only
verifier stays green), so binary rollback remains a two-way door until the
operator deliberately flips. The flip is the forward-only point and the
runbook says so. Releasing any strict subset of R-V3 is a release-process
violation; the checklist lives in the release template.

---

## 6. Write-path hardening (carried cycle-1 contract)

- **Negative grants are schema contract (`C-GRANT-DRIFT`).** A dedicated owner
  bundle step revokes direct `INSERT` on each protected surface from
  `striatumd_rw` (per phase, §7) and grants `EXECUTE` on the write functions.
  Because grant-repair or a future `GRANT ALL` could reopen the hole, the
  revoke is a **re-asserted invariant**: any grant step (migration helper,
  `doctor repair-grants`) calls a shared `reassertWriteRevokes()` afterwards.
  Gate: **T-GRANT-DRIFT** — after migrations, role setup, and a simulated
  repair, direct `INSERT` to each protected table still fails `42501` while
  the authorized path succeeds (ordering per §10).
- **SD hardening template (`C-SD-HARDEN`).** Every owner-owned write function
  (`append_audit_row`, artifact fn, event fn, `assert_daemon_authority`) is
  created from one template: `SECURITY DEFINER` owned by the owner role;
  `SET search_path = striatumd, pg_temp` pinned via `proconfig`; fully
  qualified references; no caller-controlled dynamic SQL; `REVOKE ALL FROM
  PUBLIC` then `GRANT EXECUTE TO striatumd_rw` only. Gate: **T-SD-HARDEN**
  (inspects `pg_proc.proconfig` + ACL; hostile-`search_path` regression).
- **Authority negative path (`C-EXEC-AUTH`, cycle-1 critical).** Gate:
  **T-EXEC-AUTH** — for each write fn, calling as `striatumd_rw` with no/wrong
  `striatum.daemon_auth` raises SQLSTATE `28000` and mutates **zero** rows;
  the daemon RPC path succeeds.

---

## 7. Phased write closure (binding — `C-PHASED-WRITE-CLOSURE`)

Fixed phase nomenclature, wired into release notes, doctor posture, and tests:

| phase | protected (SD-fn-only) surfaces | doctor `pg_write_boundary` | allowed claim |
| --- | --- | --- | --- |
| **P0 `audit_only`** | `audit_log` | `audit_only` | "the audit chain is DB-enforced" |
| **P1 `audit_artifacts`** | + `artifacts` | `audit_artifacts` | + "artifact writes are DB-enforced" |
| **P2 `full`** | + `events` | `full` | **only here:** "the daemon's durable write paths are DB-enforced" |

Each phase lands with its own negative test (**T-42501-P0/P1/P2**: direct DML
to each protected surface fails `42501`; the authorized path succeeds) and its
own release-note line. Durable-event transcript exclusion
(`C-EVENT-NO-TRANSCRIPTS`, **T-EVENT-NOTRANSCRIPT**: DB-level rejection of
`stdout`/`stderr`/`transcript`/`raw_output`/provider-output payload keys and
transcript-sized payloads) lands with the events phase. RLS row-scoping on
`leases`/`sessions` lands **last**, as defense-in-depth only (§4.6).

---

## 8. Schema delivery and deployment

### 8.1 Owner bundles (binding — `C-OWNER-DDL-SPLIT`)

Owner-only DDL — `pgcrypto`, `daemon_auth_registry` (with `role_name`),
`daemon_auth_log`, `assert_daemon_authority`, the `append_*` SD functions, the
revokes, and the capability markers — ships as a **versioned owner bundle**
(`go/pkg/db/sql/owner/NNNN_*.sql`), applied out-of-band as the owner role via
a dedicated verb (`striatum daemon owner-ddl apply --owner-url …`, with the
equivalent documented `psql` invocation). RFC 0079 §5 stands: the runtime role
cannot DDL owner objects. Properties:

- **Atomic per version.** Each bundle version applies in a single transaction
  and stamps its `schema_authority` marker row **last, in the same
  transaction**. A partially-applied bundle cannot persist; bundles exclude
  non-transactional DDL (`CREATE INDEX CONCURRENTLY` is forbidden).
- **Idempotent.** Re-applying a stamped version is a no-op, exit 0.
- **Runtime separation.** `ConnectAndMigrate` (runtime role) never contains
  owner DDL; a migration-lint guard fails any runtime migration touching
  owner-only objects.
- **Startup skew check.** Boot verifies the marker/dependency set **before
  serving mutations** and fails closed naming the bundle to apply
  (`daemon_pg_schema_precondition: owner bundle vN required`).

### 8.2 Capability parity (binding — `C-DEPLOY-CAPABILITY-PARITY`)

Startup verifies the **full dependency set** before serving mutations:

- `to_regprocedure` for every required write function (per active phase) and
  `assert_daemon_authority`;
- `daemon_auth_registry` and `daemon_auth_log` present; `pgcrypto` installed;
- marker parity **both directions**: every capability the binary requires is
  stamped (new-binary/old-schema), and every stamped
  `requires_daemon_auth`-class capability is one the binary supports
  (old-binary/authority-bearing-schema).

**Sequencing makes the old-binary check real.** Release **N** ships the parity
checker while no authority schema exists anywhere (markers absent ⇒ inert
pass-through). Authority-bearing owner bundles first appear in release
**N+1**; therefore every binary that can ever meet authority-bearing schema is
≥ N and fails closed at startup on unknown required capabilities.

**Accept-as-risk (scoped):** binaries older than N cannot be retroactively
startup-gated. Bounded by (a) the runbook's pinned deploy order (binary
upgrade before owner bundle) and (b) the DB backstop: a pre-N binary writing
through revoked direct DML fails `42501` loudly on first mutation — denial,
not corruption.

**Gate.** **T-DEPLOY-SKEW** covers: new-binary/no-markers ⇒ startup refusal
naming the bundle; binary-N vs authority markers ⇒ startup refusal naming the
unknown capability; missing registry / missing assert fn / missing pgcrypto ⇒
startup refusal; interrupted bundle ⇒ no marker, no partial objects;
double-apply ⇒ idempotent no-op; pre-N-binary simulation ⇒ first write fails
`42501` with zero rows mutated.

---

## 9. L0 credential lifecycle

### 9.1 Bootstrap and rotation

At every start, over the owner PEER connection (`STRIATUM_OWNER_DB_URL` for
remote-PG/owner-without-PEER; recommended at-rest protection is systemd
`LoadCredentialEncrypted=`), the daemon rotates the `striatumd_rw` password to
a fresh `crypto/rand` value held only in RAM, then bootstraps the authority
secret (§4.1). A DSN captured before a restart fails after it.

### 9.2 Fail-closed owner dependency (`C-RESTART-OWNER-DEP`)

A transient owner failure **fails closed**: the daemon refuses to serve,
emits an owner-attributable diagnostic
(`daemon_pg_owner_bootstrap_failed: cannot reach owner DSN to rotate
striatumd_rw`), and never falls back to a stale runtime credential. Accepted
operational trade. Gate: **T-OWNER-FAILCLOSED**.

### 9.3 Single-role posture is visible (`C-L0-ADOPTION-VISIBLE`)

Where owner==runtime (the documented live posture), rotation is skipped and L0
is inert. The skip surfaces as a structured `daemon doctor` **posture
finding** `rotation_skipped_single_role`; the runbook documents the
owner≠runtime split as an L0 adoption prerequisite. The spec does not claim
the runtime credential is "made uninteresting" until that split is adopted.
Gate: **T-DOCTOR-SINGLEROLE**.

### 9.4 Role-scoped rotator detection (binding — `C-ROTATOR-PROBE-ROLE-SCOPED`)

`daemon_auth_registry` records **`role_name`** — which runtime role each
instance rotates. The concurrent-rotator probe fires only on **role
collision**: a recent rotation row for the same `role_name` from a different
`instance_id` (registry evidence; `pg_stat_activity` same-role peers remain
corroborating signal only). The sanctioned multi-host posture — per-instance
roles `striatumd_rw_<instance>` — produces distinct `role_name` values and
cannot trip the probe. The single-writer invariant is per **role**, not per
PostgreSQL.

**Gate.** **T-ROTATOR-SCOPE:** fixture A (two instances, two per-instance
roles, both recently rotated) ⇒ no finding; fixture B (two instance ids, one
shared role, recent rotations) ⇒ posture finding. Both assertions in one test.

### 9.5 Owner DR and doctor reach (`C-OWNER-DR`, `C-DOCTOR-OWNER-REACH`)

The runbook ships a break-glass path for a non-decryptable owner credential
(re-encrypt on the new host, or temporary plaintext `STRIATUM_OWNER_DB_URL`,
then rotate). The doctor posture probe is an owner-owned `SECURITY DEFINER`
function returning a boolean — never the hash or password — callable by the
runtime role; remote-PG doctor obtains owner access out-of-band.

---

## 10. Test-harness fidelity (binding — `C-PGTEST-NO-DML-GRANT`)

pgtest today builds the unprivileged role imperatively in Go
(`go/pkg/pgtest/pgtest.go:75-86`) — a false-green channel for every 42501
gate. pgtest provisioning becomes: (1) run runtime migrations; (2) apply the
**same owner bundle SQL files** production uses, as the test cluster's owner;
(3) create only the per-test **login** role and grant it membership in the
migration-defined role. The per-test naming (`striatumd_rw_<db>`) survives as
a login shell over migration-defined privileges. pgtest is prohibited from
issuing `GRANT`/`REVOKE` on protected-table DML; the §5.1 test-only raw-row
writer is the sole sanctioned bypass and patches no privileges.

**Gates.** **G-PGTEST-GRANTS:** a statement-recording wrapper around pgtest
setup fails on any `GRANT`/`REVOKE` naming a protected table that did not
originate from bundle SQL. **T-42501** runs strictly after migrations + owner
bundle + a simulated `doctor repair-grants`, against migration-defined roles
only (subsumes T-HARNESS-FIDELITY and the T-GRANT-DRIFT ordering).

---

## 11. L2 — lane PG-unreachability (#87 status: binding — `C-87-CLOSURE-GATE`)

Mechanics (normative per `SYNTHESIS_cycle_2.md` §7): dedicated PG-less lane OS
user; PostgreSQL socket relocated to a `0700` directory owned by the daemon
identity and startup-asserted (`C-SOCKET-DIR-PERMS`); loopback TCP denied via
`pg_hba`/no listener; `PGHOST` scrubbed from the lane env (the lane env is
already a strict positive allowlist). Adoption posture (`C-L2-DEFAULT`):
secure-profile/fresh installs enable the hardened posture or get a
**blocking** doctor finding; legacy upgrades keep
`security.pg_socket_hardened` default-false with a doctor **warn**; the
default-on flip is a named successor minor gated on evidence. The L2 how-to
enumerates the socket-relocation blast radius (`psql`, monitoring, owner DSNs,
pidfile assumptions) and updates daemon/doctor socket defaults in lockstep
(`C-SOCKET-RELOCATE-MIGRATION`).

**#87 status language (binding).** All status surfaces — spec, runbook, issue,
doctor text — say: **"#87: mitigated, pending lane-OS-user default."** #87
closes only when all four are live in the default or a named secure profile:

1. the dedicated PG-less lane OS user;
2. the protected `0700` socket-dir posture (startup-asserted);
3. **T-LANE-ISOLATION-NEG** green in CI — a mock lane identity fails to
   connect over **both** the protected UNIX socket and loopback TCP;
4. blocking `daemon doctor` behavior for PG-reachable lanes under the secure
   profile.

Until then doctor reports the lane posture as a finding, not a pass.

---

## 12. Privacy gates

- **`daemon_auth_log` redaction (`C-AUTH-LOG-PRIVACY`).** Owner-owned;
  `event ∈ {bootstrap, rotated, rotation_skipped_single_role,
  rotation_failed, owner_bootstrap_failed}`. `detail jsonb` is written through
  a single Go writer with a strict key whitelist (`event`, `daemon_version`,
  `instance_id`, `reason_code`, `duration_ms`) plus a DSN/credential redactor;
  raw driver errors/DSNs reduce to a `reason_code` enum. Gate:
  **T-AUTHLOG-REDACT** — no secret substring survives insertion.
- **Durable event transcript exclusion (`C-EVENT-NO-TRANSCRIPTS`).** Lands
  with the events phase (§7); keeps the DB a curated record, not a transcript
  store (AGENTS.md product boundary; D028). Gate: **T-EVENT-NOTRANSCRIPT**.
- **Statement-logging residual.** Verbose statement+parameter server logging
  can log bound values including the secret — runbook hardening item (do not
  enable on the daemon's PostgreSQL); not reachable by a `striatumd_rw`-only
  observer (§17).

---

## 13. Write-authority inventory (folds `PX3-006`)

The spec gains a normative **write-authority inventory** covering **every**
daemon-owned table (47 in `striatumd.*` today, `0001_baseline.sql` through
`0023_principals.sql`), each classified as:

- `sd_gated/P0|P1|P2` — SD-function-only writes from the named phase
  (`audit_log`, `artifacts`, `events`);
- `runtime_dml` — direct runtime-role DML retained **with one-line rationale**
  (live coordination state: `jobs`, `leases`, `sessions`, `queue_messages`,
  `work_packets`, `runs`, supervision/recovery/conversation tables, …);
- `owner_only` — runtime role holds no write privilege
  (`daemon_auth_registry`, `daemon_auth_log`, `schema_authority`, migration
  bookkeeping).

The full classified table ships with the Phase-0 PR, generated against
`information_schema` so nothing is missed. A migration-guard test fails when a
table exists in `striatumd.*` without an inventory row — future tables cannot
silently bypass the L1 model.

---

## 14. Normative gate matrix (constraint → gate → lands)

These gates are **normative, not illustrative** (discharge-review carry-forward
finding 3). A build review may not accept an implementation that renames,
weakens, or skips a gate without a recorded decision.

| constraint | sev | gates | lands |
| --- | --- | --- | --- |
| C-EXTENDED-AUTH-PRELUDE | critical | T-PRELUDE-OBSERVER, G-PRELUDE-MODE | N (guard), N+1 (observer) |
| C-AUTH-TX-WRAPPER | high | G-MUTATION-TX, T-SQL-ORDER | N |
| C-AUDIT-AUTH-PRELUDE | high | T-AUDIT-FAILCLOSED | N |
| C-OWNER-DDL-SPLIT | high | T-DEPLOY-SKEW (bundle cases) | N+1 |
| C-PGTEST-NO-DML-GRANT | high | G-PGTEST-GRANTS, T-42501 ordering | N+1 |
| C-DSN-READ-SCOPE | high | step-0 doc gate + successor issue filed | step 0 |
| C-PHASED-WRITE-CLOSURE | high | T-42501-P0/P1/P2 + posture strings | N+1 → P2 |
| C-AUDIT-FORMAT-CUTOVER | high | R-V3 checklist, T-ROLLBACK-POSTURE, T-VERIFY-MIXED, T-HASH-PARITY, T-TS | N+1 |
| C-87-CLOSURE-GATE | high | doc gate + T-LANE-ISOLATION-NEG precondition | step 0 → L2 |
| C-AUTH-WINDOW-LIVENESS | high | T-AUTH-LIVENESS | N+1 |
| C-DEPLOY-CAPABILITY-PARITY | high | T-DEPLOY-SKEW (capability cases) | N (checker), N+1 (stamps) |
| C-ROTATOR-PROBE-ROLE-SCOPED | high | T-ROTATOR-SCOPE | N+1 |
| C-EXEC-AUTH (cycle 1) | critical | T-EXEC-AUTH, T-GUC-LABEL, T-REGISTRY-ACL | N+1 |
| C-GRANT-DRIFT / C-SD-HARDEN (cycle 1) | high | T-GRANT-DRIFT, T-SD-HARDEN | N+1 |
| C-ATTR-RESET-FAIL (cycle 1) | high | T-ATTR-RESET (bounded policy) | N |
| C-RESTART-OWNER-DEP / C-L0-ADOPTION-VISIBLE (cycle 1) | high | T-OWNER-FAILCLOSED, T-DOCTOR-SINGLEROLE | N+1 |
| C-AUTH-LOG-PRIVACY / C-EVENT-NO-TRANSCRIPTS (cycle 1) | high | T-AUTHLOG-REDACT, T-EVENT-NOTRANSCRIPT | N / events phase |
| C-L2-NEG-TEST / C-L2-DEFAULT (cycle 1) | high | T-LANE-ISOLATION-NEG, T-L2-DEFAULT | L2 |
| PX3-005 (medium, folded) | — | doc-presence check in guard run | step 0 |
| PX3-006 (medium, folded) | — | inventory + unclassified-table guard | N+1 |
| OPS-12 (medium, folded) | — | T-ATTR-RESET under bounded policy + destroy-rate doctor finding | N |

---

## 15. Sequencing (release-shaped, gate-first)

0. **Docs (hard gate, `PX3-005` / discharge carry-forward 1).** Before the
   first behavior-changing PR merges: amend **D164** (authority gate replaces
   the BeforeAcquire/GUC wording; narrowed G1/G2 claims; v3 format; #87
   partial), **`docs/reference/spec.md`** (daemon→PG auth model, fail-closed
   audit contract, phase nomenclature), and **RFC 0110** (cycle-2 + revision
   deltas). File the read-scope successor issue (§2). The G-MUTATION-TX guard
   run includes a doc-presence check for the spec sentences it enforces.
1. **Release N — authority plumbing, no schema authority:** `ExecBound` +
   extended-protocol prelude (§4.2) · `BeginAuthorizedMutation` migration of
   all mutating handlers + G-MUTATION-TX + T-SQL-ORDER (§4.3) · fail-closed
   audit contract, mutation-coupled append (§4.4) · capability parity checker,
   inert (§8.2) · bounded-discard reset policy (§4.7) · doctor posture probes
   (`rotation_skipped_single_role`, `pg_write_boundary` reporting `none`) ·
   `daemon_auth_log` writer + redaction (§12).
2. **Release N+1 — owner bundle v1 + L0 + Phase 0 `audit_only`:** owner bundle
   (pgcrypto, registry with `role_name`, `daemon_auth_log`,
   `assert_daemon_authority`, `append_audit_row`, `audit_log` revoke,
   capability stamps) (§8.1) · L0 bootstrap/rotation + fail-closed owner
   dependency (§9) · pgtest re-plumb (§10) · **R-V3** as one gate, flag
   default v2 (§5.2) · gates: T-EXEC-AUTH, T-42501-P0, T-GRANT-DRIFT,
   T-PRELUDE-OBSERVER, T-AUDIT-FAILCLOSED, T-AUTH-LIVENESS, T-ROTATOR-SCOPE,
   T-DEPLOY-SKEW, T-ROLLBACK-POSTURE · write-authority inventory + guard
   (§13).
3. **Phase 1 `audit_artifacts`, then Phase 2 `full`:** same template per
   surface; transcript exclusion with the events phase; the
   sole-durable-write-path claim unlocks at P2 only (§7); RLS row-scoping
   last.
4. **L2 hardening** behind `security.pg_socket_hardened` (secure-profile
   blocking / legacy warn): T-LANE-ISOLATION-NEG, `0700` assertion, relocation
   runbook → only then may #87 close (§11).
5. **Successors:** read-scope least privilege (§19) · #88-dynamic-creds ·
   remote/multi-host (per-instance roles + owner DR + doctor owner reach).

Sequenced after the RFC 0104/0105 reliability foundation. No native code, no
wire proxy, no hosted identity (`C-NO-NATIVE`).

---

## 16. Acceptance criteria

- **L0:** a fresh start rotates `striatumd_rw` with no on-disk secret in PEER
  posture; a DSN captured before a restart fails after it; owner-bootstrap
  failure fails closed with an owner-attributable diagnostic
  (T-OWNER-FAILCLOSED); the single-role skip is a doctor posture finding
  (T-DOCTOR-SINGLEROLE); the rotator probe is role-scoped (T-ROTATOR-SCOPE).
- **L1 / authority:** a direct `striatumd_rw` call to any write fn without the
  daemon-authority secret fails without mutation (T-EXEC-AUTH); the prelude is
  invisible to same-role observers (T-PRELUDE-OBSERVER, G-PRELUDE-MODE);
  mutating handlers cannot bypass `BeginAuthorizedMutation` (G-MUTATION-TX,
  T-SQL-ORDER); audit append is atomic with its mutation and fail-closed
  (T-AUDIT-FAILCLOSED); labels carry no authority (T-GUC-LABEL,
  T-REGISTRY-ACL); direct `INSERT` stays revoked across
  migration/pgtest/repair (T-GRANT-DRIFT, T-42501-P0/P1/P2,
  G-PGTEST-GRANTS); SD hardening pinned (T-SD-HARDEN); `VerifyRows` holds
  across the v2→v3 boundary with pinned parity (T-VERIFY-MIXED,
  T-HASH-PARITY, T-TS) and the cutover is one release gate
  (R-V3, T-ROLLBACK-POSTURE); deploy skew fails fast at startup, both
  directions, full dependency set (T-DEPLOY-SKEW); authority lapses are loud
  (T-AUTH-LIVENESS).
- **Attribution (L3):** the prelude is in-transaction, extended-protocol, and
  unavoidable; labels reset across commit/rollback/cancel/timeout/panic under
  the bounded-discard policy (T-ATTR-RESET).
- **Privacy:** `daemon_auth_log` is whitelisted + redacted (T-AUTHLOG-REDACT);
  `events.payload_json` rejects transcript shapes (T-EVENT-NOTRANSCRIPT).
- **L2:** under the hardened posture a lane can open neither the socket nor
  loopback (T-LANE-ISOLATION-NEG); doctor blocks for secure profiles, warns
  for legacy (T-L2-DEFAULT); `0700` asserted at startup; relocation runbook
  shipped; #87 language per §11.
- **Docs:** step-0 amendments to D164, `docs/reference/spec.md`, and RFC 0110
  land before behavior-changing merges; each phase's claim language matches
  the doctor posture string; the decision log records each per-phase decision
  on landing.

---

## 17. Residuals (accepted, explicit, none hidden)

- **Fail-closed audit availability coupling (§4.4):** RPC availability is
  coupled to audit appendability — accepted; both live in the same PostgreSQL.
- **Pre-N binaries (§8.2):** cannot be retroactively startup-gated; bounded by
  runbook deploy order + the `42501` denial backstop (denial, not corruption).
- **G2 memory-scrape residual (§2):** a full live-process scrape yields DSN
  and secret together; bounded by L0 rotation-on-restart + L2 isolation.
- **Server statement logging (§12):** verbose statement+parameter logging can
  expose bound values — runbook hardening item.
- **Heap retention (`C-SECRET-HONEST`):** pgx retains the password (and the
  daemon retains the secret) for reconnects; the guarantee is RAM-only +
  rotation-on-restart, not zeroization.
- **`V2RowHash` permanence:** stays forever as the only reader of pre-cutover
  rows — accepted cost, not tech debt.
- **Open by design:** read confidentiality against a live runtime credential
  (successor, §19); #87 until §11's four gates; #88-dynamic-creds.

---

## 18. Rejected alternatives (of record)

- **Narrowing-only for `C-EXEC-AUTH`** — the binding gate requires a direct
  call to fail without mutation; a claim-narrowing spec cannot pass it.
- **RLS / GUCs as authority** — client-controlled labels are not a trust
  boundary; RLS stays row-scoping defense-in-depth.
- **Trusted caller-supplied `row_hash`** and **porting `encoding/json`
  canonicalization to PL/pgSQL** — the v3 length-prefixed `bytea` canonical
  removes the escaping/key-order hazard instead.
- **Whole-pool extended protocol** — multi-statement migration DDL breaks and
  every query path changes behavior in one diff; the per-call override is
  scoped to the prelude.
- **Non-SQL authority carriers** (temp-table handshake, custom protocol
  message, startup payload) — text-visible, pool-stateful, or native-code
  adjacent.
- **Runtime self-heal of a lost registry row** — would make owner connectivity
  a steady-state dependency; restart is the re-bootstrap path.
- **Inherited RFC-appendix rejects** (PG C extension/bg worker, user-namespace
  PG, seccomp socket block, PAM ppid module, quorum gate,
  `pg_notify`/`SD_LISTEN_FDS` to a client, abstract-namespace socket) —
  unchanged.

---

## 19. Successors (named)

1. **Read-scope least privilege** — dedicated read-role split (or column
   grants / read RLS) sized after the L1 phases land, when the
   write-authority inventory makes the read surface enumerable. Filed as a
   GitHub issue before the first behavior-changing PR merges,
   cross-referenced from D164's amendment.
2. **#88-dynamic-creds** — `BeforeConnect` dynamic password provider; a strict
   superset that shrinks (not eliminates) the heap window.
3. **Remote/multi-host hardening** — per-instance roles, owner DR, doctor
   owner reach, cross-host client TLS certs; deferred until a real multi-host
   deployment.
4. **L2 default-on graduation** — the named successor minor that flips
   `security.pg_socket_hardened` default-on after the how-to has shipped one
   release, T-LANE-ISOLATION-NEG is green in CI, and the relocation runbook is
   published.
