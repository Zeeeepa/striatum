You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS THE THIRD
REVISION (v3)**. Two prior design runs ran this same falsification gate. v1
(`rfc-0142-p4-design`) returned `needs_revision` with three findings C1/C2/C3. v2
(`rfc-0142-p4-design-v2`) **resolved C1 and C2** — both falsifiers conceded the
finalization-boundary shape (C1) and the fail-closed activation edge (C2) are
genuinely closed — but returned `needs_revision` **again** because **C3
(ownership policy) is still open** and a **new finding N1 (per-step receipt not
crash-safe)** landed.

**Start from the v2 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/holder/HOLDER.md`). Your
job is to REVISE that spec, not write a new one from scratch. The full C3/N1
analysis and the exact prescribed fixes are in the **v2 collaboration ledger**
(also a required context doc:
`docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— the `findings:` block for C3 and N1, and §4 "What the revision must fix"); read
it in full. `SEED.md` pins the two binding constraints (C3 + N1) and the section
"Already cleared — carry forward, do NOT regress" (C1 + C2).

Your revised spec **MUST resolve BOTH cycle-2 findings (C3 + N1) per their
prescribed fix**, and **MUST carry forward C1 and C2 unregressed**. A revision
that leaves C3 or N1 open — or that merely *claims* a fix without a concrete
sub-protocol / chosen-and-tested policy — or that regresses C1 or C2, has NOT
cleared the gate. This is the gate's single allowed revision cycle, so the
cycle-3 falsifiers re-attack each finding specifically and a second
`needs_revision` ends the gate unCleared.

Read the required context docs in full first — `SEED.md`, the v2 `HOLDER.md`, and
the v2 collaboration ledger — plus the committed RFC
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258). Build on the exact anchors and file:line references the v2
spec and the SEED anchor table use; re-verify them.

Publish the **revised (v3)** falsifiable implementation spec for RFC 0142 **P4 —
the one-shot deployer** as your `HOLDER.md` artifact. Make it concrete and
falsifiable, not a restatement of the RFC. Open with an auditable resolution map
(an "Addressing the design-v2 findings" subsection) so the falsifiers can verify
C3 and N1 are resolved and C1/C2 are preserved, rather than infer it.

Hold the root reframe: **schema mutation must stop being an implicit side effect
of the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving
daemon can hold zero DDL privilege and a bad migration can never wedge the single
writer on boot.

Your spec MUST:

