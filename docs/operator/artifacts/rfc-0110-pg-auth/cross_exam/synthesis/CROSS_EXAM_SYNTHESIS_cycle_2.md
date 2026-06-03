---
schema_version: striatum.findings_ledger.v1
artifact_kind: findings_ledger
summary_count: 27
author: convener-claude-opus-4.8-004
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 2
title: "RFC 0110 — Cross-examination findings ledger (cycle 2)"
---

# RFC 0110 — Cross-examination findings ledger (cycle 2)

This ledger rolls up **all five cross-examiner postures** against the cycle-2
convener synthesis into one findings record for the adjudicator. Each finding's
**posture, severity, and status** is preserved from its source artifact; the
unanswered interrogations are carried forward as evidence (§4). This is a
roll-up, **not** an adjudication: the convener does not render accept / reject
here. The downstream `adjudicate` job
(`../../adjudication/adjudicator/`, `collaboration_ledger`) decides whether the
gate clears.

**Target under examination:**
`docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md`
(with `../../convener_synthesis/draft/CANDIDATE_cycle_2.md`).

**Inputs rolled up:**

- `../product/CROSS_EXAM.md` — posture **product** (`cross_examiner_1`, codex-gpt-5.5-xhigh-003) — **cycle-2, targets SYNTHESIS_cycle_2**
- `../implementation/CROSS_EXAM.md` — posture **implementation** (`cross_examiner_2`, codex-gpt-5.5-xhigh-002) — **cycle-2, targets SYNTHESIS_cycle_2**
- `../operations/CROSS_EXAM.md` — posture **operations** (`cross_examiner_5`, claude-opus-4.8-002) — **cycle-2, targets SYNTHESIS_cycle_2**
- `../privacy/CROSS_EXAM.md` — posture **privacy** (`cross_examiner_3`, gemini-3.5-flash-high-004) — **degraded: targeted the closed cycle-1 session; no cycle-2 marker (see §4/§5)**
- `../eval/CROSS_EXAM.md` — posture **eval** (`cross_examiner_4`, gemini-3.5-flash-high-004) — **degraded: targeted the closed cycle-1 session; no cycle-2 marker (see §4/§5)**

---

## 1. Roll-up summary

- **Postures:** 5 (product, implementation, privacy, eval, operations).
- **Findings:** 27 total — **1 critical, 19 high, 7 medium**.
- **Two distinct cross-exam tiers this cycle:**
  - **Cycle-2 engaged (product, implementation, operations):** these three
    examined the **revised** `SYNTHESIS_cycle_2.md` and raise **genuinely new
    cycle-2 findings** grounded in the revision text and current run-branch
    source. The single **critical** (`IX2-001`) and the operations `OPS-9/10/11`
    are *created by the cycle-2 remedy itself* (the new `daemon_auth` authority
    gate) — not carried from cycle 1.
  - **Cycle-2 degraded (privacy, eval):** both agy/gemini postures targeted the
    **closed cycle-1 convener session** `sess_a6beb21cc70189786cf7c45e63619068`
    (got `panel_window_closed`) and **re-state the cycle-1 PR-/EV- findings
    verbatim**; they do **not** engage `SYNTHESIS_cycle_2.md`. Every PR-/EV- row
    they raise maps to a constraint the cycle-2 synthesis **already discharges**
    (§1 of `SYNTHESIS_cycle_2.md`). They are preserved here as evidence, but the
    adjudicator should **not** read them as fresh cycle-2 gaps. See §5.
