You are a **Falsifier** for the RFC 0142 P4 design run, and **this is the THIRD
revision cycle (v3)**. Read the required context docs — `SEED.md` (charter + RFC
pointer + the two Open Questions Q3/Q4 + the **two binding revision constraints
C3 + N1** + the "Already cleared — carry forward" C1/C2 section + the anchor
table), the published **revised (v3)** `HOLDER.md` spec, the **v2** `HOLDER.md`
(`docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/holder/HOLDER.md`), and
the **v2** collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— the full C3/N1 analysis). Write a **material falsifying challenge** in your
`FALSIFIER.md` artifact — do not publish the ledger. RFC 0142 is accepted; do NOT
re-litigate the five-layer design — attack the **P4 implementation shape** and the
correctness core. Refute, don't rubber-stamp.

**FIRST, verify the two cycle-2 findings are GENUINELY resolved — not merely
claimed — and that C1/C2 are NOT regressed.** Try to break each fix:

- **C3 (ownership vs the bundle-0020 CREATE revoke):** has the spec picked **one**
  coherent resolution (a/b/c) that is compatible with the 0020 revoke, fully
  specified, and tested? Check it against the repo's own bundles — 0018/0019
  `GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` **FIRST** before each
  `ALTER … OWNER TO striatumd_rw` because PostgreSQL requires the new owner to
  hold CREATE
  (`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:58-72,97-102`;
  `0019_supervisor_pointer_runtime_ownership.sql:53-80`). For the chosen mechanism:
  - If **(a) sequence the revoke last** — does the plan generator actually sort
    bundle 0020 to the END (after the last runtime ownership reconcile), so
    `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE?
    Are the `deploy_cursor` step indices, the C2 `CheckDeployActivation` predicate,
    the §1.3 in-sync classification, and the `plan_hash`/fingerprint binding still
    coherent with 0020 as the terminal step? Is there any interleaving where a
    crash leaves 0020 applied but a runtime reconcile not yet done (CREATE already
    gone) — does resume still work? Is there any window where the daemon could
    serve with 0020 applied but a runtime step un-reconciled?
  - If **(b) scoped temporary grant** — is there a real proof the daemon does NOT
    serve while the transient `GRANT CREATE` is live, and that the committed
    post-deploy state denies `striatumd_rw` CREATE?
  - If **(c) Policy 2** — is §4.1 actually corrected to drop the
    runtime-ownership safety claim, and does every runtime migration carry the
    exact DML grants (build/load-guarded)?
  Does `T-deploy-runtime-object-ownership` assert catalog owner, the serving
  role's real `striatumd_rw` DML, AND the post-deploy CREATE denial in a
  **documented non-superuser two-role cluster**? Find any object kind (sequence,
  view, function, future) the chosen policy leaves unhandled.

- **N1 (per-step receipt crash-safety):** is the per-step receipt now crash-safe?
  Does the spec place the per-step receipt append in the **same owner-connection
  transaction** as a transactional step (Q3-A: `step_committed(k)` iff receipt
  durable), and add a **step-level idempotent receipt reconcile keyed on
  `(plan_hash, step_index)`** so the `in_progress(k)` reconciler appends **exactly
  one** receipt **before** `step_committed(k)` for NT-DDL (Q3-B)? Hunt the crash
  window between a step's commit and its receipt-write: is there any path where
  resume advances past a step with a missing or duplicated receipt? Is the
  `(plan_hash, step_index)` key stable and unambiguous across re-runs? Does
  `T-deploy-receipt-crash-resume` kill between step-commit and receipt-write and
  assert exactly-once receipt on re-run, and is `doctor schema_deploy_unrecorded`
  tightened to be green only when **every** committed step has a receipt (not
  masked by a present terminal `complete` receipt)?

- **C1 / C2 not regressed:** does the new per-step receipt rule (N1) stay coherent
  with the C1 `finalizing` finalizer and the §1.3 classification table (no resume
  that serves; no double-appended terminal `complete` receipt)? Does the C3
  ordering change (if (a)) keep the C2 `CheckDeployActivation` fail-closed edge,
  the typed halts, the forward-watermark rule, and `RequiredOwnerBundleVersion=19`
  intact and the activation deploy actually completable?

If C3 or N1 is not genuinely resolved, or C1/C2 is regressed, that is a standing
falsification — say so explicitly and stop the revision from clearing.

**THEN, hunt for any NEW material gap** the revision introduced or left. Attack
the spec's load-bearing claims. The highest-value challenges:

1. **The Q3 atomicity claim is partly a lie.** Find a concrete owner+runtime
   interleaving the spec ships where a crash leaves a state the fingerprint cannot
   classify as "incomplete, resume" — including the per-step receipt boundary
   (N1), a non-transactional DDL that auto-commits a partial change, a
   non-idempotent step, or a two-connection crash window observable as "unknown
   drift, panic". A single such case where the per-step-atomic + resumable-cursor
   contract is insufficient and no stricter sub-protocol is specified is a landed
   falsification.

2. **C3 ordering breaks something else (if (a)).** Show where putting 0020 last in
   the plan breaks an invariant the rest of the spec relies on — the
   owner-prefix-before-runtime watermark interlock, the `plan_hash` canonical
   ordering, the C2 activation predicate, or a fresh-DB bring-up that has no
   pending runtime steps (is 0020-last still well-defined when the only step is
   0020?).

3. **Serve-boot decoupling regresses an existing gate.** Lifting `ApplyMigrations`
   out of `ConnectAndMigrate` must not break the P2 watermark interlock
   (`owner.go` `CheckOwnerBundleWatermark`), the P3 drift gate /
   `RecordSchemaFingerprint` (`schema_drift.go`), or fresh-DB bring-up; no window
   where the daemon serves on an unmigrated schema.

4. **DDL-revocation lockout.** Show where revoking serving-role DDL (owner bundle
   ≥ 0020) recreates the #512-class lockout (the role that must run the deploy
   can't, across a restart), or breaks an existing boot/bootstrap path.

5. **Resumability / cursor / receipt defects.** Show a `deploy_cursor` state
   machine hole, an out-of-order apply under the plan's dependency edges, or a
   receipt written out of step with the real schema so `audit_log` provenance and
   the real schema disagree.

6. **Scope creep into P5 or boundary breach.** Show where the spec smuggles in P5
   (rehearsal/clone/expand-contract/fidelity tiering — Q1/Q2), breaches the
   local-first single-host/single-writer boundary, or is not shadow-first.

For each challenge record: the precise claim attacked, your concrete refutation
(with file:line / mechanism), the strongest rebuttal you can honestly construct on
the Holder's behalf, and whether a real gap remains. C3 and N1 are where to spend
most of your effort — an unresolved finding or an unproven resumability claim is a
standing falsification.
