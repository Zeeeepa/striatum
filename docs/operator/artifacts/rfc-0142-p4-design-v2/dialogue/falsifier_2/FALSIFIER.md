# FALSIFIER - RFC 0142 P4 v2 DDL-revoke ownership-transfer gap

author: falsifier-reviewer-004

## Revision check: C1, C2, and C3

I do not find the original C1 finalization gap still open in the same form. The
v2 holder adds `finalizing`, classifies `finalizing` with the expected
`plan_hash` as resumable finalization, and moves `complete` last after the
receipt and `schema_state` fingerprint are durable (`HOLDER.md:140-155`,
`HOLDER.md:165-178`, `HOLDER.md:387-408`, `HOLDER.md:560`). That is a real
design-level repair for the v1 "complete before receipt/fingerprint" hole.

I do not find the original C2 activation gap still open in the same form. The v2
holder specifies `CheckDeployActivation` immediately after the owner watermark
and before `ApplyMigrations`, treats an absent `deploy_cursor` as incomplete
rather than an error, adds `awaiting_deploy` / `awaiting_deploy_config` typed
halts, keeps `RequiredOwnerBundleVersion` at 19, and adds a forward-watermark
rule for revoke-unaware binaries (`HOLDER.md:293-328`, `HOLDER.md:445-484`,
`HOLDER.md:561`). That is a real design-level C2 repair, assuming the deployer
can actually complete the activation plan.

