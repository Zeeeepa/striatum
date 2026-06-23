---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 lane credential survival across a daemon boot-epoch rotation — falsifiable implementation spec (design-v6 REVISION; resolve the single remaining binding constraint BC1-W1-TOKEN)"
participants:
  - "holder-author-002"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-002"
    refs: ["dialogue:1"]
    text: "v6 is a proper revision starting from the v5 HOLDER spec; it resolves the single remaining binding constraint BC1-W1-TOKEN in ONE place and carries the entire v5-credited resolved set forward unregressed. BC1-W1-TOKEN (the v5 ground): the v5 W1 peer-credential check compared ProcessStartToken(peer.pid) — the kernel /proc/<pid>/stat field-22 start-tick — against identity.PaneStartToken, which CaptureTmuxIdentity sources from tmux #{pane_start_time} (a wall-clock unix timestamp) whenever numeric — two categorically different clock domains. v6 pins ONE coherent KERNEL start-token source: it adds LaunchResult.PaneKernelStartToken (pty.go:47-53), captures it via paneKernelStartToken, ok := ProcessStartToken(identity.PanePID) in launchPTY (pty.go:493-504) immediately after CaptureTmuxIdentity reports the pane pid and before any control connection is accepted, and W1 compares the accepted peer's ProcessStartToken(peer.pid) to result.PaneKernelStartToken by feeding the captured kernel token into PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken) (tmux_liveness.go:392-408) — field-22 on both sides, one clock domain. tmux #{pane_start_time} is kept ONLY as liveness metadata (identity.PaneStartToken unchanged, used by ProbeTmuxLiveness), never as the W1 operand, and is NOT claimed equivalent to /proc field 22. v6 adds a fail-closed empty-token rule: because PIDLiveWithStartToken(pid, '') skips the token comparison and returns live on pid alone (tmux_liveness.go:397), RunHelper asserts result.PaneKernelStartToken != '' before accepting any control connection; an empty captured token accepts NO connection and the floor is reached via the #{pane_dead_status}/recovery-sweep backstops (Falsifier A3''). The real-path test TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper (RunHelper + RequireTmux/RunAsUser) is extended to compare /proc field 22 on both sides, plus a NEGATIVE rejecting the same pid with a mismatched/stale kernel start token (the pid-reuse guard, A3') and a NEGATIVE for the empty captured token (A3''). Carries forward unregressed: the connect-out topology + named plumbing sites, the non-secret address + post-auth nonce (W3), the W2 ordering + dumpable-before-dial, the #{pane_dead_status} exit-code backstop + C2, BC2, BC3, BC4, BC5, the daemon-observed positive intent, the ensureWorkSessionBackend bypass, the W1/W2/W3 wall SHAPES, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1-A18 assertion discipline, and the modified-since-baseline authored-path build-test."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "v6 genuinely fixes the exact v5 clock-domain defect: the W1 operand is no longer tmux #{pane_start_time}; it names PaneKernelStartToken, captures it via ProcessStartToken(identity.PanePID), compares the accepted peer's ProcessStartToken(peer.pid) to that captured kernel token, keeps tmux #{pane_start_time} as liveness metadata only, and adds the right same-pid stale-token and empty-token negatives. But one NEW material gap remains inside the load-bearing W1 no-replay proof on the real tmux path: the proposed 'launch-time' kernel token is still captured only AFTER tmux respawn-pane has already started the pane command (pty.go:479-484) and after a separate CaptureTmuxIdentity round trip (pty.go:493-504); ProcessStartToken reads whatever process currently owns /proc/<pid>/stat field 22 at the time of the later read (process_identity_linux.go:13-32). The spec does not require the helper to fail closed if the launched pane has already died before the kernel token is stamped. In the same-uid adversarial model, a fast-exiting wrapper plus pid reuse before the post-launch /proc read makes the helper capture the ATTACKER's field-22 token as if it belonged to the launched wrapper; W1 then accepts the first connection from that reused pid because both peer.pid == result.PID and PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken) hold, the sibling receives the post-auth nonce, and it can emit a daemon-authenticated control frame into resealInFlightJob (a primary entry point). The v6 same-pid stale-token negative does NOT cover this: it proves W1 rejects when the captured token is stale relative to the peer; this race is the INVERSE — the captured token is fresh, but fresh for the wrong process because capture happened after the launched wrapper died. The empty-token fail-closed guard also misses it (the reused process is live, so ProcessStartToken returns a NON-empty token). A design cannot call a token 'launch-time' merely because it is read promptly after launch; for a security/authz gate, structural no-replay cannot depend on 'the window is small'. Required revision: keep the v6 field-22/field-22 design but add ONE fail-closed capture invariant — after CaptureTmuxIdentity reports identity.PanePID, capture ProcessStartToken(identity.PanePID) and immediately re-verify that tmux still reports that pane as live with the same pane id/pid before any accept loop binds the channel; if the pane is dead, missing, or identity-unverifiable at the capture boundary, do NOT stamp PaneKernelStartToken and accept NO control connection for that launch, routing through the #{pane_dead_status}/recovery-sweep typed floor; extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper (or a paired integration negative) with a 'pane dies before kernel-token capture' case that forces the wrapper to exit before W1 capture, makes the start-token read see a same-pid/reused-process token by stub/hook, and asserts the helper refuses rather than treating that token as the launched wrapper's identity. The existing same-pid stale-token and empty-token negatives remain but are necessary, not sufficient. Checks credited (no regression from the W1-token change): the BC1-W1-TOKEN direct v5 defect is fixed in the text; the empty-token trap is identified and closed; the real-path test shape is materially improved (RunHelper + RequireTmux + RunAsUser, field-22 both sides, same-pid stale-token rejection, empty-token rejection, provider isolation, sibling refusal); the v5-credited set (connect-out topology, named plumbing sites, non-secret address + post-auth nonce, W2 ordering, #{pane_dead_status} backstop + C2, BC2, BC3/F4, BC4/BC5, daemon-observed positive intent, backend-gate bypass, F2, F7 file-mirror half, AF1, AF4, no-admin-token-widening) is not regressed. needs_revision, not reject: no admin-token widening, no lane-readable reseal bearer, no credential carrying {admin, apply, recovery, surgical_recovery}; and not accept_with_findings because the residual issue is still inside the load-bearing W1 no-replay proof on the real tmux path."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "On the assigned regression/lifecycle lens, v6 preserves the v5-credited lifecycle set and fixes the direct BC1-W1-TOKEN defect: W1 no longer compares a kernel /proc field-22 token to tmux #{pane_start_time}; the proposal names PaneKernelStartToken, captures it with ProcessStartToken(identity.PanePID), compares the accepted peer's ProcessStartToken(peer.pid) to that captured kernel token, keeps tmux #{pane_start_time} out of W1, and adds same-pid stale-token plus empty-token fail-closed tests. The remaining gap is a NEW lifecycle/correctness gap at the W1 capture boundary, converging INDEPENDENTLY on falsifier-reviewer-001's ground: the spec calls the token 'launch-time' but captures it only after the tmux pane command has already been respawned (pty.go:479-484) and after CaptureTmuxIdentity returns (pty.go:493-504); CaptureTmuxIdentity records tmux's pane pid and start metadata (tmux_liveness.go:181-209) but does not pin the kernel field-22 token at process birth; ProcessStartToken reads whatever process currently owns /proc/<pid>/stat field 22 at read time (process_identity_linux.go:13-32). It does not require the helper to re-verify that the pane identity is still live and still the same pane before stamping PaneKernelStartToken. A fast-exiting wrapper plus pid reuse therefore makes W1 internally field-22-consistent while binding the channel to the wrong process: the launched wrapper exits before the helper reads ProcessStartToken(identity.PanePID); the numeric pid is reused; the helper stamps the reused process's field-22 token; W1 accepts the first connection from that reused pid because both peer.pid == result.PID and PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken) hold. Through the lifecycle lens this is exactly the wrong place to leave an unstated assumption: the authenticated frame is a PRIMARY entry point into resealInFlightJob, so if W1 binds to a reused process the rest of the lifecycle machinery can be perfectly serialized and still receive a daemon-authenticated control frame from a process that was not the launched wrapper; conversely, if the implementation notices the pane died and fails ad hoc, the spec has not required that path to route the typed session_unrecoverable_across_rotation floor instead of a raw launch/control error. The same-pid stale-token negative does not cover this (it is the inverse race). The source has the pieces: ProbeTmuxLiveness detects pane-dead, pane-pid mismatch, and pane-start mismatch states (tmux_liveness.go:257-269); v6 just does not require that re-verification after kernel-token capture and before channel binding. Required revision: keep the v6 field-22/field-22 design but add one fail-closed capture-boundary invariant — (1) capture ProcessStartToken(identity.PanePID) after CaptureTmuxIdentity; (2) before stamping PaneKernelStartToken or starting any accept loop, re-verify via tmux/liveness that the pane is not dead, that the pane id/pid still match, and that the boundary is not identity-unverifiable; (3) if dead/missing/pid-mismatched/identity-unverifiable, do not stamp and do not accept any control connection, routing through the #{pane_dead_status}/recovery-sweep typed floor; (4) extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper (or a paired integration negative) with a 'pane dies before kernel-token capture' case. Regression sweep clean (no regression found): BC1-W1-TOKEN direct v5 defect fixed; connect-out topology / W2 / W3 intact; BC2 / BC3 / F4 intact (frame carries no job_id/path/kind/body, identity daemon-derived, CapabilityReseal daemon-internal/test-only); BC4 / BC5 intact (jobs.recovery_generation + leases.reseal_grace_extended_at pinned to owner bundle 0021, stamped-generation predicate, one-grace rule, artifact.publish serialization, recovery-sweep lock order); typed floor / backend gate intact (resealInFlightJob still bypasses ensureWorkSessionBackend and routes predicate misses to the typed class — but the W1 capture-boundary miss must be specified to route there too); authored-path fold-in carried (gitChangedPathSnapshots / collectInScopeAuthoredPaths, nil baseline for isolated worktrees); F7 / AF1 / AF4 / no-widening intact. needs_revision, not reject (no admin-token widening, no lane-readable reseal bearer, no elevated credential); not accept_with_findings (the defect is still inside the load-bearing W1 no-replay proof for the real tmux path, upstream of resealInFlightJob)."
