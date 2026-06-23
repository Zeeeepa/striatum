You are the **Committer** for the RFC 0142 P4 design run (**v4 / REVISION**). The
adjudicator's collaboration ledger has cleared the gate. Publish the final,
falsification-hardened **implementation spec** as your `PROPOSAL.md` artifact —
this is the design run's primary deliverable, the spec the `rfc-0142-p4-build` run
will build contract-first.

Start from the **revised (v4)** Holder's `HOLDER.md` and fold in every challenge
the adjudicator recorded as material-and-incorporated. Because this is the v4
revision run, the committed spec MUST carry **all five** fixes as first-class,
build-ready design — the two cycle-3 fixes resolved this cycle and the three
earlier fixes carried forward:

- **BC-N1 — the immutable-transcript per-step deploy receipt protocol** (the whole
  target deploy's ordered transcript — `base_owner_version`,
  `base_runtime_version`, target frontiers, every `{step_index, step_id, role,
  sha256}`, terminal-revoke placement — persisted in `deploy_cursor`/`deploy_plan`
  BEFORE step 0; resume loads the STORED transcript by `plan_hash` rather than
  recomputing the pending-delta plan; §1.3 and `doctor schema_deploy_unrecorded`
  key off the stored transcript so `(plan_hash, step_index)` is a stable identity)
  with `T-deploy-plan-hash-resume-after-step`.
- **BC-N2 — the pre-revoke `deploy_cursor`-authoritative serve edge** (every
  deployer-aware binary, INCLUDING the no-revoke-bundle binary at
  `applied_owner == 19`, reads `deploy_cursor` before `ApplyMigrations` and before
  `RecordSchemaFingerprint` and halts `awaiting_deploy` DB-untouched on a
  non-`complete` cursor — OR a durable pre-revoke activation marker) with the
  extended `F11` + `G-old-binary-refuse`.
- **C1 — the finalization-boundary sub-protocol** carried forward (the `finalizing`
  state + idempotent finalizer + the §1.3 resumable-finalization row +
  `T-deploy-resume-finalization-crash`).
- **C2 — the fail-closed revoke-bundle + `STRIATUM_DEPLOY_DECOUPLED` activation
  edge** carried forward (typed pre-`ApplyMigrations` halt + forward-watermark rule
  + `RequiredOwnerBundleVersion = 19` + `T-deploy-revoke-activation-ordering`).
- **C3 — the chosen, revoke-compatible runtime-object ownership policy** carried
  forward (revoke-last: the DDL-revoke bundle special-cased out of the owner prefix
  and excluded from `owner-ddl apply`, applied as the terminal plan step after every
  runtime ownership reconcile while `striatumd_rw` still holds CREATE;
  `T-deploy-runtime-object-ownership` / F12 asserting catalog owner + real
  `striatumd_rw` DML + post-deploy CREATE denial in a non-superuser two-role
  cluster; `G-revoke-last`). Re-anchor the DDL-revoke bundle to the renumbered
  `>= 0021` ordinal — 0020 is now `0020_owner_bundle_watermark_read.sql` and
  `LatestOwnerBundleVersion == 20`.

…and not regress Q4, the per-step Q3-A/Q3-B body, or the shadow-first decoupling
shape. The committed spec MUST:

- **Resolve Q3 and Q4** with the decided mechanism: the per-step-atomic +
  resumable-cursor contract and the `deploy_cursor` state machine (Q3), now
  including the immutable stored transcript (BC-N1) and the pre-revoke serve edge
  (BC-N2); and the plain-verb-vs-run decision with its handling of the
  bootstrapping paradox (Q4).
- **Name the exact surfaces:** the `striatum daemon deploy` command site
  (`go/pkg/cli/localcommands/daemon.go`); the deploy plan form (ordered,
  role-tagged, dependency-edged, with the stored immutable transcript and the C3
  terminal ordering for the renumbered DDL-revoke bundle); the new `deploy_cursor`
  (and any `deploy_plan`) runtime migration (≥ 0044, additive); the hash-chained
  deploy receipt into the owner-held `audit_log` (with the per-step receipt keyed on
  the stored-transcript `(plan_hash, step_index)`); the precise edit to
  `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify` that lifts
  `ApplyMigrations` out of serve-boot while keeping the P2 watermark interlock and
  P3 drift gate intact AND adds the pre-revoke `deploy_cursor` read; and the owner
  bundle (renumbered `>= 0021`) that revokes serving-role DDL — sequenced terminal
  per C3 so it never locks out the runtime path nor strands the deploy nor opens a
  pre-revoke serve window.
- **Specify the build slices in contract-first order** (smallest safe first), each
  with its named Go tests and the exact migration/owner-bundle additions. Be
  shadow-first: the new deploy path defaults OFF behind a flag; serve-boot
  auto-apply stays the behavior until the deployer is proven, then flips; additive
  migrations only; self-record before enforce.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire tests**: (a) kill `striatum daemon
  deploy` after a committed step and prove clean resume **with a stable
  `plan_hash`/`step_index` and exactly-once per-step receipt off the stored
  transcript** (BC-N1); (b) boot a no-revoke-bundle binary against an incomplete
  pre-revoke cursor at `applied_owner == 19` and prove it **does NOT mutate schema
  and does NOT self-record** (BC-N2); (c) prove the deploy receipt is hash-chained
  and `doctor` surfaces `schema_deploy_unrecorded` (including a missing per-step
  receipt); (d) in a two-role cluster, run the activation deploy through to
  `complete` and prove the runtime objects' ownership and the post-deploy
  `striatumd_rw` CREATE denial (C3).
- **Explicitly defer P5** (rehearsal receipt, expand/contract, fidelity tiering,
  clone mechanism = Q1/Q2) — P4 is the deployer + serve-boot decoupling + DDL
  revocation only. Note the verb-surface decisions that keep the P5 door open.
- Stay strictly inside the local-first product boundary (single-host, ONE
  Postgres, ONE daemon as the single writer).

Publish the spec only after confirming the ledger verdict cleared the gate.
