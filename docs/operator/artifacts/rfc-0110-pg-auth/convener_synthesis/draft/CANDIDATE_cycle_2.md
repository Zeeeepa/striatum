# RFC 0110 — Candidate synthesis (cycle 2): implementation-ready spec

author: convener-claude-opus-4.8-003
artifact_kind: handoff
logical_name: convener_candidate_cycle_2
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 2
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/synthesis/CROSS_EXAM_SYNTHESIS_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/adjudicator/COLLABORATION_LEDGER_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/intake/INTAKE_cycle_1.md
  - docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md

This is the **cycle-2 revision** of the RFC 0110 candidate synthesis. Cycle 1 was
sent to `needs_revision` by the adjudicator (`COLLABORATION_LEDGER_cycle_1.md`,
verdict `needs_revision`) with **16 binding constraints** + **1 unresolved
question** (`Q-DYNAMIC-CREDENTIALS`) + **11 medium follow-ups** to fold in. The
prior cycle's `constraints[]` are binding input; this document **discharges each
row explicitly** (§1 ledger) and then carries the design changes that make each
discharge real (§2–§10). Per the convener role, dialogue completion is **not**
acceptance — the adjudicator ledger decides whether the gate clears.

The single load-bearing change vs. cycle 1: the **critical** `C-EXEC-AUTH`
finding is closed by **both** narrowing the product claim **and** adding a
non-spoofable, RAM-only **daemon-authority gate** inside every `SECURITY DEFINER`
write function (§2). That one mechanism also discharges `C-GUC-NONAUTH` (the
attribution GUCs become *labels only*; authority is the verified secret) and
anchors the L3 transaction prelude (`C-TX-GUC-PRELUDE`).

---

## 1. Discharge ledger (every binding row, explicitly)

Disposition vocabulary: **folded** (design changed to satisfy it), **narrowed**
(claim lowered to be true), **accepted-as-risk** (kept with rationale),
**deferred-with-owner** (out of scope this RFC, successor named). Each binding
row carries the falsifiable gate that proves the discharge — these are the rows
the cross-examiner should attack.

### 1.1 Binding constraints (16) + the unresolved question

| id | sev | disposition | where | falsifiable gate (the discharge is real iff this passes) |
| --- | --- | --- | --- | --- |
| `C-EXEC-AUTH` | critical | **folded + narrowed** | §2 | **T-EXEC-AUTH:** as `striatumd_rw` with no `striatum.daemon_auth` GUC (or a wrong one), every `SECURITY DEFINER` write fn `RAISE`s and mutates **0 rows** of `audit_log`/`artifacts`/`events`; the daemon RPC path (which sets the verified secret in-tx) succeeds. |
| `C-GUC-NONAUTH` | high | **folded** | §2.4, §3 | **T-GUC-LABEL:** as `striatumd_rw`, set fake `striatum.rpc_id`/`principal_id`/`app.session_id` → every protected write still fails (authority ≠ labels); a daemon RPC sets/clears the same labels purely for attribution. |
| `C-TX-GUC-PRELUDE` | high | **folded** | §3 | **T-TX-PRELUDE:** a guard test fails any mutating tx whose first statements omit the attribution+authority prelude; commit **and** rollback paths leave the next checkout with no labels. |
| `C-GRANT-DRIFT` | high | **folded** | §4.2 | **T-GRANT-DRIFT:** after migrations, `pgtest` role setup, **and** a simulated doctor grant-repair, direct `INSERT` to each protected table still fails `42501`. |
| `C-SD-HARDEN` | high | **folded** | §4.3 | **T-SD-HARDEN:** `pg_proc.proconfig` shows `search_path=striatumd, pg_temp`, ACL grants `EXECUTE` only to `striatumd_rw`, `PUBLIC` is revoked; a hostile-`search_path` regression still reaches the intended `striatumd` objects. |
| `C-AUTH-LOG-PRIVACY` | high | **folded** | §6 | **T-AUTHLOG-REDACT:** feeding DSNs/driver errors/passwords/tokens/params to the `daemon_auth_log` writer inserts **only** whitelisted, redacted keys; no secret substring survives. |
| `C-EVENT-NO-TRANSCRIPTS` | high | **folded** | §7 | **T-EVENT-NOTRANSCRIPT:** a DB `CHECK`/trigger rejects `events.payload_json` carrying `stdout`/`stderr`/`transcript`/`raw_output`/provider-output keys; curated metadata events insert green. |
| `C-HARNESS-PRIVILEGES` | high | **folded** | §9.1 | **T-HARNESS-FIDELITY:** harness setup fails if `pgtest` issues ad-hoc `GRANT`/`REVOKE` on protected tables; `T-42501` runs against migration-defined roles only. |
| `C-L2-NEG-TEST` | high | **folded** | §8.2 | **T-LANE-ISOLATION-NEG:** a mock lane identity attempting connect over the protected UNIX socket **and** loopback TCP fails under the hardened posture. |
| `C-ATTR-RESET-FAIL` | high | **folded** | §3.3 | **T-ATTR-RESET:** labels are gone on the next checkout after commit, rollback, `context` cancel, statement timeout, and a handler panic; dirty connections are discarded. |
| `C-RESTART-OWNER-DEP` | high | **folded + accepted-as-risk** | §5.2 | **T-OWNER-FAILCLOSED:** simulated owner-bootstrap failure → daemon refuses to serve, emits an owner-attributable diagnostic, and never falls back to a stale runtime credential. |
| `C-L0-ADOPTION-VISIBLE` | high | **folded** | §5.3 | **T-DOCTOR-SINGLEROLE:** on a single-role fixture, `daemon doctor` reports `rotation_skipped_single_role` as a **posture finding** (not just a WARN); the runbook names the owner≠runtime split as an L0 prerequisite. |
| `C-ROLLBACK-FORWARD-ONLY` | high | **folded** | §4.5 | **T-ROLLBACK-POSTURE:** v3 writes are gated behind an operator-committed `audit.hash_format=v3` flag; with it off, no v3 row exists, so a v2-only verifier on rollback sees a green chain; the runbook distinguishes format-skew from tamper. |
| `C-DDL-DEPLOY-ORDER` | high | **folded** | §4.4 | **T-DEPLOY-SKEW:** new-binary/old-schema (missing `append_audit_row`) and old-binary/premature-`REVOKE` both fail a **startup precondition** with an actionable error before serving mutations. |
| `C-ROTATION-SINGLE-WRITER` | high | **folded** | §5.4 | **T-SINGLE-WRITER:** the spec declares one daemon per `striatumd_rw` role + doctor detection of a concurrent rotator; remote/multi-host uses a per-instance role. Stated invariant + doctor probe. |
| `C-L2-DEFAULT` | high | **folded** | §8.1 | **T-L2-DEFAULT:** a secure-profile/fresh install either enables the hardened posture or emits a **blocking** doctor finding; a legacy upgrade **warns** until the named `vX.Y` default-on release. |
| `Q-DYNAMIC-CREDENTIALS` | high (question) | **deferred-with-owner** | §5.5 | Disposition recorded: deferred to successor **#88-dynamic-creds**, `C-SECRET-HONEST` preserved verbatim (RAM-only + rotation-on-restart, not live-memory secrecy). Optional `BeforeConnect` provider noted as a strict superset, not required for 0110. |

