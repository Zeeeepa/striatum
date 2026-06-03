---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: revision-convener-claude-opus-4.8-001
workflow: rfc-0110-pg-auth-panel
phase: revision_synthesis
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 1
title: "RFC 0110 — Revision synthesis (cycle 1): cycle-2 constraints discharged, implementation-ready"
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/revision_synthesis/draft/REVISION_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/adjudicator/COLLABORATION_LEDGER_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/DECISION_cycle2_proceed.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
---

# RFC 0110 — Revision synthesis (cycle 1)

The republished synthesis for RFC 0110 (daemon→PostgreSQL authentication and
the database-enforced write boundary), discharging the cycle-2 adjudicated
`constraints[]` (`../../adjudication/adjudicator/COLLABORATION_LEDGER_cycle_2.md`:
**`needs_revision`**, **12 binding constraints** — 1 critical + 11 high — and
3 medium follow-ups). The workflow's revision budget was exhausted, so operator
decision **`dec_b95396ff8f414eae5dbcced5476313ad`**
(`accepted_with_follow_up`, `../../adjudication/DECISION_cycle2_proceed.md`)
directed: accept the converged cycle-2 design and carry the cycle-2 findings
as **binding implementation constraints in the published spec**. This document
is that published spec surface.

**Standing.** Where this synthesis is silent, `SYNTHESIS_cycle_2.md` remains
normative; where they disagree, **this document wins**. The working draft
(`../draft/REVISION_cycle_1.md`) carries identical normative content; this
synthesis is the authoritative copy the discharge review checks. Every
discharge below is anchored against the current tree, re-verified at revision
time: `pkg/db/connection.go:207` (pool forces
`pgx.QueryExecModeSimpleProtocol`), `pkg/db/connection.go:82` (generic
`BeginTx` on the shared Runner interface), `pkg/rpc/server.go:119-130`
(`auditErr` consulted only to attach `AuditID` — audit append is fail-open
today), `pkg/db/audit.go:82` (`RecordRPCTransport` opens its own transaction),
`pkg/pgtest/pgtest.go:75-86` (imperative Go `GRANT`/`REVOKE` role layout).
All five cycle-2 implementation findings reproduce against today's source.

---

## 1. Discharge ledger — every cycle-2 `constraints[]` row, explicitly

Disposition vocabulary (role-mandated): **fold-in** (design changed to satisfy
the constraint), **answer** (claim narrowed/clarified to be true),
**reject-with-rationale**, **accept-as-risk** (kept, documented rationale),
**defer-with-successor** (out of scope, successor named). No high/critical row
is left open without a recorded disposition; none is rejected.

| id | sev | disposition | where | discharge gate (real iff this passes) |
| --- | --- | --- | --- | --- |
| `C-EXTENDED-AUTH-PRELUDE` | critical | **fold-in** | §2.1 | **T-PRELUDE-OBSERVER** (same-role observer never sees the secret in `pg_stat_activity`/statement tracing) + **G-PRELUDE-MODE** (unit guard fails a prelude under `QueryExecModeSimpleProtocol`). |
| `C-AUTH-TX-WRAPPER` | high | **fold-in** | §2.2 | **G-MUTATION-TX** (guard fails mutating handlers using generic `BeginTx`/`withTx`) + **T-SQL-ORDER** (`set_config('striatum.daemon_auth',…)` is statement 1, before any DML/write-fn). |
| `C-AUDIT-AUTH-PRELUDE` | high | **fold-in + accept-as-risk** (availability coupling) | §2.3 | **T-AUDIT-FAILCLOSED** (forced `append_audit_row` rejection ⇒ mutation rolls back / RPC errors; never success without the audit row). |
| `C-OWNER-DDL-SPLIT` | high | **fold-in** | §2.4 | **T-DEPLOY-SKEW** (new-binary/old-schema, old-binary/premature-revoke, partial-bundle each fail with an actionable pre-mutation diagnostic). |
| `C-PGTEST-NO-DML-GRANT` | high | **fold-in** | §2.5 | **G-PGTEST-GRANTS** (guard fails any pgtest `GRANT`/`REVOKE` of protected-table DML) + **T-42501** ordering (migrations → owner bundle → grant repair → still `42501`). |
| `C-DSN-READ-SCOPE` | high | **answer + defer-with-successor** | §2.6 | Doc gate: D164/spec state the narrowed read-scope posture before implementation merge; least-privilege read split filed as a named successor at landing. |
| `C-PHASED-WRITE-CLOSURE` | high | **fold-in** | §2.7 | Phase nomenclature + doctor posture strings + per-phase **T-42501-P0/P1/P2**; the sole-durable-write-path claim is reserved to the `full` phase. |
| `C-AUDIT-FORMAT-CUTOVER` | high | **fold-in** | §2.8 | **R-V3** single release gate (8-item checklist ships atomically) + **T-ROLLBACK-POSTURE** (flag off ⇒ no v3 row producible) + **T-VERIFY-MIXED**. |
| `C-87-CLOSURE-GATE` | high | **fold-in (answer)** | §2.9 | Doc gate: spec/runbook/issue/doctor all say *“#87: mitigated, pending lane-OS-user default”* until **T-LANE-ISOLATION-NEG** is green and the hardened profile blocks PG-reachable lanes. |
| `C-AUTH-WINDOW-LIVENESS` | high | **fold-in** | §2.10 | **T-AUTH-LIVENESS** (aged registry row ⇒ writes continue under lifetime-of-instance validity; deleted/superseded row ⇒ loud owner-attributable `daemon_auth_lost`, never silent `28000`). |
| `C-DEPLOY-CAPABILITY-PARITY` | high | **fold-in + accept-as-risk** (pre-N binaries) | §2.11 | **T-DEPLOY-SKEW** capability cases (old binary vs authority-bearing schema, missing registry, missing assert fn fail at startup for all binaries ≥ release N). |
| `C-ROTATOR-PROBE-ROLE-SCOPED` | high | **fold-in** | §2.12 | **T-ROTATOR-SCOPE** (per-instance-role multi-host fixture: no finding; same-role two-instance fixture: finding). |

