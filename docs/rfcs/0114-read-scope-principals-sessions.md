# RFC 0114: Runtime read-scope least privilege — principals, principal_clients, client_sessions

Status: proposed
Date: 2026-06-06
author: author-claude-opus-4.8-001
Context: RFC 0113 (runtime read-scope least privilege, R1 started), RFC 0110
(daemon PostgreSQL authentication and database-enforced write boundary),
RFC 0107 (multi-principal trust model), RFC 0079 §5 (owner-table migration
trap), GH #164; `go/pkg/db/sql/owner/0005_token_read_scope.sql`,
`go/pkg/db/read_authority_inventory.go`, `go/pkg/db/owner.go`,
`go/pkg/reads/doctor_pg_read_scope.go`, `go/pkg/rpc/auth_pg.go`,
`go/pkg/admin/principals.go`, `go/pkg/admin/tokens.go`,
`go/pkg/db/sql/0023_principals.sql`, `go/pkg/db/sql/0001_baseline.sql`.

## Problem

RFC 0113 R1 landed its first step: owner bundle 0005 revoked direct runtime
`SELECT` on `striatumd.clients.token_hash` / `token_salt` and moved token
authorization and token-for-update reads behind owner-owned `SECURITY DEFINER`
functions gated by `striatumd.assert_daemon_authority()`. Everything else in
the R1 group — `principals`, `principal_clients`, `client_sessions`,
`client_capabilities` — remains directly selectable by a leaked live
`striatumd_rw` credential:

- `principals.display_name` is human/operator identity prose, and
  `principal_kind` reveals whether an identity is a human.
- `principal_clients` is the attribution graph: which principal owns which
  credential. It is what makes the audit chain attributable; it is also
  exactly the map an attacker wants.
- `client_sessions` links live session ids to credentials (`client_id`),
  with transport and liveness timestamps.

Two further problems block a naive extension of the 0005 recipe:

1. **Ownership.** `clients` / `client_capabilities` are owner-held, which is
   why 0005's `REVOKE` sticks. But `principals`, `principal_clients`
   (migration `0023_principals.sql`) and `client_sessions`
   (`0001_baseline.sql`) are created by runtime-role migrations and are owned
   by `striatumd_rw` in production. A `REVOKE SELECT ... FROM striatumd_rw` on
   a table it owns does flip `has_table_privilege` to false — but the owner
   can re-grant itself `SELECT` in one statement with the same leaked
   credential, so the revoke is not a security boundary. The closure must
   transfer ownership first.
2. **Doctor honesty.** `pgReadScopeDoctorBlock()` hard-codes
   `posture: "broad_runtime_select"`. RFC 0113 §4 requires the posture to flip
   to `partial_projection_gated` when a sensitive group closes, and requires
   the posture to be inspectable rather than a constant.

This RFC is design-only (the dogfood run that produces it changes no `go/`
source and applies nothing to a live database). Implementation is a later PR.

## Goals

1. Close direct runtime reads of principal identity prose, the
   principal↔client attribution graph, and session↔credential linkage, while
   preserving every live read and write path byte-for-byte (same DTO fields).
2. Resolve the runtime-table ownership problem with a mechanism that a leaked
   live runtime credential cannot reverse.
3. Replace the hard-coded doctor posture with a derived, probe-backed posture
   that flips to `partial_projection_gated` when this group closes and can
   detect grant drift and ownership drift.
4. Extend — not fork — the RFC 0110/0113 machinery: owner bundles,
   `schema_authority` stamps, `assert_daemon_authority()`, `ReassertReadRevokes`,
   the read-authority inventory and its PG-gated guards.

## Non-Goals

- No change to write authority. `INSERT/UPDATE/DELETE` grants on these tables
  stay as they are today (`ClassRuntimeDML` in the write inventory); RFC 0110
  owns the write boundary. Where a read revoke would break a write statement,
  this RFC narrows the grant-back instead of moving writes into SQL.
