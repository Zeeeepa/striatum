# RFC 0110 Adjudication Intake (cycle 1)
author: adjudicator-codex-gpt-5.5-xhigh-001
artifact_kind: handoff
logical_name: adjudication_intake_cycle_1
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 1
target: docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_1.md
inputs:
  - docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md
  - docs/decisions/decision-log.md#D164
  - docs/operator/artifacts/rfc-0110-pg-auth/survey/scan/SURVEY.md
  - docs/operator/artifacts/rfc-0110-pg-auth/survey/synthesis/SURVEY_SYNTHESIS.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/product/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/implementation/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/privacy/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/eval/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/operations/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/synthesis/CROSS_EXAM_SYNTHESIS_cycle_1.md

## Purpose

This intake stages the curated candidate synthesis and cross-examination
findings for the adjudication step. It does not add new challenges and does not
render the final adjudication verdict. It preserves the source trajectory,
deduplicates repeated constraint shapes, and names the points the adjudicator
must decide or carry forward.

## Decision Baseline

D164 accepts RFC 0110 as a design RFC, not shipped implementation. The accepted
direction is layered under the rule "make a leaked runtime credential
uninteresting":

- **L0 credential:** owner-bootstrapped, ephemeral, RAM-only
  `striatumd_rw` password, re-rotated each daemon restart; remote PostgreSQL
  uses `STRIATUM_OWNER_DB_URL` or a systemd encrypted credential; single-role
  owner==runtime posture skips rotation.
- **L1 enforcement:** PostgreSQL enforces the write contract by revoking direct
  DML and routing writes through owner-owned `SECURITY DEFINER` functions, then
  adding RLS as a second tier; phases are `audit_log`, then `artifacts`, then
  `events`.
- **L2 isolation:** a dedicated PG-less lane OS user plus a `0700` socket
  directory is the target hardened posture for closing #87; upgrade enforcement
  is flag-gated and not yet default-on.
- **L3 attribution:** authoritative mutations carry `rpc_id` and
  `principal_id`; the candidate initially places this in pgx pool hooks with
  `SET LOCAL`, while cross-exam challenges the transaction placement.

The current operator brief records #87 as only partially closed in released
source: `daemon doctor` has a `lane_pg_reachable` warning and adoption runbook,
but the OS-user PostgreSQL denial remains an operator adoption step.

## Candidate Record

The convener candidate resolves Q1-Q9 and proposes an implementation-ready
baseline:

- **Hash decision:** use Option A, but not by porting Go `encoding/json` to
  PL/pgSQL. Instead introduce `hash_format_version = 3` with a length-prefixed
  canonical encoding over the same 15 audit fields. `VerifyRows` dispatches per
  row so v2 rows continue to verify with `V2RowHash` and v3 rows verify with the
  new format.
- **v3 format:** fixed declared field order; strings encoded as
  byte length plus UTF-8 bytes; integers encoded as length-prefixed decimal
  strings; null encoded as `-1:`; timestamps formatted as RFC3339
  second-truncated UTC strings. Tests: `T-HASH-PARITY`, `T-VERIFY-MIXED`, and
  `T-TS`.
- **L1 Phase 0:** `append_audit_row(...)` owns the chain-head lock, segment
  handling, v3 hash computation, row insert, and chain-head update; direct
  `INSERT` to `audit_log` by `striatumd_rw` is revoked and pinned by `T-42501`.
- **L1 later phases:** `artifacts` and `events` receive the same treatment;
  RLS on `leases` and `sessions` is second-tier and later.
- **L0:** rotate the runtime role password from an owner connection at startup;
  keep the guarantee honest as RAM-only plus rotation-on-restart, not
  complete in-process zeroization.
- **L2:** ship the hardening posture behind `security.pg_socket_hardened`, with
  default-on deferred to a later announced minor.
- **L3:** add per-RPC attribution labels and reset them across pooled checkouts;
  add `daemon_auth_log` for auth posture and owner-fallback diagnosis.

Candidate constraint rows already present:

| id | Candidate constraint |
| --- | --- |
| `C-HASH` | v3 audit hash is byte-identical between Go and SQL; v2 and v3 both verify. |
| `C-INSERT-REVOKE` | direct `INSERT` to `audit_log` is revoked from `striatumd_rw`; appends go through `append_audit_row`. |
| `C-OWNER-DDL` | L1 functions, REVOKEs, and role changes are owner-applied. |
| `C-UPGRADE-SAFE` | L2 enforcement is default-false for upgrades; default-on is a separate release decision. |
| `C-NO-NATIVE` | no PostgreSQL C extension, wire proxy, hosted identity, or telemetry. |
| `C-ATTR-RESET` | attribution labels reset across pooled checkouts. |
| `C-SECRET-HONEST` | L0 promises RAM-only plus rotation-on-restart, not unrecoverable live-process memory secrecy. |