### 1.2 Medium follow-ups folded into the high constraints

Per the adjudicator's "fold where practical": `C-HASH-V3`, `C-HASH-BYTEA`,
`C-SHA-PRIMITIVE`, `C-VERIFY-DISPATCH`, `C-TZ-INDEPENDENT-HASH`, `C-TEST-ROW-WRITER`
are folded into **§4.1 (the v3 hash contract)**; `C-GUC-PARAMETERIZED` into **§3.2**;
`C-SOCKET-DIR-PERMS` and `C-SOCKET-RELOCATE-MIGRATION` into **§8.3/§8.4**;
`C-OWNER-DR` and `C-DOCTOR-OWNER-REACH` into **§5.6/§5.7**. Each appears as a named
test in its section.

### 1.3 Candidate constraints carried forward unchanged

`C-INSERT-REVOKE`, `C-OWNER-DDL`, `C-UPGRADE-SAFE`, `C-NO-NATIVE`, `C-ATTR-RESET`
(strengthened by `C-ATTR-RESET-FAIL`), `C-SECRET-HONEST`, `C-HASH` (sharpened by
the §4.1 medium cluster) remain binding from cycle 1.

---

## 2. The critical fix: a non-spoofable daemon-authority gate (`C-EXEC-AUTH`, `C-GUC-NONAUTH`)

**The cycle-1 hole (PX-001, critical).** Revoking direct table DML routes writes
through owner-owned `SECURITY DEFINER` functions, but `striatumd_rw` holds
`EXECUTE`, so a process with **only the runtime DSN** can call `append_audit_row`
(later the artifact/event functions) directly and produce a **hash-correct but
unauthorized** write. In-DB hashing makes the row well-formed, not daemon-issued.
The binding `C-EXEC-AUTH.verification.gate` *requires* a negative test where a
direct `striatumd_rw` call **fails without mutation** — so narrowing the claim
alone cannot pass it. We must add a gate. We do both: narrow the claim to what is
true, and add the gate the verification demands.

### 2.1 The narrowed product claim (honest scope)

RFC 0110's headline is restated as two separable guarantees:

- **G1 — invariant integrity (DB-enforced):** a leaked runtime credential cannot
  violate the append-only / hash-chain / attempt-scope / segment invariants,
  because the *only* write path is the owner-owned function that computes the
  hash and holds the chain lock in-DB. A leaked DSN cannot forge a chain-breaking
  or attempt-violating row.
- **G2 — daemon-issuance (authority gate, §2.2):** a write only succeeds if the
  caller presents the **per-process daemon authority secret**; a process holding
  only the DSN *string* cannot mutate, because it does not hold that secret.

The **residual** is stated plainly: G2 does **not** survive a full live-process
memory scrape of a running daemon (the scrape yields both the DSN *and* the
secret). That residual is owned by **L0** (rotation-on-restart kills any scraped
secret at the next restart, §5) and **L2** (a sandboxed lane never reaches the
socket at all, §8). We do **not** claim the runtime credential is unconditionally
"uninteresting"; we claim a **leaked DSN string is** uninteresting, and a
memory-scrape window is bounded by restart. This is the D164 intent, made true.

### 2.2 Mechanism: `striatum.daemon_auth`, an owner-only RAM secret

At bootstrap, over the **owner** connection (the same PEER path as migrations),
`striatumd`:

1. generates `auth_secret` via `crypto/rand` (RAM-only, never on disk/env/DSN);
2. computes `digest = sha256(auth_secret || per-instance salt)` and `UPSERT`s
   `(instance_id, digest, rotated_at)` into a new **owner-owned** table
   `striatumd.daemon_auth_registry` — `striatumd_rw` is granted **no privilege**
   on it (cannot `SELECT` the digest, cannot write it);
