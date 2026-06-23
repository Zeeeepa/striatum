# FALSIFIER - RFC 0142 P4 v3 decoupling and post-revoke ownership gaps

author: falsifier-reviewer-005

## Revision check: C3, N1, and C1/C2

I do not use the v2 C3 contradiction as a blocker for the first activation
deploy. The v3 holder chooses the SEED-recommended resolution (a): bundle 0020
is excluded from ordinary owner-prefix / `owner-ddl apply`, appended as the
terminal deploy step, and every runtime ownership reconcile runs before the
`CREATE` revoke commits (`HOLDER.md:255-278`, `HOLDER.md:374-430`,
`HOLDER.md:614-628`). That is a coherent answer for a deploy where 0020 is still
pending: bundles 0018/0019 explicitly require and grant `CREATE ON SCHEMA
striatumd TO striatumd_rw` before `ALTER ... OWNER TO striatumd_rw`
(`0018_runtime_table_ownership_transfer.sql:58-72,97-104`;
`0019_supervisor_pointer_runtime_ownership.sql:53-80`), and v3 sequences 0020
after those reconciles.

I also do not use the original v2 N1 omission as my blocker. The v3 text now
places the per-step receipt in the same owner-connection transaction for
transactional steps and adds an idempotent `(plan_hash, step_index)` receipt
reconcile for NT-DDL (`HOLDER.md:432-463`, `HOLDER.md:491-501`,
`HOLDER.md:647-653`). Falsifier 1 raises a separate N1/plan-identity challenge;
I am not duplicating it.

C2 is carried forward in the activation-binary path: `CheckDeployActivation`
still runs before `ApplyMigrations`, absent cursor is incomplete, typed halts are
named, and `RequiredOwnerBundleVersion` stays 19 (`HOLDER.md:320-357`,
`HOLDER.md:539-571`). The standing decoupling gaps below are different: the
incomplete cursor is not authoritative for no-0020 deployer-aware binaries, and
the C3 resolution only works while 0020 is pending, not for later deploys after
the steady-state revoke has already committed.

## Challenge 1: a no-0020 binary can serve an incomplete pre-0020 deploy

### Claim attacked

The holder's load-bearing C1/C2 classification says an incomplete cursor never
serves:

- `deploy_cursor.state in {in_progress, step_committed}` with the expected plan
  is "incomplete, resume" and must refuse-to-serve `awaiting_deploy`
  (`HOLDER.md:177-182`).
- `finalizing` must refuse-to-serve, never serve, and be repaired by the
  finalizer (`HOLDER.md:181`).
- The N1/C1 coherence section restates the global claim: a resume never serves
  when the cursor is `finalizing`, `step_committed`, or `in_progress`
  (`HOLDER.md:477-479`).
- F11 says that with the revoke active or pending, boot never calls
  `ApplyMigrations`; it halts `awaiting_deploy` / `awaiting_deploy_config`, and
  its test matrix spies that `applyOne` is not entered (`HOLDER.md:651`).

But the enforcement in §3.3a is narrower than those claims. The spec defines
`revokeEmbedded` as "this binary ships the 0020 file" (`HOLDER.md:328-334`). If
`!revokeEmbedded`, `CheckDeployActivation` is inert and relies only on the
forward-watermark rule (`HOLDER.md:335-337`). That forward-watermark rule fires
only when an old/no-0020 binary observes `applied_owner >= 20`
(`HOLDER.md:554-561`). Before terminal 0020 commits, `applied_owner` is still 19
by design (`HOLDER.md:353-357`, `HOLDER.md:585-597`).

So the very window introduced to fix C3 - all runtime steps have run or are in
progress, but terminal 0020 has not committed yet - is invisible to the no-0020
serve path.

### Concrete refutation

Use the holder's own two-binary choreography.

1. The inert-landing binary ships the deployer engine, `deploy` verb, >=0044
   `deploy_cursor` migration, decoupled boot path with the flag OFF, doctor, and
   the forward-watermark rule, but does not embed the 0020 file
   (`HOLDER.md:577-584`). It boots and serves at owner watermark 19.
2. The activation binary embeds 0020. The operator parks the daemon, sets
   `STRIATUM_DEPLOY_DECOUPLED=1`, and runs `striatum daemon deploy`. The deploy
   order is non-revoke owner bundles <=19, then pending runtime steps, then
   terminal owner bundle 0020 (`HOLDER.md:585-597`).