The two `accept-as-risk` half-dispositions are scoped and bounded inside their
fold-ins (§2.3, §2.11), per the decision artifact's direction. Mediums
(`PX3-005`, `PX3-006`, `OPS-12`) are folded in §3.

---

## 2. Constraint discharges (normative)

### 2.1 `C-EXTENDED-AUTH-PRELUDE` (critical) — the prelude never rides simple protocol

**Finding basis (verified).** `connection.go:207` sets
`DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol` pool-wide (the
comment explains why: multi-statement migration DDL + client-side quoting).
Under simple protocol pgx **interpolates bound parameters into the SQL text**,
so the cycle-2 prelude `SELECT set_config('striatum.daemon_auth', $1, true)`
would serialize the secret into `pg_stat_activity.query`, observable by any
same-role `striatumd_rw` session. The cycle-2 “parameterized ⇒ invisible”
claim was false under the daemon's actual exec mode.

**Resolution (fold-in): per-call extended-protocol override; pool default
unchanged.** pgx v5 accepts a `pgx.QueryExecMode` as the first query argument,
overriding the default for that single call. The authority/attribution prelude
— and only the prelude — issues its statements as:

```
tx.Exec(ctx, "SELECT set_config('striatum.daemon_auth', $1, true)",
        pgx.QueryExecModeExec, attr.Secret)
```

`QueryExecModeExec` drives the extended protocol (Parse/Bind/Execute): bound
values travel in Bind messages, never in query text; `pg_stat_activity.query`
shows the literal `$1`. The pool default stays simple protocol, preserving the
migration-DDL rationale at `connection.go:205-207` untouched.

Mechanically, the `TxRunner` abstraction gains one dedicated method —
`ExecBound(ctx, sql, args...)` — which always passes `QueryExecModeExec` and
is the **only** API the prelude uses. All four GUCs (`striatum.daemon_auth`,
`striatum.rpc_id`, `striatum.principal_id`, `app.session_id`) ride
`ExecBound`: the labels are not secrets, but keeping principal/session ids out
of `pg_stat_activity.query` is free here and privacy-positive.

**Alternatives rejected.** (a) Flipping the whole pool to extended protocol —
needlessly risky: multi-statement migration batches break and every query path
changes behavior in one diff. (b) Non-SQL carriers (temp-table handshake,
custom protocol message, connection-startup payload) — either still
text-visible, stateful across the pool, or native-code adjacent
(`C-NO-NATIVE`). Bound parameters over the extended protocol are the proven
secret carrier in PostgreSQL.

**Residual (carried from cycle 2, unchanged):** a server operator enabling
verbose statement+parameter logging can log bound values. Runbook hardening
item; not reachable by a `striatumd_rw`-only observer.

**Gates.**
- **T-PRELUDE-OBSERVER** (PG-gated): a second `striatumd_rw` session polls
  `pg_stat_activity` (and `pg_stat_statements` where installed) while the
  daemon path performs authorized mutations under load; asserts the secret
  value never appears in any observed query text.