3. holds `auth_secret` in process memory for the pool's lifetime (same lifetime
   and honesty caveat as the rotated password, §5.5 / `C-SECRET-HONEST`).

Every owner-owned `SECURITY DEFINER` write function begins with a call to an
owner-owned helper:

```
striatumd.assert_daemon_authority()  -- SECURITY DEFINER, owner-owned
  -- reads current_setting('striatum.daemon_auth', true)  (missing -> NULL)
  -- compares sha256(presented || salt) to daemon_auth_registry.digest
  --   for the current instance, within the registry's freshness window
  -- RAISE EXCEPTION 'striatum_daemon_auth_required' (SQLSTATE 28000) on miss
```

Because the registry digest is readable **only** by the function (running as
owner), and `auth_secret` never leaves daemon RAM, a `striatumd_rw` caller cannot:
forge the secret (it is `crypto/rand`), read the digest (no grant), or learn it
from the function (it only returns void or raises). The daemon RPC path sets the
secret in the transaction prelude (§3) via **parameterized** `set_config` so it
never appears in `pg_stat_activity` query text.

### 2.3 Why this is the right shape (and what it is not)

- It is **not** a second password handshake on the wire — no native code, no
  proxy (`C-NO-NATIVE` holds). It is a transaction-local GUC verified server-side.
- It is **not** RLS-as-authority: RLS (cycle 1 §1 "second tier") stays
  defense-in-depth and is explicitly demoted to *row-scoping*, never the trust
  boundary (`C-GUC-NONAUTH`).
- It **raises the bar** from "possess a credential string that leaks through
  config files, env, logs, or a captured DSN" to "possess a RAM-only capability
  the daemon never serializes." That is a real, testable step, bounded by §2.1.

### 2.4 GUCs are labels; the secret is authority (`C-GUC-NONAUTH`)

`striatum.rpc_id`, `striatum.principal_id`, `app.session_id` are **attribution
labels only** — the write functions read them solely to store provenance, never
to authorize. The **only** authority-bearing GUC is `striatum.daemon_auth`, and
it bears authority because it is verified against an owner-only secret a raw SQL
caller cannot spoof. RLS on `leases`/`sessions` (if enabled) uses
`current_setting('app.session_id', true)` for **row scoping under an already
daemon-authorized session**, not as an auth gate.

### 2.5 Falsifiable tests

- **T-EXEC-AUTH** (binding `C-EXEC-AUTH` gate): for each of `append_audit_row`,
  the artifact write fn, the event write fn — call as `striatumd_rw` with (a) no
  `striatum.daemon_auth`, (b) a wrong secret; assert `RAISE` with SQLSTATE
  `28000` and **zero** new rows (`SELECT count(*)` unchanged). Then the daemon RPC
  path (secret set in-tx) succeeds and the row appears.
- **T-GUC-LABEL** (binding `C-GUC-NONAUTH` gate): as `striatumd_rw`, set fake
  `rpc_id`/`principal_id`/`app.session_id` and call each write fn without the
  secret → still fails; with the secret (daemon path) → succeeds and the labels
  are stored verbatim for attribution.
- **T-REGISTRY-ACL:** assert `has_table_privilege('striatumd_rw',
  'striatumd.daemon_auth_registry', 'SELECT')` is **false**.

---

## 3. L3 attribution & authority prelude — inside the transaction (`C-TX-GUC-PRELUDE`, `C-GUC-PARAMETERIZED`, `C-ATTR-RESET-FAIL`)

### 3.1 Placement correction (IX-001)

Cycle 1 put L3 in `pgxpool.BeforeAcquire` (`connection.go:208` builds the pool
plainly today, no hooks). `BeforeAcquire` fires **before** any mutation
transaction, so a `SET LOCAL` invariant cannot hold there. Corrected design: a
shared **in-transaction prelude** run immediately after `BeginTx`, before the
first DML, by every mutating handler:

```
func applyAttribution(ctx, tx, attr) error {
    // parameterized — values never enter pg_stat_activity query text
    tx.Exec(ctx, "SELECT set_config('striatum.daemon_auth', $1, true)", attr.Secret)
    tx.Exec(ctx, "SELECT set_config('striatum.rpc_id',       $1, true)", attr.RPCID)
    tx.Exec(ctx, "SELECT set_config('striatum.principal_id', $1, true)", attr.PrincipalID)
    tx.Exec(ctx, "SELECT set_config('app.session_id',        $1, true)", attr.SessionID)
}
```

`is_local = true` scopes all four to the transaction; commit or rollback clears
them automatically (no cross-checkout bleed by construction). The `daemon_auth`
secret rides the same prelude so authority and attribution are set atomically
before any write fn runs.

### 3.2 No statistics leakage (`C-GUC-PARAMETERIZED`, PR-003)

