---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 lane credential survival across a daemon boot-epoch rotation — falsifiable implementation spec (design-v5 REVISION; resolve the single remaining binding constraint BC1-CHANNEL)"
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
    text: "v5 is a proper revision starting from the v4 HOLDER spec; it resolves the single remaining binding constraint BC1-CHANNEL in one place and carries the entire v4-credited resolved set forward unregressed. BC1-CHANNEL: the v4 inherited-fd-through-exec.Cmd.ExtraFiles channel is DELETED and replaced by a CONNECT-OUT topology anchored on the production tmux respawn-pane / sudo -- env -i / env-file launch path. The pane agentloop wrapper calls PR_SET_DUMPABLE(0) FIRST, then dials OUT to a daemon-held SO_PASSCRED SOCK_SEQPACKET listener at a NON-SECRET abstract address advertised via the existing env plumbing (STRIATUM_SUPERVISOR_CONTROL_ADDR). The daemon authenticates the connecting peer by SO_PEERCRED uid+pid plus a /proc start-time match against the pane identity it already captured at launch (identity.PanePID / identity.PaneStartToken), accepts the FIRST matching connection and binds the channel to it, then delivers the single-use control_nonce daemon->wrapper post-auth (never via env or fd-inheritance). Named plumbing sites: add ControlSocketAddr to HelperLaunchSpec (helper_protocol.go:27-39) and LaunchSpec (pty.go:30-42); RunHelper (helper.go:128,:149-156) creates the listener, captures result.PID + PaneStartToken, runs acceptControlChannel; new agentloop dumpable_linux.go / control_channel.go / exitcodes.go; add tmux #{pane_dead_status} capture (tmux_liveness.go:228) as the exit-code BACKSTOP. v5 also FLAGS and fixes a material v4 drift: on the tmux path result.Cmd.Wait() resolves the tmux ATTACH CLIENT, not the pane wrapper, so a reserved 97/98 is observed via the new #{pane_dead_status} capture, never from result.Cmd.Wait(); the authenticated connect-out frame is the PRIMARY signal and the exit code a backstop. Adds the real-path test TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper through RunHelper with RequireTmux/RunAsUser asserting W1 accept + provider isolation + sibling refusal together. Folds in falsifier-reviewer-002's modified-since-baseline authored-path build-test (gitChangedPathSnapshots/collectInScopeAuthoredPaths, nil baseline for isolated worktrees). Carries forward unregressed: BC2, BC3, BC4, BC5, C2, the daemon-observed positive intent, the ensureWorkSessionBackend bypass, the W1/W2/W3 wall shapes, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, and the A1-A18 assertion discipline (extended with A3'/A4'/A7')."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "BC1-CHANNEL is fixed in the right direction (connect-out deletes inherited fd 3, the address is non-secret, the nonce is delivered post-auth, and the real plumbing sites are named) but one MATERIAL precision gap remains, and it is inside W1 — the load-bearing structural no-replay wall for the real channel. The W1 peer-credential proof compares ProcessStartToken(peer.pid) — the Linux kernel /proc/<pid>/stat field-22 token (process_identity_linux.go:11-31) — against identity.PaneStartToken, which the spec sources from tmux #{pane_start_time}. But CaptureTmuxIdentity stores verifiedStartToken(#{pane_start_time}) as PaneStartToken WHENEVER tmux returns a numeric value (tmux_liveness.go:181-208) and falls back to ProcessStartToken(panePID) ONLY when the tmux value is absent/non-numeric; verifiedStartToken only checks the value parses as an unsigned integer (:429-436), it does not convert tmux's pane-start timestamp into /proc field 22. TestProbeTmuxLivenessOK treats 1748452211 from tmux as a valid PaneStartToken (tmux_liveness_test.go:51-58), and TestCaptureTmuxIdentityIgnoresLiteralStartToken uses the kernel ProcessStartToken only as a fallback (:259-271). So on the production tmux path the v5 W1 check compares a kernel start tick (left) against a tmux pane-start timestamp (right) — two different clocks: the legitimate pane wrapper can FAIL the start-time check even with the right pid, OR the build is tempted to drop the pid-reuse guard. For a security/authz channel, 'the build will probably normalize this' is not a falsifiable implementation spec; the spec names the WRONG existing identity source for the pid-reuse defense and even cites tmux_liveness.go:194-208 (the code that sets the tmux value) as the W1 token. Required fix (small, within the v5 shape): pin one consistent kernel start-token source for W1 — capture a named kernel token from ProcessStartToken(identity.PanePID) immediately after CaptureTmuxIdentity and before any control connection is accepted, use that for W1, compare it to ProcessStartToken(peer.pid), keep tmux #{pane_start_time} only as liveness metadata unless proven equivalent, and extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper to compare /proc field 22 on both sides plus a negative for a same pid with a mismatched/stale kernel start token. Credited BC1-CHANNEL checks (no regression): production delivery path (no ExtraFiles assumption; no fd crosses the tmux client/server boundary, pty.go:479,:517-533); W2 ordering (no fd or nonce before agentloop; the shim holds only the non-secret address); same-uid socket replay (address knowledge is not authority; SO_PEERCRED must reject the sibling pid); real-path test shape; and the #{pane_dead_status} exit-code drift fix (correct, and not a separate standing BC1 falsification because the primary authenticated frame no longer relies on the exit code). Carry-forward security sweep clean: BC2 (frame carries no path/body/job identity, identity daemon-derived), BC3/F4 (CapabilityReseal daemon-internal, route-alternate test-only), C2, the daemon-observed positive intent + backend-gate bypass, F2 + no-widening (no lane-readable reseal bearer, no admin-token read path, no credential minted with {admin,apply,recovery,surgical_recovery}), F7 file-mirror half / AF1 / AF4 — no regression found. needs_revision, not reject."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "On the assigned lifecycle/regression lens, NO independent BC4/BC5 split-brain or lock-order regression lands: the v4-credited lifecycle set is carried forward in substance — jobs.recovery_generation and leases.reseal_grace_extended_at in owner bundle 0021, the skip/replace/replay gate map, the run-lock serialization against artifact.publish (lockRunForJob first) and the recovery sweep (drain-then-lock-then-expire), the backend-gate bypass confined to resealInFlightJob, and the authored-this-attempt build-test fold-in (gitChangedPathSnapshots + collectInScopeAuthoredPaths, nil baseline for isolated worktrees) are all named coherently. BUT the gate should not clear, for the SAME W1 reason falsifier-reviewer-001 raises, viewed through the lifecycle lens: the v5 connect-out rework makes the authenticated frame one of the two named entry points into resealInFlightJob (alongside the recovery sweep, under the same lock), yet the specified W1 identity check compares two different start-token domains. CaptureTmuxIdentity stores tmux #{pane_start_time} when numeric (tmux_liveness.go:181-208) while ProcessStartToken is the kernel /proc field-22 token (process_identity_linux.go:11-31); the existing unit test treats 1748452211 from tmux as a valid PaneStartToken (tmux_liveness_test.go:46-60). Lifecycle consequence: BC2/BC3/BC4/BC5 can all be correct and the reseal still NEVER enters their predicate from the primary frame, because channel authentication fails before resealInFlightJob obtains the run lock — leaving the design dependent on the #{pane_dead_status} and recovery-sweep backstops while CLAIMING a working primary connect-out entry point. For a security/authz decision, a primary entry point not specified with one consistent process identity is a standing falsification, not a build-run detail. Required fix: add a named kernel identity field/contract captured from /proc/<pane-pid>/stat field 22 after CaptureTmuxIdentity reports the pane pid and before any control connection is accepted; keep tmux #{pane_start_time} as liveness metadata unless proven equivalent; extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper to compare /proc field 22 on both sides plus a negative for a mismatched/stale kernel token. Lifecycle regression sweep clean: BC2, BC3/F4, BC4/BC5 (owner bundle 0021, stamped-generation comparison, one-grace extension, corrected lock-order map, artifact.publish + sweep serialization), the authored-path fold-in (credited), F7 file-mirror / AF1 / AF4 / no-widening — no regression; no route lets a lane read the runtime client-token, no lane-readable credential carries {admin,apply,recovery,surgical_recovery}. needs_revision, not reject."