- No `private_read_denial` claim. After this RFC the posture is
  `partial_projection_gated`; R2/R3 of RFC 0113 §3 remain open.
- No standing broad read role, no RLS as primary boundary, no hosted services
  or external identity (RFC 0113 Goals/Non-Goals carry over).
- No drop of the vestigial `client_sessions` table (see Open Questions).

## Scope and ordering

This RFC closes **all three remaining runtime-owned R1 surfaces in one owner
bundle (0006)**: `client_sessions`, `principals`, and `principal_clients`.
`client_capabilities` — the last R1 table, already owner-held — is deferred
(Open Question 1).

Justification of the grouping and internal order:

- `client_sessions` first (within the bundle and within verification): it has
  **zero live Go consumers** — the only references in `go/` are the two
  inventory maps (`read_authority_inventory.go`,
  `write_authority_inventory.go`); nothing reads or writes it since the
  Python runtime retired (RFC 0078). Closing it carries zero parity risk and
  is the cheapest live proof of the load-bearing ownership-transfer
  mechanism. It needs no projection function at all: full `SELECT` denial.
- `principals` + `principal_clients` second: highest sensitivity (identity
  prose + attribution graph) but a real parity surface — six read/write paths
  enumerated in §“Daemon read-handler changes” must keep working. They ship
  in the same bundle because the bundle is atomic per version
  (`applyOneOwnerBundle` wraps each version in one transaction), so there is
  no partially-closed intermediate state to reason about; verification still
  proceeds surface-by-surface.
- One bundle, one stamp (`identity_projection_read`), mirroring bundle 0004's
  "phase 2 full" precedent of several objects per bundle. A reviewer
  preferring two bundles (0006 sessions / 0007 principals) can split the SQL
  stanzas verbatim; nothing else in this design changes except a second stamp
  and label.

## The ownership constraint — resolution (Option A)

**Chosen: Option A — `ALTER TABLE ... OWNER TO CURRENT_USER` inside owner
bundle 0006, then projections + revokes as in 0005.**

- Option B (column/row scoping without ownership transfer) is refuted by
  mechanism: a table owner — or any role that owns the table — can always
  `GRANT SELECT` back to itself. The revoke would look closed in
  `has_table_privilege` probes until the moment a leaked credential reopens
  it with one statement. The guard test
  `TestOwnerTransferClosesSelfRegrant` (§Guard tests) pins this refutation
  executable: before transfer, the self-re-grant succeeds; after transfer it
  fails.
- Option C (defer these tables, do an already-owner-held surface first) only
  has one candidate left in R1 — `client_capabilities` — and it leaves the
  two highest-value identity surfaces named by #164 open while still not
  solving the ownership problem that R2 tables (`work_packets`, `blockers`,
  `jobs`, ... all runtime-created) will hit anyway. Option A here builds the
  ownership-transfer precedent every later phase needs.

Mechanics and preconditions:

- The bundle issues `ALTER TABLE striatumd.<t> OWNER TO CURRENT_USER` for the
  three tables. `CURRENT_USER` is whoever applies the bundle — the owner DSN
  role (`striatum daemon owner-ddl apply`, resolution: `--owner-url`, then
  `STRIATUM_DAEMON_ADMIN_DB_URL`, then daemon DSN), the same role that owns
  the 0001–0005 SD functions and the `clients` table. In pgtest the pool user
  already owns the tables, so the statement is an idempotent no-op.
- PostgreSQL preconditions, named in full: the applying role must (a) be able
  to `SET ROLE` to the current owner (`striatumd_rw`) or be superuser, (b) be
  a member of the target role (trivially itself), and (c) hold `CREATE`
  privilege on schema `striatumd` — `ALTER TABLE ... OWNER TO` requires the
  *new* owning role to be able to create objects in the table's schema. On
  the reference deployment the owner DSN is the cluster/database owner that
  created both `striatumd_rw` and the schema, which satisfies all three; the
  same role already creates owner-owned objects in `striatumd` when applying
  bundles 0001–0005, so (c) is exercised by every existing deployment.
  Deployments whose owner/admin role is not the schema owner (and differs
  from the role used in local pgtests) must first
  `GRANT striatumd_rw TO <owner>` and/or
  `GRANT CREATE ON SCHEMA striatumd TO <owner>`. If any precondition fails,
  the bundle fails atomically (per-version transaction rolls back, no stamp)
  and the operator grants the missing privilege before re-applying.
