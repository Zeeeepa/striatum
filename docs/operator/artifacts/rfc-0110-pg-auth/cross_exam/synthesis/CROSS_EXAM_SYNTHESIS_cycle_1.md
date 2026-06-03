---
schema_version: striatum.findings_ledger.v1
artifact_kind: findings_ledger
summary_count: 29
author: convener-claude-opus-4.8-002
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 1
title: "RFC 0110 — Cross-examination findings ledger (cycle 1)"
---

# RFC 0110 — Cross-examination findings ledger (cycle 1)

This ledger rolls up **all five cross-examiner postures** against the convener
candidate synthesis into one findings record for the adjudicator. Each finding's
**posture, severity, and status** is preserved verbatim from its source artifact;
the unanswered interrogations are carried forward as evidence (§4). This is a
roll-up, **not** an adjudication: the convener does not render accept / reject
here. The downstream `adjudicate` job (`../../adjudication/adjudicator/`,
`collaboration_ledger`) decides whether the gate clears.

**Target under examination:**
`docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_1.md`

**Inputs rolled up:**

- `../product/CROSS_EXAM.md` — posture **product** (`cross_examiner_1`, codex-gpt-5.5-xhigh-001)
- `../implementation/CROSS_EXAM.md` — posture **implementation** (`cross_examiner_2`, codex-gpt-5.5-xhigh-001)
- `../privacy/CROSS_EXAM.md` — posture **privacy** (`cross_examiner_3`, gemini-3.5-flash-high-001)
- `../eval/CROSS_EXAM.md` — posture **eval** (`cross_examiner_4`, gemini-3.5-flash-high-002)
- `../operations/CROSS_EXAM.md` — posture **operations** (`cross_examiner_5`, claude-opus-4.8-001)

---

## 1. Roll-up summary

- **Postures:** 5 (product, implementation, privacy, eval, operations).
- **Findings:** 29 total — **1 critical, 16 high, 12 medium**.
- **Interrogation status:** **0 of 5 cross-examiners reached the candidate.**
  Every `interrogation.open` against the candidate owner session
  `sess_a6beb21cc70189786cf7c45e63619068` returned
  `status: interrogation_unavailable`, `reason: panel_window_closed`
  (`interrogable_job_id: job_run_8e14cb48342e929d30043d6be24f9101_convener_draft`).
  The operations posture was issued as a **non-interrogable** review job and
  recorded its lead falsifying interrogation textually. **No candidate rebuttal
  was therefore possible for any finding** — every row below is **open /
  unrebutted** and is carried to adjudication as such.
- **Meta-note for the adjudicator:** the candidate synthesis (§ cross-examination
  status) states *"no falsifying interrogation was raised … `interrogation.list`
  count 0 at synthesis time."* That count reflects **panel-window closure**, not
  the absence of challenges — five postures raised 29 findings and five distinct
  falsifying interrogations that the closed window prevented from registering
  (see §4). The "count 0" should not be read as the candidate having survived
  cross-examination.

### Severity distribution by posture

| posture | critical | high | medium | total |
| --- | --- | --- | --- | --- |
| product (PX) | 1 | 2 | 2 | 5 |
| implementation (IX) | 0 | 3 | 3 | 6 |
| privacy (PR) | 0 | 3 | 2 | 5 |
| eval (EV) | 0 | 3 | 2 | 5 |
| operations (OPS) | 0 | 5 | 3 | 8 |
| **total** | **1** | **16** | **12** | **29** |

### Index (scannable; full detail in §3)

| id | posture | severity | status | requested constraint shape |
| --- | --- | --- | --- | --- |
| PX-001 | product | critical | open (no rebuttal — window closed) | `C-EXEC-AUTH` |
| PX-002 | product | high | open (no rebuttal — window closed) | `C-GUC-NONAUTH` |
| PX-003 | product | high | open (no rebuttal — window closed) | `C-L2-DEFAULT` |
| PX-004 | product | medium | open (no rebuttal — window closed) | `C-HASH-V3` |
| PX-005 | product | medium | open (no rebuttal — window closed) | `C-SD-HARDEN` |
| IX-001 | implementation | high | open (no rebuttal — window closed) | `C-TX-GUC-PRELUDE` |
| IX-002 | implementation | high | open (no rebuttal — window closed) | `C-GRANT-DRIFT` |
| IX-003 | implementation | high | open (no rebuttal — window closed) | `C-SD-HARDEN` |
| IX-004 | implementation | medium | open (no rebuttal — window closed) | `C-HASH-BYTEA` |
| IX-005 | implementation | medium | open (no rebuttal — window closed) | `C-SHA-PRIMITIVE` |
| IX-006 | implementation | medium | open (no rebuttal — window closed) | `C-VERIFY-DISPATCH` |
| PR-001 | privacy | high | open (no rebuttal — window closed) | `C-DYNAMIC-CREDENTIALS` |
| PR-002 | privacy | high | open (no rebuttal — window closed) | `C-AUTH-LOG-PRIVACY` |
| PR-003 | privacy | medium | open (no rebuttal — window closed) | `C-GUC-PARAMETERIZED` |
| PR-004 | privacy | medium | open (no rebuttal — window closed) | `C-SOCKET-DIR-PERMS` |
| PR-005 | privacy | high | open (no rebuttal — window closed) | `C-EVENT-NO-TRANSCRIPTS` |
| EV-001 | eval | high | open (no rebuttal — window closed) | `C-HARNESS-PRIVILEGES` |
| EV-002 | eval | high | open (no rebuttal — window closed) | `C-L2-NEG-TEST` |
| EV-003 | eval | medium | open (no rebuttal — window closed) | `C-TEST-ROW-WRITER` |
| EV-004 | eval | high | open (no rebuttal — window closed) | `C-ATTR-RESET-FAIL` |
| EV-005 | eval | medium | open (no rebuttal — window closed) | `C-TZ-INDEPENDENT-HASH` |
| OPS-1 | operations | high | open (no rebuttal — window closed) | `C-RESTART-OWNER-DEP` |
| OPS-2 | operations | high | open — conceded by candidate's own guard (§2/Q4) | `C-L0-ADOPTION-VISIBLE` |
| OPS-3 | operations | high | open (no rebuttal — window closed) | `C-ROLLBACK-FORWARD-ONLY` |
| OPS-4 | operations | high | open (no rebuttal — window closed) | `C-DDL-DEPLOY-ORDER` |
| OPS-5 | operations | high | open (no rebuttal — window closed) | `C-ROTATION-SINGLE-WRITER` |
| OPS-6 | operations | medium | open (no rebuttal — window closed) | `C-OWNER-DR` |
| OPS-7 | operations | medium | open (no rebuttal — window closed) | `C-DOCTOR-OWNER-REACH` |
| OPS-8 | operations | medium | open (no rebuttal — window closed) | `C-SOCKET-RELOCATE-MIGRATION` |

