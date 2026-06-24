# FALSIFIER -- RFC 0167 P0 v2 C2 / carry-forward challenge

author: falsifier-reviewer-004

## Claim Challenged

C2 / A28 claims the v2 SPEC now respects the RFC 0114 identity read-scope boundary: the run-origin stamp routes through `admin.ResolvePrincipalForClient` / `striatumd.resolve_principal_for_client`, no direct `principal_clients.principal_id` read remains, and no `SELECT(principal_id)` grant-back is needed. The carry-forward claim also says R1a/A5 read surfaces remain pure PostgreSQL joins that cannot lie.

## Material Challenge

The run-origin stamp path is repaired, but the v2 read-surface design still violates the same two-role read-scope boundary. The SPEC's authoritative `whose <run-id>` query direct-joins `striatumd.principals`:

```sql
SELECT r.run_id, r.state, r.created_by_principal_id,
       oh.handle AS origin_handle, p.principal_kind, p.disabled_at
  FROM striatumd.runs r
  LEFT JOIN striatumd.operator_handles oh ON oh.handle_id = r.created_by_handle_id
  LEFT JOIN striatumd.principals       p  ON p.principal_id = r.created_by_principal_id
 WHERE r.repository_id = $1 AND r.run_id = $2;
```

That query is not callable by the runtime role after owner bundle 0006. The bundle explicitly `REVOKE SELECT ON striatumd.principals FROM striatumd_rw` and grants runtime access through `SECURITY DEFINER` projections instead: `get_principal(text, text)` and `resolve_principal_for_client(text, text)` both have `GRANT EXECUTE ... TO striatumd_rw` (`go/pkg/db/sql/owner/0006_identity_read_scope.sql:56-79,180-186,215-221`). `ReassertReadRevokes` restates the same closure (`go/pkg/db/owner.go:454-468`). The source-side helper `admin.queryIdentityRow` likewise treats `principals` / `principal_clients` as RFC 0114 gated identity surfaces that must go through the projection when daemon authority is available (`go/pkg/admin/principals.go:35-63`).

So the C2 fix is only half-applied. The stamp no longer reads `principal_clients.principal_id`, and the projection grant is real, but the P0 payoff surface still issues a direct read against another bundle-0006 closed table. Under the two-role fixture, `whose` fails `42501` unless the implementation either grants `SELECT` on `principals` back to `striatumd_rw` (which reopens RFC 0114) or rewrites the read to use the projection. The SPEC does not name that rewrite, and its named C2 pgtest checks the stamp path only, not the read surface.

This is a carry-forward regression against R1a/A5 and R4: the read surface is advertised as authoritative and source-of-truth-only, but the stated SQL is not executable inside the accepted read-scope model. A downstream build following §2.4 literally will either fail the two-role fixture or punch a new read-scope hole to make it pass.

## Strongest Rebuttal I Can Justify

This is fixable without weakening C2. `whose` does not need a direct table join to `principals`; it can either render from `runs.created_by_principal_id` plus `operator_handles.handle` and compute the suffix locally, or fetch `principal_kind` / `disabled_at` through `striatumd.get_principal` under daemon authority. That keeps the v2 stamp repair intact and preserves the RFC 0114 closure.

But that is not what the published SPEC says. It explicitly prints the direct `LEFT JOIN striatumd.principals` query, lists `operator_handles` / `operator_sessions` runtime grants, and only calls out `resolve_principal_for_client` for the stamp. There is no named two-role test proving `whose` itself survives bundle 0006's `principals` read revoke.

## Unanswered Gap / Refuting Test

Add a two-role read-surface gate, e.g. `whose_uses_identity_projection_under_read_scope`:

1. As `SUTPool`, a control `SELECT principal_id FROM striatumd.principals LIMIT 1` must fail `42501`.
2. The real `whose <run-id>` query/handler must still succeed for a stamped run without granting table `SELECT` on `principals`.
3. The implementation must prove how it gets `principal_kind` / disabled-state data: via `get_principal` projection, or by removing those fields from the direct SQL.

Until that test exists and the §2.4 SQL is reconciled with bundle 0006, C2 cannot clear cleanly: the run stamp is projection-routed, but the P0 authoritative read surface is still a direct closed-table read.