- **G-PRELUDE-MODE** (unit guard): the prelude helper fails a test double
  whose exec mode resolves to `QueryExecModeSimpleProtocol`; asserts
  `ExecBound` is the sole prelude entry point.

### 2.2 `C-AUTH-TX-WRAPPER` — authority is a constructor, not a convention

**Finding basis (verified).** `connection.go:82` exposes generic
`BeginTx(ctx)` on the shared `Runner` interface; any handler can begin a
transaction and write before any prelude. A prose “run the prelude after
BeginTx” rule is unenforceable.

**Resolution (fold-in): `BeginAuthorizedMutation`.** A single constructor in
`pkg/db`:

```
BeginAuthorizedMutation(ctx, attr AuthorityContext) (MutationTx, error)
```

It begins the transaction, immediately issues the §2.1 prelude via `ExecBound`
(authority + the three attribution labels, `is_local = true`), and returns a
`MutationTx`. Enforcement is layered so the wrapper is unavoidable, not
preferred:

1. **Type-level:** the write-surface helpers (audit append, artifact append,
   event append — and their SD-function callers as phases land) accept only
   `MutationTx`, never `TxRunner`.
2. **Guard (G-MUTATION-TX):** a repo guard test (same family as the existing
   command-authority guardrails) enumerates mutating RPC handlers and fails
   any that reach `Runner.BeginTx`/`withTx` directly. Read-only paths,
   migrations, and recovery sweeps live on an explicit allowlist with
   rationale strings.
3. **SQL-order regression (T-SQL-ORDER):** a recording runner captures a
   compliant handler's statement stream and asserts
   `set_config('striatum.daemon_auth', …)` is statement 1, before the first
   DML or write-function call; a deliberately non-compliant fake handler
   fails.

L3 attribution and the L1 authority secret enter PG through the same prelude,
so “attribution set but authority forgotten” (or vice versa) is structurally
impossible. The wrapper lands in release N (§5) *before* any schema authority
exists — every mutating handler migrates while the change is still
behavior-neutral.

### 2.3 `C-AUDIT-AUTH-PRELUDE` — audit append is atomic with its mutation, and fail-closed

**Finding basis (verified).** `audit.go:82`: `RecordRPCTransport` opens **its
own** transaction, after the response is already computed; `server.go:119-130`
checks `auditErr` only to decide whether to attach `AuditID` — append failure
leaves the OK response intact. An L1 authority failure on the audit path would
today produce mutation-without-audit-row silently.

**Resolution (fold-in): two explicit modes, one contract.**

- **Mutation-coupled (the binding case).** For mutating RPCs the audit row is
  appended **inside the same `MutationTx`**, as its final write (via
  `append_audit_row` once Phase 0 lands; via direct insert before that). The
  mutation and its audit row commit or roll back **atomically** —
  success-without-audit-row is impossible by construction, and the audit
  append inherits the transaction's authority/attribution prelude — the same
  `daemon_auth` context as the mutation it records (the constraint's first
  clause).
- **Standalone (reads, denials, transport errors).** Non-mutating audit rows
  append in their own small authorized transaction. On append failure the RPC
  response is **converted to an error** (`audit_append_failed`), a structured
  diagnostic is emitted, and a doctor counter increments. `server.go`'s
  current ignore-`auditErr` path is removed; a nil `AuditRecorder` becomes a
  test-harness-only configuration, refused in production wiring.

**Accept-as-risk (scoped):** fail-closed couples RPC availability to audit
appendability. Accepted deliberately: the audit chain lives in the same
PostgreSQL as every durable surface, so “cannot append audit row” almost
always means “cannot do durable work at all”; a daemon that answers RPCs while
unable to record them would violate the provenance posture this RFC exists to
establish. The contract (fail-closed, both modes) enters the spec amendment
(§4) — this is the “explicit contract” the constraint demands.

**Gate.** **T-AUDIT-FAILCLOSED:** force `append_audit_row` (and the
pre-Phase-0 direct insert) to reject inside a mutating RPC; assert the
mutation rolled back and the RPC returned an error; force a standalone append
failure and assert the response is an error — never success with a missing
row.

### 2.4 `C-OWNER-DDL-SPLIT` — owner DDL has its own atomic, versioned delivery path

**Finding basis.** RFC 0079 §5 stands: the runtime role cannot DDL owner
objects (historically the daemon crash-looped when auto-migrations tried).
Dropping L1 functions/revokes into `ConnectAndMigrate` would either crash-loop
or silently skew.

