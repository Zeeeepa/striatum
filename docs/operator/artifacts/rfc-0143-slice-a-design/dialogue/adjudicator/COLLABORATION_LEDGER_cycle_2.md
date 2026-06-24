---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 Slice A — the decoupled session_unrecoverable_across_rotation typed-exit floor (D261 Option 4): falsifiable implementation spec, revision re-attack (cycle 2)"
participants:
  - "holder-author-002"
  - "falsifier-reviewer-003"
  - "falsifier-reviewer-004"
  - "adjudicator-author-002"
cycle: 2
entries:
  - kind: claim
    by: "holder-author-002"
    refs: ["dialogue:1"]
    text: "Slice A closes #512 with a pure daemon-side/process-state floor and ZERO trust-model change: (1) a reserved agentloop exit code ExitUnrecoverableAcrossRotation = 97 + a typed sentinel ErrUnrecoverableAcrossRotation (new go/pkg/agentloop/exitcodes.go; Slice A owns ONLY 97; no reseal-98, resealInFlightJob, connect-out channel, kernel-token capture, CapabilityReseal, or owner bundle 0021); (2) Spot 1 — credential-chain NARROWING via an unexported adminTokenReachedByNonOwner predicate applied AHEAD of the existing read at the runtime client-token tier in BOTH resolvers (ResolveTokenMaterial token.go:31-42, ResolveTokenMaterialFresh endpoint.go:125-136), returning ('', ErrUnrecoverableAcrossRotation) BEFORE any read for a non-owner lane (owner unaffected); callers map ONLY the sentinel to exit 97 (Run/RunContext -> main.go:109-117); the #323 rotation watcher (loop.go:602) requests the unrecoverable exit only when ResolveTokenMaterialFresh returns the sentinel AND cfg.Token.Source is the runtime client-token, else keeps the launch-token fallback; (3) Spot 2 — the daemon observes 97 from durable state on two paths (direct agent_exited.exit_code helper.go:433 -> supervision.go:425; tmux #{pane_dead_status} via an additive PaneDeadStatus on ProbeTmuxLiveness tmux_liveness.go:228/:257) and interposes an exact-code-gated stallClassSessionUnrecoverableAcrossRotation FIRST, ahead of deadAgentExitedUnsealed/agent_pid_dead in recoverStuckJobs; (4) §3.5 dissolves the v7 BC1-W1-CAPTURE-FLOOR raw-leak structurally (no W1 capture boundary at launch; 97 is produced only after agent_started, so a genuine launch failure stays a raw helper_error); (5) §3.6 the typed class is a strict refinement of agent_exited_unsealed, routing the same finalize-from-durable-artifact path, not duplicating/overriding HandleRecoveryCompleteStalled (#292); (6) §4 C2: only ErrUnrecoverableAcrossRotation maps to 97; a provider child's 97/98 is normalized to a generic error by normalizeAgentExitError (loop.go:365-379). Assertions A1-A6 each paired with a named test."
  - kind: challenge
    by: "falsifier-reviewer-003"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "DECOUPLING / DAEMON-SIDE lens (re-attack). Credits the decoupling (no W1/kernel-capture/CapabilityReseal/reseal-file/resealInFlightJob/owner-bundle-0021/code-98), the no-widening Spot 1, and the exact-code-only Spot 2 as the right no-over-fire shape; finds no Slice-B dependency (so needs_revision, not reject). The verdict-driving gap is UNCHANGED from cycle 1 and now stands a second time, source-reconfirmed: the ONLY concrete producer of 97 is the lane-side resolver sentinel, and the normal post-boot-epoch rotation path neither reaches it nor is allowed to emit it. (1) A normal supervised lane launches with its session-bound token as STRIATUM_MCP_TOKEN (supervisedEnvEntries supervision_env.go:341-343), so ResolveTokenMaterial returns Source: EnvMCPToken at step 1 (token.go:19-21) and Run stores it in cfg.Token; the Holder's rotation guard cfg.Token.Source == <runtime client-token> is therefore FALSE for the ordinary lane, so the wrapper emits no 97 even if ResolveTokenMaterialFresh later reaches the sentinel. (2) Earlier under-fire: the watcher detects rotation only via ResolveMCPEndpointFresh (loop.go:589-604); the daemon publishes fresh endpoint/epoch as owner-only files, so a striatum-lane non-owner gets EACCES/EPERM and the watcher treats it as 'nothing to compare against yet' and continues, never asking the token resolver. (3) The 'session bearer still works' premise is FALSE after #316: the env injects STRIATUM_MCP_BOOT_EPOCH (supervision_env.go:344-354), echoed as X-Striatum-Boot-Epoch (mcpconfig.go), and the MCP HTTP handler rejects a stale presented epoch as stale_daemon_identity before dispatch (http.go:681-699); codex cannot reload its launch-time -c MCP URL and the loop only writes an in-PTY wedge prompt then returns nil (loop.go:625-646). So a lane holding a valid bearer can be on a dead endpoint / present a stale epoch the new daemon refuses, cannot complete through MCP, yet emits no 97; Spot 2's exact-code rule then correctly records an ordinary class. This is an UNDER-FIRE on the exact #512 failure Slice A exists to make legible — not a Slice-B dependency, not a widening request. Required revision (any one, daemon-side/non-widening/Slice-A-only): (a) lane-readable non-secret endpoint/epoch freshness -> 97 on a proven stale launch epoch/dead endpoint; (b) map the daemon stale_daemon_identity response observed on the lane's own MCP client path to ErrUnrecoverableAcrossRotation -> 97, with a negative that ordinary non-epoch MCP/network errors do not fire; or (c) a proven in-place endpoint+epoch reconnect, reserving 97 for the rest. The codex wedge must route the typed floor. Tests: TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor, TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane / TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor, TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable; keep TestOrdinaryUnsealedExitStaysAgentExitedUnsealed + TestLaunchHandshakeFailureStaysHelperErrorNotFloor."
  - kind: challenge
    by: "falsifier-reviewer-004"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "SECURITY / LEGIBILITY / REGRESSION lens (re-attack). Confirms NO reject-class widening (runtime client-token kept owner-only; ReadTokenFile's mode&0077 guard token.go:75-92 preserved; refusal before any non-owner read; no minted credential carrying {admin, apply, recovery, surgical_recovery}) and NO Slice-B artifact. NEW material challenge (distinct from cycle 1's under-fire): the Holder's C2 forge-resistance argument is too narrow for the tmux-backed path it relies on for production lanes. A5's named test (TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker) proves only that a provider child exiting 97 is normalized by normalizeAgentExitError into a generic wrapper failure — i.e. it covers the DIRECT child-exit propagation path. It does NOT prove the provider child cannot drive the daemon-observed tmux #{pane_dead_status} to 97. Source: the production tmux control surface runs as the SAME lane uid (sudo -n -u <runAsUser>, pty.go:98-112; tmuxRunnerForSpec pty.go:310-314), the session name is deterministic from runID/laneID/supervisorID (pty.go:620-633, all present in the supervised env), the real lane command is launched by tmux respawn-pane -k (pty.go:479-484), and remain-on-exit is set before the real command runs (pty.go:459) so a dead pane stays queryable. A same-uid provider child can therefore run `tmux respawn-pane -k -t <current pane> -- sh -c 'exit 97'`: the pane process is replaced by a short-lived process that exits 97, #{pane_dead_status} reads 97, and deadAgentUnrecoverableAcrossRotation classifies session_unrecoverable_across_rotation even though the credential-chain sentinel never fired and the wrapper never returned ErrUnrecoverableAcrossRotation. This is BOTH a C2 forge of the reserved floor on the tmux carrier AND an A3 over-fire: an arbitrary provider process can manufacture the special rotation class on a healthy or ordinary-unsealed lane. The 'a child can already kill its own lane' rebuttal does not clear it: C2 is an explicit clearing assertion that the provider child cannot DRIVE the reserved floor code, and the typed class carries operational meaning (auto-finalize-from-durable-artifact, else a distinct 'operator requeue required' remediation), so a forged class makes the daemon's diagnostic false. This is the same shared-uid mutable-tmux oracle the v7 BC1-W1-ORACLE established; Slice A deletes the W1 channel but still trusts that oracle as the tmux carrier of the reserved floor. Required revision: add a real tmux-path negative (TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation) asserting recovery does NOT record the typed class when a same-uid process respawns the pane to exit 97; and EITHER observe a wrapper-owned exit status the provider child cannot set on the tmux path, OR make the tmux control surface inaccessible to the provider child before trusting #{pane_dead_status} — and if that requires the per-lane security principal from RFC 0168, say so and do not claim Slice A's tmux Spot 2 is forge-resistant yet. Keep TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker but treat it as insufficient by itself."
