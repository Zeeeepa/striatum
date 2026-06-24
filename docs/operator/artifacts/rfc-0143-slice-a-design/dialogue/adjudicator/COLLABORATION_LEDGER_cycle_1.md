---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 Slice A — the decoupled session_unrecoverable_across_rotation typed-exit floor (D261 Option 4): falsifiable implementation spec, fresh Slice-A-only design run"
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
    text: "Slice A closes #512 with a pure daemon-side/process-state floor and ZERO trust-model change. (1) A reserved agentloop exit code ExitUnrecoverableAcrossRotation = 97 + a typed sentinel ErrUnrecoverableAcrossRotation in a new go/pkg/agentloop/exitcodes.go (Slice A owns ONLY 97; no reseal-98, no resealInFlightJob, no connect-out channel, no kernel-token capture, no CapabilityReseal, no owner bundle 0021). (2) Spot 1 — credential-chain NARROWING: an unexported adminTokenReachedByNonOwner predicate added AHEAD of the existing read at the runtime client-token tier in BOTH resolvers (ResolveTokenMaterial token.go:31-42, ResolveTokenMaterialFresh endpoint.go:125-136); when a non-owner lane's chain reaches the owner-only admin runtime client-token it returns ('', ErrUnrecoverableAcrossRotation) BEFORE any read (local euid vs file-owner uid; EACCES/EPERM => non-owner; ENOENT => fall through unchanged); owner unaffected. Callers map ONLY the sentinel to a clean exit 97 (Run/RunContext loop.go:37/:78 -> main.go:109-117); the #323 rotation watcher (loop.go:602) requests the unrecoverable exit only when ResolveTokenMaterialFresh returns the sentinel AND cfg.Token.Source is the runtime client-token, otherwise keeps the existing launch-token fallback. (3) Spot 2 — daemon observes 97 from durable state on two paths: direct (agent_exited.exit_code, helper.go:433 -> supervision.go:425, no schema change) and tmux (#{pane_dead_status} via an additive PaneDeadStatus field on ProbeTmuxLiveness, tmux_liveness.go:228/:257); a new stallClassSessionUnrecoverableAcrossRotation + deadAgentUnrecoverableAcrossRotation predicate is interposed FIRST (exact-code-gated) ahead of deadAgentExitedUnsealed/agent_pid_dead in recoverStuckJobs (recovery_decision_tree.go:1136-1140 and the auto-finalize branch :957-1027), tagged into isNecrosisStallClass (:196-198) with the single disclosed change to TestNecrosisDomainMatchesConfirmedDeadConstants (additive domain growth). (4) §3.5: the launch-handshake path is NOT a floor carrier in Slice A — the reserved code is produced only AFTER RunHelper emits agent_started (helper.go:186-193), so a genuine launch failure stays a raw helper_error and the BC1-W1-CAPTURE-FLOOR raw-leak path is structurally absent. (5) §3.6: the typed class is a strict refinement of agent_exited_unsealed for the exact-97 case, routes the same finalize-from-durable-artifact path, does not duplicate/override HandleRecoveryCompleteStalled (#292). (6) §4 C2: only ErrUnrecoverableAcrossRotation maps to 97; a provider child's 97/98 is normalized to a generic error (normalizeAgentExitError loop.go:365-379). Assertions A1-A6 each paired with a named test, incl. A3 no-over-fire negative (TestOrdinaryUnsealedExitStaysAgentExitedUnsealed) and A5 (TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker)."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "DECOUPLING/DAEMON-SIDE lens. Credits the decoupling move (W1/kernel-capture/CapabilityReseal/reseal-file/resealInFlightJob/owner-bundle-0021/code-98 all deleted) and the exact-code-only Spot 2 as the right no-over-fire shape, and finds no admin-token widening and no lane-readable reseal bearer (so needs_revision, not reject). But the standing gap is gate-blocking: the ONLY causal trigger for 97 is the lane-side resolver sentinel, and the NORMAL post-boot-epoch rotation path either cannot reach that resolver from a non-owner lane or explicitly suppresses the reserved-code exit. Source-anchored: a normal supervised lane launches with its session-bound token as STRIATUM_MCP_TOKEN (supervision_env.go:333-343), so ResolveTokenMaterial returns Source: EnvMCPToken (token.go:18-21) and Run stores it in cfg.Token (loop.go:37-56) — the Holder's rotation guard cfg.Token.Source == <runtime client-token> is therefore FALSE in the ordinary case, so even if ResolveTokenMaterialFresh later returns the sentinel the spec keeps the launch token and does NOT request the unrecoverable exit; no 97. A second, earlier under-fire: the watcher detects rotation only via ResolveMCPEndpointFresh (loop.go:589-604) reading the owner-only 0700/0600 runtime endpoint/epoch files (main.go:632-640/:752-763/:798-815); a striatum-lane non-owner gets EACCES and the watcher treats it as 'nothing to compare against yet' and continues, never asking the token resolver, so the sentinel is never produced and 97 cannot be observed. The 'launch token still works' rebuttal fails because adapter reachability is also coupled to launch-time endpoint + boot-epoch state (STRIATUM_MCP_BOOT_EPOCH supervision_env.go:344-354 echoed as X-Striatum-Boot-Epoch mcpconfig.go:123-130/:217-222/:490-496; HTTP rejects a stale presented epoch as stale_daemon_identity before dispatch mcp/http.go:681-699; the claude rewrite path reuses the stale laneBootEpoch(); codex cannot reload its launch-time -c URL, loop.go:625-645). So a constructible post-restart lane is unable to complete through MCP while NEVER emitting 97; Spot 2 then records ordinary agent_exited_unsealed/agent_pid_dead. The typed floor UNDER-FIRES on the exact #512 boot-epoch rotation failure Slice A is supposed to make legible. Required revision: keep the no-widening refusal + the exact-code daemon classification, but add an executable rotation-path trigger reachable for a normal session-bound lane that does not over-fire (lane-readable non-secret endpoint/epoch freshness -> 97 on a proven stale launch epoch; OR map a daemon stale_daemon_identity response on the lane MCP client path to 97 with a negative that ordinary non-epoch MCP errors do not fire; OR update the stale boot-epoch header as part of a proven recoverable reconnect and reserve 97 for the remaining unrecoverable cases), plus tests TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane, TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor, and keep TestOrdinaryUnsealedExitStaysAgentExitedUnsealed."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "SECURITY/LEGIBILITY/REGRESSION lens. Finds NO reject-class widening (ReadTokenFile owner-mode guard preserved token.go:75-92, refusal before any non-owner read, no minted lane credential, no bearer carrying {admin, apply, recovery, surgical_recovery}) and credits C2 (normalizeAgentExitError loop.go:371-379 wraps the provider exit generic; only the sentinel maps to 97; the named test is present). The gate still does not clear under the legibility lens: the Holder makes the typed class fire ONLY from observed wrapper exit 97, but its only concrete way to emit 97 is a resolver sentinel the normal supervised boot-epoch-rotation path does not reliably reach. Same source chain as falsifier_1: session-bound EnvMCPToken (supervision_env.go:333-343, token.go:18-21, loop.go:32-56) makes the rotation guard false; owner-only fresh endpoint/epoch files make the watcher silently continue (loop.go:589-593) before it asks for fresh token material; and the bearer is not the whole client identity after #316 (STRIATUM_MCP_BOOT_EPOCH supervision_env.go:344-354; codex -c overrides mcpconfig.go:113-132; claude/agy echo laneBootEpoch() :217-222/:490-496; stale-epoch rejection mcp/http.go:681-699; codex wedge-only loop.go:625-645). So a lane wedged on stale endpoint / stale boot epoch is later classified ordinary agent_exited_unsealed/agent_pid_dead or a raw adapter/MCP failure, with NO durable session_unrecoverable_across_rotation explanation — an under-fire / raw-explanation leak for the class Slice A is supposed to make legible. Not accept_with_findings: the missing trigger is on the #512 path itself, not cosmetic; the build contract would let the strongest game-day shape die without the daemon ever observing 97. A1-A6 are named but under-specify the rotation path: no test proves a session-token lane with stale endpoint/boot-epoch state exits 97 or records the typed class. Required revision: add a reachable, non-over-firing trigger for normal supervised lanes after rotation (lane-readable non-secret endpoint/epoch freshness; OR map stale_daemon_identity to 97 with negatives for ordinary MCP/network errors; OR prove+test an in-place reconnect that updates endpoint+epoch per adapter, reserving 97 for the rest), plus TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor, TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor, TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable, and keep the negatives TestOrdinaryUnsealedExitStaysAgentExitedUnsealed + TestLaunchHandshakeFailureStaysHelperErrorNotFloor."