---

## 2. Convergent themes (where postures independently hit the same seam)

These are the seams where two or more postures converged from different angles.
They are the adjudicator's de-duplication targets — the highest-confidence
binding constraints because they survived independent attack.

1. **The leaked-DSN write surface moved, it did not close (the headline).**
   Revoking direct table DML routes writes through owner-owned
   `SECURITY DEFINER` functions, but `striatumd_rw` is granted `EXECUTE`, so any
   process holding only the runtime DSN can still call `append_audit_row` /
   artifact / event functions and produce hash-correct, **authorized-looking**
   writes outside daemon RPC. Raised as **PX-001 (critical)** and reinforced by
   **PX-002**; the candidate's own pre-emptive rebuttals (§5) defend Option-A
   necessity, v3 parity, and RAM-only — **none** answers "what stops direct
   `EXECUTE`?" The product claim *"the RPC/artifact API is the sole write path"*
   is the row most at risk.

2. **Custom GUCs are client-controlled labels, not authority — three postures.**
   **PX-002 (product)** says `striatum.rpc_id` / `principal_id` / `app.session_id`
   set via `SET LOCAL` are spoofable by the same SQL caller, so RLS keyed on them
   is not an authorization boundary. **IX-001 (impl)** says the placement is also
   wrong: `pgxpool.BeforeAcquire` fires before the mutation transaction, so a
   `SET LOCAL` invariant cannot hold there. **PR-003 (privacy)** says
   string-concatenated `SET LOCAL` leaks the values into `pg_stat_activity` query
   text. **EV-004 (eval)** says the reset can be bypassed on transaction
   abort/panic, leaking attribution across pooled checkouts. The GUC mechanism is
   attacked on **authority, transaction placement, leakage, and reset
   robustness** at once.

3. **`SECURITY DEFINER` hardening — requested twice under the same id.**
   **PX-005 (product)** and **IX-003 (implementation)** independently request
   `C-SD-HARDEN`: fixed/locked `search_path`, fully-qualified references, ACLs
   that revoke ambient public execute, no caller-controlled object names. Two
   postures, one constraint shape — extract once.

4. **v3 hash: byte-parity, format dispatch, primitive, and timezone.**
   The v3 design is accepted in principle but the seams are prose:
   **IX-004 / `C-HASH-BYTEA`** (bytea construction, not text concat; multibyte +
   control-char golden vectors), **IX-006 / `C-VERIFY-DISPATCH`** (explicit v2 /
   v3 / unknown / mixed-boundary / tamper behavior), **PX-004 / `C-HASH-V3`**
   (record the v3 format as the decision that supersedes the survey's
   "byte-identical to Go `encoding/json`" wording, v2 preserved permanently),
   **IX-005 / `C-SHA-PRIMITIVE`** (pin `pgcrypto digest` vs core `sha256`, make
   availability part of owner DDL), and **EV-005 / `C-TZ-INDEPENDENT-HASH`**
   (`to_char` is `TimeZone`-GUC sensitive; run parity under multiple DB
   timezones). All five sharpen the candidate's single `C-HASH` row.

