# FALSIFIER - RFC 0142 P4 activation and runtime-ownership gaps

author: falsifier-reviewer-004

## Challenge 1: bundle 0020 plus `STRIATUM_DEPLOY_DECOUPLED` is not an atomic activation

### Claim attacked

The Holder claims the P4 DDL revoke ships without recreating a lockout. The key
claim is that the deployer and decoupled boot path land with
`STRIATUM_DEPLOY_DECOUPLED` off, then the operator applies owner bundle 0020 and
flips the flag "together" (`HOLDER.md:316-325`). It also claims a binary built
against bundle 0020 halts cleanly when the DB is still at 0019, and that a
botched order where 0020 is applied under a binary without the deployer would
refuse cleanly rather than lock out (`HOLDER.md:326-335`).

That is the exact migration-safety claim P4 must clear: the revoke must not let
any supported boot path enter runtime `ApplyMigrations` after the serving role
has lost `CREATE` on the runtime schema.

### Concrete refutation

The proposed activation is split across two authorities that are not updated by
one transaction: durable DB state (`owner_bundle_meta` advances to 0020 and
revokes `CREATE`) and process-local configuration (`STRIATUM_DEPLOY_DECOUPLED`).
The current boot path has a real interleaving where those facts disagree and the
daemon still reaches runtime DDL.

Current source path:

1. `striatumd` defaults `--migrate` to true (`go/cmd/striatumd/main.go:51,76`).
2. Boot calls `BootstrapAndConnect(..., migrate)` at `go/cmd/striatumd/main.go:192-198`; when `migrate` is true, `BootstrapAndConnect` calls `ConnectAndMigrate` (`go/pkg/db/authority_bootstrap.go:181-194`).
3. `ConnectAndMigrate` checks the owner watermark, then immediately calls `ApplyMigrations` on the runtime pool runner (`go/pkg/db/connection.go:349-356`).
4. The watermark policy explicitly tolerates forward owner-bundle watermarks: `applied >= RequiredOwnerBundleVersion` returns nil (`go/pkg/db/owner.go:76-80,104-109`). That means an older binary with `RequiredOwnerBundleVersion == 19` and a database already advanced to 0020 does not halt. It proceeds into `ApplyMigrations`.
5. The proposed bundle 0020 revokes `CREATE ON SCHEMA striatumd FROM striatumd_rw` (`HOLDER.md:291-295`). If any pending runtime migration remains, including the P4 `deploy_cursor` migration >= 0044 if `daemon deploy` has not already run, `ApplyMigrations` executes that SQL as the runtime runner. The apply engine just executes the migration SQL on the supplied runner inside a transaction (`go/pkg/db/migrations.go:304-318`), so a `CREATE TABLE` or `CREATE INDEX` now fails with `42501`.
6. The clean non-restartable boot handling currently recognizes only `AwaitingOwnerDDLError` and `SchemaDriftError` (`go/cmd/striatumd/main.go:199-228`). A raw runtime-migration privilege error falls through to the fatal boot error path (`main.go:229`), which is the old crash-loop shape P4 is supposed to remove.

This is not a theoretical race. It is the operator sequence the Holder leaves
unspecified: apply owner bundle 0020, restart or auto-restart before the env flag
is set, or roll back to an older binary while the DB watermark is already
forward. Because forward tolerance is currently global, the older/flag-off
binary treats 0020 as harmless and only discovers the revoke inside runtime DDL.
The word "together" is carrying a state-machine invariant that the source does
not enforce.

There is also a choreography contradiction in the happy path. The Holder says P4
lands with auto-apply still default and bundle 0020 not applied (`HOLDER.md:318-320`),
but also says the binary's `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`
will advance to 20 (`HOLDER.md:326-330`). On an authority-bearing DB still at
0019, that new binary parks before serving. That may be correct, but then the
operator-facing order must be explicit: whether the old daemon stays serving,
whether the new CLI is run while the service is down, and exactly which preflight
prevents a restart between 0020 and the flag flip.

### Strongest rebuttal on the Holder's behalf

The Holder can say the intended happy path is: install the P4 CLI, run
`striatum daemon deploy` with the owner/admin DSN, apply owner bundle 0020, set
`STRIATUM_DEPLOY_DECOUPLED=ON`, then restart. It can also say the new P4 binary
will halt cleanly when owner bundle 0020 is missing, so at least the forward
upgrade path has a visible stop.

That rebuttal does not clear the gate. The spec claims the botched order refuses
cleanly, but the current source has a forward-watermark and flag-missing path
that reaches runtime `ApplyMigrations` after the revoke. P4's safety property
must cover the deployment mistakes it is explicitly designed to prevent, not only
the perfect operator sequence.

### Required design repair

P4 needs an activation protocol that fails closed before runtime DDL can run:

- Before `ConnectAndMigrate` calls `ApplyMigrations`, detect owner watermark >=
  20 with decoupled mode disabled or deploy incomplete, and return a typed
  non-restartable `awaiting_deploy` / `awaiting_deploy_config` error. The DB must
  be untouched and the remediation must name `striatum daemon deploy` and the
  missing flag/config state.
- Narrow the tolerate-forward owner-watermark policy for the first DDL-revoke
  bundle. An older binary seeing applied bundle 0020 must not proceed into
  runtime auto-apply as if the forward watermark were harmless.
- Replace "apply 0020 and flip the flag together" with a durable activation
  marker or preflighted command that checks: deployer exists, plan complete,
  `schema_state` matches, no pending runtime migrations, and decoupled boot is
  configured before the revoke can take effect.
