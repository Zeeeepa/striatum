# RFC 0110 Implementation Cross-Exam
author: cross-examiner-codex-gpt-5.5-xhigh-002
artifact_kind: handoff
logical_name: cross_examiner_2
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 2
posture: implementation
target: docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/synthesis/SYNTHESIS_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/convener_synthesis/draft/CANDIDATE_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/cross_exam/synthesis/CROSS_EXAM_SYNTHESIS_cycle_1.md
  - go/pkg/db/connection.go
  - go/pkg/db/audit.go
  - go/pkg/mutations/mutations.go
  - go/pkg/db/sql/0005_repo_local_workflow_state.sql
  - go/pkg/pgtest/pgtest.go

## Interrogation

Target attempted: `sess_eab020240ffd8880cae29de0707d17b5`

Challenge I attempted to put to the cycle-2 convener:

> The cycle-2 fix relies on parameterized `set_config` so `striatum.daemon_auth`
> never appears in PostgreSQL query text. The daemon currently forces
> `pgx.QueryExecModeSimpleProtocol`, and pgx simple protocol uses client-side
> parameter interpolation. What exact connection/query path guarantees the
> daemon-auth secret is not visible in `pg_stat_activity` or statement logs to a
> raw `striatumd_rw` connection?

Structured turn reference: `interrogation.open` returned
`capability_denied` with message `interrogator session lacks the 'interrogate'
capability` for interrogator session `sess_daf075b0aaf5816ce5972114906f6b95`.
No interrogation id was created, so no `interrogation.ask` or target rebuttal was
possible.

Rebuttal reference: none. Because the question was not delivered, the absence of
a rebuttal is recorded as process evidence only; the findings below are grounded
in the published cycle-2 synthesis and source/doc anchors.

