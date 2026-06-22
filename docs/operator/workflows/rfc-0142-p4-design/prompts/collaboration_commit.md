You are the **Committer** for the RFC 0142 P4 design run. The adjudicator's
collaboration ledger has cleared the gate. Publish the final, falsification-
hardened **implementation spec** as your `PROPOSAL.md` artifact — this is the
design run's primary deliverable, the spec the `rfc-0142-p4-build` run will build
contract-first.

Start from the Holder's `HOLDER.md` and fold in every challenge the adjudicator
recorded as material-and-incorporated. The committed spec MUST:

- **Resolve Q3 and Q4** with the decided mechanism: the per-step-atomic +
  resumable-cursor contract and the `deploy_cursor` state machine (Q3), including
  any stricter single-connection/single-transaction sub-protocol for the specific
  step classes that need it; and the plain-verb-vs-run decision with its handling
  of the bootstrapping paradox (Q4).
- **Name the exact surfaces:** the `striatum daemon deploy` command site
  (`go/pkg/cli/localcommands/daemon.go`); the deploy plan form (ordered,
  role-tagged, dependency-edged); the new `deploy_cursor` runtime migration
  (≥ 0044, additive); the hash-chained deploy receipt into the owner-held
  `audit_log`; the precise edit to `go/pkg/db/connection.go` `ConnectAndMigrate`
  that lifts `ApplyMigrations` out of serve-boot while keeping the P2 watermark
  interlock and P3 drift gate intact; and the owner bundle (≥ 0020) that revokes
  serving-role DDL — sequenced so it never locks out the runtime path before the
  deployer exists.
- **Specify the build slices in contract-first order** (smallest safe first),
  each with its named Go tests and the exact migration/owner-bundle additions.
  Be shadow-first: the new deploy path defaults OFF behind a flag; serve-boot
  auto-apply stays the behavior until the deployer is proven, then flips; additive
  migrations only; self-record before enforce.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire tests**: (a) kill `striatum daemon
  deploy` after a committed step and prove clean resume; (b) boot the daemon with
  a pending plan and prove it does NOT mutate schema; (c) prove the deploy receipt
  is hash-chained and `doctor` surfaces `schema_deploy_unrecorded`.
- **Explicitly defer P5** (rehearsal receipt, expand/contract, fidelity tiering,
  clone mechanism = Q1/Q2) — P4 is the deployer + serve-boot decoupling + DDL
  revocation only. Note the verb-surface decisions that keep the P5 door open.
- Stay strictly inside the local-first product boundary (single-host, ONE
  Postgres, ONE daemon as the single writer).

Publish the spec only after confirming the ledger verdict cleared the gate.
