---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: convener-claude-opus-4.8-004
workflow: rfc-0110-pg-auth-panel
phase: convener_synthesis
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 2
title: "RFC 0110 — Convener synthesis (cycle 2): discharged constraints + implementation-ready spec"
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/adjudicator/COLLABORATION_LEDGER_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/intake/INTAKE_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/synthesis/CROSS_EXAM_SYNTHESIS_cycle_1.md
---

# RFC 0110 — Convener synthesis (cycle 2)

The implementation-ready synthesis for RFC 0110 (daemon→PostgreSQL authentication
and the database-enforced write boundary), revised after the cycle-1 adjudicator
verdict **`needs_revision`** (`../../adjudication/adjudicator/COLLABORATION_LEDGER_cycle_1.md`).
This is the authoritative, constraint-bearing version; full rationale and code
anchors live in the cycle-2 candidate (`../draft/CANDIDATE_cycle_2.md`) and the
survey.

**Revision basis.** Cycle 1 carried **1 critical + 16 high findings** with **no
candidate rebuttal** (the cross-exam window was closed at synthesis time), so the
adjudicator could not clear the gate. The verdict converted those findings into
**16 binding constraints** + **1 unresolved question** (`Q-DYNAMIC-CREDENTIALS`)
and named **11 medium follow-ups** to fold in. **§1 discharges every prior
`constraints[]` row explicitly** with the role's disposition vocabulary
(answer / fold-in / reject-with-rationale / accept-as-risk / defer-with-successor)
and the falsifiable gate that makes each discharge real. Per the convener role,
**dialogue completion is not acceptance** — the cycle-2 adjudicator ledger
decides whether the gate clears.

**Cross-examination status (cycle 2):** this synthesis is published *before* the
cycle-2 cross-examiners run. The cycle-2 changes therefore stand live for
cross-examination; the four most attackable new claims are pre-flagged in §8.

**The one load-bearing change vs. cycle 1.** The critical `C-EXEC-AUTH` hole is
closed by **both** narrowing the product claim to what is true **and** adding a
non-spoofable, RAM-only **daemon-authority gate** (`striatum.daemon_auth`) inside
every `SECURITY DEFINER` write function (§3). That single mechanism also
discharges `C-GUC-NONAUTH` (attribution GUCs become *labels only*; authority is
the verified secret) and anchors the L3 in-transaction prelude
(`C-TX-GUC-PRELUDE`).

---

## 1. Discharge ledger — every prior binding row, explicitly

Disposition vocabulary (role-mandated): **fold-in** (design changed to satisfy
the constraint), **answer** (claim narrowed/clarified to be true),
**reject-with-rationale**, **accept-as-risk** (kept with documented rationale),
**defer-with-successor** (out of scope this RFC, successor named). Each binding
row carries the **falsifiable gate** — the discharge is real iff that gate passes.
These are the rows the cycle-2 cross-examiner should attack.

### 1.1 Binding constraints (16) + the unresolved question

| id | sev | disposition | where | falsifiable gate (discharge is real iff this passes) |
| --- | --- | --- | --- | --- |
| `C-EXEC-AUTH` | critical | **fold-in + answer** | §3 | **T-EXEC-AUTH:** as `striatumd_rw` with no/`wrong` `striatum.daemon_auth`, every `SECURITY DEFINER` write fn `RAISE`s (SQLSTATE `28000`) and mutates **0 rows** of `audit_log`/`artifacts`/`events`; the daemon RPC path (verified secret set in-tx) succeeds. |
| `C-GUC-NONAUTH` | high | **fold-in** | §3.4, §4 | **T-GUC-LABEL:** as `striatumd_rw`, set fake `rpc_id`/`principal_id`/`app.session_id` → every protected write still fails (authority ≠ labels); a daemon RPC sets/clears the same labels purely for attribution. |
| `C-TX-GUC-PRELUDE` | high | **fold-in** | §4 | **T-TX-PRELUDE:** a guard test fails any mutating tx whose first statements omit the attribution+authority prelude; commit **and** rollback leave the next checkout with no labels. |
| `C-GRANT-DRIFT` | high | **fold-in** | §5.2 | **T-GRANT-DRIFT:** after migrations, `pgtest` role setup, **and** a simulated doctor grant-repair, direct `INSERT` to each protected table still fails `42501`. |
| `C-SD-HARDEN` | high | **fold-in** | §5.3 | **T-SD-HARDEN:** `pg_proc.proconfig` shows `search_path=striatumd, pg_temp`; ACL grants `EXECUTE` only to `striatumd_rw`, `PUBLIC` revoked; a hostile-`search_path` regression still reaches the intended `striatumd` objects. |
| `C-AUTH-LOG-PRIVACY` | high | **fold-in** | §6.1 | **T-AUTHLOG-REDACT:** feeding DSNs/driver errors/passwords/tokens/params to the `daemon_auth_log` writer inserts **only** whitelisted, redacted keys; no secret substring survives. |
| `C-EVENT-NO-TRANSCRIPTS` | high | **fold-in** | §6.2 | **T-EVENT-NOTRANSCRIPT:** a DB `CHECK`/trigger rejects `events.payload_json` carrying `stdout`/`stderr`/`transcript`/`raw_output`/provider-output keys; curated metadata events insert green. |
| `C-HARNESS-PRIVILEGES` | high | **fold-in** | §7.1 | **T-HARNESS-FIDELITY:** harness setup fails if `pgtest` issues ad-hoc `GRANT`/`REVOKE` on protected tables; `T-42501` runs against migration-defined roles only. |
| `C-L2-NEG-TEST` | high | **fold-in** | §7.2 | **T-LANE-ISOLATION-NEG:** a mock lane identity attempting to connect over the protected UNIX socket **and** loopback TCP fails under the hardened posture. |
| `C-ATTR-RESET-FAIL` | high | **fold-in** | §4.3 | **T-ATTR-RESET:** labels are gone on the next checkout after commit, rollback, `context` cancel, statement timeout, and a handler panic; dirty connections are discarded. |
| `C-RESTART-OWNER-DEP` | high | **fold-in + accept-as-risk** | §5.5 | **T-OWNER-FAILCLOSED:** simulated owner-bootstrap failure → daemon refuses to serve, emits an owner-attributable diagnostic, and never falls back to a stale runtime credential. |
| `C-L0-ADOPTION-VISIBLE` | high | **fold-in** | §5.6 | **T-DOCTOR-SINGLEROLE:** on a single-role fixture, `daemon doctor` reports `rotation_skipped_single_role` as a **posture finding** (not just a WARN); the runbook names the owner≠runtime split as an L0 prerequisite. |
| `C-ROLLBACK-FORWARD-ONLY` | high | **fold-in** | §5.4 | **T-ROLLBACK-POSTURE:** v3 writes are gated behind an operator-committed `audit.hash_format=v3` flag; with it off, no v3 row exists, so a v2-only verifier on rollback sees a green chain; the runbook distinguishes format-skew from tamper. |
| `C-DDL-DEPLOY-ORDER` | high | **fold-in** | §5.7 | **T-DEPLOY-SKEW:** new-binary/old-schema (missing `append_audit_row`) and old-binary/premature-`REVOKE` both fail a **startup precondition** with an actionable error before serving mutations. |
| `C-ROTATION-SINGLE-WRITER` | high | **fold-in** | §5.8 | **T-SINGLE-WRITER:** the spec declares one daemon per `striatumd_rw` role + doctor detection of a concurrent rotator; remote/multi-host uses a per-instance role. Stated invariant + doctor probe. |
| `C-L2-DEFAULT` | high | **fold-in** | §7.3 | **T-L2-DEFAULT:** a secure-profile/fresh install either enables the hardened posture or emits a **blocking** doctor finding; a legacy upgrade **warns** until the named `vX.Y` default-on release. |
| `Q-DYNAMIC-CREDENTIALS` | high (question) | **defer-with-successor** | §5.9 | Disposition recorded: deferred to successor **#88-dynamic-creds**; `C-SECRET-HONEST` preserved verbatim (RAM-only + rotation-on-restart, **not** live-memory secrecy). Optional `BeforeConnect` provider noted as a strict superset, not required for 0110. |

