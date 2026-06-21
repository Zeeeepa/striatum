# RFC 0142 design-committee output (captured from a banked run)

These four artifacts are the deliverable of the **RFC 0142 P0 design-committee**
(`falsification_gate`), the Stage 1 design-run of the RFC 0142 pipeline. They are
preserved here as durable provenance and as the **build-ready input for the Stage 2
implementation run**.

## Provenance

- Run: `run_468d26b3c2745cc0aae764070b3d2e53` (`fg_rfc0142_design`), **banked
  (canceled)** after delivering its design substance.
- Topology: **claude** holder/adjudicator + **agy (gemini)** falsifiers — genuine
  cross-model falsification. The adjudicator ran on the author lane, independent of
  the falsifiers (same-model-pairing rule).
- The `holder` + both `falsifier` artifacts were committed on the (local) run
  branch; the `adjudicator` `collaboration_ledger` was **registered but never
  git-anchored to the run branch** by the runner — defect
  [#551](https://github.com/halbritt/striatum/issues/551). The `commit_proposal`
  job correctly escalated a `human_checkpoint` rather than fabricating a proposal
  blind to the ledger, so the run was banked and the artifacts captured here.
- These files are copied verbatim from the run; they are NOT a hand-authored
  substitute for the run's success (the run is recorded canceled, #551 is open).

## Contents

- `HOLDER.md` — the build-ready P0 spec (the claim under test): files to change,
  the two-role fixture design, the red regression test, the green control, and
  consistency with the existing static guard.
- `FALSIFIER_1.md`, `FALSIFIER_2.md` — the cross-model (agy) falsifications.
- `COLLABORATION_LEDGER_cycle_1.md` — the adjudicator verdict
  (**`accept_with_findings`**) with findings F1–F6 and **binding constraints
  C1–C5** the P0 build must discharge.

## Binding constraints the P0 build MUST discharge (from the ledger)

- **C1 (RESET ROLE escape):** Phase B (system-under-test) MUST run through a
  connection whose login user is a dedicated **non-superuser, non-owner LOGIN
  role** that is not a member of and does not inherit from the owner role — NOT
  `SET ROLE striatumd_rw` inside a privileged connection. Gate: `RESET ROLE; ALTER
  TABLE striatumd.events …` must still fail `42501`.
- **C2 (bootstrap ownership fidelity):** Phase A bootstrap MUST reproduce prod's
  actual per-table `relowner` — recent runtime-migration-created tables
  (`supervisor_buffered_packets`/0038, `event_chain_segments`/0041,
  `verifier_attestations`/0042) MUST be `striatumd_rw`-owned in the fixture. Gate:
  a differential test asserts live `pg_class.relowner` matches the
  static-guard-derived runtime-owned set; a legal runtime `ALTER` on each succeeds
  while an owner-held `ALTER` `42501`s.
- **C3 (non-superuser bootstrap):** Phase A MUST succeed under a **non-superuser
  owner DSN** — provision the transfer membership (`GRANT striatumd_rw TO
  CURRENT_USER` before `ApplyOwnerBundles`) and **REVOKE it before Phase B**; do
  not silently depend on a superuser DSN.
- **C4 (inheritance-leak self-check):** The fixture self-check MUST assert, at
  probe time (after C3's grant is revoked), that the SUT role is **not a member of
  and does not inherit from** the owner role (`pg_has_role` / `pg_auth_members`),
  in addition to `rolsuper=false` and `relowner(events)` ≠ SUT role; abort loudly
  otherwise so a red `42501` is trusted only when isolation holds.
- **C5 (search_path fidelity, low):** Pin the Phase B SUT connection's
  `search_path` (and prod session GUCs) to prod's value.

## Deferred (NOT a P0 constraint)

- **F6 (dual-deploy-path divergence):** out-of-band admin-apply would create
  runtime tables owner-owned, diverging from the boot path. No current out-of-band
  runtime-migration-as-owner path exists; this is a property of the future Layer
  3/4 deployer and is deferred there. P0's oracle is honest for the boot model
  once its scoping is stated.