verdict: "needs_revision"
rationale: "This adjudicates the REVISION re-attack (cycle 2) of the fresh Slice-A-only design run for RFC 0143; RFC 0143 is decided by D261 (Slice A ships now as PURE daemon-side observability — the Option-4 typed session_unrecoverable_across_rotation exit floor that mints no credential, widens no token, and touches no trust model; Slice B, the CapabilityReseal authority + W1 connect-out channel, is OUT OF SCOPE, blocked on RFC 0168 / #585). I judge the Slice-A implementation shape against the SEED clearing condition, not the split. Inputs read: the Holder spec (dialogue/holder/HOLDER.md), both cycle-2 falsifier re-attacks (dialogue/falsifier_1 = falsifier-reviewer-003, dialogue/falsifier_2 = falsifier-reviewer-004), the SEED.md charter (design shape, decoupling premise, six HARD CONSTRAINTS, assertions A1-A6, clearing condition), the committed RFC ## Decision (D261), and the v7 BC1-W1-CAPTURE-FLOOR finding (cycle_1 of the v7 ledger). No raw terminal output. PROCEDURAL FACT (load-bearing for this cycle): the Holder spec was NOT revised between cycle 1 and cycle 2 — HOLDER.md is byte-for-byte identical (git diff 06aa6eda..5a45e58d touches only the two FALSIFIER.md files). Only the falsifiers re-ran. So the cycle-1 required fix was never applied, and SA-ROTATION-UNDERFIRE is still mechanically present. I re-verified the gap against current source rather than restating cycle 1. TWO material challenges now stand unrebutted: (1) SA-ROTATION-UNDERFIRE (carried, falsifier-reviewer-003): the only producer of 97 is the lane-side resolver sentinel, and the normal session-bound rotation path neither reaches nor is allowed to emit it. Re-confirmed against source: supervisedEnvEntries injects STRIATUM_MCP_TOKEN as the session-bound bearer (supervision_env.go:341-343); ResolveTokenMaterial returns Source: EnvMCPToken at step 1 (token.go:19-21), so the Holder's §2.3 rotation guard cfg.Token.Source == <runtime client-token> is FALSE for the ordinary lane; the watcher keeps cfg.Token on any fresh-resolve error (loop.go:599-604) and silently continues when the owner-only fresh endpoint/epoch files are unreadable (loop.go:589-593); after #316 a still-valid bearer is not the whole identity (STRIATUM_MCP_BOOT_EPOCH supervision_env.go:344-354 echoed as X-Striatum-Boot-Epoch; stale epoch rejected as stale_daemon_identity before dispatch, http.go:681-699; codex cannot reload its -c URL and the wedge path only writes an in-PTY prompt then returns nil, loop.go:625-646). So the #512 lock-out dies with NO 97 and Spot 2's exact-code rule correctly records an ordinary class — the typed floor never becomes durable daemon state for the very case Slice A exists to make legible (a silent unsealed exit / raw stale-MCP explanation the SEED forbids). (2) SA-C2-TMUX-FORGE (NEW, falsifier-reviewer-004): the Spot-2 tmux carrier #{pane_dead_status} (§3.1) is forgeable by a same-uid provider child. The production tmux surface runs as the shared striatum-lane uid (pty.go:98-112/:310-314), the session name is deterministic (pty.go:620-633), the lane command is launched by respawn-pane -k (pty.go:479-484), and remain-on-exit is set before it runs (pty.go:459), so a same-uid child can `tmux respawn-pane -k -t <pane> -- sh -c 'exit 97'`, make #{pane_dead_status}==97, and drive deadAgentUnrecoverableAcrossRotation to record the typed class though the credential sentinel never fired. The Holder's A5 named test covers only the direct child-exit path through normalizeAgentExitError, not the tmux carrier it relies on for production lanes. This breaks two explicit clearing assertions: A5/C2 forge-resistance (incomplete for the actual carrier) and A3 no-over-fire (an arbitrary provider can manufacture the class on a healthy/ordinary lane). It is the same shared-uid mutable-tmux oracle the v7 BC1-W1-ORACLE established — Slice A deleted the W1 channel but still trusts that oracle as the floor's tmux carrier. CLEARING CONDITION, WALKED (all five required): (1) both spots concretely specified + decoupled — FAILS AS WIRED: Spot 1's anchors/decoupling are sound but its trigger is unreachable on the central rotation path, and Spot 2's tmux carrier is forgeable, so the spec does not deliver a trustworthy daemon-observable floor on the #512 path. (2) No HARD CONSTRAINT violated — FAILS on no-raw-error/no-silent-exit (the rotation lock-out leaks an ordinary class — SA-ROTATION-UNDERFIRE) AND on no-over-fire (the tmux forge — SA-C2-TMUX-FORGE); no-widening, no-Slice-B, additive, daemon-side-only all HOLD. (3) Every A1-A6 stated + named test — FAILS: A5's named test is insufficient for the tmux carrier, A3 is breached on the tmux path, and no assertion/test covers the rotation game-day shape. (4) Relationship to agent_exited_unsealed + HandleRecoveryCompleteStalled (#292) — HOLDS (§3.6). (5) No new material challenge standing unrebutted — FAILS: two stand (one carried, one new). WHY NOT reject: no path widens who can read the admin runtime client-token, no minted credential carries any of {admin, apply, recovery, surgical_recovery}, and no Slice-B artifact is smuggled in; both falsifiers explicitly confirm no widening and recommend needs_revision. WHY NOT accept / accept_with_findings: SA-ROTATION-UNDERFIRE is the central deliverable (the floor under-fires on its target #512 path), and SA-C2-TMUX-FORGE breaks two explicit clearing assertions (A3, A5/C2) on the carrier the spec uses for production lanes — neither is trackable post-clearance residue. VERDICT: needs_revision. GATE STATUS: this is the SINGLE allowed revision cycle, and it was consumed without a Holder spec change. A second needs_revision ends the gate UNCLEARED — so the Slice-A design gate does not clear in this run. Recommended operator next step (out of band, since this run's revision budget is exhausted): re-open a fresh Slice-A design run that (A) adds a reachable, non-over-firing, Slice-A-only rotation-path producer of 97 for a normal session-bound lane (the stale_daemon_identity mapping observed on the lane's own MCP client path is the most directly reachable and decoupling-safe shape) with the rotation game-day tests, AND (B) repairs C2 for the tmux carrier — either by observing a wrapper-owned exit status a same-uid child cannot forge on the tmux path, or by honestly scoping the tmux #{pane_dead_status} carrier's forge-resistance as RFC-0168-bounded (the same per-lane-uid boundary that makes Slice B unsolvable) and relying on the forge-resistant direct-path agent_exited.exit_code as the trustworthy carrier — with the tmux-path negative TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation. Everything else in the spec (the reserved code + sentinel §1; the Spot-1 narrowing refuse-before-read §2 / A1 / A4; the exact-code-only classification logic §3.2-3.4 / A2; the §3.5 launch-handshake dissolution; the §3.6 #292 relationship; the additive isNecrosisStallClass growth + the single disclosed TestNecrosisDomainMatchesConfirmedDeadConstants change; the no-widening invariant) is credited and should be carried forward unchanged."
findings:
  - id: SA-ROTATION-UNDERFIRE
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2", "dialogue:3"]
    affected_invariants:
      - "R4 legible-failure: the typed session_unrecoverable_across_rotation floor must FIRE on the normal post-boot-epoch rotation lock-out, not only a startup-time resolver refusal a session-bound lane never reaches"
    challenge: "OPEN — verdict-driving; CARRIED from cycle 1 and unaddressed (the Holder spec was not revised, so the wiring is unchanged; falsifier-reviewer-003 re-confirms). The only causal trigger for exit 97 is the lane-side resolver sentinel; the normal post-rotation supervised-lane path neither reaches it nor is allowed to emit it. Re-verified against current source: (1) a normal lane launches with its session-bound token as STRIATUM_MCP_TOKEN (supervision_env.go:341-343), so ResolveTokenMaterial returns the FIRST source EnvMCPToken (token.go:19-21) and never reaches the step-3 runtime client-token; (2) the Holder's §2.3 #323 rotation-watcher guard cfg.Token.Source == <runtime client-token> is therefore FALSE for the ordinary lane, so 97 is suppressed even if ResolveTokenMaterialFresh reaches the sentinel (the watcher keeps cfg.Token on any fresh-resolve error, loop.go:599-604); (3) the watcher silently continues when the owner-only fresh endpoint/epoch files are unreadable (loop.go:589-593) without asking the token resolver. The 'session bearer still works' premise is FALSE after #316: STRIATUM_MCP_BOOT_EPOCH (supervision_env.go:344-354) is echoed as X-Striatum-Boot-Epoch and a stale epoch is rejected as stale_daemon_identity before dispatch (http.go:681-699); codex cannot reload its -c URL and the wedge path only writes an in-PTY prompt then returns nil (loop.go:625-646). So a lane holding a valid bearer can be on a dead endpoint / present a stale epoch, cannot complete through MCP, and dies with NO 97 — recovery records an ordinary class, exactly the silent unsealed exit the SEED forbids. Not a Slice-B dependency, not a widening request: a Slice-A wiring miss on the #512 path itself. FIX: add a reachable, non-over-firing rotation-path producer of 97 for session-bound lanes — (a) lane-readable non-secret endpoint/epoch freshness -> 97 on a proven stale launch epoch/dead endpoint; or (b) map the daemon stale_daemon_identity response observed on the lane's own MCP client path to 97 (decoupling-safe; widens no token; no Slice-B artifact) with a negative that ordinary non-epoch MCP/network errors do not fire; or (c) a proven in-place endpoint+epoch reconnect per adapter, reserving 97 for the rest; route the codex wedge to the typed floor. Tests: TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor, TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane / TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor, TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable; KEEP TestOrdinaryUnsealedExitStaysAgentExitedUnsealed + TestLaunchHandshakeFailureStaysHelperErrorNotFloor."
  - id: SA-C2-TMUX-FORGE
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "C2 forge-resistance (A5): a provider child cannot drive the reserved floor code — must hold on the TMUX #{pane_dead_status} carrier, not only the direct child-exit path"
      - "HARD CONSTRAINT 4 no-over-fire (A3): a healthy / ordinary-unsealed lane must NOT be misclassified as the typed floor"
    challenge: "OPEN — NEW (falsifier-reviewer-004; security/legibility lens); material; landed unrebutted. The Spot-2 tmux carrier #{pane_dead_status} (§3.1) is forgeable by a same-uid provider child. The production tmux surface runs as the shared striatum-lane uid (pty.go:98-112/:310-314), the session name is deterministic from runID/laneID/supervisorID present in the supervised env (pty.go:620-633), the lane command is launched by tmux respawn-pane -k (pty.go:479-484), and remain-on-exit is set before it runs (pty.go:459). A same-uid child can run `tmux respawn-pane -k -t <pane> -- sh -c 'exit 97'`: the pane process is replaced by one that exits 97, #{pane_dead_status} reads 97, and deadAgentUnrecoverableAcrossRotation records session_unrecoverable_across_rotation though the credential sentinel never fired and the wrapper never returned ErrUnrecoverableAcrossRotation. This is BOTH a C2 forge on the tmux carrier and an A3 over-fire (an arbitrary provider manufactures the class on a healthy/ordinary lane). The Holder's A5 named test (TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker) covers only the DIRECT child-exit path through normalizeAgentExitError, not the tmux carrier the spec relies on for production lanes; the 'a child can already kill its lane' rebuttal fails because the typed class carries operational meaning (auto-finalize / distinct remediation) and a forged class makes the diagnostic false. This is the same shared-uid mutable-tmux oracle the v7 BC1-W1-ORACLE established. FIX: add a tmux-path negative TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation (assert recovery does NOT record the typed class when a same-uid process respawns the pane to exit 97); and EITHER observe a wrapper-owned exit status a same-uid child cannot set on the tmux path (e.g. corroborate #{pane_dead_status}==97 against the wrapper's own durable agent_exited.exit_code rather than trusting the pane status alone), OR honestly scope the tmux carrier's forge-resistance as RFC-0168-bounded (the per-lane-uid boundary that makes Slice B unsolvable) and rely on the forge-resistant direct-path agent_exited.exit_code as the trustworthy carrier — and stop claiming the tmux Spot 2 is forge-resistant until then. Keep TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker but treat it as insufficient by itself."
  - id: SA-SPOT1-NARROWING
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 1 no-widening: Spot 1 narrows (refuses a step), never adds a read path"
    challenge: "RESOLVED AS FRAMED (both falsifiers confirm no widening; resolver order re-verified token.go:18-53). adminTokenReachedByNonOwner is applied AHEAD of the existing read at the runtime client-token tier in both resolvers and returns ('', ErrUnrecoverableAcrossRotation) BEFORE any os.ReadFile for a non-owner lane; ReadTokenFile's owner-mode guard (token.go:82) is retained; owner unaffected. A narrowing, not a widening. NOTE: the sentinel it produces is the trigger that does NOT reach the normal rotation lock-out (SA-ROTATION-UNDERFIRE)."
  - id: SA-SPOT2-EXACTCODE
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 4 no-over-fire (attribution logic): the typed class fires iff the observed exit code == 97"
    challenge: "RESOLVED AS THE ATTRIBUTION RULE (both falsifiers credit the exact-code-only direction; it correctly refuses to infer the floor from complete-on-disk + lane-lost alone). The interposition order (stallClassSessionUnrecoverableAcrossRotation gated exact-code FIRST, ahead of deadAgentExitedUnsealed/agent_pid_dead) is correct and must be retained. CAVEAT (new this cycle): the rule is only as sound as its CARRIER. The direct-path carrier (agent_exited.exit_code from the wrapper's own Cmd.Wait) is forge-resistant, but the TMUX carrier (#{pane_dead_status}) is forgeable by a same-uid provider child (SA-C2-TMUX-FORGE) — so the exact-code rule, while correct, is undermined on the tmux path by an untrustworthy input. Upstream the rule also never fires on the normal rotation path because no 97 is produced (SA-ROTATION-UNDERFIRE)."
  - id: SA-LAUNCH-FLOOR-DISSOLVED
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 4 no-raw-error-leak on the launch-handshake path (the v7 BC1-W1-CAPTURE-FLOOR concern)"
    challenge: "RESOLVED / DISSOLVED FOR THE DECOUPLED WORLD (both falsifiers credit it as scoped). Slice A has no W1 capture boundary and no kernel-token capture at launch; the reserved code is produced only AFTER RunHelper emits agent_started (helper.go:186-193), and waitForHelperAgentStart returns on the first agent_started which precedes any agent_exited. So the launch/attach helper_error phase launch path requires no change and correctly stays a RAW helper_error for genuine launch failures (not the floor) — the raw-error-leak path is structurally absent on the launch handshake. NOTE: this dissolves the LAUNCH-PATH raw-leak; the separate legibility goal (route the typed floor on the post-rotation lock-out) is unmet — see SA-ROTATION-UNDERFIRE."
  - id: SA-NO-WIDENING
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 1 (no token widening) + HARD CONSTRAINT 2 (no new credential / no Slice B)"
    challenge: "RESOLVED (both falsifiers confirm; the reason this is needs_revision, not reject). No path widens who can read the admin runtime client-token; ReadTokenFile's owner-mode guard is preserved (token.go:75-92) and not relaxed to group-read; the refusal precedes any non-owner read; Slice A introduces only an exit code, a recovery class, and an additive tmux probe field — no minted credential carries {admin, apply, recovery, surgical_recovery}, and no Slice-B artifact (CapabilityReseal, connect-out channel, kernel-token capture, reseal-token file, reseal-98, owner bundle 0021) is introduced. Any rotation-path trigger or tmux-carrier repair added for the two OPEN findings MUST preserve this invariant."
  - id: SA-C2-DIRECT-PATH
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:3"]
    affected_invariants:
      - "C2 forge-resistance on the DIRECT child-exit path"
    challenge: "RESOLVED for the direct path (falsifier-reviewer-004 credits it). normalizeAgentExitError (loop.go:365-379) wraps the inner provider child's exit as a generic 'agent command exited' error and does not propagate its numeric code; the reserved 97 is emitted only by main.go:109-117 and only for errors.Is(err, ErrUnrecoverableAcrossRotation). A provider child that exits 97/98 produces a generic error -> exit 1, never 97 on the direct path. Test: TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker (A5). RETAIN — but it is INSUFFICIENT by itself: it does not cover the tmux #{pane_dead_status} carrier, which IS forgeable (SA-C2-TMUX-FORGE)."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 Slice A design run (cycle 2 / revision re-attack)