verdict: "needs_revision"
rationale: "This adjudicates the FRESH Slice-A-only design run for RFC 0143 (cycle 1), not the v1-v7 combined run. RFC 0143 is decided (D261, 2026-06-24): Slice A ships now as PURE daemon-side observability (Option 4 typed floor; mints no credential, widens no token, touches no trust model); Slice B (the CapabilityReseal authority + the W1 connect-out channel) is OUT OF SCOPE, blocked on RFC 0168 (#585). I judge the Slice-A implementation shape against the SEED clearing condition, not the split. Inputs read: the Holder spec (dialogue/holder/HOLDER.md), both falsifier re-attacks (dialogue/falsifier_1, dialogue/falsifier_2), the SEED.md charter (design shape, decoupling premise, six HARD CONSTRAINTS, assertions A1-A6, clearing condition), the committed RFC ## Decision (D261), and the v7 collaboration ledger (the BC1-W1-CAPTURE-FLOOR finding). Load-bearing source citations were independently re-verified against the current worktree HEAD on the run branch. WHAT THE SPEC GETS RIGHT (credited, carry forward unregressed): (i) Spot 1 is a genuine NARROWING, not a widening — adminTokenReachedByNonOwner is applied AHEAD of the existing read at the runtime client-token tier in both resolvers and returns ('', ErrUnrecoverableAcrossRotation) before any os.ReadFile; ReadTokenFile's owner-mode guard (token.go:82) is retained; no lane ever reads the admin token and no minted credential carries {admin, apply, recovery, surgical_recovery}. I confirmed ResolveTokenMaterial's order (token.go:18-53: EnvMCPToken, then EnvMCPTokenFile, then runtime client-token, then repo capability_token). Both falsifiers confirm no widening. (ii) The Spot-2 EXACT-CODE-ONLY classification is the correct no-over-fire shape: the typed class fires iff the observed exit == 97 (direct path agent_exited.exit_code; tmux path #{pane_dead_status}); an ordinary unsealed exit stays agent_exited_unsealed and a never-engaged crash stays agent_pid_dead. (iii) §3.5 correctly DISSOLVES the v7 BC1-W1-CAPTURE-FLOOR raw-leak on the launch-handshake path for the decoupled world: with no W1 capture boundary at launch, the reserved code is produced only AFTER agent_started, so a genuine launch failure stays a raw helper_error (correctly, it is not the floor) and there is no covered-miss to leak. (iv) §3.6 states the relationship to agent_exited_unsealed and HandleRecoveryCompleteStalled (#292): strict refinement, same finalize-from-durable-artifact path, no duplication/override. (v) §4 C2 forge-resistance is directionally satisfied with A5's named test. THE GATE NONETHELESS DOES NOT CLEAR. BOTH falsifiers independently land the SAME material, source-anchored challenge inside the Slice-A wiring — the floor UNDER-FIRES on the normal post-boot-epoch rotation lock-out — and it stands UNREBUTTED (the Holder had no further turn; the cycle ends at adjudication). I independently confirmed every load-bearing anchor of that challenge against the worktree: a normal supervised lane launches with its session-bound token as STRIATUM_MCP_TOKEN (supervision_env.go:342) so ResolveTokenMaterial returns the FIRST source EnvMCPToken (token.go:19-21) and never reaches the step-3 runtime client-token — the startup sentinel is NOT produced for it; the Holder's #323 rotation-watcher guard cfg.Token.Source == <runtime client-token> is therefore FALSE for the ordinary lane, so 97 is suppressed even if ResolveTokenMaterialFresh reaches the sentinel; and the watcher detects rotation only via ResolveMCPEndpointFresh against the owner-only 0700/0600 runtime endpoint/epoch files, where a striatum-lane non-owner gets EACCES and 'continues' silently (loop.go:589-593) without ever asking the token resolver. Crucially the Holder's own §2.3 concedes this ('A lane with a working session-bound token never reaches step 3 of the fresh resolver, so the sentinel is not even produced for it') and frames it as no-over-fire — but the rebuttal that 'a lane holding the session-bound bearer is still recoverable' is FALSE in current source: after #316 the bearer is not the whole client identity. The supervised env also injects STRIATUM_MCP_BOOT_EPOCH (supervision_env.go:352-354); the lane echoes it as X-Striatum-Boot-Epoch; the HTTP handler REJECTS a stale presented epoch as stale_daemon_identity before dispatch (mcp/http.go:681-699, code at :697); the claude rewrite path reuses the stale laneBootEpoch(); and codex cannot reload its launch-time -c MCP URL and gets only an in-PTY wedge prompt then returns nil (loop.go:625-645). So a lane that still holds a valid bearer can be pointed at a DEAD endpoint and/or present a STALE boot epoch the new daemon rejects — it cannot complete through MCP, yet emits no 97. Spot 2 then records an ordinary agent_exited_unsealed / agent_pid_dead / generic stale-MCP failure. This is exactly the silent unsealed exit / misleading dead-end the SEED's central deliverable forbids — the typed session_unrecoverable_across_rotation floor never becomes durable daemon state for the very boot-epoch rotation case Slice A exists to make legible. The challenge is not a Slice-B dependency and not a request to widen token access: it is a Slice-A wiring miss — the only specified floor signal is gated away or unreachable on the normal rotation path. CLEARING CONDITION, WALKED (all five must hold): (1) Both spots specified concretely with file:line anchors and the reserved code, both computed from daemon-side durable/process state with no inbound authenticated frame — PARTIAL/FAILS-AS-WIRED: the anchors and decoupling are sound for the cases the spec covers, but the trigger is not reachable for the normal rotation lock-out, so the spec does not actually deliver a daemon-observable floor on the #512 path. (2) No HARD CONSTRAINT violated — FAILS on no-raw-error/no-silent-exit: the covered rotation lock-out leaks a silent unsealed exit / raw stale-MCP explanation rather than the typed floor (the no-widening, no-Slice-B, no-over-fire, additive, daemon-side-only constraints all HOLD). (3) Every assertion A1-A6 stated and paired with a named test, incl. A3-neg and A5 — STRUCTURALLY PRESENT but UNDER-SPECIFIED: no assertion/test covers the actual rotation game-day shape (a session-token lane with stale endpoint/boot-epoch state exiting 97 / recording the typed class). (4) Relationship to agent_exited_unsealed and #292 stated — HOLDS (§3.6). (5) No new material challenge stands unrebutted — FAILS: the rotation under-fire is material (it would change the spec and exposes a real legibility defect on the target path) and stands unrebutted; both falsifiers land it independently from different lenses, strong corroboration it is genuine, not reviewer idiosyncrasy. WHY NOT reject: no path widens who can read the admin runtime client-token, no minted credential carries any of {admin, apply, recovery, surgical_recovery}, and no Slice-B artifact is smuggled in (no CapabilityReseal, connect-out channel, kernel-token capture, reseal-token file, reseal-98, or owner bundle 0021); both falsifiers explicitly confirm no widening and recommend needs_revision. WHY NOT accept_with_findings: the missing trigger is on the #512 path itself — the central deliverable — not a trackable post-clearance residue; the build contract as written would let the strongest game-day shape (GD-1/GD-2, the actual rotation lock-out) die without the daemon ever observing 97, so the legible floor Slice A promises is not delivered. That forecloses a clearing verdict. VERDICT: needs_revision. This run allows a SINGLE revision cycle, so the revision must clear on the falsifiers' single re-attack — be exact. THE NEXT REVISION MUST (retaining everything credited above unchanged): (A) Add an EXECUTABLE, REACHABLE, NON-OVER-FIRING rotation-path trigger that emits ExitUnrecoverableAcrossRotation = 97 for a NORMAL session-bound supervised lane that is locked out after a daemon boot-epoch rotation. Any of the falsifiers' three shapes is acceptable provided it is daemon-side/process-state, non-widening, and Slice-A-only: (a) make the non-secret endpoint/boot-epoch freshness lane-readable (no bearer exposure) and exit 97 when the lane proves it is carrying a STALE launch epoch / dead endpoint; OR (b) map the daemon's stale_daemon_identity response (mcp/http.go:697), observed on the lane's OWN MCP client path, to ErrUnrecoverableAcrossRotation -> 97 — this honors the decoupling premise (the daemon still only observes the reserved exit code from durable process state; the lane reading its own client response is not an inbound authenticated frame to the daemon), widens no token, and introduces no Slice-B artifact, and is the most directly reachable repair for session-token lanes; OR (c) prove and test an in-place reconnect that updates endpoint+epoch per adapter and reserve 97 for the remaining exact unrecoverable cases. The codex wedge path (loop.go:625-645) must route the typed floor, not only emit an in-PTY prompt. (B) Add the assertions+tests that cover the rotation game-day shape and pin the no-over-fire boundary: TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor (launch token source EnvMCPToken; lane presents a stale boot epoch / dead endpoint after rotation; assert wrapper exits 97 AND recovery records session_unrecoverable_across_rotation), TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane / TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor (non-owner lane, fresh endpoint/epoch files inaccessible; assert it does NOT silently continue and die generic), TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable (codex cannot reload -c URL; assert the terminal state is the typed floor); and KEEP the negatives TestOrdinaryUnsealedExitStaysAgentExitedUnsealed and TestLaunchHandshakeFailureStaysHelperErrorNotFloor so the new trigger does not over-fire on an ordinary unsealed exit, a healthy lane, an ordinary non-epoch MCP/network error, or a genuine launch failure. CARRY FORWARD UNREGRESSED (do NOT reopen): the reserved code 97 + the typed sentinel (§1), the Spot-1 NARROWING refuse-before-read + owner-unaffected shape (§2, A1/A4), the exact-code-only Spot-2 classification on both observation paths (§3.1-3.4, A2/A3), §3.5's structural dissolution of the launch-handshake raw-leak (the launch path is not a floor carrier; a genuine launch failure stays helper_error), §3.6's relationship to agent_exited_unsealed + HandleRecoveryCompleteStalled (#292), §4 C2 forge-resistance (A5), the additive isNecrosisStallClass growth + the single disclosed TestNecrosisDomainMatchesConfirmedDeadConstants change, and the no-admin-token-widening invariant. The fix is ADDITIVE to the existing credited shape: a reachable rotation-path producer of the SAME reserved code Spot 2 already observes."
findings:
  - id: SA-ROTATION-UNDERFIRE
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2", "dialogue:3"]
    affected_invariants:
      - "R4 legible-failure: the typed session_unrecoverable_across_rotation floor must FIRE on the normal post-boot-epoch rotation lock-out, not only a startup-time resolver refusal a session-bound lane never reaches"
    challenge: "OPEN — verdict-driving (NEW; both falsifiers, independent lenses, landed_unrebutted). The Holder's ONLY causal trigger for exit 97 is the lane-side resolver sentinel, but the normal post-rotation supervised-lane path neither reaches it nor is allowed to emit it. Independently confirmed against the worktree HEAD: (1) a normal supervised lane launches with its session-bound token as STRIATUM_MCP_TOKEN (supervision_env.go:342), so ResolveTokenMaterial returns the FIRST source EnvMCPToken (token.go:19-21) and never reaches the step-3 runtime client-token — the startup sentinel is not produced; (2) the #323 rotation-watcher guard cfg.Token.Source == <runtime client-token> is therefore FALSE for the ordinary lane, so 97 is suppressed even if ResolveTokenMaterialFresh reaches the sentinel (the Holder's §2.3 concedes this and frames it as no-over-fire); (3) the watcher detects rotation only via ResolveMCPEndpointFresh against the owner-only 0700/0600 runtime endpoint/epoch files, where a striatum-lane non-owner gets EACCES and continues silently (loop.go:589-593) without asking the token resolver. The 'session bearer still works' rebuttal is FALSE after #316: the supervised env also injects STRIATUM_MCP_BOOT_EPOCH (supervision_env.go:352-354), the lane echoes it as X-Striatum-Boot-Epoch, and the HTTP handler REJECTS a stale presented epoch as stale_daemon_identity before dispatch (mcp/http.go:681-699, :697); the claude rewrite path reuses the stale laneBootEpoch(), and codex cannot reload its launch-time -c URL (loop.go:625-645). So a lane holding a valid bearer can be on a DEAD endpoint and/or present a STALE epoch the new daemon rejects, cannot complete through MCP, and dies with NO 97 — recovery then records ordinary agent_exited_unsealed / agent_pid_dead / a generic stale-MCP failure, exactly the silent unsealed exit the SEED forbids. Not a Slice-B dependency and not a widening request: a Slice-A wiring miss on the very #512 path Slice A exists to make legible. FIX (single revision cycle): add a reachable, non-over-firing rotation-path producer of 97 for session-bound lanes — (a) lane-readable non-secret endpoint/epoch freshness -> 97 on a proven stale launch epoch / dead endpoint; OR (b) map the daemon stale_daemon_identity response observed on the lane's own MCP client path to 97 (decoupling-safe: the daemon still only observes the exit code; widens no token; no Slice-B artifact) with a negative that ordinary non-epoch MCP/network errors do not fire; OR (c) a proven in-place endpoint+epoch reconnect per adapter, reserving 97 for the rest; route the codex wedge path to the typed floor. Tests: TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor, TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane / TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor, TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable; KEEP TestOrdinaryUnsealedExitStaysAgentExitedUnsealed + TestLaunchHandshakeFailureStaysHelperErrorNotFloor as no-over-fire negatives."
  - id: SA-SPOT1-NARROWING
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 1 no-widening: Spot 1 narrows (refuses a step), never adds a read path"
    challenge: "RESOLVED AS FRAMED (both falsifiers confirm no widening; I independently confirmed the resolver order). Spot 1 adds adminTokenReachedByNonOwner AHEAD of the existing read at the runtime client-token tier in BOTH ResolveTokenMaterial (token.go:31-42) and ResolveTokenMaterialFresh (endpoint.go:125-136), returning ('', ErrUnrecoverableAcrossRotation) BEFORE any os.ReadFile when a non-owner lane's chain reaches the owner-only admin runtime client-token (local euid vs file-owner uid; EACCES/EPERM => non-owner; ENOENT => fall through unchanged; owner unaffected). ReadTokenFile's owner-mode guard (token.go:82) is retained. No lane ever reads the admin token; no minted credential carries {admin, apply, recovery, surgical_recovery}. This is a narrowing, not a widening — the single hottest blast-radius dimension is clean. NOTE: correct, but the sentinel it produces is the trigger that does NOT reach the normal rotation lock-out — see SA-ROTATION-UNDERFIRE."
  - id: SA-SPOT2-EXACTCODE
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 4 no-over-fire: the typed class fires iff the observed exit code == 97"
    challenge: "RESOLVED AS FRAMED (both falsifiers credit it as the right no-over-fire direction). Spot 2 observes 97 from durable state on two paths — direct (agent_exited.exit_code, helper.go:433 -> supervision.go:425, no schema change) and tmux (#{pane_dead_status} via an additive PaneDeadStatus field on ProbeTmuxLiveness, tmux_liveness.go:228/:257) — and the new stallClassSessionUnrecoverableAcrossRotation + deadAgentUnrecoverableAcrossRotation predicate is interposed exact-code-gated AHEAD of deadAgentExitedUnsealed/agent_pid_dead in recoverStuckJobs (recovery_decision_tree.go:1136-1140 and the auto-finalize branch :957-1027), with isNecrosisStallClass (:196-198) gaining the member (additive domain growth; single disclosed TestNecrosisDomainMatchesConfirmedDeadConstants change). An ordinary unsealed exit stays agent_exited_unsealed; a never-engaged crash stays agent_pid_dead. The shape is correct and must be retained; the gap is upstream — no reachable producer emits 97 on the normal rotation path (SA-ROTATION-UNDERFIRE)."
  - id: SA-LAUNCH-FLOOR-DISSOLVED
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 4 no-raw-error-leak on the launch-handshake path (the v7 BC1-W1-CAPTURE-FLOOR concern)"
    challenge: "RESOLVED / DISSOLVED FOR THE DECOUPLED WORLD (both falsifiers credit it as scoped). The v7 BC1-W1-CAPTURE-FLOOR raw-leak was a property of the W1 capture boundary at launch (a Slice-B mechanism). Slice A has NO W1 capture boundary and NO kernel-token capture at launch; the reserved code is produced only AFTER RunHelper emits agent_started (helper.go:186-193), and waitForHelperAgentStart returns on the first agent_started which precedes any agent_exited in the JSONL. So the launch/attach helper_error phase launch path requires no change and correctly stays a RAW helper_error for GENUINE launch failures (which are not the floor) — the raw-error-leak path is structurally absent on the launch handshake, and a genuine launch failure is never reclassified as the floor (no over-fire). NOTE: this dissolves the launch-handshake raw-leak; it does NOT address the SEPARATE legibility goal of routing the typed floor on the post-rotation lock-out — that reappears as SA-ROTATION-UNDERFIRE, now on the credential/rotation path rather than the launch path."
  - id: SA-NO-WIDENING
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "HARD CONSTRAINT 1 (no token widening) + HARD CONSTRAINT 2 (no new credential / no Slice B)"
    challenge: "RESOLVED (both falsifiers confirm; the reason this is needs_revision, not reject). No path widens who can read the admin runtime client-token; ReadTokenFile's owner-mode guard is preserved (token.go:75-92) and not relaxed to group-read; the refusal precedes any non-owner read; Slice A introduces only an exit code, a recovery class, and an additive tmux probe field — no minted credential carries {admin, apply, recovery, surgical_recovery}, and no Slice-B artifact (CapabilityReseal, connect-out channel, kernel-token capture, reseal-token file, reseal-98, owner bundle 0021) is introduced. The revision MUST preserve this: any reachable rotation-path trigger added for SA-ROTATION-UNDERFIRE must remain non-widening and Slice-A-only (a process exit code / a non-secret freshness read / a lane-observed daemon response), never a path that lets a lane read the admin token or carry an elevated verb."
  - id: SA-C2-FORGE
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:3"]
    affected_invariants:
      - "C2 forge-resistance: a provider child's 97/98 cannot drive the reserved floor code"
    challenge: "RESOLVED / directionally satisfied with a named test (falsifier_2 confirms). normalizeAgentExitError (loop.go:365-379) wraps the inner provider child's exit as a generic 'agent command exited' error and does not propagate its numeric code; the reserved 97 is emitted only by main.go:109-117 and only for errors.Is(err, ErrUnrecoverableAcrossRotation). A provider child that exits 97/98 produces a generic error -> exit 1, never 97, and never the sentinel/typed class. Test: TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker (A5). Retain verbatim; the revision's new trigger must likewise emit 97 ONLY from the agentloop's own decision, never from a provider child's status."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 Slice A design run (cycle 1)

