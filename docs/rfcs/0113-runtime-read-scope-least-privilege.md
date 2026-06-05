# RFC 0113: Runtime read-scope least privilege

Status: partially implemented (R1 token-secret projection landed)
Date: 2026-06-05
author: proposer-codex-gpt-5-001
Context: RFC 0110 (daemon PostgreSQL authentication and database-enforced write
boundary), RFC 0107 (multi-principal trust model), RFC 0096 (supervised-lane
trust boundary), GH #164; `go/pkg/db/read_authority_inventory.go`,
`go/pkg/reads/doctor_pg_read_scope.go`, `go/pkg/db/sql/owner/`.

## Problem

RFC 0110 closed the durable write-path claim in PostgreSQL: `audit_log`,
`artifacts`, and `events` are written through owner-owned
`SECURITY DEFINER` functions that assert the daemon-authority secret. It
deliberately did **not** close private reads. The runtime role still holds broad
`SELECT` on most daemon-owned tables so production read handlers can operate.

That means a leaked live runtime credential can read sensitive workflow and
identity material even though it can no longer forge protected writes. Current
`daemon doctor` is honest about this:

```json
{
  "pg_read_scope": {
    "posture": "broad_runtime_select",
    "private_read_denial": false,
    "partial_projection_gates": [
      {
        "surface": "clients",
        "denied_columns": ["token_hash", "token_salt"],
        "owner_bundle": 5,
        "authority_stamp": "auth_projection_read"
      }
    ]
  }
}
```

#164 is the named successor for reducing that read surface to a documented
least-privilege minimum.

## Goals

1. Make the runtime read surface table-scoped and guard-tested before changing
   privileges.
2. Reduce direct runtime `SELECT` on sensitive tables without breaking the
   daemon's read handlers.
3. Reuse RFC 0110's daemon-authority model for sensitive reads: a raw runtime
   DSN is not enough to read protected surfaces.
4. Keep claims keyed to a doctor posture string. `private_read_denial` remains
   `false` until all sensitive direct reads are closed.
5. Preserve local-first operation: no hosted service, external identity provider,
   telemetry, transcript capture, or external persistence.

## Non-Goals

- No broad `REVOKE SELECT` patch that breaks production reads.
- No standing broad `striatumd_read` credential as the final answer. A separate
  read role can be a migration helper, but if it retains broad sensitive SELECT,
  it merely renames the leaked-live-credential problem.
- No row-level security as the primary authority boundary. RLS may become
  defense-in-depth after a read is already daemon-authorized.
- No change to artifact contents, evidence exports, or corpus exports.

## Proposal

### 1. Read-authority inventory

Every `striatumd.*` table gets a `ReadAuthorityClass` in
`go/pkg/db/read_authority_inventory.go`:

- `runtime_sensitive_select` — broad SELECT currently exists and can expose
  user/agent prose, repository paths, token-adjacent data, principal/session
  identity, or private workflow metadata.
- `runtime_operational_select` — broad SELECT currently exists for operational
  metadata and chain pointers.
- `runtime_parity_select` — owner-maintained table deliberately readable for
  startup capability parity.
- `runtime_select_denied` — runtime role must not hold SELECT.

A PG-gated guard lists `information_schema.tables` and fails when a table lacks
a read classification. A second guard asserts `runtime_select_denied` tables
remain unreadable by `striatumd_rw`.

This is only a map of the current posture. It is not a private-read claim.

### 2. Daemon-authorized read projections

For `runtime_sensitive_select` tables, replace direct handler SELECTs with
owner-owned `SECURITY DEFINER` projection functions or narrow views that begin
with the same `assert_daemon_authority()` gate used by write functions.

The daemon reads sensitive surfaces through a single authorized read wrapper
that sets the RAM-only daemon-authority secret over the extended protocol, plus
the existing attribution labels. A raw `striatumd_rw` connection that lacks the
secret cannot execute the projection and no longer has direct table SELECT once
that table's phase closes.

