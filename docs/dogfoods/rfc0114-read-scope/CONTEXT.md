# RFC 0114 dogfood — operator-supplied source context

This dogfood produces the **successor design RFC to RFC 0113 R1** (GH #164):
the next runtime read-scope least-privilege expansion beyond the landed
token-secret column gate.

You (the panel) are designing **design only**. No production schema change,
owner bundle, or daemon restart lands from this run. The deliverable is an RFC
document.

## What already landed (the precedent you extend)

RFC 0113 R1 first step (commits `81b575ea`, `51990f52`) gated only the
**token-secret columns** on the owner-held `clients` table:

- `go/pkg/db/sql/owner/0005_token_read_scope.sql` — owner bundle 0005:
  - Creates owner-owned `SECURITY DEFINER` functions
    `striatumd.authorize_capability(...)` and `striatumd.load_token_for_update(...)`,
    each beginning with `PERFORM striatumd.assert_daemon_authority()` (the same
    RAM-only daemon-authority secret gate RFC 0110 uses for write functions).
  - `REVOKE SELECT ON striatumd.clients FROM striatumd_rw;` then
    `GRANT SELECT (client_id, client_kind, display_name, token_id, created_at,
    expires_at, revoked_at, last_used_at) ON striatumd.clients TO striatumd_rw;`
    — i.e. column-level grant-back of non-secret columns, denying `token_hash`
    and `token_salt`.
  - Stamps `schema_authority(capability='auth_projection_read', bundle_version=5)`.
- `go/pkg/db/owner.go`:
  - `LatestOwnerBundleVersion = 5`; `ownerBundleLabels[5]`.
  - `ReassertReadRevokes(ctx, runner)` — re-applies the clients read revoke +
    column grant-back whenever the `auth_projection_read` capability is stamped,
    so a stray GRANT cannot reopen the surface (grant-drift repair, mirrors
    `ReassertWriteRevokes`).
- `go/pkg/db/read_authority_inventory.go`:
  - `ReadAuthorityClass` enum: `runtime_sensitive_select`,
    `runtime_operational_select`, `runtime_parity_select`, `runtime_select_denied`.
  - `readAuthorityInventory` map classifies EVERY `striatumd.*` table.
    `principals`, `principal_clients`, `client_sessions`, `clients`,
    `client_capabilities` are all `runtime_sensitive_select`.
  - `RuntimeSensitiveReadTables()`, `RuntimeDeniedReadColumns()` (currently
    `{"clients": {"token_hash","token_salt"}}`).
- `go/pkg/reads/doctor_pg_read_scope.go`:
  - `pgReadScopeDoctorBlock()` reports `posture: "broad_runtime_select"`,
    `private_read_denial: false`, and a `partial_projection_gates` array with
    the one `clients` token-secret entry. **The posture string is currently
    HARD-CODED to `broad_runtime_select`** even though RFC 0113 §4 defines a
    `partial_projection_gated` posture. A successor that closes another surface
    must decide whether/how to flip this to `partial_projection_gated`.
- `go/pkg/rpc/auth_pg.go`:
  - `PostgresAuthorizer.authorizeWithProjection(...)` calls the owner-owned
    `striatumd.authorize_capability(...)` via `QueryRowBound` (passing the
    daemon-authority secret over the extended protocol). Falls back to direct
    `SELECT ... FROM striatumd.clients` on `42883` (function absent) so a daemon
    on an un-adopted DB still works. THIS DUAL PATH (projection-preferred,
    direct-fallback) is the parity pattern your design must preserve.
- `go/pkg/admin/tokens.go` — prefers `load_token_for_update(...)` projection,
  falls back to direct read on `42501`.

## Guard tests precedent (`go/pkg/db/read_authority_inventory_pg_test.go`)

- `TestReadAuthorityInventoryComplete` — lists `information_schema.tables` for
  schema `striatumd`; fails if any table lacks an inventory classification.
- `TestReadDeniedTablesHaveNoRuntimeSelect` — every `runtime_select_denied`
  table must return `has_table_privilege('striatumd_rw', ...) = false`.
- `TestReadDeniedColumnsHaveNoRuntimeSelect` — every column in
  `RuntimeDeniedReadColumns()` must return
  `has_column_privilege('striatumd_rw', ..., 'SELECT') = false`, and
  `clients.client_id` must still be selectable (proves only secrets are denied).

Live-PG tests run with
`STRIATUM_PG_TEST_URL='postgres://halbritt@/postgres?host=/var/run/postgresql'`.

## The candidate surfaces for this successor (RFC 0113 §3 table, the rest of R1)

RFC 0113 R1 phase scope is `clients, client_capabilities, client_sessions,
principals, principal_clients`. Token-secret columns on `clients` are done.
The next expansion candidates named by #164 / RFC 0113 are **principals
and/or client_sessions** (with their link tables). Pick and JUSTIFY the order.

### CRITICAL ownership constraint (the central design tension)

`clients` / `client_capabilities` are **owner-held** tables (owned by the
database owner role, not `striatumd_rw`). That is WHY the 0005 SECURITY DEFINER
projection works: an owner-owned `SECURITY DEFINER` function can read an
owner-held table after `REVOKE SELECT ... FROM striatumd_rw`, and the runtime
role cannot re-grant itself.

BUT `principals`, `principal_clients`, and `client_sessions` are created by
**runtime-role migrations** (`go/pkg/db/sql/0023_principals.sql`,
`go/pkg/db/sql/0001_baseline.sql`) and are therefore **owned by
`striatumd_rw`**. A table owner ALWAYS retains implicit privileges and can
re-grant itself SELECT — a plain `REVOKE SELECT FROM striatumd_rw` on a
table it OWNS does not lock it out. Your design MUST resolve this:

- Option A: an owner-applied owner-bundle step that `ALTER TABLE ... OWNER TO`
  the database owner role for these tables before revoking (mirrors how the
  owner-held tables work), then SECURITY DEFINER projection as in 0005. Note the
  RFC 0079 §5 owner-table trap: once owner-held, the runtime role can no longer
  ALTER them, so future schema changes to these tables must move to owner DDL.
- Option B: column/row scoping that does not depend on revoking the owner's own
  SELECT (weaker; may not actually deny a leaked live credential).
- Option C: defer principals/client_sessions and pick a different already
  owner-held surface first.

Resolve this explicitly; it is the load-bearing decision of the RFC.

### Sensitivity of the candidate columns

- `principals(principal_id, principal_kind, display_name, created_at, disabled_at)`
  — `display_name` is human/operator identity prose. No secret columns.
- `principal_clients(principal_id, client_id, linked_at, unlinked_at)` —
  the identity linkage graph (who owns which credential).
- `client_sessions(client_session_id, client_id, transport, envelope_version,
  methods_etag, opened_at, last_seen_at, closed_at)` — session/identity linkage;
  `client_id` ties a live session to a credential. No secret columns; lower
  prose-sensitivity than principals; **no live Go read consumer of
  `client_sessions` was found** (only inventory/write paths), which may make it
  the cheaper first move (low parity risk).

### Read consumers you must preserve parity for

- `principals`: `admin.ListPrincipals(ctx, runner)` in
  `go/pkg/admin/principals.go` (joins principals + principal_clients +
  client_capabilities) is the live read path, surfaced by the `daemon doctor`
  principals block (`go/pkg/reads/doctor_principals.go`). It already never reads
  token secrets. Your projection must return the same DTO fields.
- `client_sessions`: no live Go read consumer found — confirm during R-phase.

## Doctor posture transition (RFC 0113 §4)

- `broad_runtime_select` — current.
- `partial_projection_gated` — at least one sensitive group revoked direct
  SELECT behind daemon-authorized projection, others still broad.
  `private_read_denial=false`.
- `private_read_denial` — only when NO `runtime_sensitive_select` table remains
  directly selectable.

Today `pgReadScopeDoctorBlock()` hard-codes `broad_runtime_select`. Your RFC
should specify that closing another surface flips the posture string to
`partial_projection_gated` (and how doctor computes it — e.g. derive it from the
stamped projection-read capabilities / denied surfaces rather than a constant).

## Constraints (hard)

- Design only. No owner bundle is applied to a live DB, no daemon restart, no
  `go/` change in THIS run. The RFC describes the plan; implementation is a
  later PR.
- Local-first: no hosted service, external IdP, telemetry, transcript capture,
  or external persistence (RFC 0113 Goal 5).
- No broad `REVOKE SELECT` that breaks production reads (RFC 0113 Non-Goal).
- No standing broad `striatumd_read` role as the final answer (RFC 0113
  Non-Goal); RLS is at most defense-in-depth after a read is daemon-authorized.

## Deliverable shape

The RFC body must include, at minimum:
1. Problem + scope (which surface(s), and the justified ordering).
2. Resolution of the ownership constraint (Option A/B/C with rationale).
3. Owner-bundle plan (bundle 0006 contents: ALTER OWNER if needed, SECURITY
   DEFINER projection function(s), REVOKE/GRANT, schema_authority stamp,
   `LatestOwnerBundleVersion` bump, `ReassertReadRevokes` extension).
4. Daemon read-handler changes (projection-preferred + direct-fallback dual
   path, mirroring auth_pg.go) and the parity guarantee.
5. Guard tests (inventory completeness already covers it; add denied-table /
   denied-column negatives + projection-success + handler-parity tests; name
   them).
6. Doctor posture transition (flip to `partial_projection_gated`; how computed).
7. Rollout + verification steps (owner-applied out-of-band, doctor check,
   pgtest privilege negatives, daemon restart sequencing) — explicitly NOT done
   in this design run.
8. Revisit triggers / open questions (what graduates the posture to
   `private_read_denial`, deferred surfaces, the RFC 0079 owner-table trap
   consequence for future migrations of these tables).

Use the RFC house style of the existing `docs/rfcs/0113-*.md`.