**Resolution (fold-in).** Owner-only DDL — `pgcrypto`, `daemon_auth_registry`,
`daemon_auth_log`, `assert_daemon_authority`, the `append_*` SD functions, the
revokes, and the capability markers (§2.11) — ships as a **versioned owner
bundle** (`go/pkg/db/sql/owner/NNNN_*.sql`), applied out-of-band as the owner
role via a dedicated verb (`striatum daemon owner-ddl apply --owner-url …`,
with the equivalent documented `psql` invocation). Properties:

- **Atomic per version.** PostgreSQL DDL is transactional; each bundle version
  applies in a single transaction and stamps its `schema_authority` marker row
  **last, in the same transaction**. A partially-applied bundle cannot persist
  — the “partial owner-DDL state” of the verification matrix is structurally
  unreachable, and the test asserts exactly that (interrupt mid-bundle ⇒ no
  marker, no objects). Bundles exclude non-transactional DDL
  (`CREATE INDEX CONCURRENTLY` is forbidden in owner bundles).
- **Idempotent.** Re-applying a stamped version is a no-op with exit 0.
- **Runtime separation.** `ConnectAndMigrate` (runtime role) never contains
  owner DDL; a migration-lint guard fails any runtime migration touching
  owner-only objects.
- **Startup skew check.** Boot verifies the marker/dependency set (§2.11)
  **before serving mutations** and fails closed with the bundle version to
  apply: `daemon_pg_schema_precondition: owner bundle vN required`.

**Gate.** **T-DEPLOY-SKEW** covers: new-binary/old-schema (marker missing ⇒
startup refusal naming the bundle), old-binary/premature-revoke (§2.11),
interrupted-bundle (no marker, no partial objects), double-apply (idempotent
no-op).

### 2.5 `C-PGTEST-NO-DML-GRANT` — the harness consumes migrations, never patches privileges

**Finding basis (verified).** `pgtest.go:75-86` builds the unprivileged role
imperatively in Go (`CREATE ROLE`, blanket
`GRANT SELECT, INSERT, UPDATE, DELETE`, ad-hoc `REVOKE`s). Security tests
against that layout validate the test harness, not the production migrations —
a false-green channel for every 42501 gate.

**Resolution (fold-in).** pgtest provisioning becomes: (1) run runtime
migrations; (2) apply the **same owner bundle SQL files** production uses
(§2.4) as the test cluster's owner; (3) create only the per-test **login**
role and `GRANT` it membership in the migration-defined role. The per-test
naming (`striatumd_rw_<db>`) survives as a login shell over migration-defined
privileges. pgtest itself is prohibited from issuing `GRANT`/`REVOKE` on
protected-table DML; the cycle-2 §5.1 test-only raw-row writer remains the
sole sanctioned bypass and patches no privileges.

**Gates.** **G-PGTEST-GRANTS:** a statement-recording wrapper around pgtest
setup fails on any `GRANT`/`REVOKE` naming a protected table that did not
originate from bundle SQL. **T-42501** runs strictly after migrations + owner
bundle + a simulated `doctor repair-grants`, against migration-defined roles
only (subsumes cycle-2 **T-HARNESS-FIDELITY** and **T-GRANT-DRIFT** ordering).

### 2.6 `C-DSN-READ-SCOPE` — the claim is narrowed; least-privilege reads get a successor

**Disposition: answer + defer-with-successor.** The constraint offers two
exits; we take the honest one. RFC 0110 claims, in all operator-facing
language (D164, `docs/reference/spec.md`, the RFC, doctor strings):

> RFC 0110 prevents **unauthorized mutation** and **hash/attempt-invariant
> violation** by a runtime-credential holder. It does **not** claim read
> confidentiality against a live runtime credential in this phase:
> `striatumd_rw` retains broad `SELECT` over daemon-owned tables (artifacts,
> events, sessions, queue payloads, principals, blockers). Read exposure is
> *bounded* — a leaked DSN **string** dies at the next restart (L0 rotation),
> and a sandboxed lane cannot reach the socket at all (L2) — but a live
> credential reads what the daemon reads.

The cycle-2 “a leaked DSN is uninteresting” sentence is **retracted** as
overbroad and replaced by the bounded statement above.

**Successor (named, filed at landing):** *read-scope least privilege* — a
dedicated read-role split (or column grants / read RLS) sized after the L1
phases land, when the write-authority inventory (§3.2) makes the read surface
enumerable. Filed as a GitHub issue before the first behavior-changing PR
merges, cross-referenced from D164's amendment; until it lands, no Striatum
doc may claim private-read denial. Per the verification stage: if that
successor ever claims denial, PG-gated tests must prove raw runtime
connections cannot read the named surfaces.

