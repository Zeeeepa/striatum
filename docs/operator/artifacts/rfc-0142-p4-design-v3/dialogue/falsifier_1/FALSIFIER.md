# FALSIFIER - RFC 0142 P4 v3 moving-plan receipt gap

author: falsifier-reviewer-004

## Revision check: C3, N1, and C1/C2

I do not use the old C3 ownership-transfer contradiction as my standing blocker.
The v3 holder chooses resolution (a): bundle 0020 is excluded from the ordinary
owner prefix and from `owner-ddl apply`, then appended as the terminal deploy
step (`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:255-278`).
The runtime ownership reconcile runs before 0020 while `striatumd_rw` still has
`CREATE`, and 0020 commits last so the steady state denies `CREATE`
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:374-430`).
That is a coherent design-level close for the v2 C3 finding, assuming the build
and F12 enforce the special ordering.

I also do not find a direct regression of the C1 finalization boundary or the C2
fail-closed activation edge. The v3 holder preserves the `finalizing` state, the
idempotent terminal finalizer, and the §1.3 resumable-finalization row
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:147-168`,
`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:177-184`).
The terminal `complete` receipt remains idempotent on `(plan_hash, state=complete)`,
and `complete` is still last (`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:156-168`,
`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:481-489`).

The immediate N1 crash window is partly addressed: for transactional steps the
per-step receipt joins the same owner-connection transaction as the DDL, version
stamps, ownership reconcile, grants, and cursor advance; for NT-DDL the
`in_progress(k)` reconciler appends exactly one receipt before `step_committed(k)`
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:440-463`).
But the repair depends on `(plan_hash, step_index)` being stable across re-runs.
The v3 text does not actually specify a stable plan identity once a step has
moved the live frontier. N1 therefore remains open in a narrower, but still
material, form.

## Challenge: `(plan_hash, step_index)` is not stable under the written `BuildPlan` contract

### Claim attacked

The binding N1 fix requires an idempotent per-step receipt reconcile keyed on
`(plan_hash, step_index)`, and `doctor schema_deploy_unrecorded` must be green only
when all committed steps have receipts (`docs/operator/workflows/rfc-0142-p4-design-v3/SEED.md:159-185`).
The holder claims this key is stable because the plan is content-addressed by
`plan_hash` (`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:286-295`)
and says `doctor` reconstructs `BuildPlan` plus the current applied frontier to
determine which step indexes are committed (`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:491-499`).
RFC 0142's Layer 3 contract is stronger than that: the deployer advances a durable
cursor after each committed step so a crash resumes at the next clean boundary,
not at an unclassified or renumbered plan
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md:181-193`).

### Concrete refutation

The holder defines the plan as `BuildPlan(applied_owner, applied_runtime) ->
DeployPlan`, with steps equal to pending non-revoke owner bundles, then pending
runtime migrations, then terminal 0020 if pending
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:255-260`).
That is a pending-delta plan, not an immutable full target transcript. The current
engines named by the holder move exactly those frontiers after each step:
`ApplyMigrations` reads `current`, skips migrations `<= current`, and sets
`current = migration.Version` after a successful apply (`go/pkg/db/migrations.go:138-172`);
owner bundles read `MAX(version)` from `owner_bundle_meta`, skip `<= current`, and
set `current = bundle.Version` after each committed bundle (`go/pkg/db/owner.go:225-245`,
`go/pkg/db/owner.go:304-320`).

Concrete crash path:

```text
Initial frontier: owner=19, runtime=43
Initial plan H: [runtime:0044, runtime:0045, owner:0020]
Step indexes: 0=runtime:0044, 1=runtime:0045, 2=owner:0020
```

Step 0 follows the v3 Q3-A path. Runtime 0044 commits, the per-step receipt
`(H, 0)` is durable, and `deploy_cursor` records `step_committed(0), plan_hash=H`.
Then the process dies before step 1.

On re-run, the written `BuildPlan(applied_owner, applied_runtime)` contract reads
the live frontier `(19, 44)` and produces a different pending plan:

```text
Plan H': [runtime:0045, owner:0020]
Step indexes: 0=runtime:0045, 1=owner:0020
```

The durable cursor still says `plan_hash=H`, but the reconstructed binary plan is
`H'`. Section 1.3 only classifies `in_progress` / `step_committed` as
"incomplete, resume" when the cursor plan hash equals the binary's plan
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:177-184`).
If the deployer refuses because `H != H'`, the RFC's crash-resume promise is broken.
If it proceeds under `H'`, the step indexes have shifted and `(plan_hash, step_index)`
is no longer the stable receipt identity N1 depends on: runtime 0045 is now index 0,
while `(H, 0)` belongs to runtime 0044.

