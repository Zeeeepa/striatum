---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0168 P0 — per-lane OS uid as the lane security principal: v4 REVISION re-attack (cycle 1) — discharge the single standing v3 residual C2-RESIDUAL (OQ4-ACL-PROVISIONING-TRANSITION, the per-supervisor MCP-bearer-path realness) by re-rooting the live writeEphemeralMCPConfig bearer writer under .striatum/scratch/<supervisor_id>/, correcting the false mcpconfig.go:241/266 citation to :550-559, making A22 derive-and-exercise the live resolved path, adding A24, and making the provider/token-cache forbidden set explicit — while carrying the v1-proven hard core HC-A1..A5, the v3-DISCHARGED C1-RESIDUAL (classifyPoolUIDTaskState P1 + A21), the C1 four-state lease machine, the C2 procedure fix + A23 + the .striatum/-excluding GROUP-ACL end-state invariant, and OQ1/OQ3/OQ5/OQ6 + the narrowing invariant forward unregressed"
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
    text: "v4 REVISION of the RFC 0168 P0 falsification_gate spec, charged to discharge the SINGLE standing v3 cycle-2 residual — C2-RESIDUAL (OQ4-ACL-PROVISIONING-TRANSITION), the per-supervisor MCP-bearer-path realness — while carrying forward unregressed everything v1/v2/v3 cleared, including the v3-DISCHARGED C1-RESIDUAL. Four source-anchored parts: (1) a REQUIRED P0 build step re-roots writeEphemeralMCPConfig (mcpconfig.go:550-565) from <repoRoot>/.striatum/scratch ROOT to <repoRoot>/.striatum/scratch/<supervisor_id>/, threading STRIATUM_SUPERVISOR_ID exactly as loop.go:289/:139-145 (the pty.log writer) and the gemini markers (mcpconfig.go:245/:266) do, BEFORE re-keying the scratch ACLs, so the --x-only-scratch-root / rwx-on-supervisor-subdir final state is launch-consistent (no EACCES/#279 break; the subdir already exists from supervision_control.go:115); tmp fallback preserved, scratch-root never a target. (2) A22 TestPoolACLProvisioningNeverTransientlyExposesScratch is made REAL — it DERIVES the bearer path from the live writer (set STRIATUM_SUPERVISOR_ID, call/share the resolver, not a hand-planted fixture), asserts it resolves under .striatum/scratch/S1/, asserts no residual root-level .striatum/scratch/lane-mcp-config-* bearer after the transition, and adds a launch-positive control (S1 own uid CreateTemps successfully). (3) the provider/token-cache forbidden top-level set is made EXPLICIT (.gemini/.claude/.codex/configured credential caches) in the OQ4 allowlist + the A23 planner guard. (4) the false v3 source citation is CORRECTED — the bearer is mcpconfig.go:550-559, not :241/266 (those are the gemini-cli settings.json write + backup/created markers). A24 TestEphemeralMCPConfigResolvesUnderSupervisorScratchSubdir added for writer-realness; A23 KEPT (necessary, not sufficient). Carries HC-A1..A5, the C1 four-state lease machine, the C1-RESIDUAL P1 -> classifyPoolUIDTaskState + A21, the C2 procedure fix + the .striatum/-excluding GROUP-ACL end-state invariant, OQ1/OQ3/OQ5/OQ6, and the narrowing invariant. Build-run target: runtime migration 0046+ (0045 reserved by RFC 0170 P0; not hardcoded) + owner-bundle bump owner/0023+ for striatumd.lane_uid_leases. Re-verified against worktree HEAD f63b895f."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "C2 bearer-path realness lens. The stale v3 objection does NOT land as written and is CREDITED: the v4 holder correctly identifies the live Claude bearer writer as writeEphemeralMCPConfig (mcpconfig.go:550-580), corrects the false :241/266 citation, requires the writer to thread STRIATUM_SUPERVISOR_ID, makes A22 derive the writer resolved path, and adds A24 for writer-realness — the bearer-path sub-residual is genuinely closed. The remaining material gap is narrower and STANDS: the provider/token-cache half of the C2 fix is still not source-real when a configured credential cache is nested under an otherwise allowlisted source top-level. Source-reachable chain: command_env can set provider selector vars (laneCommandEnv rejects only PATH and STRIATUM_* keys, supervision_lane_config.go:411-450); applyLaneLaunchEnv merges them into the live lane env (supervision_env.go:102-113); the run-as filter drops keys containing TOKEN/SECRET/PASSWORD/API_KEY/CREDENTIAL/DSN but NOT CLAUDE_CONFIG_DIR (supervision_env.go:274-318); the resolver then treats CLAUDE_CONFIG_DIR as authoritative and returns filepath.Join(CLAUDE_CONFIG_DIR, .credentials.json) before HOME (laneproviderauth/resolver.go:78-83). Concrete failing case: a lane launched with command_env CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude resolves its credential at <repoRoot>/docs/.lane-auth/claude/.credentials.json; the OQ4.1(a) mandatory form enumerates <repoRoot> top-level entries, prunes the forbidden set, then applies setfacl -R -m g:striatum-lanes:rX -m d:g:striatum-lanes:rX to each remaining source entry — but docs/ is a normal source top-level, NOT one of .striatum/.git/.gemini/.claude/.codex; so the planner recursively group-grants docs/, sweeping the access+default ACL onto docs/.lane-auth/claude/ and .credentials.json. A23 PASSES because its planned target is <repoRoot>/docs, not a forbidden path itself, and its only ancestor rule is for <repoRoot> / ancestors of .striatum/ — there is NO rule rejecting a recursive grant on an ANCESTOR of a configured credential cache. A22 also passes because it exercises the real bearer under .striatum/scratch/S1/, not a nested provider cache. The missing requirement: configured credential caches must be forbidden with ANCESTRY semantics (path-aware exclusion / reject recursive grants on any ancestor of a credential cache / exclude-at-traversal), plus a test that sets CLAUDE_CONFIG_DIR under docs/ and asserts no g:striatum-lanes ACL appears on it. Until then the v4 fix closes the bearer-path problem but leaves the provider-auth forbidden set sweepable into the source allowlist."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "No-regression lens. C1 does NOT break: P1 is still the fail-closed classifyPoolUIDTaskState predicate, not processZombie; T/t, unknown state, and unreadable-still-present PIDs block returned; /proc evidence is recorded in scrub_proof/scrub_failure — I cannot construct a non-zombie pool_uid survivor that reaches returned under the v4 text. HC-A1..A5, the four-state lease machine, uq_lane_uid_held, the 3-transaction scrub boundary, reaper/restart survival, OQ1/OQ3/OQ5/OQ6, and the bearer-writer move itself are likewise carried in the text. The material no-regression gap is narrower and sits in the C2 procedure/end-state carry-forward: the v4 explicit provider/token-cache forbidden set is NOT ancestry-aware, so a configured credential cache can still be swept into an allowlisted source subtree and receive the group ACL the invariant forbids. Same concrete case as falsifier_1: command_env CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude is allowed (supervision_lane_config.go:411-450) and survives the run-as filter (supervision_env.go:274-318); the resolver places .credentials.json there (resolver.go:78-83); the OQ4 mandatory grant recursively group-grants docs/ (HOLDER.md:350-376); A23 passes because the target is docs/, not a forbidden path, and its ancestor rule is only for <repoRoot>/ancestors of .striatum/ (HOLDER.md:436-443,:613). That violates the carried end-state invariant (HOLDER.md:378-382) — a provider credential path carries g:striatum-lanes access/default ACL after provisioning — and contradicts the OQ6 carry-forward sentence that the resolved credential cache path is in the OQ4 forbidden set so re-provision can never group-grant it (HOLDER.md:491-500). This is NOT a C1 regression and NOT a replay of the old raw-root setfacl -R <repoRoot> window; it is a planner-guard gap introduced by making the provider/cache set explicit without specifying ancestry semantics. C2 carry-forward is not intact until the spec requires ancestry-aware credential-cache protection (forbid resolving inside the repo / under any allowlisted source top-level, OR reject any recursive g:striatum-lanes op whose target is equal-to, a descendant-of, or an ANCESTOR-of any configured credential-cache path) AND tests it. needs_revision."