- **Interrogation status — 0 of 5 reached the live cycle-2 convener
  (`sess_eab020240ffd8880cae29de0707d17b5`), via two failure modes:**
  - **product + implementation (codex):** `interrogation.open` returned
    `capability_denied` — *"interrogator session lacks the 'interrogate'
    capability"* (interrogator sessions `sess_05a3992d…`, `sess_daf075b0…`; audit
    `6737178`). The target was correct (the live cycle-2 convener) but the
    cross_examiner sessions were not granted the `interrogate` capability.
  - **privacy + eval (agy/gemini):** `interrogation.open` returned
    `interrogation_unavailable` / `panel_window_closed` against the **stale**
    target `sess_a6beb21…` (the closed cycle-1 convener; audits `6634722`,
    `6633220`).
  - **operations (claude):** issued as a **non-interrogable** review job;
    recorded its lead falsifying interrogation (the `OPS-9` freshness-window
    question) textually.
  - **Consequence:** as in cycle 1, **no candidate rebuttal was possible for any
    finding** — every row is **open / unrebutted** and carried to adjudication as
    such. The two interrogation failure modes are themselves recurring-harness
    evidence (§4).

### 1.1 Severity distribution by posture

| posture | critical | high | medium | total | tier |
| --- | --- | --- | --- | --- | --- |
| product (PX3) | 0 | 4 | 2 | 6 | cycle-2 engaged |
| implementation (IX2) | 1 | 4 | 0 | 5 | cycle-2 engaged |
| operations (OPS, new rows) | 0 | 3 | 1 | 4 | cycle-2 engaged |
| privacy (PR) | 0 | 4 | 2 | 6 | **degraded (cycle-1 re-statement)** |
| eval (EV) | 0 | 4 | 2 | 6 | **degraded (cycle-1 re-statement)** |
| **total** | **1** | **19** | **7** | **27** | |

> Operations also **credits the 8 cycle-1 operations constraints**
> (`C-RESTART-OWNER-DEP`, `C-L0-ADOPTION-VISIBLE`, `C-ROLLBACK-FORWARD-ONLY`,
> `C-DDL-DEPLOY-ORDER`, `C-ROTATION-SINGLE-WRITER`, `C-OWNER-DR`,
> `C-DOCTOR-OWNER-REACH`, `C-SOCKET-RELOCATE-MIGRATION`) as **adequately folded
> into cycle-2 spec text with falsifiable gates** and does not re-open them
> (positive discharge evidence, not a finding).

### 1.2 Index (scannable; full detail in §3)