author: adjudicator-author-001

> Adjudication of the **fresh Slice-A-only** design-run dialogue trajectory for
> RFC 0143 (*lane credential survival across a daemon boot-epoch rotation*). RFC
> 0143 is decided by **D261** (2026-06-24): **Slice A** ships now as **pure
> daemon-side observability** — the Option-4 typed `session_unrecoverable_across_rotation`
> exit floor, which **mints no credential, widens no token, and touches no trust
> model**; **Slice B** (the `CapabilityReseal` authority + the W1 connect-out
> channel) is **OUT OF SCOPE**, blocked on **RFC 0168 (#585)**. I judge the
> Slice-A implementation shape against the SEED clearing condition, **not** the
> split or the per-lane-uid direction. Inputs read: the Holder spec
> (`dialogue/holder/HOLDER.md`), both falsifier re-attacks
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`), the
> `SEED.md` charter (design shape, the **decoupling premise**, the six HARD
> CONSTRAINTS, assertions A1–A6, and the clearing condition), the committed RFC
> `## Decision (D261)`, and the v7 collaboration ledger (the `BC1-W1-CAPTURE-FLOOR`
> finding). No raw terminal output was read. Load-bearing source citations were
> **independently re-verified against the current worktree HEAD** on the run
> branch.

## Verdict