### 1.2 Medium follow-ups — folded into the high constraints (adjudicator's "fold where practical")

`C-HASH-V3`, `C-HASH-BYTEA`, `C-SHA-PRIMITIVE`, `C-VERIFY-DISPATCH`,
`C-TZ-INDEPENDENT-HASH`, `C-TEST-ROW-WRITER` → **§5.1 (v3 hash contract)**;
`C-GUC-PARAMETERIZED` → **§4.2**; `C-SOCKET-DIR-PERMS` and
`C-SOCKET-RELOCATE-MIGRATION` → **§7.4/§7.5**; `C-OWNER-DR` and
`C-DOCTOR-OWNER-REACH` → **§5.10/§5.11**. Each surfaces as a named test in its
section.

### 1.3 Cycle-1 candidate constraints carried forward (still binding)

`C-INSERT-REVOKE`, `C-OWNER-DDL`, `C-UPGRADE-SAFE`, `C-NO-NATIVE`,
`C-ATTR-RESET` (strengthened by `C-ATTR-RESET-FAIL`), `C-SECRET-HONEST`, and
`C-HASH` (sharpened by the §5.1 medium cluster) remain binding from cycle 1.
Nothing from cycle 1 is dropped or weakened.

---

## 2. Decisions of record (cycle 2)

The cycle-1 decisions Q1–Q9 stand; cycle 2 adds Q10 (the authority gate) and
sharpens Q1′/Q7.