verdict: "needs_revision"
rationale: "This adjudicates the design-v6 REVISION (the SIXTH falsification pass on RFC 0143) against the SEED clearing condition: a clearing verdict requires BC1-W1-TOKEN genuinely resolved with ONE coherent KERNEL identity token AND structural no-replay established on the REAL channel AND the entire v5-credited resolved set carried forward unregressed (with the modified-since-baseline build-test folded in) AND no new material challenge standing unrebutted AND the security invariant holding STRUCTURALLY. v6 is materially stronger than v5 on the exact axis v5 fell on: it RESOLVES the v5 clock-domain defect. The v5 W1 check compared ProcessStartToken(peer.pid) (kernel /proc field-22 start-tick) against identity.PaneStartToken sourced from tmux #{pane_start_time} (a wall-clock unix timestamp) — two categorically different clock domains. v6 pins ONE coherent kernel start-token source: it adds LaunchResult.PaneKernelStartToken (pty.go:47-53), captures it via ProcessStartToken(identity.PanePID) in launchPTY (pty.go:493-504) immediately after CaptureTmuxIdentity reports the pane pid and before any control connection is accepted, and W1 compares the accepted peer's ProcessStartToken(peer.pid) to that captured kernel token via PIDLiveWithStartToken — field-22 vs field-22, one clock domain. tmux #{pane_start_time} is kept ONLY as liveness metadata (not claimed equivalent to /proc field 22), excluded from W1 by construction. v6 also closes the empty-token fail-open trap I and both falsifiers can confirm in source: PIDLiveWithStartToken(pid, '') skips the token comparison (tmux_liveness.go:397) and returns live on pid alone, so v6 requires RunHelper to assert result.PaneKernelStartToken != '' before accepting any connection, reaching the floor via the #{pane_dead_status}/recovery-sweep backstops when the token is empty. The real-path test TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper is extended to compare /proc field 22 on both sides plus a same-pid stale-token negative and an empty-token negative. Both falsifiers credit all of this and find NO regression of the v5-credited set; I independently confirmed the operand fix is coherent. But the gate does NOT clear. Both falsifiers INDEPENDENTLY land the SAME NEW material, source-anchored challenge inside W1 — the load-bearing structural no-replay wall for the real channel: the captured kernel token is read POST-LAUNCH, not proven to belong to the born wrapper. I INDEPENDENTLY confirmed against the current worktree: launchPTY starts the pane command with tmux respawn-pane (pty.go:479-484), THEN calls CaptureTmuxIdentity (pty.go:493), THEN attachTmuxPTY returns LaunchResult{PID: identity.PanePID} (pty.go:517-533); ProcessStartToken reads whatever process currently owns /proc/<pid>/stat field 22 at read time (process_identity_linux.go field-22 read). So v6's capture of ProcessStartToken(identity.PanePID) necessarily occurs after the pane command launched and after a tmux round trip — a real, non-zero window. If the launched wrapper exits inside that window and its numeric pid is reused by a live same-uid sibling, the helper stamps the reused process's NON-empty field-22 token as PaneKernelStartToken; W1 then accepts the first connection from that reused pid (peer.pid == result.PID and PIDLiveWithStartToken(peer.pid, capturedToken) both hold), delivers the post-auth nonce, and the sibling can emit a daemon-authenticated control frame into resealInFlightJob — the same-uid replay surface BC1 exists to close. The empty-token fail-closed guard (A3'') does NOT cover this: the reused process is live, so ProcessStartToken returns ok==true with a non-empty token, and the != '' assertion passes. The v6 same-pid stale-token negative (A3') does NOT cover it either: it proves W1 rejects a peer whose token differs from the already-captured token; this race is the inverse — the captured token is fresh but for the WRONG process. The Holder anticipated the death-and-reuse concern (HOLDER section 'Why capturing IMMEDIATELY after launch is load-bearing') but defends it only TEMPORALLY ('microseconds after the tmux server reports the pane pid', 'before attachTmuxPTY returns') — a probabilistic argument the rubric explicitly rejects for a security/authz channel ('the build will probably normalize this is not a falsifiable implementation contract'; no-replay must hold STRUCTURALLY, not as 'the window is small'). The spec adds no fail-closed re-verification that the captured token belongs to the still-live launched pane, even though the source already has the pieces (ProbeTmuxLiveness detects pane-dead, pane-pid mismatch, and pane-start mismatch at tmux_liveness.go:257-269). Clearing condition walked (all must hold): (1) BC1-W1-TOKEN resolved with ONE coherent kernel identity token — PARTIAL: the clock-domain operand source is genuinely fixed (field-22 vs field-22), but the launch-time IDENTITY BINDING of that token is not structurally established — a post-launch capture across a reuse window can bind a token to the wrong process, which the SEED/rubric names as 'captures the kernel token too late to bind the launch-time identity'; (2) structural no-replay established on the REAL channel — FAILS: the no-replay spine IS W1, and as specified W1 can accept a same-uid sibling on a reused pid, so structural no-replay is not established on the production tmux channel; (3) v5-credited resolved set carried forward unregressed with the build-test folded in — HOLDS: both falsifiers sweep the connect-out topology + named plumbing sites, the non-secret address + post-auth nonce W3, the W2 ordering + dumpable-before-dial, the #{pane_dead_status} exit-code backstop + C2, BC2, BC3, BC4, BC5, the daemon-observed positive intent, the ensureWorkSessionBackend bypass, the W1/W2/W3 wall shapes, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1-A18 discipline, and the modified-since-baseline authored-path build-test, and find NO regression; (4) no new material challenge standing unrebutted — FAILS: the capture-boundary pid-reuse race is a NEW challenge arising from the v6 capture design, lands in BOTH falsifiers independently, and the Holder had no further turn (the cycle ends at adjudication), so it stands unrebutted; (5) the security invariant holds structurally — the no-widening half HOLDS (no admin-token read path, no lane-readable reseal bearer, no minted credential carrying {admin, apply, recovery, surgical_recovery}; both falsifiers confirm) but the no-replay half FAILS on the capture-boundary race. Why not reject: no path widens admin-token exposure and no minted credential carries any elevated verb; both falsifiers explicitly confirm the no-widening invariant and recommend needs_revision, not reject; the capture-boundary gap is a soundness/no-replay defect, not a widening. Why not accept_with_findings: BC1-W1 is the security cluster's last load-bearing closure (F1/F6 and the F2/F7 channel halves inherit it); the SEED demands structural no-replay on the REAL channel, and a W1 that can bind to a reused-pid same-uid sibling does not establish it — that forecloses a clearing verdict; it is not a trackable post-clearance finding. Verdict: needs_revision. The next revision must, in ONE place and within the v6 field-22/field-22 design: after CaptureTmuxIdentity reports identity.PanePID, capture ProcessStartToken(identity.PanePID) and re-verify (via tmux/liveness — ProbeTmuxLiveness states at tmux_liveness.go:257-269) that the pane is still live and still the same pane id/pid at the capture boundary BEFORE stamping PaneKernelStartToken and BEFORE any accept loop binds the channel; if the pane is dead, missing, pid-mismatched, or identity-unverifiable at that boundary, do NOT stamp the kernel token and accept NO control connection for that launch, routing through the #{pane_dead_status}/recovery-sweep typed session_unrecoverable_across_rotation floor (never a raw launch/control error); and extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper (or a paired integration negative) with a 'pane dies before kernel-token capture' case that forces the wrapper to exit before W1 capture, makes the token read observe a same-pid/reused-process token by stub or controlled hook, and asserts the helper refuses rather than treating that token as the launched wrapper's identity. Retain the v6 field-22/field-22 operand fix, the same-pid stale-token negative (A3'), and the empty-token fail-closed negative (A3'') — they are necessary but not sufficient. Carry forward unregressed (do NOT reopen): the connect-out topology and named plumbing sites, the non-secret address + post-auth nonce (W3), the W2 ordering + dumpable-before-dial, the #{pane_dead_status} exit-code backstop and the C2 commitment, the daemon-observed positive intent + backend-gate bypass, BC2, BC3, BC4, BC5, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1-A18 assertion discipline, and the modified-since-baseline authored-path build-test. Maintainer-ratification note (carries regardless of verdict): Slice B (the daemon-internal rpc.CapabilityReseal marker, the test-only auth-prelude route alternate, the daemon-owned supervisor control channel with connect-out SO_PEERCRED pid+start-time authentication, the reserved agentloop exit codes, the jobs.recovery_generation + leases.reseal_grace_extended_at owner-bundle-0021 columns, and endpoint/epoch republish plumbing) is a security/authz trust-model change requiring maintainer ratification before any build slice touches credential code; adjudicator clearance gates the spec's soundness, not the maintainer's product call. Slice A (the Option-4 typed-exit-code floor) is zero-trust-change but, per the still-open W1 capture-boundary ground, still must route over the connect-out non-PTY channel whose same-uid authentication (W1) is specified with one coherent kernel identity token that is structurally bound to the launched wrapper before it lands."
findings:
  - id: BC1-W1-TOKEN
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "R2 replay / false-provenance (the W1 operand domain is now coherent)"
    challenge: "RESOLVED — the v5 clock-domain operand defect is genuinely fixed and both falsifiers credit it. v6 no longer compares kernel /proc field-22 to tmux #{pane_start_time}. It adds LaunchResult.PaneKernelStartToken (pty.go:47-53), captures it via ProcessStartToken(identity.PanePID) in launchPTY (pty.go:493-504) immediately after CaptureTmuxIdentity reports the pane pid and before any control connection is accepted, and W1 compares the accepted peer's ProcessStartToken(peer.pid) to that captured kernel token via PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken) (tmux_liveness.go:392-408) — field-22 on both sides, one clock domain. tmux #{pane_start_time} is kept ONLY as liveness metadata (identity.PaneStartToken, ProbeTmuxLiveness), not claimed equivalent to /proc field 22, excluded from W1 by construction. v6 also closes the empty-token fail-open trap (PIDLiveWithStartToken(pid, '') skips the comparison, tmux_liveness.go:397) by asserting result.PaneKernelStartToken != '' before accepting any connection. The real-path test TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper is extended to compare /proc field 22 on both sides, plus a same-pid stale-token negative (A3') and an empty-token negative (A3''). Independently confirmed against the current worktree. NOTE: the launch-time IDENTITY BINDING of this now-coherent token is a separate open ground — see BC1-W1-CAPTURE."
  - id: BC1-W1-CAPTURE
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2", "dialogue:3"]
    affected_invariants:
      - "R2 replay / false-provenance (no same-uid-reachable channel a sibling lane can present on the real path)"
      - "R4 legible-failure (the typed floor must fire on a capture-boundary miss, never a raw launch/control error)"
    challenge: "OPEN — verdict-driving. The v6 kernel start token is captured POST-LAUNCH and is not structurally proven to belong to the born wrapper, so W1 can bind the channel to the wrong same-uid process. Independently confirmed against the current worktree: launchPTY starts the pane command with tmux respawn-pane (pty.go:479-484), THEN calls CaptureTmuxIdentity (pty.go:493), THEN attachTmuxPTY returns LaunchResult{PID: identity.PanePID} (pty.go:517-533); ProcessStartToken reads whatever process currently owns /proc/<pid>/stat field 22 at read time (process_identity_linux.go field-22 read). So the proposed ProcessStartToken(identity.PanePID) capture occurs after the pane command launched and after a tmux round trip — a real, non-zero window. If the launched wrapper exits inside that window and its numeric pid is reused by a live same-uid sibling, the helper stamps the reused process's NON-empty field-22 token as PaneKernelStartToken; W1 then accepts the first connection from that reused pid (peer.pid == result.PID and PIDLiveWithStartToken(peer.pid, capturedToken) both hold), delivers the post-auth nonce, and the sibling emits a daemon-authenticated control frame into resealInFlightJob (a primary entry point) — the same-uid replay surface BC1 exists to close. The empty-token fail-closed guard (A3'') does not cover this (the reused process is live → ok==true, non-empty token → the != '' assertion passes). The same-pid stale-token negative (A3') does not cover it (that is the inverse race). The Holder defends the capture only temporally ('microseconds after launch', 'before attachTmuxPTY returns'); the rubric rejects a probabilistic argument for a security/authz channel. Fix in ONE place, within the v6 field-22/field-22 design: after CaptureTmuxIdentity reports identity.PanePID, capture ProcessStartToken(identity.PanePID) and re-verify via tmux/liveness (ProbeTmuxLiveness states at tmux_liveness.go:257-269: pane-dead, pane-pid mismatch, pane-start mismatch) that the pane is still live and still the same pane id/pid at the capture boundary BEFORE stamping PaneKernelStartToken and BEFORE any accept loop binds the channel; if the pane is dead, missing, pid-mismatched, or identity-unverifiable, do NOT stamp the kernel token and accept NO control connection, routing through the #{pane_dead_status}/recovery-sweep typed session_unrecoverable_across_rotation floor; extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper (or a paired integration negative) with a 'pane dies before kernel-token capture' case forcing the wrapper to exit before W1 capture, making the token read observe a same-pid/reused-process token by stub/hook, and asserting the helper refuses rather than treating that token as the launched wrapper's identity. Keeps F1, F6, and the F2/F7 channel halves open until W1 binds the launch-time identity structurally; the Slice-A exit-97 floor is plausibly closed once that binding holds. Both falsifiers converge INDEPENDENTLY on this identical source-anchored boundary defect — strong corroboration of a genuine standing falsification."
  - id: BC2
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "RESOLVED (carried forward unregressed; both falsifiers find no regression). The connect-out SupervisorControlFrame carries NO job_id / artifact path / kind / body — identity is derived from daemon state. resealInFlightJob derives the expected-artifact set from the job's expected_artifacts (attempt-resolved via resolveExpectedArtifactCycles), reuses verifyRequiredArtifacts / ensurePerJobPublishedArtifactsDurable (mutations.go:828-876), publishes only a path that is an open expected entry from the job's own worktree, refuses unexpected paths, and routes a front-matter/author-line failure to the Option-4 floor. Keep TestCodexResealUsesReceiverNotProviderStdout (negative + positive)."
  - id: BC3
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "RESOLVED (carried forward unregressed; both falsifiers find no regression). CapabilityReseal stays a daemon-internal marker projected by resealInFlightJob (supervisor_id->session_id, internal AuthContext without the public Authorize prelude, rpc/server.go:107-111), with the public route-alternate (MethodEntry.ResealAlternate on only interrogation.answer/work.complete/artifact.publish, recording reseal not write) kept TEST-ONLY since no production bearer exists; registry_methods.go generated, command-authority-matrix reseal column + authority guardrail updated. Keep TestResealCapabilityIsDaemonInternalNotBearer / TestResealTokenCanReachOnlyResealRoutesWithoutWrite."
  - id: BC4
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:3"]
    challenge: "RESOLVED (carried forward unregressed; falsifier-reviewer-002 confirms). Concrete monotonic jobs.recovery_generation column in owner bundle 0021 (LatestOwnerBundleVersion 20->21 — confirmed currently 20; ordinal-21 reservation; modelled on review_generation); degrade-safe JobRecoveryGenerationColumnPresent probe routes to the typed floor when absent; four increment points (claim, requeue-same-attempt, recovery-sweep expire/transfer/respawn, release) each in the lease-retiring/rebinding UPDATE under lockRun; post-increment value stamped into work_packets.packet_json lease.recovery_generation and compared equal/unequal at reseal under the lock. Keep TestResealPredicateUsesStampedRecoveryGeneration / TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue."
  - id: BC5
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "R3 split-brain across rotation"
      - "R4 legible-failure (typed class must fire, never a raw lease_error)"
    challenge: "RESOLVED (carried forward unregressed; falsifier-reviewer-002, on the lifecycle lens plus a regression sweep, finds no standing material gap). (1) leases.reseal_grace_extended_at PINNED to the same owner bundle 0021 as jobs.recovery_generation (leases owner-held — created runtime 0005, absent from the owner/0018 transfer cohort), so TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn / TestFutureRuntimeMigrationsDoNotCarryOwnerDDL can fire. (2) Numeric grace (min(resealGraceWindow, heartbeat_after_seconds)) with ONE same-lease extension gated by reseal_grace_extended_at; CORRECTED skip/replace/replay lock-order gate map (resealInFlightJob skips/replaces enforceSessionBindingForSession, enforceActiveActingSession, activeLeaseFor, ensureWorkSessionBackend; replays lockRunForJob first, the FOR UPDATE rows, the reseal predicate, write-scope, durability, running->completed); artifact.publish takes lockRunForJob first; the sweep drains->locks->expires, so expired-beyond-grace ALWAYS routes the typed session_unrecoverable_across_rotation class and a requeued lease is never revived. Keep TestResealBeyondGraceRoutesTypedNotLeaseError / TestResealGraceCannotReviveRequeuedLease / TestRecoveryRequeueWinsOverExpiredLeaseReseal / GD-1b / TestResealExit98BypassesBackendGateOrRoutesTyped."
  - id: CF
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "CARRIED-FORWARD SET intact (no regression — both falsifiers confirm). Connect-out topology + named plumbing sites (no fd crosses the tmux boundary; pane wrapper dials OUT after PR_SET_DUMPABLE(0)): unregressed. Non-secret address + post-auth nonce (W3): unregressed. W2 ordering + dumpable-before-dial: unregressed. #{pane_dead_status} exit-code backstop + C2 (wrapper never propagates a provider child 97/98 into the reserved codes; TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker): unregressed. Daemon-observed positive intent + recovery-sweep backstop + ensureWorkSessionBackend bypass (routing misses to the typed floor): unregressed. W1/W2/W3 wall SHAPES: unregressed (the open ground is the W1 token's launch-time identity BINDING, not the wall shapes). F2 (no lane-readable reseal bearer; the v1 0600 same-uid file replay stays retired): held. F4 (route-alternate limited to the three routes, records reseal not write, test-only): held. F7 file-mirror half (daemon-owned lane-read-only 0644 endpoint/epoch, O_NOFOLLOW, atomic rename, missing-epoch rejected, closes #316): held; channel half inherits the open BC1-W1-CAPTURE. AF1 reachability-not-reminting (TestTokenValidAcrossRestart) and AF4 epoch/token decoupling: kept. No-admin-token-widening invariant: held + strengthened (CapabilityReseal carries no elevated verb and is never materialized into a lane-readable file; no minted credential carries {admin,apply,recovery,surgical_recovery}; TestResolveRefusesRuntimeClientTokenForLane). Modified-since-baseline authored-path build-test (gitChangedPathSnapshots/collectInScopeAuthoredPaths, nil baseline for isolated worktrees): folded in and credited. Per-claim falsifiable-assertion discipline A1-A18 (+ A3'/A3''/A4'/A7'): extended to the W1 single-kernel-token contract. Preserve all verbatim through the next revision."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 design-v6 REVISION (cycle 1)