author: adjudicator-author-002

> Adjudication of the **revision re-attack (cycle 2)** of the fresh Slice-A-only
> design-run dialogue for RFC 0143 (*lane credential survival across a daemon
> boot-epoch rotation*). RFC 0143 is decided by **D261**: **Slice A** ships now as
> **pure daemon-side observability** — the Option-4 typed
> `session_unrecoverable_across_rotation` exit floor, which **mints no credential,
> widens no token, and touches no trust model**; **Slice B** (the `CapabilityReseal`
> authority + the W1 connect-out channel) is **OUT OF SCOPE**, blocked on **RFC 0168
> (#585)**. I judge the Slice-A implementation shape against the SEED clearing
> condition, **not** the split or the per-lane-uid direction. Inputs read: the Holder
> spec (`dialogue/holder/HOLDER.md`), both **cycle-2** falsifier re-attacks
> (`dialogue/falsifier_1` = `falsifier-reviewer-003`, `dialogue/falsifier_2` =
> `falsifier-reviewer-004`), the `SEED.md` charter (design shape, the **decoupling
> premise**, the six HARD CONSTRAINTS, assertions A1–A6, the clearing condition), the
> committed RFC `## Decision (D261)`, and the v7 `BC1-W1-CAPTURE-FLOOR` finding. No
> raw terminal output was read. Load-bearing source citations were **independently
> re-verified against the current source on the run branch**.

## Verdict

**verdict: needs_revision**

The Slice-A spec remains the right shape on most axes — it deletes all of Slice B,
**narrows** the credential chain rather than widening it, gives Spot 2 the correct
**exact-code-only** attribution rule, and dissolves the v7 launch-handshake raw-leak
(§3.5). But the gate does **not** clear, and **two** material, source-confirmed
challenges now stand unrebutted.

## Procedural reality of this cycle (decisive)

**The Holder spec was not revised between cycle 1 and cycle 2.** `HOLDER.md` is
byte-for-byte identical to the spec that received `needs_revision` in cycle 1
(`git diff 06aa6eda..5a45e58d` touches only the two `FALSIFIER.md` files). Only the
falsifiers re-ran. The cycle-1 required fix — a reachable, non-over-firing
rotation-path producer of `97` plus the rotation game-day tests — was therefore
**never applied**, so `SA-ROTATION-UNDERFIRE` is mechanically still open. I did not
rely on the cycle-1 record; I re-verified every load-bearing anchor against current
source.

## The two standing challenges

1. **`SA-ROTATION-UNDERFIRE` (carried from cycle 1; falsifier-reviewer-003;
   decoupling lens).** The floor's only producer is the lane-side resolver sentinel,
   and the **normal session-bound rotation path neither reaches it nor is allowed to
   emit it**. Re-confirmed: the supervised lane launches with `STRIATUM_MCP_TOKEN`
   (`supervision_env.go:341-343`), so `ResolveTokenMaterial` returns `Source:
   EnvMCPToken` (`token.go:19-21`) and the Holder's §2.3 guard
   `cfg.Token.Source == <runtime client-token>` is **false**; the watcher keeps the
   launch token on any fresh-resolve error (`loop.go:599-604`) and silently continues
   when the owner-only fresh endpoint/epoch files are unreadable (`loop.go:589-593`);
   after **#316** a still-valid bearer is not the whole identity (stale
   `X-Striatum-Boot-Epoch` is rejected as `stale_daemon_identity`,
   `http.go:681-699`; codex cannot reload its `-c` URL and the wedge path
   `return nil`s, `loop.go:625-646`). The `#512` lock-out dies with **no `97`** and is
   recorded as an ordinary class — the silent unsealed exit the SEED forbids.