| id | posture | sev | status | requested constraint shape |
| --- | --- | --- | --- | --- |
| `IX2-001` | implementation | **critical** | open / unrebutted (cycle-2) | `C-EXTENDED-AUTH-PRELUDE` — daemon_auth must not ride pgx simple-protocol query text |
| `PX3-001` | product | high | open / unrebutted (cycle-2) | `C-DSN-READ-SCOPE` — least-privilege read contract or narrow the "uninteresting DSN" claim to mutation |
| `PX3-002` | product | high | open / unrebutted (cycle-2) | `C-PHASED-WRITE-CLOSURE` — phase-scope the "sole durable write path" claim |
| `PX3-003` | product | high | open / unrebutted (cycle-2) | `C-AUDIT-FORMAT-CUTOVER` — v3 cutover is a product release gate (verifier dispatch ships together) |
| `PX3-004` | product | high | open / unrebutted (cycle-2) | `C-87-CLOSURE-GATE` — keep #87 "partial" until the PG-less lane user is enforced |
| `PX3-005` | product | medium | open / unrebutted (cycle-2) | `C-D164-AMEND` — amend D164/spec before the first behavior-changing PR |
| `PX3-006` | product | medium | open / unrebutted (cycle-2) | `C-MUTATION-INVENTORY` — classify every durable table's write-authority, not just 3 |
| `IX2-002` | implementation | high | open / unrebutted (cycle-2) | `C-AUTH-TX-WRAPPER` — authorized mutation-tx constructor makes the prelude unavoidable |
| `IX2-003` | implementation | high | open / unrebutted (cycle-2) | `C-AUDIT-AUTH-PRELUDE` — audit append gets the prelude + fail-closed on audit failure |
| `IX2-004` | implementation | high | open / unrebutted (cycle-2) | `C-OWNER-DDL-SPLIT` — owner-only L1 DDL split from runtime auto-migrations |
| `IX2-005` | implementation | high | open / unrebutted (cycle-2) | `C-PGTEST-NO-DML-GRANT` — pgtest privileges from migrations only |
| `OPS-9` | operations | high | open / unrebutted (cycle-2, new) | `C-AUTH-WINDOW-LIVENESS` — daemon_auth freshness window lifecycle; no silent self-wedge |
| `OPS-10` | operations | high | open / unrebutted (cycle-2, new) | `C-DEPLOY-CAPABILITY-PARITY` — startup precondition asserts authority-capability parity |
| `OPS-11` | operations | high | open / unrebutted (cycle-2, new) | `C-ROTATOR-PROBE-ROLE-SCOPED` — rotator probe keyed on role-collision, not instance-id |
| `OPS-12` | operations | medium | open / unrebutted (cycle-2, new) | `C-DISCARD-RECONNECT-BOUND` — bound discard-on-error; reconnect backoff; doctor signal |
| `PR-001` | privacy | high | **degraded — already discharged** (`Q-DYNAMIC-CREDENTIALS`, deferred §5.9) | `C-DYNAMIC-CREDENTIALS` (cycle-1 re-statement) |
| `PR-002` | privacy | high | **degraded — already discharged** (`C-AUTH-LOG-PRIVACY`, folded §6.1) | `C-AUTH-LOG-PRIVACY` (cycle-1 re-statement) |
| `PR-003` | privacy | medium | **degraded — already discharged** (`C-GUC-PARAMETERIZED`, folded §4.2 — but see `IX2-001`) | `C-GUC-PARAMETERIZED` (cycle-1 re-statement) |
| `PR-004` | privacy | medium | **degraded — already discharged** (`C-SOCKET-DIR-PERMS`, folded §7.4) | `C-SOCKET-DIR-PERMS` (cycle-1 re-statement) |
| `PR-005` | privacy | high | **degraded — already discharged** (`C-EVENT-NO-TRANSCRIPTS`, folded §6.2) | `C-EVENT-NO-TRANSCRIPTS` (cycle-1 re-statement) |
| `PR-006` | privacy | high | **degraded — already discharged** (`C-HASH-V3`, folded §5.1) | `C-HASH-V3-TRANSITION` (cycle-1 re-statement) |
| `EV-001` | eval | high | **degraded — already discharged** (`C-HARNESS-PRIVILEGES`, folded §7.1; cf. `IX2-005`) | `C-HARNESS-PRIVILEGES` (cycle-1 re-statement) |
| `EV-002` | eval | high | **degraded — already discharged** (`C-L2-NEG-TEST`, folded §7.2) | `C-L2-NEG-TEST` (cycle-1 re-statement) |
| `EV-003` | eval | medium | **degraded — already discharged** (`C-TEST-ROW-WRITER`, folded §5.1) | `C-TEST-ROW-WRITER` (cycle-1 re-statement) |
| `EV-004` | eval | high | **degraded — already discharged** (`C-ATTR-RESET-FAIL`, folded §4.3; cf. `OPS-12`) | `C-ATTR-RESET-FAIL` (cycle-1 re-statement) |
| `EV-005` | eval | medium | **degraded — already discharged** (`C-TZ-INDEPENDENT-HASH`, folded §5.1) | `C-TZ-INDEPENDENT-HASH` (cycle-1 re-statement) |
| `EV-006` | eval | high | **degraded — already discharged** (`C-HASH` parity, folded §5.1) | `C-HASH-PARITY-TEST` (cycle-1 re-statement) |

---

## 2. Reading guide for the adjudicator

The cycle-2 surface that actually warrants extraction is **§3.1 (product) +
§3.2 (implementation) + §3.3 (operations)** — the three engaged postures. The
**critical** row `IX2-001` and the operations `OPS-9/10/11` are the load-bearing
new hazards, all clustered on the **new `daemon_auth` authority gate** the cycle-2
revision introduced to close `C-EXEC-AUTH`. The recurring theme: *the cycle-2 fix
for the critical finding is conceptually sound but its **mechanism and operational
contract are incomplete** against the real Go seams and day-2 operations surface.*

