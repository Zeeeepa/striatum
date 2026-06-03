---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: convener-claude-opus-4.8-001
workflow: rfc-0110-pg-auth-panel
phase: convener_synthesis
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 1
title: "RFC 0110 — Convener synthesis (cycle 1): decisions of record + binding constraints"
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/survey/synthesis/SURVEY_SYNTHESIS.md
  - docs/operator/artifacts/rfc-0110-pg-auth/survey/scan/SURVEY.md
---

# RFC 0110 — Convener synthesis (cycle 1)

The implementation-ready synthesis for RFC 0110 (daemon→PostgreSQL authentication
and the database-enforced write boundary). It consolidates the candidate
(`../draft/CANDIDATE_cycle_1.md`) into decisions of record, the binding
constraints the adjudicator should extract, and the falsifiable acceptance gates.
Full rationale and code anchors live in the candidate and survey; this artifact
is the authoritative, constraint-bearing version.

**Cross-examination status (cycle 1):** no falsifying interrogation was raised
against the candidate (`interrogation.list` count 0 at synthesis time). The three
claims the candidate flagged as most attackable are pre-emptively defended in §5.

**Constraints discharged this cycle:** none — this is cycle 1, there is no prior
`constraints[]` to discharge. On any revision cycle each prior row will be
discharged explicitly (answer / fold-in / reject-with-rationale /
accept-as-risk / defer-with-successor).

---

## 1. Decisions of record (Q1–Q9 resolved)

| # | Question | Decision |
| --- | --- | --- |
| **Q1** | L1 hash: in-DB (A) vs trusted caller-supplied (B) | **Option A** — hash computed inside the owner-owned `SECURITY DEFINER` function. B is rejected: it lets a leaked-DSN caller inject a chain-breaking `row_hash` that `VerifyRows` flags (integrity DoS); a validating B *is* A. |
| **Q1′** | How to make in-DB hashing robust | **Introduce `hash_format_version = 3`**, a length-prefixed canonical form (§3) that is escaping-free and byte-identical in Go and PL/pgSQL — *not* a PL/pgSQL re-implementation of Go `encoding/json` (that would have to match HTML-escaping + codepoint key order forever). `VerifyRows` dispatches per row on `hash_format_version`; v2 rows verify unchanged. |
| **Q2** | `ts` representation in-DB | RFC3339 second-truncated UTC **string** via `to_char(ts AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`, equal to Go `time.Time.UTC().Truncate(1s).Format(RFC3339)`. Pinned by **T-TS**. |
| **Q3** | `SECURITY DEFINER` function scope | The function `striatumd.append_audit_row(...)` owns the **whole** append: `FOR UPDATE` chain-head lock, segment open/resolve, v3 hash, INSERT, head UPDATE. `RecordRPCTransport` becomes a thin caller. |
| **Q4** | L0 remote-PG + dev guard | `STRIATUM_OWNER_DB_URL` for the owner bootstrap when no PEER socket; recommended at-rest form = systemd `LoadCredentialEncrypted=`. Single-role dev guard: owner==runtime ⇒ skip rotation with a `WARN`. |
| **Q5** | L0 in-memory password hygiene | RAM-only + rotation-on-restart is the guarantee, **not** zeroization: pgx retains the password in `pgxpool.Config` for reconnects, so we zero only the transient bootstrap copy and do not over-promise. Honest scope. |
| **Q6** | L2 adoption | Default-false `security.pg_socket_hardened`; ship how-to (lane user + `0700` socket dir); doctor block message names the remediation; flip default-on in a later announced minor. |
| **Q7** | L1 RLS scope | RLS on per-session tables `leases`, `sessions` keyed on `current_setting('app.session_id', true)`, set per-tx; **second tier**, lands after the function phases; composes with L3 `SET LOCAL` + pool reset. |
| **Q8** | `daemon_auth_log` | Owner-owned table `(auth_event_id, ts, event, daemon_version, detail jsonb)`, `event ∈ {bootstrap, rotated, rotation_skipped_single_role, rotation_failed}`, no secrets; `daemon doctor` reads it over the **owner** connection even when the runtime credential is dead. |
| **Q9** | `42501` CI gate | **T-42501** in the `pgtest` harness: direct `INSERT` into `audit_log` as `striatumd_rw_<db>` fails `42501`; the `append_audit_row` positive path succeeds; runs every migration-forward. |

---

## 2. Spec by layer (condensed; full detail + code anchors in the candidate §0–§4)

- **L0 — credential:** owner-PEER (or `STRIATUM_OWNER_DB_URL`) bootstrap →
  `ALTER ROLE striatumd_rw PASSWORD '<crypto/rand>'` → runtime `pgxpool` opens as
  `striatumd_rw`, password RAM-only, re-rotated every restart. Dev guard + doctor
  `db-credential-posture` probe (asserts a password is set without reading it).
- **L1 — enforcement:** owner-applied DDL; per phase: `SECURITY DEFINER` write
  function + `REVOKE INSERT` from `striatumd_rw` (keeps `EXECUTE` only). Phase 0
  `audit_log` (with v3 hash + new segment at cutover), Phase 1 `artifacts`
  (attempt-scope immutability in-DB), Phase 2 `events`. RLS second tier. Each
  phase provable-green before the next.
- **L2 — isolation:** distinct PG-less lane OS user + `0700`
  `unix_socket_directories` owned by a daemon identity ≠ lane identity; doctor
  escalates `lane_pg_reachable` warning→startup block behind the default-false
  flag; scrub `PGHOST` from lane env. Structural close of **#87**.
