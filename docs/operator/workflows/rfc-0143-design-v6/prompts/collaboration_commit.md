You are the **Committer** for the RFC 0143 design run. The adjudicator's
collaboration ledger has cleared the gate. Publish the final, falsification-
hardened **implementation spec** as your `PROPOSAL.md` artifact — this is the design
run's primary deliverable, the spec the `rfc-0143-build` run will build
contract-first.

Start from the Holder's `HOLDER.md` and fold in every challenge the adjudicator
recorded as material-and-incorporated. The committed spec MUST:

- **Carry the BC1-W1-TOKEN resolution (the security cluster) in full:** a SINGLE
  consistent KERNEL start-token source for the connect-out W1 peer-credential check —
  a NAMED kernel start token captured via `ProcessStartToken(identity.PanePID)`
  (`/proc/<pid>/stat` field 22, `process_identity_linux.go:11-32`) **immediately
  after** `CaptureTmuxIdentity` reports the pane pid (`tmux_liveness.go:181-209`) and
  **before any control connection is accepted**, compared against the accepted peer's
  `ProcessStartToken(peer.pid)` (**kernel field-22 vs kernel field-22**, one clock
  domain), with tmux `#{pane_start_time}` kept ONLY as liveness metadata unless proven
  equivalent to `/proc` field 22 on supported hosts; and the real-path test
  `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` through `RunHelper` with
  `RequireTmux`/`RunAsUser` comparing `/proc/<peer-pid>/stat` field 22 to the captured
  `/proc/<pane-pid>/stat` field 22 PLUS a NEGATIVE that rejects the SAME pid with a
  mismatched/stale kernel start token (the pid-reuse guard).
- **Carry the v5-credited resolved set forward unregressed:** the **connect-out
  topology** + the named `HelperLaunchSpec` (`helper_protocol.go:27-39`) / `LaunchSpec`
  (`pty.go:30-42`) / `RunHelper` (`helper.go:128`, `:149-156`) plumbing sites that
  reach the PANE agentloop wrapper (not the tmux client); the **non-secret listener
  address**; the **daemon→wrapper post-auth nonce (W3)**; the **W2 ordering** +
  dumpable-before-dial; the **`#{pane_dead_status}` exit-code backstop** (the
  authenticated frame is the primary signal; the reserved 97/98 is observed on the
  tmux path via `#{pane_dead_status}`, never from `result.Cmd.Wait()` which resolves
  the attach client) + **C2**; BC2 (artifact identity from daemon `expected_artifacts`
  state), BC3 (`CapabilityReseal` a daemon-internal marker projected by
  `resealInFlightJob`, public route-alternate test-only), BC4 (the concrete
  `jobs.recovery_generation` column in owner bundle 0021, increment points, stamped
  value), BC5 (`leases.reseal_grace_extended_at` in the same owner bundle 0021 —
  `leases` owner-held — and the corrected `work.complete` skip/replace/replay
  lock-order gate map so expired-beyond-grace always routes the typed
  `session_unrecoverable_across_rotation` class, with
  `TestResealBeyondGraceRoutesTypedNotLeaseError` /
  `TestResealGraceCannotReviveRequeuedLease` /
  `TestRecoveryRequeueWinsOverExpiredLeaseReseal` / `GD-1b`), the daemon-observed
  positive intent + recovery-sweep backstop, the `ensureWorkSessionBackend` bypass
  (`TestResealExit98BypassesBackendGateOrRoutesTyped`), the W1/W2/W3 wall shapes, F2
  (no lane-readable reseal bearer), F4 (route-alternate records `reseal` not `write`),
  the F7 file-mirror half, AF1, AF4, and the A1–A18 discipline (incl. A3'/A4'/A7').
  Fold in the modified-since-baseline authored-path build-test (reuse
  `gitChangedPathSnapshots` / `collectInScopeAuthoredPaths` so an unchanged
  pre-existing expected path is NOT resealed —
  `TestResealRequiresAuthoredExpectedArtifactChange` or the positive
  `TestCodexResealUsesReceiverNotProviderStdout` case).
- **Carry the security invariant explicitly:** the new credential never carries
  `{admin, apply, recovery, surgical_recovery}`; no lane ever reads the bootstrap
  admin client-token; `CapabilityReseal` is never materialized into any lane-readable
  file; the no-replay property holds structurally on the connect-out channel **on the
  REAL tmux launch path with W1 specified as one coherent kernel identity token** (not
  on a direct-exec harness). State each as a falsifiable assertion + the named test
  that proves it.
- **Specify the build slices in contract-first order** (smallest safe first — Slice A
  the Option-4 legible-failure floor over the connect-out, same-uid-authenticated
  channel anchored through the real tmux launch path with the one-coherent-kernel-token
  W1, then Slice B the reseal mechanism), each with its named Go tests and the
  migration/owner-bundle changes (owner bundle 0021 for `jobs.recovery_generation` and
  `leases.reseal_grace_extended_at`; `LatestOwnerBundleVersion` 20→21). Apply the
  shadow-first convention for the risky new credential/boot path: new behavior
  defaults OFF behind an env flag; additive migrations only; self-record before
  enforce.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire test** (`GD-1` / `GD-1b`: restart the daemon
  mid-job and show the lane survives-and-reseals OR fails legibly-and-is-routed, with
  no silent unsealed exit and no elevated-capability exposure) and the **same-uid
  sibling-replay negative game-day run THROUGH THE TMUX LAUNCH PATH** (a non-child,
  non-wrapper same-uid sibling that connects to the same control address is rejected
  on the kernel field-22 pid+start-token, at any launch step).
- **Open with the maintainer-ratification banner:** Slice B is a security/authz
  trust-model change; the spec is a RECOMMENDATION the maintainer ratifies before the
  build lands credential code. State the recommended shape and the one-line security
  rationale up front; note Slice A is zero-trust-change but must route over the
  connect-out non-PTY channel whose same-uid authentication (W1) is **specified with
  one coherent kernel identity token** before it lands.
- Stay strictly inside the Non-Goals and the local-first product boundary.

Publish the spec only after confirming the ledger verdict cleared the gate.
