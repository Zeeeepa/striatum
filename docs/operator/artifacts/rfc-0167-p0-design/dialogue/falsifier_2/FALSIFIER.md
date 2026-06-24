# FALSIFIER - RFC 0167 P0 R2/R4 principal projection gap

author: falsifier-reviewer-002

## Gate result

Material challenge: the holder SPEC's proposed run-origin INSERT cannot run under
the current RFC 0142 two-role runtime role after owner bundle 0006. It reads
`striatumd.principal_clients.principal_id` directly, but that column is
intentionally not selectable by `striatumd_rw`. The database will return
SQLSTATE 42501 before the write-once trigger or the new `operator_handles` grants
matter.

This stops R2/R4 clearance until the SPEC changes the stamp path and tightens its
pgtest so it executes the actual server-side dereference, not a literal value.

## Challenged claim

The holder SPEC says `runs.created_by_principal_id` is resolved from the live
token at run creation with this SQL shape:

```sql
(SELECT pc.principal_id
   FROM striatumd.principal_clients pc
  WHERE pc.client_id = current_setting('striatum.principal_id', true)
    AND pc.unlinked_at IS NULL)
```

Anchors:

- `docs/operator/artifacts/rfc-0167-p0-design/dialogue/holder/HOLDER.md:69-88`
  specifies the direct `principal_clients` subquery inside the run INSERT.
- `HOLDER.md:303-312` claims the runtime role retains enough privilege for the
  run INSERT and that `owner_bundle_0021_applies_clean` proves it.
- `HOLDER.md:366-367` says the SPEC reuses `ResolvePrincipalForClient`, but the
  run INSERT does not use the projection that function uses under two-role.

## Current source facts

Current main still has `LatestOwnerBundleVersion == 20`, so holder's ordinal 21
is the next free owner-bundle ordinal. I did not find a stop on ordinal or owner
placement. The stop is the existing RFC 0114 read-scope boundary.

The authority prelude installs the needed values in every authorized mutation:
`go/pkg/db/authority.go:116-120` sets `striatum.daemon_auth`,
`striatum.principal_id`, and `app.session_id` transaction-locally. The value in
`striatum.principal_id` is the caller's client id, so a principal dereference is
indeed required.

But current owner bundle 0006 deliberately closes direct principal reads:

- `go/pkg/db/sql/owner/0006_identity_read_scope.sql:56-78` defines the
  `SECURITY DEFINER` projection `striatumd.resolve_principal_for_client(secret,
  client_id)`, which reads `pc.principal_id` as the owner after asserting daemon
  authority.
- `go/pkg/db/sql/owner/0006_identity_read_scope.sql:185-186` grants runtime
  `EXECUTE` on that projection.
- `go/pkg/db/sql/owner/0006_identity_read_scope.sql:218-221` then revokes table
  SELECT on `principal_clients` from `striatumd_rw` and grants back only
  `(client_id, linked_at, unlinked_at)`. `principal_id` is omitted on purpose.
- `go/pkg/db/owner.go:454-468` reasserts the same revokes/grants, so this is not
  accidental drift.
- `go/pkg/admin/principals.go:35-58` documents that after bundle 0006 a direct
  read by a runtime connection fails 42501, while `ResolvePrincipalForClient`
  uses the owner-owned projection when an authority secret is present.
- `go/pkg/admin/principals.go:261-282` shows the actual helper calling
  `striatumd.resolve_principal_for_client($1, $2)` before falling back to direct
  SQL only for pre-bundle/secretless cases.

## Concrete counterexample

Run this in the RFC 0142 two-role fixture after applying current owner bundles
through 20 and then the proposed 0021 bundle.

Seed as the owner role:

```sql
INSERT INTO striatumd.principals(principal_id, principal_kind, display_name)
VALUES ('prin_maya', 'human', 'Maya');

INSERT INTO striatumd.principal_clients(principal_id, client_id, linked_at)
VALUES ('prin_maya', 'client_maya', now());
```

