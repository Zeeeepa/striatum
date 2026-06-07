# RFC 0114 Implementation Draft — Owner Bundle 0006 Read-Scope Reduction (#164)
author: operator
artifact_kind: handoff
status: complete
date: 2026-06-07

## Summary

Implements the accepted RFC 0114 design (D173) exactly as written: owner
bundle 0006 transfers ownership of `striatumd.principals`,
`striatumd.principal_clients`, and `striatumd.client_sessions` to the owner
role FIRST, installs owner-owned `SECURITY DEFINER` projections gated by
`striatumd.assert_daemon_authority()`, then revokes direct runtime `SELECT`
(`principals`, `client_sessions`: full deny; `principal_clients`: column gate
denying `principal_id`). Runtime read paths route through the projections
with the bundle-0005 dual-path fallback discipline, `daemon doctor`'s
`pg_read_scope.posture` is now derived from stamps + privilege/ownership
probes, and the pg-gated guard suite pins every closure claim. No new RPC
methods; `docs/reference/command-authority-matrix.md` unchanged.

**RFC 0114 Open Question 4's contingency was taken** (see "Deviations and
discoveries" below): PostgreSQL demands `SELECT` on the `ON CONFLICT` arbiter
columns, so the active-link upsert moved behind an owner-owned
`link_client_to_principal(p_daemon_secret, p_principal_id, p_client_id)`
write function, exactly as the RFC pre-approved. External behavior is
unchanged.

Work is committed in the per-job worktree (detached HEAD over
`striatum/rfc-0114-impl-164` tip `1dd64bd8`):

