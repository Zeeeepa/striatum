---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 lane credential survival across a daemon boot-epoch rotation — falsifiable implementation spec (design-v7 REVISION; resolve the single remaining binding constraint BC1-W1-CAPTURE)"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "v7 is a proper revision starting from the v6 HOLDER spec; it resolves the single remaining binding constraint BC1-W1-CAPTURE in ONE place and carries the entire v6-credited resolved set forward unregressed. The v6 ground: the now-coherent kernel start token (LaunchResult.PaneKernelStartToken captured via ProcessStartToken(identity.PanePID), field-22-to-field-22 against the accepted peer) is captured POST-LAUNCH and is not structurally proven to belong to the born wrapper, so a fast-exiting wrapper plus pid reuse lets W1 bind the channel to a live same-uid sibling. v7 adds ONE fail-closed capture-boundary invariant inside launchPTY (between CaptureTmuxIdentity at pty.go:493-504 and attachTmuxPTY at pty.go:507): capture paneKernelStartToken, ok := processStartToken(identity.PanePID) through an injectable seam, THEN re-verify via ProbeTmuxLiveness AFTER the read (tmux_liveness.go:212-282) that the pane is not dead (:257), not pane-pid-mismatched (:260), not pane-start-mismatched (:265), and not pane-missing/unavailable (:217-233), with ObservedPanePID == identity.PanePID; stamp PaneKernelStartToken ONLY if that post-read re-verification passes, else leave it '' (engaging the retained v6 != '' no-pid-only-accept rule), accept NO control connection, and route the typed session_unrecoverable_across_rotation floor via the #{pane_dead_status}/recovery-sweep backstops, never a raw launch/control error. The binding is argued STRUCTURAL, not temporal, via the reaping invariant: a pid cannot be reused while its previous owner is unreaped (zombie); the pane process is a direct child of (and reaped by) the tmux server; tmux reaping the pane flips it out of the live state — so P reused implies ProbeTmuxLiveness non-Healthy, whose contrapositive is that a Healthy post-read probe with ObservedPanePID == P proves P was not reaped/reused through the probe. The [probe -> accept] residual window is closed by the retained accept-time field-22 match (A3'). New falsifiable assertion A3''' with a deterministic unit (TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive, injectable start-token seam + stub TmuxRunner) and a real-path 'pane dies before kernel-token capture' negative extending TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper. Retains the v6 field-22/field-22 operand fix (LaunchResult.PaneKernelStartToken, ProcessStartToken(identity.PanePID), PIDLiveWithStartToken, empty-token != '' A3'', same-pid stale-token A3') and carries forward unregressed: the connect-out topology + named plumbing sites, the non-secret address + post-auth nonce (W3), the W2 ordering + dumpable-before-dial, the #{pane_dead_status} exit-code backstop + C2, BC2, BC3, BC4, BC5, the daemon-observed positive intent, the ensureWorkSessionBackend bypass, the W1/W2/W3 wall SHAPES, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1-A18 assertion discipline, and the modified-since-baseline authored-path build-test."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "Credits the v7 fix as a genuine STRUCTURAL answer to the exact v6 exit-then-pid-reuse race (not a temporal hand-wave): the post-read ProbeTmuxLiveness re-verification before stamping, the field-22 operand, the fail-closed unstamped token, and the real-path negative all land as framed. But ONE NEW material challenge stands one layer below that proof, inside the load-bearing W1 no-replay wall: v7 treats tmux as the TRUSTED capture oracle even though the production tmux control surface is run as the SHARED lane uid. A same-uid sibling does not need to win pid reuse after the wrapper dies; it can REPLACE the tmux pane before CaptureTmuxIdentity or before the post-read re-verification, causing v7 to structurally bind the kernel token to the current tmux pane process, not necessarily to the daemon-launched wrapper. Source-anchored: commandInvocationWithEnvFile runs any RunAsUser command as sudo -n -u <runAsUser> -- env -i (pty.go:98-112); tmuxRunnerForSpec returns RunAsTmuxRunner(spec.RunAsUser, spec.Env) (pty.go:310-314) which invokes the plain tmux client through the same run-as path (tmux_liveness.go:125-133); the session name is deterministic metadata derived from run/lane/supervisor ids (pty.go:620-632); no private tmux -S socket, daemon-owned tmux server, or non-lane-readable control boundary is named in live source or in the v7 spec; the launch sequence runs tmux respawn-pane -k and only then calls CaptureTmuxIdentity (pty.go:479-493). Concrete replay: (1) the daemon issues respawn-pane -k ... agentloop; (2) before CaptureTmuxIdentity or before the post-read re-verification completes, a same-uid sibling issues tmux respawn-pane -k -t <that pane> -- <attacker wrapper>; (3) CaptureTmuxIdentity and ProbeTmuxLiveness now observe a live, same pane id/pid/start process that is the sibling's replacement; (4) the helper stamps the replacement's /proc field-22 token as PaneKernelStartToken; (5) the replacement dials the non-secret control address and W1 accepts (peer.uid == RunAsUser, peer.pid == result.PID, matching field-22 token) and delivers the post-auth nonce. The reaping invariant does NOT rebut this: it proves only that, IF tmux is a trustworthy witness for the original launch, a passing post-read probe means the pid was not reaped+reused before the probe; here the attacker changes the tmux witness itself. The v7 real-path negative does not fire this class — it tests pane-dies-before-capture with a dead/pid-mismatched probe, whereas this attack needs a HEALTHY probe against a sibling-supplied replacement. This is exactly the same-uid threat model that rejected a 0600 reseal file; W1's whole purpose is to survive same-uid sibling knowledge of non-secret channel material, and if the sibling can replace the pane before capture W1 authenticates the replacement as the pane wrapper. Required revision: keep the v7 post-read re-verification and the v6 field-22 operand, but add one structural binding — (a) make the tmux control surface daemon-owned/inaccessible to sibling lane processes before and during capture (private tmux socket / TMUX_TMPDIR not reachable by the shared lane uid); OR (b) add an explicit birth/provenance primitive a same-uid respawn-pane cannot forge and bind PaneKernelStartToken to it before W1 can accept; OR (c) if neither can be guaranteed, fail closed to the typed session_unrecoverable_across_rotation floor instead of claiming a same-uid-safe authenticated primary channel; and add a real-path negative adjacent to TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper where a same-uid sibling issues a competing respawn-pane between the daemon's respawn and CaptureTmuxIdentity/the post-read probe and the helper REFUSES. needs_revision, not reject (no admin-token widening, no lane-readable reseal bearer, no credential carrying {admin, apply, recovery, surgical_recovery}); not accept_with_findings (the gap is still inside the W1 no-replay wall that must make the real tmux connect-out channel safe). Regression sweep clean: BC1-W1-CAPTURE resolved as framed; fail-closed path specified; real-path race test materially improved; BC1-W1-TOKEN components retained; the v6-credited set otherwise unregressed (connect-out topology, named plumbing sites, non-secret address + post-auth nonce, W2 ordering, #{pane_dead_status} backstop + C2, BC2, BC3/F4, BC4/BC5, daemon-observed positive intent, backend-gate bypass, F2, F7 file-mirror half, AF1, AF4, no-admin-token-widening)."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "On the assigned regression/lifecycle lens, finds NO regression of the v6-credited set and credits the v7 capture-boundary fix as materially stronger than v6 on the precise same-pid reuse race (read ProcessStartToken(identity.PanePID) first, then require a post-read ProbeTmuxLiveness pass before stamping PaneKernelStartToken — directly addressing the v6 'token captured after launch, not structurally tied to the born wrapper' defect). One NEW material lifecycle gap stands: v7 asserts a capture-boundary miss accepts no control connection and NEVER leaks a raw launch/control error, but the proposed mechanism does not specify a daemon-recognized typed route for the pre-attach unstamped-token path. Source-anchored: today RunHelper calls helperLaunch and, if launch returns an error, emits helper_error with phase launch and returns (helper.go:157-165); the required-tmux launch-failure test locks that shape (helper_test.go:330-374); the launch handshake treats a helper error before agent_started as a raw 'PTY helper failed before attach' (supervision_launch.go:562-580). The helper event schema has NO typed capture-boundary event — it admits only agent_exited, agent_started, artifact_observed, attach_client_exited, helper_error, packet_accepted, process_terminated, progress, and helper_error keeps only phase and error in durable payloads (supervision.go:220-234, :412-443). The only connect-out frame schema v7 specifies is sent over an authenticated connection AFTER W1 accepts (HOLDER.md:497-503), and the capture-boundary miss explicitly accepts NO connection. So the v7 miss path is caught between two mechanisms: (1) if the pane died/identity became unverifiable before attachTmuxPTY, the helper can fail before agent_started, which current supervision records as a raw helper_error/launch failure, not the typed floor; (2) if the helper continues with an unstamped token, the primary authenticated frame is intentionally disabled, so the design must rely on #{pane_dead_status} or the recovery sweep, but v7 does not name the implementation hook that converts the launch-time 'unstamped because not reverified' condition into the same session_unrecoverable_across_rotation class before ordinary stale/unsealed recovery sees it. The recovery sweep may eventually find a complete-on-disk deliverable, but the SEED requires the capture-boundary failure path ITSELF to route the typed floor and never leak a raw launch/control error (SEED.md:401-421); as written the Holder asserts that outcome without adding a typed event, a supervision recorder branch, or a reseal-sweep predicate that distinguishes 'unstamped W1 token due to capture-boundary miss' from ordinary helper launch failure or generic agent-exited-unsealed recovery. The new A3''' negative proves 'no stamp / no accept' but not the required typed floor. Required revision: (1) define a typed helper/supervision event or explicit daemon-side recorder branch for PaneKernelStartToken unstamped due to a capture-boundary miss, carrying class session_unrecoverable_across_rotation rather than generic helper_error; (2) state how that event is emitted even when the miss occurs before attachTmuxPTY or before agent_started, or move the re-verification to a point where the typed backstop is actually available; (3) extend TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive and the real-path 'pane dies before kernel-token capture' negative to assert the durable daemon event/blocker class, not only 'token unstamped' and 'no connection accepted'; (4) add a negative for the existing raw path: a capture-boundary miss must not produce a durable supervisor.helper_error / launch-handshake failure as the terminal explanation when the intended class is session_unrecoverable_across_rotation. needs_revision, not reject (no admin-token widening, no lane-readable reseal bearer, no elevated credential); not accept_with_findings (the gap is on the failure route for the new v7 capture-boundary mechanism itself, which the SEED requires to route the typed floor). Regression sweep clean: BC1-W1-TOKEN intact; BC1-W1-CAPTURE materially improved subject to the typed-route issue; connect-out / W2 / W3 / C2 intact; BC2 / BC3 / F4 intact; BC4 / BC5 intact; daemon-observed positive intent / backend-gate bypass intact; authored-path fold-in intact; F7 / AF1 / AF4 / no-widening intact."
verdict: "needs_revision"
rationale: "This adjudicates the design-v7 REVISION (the SEVENTH falsification pass on RFC 0143) against the SEED clearing condition: a clearing verdict requires BC1-W1-CAPTURE genuinely resolved with a fail-closed capture-boundary re-verification that STRUCTURALLY binds the kernel token to the still-live launched wrapper AND structural no-replay established on the REAL channel AND the entire v6-credited resolved set carried forward unregressed (with the modified-since-baseline build-test folded in) AND no new material challenge standing unrebutted AND the security invariant holding STRUCTURALLY (no widening, no replay, no split-brain). v7 is materially stronger than v6 on the exact axis v6 fell on: it RESOLVES the v6-FRAMED capture race. The v6 ground was the post-launch-exit-then-pid-reuse window — the kernel token is read after tmux respawn-pane has launched the pane and after a CaptureTmuxIdentity round trip, so a fast-exiting wrapper plus pid reuse made the helper stamp a reused live process's non-empty field-22 token. v7 closes that window STRUCTURALLY (not temporally): it captures processStartToken(identity.PanePID) through an injectable seam, THEN re-verifies via ProbeTmuxLiveness AFTER the read, and stamps PaneKernelStartToken ONLY on a passing post-read probe (Healthy/TmuxLivenessOK, ObservedPanePID == identity.PanePID), else leaves it '' and accepts no connection. The structural argument is the reaping invariant: a pid cannot be reused while its previous owner is unreaped (zombie); the pane process is reaped by the tmux server; tmux reaping the pane flips it out of the live state — so a Healthy post-read probe with ObservedPanePID == P proves P was not reaped/reused through that probe, and because the probe follows the kernel read, the read saw P's own process. The [probe -> accept] residual is closed by the retained accept-time field-22 match (A3'). I independently confirmed every load-bearing v7 site against the current worktree HEAD: launchPTY runs tmux respawn-pane -k (pty.go:479-484) then CaptureTmuxIdentity (pty.go:493) then attachTmuxPTY returning LaunchResult{PID: identity.PanePID} (pty.go:507, :527-534); ProcessStartToken reads /proc/<pid>/stat field 22 at read time (process_identity_linux.go:13-32); ProbeTmuxLiveness states pane-dead (tmux_liveness.go:257), pane-pid-mismatch (:260), pane-start-mismatch (:265), pane-missing/unavailable (:217-233), and returns Healthy/TmuxLivenessOK with ObservedPanePID (:274-281); PIDLiveWithStartToken compares field 22 and SKIPS the comparison on an empty expectedStart (tmux_liveness.go:392-408, skip at :397). Both falsifiers credit the v7 fix as a genuine structural answer to the v6-framed race and find NO regression of the v6-credited set; I independently confirmed the operand fix and the post-read re-verification are coherent for that window. But the gate does NOT clear. TWO NEW material, source-anchored challenges land — both inside W1, the load-bearing structural no-replay wall for the real channel — and BOTH stand unrebutted (the Holder had no further turn; the cycle ends at adjudication). CHALLENGE 1 (falsifier-reviewer-001, verdict-driving): the v7 capture ORACLE is same-uid mutable. v7's reaping invariant proves the pid was not reaped/reused before the probe ONLY on the assumption that tmux is an untampered witness for the daemon's original launch — it does NOT prove the process being re-verified is the process the daemon launched. I independently confirmed against the worktree that the production tmux control surface is run as the SHARED lane uid: commandInvocationWithEnvFile wraps every RunAsUser command as sudo -n -u <runAsUser> -- env -i (pty.go:98-112); tmuxRunnerForSpec returns RunAsTmuxRunner (pty.go:310-314) whose Run invokes bare tmux via that same run-as path (tmux_liveness.go:125-133); the session name is deterministic metadata derived from run/lane/supervisor ids plus a sha256 suffix (pty.go:620-633); and there is NO private tmux -S socket, TMUX_TMPDIR isolation, or daemon-owned tmux server anywhere in go/pkg/supervisor (only -s session-name args and bare tmux). So a same-uid sibling can address the same per-uid tmux server and, between the daemon's respawn-pane -k (pty.go:479-484) and CaptureTmuxIdentity (pty.go:493) or before the post-read probe, issue its own tmux respawn-pane -k -t <that pane> -- <attacker wrapper>. CaptureTmuxIdentity then records the REPLACEMENT's pid/start, the post-read ProbeTmuxLiveness observes a live same-pane-id/pid/start process (because identity was captured AS the replacement), the helper stamps the replacement's non-empty field-22 token, and W1 accepts the replacement (peer.uid == RunAsUser, peer.pid == result.PID, matching field-22) and delivers the post-auth nonce. The daemon never independently observes the identity of the process it launched — it only ever queries tmux AFTER the launch — so it cannot distinguish its own respawn from a sibling's competing respawn. This is precisely the SEED's own framing of BC1-W1-CAPTURE: 'the kernel start token is captured POST-LAUNCH and is not structurally proven to belong to the BORN [daemon-launched] wrapper.' v7 proves the token belongs to the live PANE process; it does NOT prove it belongs to the born wrapper, because the pane is a same-uid-mutable tmux object. This is the identical same-uid threat model that rejected a 0600 reseal file (a same-uid replay surface). The Holder's spine paragraph ('the captured token can no longer be the sibling's own ... structurally bound to the still-live launched pane (the reaping invariant)') EMBODIES the assumption the challenge breaks; it does not rebut it. CHALLENGE 2 (falsifier-reviewer-002, verdict-driving): the capture-boundary-miss fail-closed path is not lifecycle-wired to the typed floor. The SEED's BC1-W1-CAPTURE resolution criterion (2) requires that a dead/missing/pid-mismatched/identity-unverifiable pane at the capture boundary route the typed session_unrecoverable_across_rotation floor, NEVER a raw launch/control error. v7 places the new check inside launchPTY before attachTmuxPTY/agent_started and asserts the floor is reached via #{pane_dead_status}/recovery-sweep with 'no new error path' — but it names no hook that converts 'unstamped because not reverified' into the typed class. I independently confirmed: RunHelper emits helper_error phase launch on a launch error and returns (helper.go:157-165); the launch handshake treats a helper error before agent_started as a raw 'PTY helper failed before attach' (supervision_launch.go:562-580, esp. :578-579); and the helper event schema admits no typed capture-boundary event, only {agent_exited, agent_started, artifact_observed, attach_client_exited, helper_error, packet_accepted, process_terminated, progress} (supervision.go:220-234). So a pre-attach capture-boundary miss can surface as a raw helper_error/launch-handshake failure — exactly the 'raw launch/control error' the SEED forbids — and the new A3''' negative asserts only 'no stamp / no accept', not the durable typed class. Clearing condition walked (all must hold): (1) BC1-W1-CAPTURE resolved with structural binding to the still-live launched WRAPPER — FAILS: the v6-framed exit+reuse sub-race is resolved, but the binding is to the live pane process, which a same-uid sibling can substitute via respawn-pane, so the token is not structurally bound to the born wrapper (CHALLENGE 1); (2) structural no-replay established on the REAL channel — FAILS: the no-replay spine IS W1, and as specified W1 can authenticate a same-uid sibling's pane replacement on the production tmux path (CHALLENGE 1); (3) the v6-credited resolved set carried forward unregressed with the build-test folded in — HOLDS: both falsifiers sweep the BC1-W1-TOKEN field-22/field-22 operand fix (LaunchResult.PaneKernelStartToken, ProcessStartToken(identity.PanePID), empty-token != '', A3'/A3''), the connect-out topology + named plumbing sites, the non-secret address + post-auth nonce W3, the W2 ordering + dumpable-before-dial, the #{pane_dead_status} exit-code backstop + C2, BC2, BC3, BC4, BC5, the daemon-observed positive intent, the ensureWorkSessionBackend bypass, the W1/W2/W3 wall shapes, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1-A18 discipline, and the modified-since-baseline authored-path build-test, and find NO regression; (4) no new material challenge standing unrebutted — FAILS: two new material challenges land (the same-uid capture oracle and the unwired typed-floor route), both unrebutted; (5) the security invariant holds structurally — PARTIAL: the no-widening half HOLDS (no admin-token read path, no lane-readable reseal bearer, no minted credential carrying {admin, apply, recovery, surgical_recovery}; both falsifiers confirm), but the no-replay half FAILS on the same-uid capture-oracle path and the legible-failure half FAILS on the unwired capture-boundary-miss route. Why not reject: no path widens admin-token exposure and no minted credential carries any elevated verb; both falsifiers explicitly confirm the no-widening invariant and recommend needs_revision, not reject; both gaps are soundness/no-replay/legible-failure defects, not a widening. Why not accept_with_findings: BC1-W1 is the security cluster's last load-bearing closure (F1/F6 and the F2/F7 channel halves inherit it); the SEED demands structural no-replay on the REAL channel and a capture-boundary path that routes the typed floor and never a raw error — a W1 that can authenticate a same-uid pane replacement does not establish structural no-replay, and an unwired fail-closed route can leak a raw helper_error, so neither is a trackable post-clearance finding; each forecloses a clearing verdict. Verdict: needs_revision. The next revision must, retaining the v7 post-read ProbeTmuxLiveness re-verification + the reaping-invariant closure of the exit+reuse window and the v6 field-22/field-22 operand fix (A3'/A3''/A3'''): (A) make the kernel-token capture bind to the DAEMON-LAUNCHED wrapper, not merely to whatever process the same-uid-mutable tmux oracle reports — one of: make the tmux control surface daemon-owned/unreachable to sibling lane processes before and during capture (a private tmux -S socket / TMUX_TMPDIR whose control path the shared lane uid cannot reach, so a sibling cannot address or respawn-pane the pane); OR add an explicit birth/provenance primitive a same-uid respawn-pane cannot forge (a value the daemon controls at the launch it issues, not derivable from a post-respawn tmux query) and stamp PaneKernelStartToken from it before W1 can accept; OR, if neither can be guaranteed, fail closed to the typed floor rather than claim a same-uid-safe authenticated primary channel — and add a real-path negative adjacent to TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper where a same-uid sibling issues a competing respawn-pane against the target pane between the daemon's respawn and CaptureTmuxIdentity/the post-read probe and the helper REFUSES rather than stamping the replacement's token and accepting its frame; AND (B) wire the capture-boundary-miss fail-closed path to the typed session_unrecoverable_across_rotation floor — define a typed helper/supervision event or daemon-side recorder branch for 'PaneKernelStartToken unstamped due to a capture-boundary miss' carrying that class (not generic helper_error), state how it is emitted even when the miss occurs before attachTmuxPTY/agent_started (or move the re-verification to a point where the typed backstop is available), extend TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive and the real-path negative to assert the durable typed class (not only 'no stamp / no accept'), and add a negative that a capture-boundary miss does NOT produce a raw supervisor.helper_error / launch-handshake failure as the terminal explanation. Carry forward unregressed (do NOT reopen): the v7 post-read re-verification, the reaping-invariant closure of the exit+reuse window, the v6 field-22/field-22 operand fix, A3'/A3''/A3''', the connect-out topology and named plumbing sites, the non-secret address + post-auth nonce W3, the W2 ordering + dumpable-before-dial, the #{pane_dead_status} exit-code backstop + C2, the daemon-observed positive intent + backend-gate bypass, BC2, BC3, BC4, BC5, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1-A18 assertion discipline, and the modified-since-baseline authored-path build-test. Maintainer-ratification note (carries regardless of verdict): Slice B (the daemon-internal rpc.CapabilityReseal marker, the test-only auth-prelude route alternate, the daemon-owned supervisor control channel with connect-out SO_PEERCRED pid+kernel-start-token authentication, the reserved agentloop exit codes, the jobs.recovery_generation + leases.reseal_grace_extended_at owner-bundle-0021 columns, and endpoint/epoch republish plumbing) is a security/authz trust-model change requiring maintainer ratification before any build slice touches credential code; adjudicator clearance gates the spec's soundness, not the maintainer's product call. Slice A (the Option-4 typed-exit-code floor) is zero-trust-change but, per the still-open BC1-W1-CAPTURE ground, still must route over the connect-out non-PTY channel whose same-uid authentication (W1) is specified with one coherent kernel identity token STRUCTURALLY BOUND to the launched wrapper — which requires both the same-uid-safe capture oracle (A) and the wired typed-floor route (B) — before it lands."
findings:
  - id: BC1-W1-CAPTURE
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "R2 replay / false-provenance (the v6-framed post-launch exit-then-pid-reuse window is now structurally closed)"
    challenge: "RESOLVED AS FRAMED (both falsifiers credit it; I independently confirmed coherent). The exact v6 ground — the kernel token is captured post-launch so a fast-exiting wrapper plus pid reuse stamps a reused live process's non-empty field-22 token — is closed STRUCTURALLY, not temporally. v7 captures processStartToken(identity.PanePID) through an injectable seam, THEN re-verifies via ProbeTmuxLiveness AFTER the read, and stamps PaneKernelStartToken only on a passing post-read probe (Healthy/TmuxLivenessOK, ObservedPanePID == identity.PanePID), else leaves it '' (engaging the v6 != '' no-pid-only-accept rule) and accepts no connection. The reaping invariant (pid not reusable until reaped; tmux server reaps the pane; reaping flips the pane out of the live state) makes a Healthy post-read probe with ObservedPanePID == P prove P was not reaped/reused through the probe; because the probe follows the kernel read, the read saw P's own process. The [probe -> accept] residual is closed by the retained accept-time field-22 match (A3', PIDLivenessIdentityMismatch at tmux_liveness.go:404). NOTE: this resolves the exit+reuse window ONLY; the broader capture-boundary binding of the token to the daemon-launched wrapper remains open on TWO new grounds — see BC1-W1-ORACLE and BC1-W1-CAPTURE-FLOOR. BC1-W1-CAPTURE therefore does NOT clear the gate even though its originally-framed sub-race is resolved."
  - id: BC1-W1-ORACLE
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2"]
    affected_invariants:
      - "R2 replay / false-provenance (no same-uid-reachable channel a sibling lane can present on the real path)"
    challenge: "OPEN — verdict-driving (NEW; falsifier-reviewer-001). v7 treats tmux as the trusted capture oracle, but the production tmux control surface runs as the SHARED lane uid, so a same-uid sibling can REPLACE the pane before/during capture and v7 binds the kernel token to the replacement, not to the daemon-launched wrapper. Independently confirmed against the current worktree: RunAsUser commands run as sudo -n -u <runAsUser> -- env -i (pty.go:98-112); tmuxRunnerForSpec returns RunAsTmuxRunner (pty.go:310-314) invoking bare tmux via the same run-as path (tmux_liveness.go:125-133); the session name is deterministic (pty.go:620-633); and there is NO private tmux -S socket, TMUX_TMPDIR isolation, or daemon-owned tmux server anywhere in go/pkg/supervisor. Replay: between the daemon's respawn-pane -k (pty.go:479-484) and CaptureTmuxIdentity (pty.go:493) or before the post-read probe, a same-uid sibling issues tmux respawn-pane -k -t <that pane> -- <attacker>; CaptureTmuxIdentity records the replacement's pid/start, the post-read ProbeTmuxLiveness observes a live same-pane-id/pid/start process (because identity was captured AS the replacement), the helper stamps the replacement's non-empty field-22 token, and W1 accepts it and delivers the nonce. The daemon never independently observes the identity of the process it launched (it only queries tmux AFTER launch), so it cannot distinguish its own respawn from a sibling's competing respawn. The reaping invariant does not rebut this — it assumes tmux is an untampered witness; the attacker mutates the witness. This is the SEED's own framing ('not structurally proven to belong to the BORN wrapper') and the identical same-uid threat model that rejected a 0600 reseal file. The v7 real-path negative does not fire this class (it needs a dead/pid-mismatched probe; this attack supplies a HEALTHY replacement). Fix: (a) daemon-owned/private tmux control surface unreachable to sibling lane processes during capture (private tmux -S socket / TMUX_TMPDIR); OR (b) an explicit birth/provenance primitive a same-uid respawn-pane cannot forge, stamped into PaneKernelStartToken before W1 can accept; OR (c) fail closed to the typed floor rather than claim a same-uid-safe authenticated primary channel; plus a real-path negative where a same-uid sibling competes a respawn-pane between the daemon's respawn and CaptureTmuxIdentity/the post-read probe and the helper REFUSES. Keeps F1, F6, and the F2/F7 channel halves open until W1 binds the launch-time identity of the daemon-launched wrapper structurally. needs_revision, not reject (no widening). Both the convergence framing of prior cycles (BC1-CHANNEL -> BC1-W1-TOKEN -> BC1-W1-CAPTURE) continues here: the root is that the daemon's only handle on 'the wrapper' is a post-launch tmux query against a same-uid-mutable oracle."
  - id: BC1-W1-CAPTURE-FLOOR
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "R4 legible-failure (the typed floor must fire on a capture-boundary miss, never a raw launch/control error)"
    challenge: "OPEN — verdict-driving (NEW; falsifier-reviewer-002). v7's capture-boundary-miss fail-closed branch is not lifecycle-wired to the typed floor the SEED requires. The check sits inside launchPTY before attachTmuxPTY/agent_started, and v7 asserts the floor is reached via #{pane_dead_status}/recovery-sweep with 'no new error path', but names no hook converting 'unstamped because not reverified' into the session_unrecoverable_across_rotation class before ordinary stale/unsealed recovery sees it. Independently confirmed against the current worktree: RunHelper emits helper_error phase launch on a launch error and returns (helper.go:157-165); the launch handshake treats a helper error before agent_started as a raw 'PTY helper failed before attach' (supervision_launch.go:562-580, esp. :578-579); the helper event schema admits no typed capture-boundary event, only {agent_exited, agent_started, artifact_observed, attach_client_exited, helper_error, packet_accepted, process_terminated, progress} (supervision.go:220-234). So a pre-attach capture-boundary miss can surface as a raw helper_error/launch-handshake failure — exactly the raw launch/control error the SEED forbids (SEED.md:401-421) — and the connect-out frame schema is sent only AFTER W1 accepts, which the miss path disables. The new A3''' negative asserts only 'no stamp / no accept', not the durable typed class. Fix: (1) define a typed helper/supervision event or daemon-side recorder branch for 'PaneKernelStartToken unstamped due to a capture-boundary miss' carrying class session_unrecoverable_across_rotation (not generic helper_error); (2) state how it is emitted even when the miss occurs before attachTmuxPTY/agent_started, or move the re-verification to a point where the typed backstop is available; (3) extend TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive and the real-path 'pane dies before kernel-token capture' negative to assert the durable typed class; (4) add a negative that a capture-boundary miss does NOT produce a raw supervisor.helper_error / launch-handshake failure as the terminal explanation. needs_revision, not reject (no widening)."
  - id: BC1-W1-TOKEN
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "R2 replay / false-provenance (the W1 operand domain is coherent — field-22 on both sides)"
    challenge: "RESOLVED (carried forward unregressed; both falsifiers confirm). v7 retains the v6 clock-domain operand fix verbatim: LaunchResult.PaneKernelStartToken (pty.go:47-53), captured via ProcessStartToken(identity.PanePID); W1 compares the accepted peer's ProcessStartToken(peer.pid) to that captured kernel token via PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken) (tmux_liveness.go:392-408) — field-22 on both sides, one clock domain; tmux #{pane_start_time} stays liveness metadata only (the ProbeTmuxLiveness expectedStart operand, never the W1 operand); the empty-token fail-open trap stays closed by the != '' assertion (the skip branch is at tmux_liveness.go:397); A3' (same-pid stale-token) and A3'' (empty-token) retained. v7 BUILDS the capture-boundary binding ON this; the operand source is unchanged and must be retained."
  - id: CF
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "CARRIED-FORWARD SET intact (no regression — both falsifiers' regression sweeps confirm). Connect-out topology + named plumbing sites (no fd crosses the tmux boundary; pane wrapper dials OUT after PR_SET_DUMPABLE(0)): unregressed. Non-secret address (STRIATUM_SUPERVISOR_CONTROL_ADDR) + post-auth nonce (W3, daemon->wrapper): unregressed. W2 ordering + dumpable-before-dial: unregressed. #{pane_dead_status} exit-code backstop + C2 (wrapper never propagates a provider child 97/98 into the reserved codes; TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker): unregressed. BC2 (reseal artifact identity from the job's expected_artifacts, daemon-derived, refuses unexpected paths; TestCodexResealUsesReceiverNotProviderStdout): unregressed. BC3 / F4 (CapabilityReseal daemon-internal marker, public route-alternate test-only, records reseal not write on only the three routes; command-authority-matrix reseal column): unregressed. BC4 / BC5 (jobs.recovery_generation + leases.reseal_grace_extended_at pinned to owner bundle 0021, LatestOwnerBundleVersion 20->21 confirmed currently 20 at owner.go:23; stamped-generation predicate; one-grace rule; corrected skip/replace/replay lock-order gate map; artifact.publish + recovery-sweep serialization): unregressed. Daemon-observed positive intent + recovery-sweep backstop + ensureWorkSessionBackend bypass (routing misses to the typed floor): unregressed (BUT the new capture-boundary miss must be wired to that predicate — see BC1-W1-CAPTURE-FLOOR). W1/W2/W3 wall SHAPES: unregressed (the open grounds are W1's capture-boundary identity binding and its fail-closed route, not the wall shapes). F2 (no lane-readable reseal bearer; the v1 0600 same-uid file replay stays retired): held. F7 file-mirror half (daemon-owned lane-read-only 0644 endpoint/epoch, O_NOFOLLOW, atomic rename, missing-epoch rejected, closes #316): held; channel half inherits the open BC1-W1 grounds. AF1 reachability-not-reminting (TestTokenValidAcrossRestart) and AF4 epoch/token decoupling: kept. No-admin-token-widening invariant: held + strengthened (CapabilityReseal carries no elevated verb and is never materialized into a lane-readable file; no minted credential carries {admin, apply, recovery, surgical_recovery}; TestResolveRefusesRuntimeClientTokenForLane). Modified-since-baseline authored-path build-test (gitChangedPathSnapshots/collectInScopeAuthoredPaths, nil baseline for isolated worktrees): folded in and credited. Per-claim falsifiable-assertion discipline A1-A18 (+ A3'/A3''/A3'''/A4'/A7'): extended to the W1 capture-boundary contract. Preserve all verbatim through the next revision."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 design-v7 REVISION (cycle 1)

