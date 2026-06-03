# RFC 0110 Adjudication Intake (cycle 2)
author: adjudicator-codex-gpt-5.5-xhigh-003
artifact_kind: handoff
logical_name: adjudication_intake_cycle_2
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 2
target: docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
inputs:
  - docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md
  - docs/decisions/decision-log.md#D164
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/adjudicator/COLLABORATION_LEDGER_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/intake/INTAKE_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/product/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/implementation/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/privacy/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/eval/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/operations/CROSS_EXAM.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/synthesis/CROSS_EXAM_SYNTHESIS_cycle_2.md

## Purpose

This intake stages the cycle-2 candidate synthesis and cross-examination
findings for adjudication. It does not add new challenges, render a verdict, or
decide which constraints become binding. It preserves the curated trajectory,
separates fresh cycle-2 findings from degraded cycle-1 re-statements, and names
the existing questions the adjudicator must resolve.

## Decision Baseline

D164 accepts RFC 0110 as a design RFC, with phased implementation following. The
accepted model remains the L0-L3 stack under the rule "make a leaked runtime
credential uninteresting":

- **L0 credential:** owner-bootstrapped, RAM-only `striatumd_rw` password,
  re-rotated on daemon restart, with remote PostgreSQL using an explicit owner
  credential path.
- **L1 enforcement:** PostgreSQL enforces the write contract with direct DML
  revoked from the runtime role and owner-owned `SECURITY DEFINER` functions
  plus later RLS.
- **L2 isolation:** dedicated PG-less lane OS user and `0700` socket directory
  become the hardened posture for closing #87, with upgrade-safe rollout.
- **L3 attribution:** authoritative mutations carry `rpc_id` and
  `principal_id`, but cycle-1 cross-exam established that GUCs are labels unless
  backed by a non-spoofable authority source.

Cycle 1 ended in `needs_revision`. The adjudicator converted one critical and
sixteen high findings into binding revision constraints, carried
`Q-DYNAMIC-CREDENTIALS` as an unresolved question, and identified medium
follow-ups for practical folding. The cycle-2 candidate is evaluated against
that ledger.

## Cycle-2 Candidate Record

The cycle-2 convener synthesis explicitly discharges every cycle-1 binding row
and preserves the cycle-1 candidate constraints. The load-bearing revision is
the new daemon-authority gate:

- **Claim split:** the synthesis separates invariant integrity from
  daemon-issuance. A leaked DSN string should not permit invariant-breaking or
  unauthorized mutation, but the design does not claim resistance to a full live
  daemon memory scrape.
- **`striatum.daemon_auth`:** the daemon creates a RAM-only authority secret,
  records only an owner-visible digest in `striatumd.daemon_auth_registry`, and
  every owner-owned write function calls `assert_daemon_authority()`.
- **GUC demotion:** `striatum.rpc_id`, `striatum.principal_id`, and
  `app.session_id` are attribution or row-scoping labels only. Authority is the
  verified `daemon_auth` secret.
- **Transaction placement:** the old `pgxpool.BeforeAcquire` idea is replaced
  with an in-transaction prelude immediately after `BeginTx` and before any
  DML, including transaction-local `set_config` calls for authority and
  attribution.
- **v3 hash contract:** the synthesis keeps Option A, but uses
  `hash_format_version = 3` with length-prefixed bytea canonical encoding rather
  than trying to reproduce Go JSON in PL/pgSQL. `VerifyRows` dispatches by hash
  format and fails on unknown formats.
- **Owner DDL / grant posture:** owner-owned functions, revokes, `pgcrypto`, and
  startup preconditions are separated from runtime writes; negative grant drift,
  `SECURITY DEFINER` hardening, and pgtest privilege fidelity are named gates.
- **Operational honesty:** owner-bootstrap failure fails closed, single-role
  owner==runtime posture is doctor-visible, v3 cutover is forward-only after an
  operator flag, and dynamic credential providers are deferred as a successor
  beyond the RFC 0110 guarantee.