**verdict: needs_revision**

The Slice-A spec is materially the right shape on every axis except one — and
that one is the **central deliverable**. It correctly deletes all of Slice B
(no W1 channel, no kernel-token capture, no `CapabilityReseal`, no reseal-token
file, no `resealInFlightJob`, no owner bundle 0021, no reserved code 98), it
**narrows** the credential chain rather than widening it (Spot 1 refuses the
owner-only admin runtime `client-token` for a non-owner lane **before any read**;
owner unaffected), and it gives Spot 2 the **right exact-code-only** classification
(the typed class fires **iff** the observed wrapper exit code is `97`, so ordinary
unsealed exits stay `agent_exited_unsealed`). It also dissolves the v7
`BC1-W1-CAPTURE-FLOOR` raw-leak on the launch-handshake path **structurally** for
the decoupled world (§3.5).

But the gate does **not** clear. **Both falsifiers — independently, from the
decoupling lens and the security/legibility lens — land the same material,
source-anchored challenge, and it stands unrebutted** (the Holder had no further
turn): the floor's **only** trigger is a lane-side resolver sentinel that the
**normal post-boot-epoch rotation path does not reach and is not allowed to
emit**, so the very `#512` rotation lock-out Slice A exists to make legible dies
with **no `97`** and is recorded as an ordinary class. The legible floor is not
delivered on its target path.