author: adjudicator-author-001

> Adjudication of the design-v6 REVISION dialogue trajectory for RFC 0143
> (*lane credential survival across a daemon boot-epoch rotation*). This is the
> **sixth** falsification pass on the RFC: v1 (`rfc-0143-design`) returned
> `needs_revision` with seven findings F1–F7; v2 resolved **F2/F4** and distilled
> the residue into the five binding constraints BC1–BC5; v3 resolved **BC2, BC3,
> BC4**; v4 resolved **BC5, C2, and the daemon-observed positive intent**, falling
> on **BC1-CHANNEL** (the W1/W2/W3 walls were specified for a direct
> `exec.Cmd.ExtraFiles` child while the production lane is tmux-backed); v5
> **resolved BC1-CHANNEL** by deleting the inherited-fd channel and adopting the
> **connect-out topology**, but fell — both falsifiers independently — on
> **BC1-W1-TOKEN** (W1 compared a kernel `/proc` field-22 start-tick against a tmux
> `#{pane_start_time}` wall-clock timestamp — two clock domains). This v6 run
> revised the spec to close BC1-W1-TOKEN; the two falsifiers re-attacked it.
> Inputs read: the revised Holder spec (`dialogue/holder/HOLDER.md`), both
> falsifier re-attacks (`dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`), the `SEED.md` charter (the ratified design
> shape, the v5-credited carry-forward set, and the BC1-W1-TOKEN constraint with
> its exact source anchors), and the **v5** collaboration ledger (the
> BC1-W1-TOKEN finding + the exact "next revision must…" list). No raw terminal
> output was read. Load-bearing source citations were independently re-verified
> against the current worktree.