- **L2 hardening:** secure-profile or fresh installs should enable the hardened
  posture or block in doctor; legacy upgrades warn until a named default-on
  release. Negative lane connection tests and socket relocation runbook updates
  remain required.

The convener records these as discharges, not acceptance. The cycle-2
adjudicator must decide whether the discharge gates are sufficient after the
new cross-exam findings.

## Cross-Exam Status

The cycle-2 cross-exam synthesis rolls up five postures and preserves each
source posture, severity, and status.

- **Findings:** 27 total: 1 critical, 19 high, 7 medium.
- **Cycle-2 engaged postures:** product, implementation, and operations examined
  `SYNTHESIS_cycle_2.md`. These are the fresh findings to extract for cycle 2.
- **Degraded postures:** privacy and eval targeted the closed cycle-1 convener
  session, lack a cycle-2 marker, and restate cycle-1 PR-/EV- findings. They are
  evidence and harness signal, but the synthesis says those rows map to
  constraints already discharged in cycle 2.
- **Interrogations:** no cross-examiner obtained a live rebuttal from the
  cycle-2 convener. Product and implementation targeted the correct live
  session but were refused for missing `interrogate` capability. Privacy and
  eval targeted the stale cycle-1 session and hit `panel_window_closed`.
  Operations was non-interrogable and recorded its question textually.

The absence of rebuttals means the engaged cycle-2 rows remain open and
unrebutted.

## Fresh Cycle-2 Findings To Stage

These rows are grounded in the revised synthesis and current source anchors.
They are the active extraction surface for adjudication.

| id | source | severity | staged constraint shape |
| --- | --- | --- | --- |
| `IX2-001` | implementation | critical | `C-EXTENDED-AUTH-PRELUDE`: the daemon-auth prelude must not run through pgx simple protocol if that interpolates parameters into query text; prove the secret is not visible to a same-role observer. |
| `IX2-002` | implementation | high | `C-AUTH-TX-WRAPPER`: make the authority and attribution prelude unavoidable through an authorized mutation transaction wrapper, not handler memory. |
| `IX2-003` | implementation | high | `C-AUDIT-AUTH-PRELUDE`: audit append needs the same authority context and fail-closed semantics; no mutation should succeed while its audit row silently fails. |
| `IX2-004` | implementation | high | `C-OWNER-DDL-SPLIT`: owner-only L1 DDL must be delivered separately from runtime auto-migrations, with skew tests before first mutation. |
| `IX2-005` | implementation | high | `C-PGTEST-NO-DML-GRANT`: pgtest must not patch protected-table DML grants outside migration-owned SQL. |
| `PX3-001` | product | high | `C-DSN-READ-SCOPE`: either define and test least-privilege read scope, or narrow the "leaked DSN is uninteresting" claim to unauthorized mutation and invariant violation. |
| `PX3-002` | product | high | `C-PHASED-WRITE-CLOSURE`: phase-scope claims about the sole durable write path until audit_log, artifacts, and events are all protected. |
| `PX3-003` | product | high | `C-AUDIT-FORMAT-CUTOVER`: treat v3 hash cutover as a product release gate; SQL append, Go verifier dispatch, mixed-format tests, and default-v2 flag ship together. |
| `PX3-004` | product | high | `C-87-CLOSURE-GATE`: keep #87 partial until the PG-less lane user, negative isolation test, and hardened doctor block are green in the default or named secure profile. |
| `PX3-005` | product | medium | `C-D164-AMEND`: update decision/spec text before behavior-changing implementation so the operator-facing contract matches cycle-2 scope. |
| `PX3-006` | product | medium | `C-MUTATION-INVENTORY`: classify durable table write authority across the daemon-owned schema, not only audit_log, artifacts, and events. |
| `OPS-9` | operations | high | `C-AUTH-WINDOW-LIVENESS`: define daemon-auth freshness lifecycle or refresh; expiry must be fail-closed and doctor-visible, not a silent total-write wedge. |
| `OPS-10` | operations | high | `C-DEPLOY-CAPABILITY-PARITY`: startup preconditions must assert that the binary can satisfy owner DDL requiring daemon-auth, not merely that functions exist. |
| `OPS-11` | operations | high | `C-ROTATOR-PROBE-ROLE-SCOPED`: concurrent-rotator detection must key on two live instances rotating the same role, not merely different instance ids. |
| `OPS-12` | operations | medium | `C-DISCARD-RECONNECT-BOUND`: bound destroy-on-error behavior, reconnect backoff, and doctor signals so reset hardening does not amplify rotation or transient-PG failures. |