**Why not `reject`.** No path widens who can read the admin runtime `client-token`;
no minted credential carries any of `{admin, apply, recovery, surgical_recovery}`;
no Slice-B artifact is smuggled in. Both falsifiers confirm the no-widening
invariant and recommend `needs_revision`. The defect is an under-fire / legibility
miss, not a widening.

**Why not `accept_with_findings`.** The missing trigger is on the `#512` path
**itself** — the core deliverable — not a cosmetic residue. The build contract as
written would let the strongest game-day shape (GD-1/GD-2, the actual rotation
lock-out) die without the daemon ever observing `97`. That is not a trackable
post-clearance finding; it forecloses a clearing verdict.

> **This run allows a SINGLE revision cycle.** A second `needs_revision` on
> re-attack ends the gate uncleared, so the revision below is stated to be
> sufficient to clear: it is **additive** to the credited shape — a reachable
> producer of the **same** reserved code Spot 2 already observes — and changes
> none of the credited mechanics.

## The clearing condition, walked

A clearing verdict requires **all five**; the rotation-trigger-dependent ones
fail:

1. **Both spots specified concretely + decoupled — PARTIAL / fails as wired.**
   The file:line anchors, the reserved code, and the decoupling are sound *for the
   cases the spec covers*, but the trigger is **unreachable** on the normal
   rotation path, so the spec does not actually yield a daemon-observable floor on
   the `#512` path (CHALLENGE / `SA-ROTATION-UNDERFIRE`).