0. **Resolve both binding revision constraints — the gating requirement.**

   - **C3 (ownership vs the bundle-0020 CREATE revoke — resolve the
     contradiction).** The v2 spec chose Policy 1 (`ALTER … OWNER TO
     striatumd_rw`), but that transfer requires the new owner to hold `CREATE ON
     SCHEMA striatumd`, which bundle 0020 REVOKEs — so post-0020 the reconcile
     fails `permission denied for schema striatumd` (the repo's own bundles
     0018/0019 GRANT `CREATE` FIRST for exactly this reason:
     `go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:58-72,97-102`;
     `0019_supervisor_pointer_runtime_ownership.sql:53-80`). Pick and **fully
     specify ONE coherent resolution** with a test (the SEED recommends **(a)**):
     - **(a) Sequence the revoke last** — apply the bundle-0020 `REVOKE CREATE ON
       SCHEMA striatumd FROM striatumd_rw` as the **FINAL step** of the deploy
       plan, AFTER all runtime steps + ownership reconciles complete, so
       `ALTER … OWNER TO striatumd_rw` succeeds during the deploy (CREATE still
       held) and the steady state still denies CREATE. Specify how the plan
       generator special-cases bundle 0020 to sort to the END (not into the
       owner-prefix), and how the `deploy_cursor` indices, the C2
       `CheckDeployActivation` predicate, the §1.3 in-sync classification, and the
       `plan_hash`/fingerprint binding stay coherent with 0020 as the terminal
       step (the deploy is `complete` only once 0020 has committed); OR
     - **(b) Scoped temporary grant** — the deployer (owner connection) does
       `GRANT CREATE … TO striatumd_rw` immediately before the ownership-transfer
       loop and `REVOKE` immediately after, within the deploy, with a test proving
       (i) the post-deploy steady state denies `striatumd_rw` CREATE and (ii) the
       daemon does not serve while the grant is live; OR
     - **(c) Policy 2** — owner/admin OWNS new runtime objects + a build/load
       guard that every runtime migration grants the exact DML `striatumd_rw`
       needs, and **correct §4.1 to drop the runtime-ownership safety claim**.
     Resolve the v2 F12 internal inconsistency. Add
     **`T-deploy-runtime-object-ownership`** asserting, in a documented
     non-superuser two-role cluster: the catalog owner of every created object
     (table, its index, its sequence) under the chosen policy; the serving role's
     real DML under `striatumd_rw`; AND the **post-deploy CREATE denial**
     (`SET ROLE striatumd_rw; CREATE TABLE → 42501`). Re-run and assert idempotence.

   - **N1 (per-step receipt crash-safety).** The per-step receipt is appended over
     the OWNER connection (`append_audit_row`, owner-only) and so cannot share a
     cross-connection transaction — the same two-connection constraint as C1. A
     crash between a step's commit and its receipt-write must be reconciled
     **idempotently** — mirror the C1 finalizer: an **idempotent per-step receipt
     reconcile keyed on `(plan_hash, step_index)`** (and `step_id`/`sha256`) that
     the resume/finalizer completes on re-run, so every applied step provably ends
     with **exactly one** hash-chained receipt and `doctor
     schema_deploy_unrecorded` is green **only when all committed steps have
     receipts**. Specify:
     - **Q3-A (transactional step):** the step receipt append occurs in the **same
       owner-connection transaction** as the DDL + ownership reconcile + grants +
       version stamp + cursor advance (legal — the deployer applies runtime steps
       over the owner connection and `append_audit_row` runs in the caller's
       transaction); name that connection/role. `step_committed(k)` iff receipt
       durable.
     - **Q3-B (non-transactional step):** the `in_progress(k)` reconciler
       idempotently appends exactly one receipt keyed on `(plan_hash, step_index)`
       **before** writing `step_committed(k)`, resolving duplicate-vs-skip.
     - Tighten `doctor schema_deploy_unrecorded` so a missing per-step receipt is
       surfaced, not masked by a present terminal receipt.
     Add **`T-deploy-receipt-crash-resume`** killing between step-commit and
     receipt-write (and at the other crash points) and asserting exactly-once
     receipt per applied step on re-run, one terminal `complete` receipt, final
     schema equality, and a green doctor.

   Explicitly call out, in the revised spec, **how** C3 and N1 are now closed
   (which C3 resolution a/b/c you pinned and why), and **confirm** C1 (the
   `finalizing` state + idempotent finalizer + §1.3 row + F10) and C2
   (`CheckDeployActivation` before `ApplyMigrations`, the typed halts, the
   forward-watermark rule, `RequiredOwnerBundleVersion` kept at 19, F11) are
   carried forward **verbatim from the v2 HOLDER** and not regressed. Keep C1 and
   N1 coherent — they are the same atomic-or-idempotent provenance discipline at
   the terminal vs the per-step boundary.

1. **Keep Q3 and Q4 resolved.** Q3 — the per-step-atomic + resumable-cursor
   contract (now including the per-step receipt, N1) is sufficient for every
   owner+runtime interleaving P4 ships; the `deploy_cursor` states and crash-resume
   semantics are precise. Q4 — plain verb now with the three run-shape seams.
   Carry both forward; do not re-litigate.

2. **Keep the deployer surface and the serve-boot decoupling intact** (carry
   forward from v2): the `striatum daemon deploy` command site
   (`go/pkg/cli/localcommands/daemon.go`); the embed-FS-derived deploy plan (with
   the C3 ordering change if you pick (a)); the `deploy_cursor` runtime migration
   (≥ 0044); the hash-chained deploy receipt into the owner-held `audit_log`; the
   lift of `ApplyMigrations` out of `connection.go` `ConnectAndMigrate` /
   `ConnectAndVerify` with the P2 watermark interlock and P3 drift gate intact.

3. **Keep the serving-role DDL revocation (owner bundle ≥ 0020)** — but with the
   C3 resolution applied (if (a), 0020 is the plan's terminal step). State exactly
   how it ships without lockout, now that the C3 contradiction is removed.

4. **State each load-bearing claim as a falsifiable assertion + its named test /
   game-day step.** Carry F1–F11 forward (re-confirm), and ensure the C3 and N1
   tests are present and sharp: `T-deploy-runtime-object-ownership` (catalog owner
   + real DML + post-deploy CREATE denial in a two-role cluster) and
   `T-deploy-receipt-crash-resume` (exactly-once per-step receipt across the
   step-commit/receipt-write crash boundary).

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone = Q1/Q2).
   Shadow-first for the new path: default OFF behind a flag, additive migrations
   only, self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