## Cross-Exam Status

The cross-exam synthesis rolls up five postures: product, implementation,
privacy, eval, and operations.

- Findings: **29 total**: 1 critical, 16 high, 12 medium.
- Interrogation status: the four interrogable postures attempted to question
  the candidate owner session and received `interrogation_unavailable` with
  `reason: panel_window_closed`; the operations posture was non-interrogable and
  recorded its lead falsifying question textually.
- Candidate rebuttals: none. Every finding is open and unrebutted, except
  OPS-2 is effectively conceded by the candidate's own single-role guard.
- Important correction to the candidate's synthesis: its statement that
  `interrogation.list` was zero reflects the closed panel window, not the
  absence of challenges.

The cross-exam roll-up recommends downstream **needs_revision**. This intake
records that recommendation as source input rather than rendering a new verdict.

## Load-Bearing Seams

### 1. Direct function execution by a leaked runtime DSN

Source rows: PX-001 (critical), PX-002 (high).

The candidate revokes malformed direct table DML but still grants `EXECUTE` on
owner-owned write functions to `striatumd_rw`. A process with only the runtime
DSN could call `append_audit_row` and later artifact/event functions outside
daemon RPC. In-DB hashing makes those writes hash-correct; it does not make them
daemon-authorized.

Adjudication input: either the spec narrows the product claim to "a leaked
runtime credential cannot violate append/hash/attempt invariants but can still
perform valid function writes", or it extracts `C-EXEC-AUTH` and requires a
non-spoofable daemon-authority gate for every write function.

### 2. GUCs are labels unless anchored to authority

Source rows: PX-002, IX-001, PR-003, EV-004.

The candidate treats `striatum.rpc_id`, `striatum.principal_id`, and
`app.session_id` as L3/RLS inputs. Cross-exam attacks the mechanism from four
angles:

- raw SQL callers can set custom GUCs themselves, so they are not authority;
- `pgxpool.BeforeAcquire` occurs before a mutation transaction, so it cannot
  provide a transaction-local `SET LOCAL` invariant;
- string-concatenated `SET LOCAL` leaks values in `pg_stat_activity`;
- reset behavior must survive aborts, cancellations, and panics.

Adjudication input: extract `C-GUC-NONAUTH`, `C-TX-GUC-PRELUDE`,
`C-GUC-PARAMETERIZED`, and `C-ATTR-RESET-FAIL`; treat GUCs as attribution
labels unless a separate non-spoofable server-side authority record is verified
by the write functions.

### 3. SECURITY DEFINER hardening

Source rows: PX-005 and IX-003.

Two independent postures request the same constraint. Owner-owned
`SECURITY DEFINER` functions must not introduce a privilege-confusion surface.

Adjudication input: extract `C-SD-HARDEN` once: locked-down `search_path` or
fully qualified references, no caller-controlled object names/operators, no
ambient public execute, grants only to intended roles, and migration tests over
`pg_proc`/ACLs plus hostile search-path coverage.

### 4. v3 hash contract and rollback

Source rows: PX-004, IX-004, IX-005, IX-006, EV-003, EV-005, OPS-3.

The v3 length-prefixed design is accepted in principle by cross-exam, but it
needs to become explicit contract text and deployment policy:

- v3 supersedes the survey's "match Go `encoding/json`" wording for new rows;
- SQL hashing should be bytea-based, with fixed UTF-8 conversion and golden
  vectors including multibyte/control characters and HTML-escaped v2 hazards;
- the hash primitive must be pinned as stock PostgreSQL capability;
- `VerifyRows` needs explicit v2/v3/unknown-format/mixed-boundary behavior;
- a privileged test-only raw-row writer is needed to seed malformed chains;
- timestamp parity must be tested under multiple DB `TimeZone` settings;
- Phase 0 is a rollback boundary unless the old verifier is backported or v3
  writes are operator-committed behind a flag.

Adjudication input: candidate `C-HASH` should be sharpened by `C-HASH-V3`,
`C-HASH-BYTEA`, `C-SHA-PRIMITIVE`, `C-VERIFY-DISPATCH`,
`C-TEST-ROW-WRITER`, `C-TZ-INDEPENDENT-HASH`, and
`C-ROLLBACK-FORWARD-ONLY`.

### 5. Privilege and harness drift

Source rows: IX-002 and EV-001.