2. **No HARD CONSTRAINT violated — FAILS on no-raw-error / no-silent-exit.** The
   covered rotation lock-out leaks a **silent unsealed exit / raw stale-MCP
   explanation** instead of the typed floor. (no-widening, no-Slice-B, no-over-fire,
   additive, daemon-side-only all **hold**.)
3. **A1–A6 stated + named tests, incl. A3-neg and A5 — structurally present but
   under-specified.** No assertion/test covers the actual rotation game-day shape
   (a session-token lane carrying stale endpoint/boot-epoch state exiting `97` /
   recording the typed class).
4. **Relationship to `agent_exited_unsealed` + `HandleRecoveryCompleteStalled`
   (#292) stated — HOLDS** (§3.6: strict refinement, same finalize-from-durable
   path, no duplication/override).
5. **No new material challenge standing unrebutted — FAILS.** The rotation
   under-fire is material and unrebutted; both falsifiers land it independently.

## What the spec genuinely got right (credited — carry forward unregressed)

- **Spot 1 is a NARROWING, not a widening** (`SA-SPOT1-NARROWING`,
  `SA-NO-WIDENING`). `adminTokenReachedByNonOwner` is applied **ahead of** the
  existing read at the runtime `client-token` tier in **both** resolvers
  (`token.go:31-42`, `endpoint.go:125-136`) and returns the sentinel **before any
  `os.ReadFile`**; `ReadTokenFile`'s owner-mode guard (`token.go:82`) is retained.
  I confirmed the resolver order in `token.go:18-53`.
- **Spot 2 is exact-code-only** (`SA-SPOT2-EXACTCODE`). The typed class fires
  **iff** the observed exit `== 97`, on both observation paths (direct
  `agent_exited.exit_code`; tmux `#{pane_dead_status}`), interposed ahead of the
  existing `agent_exited_unsealed`/`agent_pid_dead` branches — the correct
  no-over-fire shape.
- **§3.5 dissolves the launch-handshake raw-leak structurally**
  (`SA-LAUNCH-FLOOR-DISSOLVED`). With no W1 capture boundary at launch, the
  reserved code is produced only **after** `agent_started` (`helper.go:186-193`),
  so a genuine launch failure correctly stays a raw `helper_error` and there is no
  covered miss to leak.
- **§3.6 states the `agent_exited_unsealed` / #292 relationship** (strict
  refinement; same finalize path; no duplication/override).
- **§4 C2 forge-resistance** is directionally satisfied with A5's named test
  (`SA-C2-FORGE`).

## The verdict-driving gap (independently confirmed against the worktree)

The Holder's `97` has exactly one producer: the lane-side sentinel
`ErrUnrecoverableAcrossRotation`, emitted when a resolver reaches the owner-only
admin runtime `client-token` for a non-owner lane. The **normal supervised lane**
never gets there on the rotation path:

- It launches with its **session-bound** token as `STRIATUM_MCP_TOKEN`
  (`supervision_env.go:342`), so `ResolveTokenMaterial` returns the **first**
  source `EnvMCPToken` (`token.go:19-21`) and **never reaches** the step-3 runtime
  `client-token` — the startup sentinel is not produced.
- The Holder's #323 rotation-watcher guard `cfg.Token.Source == <runtime
  client-token>` is therefore **false** for the ordinary lane, so `97` is
  **suppressed** even if `ResolveTokenMaterialFresh` reaches the sentinel. The
  Holder's own §2.3 concedes this and frames it as no-over-fire.
- The watcher only detects rotation via `ResolveMCPEndpointFresh` against the
  owner-only `0700`/`0600` runtime endpoint/epoch files; a `striatum-lane`
  non-owner gets `EACCES` and **continues silently** (`loop.go:589-593`) without
  ever asking the token resolver.

The Holder's rebuttal — "a lane holding the session-bound bearer is still
recoverable, so suppressing `97` is correct" — is **false in current source**.
After **#316** the bearer is not the whole client identity:

- the supervised env also injects `STRIATUM_MCP_BOOT_EPOCH`
  (`supervision_env.go:352-354`); the lane echoes it as `X-Striatum-Boot-Epoch`;
- the HTTP handler **rejects a stale presented epoch as `stale_daemon_identity`
  before dispatch** (`mcp/http.go:681-699`, code at `:697`);
- the claude rewrite path reuses the **stale** `laneBootEpoch()`; codex **cannot
  reload** its launch-time `-c` MCP URL and gets only an in-PTY wedge prompt then
  returns `nil` (`loop.go:625-645`).

So a lane that still holds a valid bearer can be on a **dead endpoint** and/or
present a **stale epoch** the new daemon rejects — it cannot complete through MCP,
yet emits no `97`. Spot 2 then records an ordinary `agent_exited_unsealed` /
`agent_pid_dead` / generic stale-MCP failure. This is exactly the **silent
unsealed exit / misleading dead-end** the SEED's central deliverable forbids: the
typed `session_unrecoverable_across_rotation` floor never becomes durable daemon
state for the boot-epoch rotation case Slice A exists to make legible. It is a
**Slice-A wiring miss**, not a Slice-B dependency and not a widening request.

## Per-HARD-CONSTRAINT disposition

| HARD CONSTRAINT | Disposition |
|---|---|
| 1 — No token widening | **RESOLVED.** Spot 1 narrows; refuse-before-read; owner-mode guard retained; no lane reads the admin token; no elevated minted credential. |
| 2 — No new credential / no Slice B | **RESOLVED.** Only an exit code + recovery class + additive tmux field; no `CapabilityReseal`/channel/kernel-capture/reseal-file/reseal-98/owner-bundle-0021. |
| 3 — Daemon-side / process state only | **RESOLVED** for covered cases (euid-vs-owner; `agent_exited.exit_code`; `#{pane_dead_status}`; existing artifact/reconstructability/liveness). |
| 4 — No over-fire | **RESOLVED.** Exact-code-only Spot 2 keeps ordinary unsealed exits and healthy lanes in their existing class. |
| 4 — No raw-error leak / no silent exit | **OPEN.** The launch-handshake leak is dissolved (§3.5), but the **normal rotation lock-out leaks a silent unsealed exit / raw stale-MCP explanation** instead of the typed floor (`SA-ROTATION-UNDERFIRE`). |
| 5 — Additive-only | **RESOLVED.** New file/branch/class/field; single disclosed `TestNecrosisDomainMatchesConfirmedDeadConstants` growth is additive. |
| 6 — Product-boundary-safe | **RESOLVED.** No hosted service, no durable transcript, no external persistence. |

## Per-assertion (A1–A6) disposition

| Assertion | Disposition |
|---|---|
| A1 (Spot 1 narrowing + owner-unaffected) | **RESOLVED** as a shape; tests named (`TestResolveRefusesRuntimeClientTokenForLane` + owner companion + the `main.go` mapping test). The sentinel it produces is the trigger that does not reach the rotation lock-out (A-coverage gap). |
| A2 (reserved code → typed class, both paths) + A2-neg | **RESOLVED** for the observation/classification; tests named. Depends on a `97` being produced, which the rotation path does not (`SA-ROTATION-UNDERFIRE`). |
| A3 (no over-fire) | **RESOLVED**; `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed` named — **retain**. |
| A4 (no widening) | **RESOLVED**; `TestLaneNeverReadsAdminRuntimeToken` + A1 split. |
| A5 (C2 forge-resistance) | **RESOLVED**; `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` named — **retain**. |
| A6 (no regression; additive) | **RESOLVED**; additive growth disclosed. |
| **A-coverage (rotation game-day shape)** | **OPEN.** No assertion/test proves a session-token lane with stale endpoint/boot-epoch state exits `97` / records the typed class — the gap behind `SA-ROTATION-UNDERFIRE`. |

## Falsifier challenge dispositions

- **falsifier-reviewer-001 — rotation path under-fires before it can emit `97`
  (DECOUPLING lens; material; landed unrebutted).** Claim challenged: Slice A
  closes #512 with a daemon-side floor. Material? **Yes** — it exposes that the
  only floor trigger is gated away (`cfg.Token.Source == EnvMCPToken`) or
  unreachable (owner-only fresh endpoint files → silent continue) for the normal
  supervised lane, and would change the spec (add a reachable rotation trigger +
  tests). Rebutted? **No** — the Holder's §2.3 frames the suppression as a feature;
  the bearer-still-works premise is false after #316. Disposition:
  **`SA-ROTATION-UNDERFIRE` open; verdict-driving.** No widening; `needs_revision`,
  not `reject`.