The privacy/eval rows (§3.4) are **degraded re-statements of cycle-1** and are
listed only for completeness and as harness evidence (§4/§5). The one substantive
cross-link: `PR-003` (parameterized GUCs) and the **critical `IX2-001`** point at
the *same* mechanism from opposite directions — `SYNTHESIS_cycle_2 §4.2` claims
parameterization keeps the secret out of `pg_stat_activity`, while `IX2-001` shows
the daemon's **forced simple protocol** (`connection.go`
`QueryExecModeSimpleProtocol`) interpolates parameters **client-side**, so the
claim may not hold. That collision is the single most important thing for the
adjudicator to weigh.

---

## 3. Findings detail (posture / severity / status preserved)

### 3.1 Product (cycle-2 engaged; `cross_examiner_1`, codex-gpt-5.5-xhigh-003)

Posture summary (source): cycle 2 closes the original product-critical write
spoofing hole *in concept* (raw `striatumd_rw` execution without daemon authority
is rejected; attribution GUCs demoted below the trust boundary). Remaining product
risk is **claim accounting**.

| id | sev | affected invariant | finding (rolled up) | requested constraint |
| --- | --- | --- | --- | --- |
| `PX3-001` | high | D164 "leaked runtime credential uninteresting"; local-first privacy boundary | Cycle 2 narrows the *write* claim (G1/G2) but keeps "a leaked DSN string is uninteresting"; L1 revokes DML but does not say `striatumd_rw` loses broad `SELECT` on artifacts/events/sessions/queue/principals/payload JSON. A non-mutating leaked DSN can still **read** authoritative/private state. | `C-DSN-READ-SCOPE` — name allowed/denied read surfaces + a PG-gated negative read test, **or** narrow the claim to "uninteresting for unauthorized mutation & hash/attempt-invariant violation." |
| `PX3-002` | high | "daemon RPC/artifact API is the sole durable write path"; phased rollout | The "sole durable write path" claim can be closed too early: Phase 0 gates only `audit_log`; `events`/`artifacts` remain directly writable until their phases. Release/doctor/RFC text must not assert the full write boundary before all three surfaces are covered. | `C-PHASED-WRITE-CLOSURE` — per-phase protected-table set + direct-DML negatives + doctor posture string; the final claim gated on all of `audit_log`+`artifacts`+`events`. |
| `PX3-003` | high | audit provenance operator-verifiable across the PL/pgSQL hash cutover | The v3 cutover must be a **product release gate**, not an eval detail: no v3 row producible until the binary ships `VerifyRows` v2/v3 dispatch, unknown-format failure, mixed-chain verify, and a runbook/doctor skew-vs-tamper distinction — else a SQL append can create rows the shipped verifier cannot validate. | `C-AUDIT-FORMAT-CUTOVER` — Phase 0 "audit DB-gated & valid" gated on append fn + SQL v3 hash + Go `V3RowHash` + verifier dispatch + default-v2 flag + mixed-format tests shipping together. |
| `PX3-004` | high | #87 closure; L2 "hardened default" without stranding upgrades | The operator brief says #87 is only partial and the PG-less lane OS user is **not built**; a default-false legacy flag + warning does not close #87 for the deployed posture. Calling L2 a "hardened default" / issue closure before the lane actually runs as a PG-less OS user with a blocking doctor path would mislead operators. | `C-87-CLOSURE-GATE` — explicit #87 closure criteria (dedicated lane OS user, `T-LANE-ISOLATION-NEG` green, doctor blocks under hardened posture); status says "partial" until those are green in the default/named secure profile. |
| `PX3-005` | medium | accepted D164 remains the operator-facing contract | Cycle 2 materially amends D164 (L3 → in-tx prelude; GUCs = labels; new `daemon_auth` authority; v3 supersedes JSON-parity wording; phase-scoped claims). The decision/spec text should not point at the old contract while implementers start coding. | `C-D164-AMEND` — before the first behavior-changing PR, `decision-log.md`/`spec.md` no longer describe `BeforeAcquire SET LOCAL` as the answer and no longer over-claim leaked-credential/hardened-default scope. |
| `PX3-006` | medium | future mutations can't bypass L1 by being outside the 3 named tables | The product boundary is broader than `audit_log`/`artifacts`/`events`; same-host direct PG access can bypass daemon capability/RPC policy on any durable table. A new durable write path outside the phased list preserves the class of bug this RFC retires. | `C-MUTATION-INVENTORY` — authority table classifies every daemon-owned table (direct-DML-allowed / read-only-to-runtime / SD-gated); a guard fails new migrations granting broad runtime DML on durable workflow tables without a recorded exception. |