The current migration/test posture grants broad DML to the runtime role and
`pgtest` can imperatively patch privileges in Go. A `42501` test is only useful
if it verifies migration-enforced production privileges rather than a
harness-specific state.

Adjudication input: extract `C-GRANT-DRIFT` and `C-HARNESS-PRIVILEGES`. Grant
repair, migration helpers, and pgtest setup must preserve direct-INSERT revokes
for each protected table.

### 6. L0 operational and privacy constraints

Source rows: PR-001, PR-002, OPS-1, OPS-2, OPS-5, OPS-6, OPS-7.

The candidate's L0 scope is honest about pgx retaining credentials, but privacy
and operations push harder:

- `C-DYNAMIC-CREDENTIALS` asks for dynamic password providers to shorten heap
  residency; this conflicts with, or at least exceeds, the candidate's
  `C-SECRET-HONEST` guarantee and must be reconciled.
- `C-AUTH-LOG-PRIVACY` requires strict whitelisting and redaction for
  `daemon_auth_log.detail`.
- `C-RESTART-OWNER-DEP` requires fail-closed, owner-attributable startup
  diagnostics and explicit documentation of the new owner dependency.
- `C-L0-ADOPTION-VISIBLE` requires the single-role rotation skip to appear in
  `daemon doctor`, and documents owner/runtime role split as an adoption
  prerequisite.
- `C-ROTATION-SINGLE-WRITER` requires either a single-daemon-per-runtime-role
  invariant or per-instance roles for remote/multi-host paths.
- `C-OWNER-DR` and `C-DOCTOR-OWNER-REACH` require break-glass owner credential
  recovery and a doctor path that works without superuser-only catalog reads.

### 7. L2 defaults, tests, and socket migration

Source rows: PX-003, EV-002, PR-004, OPS-8.

The candidate protects upgrades with a default-false flag, but cross-exam asks
the spec to state the target posture for new secure adoptions and prove the
boundary:

- `C-L2-DEFAULT`: secure-profile/fresh adoption either enables the hardened
  posture or emits a blocking doctor finding; legacy upgrades warn until the
  planned default-on release.
- `C-L2-NEG-TEST`: a lane-like unprivileged process must fail to connect over
  socket and loopback under the hardened posture.
- `C-SOCKET-DIR-PERMS`: startup/doctor checks verify the socket directory is
  exactly `0700`.
- `C-SOCKET-RELOCATE-MIGRATION`: the L2 how-to must enumerate psql,
  monitoring, owner DSN, `--as-owner`, doctor defaults, and pidfile blast radius
  when the socket path moves.

### 8. Durable metadata privacy

Source row: PR-005.

`events.payload_json` remains a broad durable JSON surface. Cross-exam asks for
`C-EVENT-NO-TRANSCRIPTS`: DB-level validation or constraints that prevent raw
provider output, stdout/stderr, or transcript-like keys from entering durable
event payloads. This is additive to the L1 event phase and D028 boundary.

## Constraint Catalog For Adjudication

The adjudicator should treat the following as the staged constraint set to
accept, reject, narrow, defer with owner, or convert into productive revision
questions.

