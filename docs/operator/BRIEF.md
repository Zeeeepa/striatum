---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-24_v2.37.1-deployed"
supersedes: "brief_2026-06-24_audit-closeout-gates"
scope_links: ["docs/operator/plans/provenance-durability-campaign-2026-06-14.md", "docs/operator/plans/rfc-0126-0128-implementation-campaign-2026-06-14.md", "docs/rfcs/0126-multi-reviewer-revision-coherence.md", "docs/decisions/decision-log.md", "CHANGELOG.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: operator-claude-opus-4-8-001

## 2026-06-24 delta — v2.37.1 hotfix release

**v2.37.1 (2026-06-24)** supersedes v2.37.0 for deployment. The v2.37.0
release artifact installed and owner bundle **0022** applied successfully, but
the daemon then refused startup with
`schema stamps capability "operator_identity_run_attribution" this binary does
not support`: the new 0022 read-projection stamp was present in
`readScopeReasserts` but missing from `SupportedAuthorityCapabilities()`.
v2.37.1 adds that capability to the supported daemon-authority inventory and
adds a unit guard that every write/read reassertion stamp is declared supported.

**Local deploy status:** v2.37.1 is published and deployed on this host from the
GitHub release artifact. The hosted release workflow passed the archive build,
installed-CLI gate, and release publication. The linux-amd64 tarball checksum
verified, owner bundle 0022 re-apply is idempotent, the system daemon is running
v2.37.1 (`48aa4a17`, clean), and `doctor` is green with `problem_count=0`,
owner bundle 22 in sync, schema version 44, and schema drift in sync.

## 2026-06-24 delta — v2.37.0 release

**v2.37.0 (2026-06-24)** cuts the post-v2.36.0 source state: RFC 0167 P0
operator identity/run attribution, session-bound `operator.bootstrap`, owner
bundle **0022**, RFC 0143 Slice A recovery legibility, D269/#527 fan-in barrier
default-live cutover, and the D264-D269 audit closeout gates. The release also
adds a mechanical README release-row gate:
`scripts/check_release_version.py` runs through `make check-docs` and fails when
`VERSION`, the README Project Status version row, and the matching CHANGELOG
release header disagree.

**Superseded deploy note:** do not deploy v2.37.0 after owner bundle 0022. It
omits the `operator_identity_run_attribution` capability from the startup parity
inventory and will refuse the schema after the DDL is applied. Deploy v2.37.1
instead.

## 2026-06-24 delta — audit closeout gates

The 2026-06-24 architecture-audit closeout added D264-D269. Operators must not
launch new feature-wave RFC design/build work while `striatum doctor` is red.
Use direct sync-guarded operator commits, not daemon dogfood flows, for narrow
source/truth fixes until integrity is green again. `docs/operator/rfc-roadmap.md`
now carries the active WIP cap, self-hosting-tax classification, and
subtraction-release checklist. D269 closes the #527 source cutover with PG/unit
proof: fan-in is live by default, `STRIATUM_BARRIER_FANIN=0` is the kill switch,
and live deployment equivalence is now the post-green validation path.

## 2026-06-24 delta — RFC 0167 P0 built + verified + integrated

RFC 0167 P0 (operator identity & run attribution, D260/D263) is **on `main`**
(`525c4696`), landed autonomously through Striatum's own design → build → verify
workflows. **Design:** a `falsification_gate` committee ran **4 cycles**, each
surfacing a real, source-verified, build-breaking defect (pre-run session
impossible under `sessions.run_id NOT NULL`; the run-origin stamp hitting the
0006 token read-scope `42501`; the operator token lacking `run.prepare` admin;
the operator-session token over-granting + a composed `client_id→principal_id`
re-leak) before clearing `accept_with_findings` with two binding §F constraints.
**Build:** a `code_change` run implemented all ten §9 items — owner bundle
**0022** (`operator_handles` + `operator_sessions` + `runs.created_by_principal_id`
/`created_by_handle_id` write-once trigger + the SECURITY DEFINER identity
projections `run_origin_identity`/`runs_for_origin_client`/`runs_missing_origin`
+ the `runs` REVOKE/re-GRANT + `operator_handles`/`operator_sessions`
column-scoped grants + the three `runs` star-reader conversions),
`mintOperatorSessionToken`, the `operator.bootstrap`/`heartbeat`/`close` RPCs
with the `striatum operator bootstrap` CLI as their client, `striatum whose`,
`status --mine`, and the `attribution_unknown` doctor advisory. **Verify:** all
**10 live two-role pgtests PASS** under the non-superuser owner DSN (the C2″
composed-route closure, the write-once trigger, the two-`maya` disambiguation,
the operator-token authorization, the drift reassert); `go build`/`go vet` green.

Bundle 0022's deployable release is v2.37.1. Apply it only with a binary that
declares `operator_identity_run_attribution` in
`SupportedAuthorityCapabilities()`, then restart the daemon; after that, `whose`,
`status --mine`, and the operator-bootstrap mint RPC are live. **P1–P3** (custody
log; honest bylines + handoff naming + chips + opt-in OSC title; lineage) are
sequenced behind this P0 release/deploy.

## 2026-06-23 delta — v2.36.0 released

**v2.36.0 (2026-06-23)** — a bugfix-only cut over v2.35.0. No new RFC
implementations: the in-flight RFC 0142 P4 / 0143 / 0165 work is design-phase
only and rides along as inert design artifacts. Fixes: owner-bundle watermark
read grant (owner bundle 0020, #581) plus its `42501` clean-halt classify so a
deploy no longer crash-loops (~112s → one clean exit 79); `doctor` no longer
false-reds superseded run-branch artifacts; checkpoint artifact-integrity
preflight on `resolve continue/override`; and the GitHub Release publish
pipeline (#582 — codex installed-CLI seat + 30m `installed-cli-check` timeout,
gate re-coupled). **Deploy:** run `striatum daemon owner-ddl apply` before
restarting onto the new binary (owner bundle 0020, or the daemon clean-halts
exit 79); local prod was already 20/20 in-sync at cut time, so the tag mostly
formalizes code already proven live. Cut via direct sync-guarded commit + tag
`v2.36.0` (no PR).

## 2026-06-23 delta — RFC 0165 v2 quarantined on artifact integrity

RFC 0165 design v2 / #583 must not be continued or overridden: run
`run_02c4fc6ad7cb5092ae4d5c67651e22a8` is parked at blocker
`blk_66f3a29175ac2a58d509fa790e59c519`; ledger artifact `art_ae48cc3014f1ecad37303d8f0ab49b57` is unreadable. Treat v2 notes as
diagnostic-only; repair runner artifact integrity, then start v3 from a
refusal/coverage matrix. See `docs/operator/artifacts/rfc-0165-design-v2/QUARANTINE.md`.

## 2026-06-23 delta — owner-bundle watermark read hotfix

Restarting the v2.35.0 daemon onto the RFC 0142 Layer 2 owner-bundle watermark
interlock exposed #581's grant gap: `striatumd_rw` could not `SELECT`
`striatumd.owner_bundle_meta`, so the daemon cleanly halted with exit 79 even
after `striatum daemon owner-ddl apply` reported bundle 0019 current. Hotfix
owner bundle **0020** grants runtime-role `SELECT` on `owner_bundle_meta` while
keeping the table owner-owned and write-owner-only; the read-authority inventory
now classifies it as `runtime_parity_select`.

## 2026-06-22 delta — v2.35.0 released

**v2.35.0 (2026-06-22)** — large feature-wave cut: 207 commits since v2.34.1
(20 feat / 48 fix), ~12 RFC graduations incl. 0128 P0 (cross-repo write-scope
guardrail), 0135 (barrier cutover), 0136 P1, 0137 (Prometheus exporter), 0141
(gate attestation), 0142 P0–P3 (safe DB-change deployment), 0163/0164. Process:
**no GitHub PRs** — `main` via daemon run-integration or direct sync-guarded
commit (AGENTS.md); + shared-checkout hygiene policy. RFC 0128 P0 re-landed via a
daemon `code_change` run (#575), not a hand-merge; release gated on green CI after
the `schema_state` authority-inventory fix (#570).

## 2026-06-16…21 deltas (pre-v2.35; see CHANGELOG + decision log)

RFC 0142 P0 (3-stage self-host dogfood, PR #553, runner defect #551); the
lane-perms ACL cluster (#537/#539 fixed, #512 → RFC 0143); and #515 (builtin
verifier verifies a nested Go module, PR #528). Details in CHANGELOG.

## 2026-06-18 deltas — v2.34.1 / v2.34.0 (superseded; see CHANGELOG)

**v2.34.1** docs/maintenance cut (#405–#408). **v2.34.0** six reliability/security
fixes (#355–#363) + the RFC 0135 `(entity, seal)` sealed-barrier primitive (P0–P6,
migrations 0029–0032 + owner bundle 0013), shipped opt-in/shadow and since cut over
live (RFC 0135 graduation rolled into v2.35.0).

## 2026-06-17 delta — #311 P0 per-job quarantine (D209)

#311 P0 (per-job quarantine + run finalize-the-majority) is IMPLEMENTED on a
feature branch (not yet deployed). When a single job exhausts its recovery
budget but its downstream is clear, the decision tree now quarantines ONLY that
job and finalizes the run on its completed deliverables, instead of wedging the
whole run at `needs_operator` (the #311 incident). New non-terminal job state
`quarantined` (owner bundle `0012` — apply `striatum daemon owner-ddl apply`
before the new daemon image; bundle 0011 reserved for #330). New verb
`recovery accept-quarantined <run-id> <job-id>` is the operator's narrow action
(resolves the blocker, marks the job canceled-by-operator). Eligibility is
gated on a transitive-downstream check, the RFC 0118 provenance gate, a per-run
cap (`recovery_policy.max_quarantinable_jobs`, default 1), and the owner-bundle
having landed — any guard failing falls back to the unchanged whole-run
escalation. See D209, CHANGELOG, and `spec.md` (completion section).

## 2026-06-16 delta — reliability reset closeout

`striatum-reliability-reset-2026-06-16` completed on `proximal` as
`run_8489e7d2df3b56e1ed7fdb49ff5c8ba7` with all 8 jobs completed,
`completion_mode=lanes_attested`, and accepting verdicts with findings on every
review gate. The useful outputs are durable run artifacts:

- `RESET_SYNTHESIS.md` and `SUPPORT_LEDGER.md` are blob-backed, verified in the
  run completion record.
- Checked-out finding artifacts live under
  `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/`.
- `FINAL_REVIEW.md` accepted the reset plan only with release-gate conditions:
  one current-state issue frontier, no stale README status, closed #302/#308/#309
  regressions kept covered by a live recovery fixture, bounded doctor warnings,
  and no feature growth until those gates pass.
- The run itself reproduced the final-review `agent_exited_unsealed` class
  twice before a fresh lane sealed the verdict. Recovery stayed daemon-bound:
  `recovery requeue-stale`, scoped operator decision
  `DECISION-striatum-reliability-reset-final-review-requeue-2026-06-16`, and
  `escalation resolve`.

Live GitHub open issues were rechecked on 2026-06-17T00:29Z and #329 was fixed
in the read-side helper-event drain authority slice. The current issue frontier
is the 18 open GitHub issues listed in [Blockers / Open Issues](#blockers--open-issues-18);
older #212/#263-#267 text is historical only.

## State

Latest release is **v2.37.1 (2026-06-24)** — hotfixes the v2.37.0 owner-bundle
0022 capability-parity deploy skew after shipping operator identity/run
attribution (RFC 0167 P0, owner bundle 0022), session-bound operator bootstrap,
RFC 0143 Slice A recovery legibility, D269 fan-in barrier default-live cutover,
and the audit closeout gates. **v2.36.0 (2026-06-23)** was a bugfix-only cut over
v2.35.0 (#581 owner-bundle watermark deploy crash-loop, #582 release publish
pipeline, doctor superseded-artifact false-red, checkpoint artifact-integrity;
owner bundle 0020). **v2.35.0 (2026-06-22)** — a large feature-wave cut (207 commits
since v2.34.1; ~12 RFC graduations incl. 0142 P0–P3; see the top delta).
**v2.34.1 (2026-06-18)** was a docs/maintenance cut (no code change). **v2.34.0
(2026-06-18)** packaged six reliability/security fixes +
the RFC 0135 sealed-barrier primitive (opt-in/shadow). The earlier
**v2.33.0 (2026-06-16)** tag at `564a8209` packaged the post-v2.32.0 landing set:
doctor integrity legibility **P0+P1** (D204/D205, #300), **#290/D206** fan-in
run-branch integration, **#296** codex push-lane loud-fallback, **#301/#307**
workflowgenerate fixes, **#304** dangling-blocker resolution, and **#311**
recovery-escalation legibility (details in CHANGELOG `v2.33.0`). The prior
**v2.32.0** (2026-06-13) packaged RFC 0118/0119/0120 plus session-recovery edge
fixes; the v2.10.0 → v2.31.0 release burst is indexed in CHANGELOG and the
decision log. Historical open sets such as #212/#263-#267 are no longer current.

## Deep architecture review 2026-06-11 — mostly closed in v2.34.0

`STRIATUM_DEEP_ARCHITECTURE_REVIEW_CLAUDE_FABLE_5_2026-06-11.md` (`0e8671ed`):
verdict **ROUGHLY RIGHT-SIZED · ON TRACK**. v2.34.0 closed the ranked blocker,
untested spine, scoped deletion pass, conformance honesty, truth mechanization,
and boundary-hygiene batch. **STILL OPEN:** P1 token-out-of-argv and
`docs/operator/` exhaust relocation (#364, ready-for-human).

## Current Frontier

- **Reliability reset gate is active.** Do not start feature growth, new
  workflow-shape graduation, broader auto-spawn authority, or release work until
  the trust-restoration gates below are green or explicitly quarantined with
  owner, reason, and removal condition.
- **Recovery fixture gate:** #302/#308/#309 are closed, but their failure class
  remains load-bearing evidence: prove `agent_exited_unsealed` plus durable,
  valid artifacts reaches completion without renewed-lease waiting, then keep
  `striatum doctor --json` green.
- **Docs/current-state truth:** this brief, README status, docs index, and
  roadmap/todo references must share one current issue frontier. This revision
  updates the brief/README/index; roadmap/todo guardrails remain follow-up work.
- **RFC 0120 (await-packet idle exit + wake boundary, D180) — LANDED.**
  Phase 1 terminal idle envelopes carry `idle_behavior=exit_session`;
  bootstrap no longer tells lanes to poll after `no_work`; the PTY receiver
  exits the lane cleanly. Phase 2 landed on main in `81b51959`: the
  notify-only wake bus adds read-shaped `wake.wait`, post-commit wake hints
  for work/message/turn availability, and `run drive` wake waits with bounded
  missed-notification fallback. Wake events stay hints over committed state,
  never authoritative. The earlier `issue-248-wake-bus-implementation` runs
  were canceled/superseded dogfood attempts; do not drive them as live work.
- **RFC 0119 (warm-tier memory boundary, D179) — accepted; hot tier
  implemented.** Authorizes the `hippo`/`fornix` warm-tier adjunct (separate
  repo, `~/git/hippo`) + a striatum-native read-only hot tier (`recall.*`
  over the daemon's own artifact stream, scaffold-time digest injection,
  default-off redacted `lane_trajectory` export, `progress_note`-only git
  eviction). The hot tier shipped (`recall.*`, `RecallMemory`, commit
  `80dc82e7`) with C1-C4 discharged; only the runtime evictor (D193) remains
  deferred. No
  `memory.*` capability, no retrieval-dependent state transition.
- **RFC 0118 (#240)** implementation is on main and the issue is closed:
  frozen verdict provenance stamps, override posture/basis, completion
  provenance gate + `needs_operator` escalation, durable
  `run_completion_record`, and `recovery.invalidate_job` supersede receipts.
  The accumulated post-v2.31.0 work shipped in v2.32.0.
- **Live housekeeping:** `doctor` is OK (0 problems) but still warns that
  the local Codex config points at a stale MCP endpoint unless launched
  through `striatum codex`. The worktree-ref-safety/run-drive residue in
  #259/#260/#261, the config crash-loop recovery in #262, and the
  blob-gated artifact-anchor doctor check in #217 are closed on `main`.

## Next Actions

1. **Keep current-state docs truthful:** after every issue-closeout or release,
   refresh this brief, README status, docs index summaries, and any roadmap/todo
   surface that claims to list current open work; the README version row is now
   mechanically gated by `make check-docs`.
2. **Work the active defect frontier first:** #612, #579, #576, #512, and #506
   are the current operator-facing recovery defects.
3. **Bound doctor warnings:** keep `problem_count=0`, but turn the warning channel
   into named classes with allowed baselines/deltas.

## Blockers / Open Issues (21)

Open GitHub tracker state rechecked on 2026-06-24 after the v2.37.1 deploy.

- **Active defects / recovery:** #612 cross-user falsifier handoff publish wedge,
  #579 idle-stalled builder lane blocks downstream jobs, #576 lease-warmed lane
  never completes, #512 boot-epoch rotation reseal blocked by shared-lane token
  ownership, #506 reviewer over-rejection/blob-exhaust legibility.
- **Reliability/security follow-ups:** #593 retrospective, #592 RFC 0142 P4
  activation/verify run, #590 gate-compute timing, #589 structural-root precheck,
  #588 falsification recursion tripwire, #587 auto-bank/rescaffold clean revision
  cycles, #585 RFC 0143 Slice B blocked on per-lane security principal.
- **Feature/design backlog:** #611/#610/#609 RFC 0167 P3/P2/P1, #578 schema-drift
  refuse-to-serve flip, #577 verified-stale rung, #572 RFC 0142 P5 rehearsal
  receipt, #569 provider-auth absence-of-success alerting, #387 events/audit-log
  partitioning, #380 remaining git-hoist lock holders.

## Hazards / Do Not

- **Operators scaffold dogfoods; they do not implement role artifacts.**
- **Hold the anti-bets** (review §F.4 + decision log): no new shapes while
  the RFC 0106 freeze holds; no daemon auto-spawn before the D175 evidence
  trigger; no Engram/memory absorption (D179 boundary is narrow and
  test-gated); no hosted/multi-tenant anything.
- Stay on the daemon boundary: no direct Postgres, no tmux/marker-file
  state, no telemetry/transcript capture without a product decision.
- Trust only returned JSON; verify every state-changer with a read;
  state-changing calls sequential.
- The daemon runs as the **system** unit (`/etc/systemd/system/striatumd.service`);
  restart with `sudo systemctl restart striatumd` (NOT `--user`). `make install`
  does NOT restart it; build from a clean worktree off `origin/main` (not the
  contended shared tree), then verify the running `/proc/<pid>/exe` sha.
- **The user-scope `striatumd.service` is masked/removed — do NOT recreate it via
  `striatum daemon install`.** It lacks the owner-DB env (`STRIATUM_OWNER_DB_URL`
  …) so it crash-loops on `daemon_pg_owner_bootstrap_failed` and conflicts with
  the system unit (recurring daemon-down incidents). Fix the install path before
  un-masking — see #509.
- CI always runs pgtests; check `gh run list` before assuming green.
  Reproduce lint locally with golangci-lint v2.12.2 (pinned in
  `go/Makefile`; absent binary = invisible red).
- Concurrent agents sweep the shared tree: commit deliverables same-turn;
  land code via isolated worktrees off `origin/main`.

## Pointers

- `STRIATUM_DEEP_ARCHITECTURE_REVIEW_CLAUDE_FABLE_5_2026-06-11.md` — the
  standing work-list (§E missing pieces, §G recommendations)
- `docs/rfcs/0118-gate-run-completion-on-attested-provenance.md`
- `docs/rfcs/0119-warm-tier-memory-boundary.md` (+ hippo RFC 0001)
- `docs/rfcs/0120-await-packet-idle-exit-and-wake-boundary.md`
- `docs/rfcs/0116-zero-operator-touch-dag.md` / `0117-worktree-branch-ref-safety.md`
- `docs/decisions/decision-log.md` (D161–D181 cover this brief's span)
- `CHANGELOG.md` (v2.10.0 → v2.37.1 + Unreleased)
- `docs/reference/command-authority-matrix.md` (lags 16 live methods —
  reconcile on contact, per AGENTS rule)