Projection shape is table-specific:

- Token and principal surfaces return only fields handlers need, never token
  hashes/salts or revoked secret material unless a handler has a documented
  reason.
- Packet/prose surfaces return the existing DTO fields needed by status, why,
  dashboard, evidence, archive, and artifact reads.
- Raw JSON fields are allowed only when the caller surface already exposes that
  JSON today and the field is covered by the explicit redaction/export policy.

### 3. Phased closure

Close sensitive reads in table groups so each phase can prove handler parity and
negative privileges before the next group:

| Phase | Scope | Examples | Doctor posture |
|---|---|---|---|
| R0 | Inventory only | all tables classified, denied owner-only reads pinned | `broad_runtime_select` |
| R1 | Token/principal privacy | `clients`, `client_capabilities`, `client_sessions`, `principals`, `principal_clients` | `partial_projection_gated` |
| R2 | Work-packet and prose-heavy workflow data | `work_packets`, `queue_messages`, `blockers`, `verdicts`, `conversations`, `interrogations`, `escalation_inbox`, `trajectory_segments` | `partial_projection_gated` |
| R3 | Artifact/event/workflow metadata | `artifacts`, `events`, `workflow_snapshots`, `jobs`, `runs`, `sessions`, `repositories` | `private_read_denial` only if no `runtime_sensitive_select` table remains directly selectable |

R1 has started with the narrowest high-value reduction: owner bundle 0005
revokes direct runtime `SELECT` on `striatumd.clients.token_hash` and
`striatumd.clients.token_salt`. Token authorization and token-for-update secret
reads now route through owner-owned `SECURITY DEFINER` functions guarded by
`assert_daemon_authority()`. The rest of R1 remains open.

Operational tables may stay directly selectable when they carry no sensitive
workflow or identity material, but their classification must stay explicit.

### 4. Doctor posture

`pg_read_scope` remains separate from `pg_write_boundary`.

Postures:

- `broad_runtime_select` — current no-claim state. Runtime role retains broad
  direct SELECT. `private_read_denial=false`.
- `partial_projection_gated` — at least one sensitive group has revoked direct
  SELECT and moved behind daemon-authorized projections, but others remain
  broad. `private_read_denial=false`.
- `private_read_denial` — no table classified `runtime_sensitive_select` remains
  directly selectable by the runtime role; sensitive reads require daemon
  authority. `private_read_denial=true`.

Doctor should report each sensitive table with its class and whether direct
runtime SELECT is still present, so the posture is inspectable rather than a
single unverifiable string.

## Acceptance

- All daemon-owned tables are classified by `ReadAuthorityClass` and guarded by a
  PG-gated completeness test.
- For each closed phase, a raw runtime-role connection gets SQLSTATE `42501`
  when directly selecting protected tables.
- Authorized projection calls succeed through the same daemon wrapper production
  handlers use.
- Handler parity tests prove status, why, dashboard, artifact reads, evidence
  export, archive, token/principal administration, and recovery reads still
  return the same allowed DTO fields after each table group closes.
- Projection tests prove token hashes/salts and other explicitly denied fields
  are not returned.
- `daemon doctor` reports `partial_projection_gated` during partial rollout and
  `private_read_denial=true` only after every sensitive direct SELECT is gone.
- `docs/reference/spec.md`, `docs/how-to/postgres-transition.md`, and
  `docs/decisions/decision-log.md` update claims only when the matching posture
  lands.

## Open Questions

1. Should R1 use table-specific functions only, or are owner-owned projection
   views acceptable when they can call an authority-checking function safely?
2. Should `artifacts` and `events` expose their metadata through the same
   write-boundary SD functions' companion read functions, or through a separate
   read-projection package?
3. Is `repositories.repo_root` sensitive enough to move in R2 rather than R3?
4. Should a future broad read-only role exist solely for offline maintenance
   tools, and if so, how is it kept outside the live daemon/lane credential
   model?
