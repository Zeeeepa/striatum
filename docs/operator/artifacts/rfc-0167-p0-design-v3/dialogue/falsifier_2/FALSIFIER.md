# FALSIFIER - RFC 0167 P0 v3 C2prime composed read-scope challenge

author: falsifier-reviewer-004

## Claim Challenged

C2prime / A33-A34 claims the v3 SPEC genuinely closes the runtime-readable operator-session `client_id -> principal_id` mapping by changing `operator_sessions` from table-wide SELECT to column-scoped SELECT that excludes both `principal_id` and `client_id`, while preserving full SELECT on `operator_handles` and the current token/capability substrate.

## Material Challenge

The direct `operator_sessions` fix is real, but the negative control is incomplete. The SPEC's required failing query is only:

```sql
SELECT client_id, principal_id
  FROM striatumd.operator_sessions
 WHERE state = 'active';
```

That can fail with `42501` exactly as v3 intends and still leave `striatumd_rw` able to recover the same `client_id -> principal_id` mapping through a composed route:

```sql
SELECT DISTINCT cc.client_id, oh.principal_id
  FROM striatumd.client_capabilities cc
  JOIN striatumd.operator_handles oh
    ON oh.leased_session_id = cc.session_id
   AND (cc.repository_id IS NULL OR cc.repository_id = oh.repository_id)
 WHERE cc.session_id IS NOT NULL
   AND cc.revoked_at IS NULL
   AND oh.released_at IS NULL;
```

The v3 SPEC creates both sides of that join. It proposes an operator-session token whose `client_capabilities` rows each carry `session_id = operatorSessionID` (`docs/operator/artifacts/rfc-0167-p0-design-v3/dialogue/holder/HOLDER.md:204-218`, `:888-891`). The RFC/v3 handle model stores `operator_handles.principal_id` and `operator_handles.leased_session_id` (`docs/rfcs/0167-operator-identity-and-run-attribution.md:106-115`; `HOLDER.md:470-478`). The v3 bundle then grants full `SELECT, INSERT, UPDATE` on `operator_handles` to `striatumd_rw`, explicitly arguing that full-table SELECT is safe because the table has no `client_id` column (`HOLDER.md:650-654`, `:682-688`).

Current source leaves the other join edge readable to the runtime role. Runtime migration 0005 grants `SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw` (`go/pkg/db/sql/0005_repo_local_workflow_state.sql:467-472`). `client_capabilities` contains `client_id` from baseline (`go/pkg/db/sql/0001_baseline.sql:57-65`) and receives nullable `session_id` in migration 0022 (`go/pkg/db/sql/0022_session_bound_capability.sql:11-15`). Owner bundle 0005 narrows `clients` table SELECT only (`go/pkg/db/sql/owner/0005_token_read_scope.sql:159-166`); owner bundle 0006 closes `client_sessions`, `principals`, and `principal_clients`, with `principal_clients.principal_id` withheld for the explicit reason that a leaked runtime credential should see client ids and timestamps, not whose credentials they are (`go/pkg/db/sql/owner/0006_identity_read_scope.sql:204-220`). It does not revoke `client_capabilities`, and the current read-authority inventory still classifies `client_capabilities` as a sensitive broad SELECT surface (`go/pkg/db/read_authority_inventory.go:52-56`).

So `operator_sessions.client_id` and `operator_sessions.principal_id` are not needed for the leak. `client_capabilities` supplies `client_id -> operator_session_id`; `operator_handles` supplies `operator_session_id -> principal_id`; together they reconstruct `client_id -> principal_id` for active operator sessions. This is exactly the C2prime threat model: can `striatumd_rw` recover the mapping by any route, not merely by selecting both columns from the same table.

The A33 test as written would produce a false clearing signal. It proves the obvious v2 grant-back is gone, but it does not prove the ACL graph is closed. A34's positive control can still pass at the same time, because create / heartbeat / close / run stamp / `whose` / `status --mine` do not require this composed route to be closed.

## Strongest Rebuttal I Can Justify

The v3 change is a substantive improvement over v2. The direct `operator_sessions` table-wide SELECT grant is gone, and the direct `SELECT client_id, principal_id FROM operator_sessions` control should fail under the proposed column grant. It is also fair that `operator_handles.principal_id` by itself is not a credential mapping, and `operator_handles` must remain readable enough for `whose` and status rendering.

But C2prime was framed as a read-scope closure over any recoverable `client_id -> principal_id` path. Once the operator-token design stores the operator session id in `client_capabilities.session_id`, full SELECT on `operator_handles` turns `leased_session_id` into a bridge from runtime-readable client ids to runtime-readable principals. Splitting the pair across two runtime-readable tables does not preserve the bundle-0006 guarantee.

## Unanswered Gap / Refuting Test

Add a C2prime negative control under `striatumd_rw` for the composed route above. The test should require one of three outcomes:

1. the query fails `42501` because at least one identity-bearing join column is not runtime-readable;
2. the query is structurally impossible because the runtime role only reaches the needed data through SECURITY DEFINER projections; or
3. the query returns no identity-bearing rows for active operator sessions.

A buildable fix only needs to close one join edge. Options include column-scoping `operator_handles` direct SELECT to withhold `principal_id` and/or `leased_session_id` while rendering `whose` through a SECURITY DEFINER projection, moving token/session lookup behind the existing authorization projection so `client_capabilities.session_id` is not broadly readable, or adding a dedicated projection for the precise liveness/rendering reads P0 needs.

I did not find a separate carry-forward regression in R1a honesty, the R1b handle architecture, R1c guarded heartbeat, bundle ordinal 0021 / forward-only / watermark ordering, the four R3 open questions, or the v2 C1/C2 direct-stamp mechanisms. The standing regression is narrower but gate-critical: R4's RFC-0114 read-scope reuse is still punched through by the composed `client_capabilities -> operator_handles` path.