You are the **Adjudicator** for the RFC 0142 P4 design run, and **this is the
FOURTH revision cycle (v4)**. Read only the curated dialogue trajectory (the
**revised (v4)** Holder's `HOLDER.md` spec and the two falsifiers' `FALSIFIER.md`
challenges) plus the `SEED.md` charter, with the **v3** `HOLDER.md` and the **v3**
collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/...` — its §4 "What the
revision must fix") as context for what the revision had to fix. Publish a
`collaboration_ledger` artifact whose verdict reflects whether (a) the **two
cycle-3 findings BC-N1 + BC-N2 are genuinely resolved** in the revised spec, (b)
the already-cleared findings **C1 + C2 + C3 are carried forward intact (not
regressed)**, and (c) no **new** material challenge landed and stood unrebutted.
RFC 0142 is accepted; judge the P4 implementation shape, not the five-layer design.

**A clearing verdict (`accept` / `accept_with_findings`) REQUIRES all of: BC-N1
genuinely resolved, BC-N2 genuinely resolved, C1 intact, C2 intact, C3 intact, and
no new material challenge standing.** If BC-N1 or BC-N2 is still open — or a
falsifier shows the prescribed fix is only claimed, not actually implemented as a
concrete sub-protocol — or if C1, C2, or C3 has been regressed, the verdict is
`needs_revision` (note: the workflow allows only **one** revision cycle, so a
second `needs_revision` ends the gate unCleared; judge accordingly and be exact).

Specifically:

- **BC-N1 is resolved only if** the spec materializes the IMMUTABLE ordered
  transcript (`base_owner_version`, `base_runtime_version`, target frontiers, every
  `{step_index, step_id, role, sha256}`, terminal-revoke placement) and persists it
  in `deploy_cursor` (or a `deploy_plan` table) **BEFORE step 0 mutates the
  frontier**; resume loads the STORED transcript by `plan_hash` rather than
  recomputing the pending-delta `BuildPlan(current_owner, current_runtime)` over the
  moved live frontiers; §1.3 classifies an incomplete cursor whose `plan_hash` is
  not the binary's freshly-computed pending plan as a recoverable resume state (not
  an unclassified drift bucket); the per-step `doctor schema_deploy_unrecorded`
  enumerates applied steps from the stored transcript; and
  `T-deploy-plan-hash-resume-after-step` kills after step 0 commits (and after step
  1) and asserts the re-run reuses `plan_hash`, preserves the `step_index`es,
  recognizes prior receipts, and ends green. The v3 break (a re-run rebuilding
  `H' != H` over the moved frontier and renumbering `step_index`) must be provably
  closed.
- **BC-N2 is resolved only if** `deploy_cursor` is made authoritative before the
  terminal revoke: **every** deployer-aware binary — **including the
  no-revoke-bundle landing binary at `applied_owner == 19`** — reads `deploy_cursor`
  before `ApplyMigrations` and before `RecordSchemaFingerprint` and halts
  `awaiting_deploy` DB-untouched on a non-`complete` cursor, regardless of
  `revokeEmbedded`/forward-watermark; OR a durable pre-revoke activation marker
  halts no-revoke binaries at watermark 19. `F11` must be extended to assert
  `ApplyMigrations` NOT called, `RecordSchemaFingerprint` NOT called, DB
  byte-identical, and an `awaiting_deploy` halt in that window; `G-old-binary-refuse`
  must be extended to prove the pre-revoke window cannot be served. The fix must NOT
  regress C3 revoke-last (no block on completion; no stranded
  `ALTER … OWNER TO striatumd_rw`).
- **C1 / C2 / C3 intact only if** the new stored-transcript receipt rule stays
  coherent with the `finalizing` finalizer + §1.3 table (no resume serves; terminal
  `complete` receipt remains exactly-once); the BC-N2 pre-revoke edge keeps the C2
  fail-closed `CheckDeployActivation` edge, typed halts, forward-watermark rule, and
  `RequiredOwnerBundleVersion = 19`; and C3 (the DDL-revoke bundle, re-anchored to
  the renumbered `>= 0021` ordinal since 0020 is now
  `0020_owner_bundle_watermark_read.sql` / `LatestOwnerBundleVersion==20`,
  special-cased + terminal + excluded from `owner-ddl apply`, F12/`G-revoke-last`)
  is preserved with an actually-completable activation deploy.

Record in the ledger, per finding BC-N1 / BC-N2 / C1 / C2 / C3 **and** per new
falsifier challenge: the claim challenged, whether it was material (would change the
spec or expose a real correctness defect), whether the revised spec resolves/rebuts
it or it stands unrebutted, and the disposition. Explicitly state, for each of
BC-N1, BC-N2, C1, C2, C3, whether it is RESOLVED / INTACT.

Verdict guidance:

- **needs_revision** if BC-N1 or BC-N2 remains open, C1/C2/C3 is regressed, or any
  new material challenge stands unrebutted — especially: a concrete owner+runtime
  (or per-step-receipt / pre-revoke-serve) interleaving where the per-step-atomic +
  resumable-cursor contract is insufficient and no stricter sub-protocol is
  specified (the Q3 correctness core — this alone forces needs_revision); a receipt
  key still unstable across resume; a no-revoke binary that can still serve a
  pre-revoke incomplete deploy; a serve-boot decoupling that regresses P2/P3 or
  fresh-DB bring-up; or scope creep into P5 / a non-shadow-first new path. Say
  exactly what the revision must fix.
- **accept** / **accept_with_findings** only if **BC-N1 and BC-N2 are both
  genuinely resolved** (BC-N1 the immutable stored transcript + stable
  `(plan_hash, step_index)` across resume + `T-deploy-plan-hash-resume-after-step`;
  BC-N2 the pre-revoke `deploy_cursor`-authoritative edge for no-revoke binaries +
  extended `F11` / `G-old-binary-refuse`), **C1, C2, and C3 are carried forward
  intact**, **every new material challenge was directly rebutted or incorporated**,
  **Q3 and Q4 remain resolved with a concrete mechanism**, the serve-boot decoupling
  provably preserves P2/P3 and fresh-DB bring-up, and each load-bearing claim
  carries a named falsifying test / game-day step. A clearing verdict is `accept` or
  `accept_with_findings`, never the literal word `clear`. A spec that merely
  *claims* the two fixes without the concrete immutable-transcript resume protocol /
  pre-revoke serve edge has NOT cleared the gate.

The ledger verdict — not falsifier completion — clears the phase gate.