verdict: "needs_revision"
rationale: "Cycle-1 adjudication of the RFC 0168 P0 v4 REVISION falsification_gate trajectory: the revised v4 Holder (dialogue/holder/HOLDER.md, author holder-author-001) re-attacked by two falsifiers — falsifier_1 (falsifier-reviewer-001, C2 bearer-path realness lens) and falsifier_2 (falsifier-reviewer-002, no-regression lens); the cycle ends at adjudication with no further holder turn, so any landed challenge stands unrebutted. Inputs read: the revised Holder spec, both falsifier re-attacks, the v4 SEED.md charter + adjudicator role, RFC 0168, and — as required context for what the revision had to fix — the v3 SPEC (context/v3_HOLDER.md) and the v3 cycle-2 ledger (context/v3_LEDGER_cycle_2.md, its OQ4-ACL-PROVISIONING-TRANSITION open finding). No raw terminal output or private diagnostics were read as evidence. Every load-bearing source citation below was INDEPENDENTLY re-verified against the run-branch worktree HEAD 0c5e937c (the run branch has advanced past the f63b895f the holder cited; all cited sites are unchanged at HEAD). GATE CLEARING CONDITION (SEED + adjudicator role + packet objective): a clearing verdict (accept / accept_with_findings) REQUIRES ALL of — C2-RESIDUAL GENUINELY DISCHARGED (bearer writer move stated as a required build step anchored to mcpconfig.go:550-559, citation corrected, final scratch ACL launch-consistent, A22 a real transition test, the provider/token-cache forbidden set explicit, and no falsifier exhibiting a transient exposure window / fake test or guard / launch-breaking final ACL); C1-RESIDUAL STILL DISCHARGED; NO carry-forward regressed (HC-A1..A5, the C1 lease machine, the C2 procedure fix + A23 + the GROUP-ACL end-state invariant, OQ1/OQ3/OQ5/OQ6, the narrowing invariant); and NO standing material challenge. The v4 revision is a strong, genuine pass on the bearer-path SUB-PART of its charged residual, but a material, source-confirmed challenge against the provider/credential-cache SUB-PART of the SAME residual lands from BOTH falsifiers and stands unrebutted, so the gate does NOT clear. --- C2-RESIDUAL (OQ4-ACL-PROVISIONING-TRANSITION) — NOT GENUINELY DISCHARGED (verdict-driving). BEARER-PATH SUB-PART: DISCHARGED. I independently confirmed the live MCP bearer (the Authorization: Bearer-carrying lane-mcp-config-*.json) is created by writeEphemeralMCPConfig (go/pkg/agentloop/mcpconfig.go:550-580): dir := filepath.Join(repoRoot, .striatum, scratch) at :555 (ROOT, no supervisor id), os.Stat -> os.TempDir() fallback at :556-558, os.CreateTemp(dir, lane-mcp-config-*.json) at :559, chmod 0600 at :565 — it does not thread STRIATUM_SUPERVISOR_ID; and that mcpconfig.go:241 is os.WriteFile of the gemini settings.json (error string write gemini settings) and :266 is filepath.Join(repoRoot, .striatum, scratch, supervisorID) for the gemini settings.json.backup/.created markers — NOT the bearer. The v4 holder corrects the false v3 :241/266 citation to :550-559, states the re-root migration (move the primary dir to .striatum/scratch/<supervisor_id>/, preserve the tmp fallback, never target the scratch root, threading STRIATUM_SUPERVISOR_ID exactly as loop.go does) as a REQUIRED P0 build step ordered BEFORE re-keying the scratch ACLs, makes A22 derive-and-exercise the live resolved path with a no-residual-root assertion and a launch-positive control, and adds A24 for writer-realness. I confirmed scratch_acl.go:31-49 grants .striatum/scratch rwx+default EXACTLY because writeEphemeralMCPConfig CreateTemps the bearer there (#279), so the holder EACCES/#279 launch-break analysis is source-true AND the re-root genuinely removes the need for rwx on the scratch root, making the --x-only-scratch-root final ACL launch-consistent (the subdir is pre-created at supervision_control.go:115). The bearer-path realness sub-part — the SINGLE residual the v4 revision was primarily scoped to fix — is GENUINELY DISCHARGED, and falsifier_1 explicitly credits it. PROVIDER/CREDENTIAL-CACHE SUB-PART: NOT DISCHARGED — the standing material challenge. SEED required-fix #3 charged the holder to make the provider/token-cache forbidden set explicit SO THAT a re-provision cannot sweep a provider auth path into the source allowlist; falsifier_1 guidance asks directly: Can a provider auth path enter the source allowlist? Both falsifiers demonstrate the answer is YES, and I independently verified the entire chain at source: (1) validateLaneCommandEnvKey (supervision_lane_config.go:440-451) rejects only empty keys, PATH, and STRIATUM_*-prefixed keys — CLAUDE_CONFIG_DIR is allowed; (2) applyLaneLaunchEnv (supervision_env.go:110-113) merges command_env into the lane env; (3) sensitiveRunAsEnvKey (supervision_env.go:303-318) drops only STRIATUM_MCP_TOKEN/STRIATUM_MCP_TOKEN_FILE/DATABASE_URL/PGPASSWORD and keys CONTAINING TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN — CLAUDE_CONFIG_DIR contains none of those substrings, so it survives the run-as filter; (4) the Claude branch of the resolver (laneproviderauth/resolver.go:78-85) takes CLAUDE_CONFIG_DIR as authoritative and resolves the credential to filepath.Join(Clean(dir), .credentials.json) BEFORE falling back to HOME. So a lane launched with command_env CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude resolves its provider credential inside the repo. The holder OQ4.1(a) mandatory form (enumerate <repoRoot> top-level entries, prune the forbidden set, then setfacl -R -m g:striatum-lanes:rX -m d:g:striatum-lanes:rX each remaining source entry) rests on the explicit assumption (HOLDER.md OQ4.1a) that every forbidden path is a SIBLING of the source entries — never a descendant of one. That assumption is FALSE for a configured credential cache nested under an allowlisted source top-level: docs/ is not in the forbidden top-level name set {.striatum,.git,.gemini,.claude,.codex,credential-caches}, so docs/ survives the prune and the recursive setfacl -R sweeps a g:striatum-lanes access+default ACL onto docs/.lane-auth/claude/ and its .credentials.json (the default covers future hydrated credentials). A23 as written does NOT catch this: it rejects ops that TARGET <repoRoot> / an ancestor of .striatum/ as a raw recursive root, or that TARGET .striatum/.git/.gemini/.claude/.codex/a credential-cache path itself, or that set a d: entry on a dir with .striatum/ as a descendant — it has NO rule rejecting a recursive grant whose target is an ANCESTOR of a configured credential cache (when that ancestor, like docs/, is itself allowlisted). The op here targets docs/, which is none of those, so A23 PASSES. This is a real cross-lane provider-credential read: a different pool uid in striatum-lanes can read a provider .credentials.json that the OQ4 invariant (HOLDER.md: no path under .striatum/, nor .git/, nor any provider/token-cache path carries a g:striatum-lanes access OR default ACL — before, during, OR after provisioning) and the OQ6 carry-forward sentence (the resolved credential cache path is in the OQ4 forbidden set so a re-provision can never group-grant it) both say must never be group-readable. The holder text does NOT pin ancestry-aware credential-cache exclusion anywhere; the falsifiers steelman that intent in their Strongest Rebuttal sections and correctly conclude the falsifiable contract v4 actually pins (top-level name prune + recursive apply; A23 = exact-target rejection + .striatum/-only ancestor rule) permits a faithful build that updates the bearer writer, makes A22/A24 green, and STILL group-grants a nested CLAUDE_CONFIG_DIR cache. WHY GATE-STOPPING (needs_revision, not accept_with_findings): this is a soundness defect INSIDE the C2-RESIDUAL clearing requirement, not trackable post-clearance polish. A22/A23/A24/A16 can all be green while a configured credential cache is permanently group-exposed — the same green-test-over-the-wrong-path pattern the v3 ledger called gate-stopping, here for the provider-auth boundary instead of the bearer. The SEED required-fix #3 PURPOSE (so a re-provision cannot sweep a provider auth path into the source allowlist) is unmet; the packet clearing criterion the provider/token-cache forbidden set is explicit is satisfied only by NAME, not in substance; and the broad clearing criterion NO STANDING MATERIAL CHALLENGE is violated by a source-verified, unrebutted challenge from both falsifiers. C2-RESIDUAL NOT DISCHARGED. --- C1-RESIDUAL (OQ2-SCRUB-POSTCONDITION) — STILL DISCHARGED. The bearer-path fix does not touch the scrub path; OQ2 is carried verbatim from the v3-DISCHARGED form. P1 is the fail-closed three-way classifyPoolUIDTaskState predicate (tolerate only true zombie/dead /proc state in {Z,X,x} or a fully-vanished PID; BLOCK every other observed state incl. T/t, any unrecognized char, and an unreadable/ambiguous /proc read for a still-present PID -> quarantined + typed lane_uid_scrub_failed), explicitly NOT the binary processZombie; /proc evidence is recorded; A21 is intact. falsifier_2 attempted and could not construct a non-zombie pool_uid survivor that reaches returned; I concur — no non-zombie survivor can re-lease. C1-RESIDUAL DISCHARGED, not regressed. --- CARRY-FORWARDS — INTACT. HC-A1..A5 INTACT (carried verbatim from v1/v2/v3 Part 1; the bearer-path fix does not touch the launch path or the structural no-replay claim; both falsifiers re-checked and found no regression). The C1 durable four-state lease machine + uq_lane_uid_held + 3-transaction boundary + reaper + quarantine-survives-restart + dirty-excluding exhaustion INTACT. OQ1/OQ3/OQ5/OQ6 INTACT. The narrowing invariant INTACT — no admin-token widening, no lane-readable shared reseal bearer; the bearer-path fix is a narrowing (a 0600 file moves deeper into the lane per-supervisor private dir and the shared-scratch-root rwx grant is removed). The C2 PROCEDURE fix (mandatory allowlist / exclude-at-traversal form; raw setfacl -R <repoRoot>-then-strip explicitly forbidden) and the A23 RAW-RECURSIVE-ROOT case INTACT, and the GROUP-ACL end-state invariant for .striatum/ / .git/ (top-level siblings, with A23 ancestor coverage for .striatum/) INTACT. The single gap is the NEWLY-EXPLICIT provider/credential-cache EXTENSION of that invariant (SEED fix #3), which is not ancestry-enforceable under the v4 form — classified as the OPEN OQ4-ACL-PROVISIONING-TRANSITION item, NOT a regression of a carried claim (matching falsifier_2's own framing: a planner-guard gap introduced by making the provider/cache set explicit without ancestry semantics, not a C1 regression and not the old raw-root window). --- WHY NOT REJECT: C1-RESIDUAL is genuinely discharged; the C2 bearer-path realness (the primary scoped residual) is genuinely discharged; the C2 procedure/raw-recursive-root half is correct; no path widens admin-token exposure or mints a lane-readable shared reseal bearer; both falsifiers confirm the no-widening invariant and the proven hard core; and the direction (per-lane pooled OS uid) is maintainer-ratified (D261, 2026-06-24) and not relitigated. The remaining correction is narrow and precisely specified. WHY NOT ACCEPT / ACCEPT_WITH_FINDINGS: a standing, source-verified, unrebutted material challenge exists; the GROUP-ACL invariant is demonstrably violable by a faithful build; the defect is inside the C2-RESIDUAL clearing requirement, not post-clearance. --- PER-RESIDUAL RECORD: C1-RESIDUAL DISCHARGED; C2-RESIDUAL NOT DISCHARGED (bearer-path realness sub-part DISCHARGED — writer move is a required build step anchored to mcpconfig.go:550-559, citation corrected, final ACL launch-consistent, A22 real, A24 added; provider/credential-cache ancestry sub-part OPEN — a nested CLAUDE_CONFIG_DIR cache is swept into the source allowlist by the top-level-name prune + recursive apply, A23 does not reject an ancestor-of-cache target). CARRY-FORWARDS INTACT: HC-A1..A5 INTACT, C1 lease machine INTACT, C2 procedure fix + A23 raw-root case INTACT, GROUP-ACL end-state invariant (.striatum//.git/ core) INTACT, OQ1 INTACT, OQ3 INTACT, OQ5 INTACT, OQ6 INTACT, narrowing INTACT. FALSIFIER DISPOSITIONS: falsifier_1 (reviewer-001) — bearer-path challenge REBUTTED-by-spec (the v4 holder fixed it, falsifier credits it); its provider/credential-cache material challenge STANDS (landed_unrebutted). falsifier_2 (reviewer-002) — C1 + carry-forward no-regression challenges find nothing (rebutted-by-spec); its provider/credential-cache material challenge STANDS (landed_unrebutted). --- VERDICT: needs_revision. GATE-CYCLE CONSEQUENCE: per the SEED and adjudicator role this is v4's single proper revision round; this needs_revision EXHAUSTS the v4 gate UNCLEARED and routes the remaining residual to the OPERATOR — do NOT ratchet into another holder cycle. The committer does NOT publish the consolidated SPEC, and the operator does NOT ratify D272 on this verdict (D271 remains reserved by the concurrent RFC 0170 P0 design; D272 is RFC 0168's free reservation). The remaining fix is small and source-anchored: make the provider/token-cache forbidden set ANCESTRY-AWARE — either forbid provider credential/cache directories from resolving inside the repository or beneath any group-ACL-allowlisted source top-level, OR extend A23 / the planner to reject any recursive g:striatum-lanes op whose target is equal-to, a descendant-of, or an ANCESTOR-of any configured credential-cache path (the same ancestor semantics A23 already gives .striatum/), OR walk allowlisted source trees with exclude-at-traversal that prunes forbidden descendants before any grant; and add the concrete test (set CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude, provision ACLs, assert no g:striatum-lanes access/default ACL on that directory or its .credentials.json before/during/after). The maintainer-ratified direction (per-lane pooled OS uid, D261) carries regardless — adjudicator clearance gates the spec's soundness, not the product call."
findings:
  - id: HC-ORACLE-INTACT
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "structural no-replay: a per-lane uid dissolves the BC1-W1-ORACLE same-uid tmux/0600/ptrace replay class on this host (Yama ptrace_scope=1) — CARRIED FORWARD UNREGRESSED from v1/v2/v3"
    challenge: "INTACT (carry-forward). The v1-proven hard core HC-A1..A5 (per-uid 0700 tmux socket so a sibling cannot respawn the target pane; cross-uid 0600 DAC — the surface that makes RFC 0143 Slice B's reseal token safe; cross-uid ptrace/setns//proc denial by ptrace_may_access — the exact axis namespace-inode failed under D261; SO_PEERCRED uid discriminator; every residual same-uid surface closed) is carried into v4 verbatim (HOLDER Part 1). The v4 bearer-path fix touches neither the launch path nor the structural claim. Both falsifiers independently re-checked and found NO regression. Not reopened, not regressed."
  - id: OQ2-SCRUB-POSTCONDITION
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:3"]
    affected_invariants:
      - "no cross-lease same-uid residue: a returned uid must be PROVABLY empty before re-lease — the scrub postcondition must prove zero non-zombie uid-owned tasks (C1-RESIDUAL), DISCHARGED in v3 and carried unregressed in v4"
    challenge: "DISCHARGED and CARRIED unregressed. P1 is the fail-closed three-way classifyPoolUIDTaskState predicate (tolerate only true zombie/dead /proc state in {Z,X,x} or a fully-vanished PID; BLOCK every other observed state incl. T (stopped) / t (tracing-stop), any unrecognized char, AND an unreadable/ambiguous /proc read for a still-present PID -> quarantined + typed lane_uid_scrub_failed), explicitly NOT the binary processZombie (tmux_liveness.go:599-614, which returns true only for Z and false on a read error). /proc evidence (observed PIDs + state chars + classification) recorded in scrub_proof (pass) and scrub_failure (block); A21 TestStoppedOrTracedUIDProcessBlocksReturn intact. The bearer-path fix (OQ4.0) does not touch the scrub path. falsifier_2 attempted the non-zombie-survivor refuter and could not construct a non-zombie pool_uid task that reaches returned; I concur. C1-RESIDUAL STILL DISCHARGED, no regression."
  - id: OQ4-ACL-PROVISIONING-TRANSITION
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "ACL exactly-enough without over-grant: NO path under .striatum/, nor .git/, nor any provider/token-cache path (.gemini/.claude/.codex/a configured credential cache such as a CLAUDE_CONFIG_DIR resolved inside the repo) may carry a g:striatum-lanes access OR default ACL entry — before, during, OR after provisioning; AND the C2 transition test (A22) must exercise the REAL live bearer path; AND the final per-supervisor scratch ACL must not break the lane launch path"
    closest_acceptable_answer: "Bearer-path sub-part is DISCHARGED — no change needed there. For the open provider/credential-cache ancestry sub-part, the spec must make the forbidden set ANCESTRY-AWARE: (a) forbid provider credential/cache directories from resolving inside the repository or beneath any group-ACL-allowlisted source top-level, OR (b) extend A23 (and the planner) to reject any recursive g:striatum-lanes op whose target is equal-to, a descendant-of, OR an ANCESTOR-of any configured credential-cache path — the same ancestor semantics A23 already grants .striatum/ — OR (c) walk allowlisted source trees with exclude-at-traversal that prunes forbidden descendants before applying any grant; AND add a concrete test that sets CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude, provisions ACLs, and asserts no g:striatum-lanes access/default ACL on that directory or its .credentials.json before/during/after. Routed to the operator (single v4 cycle exhausted)."
    challenge: "OPEN — verdict-driving. C2-RESIDUAL NOT genuinely discharged. BEARER-PATH SUB-PART DISCHARGED: independently verified the live bearer is writeEphemeralMCPConfig (mcpconfig.go:550-580; dir=.striatum/scratch ROOT at :555, os.TempDir fallback :556-558, CreateTemp :559, chmod 0600 :565, no STRIATUM_SUPERVISOR_ID), and that :241/266 are the gemini settings.json write + backup/created markers, not the bearer — so the holder citation correction to :550-559 is source-true; the re-root migration is stated as a required P0 build step ordered before re-keying the scratch ACLs; A22 is made real (derives the live resolved path, asserts no residual root-level bearer, launch-positive control); A24 added; scratch_acl.go:31-49 confirms the #279 rwx-on-scratch-root grant exists precisely because the writer CreateTemps there, so the re-root makes the --x-only-root final ACL launch-consistent. PROVIDER/CREDENTIAL-CACHE SUB-PART OPEN (both falsifiers, source-verified, unrebutted): the SEED required the explicit forbidden set so a re-provision cannot sweep a provider auth path into the source allowlist, but it still can. Verified chain: command_env CLAUDE_CONFIG_DIR is allowed (validateLaneCommandEnvKey bars only PATH/STRIATUM_*, supervision_lane_config.go:440-451), merged into the lane env (supervision_env.go:110-113), survives the run-as filter (sensitiveRunAsEnvKey drops only TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN substrings + a small denylist; CLAUDE_CONFIG_DIR matches none, supervision_env.go:303-318), and is taken authoritatively by resolver.go:78-85 to place .credentials.json inside it. The OQ4.1(a) mandatory form (top-level prune + recursive setfacl -R on each remaining source entry) rests on the false assumption that every forbidden path is a sibling never a descendant of a source entry: a CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude nests the cache UNDER allowlisted docs/, so the recursive grant on docs/ sweeps a g:striatum-lanes access+default ACL onto docs/.lane-auth/claude/.credentials.json. A23 passes because its target is docs/ (not a forbidden path) and its only ancestor rule covers <repoRoot>/ancestors of .striatum/ — there is no ancestor-of-credential-cache rule. This violates the OQ4 end-state invariant (HOLDER.md:378-382) and the OQ6 carry-forward sentence (HOLDER.md:491-500). A real cross-lane provider-credential read with A22/A23/A24/A16 all green — a soundness defect inside the C2-RESIDUAL gate, not post-clearance polish. needs_revision."
  - id: OQ-CARRY-FORWARD-INTACT
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "the v1/v2/v3-credited carried set into v4 unregressed: the C1 durable four-state lease machine, the C2 procedure fix + A23 raw-recursive-root prohibition, the .striatum//.git/-excluding GROUP-ACL end-state invariant, OQ1 sizing + fail-closed exhaustion, OQ3 launch-as-only provisioning, OQ5 generation token, OQ6 hydration shape, restart-survival, and the narrowing invariant"
    challenge: "INTACT (carry-forward, no regression from the bearer-path fix). Both falsifiers report NO regression and I concur. C1 lease machine: four-state active/scrubbing/quarantined/returned, partial held-unique index, three-transaction allocate/scrub-begin/scrub-finalize boundary, failed-proof-quarantines, leaked-active + stuck-scrubbing reaper, quarantine-survives-restart, dirty-excluding exhaustion — all carried. C2 procedure fix: the mandatory allowlist / exclude-at-traversal form and the explicit raw setfacl -R <repoRoot>-then-strip prohibition are carried; A23's raw-recursive-root case is INTACT and the right shape; the GROUP-ACL end-state invariant is INTACT for .striatum/ / .git/ (top-level siblings, with A23 ancestor coverage for .striatum/). OQ1: host-global ceiling + typed fail-closed lane_uid_pool_exhausted, v1 caveat stays closed (A20). OQ3: static host-runbook pool, daemon holds only the launch-as %striatum-lanes grant (A12). OQ5: leased-uid + monotonic per-uid generation token compared on every attestation AND control-frame path (A14). OQ6: per-spawn per-uid hydration into the leased uid's 0600 store scrubbed on return, contingency closed by P3 (A15) — NOTE its sentence that the resolved credential cache path is in the OQ4 forbidden set so a re-provision can never group-grant it is the very claim the OPEN OQ4 finding shows is not ancestry-enforceable. Restart-survival: free set DERIVED from PostgreSQL (A8'/A11'/A19). NARROWING: the bearer-path fix only removes surface; no new authority, no admin-token widening, no lane-readable shared reseal bearer. The provider/credential-cache ancestry gap is the NEWLY-EXPLICIT extension of the GROUP-ACL invariant (SEED fix #3), classified under the OPEN OQ4-ACL-PROVISIONING-TRANSITION finding — NOT a regression of a carried claim. Do NOT reopen or regress the carried set."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0168 P0 design (v4 REVISION, cycle 1)