All GUCs are set via `SELECT set_config($1,$2,true)` with **bound parameters**,
not string-concatenated `SET LOCAL`. This keeps `principal_id` **and** the
`daemon_auth` secret out of `pg_stat_activity.query`. Residual posture note: an
operator who enables `log_statement=all` with parameter logging could still log
bound values — documented in the L0 runbook as an operator hardening item (do not
enable verbose statement+parameter logging on the daemon's PG).

### 3.3 Reset robustness across failure paths (`C-ATTR-RESET-FAIL`, EV-004)

Transaction-local GUCs vanish on commit/rollback, but to defend pool reuse under
abort/cancel/panic we add an `AfterRelease` hook issuing `DISCARD ALL`
(clears all session GUCs + temp state) and, on any transaction that errored or
was cancelled, the handler **discards the connection** (`pgxpool` destroy) rather
than returning it. So even a panicked handler cannot return a connection carrying
a stale `principal_id` or `daemon_auth`.

- **T-TX-PRELUDE** (binding `C-TX-GUC-PRELUDE` gate): a guard test wraps a fake
  mutating handler whose first statement is a write — assert it fails the prelude
  guard; a compliant handler passes; after commit and after rollback, a fresh
  checkout reads `current_setting('striatum.principal_id', true)` as NULL.
- **T-ATTR-RESET** (binding `C-ATTR-RESET-FAIL` gate): set labels, then force each
  of {commit, rollback, `context` cancel, statement timeout, handler panic};
  assert the next checkout observes no prior `rpc_id`/`principal_id`/`app.session_id`
  and no `daemon_auth`.

---

## 4. L1 — PostgreSQL enforces the write contract

### 4.1 The v3 hash contract (folds `C-HASH-V3`/`C-HASH-BYTEA`/`C-SHA-PRIMITIVE`/`C-VERIFY-DISPATCH`/`C-TZ-INDEPENDENT-HASH`/`C-TEST-ROW-WRITER`)

Cycle 1's v3 length-prefixed canonical (same 15 fields, fixed declared order,
`octet_length:bytes` strings, `-1:` null, RFC3339 second-truncated UTC ts) is
retained and **sharpened to executable contract**:

- **`C-HASH-V3` (supersession, normative):** v3 is the accepted format for **new**
  rows and **supersedes** the survey's "byte-identical to Go `encoding/json`"
  wording. `V2RowHash` (`audit.go:25`, `json.Marshal` of the 15-field map) is
  preserved **permanently** as the only reader of pre-cutover rows. The
  decision-log entry on landing names the mixed-format invariant.
- **`C-HASH-BYTEA` (construction):** the PL/pgSQL side builds hashed material as
  **`bytea`**, not text concat: each field is `convert_to(value, 'UTF8')`,
  length-prefixed with the **byte** length (`octet_length(convert_to(...))`),
  integers as their decimal ASCII string length-prefixed, null as the 3 bytes
  `-1:`. Go's `V3RowHash` builds the identical `[]byte`. No implicit casts.