Then execute the holder's run stamp shape as `striatumd_rw` inside an authorized
mutation transaction:

```sql
SELECT
  set_config('striatum.daemon_auth', '<real-daemon-secret>', true),
  set_config('striatum.principal_id', 'client_maya', true),
  set_config('app.session_id', 'sess_maya', true);

INSERT INTO striatumd.runs (
  repository_id, run_id, workflow_snapshot_id, repo_root, state,
  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at,
  created_by_principal_id
) VALUES (
  'repo_x', 'run_x', 'snap_x', '/tmp/repo', 'ready',
  NULL, NULL, NULL, NULL, now(),
  (SELECT pc.principal_id
     FROM striatumd.principal_clients pc
    WHERE pc.client_id = current_setting('striatum.principal_id', true)
      AND pc.unlinked_at IS NULL)
);
```

Expected by the holder SPEC: the runtime insert succeeds and stamps
`prin_maya`.

Actual under the current two-role ACL: SQLSTATE 42501, because the runtime role
has no SELECT privilege on `principal_clients.principal_id`. The failure is on
an existing RFC 0114 identity surface, not on the new `runs` columns or the new
`operator_handles` table.

A test that merely inserts a literal `created_by_principal_id = 'prin_maya'` is a
false green. It proves the new column is insertable, but it bypasses the exact
server-side dereference the SPEC claims is safe.

## Why this is R2/R4 material

R2 requires clean apply and runtime behavior under the non-superuser two-role
fixture. The proposed `owner_bundle_0021_applies_clean` must prove the real
run-origin stamp path. As written, the path either fails 42501 or can pass only
by testing a different SQL shape than the SPEC publishes.

R4 requires reusing RFC 0107/RFC 0114 machinery, not punching through it. The
safe reuse point is the existing daemon-authorized projection. Directly selecting
`principal_clients.principal_id` reopens the read surface bundle 0006 closed.
Do not fix this by granting `SELECT(principal_id)` back to `striatumd_rw`; that
would weaken the current read-scope contract.

## Repair target

Keep the stamp server-side, but route it through the existing projection inside
the authorized `run.prepare` transaction. Either of these is consistent with the
current source boundary:

1. In Go, inside the `run.prepare` authorized transaction, call
   `admin.ResolvePrincipalForClient(ctx, tx, auth.ClientID)` or the equivalent
   value from the authority context, then pass the resulting `principal_id` as a
   bound INSERT value. That helper already uses the `SECURITY DEFINER`
   projection under two-role.
2. In SQL, call the projection directly from the INSERT:

```sql
(SELECT principal_id
   FROM striatumd.resolve_principal_for_client(
          current_setting('striatum.daemon_auth', true),
          current_setting('striatum.principal_id', true)))
```

The handle subquery over `operator_handles` can remain separate, provided the
lease acquisition path also obtains `principal_id` through the same projection
rather than a direct `principal_clients` read.

## Refuting test

Replace or extend `owner_bundle_0021_applies_clean` with a two-role pgtest named
`run_origin_stamp_uses_identity_projection`:

1. Seed a principal/client active link as `OwnerPool`.
2. As `SUTPool`, assert a direct
   `SELECT pc.principal_id FROM striatumd.principal_clients pc ...` fails
   SQLSTATE 42501. This is the control proving the test is exercising the
   current read-scope boundary.
3. In an authorized runtime transaction, execute the exact run-origin stamp SQL
   the implementation will use.
4. Assert the run row stores the expected `created_by_principal_id`, and assert a
   forged envelope/request parameter cannot affect it.
5. Assert the test fails if the implementation reads `principal_clients` or
   `principals` directly instead of using `striatumd.resolve_principal_for_client`
   or `admin.ResolvePrincipalForClient`.

Until that test passes against the real stamp path, A15/A16 are not proven and
R4 reuse is only claimed, not demonstrated.