## Verdict

**verdict: needs_revision**

v6 is **materially stronger than v5 on the exact axis v5 fell on**: it **resolves
the v5 clock-domain defect**. W1 no longer compares a kernel `/proc` field-22
start-tick against a tmux `#{pane_start_time}` wall-clock timestamp. v6 adds
`LaunchResult.PaneKernelStartToken`, captures it via
`ProcessStartToken(identity.PanePID)` immediately after `CaptureTmuxIdentity`
reports the pane pid and before any control connection is accepted, and compares the
accepted peer's `ProcessStartToken(peer.pid)` to that captured kernel token via
`PIDLiveWithStartToken` — **field-22 on both sides, one clock domain.** It keeps tmux
`#{pane_start_time}` as liveness metadata only (not claimed equivalent), closes the
`PIDLiveWithStartToken(pid, "")` empty-token fail-open trap with a fail-closed
`!= ""` assertion, and extends the real-path test with a same-pid stale-token
negative and an empty-token negative. Both falsifiers credit all of this and find
**no regression** of the v5-credited set; I independently confirmed the operand fix
is coherent.

But the gate does **not** clear. **Both falsifiers independently land the same NEW
material, source-anchored challenge inside W1** — the load-bearing structural
no-replay wall — and it stands **unrebutted**: the now-coherent kernel token is
captured **post-launch** and is not structurally proven to belong to the **born**
wrapper.