- **`C-SHA-PRIMITIVE` (pinned):** hashing uses **`pgcrypto`'s `digest(bytea,
  'sha256')`**; the owner migration runs `CREATE EXTENSION IF NOT EXISTS pgcrypto`
  and a startup precondition (§4.4) asserts it is present *before* any `REVOKE`.
  `pgcrypto` is a stock contrib module, so `C-NO-NATIVE` holds (it is not a custom
  C extension we author). Rationale recorded vs. core `sha256()` (which would pin
  a PG-version floor instead).
- **`C-TZ-INDEPENDENT-HASH` (timezone):** the ts string is derived with
  `to_char((ts AT TIME ZONE 'UTC'), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`. The explicit
  `AT TIME ZONE 'UTC'` makes the value **independent of the session `TimeZone`
  GUC**; parity is tested under multiple `TimeZone` settings to prove it.
- **`C-VERIFY-DISPATCH` (verifier):** `VerifyRows` (`audit.go:213`) gains an
  explicit switch on `hash_format_version`: `2 → V2RowHash`, `3 → V3RowHash`,
  **unknown → verifier failure** (never a silent v2 fallback). Chain continuity
  (`previous_hash` linkage) is checked across a v2→v3 segment boundary.
- **`C-TEST-ROW-WRITER` (harness):** an owner-only, test-only raw-row writer
  (bypassing the `SECURITY DEFINER` fn) seeds malformed/mixed/tampered chains so
  the verifier's mixed-format + tamper paths are testable **without** a
  production bypass.

Tests: **T-HASH-PARITY** (Go `V3RowHash` vs the SQL fn byte-for-byte, vectors:
ASCII, multibyte UTF-8, `<`/`>`/`&`, U+2028/U+2029, empty string, null, integer
fields, ts edge cases), **T-TS** (run under `TimeZone` ∈ {UTC, EST,
Asia/Kolkata, Australia/Lord_Howe}), **T-VERIFY-MIXED** (v2-only, v3-only, mixed
v2→v3 with continuity, unknown-format failure, per-format tamper detection).

### 4.2 Negative grants are schema contract (`C-GRANT-DRIFT`, IX-002)

Today `0005_repo_local_workflow_state.sql:470` does
`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd TO
striatumd_rw` and only `:471/:472` revoke UPDATE/DELETE on `events`/`artifacts`;
`audit_log` append-only is **trigger-only** (`0001_baseline.sql:154-165`), and
`striatumd_rw` holds direct `INSERT` on all three. The revoke must be **durable**,
not a one-off:

- A dedicated **owner** migration runs **after** any `GRANT ALL`, issuing
  `REVOKE INSERT ON striatumd.audit_log` (then `artifacts`, then `events`) `FROM
  striatumd_rw` and `GRANT EXECUTE` on the write functions. Because grant-repair
  or a future `GRANT ALL` could reopen it, the revoke is encoded as a **re-asserted
  invariant**: the migration helper and any `doctor repair-grants` path call a
  shared `reassertWriteRevokes()` that always re-applies the revokes after any
  grant step.
- `pgtest` is forbidden from patching these grants in Go (§9.1).

- **T-GRANT-DRIFT** (binding gate): after migrations, after `pgtest` role setup,
  and after a simulated `doctor repair-grants`, a direct `INSERT` to each
  protected table as the migration-defined unprivileged role fails SQLSTATE
  `42501`; the `append_*` path (with authority) succeeds.

### 4.3 `SECURITY DEFINER` hardening template (`C-SD-HARDEN`, PX-005+IX-003)

Every owner-owned write function (`append_audit_row`, artifact, event,
`assert_daemon_authority`) is created from one hardened template:

- `SECURITY DEFINER` **owned by the owner role**;
- `SET search_path = striatumd, pg_temp` pinned via `proconfig` (no ambient
  resolution); **all** object references fully qualified (`striatumd.audit_log`,
  `pg_catalog.…`);
- no caller-controlled object names, operators, or dynamic SQL on caller input;
- `REVOKE ALL ON FUNCTION … FROM PUBLIC` then `GRANT EXECUTE … TO striatumd_rw`
  only.

- **T-SD-HARDEN** (binding gate): a migration test inspects `pg_proc.proconfig`
  (asserts the pinned `search_path`) and the function ACL (asserts `PUBLIC` has no
  execute, `striatumd_rw` does), plus a **hostile-`search_path` regression** that
  sets a malicious `search_path` / shadow schema before the call and proves the
  function still reaches the intended `striatumd` objects and the authority check.

### 4.4 Deploy order + startup preconditions (`C-DDL-DEPLOY-ORDER`, OPS-4)

L1 DDL is **owner-applied out-of-band** (RFC 0079 §5: `striatumd_rw` cannot DDL
owner tables — confirmed: migrations crash-loop if the runtime role alters owner
objects). The binary's own auto-migrations run as the runtime role and must
**not** attempt the owner DDL. Ordering is pinned and made fail-fast:

- **Order:** owner DDL (functions + revokes + `pgcrypto`) is applied **before**
  the new binary that depends on it. Both are **idempotent**.
- **Startup precondition (new-binary/old-schema):** on boot the daemon checks
  `to_regprocedure('striatumd.append_audit_row(...)')` is present and `pgcrypto`
  is installed; if absent it **fails closed** with
  `daemon_pg_schema_precondition: append_audit_row missing — apply owner DDL
  vX.Y first` rather than serving mutations that would hit a missing function.
- **Old-binary/premature-REVOKE:** if `INSERT` was revoked but the running binary
  still writes via direct `INSERT`, the same precondition detects the mismatch
  (binary capability vs schema state) and fails closed with an actionable message.

- **T-DEPLOY-SKEW** (binding gate): both skew directions are simulated; each fails
  the precondition with an actionable diagnostic **before** the first mutation.

### 4.5 Forward-only rollback posture (`C-ROLLBACK-FORWARD-ONLY`, OPS-3)

Once a v3 row is written, a rolled-back v2-only `VerifyRows` would report the
chain **broken** — indistinguishable from tamper. We choose the **flagged
cutover** option (cheapest, reversible):

- v3 writes are gated behind an operator-committed flag `audit.hash_format` (v2 |
  v3), default **v2**. While v2, no v3 row exists, so a binary rollback to a
  v2-only verifier sees a **green** chain — rollback stays a two-way door until
  the operator deliberately flips to v3.
- The flip to v3 is declared **forward-only** in the runbook: the rollback target
  after the flip must already understand v3 (i.e., roll back only to a binary
  with the dispatching verifier), and the runbook gives a `VerifyRows`
  *verify-tolerance* note distinguishing "format skew from a too-old verifier"
  from "genuine tamper" (skew = *every* row past the cutover fails identically;
  tamper = an isolated row fails).

- **T-ROLLBACK-POSTURE** (binding gate / expected-stage): with the flag off, the
  suite asserts no v3 row is producible and a v2-only verifier is green; the
  acceptance gate text names the skew-vs-tamper distinction.

### 4.6 Phasing (unchanged shape, now gated)

Phase 0 `audit_log` → Phase 1 `artifacts` → Phase 2 `events`, each
independently shippable and **provable-green** before the next (DC2). RLS on
`leases`/`sessions` is **last** and explicitly **defense-in-depth row-scoping**,
not an authority boundary (§2.4).

---

## 5. L0 — Credential: ephemeral, owner-bootstrapped, RAM-only

### 5.1 Bootstrap (unchanged core)

At startup `striatumd` opens a short-lived **owner** connection (PEER unix-socket
by default; `STRIATUM_OWNER_DB_URL` for remote, recommended via a systemd
encrypted credential), `ALTER ROLE striatumd_rw PASSWORD '<crypto/rand>'`, seeds
the `daemon_auth_registry` (§2.2), closes the owner connection, and opens the
runtime `pgxpool` as `striatumd_rw`.

### 5.2 Fail-closed on owner failure (`C-RESTART-OWNER-DEP`, OPS-1) — accepted-as-risk

L0 makes **every restart** depend on owner connectivity to derive the runtime
password. This is an **accepted operational trade**, documented as such: a
transient owner failure now **fails closed** — the daemon refuses to serve and
emits an **owner-attributable** diagnostic
(`daemon_pg_owner_bootstrap_failed: cannot reach owner DSN to rotate
striatumd_rw`), and **never** falls back to a stale/previous runtime credential
(which would defeat rotation). The brief records this as the deliberate cost of
the anti-leak property.

- **T-OWNER-FAILCLOSED** (binding gate): simulate owner-bootstrap failure →
  assert the daemon does not serve mutations, the diagnostic names owner
  connectivity, and no prior credential is reused.

### 5.3 Single-role posture is visible (`C-L0-ADOPTION-VISIBLE`, OPS-2)

On the documented live posture (`postgres://halbritt@/postgres?host=/var/run/postgresql`,
owner==runtime, no separate `striatumd_rw`), rotation is **skipped** — so L0 is
inert there. Cycle 1 only `WARN`ed. Corrected: the skip surfaces as a **`daemon
doctor` posture finding** `rotation_skipped_single_role` (structured, not just a
log line), and the runbook documents the **owner≠runtime role split as an L0
adoption prerequisite** — the spec does **not** claim the runtime credential is
"made uninteresting" until that split is adopted.