5. **Socket-dir hardening blast radius — privacy + operations.**
   **PR-004 / `C-SOCKET-DIR-PERMS`** (assert `0700` at startup, doctor-violate on
   drift) and **OPS-8 / `C-SOCKET-RELOCATE-MIGRATION`** (relocating the socket
   breaks psql / monitoring / owner DSN / `--as-owner` / the daemon's own pidfile)
   are the two faces of the L2 socket move.

6. **Test-harness fidelity — eval + implementation.**
   **EV-001 / `C-HARNESS-PRIVILEGES`** and **IX-002 / `C-GRANT-DRIFT`** both warn
   that `pgtest` constructs the unprivileged role imperatively in Go (ad-hoc
   `GRANT`/`SET ROLE`), so the suite can validate a polluted harness state rather
   than the migrations-enforced production privileges; **EV-002 / `C-L2-NEG-TEST`**
   adds the missing negative L2 isolation test.

7. **Operations: silent inversions of the RFC's own goals + availability wedges.**
   **OPS-2** — L0 is a **silent no-op** in the as-shipped single-role live
   posture (`postgres://halbritt@/postgres?host=/var/run/postgresql`); the dev
   guard skips rotation, so the headline anti-leak property is inert until an
   undocumented owner≠runtime split. **OPS-5** — a single shared per-restart
   rotated role is implicitly single-writer-per-PG, contradicting the stated
   RFC-0107 remote-PG / multi-principal substrate goal. **OPS-1 / OPS-3 / OPS-4**
   are availability wedges of a hard-prerequisite daemon (every-restart owner
   dependency; forward-only v3 rollback indistinguishable from tamper;
   owner-DDL↔binary deploy-skew wedging audit writes).

---

## 3. Findings (full detail, grouped by posture)

Each row preserves the source `id`, `severity`, `affected invariant`, the
challenge/finding, the closest acceptable answer, and the required constraint
shape. Status for every row: **open — no candidate rebuttal (panel window
closed)** unless noted.

### 3.1 Product posture — `cross_examiner_1` (codex-gpt-5.5-xhigh-001)

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape |
| --- | --- | --- | --- | --- | --- |
| PX-001 | critical | "Make a leaked runtime credential uninteresting"; "the RPC/artifact API is the sole write path." | The candidate closes malformed direct table DML, but not direct invocation of the new write API. With `EXECUTE` on `append_audit_row` (later artifact/event functions), any process holding only the runtime DSN can call them outside daemon RPC; in-DB hashing makes the forged row hash-correct, not authorized. | Either narrow the product claim to "leaked runtime credentials cannot violate append/hash/attempt invariants, but can still perform valid function writes" (keep L0/L2 as the real anti-leak layers), or add a non-spoofable daemon-authority gate to every `SECURITY DEFINER` write function. | `C-EXEC-AUTH`: a PG-gated negative test connects as `striatumd_rw` with no valid daemon capability context and proves direct calls to each write function fail without mutating `audit_log`/`artifacts`/`events`; the normal daemon RPC path still succeeds. |
| PX-002 | high | L3 attribution and L1 RLS are trustworthy enforcement inputs. | The synthesis treats `striatum.rpc_id`/`principal_id`/`app.session_id` as if they carry authority. GUCs set with `SET`/`SET LOCAL` are client-controlled labels unless anchored to something the raw SQL caller cannot forge; RLS keyed only on `current_setting('app.session_id', true)` is not an authorization boundary against a leaked runtime credential. | Specify these GUCs as **audit labels only** unless the write functions verify them against a server-side, unexpired RPC/capability record not spoofable by `striatumd_rw`. | `C-GUC-NONAUTH`: direct SQL as `striatumd_rw` sets fake GUCs; every protected write still fails. A positive test proves real daemon RPC sets/clears the same labels for attribution. |
| PX-003 | high | L2 "hardened default" closes #87 without stranding upgrades. | The product phrase "PG-less lane OS user + 0700 socket dir as hardened default (closes #87)" is underspecified given `security.pg_socket_hardened` is default-false with a later flip; what is default for new installs and what concrete release/evidence gate closes the warning-only state? | Split compatibility from target posture: existing installs keep a default-false upgrade flag; new secure adoptions are guided to the PG-less lane user + 0700 dir immediately with `daemon doctor` naming the remediation; define the default-on graduation gate now. | `C-L2-DEFAULT`: fresh/adopted secure-profile installs either enable the hardened posture or emit a blocking doctor finding; legacy upgrades warn until the announced flip; the RFC records flip criteria + successor release. |
| PX-004 | medium | C-HASH / DC1 remains an accepted, decision-backed contract. | v3 length-prefixed hashing is product-acceptable, but it changes the survey's binding wording from "byte-identical to Go `encoding/json`" to "versioned format for new rows, v2 preserved" — a real contract change, not an implementation detail. | Record v3 as the explicit answer that supersedes the survey's C-HASH wording for new rows while preserving v2 verification permanently; the decision-log/spec update must name the mixed-format invariant. | `C-HASH-V3`: `VerifyRows` dispatches per `hash_format_version`; v2 rows verify with `V2RowHash`, v3 with the length-prefixed format; a mixed v2-to-v3 segment-boundary chain verifies green and tamper tests fail red. |
| PX-005 | medium | Owner-owned `SECURITY DEFINER` functions are a hardened DB boundary, not a new privilege-confusion surface. | The synthesis requires owner-owned `SECURITY DEFINER` functions but does not state the standard hardening contract (fixed `search_path`, fully-qualified references, no ambient public execute, no caller-controlled names), risking a new owner-privilege footgun. | Make `SECURITY DEFINER` hardening a named L1 acceptance constraint for every function phase. | `C-SD-HARDEN`: every owner-owned write function sets a locked-down `search_path` (or fully qualifies references), grants `EXECUTE` only to intended roles, revokes ambient public execute, and has a migration test inspecting `pg_proc`/ACL state. |

**Product posture verdict (recorded):** the candidate is strongest on hash parity
and sequencing but the product claim overreaches exactly where D164 is meant to
harden — a leaked runtime DB credential. The implementation-ready spec must either
lower that claim honestly or add a testable DB-side authority gate a raw
`striatumd_rw` connection cannot spoof.

### 3.2 Implementation posture — `cross_examiner_2` (codex-gpt-5.5-xhigh-001)

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape |
| --- | --- | --- | --- | --- | --- |
| IX-001 | high | L3 labels are set transaction-locally before every authoritative mutation. | The synthesis compresses L3 into `pgxpool.BeforeAcquire`/`AfterRelease` GUC set/reset, but the current pool (`go/pkg/db/connection.go`) has no hooks, and `BeforeAcquire` is the wrong place for `SET LOCAL` (it fires before the mutation transaction). Session-level `SET`+reset is possible, but then the spec must stop saying transaction-local. | Move L3 into the transaction boundary: after `BeginTx`, before the first DML, run a shared `SET LOCAL` prelude for `rpc_id`/`principal_id`/(optional `app.session_id`); make all mutation handlers use it. If session-level `SET` is chosen, rename the guarantee and make reset failure a poisoned-connection discard. | `C-TX-GUC-PRELUDE`: a guard test fails any mutating transaction whose first statements omit the attribution prelude; a rollback/commit-path test proves labels are gone on the next checkout. |
| IX-002 | high | L1 revokes survive migrations, pgtest setup, and grant repair. | Migrations grant broad table DML to `striatumd_rw` (`0005_repo_local_workflow_state.sql` grants `SELECT,INSERT,UPDATE,DELETE ON ALL TABLES`) and `pgtest` recreates that posture per-test role; a one-off `REVOKE INSERT` can be silently re-opened by later grant-repair or test setup. | Treat negative grants as part of the schema contract: update migration helpers, pgtest role setup, and any doctor grant-repair path so direct INSERT stays revoked on `audit_log`, then `artifacts`, then `events`. | `C-GRANT-DRIFT`: a migration-forward test and a doctor/repair-grants test assert direct INSERT to each protected table still fails `42501` after all grant setup runs. |
| IX-003 | high | Owner-owned `SECURITY DEFINER` functions are safe to execute from `striatumd_rw`. | The candidate requires owner-owned write functions but does not specify hardening; `SECURITY DEFINER` without fixed `search_path`, fully-qualified references, and explicit ACLs is a known privilege-confusion surface — L1 should not swap table-DML exposure for owner-function exposure dependent on ambient lookup. | Make the DDL template for every L1 function include a locked-down `search_path`/fully-qualified references, revoke ambient public execute, grant only intended roles, and keep bodies free of caller-controlled names/operators. | `C-SD-HARDEN`: inspect `pg_proc.proconfig` and ACLs in tests, plus a hostile-schema/search-path regression proving the function still reaches the intended `striatumd` objects. *(Same constraint id as PX-005.)* |
| IX-004 | medium | V3 hash parity is byte-exact across Go and PL/pgSQL. | PostgreSQL text concatenation is not byte construction. The PL/pgSQL side must build hashed material as bytea from `convert_to(value,'UTF8')`, length-prefix bytes, and stable integer/timestamp text — not rely on implicit text concat or locale-sensitive formatting. | Specify the SQL hash helper as a bytea-producing function with fixed field order, fixed UTF-8 conversion, fixed RFC3339 UTC formatting, and no implicit casts whose output can vary. | `C-HASH-BYTEA`: golden tests include ASCII, multibyte UTF-8, `<`, `>`, `&`, U+2028/U+2029, empty string, null, integer fields, and timestamp edge cases, comparing Go `V3RowHash` to SQL output byte-for-byte. |
| IX-005 | medium | The SQL hash primitive is stock, available, and owner-applied. | The synthesis names `sha256`/`digest` behavior but does not pin the primitive: `digest(...,'sha256')` implies `pgcrypto` (owner migration must install/check it; the non-native claim must allow that stock extension); a core `sha256(bytea)` instead requires naming the supported PG version floor. | Pick one primitive and make its availability part of owner-applied DDL plus doctor/migration tests. | `C-SHA-PRIMITIVE`: migrations create/verify the stock extension required for hashing or assert the core function exists on the supported PG version; missing support fails before revoking direct INSERT. |
| IX-006 | medium | `VerifyRows` remains a permanent mixed-format verifier. | Current `VerifyRows` recomputes `V2RowHash` unconditionally; the candidate says it will dispatch on `hash_format_version` but needs explicit behavior for v2, v3, unknown formats, segment boundaries, and tamper cases. | Add `V3RowHash`, make `VerifyRows` switch on format version, keep v2 permanently, and report unknown versions as verifier failures (not a silent v2 fallback). | `C-VERIFY-DISPATCH`: tests cover v2-only, v3-only, mixed v2-to-v3 with `previous_hash` continuity, unknown-format failure, and tamper detection for each format. |

**Implementation posture verdict (recorded):** close enough to implement, but it
needs hard executable contracts at the seams where prose currently does the work.
The most urgent correction is the L3 GUC placement (a `BeforeAcquire` hook cannot
provide a transaction-local `SET LOCAL` invariant by itself).

### 3.3 Privacy posture — `cross_examiner_3` (gemini-3.5-flash-high-001)

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape |
| --- | --- | --- | --- | --- | --- |
| PR-001 | high | L0 credentials remain strictly ephemeral and RAM-only in-memory. | The rotated DB password is stored in the static `pgxpool.Config` for reconnection, leaving plaintext credentials resident in the Go heap indefinitely and widening the core-dump / heap-inspection surface. | Retrieve the rotated password dynamically via a password callback / connection hook (e.g. `BeforeConnect`) and zero transient credential strings immediately after connection establishment. | `C-DYNAMIC-CREDENTIALS`: the pool uses dynamic password providers to minimize plaintext-password lifetime in heap memory. *(Tension with candidate §Q5/C-SECRET-HONEST, which deliberately declines zeroization because pgx retains the password for reconnect — adjudicator to reconcile.)* |
| PR-002 | high | `daemon_auth_log` contains no secret credentials or sensitive parameters. | The proposed JSONB `detail` field is unconstrained, risking raw driver connection errors, unredacted DSNs, or failed connection parameters being logged during bootstrap/rotation failures. | Define a strict serialization whitelist for `detail` keys; explicitly sanitize/redact any error messages containing passwords, tokens, or DSN patterns before insertion. | `C-AUTH-LOG-PRIVACY`: logs written to `daemon_auth_log` undergo strict DSN/credential redaction, validated by error-path serialization tests. |
| PR-003 | medium | L3 attribution labels are not visible to unauthorized local DB sessions. | Setting custom GUCs via string-concatenated SQL (`SET LOCAL striatum.principal_id = '...'`) exposes the values in `pg_stat_activity` query text, making session/principal identifiers visible to other local sessions. | Set GUCs exclusively via parameterized pgx queries / `SELECT set_config('striatum.principal_id', $1, true)` so values are masked in system views. | `C-GUC-PARAMETERIZED`: custom GUCs are configured using parameterized statements to prevent parameter leakage in DB statistics. |
| PR-004 | medium | Sandboxed lane users cannot connect out-of-band to PostgreSQL. | Relocating UNIX sockets to a `0700` dir restricts connection to the daemon identity, but the daemon does not verify parent-directory permissions at startup; any manual override or drift allows local-user bypass. | Enforce directory-permission checks at startup, refusing to bind the PG client socket if the directory is more permissive than `0700`. | `C-SOCKET-DIR-PERMS`: daemon init asserts socket-folder permissions are exactly `0700` and reports relaxed configurations as doctor violations. |
| PR-005 | high | Curated DB records exclude raw provider transcripts (D028 compliance). | The `events` table stores arbitrary `payload_json`; without DB-level validation, LLM transcripts or terminal stdout/stderr could bleed into durable columns via custom events. | Apply DB-level constraints/validation to `events.payload_json` to reject raw provider outputs and transcript-related keys. | `C-EVENT-NO-TRANSCRIPTS`: trigger-based or schema constraints verify event payloads contain only curated metadata, rejecting stdout/stderr streams. |

**Privacy posture summary (recorded):** the load-bearing concerns are credential
leakage to the Go heap (PR-001) and unredacted parameters bleeding into
`daemon_auth_log` (PR-002); parameterizing GUCs (PR-003) and enforcing socket-dir
constraints (PR-004) satisfy the D028/D151 boundaries.

### 3.4 Eval posture — `cross_examiner_4` (gemini-3.5-flash-high-002)

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape |
| --- | --- | --- | --- | --- | --- |
| EV-001 | high | Harness fidelity: tests run under the true production security model. | `pgtest.Pools` runs ad-hoc `GRANT`/`SET ROLE` in Go to configure the unprivileged role, overriding schema privileges applied by migrations; this pollutes the environment and prevents the suite from validating that the migrations themselves enforce privileges (e.g. revoking INSERT). | Remove ad-hoc privilege modifications from Go test helpers; roles/privileges are created strictly by owner-applied migrations, and `pgtest` connects only using those predefined roles. | `C-HARNESS-PRIVILEGES`: `pgtest` connections use roles whose permissions are defined solely by schema migrations; Go test code is prohibited from `GRANT`/`REVOKE` to patch permissions during a run. |
| EV-002 | high | L2 isolation prevents untrusted lane access to PostgreSQL. | The candidate includes positive L2 tests (doctor blocks under flag) but no negative test asserting a process in the lane sandbox is actually blocked from PG; drift in `pg_hba.conf` or socket-dir perms would go undetected. | Add a negative test `T-LANE-ISOLATION-NEG` where a mock lane process attempts to connect over UNIX socket and loopback TCP, asserting all attempts fail. | `C-L2-NEG-TEST`: the suite runs a non-privileged runner process attempting out-of-band DB access, proving it is blocked under the hardened default. |
| EV-003 | medium | `VerifyRows` correctly validates mixed v2-to-v3 and tampered chains. | Verifying the v2→v3 transition is load-bearing, but the harness has no mechanism to write malformed/mixed-version rows; modifying production code to allow test-only bypasses degrades design integrity. | Equip `pgtest` with a privileged backdoor utility to write raw rows directly to `audit_log` (bypassing the `SECURITY DEFINER` functions) to construct mixed/tampered chains for the verifier. | `C-TEST-ROW-WRITER`: a test-only DB utility can write arbitrary invalid/tampered rows to verify `VerifyRows`' mixed-format and tamper-detection coverage. |
| EV-004 | high | L3 attribution GUCs are reliably reset on connection reuse. | Attribution labels must reset on pool release, but if a transaction panics/aborts/times out the `pgx` `AfterRelease` hook may be bypassed or leave GUCs dirty, causing cross-transaction info leaks. | Write a test that checks out a connection, sets attribution GUCs, simulates a transaction failure/panic, then verifies a subsequent checkout has GUCs reset to defaults. | `C-ATTR-RESET-FAIL`: `T-ATTR-RESET` proves labels are cleared even after transaction aborts, query cancellations, and driver panics. |
| EV-005 | medium | V3 timestamp formatting is locale- and timezone-independent. | The DB-computed timestamp relies on `to_char` to match Go's RFC3339; PostgreSQL `to_char` behavior can vary with the server `TimeZone` GUC, risking hash mismatch in remote-PG or skewed hosts. | Run `T-TS` under multiple active PostgreSQL timezones (UTC, EST, Asia/Kolkata, Australia/Lord_Howe) and assert in-DB hashing produces identical output byte-for-byte. | `C-TZ-INDEPENDENT-HASH`: `T-TS` runs hashing checks against diverse DB `TimeZone` settings to prove formatting/hashing are timezone-independent. |

**Eval posture summary (recorded):** the harness `pgtest` has a critical gap — it
constructs the unprivileged model imperatively in Go rather than verifying the
declarative migrations. Privilege pollution (EV-001), negative L2 validation
(EV-002), and pool-reset robustness (EV-004) are required to make the RFC 0110
implementation verifiable.

### 3.5 Operations posture — `cross_examiner_5` (claude-opus-4.8-001) — non-interrogable review job

| id | severity | affected invariant | one-line challenge | closest acceptable answer | constraint shape |
| --- | --- | --- | --- | --- | --- |
| OPS-1 | high | daemon availability (hard prerequisite for every verb) | Every restart now hard-depends on owner-credential availability to derive the runtime password; a transient owner failure bricks startup where a runtime-DSN start would succeed today. | Fail closed with an owner-attributable diagnostic; document the new every-restart owner dependency as a deliberate trade. | `C-RESTART-OWNER-DEP`: L0 fails closed with an owner-attributable diagnostic on owner-bootstrap failure; the every-restart owner dependency is documented as an accepted trade. |
| OPS-2 | high | L0 goal "make a leaked runtime credential uninteresting" | In the documented live single-role posture the dev guard **skips rotation**, so L0 is a silent no-op; the only signal is a WARN. **(Conceded: candidate §2/Q4's own answer is "no" — L0 inert until an owner≠runtime split.)** | Make the skip a `daemon doctor` posture finding (not just a log line) + document the owner≠runtime role-split as an L0 adoption prerequisite. | `C-L0-ADOPTION-VISIBLE`: single-role rotation-skip surfaces as a `daemon doctor` posture finding (not only a WARN); owner≠runtime split is a documented L0 prerequisite. |
| OPS-3 | high | audit-chain verifiability across a routine binary rollback | After Phase 0 writes a v3 row, the prior daemon's v2-only `VerifyRows` reports the chain broken — rollback becomes a one-way door indistinguishable from tamper. | Declare Phase 0 forward-only; gate v3 writes behind an operator-committed flag, OR backport per-format dispatch to the rollback target, OR document the one-way door + verify-tolerance runbook. | `C-ROLLBACK-FORWARD-ONLY`: L1 Phase 0 declared forward-only; rollback posture specified (flagged cutover / backported verifier / documented one-way door + verify tolerance). |
| OPS-4 | high | write availability (`audit_log` on every mutation's path) | Migrations auto-apply as the **runtime** role at startup, but L1 DDL must be **owner-applied** out-of-band with no defined ordering vs. the binary; either skew direction wedges all audit writes. | Add a startup schema-precondition check + define the owner-DDL-then-binary upgrade ordering so skew fails fast with an actionable error. | `C-DDL-DEPLOY-ORDER`: owner-applied-DDL↔runtime-binary upgrade ordering defined + a startup precondition check; skew fails fast with an actionable error rather than silently wedging audit writes. |
| OPS-5 | high | the RFC's own multi-principal / remote-PG substrate goal (RFC 0107) | One shared `striatumd_rw` role rotated per restart is implicitly single-writer-per-PG; a rolling restart or a second/remote daemon invalidates the incumbent's reconnect (the single-instance guard is per-socket-dir, not per-PG-role). | State the single-daemon-per-role invariant + doctor detection, OR scope the rotated credential per daemon instance for the remote-PG path. | `C-ROTATION-SINGLE-WRITER`: rotation assumes one daemon per `striatumd_rw` role (doctor-detectable), or remote-PG uses a per-instance role — never a shared rotated role across concurrent daemons. |
| OPS-6 | medium | operator's ability to recover their own daemon (DR) | A non-decryptable systemd `LoadCredentialEncrypted` owner secret (host re-provision / TPM reset) locks the operator out with no in-band recovery. | Ship a break-glass runbook (re-encrypt on new host / temporary plaintext owner DSN to re-bootstrap). | `C-OWNER-DR`: the spec/runbook includes a break-glass recovery path for an unavailable/non-decryptable owner credential. |
| OPS-7 | medium | diagnosability when the runtime credential is dead (`daemon_auth_log` raison d'être) | The owner-fallback read assumes owner access in the doctor's invocation context; an interactive `daemon doctor` in the remote-PG posture has no owner connection — blind in the exact failure it targets. The `pg_authid` posture probe also needs superuser-only catalog access. | Specify how doctor obtains owner access out-of-band; replace the `pg_authid` read with a privilege-safe probe. | `C-DOCTOR-OWNER-REACH`: owner-fallback diagnosis specifies how `daemon doctor` reaches owner access in the remote-PG posture; the posture probe avoids superuser-only catalogs. |
| OPS-8 | medium | upgrade safety / operator continuity (extends DC3 past the flag default) | Relocating PG's socket to a `0700` dir changes the path away from `host=/var/run/postgresql` for psql, monitoring, ad-hoc owner DSNs, the `--as-owner` path, **and the daemon's own pidfile** — every out-of-band path breaks when hardening is enabled. | L2 how-to must enumerate the blast radius and update the daemon/doctor socket defaults in lockstep with the flag, with the exact new connection string. | `C-SOCKET-RELOCATE-MIGRATION`: enabling L2 socket hardening ships a connection-string migration note (psql/monitoring/owner DSN/pidfile) and updates daemon/doctor defaults in lockstep. |

**Operations posture verdict (recorded):** **not yet implementation-ready** — two
HIGH findings are *silent inversions* of the RFC's own goals (OPS-2 L0 inert in
the as-shipped deployment; OPS-5 single-writer-per-PG vs. the stated remote-PG /
RFC-0107 substrate) and two more are *availability wedges* of a hard-prerequisite
daemon (OPS-3 rollback, OPS-4 deploy ordering). None requires abandoning the
L0–L3 design; each has a bounded closest-acceptable answer. Recommended
disposition: **needs_revision**.

---

## 4. Unanswered interrogations carried forward as evidence

Every cross-examiner attempted to put a falsifying interrogation to the candidate
owner session and was refused by a closed panel window. These are preserved here
as **evidence for the adjudicator** — they are the questions the candidate has not
answered, and per the candidate synthesis they were never registered
(`interrogation.list` count 0). The closed-window refusal is itself evidence that
cross-examination did not actually run against a live owner.

**Common refusal envelope (all four interrogable postures):**
`interrogation.open → status: interrogation_unavailable, reason: panel_window_closed,
target_session_id: sess_a6beb21cc70189786cf7c45e63619068,
interrogable_job_id: job_run_8e14cb48342e929d30043d6be24f9101_convener_draft`.
No interrogation id was created, so no `interrogation.ask` or target rebuttal was
possible.

| posture | audit id | evidence for | unanswered interrogation |
| --- | --- | --- | --- |
| product | `6618478` | PX-001 / PX-002 | "Your L1 grants `EXECUTE` on owner-owned write functions to `striatumd_rw`. What prevents a process with only the runtime DSN, and no daemon MCP or capability token, from calling `append_audit_row` / artifact / event functions directly with syntactically valid inputs and producing hash-correct but unauthorized writes? If the answer is L3 GUCs or RLS, why are those not spoofable by the same SQL caller?" |
| implementation | `6622578` | IX-001 | "Where exactly is `SET LOCAL striatum.rpc_id`, `striatum.principal_id`, and `app.session_id` executed? `pgxpool.BeforeAcquire` runs before a mutation transaction exists, while `SET LOCAL` only has transaction scope. Which wrapper proves these labels are set after `BEGIN` and before the first SQL statement for every mutating handler?" |
| privacy | `6634722` | PR-002 | "The proposed `daemon_auth_log` table captures arbitrary JSONB `detail` payloads on rotation and bootstrap events. How do we guarantee that connection strings, raw driver configuration structs, or credentials from failed DSNs do not bleed into these persistent logs, violating the RAM-only and credential-secrecy guarantees of L0?" |
| eval | `6633220` | EV-001 | "The test harness (`pgtest.Pools`) executes ad-hoc `GRANT` and `SET ROLE` statements in Go to configure the unprivileged test role, overriding the schema privileges applied by migrations. How can we ensure privilege tests like `T-42501` are not testing a polluted harness-specific state rather than the actual migrations-enforced production privileges?" |
| operations | n/a (non-interrogable review job; recorded textually) | OPS-2 (and the operations set) | "On the documented *live* deployment (`postgres://halbritt@/postgres?host=/var/run/postgresql`), where the daemon authenticates over PEER as its own OS user and there is no separate `striatumd_rw` role — does L0 rotation actually run, and does the operator get any L0 hardening on the day the L0 release lands?" — The candidate's own §2/Q4 answers **no** (single-role guard skips rotation), which is recorded as OPS-2. |

---

## 5. Constraint-shape catalog (seed for the adjudicator's extraction)

Each `C-*` is the constraint shape its cross-examiner requested. They are additive
to the candidate's existing `C-HASH / C-INSERT-REVOKE / C-OWNER-DDL /
C-UPGRADE-SAFE / C-NO-NATIVE / C-ATTR-RESET / C-SECRET-HONEST` set. The
adjudicator decides which become binding; **`C-SD-HARDEN` is requested by two
postures (PX-005, IX-003)** — extract once.

| constraint | from | severity | sharpens / extends candidate row |
| --- | --- | --- | --- |
| `C-EXEC-AUTH` | PX-001 | critical | new — direct-`EXECUTE` authority gate vs. C-INSERT-REVOKE |
| `C-GUC-NONAUTH` | PX-002 | high | new — GUCs are labels, not authority (vs. L3 / Q7 RLS) |
| `C-TX-GUC-PRELUDE` | IX-001 | high | corrects L3 placement (vs. §2 L3 / C-ATTR-RESET) |
| `C-GRANT-DRIFT` | IX-002 | high | extends C-INSERT-REVOKE to migration/pgtest/grant-repair |
| `C-SD-HARDEN` | PX-005 + IX-003 | high | new — `SECURITY DEFINER` hardening (extends C-OWNER-DDL) |
| `C-ATTR-RESET-FAIL` | EV-004 | high | strengthens C-ATTR-RESET (abort/panic/cancel) |
| `C-HARNESS-PRIVILEGES` | EV-001 | high | new — harness verifies migrations, not Go-patched privileges |
| `C-L2-NEG-TEST` | EV-002 | high | new — negative L2 isolation test (extends C-UPGRADE-SAFE/L2) |
| `C-DYNAMIC-CREDENTIALS` | PR-001 | high | tension with C-SECRET-HONEST (adjudicator to reconcile) |
| `C-AUTH-LOG-PRIVACY` | PR-002 | high | new — `daemon_auth_log` detail redaction (extends Q8) |
| `C-EVENT-NO-TRANSCRIPTS` | PR-005 | high | new — D028 DB-level transcript exclusion |
| `C-RESTART-OWNER-DEP` | OPS-1 | high | new — every-restart owner dependency, fail-closed |
| `C-L0-ADOPTION-VISIBLE` | OPS-2 | high | new — L0 inert in single-role; doctor posture finding |
| `C-ROLLBACK-FORWARD-ONLY` | OPS-3 | high | new — v3 cutover forward-only vs. binary rollback |
| `C-DDL-DEPLOY-ORDER` | OPS-4 | high | extends C-OWNER-DDL with deploy ordering + precondition |
| `C-ROTATION-SINGLE-WRITER` | OPS-5 | high | new — single-writer-per-PG vs. RFC-0107 substrate |
| `C-HASH-V3` | PX-004 | medium | records v3 as superseding survey C-HASH wording |
| `C-HASH-BYTEA` | IX-004 | medium | sharpens C-HASH (bytea construction + golden vectors) |
| `C-SHA-PRIMITIVE` | IX-005 | medium | sharpens C-HASH (pin pgcrypto/core; owner DDL) |
| `C-VERIFY-DISPATCH` | IX-006 | medium | sharpens C-HASH (explicit dispatch/unknown/tamper) |
| `C-TZ-INDEPENDENT-HASH` | EV-005 | medium | sharpens C-HASH (timezone-independent `to_char`) |
| `C-GUC-PARAMETERIZED` | PR-003 | medium | new — GUCs via `set_config($1)` (no stat leakage) |
| `C-SOCKET-DIR-PERMS` | PR-004 | medium | new — startup `0700` assertion (extends L2) |
| `C-TEST-ROW-WRITER` | EV-003 | medium | new — test-only raw-row writer for verify coverage |
| `C-OWNER-DR` | OPS-6 | medium | new — break-glass for non-decryptable owner secret |
| `C-DOCTOR-OWNER-REACH` | OPS-7 | medium | new — owner-fallback reach + privilege-safe probe |
| `C-SOCKET-RELOCATE-MIGRATION` | OPS-8 | medium | new — socket-move blast radius + lockstep defaults |

---

## 6. Status note for the adjudicator

- **Disposition is not decided here.** This findings_ledger preserves postures,
  severities, and statuses; the `adjudicate` job's `collaboration_ledger` renders
  the verdict and extracts the binding `constraints[]`. Per the convener role,
  dialogue completion is not acceptance.
- **All 29 findings are open / unrebutted** because the candidate's interrogation
  window was closed for the entire cross-examination phase. The single finding
  the candidate has effectively conceded is **OPS-2** (its own §2/Q4 guard).
- **One critical row (PX-001) and the GUC-authority cluster (PX-002 / IX-001 /
  PR-003 / EV-004) are the most load-bearing.** If the adjudicator extracts
  nothing else, the leaked-DSN direct-`EXECUTE` authority gate (`C-EXEC-AUTH`)
  and the GUCs-are-labels-not-authority constraint (`C-GUC-NONAUTH`) are the two
  that decide whether RFC 0110's headline product claim survives.
- **Recommended downstream disposition (advisory, not binding):**
  **needs_revision** — one critical and sixteen high findings remain unanswered,
  with two HIGH operations findings that are silent inversions of the RFC's own
  goals.

— rolled up for adjudication (`../../adjudication/`).