**Why not `reject`.** No path widens admin-token exposure, and no minted credential
carries any of `{admin, apply, recovery, surgical_recovery}`. Both falsifiers confirm
the no-widening invariant holds and recommend `needs_revision`, not `reject`. The
capture-boundary gap is a fail-closed **soundness/no-replay** defect, not a widening.

**Why not `accept_with_findings`.** BC1-W1 is the security cluster's last
load-bearing closure (F1/F6 and the F2/F7 channel halves inherit it). A clearing
verdict requires structural no-replay on the **real** channel — and a W1 that can
bind to a **reused-pid same-uid sibling** does not establish it. That is not a
trackable post-clearance finding; it forecloses a clearing verdict.

## The clearing condition, walked

A clearing verdict requires **all** to hold; the W1-capture-dependent ones fail:

1. **BC1-W1-TOKEN resolved with ONE coherent kernel identity token — PARTIAL.** The
   clock-domain operand *source* is genuinely fixed (field-22 vs field-22). But the
   **launch-time identity binding** of that token is not structurally established: a
   post-launch capture across a pid-reuse window can bind the token to the wrong
   process — exactly the SEED/rubric `needs_revision` condition "captures the kernel
   token too late to bind the launch-time identity."
2. **Structural no-replay established on the REAL channel — FAILS.** The no-replay
   spine **is** W1; as specified it can accept a same-uid sibling on a reused pid, so
   structural no-replay is not established on the production tmux channel.
