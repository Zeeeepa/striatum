---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 lane credential survival across a daemon boot-epoch rotation — falsifiable implementation spec (design-v4 REVISION; resolve BC1 + BC5)"
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
    text: "v4 is a proper revision starting from the v3 HOLDER spec; it resolves the two remaining binding constraints in one place each and carries the v3-credited set forward unregressed. BC1: the inherited-fd control channel is now claimed structurally same-uid-authenticated by three independent walls — W1 per-message SCM_CREDENTIALS peer-credentials binding every frame to the launched wrapper pid plus start-time (SO_PASSCRED on the helper end; load-bearing even if W2/W3 leak), W2 PR_SET_DUMPABLE(0) as the wrapper first runtime action reinforced by the sudo setuid launch so the /proc fd and environ surfaces are root-owned, W3 the single-use control_nonce delivered over the socketpair itself and out of the env; reserved exit codes 97/98 are reserved by commitment (the wrapper never propagates a provider child status into them, remapping a child 97/98 to a non-control agent_exited); and the positive intent is daemon-OBSERVED, not provider-asserted — resealInFlightJob fires only on a precise post-rotation condition the daemon verifies against its own state plus worktree (boot-epoch rotated since the packet; job still running with the stamped recovery_generation matching live and lease within grace; every required expected_artifact present and content-hash-modified since the write_scope_baseline), with a recovery-sweep backstop and ensureWorkSessionBackend deliberately bypassed so a stopped supervisor routes the typed class instead of leaking a backend error. BC5: leases.reseal_grace_extended_at is pinned to the same owner bundle 0021 as jobs.recovery_generation (leases is owner-held — created by runtime 0005, absent from the owner/0018 transfer cohort), and the work.complete lock-order story is corrected with an exact gate map (resealInFlightJob skips/replaces enforceSessionBindingForSession, enforceActiveActingSession, activeLeaseFor and ensureWorkSessionBackend, and replays lockRunForJob, the FOR UPDATE rows, the reseal predicate, write-scope, durability and the running->completed transition). Carries forward BC2, BC3, BC4, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, and the assertion discipline (now A1-A18)."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "BC1 is not genuinely closed on the production launch topology. The v4 walls are the right walls for the v3 /proc replay, but the spec installs them via a DIRECT exec.Cmd.ExtraFiles child exec, while the production supervised lane is tmux-backed: launchPTY creates a tmux session and runs the lane command via tmux respawn-pane (pty.go:402-483, :506-533) under sudo -n -u RunAsUser -- env -i (pty.go:98-112, :214-223) wrapped by the env-file shell shim (set -a; . arg1; rm; shift; exec args, pty.go:24, :282-287). HelperLaunchSpec has NO control-fd field (helper_protocol.go:24-39); LaunchSpec has no ExtraFiles or control-fd field (pty.go:27-41); RunHelper forwards only command/env/wd/run-as/tmux (helper.go:149-156). So the load-bearing migration site is missing: passing an fd via ExtraFiles to the tmux client does not prove the pane wrapper inherited it; passing it through the env-file shim makes fd 3 live BEFORE agentloop can run PR_SET_DUMPABLE(0), contradicting the required ordering; and any env-var, filesystem-socket-name, or lane-readable handoff to bridge tmux reopens the same same-uid surface BC1 must close. The named tests can pass on a direct os/exec harness while the real tmux lane never receives fd 3 (TestControlFrameRequiresExpectedWrapperPeerCredentials proves W1 for a direct child; TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc proves dumpability after agentloop starts but misses the earlier tmux/env-shim window). C2 (reserved exit codes 97/98 reserved by commitment) and the daemon-observed positive intent plus the ensureWorkSessionBackend bypass are credited resolved, and the v3-credited security carry-forward set (BC2, BC3, F2, F4, no-widening) is not regressed. Required: pin control-fd delivery and dumpability through the production tmux/run-as/env-file path (or explicitly change the launch topology), name the exact sites, and add a real-path test such as TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper run through RunHelper with RequireTmux/RunAsUser. needs_revision, not reject (no admin-token widening, no lane-readable elevated credential)."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: not_material
    text: "Under the BC5 lifecycle lens no independent falsification lands; BC5 should be treated as resolved by this revision. Migration site RESOLVED: striatumd.leases is owner-held (created in runtime 0005:166-182; absent from the owner/0018 transfer cohort:80-90; the owner frontier is 20 so 0021 is the next slot), so the v3 owner-bundle-vs-runtime ambiguity is gone and TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn / TestFutureRuntimeMigrationsDoNotCarryOwnerDDL can actually fire. Lock-order RESOLVED at design level: source matches the corrected diagnosis (HandleCompleteWork runs enforceSessionBindingForSession and enforceActiveActingSession before lockRunForJob, and activeLeaseFor plus ensureWorkSessionBackend after, lifecycle.go:1124-1182; artifact.publish takes lockRunForJob first, artifact.go:75-85; the sweep drains then locks then expires, recovery.go:575-621), and v4 names the exact skip/replace/replay map and the reseal-vs-sweep serialization. BC4, the F7 file-mirror half, and the no-admin-token-widening invariant are not regressed under this lens. One implementation-watch item only: the modified-since-packet proof must NOT treat present-and-absent-from-write_scope_baseline.changed_paths as sufficient by itself — for per-job isolated worktrees the baseline is nil (write_scope_guard.go:69-85) and source-change publication already uses gitChangedPathSnapshots plus collectInScopeAuthoredPaths (claim.go:601-630; artifact_source_publish.go:69-88, :255-290), so the build must reuse that authored-path attribution; this is a build-test precision item closable by a single test (seed a clean pre-existing expected path, assert typed floor; modify it, assert positive reseal) folded into the positive TestCodexResealUsesReceiverNotProviderStdout or a new TestResealRequiresAuthoredExpectedArtifactChange. It does not rise to a standing material BC5 falsification."