3. Kill the deploy after a runtime step commits and writes
   `deploy_cursor.state = step_committed(k)`, but before terminal 0020 commits
   and before the finalizer reaches `complete`. This is explicitly resumable
   under §1.3, and it is exactly the C3-safe point where `striatumd_rw` still
   holds `CREATE`.
4. Restart or roll back to the inert-landing binary, or any deployer-aware binary
   that does not embed 0020. This is not an exotic actor: the holder makes it the
   first required rollout binary and says it is serving before activation
   (`HOLDER.md:577-584`).
5. On that boot, `applied_owner` is still 19. The current source path is
   `CheckOwnerBundleWatermark` before `ApplyMigrations`, then the P3 drift check
   plus `RecordSchemaFingerprint` (`go/pkg/db/connection.go:341-353`,
   `go/pkg/db/connection.go:376-402`). Current `CheckOwnerBundleWatermark`
   tolerates `applied >= required`, and today `LatestOwnerBundleVersion` /
   `RequiredOwnerBundleVersion` are 19 (`go/pkg/db/owner.go:17-35`,
   `go/pkg/db/owner.go:99-154`). The v3 forward-watermark change still only
   changes the `applied >= 20` no-0020 case, so it does not fire here.
6. Because `!revokeEmbedded`, the v3 `CheckDeployActivation` predicate is inert
   and never reads `deploy_cursor` (`HOLDER.md:335-337`). The no-0020 binary can
   therefore proceed to the legacy serving path. If the recorded fingerprint
   differs, the landed P3 gate is shadow by default: `CheckSchemaDrift` returns
   `(drifted=true, nil)` when `STRIATUM_SCHEMA_DRIFT_REFUSE` is unset
   (`go/pkg/db/schema_drift.go:15-28`, `go/pkg/db/schema_drift.go:239-274`), and
   `ConnectAndMigrate` then self-records this binary's expected fingerprint
   (`go/pkg/db/connection.go:384-402`, `go/pkg/db/schema_drift.go:163-195`).

The result is a database with an incomplete deploy cursor that the v3 table says
must halt `awaiting_deploy`, but a permitted rollout binary never consults the
cursor and can serve anyway. It can also self-record a fingerprint while the P4
terminal receipt/fingerprint finalizer has not run.

### Why this is material

This is not just "CREATE has not been revoked yet, so no harm." The P4 safety
claim is broader: schema mutation stops being a restart side effect, incomplete
plans are classified as resume-only, and the serving path does not stamp its own
view of a half-applied deploy. This interleaving breaks all three:

- A boot on the no-0020 binary reaches the legacy `ApplyMigrations`/self-record
  path while `deploy_cursor` is incomplete, despite F11's "pending deploy never
  calls `ApplyMigrations`" assertion.
- The §1.3 second signal is not authoritative across the rollout pair. It only
  gates binaries that embed 0020, so `step_committed` is not globally
  refuse-to-serve.
- The P3 shadow self-record can overwrite `schema_state` from the wrong boot
  path before the P4 finalizer writes the terminal deploy receipt and
  fingerprint. That regresses the P3/P4 split the holder calls load-bearing:
  verify-only boot must not mask deploy state (`HOLDER.md:308-315`), but the
  no-0020 boot is still mutate-and-self-record.
- F11's old-binary matrix misses this case. It tests no-0020 old binary only
  when `applied_owner = 20` (`HOLDER.md:651`), but C3's revoke-last design makes
  the dangerous incomplete-deploy window occur at `applied_owner = 19`.

The strongest concrete reproducer is:

```text
state before activation:
  owner_bundle_meta max = 19
  runtime frontier includes deploy_cursor
  inert/no-0020 binary can serve

activation:
  run deploy with an activation binary
  kill after runtime step k commits cursor step_committed(k)
  kill before owner:0020 and before finalizing/complete

bad restart:
  boot inert/no-0020 binary
  CheckDeployActivation inert because !revokeEmbedded
  forward-watermark inert because applied_owner == 19
  boot does not read deploy_cursor
  P3 drift is shadow and may self-record
  daemon serves while deploy_cursor is incomplete
```

### Strongest rebuttal on the Holder's behalf

