---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0168 P0 — per-lane OS uid as the lane security principal: falsifiable implementation spec (fresh v1; prove a per-lane uid dissolves BC1-W1-ORACLE on this host and discharge the six design-gate open questions into build-bearing constraints)"
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
    text: "Fresh v1 falsification_gate spec for RFC 0168 P0 (per-lane pooled OS uid as the lane principal; direction ratified D261, not relitigated). PART 1 (hard core): a per-lane uid dissolves BC1-W1-ORACLE on this host (Yama ptrace_scope=1) via four structural sub-claims, each anchored to the real run-as launch path (`sudo -n -u <runAsUser> -- env -i ... tmux ...`, pty.go:98-112, tmuxRunnerForSpec/RunAsTmuxRunner pty.go:310-314 + tmux_liveness.go:125-149, bare tmux with a deterministic session name pty.go:620-633 and no `-S`/TMUX_TMPDIR): HC-A1 tmux control surface is per-uid (`/tmp/tmux-<uid>` 0700, srwx------; a different-uid sibling has no traverse and cannot respawn-pane the target pane); HC-A2 a U_t-owned 0600 file in a 0700 HOME is unreadable by U_s (the RFC 0143 Slice B reseal-token surface); HC-A3 cross-uid ptrace/setns//proc secret reads denied by ptrace_may_access (the exact axis namespace-inode failed and uid succeeds); HC-A4 the daemon control socket admits only the leased uid via SO_PEERCRED; HC-A5 names and closes every residual same-uid surface (shared parent = only the trusted daemon; shared tmux server now per-uid; world/group-readable path closed by 0600/0700 on private surfaces; daemon-bridging is lane-vs-daemon not lane-to-lane). PART 2 discharges the six OQs as build-bearing constraints + refuting tests: OQ1 host-global pool ceiling (first such ceiling; default operator-chosen N) + typed fail-closed `lane_uid_pool_exhausted` refuse-and-requeue (never share, never auto-grow, never useradd); OQ2 a daemon-owned PostgreSQL `lane_uid_leases` table {repository_id, pool_uid, session_id, supervisor_id, generation, state active|returned, leased_at, returned_at, scrub_status} with a unique active index on pool_uid, session-bound, a 4-step return scrub (per-uid `kill -KILL -1` domain, credential store delete, HOME scrub, mark returned only after clean; quarantine on scrub failure), a recovery-sweep leaked-uid reaper (recovery.go:553/856/2516), and restart-survival by reconstructing the free set from the DB after a fresh boot epoch (main.go:459-468/665-677); OQ3 host-runbook static pool, daemon launch-as-only via a widened `(%striatum-lanes) NOPASSWD: ALL` runas-group grant, NO uid-lifecycle authority; OQ4 a DEFAULT group ACL `g:striatum-lanes:rX` recursively on <repo> for shared read + per-uid 0600/0700 private secrets + per-job worktree chowned to the leasing uid, with non-leased repo read declared harmless; OQ5 attestation binds the leased uid + a monotonic per-uid generation token (folded in like MCPBootEpoch, mutations.go:41-48) refusing stale-generation actors; OQ6 the RFC 0165 spawn-time hydrator (#583) re-targeted to the leased uid's HOME, per-spawn fresh, scrubbed on return. PART 3 scopes P0 = the minimum (provisioning + lease + scrub + attestation + credential) that lets a lane own a 0600 reseal token and unblock RFC 0143 Slice B, with deferred seams named, local-first boundary intact, and the change framed as a narrowing (no new authority). Every load-bearing source citation re-verified against worktree HEAD."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "Lease-lifecycle / exhaustion lens. Credits the build-shaped resolution of most of OQ2/OQ1 (fixed host-provisioned pool, session-bound rather than job-bound leases, durable PostgreSQL state, typed fail-closed exhaustion, DB-derived restart reconstruction). Standing material challenge: the spec does not actually make a RETURNED uid safe to re-lease, because its durable state model is incomplete at exactly the failure boundary the gate depends on. The schema declares `state active|returned` only, but the prose relies on a third behavior `quarantined` (HOLDER.md:253-255,:280-283) with no representation, and the free-set predicate is `uids with no active lease row` (HOLDER.md:301-302). That leaves no build-bearing `scrubbing` or `quarantined` state, no transaction/advisory-lock boundary preventing allocation while scrub is in progress, and no required post-scrub PROOF that the uid process/home/credential domain is empty before return — only commands (`sudo -n -u <pool_uid> -- kill -KILL -1`, delete creds, rm HOME) whose zero return does not establish an empty kill domain. Concrete failing case: S1 leaves a daemonized same-uid process / live uid-owned tmux server / credential file / HOME scratch; S1 closes (or the daemon dies and the post-restart sweep declares S1 dead); the scrub does not reach a clean postcondition (an injected failure, a process still in /proc for U, a deletion failure); the daemon must record `U not leased to a live session but also not free`, which the table cannot express — mark it `returned`+`scrub_status=failed` and the free-set rule allocates U to S2 (dirty reuse = same-uid cross-lease leak); leave the row `active` and it is a zombie-active lease for a dead session with no operator recovery path, no doctor surface, and false `lane_uid_pool_exhausted` accounting. The dangerous branch is S2 running as U while S1 process/credential residue survives — at which point per-lane-uid isolation no longer separates the principals, and the OQ5 generation token only guards daemon-side attestation, not same-uid local reads/tmux/socket/file interactions inside U's domain. Source-anchored against the live narrow teardown (supervision_control.go:557-580,:631-637) confirming P0 must add a NEW scrub primitive, and the launch-as-only boundary (lane-sandbox.md:88-100,:415-417). Required revision: explicit lease states active/scrubbing/quarantined/returned with the free predicate = only `returned` rows with a clean scrub generation; allocation under a lock that cannot race scrubbing/quarantined rows for the same pool_uid; a defined scrub POSTCONDITION (zero non-zombie uid-owned processes, no uid-owned tmux server/socket, credential store absent, HOME private scratch empty, reseal-token path absent) proven before `returned`; a failed-scrub lifecycle (persist quarantined, typed `lane_uid_scrub_failed`, doctor surface, non-free across restart, operator/recovery retry that transitions only quarantined->returned after the same proof); and negative tests extending A7/A9/A10/A11 (injected scrub failure quarantines and is never allocated; restart preserves quarantine; exhausting clean uids with one quarantined yields typed exhaustion not dirty reuse/shared-uid fallback). needs_revision (per-lane uid still right; no admin-token widening, no lane-readable shared bearer), not accept_with_findings (complete scrub + leaked-uid reaping + restart-safe reconstruction + exhaustion are OQ1/OQ2 gate conditions)."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "Structural-security / attestation / credential / ACL lens. Credits the core direction and several assigned claims: a different OS uid is the right structural answer to BC1-W1-ORACLE (no residual cross-uid ptrace/setns/respawn-pane hole found in the hard core); A1-A5 are the right negative-control shape; the OQ5 generation token is the right anti-recycle primitive (given all attestation AND control-frame paths compare it to the live uid-lease generation); OQ6 per-spawn per-uid hydration is directionally sound contingent on the scrub/quarantine gap (falsifier-reviewer-001) being fixed. Standing material challenge inside OQ4: the proposed pool ACL treats the target-repo ROOT as harmless shared source, but `.striatum/` is daemon/operator-private operational scratch that lives UNDER the repo root. The holder's command `setfacl -R -m g:striatum-lanes:rX <repo>` + `setfacl -R -d -m g:striatum-lanes:rX <repo>` recurses through `.striatum/` and grants every pool uid — including a uid NOT currently leased to that repo — read/traverse on a tree containing per-launch MCP config (0600 bearer material), PTY logs, helper scratch, pidfiles, the capability-token cache, and foreign per-job worktrees. That breaks the holder's own `private surfaces carry no group ACL` invariant and contradicts the product boundary. Source-anchored: the runbook withholds blanket `.striatum/` access and carves out only the lane's own per-job worktree (lane-sandbox.md:100-104,:348-355); the agent guide/spec classify `.striatum/` as operational scratch not provenance (how-to-agent.md:54-59, spec.md:70-75); the source scratch ACL helper is intentionally narrow — `.striatum`:`u:<lane>:--x` traverse-only + `.striatum/scratch`:`u:<lane>:rwx`+default with the inline `never broaden read access to private operator state` comment (scratch_acl.go:31-48; supervision_control.go:112-126); the ephemeral MCP config carrying the bearer is a 0600 file under `.striatum/scratch` (mcpconfig.go:550-571) and PTY logs are 0600 `.striatum/scratch/<supervisor_id>/pty.log` (loop.go:280-300); the current recursive repo-ACL helper is `setfacl -R` over repoRoot + `.striatum/worktrees` (repo_acl.go:21-31,:120-135), so a pool-group version recurses into existing `.striatum/` unless an exclusion is specified; and a host sanity check confirms `setfacl -R -m g:<group>:rX` on an existing 0600 file adds effective group read (so `mode is 0600` is not a sufficient rebuttal). Concrete failing case: an adopted repo with existing `.striatum/scratch/<supervisor_id>/lane-mcp-config-*.json`, pty.log, token cache; the operator runs the OQ4 command; a non-leased pool uid U_s then reads another lane's session-bound control-plane bearer and calls the daemon as that session, or reads private PTY diagnostics — the same class of cross-lane replay surface RFC 0168 exists to remove. A13 only checks shared repo read, private HOME/reseal-token readability, worktree write, and PG denial; it does NOT assert `.striatum/`, ephemeral MCP config, PTY logs, foreign worktrees, or token caches stay unreadable to non-leased pool uids after provisioning. Required revision: do not apply `g:striatum-lanes:rX`/default-group recursively to the raw repo root unless `.striatum/`/`.git/`/`.gemini/`/`.claude/`/provider-auth/token-cache paths are explicitly excluded (prefer an allowlist of source/artifact paths); keep `.striatum/` daemon/operator-private (only `--x` to the currently-leased uid when needed); grant `.striatum/worktrees/<id>` only to the leasing uid; use a per-supervisor per-leased-uid ACL on `.striatum/scratch/<supervisor_id>` removed on teardown/scrub; extend A13 into a negative test (`TestPoolACLDoesNotExposeOperationalScratchOrForeignWorktrees`) over seeded existing AND future scratch; extend `make lane-isolation-check`/doctor to fail when `.striatum/` or token-bearing paths carry `striatum-lanes` group read outside the explicit current-lease exception. needs_revision (per-lane uid model right once the ACL boundary is exact; a narrowing otherwise), not accept_with_findings (OQ4 is one of the six gate questions and the packet asks for exactly-enough-without-over-grant)."