Two degraded rows corroborate fresh findings without adding new constraints:
`PR-003` supports the secret/query-text concern behind `IX2-001`, and `EV-004`
supports the reset/discard robustness concern behind `OPS-12`.

## Already-Discharged Re-Statements

The privacy and eval ledgers are preserved but degraded for cycle 2. They target
the closed cycle-1 convener and restate cycle-1 findings:

- `PR-001` maps to `Q-DYNAMIC-CREDENTIALS`, deferred to a successor while
  preserving `C-SECRET-HONEST`.
- `PR-002` maps to `C-AUTH-LOG-PRIVACY`, folded into the
  `daemon_auth_log.detail` whitelist and redaction gate.
- `PR-003` maps to `C-GUC-PARAMETERIZED`, folded into the parameterized GUC
  prelude, with the active simple-protocol collision now represented by
  `IX2-001`.
- `PR-004` maps to `C-SOCKET-DIR-PERMS`.
- `PR-005` maps to `C-EVENT-NO-TRANSCRIPTS`.
- `PR-006` maps to `C-HASH-V3`.
- `EV-001` maps to `C-HARNESS-PRIVILEGES`, with the active harness-grant
  concern now represented by `IX2-005`.
- `EV-002` maps to `C-L2-NEG-TEST`.
- `EV-003` maps to `C-TEST-ROW-WRITER`.
- `EV-004` maps to `C-ATTR-RESET-FAIL`, with the active discard/reconnect
  concern now represented by `OPS-12`.
- `EV-005` maps to `C-TZ-INDEPENDENT-HASH`.
- `EV-006` maps to `C-HASH-PARITY`.

Adjudication input: do not re-extract these as fresh cycle-2 gaps unless the
adjudicator decides the cycle-2 discharge is inadequate. Their main new value is
harness evidence: two postures did not actually examine the revised target.

## Load-Bearing Seams

### 1. Daemon-auth transport secrecy

Source rows: `IX2-001`, corroborated by `PR-003`.

Cycle 2's authority gate depends on the `daemon_auth` secret remaining outside
query text visible to a raw `striatumd_rw` observer. The implementation posture
points at `QueryExecModeSimpleProtocol`, where parameter interpolation may happen
client-side. If true for the prelude path, the authority gate collapses to
"possess DSN plus observe pg_stat_activity or logs."

Adjudication input: either require `C-EXTENDED-AUTH-PRELUDE` or choose a
different non-text authority carrier. This is the single critical cycle-2 row.

### 2. Prelude cannot depend on handler memory

Source rows: `IX2-002`, `IX2-003`, `OPS-10`.

The synthesis corrected placement from `BeforeAcquire` to in-transaction
prelude, but current Go seams expose generic `withTx` and independent audit
recording. Cross-exam asks for an authorized mutation transaction wrapper, audit
prelude propagation, fail-closed audit semantics, and deploy capability parity.

Adjudication input: decide whether the spec must require a concrete
`withAuthorizedMutationTx`-style path and audit fail-closed rule before L1 Phase
0 can be considered implementation-ready.

### 3. Owner DDL and privilege fidelity