verdict: "needs_revision"
rationale: "This adjudicates the design-v5 REVISION (the FIFTH falsification pass on RFC 0143) against the SEED clearing condition: a clearing verdict requires BC1-CHANNEL genuinely resolved with a concrete mechanism anchored through the production launch path AND structural no-replay established on the REAL channel AND the entire v4-credited resolved set carried forward unregressed (with the modified-since-baseline build-test folded in) AND no new material challenge standing unrebutted AND the security invariant holding STRUCTURALLY. v5 is materially stronger than v4 on the one axis v4 fell on: it DELETES the v4 inherited-fd-through-ExtraFiles channel and adopts the SEED's connect-out topology, which genuinely dissolves the v4 fd-through-tmux hole — the pane wrapper dials OUT after PR_SET_DUMPABLE(0), no fd crosses the tmux client/server boundary, the listener address is non-secret (W1 authenticates the peer, not knowledge of the address), and the nonce is delivered daemon->wrapper post-auth so it never appears in env or on disk. Both falsifiers credit that the production delivery path no longer assumes exec.Cmd.ExtraFiles, that the W2 ordering is now trivially satisfied (no inherited fd or nonce exists before agentloop runs), that the named plumbing sites reach the pane wrapper and not the tmux client, and that the real-path test shape is correct. v5 also FLAGS and fixes a real v4 drift the gate had not surfaced: on the tmux path result.Cmd.Wait() resolves the ATTACH CLIENT, not the pane wrapper, so a reserved 97/98 must be observed via a new #{pane_dead_status} capture (the authenticated frame is the primary signal, the exit code a backstop) — falsifier-reviewer-001 explicitly credits this and does NOT treat it as a separate standing falsification. But the gate does NOT clear. Both falsifiers INDEPENDENTLY land the SAME material, source-anchored challenge inside W1 — the load-bearing structural no-replay wall for the real channel: the v5 peer-credential proof compares ProcessStartToken(peer.pid) (the kernel /proc/<pid>/stat field-22 token) against identity.PaneStartToken, but the spec sources PaneStartToken from tmux #{pane_start_time}, which current source treats as an OPAQUE tmux start token, not the kernel field-22 value. I INDEPENDENTLY confirmed against the current worktree: CaptureTmuxIdentity stores verifiedStartToken(#{pane_start_time}) as PaneStartToken whenever tmux returns a numeric value and falls back to ProcessStartToken(panePID) only when the tmux value is absent/non-numeric (tmux_liveness.go:181-209, esp. :194-202); verifiedStartToken merely checks the value parses as an unsigned integer; and ProcessStartToken is explicitly the Linux /proc/<pid>/stat field-22 start-tick (process_identity_linux.go:11-32). tmux #{pane_start_time} is a wall-clock unix timestamp (the existing test treats 1748452211 as a valid PaneStartToken) while /proc field 22 is start-ticks since boot — categorically different domains, not merely different formats. So on the PRODUCTION tmux path the v5 W1 check compares a kernel start tick against a tmux pane-start timestamp, and the holder even cites tmux_liveness.go:194-208 (the code that sets the tmux value) as the W1 token source. The consequence is security-load-bearing two ways: either the legitimate pane wrapper is REJECTED before resealInFlightJob ever takes the run lock — so the claimed primary connect-out entry point does not actually work and the design silently depends on the #{pane_dead_status} and recovery-sweep backstops while claiming a working primary — or the build is pressured to DROP/weaken the pid-reuse guard, reopening the same-uid replay surface BC1 exists to close. The SEED and the adjudicator rubric name exactly this as a needs_revision condition: 'a connect-out whose SO_PEERCRED check is not bound to the launched wrapper pid+start-time.' Clearing condition walked (all must hold): (1) BC1-CHANNEL resolved with a concrete mechanism anchored through the production launch path — FAILS: the connect-out topology, the fd-through-tmux closure, the non-secret address, and the W2 ordering are resolved, but the W1 peer-credential authentication — the channel's authentication primitive — is specified against an incoherent identity token and is therefore not yet a coherent, buildable contract; (2) structural no-replay established on the REAL channel — FAILS: the no-replay spine IS W1 (pid+start-time peer-cred), and as written W1 either rejects the real wrapper or must be weakened to pass, so structural no-replay is not established on the production channel; (3) v4-credited set carried forward unregressed with the build-test folded in — HOLDS: both falsifiers sweep BC2, BC3, BC4, BC5, C2, the daemon-observed positive intent, the ensureWorkSessionBackend bypass, the W1/W2/W3 wall shapes, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, and the A1-A18 discipline (extended A3'/A4'/A7') and find NO regression, and the modified-since-baseline authored-path build-test is folded in and credited; (4) no new material challenge standing unrebutted — FAILS: the W1 start-token incoherence is a NEW challenge arising from the v5 connect-out rework, lands in BOTH falsifiers, and the Holder had no further turn (the cycle ends at adjudication), so it stands unrebutted; (5) the security invariant holds structurally — the no-widening half HOLDS (no admin-token read path, no minted credential carrying {admin,apply,recovery,surgical_recovery}) but the no-replay half FAILS on the W1 incoherence. Why not reject: no path widens admin-token exposure and no minted credential carries any elevated verb; both falsifiers explicitly confirm the no-widening invariant; the W1 gap is a soundness defect (fail-closed: it risks rejecting the legit wrapper), not a widening, and falsifier-reviewer-001 recommends needs_revision, not reject. Why not accept_with_findings: BC1-CHANNEL is the security cluster's load-bearing closure (F1/F6 and the F2/F7 channel halves inherit it); the SEED demands structural no-replay on the REAL channel, and a W1 proof whose two sides are different clocks does not establish it — that forecloses a clearing verdict; it is not a trackable post-clearance finding. Verdict: needs_revision. The next revision must, in ONE place: pin a single consistent KERNEL start-token source for W1 — capture a named kernel start token from ProcessStartToken(identity.PanePID) IMMEDIATELY after CaptureTmuxIdentity reports the pane pid and BEFORE any control connection is accepted, persist/use that value for the W1 peer-credential check, and compare the accepted peer's ProcessStartToken(peer.pid) (/proc field 22) to that captured kernel token; keep tmux #{pane_start_time} only as liveness metadata unless the implementation PROVES it equivalent to /proc field 22 on supported hosts; and extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper so the accepted connect-out frame compares /proc/<peer-pid>/stat field 22 to the captured /proc/<pane-pid>/stat field 22, adding a negative that rejects the same pid with a mismatched/stale kernel start token. Carry forward unregressed (do NOT reopen): the connect-out topology and named plumbing sites, the non-secret address + post-auth nonce (W3), the W2 ordering + dumpable-before-dial, the #{pane_dead_status} exit-code backstop and the C2 commitment, the daemon-observed positive intent + backend-gate bypass, BC2, BC3, BC4, BC5, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, the A1-A18 assertion discipline, and the modified-since-baseline authored-path build-test. Maintainer-ratification note (carries regardless of verdict): Slice B (the daemon-internal rpc.CapabilityReseal marker, the test-only auth-prelude route alternate, the daemon-owned supervisor control channel with connect-out SO_PEERCRED pid+start-time authentication, the reserved agentloop exit codes, the jobs.recovery_generation + leases.reseal_grace_extended_at owner-bundle-0021 columns, and endpoint/epoch republish plumbing) is a security/authz trust-model change requiring maintainer ratification before any build slice touches credential code; adjudicator clearance gates the spec's soundness, not the maintainer's product call. Slice A (the Option-4 typed-exit-code floor) is zero-trust-change but, per the still-open BC1-CHANNEL, still must route over a real, non-PTY channel whose same-uid authentication is anchored through the production tmux/sudo/env-file launch path AND specified with one coherent kernel identity token before it lands."
findings:
  - id: BC1-CHANNEL
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2", "dialogue:3"]
    affected_invariants:
      - "R2 replay / false-provenance (no same-uid-reachable channel a sibling lane can present on the real path)"
      - "product-boundary: terminal output is not authoritative workflow state"
      - "R4 legible-failure (the typed floor must fire, never leak a raw backend error)"
    challenge: "OPEN on a single material precision ground inside W1; the connect-out topology and its installation otherwise CLEAR. v5 deletes the v4 inherited-fd-through-exec.Cmd.ExtraFiles channel and adopts a CONNECT-OUT channel: the pane wrapper sets PR_SET_DUMPABLE(0) first then dials a daemon-held SO_PASSCRED listener at a non-secret abstract address (advertised over the existing env plumbing), the nonce is delivered daemon->wrapper post-auth, and the named sites reach the pane wrapper (ControlSocketAddr on HelperLaunchSpec helper_protocol.go:27-39 / LaunchSpec pty.go:30-42; RunHelper helper.go:128,:149-156 creates the listener + acceptControlChannel; new agentloop dumpable_linux.go/control_channel.go/exitcodes.go; tmux #{pane_dead_status} capture tmux_liveness.go:228). This dissolves the v4 fd-through-tmux hole (no fd crosses the tmux client/server boundary; W2 ordering trivially satisfied since no fd/nonce exists pre-agentloop) and BOTH falsifiers credit it; v5 also flags+fixes a real v4 drift (result.Cmd.Wait() is the tmux ATTACH CLIENT, not the pane wrapper, so 97/98 is observed via #{pane_dead_status}, with the authenticated frame primary and the exit code a backstop). What stays OPEN (both falsifiers, independently; INDEPENDENTLY confirmed against current source): the W1 peer-credential proof — the load-bearing structural no-replay wall — compares ProcessStartToken(peer.pid) (kernel /proc/<pid>/stat field 22, process_identity_linux.go:11-32) against identity.PaneStartToken, but CaptureTmuxIdentity sources PaneStartToken from tmux #{pane_start_time} whenever numeric (tmux_liveness.go:194-202; verifiedStartToken only checks it parses as an unsigned int), falling back to the kernel token only when tmux is absent/non-numeric. tmux #{pane_start_time} is a wall-clock unix timestamp (the existing test treats 1748452211 as a valid PaneStartToken, tmux_liveness_test.go) while /proc field 22 is start-ticks since boot — different domains. So on the production tmux path W1 compares two different clocks: either the legitimate pane wrapper is REJECTED before resealInFlightJob takes the run lock (the claimed primary connect-out entry point does not work; the design silently leans on the #{pane_dead_status}/recovery-sweep backstops) or the build is pressured to drop the pid-reuse guard (reopening the same-uid replay surface). Fix in ONE place: capture a named KERNEL start token from ProcessStartToken(identity.PanePID) immediately after CaptureTmuxIdentity and before any control connection is accepted; use it for W1 and compare it to ProcessStartToken(peer.pid); keep tmux #{pane_start_time} as liveness metadata only unless proven equivalent; extend TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper to compare /proc field 22 on BOTH sides plus a negative for the same pid with a mismatched/stale kernel start token. Keeps F1, F6, and the F2/F7 channel halves open until W1 is specified with one coherent kernel identity; the Slice-A exit-97 floor is plausibly closed once W1 is coherent."
  - id: BC2
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "RESOLVED (carried forward unregressed; both falsifiers find no regression). The connect-out SupervisorControlFrame carries NO job_id / artifact path / kind / body — identity is derived from daemon state. resealInFlightJob derives the expected-artifact set from the job's expected_artifacts (daemon state, attempt-resolved via resolveExpectedArtifactCycles), reuses verifyRequiredArtifacts / ensurePerJobPublishedArtifactsDurable (mutations.go:828-876), publishes only a path that is an open expected entry from the job's own worktree, refuses unexpected paths, and routes a front-matter/author-line failure to the Option-4 floor. The positive case (a complete-on-disk post-rotation deliverable IS resealed by the daemon-observed condition) is preserved. Keep TestCodexResealUsesReceiverNotProviderStdout (negative + positive)."
  - id: BC3
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "RESOLVED (carried forward unregressed; both falsifiers find no regression). CapabilityReseal stays a daemon-internal marker projected by resealInFlightJob (supervisor_id->session_id, internal AuthContext without the public Authorize prelude, rpc/server.go:107-111), with the public route-alternate (MethodEntry.ResealAlternate on only interrogation.answer/work.complete/artifact.publish, recording reseal not write) kept TEST-ONLY since no production bearer exists; registry_methods.go generated, command-authority-matrix reseal column + authority guardrail updated. No live caller can present CapabilityReseal. Keep TestResealCapabilityIsDaemonInternalNotBearer / TestResealTokenCanReachOnlyResealRoutesWithoutWrite."
  - id: BC4
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:3"]
    challenge: "RESOLVED (carried forward unregressed; falsifier-reviewer-002 confirms). Concrete monotonic jobs.recovery_generation column in owner bundle 0021 (LatestOwnerBundleVersion 20->21, ordinal-21 reservation, modelled on review_generation); degrade-safe JobRecoveryGenerationColumnPresent probe routes to the typed floor when absent; four increment points (claim, requeue-same-attempt, recovery-sweep expire/transfer/respawn, release) each in the lease-retiring/rebinding UPDATE under lockRun; post-increment value stamped into work_packets.packet_json lease.recovery_generation and compared equal/unequal at reseal under the lock. Keep TestResealPredicateUsesStampedRecoveryGeneration / TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue."
  - id: BC5
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "R3 split-brain across rotation"
      - "R4 legible-failure (typed class must fire, never a raw lease_error)"
    challenge: "RESOLVED (carried forward unregressed; falsifier-reviewer-002, on the lifecycle lens plus a regression sweep, finds no standing material gap). (1) leases.reseal_grace_extended_at PINNED to the same owner bundle 0021 as jobs.recovery_generation (leases owner-held — created runtime 0005, absent from the owner/0018 transfer cohort), so TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn / TestFutureRuntimeMigrationsDoNotCarryOwnerDDL can fire. (2) Numeric grace (min(resealGraceWindow, heartbeat_after_seconds)) with ONE same-lease extension gated by reseal_grace_extended_at; CORRECTED skip/replace/replay lock-order gate map (resealInFlightJob skips/replaces enforceSessionBindingForSession, enforceActiveActingSession, activeLeaseFor, ensureWorkSessionBackend; replays lockRunForJob first, the FOR UPDATE rows, the reseal predicate, write-scope, durability, running->completed); artifact.publish takes lockRunForJob first; the sweep drains->locks->expires, so expired-beyond-grace ALWAYS routes the typed session_unrecoverable_across_rotation class and a requeued lease is never revived. The modified-since-baseline authored-path build-test is folded in and credited. Keep TestResealBeyondGraceRoutesTypedNotLeaseError / TestResealGraceCannotReviveRequeuedLease / TestRecoveryRequeueWinsOverExpiredLeaseReseal / GD-1b / TestResealExit98BypassesBackendGateOrRoutesTyped."
  - id: CF
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "CARRIED-FORWARD SET intact (no regression — both falsifiers confirm). C2: the wrapper never propagates a provider child 97/98 into the reserved agentloop codes (remap to a non-control agent_exited); TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker. Daemon-observed positive intent + ensureWorkSessionBackend bypass: the reseal predicate stays daemon-observed (post-rotation + stamped-generation match + lease-in-grace + every required expected_artifact present AND authored-this-attempt) and bypasses the backend gate only inside resealInFlightJob, routing misses to the typed floor. W1/W2/W3 wall SHAPES: unregressed (the open ground is only W1's start-token SOURCE, not the wall shapes). F2: no lane-readable reseal bearer reintroduced; the v1 0600 same-uid file replay stays retired. F4: route-alternate limited to the three routes, records reseal not write, test-only. F7 file-mirror half: daemon-owned lane-read-only 0644 endpoint/epoch, O_NOFOLLOW, atomic rename, missing-epoch rejected (closes #316); channel half inherits the open BC1-CHANNEL. AF1 reachability-not-reminting (TestTokenValidAcrossRestart) and AF4 epoch/token decoupling: kept. No-admin-token-widening invariant: held + strengthened (CapabilityReseal carries no elevated verb and is never materialized into a lane-readable file; no minted credential carries admin/apply/recovery/surgical_recovery; TestResolveRefusesRuntimeClientTokenForLane). Modified-since-baseline authored-path build-test (gitChangedPathSnapshots/collectInScopeAuthoredPaths, nil baseline for isolated worktrees): folded in and credited. Per-claim falsifiable-assertion discipline: extended to A1-A18 + A3'/A4'/A7' covering the connect-out installation, the W2 ordering on the real path, and the #{pane_dead_status} backstop. Preserve all verbatim through the next revision."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 design-v5 REVISION (cycle 1)