| # | Question | Decision (cycle 2) |
| --- | --- | --- |
| **Q1** | L1 hash: in-DB (A) vs trusted caller-supplied (B) | **Option A** (unchanged). B rejected: a leaked-DSN caller could inject a chain-breaking `row_hash`; a validating B *is* A. |
| **Q1′** | Robust in-DB hashing | **`hash_format_version = 3`**, length-prefixed `bytea` canonical (§5.1) — escaping-free, key-order-free, byte-identical in Go (`V3RowHash`) and PL/pgSQL. `VerifyRows` dispatches per row; **unknown format → verifier failure** (never silent v2 fallback). |
| **Q2** | `ts` representation in-DB | RFC3339 second-truncated **UTC string** via `to_char((ts AT TIME ZONE 'UTC'), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`; the explicit `AT TIME ZONE 'UTC'` makes it independent of the session `TimeZone` GUC. Pinned by **T-TS**. |
| **Q3** | `SECURITY DEFINER` scope | `striatumd.append_audit_row(...)` owns the whole append (chain-head `FOR UPDATE`, segment open/resolve, v3 hash, INSERT, head UPDATE) **and** begins with `assert_daemon_authority()`. `RecordRPCTransport` is a thin caller. |
| **Q4** | L0 remote-PG + dev guard | `STRIATUM_OWNER_DB_URL` for owner bootstrap without PEER; recommended at-rest = systemd `LoadCredentialEncrypted=`. Single-role guard: owner==runtime ⇒ skip rotation **and surface a doctor posture finding** (Q-upgrade vs Q-OPS-2, §5.6). |
| **Q5** | In-memory password hygiene | **RAM-only + rotation-on-restart**, not zeroization (pgx retains the password in `pgxpool.Config` for reconnects). Honest scope; `C-SECRET-HONEST` preserved. |
| **Q6** | L2 adoption | Default-false `security.pg_socket_hardened` for **legacy upgrades**; **secure-profile/fresh installs** enable it or get a **blocking** doctor finding; default-on flip named in a successor minor gated by evidence (§7.3). |
| **Q7** | L1 RLS scope | RLS on `leases`/`sessions` keyed on `current_setting('app.session_id', true)` is **defense-in-depth row-scoping only, demoted below the authority gate** — never the trust boundary (§3.4). Lands last. |
| **Q8** | `daemon_auth_log` | Owner-owned table; `event ∈ {bootstrap, rotated, rotation_skipped_single_role, rotation_failed, owner_bootstrap_failed}`; `detail jsonb` written through a **strict whitelist + redactor** (§6.1); `daemon doctor` reads it over the owner connection even when the runtime credential is dead. |
| **Q9** | `42501` CI gate | **T-42501** in `pgtest` against **migration-defined roles** (not Go-patched): direct `INSERT` fails `42501`; the `append_*` path with authority succeeds; runs every migration-forward (§5.2, §7.1). |
| **Q10** | *(new)* Authority for write functions | **A per-process RAM-only daemon-authority secret** (`striatum.daemon_auth`), verified server-side by an owner-only `assert_daemon_authority()` against an owner-owned digest registry `striatumd_rw` cannot read. **No native code, no wire proxy** — a transaction-local GUC verified in-DB (§3). This is the `C-EXEC-AUTH`/`C-GUC-NONAUTH` close. |

---

## 3. The critical fix — non-spoofable daemon-authority gate (`C-EXEC-AUTH`, `C-GUC-NONAUTH`)

**The cycle-1 hole (PX-001, critical).** Revoking direct DML routes writes through
owner-owned `SECURITY DEFINER` functions, but `striatumd_rw` holds `EXECUTE`, so a
process with **only the runtime DSN** could call `append_audit_row` directly and
produce a **hash-correct but unauthorized** write. In-DB hashing makes the row
well-formed, not daemon-issued. The binding `C-EXEC-AUTH` gate *requires* a
negative test where a direct `striatumd_rw` call **fails without mutation** — so
narrowing the claim alone cannot pass it. We do **both**.

### 3.1 The narrowed product claim (honest scope — the *answer* half)

RFC 0110's headline splits into two separable guarantees:

- **G1 — invariant integrity (DB-enforced):** a leaked runtime credential **cannot**
  violate the append-only / hash-chain / attempt-scope / segment invariants — the
  *only* write path computes the hash and holds the chain lock in-DB.
- **G2 — daemon-issuance (authority gate, §3.2):** a write succeeds **only** if the
  caller presents the per-process daemon-authority secret; a process holding only
  the DSN *string* cannot mutate.

**Residual stated plainly:** G2 does **not** survive a full live-process memory
scrape of a running daemon (the scrape yields DSN *and* secret). That residual is
owned by **L0** (rotation-on-restart kills any scraped secret at next restart, §5)
and **L2** (a sandboxed lane never reaches the socket, §7). We claim a **leaked
DSN string** is uninteresting and a memory-scrape window is **bounded by restart** —
not that the runtime credential is unconditionally uninteresting. This is the D164
intent, made true.

### 3.2 Mechanism: `striatum.daemon_auth`, an owner-only RAM secret

At bootstrap, over the **owner** PEER connection, `striatumd`:

1. generates `auth_secret` via `crypto/rand` (RAM-only — never disk/env/DSN);
2. computes `digest = sha256(auth_secret || per-instance salt)` and UPSERTs
   `(instance_id, digest, rotated_at)` into a new **owner-owned** table
   `striatumd.daemon_auth_registry` on which `striatumd_rw` holds **no privilege**
   (cannot `SELECT` the digest, cannot write it);
3. holds `auth_secret` in process memory for the pool's lifetime (same honesty
   caveat as the rotated password, §5.9 / `C-SECRET-HONEST`).

Every owner-owned `SECURITY DEFINER` write function begins with:

```
striatumd.assert_daemon_authority()   -- SECURITY DEFINER, owner-owned
  -- reads current_setting('striatum.daemon_auth', true)  (missing -> NULL)
  -- compares sha256(presented || salt) to daemon_auth_registry.digest
  --   for the current instance, within the registry's freshness window
  -- RAISE EXCEPTION 'striatum_daemon_auth_required' (SQLSTATE 28000) on miss
```

A `striatumd_rw` caller cannot: forge the secret (`crypto/rand`), read the digest
(no grant), or learn it from the function (void/raise only). The daemon RPC path
sets the secret in the transaction prelude (§4) via **parameterized** `set_config`
so it never appears in `pg_stat_activity` query text.

### 3.3 Why this shape (and what it is not)

- **Not** a second wire handshake — no native code, no proxy (`C-NO-NATIVE` holds).
  A transaction-local GUC verified server-side.
- **Not** RLS-as-authority: RLS stays defense-in-depth row-scoping, demoted below
  the trust boundary (`C-GUC-NONAUTH`, §3.4).
- **Raises the bar** from "possess a credential string that leaks through config
  files, env, logs, or a captured DSN" to "possess a RAM-only capability the daemon
  never serializes" — a real, testable step bounded by §3.1.

### 3.4 GUCs are labels; the secret is authority (`C-GUC-NONAUTH`)