verdict: "needs_revision"
rationale: "This adjudicates the design-v4 REVISION (the FOURTH falsification pass on RFC 0143) against the SEED clearing condition: a clearing verdict requires BOTH remaining binding constraints BC1 (all three v3 grounds) and BC5 (both precision items) genuinely resolved with a concrete mechanism AND the v3-credited resolved set carried forward unregressed AND no new material challenge standing unrebutted AND the security invariant holding STRUCTURALLY (no widening, no replay, no split-brain). v4 is materially stronger than v3 on every axis the SEED required: it adds the three structural channel walls (W1 per-message SCM_CREDENTIALS peer-credentials bound to the launched wrapper pid+start-time, W2 PR_SET_DUMPABLE(0) reinforced by the sudo setuid launch, W3 the nonce delivered over the socketpair and out of the env), commits the wrapper to never propagate provider child statuses 97/98 into the reserved codes (C2), replaces the v3 positive-intent gap with a DAEMON-OBSERVED post-rotation condition (boot-epoch rotated since the packet + stamped-generation match + lease-in-grace + every required expected_artifact present and content-hash-modified since the write_scope_baseline) plus a recovery-sweep backstop, and explicitly bypasses ensureWorkSessionBackend so a stopped supervisor routes the typed class rather than leaking a backend error. On BC5 it pins leases.reseal_grace_extended_at to the same owner bundle 0021 as jobs.recovery_generation (leases is owner-held — created by runtime 0005, absent from the owner/0018 transfer cohort) and corrects the work.complete lock-order story with an exact skip/replace/replay gate map. But the gate does NOT clear. BC5 IS resolved: falsifier-reviewer-002, on the assigned lifecycle lens plus a regression sweep, finds both v3 precision items answered concretely and no standing material lifecycle/correctness gap (its modified-since-baseline pressure is, by its own assessment, a build-test precision item closable by one test, not a standing falsification). BC1 is NOT resolved: falsifier-reviewer-001 lands a material, unrebutted challenge — the W1/W2/W3 walls are correct in shape, but the spec installs them on a DIRECT exec.Cmd.ExtraFiles child exec while the production supervised lane is tmux-backed (launchPTY -> tmux respawn-pane under sudo -n -u RunAsUser -- env -i, wrapped by the env-file shell shim). HelperLaunchSpec/LaunchSpec carry no control-fd field and RunHelper forwards no fd, so the load-bearing migration site (how fd 3 reaches the pane wrapper, before the nonce is live, without a same-uid-readable handoff) is unspecified: passing an fd via ExtraFiles to the tmux client does not reach the tmux-server-spawned pane process; passing it through the env-file shim makes fd 3 live before agentloop can run PR_SET_DUMPABLE(0) (the required ordering breaks); and any env-var/filesystem-socket-name/lane-readable bridge reopens the exact same-uid surface BC1 exists to close. The named tests can pass on a direct os/exec harness while the real tmux lane never receives fd 3 — i.e. the load-bearing test would not actually fire against the production path, a SEED-named needs_revision condition. I credit falsifier-reviewer-001's load-bearing source citations and INDEPENDENTLY confirmed them against the current worktree: HelperLaunchSpec has no control-fd field (helper_protocol.go:27-39); LaunchSpec has no ExtraFiles/control-fd field (pty.go:30-41); the run-as path is sudo -n -u RunAsUser -- env -i with the env-file shim launchEnvFileExec (pty.go:24, :98-112); launchPTY uses tmux respawn-pane (pty.go:479); and there is no socketpair / SCM_CREDENTIALS / SO_PASSCRED / PR_SET_DUMPABLE / ExtraFiles primitive anywhere in go/pkg/supervisor or go/pkg/agentloop today. So the v4 channel is a plausible direct-child design but not yet a buildable Striatum supervisor control channel. Clearing condition walked (all four must hold): (1) BC1 AND BC5 resolved with a concrete mechanism — FAILS (BC5 resolved; BC1 open on the channel-installation ground; C2 and the positive-intent/backend-gate sub-grounds are resolved); (2) v3-credited set carried forward unregressed — HOLDS (both falsifiers confirm BC2, BC3, BC4, F2, F4, the F7 file-mirror half, AF1, AF4, no-widening, and the A1-A18 discipline intact); (3) no new material challenge standing unrebutted — FAILS (falsifier-reviewer-001's tmux-channel challenge lands and is not pre-empted by the spec text; the cycle ends at adjudication and the holder had no further turn); (4) security invariant holds STRUCTURALLY — FAILS (no-replay is promised on a direct-exec harness, not enforced on the production tmux channel; the only bridges that would install fd 3 there reintroduce a same-uid-readable handoff, so structural no-replay is not established — and the prompt forbids treating this as a trackable post-clearance finding). Why not reject: no path widens admin-token exposure and no minted credential carries any of {admin, apply, recovery, surgical_recovery}; both falsifiers explicitly confirm the no-widening invariant holds, falsifier-reviewer-001 recommends needs_revision (not reject), and W1 plus the daemon-observed trigger materially shrink the residual blast radius versus v3. Why not accept_with_findings: BC1 is the security cluster's load-bearing closure for F1/F6 and the F2/F7 channel halves; the SEED demands the security invariant hold STRUCTURALLY, and structural no-replay is not yet established on the production channel — that forecloses a clearing verdict; it is not a post-clearance finding. Verdict: needs_revision. The next (out-of-run) revision must, in ONE place: pin the control-fd delivery + dumpability mechanism through the production tmux / sudo run-as / env-file path (or explicitly change the launch topology), name the exact HelperLaunchSpec/LaunchSpec/RunHelper plumbing sites that reach the pane agentloop wrapper rather than the tmux client, guarantee no same-uid-readable shim process holds fd 3 or the nonce before PR_SET_DUMPABLE(0) is effective, and add a REAL-PATH test (e.g. TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper) that launches through RunHelper with RequireTmux/RunAsUser and asserts, together, that the wrapper can send an accepted frame stamped with the launched wrapper pid+start-time, the provider lacks fd 3, and a non-child/non-wrapper same-uid sibling cannot open /proc/<wrapper-pid>/fd/3 or recover the nonce at any point in the launch chain. Carry forward unregressed: C2, the daemon-observed positive-intent source and the backend-gate bypass, BC2, BC3, BC4, BC5, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, and the A1-A18 assertion discipline; and fold in falsifier-reviewer-002's modified-since-baseline build-test (reuse the gitChangedPathSnapshots/collectInScopeAuthoredPaths authored-path attribution, baseline nil for isolated worktrees, so an unchanged pre-existing expected path is NOT resealed). Maintainer-ratification note (carries regardless of verdict): Slice B (the daemon-internal rpc.CapabilityReseal marker, the test-only auth-prelude route alternate, the inherited-fd supervisor control channel with per-message peer-credential authentication, the reserved agentloop exit codes, the jobs.recovery_generation + leases.reseal_grace_extended_at owner-bundle-0021 columns, and endpoint/epoch republish plumbing) is a security/authz trust-model change requiring maintainer ratification before any build slice touches credential code; adjudicator clearance gates the spec's soundness, not the maintainer's product call. Slice A (the Option-4 typed-exit-code floor) is zero-trust-change but, per the still-open BC1, still must route over a real, non-PTY channel with the same-uid authentication fixed AND ANCHORED THROUGH THE TMUX LAUNCH PATH before it lands."
findings:
  - id: BC1
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2"]
    affected_invariants:
      - "R2 replay / false-provenance (no same-uid-reachable channel a sibling lane can present)"
      - "product-boundary: terminal output is not authoritative workflow state"
      - "R4 legible-failure (the typed floor must fire, never leak a raw backend error)"
    challenge: "Same-uid-authenticated control channel NOT genuinely closed on the PRODUCTION launch topology (one material ground; the other two v3 grounds resolved). The W1 per-message SCM_CREDENTIALS peer-credential wall (bound to the launched wrapper pid+start-time), W2 PR_SET_DUMPABLE(0)-before-fd/nonce, and W3 nonce-over-socketpair-out-of-env are the right walls, but the spec installs them on a DIRECT exec.Cmd.ExtraFiles child exec while the production supervised lane runs inside a tmux pane: launchPTY -> tmux respawn-pane (pty.go:479) under sudo -n -u RunAsUser -- env -i (pty.go:98-112) wrapped by the env-file shell shim launchEnvFileExec (pty.go:24, :282-287). HelperLaunchSpec has no control-fd field (helper_protocol.go:27-39); LaunchSpec has no ExtraFiles/control-fd field (pty.go:30-41); RunHelper forwards no fd (helper.go:149-156); and no socketpair/SCM_CREDENTIALS/SO_PASSCRED/PR_SET_DUMPABLE/ExtraFiles primitive exists in go/pkg/supervisor or go/pkg/agentloop today (independently confirmed). The load-bearing migration site is unspecified: an fd via ExtraFiles to the tmux client does not reach the tmux-server-spawned pane wrapper; routing it through the env-file shim makes fd 3 live before agentloop can run PR_SET_DUMPABLE(0) (breaking the required ordering); and any env-var/filesystem-socket-name/lane-readable bridge reopens the same same-uid surface BC1 must close. The named tests (TestControlFrameRequiresExpectedWrapperPeerCredentials / TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc) can pass on a direct os/exec harness while the real tmux lane never receives fd 3 — the load-bearing test would not actually fire against the production path. RESOLVED sub-grounds carried forward: C2 (the wrapper never propagates a provider child 97/98 into the reserved codes; TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker) and the v3 positive-intent gap (now daemon-observed: post-rotation + stamped-generation match + lease-in-grace + all expected_artifacts present-and-modified-since-baseline, recovery-sweep backstop, ensureWorkSessionBackend bypassed; TestResealExit98BypassesBackendGateOrRoutesTyped, positive TestCodexResealUsesReceiverNotProviderStdout). Fix: pin control-fd delivery + dumpability through the tmux/run-as/env-file path (or change the launch topology) at the exact HelperLaunchSpec/LaunchSpec/RunHelper sites that reach the pane wrapper, guarantee no same-uid-readable shim holds fd 3 or the nonce before PR_SET_DUMPABLE(0) is effective, and add a real-path TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper through RunHelper with RequireTmux/RunAsUser asserting all three facts together. Keeps F1, F6, and the F2/F7 channel halves open; the Slice-A exit-97 floor is plausibly closed once the channel is anchored on the production path."
  - id: BC2
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2"]
    challenge: "RESOLVED (carried forward unregressed; falsifier-reviewer-001 finds no regression). resealInFlightJob derives the expected-artifact set from the job's expected_artifacts (daemon state, attempt-resolved), reuses verifyRequiredArtifacts/ensurePerJobPublishedArtifactsDurable, publishes only a path that is an open expected entry from the job's own worktree, refuses unexpected paths, and routes a front-matter/author-line failure to the Option-4 floor. v4 adds the POSITIVE case (a complete-on-disk deliverable post-rotation IS automatically resealed by the daemon-observed condition). Keep TestCodexResealUsesReceiverNotProviderStdout. The identity-from-state property is sound; its trigger is now the daemon-observed condition (no longer inheriting the open BC1 channel for the positive path)."
  - id: BC3
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2"]
    challenge: "RESOLVED (carried forward unregressed; both falsifiers find no regression). CapabilityReseal stays a daemon-internal marker projected by resealInFlightJob (supervisor_id->session_id, internal AuthContext without the public Authorize prelude), with the public route-alternate kept test-only since no production bearer exists; registry_methods.go generated, command-authority-matrix reseal column + authority guardrail updated. Keep TestResealCapabilityIsDaemonInternalNotBearer / TestResealTokenCanReachOnlyResealRoutesWithoutWrite. The publish/complete reuse path's backend-gate question is now resolved under BC1/BC5 (ensureWorkSessionBackend bypass)."
  - id: BC4
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:3"]
    challenge: "RESOLVED (carried forward unregressed; falsifier-reviewer-002 confirms). Concrete monotonic jobs.recovery_generation column in owner bundle 0021, LatestOwnerBundleVersion 20->21 with the ordinal-21 reservation, modelled on review_generation; degrade-safe JobRecoveryGenerationColumnPresent probe routes to the typed floor when absent; four increment points each in the lease-retiring/rebinding UPDATE under lockRun; post-increment value stamped into work_packets.packet_json lease.recovery_generation and compared equal/unequal at reseal under the lock. Keep TestResealPredicateUsesStampedRecoveryGeneration / TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue."
  - id: BC5
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "R3 split-brain across rotation"
      - "R4 legible-failure (typed class must fire, never a raw lease_error)"
    challenge: "RESOLVED — both v3 precision items fixed (falsifier-reviewer-002, on the assigned lifecycle lens plus a regression sweep, finds no standing material lifecycle gap). (1) Migration site PINNED: leases.reseal_grace_extended_at lands in the same owner bundle 0021 as jobs.recovery_generation; leases is owner-held (created runtime 0005:166-182; absent from the owner/0018 transfer cohort:80-90; owner frontier 20 so 0021 is next), so TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn / TestFutureRuntimeMigrationsDoNotCarryOwnerDDL can actually fire. (2) Lock order CORRECTED: source matches the diagnosis (HandleCompleteWork runs enforceSessionBindingForSession + enforceActiveActingSession before lockRunForJob, activeLeaseFor + ensureWorkSessionBackend after, lifecycle.go:1124-1182; artifact.publish takes lockRunForJob first, artifact.go:75-85; the sweep drains-then-locks-then-expires, recovery.go:575-621), and v4 names the exact skip/replace (session-binding, active-session, activeLeaseFor, ensureWorkSessionBackend) and replay (lockRunForJob, FOR UPDATE rows, reseal predicate, write-scope, durability, transition) map plus the reseal-vs-sweep serialization, so expired-beyond-grace ALWAYS routes the typed session_unrecoverable_across_rotation class. Keep TestResealBeyondGraceRoutesTypedNotLeaseError / TestResealGraceCannotReviveRequeuedLease / TestRecoveryRequeueWinsOverExpiredLeaseReseal / GD-1b / TestResealExit98BypassesBackendGateOrRoutesTyped. NON-MATERIAL build-test watch (folds into the build run, not BC-blocking): the modified-since-packet proof must reuse the existing authored-path attribution (gitChangedPathSnapshots/collectInScopeAuthoredPaths, baseline nil for isolated worktrees; claim.go:601-630; artifact_source_publish.go:69-88, :255-290; write_scope_guard.go:69-85) so a clean pre-existing expected path is NOT resealed — close with TestResealRequiresAuthoredExpectedArtifactChange or the positive TestCodexResealUsesReceiverNotProviderStdout case."
  - id: CF
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2", "dialogue:3"]
    challenge: "CARRIED-FORWARD SET intact (no regression — both falsifiers confirm). F2: no lane-readable reseal bearer reintroduced; the v1 0600 same-uid file replay stays retired (the channel residual is BC1, not a reopening). F4: the public route-alternate stays limited to interrogation.answer/work.complete/artifact.publish, records reseal not write, not the production no-token path. F7 file-mirror half: daemon-owned lane-read-only endpoint/epoch, O_NOFOLLOW, atomic rename, missing-epoch rejected (channel half inherits the open BC1). AF1 reachability-not-reminting (TestTokenValidAcrossRestart) and AF4 epoch/token decoupling: kept. No-admin-token-widening invariant: held + strengthened (CapabilityReseal carries no elevated verb and is never materialized into any lane-readable file; no minted credential carries admin/apply/recovery/surgical_recovery; TestResolveRefusesRuntimeClientTokenForLane). Per-claim falsifiable-assertion discipline: extended to A1-A18 covering the channel walls, reserved-exit commitment, daemon-observed intent, migration sites and lock order. Preserve all verbatim through the next revision."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 design-v4 REVISION (cycle 1)