author: adjudicator-author-001

> Adjudication of the design-v5 REVISION dialogue trajectory for RFC 0143
> (*lane credential survival across a daemon boot-epoch rotation*). This is the
> **fifth** falsification pass on the RFC: v1 (`rfc-0143-design`) returned
> `needs_revision` with seven findings F1–F7; v2 resolved **F2/F4** and distilled
> the residue into the five binding constraints BC1–BC5; v3 resolved **BC2, BC3,
> BC4**; v4 resolved **BC5, two of BC1's three sub-grounds (C2 + the
> daemon-observed positive intent with the backend-gate bypass)** and carried the
> v3-credited set forward, but returned `needs_revision` on a single ground:
> **BC1-CHANNEL — the W1/W2/W3 walls were specified for a direct
> `exec.Cmd.ExtraFiles` child exec, while the production supervised lane is
> tmux-backed, so the control-fd delivery through the real launch path was
> unspecified.** This v5 run revised the spec to close BC1-CHANNEL; the two
> falsifiers re-attacked it. Inputs read: the revised Holder spec
> (`dialogue/holder/HOLDER.md`), both falsifier re-attacks
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`), the
> `SEED.md` charter (the ratified design shape, the v4-credited carry-forward set,
> and the BC1-CHANNEL constraint with its exact source anchors), the **v4**
> `HOLDER.md` (the spec being revised) and the **v4** collaboration ledger (the
> BC1-CHANNEL finding + the exact "next revision must…" list). No raw terminal
> output was read. Load-bearing source citations were independently re-verified
> against the current worktree.

## Verdict

**verdict: needs_revision**

v5 is **materially stronger than v4 on the exact axis v4 fell on**: it **deletes**
the v4 inherited-fd-through-`exec.Cmd.ExtraFiles` channel and adopts the SEED's
**connect-out topology**, which genuinely dissolves the v4 *fd-through-tmux* hole —
the pane wrapper dials **out** after `PR_SET_DUMPABLE(0)`, **no fd crosses the tmux
client/server boundary**, the listener address is **non-secret** (W1 authenticates
the peer, not knowledge of the address), and the nonce is delivered
**daemon→wrapper post-auth** so it never appears in env or on disk. v5 also **flags
and fixes a real v4 drift** the prior gate had not surfaced: on the tmux path
`result.Cmd.Wait()` resolves the **attach client**, not the pane wrapper, so a
reserved 97/98 is observed via a new `#{pane_dead_status}` capture (the
authenticated frame is primary, the exit code a backstop). But the gate does
**not** clear: **both falsifiers independently land the same material,
source-anchored challenge inside W1** — the load-bearing structural no-replay wall —
and it stands **unrebutted**.