### 2.7 `C-PHASED-WRITE-CLOSURE` — per-phase protected surfaces, named and tested

**Resolution (fold-in).** The L1 rollout gets fixed phase nomenclature, wired
into release notes, doctor posture, and tests:

| phase | protected (SD-fn-only) surfaces | doctor `pg_write_boundary` | allowed claim |
| --- | --- | --- | --- |
| **P0 `audit_only`** | `audit_log` | `audit_only` | “the audit chain is DB-enforced” |
| **P1 `audit_artifacts`** | + `artifacts` | `audit_artifacts` | + “artifact writes are DB-enforced” |
| **P2 `full`** | + `events` | `full` | **only here:** “the daemon's durable write paths are DB-enforced” (sole-durable-write-path claim) |

Each phase lands with its own negative test (**T-42501-P0/P1/P2**: direct DML
to each protected surface fails `42501`; the authorized path succeeds) and its
own release-note line. The sole-durable-write-path sentence is **reserved** to
P2 in every artifact (spec, D164, RFC, doctor text, README-level claims); the
spec amendment (§4) keys the claim to the doctor posture string so they cannot
drift apart.

### 2.8 `C-AUDIT-FORMAT-CUTOVER` — v3 is one release gate, not a scattering

**Resolution (fold-in).** The v3 hash cutover ships as a single named release
gate **R-V3**, all eight items in one release, none earlier:

1. SQL `append_audit_row` (the only SQL writer);
2. SQL v3 `bytea` hash builder (cycle-2 §5.1 construction);
3. Go `V3RowHash` (byte-identical builder);
4. `VerifyRows` v2/v3 dispatch (`audit.go:213` gains the switch);
5. unknown-format ⇒ **verifier failure** (never silent v2 fallback);
6. operator-committed `audit.hash_format` flag, **default v2**;
7. mixed-format tests (**T-HASH-PARITY**, **T-TS**, **T-VERIFY-MIXED**:
   v2-only, v3-only, mixed-with-continuity, unknown-format, per-format
   tamper);
8. rollback/skew runbook (skew = every post-cutover row fails identically;
   tamper = isolated row fails).

With the flag at its default, **no SQL append can emit a v3 row**
(**T-ROLLBACK-POSTURE** asserts no v3 row is producible flag-off and a v2-only
verifier stays green), so binary rollback remains a two-way door until the
operator deliberately flips — the flip is the forward-only point and the
runbook says so. Releasing any strict subset of R-V3 is a release-process
violation; the checklist lives in the release template.

### 2.9 `C-87-CLOSURE-GATE` — #87 stays “mitigated, pending lane-OS-user default”

**Resolution (fold-in/answer).** All #87 status language — spec, runbook,
issue, doctor text, and cycle-2 §7.5's “structural close of #87” sentence — is
amended to: **“mitigated, pending lane-OS-user default.”** #87 closes only
when **all four** are live in the default or a named secure profile:

1. the dedicated PG-less lane OS user;
2. the protected `0700` socket-dir posture (startup-asserted);
3. **T-LANE-ISOLATION-NEG** green in CI (socket *and* loopback denied);
4. blocking `daemon doctor` behavior for PG-reachable lanes under the secure
   profile.

Until then doctor reports the lane posture as a finding, not a pass. The
operator brief's “partial” record is the source of truth this language now
matches. This also implements the decision artifact's “soften premature #87
closure language” follow-up verbatim.

### 2.10 `C-AUTH-WINDOW-LIVENESS` — lifetime-of-instance validity; lapses are loud

**Resolution (fold-in).** The cycle-2 §3.2 “freshness window” clause is
**deleted** — it was the self-wedge OPS-9 predicted. The lifecycle is now
explicit:

- **Validity = lifetime of instance.** `assert_daemon_authority` matches the
  presented secret against the registry digest for the instance, with **no
  TTL/freshness check in the hot path**. A correct secret can never age out
  while its daemon lives; there is **no periodic refresh**, hence no
  refresh-failure wedge — the failure mode OPS-9 feared is structurally
  absent, not handled.
- `rotated_at` is **diagnostic only**: consumed by `daemon doctor` and the
  rotator probe (§2.12), never by the assert path.