author: adjudicator-author-001

> Adjudication of the design-v4 REVISION dialogue trajectory for RFC 0143
> (*lane credential survival across a daemon boot-epoch rotation*). This is the
> **fourth** falsification pass on the RFC: v1 (`rfc-0143-design`) returned
> `needs_revision` with seven findings F1–F7; v2 (`rfc-0143-design-v2`) resolved
> **F2** and **F4** and distilled the residue into the five binding constraints
> BC1–BC5; v3 (`rfc-0143-design-v3`) resolved **BC2, BC3, BC4** and carried the
> v2-credited set forward unregressed, but returned `needs_revision` with **BC1
> open on three independent grounds** and **BC5 with two precision items**. This
> v4 run revised the spec to close BC1 and BC5; the two falsifiers re-attacked it.
> Inputs read: the revised Holder spec (`dialogue/holder/HOLDER.md`), both
> falsifier re-attacks (`dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`), the `SEED.md` charter (the ratified design
> shape, the v3-credited carry-forward set, and the BC1/BC5 constraints with their
> exact source anchors), the **v3** `HOLDER.md` (the spec being revised), and the
> **v3** collaboration ledger (the verdict + full BC1/BC5 analysis). No raw
> terminal output was read. Load-bearing source citations were independently
> re-verified against the current worktree.

