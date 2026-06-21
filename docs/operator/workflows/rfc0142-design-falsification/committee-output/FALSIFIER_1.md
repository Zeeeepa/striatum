# FALSIFIER 1 — Challenge to RFC 0142 P0 Spec

author: falsifier-reviewer-001
role: falsifier
gate: fg_rfc0142_design (falsification)
scope: RFC 0142 P0 only (Layer 1a)
status: active challenge

This challenge attacks the executable oracle design proposed in `HOLDER.md`. While the direction of the two-role fixture is sound, the proposed `SET ROLE`-based implementation carries technical flaws that introduce security escapes and verification gaps.

---

## Challenge 1: The `RESET ROLE` Privilege Escalation Sandbox Escape (False Green)

### Claim Challenged
From `HOLDER.md` §3.2 & §3.3:
> *Phase B — system-under-test as `striatumd_rw` ... through a pool whose connections `SET ROLE striatumd_rw` directly — the canonical runtime role, so privileges equal prod exactly...*
> *After `SET ROLE striatumd_rw`, PostgreSQL carries out privilege checks as `striatumd_rw` for the duration.*

### Counterexample / Mechanism
In the proposed fixture, the underlying session user (login user) is the privileged DSN user (owner/superuser). When pgx executes SUT migrations through this pool, the connection uses `AfterConnect` to run `SET ROLE striatumd_rw`. 

However, PostgreSQL session control allows the active role to be changed dynamically. Any SQL execution context—including custom dynamic SQL, helper functions, or a migration script itself—can execute:
```sql
RESET ROLE;
-- or
SET ROLE NONE;
```
Under PostgreSQL semantics, `RESET ROLE` resets the current user identifier to the **session user identifier** (the login user). 

- **In the test fixture:** `RESET ROLE` escalates privileges back to the privileged DSN user (owner/superuser).
- **In production:** The daemon connects *directly* as `striatumd_rw` (with no owner-role in the session ancestry), so `RESET ROLE` only falls back to `striatumd_rw` itself.

Therefore, a candidate migration containing a `RESET ROLE` bypass followed by an illegal owner-table alteration (e.g., `RESET ROLE; ALTER TABLE striatumd.events ADD COLUMN p0_probe integer;`) will execute successfully in the test fixture (causing a **false green**) but will fail with `42501` and crash-loop the daemon in production.

### Strongest Rebuttal
The holder might argue that migration scripts are static, reviewed, and would not contain `RESET ROLE` or `SET ROLE NONE`.

### Residual Gap
An honest executable oracle must be escape-proof. To prevent session escalation, Phase B SUT migrations must run in a session where the login user itself is constrained. 
The spec must be updated to require that pgtest provisions a dedicated LOGIN role (without superuser or owner privileges, and without parent membership to the owner role) for SUT execution, rather than relying on `SET ROLE` within a privileged connection.

### Severity
Fixable gap (Severity: Medium-High, binding constraint).

---

## Challenge 2: Owner Bundle Drift & Coverage Gaps for Recent Runtime Tables

### Claim Challenged
From `HOLDER.md` §3.5 & §3.7:
> *The green control ... succeeds, because owner bundle 0018 transferred job_recovery_state ownership to `striatumd_rw`...*
> *Both the static guard and the live oracle read from one source — the owner bundle files. P0 MUST NOT introduce a hardcoded owner-table list...*

### Counterexample / Mechanism
The holder relies on the applied owner bundles to establish a realistic ownership topology. However, the existing owner bundles (up to `LatestOwnerBundleVersion = 19`) have drifted and fail to cover all tables created by runtime migrations. Specifically, runtime migrations `0038`, `0041`, and `0042` created three new tables:
1. `striatumd.supervisor_buffered_packets` (migration `0038`)
2. `striatumd.event_chain_segments` (migration `0041`)
3. `striatumd.verifier_attestations` (migration `0042`)

These tables are created during Phase A (as the owner role) but are **never** transferred to `striatumd_rw` in owner bundles `0018` or `0019`. As a result, in Phase B, they remain owned by the owner role. 

If a candidate migration tries to `ALTER` or drop indexes on `verifier_attestations` under `striatumd_rw`, the test will fail with `42501`. While this is technically a "true red" (it would also fail in prod), the developer expects these tables to be runtime-writable, creating a confusing "false red" from the developer's design perspective because the database itself is in an inconsistent state.

### Strongest Rebuttal
The holder could argue that the fixture is behaving correctly and exposing an actual production deficiency: the owner bundles are indeed incomplete. The fixture should not mask this gap.

### Residual Gap
If the fixture fails because of pre-existing owner bundle omissions, the test suite becomes unusable for new runtime changes until a new owner bundle is authored. 
The P0 spec must clarify how this is resolved: either the fixture must explicitly log warnings for tables created in runtime migrations that lack runtime ownership (without failing the test, unless they are altered), or the P0 spec must mandate that the downstream implementation run includes a new owner bundle (`0020`) to transfer the remaining cohort (`supervisor_buffered_packets`, `event_chain_segments`, and `verifier_attestations`) to restore consistency.

### Severity
Fixable gap (Severity: Medium, binding constraint).

---

## Challenge 3: Implicit `search_path` and Temp Object Privilege Leaks

### Claim Challenged
From `HOLDER.md` §3.3:
> *This is the direct answer to SEED falsification target #1: role membership does not leak owner power, because the operations that fail gate on ownership...*

### Counterexample / Mechanism
When executing Phase B under `SET ROLE striatumd_rw`, the active schema `search_path` defaults to the session user's configuration unless explicitly overridden. If the session user (owner) has schema usage or search path overrides, the active resolution path for relations might resolve objects in the `pg_catalog` or temporary schema `pg_temp` using the owner's higher privileges.

In particular, PostgreSQL temporary tables (`pg_temp`) are session-private but check permissions based on the active role. If `striatumd_rw` creates temporary tables that interact with owner-owned schemas, subtle differences in relation lookup order (`search_path`) can allow references to bypass expected schema boundaries or shadow runtime tables, causing tests to evaluate incorrectly.

### Strongest Rebuttal
`striatumd_rw` has basic DML permissions anyway, and temporary tables cannot mutate persistent owner tables.

### Residual Gap
The Phase B connection pool configuration must explicitly set a clean, isolated `search_path` (e.g. `SET search_path TO striatumd, public`) and disable access to other schemas to guarantee that relation resolution matches the production environment.

### Severity
Fixable gap (Severity: Low, binding constraint).