verdict: "needs_revision"
rationale: "Adjudicates the FRESH v1 RFC 0168 P0 falsification_gate dialogue (one holder spec, two independent falsifier re-attacks; the cycle ends at adjudication with no further holder turn) against the SEED clearing condition: a clearing verdict (accept / accept_with_findings) requires the STRUCTURAL claim proven (a per-lane uid genuinely dissolves BC1-W1-ORACLE on this host under Yama ptrace_scope=1 with no residual same-uid surface), the lease/scrub/reaper COMPLETE, restart-survival ESTABLISHED, exhaustion SAFE, provisioning coherent with the daemon privilege boundary, attestation re-binding to the pooled uid with recycle-confusion prevented, the per-uid credential store free of stale-copy/cross-uid leak, ACLs exactly-enough without over-grant, the change a narrowing, AND no standing falsifier challenge. This v1 is strong and closes most of the surface: I independently re-verified the load-bearing sites and CREDIT the hard core. The structural claim is PROVEN — both falsifiers credit HC-A1..A5 and falsifier-reviewer-002 explicitly finds no residual cross-uid ptrace/setns/respawn-pane hole. I confirmed against worktree HEAD that lanes launch bare tmux through `sudo -n -u <runAsUser> -- env -i` with a deterministic session name and no `-S`/TMUX_TMPDIR (pty.go:98-112,:310-314,:620-633; tmux_liveness.go:125-149), so distinct run-as uids necessarily land on tmux's default per-uid 0700 socket (HC-A1), a different uid fails ptrace_may_access for ptrace/setns//proc (HC-A3 — the exact axis namespace-inode failed under D261), and a 0600/0700 lane-owned file is cross-uid-unreadable (HC-A2). HC-A4's SO_PEERCRED uid discriminator and HC-A5's residual-surface closure are coherent. RESTART-SURVIVAL is ESTABLISHED: binding the uid to the session in daemon-owned PostgreSQL and DERIVING the free set from the table (never from memory) is the correct answer to the boot-epoch rotation D261 targets (the boot epoch is fresh-per-process and not persisted, main.go:459-468; the DB rows survive). PROVISIONING (OQ3) is RESOLVED — a static host-runbook pool with the daemon holding only the widened launch-as-only `(%striatum-lanes)` grant and NO useradd/userdel authority preserves the privilege boundary (both falsifiers credit it). OQ1 sizing (host-global ceiling across all runs) + the typed fail-closed refuse-and-requeue default are the right safe choices. OQ5's leased-uid + monotonic generation token is the right anti-recycle primitive. OQ6's per-spawn per-uid hydration with scrub-on-return is the right shape. The change is a NARROWING (no new authority; both falsifiers confirm no admin-token widening and no lane-readable shared reseal bearer). BUT the gate does NOT clear: TWO new material, source-anchored challenges land, both inside SEED gate conditions, and BOTH stand unrebutted (the holder had no further turn). CHALLENGE 1 (falsifier-reviewer-001, verdict-driving — OQ2 lease lifecycle incomplete at the failure boundary): the durable state machine cannot represent a returned-but-dirty uid. The schema is `state active|returned` while the prose relies on `quarantined` (HOLDER.md:253-255,:280-283) with no representation, and the free predicate is `no active lease row` (HOLDER.md:301-302). I confirm there is no `scrubbing`/`quarantined` state, no allocation/scrub transaction boundary, and no scrub POSTCONDITION proof — `mark returned only after 1-3 succeed` rests on command exit codes (`kill -KILL -1` returning 0 does not prove an empty per-uid kill domain; a process can survive, reappear, or recreate HOME files after the command returns). So a scrub failure forces the daemon into a state the table cannot express: mark `returned`+`scrub_status=failed` and the free-set rule re-leases the dirty uid to S2 (a same-uid cross-lease residue leak — the exact class RFC 0168 must eliminate); leave it `active` and it is a zombie-active lease for a dead session with no recovery path, no doctor surface, and false exhaustion accounting that contaminates OQ1. The OQ5 generation token does not rescue this — it guards daemon-side attestation, not same-uid local reads/tmux/file interactions inside U's OS domain. A9 asserts zero-residue positively but never defines the negative path. The SEED makes `the lease/scrub/reaper complete` and `exhaustion safe` clearing conditions; an incomplete failure-path state machine is a gate hole, not post-clearance polish. CHALLENGE 2 (falsifier-reviewer-002, verdict-driving — OQ4 ACL over-grants `.striatum/`): the recursive DEFAULT group ACL `setfacl -R -m g:striatum-lanes:rX <repo>` (+ default) over the repo ROOT reaches `.striatum/`, which is daemon/operator-private control-plane scratch, not shared source. I independently verified the source anchors: scratch_acl.go:31-48 grants `.striatum` only `u:<lane>:--x` traverse with the explicit `never broaden read access to private operator state` invariant; the ephemeral MCP config carrying the daemon BEARER is a 0600 file written under `.striatum/scratch` (mcpconfig.go:550-571); the current recursive repo-ACL helper is `setfacl -R` over repoRoot + `.striatum/worktrees` (repo_acl.go:25-31,:118-135) and the holder widens its single `u:<lane>` grant to a whole-group `g:striatum-lanes` grant. The result: a pool uid NOT leased to this repo gets group read on another lane's session-bound MCP bearer, PTY logs, token cache, and foreign worktrees (and `setfacl -R ...:rX` adds unconditional group `r` to existing 0600 files, so `mode is 0600` is not a rebuttal). That breaks the holder's own `private surfaces carry no group ACL` invariant, contradicts the lane-sandbox runbook's `.striatum/` carve-out, and re-introduces exactly the cross-lane control-plane replay surface RFC 0168 exists to remove. A13 does not test `.striatum/`/MCP-config/PTY-log/foreign-worktree exposure, so the hole is both real and untested. OQ4 is one of the six gate questions and the packet asks for ACLs that grant exactly-enough without over-grant; the proposed ACL over-grants. Per-OQ resolution record: OQ1 RESOLVED (sizing + fail-closed refuse), with the caveat that exhaustion accounting reads the OQ2 quarantine state that is unspecified; OQ2 NOT RESOLVED (verdict-driving — quarantine/scrubbing durable state + postcondition proof missing); OQ3 RESOLVED; OQ4 NOT RESOLVED (verdict-driving — `.striatum/` over-grant); OQ5 RESOLVED, conditioned on every attestation AND control-frame path comparing the live generation; OQ6 RESOLVED-CONTINGENT on the OQ2 scrub closing (a failed scrub otherwise leaves a credential store for the next lease). Why not reject: no path widens admin-token exposure, no minted credential carries an elevated verb, no lane-readable shared reseal bearer exists; both falsifiers confirm the no-widening invariant and recommend needs_revision; the per-lane-uid direction (D261) is the right structural move and the hard core holds. Why not accept_with_findings: OQ2 lease-lifecycle completeness and OQ4 ACL exactness are SEED gate conditions (`a SPEC that leaves ... the scrub/lease lifecycle incomplete ... has NOT cleared the gate`); a state machine that can re-lease a dirty uid and an ACL that exposes another lane's control-plane bearer are soundness/no-replay defects inside the gate, not trackable post-clearance findings — each forecloses a clearing verdict. Verdict: needs_revision. Gate-cycle note: per the SEED/packet this is the single allowed v1 revision cycle; a second needs_revision on re-attack ends the gate uncleared and routes to the operator for a fresh -v2 run with a revising holder."
findings:
  - id: HC-ORACLE-DISSOLVED
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "structural no-replay: a per-lane uid dissolves the BC1-W1-ORACLE same-uid tmux/0600/ptrace replay class on this host (Yama ptrace_scope=1)"
    challenge: "PROVEN AS FRAMED (both falsifiers credit; I independently re-verified). HC-A1: lanes launch bare tmux via `sudo -n -u <runAsUser> -- env -i` with a deterministic session name and no `-S`/TMUX_TMPDIR (pty.go:98-112,:310-314,:620-633; tmux_liveness.go:125-149), so each distinct run-as uid necessarily lands on tmux's default per-uid `/tmp/tmux-<uid>` 0700 socket and a different-uid sibling has no traverse to respawn-pane the target pane. HC-A2: a U_t-owned 0600 file in a 0700 HOME is cross-uid-unreadable under POSIX DAC. HC-A3: cross-uid ptrace/setns//proc secret reads are denied by ptrace_may_access independently of Yama — the exact axis namespace-inode failed (D261) and uid succeeds. HC-A4: the daemon control socket's SO_PEERCRED gives a meaningful uid discriminator. HC-A5: every residual same-uid surface (shared parent = only the trusted daemon; shared tmux server now per-uid; world/group-readable path closed on private surfaces; daemon-bridging is lane-vs-daemon) is named and closed. falsifier-reviewer-002 explicitly finds no residual cross-uid ptrace/setns/respawn-pane hole. The structural claim the whole RFC leans on holds; the open grounds below are in the lease-lifecycle and ACL machinery, not the hard core."
  - id: OQ2-LEASE-LIFECYCLE
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:1", "dialogue:2"]
    affected_invariants:
      - "no cross-lease same-uid residue: a returned uid must be provably-empty before re-lease, and a dirty/leaked uid must be durably non-free"
    challenge: "OPEN — verdict-driving (falsifier-reviewer-001). The durable state machine is incomplete at the failure boundary. Schema is `state active|returned` (HOLDER.md:246-257) while the prose relies on an unrepresented `quarantined` (HOLDER.md:253-255,:280-283) and the free predicate is `no active lease row` (HOLDER.md:301-302). There is no `scrubbing`/`quarantined` durable state, no allocation/scrub transaction boundary, and no scrub POSTCONDITION proof — `mark returned only after 1-3 succeed` rests on command exit codes (`sudo -n -u <pool_uid> -- kill -KILL -1` returning 0 does not prove an empty per-uid kill domain; a process can survive/reappear/recreate HOME files after the command returns). A scrub failure forces a state the table cannot express: `returned`+`scrub_status=failed` re-leases the dirty uid (same-uid cross-lease residue leak); `active` is a zombie lease for a dead session with no recovery path, no doctor surface, false exhaustion accounting. A9 asserts zero-residue positively but never defines the negative path. Live narrow teardown (supervision_control.go:557-580,:631-637) confirmed P0 must add a NEW scrub primitive. Required revision: explicit active/scrubbing/quarantined/returned states; free = only `returned` with a clean scrub generation; allocation under a lock that cannot race scrubbing/quarantined rows for the same pool_uid; a proven scrub postcondition (zero non-zombie uid-owned processes, no uid-owned tmux server/socket, credential store absent, HOME private scratch empty, reseal-token absent) before `returned`; a failed-scrub lifecycle (persist quarantined, typed `lane_uid_scrub_failed`, doctor surface, non-free across restart, recovery retry transitioning only quarantined->returned after the same proof); negative tests extending A7/A9/A10/A11. needs_revision, not reject (no widening); the SEED makes the lease/scrub/reaper completeness a clearing condition, so not a post-clearance finding."
  - id: OQ4-ACL-SCRATCH-OVERGRANT
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:1", "dialogue:3"]
    affected_invariants:
      - "ACL exactly-enough without over-grant: a non-leased pool uid must not read another lane's control-plane scratch (MCP bearer, PTY logs, token cache, foreign worktrees)"
    challenge: "OPEN — verdict-driving (falsifier-reviewer-002). OQ4's `setfacl -R -m g:striatum-lanes:rX <repo>` + default ACL recurses through `.striatum/`, which is daemon/operator-private operational scratch under the repo root, granting every pool uid (incl. a uid NOT leased to that repo) read/traverse on per-launch MCP config (0600 bearer; mcpconfig.go:550-571), PTY logs (0600; loop.go:280-300), pidfiles, the capability-token cache, and foreign per-job worktrees. I independently verified the carve-out the spec violates: scratch_acl.go:31-48 grants `.striatum` only `u:<lane>:--x` traverse with the explicit `never broaden read access to private operator state` invariant; the runbook withholds blanket `.striatum/` access (lane-sandbox.md:100-104,:348-355); the current recursive repo-ACL helper is `setfacl -R` over repoRoot + `.striatum/worktrees` (repo_acl.go:25-31,:118-135) and the holder widens its single `u:<lane>` grant to a whole-group grant; and `setfacl -R ...:rX` adds unconditional group `r` to existing 0600 files (so `mode is 0600` is not a rebuttal). This breaks the holder's own `private surfaces carry no group ACL` invariant and re-introduces the cross-lane control-plane replay surface RFC 0168 exists to remove. A13 does not test `.striatum/`/MCP-config/PTY-log/foreign-worktree exposure. Required revision: exclude `.striatum/`/`.git/`/`.gemini/`/`.claude/`/provider-auth/token-cache from the recursive group grant (prefer a source/artifact allowlist); keep `.striatum/` daemon/operator-private with only a current-lease `--x` exception; grant `.striatum/worktrees/<id>` only to the leasing uid; per-supervisor per-leased-uid ACL on `.striatum/scratch/<supervisor_id>` removed on teardown/scrub; extend A13 into a negative test over existing AND future scratch; extend `make lane-isolation-check`/doctor. needs_revision, not reject (a narrowing once exact); OQ4 is a gate question, so not accept_with_findings."
  - id: OQ-CREDITED-SET
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    affected_invariants:
      - "the credited resolved set carried into the revision (OQ1 sizing+exhaustion, OQ3 provisioning, OQ5 generation token, OQ6 hydration shape, restart-survival, narrowing)"
    challenge: "CREDITED (both falsifiers; carry into the revision unregressed). OQ1: host-global pool ceiling across all runs + typed fail-closed `lane_uid_pool_exhausted` refuse-and-requeue (never share, never auto-grow, never useradd) is the right safe default; the ONLY residue is that exhaustion accounting reads the OQ2 quarantine state that is unspecified (resolve with OQ2). OQ3: static host-runbook pool with the daemon holding only the widened launch-as-only `(%striatum-lanes)` grant and NO uid-lifecycle authority preserves the privilege boundary (A12). RESTART-SURVIVAL: session-bound uid leases in daemon-owned PostgreSQL with the free set DERIVED from the table (never memory) correctly reconstructs the binding after a fresh boot epoch (main.go:459-468/665-677) — the D261 load-bearing property — modulo the quarantine-across-restart sub-case that rides on OQ2. OQ5: the leased-uid + monotonic per-uid generation token is the right anti-recycle primitive, CONDITIONED on every attestation AND control-frame path comparing the live generation. OQ6: per-spawn per-uid RFC 0165 hydration into the leased uid's 0600 store, scrubbed on return, is the right shape — RESOLVED-CONTINGENT on the OQ2 scrub state machine closing (a failed scrub otherwise leaves a credential store for the next lease). NARROWING confirmed: no new authority, no admin-token widening, no lane-readable shared reseal bearer. Preserve all of this through the revision; do not reopen."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0168 P0 design (fresh v1, cycle 1)