- Existing FKs survive ownership transfer unchanged
  (`client_sessions.client_id → clients`, `principal_clients.principal_id →
  principals`); FK RI checks execute with table-owner privileges, so they
  keep working after the revokes.

**RFC 0079 §5 consequence (owner-table trap), accepted explicitly:** once
owner-held, these three tables can no longer be altered by runtime-role
migrations. Any future schema change to `principals`, `principal_clients`, or
`client_sessions` must ship as an owner bundle (or owner-applied
`daemon migrate-db --admin-url` DDL), exactly as `clients` /
`client_capabilities` work today. The creating migrations 0001/0023 use
`CREATE TABLE IF NOT EXISTS` and run once per `schema_migrations` version, so
re-running migrations on an adopted database attempts no ALTER and cannot
crash-loop the daemon. Fresh bootstrap order also converges: runtime
migrations create the tables (runtime-owned), then `owner-ddl apply` transfers
and closes them.

### The write entanglement (discovered constraint)

PostgreSQL requires `SELECT` privilege on columns *read* by a write
statement. Two live `principal_clients` writers read columns:

- `admin.LinkClientToPrincipal` — `INSERT ... ON CONFLICT (principal_id,
  client_id) DO UPDATE SET unlinked_at = NULL, linked_at = now()`
  (`principals.go:144`). Its `DO UPDATE` expressions and condition read no
  existing columns, so per the documented privilege rule it needs only
  `INSERT` + `UPDATE(unlinked_at, linked_at)` — **not** `SELECT`. This
  assumption is pinned by guard test `TestLinkUpsertAndUnlinkSurviveReadClose`;
  the contingency if it proves wrong is in Open Question 4.
- `admin/tokens.go:unlinkClientFromPrincipals` — `UPDATE ... SET unlinked_at
  = now() WHERE client_id = $1 AND unlinked_at IS NULL` (`tokens.go:275`).
  The `WHERE` clause **does** read `client_id` and `unlinked_at`, so those two
  columns must stay runtime-selectable.

Therefore `principal_clients` gets the 0005-style **column** gate — deny
exactly `principal_id`, the column that makes the linkage a graph — rather
than a full table revoke, and both writers keep working unmodified. Without
`principal_id`, a leaked credential sees only client ids (already readable on
the non-secret `clients` projection from 0005) and link timestamps, not *whose*
credentials they are. `principals` itself needs no such carve-out: its only
direct write is the plain `INSERT` in `CreatePrincipal` (no conflict clause,
no `WHERE`), so full table `SELECT` denial is safe there.

## Owner bundle 0006 plan

File: `go/pkg/db/sql/owner/0006_identity_read_scope.sql`. Contents, in order:

1. **Projection functions** — all `LANGUAGE plpgsql SECURITY DEFINER SET
   search_path = striatumd, public, pg_temp`, each beginning with
   `PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);`
   `PERFORM striatumd.assert_daemon_authority();` (the `authorize_capability`
   secret-as-parameter pattern, chosen over `load_token_for_update`'s
   ambient-GUC pattern because the doctor path calls on a bare `db.Runner`
   with no authorized-mutation prelude):

   ```sql
   striatumd.get_principal(p_daemon_secret text, p_principal_id text)
     RETURNS TABLE (principal_id text, principal_kind text, display_name text,
                    created_at timestamptz, disabled_at timestamptz)

   striatumd.resolve_principal_for_client(p_daemon_secret text, p_client_id text)
     RETURNS TABLE (principal_id text, principal_kind text, display_name text,
                    disabled_at timestamptz)

   striatumd.list_principal_scopes(p_daemon_secret text)
     RETURNS text
   ```

   Bodies are verbatim transplants of the current handler SQL:
   `resolve_principal_for_client` is the `principal_clients ⋈ principals`
   active-link join from `ResolvePrincipalForClient`
   (`principals.go:167-171`); `list_principal_scopes` is the
   `jsonb_agg(...)::text` aggregate from `ListPrincipals`
   (`principals.go:190-243`), unchanged, so the JSON payload — and therefore
   the `PrincipalScope` decode — is bit-identical. No projection is created
   for `client_sessions` (no consumer to serve).

   `REVOKE ALL ON FUNCTION ... FROM PUBLIC; GRANT EXECUTE ... TO striatumd_rw;`
   for each, as 0005 does.