## Verdict

**verdict: needs_revision**

v4 is **materially stronger than v3** and **resolves BC5 cleanly** while carrying
the entire v3-credited set forward unregressed. It also closes **two of BC1's
three v3 grounds** — the reserved-exit-code commitment (C2) and the
positive-intent source (now daemon-observed, with the `ensureWorkSessionBackend`
backend-gate conflict resolved by an explicit bypass). But it does **not** clear
the gate: **BC1's third ground — installing the same-uid-authenticated control
channel on the real launch path — stays open**, and falsifier-reviewer-001's
re-attack on it **lands unrebutted**. This is a security/authz-hot gate held high;
it is not yet a buildable spec.

**Why not `reject`.** No path widens admin-token exposure, and no minted
credential carries any of `{admin, apply, recovery, surgical_recovery}`. Both
falsifiers confirm the no-widening invariant holds; falsifier-reviewer-001
recommends `needs_revision`, not `reject`; and v4's W1 peer-credential design plus
the daemon-observed trigger materially shrink the residual blast radius vs v3.

**Why not `accept_with_findings`.** BC1 is a **security-cluster** binding
constraint and the SEED's load-bearing closure for F1/F6 and the F2/F7 channel
halves. A clearing verdict requires the security invariant to hold
**structurally** — and **structural no-replay is not yet established on the
production tmux channel**: the W1/W2/W3 walls are specified for a direct
`exec.Cmd.ExtraFiles` child exec, while the supervised lane runs inside a tmux
pane spawned by `tmux respawn-pane` under `sudo … env -i` and an env-file shim,
and the only mechanisms that would bridge fd 3 to that pane either break the
`PR_SET_DUMPABLE(0)`-before-the-fd ordering or reintroduce a same-uid-readable
handoff. That is not a trackable post-clearance finding; it forecloses a clearing
verdict.