- **Authority lapse is loud.** If the registry row is missing, wiped, or
  superseded (a new same-role rotator UPSERTed over it), write functions raise
  `28000`; the daemon maps that SQLSTATE on the authorized path to a
  structured, owner-attributable `daemon_auth_lost` error naming the probable
  cause (“registry row for instance `<id>` missing/superseded — restart the
  daemon to re-bootstrap, or check for a concurrent rotator”) plus a doctor
  finding. Never a bare/silent `28000` write failure. Runtime self-heal via a
  live owner connection is **rejected** for 0110 (it would make owner
  connectivity a steady-state dependency rather than a bootstrap one); restart
  is the re-bootstrap path, consistent with cycle-2 §5.5 fail-closed.

**Gate.** **T-AUTH-LIVENESS:** (a) age `rotated_at` arbitrarily ⇒ authorized
writes keep succeeding (no TTL wedge); (b) delete/overwrite the registry row ⇒
next authorized write fails as structured `daemon_auth_lost` with the
diagnostic and doctor finding, and no mutation lands.

### 2.11 `C-DEPLOY-CAPABILITY-PARITY` — startup parity over the full dependency set

**Resolution (fold-in).** The owner bundle stamps **capability markers**
(`schema_authority(capability, version, applied_at)` rows, stamped atomically
with their DDL, §2.4). Startup verifies the **full dependency set** before
serving mutations:

- `to_regprocedure` for every required write function (per active phase) and
  `assert_daemon_authority`;
- `daemon_auth_registry` (and `daemon_auth_log`) present;
- `pgcrypto` installed;
- marker parity **both directions**: every capability the binary requires is
  stamped (new-binary/old-schema), and every stamped
  `requires_daemon_auth`-class capability is one the binary supports
  (old-binary/authority-bearing-schema).

**Sequencing makes the old-binary check real.** Release **N** ships the parity
checker while no authority schema exists anywhere (markers absent ⇒ inert
pass-through). Authority-bearing owner bundles first appear in release
**N+1**. Therefore every binary that can ever meet authority-bearing schema is
≥ N and **fails closed at startup** on unknown required capabilities.

**Accept-as-risk (scoped):** binaries **older than N** cannot be made to fail
at startup retroactively. Bounded by (a) the runbook's pinned deploy order
(binary upgrade *before* owner bundle), and (b) the DB itself: a pre-N binary
writing through revoked direct DML fails `42501` loudly on first mutation —
denial, not corruption. **T-DEPLOY-SKEW** asserts that backstop explicitly
rather than pretending pre-N startup coverage exists.

**Gate.** **T-DEPLOY-SKEW** (capability cases): new-binary/no-markers ⇒
startup refusal naming the bundle; binary-N/authority-markers ⇒ startup
refusal naming the unknown capability; missing registry / missing assert fn /
missing pgcrypto ⇒ startup refusal; pre-N-binary simulation ⇒ first write
fails `42501` with zero rows mutated.

### 2.12 `C-ROTATOR-PROBE-ROLE-SCOPED` — collision is a role property, not an instance property

**Resolution (fold-in).** `daemon_auth_registry` gains a **`role_name`**
column recording which runtime role each instance rotates. The
concurrent-rotator probe fires only on **role collision**: a recent rotation
row for the **same `role_name`** from a **different `instance_id`** (registry
evidence; `pg_stat_activity` same-role peers remain corroborating signal
only). The sanctioned multi-host posture — per-instance roles
(`striatumd_rw_<instance>`, cycle-2 §5.8) — produces distinct `role_name`
values and **cannot** trip the probe.

**Gate.** **T-ROTATOR-SCOPE:** fixture A (two instances, two per-instance
roles, both recently rotated) ⇒ no finding; fixture B (two instance ids, one
shared role, recent rotations) ⇒ posture finding. Both assertions live in the
same test so scope regressions are visible.

---

## 3. Medium follow-ups (folded, per the adjudicator's direction)

### 3.1 `PX3-005` — docs before behavior (fold-in)

Sequencing step 0 (§5) is a **hard gate**: before the first behavior-changing
PR merges, amend (a) **D164** (decision log) — authority gate replaces the
BeforeAcquire/GUC wording, narrowed G1/G2 claims, v3 format, #87 partial; (b)
**`docs/reference/spec.md`** — daemon→PG auth model, the fail-closed audit
contract (§2.3), phase nomenclature (§2.7); (c) **RFC 0110** — cycle-2 plus
this revision's deltas. The PR that lands release-N code must be preceded by
(or contain, doc-commits-first) these amendments; the G-MUTATION-TX guard run
includes a doc-presence check for the spec sentences it enforces.