- Add `T-deploy-revoke-activation-ordering`: 0020 before flag, 0020 before
  deploy, old binary plus 0020 plus pending runtime migration, and P4 binary plus
  flag off plus pending runtime migration. The assertion should be: no
  `ApplyMigrations`, no schema mutation, clean non-restartable halt, actionable
  deploy/config remediation.

### Real gap remaining

Real gap remains. The DDL revoke is safe only if the operator sequence is
perfect. As written, a very plausible ordering miss routes boot back through the
old runtime migration path under a role that no longer has runtime-schema `CREATE`,
recreating the #512-class lockout shape P4 is meant to retire.

## Challenge 2: owner/admin-applied runtime steps change the runtime ownership contract

### Claim attacked

The Holder says the deployer applies runtime DDL over the owner/admin connection
because after bundle 0020 the serving role cannot create objects (`HOLDER.md:245-250`,
`HOLDER.md:309-314`). It also says the serving role retains ownership of existing
runtime tables, that full re-ownership by the owner is out of P4 scope, and that
this residual ownership is acceptable because the serve path will issue no DDL
once `ApplyMigrations` is lifted (`HOLDER.md:297-305`). The build order then says
the new `deploy_cursor` migration is "runtime-owned" and modeled on
`0043_schema_state.sql` (`HOLDER.md:361-363`, `HOLDER.md:427`).

Those claims do not compose unless P4 defines a post-deploy ownership/grant
contract for runtime objects.

### Concrete refutation

The current runtime migration engine does not preserve runtime ownership when it
is driven by an owner/admin connection. `applyOne` opens a transaction, executes
the migration SQL on the provided runner, then stamps `schema_migrations` and
`schema_meta` (`go/pkg/db/migrations.go:304-350`). There is no `SET ROLE
striatumd_rw`, no `ALTER ... OWNER TO striatumd_rw`, and no central post-step
ownership/grant reconciliation.

Therefore the same runtime migration has different object ownership depending on
who runs it:

- Today, boot-time runtime migrations run through the runtime pool after
  `BootstrapAndConnect`, so newly created runtime objects are naturally owned by
  `striatumd_rw`.
- Under the Holder's P4 deployer, runtime steps run through the owner/admin
  connection. A new table, index, sequence, view, or function created by runtime
  SQL is owner-owned unless the migration itself contains explicit ownership
  transfer and grants.

Existing migration style is not a closed contract for that new posture. Migration
0043 creates `striatumd.schema_state` and grants read/write to `striatumd_rw`, but
it does not transfer ownership (`go/pkg/db/sql/0043_schema_state.sql:39-50`). The
Holder proposes to model `deploy_cursor` on 0043, while also calling it
runtime-owned. If P4 applies that migration as owner/admin, the object is not
runtime-owned in the same sense as today's runtime-created objects.

This matters for more than wording. P4 is trying to make the serving path zero
DDL without reopening #442. If future runtime objects become owner-owned with
runtime DML grants, that may be a good policy, but it is a different policy from
"runtime tables remain `striatumd_rw`-owned." It must be reflected in the two-role
fixture, load/preflight guards, permission tests, and operator docs. If future
runtime objects should remain runtime-owned, the deployer must define exactly how
an owner/admin execution context creates them while preserving runtime ownership.

### Strongest rebuttal on the Holder's behalf

The Holder can plausibly choose the owner-owned/DML-granted policy. Once P4 lands,
the serving daemon should not need DDL on future runtime objects, and P0's
two-role fixture can catch missing DML grants. That policy also avoids a broader
owner re-homing pass over existing runtime tables, which the Holder correctly
identifies as risky and outside P4.

But that is not the spec currently written. The spec relies on runtime ownership
as a residual-capability explanation while also changing runtime-step execution
to an owner/admin runner. It never states whether new runtime objects after P4
are owner-owned or runtime-owned, nor how tests prove the chosen invariant.

### Required design repair

P4 needs to choose and test one invariant:

- If future runtime objects remain `striatumd_rw`-owned, the deployer must include
  a post-step ownership-transfer protocol covering tables, indexes, sequences,
  views, functions, and future objects, with a two-role integration test.
- If future runtime objects become owner-owned with runtime DML grants, the spec
  must say so directly, stop treating runtime ownership as a P4 safety property
  for new objects, and add a build/load guard that every runtime migration grants
  exactly the serving privileges the daemon needs.
- Add `T-deploy-runtime-object-ownership`: apply a new runtime migration through
  the deployer owner connection and assert both catalog ownership and real
  `striatumd_rw` serving behavior for SELECT/INSERT/UPDATE/DELETE, plus failure
  for DDL after bundle 0020.

### Real gap remaining

Real gap remains unless the Holder pins this ownership policy. The deployer can
apply runtime DDL as owner/admin, but P4 cannot be build-ready while the runtime
object owner/grant invariant changes implicitly as a side effect of moving
`ApplyMigrations` out of serve-boot.

## Verdict

The Holder's direction is right: move schema mutation out of serve-boot and make
the deployer explicit. The P4 implementation shape still has two material gaps in
the decoupling and migration-safety lens. First, DDL-revoke activation is not a
durable state-machine edge and can still send boot into runtime DDL after the
runtime role loses `CREATE`. Second, owner/admin-applied runtime migrations change
object ownership semantics without a stated invariant. Both should force revision
before the P4 proposal is treated as build-ready.