2. **`SA-C2-TMUX-FORGE` (NEW; falsifier-reviewer-004; security/legibility lens).**
   The Spot-2 **tmux carrier** `#{pane_dead_status}` (§3.1) is **forgeable by a
   same-uid provider child**. The production tmux surface runs as the shared
   `striatum-lane` uid (`pty.go:98-112`/`:310-314`), the session name is deterministic
   (`pty.go:620-633`), the lane command is launched by `respawn-pane -k`
   (`pty.go:479-484`), and `remain-on-exit` is set before it runs (`pty.go:459`). A
   same-uid child can `tmux respawn-pane -k -t <pane> -- sh -c 'exit 97'`, drive
   `#{pane_dead_status}==97`, and make `deadAgentUnrecoverableAcrossRotation` record
   the typed class though the credential sentinel never fired. This is both a **C2
   forge** on the tmux carrier and an **A3 over-fire**. The Holder's A5 test covers
   only the **direct** child-exit path, not the tmux carrier it relies on for
   production lanes — the same shared-uid mutable-tmux oracle the v7 `BC1-W1-ORACLE`
   established.

**Why not `reject`.** No path widens who can read the admin runtime `client-token`;
no minted credential carries any of `{admin, apply, recovery, surgical_recovery}`; no
Slice-B artifact is smuggled in. Both falsifiers confirm the no-widening invariant.