- `bcb643e1` feat(db): owner bundle 0006 — identity read-scope reduction (#164, RFC 0114)
- `25ea0ce2` fix(db): take RFC 0114 OQ4 contingency — SD link_client_to_principal

## What landed where

| File | Change |
|---|---|
| `go/pkg/db/sql/owner/0006_identity_read_scope.sql` | NEW. Projections (`get_principal`, `resolve_principal_for_client`, `list_principal_scopes` — verbatim transplants of the handler SQL, secret-as-parameter pattern), OQ4 `link_client_to_principal` SD write function, `ALTER TABLE ... OWNER TO CURRENT_USER` for the three tables, revokes + explicit DML/column grant-backs, `identity_projection_read` stamp (bundle 6). |
| `go/pkg/db/owner.go` | `LatestOwnerBundleVersion` 5 → 6; bundle 6 label; `ReassertReadRevokes` restructured map-driven (`readScopeReasserts`: `auth_projection_read` → the two 0005 clients statements, `identity_projection_read` → the six bundle-6 statements verbatim). |
| `go/pkg/db/capability_parity.go` | `SupportedAuthorityCapabilities()` gains `identity_projection_read` (the bundle-0005 registration precedent). |
| `go/pkg/db/read_authority_inventory.go` | New class `ReadClassRuntimeProjection` (`runtime_projection_read`); `principals` reclassified to it; `client_sessions` → `runtime_select_denied`; `principal_clients` stays `runtime_sensitive_select`; `RuntimeDeniedReadColumns()` gains `principal_clients: [principal_id]`. |
| `go/pkg/db/connection.go` | `PgxRunner.ExecBound` — pool-scoped sibling of `PgxTxRunner.ExecBound` for the OQ4 write call (extended protocol; secret never in `pg_stat_activity` query text). |
| `go/pkg/admin/principals.go` | Dual-path helpers `queryIdentityRow` / `execIdentityWrite` (secret gate + SQLSTATE 42883 fallback, the `authorizeWithProjection` + `loadTokenForUpdate` precedents combined); all six consumer-map call sites routed: `CreatePrincipal` pre-check, both existence checks (`principalExists`), the active-link check, `ResolvePrincipalForClient`, `ListPrincipals`, and the link upsert (via `link_client_to_principal`). DTO decode paths untouched. |
| `go/pkg/cli/localcommands/daemon.go` | `runDaemonOwnerDDL` now calls `ReassertWriteRevokes` + `ReassertReadRevokes` after `ApplyOwnerBundles` — re-running `striatum daemon owner-ddl apply` is the documented grant-drift repair (previously neither reassert had a production caller). |
| `go/pkg/reads/doctor_pg_read_scope.go` | `pgReadScopeDoctorBlock(ctx, runner)` — posture derived from `schema_authority` stamps + `has_table_privilege`/`has_column_privilege` probes + `pg_tables.tableowner` ownership probes against the static gate map (clients/columns/b5; principals/table/b6; principal_clients/columns/b6; client_sessions/table/b6). Posture rules and `grant_drift` array per the RFC; `private_read_denial` remains `posture == "private_read_denial"` (false). |
| `go/pkg/reads/doctor.go` | Passes `(ctx, runner)` to the read-scope block. |
| `go/pkg/db/read_authority_inventory_pg_test.go` | + `TestProjectionReadTablesHaveNoRuntimeSelect`, `TestClientSessionsRetainRuntimeDML`, `TestOwnerTransferClosesSelfRegrant`, `TestIdentityGateTablesNotRuntimeOwned`; `TestReadDeniedColumnsHaveNoRuntimeSelect` gains the `principal_clients.client_id/linked_at/unlinked_at` positive controls. |
| `go/pkg/db/authority_enforcement_pg_test.go` | + `TestPrincipalProjectionsRequireDaemonAuthority` (now covering the OQ4 write function too), `TestReassertReadRevokesReclosesIdentitySelect`. |
| `go/pkg/admin/principals_pg_test.go` | NEW. `TestPrincipalReadProjectionParity`, `TestCreatePrincipalSemanticsAfterReadClose`, `TestLinkUpsertAndUnlinkSurviveReadClose`, `TestTokenRotationCarriesPrincipalAfterReadClose`. |
| `go/pkg/reads/doctor_pg_read_scope_pg_test.go` | NEW. `TestPgReadScopePostureDerivation`. |
| `go/pkg/reads/doctor_pg_read_scope_test.go` | Rewritten for the derived block (nil-runner case: four gates, all unstamped, posture broad). |
| `CHANGELOG.md` | Entry under `Unreleased`. |

## Gates and observed results

All commands run from the per-job worktree. `make lint`:

```
"/home/striatum-lane/go/bin/golangci-lint" run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...
0 issues.
```

`make typecheck` green; `make test` (no PG URL): 35 packages `ok`, zero
`FAIL`. Note: the full suite must run with the lane control variables unset
(`env -u STRIATUM_REPOSITORY_ID ...`) — `TestDispatchRoutesDaemonTokenCreate`
asserts daemon-global routes carry no `repository_id` and fails when the
supervisor-injected `STRIATUM_REPOSITORY_ID` leaks into the test process.
Pre-existing behavior, verified independent of this change.

### PG-gated guard tests (live PostgreSQL)

Pasted verbatim from `go test -v` (filtered to result lines), run against the
lane-local scratch cluster (see "Deviations" for why):

`go/pkg/db`:

```
--- PASS: TestPrincipalProjectionsRequireDaemonAuthority (2.74s)
--- PASS: TestReassertReadRevokesReclosesIdentitySelect (2.92s)
--- PASS: TestReadAuthorityInventoryComplete (3.09s)
--- PASS: TestReadDeniedTablesHaveNoRuntimeSelect (2.99s)
--- PASS: TestReadDeniedColumnsHaveNoRuntimeSelect (3.05s)
--- PASS: TestProjectionReadTablesHaveNoRuntimeSelect (2.94s)
--- PASS: TestClientSessionsRetainRuntimeDML (2.62s)
--- PASS: TestOwnerTransferClosesSelfRegrant (2.73s)
--- PASS: TestIdentityGateTablesNotRuntimeOwned (3.16s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/db	26.238s
```

`go/pkg/admin` (full package, including all pre-existing principal/token PG
tests, ran green — the four new ones):

```
--- PASS: TestPrincipalReadProjectionParity (2.96s)
--- PASS: TestCreatePrincipalSemanticsAfterReadClose (2.86s)
--- PASS: TestLinkUpsertAndUnlinkSurviveReadClose (2.92s)
--- PASS: TestTokenRotationCarriesPrincipalAfterReadClose (2.69s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/admin	11.448s
```

`go/pkg/reads`:

```
--- PASS: TestPgReadScopePostureDerivation (2.92s)
--- PASS: TestPgReadScopeDoctorBlock (0.00s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/reads	2.925s
```

Bundle-0005 regression check (unchanged surface still closed):

```
--- PASS: TestTokenSecretColumnsUseAuthorityProjection (1.22s)
```

Full live run: `make test` with `STRIATUM_PG_TEST_URL` set → 34 packages
`ok`; the only failures are three pre-existing bootstrap-rotation tests
(`TestBootstrapRotatesPasswordTwoRole`,
`TestBootstrapTwoRoleWithoutOwnerURLActionable`,
`TestBootstrapAndConnectRecoversFromStaleRuntimePassword`) that hard-code
`localhost:5432` (`authority_bootstrap_pg_test.go:106,209,259`) and therefore
only run on a CI-shaped cluster occupying the default port. They fail at
their connection *precondition*, touch no file in this diff, and `go/pkg/db`
is fully green live with only those three skipped:

```
ok  	github.com/halbritt/striatum/go/pkg/db	138.534s
```

## Doctor posture evidence (before/after)

Pinned executable by `TestPgReadScopePostureDerivation` (PASS above), which
asserts the exact sequence on a live database:

1. **Before bundle 0006** (no authority schema): `pg_read_scope.posture =
   "broad_runtime_select"`, all four gates `stamped: false`.
2. **After `ApplyOwnerBundles`** (bundle 6 stamped, probes verify):
   `posture = "partial_projection_gated"`; gates `clients` (columns, b5),
   `principals` (table, b6), `principal_clients` (columns, b6),
   `client_sessions` (table, b6) all `stamped+verified`, table gates
   `owner_ok: true`; `private_read_denial: false`.
3. **Grant drift** (`GRANT SELECT ON striatumd.principals TO striatumd_rw`):
   posture downgrades to `broad_runtime_select` with
   `grant_drift: ["principals"]`.
4. **Repair** (`ReassertReadRevokes`, what `owner-ddl apply` now re-runs):
   posture returns to `partial_projection_gated`.
5. **0005-only state** (identity stamp removed): posture
   `broad_runtime_select` — column gates alone never claim the flip.

## Operator runbook — applying bundle 0006 to production (out-of-band)

The bundle is owner-applied per RFC 0079 §5; the daemon never applies this
DDL at runtime. Order matters: **binary first, bundle second.**

1. **Deploy the binary before the bundle**: `make install`, then
   `systemctl --user restart striatumd` (install does not restart the running
   daemon; verify the running image via `/proc/<pid>/exe`). The new binary on
   the un-bundled database takes the 42883/secretless fallbacks everywhere;
   doctor still reports `broad_runtime_select` with the bundle-6 gates listed
   un-stamped. (Reverse order leaves an old binary whose direct principal
   reads fail 42501 until the restart.)
2. **Check owner-role preconditions** (RFC 0114 §ownership): the owner DSN
   role must (a) be able to `SET ROLE striatumd_rw` or be superuser, (b) be a
   member of the target role (trivially itself), and (c) hold `CREATE` on
   schema `striatumd`. On the reference deployment the owner DSN is the
   database owner that created `striatumd_rw` and the schema, which satisfies
   all three. If not: `GRANT striatumd_rw TO <owner>` and/or
   `GRANT CREATE ON SCHEMA striatumd TO <owner>`.
3. **Apply**: `striatum daemon owner-ddl apply` (owner DSN resolution:
   `--owner-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL`, then daemon DSN).
   Bundle 6 applies atomically (per-version transaction; a precondition
   failure rolls back unstamped — grant the missing privilege and re-apply),
   and the write+read reasserts now run automatically afterwards.
4. **Verify posture**: `striatum daemon doctor` →
   `pg_read_scope.posture = "partial_projection_gated"`, all bundle-6 gates
   `stamped+verified+owner_ok`, principals block unchanged shape.
5. **Verify negatives as the runtime role** (psql with the runtime DSN):
   `SELECT ... FROM striatumd.principals` / `client_sessions` and
   `SELECT principal_id FROM striatumd.principal_clients` → SQLSTATE `42501`;
   positive control `SELECT client_id FROM striatumd.principal_clients`
   succeeds.
6. **Live parity drill**: `daemon.token.rotate` with principal attribution;
   doctor principals block before/after shows the same fields; audit
   attribution intact.
7. **Drift drill** (optional): as owner,
   `GRANT SELECT ON striatumd.principals TO striatumd_rw`; doctor reports
   `grant_drift`; re-run `striatum daemon owner-ddl apply`; doctor verifies
   re-closed.

**RFC 0079 §5 trap, now in force for these tables**: once owner-held,
`principals` / `principal_clients` / `client_sessions` can no longer be
altered by runtime-role migrations — future schema changes must ship as owner
bundles (or owner-applied `daemon migrate-db --admin-url` DDL), exactly as
`clients` / `client_capabilities` today.

## Deviations and discoveries

1. **OQ4 contingency taken (design-anticipated).**
   `TestLinkUpsertAndUnlinkSurviveReadClose` went red exactly as RFC 0114
   predicted it could: PostgreSQL requires `SELECT` on the `ON CONFLICT`
   arbiter columns. Verified live, minimal repro: with
   `GRANT INSERT, UPDATE, DELETE` + `GRANT SELECT (c, linked, unlinked)`,
   `INSERT ... ON CONFLICT (p, c) DO UPDATE SET ...` fails
   `42501 permission denied`; after `GRANT SELECT (p)` the same statement
   succeeds. Since `principal_id` is an arbiter column AND the gated column,
   the upsert moved behind the owner-owned SD
   `link_client_to_principal(p_daemon_secret, p_principal_id, p_client_id)`
   per the RFC's pre-approved contingency. The Go function is a thin
   dual-path caller; existence/active-link checks stay in Go through the read
   projections, so error semantics and DTOs are unchanged.
2. **Lane PG environment.** The work packet promised `STRIATUM_PG_TEST_URL`
   in the lane environment; it was absent (checked the lane shell, the
   supervisor process env, and every `striatum-lane` process). The system
   cluster on 5432 rejects the lane OS user via pg_hba (the L2 lane sandbox
   working as designed) and the 54329 listener requires credentials the lane
   does not hold. Reported via `session.report` (question, lease_held).
   Unblocked by running a throwaway lane-local PostgreSQL 16.14 cluster
   (`initdb` under `~/scratch/pgtest-164`, port 54399, superuser
   `striatum_pgtest`) — the pgtest harness creates scratch databases as the
   test user, which is the correct simulation of the production owner per the
   task constraints. CI (postgres on 5432) additionally covers the three
   port-bound bootstrap tests this lane cannot run.
3. **Pre-existing env-sensitivity, not fixed here**: the dispatch test
   failure under a set `STRIATUM_REPOSITORY_ID` (see Gates) and the three
   `localhost:5432`-bound bootstrap tests are both pre-existing test
   environment couplings, out of scope for this packet's write scope and
   objective; flagged for a follow-up issue.

## Follow-ups proposed (not in scope)

- `client_capabilities` closure (RFC 0114 Open Question 1, stamp
  `capability_projection_read`) — completes R1.
- Spec/`postgres-transition.md` owner-table-trap doc updates "when the
  posture lands" per the RFC's acceptance note — i.e. once bundle 0006 is
  applied to the production database, not at code-land time.
- Decouple the three bootstrap-rotation tests from `localhost:5432` (derive
  host:port from `STRIATUM_PG_TEST_URL`), and harden the dispatch test
  against ambient `STRIATUM_REPOSITORY_ID`.