C3 is not genuinely resolved. The v2 holder chooses policy 1: runtime objects
stay `striatumd_rw`-owned. Its proposed mechanism is to run runtime migration SQL
over the owner connection, then in the same step transaction run
`ALTER <kind> striatumd.<name> OWNER TO striatumd_rw` for objects the step
created, reassert DML grants, advance `deploy_cursor`, and commit
(`HOLDER.md:347-374`). But the same P4 design says owner bundle 0020 revokes
`CREATE ON SCHEMA striatumd FROM striatumd_rw` (`HOLDER.md:428-431`) and F12
requires both "objects owned by `striatumd_rw`" and, after 0020, "`striatumd_rw;
CREATE TABLE` -> `42501`" (`HOLDER.md:562`). Those two requirements are
inconsistent with the current PostgreSQL ownership-transfer rule the repository
already relies on.

## Challenge: 0020 removes the privilege required for the C3 ownership transfer

### Claim attacked

The holder's C3 close depends on the deployer being able to create a runtime
object as the owner/admin role and then transfer ownership to `striatumd_rw`
inside the same step transaction (`HOLDER.md:347-374`). The activation plan
applies owner bundle 0020 and then pending runtime steps as one ordered plan
(`HOLDER.md:504-507`), and the plan generator preserves the owner-prefix-before-
runtime ordering (`HOLDER.md:241-247`). So once the first post-0020 runtime object
is created, the revoke is already in effect.

The current source says that transfer needs `striatumd_rw` to hold `CREATE` on
the schema. Owner bundle 0018 documents the exact prerequisite:
`striatumd_rw MUST hold CREATE on schema striatumd`; otherwise `ALTER ... OWNER TO
striatumd_rw` fails `permission denied for schema striatumd`
(`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:58-72`). That
bundle grants `CREATE` before transferring ownership
(`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:97-102`). Bundle
0019 repeats the same prerequisite and grant before its transfer
(`go/pkg/db/sql/owner/0019_supervisor_pointer_runtime_ownership.sql:53-80`).

P4 bundle 0020 revokes exactly that prerequisite.

### Concrete refutation

Take the bootstrap-order case the v2 spec explicitly tries to close: a DB never
ran the inert binary, so `deploy_cursor` is absent. The activation binary parks,
the operator sets `STRIATUM_DEPLOY_DECOUPLED=1`, and runs
`striatum daemon deploy`. Per the holder, deploy applies bundle 0020 and any
pending runtime steps as one ordered plan, creating `deploy_cursor` over the
owner connection and reconciling its ownership to `striatumd_rw`
(`HOLDER.md:512-517`).

The ordered plan applies pending owner bundles before runtime migrations
(`HOLDER.md:241-247`). Therefore bundle 0020 has already executed:

```sql
REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw;
```

Then the runtime step creates `striatumd.deploy_cursor` as the owner/admin role
and attempts the C3 reconciliation:

```sql
ALTER TABLE striatumd.deploy_cursor OWNER TO striatumd_rw;
```

On the documented two-role topology, that transfer fails because the new owner no
longer has `CREATE` on `striatumd`. The repository's existing owner bundles are
explicit about this PostgreSQL requirement and grant `CREATE` first for that
reason. The v2 C3 policy instead removes the prerequisite before using it.

This is not limited to first bootstrap. Any future object-creating runtime
migration after 0020 hits the same wall: the deployer can create the object as
owner, but cannot make it `striatumd_rw`-owned under the chosen policy unless it
restores `CREATE` to `striatumd_rw` for the transfer. If it does not restore it,
`T-deploy-runtime-object-ownership` cannot pass. If it does restore it, the spec
must say so and prove the restore is transactional, invisible to serving, and
revoked again before commit.

### Why this is material

This reopens the C3 gate and turns into an activation failure, not just a catalog
tidiness issue. The intended activation sequence parks the daemon and runs
`daemon deploy` as the only way to apply 0020 safely (`HOLDER.md:499-510`). But
the same deploy run can strand itself after 0020, before the runtime frontier is
complete, at the first runtime step whose ownership must be reconciled. A
subsequent boot sees an incomplete deploy and halts `awaiting_deploy`, which is
cleaner than the old raw `42501` crash loop but still leaves the operator in the
same practical class of lockout: the only command that can finish the deploy has
a self-inflicted privilege contradiction.

It also means the bootstrap-order C2 sharpening is not actually closed. The
holder says absent `deploy_cursor` should be classified as incomplete and repaired
by `daemon deploy` creating the table first (`HOLDER.md:512-517`). Under the
owner-prefix plan, 0020 revokes `CREATE` before that table can be transferred to
`striatumd_rw`, so the repair path fails exactly where it is supposed to prove
the activation ordering safe.

### Strongest rebuttal on the holder's behalf

The holder can say the owner/admin DSN might be a superuser, or that the deployer
could temporarily grant `CREATE ON SCHEMA striatumd TO striatumd_rw`, perform the
`ALTER OWNER`, then revoke `CREATE` again inside the same transaction before
commit. That would be a plausible repair: no serving daemon is supposed to run
during activation, and if the grant/revoke is never committed as an externally
visible state then the serving role still lacks `CREATE` after 0020.

That is not the spec as written. The spec says 0020 revokes `CREATE`, then the
runtime-step wrapper does the ownership diff and `ALTER OWNER`; it never names a
temporary grant, a superuser-only precondition, or a test proving the grant is not
visible after commit. The existing source comments show why that omission matters:
the prior bundles had to grant `CREATE` explicitly before the same kind of
ownership transfer would work.

### Required repair

The revision needs one concrete policy that is compatible with 0020:

1. Keep policy 1, but specify an owner-connection, same-transaction temporary
   `GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` around each ownership
   transfer, followed by `REVOKE CREATE` before commit. Extend F12 to assert the
   transfer succeeds in a non-superuser two-role cluster, the committed post-state
   denies `striatumd_rw CREATE`, and no boot can serve while the temporary grant
   exists.
2. Or change to policy 2: owner/admin owns post-0020 runtime objects and every
   runtime migration must carry exact DML grants, then correct §4.1 so it stops
   claiming new runtime objects remain `striatumd_rw`-owned.
3. Or change the activation ordering so the `deploy_cursor` runtime migration and
   any ownership-transfer prerequisite are applied before 0020, then define how
   future post-0020 object-creating runtime migrations are handled. This still
   needs the policy-1 temporary grant or policy-2 owner-owned rule for future
   objects.

Add a refuting test to F12 that runs after 0020 in a documented non-superuser
two-role cluster: create a runtime table via the deployer, attempt the ownership
reconciliation, assert the object owner and DML behavior, and assert
`striatumd_rw` cannot create a fresh table after the deploy commits.

## Verdict

Real gap remains. C2 is much better specified than v1, but it relies on a
deployer that can complete the 0020-plus-runtime activation plan. The C3 repair
cannot do that as written because 0020 revokes the schema privilege PostgreSQL
requires for transferring ownership to `striatumd_rw`. The revision therefore has
not cleared the decoupling / migration-safety gate.