`striatum.rpc_id`, `striatum.principal_id`, `app.session_id` are **attribution
labels only** — read solely to store provenance, never to authorize. The **only**
authority-bearing GUC is `striatum.daemon_auth`. RLS on `leases`/`sessions` uses
`current_setting('app.session_id', true)` for **row scoping under an already
daemon-authorized session**, not as an auth gate.

### 3.5 Falsifiable tests

- **T-EXEC-AUTH** (binding `C-EXEC-AUTH`): for each of `append_audit_row`, the
  artifact fn, the event fn — call as `striatumd_rw` with (a) no `daemon_auth`, (b)
  a wrong secret; assert `RAISE` SQLSTATE `28000` and **zero** new rows. Then the
  daemon RPC path (secret in-tx) succeeds and the row appears.
- **T-GUC-LABEL** (binding `C-GUC-NONAUTH`): as `striatumd_rw`, set fake
  `rpc_id`/`principal_id`/`app.session_id` without the secret → still fails; with
  the secret (daemon path) → succeeds and labels stored verbatim for attribution.
- **T-REGISTRY-ACL:** `has_table_privilege('striatumd_rw',
  'striatumd.daemon_auth_registry', 'SELECT')` is **false**.

---

## 4. L3 — in-transaction attribution & authority prelude (`C-TX-GUC-PRELUDE`, `C-GUC-PARAMETERIZED`, `C-ATTR-RESET-FAIL`)

### 4.1 Placement correction (IX-001)

Cycle 1 put L3 in `pgxpool.BeforeAcquire` (`connection.go:208` builds the pool
plainly today). `BeforeAcquire` fires **before** the mutation transaction, so a
`SET LOCAL` invariant cannot hold there. Corrected: a shared **in-transaction
prelude** runs immediately after `BeginTx`, before the first DML, in every mutating
handler:

```
func applyAttribution(ctx, tx, attr) error {
    // parameterized — values never enter pg_stat_activity query text
    tx.Exec(ctx, "SELECT set_config('striatum.daemon_auth', $1, true)", attr.Secret)
    tx.Exec(ctx, "SELECT set_config('striatum.rpc_id',       $1, true)", attr.RPCID)
    tx.Exec(ctx, "SELECT set_config('striatum.principal_id', $1, true)", attr.PrincipalID)
    tx.Exec(ctx, "SELECT set_config('app.session_id',        $1, true)", attr.SessionID)
}
```

`is_local = true` scopes all four to the transaction; commit/rollback clears them
automatically (no cross-checkout bleed by construction). The `daemon_auth` secret
rides the same prelude so authority and attribution are set atomically before any
write fn runs.

### 4.2 No statistics leakage (`C-GUC-PARAMETERIZED`, PR-003)