| constraint | source | severity | disposition input |
| --- | --- | --- | --- |
| `C-EXEC-AUTH` | PX-001 | critical | Required if the headline claim remains "RPC/artifact API is the sole write path"; otherwise the claim must be narrowed. |
| `C-GUC-NONAUTH` | PX-002 | high | Required unless write functions verify a non-spoofable server-side authority record. |
| `C-TX-GUC-PRELUDE` | IX-001 | high | Corrects candidate L3 placement; `BeforeAcquire` is insufficient for `SET LOCAL`. |
| `C-GRANT-DRIFT` | IX-002 | high | Keeps direct-INSERT revokes from being undone by migration helpers, pgtest, or doctor repair. |
| `C-SD-HARDEN` | PX-005, IX-003 | high | Deduplicated; required for all owner-owned write functions. |
| `C-HARNESS-PRIVILEGES` | EV-001 | high | Required for `T-42501` and related security tests to prove production SQL privileges. |
| `C-L2-NEG-TEST` | EV-002 | high | Required to prove lane isolation, not only doctor posture. |
| `C-ATTR-RESET-FAIL` | EV-004 | high | Strengthens candidate `C-ATTR-RESET`. |
| `C-DYNAMIC-CREDENTIALS` | PR-001 | high | Tension with `C-SECRET-HONEST`; adjudicator must reconcile or defer explicitly. |
| `C-AUTH-LOG-PRIVACY` | PR-002 | high | Required for `daemon_auth_log.detail`. |
| `C-EVENT-NO-TRANSCRIPTS` | PR-005 | high | Required if L1 event phase expands durable DB validation. |
| `C-RESTART-OWNER-DEP` | OPS-1 | high | Required because the daemon is a hard prerequisite for every verb. |
| `C-L0-ADOPTION-VISIBLE` | OPS-2 | high | Required because the current single-role posture makes L0 inert. |
| `C-ROLLBACK-FORWARD-ONLY` | OPS-3 | high | Required before v3 rows are written. |
| `C-DDL-DEPLOY-ORDER` | OPS-4 | high | Required before owner-applied L1 DDL and runtime binary skew can occur. |
| `C-ROTATION-SINGLE-WRITER` | OPS-5 | high | Required for remote-PG and multi-daemon clarity. |
| `C-L2-DEFAULT` | PX-003 | high | Required to separate upgrade compatibility from target secure posture. |
| `C-HASH-V3` | PX-004 | medium | Records v3 as the accepted mixed-format contract for new rows. |
| `C-HASH-BYTEA` | IX-004 | medium | Sharpens SQL byte construction for v3 parity. |
| `C-SHA-PRIMITIVE` | IX-005 | medium | Pins stock hash primitive/extension and owner DDL checks. |
| `C-VERIFY-DISPATCH` | IX-006 | medium | Makes mixed-format verification behavior explicit. |
| `C-TEST-ROW-WRITER` | EV-003 | medium | Enables verifier tamper/mixed-format tests without production bypasses. |
| `C-TZ-INDEPENDENT-HASH` | EV-005 | medium | Extends timestamp/hash parity across DB timezone settings. |
| `C-GUC-PARAMETERIZED` | PR-003 | medium | Prevents attribution value leakage through query text. |
| `C-SOCKET-DIR-PERMS` | PR-004 | medium | Pins `0700` socket directory enforcement. |
| `C-OWNER-DR` | OPS-6 | medium | Required runbook coverage for non-decryptable owner credentials. |
| `C-DOCTOR-OWNER-REACH` | OPS-7 | medium | Required for owner-fallback diagnosis in remote-PG posture. |
| `C-SOCKET-RELOCATE-MIGRATION` | OPS-8 | medium | Required runbook/default updates for L2 socket relocation. |

## Sequencing Inputs

The staged inputs imply this implementation sequence if the adjudicator sends
the spec to revision. This is a reordering of source constraints, not a new
challenge:

1. **Spec correction before code:** decide the direct-`EXECUTE` authority claim,
   record v3 mixed-format semantics, define GUCs as labels or backed by
   non-spoofable authority, write the `SECURITY DEFINER` hardening template, and
   specify owner-DDL/binary deploy order plus rollback posture.
2. **L3 + L0 doctor probe:** implement transaction-bound attribution prelude and
   robust reset, parameterized GUC setting, `daemon_auth_log` privacy, owner
   reachability, and single-role posture visibility.
3. **L0 rotation + L1 Phase 0:** land owner bootstrap, restart diagnostics,
   runtime credential posture, v3 hash helpers, `append_audit_row`, `T-42501`,
   grant-drift guards, harness privilege fidelity, and rollback/forward-only
   gates.
4. **L1 Phase 1/2 and RLS:** move artifacts and events behind hardened write
   functions; apply direct-execute authority decision consistently; add
   transcript-exclusion validation where the event phase expands DB-level
   enforcement; apply RLS only after the function gates.
5. **L2 hardening:** ship the PG-less lane user, `0700` socket-dir checks,
   negative lane isolation tests, secure-adoption/default-on criteria, and
   socket relocation runbook.
6. **Remote/multi-host follow-up:** keep cross-host certs deferred as RFC 0110
   states, but resolve `C-ROTATION-SINGLE-WRITER`, owner DR, and doctor owner
   access before claiming remote-PG readiness.

## Adjudication Checklist

- The candidate's hash design is a viable replacement for Go JSON parity only
  if the v3 mixed-format contract and deployment/rollback behavior become
  explicit.
- The candidate is not yet complete on the core D164 product claim unless
  `C-EXEC-AUTH` is accepted or the claim is narrowed.
- GUC-backed attribution should not be treated as authorization without an
  additional non-spoofable authority source.
- The source trajectory contains no candidate rebuttals because the
  interrogation window was closed; the absence of registered interrogations is
  not evidence of cleared cross-examination.
- The cross-exam source recommendation is `needs_revision`, driven by one
  critical finding, sixteen high findings, and the OPS-2 conceded single-role
  no-op.
