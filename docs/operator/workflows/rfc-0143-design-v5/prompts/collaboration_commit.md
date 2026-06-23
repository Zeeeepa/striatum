You are the **Committer** for the RFC 0143 design run. The adjudicator's
collaboration ledger has cleared the gate. Publish the final, falsification-
hardened **implementation spec** as your `PROPOSAL.md` artifact — this is the design
run's primary deliverable, the spec the `rfc-0143-build` run will build
contract-first.

Start from the Holder's `HOLDER.md` and fold in every challenge the adjudicator
recorded as material-and-incorporated. The committed spec MUST:

- **Carry the BC1-CHANNEL resolution (the security cluster) in full:** the concrete
  control-fd delivery + dumpability mechanism **ANCHORED THROUGH THE PRODUCTION
  tmux/sudo run-as/env-file launch path** — naming the exact `HelperLaunchSpec`
  (`helper_protocol.go:27-39`) / `LaunchSpec` (`pty.go:30-41`) / `RunHelper`
  (`helper.go:149-156`) plumbing sites that reach the PANE agentloop wrapper (not the
  tmux client), OR the explicit launch-topology change (e.g. connect-out: the pane
  wrapper calls `PR_SET_DUMPABLE(0)` FIRST, then connects OUT to a daemon-held
  listener authenticated via `SO_PEERCRED` uid+pid+start-time of the launched wrapper
  so a same-uid sibling is rejected and no fd is inherited through tmux); the
  preserved W2 ordering (no same-uid shim holds fd 3 or the nonce before
  `PR_SET_DUMPABLE(0)` is effective); and the real-path test
  `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` through `RunHelper` with
  `RequireTmux`/`RunAsUser` asserting together that the wrapper can send an accepted
  frame stamped with the launched wrapper pid+start-time (W1), the provider lacks fd
  3, and a non-child/non-wrapper same-uid sibling cannot open `/proc/<wrapper-pid>/fd/3`
  or recover the nonce at any point (W2/W3).
- **Carry the v4-credited resolved set forward unregressed:** BC2 (artifact identity
  from daemon `expected_artifacts` state), BC3 (`CapabilityReseal` a daemon-internal
  marker projected by `resealInFlightJob`, public route-alternate test-only), BC4 (the
  concrete `jobs.recovery_generation` column in owner bundle 0021, increment points,
  stamped value), BC5 (`leases.reseal_grace_extended_at` in the same owner bundle 0021
  — `leases` owner-held — and the corrected `work.complete` skip/replace/replay
  lock-order gate map so expired-beyond-grace always routes the typed
  `session_unrecoverable_across_rotation` class, with
  `TestResealBeyondGraceRoutesTypedNotLeaseError` /
  `TestResealGraceCannotReviveRequeuedLease` /
  `TestRecoveryRequeueWinsOverExpiredLeaseReseal` / `GD-1b`), C2 (the wrapper never
  propagates a provider child's 97/98 into the reserved agentloop codes), the
  daemon-observed positive intent + recovery-sweep backstop, the
  `ensureWorkSessionBackend` bypass (`TestResealExit98BypassesBackendGateOrRoutesTyped`),
  the W1/W2/W3 wall shapes, F2 (no lane-readable reseal bearer), F4 (route-alternate
  records `reseal` not `write`), the F7 file-mirror half, AF1, AF4, and the A1–A18
  discipline. Fold in the modified-since-baseline authored-path build-test (reuse
  `gitChangedPathSnapshots` / `collectInScopeAuthoredPaths` so an unchanged
  pre-existing expected path is NOT resealed —
  `TestResealRequiresAuthoredExpectedArtifactChange` or the positive
  `TestCodexResealUsesReceiverNotProviderStdout` case).
- **Carry the security invariant explicitly:** the new credential never carries
  `{admin, apply, recovery, surgical_recovery}`; no lane ever reads the bootstrap
  admin client-token; `CapabilityReseal` is never materialized into any lane-readable
  file; the no-replay property holds structurally on the BC1-CHANNEL channel **on the
  REAL tmux launch path** (not on a direct-exec harness). State each as a falsifiable
  assertion + the named test that proves it.
- **Specify the build slices in contract-first order** (smallest safe first — Slice A
  the Option-4 legible-failure floor over the same-uid-authenticated channel anchored
  through the real tmux launch path, then Slice B the reseal mechanism), each with its
  named Go tests and the migration/owner-bundle changes (owner bundle 0021 for
  `jobs.recovery_generation` and `leases.reseal_grace_extended_at`;
  `LatestOwnerBundleVersion` 20→21). Apply the shadow-first convention for the risky
  new credential/boot path: new behavior defaults OFF behind an env flag; additive
  migrations only; self-record before enforce.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire test** (`GD-1` / `GD-1b`: restart the daemon
  mid-job and show the lane survives-and-reseals OR fails legibly-and-is-routed, with
  no silent unsealed exit and no elevated-capability exposure) and the **same-uid
  sibling-replay negative game-day run THROUGH THE TMUX LAUNCH PATH** (a non-child,
  non-wrapper same-uid sibling cannot open the wrapper's control fd or connect-out
  socket as the wrapper, at any launch step).
- **Open with the maintainer-ratification banner:** Slice B is a security/authz
  trust-model change; the spec is a RECOMMENDATION the maintainer ratifies before the
  build lands credential code. State the recommended shape and the one-line security
  rationale up front; note Slice A is zero-trust-change but must route over a real
  non-PTY channel with the same-uid authentication **anchored through the production
  tmux/sudo/env-file launch path** before it lands.
- Stay strictly inside the Non-Goals and the local-first product boundary.

Publish the spec only after confirming the ledger verdict cleared the gate.