author: adjudicator-author-001

> Adjudication of the design-v7 REVISION dialogue trajectory for RFC 0143
> (*lane credential survival across a daemon boot-epoch rotation*). This is the
> **seventh** falsification pass on the RFC: v1 (`rfc-0143-design`) returned
> `needs_revision` with seven findings F1–F7; v2 resolved **F2/F4** and distilled
> the residue into the five binding constraints BC1–BC5; v3 resolved **BC2, BC3,
> BC4**; v4 resolved **BC5, C2, and the daemon-observed positive intent**, falling
> on **BC1-CHANNEL** (the W1/W2/W3 walls were specified for a direct
> `exec.Cmd.ExtraFiles` child while the production lane is tmux-backed); v5
> **resolved BC1-CHANNEL** via the **connect-out topology**, falling — both
> falsifiers independently — on **BC1-W1-TOKEN** (W1 compared a kernel `/proc`
> field-22 start-tick against a tmux `#{pane_start_time}` wall-clock timestamp);
> v6 **resolved BC1-W1-TOKEN** (one coherent kernel start-token source,
> field-22/field-22), falling — both falsifiers independently — on
> **BC1-W1-CAPTURE** (the kernel token is captured post-launch and not
> structurally bound to the born wrapper). This v7 run revised the spec to close
> BC1-W1-CAPTURE; the two falsifiers re-attacked it. Inputs read: the revised
> Holder spec (`dialogue/holder/HOLDER.md`), both falsifier re-attacks
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`), the
> `SEED.md` charter (the ratified design shape, the v6-credited carry-forward set,
> and the BC1-W1-CAPTURE constraint with its exact source anchors and four
> resolution criteria), and the **v6** collaboration ledger (the BC1-W1-CAPTURE
> finding + the exact "next revision must…" list). No raw terminal output was
> read. Load-bearing source citations were independently re-verified against the
> current worktree HEAD.

## Verdict

**verdict: needs_revision**

v7 is **materially stronger than v6 on the exact axis v6 fell on**: it **resolves
the v6-FRAMED capture race**. The v6 ground was the post-launch
**exit-then-pid-reuse** window — the kernel token is read after `tmux respawn-pane`
launches the pane and after a `CaptureTmuxIdentity` round trip, so a fast-exiting
wrapper plus pid reuse stamped a reused **live** process's non-empty field-22 token.
v7 closes that window **structurally, not temporally**: it captures
`processStartToken(identity.PanePID)` through an injectable seam, **then** re-verifies
via `ProbeTmuxLiveness` **after** the read, and stamps `PaneKernelStartToken` **only**
on a passing post-read probe (`Healthy`/`TmuxLivenessOK`, `ObservedPanePID ==
identity.PanePID`), else leaves it `""` (engaging the v6 `!= ""` no-pid-only-accept
rule) and accepts no connection. The structural argument is the **reaping invariant**:
a pid is not reusable until its previous owner is reaped; the pane process is reaped
by the tmux server; reaping flips the pane out of the live state — so a `Healthy`
post-read probe with `ObservedPanePID == P` proves `P` was not reaped/reused through
the probe, and because the probe **follows** the kernel read, the read saw `P`'s own
process. The `[probe → accept]` residual is closed by the retained accept-time
field-22 match (A3'). **Both falsifiers credit all of this and find no regression of
the v6-credited set; I independently confirmed the operand fix and the post-read
re-verification are coherent for that window.**

But the gate does **not** clear. **Two NEW material, source-anchored challenges
land — both inside W1, the load-bearing structural no-replay wall — and both stand
unrebutted** (the Holder had no further turn; the cycle ends at adjudication).

**Why not `reject`.** No path widens admin-token exposure, and no minted credential
carries any of `{admin, apply, recovery, surgical_recovery}`; no lane-readable reseal
bearer exists. Both falsifiers confirm the no-widening invariant and recommend
`needs_revision`, not `reject`. Both gaps are **soundness / no-replay /
legible-failure** defects, not a widening.

**Why not `accept_with_findings`.** BC1-W1 is the security cluster's last
load-bearing closure (F1/F6 and the F2/F7 channel halves inherit it). A clearing
verdict requires structural no-replay on the **real** channel and a capture-boundary
path that routes the typed floor and **never** a raw error. A W1 that can authenticate
a same-uid **pane replacement** does not establish structural no-replay; an unwired
fail-closed route can leak a raw `helper_error`. Neither is a trackable
post-clearance finding; each forecloses a clearing verdict.

## The clearing condition, walked

A clearing verdict requires **all** to hold; the W1-capture-dependent ones fail:

1. **BC1-W1-CAPTURE resolved with a structural binding to the still-live launched
   *wrapper* — FAILS.** The v6-framed exit+reuse sub-race is resolved, but the binding
   is to the live **pane process**, which a same-uid sibling can substitute via
   `respawn-pane`; the token is not structurally bound to the **born** (daemon-launched)
   wrapper (CHALLENGE 1 / BC1-W1-ORACLE).
2. **Structural no-replay established on the REAL channel — FAILS.** The no-replay
   spine **is** W1; as specified it can authenticate a same-uid sibling's pane
   replacement on the production tmux path (CHALLENGE 1).
3. **The v6-credited resolved set carried forward unregressed (build-test folded in)
   — HOLDS.** Both falsifiers sweep the BC1-W1-TOKEN field-22/field-22 operand fix
   (incl. `LaunchResult.PaneKernelStartToken`, the empty-token `!= ""` rule, A3'/A3''),
   the connect-out topology + named plumbing sites, the non-secret address + post-auth
   nonce W3, the W2 ordering + dumpable-before-dial, the `#{pane_dead_status}` backstop
   + C2, BC2, BC3, BC4, BC5, the daemon-observed positive intent, the
   `ensureWorkSessionBackend` bypass, the W1/W2/W3 wall *shapes*, F2, F4, the F7
   file-mirror half, AF1, AF4, the no-admin-token-widening invariant, and the A1–A18
   discipline, and find **no regression**; the modified-since-baseline authored-path
   build-test is folded in.