All GUCs set via `SELECT set_config($1,$2,true)` with **bound parameters**, not
string-concatenated `SET LOCAL` — keeps `principal_id` and the `daemon_auth` secret
out of `pg_stat_activity.query`. **Residual:** an operator enabling
`log_statement=all` with parameter logging could still log bound values —
documented as an L0 runbook hardening item (do not enable verbose
statement+parameter logging on the daemon's PG).

### 4.3 Reset robustness across failure paths (`C-ATTR-RESET-FAIL`, EV-004)

Transaction-local GUCs vanish on commit/rollback; to defend pool reuse under
abort/cancel/panic an `AfterRelease` hook issues `DISCARD ALL`, and on any
transaction that errored or was cancelled the handler **discards the connection**
(`pgxpool` destroy) rather than returning it. Even a panicked handler cannot return
a connection carrying a stale `principal_id` or `daemon_auth`.

- **T-TX-PRELUDE** (binding `C-TX-GUC-PRELUDE`): a guard test fails a fake mutating
  handler whose first statement is a write; a compliant handler passes; after commit
  and after rollback a fresh checkout reads `current_setting('striatum.principal_id',
  true)` as NULL.
- **T-ATTR-RESET** (binding `C-ATTR-RESET-FAIL`): set labels, then force each of
  {commit, rollback, `context` cancel, statement timeout, handler panic}; assert the
  next checkout observes no prior `rpc_id`/`principal_id`/`app.session_id` and no
  `daemon_auth`.

---

## 5. L0 / L1 — credential lifecycle + DB-enforced write contract

### 5.1 The v3 hash contract (normative; folds `C-HASH-V3`/`C-HASH-BYTEA`/`C-SHA-PRIMITIVE`/`C-VERIFY-DISPATCH`/`C-TZ-INDEPENDENT-HASH`/`C-TEST-ROW-WRITER`)

Same 15 fields, fixed declared order:
`ts, schema_version, hash_format_version, daemon_version, client_id, repository_id, method, decision, denial_reason, transport, request_id, exit_code, params_sha256, previous_hash, segment_id`.

`row_hash = lower_hex(sha256( concat over fields of Enc(field) ))`, sharpened to
executable contract:

- **`C-HASH-V3` (supersession, normative):** v3 is the accepted format for **new**
  rows and **supersedes** the survey's "byte-identical to Go `encoding/json`"
  wording. `V2RowHash` (`audit.go:25`) is preserved **permanently** as the only
  reader of pre-cutover rows. The decision-log entry names the mixed-format
  invariant on landing.
- **`C-HASH-BYTEA` (construction):** PL/pgSQL builds hashed material as **`bytea`**:
  string `s` → `dec(octet_length(convert_to(s,'UTF8'))) || ':' || utf8_bytes(s)`;
  integer `n` → decimal ASCII, length-prefixed like a string; null → the 3 bytes
  `-1:`. Go's `V3RowHash` builds the identical `[]byte`. No implicit casts.
- **`C-SHA-PRIMITIVE` (pinned):** hashing uses **`pgcrypto`'s `digest(bytea,
  'sha256')`**; the owner migration runs `CREATE EXTENSION IF NOT EXISTS pgcrypto`
  and a startup precondition (§5.7) asserts it present *before* any `REVOKE`.
  `pgcrypto` is stock contrib, so `C-NO-NATIVE` holds. Rationale recorded vs. core
  `sha256()` (which would pin a PG-version floor).
- **`C-TZ-INDEPENDENT-HASH`:** the ts string uses `to_char((ts AT TIME ZONE 'UTC'),
  …)`; the explicit `AT TIME ZONE 'UTC'` makes it independent of the session
  `TimeZone` GUC; tested under multiple `TimeZone` settings.
- **`C-VERIFY-DISPATCH`:** `VerifyRows` (`audit.go:213`) gains an explicit switch on
  `hash_format_version`: `2→V2RowHash`, `3→V3RowHash`, **unknown→verifier failure**
  (never a silent v2 fallback); chain continuity checked across the v2→v3 boundary.
- **`C-TEST-ROW-WRITER`:** an owner-only, test-only raw-row writer (bypassing the SD
  fn) seeds malformed/mixed/tampered chains so the verifier paths are testable
  **without** a production bypass.

Tests: **T-HASH-PARITY** (Go vs SQL byte-for-byte over ASCII, multibyte UTF-8,
`<`/`>`/`&`, U+2028/U+2029, empty, null, integers, ts edges), **T-TS** (`TimeZone` ∈
{UTC, EST, Asia/Kolkata, Australia/Lord_Howe}), **T-VERIFY-MIXED** (v2-only,
v3-only, mixed-with-continuity, unknown-format failure, per-format tamper).

### 5.2 Negative grants are schema contract (`C-GRANT-DRIFT`, IX-002)

Today `0005_repo_local_workflow_state.sql:470` `GRANT SELECT,INSERT,UPDATE,DELETE …
TO striatumd_rw`; `:471/:472` revoke only UPDATE/DELETE on `events`/`artifacts`;
`audit_log` append-only is trigger-only (`0001_baseline.sql:154-165`) and
`striatumd_rw` holds direct `INSERT` on all three. The revoke must be **durable**:

- a dedicated **owner** migration runs **after** any `GRANT ALL`, issuing
  `REVOKE INSERT` on `audit_log`, then `artifacts`, then `events` `FROM striatumd_rw`
  and `GRANT EXECUTE` on the write functions;
- because grant-repair or a future `GRANT ALL` could reopen it, the revoke is a
  **re-asserted invariant**: the migration helper and any `doctor repair-grants`
  path call a shared `reassertWriteRevokes()` that always re-applies the revokes
  after any grant step;
- `pgtest` is forbidden from patching these grants in Go (§7.1).

- **T-GRANT-DRIFT** (binding): after migrations, after `pgtest` role setup, and
  after a simulated `doctor repair-grants`, a direct `INSERT` to each protected
  table as the migration-defined unprivileged role fails `42501`; the authorized
  `append_*` path succeeds.

### 5.3 `SECURITY DEFINER` hardening template (`C-SD-HARDEN`, PX-005/IX-003)

Every owner-owned write fn (`append_audit_row`, artifact, event,
`assert_daemon_authority`) is created from one template: `SECURITY DEFINER` owned by
the owner role; `SET search_path = striatumd, pg_temp` pinned via `proconfig`; all
references fully qualified; no caller-controlled object names/operators/dynamic SQL
on caller input; `REVOKE ALL … FROM PUBLIC` then `GRANT EXECUTE … TO striatumd_rw`
only.

- **T-SD-HARDEN** (binding): a migration test inspects `pg_proc.proconfig` and the
  function ACL, plus a **hostile-`search_path` regression** proving the function
  still reaches the intended `striatumd` objects and the authority check.

### 5.4 Forward-only rollback posture (`C-ROLLBACK-FORWARD-ONLY`, OPS-3)

Once a v3 row exists, a rolled-back v2-only `VerifyRows` would report the chain
**broken** — indistinguishable from tamper. We choose the **flagged cutover**
(cheapest, reversible):

- v3 writes are gated behind operator-committed `audit.hash_format` (v2 | v3),
  default **v2**. While v2, no v3 row exists, so a binary rollback to a v2-only
  verifier sees a **green** chain — rollback stays a two-way door until the operator
  deliberately flips to v3;
- the flip is declared **forward-only** in the runbook (roll back only to a binary
  with the dispatching verifier); a verify-tolerance note distinguishes skew (*every*
  post-cutover row fails identically) from tamper (an isolated row fails).

- **T-ROLLBACK-POSTURE** (binding/expected-stage): flag off → no v3 row producible,
  v2-only verifier green; the acceptance gate text names the skew-vs-tamper
  distinction.

### 5.5 Fail-closed on owner failure (`C-RESTART-OWNER-DEP`, OPS-1) — fold-in + accept-as-risk

L0 makes **every restart** depend on owner connectivity. This is an **accepted
operational trade**, documented as such: a transient owner failure **fails closed** —
the daemon refuses to serve and emits an **owner-attributable** diagnostic
(`daemon_pg_owner_bootstrap_failed: cannot reach owner DSN to rotate striatumd_rw`),
and **never** falls back to a stale runtime credential (which would defeat
rotation). The brief records this as the deliberate cost of the anti-leak property.

- **T-OWNER-FAILCLOSED** (binding): simulate owner-bootstrap failure → daemon does
  not serve mutations, the diagnostic names owner connectivity, no prior credential
  is reused.

### 5.6 Single-role posture is visible (`C-L0-ADOPTION-VISIBLE`, OPS-2)

On the documented live posture
(`postgres://halbritt@/postgres?host=/var/run/postgresql`, owner==runtime, no
separate `striatumd_rw`), rotation is **skipped** — L0 is inert there. Cycle 1 only
`WARN`ed. Corrected: the skip surfaces as a **`daemon doctor` posture finding**
`rotation_skipped_single_role` (structured), and the runbook documents the
**owner≠runtime role split as an L0 adoption prerequisite** — the spec does **not**
claim the runtime credential is "made uninteresting" until that split is adopted.