### 3.2 `PX3-006` — whole-schema write-authority inventory (fold-in)

The spec gains a normative **write-authority inventory** covering **every**
daemon-owned table (47 in `striatumd.*` today, `0001_baseline.sql` through
`0023_principals.sql`), each classified as one of:

- `sd_gated/P0|P1|P2` — SD-function-only writes from the named phase
  (`audit_log`, `artifacts`, `events`);
- `runtime_dml` — direct runtime-role DML retained **with one-line rationale**
  (live coordination state: `jobs`, `leases`, `sessions`, `queue_messages`,
  `work_packets`, `runs`, supervision/recovery/conversation tables, …);
- `owner_only` — runtime role holds no write privilege
  (`daemon_auth_registry`, `daemon_auth_log`, `schema_authority`, migration
  bookkeeping).

The full classified table ships with the Phase-0 PR (generated against
`information_schema` so nothing is missed). A migration-guard test fails when
a table exists in `striatumd.*` without an inventory row — **future tables
cannot silently bypass the L1 model**, which is the finding's point.

### 3.3 `OPS-12` — bounded discard, no reconnect storms (fold-in)

Cycle-2 §4.3 destroyed the connection on *every* tx error or cancellation.
Narrowed:

- **Destroy** only when session state is ambiguous: error/cancel/timeout
  **mid-prelude**, handler panic, or unknown transaction outcome. A clean
  `ROLLBACK` clears `SET LOCAL` GUCs by PostgreSQL semantics — the connection
  returns to the pool normally (the `AfterRelease` `DISCARD ALL` backstop
  stays).
- **Bound the blast radius:** connection re-establishment uses jittered
  backoff, and a destroy-rate ceiling (doctor finding past N destroys/minute)
  turns a mass-cancel or PG-stress event into a visible posture signal instead
  of a reconnect storm colliding with the rotated password.
- **T-ATTR-RESET** keeps all five cases (commit, rollback, cancel, timeout,
  panic) green under the narrowed policy — the reset property is unchanged;
  only the *mechanism* for the clean cases relaxes from destroy to
  SET-LOCAL-semantics + DISCARD.

---

## 4. Normative amendments to `SYNTHESIS_cycle_2.md`

