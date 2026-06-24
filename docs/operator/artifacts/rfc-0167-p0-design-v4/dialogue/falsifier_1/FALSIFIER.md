# FALSIFIER — RFC 0167 P0 design v4 C2 double-prime challenge

author: falsifier-reviewer-001

## Claim Challenged

The v4 holder says C2 double-prime is closed over the full ACL graph: every runtime-readable principal-bearing surface is enumerated, Route 1 (`client_capabilities` joined to `operator_handles`) is closed by excluding `operator_handles.principal_id`, Route 2 (`client_capabilities` joined through `operator_handles` to `runs`) is closed by excluding `runs.created_by_principal_id`, and there is no third route. The specific challenged claims are A37/A38 and the table at `docs/operator/artifacts/rfc-0167-p0-design-v4/dialogue/holder/HOLDER.md:216-229`, plus the controls at `HOLDER.md:807-815`, which test only the two named composed routes and the converted `runs` star-readers.

## Material Challenge: `spawn_authorization_grants.owner_principal_id` Is An Unhandled Third Route Or An Unresolved Attribution Fork

The SPEC's third-route enumeration omits the existing RFC 0122 scheduler grant table. That table is not hypothetical P1 custody: it is live source, runtime-readable, and keyed by `run_id`. Migration 0027 creates `striatumd.spawn_authorization_grants` with `run_id` and `owner_principal_id` (`go/pkg/db/sql/0027_spawn_authorization_grants.sql:37-42`) and grants table-wide `SELECT, INSERT, UPDATE` to `striatumd_rw` (`0027_spawn_authorization_grants.sql:57-61`). Owner bundle 0018 then transfers `spawn_authorization_grants` into the runtime-owned cohort (`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:80-103`). Current code captures this row atomically when an auto-spawn run starts (`go/pkg/mutations/run.go:144-153`) and the scheduler later reads `owner_principal_id` directly (`go/pkg/mutations/scheduler.go:28-35`) and installs it as the scheduler spawn identity (`scheduler.go:239-253`).

That creates a fork the v4 SPEC does not resolve:

1. If RFC 0167 P0 makes `spawn_authorization_grants.owner_principal_id` hold an actual `principal_id`, as the column name and RFC 0122/0167 language require, then C2 double-prime has a third composed route. As `striatumd_rw`, for an auto-spawn-enabled run with an active grant, this avoids both columns the holder revokes:

```sql
SELECT DISTINCT cc.client_id, sag.owner_principal_id
  FROM striatumd.client_capabilities cc
  JOIN striatumd.operator_handles oh
    ON oh.leased_session_id = cc.session_id
  JOIN striatumd.runs r
    ON r.repository_id = oh.repository_id
   AND r.created_by_handle_id = oh.handle_id
  JOIN striatumd.spawn_authorization_grants sag
    ON sag.repository_id = r.repository_id
   AND sag.run_id = r.run_id
 WHERE cc.session_id IS NOT NULL
   AND cc.revoked_at IS NULL
   AND oh.released_at IS NULL
   AND sag.revoked_at IS NULL;
```

The selected/joined columns are all runtime-readable under the holder's own grant plan: `cc.client_id`/`cc.session_id`, `oh.leased_session_id`/`oh.handle_id`/`oh.repository_id`, `r.repository_id`/`r.run_id`/`r.created_by_handle_id`, and the current table-wide `spawn_authorization_grants.owner_principal_id`. A returned row reconstructs exactly the forbidden `client_id -> principal_id` mapping for the class of runs with auto_spawn grants. C2 double-prime is an ANY-route property; a leak for auto_spawn runs is still a leak.

This route is worse than the `runs` column in one respect: the holder's `runs` revoke relies on `runs` being owner-held and therefore not self-regrantable (`HOLDER.md:462-467`). `spawn_authorization_grants` is explicitly runtime-owned after 0018, so a plain column revoke on it would not be a stable boundary unless the build also moves that identity read behind an owner-held/projection boundary or otherwise changes ownership/read authority.

2. If the build keeps `owner_principal_id` as today's client id instead, then the route is not a `principal_id` leak, but the SPEC still fails to integrate RFC 0167 with RFC 0122. The holder acknowledges that the authority GUC currently holds a `client_id` and that run-origin principal resolution must happen via `ResolvePrincipalForClient` (`HOLDER.md:469-471`). RFC 0167, however, says auto-spawned scheduler runs should render under the same principal model (`docs/rfcs/0167-operator-identity-and-run-attribution.md:146-155`) and that every surface, including evidence export, resolves identity through `principal_id` (`docs/rfcs/0167-operator-identity-and-run-attribution.md:246-253`). Leaving a live scheduler grant column named `owner_principal_id` as a client id is therefore not a read-scope closure; it is an unmodelled exception that can keep scheduler attribution off the new principal stamp path.

The current v4 controls do not catch either branch. `composed_identity_map_unreadable` tests only Route 1 and Route 2 (`HOLDER.md:807-808`), and `whose_status_mine_via_projection` tests the new projections and the converted `runs` readers (`HOLDER.md:809-815`). Neither seeds an auto_spawn run, inspects `spawn_authorization_grants`, checks `information_schema.role_column_grants` for this existing `*principal_id*` column, nor asserts that the scheduler grant identity is projection-safe.

## Strongest Rebuttal And Why It Does Not Clear The Gate

The strongest rebuttal is that P1 custody/lineage is out of P0, and the current migration comment says `owner_principal_id` is "today, the run owner's client id" (`0027_spawn_authorization_grants.sql:18-24`), so this column might not expose a principal yet. That rebuttal is not enough for a C2 clearing verdict. The table is not P1; it is already live RFC 0122 scheduler authority state. RFC 0167's accepted context explicitly includes RFC 0122, and the P0 stamp is supposed to make autonomous scheduler-origin attribution use the same principal rules. The holder cannot both claim "No remaining runtime-readable column pairs a credential identifier with a principal_id" and omit a runtime-owned/readable table whose purpose is to replay the run owner identity.

## Required Refutation Test / Fix Shape

Add a third C2 negative control over `cc -> oh -> runs -> spawn_authorization_grants` for an auto_spawn-enabled run. The test must prove one of these, under `striatumd_rw` after the 0021 owner bundle and read-scope reassertions:

- the query above fails `42501`;
- or it returns no identity rows because at least one join edge is structurally unavailable;
- or `spawn_authorization_grants.owner_principal_id` is explicitly not a `principal_id`, with a separate positive control proving scheduler attribution still resolves to the RFC 0167 principal through a daemon-secret-gated projection.

If the intended fix is to store a real principal in the grant, the table needs the same kind of treatment as the other identity-bearing surfaces: column-scope `SELECT` excluding `owner_principal_id`, an owner-held SECURITY DEFINER scheduler projection, stable ownership/authority for the gate, and a `readScopeReasserts` entry. Without that, A37 is false or at least unproved, and the C2 double-prime gate should not clear.
