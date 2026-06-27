---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: |-
  RFC 0168 P0 — per-lane OS uid as the lane security principal: v5 REVISION re-attack (cycle 1) — discharge the SINGLE standing v4 residual, the provider/credential-cache ANCESTRY sub-part of OQ4-ACL-PROVISIONING-TRANSITION, by making the forbidden set ancestry-enforced at three chokepoints (launch-time resolution-domain ban OQ4.1.1, planner/A23 ancestor rule, physical no-symlink-follow apply OQ4.1(c)) plus A25, while carrying the v1-proven hard core HC-A1..A5, the v3-DISCHARGED C1-RESIDUAL (classifyPoolUIDTaskState P1 + A21), the v4-DISCHARGED C2 bearer-path sub-part (re-rooted writeEphemeralMCPConfig + real A22 + A24), the C1 four-state lease machine, the C2 procedure fix + A23 + the GROUP-ACL end-state invariant, and OQ1/OQ3/OQ5/OQ6 + the narrowing invariant forward unregressed
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
    text: |-
      v5 REVISION of the RFC 0168 P0 falsification_gate spec, scoped to discharge the SINGLE standing v4 cycle-1 residual — the provider/credential-cache ANCESTRY sub-part of OQ4-ACL-PROVISIONING-TRANSITION — while carrying forward unregressed everything v1/v2/v3/v4 cleared (the v3-DISCHARGED C1-RESIDUAL and the v4-DISCHARGED C2 bearer-path fix kept verbatim in substance). The fix replaces v4's explicit-by-name forbidden set with ONE coherent ancestry invariant (no provider credential cache is ever reachable by a g:striatum-lanes ACL) enforced at three coordinated chokepoints: (1) OQ4.1.1, a NEW launch-time resolution-domain ban that resolves every provider credential/cache directory the resolver would derive from the launch env and REFUSES the launch fail-closed (typed lane_credential_cache_inside_repo, job queued/recoverable) when it resolves inside <repoRoot> — load-bearing because the per-lane command_env cache path is known only at launch while the group source ACL is provisioned repo-globally (provisionCommitteeACLs, repo_acl.go:97-140); (2) the pure ACL planner/A23 extended to reject any g:striatum-lanes op equal-to / descendant-of / ANCESTOR-of any statically-configured credential-cache path (the same ancestor semantics A23 already gives .striatum/); (3) OQ4.1(c), a physical (no-symlink-follow) recursive apply (setfacl -RP) so the grant never escapes its planned in-repo source targets. Adds A25 TestProviderCredentialCacheNeverInGroupACLDomain exercising the live env chain (validateLaneCommandEnvKey -> applyLaneLaunchEnv -> sensitiveRunAsEnvKey -> ResolveCredential) over the ledger's exact CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude case plus symlink and created-after-provisioning probes; updates the OQ4 invariant + OQ6 carry-forward sentence to ancestry enforcement. Build-run target: runtime migration 0046+ (0045 reserved by RFC 0170 P0; not hardcoded) + owner-bundle bump owner/0023+; the provider-ancestry fix is Go-only (no schema). Re-verified against run-branch worktree HEAD 621312c4.
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: |-
      Provider/credential-cache ancestry lens — LANDS, unrebutted. The exact prescribed case (CLAUDE_CONFIG_DIR nested under docs/) is genuinely closed by OQ4.1.1, but the discharge mechanism is only as complete as Striatum's resolver roster, and the real Claude runtime reads a credential selector the resolver does NOT model. Counterexample: command_env CLAUDE_SECURESTORAGE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/secure (with CLAUDE_CONFIG_DIR unset or set outside the repo). This key is admitted by validateLaneCommandEnvKey (bars only empty/PATH/STRIATUM_*, supervision_lane_config.go:440-451), merged by applyLaneLaunchEnv (supervision_env.go:110-118), and survives sensitiveRunAsEnvKey (drops only TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN substrings + a small denylist; CLAUDE_SECURESTORAGE_CONFIG_DIR contains none, supervision_env.go:303-318). But OQ4.1.1's guard inspects only the directory parents of ResolveCredential's result, and resolver.go:24-32 declares only CODEX_HOME, CLAUDE_CONFIG_DIR, and HOME — its Claude branch checks CLAUDE_CONFIG_DIR then falls back to HOME/.claude (resolver.go:78-92); it never consults CLAUDE_SECURESTORAGE_CONFIG_DIR. So the launch passes the inside-repo ban while the provider process reads <repoRoot>/docs/.lane-auth/secure/.credentials.json. The OQ4.1(a) mandatory grant still group-grants docs/ (access + default ACL), so the nested secure store is swept (or, if created after provisioning, inherits the default group ACL — a local POSIX ACL check with the current primary group as a stand-in showed a late-created docs/.lane-auth/secure/.credentials.json carrying group:<group>:r-x #effective:r-- after setfacl -RP -m g:<group>:rX -m d:g:<group>:rX docs). A23 does not catch it (per-lane command_env path, not statically configured). Physical -RP does not help (a real descendant of docs/, not a symlink escape). A25 as written stays GREEN because it tests CLAUDE_CONFIG_DIR through the same incomplete resolver. Evidence the missed path is a real Claude cache, not an env-name guess: claude --version reports 2.1.195, strings of the CLI contain CLAUDE_SECURESTORAGE_CONFIG_DIR/.credentials.json/claudeAiOauth, and an empty-env strace shows the CLI open secure/.credentials.json. Strongest-rebuttal pre-empt: holder could say future provider config-dir env keys must be added to the resolver roster — not enough for this gate, because the current Claude runtime already has another selector TODAY and A25 derives from the same incomplete resolver, so the test can be green while a nested cache under docs/ carries a group ACL. The ancestry residual remains OPEN until the build models/tests CLAUDE_SECURESTORAGE_CONFIG_DIR alongside CLAUDE_CONFIG_DIR or fails closed on any provider-specific credential selector in command_env not covered by the resolver.
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: not_material
    text: |-
      No-regression lens — no independent carry-forward regression found. The falsifier tried and could not construct a concrete carried-claim regression. (1) Bearer path / A22 / launch consistency: the v5 text keeps the v4 bearer-path discharge as a required build step (writeEphemeralMCPConfig moved from .striatum/scratch root to .striatum/scratch/<supervisor_id>/, never falling back to scratch root, landing before scratch is pushed to traverse-only; A22 still derives the path from the live writer, asserts no residual root-level bearer, runs before/during/after cross-uid reads with S1's positive CreateTemp control) — the old root-bearer / #279 EACCES failure cannot be replayed. (2) C1 scrub classifier: OQ2 is still the fail-closed P1 wired to classifyPoolUIDTaskState, not processZombie; T/t, unknown state, and unreadable-still-present PIDs still block returned; /proc evidence recorded; the four-state machine and uq_lane_uid_held preserved — no non-zombie pool_uid survivor reaches returned. (3) C2 procedure / A23 / group-ACL end state: the raw setfacl -R <repoRoot> path stays forbidden; the mandatory prune-then-apply form is intact, now physical (-P); A23 still rejects a raw recursive root targeting <repoRoot>/an ancestor of .striatum/ and targets under .striatum/.git/.gemini/.claude/.codex; the new ancestor-of-cache rule is additional restriction, not a weakening. (4) HC-A1..A5, OQ1/OQ3/OQ5/OQ6, restart reconstruction, quarantine survival, and the narrowing invariant are restated, not loosened; no admin-token widening, no lane-readable shared reseal bearer; OQ6's credential-cache sentence is re-backed by the ancestry mechanism. The falsifier explicitly defers Falsifier 1's CLAUDE_SECURESTORAGE_CONFIG_DIR challenge to the adjudicator as a completeness challenge to the NEW fix, not a separate no-regression failure under this lens. NOT MATERIAL as an independent carry-forward regression.
verdict: "needs_revision"
rationale: |-
  Cycle-1 adjudication of the RFC 0168 P0 v5 REVISION falsification_gate trajectory: the revised v5 Holder (dialogue/holder/HOLDER.md, holder-author-001) was re-attacked by two falsifiers — falsifier_1 (falsifier-reviewer-001, provider/credential-cache ancestry lens) and falsifier_2 (falsifier-reviewer-002, no-regression lens). This is a falsification_gate with no holder rebuttal turn, so any landed challenge stands unrebutted. Inputs read: the revised Holder spec, both falsifier re-attacks, the v5 SEED.md charter + adjudicator role, RFC 0168, and — as required context for what the revision had to fix — the v4 SPEC (context/v4_HOLDER.md) and the v4 cycle-1 ledger (context/v4_LEDGER_cycle_1.md, its OPEN OQ4 provider/credential-cache ancestry finding + closest_acceptable_answer). No raw provider logs or private workflow diagnostics were read as workflow-state evidence. Every load-bearing source citation below was INDEPENDENTLY re-verified against the run-branch worktree HEAD 5fd0c81e (the branch has advanced past the 621312c4 the holder cited; all cited sites are unchanged at HEAD). GATE CLEARING CONDITION (SEED + adjudicator role + packet objective): a clearing verdict (accept / accept_with_findings) REQUIRES ALL of — (PROVIDER-ANCESTRY DISCHARGED) the forbidden set is genuinely ancestry-aware so no g:striatum-lanes access OR default ACL can land on a credential cache nested under an allowlisted source top-level, via an enforced mechanism with a real concrete test, and NO falsifier exhibits a nested cache that still receives a group ACL; (BEARER-PATH STILL DISCHARGED) the writer move + real A22 intact; (C1-RESIDUAL STILL DISCHARGED) P1 = classifyPoolUIDTaskState, A21, /proc evidence; (NO CARRY-FORWARD REGRESSED); and NO standing material challenge. The v5 revision implements the v4-prescribed mechanism faithfully and closes the exact prescribed case, but a material, source-confirmed, host-corroborated challenge against the SAME provider/credential-cache ancestry residual LANDS from falsifier_1 and STANDS unrebutted, so the gate does NOT clear. --- OQ4 PROVIDER/CREDENTIAL-CACHE ANCESTRY SUB-PART — NOT GENUINELY DISCHARGED (verdict-driving). The holder did implement all three prescribed chokepoints and A25, and the v4 ledger's exact case (CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude) IS genuinely closed by OQ4.1.1 (the resolver derives that path and the launch is refused fail-closed). The residual remains OPEN because the mechanism is ALLOWLIST-scoped (the protected set = the directory parents of ResolveCredential's result) rather than FAIL-CLOSED against unmodeled provider credential selectors — and the live Claude runtime has one TODAY. I independently verified the full chain at HEAD 5fd0c81e: (1) validateLaneCommandEnvKey (go/pkg/mutations/supervision_lane_config.go) rejects only an empty key, PATH, and STRIATUM_*-prefixed keys — CLAUDE_SECURESTORAGE_CONFIG_DIR is ADMITTED; (2) applyLaneLaunchEnv (supervision_env.go:110-118) merges command_env into the lane env; (3) sensitiveRunAsEnvKey (supervision_env.go:303-318) drops STRIATUM_MCP_TOKEN/STRIATUM_MCP_TOKEN_FILE/DATABASE_URL/PGPASSWORD and keys CONTAINING TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN — CLAUDE_SECURESTORAGE_CONFIG_DIR contains none of those substrings, so it SURVIVES the run-as filter; (4) ResolveCredential (go/pkg/laneproviderauth/resolver.go) models ONLY CODEX_HOME, CLAUDE_CONFIG_DIR, and HOME (const block resolver.go:24-32; Claude branch resolver.go:78-92 checks CLAUDE_CONFIG_DIR then HOME/.claude) — it NEVER consults CLAUDE_SECURESTORAGE_CONFIG_DIR, and a repo-wide grep confirms ZERO occurrences of SECURESTORAGE anywhere in go/. So OQ4.1.1's guard, defined as the parents of ResolveCredential's result, provably cannot inspect the secure-storage cache directory; a lane launched with command_env CLAUDE_SECURESTORAGE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/secure (CLAUDE_CONFIG_DIR unset/outside) resolves only HOME/.claude (outside the repo, passes the ban) while the provider process reads the in-repo secure cache. The OQ4.1(a) mandatory form group-grants docs/ with an access + DEFAULT ACL, so the nested secure store is swept if present and INHERITS the default group ACL if created later (standard POSIX default-ACL semantics; the falsifier's local stand-in repro showed group:<group>:r-x #effective:r-- on a late-created docs/.lane-auth/secure/.credentials.json). A23 does not catch it (a per-lane command_env path, invisible to the static planner — the holder's own text concedes this). Physical -RP does not catch it (a real descendant of docs/, not a symlink escape). A25 stays GREEN because it drives CLAUDE_CONFIG_DIR through the same incomplete resolver. The provider-side materiality — that the Claude CLI actually reads this key — is corroborated by the installed binary's own static strings (claude 2.1.195 contains both CLAUDE_CONFIG_DIR and CLAUDE_SECURESTORAGE_CONFIG_DIR), a public fact about the installed CLI, not a private diagnostic. Result: a real cross-lane provider-credential read (a sibling pool uid in striatum-lanes gets r-x on the cache dir and r-- on .credentials.json) — exactly the BC1 surface RFC 0168 exists to close — while A22/A23/A24/A25/A16 can all be green. WHY GATE-STOPPING (needs_revision, not accept_with_findings): this is a SOUNDNESS defect INSIDE the OQ4 ancestry residual's clearing requirement, not trackable post-clearance polish. It is the same green-test-over-the-wrong-path pattern the v3/v4 ledgers called gate-stopping, here one level deeper: the discharge rests on an enumerated resolver roster that is incomplete relative to the real provider. The packet clearing criterion no falsifier exhibits a nested cache that still receives a group ACL is VIOLATED by falsifier_1's exhibit, and the broad criterion NO STANDING MATERIAL CHALLENGE is violated by a source-verified, host-corroborated, unrebutted challenge. PROVIDER/CREDENTIAL-CACHE ANCESTRY SUB-PART NOT DISCHARGED. --- C2 BEARER-PATH SUB-PART — STILL DISCHARGED (non-regressed). Carried verbatim from the v4 discharge: writeEphemeralMCPConfig re-rooted under .striatum/scratch/<supervisor_id>/ as a required P0 build step before re-keying the scratch ACLs (mcpconfig.go:550-580 is still the live root-scratch bearer writer at HEAD; the re-root removes the #279 rwx-on-scratch-root need and keeps the --x-only-root final ACL launch-consistent), A22 made real (derives the path from the live writer, asserts no residual root-level bearer, S1 positive CreateTemp control), A24 added. falsifier_1 credits this and falsifier_2 finds no regression; I concur. --- C1-RESIDUAL (OQ2-SCRUB-POSTCONDITION) — STILL DISCHARGED. The ancestry fix does not touch the scrub path; OQ2 is carried verbatim from the v3-DISCHARGED form. P1 is the fail-closed three-way classifyPoolUIDTaskState predicate (tolerate only true zombie/dead /proc state in {Z,X,x} or a vanished PID; BLOCK every other observed state incl. T/t, any unrecognized char, and an unreadable/ambiguous /proc read for a still-present PID -> quarantined + typed lane_uid_scrub_failed), explicitly NOT the binary processZombie; /proc evidence recorded; A21 intact. falsifier_2 could not construct a non-zombie pool_uid survivor that reaches returned; I concur. --- CARRY-FORWARDS — INTACT. HC-A1..A5 INTACT (carried verbatim from v1/v2/v3/v4 Part 1; the ancestry fix touches neither the launch path nor the structural no-replay claim). The C1 durable four-state lease machine + uq_lane_uid_held + 3-transaction boundary + reaper + quarantine-survives-restart + dirty-excluding exhaustion INTACT. OQ1/OQ3/OQ5/OQ6 INTACT. The narrowing invariant INTACT (no admin-token widening, no lane-readable shared reseal bearer; the ancestry fix only removes surface). The C2 PROCEDURE fix (mandatory allowlist / prune-then-apply, raw setfacl -R <repoRoot> forbidden) and A23's RAW-RECURSIVE-ROOT case INTACT; the GROUP-ACL end-state invariant for .striatum/ / .git/ INTACT. The single open item is the NEWLY-ENFORCED provider/credential-cache EXTENSION of that invariant, classified under the OPEN OQ4-ACL-PROVISIONING-TRANSITION finding — NOT a regression of a carried claim (matching falsifier_2's own framing: a completeness gap in the NEW fix, not a no-regression failure). --- PER-RESIDUAL RECORD: provider/credential-cache ANCESTRY sub-part NOT DISCHARGED (a nested CLAUDE_SECURESTORAGE_CONFIG_DIR cache under docs/ still receives the group ACL; OQ4.1.1/A25 derive from a resolver roster that omits the key; A23 and physical apply do not reach it). C2 BEARER-PATH sub-part DISCHARGED. C1-RESIDUAL DISCHARGED. CARRY-FORWARDS INTACT: HC-A1..A5, C1 lease machine, C2 procedure fix + A23 raw-root case, GROUP-ACL end-state (.striatum//.git/ core), OQ1, OQ3, OQ5, OQ6, narrowing. FALSIFIER DISPOSITIONS: falsifier_1 (reviewer-001) — provider/credential-cache ancestry challenge STANDS (landed_unrebutted). falsifier_2 (reviewer-002) — no-regression challenges find nothing and its only material point defers to falsifier_1 (not_material as an independent regression). --- WHY NOT REJECT: the maintainer-ratified direction (per-lane pooled OS uid, D261, 2026-06-24) is sound and not relitigated; C1-RESIDUAL and the C2 bearer-path are genuinely discharged; the C2 procedure/raw-root half and the .striatum//.git/ end-state are correct; no path widens admin-token exposure or mints a lane-readable shared reseal bearer; the prescribed CLAUDE_CONFIG_DIR case is genuinely closed; and the remaining correction is small and precisely specified. WHY NOT ACCEPT / ACCEPT_WITH_FINDINGS: a standing, source-verified, host-corroborated, unrebutted material challenge exists; the OQ4 ancestry invariant (no provider credential cache reachable by a g:striatum-lanes ACL) is demonstrably violable by a faithful build of the v5 spec with the verifying test green; the defect is inside the OQ4 clearing requirement, not post-clearance. --- VERDICT: needs_revision. GATE-CYCLE CONSEQUENCE: per the SEED and adjudicator role this is v5's SINGLE proper revision round; this needs_revision EXHAUSTS the v5 gate UNCLEARED and routes the remaining residual to the OPERATOR — do NOT ratchet into another holder cycle. The committer does NOT publish the consolidated SPEC, and the operator does NOT ratify D272 on this verdict (D271 remains reserved by the concurrent RFC 0170 P0 design; D272 is RFC 0168's free reservation). The remaining fix is small and source-anchored: make the credential-cache protected set PROVIDER-COMPLETE and FAIL-CLOSED — model CLAUDE_SECURESTORAGE_CONFIG_DIR (and any other config-dir/secure-storage selector the in-scope provider CLIs actually read) in the resolver roster so OQ4.1.1 and A25 cover it, AND/OR fail the launch closed whenever any provider-specific credential-dir env key present in command_env is NOT covered by the resolver (so an unmodeled future selector cannot silently bypass the ban); and extend A25 to drive the secure-storage selector through the live chain and assert no g:striatum-lanes access/default ACL on a nested secure cache before/during/after. The maintainer-ratified direction (D261) carries regardless — adjudicator clearance gates the spec's soundness, not the product call.
findings:
  - id: HC-ORACLE-INTACT
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - |-
        structural no-replay: a per-lane uid dissolves the BC1-W1-ORACLE same-uid tmux/0600/ptrace replay class on this host (Yama ptrace_scope=1) — CARRIED FORWARD UNREGRESSED from v1/v2/v3/v4
    challenge: |-
      INTACT (carry-forward). The v1-proven hard core HC-A1..A5 (per-uid 0700 tmux socket so a sibling cannot respawn the target pane; cross-uid 0600 DAC — the surface that makes RFC 0143 Slice B's reseal token safe; cross-uid ptrace/setns//proc denial by ptrace_may_access — the exact axis namespace-inode failed under D261; SO_PEERCRED uid discriminator; every residual same-uid surface closed) is carried into v5 verbatim (HOLDER Part 1). The v5 ancestry fix touches neither the launch path nor the structural claim. falsifier_2 re-checked and found no regression. Not reopened, not regressed.
  - id: OQ2-SCRUB-POSTCONDITION
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:3"]
    affected_invariants:
      - |-
        no cross-lease same-uid residue: a returned uid must be PROVABLY empty before re-lease — the scrub postcondition must prove zero non-zombie uid-owned tasks (C1-RESIDUAL), DISCHARGED in v3 and carried unregressed through v4 and v5
    challenge: |-
      DISCHARGED and CARRIED unregressed. P1 is the fail-closed three-way classifyPoolUIDTaskState predicate (tolerate only true zombie/dead /proc state in {Z,X,x} or a fully-vanished PID; BLOCK every other observed state incl. T (stopped) / t (tracing-stop), any unrecognized char, AND an unreadable/ambiguous /proc read for a still-present PID -> quarantined + typed lane_uid_scrub_failed), explicitly NOT the binary processZombie (tmux_liveness.go:599-614, which returns true only for Z and false on a read error). /proc evidence (observed PIDs + state chars + classification) recorded in scrub_proof (pass) and scrub_failure (block); A21 TestStoppedOrTracedUIDProcessBlocksReturn intact. The v5 ancestry fix does not touch the scrub path. falsifier_2 attempted the non-zombie-survivor refuter and could not construct a non-zombie pool_uid task that reaches returned; I concur. C1-RESIDUAL STILL DISCHARGED, no regression.
  - id: OQ4-ACL-PROVISIONING-TRANSITION
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - |-
        ACL exactly-enough without over-grant: NO path that is equal-to, a descendant-of, OR an ancestor-of any provider credential/cache directory may carry a g:striatum-lanes access OR default ACL entry — before, during, OR after provisioning — for EVERY credential selector the in-scope provider CLI actually reads (not only the subset the Striatum resolver models); AND the discharge test (A25) must exercise the REAL selector the provider reads
    challenge: |-
      OPEN — verdict-driving. The v5 revision faithfully implements the v4-prescribed ancestry mechanism (three chokepoints + A25) and genuinely closes the prescribed CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude case. The residual stays OPEN because the mechanism is ALLOWLIST-scoped (the protected set = the parents of ResolveCredential's result), not FAIL-CLOSED against unmodeled provider credential selectors, and the live Claude runtime reads one today. Source-verified at HEAD 5fd0c81e: command_env CLAUDE_SECURESTORAGE_CONFIG_DIR is admitted (validateLaneCommandEnvKey bars only empty/PATH/STRIATUM_*, supervision_lane_config.go:440-451), merged (supervision_env.go:110-118), and survives the run-as filter (sensitiveRunAsEnvKey drops only TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN substrings + a small denylist; CLAUDE_SECURESTORAGE_CONFIG_DIR matches none, supervision_env.go:303-318). But ResolveCredential (resolver.go:24-32 const block; Claude branch :78-92) models ONLY CODEX_HOME/CLAUDE_CONFIG_DIR/HOME and never consults CLAUDE_SECURESTORAGE_CONFIG_DIR (grep confirms ZERO SECURESTORAGE occurrences in go/), so OQ4.1.1's guard cannot inspect that cache dir; a lane with CLAUDE_SECURESTORAGE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/secure (CLAUDE_CONFIG_DIR unset/outside) passes the inside-repo ban while the provider reads the in-repo secure cache. The OQ4.1(a) grant on docs/ (access + DEFAULT ACL) sweeps the nested store if present and is INHERITED by a late-created .credentials.json via POSIX default-ACL semantics. A23 does not reach it (per-lane command_env path, invisible to the static planner — holder concedes this); physical -RP does not reach it (real descendant, not a symlink); A25 stays green because it drives CLAUDE_CONFIG_DIR through the same incomplete resolver. The installed claude 2.1.195 binary's static strings contain CLAUDE_SECURESTORAGE_CONFIG_DIR, corroborating the provider actually reads it. Net: a real cross-lane provider-credential read with the verifying tests green — a soundness defect INSIDE the OQ4 ancestry clearing requirement, violating the OQ4 end-state invariant and the OQ6 carry-forward sentence. falsifier_1 STANDS (landed_unrebutted); falsifier_2 defers to it. needs_revision.
    closest_acceptable_answer: |-
      The bearer-path and C1 sub-parts are DISCHARGED — no change needed there. For the OPEN provider/credential-cache ancestry sub-part, make the protected credential-cache set PROVIDER-COMPLETE and FAIL-CLOSED: (a) model CLAUDE_SECURESTORAGE_CONFIG_DIR (and any other config-dir / secure-storage env key the in-scope provider CLIs actually read) in the resolver roster so OQ4.1.1's resolution-domain ban and A25 cover it; AND/OR (b) fail the launch closed (typed lane_credential_cache_inside_repo or a sibling typed floor) whenever ANY provider-specific credential-dir env key present in command_env is NOT covered by the resolver — so an unmodeled present-or-future selector cannot silently bypass the ban rather than being assumed safe; AND (c) extend A25 to drive the secure-storage selector (CLAUDE_SECURESTORAGE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/secure) through the live env chain and assert no g:striatum-lanes access OR default ACL on that directory or its .credentials.json before/during/after, including the created-after-provisioning case. Routed to the operator (single v5 cycle exhausted).
  - id: OQ-CARRY-FORWARD-INTACT
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - |-
        the v1/v2/v3/v4-credited carried set into v5 unregressed: the v4-DISCHARGED C2 bearer-path fix (re-rooted writeEphemeralMCPConfig + real A22 + A24), the C1 durable four-state lease machine, the C2 procedure fix + A23 raw-recursive-root prohibition, the .striatum//.git/-excluding GROUP-ACL end-state invariant, OQ1 sizing + fail-closed exhaustion, OQ3 launch-as-only provisioning, OQ5 generation token, OQ6 hydration shape, restart-survival, and the narrowing invariant
    challenge: |-
      INTACT (carry-forward, no regression from the ancestry fix). Both falsifiers report no carry-forward regression and I concur. C2 BEARER-PATH sub-part: DISCHARGED and carried verbatim — writeEphemeralMCPConfig re-rooted under .striatum/scratch/<supervisor_id>/ as a required build step (mcpconfig.go:550-580 still the live root-scratch writer at HEAD; the re-root removes the #279 rwx-on-scratch-root need and keeps the --x-only-root final ACL launch-consistent), A22 real, A24 added; falsifier_1 credits it. C1 lease machine: four-state active/scrubbing/quarantined/returned, partial held-unique index, three-transaction allocate/scrub-begin/scrub-finalize boundary, failed-proof-quarantines, leaked-active + stuck-scrubbing reaper, quarantine-survives-restart, dirty-excluding exhaustion — all carried. C2 procedure fix: the mandatory prune-then-apply form and the explicit raw setfacl -R <repoRoot> prohibition are carried; A23's raw-recursive-root case is INTACT and the right shape; the GROUP-ACL end-state invariant is INTACT for .striatum/ / .git/ (top-level siblings, A23 ancestor coverage for .striatum/). OQ1: host-global ceiling + typed fail-closed lane_uid_pool_exhausted, v1 caveat stays closed (A20). OQ3: static host-runbook pool, daemon holds only the launch-as %striatum-lanes grant (A12). OQ5: leased-uid + monotonic per-uid generation token compared on every attestation AND control-frame path (A14). OQ6: per-spawn per-uid hydration into the leased uid's 0600 store scrubbed on return, contingency closed by P3 (A15) — NOTE its re-backed sentence that the resolved credential cache stays outside the group-ACL domain is the very claim the OPEN OQ4 finding shows is not yet provider-complete (the CLAUDE_SECURESTORAGE_CONFIG_DIR path escapes it). Restart-survival: free set DERIVED from PostgreSQL (A8'/A11'/A19). NARROWING: the ancestry fix only removes surface; no new authority, no admin-token widening, no lane-readable shared reseal bearer. The provider/credential-cache completeness gap is classified under the OPEN OQ4-ACL-PROVISIONING-TRANSITION finding — NOT a regression of a carried claim. Do NOT reopen or regress the carried set.
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0168 P0 design (v5 REVISION, cycle 1)

author: adjudicator-author-001

> Cycle-1 adjudication of the RFC 0168 P0 **v5 REVISION** `falsification_gate`
> dialogue trajectory (*per-lane OS uid as the lane security principal*). The v5
> revision was charged to discharge the **single** standing v4 cycle-1 residual —
> the **provider/credential-cache ANCESTRY** sub-part of
> `OQ4-ACL-PROVISIONING-TRANSITION` — while carrying everything v1/v2/v3/v4 cleared
> forward unregressed, including the v3-DISCHARGED **C1-RESIDUAL** and the
> v4-DISCHARGED **C2 bearer-path** fix. Inputs read: the revised Holder spec
> (`dialogue/holder/HOLDER.md`), both falsifier re-attacks
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`), the
> v5 `SEED.md` charter + adjudicator role, RFC 0168, and — as required context —
> the v4 SPEC (`context/v4_HOLDER.md`) and the v4 cycle-1 ledger
> (`context/v4_LEDGER_cycle_1.md`). No raw provider logs or private workflow
> diagnostics were read as workflow-state evidence. **Every load-bearing source
> citation below was independently re-verified against the run-branch worktree
> HEAD `5fd0c81e`** (the branch has advanced past the `621312c4` the holder cited;
> the cited sites are unchanged). The direction (a pre-provisioned pool of
> per-lane uids, leased per lane) is maintainer-ratified (**D261**, 2026-06-24)
> and is **not** relitigated.

## Verdict

**verdict: needs_revision**

The v5 revision **faithfully implements the v4-prescribed ancestry mechanism**
(launch-time resolution-domain ban OQ4.1.1 + planner/A23 ancestor rule + physical
no-symlink apply + A25) and **genuinely closes the exact prescribed case**
(`CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`). But a **material,
source-confirmed, host-corroborated** challenge against the **same**
provider/credential-cache ancestry residual lands from **falsifier_1** and stands
**unrebutted** (a `falsification_gate` has no holder rebuttal turn), so the gate
does **not** clear. A clearing verdict requires the provider-ancestry residual
**genuinely discharged** (with *no* falsifier exhibiting a nested cache that still
receives a group ACL), the bearer-path + C1-RESIDUAL still discharged, no
carry-forward regressed, and **no standing material challenge**.

- **`OQ4-ACL-PROVISIONING-TRANSITION` — provider/credential-cache ANCESTRY sub-part
  — NOT DISCHARGED (the standing challenge).** The discharge mechanism is
  **allowlist-scoped** — its protected set is the directory parents of
  `ResolveCredential`'s result — rather than **fail-closed** against unmodeled
  provider credential selectors, and the live Claude runtime has one **today**. I
  verified at HEAD `5fd0c81e`: `command_env CLAUDE_SECURESTORAGE_CONFIG_DIR` is
  admitted (`supervision_lane_config.go:440-451`), merged
  (`supervision_env.go:110-118`), and **survives** the run-as filter
  (`supervision_env.go:303-318` — it contains none of the dropped substrings); but
  `ResolveCredential` (`resolver.go:24-32`, `:78-92`) models **only**
  `CODEX_HOME`/`CLAUDE_CONFIG_DIR`/`HOME` and **never** consults
  `CLAUDE_SECURESTORAGE_CONFIG_DIR` (grep: **zero** `SECURESTORAGE` in `go/`). So
  OQ4.1.1's guard cannot inspect that cache dir; a lane with
  `CLAUDE_SECURESTORAGE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/secure`
  (`CLAUDE_CONFIG_DIR` unset/outside) **passes** the inside-repo ban while the
  provider reads the in-repo secure cache, and the OQ4.1(a) `docs/` grant (access +
  **default** ACL) sweeps or is inherited onto it. **A23** can't reach it (per-lane
  path), **physical `-RP`** can't reach it (real descendant, not a symlink), **A25**
  stays **green** (it drives `CLAUDE_CONFIG_DIR` through the same incomplete
  resolver). The installed `claude 2.1.195` binary's static strings contain
  `CLAUDE_SECURESTORAGE_CONFIG_DIR`, corroborating the provider reads it. A real
  cross-lane provider-credential read with the verifying tests green — a
  **soundness defect inside** the OQ4 clearing requirement, **not** post-clearance
  polish.

- **C2 bearer-path sub-part — STILL DISCHARGED.** Carried verbatim:
  `writeEphemeralMCPConfig` re-rooted under `.striatum/scratch/<supervisor_id>/` as
  a required build step, real A22, A24 added; the final `--x`-only-root ACL stays
  launch-consistent. `falsifier_1` credits it; `falsifier_2` finds no regression.

- **C1-RESIDUAL (`OQ2-SCRUB-POSTCONDITION`) — STILL DISCHARGED.** P1 is the
  fail-closed three-way `classifyPoolUIDTaskState` predicate (not `processZombie`);
  `T`/`t`/unknown/unreadable-still-present block `returned`; `/proc` evidence
  recorded; A21 intact. The ancestry fix doesn't touch the scrub path.
  `falsifier_2` could not construct a non-zombie `pool_uid` survivor that re-leases;
  I concur.

- **Carry-forwards — INTACT.** HC-A1..A5, the C1 four-state lease machine, the C2
  procedure fix + A23 raw-recursive-root case, the GROUP-ACL end-state invariant
  for `.striatum/`/`.git/`, OQ1/OQ3/OQ5/OQ6, and the narrowing invariant are all
  carried unregressed. The provider/credential-cache completeness gap is the
  **open** OQ4 item, **not** a regression of a carried claim (matching
  `falsifier_2`'s own framing). No admin-token widening, no lane-readable shared
  reseal bearer.

`needs_revision`, **not** `reject` (the direction is D261-ratified; C1-RESIDUAL and
the C2 bearer-path are genuinely discharged; the C2 procedure/raw-root half and the
`.striatum/`/`.git/` end-state are correct; the prescribed `CLAUDE_CONFIG_DIR` case
is closed; the remaining correction is small and precisely specified) and **not**
`accept`/`accept_with_findings` (a standing, source-verified, host-corroborated,
unrebutted material challenge; the OQ4 ancestry invariant is demonstrably violable
by a faithful build with the verifying test green; the defect sits inside the OQ4
clearing requirement).

## Per-residual / per-carry-forward / per-falsifier record

- **C1-RESIDUAL** — **DISCHARGED**.
- **C2 bearer-path sub-part** — **DISCHARGED** (carried verbatim from v4).
- **OQ4 provider/credential-cache ANCESTRY sub-part** — **NOT DISCHARGED**: the
  prescribed `CLAUDE_CONFIG_DIR`-under-`docs/` case is genuinely closed, but a
  nested `CLAUDE_SECURESTORAGE_CONFIG_DIR` cache under `docs/` still receives the
  group ACL because OQ4.1.1/A25 derive their protected set from a resolver roster
  that omits the key, and A23/physical-apply do not reach a per-lane real-descendant
  cache.
- **Carry-forwards** — **INTACT**: HC-A1..A5; C1 lease machine; C2 procedure fix +
  A23 raw-root case; GROUP-ACL end-state invariant (`.striatum/`/`.git/` core); OQ1;
  OQ3; OQ5; OQ6; narrowing.
- **`falsifier_1` (reviewer-001)** — provider/credential-cache ancestry challenge
  **STANDS** (`landed_unrebutted`).
- **`falsifier_2` (reviewer-002)** — no-regression challenges find nothing; its only
  material point defers to `falsifier_1` (**`not_material`** as an independent
  carry-forward regression).

## Gate-cycle consequence

This was the **single allowed v5 revision cycle**. A `needs_revision` here
**exhausts the gate uncleared** and routes the residual to the **operator** — do
**not** ratchet into another holder cycle. The committer does **not** publish the
consolidated SPEC, and the operator does **not** ratify **D272** on this verdict
(**D271** remains reserved by the concurrent RFC 0170 P0 design; D272 is RFC 0168's
free reservation). The remaining correction is small and source-anchored: make the
credential-cache protected set **provider-complete and fail-closed** — model
`CLAUDE_SECURESTORAGE_CONFIG_DIR` (and any other config-dir/secure-storage selector
the in-scope provider CLIs actually read) in the resolver roster so OQ4.1.1 and A25
cover it, **and/or** fail the launch closed whenever any provider-specific
credential-dir env key present in `command_env` is not covered by the resolver (so
an unmodeled selector cannot silently bypass the ban); **and** extend A25 to drive
the secure-storage selector through the live chain and assert no `g:striatum-lanes`
access/default ACL on a nested secure cache before/during/after. The
maintainer-ratified direction (per-lane pooled OS uid, D261) carries regardless —
adjudicator clearance gates the spec's **soundness**, not the product call.

<sub>Adjudicator collaboration ledger — RFC 0168 P0 `falsification_gate` design run,
**REVISION v5, cycle 1**. **OQ4 provider/credential-cache ANCESTRY sub-part NOT
DISCHARGED**: the v5 fix implements the prescribed three-chokepoint ancestry
mechanism and closes the prescribed `CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`
case, but the launch-time ban (OQ4.1.1) and A25 derive their protected set from
`ResolveCredential`'s roster, which models only `CLAUDE_CONFIG_DIR`/`CODEX_HOME`/`HOME`
and **not** `CLAUDE_SECURESTORAGE_CONFIG_DIR` — a credential selector the live Claude
CLI (2.1.195) actually reads (verified at HEAD `5fd0c81e`: admitted by
`validateLaneCommandEnvKey`, survives `sensitiveRunAsEnvKey`, unmodeled by
`resolver.go`; zero `SECURESTORAGE` in `go/`). A nested secure-storage cache under
allowlisted `docs/` still receives the `g:striatum-lanes` access+default ACL (A23
can't see the per-lane path; physical `-RP` can't help a real descendant; A25 stays
green) — a real cross-lane provider-credential read inside the OQ4 clearing
requirement. **C2 bearer-path STILL DISCHARGED** (re-rooted writer, real A22, A24).
**C1-RESIDUAL STILL DISCHARGED** (P1 -> `classifyPoolUIDTaskState`, A21, `/proc`
evidence). Carry-forwards INTACT (HC-A1..A5, the C1 lease machine, the C2 procedure
fix + A23 raw-root case, the GROUP-ACL end-state `.striatum/`/`.git/` core,
OQ1/OQ3/OQ5/OQ6, narrowing). `needs_revision` — single v5 cycle exhausted; the
provider-complete/fail-closed credential-cache fix (model the secure-storage selector
+ fail-closed on unmodeled selectors + a real A25 over it) routes to the operator;
the committer does not publish the consolidated SPEC and the operator does not ratify
D272. D261 carries regardless.</sub>