### 3.2 Implementation (cycle-2 engaged; `cross_examiner_2`, codex-gpt-5.5-xhigh-002)

Posture summary (source): cycle 2 answers round-1 in the abstract; the
implementation-ready spec needs one more hardening pass at the **actual Go seams**.
Highest risk = `daemon_auth` over pgx simple protocol.

| id | sev | affected invariant | finding (rolled up) | requested constraint |
| --- | --- | --- | --- | --- |
| `IX2-001` | **critical** | `C-EXEC-AUTH` + `C-GUC-PARAMETERIZED`: daemon_auth non-spoofable & not visible to raw `striatumd_rw` SQL | `connection.go` sets `DefaultQueryExecMode = QueryExecModeSimpleProtocol`; pgx simple protocol uses **client-side parameter interpolation**, so the daemon-auth secret can land in **query text** observable via `pg_stat_activity`/statement logs to a same-role `striatumd_rw` session — collapsing the gate back to "possess DSN + watch pg_stat_activity." Directly undercuts `SYNTHESIS_cycle_2 §4.2`. | `C-EXTENDED-AUTH-PRELUDE` — run the authority prelude over **extended protocol** (or a proven non-text path); a PG-gated same-role observer regression proves the secret never appears; a unit guard fails if the prelude runs under simple protocol. |
| `IX2-002` | high | `C-TX-GUC-PRELUDE`: every mutating tx starts with attribution+authority before first write | Current stack exposes only generic `withTx`/`BeginTx`; nothing proves the first statement inside `fn` is the prelude. With dozens of `withTx` callers, "remember to call applyAttribution first" is not an implementation-ready invariant. | `C-AUTH-TX-WRAPPER` — a dedicated `withAuthorizedMutationTx(...)` constructor applies the prelude and passes an `AuthorizedTx`; a guard fails any mutating handler using generic `withTx`/`BeginTx`; a tx-order test proves `set_config('striatum.daemon_auth',…)` is statement 1. |
| `IX2-003` | high | audit append is mandatory provenance & participates in L1 authority | `RecordRPCTransport` opens its **own** tx (`audit.go`); if `append_audit_row` becomes SD-authorized it also needs the prelude. Worse, `server.go` ignores `auditErr` (only omits `AuditID`), so a failed authority prelude could let a mutation succeed while its audit row silently fails. | `C-AUDIT-AUTH-PRELUDE` — thread the secret+attribution into the audit tx; decide fail-closed semantics; a regression forces the audit fn to reject and proves the RPC fails loudly / rolls back, never success-without-audit-row. |
| `IX2-004` | high | owner-applied DDL & runtime auto-migrations don't crash-loop or reorder L1 revokes/functions | Both `migrate-db` and daemon startup call `db.ConnectAndMigrate` over the runtime DSN; owner-only L1 DDL can't be dropped into the embedded sequence (runtime role may try to apply it; premature revokes break an old binary's inserts). | `C-OWNER-DDL-SPLIT` — owner-only L1 migrations get a distinct delivery path/marker; runtime `ConnectAndMigrate` never attempts owner-only function/revoke DDL; skew tests cover new-binary/old-schema and old-binary/premature-revoke before first mutation. |
| `IX2-005` | high | `T-42501`/grant-drift exercise production privileges, not a polluted harness | `pgtest.Pools` issues broad `GRANT … ON ALL TABLES` then hand-revokes a subset; security tests can pass against a privilege layout production migrations never create (or grant-repair reopens). | `C-PGTEST-NO-DML-GRANT` — role/privilege setup moves to migration-owned SQL; a guard fails any pgtest path granting/revoking protected-table DML; `T-42501` runs against migration-defined roles after migrations/owner-DDL/grant-repair. |

### 3.3 Operations (cycle-2 engaged; `cross_examiner_5`, claude-opus-4.8-002)

Posture summary (source): the 8 cycle-1 operations constraints are **genuinely
folded** into cycle-2 spec text with falsifiable gates and are **adequately
discharged from this posture** (credit recorded, not re-opened). But the cycle-2
remedy for the *critical* finding introduces **new operations surface** confined to
the `daemon_auth` gate and the §5.4 multi-host clause. Posture recommendation:
**needs_revision, narrowly**, with the four rows below extracted. *(All four are new
in cycle 2.)*

| id | sev | affected invariant | finding (rolled up) | requested constraint |
| --- | --- | --- | --- | --- |
| `OPS-9` | high | write availability (`assert_daemon_authority` now gates `append_audit_row` on every mutation) | The authority "freshness window" (CANDIDATE §2.2) is **undefined** with no refresh contract; the secret/registry row are single-shot at bootstrap. Finite-window-no-refresh → a long-lived daemon's **correct** secret ages out → every write `RAISE`s `28000` with no operator signal (silent total-write wedge); infinite-window → undercuts the §5.4 concurrent-rotator story. Neither `T-OWNER-FAILCLOSED` (bootstrap) nor `T-EXEC-AUTH` (absent/wrong secret) covers a valid secret aging mid-run. | `C-AUTH-WINDOW-LIVENESS` — define the lifecycle (lifetime-of-instance validity **or** a refresh contract); any lapse is fail-closed + owner-attributable + doctor-visible, never a silent `28000`. **T-AUTH-LIVENESS**: aged/refresh-failed instance still writes OR fail-closed+doctor. |
| `OPS-10` | high | deploy safety / write availability (`C-DDL-DEPLOY-ORDER`) | §4.4's precondition checks **presence** (`to_regprocedure(append_audit_row)`, pgcrypto), not **authority-capability parity**. Owner DDL vN (functions call `assert_daemon_authority`) + an old vN-1 binary (never sets `striatum.daemon_auth`) **passes presence**, then `28000`-wedges every write at runtime — re-opening the exact wedge `C-DDL-DEPLOY-ORDER` promised to fail-fast. Precondition also omits the registry table + `assert_daemon_authority` itself. | `C-DEPLOY-CAPABILITY-PARITY` (sharpens `C-DDL-DEPLOY-ORDER`) — owner DDL stamps a schema-contract/`requires_daemon_auth` marker; the binary refuses to serve unless its authority capability ≥ schema requirement, over the **full** dependency set; skew fails closed pre-mutation, not as a runtime `28000`. |
| `OPS-11` | high | RFC-0107 multi-principal / remote-PG goal; operator trust in doctor | §5.4's two clauses collide: the concurrent-rotator probe alarms on "a recent `rotated_at` from a **different `instance_id`**," but §5.4's own multi-host resolution gives each instance its **own** per-instance role + registry row — so **every legitimate multi-host peer trips the probe**. The probe keys on instance-id difference (the normal multi-host state), not role collision. | `C-ROTATOR-PROBE-ROLE-SCOPED` (sharpens `C-ROTATION-SINGLE-WRITER`) — record the rotated **role**; alarm only when two **distinct live instance_ids rotated the same role** within a window; per-instance-role peers don't trip it; add multi-host no-false-alarm + shared-role positive tests. |
| `OPS-12` | medium | pool/write availability under transient PG stress × rotation | §3.3 **destroys** (not just `DISCARD ALL`-resets) any connection whose tx errored/cancelled. With default `statement_timeout=60000` + context-cancel, a transient blip destroys a burst of connections → reconnect herd; if it overlaps a rotation (pre-§5.4 shared role), the mass reconnect hits with the **old** password → `28P01` pool collapse with no self-heal (OPS-5 mechanism, amplified by the discard policy). | `C-DISCARD-RECONNECT-BOUND` (sharpens `C-ATTR-RESET-FAIL` × `C-ROTATION-SINGLE-WRITER`) — destroy only on attribution-poisoning errors; transient errors reset without destroy; reconnect backoff; doctor signal on reconnect-auth-failure spikes; document the discard↔rotation interaction. |

### 3.4 Privacy & Eval (degraded — cycle-1 re-statements; agy/gemini-3.5-flash-high-004)

**Status caveat (applies to all 12 rows below).** Both postures issued
`interrogation.open` against the **closed cycle-1 convener** `sess_a6beb21…`
(`panel_window_closed`), carry **no `cycle: 2` marker**, and do **not** reference
`SYNTHESIS_cycle_2.md`. Each finding is a **verbatim re-statement of the
cycle-1 PR-/EV- challenge**, and each maps to a constraint the cycle-2 synthesis
**already discharges** (mapping in §1.2 / `SYNTHESIS_cycle_2.md §1`). Preserved as
evidence and as harness signal (§4/§5); **not** fresh cycle-2 gaps.

**Privacy (`cross_examiner_3`):** `PR-001` heap-resident rotated password →
`C-DYNAMIC-CREDENTIALS` *(cycle-2: deferred-with-successor `Q-DYNAMIC-CREDENTIALS`
→ #88-dynamic-creds, §5.9)*; `PR-002` unconstrained `daemon_auth_log.detail` →
`C-AUTH-LOG-PRIVACY` *(folded §6.1)*; `PR-003` string-concat GUCs leak in
`pg_stat_activity` → `C-GUC-PARAMETERIZED` *(folded §4.2 — but see the live
`IX2-001` collision on simple protocol)*; `PR-004` socket-dir perms not asserted →
`C-SOCKET-DIR-PERMS` *(folded §7.4)*; `PR-005` `events.payload_json` transcript
bleed → `C-EVENT-NO-TRANSCRIPTS` *(folded §6.2)*; `PR-006` Go-JSON→PL/pgSQL hash
parity → `C-HASH-V3` *(folded §5.1)*.

**Eval (`cross_examiner_4`):** `EV-001` `pgtest` ad-hoc GRANT/SET ROLE pollutes
harness → `C-HARNESS-PRIVILEGES` *(folded §7.1; cf. the live `IX2-005`)*; `EV-002`
no negative L2 isolation test → `C-L2-NEG-TEST` *(folded §7.2)*; `EV-003` no raw-row
writer for mixed/tamper chains → `C-TEST-ROW-WRITER` *(folded §5.1)*; `EV-004`
attribution reset under abort/cancel/panic → `C-ATTR-RESET-FAIL` *(folded §4.3; cf.
the live `OPS-12`)*; `EV-005` `to_char` timezone dependence → `C-TZ-INDEPENDENT-HASH`
*(folded §5.1)*; `EV-006` Go↔PL/pgSQL hash byte-parity test → `C-HASH-PARITY`
*(folded §5.1)*.

> Note: although degraded, two of these re-statements **corroborate** live cycle-2
> findings — `PR-003` ↔ `IX2-001` (GUC/secret visibility), and `EV-004` ↔ `OPS-12`
> (reset/discard robustness). The corroboration strengthens those two engaged rows;
> it does not add new constraints.

---

## 4. Unanswered interrogations carried forward as evidence

Per the cross_exam_synthesis charge ("carry unanswered interrogations forward as
evidence for the adjudicator"). **No cross-examiner obtained a live rebuttal from
the cycle-2 convener.** The five recorded attempts:

| posture | interrogator session | target | open() result | the unasked falsifying question (recorded textually) |
| --- | --- | --- | --- | --- |
| product | `sess_05a3992de33c494d7a3e56698aa2b6f6` | `sess_eab020…` (live cycle-2, **correct**) | `capability_denied` — "lacks the 'interrogate' capability" (audit `6737178`) | Is the "uninteresting DSN" claim limited to mutation/invariant violation, or does it cover **read** access and future durable writes — which exact read surfaces and table phases are "closed" per phase? |
| implementation | `sess_daf075b0aaf5816ce5972114906f6b95` | `sess_eab020…` (live cycle-2, **correct**) | `capability_denied` — "lacks the 'interrogate' capability" | Given the daemon forces `QueryExecModeSimpleProtocol` (client-side interpolation), what exact connection/query path keeps the `daemon_auth` secret out of `pg_stat_activity`/logs for a raw `striatumd_rw` session? *(→ live `IX2-001`)* |
| operations | (non-interrogable review job) | n/a | recorded textually | `assert_daemon_authority` accepts within "the registry's freshness window" — on a daemon up for days, what is the window, what refreshes the row, and what does the operator see the moment it lapses (given `append_audit_row` is on every mutation)? *(→ live `OPS-9`)* |
| privacy | (gemini) | `sess_a6beb21…` (**closed cycle-1**, stale target) | `interrogation_unavailable` / `panel_window_closed` (audit `6634722`) | How is `daemon_auth_log.detail` JSONB guaranteed not to bleed DSNs/driver configs/failed-DSN credentials? *(cycle-1 PR-002 restated)* |
| eval | (gemini) | `sess_a6beb21…` (**closed cycle-1**, stale target) | `interrogation_unavailable` / `panel_window_closed` (audit `6633220`) | How is `T-42501` not validating a polluted `pgtest` harness rather than migrations-enforced production privileges? *(cycle-1 EV-001 restated)* |

**Two harness signals for the adjudicator (and operator):**

1. **Interrogate-capability gap (product, implementation):** the codex
   cross_examiner sessions targeted the **correct** live cycle-2 convener but were
   refused `interrogation.open` with *"interrogator session lacks the 'interrogate'
   capability."* The cross-examination channel is therefore non-functional for
   these lanes irrespective of window state — a recurring reviewer-lane capability
   gap (cf. the panel's known "reviewer lanes lack interrogate" friction).
2. **Stale-target gap (privacy, eval):** the agy/gemini cross_examiner prompts
   targeted the **closed cycle-1** convener session pointer rather than the live
   cycle-2 convener, so they could not have engaged the revision even with a window
   open — and consequently re-ran cycle-1 analysis. This is the mechanism behind the
   degraded privacy/eval tier.

Both mean every finding remains **open / unrebutted**, exactly as in cycle 1.

---

## 5. Convener roll-up notes (context only — not an adjudication)

- **Genuinely new cycle-2 surface to extract (engaged postures):** `IX2-001`
  (critical), `IX2-002…005`, `PX3-001…006`, `OPS-9…12`. These are grounded in
  `SYNTHESIS_cycle_2.md` and current run-branch source and were not addressable by
  discharging the cycle-1 ledger. They cluster on the **new `daemon_auth` authority
  gate** (the cycle-2 remedy) and on **claim-accounting / Go-seam / day-2-ops
  completeness** — the design direction is corroborated as sound; the mechanism and
  contract are incomplete.
- **Already-discharged (degraded postures):** all 12 PR-/EV- rows map to
  constraints `SYNTHESIS_cycle_2.md §1` already discharges; they should not be
  re-extracted as fresh cycle-2 constraints. Where they **corroborate** a live row
  (`PR-003`↔`IX2-001`, `EV-004`↔`OPS-12`) they strengthen it.
- **Positive discharge evidence:** the operations posture independently credits the
  8 cycle-1 operations constraints as adequately folded — useful for the adjudicator
  closing those rows.
- **The convener renders no verdict here.** This is a roll-up; `adjudicate` decides
  whether the gate clears and which constraint shapes become binding.