Source rows: `IX2-004`, `IX2-005`, plus cycle-1 `C-GRANT-DRIFT`,
`C-HARNESS-PRIVILEGES`, and `C-DDL-DEPLOY-ORDER`.

Owner-only L1 DDL cannot be casually inserted into runtime auto-migrations, and
tests cannot pass against hand-patched grants. The row set asks for owner-only
migration delivery or markers, runtime preconditions, production-shaped pgtest
roles, and guards against protected-table DML grants in the harness.

Adjudication input: keep this as implementation sequencing work rather than a
new design fork. It is a deployability and false-green prevention gate.

### 4. Product claim accounting

Source rows: `PX3-001` through `PX3-006`.

Product cross-exam accepts the concept of closing the original write spoofing
hole, but challenges over-claiming. The DSN may still read durable/private
state; only protected phases are closed at each release; #87 remains partial
until PG-less lanes are enforced; D164/spec text must be amended to reflect the
cycle-2 authority model; and durable table authority must be inventoried beyond
the first three tables.

Adjudication input: decide whether these become binding constraints in the
revision ledger or are handled as final spec-publication conditions. They are
not attacks on the L0-L3 direction; they are gates on truthful operator-facing
claims.

### 5. Daemon-auth day-2 operations

Source rows: `OPS-9`, `OPS-10`, `OPS-11`, `OPS-12`.

The new authority gate creates new operational failure modes: undefined
freshness windows, schema/binary parity holes, false-positive rotator probes in
per-instance-role multi-host deployments, and destroy-on-error reconnect
amplification during transient failures or rotation. Operations credits the
cycle-1 operations constraints as folded; these four are new cycle-2 rows.

Adjudication input: if the gate is accepted, it likely needs these operational
contracts to avoid replacing the original security hole with a total-write wedge
or misleading doctor posture.

## Sequencing Inputs

The staged cycle-2 record implies this sequence if the adjudicator sends another
revision:

1. **Spec corrections before code:** amend D164/spec language for claim scope,
   phased closure, GUC labels, daemon-auth authority, v3 cutover, #87 status,
   and the durable-table authority inventory.
2. **Authority mechanism hardening:** resolve the simple-protocol problem, make
   the authorized transaction wrapper unavoidable, thread authority into audit,
   and define fail-closed audit semantics.
3. **Owner DDL and harness fidelity:** split owner-only L1 DDL from runtime
   migrations, add schema/binary authority parity checks, and move pgtest
   privileges to migration-owned SQL.
4. **L3 and L0 doctor probes:** keep the in-transaction prelude and reset tests,
   but include the active `OPS-12` reconnect/discard bound and doctor signal.
5. **L0 rotation and L1 Phase 0:** only after authority transport secrecy,
   audit prelude, owner DDL split, v3 cutover gating, `T-42501`, grant-drift,
   and rollback posture are all pinned.
6. **L1 Phase 1/2 and L2:** close artifacts/events only when each protected
   table phase has direct-DML negatives and authority functions; keep #87
   partial until PG-less lane isolation is enforced and tested.

## Adjudication Checklist

- The cycle-2 daemon-authority gate is the correct response to `C-EXEC-AUTH` in
  concept, but `IX2-001` challenges whether the current pgx simple-protocol seam
  makes the secret observable.
- The revised GUC model should be judged as labels-plus-secret, not as
  GUC/RLS-based authorization.
- Product claims should be phase-scoped: audit-only closure is not full durable
  write-boundary closure; L2 warning mode is not #87 closure.
- Privacy/eval cycle-2 rows are degraded re-statements unless used to
  corroborate engaged findings.
- No live convener rebuttal exists for cycle 2, because interrogation failed by
  capability gap or stale target.
- The cross-exam roll-up recommends a narrow `needs_revision` focused on the
  daemon-auth mechanism, claim accounting, owner-DDL/harness deployment seams,
  and day-2 authority-liveness contracts. This intake records that
  recommendation as source input, not as an adjudication verdict.