4. **No new material challenge standing unrebutted — FAILS.** Two new material
   challenges land (the same-uid capture **oracle** and the **unwired typed-floor
   route**), both unrebutted.
5. **The security invariant holds structurally — PARTIAL.** The **no-widening** half
   holds; the **no-replay** half fails on the same-uid capture-oracle path, and the
   **legible-failure** half fails on the unwired capture-boundary-miss route.

## What v7 genuinely closed (credited)

- **The v6-framed exit-then-pid-reuse race is dissolved STRUCTURALLY.** The post-read
  `ProbeTmuxLiveness` re-verification (taken **after** the kernel read), gating the
  stamp on `Healthy`/`ObservedPanePID == identity.PanePID`, plus the reaping invariant,
  is a genuine structural answer (not "the window is small") for that window. I confirmed
  the cited states in source: pane-dead (`tmux_liveness.go:257`), pane-pid-mismatch
  (`:260`), pane-start-mismatch (`:265`), pane-missing/unavailable (`:217-233`), and the
  `Healthy`/`TmuxLivenessOK` return with `ObservedPanePID` (`:274-281`).
- **The fail-closed *intent* is specified.** Empty/unreadable/dead/missing/
  pid-mismatched/start-mismatched capture states leave `PaneKernelStartToken == ""`, and
  W1's `!= ""` assertion accepts no connection (no pid-only degrade). *(The remaining
  gap is the typed-floor **route** for that miss — see BC1-W1-CAPTURE-FLOOR.)*