- **L3 — attribution:** `pgxpool.BeforeAcquire`/`AfterRelease` set/reset
  `striatum.rpc_id` + `striatum.principal_id` (and `app.session_id`); the audit
  function stores them; `daemon_auth_log` for owner-fallback diagnosis. Lands
  first with the L0 doctor probe (no behavior change).

---

## 3. The v3 canonical form (normative — the adjudicator's most load-bearing row)

Same 15 fields, fixed **declared** order:
`ts, schema_version, hash_format_version, daemon_version, client_id, repository_id, method, decision, denial_reason, transport, request_id, exit_code, params_sha256, previous_hash, segment_id`.

`row_hash = lower_hex(sha256( concat over fields of Enc(field) ))` where:

- string `s` → `dec(octet_length(s)) || ":" || utf8_bytes(s)`
- integer `n` → encode `dec(n)` as a string (length-prefixed like a string)
- null → the literal bytes `-1:` (unreachable by `octet_length`, so distinct from
  empty string `0:`)
- `ts` → the RFC3339 second-truncated UTC string, length-prefixed (Q2)

Length-prefixing makes the encoding injective (no field content can forge a
delimiter); there is no JSON, no key-order dependence, and no HTML-escaping.
Go (`V3RowHash`, `strconv` + `len([]byte)`) and PL/pgSQL
(`octet_length(convert_to(v,'UTF8'))` + `||`) build the identical bytes;
**T-HASH-PARITY** pins them equal over a battery including `<`, `>`, `&`, empty
strings, and nulls.

---

## 4. Binding constraints (for extraction)

| id | Constraint | Verified by |
| --- | --- | --- |
| **C-HASH** | The audit `row_hash` is computed in-DB and is byte-identical to the Go `V3RowHash` recomputation; `VerifyRows` dispatches per `hash_format_version` so v2 and v3 both verify. A mismatch silently breaks every chain. | T-HASH-PARITY, T-VERIFY-MIXED, T-TS |
| **C-INSERT-REVOKE** | L1 Phase 0 `REVOKE INSERT ON audit_log FROM striatumd_rw`; inserts only via `append_audit_row`. (INSERT is live today via `0005:470`.) | T-42501 |
| **C-OWNER-DDL** | All L1 functions/REVOKEs and role changes are owner-applied (RFC 0079 §5); `striatumd_rw` cannot own them. | migration applied `--as-owner` |
| **C-UPGRADE-SAFE** | L2 enforcement default-false; default-on is a separate announced minor; PEER installs keep working on upgrade. | flag-gated doctor test |
| **C-NO-NATIVE** | No C extension, no wire proxy, no external identity/telemetry. | design review |
| **C-ATTR-RESET** | L3 `rpc_id`/`principal_id`/`app.session_id` reset across pooled checkouts; provenance never bleeds. | T-ATTR-RESET |
| **C-SECRET-HONEST** | L0 guarantees RAM-only + rotation-on-restart, **not** in-process zeroization (pgx retains the password for reconnect); docs/doctor must not over-claim secrecy, and never read/log the value. | doctor posture probe; spec wording review |

---

## 5. Pre-emptive rebuttals (the candidate's three most-attackable claims)

- **"Option A is necessary" (candidate §0.1).** Challenge: *isn't revoking direct
  DML enough — why also compute the hash in-DB?* Rebuttal: REVOKE alone routes
  writes through the function, but if the function trusts a caller-supplied hash
  the caller can still store a chain-breaking value. Computing the hash in-DB is
  what makes the stored `row_hash` correct-by-construction; without it, `EXECUTE`
  is a forge-the-integrity-check primitive.
- **"v3 byte-parity is actually reproducible in PL/pgSQL" (§3).** Challenge:
  *PL/pgSQL string handling will diverge somewhere.* Rebuttal: the v3 form uses
  only `octet_length`, integer→text, and byte concat — no locale, no escaping, no
  ordering. The one real risk (UTF-8 byte length vs character length) is pinned:
  `octet_length(convert_to(v,'UTF8'))` is bytes, matching Go `len([]byte)`.
  T-HASH-PARITY runs the SQL against a real PG and fails on any divergence.
- **"RAM-only" (§Q5).** Challenge: *you said the password is in memory; isn't that
  a leak surface?* Rebuttal: yes, and we say so — the guarantee is deliberately
  rotation-on-restart, not unrecoverable-from-memory. A core-dump/swap capture is
  dead after the next restart. Stronger in-process hygiene is a separate effort
  and likely out of scope for a local-first tool (C-SECRET-HONEST).

---

## 6. Sequencing & acceptance

**Sequence:** (1) L3 + L0 doctor probe → (2) L0 rotation + L1 Phase 0 → (3) L1
Phase 1/2 → (4) L1 RLS → (5) L2 behind the flag → (6) cross-host certs deferred.
After RFC 0104/0105; does not block RFC 0103.

**Acceptance gates:** T-42501 (`42501` on direct audit INSERT), T-HASH-PARITY +
T-VERIFY-MIXED + T-TS (hash parity & mixed-format verify), T-ATTR-RESET
(provenance reset), doctor block under the L2 flag, doctor credential-posture
without leaking the secret, owner-fallback `daemon_auth_log` read. On landing:
`docs/reference/spec.md` carries the auth model + write-boundary invariant;
`decision-log.md` records each per-phase decision.

---

## 7. Residue (flagged, not hidden)

- `V2RowHash` (Go) stays permanently as the verifier for pre-cutover rows;
  accepted cost.
- L0 cannot make the runtime password unrecoverable from live process memory
  (C-SECRET-HONEST).
- L1 RLS is defense-in-depth, explicitly last; the function-gate phases are the
  primary boundary.
