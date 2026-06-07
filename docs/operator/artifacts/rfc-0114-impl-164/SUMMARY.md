---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - docs/operator/artifacts/rfc-0114-impl-164/DRAFT.md
  - docs/operator/artifacts/rfc-0114-impl-164/review/REVIEW.md
author: operator
title: RFC 0114 owner bundle 0006 — apply & finalize summary (#164)
run_id: run_642ff6984f6f4a39feca733f9bc50e50
session_id: sess_dd8c2b403d77f7df673aa712ad800537
created_at: 2026-06-07T10:05:00Z
---

# RFC 0114 / D173 — Apply & Finalize Summary (#164)

Review verdict: **accept** (severity info, zero blocking findings). This
apply job consumed the reviewed implementation commits explicitly by id,
re-ran the full gate set from the per-job worktree, confirmed the
`CHANGELOG.md` Unreleased entry is final, and advanced the
`striatum/rfc-0114-impl-164` branch ref so the reviewed commits are no
longer detached (the reviewer's one residual note).

Implementation commits (reviewed, unchanged by this job):

- `bcb643e1` feat(db): owner bundle 0006 — identity read-scope reduction (#164, RFC 0114)
- `25ea0ce2` fix(db): take RFC 0114 OQ4 contingency — SD link_client_to_principal
- `d68d9dae` docs(rfc-0114): draft handoff for owner bundle 0006 implementation (#164)

This summary lands as one additional docs-only commit on top of `d68d9dae`;
after that commit, `refs/heads/striatum/rfc-0114-impl-164` points at the
apply-job head (previously `1dd64bd8`, which left the three commits above
unreferenced).

## Per-finding disposition

| # | Reviewer item | Disposition |
|---|---|---|
| 1 | Verdict `accept` — no blocking findings in code, SQL bundle, fallback semantics, doctor posture, or guard coverage | Nothing to apply; implementation unchanged. |
| 2 | Residual note: implementation commits detached over `striatum/rfc-0114-impl-164` at `1dd64bd8`; "the downstream apply job should consume the commit ids explicitly or update the branch/ref before final integration" | **Applied.** Worktree checked out `d68d9dae` explicitly by commit id; after the summary commit the branch ref is advanced to the apply-job head (verified not checked out in any other worktree before the ref update). |
| 3 | `CHANGELOG.md` finalization (task requirement, no reviewer objection) | **Verified final, no edit.** The Unreleased entry written in `bcb643e1` matches the accepted implementation exactly (ownership-before-revoke mechanism, OQ4 contingency, reassert wiring, derived doctor posture, inventory reclassification). Inventing churn on an accepted entry would be noise. |

## Final file list

Every file the implementation touches (diff `1dd64bd8..HEAD`):

| File | Status |
|---|---|
| `go/pkg/db/sql/owner/0006_identity_read_scope.sql` | NEW — owner bundle 0006: SD projections, OQ4 write fn, ownership transfer, revokes + grant-backs, `identity_projection_read` stamp |
| `go/pkg/db/owner.go` | bundle version 5 → 6; map-driven `ReassertReadRevokes` |
| `go/pkg/db/capability_parity.go` | + `identity_projection_read` capability |
| `go/pkg/db/read_authority_inventory.go` | new class `runtime_projection_read`; reclassifications; denied-columns map |
| `go/pkg/db/connection.go` | `PgxRunner.ExecBound` (extended protocol; secret out of query text) |
| `go/pkg/admin/principals.go` | dual-path `queryIdentityRow` / `execIdentityWrite`; six call sites routed |
| `go/pkg/cli/localcommands/daemon.go` | `owner-ddl apply` re-runs write + read reasserts |
| `go/pkg/reads/doctor_pg_read_scope.go` | derived `pg_read_scope` posture (stamps + privilege/ownership probes) |
| `go/pkg/reads/doctor.go` | passes `(ctx, runner)` to the read-scope block |
| `go/pkg/db/read_authority_inventory_pg_test.go` | + 4 guard tests; column positive controls |
| `go/pkg/db/authority_enforcement_pg_test.go` | + 2 guard tests (projection authority incl. OQ4 fn; reassert re-close) |
| `go/pkg/admin/principals_pg_test.go` | NEW — 4 post-close semantics/parity guards |
| `go/pkg/reads/doctor_pg_read_scope_pg_test.go` | NEW — live posture derivation sequence |
| `go/pkg/reads/doctor_pg_read_scope_test.go` | rewritten for the derived block |
| `CHANGELOG.md` | Unreleased entry (final, see disposition #3) |
| `docs/operator/artifacts/rfc-0114-impl-164/DRAFT.md` | author handoff (workflow artifact) |
| `docs/operator/artifacts/rfc-0114-impl-164/SUMMARY.md` | this artifact (apply job) |

No RPC surface changes; `docs/reference/command-authority-matrix.md`
intentionally untouched.

## Gates — re-run by this apply job, observed results

All commands run from the per-job worktree at the apply-job head. The lane
injects `STRIATUM_REPOSITORY_ID`, which trips the pre-existing
`TestDispatchRoutesDaemonTokenCreate` ambient-env coupling (documented in
DRAFT.md deviation 3, reproduced here on the first `make typecheck`
attempt), so the suite gates run with `env -u STRIATUM_REPOSITORY_ID`.

`make lint` — this is byte-for-byte the CI invocation (CI installs the same
pinned `golangci-lint v2.12.2` and runs `make -C go check`; local binary
verified `version 2.12.2`):

```
"$HOME/go/bin/golangci-lint" run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...
0 issues.
```

`env -u STRIATUM_REPOSITORY_ID make typecheck` (delegates to
`go test ./...`): 35 packages `ok`, zero `FAIL`, exit 0.

`env -u STRIATUM_REPOSITORY_ID make test`: 35 packages `ok`, zero `FAIL`,
ending:

```
ok  	github.com/halbritt/striatum/go/pkg/workflowtemplates	(cached)
```

### PG-gated guard tests (live PostgreSQL)

`STRIATUM_PG_TEST_URL` is not provided in this lane (same as both prior
jobs); ran against a throwaway PostgreSQL 16.14 cluster (`initdb` under the
lane scratch home, port 54399, superuser `striatum_pgtest`, trust auth,
`STRIATUM_PG_TEST_URL=postgres://striatum_pgtest@localhost:54399/postgres?sslmode=disable`),
`go test -v -count=1`.

`go/pkg/db` (bundle-0006 guards + bundle-0005 regression):

```
--- PASS: TestTokenSecretColumnsUseAuthorityProjection (0.18s)
--- PASS: TestPrincipalProjectionsRequireDaemonAuthority (0.18s)
--- PASS: TestReassertReadRevokesReclosesIdentitySelect (0.17s)
--- PASS: TestReadAuthorityInventoryComplete (0.21s)
--- PASS: TestReadDeniedTablesHaveNoRuntimeSelect (0.17s)
--- PASS: TestReadDeniedColumnsHaveNoRuntimeSelect (0.20s)
--- PASS: TestProjectionReadTablesHaveNoRuntimeSelect (0.16s)
--- PASS: TestClientSessionsRetainRuntimeDML (0.16s)
--- PASS: TestOwnerTransferClosesSelfRegrant (0.16s)
--- PASS: TestIdentityGateTablesNotRuntimeOwned (0.16s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/db	1.754s
```

`go/pkg/admin`:

```
--- PASS: TestPrincipalReadProjectionParity (0.19s)
--- PASS: TestCreatePrincipalSemanticsAfterReadClose (0.17s)
--- PASS: TestLinkUpsertAndUnlinkSurviveReadClose (0.18s)
--- PASS: TestTokenRotationCarriesPrincipalAfterReadClose (0.16s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/admin	0.699s
```

`go/pkg/reads`:

```
--- PASS: TestPgReadScopePostureDerivation (0.18s)
--- PASS: TestPgReadScopeDoctorBlock (0.00s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/reads	0.189s
```

Three independent gate runs (author draft, reviewer's isolated cluster on
55432, this apply job on 54399) now agree on the full guard suite.

## Operator runbook — applying owner bundle 0006 to production (out-of-band)

The bundle is owner-applied per RFC 0079 §5; the daemon never applies this
DDL at runtime. Order matters: **binary first, bundle second.**

1. **Deploy the binary before the bundle.** `make install`, then restart
   the daemon (`systemctl --user restart striatumd`; `make install` does
   not restart the running daemon — verify the running image via
   `/proc/<pid>/exe`). The new binary on the un-bundled database takes the
   SQLSTATE `42883` / secretless fallbacks everywhere; `striatum daemon
   doctor` still reports `pg_read_scope.posture = "broad_runtime_select"`
   with the bundle-6 gates listed un-stamped. Reverse order leaves an old
   binary whose direct principal reads fail `42501` until the restart.
2. **Check owner-role preconditions** (RFC 0114 ownership section). The
   owner DSN role must: (a) be able to `SET ROLE striatumd_rw` or be
   superuser, (b) be a member of the target role (trivially itself), and
   (c) hold `CREATE` on schema `striatumd`. On the reference deployment
   the owner DSN is the database owner that created `striatumd_rw` and the
   schema, satisfying all three. If not:
   `GRANT striatumd_rw TO <owner>` and/or
   `GRANT CREATE ON SCHEMA striatumd TO <owner>`.
3. **Apply**: `striatum daemon owner-ddl apply` (owner DSN resolution
   order: `--owner-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL`, then the
   daemon DSN). Bundle 6 applies atomically — each bundle version runs in
   its own transaction, so a precondition failure rolls back unstamped;
   grant the missing privilege and re-apply. The write + read reasserts
   now run automatically after the bundles.
4. **Verify posture flip**: `striatum daemon doctor` →
   `pg_read_scope.posture = "partial_projection_gated"`; all four gates
   (`clients` columns/b5, `principals` table/b6, `principal_clients`
   columns/b6, `client_sessions` table/b6) report `stamped` + `verified`,
   table gates additionally `owner_ok: true`; `grant_drift` empty;
   `private_read_denial: false` (expected — see "What stays open").
   The principals doctor block keeps its pre-bundle shape.
5. **Verify negatives as the runtime role** (psql with the runtime DSN):
   `SELECT count(*) FROM striatumd.principals`,
   `SELECT count(*) FROM striatumd.client_sessions`, and
   `SELECT principal_id FROM striatumd.principal_clients LIMIT 1` must all
   fail SQLSTATE `42501`; positive control
   `SELECT client_id FROM striatumd.principal_clients LIMIT 1` succeeds.
6. **Live parity drill**: run `daemon.token.rotate` with principal
   attribution; the doctor principals block shows the same fields
   before/after and audit attribution stays intact.
7. **Drift drill (optional)**: as owner,
   `GRANT SELECT ON striatumd.principals TO striatumd_rw`; doctor
   downgrades to `broad_runtime_select` with `grant_drift: ["principals"]`;
   re-run `striatum daemon owner-ddl apply`; doctor verifies re-closed to
   `partial_projection_gated`. (The reviewer additionally exercised the
   ownership-drift variant live; the posture probes catch both.)

**RFC 0079 §5 trap, now in force for these tables**: once owner-held,
`principals` / `principal_clients` / `client_sessions` can no longer be
altered by runtime-role migrations — future schema changes to them must
ship as owner bundles (or owner-applied `daemon migrate-db --admin-url`
DDL), exactly as `clients` / `client_capabilities` today.

## What stays open on #164 after this lands

- **R2/R3 surfaces stay open; `private_read_denial` remains `false`.**
  The posture ceiling after bundle 0006 is `partial_projection_gated`;
  graduation to `private_read_denial` requires the RFC 0113 §3 R2
  (prose/workflow tables) and R3 (artifact/event metadata) closures, which
  need the projection-pattern precedent this RFC establishes but are not
  part of it.
- **`client_capabilities` closure** (RFC 0114 Open Question 1, stamp
  `capability_projection_read`) — the last R1 table; completing it closes
  R1.
- **Production apply is pending.** Code landing does not flip the posture;
  the runbook above must be executed against the production database
  out-of-band. Until then doctor truthfully reports
  `broad_runtime_select`.
- **Doc updates ride the posture, not the code-land**: `docs/reference/spec.md`
  and `docs/how-to/postgres-transition.md` owner-table-trap updates happen
  once bundle 0006 is applied to the production database (RFC acceptance
  note).
- **Test-environment decoupling follow-ups** (pre-existing, flagged in
  DRAFT.md): three bootstrap-rotation tests hard-code `localhost:5432`;
  `TestDispatchRoutesDaemonTokenCreate` is sensitive to ambient
  `STRIATUM_REPOSITORY_ID`.
- **Integration note**: this branch forked before the v2.30.0 release
  commit on `main`; at integration time the Unreleased CHANGELOG section
  merges above `## v2.30.0` (trivial, docs-only reconciliation).