**Why not `reject`.** No path widens admin-token exposure, and no minted credential
carries any of `{admin, apply, recovery, surgical_recovery}`. Both falsifiers
confirm the no-widening invariant holds; falsifier-reviewer-001 recommends
`needs_revision`, not `reject`; and the W1 gap is a fail-closed **soundness** defect
(it risks *rejecting* the legitimate wrapper), not a widening.

**Why not `accept_with_findings`.** BC1-CHANNEL is the security cluster's
load-bearing closure for F1/F6 and the F2/F7 channel halves. A clearing verdict
requires structural no-replay to hold on the **real** channel — and the no-replay
spine **is** W1 (pid + start-time peer-credential). As specified, W1 compares a
kernel `/proc` field-22 start tick on the peer side against a **tmux
`#{pane_start_time}` wall-clock timestamp** on the captured side — **two different
clocks** — so it either rejects the real wrapper or must be weakened to pass.
Structural no-replay is therefore **not established** on the production channel.
That is not a trackable post-clearance finding; it forecloses a clearing verdict.

### The clearing condition, walked

A clearing verdict requires **all** to hold; **the BC1-CHANNEL-dependent ones fail**:

1. **BC1-CHANNEL resolved with a concrete mechanism anchored through the production
   launch path — FAILS.** The connect-out topology, the fd-through-tmux closure, the
   non-secret address, and the W2 ordering are resolved; but the **W1
   peer-credential authentication — the channel's authentication primitive — is
   specified against an incoherent identity token** and is not yet a coherent,
   buildable contract.