- **The real-path race test is materially improved** with the deterministic unit
  `TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive` (injectable seam + stub
  `TmuxRunner`) and the real-path "pane dies before kernel-token capture" negative.
- **The v6 BC1-W1-TOKEN operand fix is retained** verbatim (field-22/field-22, A3'/A3'')
  and the v6-credited set is **not regressed** (both falsifiers; see finding CF).

## The two open grounds (independently confirmed against the worktree)

### CHALLENGE 1 — BC1-W1-ORACLE: the tmux capture oracle is same-uid mutable (verdict-driving)

v7's reaping invariant proves a passing post-read probe means the pid was not
reaped/reused before the probe **only on the assumption that tmux is an untampered
witness for the daemon's original launch.** It does **not** prove the re-verified
process is the process the daemon launched. I independently confirmed that the
production tmux control surface is run as the **shared lane uid**:

- `commandInvocationWithEnvFile` wraps every `RunAsUser` command as
  `sudo -n -u <runAsUser> -- env -i …` (`pty.go:98-112`).
- `tmuxRunnerForSpec` returns `RunAsTmuxRunner(spec.RunAsUser, spec.Env)`
  (`pty.go:310-314`), whose `Run` invokes **bare `tmux`** via that same run-as path
  (`tmux_liveness.go:125-133`).
- The session name is deterministic metadata derived from run/lane/supervisor ids +
  a sha256 suffix (`pty.go:620-633`).
- There is **no** private `tmux -S` socket, `TMUX_TMPDIR` isolation, or daemon-owned
  tmux server anywhere in `go/pkg/supervisor` (only `-s` session-name args and bare
  `tmux`).

So a same-uid sibling can address the same per-uid tmux server and, between the
daemon's `respawn-pane -k` (`pty.go:479-484`) and `CaptureTmuxIdentity`
(`pty.go:493`) — or before the post-read probe — issue its own
`tmux respawn-pane -k -t <that pane> -- <attacker>`. `CaptureTmuxIdentity` then
records the **replacement's** pid/start, the post-read `ProbeTmuxLiveness` observes a
live same-pane-id/pid/start process (because `identity` was captured **as** the
replacement), the helper stamps the replacement's non-empty field-22 token, and W1
accepts it (`peer.uid == RunAsUser`, `peer.pid == result.PID`, matching field-22) and
delivers the post-auth nonce. **The daemon never independently observes the identity
of the process it launched** — it only ever queries tmux *after* the launch — so it
cannot distinguish its own respawn from a sibling's competing respawn. This is exactly
the SEED's own framing ("not structurally proven to belong to the **born** wrapper")
and the identical same-uid threat model that rejected a `0600` reseal file. The
Holder's spine paragraph ("the captured token can no longer be the sibling's own …
structurally bound to the still-live launched pane (the reaping invariant)")
**embodies** the assumption this challenge breaks; it does not rebut it. The v7
real-path negative does not fire this class (it needs a dead/pid-mismatched probe;
this attack supplies a **healthy** replacement).

### CHALLENGE 2 — BC1-W1-CAPTURE-FLOOR: the capture-boundary-miss typed floor is not wired (verdict-driving)

The SEED's BC1-W1-CAPTURE resolution criterion (2) requires that a dead/missing/
pid-mismatched/identity-unverifiable pane at the capture boundary route the typed
`session_unrecoverable_across_rotation` floor, **never** a raw launch/control error.
v7 places the new check inside `launchPTY` before `attachTmuxPTY`/`agent_started` and
asserts the floor is reached via `#{pane_dead_status}`/recovery-sweep with "no new
error path" — but names **no hook** converting "unstamped because not reverified"
into the typed class. I independently confirmed:

- `RunHelper` emits `helper_error` phase `launch` on a launch error and returns
  (`helper.go:157-165`).
- The launch handshake treats a helper error **before `agent_started`** as a raw
  "PTY helper failed before attach" (`supervision_launch.go:562-580`, esp. `:578-579`).
- The helper event schema admits **no** typed capture-boundary event — only
  `agent_exited, agent_started, artifact_observed, attach_client_exited, helper_error,
  packet_accepted, process_terminated, progress` (`supervision.go:220-234`).

So a pre-attach capture-boundary miss can surface as a raw `helper_error` /
launch-handshake failure — exactly the raw launch/control error the SEED forbids
(`SEED.md:401-421`) — and the connect-out frame schema is sent only **after** W1
accepts, which the miss path disables. The new A3''' negative asserts only "no stamp /
no accept", not the durable typed class.

## Falsifier challenge dispositions

- **falsifier-reviewer-001 — same-uid mutable tmux capture oracle (material; landed
  unrebutted).** Claim challenged: v7 structurally binds the kernel token to the
  *daemon-launched* wrapper. Material? **Yes** — it exposes a real same-uid replay
  surface inside the no-replay wall and would change the spec (a daemon-owned/private
  tmux control surface, or an unforgeable birth/provenance primitive, or fail-closed)
  plus a competing-`respawn-pane` real-path negative. Rebutted? **No** — the reaping
  invariant assumes an untampered tmux witness, which this attack mutates; the Holder
  had no further turn. Disposition: **BC1-W1-ORACLE open; verdict-driving.** No
  widening; recommends `needs_revision`, not `reject`.
- **falsifier-reviewer-002 — capture-boundary-miss typed floor not wired (material;
  landed unrebutted).** Claim challenged: the miss path routes the typed floor and
  never a raw launch/control error. Material? **Yes** — current source routes a
  pre-attach failure as a raw `helper_error`/launch-handshake failure, and no typed
  capture-boundary event exists; the SEED requires the miss path itself to route the
  typed class. Rebutted? **No.** Regression sweep: **no** regression of the
  v6-credited set (BC1-W1-TOKEN, connect-out/W2/W3/C2, BC2/BC3/F4, BC4/BC5,
  daemon-observed intent + backend-gate bypass, authored-path fold-in, F7/AF1/AF4,
  no-widening). Disposition: **BC1-W1-CAPTURE-FLOOR open; lifecycle core otherwise
  intact.**

Both falsifiers **credit the v7 fix on the exact v6-framed race** and then land
**distinct, independent, source-confirmed** new grounds inside W1 — strong
corroboration that the residue is genuine, not reviewer idiosyncrasy. The convergence
progression of prior cycles (BC1-CHANNEL → BC1-W1-TOKEN → BC1-W1-CAPTURE) continues:
the root is that the daemon's only handle on "the wrapper" is a **post-launch tmux
query against a same-uid-mutable oracle**, and the fail-closed leg of that handle is
not yet routed to the typed floor.

## What the next revision MUST fix to clear on re-attack

Retain the v7 post-read `ProbeTmuxLiveness` re-verification + the reaping-invariant
closure of the exit+reuse window and the v6 field-22/field-22 operand fix
(A3'/A3''/A3'''). Then, in addition:

1. **(BC1-W1-ORACLE) Bind the kernel-token capture to the DAEMON-LAUNCHED wrapper, not
   merely to whatever the same-uid-mutable tmux oracle reports.** One of:
   - make the tmux control surface **daemon-owned / unreachable to sibling lane
     processes** before and during capture (a private `tmux -S <socket>` /
     `TMUX_TMPDIR` whose control path the shared lane uid cannot reach, so a sibling
     cannot address or `respawn-pane` the pane); **or**
   - add an explicit **birth/provenance primitive a same-uid `respawn-pane` cannot
     forge** (a value the daemon controls at the launch *it* issues, not derivable from
     a post-respawn tmux query) and stamp `PaneKernelStartToken` from it before W1 can
     accept; **or**
   - if neither can be guaranteed, **fail closed** to the typed floor rather than claim
     a same-uid-safe authenticated primary channel.
   Add a real-path negative adjacent to `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`
   where a same-uid sibling issues a competing `respawn-pane` against the target pane
   between the daemon's respawn and `CaptureTmuxIdentity`/the post-read probe and the
   helper **refuses** rather than stamping the replacement's token and accepting its
   frame.
2. **(BC1-W1-CAPTURE-FLOOR) Wire the capture-boundary-miss fail-closed path to the typed
   `session_unrecoverable_across_rotation` floor.**
   - Define a typed helper/supervision event or daemon-side recorder branch for
     "`PaneKernelStartToken` unstamped due to a capture-boundary miss" carrying that
     class (not generic `helper_error`).
   - State how it is emitted even when the miss occurs before
     `attachTmuxPTY`/`agent_started`, or move the re-verification to a point where the
     typed backstop is available.
   - Extend `TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive` and the
     real-path "pane dies before kernel-token capture" negative to assert the **durable
     typed class** (not only "no stamp / no accept").
   - Add a negative that a capture-boundary miss does **not** produce a raw
     `supervisor.helper_error` / launch-handshake failure as the terminal explanation.