- **falsifier-reviewer-002 — the normal rotation lock-out can die without emitting
  `97` (SECURITY/LEGIBILITY/REGRESSION lens; material; landed unrebutted).** Claim
  challenged: the typed floor is delivered for #512. Material? **Yes** — same root,
  reinforced with the #316 boot-epoch coupling (`stale_daemon_identity`,
  `mcp/http.go:697`) and the codex wedge path, and the observation that A1–A6 name
  no test for the rotation game-day shape. Rebutted? **No.** Security/regression
  sweep: **no** widening, **no** elevated credential, C2 directionally satisfied,
  exact-code no-over-fire shape correct. Disposition: **`SA-ROTATION-UNDERFIRE`
  open; the no-widening / no-Slice-B / additive constraints intact.**

Both falsifiers **credit the Slice-A shape** and then land **distinct,
independent, source-confirmed** statements of the **same** under-fire — strong
corroboration the residue is genuine, not reviewer idiosyncrasy. The lineage from
the v7 ledger is clean: v7's `BC1-W1-ORACLE` dissolves with the deleted W1
channel; v7's `BC1-W1-CAPTURE-FLOOR` raw-leak dissolves on the launch path (§3.5)
— but its **legibility goal** (route the typed floor on the failure, never a raw
error) **reappears** on the credential/rotation path, where it is now unmet.

## What the next revision MUST fix to clear on the single re-attack

Retain **everything credited above unchanged**. Then, additively:

1. **(`SA-ROTATION-UNDERFIRE`) Add an executable, reachable, non-over-firing
   rotation-path producer of `ExitUnrecoverableAcrossRotation = 97`** for a
   **normal session-bound supervised lane** locked out after a daemon boot-epoch
   rotation. Any of the falsifiers' three shapes is acceptable provided it stays
   **daemon-side / process-state, non-widening, and Slice-A-only**:
   - **(a)** make the **non-secret** endpoint/boot-epoch freshness **lane-readable**
     (no bearer exposure) and exit `97` when the lane proves it carries a **stale**
     launch epoch / dead endpoint; **or**
   - **(b)** map the daemon's **`stale_daemon_identity`** response (`mcp/http.go:697`),
     observed on the lane's **own** MCP client path, to `ErrUnrecoverableAcrossRotation`
     → `97`. **This honors the decoupling premise** — the daemon still only observes
     the reserved exit code from durable **process** state; the lane reading its own
     client response is **not** an inbound authenticated frame *to the daemon* — and
     it widens no token and introduces no Slice-B artifact. It is the **most directly
     reachable** repair for session-token lanes. Pair it with a negative that an
     **ordinary non-epoch MCP/network error does not** fire the floor; **or**
   - **(c)** prove and test an **in-place reconnect** that updates endpoint + epoch
     per adapter and reserve `97` for the remaining exact unrecoverable cases.

   The **codex** wedge path (`loop.go:625-645`) must **route the typed floor**, not
   only emit an in-PTY prompt.
2. **(A-coverage) Add the assertions + tests for the rotation game-day shape and
   pin the no-over-fire boundary:**
   - `TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor` — launch token
     source `EnvMCPToken`; lane presents a stale boot epoch / dead endpoint after
     rotation; assert the wrapper exits `97` **and** recovery records
     `session_unrecoverable_across_rotation`.
   - `TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane` /
     `TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor` —
     non-owner lane, fresh endpoint/epoch files inaccessible; assert it does **not**
     silently continue and die generic.
   - `TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable` — codex cannot reload
     its `-c` URL; assert the terminal state is the typed floor.
   - **KEEP** `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed` **and**
     `TestLaunchHandshakeFailureStaysHelperErrorNotFloor` so the new trigger does
     not over-fire on an ordinary unsealed exit, a healthy lane, an ordinary
     non-epoch MCP/network error, or a genuine launch failure.

## Carry forward unregressed (do NOT reopen)

The reserved code `97` + the typed sentinel (§1); the Spot-1 narrowing
refuse-before-read + owner-unaffected shape (§2, A1/A4); the exact-code-only Spot-2
classification on both observation paths (§3.1–3.4, A2/A3); §3.5's structural
dissolution of the launch-handshake raw-leak; §3.6's relationship to
`agent_exited_unsealed` + `HandleRecoveryCompleteStalled` (#292); §4 C2
forge-resistance (A5); the additive `isNecrosisStallClass` growth + the single
disclosed `TestNecrosisDomainMatchesConfirmedDeadConstants` change; and the
no-admin-token-widening invariant. The fix is **additive**: a reachable
rotation-path producer of the **same** reserved code Spot 2 already observes.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 **Slice A** falsification-gate
design run (cycle 1). The ledger verdict — not falsifier completion — gates the
phase: `needs_revision` returns the spec uncleared. The spec correctly deletes all
of Slice B, **narrows** the credential chain (no widening), gives Spot 2 the right
exact-code-only classification, and dissolves the v7 `BC1-W1-CAPTURE-FLOOR`
raw-leak on the launch path. But both falsifiers independently land the same
source-confirmed material gap: the floor's only trigger is a lane-side sentinel the
normal post-boot-epoch rotation path neither reaches (session-bound `EnvMCPToken`;
owner-only fresh endpoint files → silent continue) nor is allowed to emit
(`cfg.Token.Source == EnvMCPToken` suppresses the gated exit), so the `#512`
rotation lock-out — now coupled to stale endpoint + stale boot-epoch state after
#316 — dies with no `97` and is recorded as an ordinary class. The legible floor
under-fires on its target path. No admin-token widening, no elevated credential, no
Slice-B smuggling → `needs_revision`, not `reject`; the gap is the central
deliverable, not a residue → not `accept_with_findings`. The single revision must
add a reachable, non-over-firing, Slice-A-only rotation-path producer of the same
reserved `97` (the `stale_daemon_identity` mapping is the most directly reachable
and decoupling-safe), plus the rotation game-day tests, retaining the entire
credited shape.</sub>