- **T-DOCTOR-SINGLEROLE** (binding): a single-role fixture → `daemon doctor` reports
  `rotation_skipped_single_role` as a posture finding; the runbook names the split.

### 5.7 Deploy order + startup preconditions (`C-DDL-DEPLOY-ORDER`, OPS-4)

L1 DDL is **owner-applied out-of-band** (RFC 0079 §5: `striatumd_rw` cannot DDL
owner tables — confirmed: the daemon crash-loops if the runtime role alters owner
objects). The binary's runtime-role auto-migrations must **not** attempt owner DDL.
Ordering is pinned and fail-fast:

- **Order:** owner DDL (functions + revokes + `pgcrypto`) applied **before** the new
  binary that depends on it; both **idempotent**.
- **Startup precondition (new-binary/old-schema):** on boot the daemon checks
  `to_regprocedure('striatumd.append_audit_row(...)')` present and `pgcrypto`
  installed; if absent it **fails closed** with
  `daemon_pg_schema_precondition: append_audit_row missing — apply owner DDL vX.Y
  first` rather than serving mutations against a missing function.
- **Old-binary/premature-REVOKE:** if `INSERT` was revoked but the binary still
  writes via direct `INSERT`, the precondition detects the mismatch and fails closed
  with an actionable message.

- **T-DEPLOY-SKEW** (binding): both skew directions simulated; each fails the
  precondition with an actionable diagnostic **before** the first mutation.

### 5.8 Single-writer-per-role invariant (`C-ROTATION-SINGLE-WRITER`, OPS-5)

One shared `striatumd_rw` password rotated per restart is implicitly
**single-writer-per-PG** (a rolling restart or second/remote daemon would
invalidate the incumbent's reconnect password). Resolution:

- **Default (local):** declare a **single-daemon-per-`striatumd_rw`-role**
  invariant; `daemon doctor` detects a concurrent rotator (a recent
  `daemon_auth_registry.rotated_at` from a different `instance_id`, or a
  `pg_stat_activity` peer on the role) and reports a posture finding.
- **Remote/multi-host:** use a **per-instance role** (`striatumd_rw_<instance>`) so
  concurrent daemons never share a rotated credential. Cross-host client TLS certs
  remain **deferred** until a real multi-host deployment.

- **T-SINGLE-WRITER** (expected-stage): the spec states the invariant + doctor
  probe; the remote-PG section names the per-instance-role behavior.

### 5.9 Dynamic-credentials disposition (`Q-DYNAMIC-CREDENTIALS`, PR-001) — defer-with-successor

PR-001 wants a `BeforeConnect` dynamic password provider to shorten heap residency;
this conflicts with the honest `C-SECRET-HONEST` (pgx retains the password in
`pgxpool.Config` for reconnects → the property is **RAM-only + rotation-on-restart**,
not zeroization). Disposition: **deferred to successor #88-dynamic-creds**.
`C-SECRET-HONEST` is preserved verbatim for 0110; a `BeforeConnect` provider is
recorded as a strict **superset** that shrinks (not eliminates) the heap window. The
same honesty applies to the `daemon_auth` secret (§3.2). This RFC does **not**
over-promise live-process memory secrecy.

### 5.10 Owner break-glass DR (`C-OWNER-DR`, OPS-6)

A non-decryptable systemd `LoadCredentialEncrypted` owner secret (host
re-provision / TPM reset) would lock the operator out. The L0 runbook ships a
**break-glass** path: re-encrypt the owner credential on the new host, or supply a
temporary plaintext owner DSN (`STRIATUM_OWNER_DB_URL`) to re-bootstrap, then
rotate.

### 5.11 Doctor owner reach + privilege-safe probe (`C-DOCTOR-OWNER-REACH`, OPS-7)

Cycle 1's probe used `SELECT rolpassword … FROM pg_authid` (superuser-only; blind in
remote-PG). Corrected: the posture probe uses an **owner-owned `SECURITY DEFINER`
posture function** returning a boolean "rotated/posture OK" **without exposing the
hash or password**, callable by the runtime role; the runbook specifies how
`daemon doctor` obtains **owner** access out-of-band in remote-PG (operator supplies
the owner DSN for the diagnostic), so "the runtime credential is dead" stays
diagnosable.

---

## 6. Privacy gates (L1)

### 6.1 `daemon_auth_log` redaction (`C-AUTH-LOG-PRIVACY`, PR-002)

The owner-owned `daemon_auth_log` must never persist secrets. `detail jsonb` is
written through a **strict key whitelist + redaction path**, not raw:

- a single Go writer accepts only whitelisted keys (`event`, `daemon_version`,
  `instance_id`, `reason_code`, `duration_ms`); anything else is dropped;
- any string value passes a DSN/credential **redactor** (regex over `password=`,
  `://user:pass@`, token/`secret` patterns) before insert;
- raw driver `error` structs / DSNs reduce to a `reason_code` enum, never stored
  verbatim.

- **T-AUTHLOG-REDACT** (binding): feed DSNs, raw driver errors, passwords, tokens,
  params; assert the inserted row contains **only** whitelisted keys and **no**
  secret substring (grep the stored JSON).

