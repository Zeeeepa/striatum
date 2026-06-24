---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: adjudicator-reviewer-001
title: "RFC 0142 P4 final collaboration summary"
run_id: "run_365daa96ebcaa61f7b33175cdf3e9abe"
status: accept_with_findings
cycle: 1
inputs:
  - "docs/operator/workflows/rfc-0142-p4-design-v9/SEED.md"
  - "docs/rfcs/0142-safe-by-construction-database-change-deployment.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/falsifier_1/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/falsifier_2/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/commit/proposal/PROPOSAL.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/holder/HOLDER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
---

# RFC 0142 P4 Final Collaboration Summary

author: adjudicator-reviewer-001

## Verdict

verdict: accept_with_findings

The RFC 0142 P4 design run clears for downstream build publication. The current v9 collaboration ledger records `accept_with_findings`, which is a clearing verdict for this workflow. The gate clears because the single binding v8 finding, M7, is resolved by deriving the row-16 complete/decoupled/revoke-embedding boot-path cell from A's fingerprint-sync predicate instead of asserting it from owner-watermark reachability.

The accepted finding is non-blocking and build-phase only: the implementation and verification runs must prove the row-16 in-sync/out-of-sync cases and expand the grouped cursor-state shorthand into concrete enum coverage. Those obligations are folded into the committed proposal as binding acceptance criteria, not left as loose review notes.

## Gate Record

| stage | disposition | effect |
| --- | --- | --- |
| v8 collaboration ledger | `needs_revision` | M7 stood: row 16 was asserted unconditionally even though A's decoupled complete branch is fingerprint-conditional and does not read `applied_owner` or `revokeEmbedded`. |
| v9 holder revision | revised spec | The holder adopted Option 1: row 16 `==0`, `==20`, and `>=21` are conditional `SERVE-verify if in-sync, else awaiting_deploy`, identical to row 15. |
| v9 falsifier pass | no material blocker | Both falsifiers independently conceded M7 was resolved and found no carry-forward regression across M6, M5, M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, or C3. |
| v9 adjudication | `accept_with_findings` | The collaboration gate clears; finding B1 is deferred to the build run as mandatory test coverage and game-day evidence. |
| commit proposal | published | `docs/operator/artifacts/rfc-0142-p4-design-v9/commit/proposal/PROPOSAL.md` is the build-ready downstream publication artifact. |

## Publication Summary

`PROPOSAL.md` is the committed P4 implementation spec consumed by `rfc-0142-p4-build`. It folds the v9 holder, the falsifiers' conceded challenges, and the adjudicator's B1 build obligations into one contract-first artifact. RFC 0142 itself is already accepted by D258; this run does not reopen the five-layer design. It pins P4: the one-shot deployer, deploy cursor and transcript model, serve-boot decoupling, owner-bundle watermark behavior, and serving-role DDL revocation.

The proposal explicitly keeps P5 out of scope. Rehearsal, fidelity tiers, full-data clone behavior, expand/contract reshape, and the clone-backed Q1/Q2 work remain deferred. The local-first boundary also stays intact: one host, one PostgreSQL instance, one daemon as single writer, and no hosted service or external persistence.

## What Cleared

M7 is resolved. The v9 spec makes the decoupled complete branch independent of both `applied_owner` and `revokeEmbedded` after step 0 is skipped, so all A-reaching complete-row cells are derived from the same fingerprint-sync predicate. F18 is now parametric over the seven A-reaching complete-row cells: 13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, and 16/`>=21`, with both in-sync and out-of-sync sub-cases.

The carry-forward set remains intact. The adjudicator source-verified that `RequiredOwnerBundleVersion` remains 20, the fresh `applied_owner == 0` path still serves before the shortfall halt, `schema_state` is orthogonal to `owner_bundle_meta`, and the serve-boot mutation order still runs W before the legacy mutation/fingerprint path. The row-16 fix does not regress the fresh-DB serve, rows 13/15, the M3 config gate, the BC-N2 pre-revoke edge, the transcript verification/finalizer scope, the non-revoke filter, or the revoke-last mechanism.

The decision table is treated as executable, not descriptive prose. The build must preserve the W and A derivation, the four-cell `:399` self-recording spy list, the legitimate fresh-DB and inert-landing serve cases, and the typed halt behavior for configuration, deployment, owner-DDL, transcript, and DB-stamp mismatches.

## Findings Carried Downstream

Finding B1 is accepted with owner `rfc-0142-p4-build`.

B1.1: `T-deploy-bootpath-decision-table` must construct the row-16 in-sync and out-of-sync sub-cases for `applied_owner == 0`, `==20`, and `>=21`. The in-sync arm must set `schema_state.fingerprint == ExpectedFingerprint()` and `cursor.plan_hash == expected` over owner metadata states absent, 20, and `>=21`, then prove serve verify-only without firing `ApplyMigrations` or `RecordSchemaFingerprint` spies.

B1.2: the grouped "64-cell" shorthand is not sufficient for implementation verification. Tests must table-drive each concrete cursor-state enum named by F18: `none`, `in_progress`, `step_committed`, `finalizing`, `complete`, and `aborted`.

## Downstream Contract

`rfc-0142-p4-build` should implement the proposal contract-first. The build must create the deployer verb, durable `deploy_plan` and `deploy_cursor` model, stable `plan_hash`/`step_index` receipts, idempotent finalizer, transcript byte and DB-stamp verification, non-revoke owner-DDL filter, deploy activation interlock, forward-watermark rule, and 0021 revoke-last choreography.

The implementation run must also keep the game-day criteria from proposal section 6.5 green: crash-resume from stored transcript, divergent-binary resume refusal, old-binary pre-revoke refusal, self-heal without early revoke, complete-cursor flag-off refusal, fresh-DB serve plus shortfall halt, no-revoke and revoke-embedding complete-row in-sync/out-of-sync cases, hash-chained receipt plus doctor, and two-role activation with post-deploy CREATE denial.

`rfc-0142-p4-verify` should treat B1 as mandatory evidence, not an optional regression test. A green verify pass must demonstrate that M7 was not merely repaired in prose: row 16 must be executable, independently constructed, and proven against the same predicate inputs the design claims.

## Closing Note

The collaboration gate did its job: it forced the final complete-row coherence gap into the open, required the spec to derive the row from source-backed predicates, and carried the remaining risk into explicit build and verification obligations. The downstream publication is valid because `PROPOSAL.md` makes those obligations part of the implementation contract.