### The clearing condition, walked

A clearing verdict requires **all four** to hold; **two fail**:

1. **BC1 and BC5 resolved with a concrete mechanism — FAILS.** BC5 is resolved
   (both precision items fixed). BC1's C2 and positive-intent grounds are
   resolved, but its channel-installation ground stays **open**.
2. **The v3-credited resolved set carried forward unregressed — HOLDS.** BC2, BC3,
   BC4, F2, F4, the F7 file-mirror half, AF1, AF4, the no-admin-token-widening
   invariant, and the A1–A18 discipline are all intact (finding **CF**; both
   falsifiers confirm).
3. **No new material challenge standing unrebutted — FAILS.**
   falsifier-reviewer-001's tmux-channel-installation challenge lands and is not
   pre-empted by the spec text (the cycle ends at adjudication; the Holder had no
   further turn).
4. **The security invariant holds STRUCTURALLY — FAILS.** No-replay is promised on
   a direct-exec harness, not enforced on the production tmux channel.

## Constraint-by-constraint walk (BC1 + BC5 + carry-forward)

| Constraint | Cluster | Disposition | One-line reason |
| --- | --- | --- | --- |
| **BC1** | security | **open** (1 of 3 grounds) | W1/W2/W3 walls are correct but specified for a direct `exec.Cmd.ExtraFiles` child exec; the production tmux/`respawn-pane`/`sudo`/env-file launch has no control-fd plumbing and no anchored delivery site, so the channel is not yet buildable without reopening the same-uid surface. C2 and the daemon-observed positive intent ARE resolved. |
| **BC5** | lifecycle | **resolved** | `leases.reseal_grace_extended_at` pinned to owner bundle 0021 (`leases` owner-held); `work.complete` lock-order corrected with the exact skip/replace/replay gate map and reseal-vs-sweep serialization. One non-material build-test watch (modified-since-baseline). |
| **CF** | carry-forward | **intact** | BC2, BC3, BC4, F2, F4, F7 file-mirror half, AF1, AF4, no-widening, A1–A18 — all unregressed. |