### 6.2 Durable event transcript exclusion (`C-EVENT-NO-TRANSCRIPTS`, PR-005)

`events.payload_json` is a broad durable surface; without DB-level validation,
transcript-like provider output could enter authoritative storage (violating D028).
The L1 events phase adds **DB-level** validation in the event write function (plus a
backing `CHECK`/trigger):

- reject payloads whose top-level keys include `stdout`, `stderr`, `transcript`,
  `raw_output`, `provider_output`, `chat`, or `messages`;
- reject payloads over a curated-metadata size ceiling (a transcript smell);
- allow curated, typed, bounded-key metadata events.

- **T-EVENT-NOTRANSCRIPT** (binding): each forbidden shape is rejected at the DB
  boundary; curated metadata events insert green.

Keeps the DB a **curated** record, not a transcript store (AGENTS.md product
boundary; D028).

---

## 7. L2 isolation + harness fidelity

### 7.1 `pgtest` verifies migrations, not Go-patched state (`C-HARNESS-PRIVILEGES`, EV-001)

`pgtest.Pools` today constructs the unprivileged role imperatively in Go (ad-hoc
`GRANT`/`SET ROLE`), so a security test could validate a **polluted** harness — a
false-green risk for `T-42501`. Corrected: roles/privileges are produced **solely**
by owner-applied migrations; `pgtest` connects using those roles and is
**prohibited** from issuing `GRANT`/`REVOKE` on protected tables during a run; the
test-only raw-row writer (§5.1) is the *only* sanctioned bypass and patches **no**
privileges.

- **T-HARNESS-FIDELITY** (binding): harness setup fails if `pgtest` issues ad-hoc
  `GRANT`/`REVOKE` for protected-table privileges; confirms `T-42501` runs against
  migration-defined roles.

### 7.2 Negative lane isolation test (`C-L2-NEG-TEST`, EV-002)

Cycle 1 had only positive doctor-posture tests. Add a **negative** test:

- **T-LANE-ISOLATION-NEG** (binding): a mock lane identity (the dedicated PG-less OS
  user) attempts to connect over (a) the protected `0700` UNIX socket dir and (b)
  loopback TCP; assert **both fail** under the hardened posture (socket: directory
  traversal denied; TCP: `pg_hba`/no listener).

### 7.3 Secure-profile vs legacy upgrade (`C-L2-DEFAULT`, PX-003)

- **Fresh / secure-profile:** the hardened posture (PG-less lane OS user + `0700`
  socket dir) is **enabled**, or `daemon doctor` emits a **blocking** finding naming
  the exact remediation;
- **Legacy upgrade:** `security.pg_socket_hardened` stays **default-false** and
  `daemon doctor` **warns** (not blocks) so existing PEER installs are not stranded;
- **Default-on graduation gate (named now):** the default flips in a named successor
  minor **after** (a) the L2 how-to has shipped one release, (b)
  `T-LANE-ISOLATION-NEG` is green in CI, and (c) the socket-relocation runbook (§7.5)
  is published. The RFC records the successor release id on landing.

### 7.4 Socket-dir `0700` startup assertion (`C-SOCKET-DIR-PERMS`, PR-004)

The daemon asserts at startup that the socket directory mode is **exactly `0700`**
and owned by the daemon identity (distinct from the lane identity); a more permissive
mode is a `daemon doctor` violation and (under the hardened flag) a startup block.

### 7.5 Socket relocation blast radius (`C-SOCKET-RELOCATE-MIGRATION`, OPS-8)

Relocating PG's socket out of `/var/run/postgresql` to a `0700` dir breaks every
out-of-band path: `psql`, monitoring, ad-hoc owner DSNs, the `--as-owner` path, and
the daemon's own pidfile assumptions. The L2 how-to **enumerates the blast radius**
and updates daemon/doctor socket defaults **in lockstep** with the flag, giving the
exact new connection string (`host=<0700-dir>`). Plus a `PGHOST` scrub from the lane
env (the env is already a strict positive allowlist —
`supervisedEnvPassThrough`, `supervision_control.go`). This is the structural close
of **#87** (today advisory-only: `doctor_lane_sandbox.go` emits a `lane_pg_reachable`
warning cleared by `STRIATUM_LANE_OS_USER`).

---

## 8. Binding constraints carried into cycle-2 extraction

The constraints below remain binding inputs for the cycle-2 cross-exam/adjudication.
The 16 discharged in §1.1 keep their ids and gates; the cycle-1 candidate
constraints (§1.3) carry forward. The adjudicator should extract against the §1.1
gates as the falsification surface.

**Standing live for cross-examination (cycle 2).** The most attackable new claims:

1. **§3.2** — is the `daemon_auth` secret *actually* non-spoofable by a `striatumd_rw`
   caller on the same connection, and is the §3.1 residual scoped honestly (does the
   secret add real adversary cost over the DSN, or only relocate it)?
2. **§4.1/§4.3** — does the in-transaction prelude + `DISCARD ALL` + discard-on-error
   truly close every attribution-bleed path EV-004 named (cancel, timeout, panic),
   including the `daemon_auth` secret itself?
3. **§5.7** — do the startup preconditions catch *both* skew directions
   (new-binary/old-schema **and** old-binary/premature-REVOKE) before the first
   mutation, idempotently?
4. **§5.1** — `pgcrypto digest` byte-parity with Go `V3RowHash` across the full
   hostile vector set (multibyte, line/paragraph separators, nulls, integers, ts
   edges) under multiple DB timezones, with `VerifyRows` failing closed on an unknown
   format.

---

## 9. Sequencing (revised, DC4)