author: adjudicator-author-001

> Adjudication of the FRESH v1 `falsification_gate` dialogue trajectory for
> **RFC 0168 P0** (*per-lane OS uid as the lane security principal*). The direction
> (a pre-provisioned pool of per-lane uids, leased per lane) is maintainer-ratified
> (**D261**, 2026-06-24) and is **not** relitigated. This run hardens the spec: prove
> the per-lane uid dissolves the `BC1-W1-ORACLE` same-uid replay class that made
> RFC 0143's authenticated reseal channel unsolvable across seven cycles, and discharge
> the six design-gate open questions into build-bearing, falsifiable constraints.
> Inputs read: the Holder spec (`dialogue/holder/HOLDER.md`), both falsifier re-attacks
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`), the
> `SEED.md` charter, RFC 0168 (`docs/rfcs/0168-per-lane-security-principal.md`), and the
> RFC 0143 v7 adjudicator ledger that named `BC1-W1-ORACLE`
> (`docs/operator/artifacts/rfc-0143-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`).
> No raw terminal output was read. Load-bearing source citations were independently
> re-verified against the current worktree HEAD.

## Verdict

**verdict: needs_revision**

This v1 is a strong spec that closes most of the surface. The **structural claim is
proven** and most of the six open questions are genuinely discharged. But **two new,
material, source-anchored challenges land — both inside SEED gate conditions — and
both stand unrebutted** (one holder turn, two independent falsifier re-attacks, the
cycle ends at adjudication). A clearing verdict requires the lease/scrub/reaper
**complete** and ACLs **exactly-enough without over-grant**; neither holds.

**Why not `reject`.** No path widens admin-token exposure, no minted credential carries
an elevated verb, and no lane-readable shared reseal bearer exists. Both falsifiers
confirm the no-widening invariant and recommend `needs_revision`. The per-lane-uid
direction is the right structural move (D261) and the hard core holds — the defects are
soundness gaps in the lease-lifecycle and ACL machinery, not a widening.

**Why not `accept_with_findings`.** OQ2 lease-lifecycle completeness and OQ4 ACL
exactness are SEED gate conditions (*"a SPEC that leaves … the scrub/lease lifecycle
incomplete … has NOT cleared the gate"*). A state machine that can re-lease a **dirty**
uid and an ACL that exposes another lane's control-plane **bearer** are no-replay/
soundness defects inside the gate, not trackable post-clearance findings; each
forecloses a clearing verdict.

## The clearing condition, walked

A clearing verdict requires **all** to hold:

1. **Structural claim proven (per-lane uid dissolves `BC1-W1-ORACLE`) — HOLDS.** Both
   falsifiers credit HC-A1..A5; I independently re-verified the launch path and the
   per-uid tmux socket / cross-uid DAC+ptrace denial (finding `HC-ORACLE-DISSOLVED`).
2. **Lease/scrub/reaper complete — FAILS.** The durable state machine cannot represent
   a returned-but-dirty uid; a scrub failure either re-leases a dirty uid or strands a
   zombie-active lease (CHALLENGE 1 / `OQ2-LEASE-LIFECYCLE`).
3. **Restart-survival established — HOLDS** (modulo the quarantine-across-restart
   sub-case, which rides on the OQ2 fix). The DB-derived free set is the correct answer
   to the boot-epoch rotation D261 targets.
4. **Exhaustion safe — HOLDS in the happy path**, but its accounting reads the
   unspecified OQ2 quarantine state, so it inherits the OQ2 gap.
5. **Provisioning coherent with the daemon privilege boundary — HOLDS** (OQ3: static
   runbook pool, launch-as-only, no uid-lifecycle authority).
6. **Attestation re-binds + recycle-confusion prevented — HOLDS** (OQ5 generation
   token), conditioned on every attestation **and** control-frame path comparing it.
7. **Credential store no stale-copy/cross-uid leak — HOLDS contingent on OQ2** (a
   failed scrub leaves a store for the next lease).
8. **ACLs exactly-enough without over-grant — FAILS.** The recursive group ACL reaches
   `.striatum/` control-plane scratch (CHALLENGE 2 / `OQ4-ACL-SCRATCH-OVERGRANT`).
9. **Change is a narrowing — HOLDS** (no new authority; both falsifiers confirm).
10. **No standing falsifier challenge — FAILS.** Two material challenges stand
    unrebutted.

## Per–open-question resolution record

| OQ | Topic | Status |
| --- | --- | --- |
| **OQ1** | Pool size + exhaustion | **RESOLVED** (host-global ceiling + typed fail-closed refuse-and-requeue), with the exhaustion-accounting caveat that it reads the unspecified OQ2 quarantine state |
| **OQ2** | Lease/scrub/reaper + restart-survival | **NOT RESOLVED — verdict-driving.** Quarantine/scrubbing durable state + scrub-postcondition proof missing; restart-survival itself is otherwise sound |
| **OQ3** | Provisioning ownership | **RESOLVED** (static host runbook; daemon launch-as-only; no useradd/userdel authority) |
| **OQ4** | ACL interaction | **NOT RESOLVED — verdict-driving.** Recursive group ACL over-grants `.striatum/` control-plane scratch |
| **OQ5** | Attestation + recycle token | **RESOLVED**, conditioned on every attestation **and** control-frame path comparing the live uid-lease generation |
| **OQ6** | Per-uid credential store | **RESOLVED-CONTINGENT** on the OQ2 scrub state machine closing |

## What v1 genuinely proved (credited)

- **The hard core.** A per-lane uid dissolves `BC1-W1-ORACLE` on this host: distinct
  run-as uids land on tmux's default per-uid `0700` socket (so a sibling cannot
  `respawn-pane` the target pane), a `0600`/`0700` lane-owned file is cross-uid
  unreadable, cross-uid `ptrace`/`setns`/`/proc` reads are denied by
  `ptrace_may_access` (the exact axis namespace-inode failed under D261), `SO_PEERCRED`
  gives a meaningful uid discriminator, and every residual same-uid surface is named and
  closed. Verified against `pty.go:98-112,:310-314,:620-633` and
  `tmux_liveness.go:125-149`.
- **Restart-survival.** Binding the uid to the **session** in daemon-owned PostgreSQL
  and **deriving** the free set from the table (never from memory) is the correct answer
  to the boot-epoch rotation the whole of RFC 0143 targets.
- **The privilege boundary (OQ3).** A static host-runbook pool with the daemon holding
  only the widened launch-as-only `(%striatum-lanes)` grant — and no `useradd`/`userdel`
  authority — keeps RFC 0168 a narrowing.
- **The recycle token (OQ5)** and **per-spawn per-uid hydration (OQ6)** are the right
  shapes; the no-widening invariant holds throughout.

## The two open grounds (independently confirmed against the worktree)

### CHALLENGE 1 — OQ2-LEASE-LIFECYCLE: a returned-but-dirty uid is not representable (verdict-driving)

The holder's `lane_uid_leases` schema is `state active|returned`, but the prose relies
on an unrepresented `quarantined` state and a free predicate of *"uids with no active
lease row."* There is no `scrubbing`/`quarantined` durable state, no transaction/lock
boundary preventing allocation while scrub is in progress, and no scrub **postcondition
proof** — *"mark `returned` only after 1–3 succeed"* rests on command exit codes, and
`sudo -n -u <pool_uid> -- kill -KILL -1` returning `0` does not establish an empty
per-uid kill domain (a process can survive in `D` state, reappear, or recreate HOME
files after the command returns). On a scrub failure the daemon must record *"U is not
leased to a live session, but also not free"* — a state the table cannot express. Mark
it `returned`+`scrub_status=failed` and the free-set rule re-leases the dirty uid to S2
(a **same-uid cross-lease residue leak** — exactly the class RFC 0168 must eliminate);
leave it `active` and it is a zombie-active lease for a dead session with no recovery
path, no `doctor` surface, and false `lane_uid_pool_exhausted` accounting. The OQ5
generation token does not rescue this: it guards daemon-side attestation, not same-uid
local reads / tmux / file interactions inside U's OS domain. A9 asserts zero-residue
**positively** but never defines the negative path. The SEED makes *the lease/scrub/
reaper complete* and *exhaustion safe* clearing conditions, so this is a gate hole.

### CHALLENGE 2 — OQ4-ACL-SCRATCH-OVERGRANT: the recursive group ACL reaches `.striatum/` (verdict-driving)

OQ4's `setfacl -R -m g:striatum-lanes:rX <repo>` (+ the default group ACL) recurses
through `.striatum/`, which is **daemon/operator-private operational scratch** under the
repo root — not shared source. I independently verified the carve-out the spec violates:

- `scratch_acl.go:31-48` grants `.striatum` only `u:<lane>:--x` (traverse) with the
  explicit invariant *"never broaden read access to private operator state"*.
- The ephemeral MCP config carrying the daemon **bearer** is a `0600` file written under
  `.striatum/scratch` (`mcpconfig.go:550-571`); PTY logs are `0600`
  `.striatum/scratch/<supervisor_id>/pty.log` (`loop.go:280-300`).
- The current recursive repo-ACL helper is `setfacl -R` over `repoRoot` +
  `.striatum/worktrees` (`repo_acl.go:25-31,:118-135`); the holder widens its single
  `u:<lane>` grant to a whole-group `g:striatum-lanes` grant — so **every** pool uid,
  including one **not leased to this repo**, gets group read on another lane's
  session-bound MCP bearer, PTY logs, token cache, and foreign worktrees.
- `setfacl -R …:rX` adds **unconditional** group `r` to existing `0600` files, so
  *"the mode is `0600`"* is not a sufficient rebuttal after the recursive grant.

This breaks the holder's own *"private surfaces carry no group ACL"* invariant,
contradicts the lane-sandbox runbook's `.striatum/` carve-out, and re-introduces the
cross-lane control-plane replay surface RFC 0168 exists to remove. A13 does not test
`.striatum/` / MCP-config / PTY-log / foreign-worktree exposure, so the hole is both
real and untested. OQ4 is one of the six gate questions; the proposed ACL over-grants.

## Falsifier challenge dispositions

- **falsifier-reviewer-001 — OQ2 quarantine/scrub state machine (material; landed
  unrebutted).** Claim challenged: a returned uid is safe to re-lease. Material? **Yes**
  — it exposes a real same-uid cross-lease residue leak on the failure path and would
  change the spec (explicit states, allocation lock, scrub postcondition proof, a
  failed-scrub lifecycle, negative tests). Rebutted? **No** — the holder had no further
  turn; the strongest implicit rebuttal (`scrub_status` encodes failure while the row
  stays `active`) is not the specified falsifiable state machine and redefines "active".
  Disposition: **`OQ2-LEASE-LIFECYCLE` open; verdict-driving.** No widening;
  `needs_revision`.
- **falsifier-reviewer-002 — OQ4 `.striatum/` ACL over-grant (material; landed
  unrebutted).** Claim challenged: non-leased repo read is harmless. Material? **Yes** —
  `.striatum/` carries control-plane bearer material the product keeps lane-private, and
  the recursive group ACL exposes it to non-leased pool uids. Rebutted? **No.** Credits
  the hard core (no residual cross-uid `ptrace`/`setns`/`respawn-pane` hole), the OQ5
  token, and the OQ6 shape (contingent on the OQ2 fix). Disposition:
  **`OQ4-ACL-SCRATCH-OVERGRANT` open; verdict-driving.** No widening; `needs_revision`.

Both falsifiers **credit the structural hard core** and then land **distinct,
independent, source-confirmed** new grounds in **different** gate questions (OQ2 vs
OQ4) — strong corroboration that the residue is genuine, not reviewer idiosyncrasy.

## What the next revision MUST fix to clear on re-attack

Retain the entire credited set (the proven hard core HC-A1..A5; restart-survival via the
DB-derived free set; OQ1 host-global ceiling + typed fail-closed refuse; OQ3
launch-as-only provisioning; the OQ5 generation token; the OQ6 per-spawn hydration
shape; the no-widening invariant). Then, in addition:

1. **(OQ2-LEASE-LIFECYCLE) Make a dirty/leaked uid impossible to re-lease as an
   executable state machine.**
   - Add explicit lease states `active` / `scrubbing` / `quarantined` / `returned`; define
     the free predicate as **only** `returned` rows with a clean scrub generation (never
     "no active row").
   - Allocate under a transaction/advisory lock that cannot race a `scrubbing` or
     `quarantined` row for the same `pool_uid`.
   - Define the scrub **postcondition** (not just commands): zero non-zombie uid-owned
     processes that can run user code, no uid-owned tmux server/socket, credential store
     absent, HOME private scratch empty/reset, reseal-token path absent — mark `returned`
     **only** after that proof.
   - Define the failed-scrub lifecycle: persist `quarantined`, emit typed
     `lane_uid_scrub_failed`, expose it in `doctor`, keep it non-free **across restart**,
     and provide a recovery/operator retry that transitions **only** `quarantined →
     returned` after the same proof.
   - Extend A7/A9/A10/A11 with negative tests (injected scrub failure quarantines and is
     never allocated; restart preserves quarantine; exhausting clean uids with one
     quarantined yields typed exhaustion, not dirty reuse or shared-uid fallback).
2. **(OQ4-ACL-SCRATCH-OVERGRANT) Make the ACL boundary exact.**
   - Do **not** apply `g:striatum-lanes:rX` / a default group ACL recursively to the raw
     repo root unless `.striatum/`, `.git/`, `.gemini/`, `.claude/`, provider-auth, and
     token/cache paths are explicitly excluded; prefer an allowlist of source/artifact
     paths.
   - Keep `.striatum/` daemon/operator-private (only `--x` to the **currently leased**
     uid when needed); grant `.striatum/worktrees/<id>` only to the **leasing** uid; use
     a per-supervisor, per-leased-uid ACL on `.striatum/scratch/<supervisor_id>` removed
     on teardown/scrub.
   - Extend A13 into a negative test
     (`TestPoolACLDoesNotExposeOperationalScratchOrForeignWorktrees`) over seeded
     **existing and future** scratch (MCP config, token cache, PTY log, foreign worktree,
     legacy `.gemini`/`.claude`); extend `make lane-isolation-check` / `doctor` to fail
     when `.striatum/` or token-bearing paths carry `striatum-lanes` group read outside
     the explicit current-lease exception.

Everything else carries forward **unchanged** (do **not** reopen): the proven hard core,
restart-survival, OQ1 sizing + fail-closed exhaustion, OQ3 provisioning, the OQ5
generation token, the OQ6 hydration shape, and the narrowing invariant. Resolving OQ2
also closes the OQ1 exhaustion-accounting caveat and the OQ6 stale-store contingency,
since both read the quarantine state OQ2 will define.

## Gate-cycle note

Per the SEED and the packet objective this is the **single allowed v1 revision cycle**:
a revising holder gets one pass to close `OQ2-LEASE-LIFECYCLE` and
`OQ4-ACL-SCRATCH-OVERGRANT` while carrying the credited set forward unregressed. A second
`needs_revision` on re-attack ends the gate **uncleared** and routes to the operator
(who spins a fresh `-v2` run with a revising holder). The maintainer-ratified
**direction** (per-lane pooled OS uid, D261) carries regardless of this verdict;
adjudicator clearance gates the **spec's soundness**, not the product call.

---
<sub>Adjudicator collaboration ledger for the RFC 0168 P0 `falsification_gate` design
run (fresh v1, cycle 1). The ledger verdict — not falsifier completion — gates the
phase: `needs_revision` returns the spec uncleared. v1 PROVES the hard core (a per-lane
uid dissolves `BC1-W1-ORACLE` on this host: per-uid tmux socket, `0600` DAC, cross-uid
`ptrace`/`setns` denial, `SO_PEERCRED` uid discriminator, every residual same-uid
surface closed — both falsifiers credit it) and genuinely discharges OQ3 (provisioning),
OQ5 (the recycle generation token), restart-survival (DB-derived free set), and the
narrowing invariant; OQ1/OQ6 are resolved-contingent on OQ2. But two material,
source-confirmed challenges stand unrebutted inside the gate: OQ2's lease lifecycle
cannot durably represent a returned-but-dirty/quarantined uid or prove an empty scrub
postcondition, so a scrub failure re-leases a dirty uid or strands a zombie-active lease;
and OQ4's recursive `g:striatum-lanes:rX` ACL over the repo root reaches `.striatum/`
control-plane scratch (the `0600` MCP bearer, PTY logs, token cache, foreign worktrees),
re-introducing the cross-lane replay surface the RFC exists to remove. No admin-token
widening, no lane-readable shared reseal bearer, no elevated credential — `needs_revision`,
not `reject`; both gaps are inside SEED gate conditions (lease/scrub completeness, ACL
exactness), so not `accept_with_findings`.</sub>