| cycle-2 section | amendment |
| --- | --- |
| §3.1 (claim) | “Leaked DSN string is uninteresting” → the §2.6 bounded statement; G2 wording keyed to phase nomenclature (§2.7). |
| §3.2 (registry) | “within the registry's freshness window” **deleted** → lifetime-of-instance validity + loud-lapse contract (§2.10); registry gains `role_name` (§2.12). |
| §4.1 (prelude) | Code sketch now issues via `ExecBound` (extended protocol, §2.1) inside `BeginAuthorizedMutation` (§2.2); a plain `tx.Exec` prelude is non-conforming. |
| §4.3 (reset) | Destroy-on-any-error → §3.3 bounded policy. |
| §5.2/§7.1 (grants/harness) | pgtest provisioning and T-42501/T-GRANT-DRIFT ordering per §2.5. |
| §5.7 (deploy order) | Startup precondition expands to full capability parity + two-release sequencing (§2.11); owner DDL delivery formalized as atomic stamped bundles (§2.4). |
| §5.8 (single-writer) | Doctor probe re-keyed to role collision (§2.12). |
| §6.1/§6.2 (privacy) | Unchanged — no cycle-2 constraint touches them. |
| §7.5 (#87) | “Structural close of #87” → “close **path**; status: mitigated, pending lane-OS-user default” (§2.9). |
| §9 (sequencing) | Replaced by §5 below. |
| §10 (acceptance) | Audit-append acceptance adds the fail-closed contract (§2.3); L1 acceptance reads per-phase (§2.7). |

Everything else in `SYNTHESIS_cycle_2.md` (v3 hash construction, SD-hardening
template, privacy gates, L2 mechanics, rejected alternatives, residue) stands
as written.

---

## 5. Sequencing (release-shaped, gate-first)

0. **Docs (PX3-005 hard gate):** D164 + `docs/reference/spec.md` + RFC 0110
   amendments; file the read-scope successor issue (§2.6).
1. **Release N — authority plumbing, no schema authority:** `ExecBound` +
   extended-protocol prelude (§2.1) · `BeginAuthorizedMutation` migration of
   all mutating handlers + G-MUTATION-TX + T-SQL-ORDER (§2.2) · fail-closed
   audit contract, mutation-coupled append (§2.3) · capability parity checker,
   inert (§2.11) · bounded-discard reset policy (§3.3) · doctor posture probes
   (`rotation_skipped_single_role`, `pg_write_boundary` reporting `none`) ·
   `daemon_auth_log` writer + redaction.
2. **Release N+1 — owner bundle v1 + L0 + Phase 0 `audit_only`:** owner bundle
   (pgcrypto, registry **with `role_name`**, `daemon_auth_log`,
   `assert_daemon_authority`, `append_audit_row`, `audit_log` revoke,
   capability stamps) (§2.4) · L0 bootstrap/rotation + fail-closed owner
   dependency · pgtest re-plumb (§2.5) · **R-V3** as one gate, flag default v2
   (§2.8) · gates: T-EXEC-AUTH, T-42501-P0, T-GRANT-DRIFT, T-PRELUDE-OBSERVER,
   T-AUDIT-FAILCLOSED, T-AUTH-LIVENESS, T-ROTATOR-SCOPE, T-DEPLOY-SKEW,
   T-ROLLBACK-POSTURE · write-authority inventory + guard (§3.2).
3. **Phase 1 `audit_artifacts`, then Phase 2 `full`:** same template per
   surface; transcript exclusion with the events phase; the
   sole-durable-write-path claim unlocks at P2 only (§2.7); RLS row-scoping
   last.
4. **L2 hardening** behind `security.pg_socket_hardened` (secure-profile
   blocking / legacy warn): T-LANE-ISOLATION-NEG, `0700` assertion, relocation
   runbook → only then may #87 close (§2.9).
5. **Successors:** read-scope least privilege (§2.6) · #88-dynamic-creds ·
   remote/multi-host (per-instance roles + owner DR + doctor owner reach).

Still sequenced after the RFC 0104/0105 reliability foundation; still no
native code, no wire proxy, no hosted identity (`C-NO-NATIVE`).

---

## 6. Consolidated constraint → gate matrix

| constraint | gates | lands |
| --- | --- | --- |
| C-EXTENDED-AUTH-PRELUDE | T-PRELUDE-OBSERVER, G-PRELUDE-MODE | N (guard), N+1 (observer) |
| C-AUTH-TX-WRAPPER | G-MUTATION-TX, T-SQL-ORDER | N |
| C-AUDIT-AUTH-PRELUDE | T-AUDIT-FAILCLOSED | N |
| C-OWNER-DDL-SPLIT | T-DEPLOY-SKEW (bundle cases) | N+1 |
| C-PGTEST-NO-DML-GRANT | G-PGTEST-GRANTS, T-42501 ordering | N+1 |
| C-DSN-READ-SCOPE | doc gate + successor issue filed | step 0 |
| C-PHASED-WRITE-CLOSURE | T-42501-P0/P1/P2 + posture strings | N+1 → P2 |
| C-AUDIT-FORMAT-CUTOVER | R-V3 checklist, T-ROLLBACK-POSTURE, T-VERIFY-MIXED, T-HASH-PARITY, T-TS | N+1 |
| C-87-CLOSURE-GATE | doc gate + T-LANE-ISOLATION-NEG precondition | step 0 → L2 |
| C-AUTH-WINDOW-LIVENESS | T-AUTH-LIVENESS | N+1 |
| C-DEPLOY-CAPABILITY-PARITY | T-DEPLOY-SKEW (capability cases) | N (checker), N+1 (stamps) |
| C-ROTATOR-PROBE-ROLE-SCOPED | T-ROTATOR-SCOPE | N+1 |
| PX3-005 | doc-presence check in guard run | step 0 |
| PX3-006 | inventory + unclassified-table guard | N+1 |
| OPS-12 | T-ATTR-RESET under bounded policy + destroy-rate doctor finding | N |

---

## 7. Residue (explicit, none hidden)

- **§2.3:** fail-closed audit couples availability to audit appendability —
  accepted; both live in the same PG.
- **§2.11:** binaries older than release N meet authority schema with only the
  `42501` backstop + runbook ordering — accepted, denial-not-corruption.
- **Carried from cycle 2 unchanged:** G2 memory-scrape residual (bounded by L0
  rotation + L2 isolation); verbose statement+parameter server logging can
  expose bound values (runbook item); pgx heap retention of password and
  secret (`C-SECRET-HONEST`, RAM-only + rotation-on-restart, not zeroization);
  `V2RowHash` remains permanently as the pre-cutover reader.
- **Open by design:** read confidentiality against a live runtime credential
  (successor, §2.6); #87 (open until §2.9's four gates); #88-dynamic-creds.
