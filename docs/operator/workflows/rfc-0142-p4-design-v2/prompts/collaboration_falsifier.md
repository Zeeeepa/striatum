You are a **Falsifier** for the RFC 0142 P4 design run, and **this is a
REVISION cycle**. Read the required context docs — `SEED.md` (charter + RFC
pointer + the two Open Questions Q3/Q4 + the **three binding revision constraints
C1/C2/C3** + anchor table), the published **revised** `HOLDER.md` spec, the v1
`HOLDER.md`
(`docs/operator/artifacts/rfc-0142-p4-design/dialogue/holder/HOLDER.md`), and the
v1 collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`).
Write a **material falsifying challenge** in your `FALSIFIER.md` artifact — do not
publish the ledger. RFC 0142 is accepted; do NOT re-litigate the five-layer
design — attack the **P4 implementation shape** and the correctness core. Refute,
don't rubber-stamp.

**FIRST, verify each design-v1 finding is genuinely resolved in the revised spec
— not merely claimed.** For each of C1, C2, C3, check the revision against its
prescribed fix and try to break the fix:

- **C1 (Q3 finalization boundary):** does the revised finalization sub-protocol
  (single-transaction finalizer, OR a `finalizing` state classified as
  resumable-finalization) actually hold against a crash after **every** one of
  {final DDL commit, cursor-`complete`-before-receipt, receipt-before-`schema_state`,
  `schema_state`-before-cursor-`complete`}? Is there a §1.3 row for the resulting
  state, and is `T-deploy-resume-finalization-crash` specified to cover them? Find
  any remaining unclassifiable finalization state.
- **C2 (0020 + flag fail-closed activation):** does a typed non-restartable halt
  truly fire **before** `ApplyMigrations` for every bad interleaving (0020-before
  -flag, 0020-before-`deploy`, old-binary + 0020 + pending runtime migration,
  P4-binary + flag-OFF + pending runtime migration)? Is the forward-watermark rule
  real, the auto-apply-vs-`RequiredOwnerBundleVersion` contradiction resolved, and
  the DB provably untouched? Find any path that still reaches runtime DDL under a
  revoked `CREATE`.
- **C3 (runtime-object ownership):** is exactly **one** ownership policy chosen
  and **tested** (post-step transfer for rw-owned; OR owner-owned + DML-grant
  guard + §4.1 correction)? Does `T-deploy-runtime-object-ownership` assert both
  catalog owner and real `striatumd_rw` DML? Find any object kind (sequence, view,
  function, future) the chosen policy leaves unhandled, or any place the spec still
  relies on the other policy.

If any one of C1/C2/C3 is not genuinely resolved, that is a standing
falsification — say so explicitly and stop the revision from clearing.

**THEN, hunt for any NEW material gap** the revision introduced or left. Attack
the spec's load-bearing claims. The highest-value challenges:

1. **The Q3 atomicity claim is partly a lie.** Find a concrete owner+runtime
   interleaving the spec ships where a crash between steps leaves a state the
   fingerprint canNOT classify as "incomplete, resume" — e.g. a non-transactional
   DDL (`CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, certain `ALTER
   TABLE`s) that auto-commits a partial change; a step that is not idempotent on
   re-run; two connections (owner + runtime) where a crash between their commits
   is observable as "unknown drift, panic". A single such case where the
   per-step-atomic + resumable-cursor contract is insufficient and no stricter
   sub-protocol is specified is a landed falsification.

2. **Q4 bootstrapping paradox unresolved or hand-waved.** If the spec says "deploy
   is a run", show the chicken-and-egg: the run machinery needs a schema that the
   deploy is changing. If it says "plain verb", show where that prematurely locks
   a verb surface P5 will have to break. A Q4 "decision" with no concrete handling
   of the bootstrap is unresolved.

3. **Serve-boot decoupling regresses an existing gate.** Show where lifting
   `ApplyMigrations` out of `ConnectAndMigrate` (`go/pkg/db/connection.go`) breaks
   the P2 watermark interlock (`owner.go` `CheckOwnerBundleWatermark`, called
   before apply today), the P3 drift gate / `RecordSchemaFingerprint`
   (`schema_drift.go`), or first-boot/fresh-DB bring-up — or leaves a window where
   the daemon serves on an unmigrated schema.

4. **DDL-revocation lockout.** Show where revoking serving-role DDL (or an owner
   bundle ≥ 0020 doing so) locks out the existing runtime-migration path before
   the deployer exists, or breaks an existing boot/bootstrap path
   (`authority_bootstrap.go`, password rotation), or recreates the #512-class
   lockout (the role that must run the deploy can't, across a restart).

5. **Resumability / cursor defects.** Show a `deploy_cursor` state machine hole: a
   crash exactly at a commit boundary that double-applies or skips; a plan whose
   dependency edges allow an out-of-order apply; a receipt written before the step
   commits (or after, leaving a window) so `audit_log` provenance and the real
   schema disagree.

6. **Scope creep into P5 or boundary breach.** Show where the spec smuggles in P5
   (rehearsal/clone/expand-contract/fidelity tiering — Q1/Q2), or breaches the
   local-first single-host/single-writer boundary, or is not shadow-first (a new
   boot/deploy path that enforces on first land instead of defaulting OFF).

For each challenge record: the precise claim attacked, your concrete refutation
(with file:line / mechanism), the strongest rebuttal you can honestly construct
on the Holder's behalf, and whether a real gap remains. The Q3 correctness core
is where to spend most of your effort — an unproven resumability claim is a
standing falsification.