- **T-DOCTOR-SINGLEROLE** (binding gate): a single-role fixture →
  `daemon doctor` reports `rotation_skipped_single_role` as a posture finding;
  the runbook names the split.

### 5.4 Single-writer-per-role invariant (`C-ROTATION-SINGLE-WRITER`, OPS-5)

One shared `striatumd_rw` password rotated per restart is implicitly
**single-writer-per-PG**: a rolling restart or a second/remote daemon would
invalidate the incumbent's reconnect password. Resolution:

- **Default (local):** declare a **single-daemon-per-`striatumd_rw`-role**
  invariant; `daemon doctor` detects a concurrent rotator (e.g., a recent
  `daemon_auth_registry.rotated_at` from a different `instance_id`, or a
  `pg_stat_activity` peer on the role) and reports a posture finding.
- **Remote/multi-host path:** use a **per-instance role** (`striatumd_rw_<instance>`)
  so concurrent daemons never share a rotated credential. Cross-host client TLS
  certs remain **deferred** (cycle 1 §5.6) until a real multi-host deployment.

- **T-SINGLE-WRITER** (expected-stage): the spec states the invariant + doctor
  probe; the remote-PG section names the per-instance-role behavior.

### 5.5 Dynamic-credentials disposition (`Q-DYNAMIC-CREDENTIALS`, PR-001) — deferred-with-owner

PR-001 wants a `BeforeConnect` dynamic password provider to shorten heap
residency; this is in **tension** with the candidate's honest `C-SECRET-HONEST`
(pgx retains the password in `pgxpool.Config` for reconnects, so the security
property is **RAM-only + rotation-on-restart**, not zeroization). Disposition:
**deferred-with-owner**. `C-SECRET-HONEST` is preserved verbatim for 0110; a
`BeforeConnect` provider is recorded as a strict **superset** improvement that
shrinks (not eliminates) the heap window, assigned to successor
**#88-dynamic-creds**. The same honesty applies to the `daemon_auth` secret
(§2.2). This RFC does **not** over-promise live-process memory secrecy.

### 5.6 Owner break-glass DR (`C-OWNER-DR`, OPS-6)

A non-decryptable systemd `LoadCredentialEncrypted` owner secret (host
re-provision / TPM reset) would lock the operator out. The L0 runbook ships a
**break-glass** path: re-encrypt the owner credential on the new host, or supply
a temporary plaintext owner DSN (`STRIATUM_OWNER_DB_URL`) to re-bootstrap, then
rotate. Documented as the recovery story for an unavailable owner credential.

### 5.7 Doctor owner reach + privilege-safe probe (`C-DOCTOR-OWNER-REACH`, OPS-7)

Cycle 1's posture probe used `SELECT rolpassword … FROM pg_authid`, which needs
**superuser** catalog access — blind in the remote-PG posture. Corrected:

- the posture probe avoids superuser-only catalogs — it uses an **owner-owned
  `SECURITY DEFINER` posture function** that returns a boolean "rotated/posture
  OK" **without exposing the hash or password**, callable by the runtime role;
- the runbook specifies how `daemon doctor` obtains **owner** access
  out-of-band in the remote-PG posture (operator supplies the owner DSN for the
  diagnostic invocation), so "the runtime credential is dead" stays diagnosable
  over the owner connection.

---

## 6. `daemon_auth_log` privacy (`C-AUTH-LOG-PRIVACY`, PR-002)

The owner-owned `daemon_auth_log` (auth posture/transitions; `event ∈ {bootstrap,
rotated, rotation_skipped_single_role, rotation_failed, owner_bootstrap_failed}`)
must never persist secrets. The `detail jsonb` is written through a **strict key
whitelist + redaction path**, not raw:

- a single Go writer accepts only whitelisted keys (`event`, `daemon_version`,
  `instance_id`, `reason_code`, `duration_ms`); **anything else is dropped**;
- any string value is run through a DSN/credential **redactor** (regex over
  `password=`, `://user:pass@`, token/`secret` patterns) before insert;
- raw driver `error` structs / DSNs are reduced to a `reason_code` enum, never
  stored verbatim.

- **T-AUTHLOG-REDACT** (binding gate): feed DSNs, raw driver errors, passwords,
  tokens, and connection params to the writer; assert the inserted row contains
  **only** whitelisted keys and **no** secret substring (grep the stored JSON).

---

## 7. Durable event transcript exclusion (`C-EVENT-NO-TRANSCRIPTS`, PR-005)

`events.payload_json` is a broad durable surface; without DB-level validation,
transcript-like provider output could enter authoritative storage (violating
D028). The L1 events phase adds **DB-level** validation in the event write
function (and a backing `CHECK`/trigger as belt-and-suspenders):