**Why not `accept` / `accept_with_findings`.** `SA-ROTATION-UNDERFIRE` is the
**central deliverable** (the floor under-fires on its target `#512` path), and
`SA-C2-TMUX-FORGE` breaks two **explicit clearing assertions** (A3, A5/C2) on the
carrier the spec uses for production lanes. Neither is trackable post-clearance
residue.

## The clearing condition, walked

A clearing verdict requires **all five**:

1. **Both spots concretely specified + decoupled — FAILS as wired.** Spot 1's
   anchors/decoupling are sound, but its trigger is unreachable on the central
   rotation path; Spot 2's tmux carrier is forgeable. The spec does not yield a
   trustworthy daemon-observable floor on the `#512` path.
2. **No HARD CONSTRAINT violated — FAILS** on no-raw-error/no-silent-exit
   (`SA-ROTATION-UNDERFIRE`) **and** on no-over-fire (`SA-C2-TMUX-FORGE`).
   No-widening, no-Slice-B, additive, daemon-side-only all hold.
3. **A1–A6 stated + named tests — FAILS.** A5's named test is insufficient for the
   tmux carrier; A3 is breached on the tmux path; no assertion/test covers the
   rotation game-day shape.
4. **Relationship to `agent_exited_unsealed` + `HandleRecoveryCompleteStalled`
   (#292) — HOLDS** (§3.6).
5. **No new material challenge standing unrebutted — FAILS.** Two stand.

## Per-HARD-CONSTRAINT disposition (RESOLVED / INTACT / OPEN)

| HARD CONSTRAINT | Disposition |
|---|---|
| 1 — No token widening | **INTACT.** Spot 1 narrows; refuse-before-read; owner-mode guard retained; no lane reads the admin token; no elevated minted credential. |
| 2 — No new credential / no Slice B | **INTACT.** Only an exit code + recovery class + additive tmux field; no `CapabilityReseal`/channel/kernel-capture/reseal-file/reseal-98/owner-bundle-0021. |
| 3 — Daemon-side / process state only | **INTACT** for the predicates as specified (euid-vs-owner; `agent_exited.exit_code`; `#{pane_dead_status}`; existing artifact/reconstructability/liveness). The decoupling premise is honored in the wiring; the tmux-carrier *provenance* concern is recorded under no-over-fire, not here. |
| 4 — No over-fire | **OPEN.** The exact-code attribution logic is correct, but the tmux `#{pane_dead_status}` carrier is forgeable by a same-uid provider child → over-fire (`SA-C2-TMUX-FORGE`). |
| 4 — No raw-error leak / no silent exit | **OPEN.** The launch-handshake leak is dissolved (§3.5), but the normal rotation lock-out leaks a **silent unsealed exit / ordinary class** (`SA-ROTATION-UNDERFIRE`). |
| 5 — Additive-only | **INTACT.** New file/branch/class/field; the single disclosed `TestNecrosisDomainMatchesConfirmedDeadConstants` growth is additive. |
| 6 — Product-boundary-safe | **INTACT.** No hosted service, no durable transcript, no external persistence. |

## Per-assertion (A1–A6) disposition (RESOLVED / INTACT / OPEN)

| Assertion | Disposition |
|---|---|
| A1 (Spot 1 narrowing + owner-unaffected) | **RESOLVED** as a shape (tests named). But the sentinel it produces does not reach the central rotation lock-out — coverage gap (`SA-ROTATION-UNDERFIRE`). |
| A2 (reserved code → typed class, both paths) + A2-neg | **RESOLVED** for the classification logic; depends on a `97` being produced (under-fire) **and** on a trustworthy carrier — the tmux observation path is compromised by the forge (`SA-C2-TMUX-FORGE`). |
| A3 (no over-fire) | **OPEN.** The ordinary-unsealed negative holds, but the tmux forge is a genuine over-fire path (`SA-C2-TMUX-FORGE`). |
| A4 (no widening) | **INTACT.** `TestLaneNeverReadsAdminRuntimeToken` + the A1 owner/non-owner split. |
| A5 (C2 forge-resistance) | **OPEN.** The named test covers only the **direct** child-exit path; the tmux `#{pane_dead_status}` carrier is forgeable and untested (`SA-C2-TMUX-FORGE`). The direct path itself is resolved (`SA-C2-DIRECT-PATH`). |
| A6 (no regression; additive) | **INTACT.** Additive growth disclosed; no existing meaning changed. |
| **A-coverage (rotation game-day shape)** | **OPEN.** No assertion/test proves a session-token lane with stale endpoint/boot-epoch state exits `97` / records the typed class. |

## Falsifier challenge dispositions

- **falsifier-reviewer-003 — rotation path under-fires before it can emit `97`
  (decoupling lens; material; landed unrebutted).** Material? **Yes** — the only floor
  trigger is gated away (`cfg.Token.Source == EnvMCPToken`) or unreachable
  (owner-only fresh files → silent continue) for the normal lane. Rebutted? **No** —
  the spec is unchanged and the bearer-still-works premise is false after #316.
  Disposition: **`SA-ROTATION-UNDERFIRE` OPEN; verdict-driving.**
- **falsifier-reviewer-004 — tmux `#{pane_dead_status}` lets a same-uid provider
  child forge `97` (security/legibility lens; material; landed unrebutted).**
  Material? **Yes** — it breaks the explicit C2 (A5) and no-over-fire (A3) clearing
  assertions on the carrier the spec relies on for production lanes. Rebutted? **No**
  — the spec's A5 reasoning is confined to the direct path. Security/regression sweep:
  **no** widening, **no** elevated credential, **no** Slice-B artifact. Disposition:
  **`SA-C2-TMUX-FORGE` OPEN; verdict-reinforcing; no-widening / no-Slice-B / additive
  intact.**

Both falsifiers again **credit the Slice-A shape** and confine their objections to
genuine, source-confirmed wiring gaps — strong corroboration these are real, not
reviewer idiosyncrasy.

## Gate status and recommended next step

This run allows a **single** revision cycle, and that cycle was consumed **without a
Holder spec change**. A second `needs_revision` ends the gate uncleared, so **the
Slice-A design gate does not clear in this run.** Because the revision budget is
exhausted, the repair must happen **out of band** — recommended operator action: open
a **fresh Slice-A design run** that retains everything credited below and adds the two
missing pieces.

### The next spec MUST (retaining everything credited unchanged)

1. **(`SA-ROTATION-UNDERFIRE`) Add an executable, reachable, non-over-firing
   rotation-path producer of `ExitUnrecoverableAcrossRotation = 97`** for a **normal
   session-bound supervised lane** locked out after a daemon boot-epoch rotation —
   daemon-side / process-state, non-widening, Slice-A-only. Most directly reachable
   and decoupling-safe: **map the daemon's `stale_daemon_identity` response
   (`http.go:681-699`), observed on the lane's own MCP client path, to
   `ErrUnrecoverableAcrossRotation` → `97`**, with a negative that ordinary non-epoch
   MCP/network errors do **not** fire. The **codex** wedge path (`loop.go:625-646`)
   must route the typed floor, not only emit an in-PTY prompt. Tests:
   `TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor`,
   `TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane` /
   `TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor`,
   `TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable`.
2. **(`SA-C2-TMUX-FORGE`) Repair C2 for the tmux carrier.** Add the tmux-path
   negative `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation`; and
   **either** corroborate `#{pane_dead_status}==97` against the wrapper's own durable
   `agent_exited.exit_code` (a status a same-uid child cannot forge on the tmux path)
   before recording the typed class, **or** honestly scope the tmux carrier's
   forge-resistance as **RFC-0168-bounded** (the same per-lane-uid boundary that makes
   Slice B unsolvable) and rely on the **forge-resistant direct-path
   `agent_exited.exit_code`** as the trustworthy carrier — and stop claiming the tmux
   Spot 2 is forge-resistant until then.
3. **KEEP** `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed`,
   `TestLaunchHandshakeFailureStaysHelperErrorNotFloor`, and
   `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` as no-over-fire /
   direct-path negatives.

## Carry forward unregressed (credited — do NOT reopen)

The reserved code `97` + the typed sentinel (§1); the Spot-1 narrowing
refuse-before-read + owner-unaffected shape (§2, A1/A4); the **exact-code-only Spot-2
classification logic** (§3.2–3.4, A2) — sound as a rule, pending a trustworthy
carrier; §3.5's structural dissolution of the launch-handshake raw-leak; §3.6's
relationship to `agent_exited_unsealed` + `HandleRecoveryCompleteStalled` (#292); the
direct-path C2 forge-resistance (§4, A5 — `SA-C2-DIRECT-PATH`); the additive
`isNecrosisStallClass` growth + the single disclosed
`TestNecrosisDomainMatchesConfirmedDeadConstants` change; and the
no-admin-token-widening invariant.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 **Slice A** falsification-gate
design run (**cycle 2 / revision re-attack**). The ledger verdict — not falsifier
completion — gates the phase: `needs_revision` returns the spec uncleared, and this is
the single allowed revision cycle. The Holder spec was **not revised** this cycle
(byte-identical to cycle 1), so the cycle-1 required rotation-path fix was never
applied and `SA-ROTATION-UNDERFIRE` still stands; falsifier-reviewer-004 additionally
lands a **new** material challenge, `SA-C2-TMUX-FORGE` — the tmux `#{pane_dead_status}`
carrier is forgeable by a same-uid provider child, breaking the C2 (A5) and
no-over-fire (A3) clearing assertions on the production carrier. No admin-token
widening, no elevated credential, no Slice-B smuggling → `needs_revision`, not
`reject`; both gaps are central, not residue → not `accept_with_findings`. Because the
revision budget is exhausted, the repair must be re-scoped into a fresh Slice-A design
run: add the reachable, non-over-firing `stale_daemon_identity → 97` rotation producer
(+ game-day tests) and repair the tmux carrier (corroborate against the wrapper's own
`agent_exited.exit_code`, or scope the tmux carrier as RFC-0168-bounded and rely on the
forge-resistant direct path), retaining the entire credited shape.</sub>