1. **Spec corrections (this document) before code:** §3 authority claim, §5.1 v3
   mixed-format contract, §3.4 GUCs-as-labels, §5.3 SD template, §5.7 deploy order,
   §5.4 rollback posture.
2. **L3 + L0 doctor probes (no behavior change):** §4 transaction prelude +
   parameterized GUCs + robust reset; §5.6/§5.11 doctor posture findings; §6.1
   `daemon_auth_log` privacy. Lands first.
3. **L0 rotation + L1 Phase 0:** §5.5 owner bootstrap + fail-closed; §3
   `daemon_auth_registry` + `assert_daemon_authority`; §5.1 v3 helpers;
   `append_audit_row`; **T-EXEC-AUTH**, **T-42501**, **T-GRANT-DRIFT**,
   **T-HARNESS-FIDELITY**, **T-ROLLBACK-POSTURE**, **T-DEPLOY-SKEW**.
4. **L1 Phase 1 (`artifacts`) → Phase 2 (`events`):** same hardened write fn +
   authority gate; §6.2 transcript exclusion in the events phase; each provable-green
   first. RLS on `leases`/`sessions` **last**, row-scoping only.
5. **L2 hardening** behind `security.pg_socket_hardened` (default-false; §7.3 named
   default-on gate): **T-LANE-ISOLATION-NEG**, §7.4 `0700` assertion, §7.5 relocation
   runbook.
6. **Remote/multi-host follow-up:** §5.8 per-instance roles, §5.10 owner DR, §5.11
   doctor owner reach before claiming remote-PG readiness; cross-host TLS certs
   deferred.

Sequenced **after** the RFC 0104/0105 reliability foundation; does **not** block RFC
0103's remaining work. No native code, no wire proxy, no hosted identity
(`C-NO-NATIVE`).

---

## 10. Acceptance (maps to RFC §Acceptance)

- **L0:** a fresh start rotates `striatumd_rw` with no on-disk secret in PEER
  posture; a DSN captured before a restart fails after it; owner-bootstrap failure
  **fails closed** with an owner-attributable diagnostic (**T-OWNER-FAILCLOSED**);
  the single-role skip is a **doctor posture finding** (**T-DOCTOR-SINGLEROLE**);
  single-writer-per-role invariant + doctor detection (**T-SINGLE-WRITER**).
- **L1 / authority:** a direct `striatumd_rw` call to any write fn without the
  daemon-authority secret **fails without mutation** (**T-EXEC-AUTH**); GUC labels
  carry no authority (**T-GUC-LABEL**, **T-REGISTRY-ACL**); direct `INSERT` is
  revoked and stays revoked across migration/pgtest/repair (**T-GRANT-DRIFT** /
  **T-42501**); SD hardening pinned (**T-SD-HARDEN**); `VerifyRows` holds across the
  v2→v3 boundary (**T-VERIFY-MIXED**) with pinned parity (**T-HASH-PARITY**,
  **T-TS**); deploy skew fails fast (**T-DEPLOY-SKEW**); rollback posture defined
  (**T-ROLLBACK-POSTURE**).
- **Attribution:** prelude in-transaction and parameterized; labels reset across
  commit/rollback/cancel/timeout/panic (**T-TX-PRELUDE**, **T-ATTR-RESET**).
- **Privacy:** `daemon_auth_log` whitelisted+redacted (**T-AUTHLOG-REDACT**);
  `events.payload_json` rejects transcript shapes (**T-EVENT-NOTRANSCRIPT**).
- **L2:** with the hardened default, a lane cannot open the socket or loopback
  (**T-LANE-ISOLATION-NEG**); `daemon doctor` **blocks** for secure profiles and
  **warns** for legacy upgrades (**T-L2-DEFAULT**); `0700` asserted at startup;
  relocation runbook shipped.
- `docs/reference/spec.md` carries the daemon→PG auth model + the DB-enforced
  write-boundary invariant (G1+G2, §3.1); `decision-log.md` records each per-phase
  decision on landing.

---

## 11. Rejected / residue (explicitly flagged)

**Rejected:**

- **Narrowing-only for `C-EXEC-AUTH`** (no gate) — rejected: the binding verification
  requires a direct call to **fail without mutation**, which a narrow-claim-only spec
  cannot pass. We add the §3 gate.
- **RLS / GUCs as authority** — rejected: client-controlled labels are not a trust
  boundary (`C-GUC-NONAUTH`); RLS stays row-scoping defense-in-depth.
- **Option B (trusted caller-supplied `row_hash`)** and **porting `encoding/json` to
  PL/pgSQL** — still rejected (cycle 1); the v3 length-prefixed `bytea` canonical
  removes the HTML-escaping/key-order hazard.
- **Inherited RFC-appendix rejects** (PG C extension/bg worker, user-namespace PG,
  `seccomp` socket block, PAM ppid module, quorum gate, `pg_notify`/`SD_LISTEN_FDS`
  to a client, abstract-namespace socket) — unchanged.

**Residue (not hidden):**

- **§3.1 G2 residual:** the authority gate does **not** survive a full live-process
  memory scrape (yields DSN + secret together); bounded by L0 rotation-on-restart and
  L2 lane isolation, not eliminated.
- **§4.2 logging residual:** an operator enabling verbose statement+parameter logging
  could log bound GUC values incl. the secret — runbook hardening item.
- **§5.9 heap residual:** pgx retains the password (and we retain the secret) for
  reconnect; the guarantee is RAM-only + rotation-on-restart, not zeroization
  (`C-SECRET-HONEST`); dynamic-provider shrink deferred to **#88-dynamic-creds**.
- The v2 `V2RowHash` path stays **permanently** as the only reader of pre-cutover
  rows (accepted cost, not tech debt).