2. **Structural no-replay established on the REAL channel — FAILS.** The no-replay
   spine is W1; as written it either rejects the legitimate wrapper or must be
   weakened, so structural no-replay is not established on the production tmux
   channel.
3. **The v4-credited resolved set carried forward unregressed (build-test folded
   in) — HOLDS.** Both falsifiers sweep BC2, BC3, BC4, BC5, C2, the daemon-observed
   positive intent, the `ensureWorkSessionBackend` bypass, the W1/W2/W3 wall
   *shapes*, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening
   invariant, and the A1–A18 discipline (extended A3'/A4'/A7') and find **no
   regression**; the modified-since-baseline authored-path build-test is folded in
   and credited.
4. **No new material challenge standing unrebutted — FAILS.** The W1 start-token
   incoherence is a **new** challenge arising from the v5 connect-out rework, lands
   in **both** falsifiers, and the Holder had no further turn (the cycle ends at
   adjudication), so it stands unrebutted.
5. **The security invariant holds structurally — partial.** The **no-widening** half
   holds; the **no-replay** half fails on the W1 incoherence.

## Constraint-by-constraint walk

| Constraint | Cluster | Disposition | One-line reason |
| --- | --- | --- | --- |
| **BC1-CHANNEL** | security | **open** (W1 start-token source) | Connect-out topology, fd-through-tmux closure, non-secret address, W2 ordering, and the `#{pane_dead_status}` drift fix all CLEAR; but W1 compares `ProcessStartToken` (kernel `/proc` field 22) against `identity.PaneStartToken` (tmux `#{pane_start_time}` — a different clock), so the real-channel no-replay proof is incoherent. |
| **BC2** | security | **resolved** | Frame carries no path/body/job identity; reseal identity derived from daemon state; refuses unexpected paths; front-matter failure → floor. |
| **BC3** | security | **resolved** | `CapabilityReseal` daemon-internal marker; public route-alternate test-only. |
| **BC4** | lifecycle | **resolved** | Monotonic `jobs.recovery_generation` (owner bundle 0021), stamped value compared under the lock. |
| **BC5** | lifecycle | **resolved** | `leases.reseal_grace_extended_at` pinned to owner bundle 0021; numeric grace + one extension; corrected skip/replace/replay gate map and reseal-vs-sweep serialization. |
| **CF** | carry-forward | **intact** | C2, daemon-observed intent + backend-gate bypass, W1/W2/W3 *shapes*, F2, F4, F7 file-mirror half, AF1, AF4, no-widening, A1–A18, and the authored-path build-test — all unregressed. |

## What v5 genuinely closed (credited)

- **The v4 fd-through-tmux hole is dissolved.** The connect-out channel removes the
  inherited control fd entirely; there is no fd to route through the tmux
  client/server boundary or the env-file shim. Both falsifiers credit that v5 no
  longer assumes `exec.Cmd.ExtraFiles` (`pty.go:479`, `:517-533`).
- **W2 ordering is now trivially satisfied on the real path.** No control fd or
  nonce exists before agentloop runs; the env-file shim can hold only the
  **non-secret** address, and `PR_SET_DUMPABLE(0)` runs before the wrapper dials.
  Reinforced by the `sudo` setuid launch resetting `dumpable`.
- **Address-knowledge-is-not-authority.** The abstract listener address is
  non-secret by construction; a sibling that learns it and connects is refused on
  pid/start-time. The nonce is delivered daemon→wrapper **post-auth**.
- **Named plumbing reaches the pane wrapper, not the tmux client.**
  `ControlSocketAddr` on `HelperLaunchSpec` (`helper_protocol.go:27-39`) /
  `LaunchSpec` (`pty.go:30-42`); `RunHelper` (`helper.go:128`, `:149-156`) creates
  the listener + `acceptControlChannel`; new agentloop `dumpable_linux.go` /
  `control_channel.go` / `exitcodes.go`.
- **A real v4 drift flagged and fixed.** On the tmux path `result.Cmd.Wait()`
  resolves the **attach client**, so the pane wrapper's reserved 97/98 must be read
  via a new `#{pane_dead_status}` capture (`tmux_liveness.go:228`), with the
  authenticated frame primary and the exit code a backstop. falsifier-reviewer-001
  explicitly credits this and does not treat it as a separate standing
  falsification.

## The one open ground — BC1-CHANNEL W1 start-token coherence (independently confirmed)

The v5 W1 peer-credential check (HOLDER §BC1-CHANNEL-(a) point 3) accepts a
connecting peer iff `peer.uid == RunAsUser uid`, `peer.pid == result.PID`
(`identity.PanePID`), and `ProcessStartToken(peer.pid)` (kernel `/proc/<pid>/stat`
field 22) `== identity.PaneStartToken`. The defect is the **right-hand operand**.
I independently confirmed against the current worktree:

- `CaptureTmuxIdentity` sets `PaneStartToken = verifiedStartToken(parts[3])` where
  `parts[3]` is tmux `#{pane_start_time}`, and falls back to
  `ProcessStartToken(panePID)` **only** when the tmux value is empty/non-numeric
  (`go/pkg/supervisor/tmux_liveness.go:181-209`, esp. `:194-202`).
- `verifiedStartToken` merely checks the value parses as an unsigned integer — it
  does **not** convert a tmux pane-start timestamp into a `/proc` field-22 token.
- `ProcessStartToken` is explicitly the Linux `/proc/<pid>/stat` field-22 start tick
  (`go/pkg/supervisor/process_identity_linux.go:11-32`).

tmux `#{pane_start_time}` is a wall-clock unix timestamp (the existing
`TestProbeTmuxLivenessOK` treats `1748452211` as a valid `PaneStartToken`) while
`/proc` field 22 is start-ticks since boot — **categorically different domains**.
So on the **production tmux path** (where tmux returns a numeric value) the v5 W1
check compares a kernel start tick against a tmux pane-start timestamp. The holder
even cites `tmux_liveness.go:194-208` — the code that sets the tmux value — as the
W1 token source.

**Why this is material, not a build-run detail.** W1 is the load-bearing structural
no-replay wall for the real channel, and the authenticated frame is one of the two
named entry points into `resealInFlightJob` (under the run lock). With the operands
in different domains, **either** the legitimate pane wrapper is rejected before the
reseal predicate ever runs — so the claimed *primary* connect-out entry point does
not actually work and the design silently leans on the `#{pane_dead_status}` and
recovery-sweep backstops — **or** the build is pressured to drop/weaken the
pid-reuse guard, reopening the same-uid replay surface BC1 exists to close. The SEED
and the adjudicator rubric name exactly this as a `needs_revision` condition: *"a
connect-out whose `SO_PEERCRED` check is not bound to the launched wrapper
pid+start-time."* For a security/authz channel, "the build will probably normalize
this" is not a falsifiable implementation contract.

## Falsifier challenge dispositions

- **falsifier-reviewer-001 — W1 start-time binding uses two different clocks
  (material; landed unrebutted).** Claim challenged: the daemon can authenticate the
  connect-out peer by `/proc` field-22 start-time matched against the
  already-captured `PaneStartToken`. Material? **Yes** — it would change the spec
  (pin a single kernel start-token source for W1) and exposes a real security
  defect inside the no-replay wall. Rebutted? **No** — the spec text names the
  tmux-sourced token and cites the code that sets it; the Holder had no further
  turn. Disposition: **BC1-CHANNEL remains open; verdict-driving.** No
  admin-token widening; recommends needs_revision, not reject.
- **falsifier-reviewer-002 — primary reseal frame uses an incoherent pane identity
  token (material; landed unrebutted).** Claim challenged: the authenticated frame
  is a working primary entry point into `resealInFlightJob`. Material? **Yes** —
  same W1 domain mismatch, viewed through the lifecycle lens: channel
  authentication can fail before the reseal predicate takes the run lock, so the
  primary channel is not yet a falsifiable contract. Rebutted? **No.** Lifecycle
  regression sweep: **no** BC2/BC3/BC4/BC5/F7/AF1/AF4/no-widening regression; the
  authored-path build-test fold-in credited. Disposition: **lifecycle core intact;
  BC1-CHANNEL open on the W1 ground.**

Both falsifiers converge **independently** on the identical source-anchored W1
defect — strong corroboration that this is a genuine standing falsification, not a
single reviewer's idiosyncrasy.

## What the next revision MUST fix to clear on re-attack

One item, within BC1-CHANNEL — the W1 start-token source of truth:

1. **Pin one consistent KERNEL start-token source for W1.** Capture a named kernel
   start token from `ProcessStartToken(identity.PanePID)` **immediately after**
   `CaptureTmuxIdentity` reports the pane pid and **before any control connection is
   accepted**; persist/use that value for the W1 peer-credential check; compare the
   accepted peer's `ProcessStartToken(peer.pid)` (`/proc` field 22) to that captured
   kernel token. Keep tmux `#{pane_start_time}` as **liveness metadata only** unless
   the implementation **proves** it equivalent to `/proc` field 22 on supported
   hosts.
2. **Make the real-path test assert the same kernel token on both sides.** Extend
   `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (through `RunHelper` with
   `RequireTmux`/`RunAsUser`) so the accepted connect-out frame compares
   `/proc/<peer-pid>/stat` field 22 to the captured `/proc/<pane-pid>/stat` field
   22, **and** add a negative that rejects the same pid with a mismatched/stale
   kernel start token.

Everything else is sound and carries forward **unchanged**: the connect-out
topology and named plumbing sites, the non-secret address + post-auth nonce (W3),
the W2 ordering + dumpable-before-dial, the `#{pane_dead_status}` exit-code backstop
and the C2 commitment, the daemon-observed positive intent + the
`ensureWorkSessionBackend` bypass, BC2, BC3, BC4, BC5, F2, F4, the F7 file-mirror
half, AF1, AF4, the no-admin-token-widening invariant, the A1–A18 assertion
discipline, and the modified-since-baseline authored-path build-test.

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
typed-exit-code floor) is zero-trust-change, but per the still-open BC1-CHANNEL it
must route over a real, non-PTY channel whose same-uid authentication is anchored
through the production tmux/sudo/env-file launch path **and specified with one
coherent kernel identity token** before it lands.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 falsification-gate
design-v5 REVISION run (cycle 1). The ledger verdict — not falsifier completion —
gates the phase: `needs_revision` returns the spec uncleared. v5 dissolves the v4
fd-through-tmux hole with a connect-out channel and carries the v4-credited set
forward unregressed (flagging and fixing a real exit-code-on-tmux drift along the
way), but both falsifiers independently land the same material, source-confirmed
W1 defect — the peer-credential start-token compares a kernel `/proc` field-22 tick
against a tmux `#{pane_start_time}` wall-clock value — so structural no-replay is
not yet established on the production channel and BC1-CHANNEL stays open on a single,
narrowly-specified ground.</sub>