## findings[]

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape required |
| --- | --- | --- | --- | --- | --- |
| IX2-001 | critical | `C-EXEC-AUTH`; `C-GUC-PARAMETERIZED`; daemon-auth secret is non-spoofable and not visible to raw `striatumd_rw` SQL. | Cycle 2 says the authority prelude uses parameterized `set_config` so the daemon-auth secret never appears in `pg_stat_activity.query`. Current `go/pkg/db/connection.go` sets `cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol`, and the local pgx v5 source documents that simple protocol uses client-side parameter interpolation. Under the same DB role, a raw `striatumd_rw` session can plausibly observe another daemon session's query text and learn the secret, collapsing the authority gate back to "possess DSN + watch pg_stat_activity/logs." | Do not run the authority prelude over the simple-protocol pool. Either move hot daemon queries to extended protocol and keep simple protocol only for migrations, or issue the prelude through an explicit extended-protocol connection/path that proves parameter values are not sent as SQL text. If simple protocol must remain global, the design needs a different authority carrier than a secret-bearing GUC. | `C-EXTENDED-AUTH-PRELUDE`: a PG-gated regression runs as `striatumd_rw`, triggers a daemon-authorized mutation, samples `pg_stat_activity`/statement tracing from a same-role observer, and proves the daemon-auth value never appears. A unit guard fails if the prelude path executes under `QueryExecModeSimpleProtocol`. |
| IX2-002 | high | `C-TX-GUC-PRELUDE`; every mutating transaction starts with attribution + authority before its first write. | Cycle 2 fixes the old `BeforeAcquire` placement in prose, but the current mutation stack exposes only generic `withTx(ctx, runner, fn)` and `runner.BeginTx(ctx)`. Nothing in that API can prove the first statement inside `fn` is the authority prelude; each handler would have to remember to call `applyAttribution` before any SQL. With dozens of `withTx` callers, that is not an implementation-ready invariant. | Introduce a dedicated mutation transaction constructor, for example `withAuthorizedMutationTx(ctx, runner, attrs, fn)`, that applies the prelude before calling `fn` and passes an `AuthorizedTx` type to mutating handlers. Leave generic `withTx` for non-authoritative bootstrap/read-maintenance paths only. | `C-AUTH-TX-WRAPPER`: a static or hermetic guard enumerates mutating RPC handlers and fails any handler that calls generic `withTx`/`BeginTx` directly instead of the authorized wrapper. A fake transaction test records SQL order and proves `set_config('striatum.daemon_auth', ...)` is statement 1 before the first DML/write function call. |
| IX2-003 | high | Audit append is mandatory provenance for every RPC and participates in L1 authority. | `go/pkg/rpc/server.go` records audit after the handler returns; `go/pkg/db/audit.go` opens its own transaction in `RecordRPCTransport`. If `append_audit_row` becomes a daemon-authorized SECURITY DEFINER function, this independent audit transaction also needs the daemon-auth prelude. Worse, current `server.go` ignores `auditErr` except omitting `response.AuditID`, so a missing/failed authority prelude could let a mutation succeed while its audit row silently fails. | Thread the daemon-authority secret and attribution context into `AuditRecorder`, apply the authorized prelude inside the audit transaction, and decide fail-closed semantics for audit append failure before L1 lands. Audit append failure after a mutation cannot be a quiet best-effort side effect. | `C-AUDIT-AUTH-PRELUDE`: `RecordRPCTransport` calls `append_audit_row` only after applying daemon authority and attribution labels; a regression forces the audit function to reject and proves the RPC path fails loudly or rolls back according to the recorded contract, never returns success without an audit row. |
| IX2-004 | high | Owner-applied DDL and runtime auto-migrations do not crash-loop or reorder L1 revokes/functions. | Cycle 2 names owner-applied DDL and startup preconditions, but current `striatum daemon migrate-db` still calls `db.ConnectAndMigrate`, and daemon startup also calls `ConnectAndMigrate` through the runtime DSN. Owner-only L1 DDL cannot simply be dropped into the normal embedded migration sequence: the runtime role may try to apply it on startup, while premature revokes can break an old binary's direct inserts. | Split owner-only L1 DDL from runtime auto-migrations with an explicit version/precondition handshake. The new binary should refuse to serve if owner DDL is absent; the old binary should not auto-apply revokes it cannot use. The owner migration path should be idempotent and independently testable with owner and runtime roles. | `C-OWNER-DDL-SPLIT`: owner-only L1 migrations have a distinct delivery path or marker; runtime `ConnectAndMigrate` never attempts owner-only function/revoke DDL; skew tests cover new-binary/old-schema and old-binary/premature-revoke before the first mutation. |
| IX2-005 | high | `T-42501` and grant-drift tests exercise production privileges, not a polluted harness. | Cycle 2 keeps `T-HARNESS-FIDELITY`, and the source still shows why it is load-bearing: `go/pkg/pgtest/pgtest.go` creates per-test runtime roles by issuing broad `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES` and then hand-revoking a small subset. If the RFC implementation keeps that helper shape, security tests can pass against a privilege layout that production migrations never create or that later grant repair reopens. | Move role/privilege setup into SQL fixtures that mirror owner/runtime migrations, and make `pgtest` only connect as those roles. If a test needs a bypass for verifier tamper cases, give it a narrowly named owner-only test utility rather than patching runtime grants. | `C-PGTEST-NO-DML-GRANT`: a guard fails any pgtest setup path that grants or revokes protected-table DML outside migration-owned SQL. `T-42501` runs against migration-defined roles after migrations, after owner DDL, and after any grant-repair helper. |

## Implementation posture summary

Cycle 2 answers the first-round implementation findings in the abstract, but
the implementation-ready spec still needs one more hardening pass around the
actual Go seams. The highest-risk issue is `daemon_auth` over pgx simple
protocol: if the secret is interpolated into query text, the new authority gate
can be observed by the same raw DB role it is supposed to stop. After that, the
work is about making the corrected prelude unavoidable, making audit append
fail closed, separating owner DDL from runtime auto-migrations, and keeping the
test harness honest.