- reject payloads whose top-level keys include `stdout`, `stderr`, `transcript`,
  `raw_output`, `provider_output`, `chat`, or `messages`;
- reject payloads over a curated-metadata size ceiling (a transcript smell);
- allow curated metadata events (typed, bounded keys).

- **T-EVENT-NOTRANSCRIPT** (binding gate): insertion of each forbidden shape is
  rejected at the DB boundary; curated metadata events insert green.

This keeps the DB a **curated** record, not a transcript store (AGENTS.md product
boundary; D028).

---

## 8. L2 — Isolation: lanes cannot reach PG out-of-band

### 8.1 Secure-profile vs legacy upgrade (`C-L2-DEFAULT`, PX-003)

Separate compatibility from target posture:

- **Fresh / secure-profile adoption:** the hardened posture (PG-less lane OS user
  + `0700` socket dir) is **enabled**, or `daemon doctor` emits a **blocking**
  finding naming the exact remediation. New installs are guided to the secure
  posture immediately.
- **Legacy upgrade:** `security.pg_socket_hardened` stays **default-false** and
  `daemon doctor` **warns** (not blocks) so existing PEER installs are not
  stranded on upgrade day.
- **Default-on graduation gate (named now):** default flips on in a named
  successor minor **after** (a) the L2 how-to has shipped one release, (b)
  `T-LANE-ISOLATION-NEG` is green in CI, and (c) the socket-relocation runbook
  (§8.4) is published. The RFC records the successor release id on landing.

### 8.2 Negative lane isolation test (`C-L2-NEG-TEST`, EV-002)

Cycle 1 had only positive doctor-posture tests. Add a **negative** test:

- **T-LANE-ISOLATION-NEG** (binding gate): a mock lane identity (the dedicated
  PG-less OS user) attempts to connect to PG over (a) the protected `0700` UNIX
  socket dir and (b) loopback TCP; assert **both fail** under the hardened
  posture (socket: directory traversal denied; TCP: `pg_hba`/no listener).

### 8.3 Socket-dir `0700` startup assertion (`C-SOCKET-DIR-PERMS`, PR-004)

The daemon asserts at startup that the socket directory mode is **exactly `0700`**
and owned by the daemon identity (distinct from the lane identity); a more
permissive mode is a **`daemon doctor` violation** and (under the hardened flag) a
startup block. Drift is caught, not assumed-away.

### 8.4 Socket relocation blast radius (`C-SOCKET-RELOCATE-MIGRATION`, OPS-8)

Relocating PG's socket out of `/var/run/postgresql` to a `0700` dir breaks every
out-of-band path: `psql`, monitoring, ad-hoc owner DSNs, the `--as-owner` path,
**and the daemon's own pidfile** assumptions. The L2 how-to **enumerates the
blast radius** and updates daemon/doctor socket defaults **in lockstep** with the
flag, giving the exact new connection string (`host=<0700-dir>`). Enabling
hardening ships this migration note; defaults move together so nothing silently
points at the old path.

Plus the `PGHOST` scrub from the lane env (defense-in-depth; the env is already a
strict positive allowlist — `supervisedEnvPassThrough`, `supervision_control.go`),
belt-and-suspenders against a lane handing a guessed socket path to a libpq tool.

This is the structural close of **#87** (today advisory-only — confirmed:
`doctor_lane_sandbox.go` emits a `lane_pg_reachable` *warning* cleared by
`STRIATUM_LANE_OS_USER`).

---

## 9. Test-harness fidelity

### 9.1 `pgtest` verifies migrations, not Go-patched state (`C-HARNESS-PRIVILEGES`, EV-001)

`pgtest.Pools` today constructs the unprivileged role imperatively in Go (ad-hoc
`GRANT`/`SET ROLE`), so a security test could validate a **polluted** harness
rather than the migrations-enforced production privileges — a false-green risk for
`T-42501`. Corrected:

- roles and privileges are produced **solely** by owner-applied migrations;
  `pgtest` connects using those migration-defined roles and is **prohibited** from
  issuing `GRANT`/`REVOKE` on protected tables during a run;
- the test-only **raw-row writer** (§4.1, `C-TEST-ROW-WRITER`) is the *only*
  sanctioned bypass, and it writes raw rows for verifier coverage — it does **not**
  patch privileges.

- **T-HARNESS-FIDELITY** (binding gate): the harness setup test fails if `pgtest`
  issues ad-hoc `GRANT`/`REVOKE` for protected-table privileges, and confirms
  `T-42501` runs against migration-defined roles.

---

## 10. Sequencing (revised, DC4)

1. **Spec corrections (this document) before code:** the §2 authority claim,
   §4.1 v3 mixed-format contract, §2.4 GUCs-as-labels, §4.3 `SECURITY DEFINER`
   template, §4.4 deploy order, §4.5 rollback posture.
2. **L3 + L0 doctor probes (no behavior change):** §3 transaction prelude +
   parameterized GUCs + robust reset; §5.3/§5.7 doctor posture findings; §6
   `daemon_auth_log` privacy. Lands first.
3. **L0 rotation + L1 Phase 0:** §5 owner bootstrap + fail-closed (§5.2); §2
   `daemon_auth_registry` + `assert_daemon_authority`; §4.1 v3 helpers; `append_audit_row`;
   **T-EXEC-AUTH**, **T-42501**, **T-GRANT-DRIFT**, **T-HARNESS-FIDELITY**,
   **T-ROLLBACK-POSTURE**, **T-DEPLOY-SKEW**.