3. **The v5-credited resolved set carried forward unregressed (build-test folded in)
   — HOLDS.** Both falsifiers sweep the connect-out topology + named plumbing sites,
   the non-secret address + post-auth nonce (W3), the W2 ordering +
   dumpable-before-dial, the `#{pane_dead_status}` backstop + C2, BC2, BC3, BC4, BC5,
   the daemon-observed positive intent, the `ensureWorkSessionBackend` bypass, the
   W1/W2/W3 wall *shapes*, F2, F4, the F7 file-mirror half, AF1, AF4, the
   no-admin-token-widening invariant, and the A1–A18 discipline, and find **no
   regression**; the modified-since-baseline authored-path build-test is folded in.
4. **No new material challenge standing unrebutted — FAILS.** The capture-boundary
   pid-reuse race is a **new** challenge arising from the v6 capture design, lands in
   **both** falsifiers, and the Holder had no further turn (the cycle ends at
   adjudication), so it stands unrebutted.
5. **The security invariant holds structurally — partial.** The **no-widening** half
   holds; the **no-replay** half fails on the capture-boundary race.

## What v6 genuinely closed (credited)

- **The v5 clock-domain defect is dissolved.** W1's operand is one coherent kernel
  token: `ProcessStartToken(identity.PanePID)` captured at launch (`pty.go:493-504`,
  threaded onto `LaunchResult.PaneKernelStartToken`, `pty.go:47-53`) compared to the
  accepted peer's `ProcessStartToken(peer.pid)` via
  `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`
  (`tmux_liveness.go:392-408`) — field-22 vs field-22. tmux `#{pane_start_time}` is
  kept as liveness metadata only, excluded from W1 by construction.