This also breaks the tightened doctor. A doctor that reconstructs `BuildPlan` from
the current applied frontier after runtime 0044 has committed no longer sees
runtime 0044 in the pending plan. It has no specified way to require the already
committed `(H, 0)` receipt or to detect its deletion. After runtime 0045 commits,
`BuildPlan(19, 45)` shrinks again to `[owner:0020]`; after terminal 0020 commits but
before finalization, `BuildPlan(20, 45)` can be empty even though the cursor still
needs `step_committed(N-1) -> finalizing -> complete` under the original `N`.

The v3 same-transaction receipt rule is real for a single step, but it does not
make the receipt key stable across process restart. The defect is now plan
identity, not receipt atomicity.

### Strongest rebuttal for the Holder

The Holder can say `BuildPlan` was intended to produce the full target transcript
for the binary, independent of the current applied frontier, and that
`deploy_cursor.plan_hash` pins the original plan for all future resumes. If that
were specified, the N1 key would be defensible: the deployer would resume at
`k+1` inside the same transcript even though live frontiers advanced.

That is not what v3 writes. `deploy_cursor` stores only `plan_hash`, `state`,
`step_index`, and `step_id`, not the ordered transcript, base frontiers, or target
frontiers (`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md:121-133`).
The plan builder is explicitly parameterized by `applied_owner` and
`applied_runtime`, and the doctor explicitly reconstructs from the current applied
frontier. The rebuttal requires a missing artifact: an immutable persisted deploy
plan or an unambiguous rule for reconstructing the original transcript from a
fixed base.

### Required repair

1. Before step 0 mutates anything, materialize an immutable deploy transcript keyed
   by `plan_hash`: base owner/runtime frontiers, target owner/runtime frontiers,
   every `{step_index, step_id, role, sha256, transactional}`, and terminal 0020
   placement.
2. On resume, load the stored transcript by `deploy_cursor.plan_hash`; do not use
   the moving pending-delta `BuildPlan(current_owner, current_runtime)` as the
   source of truth. Verify the stored transcript still matches the binary bytes,
   then resume at the next step in that transcript.
3. Redefine §1.3 for an incomplete cursor whose `plan_hash` is not the current
   binary's expected plan. It must remain refuse-to-serve, but the action should be
   "resume with the pinned plan/binary or explicit reconcile", not unclassified drift.
4. Make `doctor schema_deploy_unrecorded` enumerate committed steps from the stored
   transcript plus the cursor/frontier state, so deleting the `(H, 0)` receipt after
   runtime 0044 commits is detected even though current `BuildPlan` would omit 0044.
5. Add `T-deploy-plan-hash-stable-after-step`: with at least two runtime steps plus
   terminal 0020, kill after step 0 and after step 1. Re-run must keep the original
   `plan_hash`, preserve original step indexes, recognize prior receipts, complete
   the remaining steps, and leave `doctor schema_deploy_unrecorded` green. Also delete
   one prior per-step receipt after a partial commit and assert doctor warns.

### Verdict

Real gap remains. C3 is coherently resolved at the design level, and C1/C2 are not
regressed by the text I reviewed. The original N1 same-transaction/idempotent-reconcile
hole is narrowed, but the spec builds the exactly-once receipt key on a plan hash
and step index that are not stable under its written pending-delta `BuildPlan`
contract. A crash after a committed step can leave the cursor on plan `H` while
re-run and doctor reconstruct plan `H'`; that is neither the promised
"incomplete, resume" state nor a doctor-checkable receipt chain. N1 is therefore
not genuinely resolved until plan identity is made immutable across resume.