author: adjudicator-author-001

> Cycle-1 adjudication of the RFC 0168 P0 **v4 REVISION** `falsification_gate`
> dialogue trajectory (*per-lane OS uid as the lane security principal*). The v4
> revision was charged to discharge the **single** standing v3 cycle-2 residual —
> **C2-RESIDUAL** (`OQ4-ACL-PROVISIONING-TRANSITION`, the per-supervisor
> MCP-bearer-path realness) — while carrying everything v1/v2/v3 cleared forward
> unregressed, including the v3-DISCHARGED **C1-RESIDUAL**. Inputs read: the revised
> Holder spec (`dialogue/holder/HOLDER.md`), both falsifier re-attacks
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`), the v4
> `SEED.md` charter + adjudicator role, RFC 0168, and — as required context — the v3
> SPEC (`context/v3_HOLDER.md`) and the v3 cycle-2 ledger
> (`context/v3_LEDGER_cycle_2.md`). No raw terminal output or private diagnostics
> were read as evidence. **Every load-bearing source citation below was
> independently re-verified against the run-branch worktree HEAD `0c5e937c`** (the
> branch has advanced past the `f63b895f` the holder cited; the cited sites are
> unchanged). The direction (a pre-provisioned pool of per-lane uids, leased per
> lane) is maintainer-ratified (**D261**, 2026-06-24) and is **not** relitigated.

## Verdict

**verdict: needs_revision**

The v4 revision **genuinely discharges the bearer-path sub-part of its charged
residual**, but a **material, source-confirmed** challenge against the
**provider/credential-cache sub-part of the same residual** lands from **both**
falsifiers and stands **unrebutted** (no further holder turn), so the gate does
**not** clear. A clearing verdict requires C2-RESIDUAL **genuinely discharged in
full**, C1-RESIDUAL still discharged, no carry-forward regressed, and **no standing
material challenge**.

- **C2-RESIDUAL (`OQ4-ACL-PROVISIONING-TRANSITION`) — NOT DISCHARGED.**
  - **Bearer-path realness — DISCHARGED.** I independently verified the live bearer
    is `writeEphemeralMCPConfig` (`mcpconfig.go:550-580`, `.striatum/scratch` **root**
    at `:555`, no `STRIATUM_SUPERVISOR_ID`) and that `:241/266` are the **gemini**
    `settings.json` write + backup/created markers — so the holder's citation
    correction to `:550-559` is **source-true**. The re-root migration is stated as a
    **required P0 build step** ordered before re-keying the scratch ACLs; **A22** is
    made real (derives the live resolved path, asserts no residual root-level bearer,
    launch-positive control); **A24** added. `scratch_acl.go:31-49` confirms the `#279`
    `rwx`-on-scratch-root grant exists *because* the writer `CreateTemp`s there, so the
    re-root makes the `--x`-only-root final ACL **launch-consistent**. `falsifier_1`
    credits this.
  - **Provider/credential-cache forbidden set — OPEN (the standing challenge).** The
    set is explicit by **name** but **not ancestry-aware**. I verified the full
    source chain: `command_env CLAUDE_CONFIG_DIR` is allowed
    (`supervision_lane_config.go:440-451`), merged into the lane env
    (`supervision_env.go:110-113`), survives the run-as filter
    (`supervision_env.go:303-318` — `CLAUDE_CONFIG_DIR` contains none of the dropped
    substrings), and is taken authoritatively by `resolver.go:78-85` to place
    `.credentials.json` inside it. A `CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`
    nests the cache under **allowlisted** `docs/`, so the OQ4.1(a) mandatory form
    (top-level-name prune + recursive `setfacl -R`) group-grants `docs/` and sweeps the
    access+default ACL onto `.credentials.json`. **A23** passes (its target is `docs/`,
    not a forbidden path; its only ancestor rule covers `.striatum/`). This is a real
    cross-lane provider-credential read that **violates** the OQ4 end-state invariant
    and the OQ6 carry-forward sentence, with `A22/A23/A24/A16` all green — a soundness
    defect **inside** the C2-RESIDUAL gate, **not** post-clearance polish.

