# FALSIFIER 2 — Challenge to RFC 0142 P0 Spec

author: falsifier-reviewer-002
role: falsifier
gate: fg_rfc0142_design (falsification)
scope: RFC 0142 P0 only (Layer 1a)
status: active challenge

This challenge presents three distinct lines of attack against the executable oracle design specified in `HOLDER.md`. While the concept of a two-role test fixture is the correct foundation, the bootstrap mechanics, deployment path assumptions, and self-check validations contain critical gaps that would cause both false reds and false greens in CI.

---

## Challenge 1: The Non-Superuser Ownership Transfer Privilege Failure (Broken Bootstrap)

### Claim Challenged
From `HOLDER.md` §3.2 (Phase A):
> *Phase A — bootstrap as the owner (DSN user): `db.ConnectAndMigrate` (runtime 0001-0042) then `db.ApplyOwnerBundles(..., "test")` (owner 0001-0019). This realizes the real split: owner-held tables owned by the owner; the bundle-0018/0019 cohort transferred to `striatumd_rw` ownership...*

### Counterexample / Mechanism
Under PostgreSQL security semantics, a non-superuser role (the bootstrap/owner role connected via the test DSN) cannot change the owner of an existing table to another role (via `ALTER TABLE ... OWNER TO striatumd_rw`) unless the executing role is a **member** of the target role (`striatumd_rw`).

In typical local-first developer and CI environments, the PostgreSQL DSN user is configured as a non-superuser database owner for isolation. During Phase A bootstrap, the DSN user runs `ApplyOwnerBundles` to transfer ownership. Because `pgtest.go`'s `ensureRuntimeRole()` only creates the `striatumd_rw` role but does **not** grant its membership to the DSN/owner user, PostgreSQL will reject the transfer operation:
```sql
ALTER TABLE striatumd.job_recovery_state OWNER TO striatumd_rw;
-- Raises: ERROR: must be member of role "striatumd_rw" (SQLSTATE 42501)
```
This causes the entire bootstrap phase to fail with a privilege error before any candidate migrations can be probed, rendering the executable oracle unusable under standard non-superuser DSN configurations.

### Strongest Rebuttal
The holder might argue that test environments should run as a superuser (such as `postgres` or `root`) which bypasses role membership checks during ownership transfers.

### Residual Gap
Enforcing superuser status for local or CI test execution violates least-privilege principles and introduces environmental fragility. The P0 spec must require that Phase A bootstrap explicitly executes a temporary grant of `striatumd_rw` to the DSN/owner role (e.g. `GRANT striatumd_rw TO CURRENT_USER`) before invoking the owner bundles, and revokes it afterwards, to ensure a non-superuser owner can successfully complete the bootstrap.

### Severity
Fixable gap (Severity: Medium, binding constraint).

---

## Challenge 2: Dual-Deploy Path Ownership Divergence (False Green for Admin-Deploys)

### Claim Challenged
From `HOLDER.md` §3.2 (Phase B):
> *Phase B — system-under-test as `striatumd_rw` ... apply the candidate runtime migration...*
And §1's core claim:
> *"A two-role pgtest fixture ... will red the PR for exactly the migrations that would `42501` in prod, and green for the ones that wouldn't..."*

### Counterexample / Mechanism
In production, Striatum supports two separate deployment pathways for runtime migrations:
1. **Boot-time Auto-Migration**: The daemon starts up, connects as the runtime role `striatumd_rw`, and applies pending migrations (which creates new relations owned by `striatumd_rw`).
2. **Out-of-band Admin Migration**: The operator runs `striatum daemon migrate-db` connecting via the owner/admin DSN (which creates new relations owned by the **owner/admin role**).

If a candidate runtime migration creates a table, and then a subsequent statement (in the same or a future runtime migration) attempts to `ALTER` or drop that table:
- **In the test fixture**: Phase B executes all candidate migrations as `striatumd_rw`. Consequently, `striatumd_rw` creates the table and owns it. The subsequent `ALTER` succeeds, yielding a **green** verdict.
- **In production**: If the operator applies the migrations out-of-band using `migrate-db` (owner/admin DSN), the table is created by and owned by the owner role. When the daemon restarts (running as `striatumd_rw`), any runtime attempt to alter or drop this table will fail with `42501` "must be owner of table".

The fixture's boot-only focus misses this divergence, resulting in a **false green** for migrations that are unsafe under admin-initiated deployments.

### Strongest Rebuttal
The holder could assert that runtime migrations should only create tables that are either immediately transferred to `striatumd_rw` by a matching owner bundle, or that out-of-band migration execution is an administrative exception that operators must manually remediate.

### Residual Gap
Since owner bundles do not automatically track new runtime tables without manual updates to the owner bundle files, admin-applied migrations will inevitably leave new runtime tables owned by the owner role. The P0 spec must mandate that the fixture runs a differential test simulating *both* deploy pathways (boot-time vs. admin-DSN) for candidate migrations, or ensure that runtime-table creation is structurally restricted to the runtime role regardless of the connection's administrative DSN.

### Severity
Fixable gap (Severity: Medium, binding constraint).

---

## Challenge 3: Incomplete Owner-Role Isolation via Shared Cluster Roles

### Claim Challenged
From `HOLDER.md` §3.6 (Fixture self-check):
> *`SELECT rolsuper FROM pg_roles WHERE rolname = current_user` = `false`*
> *`SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid = 'striatumd.events'::regclass` ≠ `striatumd_rw`*

### Counterexample / Mechanism
The proposed self-checks only assert direct ownership of `events` and the direct superuser flag of `striatumd_rw`. However, they fail to assert the absence of role inheritance loops within the PostgreSQL cluster.

Because role memberships in PostgreSQL are cluster-wide, if the test runner executes in a cluster where the owner/admin role was previously granted membership to `striatumd_rw` with `INHERIT TRUE` (or vice versa), or if a parallel test package leaks membership edges, the `striatumd_rw` role might implicitly inherit administrative privileges. 

Under this condition, `striatumd_rw` would successfully execute the prohibited owner-table touched `ALTER` statement in Phase B (yielding a **false green** in the red test), yet fail in production where role inheritance is strictly disabled. The self-check as written would pass because `rolsuper` is false and direct relowner is not `striatumd_rw`, masking the privilege leak.

### Strongest Rebuttal
The developer can trust that the cluster role configuration is clean because `pgtest` provisions an isolated cluster.

### Residual Gap
Many local and CI environments reuse a persistent PostgreSQL cluster across test runs where roles are not rebuilt from scratch. To prevent false greens caused by inherited privileges, the fixture self-check must actively assert that `striatumd_rw` is not a member of, and does not inherit from, the database owner role. The self-check must include a query checking `pg_has_role()` or `pg_auth_members` to verify absolute role isolation.

### Severity
Fixable gap (Severity: Low, binding constraint).