Everything else carries forward **unchanged** (do **not** reopen): the v7 post-read
re-verification, the reaping-invariant closure of the exit+reuse window, the v6
field-22/field-22 operand fix, A3'/A3''/A3''', the connect-out topology and named
plumbing sites, the non-secret address + post-auth nonce (W3), the W2 ordering +
dumpable-before-dial, the `#{pane_dead_status}` exit-code backstop and the C2
commitment, the daemon-observed positive intent + the `ensureWorkSessionBackend`
bypass, BC2, BC3, BC4, BC5, F2, F4, the F7 file-mirror half, AF1, AF4, the
no-admin-token-widening invariant, the A1–A18 assertion discipline, and the
modified-since-baseline authored-path build-test.

## Note on maintainer ratification (carries forward regardless of verdict)

Even when a future revision clears, the chosen direction — the daemon-internal
`rpc.CapabilityReseal` marker, the test-only auth-prelude route alternate, the
daemon-owned supervisor control channel with **connect-out `SO_PEERCRED`
pid+kernel-start-token** authentication, the reserved agentloop exit codes, the
`jobs.recovery_generation` + `leases.reseal_grace_extended_at` owner-bundle-0021
columns, and the endpoint/epoch republish plumbing — is a **security/authz
trust-model change** requiring **maintainer ratification** before any build slice
touches credential code. Adjudicator clearance gates the spec's **soundness**; it is
**not** the maintainer's product call on the credential code. Slice A (the Option-4
typed-exit-code floor) is zero-trust-change, but per the still-open BC1-W1-CAPTURE
ground it must route over the connect-out, non-PTY channel whose same-uid
authentication (W1) is specified with one coherent kernel identity token
**structurally bound to the launched wrapper** — which requires both the
same-uid-safe capture oracle (fix 1) and the wired typed-floor route (fix 2) — before
it lands.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 falsification-gate
design-v7 REVISION run (cycle 1). The ledger verdict — not falsifier completion —
gates the phase: `needs_revision` returns the spec uncleared. v7 dissolves the
v6-framed capture race **structurally** (post-read `ProbeTmuxLiveness`
re-verification before stamping `PaneKernelStartToken`, gated on
`Healthy`/`ObservedPanePID == identity.PanePID`, anchored on the kernel/tmux reaping
invariant) and carries the v6-credited set forward unregressed, but both falsifiers
independently land distinct, source-confirmed new material grounds inside W1: (1) the
tmux capture oracle is same-uid mutable — the production tmux surface runs as the
shared lane uid with a deterministic session name and no private socket, so a
competing `respawn-pane` binds W1 to a sibling's replacement, not to the
daemon-launched wrapper; and (2) the capture-boundary-miss fail-closed path is not
wired to the typed `session_unrecoverable_across_rotation` floor (a pre-attach miss
surfaces as a raw `helper_error`/launch-handshake failure; no typed capture-boundary
event exists). Structural no-replay is therefore not yet established on the production
channel, and BC1-W1 stays open on two narrowly-specified grounds. No admin-token
widening, no lane-readable reseal bearer, no elevated credential — `needs_revision`,
not `reject`; the grounds are inside the load-bearing no-replay wall, so not
`accept_with_findings`.</sub>