The holder can say the operator choreography parks the daemon during activation
and restarts only the activation binary after `striatum daemon deploy` completes.
It can also say that, before 0020 commits, no DDL revoke has happened, so the old
runtime role is not yet missing `CREATE`; the exact v2 C3 transfer failure is
not present in this window.

That rebuttal is not enough for a safe-by-construction deployer. The P4 design
explicitly sells crash-resume and rollback-resistant classification, not "the
operator must not restart the previous binary during the crash window." The spec
itself includes old/no-0020 binary cases in F11 and adds `G-old-binary-refuse`
(`HOLDER.md:607-612`, `HOLDER.md:651`), so this is within scope. The missing
case is simply earlier than 0020: `applied_owner == 19` plus
`deploy_cursor.state != complete`.

### Required repair

The revision needs one hard edge that makes `deploy_cursor` authoritative before
terminal 0020, not only after it:

1. Make every deployer-aware binary, including the inert/no-0020 landing binary,
   read `deploy_cursor` before `ApplyMigrations` and before
   `RecordSchemaFingerprint`. If the cursor exists and is not `complete` for a
   plan this binary can prove safe, return `awaiting_deploy` DB-untouched even
   when `!revokeEmbedded`.
2. Or introduce a durable pre-0020 activation marker that no older serve path can
   ignore, and set it before the first deploy step that can leave an incomplete
   cursor. The marker must halt no-0020 binaries at owner watermark 19, not just
   after 0020 raises the watermark to 20.
3. Extend F11 with the missing case: no-0020 deployer-aware binary,
   `applied_owner = 19`, `deploy_cursor.state in {in_progress, step_committed,
   finalizing}`, optional fingerprint mismatch, and pending/no pending runtime
   migrations. Assert `ApplyMigrations` is not called, `RecordSchemaFingerprint`
   is not called, the DB is byte-identical, and the halt is `awaiting_deploy`.
4. Extend `G-old-binary-refuse` so it proves the pre-0020 incomplete-deploy
   window cannot be served, not merely that 0020 is refused when the >=0044
   marker is absent.

## Challenge 2: C3 only works for the activation deploy, not later object-creating runtime deploys

### Claim attacked

The holder claims C3 is resolved by sequencing bundle 0020 last while preserving
Policy 1: runtime objects stay `striatumd_rw`-owned (`HOLDER.md:374-430`). It
also claims the post-deploy steady state denies `striatumd_rw` `CREATE`
(`HOLDER.md:507-527`) while new runtime objects created by the deployer are
reconciled back to `striatumd_rw` ownership (`HOLDER.md:529-533`). F12 tests a
plan containing "a new runtime migration creating a table + index + sequence" plus
terminal 0020, then asserts catalog owner, DML as `striatumd_rw`, and post-deploy
`CREATE` denial (`HOLDER.md:652`).

That test shape proves only the first activation plan where 0020 is still
pending. It does not prove the ordinary later case after 0020 has already
committed.

### Concrete refutation

After a successful activation deploy:

```text
owner_bundle_meta max = 20
deploy_cursor state = complete
striatumd_rw has_schema_privilege(striatumd, CREATE) = false
```

Now ship a later P4-era binary with a new runtime migration that creates a table
or sequence. This is in scope, not speculative outside the design: RFC 0142 Layer
3 says `striatum daemon deploy` is the only mutator and applies every schema step
with a resumable cursor and deploy receipt (`docs/rfcs/0142-safe-by-construction-database-change-deployment.md:181-193`).
The holder's own mechanism says the catalog diff covers tables, indexes,
sequences, views, matviews, and future object kinds (`HOLDER.md:405-417`), and
the current runtime migration history regularly creates tables and indexes
(`go/pkg/db/sql/0043_schema_state.sql:39`; many earlier migrations do the same).

For that later deploy, 0020 is no longer pending, so "sort the revoke last" gives
the deployer no step that can be delayed. The deployer's Policy-1 runtime step
still does:

1. create the new object over the owner connection;
2. run `ALTER <kind> striatumd.<name> OWNER TO striatumd_rw`;
3. assert serving-role DML; then continue (`HOLDER.md:394-424`).