- **C1-RESIDUAL (`OQ2-SCRUB-POSTCONDITION`) — STILL DISCHARGED.** P1 is the
  fail-closed three-way `classifyPoolUIDTaskState` predicate (not `processZombie`);
  `T`/`t`/unknown/unreadable-still-present block `returned`; `/proc` evidence
  recorded; A21 intact. The bearer-path fix doesn't touch the scrub path.
  `falsifier_2` could not construct a non-zombie `pool_uid` survivor that re-leases;
  I concur.

- **Carry-forwards — INTACT.** HC-A1..A5, the C1 four-state lease machine, the C2
  procedure fix + A23 raw-recursive-root case, the GROUP-ACL end-state invariant for
  `.striatum/`/`.git/`, OQ1/OQ3/OQ5/OQ6, and the narrowing invariant are all carried
  unregressed. The provider/credential-cache ancestry gap is the **newly-explicit
  extension** of the GROUP-ACL invariant (SEED fix #3) — classified under the **open**
  OQ4 finding, **not** a regression of a carried claim (matching `falsifier_2`'s own
  framing). No admin-token widening, no lane-readable shared reseal bearer.

`needs_revision`, **not** `reject` (C1-RESIDUAL discharged, the C2 bearer-path
realness and procedure halves correct, the direction D261-ratified, the fix narrow
and precise) and **not** `accept`/`accept_with_findings` (a standing, source-verified,
unrebutted material challenge; the GROUP-ACL invariant is demonstrably violable by a
faithful build; the defect sits inside the C2-RESIDUAL clearing requirement).

## Per-residual / per-carry-forward / per-falsifier record

- **C1-RESIDUAL** — **DISCHARGED**.
- **C2-RESIDUAL** — **NOT DISCHARGED**: bearer-path realness sub-part **DISCHARGED**
  (writer move a required build step anchored to `mcpconfig.go:550-559`, citation
  corrected, final ACL launch-consistent, A22 real, A24 added); provider/credential-cache
  ancestry sub-part **OPEN** (a nested `CLAUDE_CONFIG_DIR` cache is swept into the source
  allowlist; A23 does not reject an ancestor-of-cache target).
- **Carry-forwards** — **INTACT**: HC-A1..A5; C1 lease machine; C2 procedure fix + A23
  raw-root case; GROUP-ACL end-state invariant (`.striatum/`/`.git/` core); OQ1; OQ3; OQ5;
  OQ6; narrowing.
- **`falsifier_1` (reviewer-001)** — bearer-path challenge **REBUTTED-by-spec**; its
  provider/credential-cache material challenge **STANDS** (`landed_unrebutted`).
- **`falsifier_2` (reviewer-002)** — C1 + carry-forward no-regression challenges find
  nothing (**rebutted-by-spec**); its provider/credential-cache material challenge
  **STANDS** (`landed_unrebutted`).

## Gate-cycle consequence

This was the **single allowed v4 revision cycle**. A `needs_revision` here
**exhausts the gate uncleared** and routes the residual to the **operator** — do
**not** ratchet into another holder cycle. The committer does **not** publish the
consolidated SPEC, and the operator does **not** ratify **D272** on this verdict
(**D271** remains reserved by the concurrent RFC 0170 P0 design; D272 is RFC 0168's
free reservation). The remaining correction is small and source-anchored: make the
provider/token-cache forbidden set **ancestry-aware** — forbid provider credential/cache
directories from resolving inside the repository or beneath any group-ACL-allowlisted
source top-level, **or** extend A23/the planner to reject any recursive
`g:striatum-lanes` op whose target is equal-to, a descendant-of, or an **ancestor-of**
any configured credential-cache path (the same ancestor semantics A23 already gives
`.striatum/`), **or** walk allowlisted source trees with exclude-at-traversal that prunes
forbidden descendants before any grant; **and** add the concrete test
(`CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`, provision ACLs, assert no
`g:striatum-lanes` access/default ACL on that directory or its `.credentials.json`
before/during/after). The maintainer-ratified direction (per-lane pooled OS uid, D261)
carries regardless — adjudicator clearance gates the spec's **soundness**, not the
product call.

<sub>Adjudicator collaboration ledger — RFC 0168 P0 `falsification_gate` design run,
**REVISION v4, cycle 1**. **C2-RESIDUAL NOT DISCHARGED**: the bearer-path realness
sub-part is genuinely discharged (the live bearer writer `writeEphemeralMCPConfig`
`mcpconfig.go:550-559` re-rooted under `.striatum/scratch/<supervisor_id>/` as a required
build step, the false `:241/266` citation corrected, A22 made real + A24 added, the final
ACL launch-consistent via the `#279` re-root), but a material, source-verified, unrebutted
challenge from **both** falsifiers stands against the provider/credential-cache sub-part:
the explicit forbidden set is name-based not ancestry-aware, so a `command_env`-set
`CLAUDE_CONFIG_DIR` nested under an allowlisted source top-level (`docs/`) is swept into
the `g:striatum-lanes` source grant by the top-level-prune + recursive `setfacl -R` form,
and A23 (exact-target + `.striatum/`-only ancestor rule) does not catch it — a real
cross-lane provider-credential read violating the OQ4 end-state invariant and the OQ6
carry-forward sentence. **C1-RESIDUAL STILL DISCHARGED** (P1 -> `classifyPoolUIDTaskState`,
A21, `/proc` evidence). Carry-forwards INTACT (HC-A1..A5, the C1 lease machine, the C2
procedure fix + A23 raw-root case, the GROUP-ACL end-state `.striatum/`/`.git/` core,
OQ1/OQ3/OQ5/OQ6, narrowing). `needs_revision` — single v4 cycle exhausted; the provider/
credential-cache ancestry residual (make the forbidden set ancestry-aware + add the
`CLAUDE_CONFIG_DIR`-under-`docs/` test) routes to the operator; the committer does not
publish the consolidated SPEC and the operator does not ratify D272. D261 carries
regardless.</sub>