4. **L1 Phase 1 (`artifacts`) → Phase 2 (`events`):** same hardened write fn +
   authority gate; §7 transcript exclusion in the events phase; each
   provable-green first. RLS on `leases`/`sessions` **last**, row-scoping only.
5. **L2 hardening** behind `security.pg_socket_hardened` (default-false; §8.1
   named default-on gate): **T-LANE-ISOLATION-NEG**, §8.3 `0700` assertion, §8.4
   relocation runbook.
6. **Remote/multi-host follow-up:** resolve §5.4 per-instance roles, §5.6 owner
   DR, §5.7 doctor owner reach before claiming remote-PG readiness; cross-host
   TLS certs deferred.

Sequenced **after** the RFC 0104/0105 reliability foundation; does **not** block
RFC 0103's remaining work. No native code, no wire proxy, no hosted identity
(`C-NO-NATIVE`).

---

## 11. Acceptance (maps to RFC §Acceptance)

- **L0:** a fresh start rotates `striatumd_rw` with no on-disk secret in PEER
  posture; a DSN captured before a restart fails after it; owner-bootstrap failure
  **fails closed** with an owner-attributable diagnostic (**T-OWNER-FAILCLOSED**);
  the single-role skip is a **doctor posture finding** (**T-DOCTOR-SINGLEROLE**);
  single-writer-per-role invariant + doctor detection (**T-SINGLE-WRITER**).
- **L1 / authority:** a direct `striatumd_rw` call to any write fn without the
  daemon-authority secret **fails without mutation** (**T-EXEC-AUTH**); GUC labels
  carry no authority (**T-GUC-LABEL**); direct `INSERT` is revoked and stays
  revoked across migration/pgtest/repair (**T-GRANT-DRIFT** / **T-42501**);
  `SECURITY DEFINER` hardening pinned (**T-SD-HARDEN**); `VerifyRows` holds across
  the v2→v3 boundary (**T-VERIFY-MIXED**) with pinned parity (**T-HASH-PARITY**,
  **T-TS**); deploy skew fails fast (**T-DEPLOY-SKEW**); rollback posture defined
  (**T-ROLLBACK-POSTURE**).
- **Attribution:** prelude is in-transaction and parameterized; labels reset
  across commit/rollback/cancel/timeout/panic (**T-TX-PRELUDE**, **T-ATTR-RESET**).
- **Privacy:** `daemon_auth_log` is whitelisted+redacted (**T-AUTHLOG-REDACT**);
  `events.payload_json` rejects transcript shapes (**T-EVENT-NOTRANSCRIPT**).
- **L2:** with the hardened default, a lane cannot open the socket or loopback
  (**T-LANE-ISOLATION-NEG**); `daemon doctor` **blocks** for secure profiles and
  **warns** for legacy upgrades (**T-L2-DEFAULT**); `0700` asserted at startup;
  relocation runbook shipped.
- `docs/reference/spec.md` carries the daemon→PG auth model + the
  DB-enforced write-boundary invariant (G1+G2, §2.1); `decision-log.md` records
  each per-phase decision on landing.

---

## 12. Rejected / residue (explicitly flagged)

- **Narrowing-only for `C-EXEC-AUTH`** (no gate) — rejected: the binding
  verification requires a direct call to **fail without mutation**, which a
  narrow-claim-only spec cannot pass. We add the §2 gate.
- **RLS / GUCs as authority** — rejected: client-controlled labels are not a trust
  boundary (`C-GUC-NONAUTH`); RLS stays row-scoping defense-in-depth.
- **Option B (trusted caller-supplied `row_hash`)**, **porting `encoding/json` to
  PL/pgSQL** — still rejected (cycle 1 §7); v3 length-prefixed bytea canonical
  removes the HTML-escaping/key-order hazard.
- **Inherited RFC-appendix rejects** (PG C extension/bg worker, user-namespace PG,
  `seccomp` socket block, PAM ppid module, quorum gate, `pg_notify`/`SD_LISTEN_FDS`
  to a client, abstract-namespace socket) — unchanged.

**Residue (not hidden):**

- **§2.1 G2 residual:** the authority gate does **not** survive a full
  live-process memory scrape (yields DSN + secret together); bounded by L0
  rotation-on-restart and L2 lane isolation, not eliminated.
- **§3.2 logging residual:** an operator enabling verbose statement+parameter
  logging could log bound GUC values incl. the secret — runbook hardening item.
- **§5.5 heap residual:** pgx retains the password (and we retain the secret) for
  reconnect; the guarantee is RAM-only + rotation-on-restart, not zeroization
  (`C-SECRET-HONEST`); dynamic-provider shrink deferred to **#88-dynamic-creds**.
- The v2 `V2RowHash` path stays **permanently** as the only reader of pre-cutover
  rows (accepted cost, not tech debt).

---

**Standing live for cross-examination.** The most attackable new claims are:
**§2.2** (is the `daemon_auth` secret *actually* non-spoofable by a `striatumd_rw`
caller on the same connection, and is the §2.1 residual scoped honestly?),
**§3.1/§3.3** (does the in-transaction prelude + discard-on-error truly close the
attribution-bleed paths EV-004 named?), **§4.4** (do the startup preconditions
catch *both* skew directions before the first mutation?), and **§4.1**
(`pgcrypto digest` byte-parity with Go `V3RowHash` across the full hostile vector
set, under multiple DB timezones).