2. **Ownership transfer:**

   ```sql
   ALTER TABLE striatumd.client_sessions   OWNER TO CURRENT_USER;
   ALTER TABLE striatumd.principals        OWNER TO CURRENT_USER;
   ALTER TABLE striatumd.principal_clients OWNER TO CURRENT_USER;
   ```

3. **Revokes and grant-backs** (write surface preserved exactly; read surface
   narrowed):

   ```sql
   REVOKE SELECT ON striatumd.client_sessions FROM striatumd_rw;
   GRANT INSERT, UPDATE, DELETE ON striatumd.client_sessions TO striatumd_rw;

   REVOKE SELECT ON striatumd.principals FROM striatumd_rw;
   GRANT INSERT, UPDATE, DELETE ON striatumd.principals TO striatumd_rw;

   REVOKE SELECT ON striatumd.principal_clients FROM striatumd_rw;
   GRANT SELECT (client_id, linked_at, unlinked_at)
     ON striatumd.principal_clients TO striatumd_rw;
   GRANT INSERT, UPDATE, DELETE ON striatumd.principal_clients TO striatumd_rw;
   ```

   The DML grant-backs are restated explicitly (not inherited from migration
   0023's grants) so the bundle, and `ReassertReadRevokes` below, fully
   determine the post-close ACL.

4. **Stamp:**

   ```sql
   INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
   VALUES ('identity_projection_read', true, 6)
   ON CONFLICT (capability) DO NOTHING;
   ```

Go-side bundle bookkeeping (`go/pkg/db/owner.go`):

- `LatestOwnerBundleVersion = 6`.
- `ownerBundleLabels[6] = "runtime read scope R1 step 2: principal/session
  identity projections + SELECT revokes + ownership transfer (RFC 0114)"`.
- `ReassertReadRevokes` is restructured map-driven (mirroring
  `capabilityProtectedTable`): `readScopeReasserts` maps
  `auth_projection_read` → the existing two `clients` statements, and
  `identity_projection_read` → the six statements of item 3 verbatim. Stamps
  drive re-assertion, so a deployment re-closes exactly the surfaces its
  applied bundles closed.
- **New wiring:** `runDaemonOwnerDDL` (`localcommands/daemon.go`) calls
  `ReassertWriteRevokes` + `ReassertReadRevokes` after `ApplyOwnerBundles`.
  Today neither function has a production caller (only tests), and
  re-applying a stamped bundle is a no-op — meaning grant-drift repair
  currently has **no operator verb**. After this change, re-running
  `striatum daemon owner-ddl apply` is the documented drift-repair action.

Inventory changes (`go/pkg/db/read_authority_inventory.go`):

- New class `ReadClassRuntimeProjection ReadAuthorityClass =
  "runtime_projection_read"` — table readable by the daemon only through
  daemon-authorized projections; runtime role holds no table `SELECT`.
- Reclassify: `principals` → `runtime_projection_read`; `client_sessions` →
  `runtime_select_denied` (denied with no projection — nothing consumes it);
  `principal_clients` stays `runtime_sensitive_select` with a column gate,
  exactly the `clients` precedent.
- `RuntimeDeniedReadColumns()` gains `"principal_clients": {"principal_id"}`.

## Daemon read-handler changes

All in `go/pkg/admin`, plus a small shared helper. The complete consumer map
(every statement that touches the three tables today):

| Caller | Today | After |
|---|---|---|
| `CreatePrincipal` idempotency pre-check (`principals.go:90`) | `SELECT principal_kind, display_name FROM principals` | `get_principal` projection |
| `CreatePrincipal` insert (`principals.go:105`) | plain `INSERT` | unchanged (INSERT grant) |
| `LinkClientToPrincipal` existence check (`principals.go:121`) | `SELECT '1' FROM principals` | `get_principal` projection |
| `LinkClientToPrincipal` active-link check (`principals.go:130`) | `SELECT principal_id FROM principal_clients` | `resolve_principal_for_client` projection (principal_id field) |
| `LinkClientToPrincipal` upsert (`principals.go:143`) | `INSERT ... ON CONFLICT DO UPDATE` | unchanged (no column read; pinned by guard test) |
| `ResolvePrincipalForClient` (`principals.go:166`) | join `principal_clients ⋈ principals` | `resolve_principal_for_client` projection |
| `ListPrincipals` (`principals.go:190`) | jsonb aggregate | `list_principal_scopes` projection |
| `attributeClientToPrincipal` existence check (`principals.go:278`) | `SELECT '1' FROM principals` | `get_principal` projection |
| `unlinkClientFromPrincipals` (`tokens.go:275`) | `UPDATE ... WHERE client_id/unlinked_at` | unchanged (column SELECT grant-back) |

**Dual path** — one helper, used by all four projection call sites, combining
the two landed precedents (`PostgresAuthorizer.authorizeWithProjection`'s
42883 fallback, `loadTokenForUpdate`'s secret gate):

- If `db.AuthorityFromContext(ctx).Secret == ""` (daemon without a
  bootstrapped authority registry, direct handler unit tests): run today's
  direct SQL. On an un-adopted database this is the permanent path and
  nothing changes.
- Otherwise call the projection via `QueryRowBound` (extended protocol — the
  secret never appears in `pg_stat_activity` query text, C-EXTENDED-AUTH-
  PRELUDE), passing the secret as the first argument. Both `PgxRunner` and
  `PgxTxRunner` implement `QueryRowBound`, so the same helper serves the
  doctor path (bare runner) and the token-create/rotate transaction
  (`carryPrincipalAcrossRotation`, `attributeClientToPrincipal`).
- On SQLSTATE `42883` (function absent: authority bootstrapped but bundle 6
  not yet applied): fall back to the direct SQL, preserving today's behavior.
- Any other error surfaces. After bundle 6 a direct read by a secretless
  daemon fails `42501`, which is a real misconfiguration and must be visible,
  not masked — the same posture `loadTokenForUpdate` takes today.

**Parity guarantee:** the projection SQL bodies are verbatim copies of the
current queries, and the Go decode paths (`scanTokenRecord`-style row scans,
the `PrincipalScope` JSON unmarshal) are unchanged. DTO fields before/after:
`PrincipalRef{principal_id, kind, display_name, disabled}`,
`PrincipalScope{principal_id, kind, display_name, disabled, client_count,
repositories, capabilities[{capability, repository_id, session_bound}]}`. The
`daemon doctor` principals block (`doctor_principals.go`) is untouched — it
calls `ListPrincipals` and keeps its never-fail-closed error capture, which
also gracefully degrades (block-level `errors` array) if an old binary ever
runs against a bundled database.

`client_sessions` needs no handler work: the R-phase confirmation requested by
the dogfood context is done — `grep` over `go/` finds no reader or writer
outside the two inventory maps.

## Guard tests

PG-gated (skip without `STRIATUM_PG_TEST_URL`), following
`read_authority_inventory_pg_test.go` / `authority_enforcement_pg_test.go`
conventions; all apply owner bundles first via `db.ApplyOwnerBundles`.

Existing, unchanged or auto-extended:

- `TestReadAuthorityInventoryComplete` — unchanged; fails if a future table
  lands unclassified.
- `TestReadDeniedTablesHaveNoRuntimeSelect` — auto-covers `client_sessions`
  once reclassified `runtime_select_denied`.
- `TestReadDeniedColumnsHaveNoRuntimeSelect` — auto-covers
  `principal_clients.principal_id` via the `RuntimeDeniedReadColumns`
  extension; add the positive control that `principal_clients.client_id`
  stays selectable (mirroring the existing `clients.client_id` control).

New, in `go/pkg/db/read_authority_inventory_pg_test.go`:

- `TestProjectionReadTablesHaveNoRuntimeSelect` — every
  `runtime_projection_read` table returns
  `has_table_privilege('striatumd_rw', ..., 'SELECT') = false`.
- `TestClientSessionsRetainRuntimeDML` — INSERT/UPDATE/DELETE privileges
  remain true for all three tables: this RFC narrows reads only.
- `TestOwnerTransferClosesSelfRegrant` — creates a scratch table owned by
  `striatumd_rw`, revokes its own SELECT, proves the runtime role can
  re-grant itself (Option B refutation, executable); then transfers ownership
  and proves the self-re-grant now fails. This is the mechanism proof pgtest
  would otherwise mask, because the pgtest pool user — not `striatumd_rw` —
  owns the real tables there, making the bundle's `ALTER ... OWNER TO
  CURRENT_USER` a no-op in CI.
- `TestIdentityGateTablesNotRuntimeOwned` — `pg_tables.tableowner <>
  'striatumd_rw'` for the three tables after bundles (vacuously green in
  pgtest; the live-prod equivalent is the doctor ownership probe below).

New, in `go/pkg/db/authority_enforcement_pg_test.go`:

- `TestPrincipalProjectionsRequireDaemonAuthority` — each of the three
  functions raises via `assert_daemon_authority()` under a wrong/absent
  secret and returns rows under the correct one.
- `TestReassertReadRevokesReclosesIdentitySelect` — extends the existing
  drift test at `:647`: owner re-grants broad SELECT on `principals` /
  `principal_clients` / `client_sessions`, `ReassertReadRevokes` re-closes
  all three (and still re-closes the 0005 `clients` columns).

New, in `go/pkg/admin` PG tests:

- `TestPrincipalReadProjectionParity` — seeds principals, links, and
  capability grants; asserts `ListPrincipals` and
  `ResolvePrincipalForClient` return identical DTOs through the direct path
  (secretless) and the projection path (authority secret set) on a bundled
  database.
- `TestCreatePrincipalSemanticsAfterReadClose` — as `striatumd_rw` on a
  closed database, all three `CreatePrincipal` behaviors survive the
  projection split: a fresh principal insert succeeds (plain `INSERT` needs
  no `SELECT`), an idempotent same-definition replay returns success, and a
  conflicting redefinition is refused. The idempotency pre-check now reads
  through `get_principal`, so this test is where a full `SELECT` denial on
  `principals` could otherwise hide a projection/direct-SQL split (review
  finding F1).
- `TestLinkUpsertAndUnlinkSurviveReadClose` — as `striatumd_rw` on a closed
  database: link, idempotent re-link, conflicting-principal refusal,
  unlink-then-revive, and `unlinkClientFromPrincipals` all succeed. Pins the
  ON CONFLICT privilege assumption; if PostgreSQL ever demands more, this is
  the test that goes red (contingency: Open Question 4).
- `TestTokenRotationCarriesPrincipalAfterReadClose` — end-to-end
  `carryPrincipalAcrossRotation` on a closed database: rotation re-attributes
  the new client and unlinks the old one.

New, in `go/pkg/reads`:

- `TestPgReadScopePostureDerivation` — posture is `broad_runtime_select` with
  only the 0005 stamp; flips to `partial_projection_gated` when
  `identity_projection_read` is stamped and probes verify; reports drift (and
  downgrades) when a closed surface is re-granted or a gated table is
  runtime-owned.

## Doctor posture transition

`pgReadScopeDoctorBlock` becomes `pgReadScopeDoctorBlock(ctx, runner)` and
derives the posture from three inputs instead of a constant:

1. **Stamps** — `SELECT capability, bundle_version FROM
   striatumd.schema_authority` (a `runtime_parity_select` table, readable by
   the runtime role precisely for this kind of capability parity).
2. **A static gate map in Go** (the doctor analog of `readScopeReasserts`):
   `auth_projection_read` → column gate `{clients: token_hash, token_salt}`;
   `identity_projection_read` → table gates `{principals, client_sessions}`
   + column gate `{principal_clients: principal_id}`.
3. **Live probes** — for each stamped gate, `has_table_privilege` /
   `has_column_privilege('striatumd_rw', ..., 'SELECT')` must be false, and
   for table gates `pg_tables.tableowner <> 'striatumd_rw'` (the ownership
   probe that catches a missed/undone transfer, which privilege probes alone
   cannot, since a runtime-owned table is one self-`GRANT` from open).

Posture rules:

- any stamped gate whose probe fails → `posture: "broad_runtime_select"` plus
  a `grant_drift` array naming the failing surfaces and the remedy
  (`striatum daemon owner-ddl apply` re-runs `ReassertReadRevokes`);
- else if every `runtime_sensitive_select` table is covered by a verified
  table-level gate → `private_read_denial` (unreachable until R2/R3; kept in
  the rule so the graduation is mechanical, not editorial);
- else if at least one verified **table-level** gate exists →
  `partial_projection_gated`;
- else (column gates only — today's 0005-only state) →
  `broad_runtime_select`, preserving the current reported posture exactly
  until bundle 6 actually applies.

`private_read_denial` stays `false` (it is `posture == "private_read_denial"`).
The `partial_projection_gates` array grows from one hard-coded entry to the
per-surface derivation: `clients` (columns, bundle 5), `principals` (table,
bundle 6), `principal_clients` (column `principal_id`, bundle 6),
`client_sessions` (table, bundle 6), each with `stamped`, `verified`, and
`owner_ok` booleans so the posture is inspectable claim-by-claim (RFC 0113 §4).

## Rollout + verification

**None of this is executed in this design run.** Sequencing for the
implementation PR and its deployment:

1. Land the PR: bundle file, `owner.go` bookkeeping + `ReassertReadRevokes`
   extension + `owner-ddl apply` reassert wiring, dual-path handlers,
   inventory reclassification, doctor derivation, all guard tests. CI pgtest
   proves everything except the production ownership transfer (masked per
   above; compensated by `TestOwnerTransferClosesSelfRegrant` and the doctor
   ownership probe).
2. Deploy the binary **before** the bundle: `make install` then
   `systemctl --user restart striatumd` (install does not restart the running
   daemon; verify the running image via `/proc/<pid>/exe`). The new binary on
   the un-bundled database takes the 42883/secretless fallbacks everywhere;
   doctor still reports `broad_runtime_select` with the bundle-6 gate listed
   as un-stamped.
3. Apply out-of-band as the owner: `striatum daemon owner-ddl apply` →
   bundle 6 applies atomically and the reasserts run. The owner DSN role must
   satisfy the three preconditions in §“The ownership constraint” (membership
   or superuser, plus `CREATE` on schema `striatumd`); a precondition failure
   rolls the bundle back unstamped and the operator grants the missing
   privilege and re-applies. (Reverse order — bundle before binary — leaves
   an old binary whose direct principal reads fail `42501`: doctor principals
   degrades to its `errors` array and rotation-with-attribution fails until
   the restart. The order above avoids any such window; no daemon stop is
   required for the apply itself.)
4. Verify posture: `striatum daemon doctor` → `pg_read_scope.posture =
   "partial_projection_gated"`, all bundle-6 gates `stamped+verified+owner_ok`,
   principals block unchanged shape.
5. Verify negatives as the runtime role (psql with the runtime DSN):
   `SELECT ... FROM striatumd.principals / client_sessions` and
   `SELECT principal_id FROM striatumd.principal_clients` → SQLSTATE `42501`;
   positive control `SELECT client_id FROM striatumd.principal_clients`
   succeeds.
6. Live parity drill: `daemon.token.rotate` with principal attribution; doctor
   principals block before/after shows the same fields; audit attribution
   intact.
7. Drift drill: as owner, `GRANT SELECT ON striatumd.principals TO
   striatumd_rw`; doctor reports `grant_drift`; re-run `owner-ddl apply`;
   doctor verifies re-closed.

## Acceptance

- All three tables closed per the matrix above; raw runtime-role direct reads
  fail `42501`; the `principal_clients.client_id` positive control passes.
- Every projection call succeeds only with daemon authority; wrong/absent
  secret raises through `assert_daemon_authority()`.
- Handler parity: `ListPrincipals`, `ResolvePrincipalForClient`,
  `CreatePrincipal` (fresh insert, idempotent replay, conflicting
  redefinition refusal), `LinkClientToPrincipal`,
  `unlinkClientFromPrincipals`, and token rotation with attribution return
  the same DTO fields / behavior on closed and un-adopted databases (the
  named tests).
- Doctor posture is derived, flips to `partial_projection_gated` only when
  bundle 6 is stamped and probes verify, detects grant and ownership drift,
  and `private_read_denial` remains `false`.
- `striatum daemon owner-ddl apply` re-asserts read+write revokes (drift
  repair has an operator verb).
- `docs/reference/spec.md`, `docs/how-to/postgres-transition.md` (owner-table
  trap now covering the three tables), and the decision log update only when
  the posture lands.

## Open questions / revisit triggers

1. **`client_capabilities`** is the last R1 table; it is already owner-held
   (no transfer needed) but is read by the `auth_pg.go` direct fallback and
   the `ListPrincipals` grants CTE. Proposed as RFC 0114's immediate
   successor (stamp `capability_projection_read`); completing it closes R1.
2. **Graduation to `private_read_denial`** is mechanical per the doctor rule:
   it requires R2 (prose/workflow tables) and R3 (artifact/event metadata)
   from RFC 0113 §3 — all runtime-owned, all needing the ownership-transfer
   precedent this RFC establishes. Revisit after R1 completes.
3. **Vestigial `client_sessions`:** no Go consumer exists. Should a later
   owner bundle drop the table instead of fencing it? Until decided, the
   revisit trigger is: any new session-tracking consumer must read through a
   new daemon-authorized projection, never via a re-grant.
4. **ON CONFLICT privilege assumption:** if
   `TestLinkUpsertAndUnlinkSurviveReadClose` shows PostgreSQL requiring
   `SELECT` beyond the documented expression/condition rule, the contingency
   is an owner-owned `SECURITY DEFINER`
   `link_client_to_principal(p_daemon_secret, p_principal_id, p_client_id)`
   write function absorbing the upsert (and, symmetrically,
   `unlink_client_from_principals`), with the Go functions becoming thin
   callers. This widens the bundle but changes no external behavior.
5. **Projection/JSON coupling:** `list_principal_scopes` freezes the
   `PrincipalScope` aggregate shape into owner DDL; evolving the DTO requires
   a `CREATE OR REPLACE` in a later bundle. Acceptable for a doctor-only
   read, or should the projection return normalized rows and let Go
   aggregate? (Current design: keep the verbatim aggregate for parity.)
6. **`repositories.repo_root` sensitivity** (RFC 0113 Open Question 3): R2 vs
   R3 placement — unchanged by this RFC, carried forward.
7. **Guarding the trap:** should a lint/guard refuse runtime migrations that
   reference the owner-held table set (now growing beyond `clients` /
   `client_capabilities`), making the RFC 0079 §5 trap structurally
   unhittable rather than documented? Candidate follow-up to file with the
   implementation PR.