### BC1 — Same-uid-authenticated channel + daemon-observed intent (open on 1 of 3 grounds)

v4 makes **real progress** the gate credits:

- **C2 (reserved exit codes) — RESOLVED.** The wrapper is committed to never
  propagate a provider child's 97/98 into the reserved agentloop codes (remap to a
  non-control `agent_exited`); `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`.
  falsifier-reviewer-001: "I do not see a remaining design-level gap in C2."
- **Positive-intent + backend gate — RESOLVED.** v4 takes SEED option (a):
  `resealInFlightJob` fires only on a **daemon-observed** post-rotation condition
  (boot-epoch rotated since the packet, stamped-generation match, lease-in-grace,
  every required `expected_artifact` present **and content-hash-modified since the
  `write_scope_baseline`**), with a **recovery-sweep backstop** for "the wrapper
  can't even signal," and it **bypasses `ensureWorkSessionBackend`** so a stopped
  supervisor routes the typed class instead of leaking a backend error
  (`TestResealExit98BypassesBackendGateOrRoutesTyped`; positive
  `TestCodexResealUsesReceiverNotProviderStdout`). falsifier-reviewer-001: "I do
  not see the v3 positive-intent gap still standing apart from the channel-launch
  issue."

But the **third ground stays open and lands unrebutted** (falsifier-reviewer-001):
the W1 per-message `SCM_CREDENTIALS` peer-credential wall (bound to the launched
wrapper pid + start-time), the W2 `PR_SET_DUMPABLE(0)`-before-the-fd/nonce wall,
and the W3 nonce-over-socketpair-out-of-env wall are the **right** walls — but the
spec installs them on a **direct `exec.Cmd.ExtraFiles` child exec**, while the
production supervised lane is **tmux-backed**: `launchPTY` runs the lane command via
`tmux respawn-pane` (`pty.go:479`) under `sudo -n -u <RunAsUser> -- env -i`
(`pty.go:98-112`) wrapped by the env-file shell shim `set -a; . "$1"; … exec "$@"`
(`pty.go:24`, `:282-287`). I independently confirmed against the current worktree:
`HelperLaunchSpec` carries **no** control-fd field (`helper_protocol.go:27-39`),
`LaunchSpec` carries **no** `ExtraFiles`/control-fd field (`pty.go:30-41`),
`RunHelper` forwards no fd, and **no** `socketpair`/`SCM_CREDENTIALS`/`SO_PASSCRED`/
`PR_SET_DUMPABLE`/`ExtraFiles` primitive exists in `go/pkg/supervisor` or
`go/pkg/agentloop` today. So the **load-bearing migration site is unspecified**:

- An fd passed via `ExtraFiles` to the `tmux` client does **not** reach the
  tmux-server-spawned pane wrapper.
- Routing fd 3 through the env-file shim makes it **live before** agentloop can run
  `PR_SET_DUMPABLE(0)` — breaking the required "dumpable=0 before the fd/nonce are
  live" ordering.
- Any env-var, filesystem-socket-name, or lane-readable handoff to bridge tmux
  **reopens the exact same-uid surface** BC1 exists to close.
- The named tests can pass on a **direct `os/exec` harness** while the real tmux
  lane never receives fd 3 — i.e. the load-bearing test **would not actually fire**
  against the production path, a SEED-named `needs_revision` condition.

W1 being load-bearing ("a stolen fd is rejected because the kernel stamps the
sibling's pid") is **necessary but not sufficient**: it presumes the channel is
installed so that the wrapper's frames arrive with the wrapper's own credentials —
which is precisely what the unspecified production delivery has not shown without a
same-uid-readable bootstrap gap. **BC1 stays open.**

### BC5 — Pinned migration site + corrected lock order (resolved)

falsifier-reviewer-002, on the assigned lifecycle lens plus a regression sweep,
finds **no independent BC5 falsification** and confirms both v3 precision items are
answered concretely:

1. **Migration site pinned.** `leases.reseal_grace_extended_at` lands in the **same
   owner bundle 0021** as `jobs.recovery_generation`. `striatumd.leases` is
   owner-held (created in runtime `0005:166-182`; **absent** from the owner/0018
   ownership-transfer cohort `:80-90`; owner frontier is 20, so 0021 is the next
   slot), so the v3 owner-bundle-vs-runtime ambiguity is gone and
   `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn` /
   `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` can actually fire.
2. **Lock-order corrected.** Current source matches v4's diagnosis
   (`HandleCompleteWork` runs `enforceSessionBindingForSession` +
   `enforceActiveActingSession` **before** `lockRunForJob`, and `activeLeaseFor` +
   `ensureWorkSessionBackend` **after**, `lifecycle.go:1124-1182`; `artifact.publish`
   takes `lockRunForJob` first, `artifact.go:75-85`; the sweep drains→locks→expires,
   `recovery.go:575-621`). v4 names the exact **skip/replace** set
   (`enforceSessionBindingForSession`, `enforceActiveActingSession`,
   `activeLeaseFor`, `ensureWorkSessionBackend`) and the **replay** set
   (`lockRunForJob`, the `FOR UPDATE` rows, the reseal predicate, write-scope,
   durability, the `running→completed` transition), plus the reseal-vs-sweep
   serialization, so expired-beyond-grace **always** routes the typed
   `session_unrecoverable_across_rotation` class — never a raw `lease_error`, never a
   revived requeued lease.

**Non-material build-test watch (does not block):** the modified-since-packet proof
must not treat "present + absent from `write_scope_baseline.changed_paths`" as
sufficient by itself; the build must reuse the existing authored-path attribution
(`gitChangedPathSnapshots` / `collectInScopeAuthoredPaths`, baseline nil for
isolated worktrees; `claim.go:601-630`, `artifact_source_publish.go:69-88`/`:255-290`,
`write_scope_guard.go:69-85`) so a clean pre-existing expected path is **not**
resealed. Close it with `TestResealRequiresAuthoredExpectedArtifactChange` or the
positive `TestCodexResealUsesReceiverNotProviderStdout` case. **BC5 is resolved.**

### CF — Carry-forward set (intact, unregressed)

Both falsifiers confirm no regression: **F2** (no lane-readable reseal bearer; the
v1 `0600` same-uid file replay stays retired), **F4** (route-alternate limited to
the three routes, records `reseal` not `write`, test-only), **F7 file-mirror half**
(daemon-owned lane-read-only endpoint/epoch, `O_NOFOLLOW`, atomic rename,
missing-epoch rejected — channel half inherits the open BC1), **AF1**/**AF4**, the
**no-admin-token-widening invariant** (held + strengthened — `CapabilityReseal`
carries no elevated verb and is never materialized into any lane-readable file),
and the **per-claim assertion discipline** (now A1–A18, extended to the channel
walls, reserved-exit commitment, daemon-observed intent, migration sites and lock
order). **BC2, BC3, BC4** are likewise carried forward resolved. Preserve all
verbatim through the next revision.

## Falsifier challenge dispositions

- **falsifier-reviewer-001 — BC1 channel installation (material; landed
  unrebutted).** Claim challenged: the W1/W2/W3 same-uid authentication is
  source-anchored for the production supervised lane. Material? **Yes** — it would
  change the spec (pin the control-fd delivery + dumpability through the
  tmux/run-as/env-file path or change the launch topology) and exposes a real
  defect (the channel is not installable on the production path without reopening
  a same-uid surface, and the named tests would not fire there). Rebutted? **No** —
  the spec text asserts `exec.Cmd.ExtraFiles` direct inheritance and does not
  address the tmux indirection; the Holder had no further turn. Disposition: **BC1
  remains open; verdict-driving.**
- **falsifier-reviewer-002 — BC5 modified-since-packet proof (not material).**
  Claim challenged: "present + absent from baseline" alone proves the deliverable
  was authored this attempt. Material? **No** — by the falsifier's own assessment
  it is a build-test precision item, not an unpinned site or an incorrect
  lock-order claim, closable by one test; BC5's load-bearing fixes (migration site,
  lock order) stand. Disposition: **BC5 resolved; recorded as a build-run watch
  item, not BC-blocking.**

## What the next (out-of-run) revision MUST fix to clear on re-attack

One item, within BC1 — the production-channel installation:

1. **Pin the control-fd delivery + dumpability through the real launch path.** Name
   the exact `HelperLaunchSpec` / `LaunchSpec` / `RunHelper` plumbing that gets one
   `socketpair` end to the **pane's agentloop wrapper** (not merely the `tmux`
   client) under `sudo … env -i` + the env-file shim — or explicitly change the
   production launch topology. Guarantee **no same-uid-readable shim process holds
   fd 3 or the nonce before `PR_SET_DUMPABLE(0)` is effective** (resolve the
   ordering: the env-file shim runs before agentloop's first instruction).
2. **Add a real-path test.** `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (or
   equivalent) launched through `RunHelper` with `RequireTmux`/`RunAsUser`,
   asserting **together**: the wrapper can send a frame the helper accepts (stamped
   with the launched wrapper pid + start-time, W1); the provider child lacks fd 3;
   and a non-child/non-wrapper same-uid sibling cannot open `/proc/<wrapper-pid>/fd/3`
   or recover the nonce at **any** point in the launch chain (W2/W3). The
   direct-`os/exec` versions of `TestControlFrameRequiresExpectedWrapperPeerCredentials`
   / `TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` are necessary but
   not sufficient.

Everything else is sound and carries forward unchanged: C2, the daemon-observed
positive-intent source and the `ensureWorkSessionBackend` bypass, BC2, BC3, BC4,
**BC5** (with falsifier-reviewer-002's authored-path build-test folded in), F2, F4,
the F7 file-mirror half, AF1, AF4, the no-admin-token-widening invariant, and the
A1–A18 assertion discipline.

## Note on maintainer ratification (carries forward regardless of verdict)

Even when a future revision clears, the chosen direction — the daemon-internal
`rpc.CapabilityReseal` marker, the test-only auth-prelude route alternate, the
inherited-fd supervisor control channel with per-message peer-credential
authentication, the reserved agentloop exit codes, the `jobs.recovery_generation`
+ `leases.reseal_grace_extended_at` owner-bundle-0021 columns, and the
endpoint/epoch republish plumbing — is a **security/authz trust-model change**
requiring **maintainer ratification** before any build slice touches credential
code. Adjudicator clearance gates the spec's **soundness**; it is **not** the
maintainer's product call on the credential code. Slice A (the Option-4
typed-exit-code floor) is zero-trust-change, but per the still-open BC1 it must
route over a real, non-PTY channel with the same-uid authentication fixed **and
anchored through the tmux launch path** before it lands.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 falsification-gate
design-v4 REVISION run (cycle 1). The ledger verdict — not falsifier completion —
gates the phase: `needs_revision` returns the spec uncleared. v4 resolves BC5 and
two of BC1's three v3 grounds (the reserved-exit commitment and the daemon-observed
positive-intent source with the backend-gate bypass) and carries the v3-credited
set forward unregressed, but BC1's third ground — installing the
same-uid-authenticated control channel on the production tmux/`respawn-pane`/`sudo`/
env-file launch path — stays open, and falsifier-reviewer-001's re-attack on it
lands unrebutted; structural no-replay is therefore not yet established on the real
channel.</sub>