But the exact PostgreSQL prerequisite that reopened C3 still applies: the new
owner must hold `CREATE ON SCHEMA striatumd`. The repo's own bundles say that
plainly, and they grant CREATE first for this reason
(`0018_runtime_table_ownership_transfer.sql:64-72,97-104`;
`0019_supervisor_pointer_runtime_ownership.sql:53-80`). Post-activation, bundle
0020 has revoked precisely that privilege. The v3 step-1 precondition even says
to halt `deploy_create_prerequisite_missing` if
`has_schema_privilege('striatumd_rw','striatumd','CREATE')` is false
(`HOLDER.md:400-404`). In the ordinary post-activation state, it is false by
design.

So the next object-creating runtime migration after 0020 cannot satisfy all three
v3 claims:

- `ALTER ... OWNER TO striatumd_rw` succeeds;
- the committed steady state denies `striatumd_rw` `CREATE`;
- future/new runtime objects still become `striatumd_rw`-owned under Policy 1.

The v3 resolution makes those facts coexist only in the one deploy where 0020 is
pending and can be delayed until the end. It does not specify a mechanism for the
next deploy after 0020 has already committed.

### Why this is material

This is the same lockout class, shifted one release later. After activation, a
new binary with a table-creating runtime migration will make serve-boot halt
`awaiting_deploy` as intended. The only remediation is `striatum daemon deploy`.
But the deployer, following the v3 spec, hits its own CREATE prerequisite guard
or a raw `42501` at the first ownership reconcile. A bare restart cannot fix it,
and because the runtime role's CREATE denial is the intended steady state, the
operator is stuck unless they violate the spec with an ad hoc grant or switch
ownership policy.

F12 misses this because it tests only:

```text
plan = {new runtime object-creating migration} + {terminal 0020}
```

It must also test:

```text
state = already complete with owner_bundle_meta max = 20 and striatumd_rw CREATE denied
plan = {new runtime object-creating migration only}
```

Under the current v3 mechanism, that second test fails before or during
`ALTER ... OWNER TO striatumd_rw`.

### Strongest rebuttal on the Holder's behalf

The holder can say P4's immediate activation bundle only needs to carry the new
`deploy_cursor` migration and 0020, and future post-activation object-creating
runtime migrations can be deferred until another design slice.

That is not the spec as written. P4 installs the permanent one-shot deployer,
declares `striatum daemon deploy` the only schema mutator, keeps the Policy-1
claim for new runtime objects, and explicitly avoids Policy 2 or scoped temporary
grants (`HOLDER.md:382-392`, `HOLDER.md:529-537`, `HOLDER.md:657-680`). If future
runtime object creation is out of scope, the spec must say so and add a build
guard forbidding such migrations after 0020. Otherwise it needs one of the two
mechanisms it rejected: a scoped temporary `GRANT CREATE` around later ownership
transfers, or Policy 2 with exact DML grants and a §4.1 correction.

### Required repair

The revision needs to make the post-activation policy coherent, not just the
activation plan:

1. Extend resolution (a) with a post-0020 rule: either runtime migrations that
   create new ownable objects are forbidden until another policy lands, enforced
   by a build/load guard, or the deployer temporarily grants `CREATE` to
   `striatumd_rw` around each later `ALTER OWNER` and revokes it before commit.
2. Or switch to Policy 2 after 0020: owner/admin owns new runtime objects, each
   runtime migration carries exact DML grants, and §4.1 stops claiming new runtime
   objects stay `striatumd_rw`-owned.
3. Extend `T-deploy-runtime-object-ownership` with a second phase: first complete
   a deploy that applies terminal 0020 and proves `striatumd_rw` CREATE denial;
   then run a subsequent deploy whose pending runtime migration creates a table,
   index, and sequence. Assert either the expected guard/refusal or successful
   ownership/DML plus post-deploy CREATE denial under the chosen mechanism.

## Verdict

Real gaps remain.

The direct v2 C3 activation contradiction is fixed for the first deploy by
0020-last, and the original N1 receipt-omission text is improved. But the
decoupling gate still should not clear:

- the no-0020 deployer-aware binary can ignore an incomplete pre-0020 cursor and
  serve/self-record at owner watermark 19; and
- C3's Policy-1 ownership mechanism fails again for the first object-creating
  runtime migration after 0020 is already applied and CREATE is intentionally
  denied.

Both are material migration-safety failures in the #512 class: the design's clean
halt points can name `striatum daemon deploy` as remediation, but the state
machine does not guarantee that remediation is safe and completable in the
interleavings it actually ships.