- **The empty-token fail-open trap is closed.** I confirmed in source that
  `PIDLiveWithStartToken(pid, "")` skips the token comparison (`tmux_liveness.go:397`)
  and returns live on pid alone. v6 requires `RunHelper` to assert
  `result.PaneKernelStartToken != ""` before accepting any connection, reaching the
  floor via the `#{pane_dead_status}`/recovery-sweep backstops when the token is empty
  (A3'').
- **The real-path test shape is materially improved.** `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`
  (RunHelper + RequireTmux/RunAsUser) now compares `/proc` field 22 on both sides,
  with a same-pid stale-token negative (A3') and an empty-token negative (A3'').
- **The v5-credited set is not regressed by the W1-token change** (both falsifiers'
  regression sweeps; see finding CF).

## The one open ground — BC1-W1-CAPTURE (independently confirmed)

The v6 W1 predicate accepts a peer iff `peer.uid == RunAsUser uid`,
`peer.pid == result.PID`, `result.PaneKernelStartToken != ""`, and
`PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)` returns live with a
matching field-22 token. The operand is coherent, but the **capture is post-launch**.
I independently confirmed against the current worktree:

- `launchPTY` starts the pane command with `tmux respawn-pane` (`pty.go:479-484`),
  **then** calls `CaptureTmuxIdentity` (`pty.go:493`), **then** `attachTmuxPTY`
  returns `LaunchResult{PID: identity.PanePID}` (`pty.go:517-533`).
- `CaptureTmuxIdentity` records tmux's pane pid + start metadata
  (`tmux_liveness.go:181-209`); it does **not** pin the kernel field-22 token at
  process birth.
- `ProcessStartToken` reads whatever process currently owns `/proc/<pid>/stat` field
  22 at read time (`process_identity_linux.go`).

So v6's `ProcessStartToken(identity.PanePID)` capture necessarily occurs **after** the
pane command launched and after a tmux round trip — a real, non-zero window. If the
launched wrapper exits inside that window and its numeric pid is reused by a **live**
same-uid sibling, the helper stamps the reused process's **non-empty** field-22 token
as `PaneKernelStartToken`; W1 then accepts the first connection from that reused pid
(`peer.pid == result.PID` and `PIDLiveWithStartToken(peer.pid, capturedToken)` both
hold), delivers the post-auth nonce, and the sibling can emit a daemon-authenticated
control frame into `resealInFlightJob` — the same-uid replay surface BC1 exists to
close.

- The **empty-token fail-closed guard (A3'') does not cover this**: the reused process
  is live → `ProcessStartToken` returns `ok==true` with a non-empty token → the
  `!= ""` assertion passes.
- The **same-pid stale-token negative (A3') does not cover this**: it proves W1
  rejects a peer whose token differs from the already-captured token; this race is the
  **inverse** — the captured token is fresh but for the **wrong process**.
- The Holder anticipated the death-and-reuse concern (HOLDER §"Why capturing
  IMMEDIATELY after launch is load-bearing") but defends it only **temporally**
  ("microseconds after the tmux server reports the pane pid", "before `attachTmuxPTY`
  returns"). For a security/authz channel the rubric explicitly rejects a probabilistic
  argument ("the build will probably normalize this is not a falsifiable
  implementation contract"); no-replay must hold **structurally**.

The source already has the pieces to close it: `ProbeTmuxLiveness` detects pane-dead
(`tmux_liveness.go:257`), pane-pid mismatch (`:260`), and pane-start mismatch (`:265`)
— exactly the capture-boundary re-verification both falsifiers prescribe.

## Falsifier challenge dispositions

- **falsifier-reviewer-001 — captured kernel token is post-launch, not proven
  launch-time (material; landed unrebutted).** Claim challenged: the v6 capture binds
  the launch-time identity of the actual pane wrapper. Material? **Yes** — it would
  change the spec (add a fail-closed capture-boundary liveness invariant + a "pane
  dies before kernel-token capture" negative) and exposes a real same-uid replay
  surface inside the no-replay wall. Rebutted? **No** — the Holder's only defenses are
  the temporal "microseconds after launch" and the empty-token guard, neither of which
  covers the live-reused-pid case; the Holder had no further turn. Disposition:
  **BC1-W1-CAPTURE open; verdict-driving.** No admin-token widening; recommends
  needs_revision, not reject.
- **falsifier-reviewer-002 — W1 needs a capture-boundary liveness guard (material;
  landed unrebutted).** Claim challenged: the authenticated frame is a working primary
  entry point bound to the launched wrapper. Material? **Yes** — same boundary defect
  through the lifecycle lens: a perfectly serialized lifecycle can still consume a
  daemon-authenticated frame from a reused process, and an ad-hoc pane-died failure is
  not required to route the typed floor. Rebutted? **No.** Lifecycle/regression sweep:
  **no** BC2/BC3/BC4/BC5/F7/AF1/AF4/no-widening regression; the authored-path
  build-test fold-in credited. Disposition: **lifecycle core intact; BC1-W1-CAPTURE
  open on the boundary ground.**

Both falsifiers converge **independently** on the identical source-anchored
capture-boundary defect — strong corroboration that this is a genuine standing
falsification, not a single reviewer's idiosyncrasy.

## What the next revision MUST fix to clear on re-attack

One item, within BC1-W1 — the launch-time identity binding of the (now coherent)
kernel token; retain the v6 field-22/field-22 operand fix and the A3'/A3'' negatives:

1. **Bind the captured kernel token to the still-live launched pane.** After
   `CaptureTmuxIdentity` reports `identity.PanePID`, capture
   `ProcessStartToken(identity.PanePID)` **and** re-verify via tmux/liveness
   (`ProbeTmuxLiveness` states at `tmux_liveness.go:257-269`: pane-dead, pane-pid
   mismatch, pane-start mismatch) that the pane is still live and still the same pane
   id/pid at the capture boundary **before** stamping `PaneKernelStartToken` and
   **before** any accept loop binds the channel.
2. **Fail closed at the capture boundary.** If the pane is dead, missing,
   pid-mismatched, or identity-unverifiable at that boundary, do **not** stamp the
   kernel token and accept **no** control connection for that launch; route through the
   `#{pane_dead_status}`/recovery-sweep typed `session_unrecoverable_across_rotation`
   floor (never a raw launch/control error).
3. **Make the real-path test fire the race.** Extend
   `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (or a paired integration negative)
   with a "pane dies before kernel-token capture" case that forces the wrapper to exit
   before W1 capture, makes the token read observe a same-pid/reused-process token by
   stub or controlled hook, and asserts the helper **refuses** rather than treating
   that token as the launched wrapper's identity.

Everything else is sound and carries forward **unchanged**: the v6 field-22/field-22
operand fix, the same-pid stale-token negative (A3'), the empty-token fail-closed
negative (A3''), the connect-out topology and named plumbing sites, the non-secret
address + post-auth nonce (W3), the W2 ordering + dumpable-before-dial, the
`#{pane_dead_status}` exit-code backstop and the C2 commitment, the daemon-observed
positive intent + the `ensureWorkSessionBackend` bypass, BC2, BC3, BC4, BC5, F2, F4,
the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1–A18
assertion discipline, and the modified-since-baseline authored-path build-test.

## Note on maintainer ratification (carries forward regardless of verdict)

Even when a future revision clears, the chosen direction — the daemon-internal
`rpc.CapabilityReseal` marker, the test-only auth-prelude route alternate, the
daemon-owned supervisor control channel with **connect-out `SO_PEERCRED`
pid+start-time** authentication, the reserved agentloop exit codes, the
`jobs.recovery_generation` + `leases.reseal_grace_extended_at` owner-bundle-0021
columns, and the endpoint/epoch republish plumbing — is a **security/authz
trust-model change** requiring **maintainer ratification** before any build slice
touches credential code. Adjudicator clearance gates the spec's **soundness**; it is
**not** the maintainer's product call on the credential code. Slice A (the Option-4
typed-exit-code floor) is zero-trust-change, but per the still-open W1
capture-boundary ground it must route over the connect-out, non-PTY channel whose
same-uid authentication (W1) is specified with one coherent kernel identity token
**structurally bound to the launched wrapper** before it lands.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 falsification-gate
design-v6 REVISION run (cycle 1). The ledger verdict — not falsifier completion —
gates the phase: `needs_revision` returns the spec uncleared. v6 dissolves the v5
clock-domain defect (W1 now compares `/proc` field 22 on both sides, with tmux
`#{pane_start_time}` kept as liveness metadata only and a fail-closed empty-token
rule) and carries the v5-credited set forward unregressed, but both falsifiers
independently land the same NEW material, source-confirmed defect — the kernel token
is captured post-launch and is not structurally bound to the born wrapper, so a
fast-exiting wrapper plus pid reuse lets W1 accept a same-uid sibling — so structural
no-replay is not yet established on the production channel and BC1-W1 stays open on a
single, narrowly-specified capture-boundary ground.</sub>
