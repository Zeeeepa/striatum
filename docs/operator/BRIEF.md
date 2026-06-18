---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-18_v2.34.0-release"
supersedes: "brief_2026-06-16_reliability-reset-closeout"
scope_links: ["docs/operator/plans/provenance-durability-campaign-2026-06-14.md", "docs/operator/plans/rfc-0126-0128-implementation-campaign-2026-06-14.md", "docs/rfcs/0126-multi-reviewer-revision-coherence.md", "docs/decisions/decision-log.md", "CHANGELOG.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: operator-claude-opus-4-8-001

## 2026-06-18 delta — v2.34.0 released

**v2.34.0** packages six deployed reliability/security fixes plus the RFC 0135
sealed-barrier primitive. Fixes (merged, daemon redeployed, `doctor` green, issues
closed): #355 recovery-reconcile convoy (SQLSTATE 57014, pre-tx oracle), #356
untested-spine tests + `escalation.resolve` run-lock, #357 dead-code deletion
(~1280 LOC, supervisor liveness twin), #358 boundary/security batch (`seenRequests`
DoS cap, CLI read deadline, RFC 0111 suggestions, FIFO delivered-lie, CSP, blob
flag), #363 `supervise.rebridge` on-contract + registry-guard blind-spot, #359b
docs/index. This closes most of the 2026-06-11 deep-review P0/P1 work-list (untested
spine, deletion pass, conformance honesty, truth mechanization, boundary hygiene).

**RFC 0135 (D214/D215/D216)** — the unified `(entity, seal)` sealed-expectation
barrier across fan-in / quorum / 0095-revision / 0108-integrate — is fully
IMPLEMENTED (P0–P6; migrations 0029–0032 + owner bundle 0013) but **opt-in/shadow:
D206 stays the default, ZERO behavior change**. Go-live (apply owner bundle 0013 +
flip each gate to consume the primitive) is DEFERRED. Remaining: P4b (#341/#342),
#343, ready-for-human #361/#362/#364, #372. See the RFC 0135 slice plan and
`/tmp/striatum-handoff-rfc0135-remaining.md`. A concurrent session landed perf-
observability latches (#375–#381) alongside.

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

## 2026-06-16 delta — #300 P1 LANDED + DEPLOYED (doctor artifact problems → 0, D205)

The open **P1 of #300** (flagged in the delta below) is done and live.

- **D205 landed + deployed.** `striatum doctor`'s artifact-integrity check now
  takes its 42 residual historical-loss problems to **`problem_count: 0`** via
  three additive rules (read-only `go/pkg/reads/`): **(A)** default-branch
  *history* awareness → clean; **(B)** `artifact_superseded_on_default_branch`
  warning (path live on default tip, content revised pre-merge); **(C)**
  `artifact_acknowledged_loss` warning from a curated, **sha-bound** baseline
  (`docs/operator/doctor-acknowledged-loss.json`, schema
  `striatum.doctor.acknowledged_loss.v1`). An unlisted genuine loss still reds
  `ok` (load-bearing safety, tested both ways). Daemon redeployed — running pid's
  `/proc/<pid>/exe` embeds git sha `f0c29f67`, NOT `(deleted)`; live doctor shows
  the artifact block `problem_count: 0`, `acknowledged_loss_status: loaded`, with
  16 `artifact_acknowledged_loss` + 12 `artifact_superseded_on_default_branch`
  warnings.
- **The real split was 14 / 12 / 16, not the handoff's 27 / 15.** By *content
  sha* (not path presence): 14 recoverable via history (Rule A), 12 superseded
  (Rule B), 16 genuinely path-gone (Rule C baseline). The 16 are immaterial early
  dogfood drafts (`docs/issues/22-27` handoffs, `agy-loop-smoke`, `f42`/`f44`
  driver handoffs, `interrogating-panel`, `rfc-0088-p1` verify, `rfc-0098`
  handoff, `ace-graduation` drafts, `docs/dogfood/058` synthesis). This is the
  "acknowledge" half of the #303 acknowledge/prune tier; a record-prune verb
  remains #303's domain.
- **Built via the `doctor-integrity-legibility-p1` dogfood** (`run_d6134a8a`,
  `code_change` single claude lane, `publish_source_changes`), operator-gated
  (build, CI lint `0 issues`, full `pkg/reads` tests incl. both safety
  sub-cases). The apply lane hit **#289 again** (`agent_exited_unsealed` →
  `recovery_exhausted`); finalized through the daemon with `recovery
  complete-stalled` — and the **#292 lease-timing gap recurred**: the apply lease
  did not time-expire for ~31 min (a requeue renewed it), so `complete-stalled`
  refused until then. Reinforces that follow-up (a way to finalize a confirmed-dead
  unsealed lane without waiting out the renewed lease).
- **`doctor ok=True, problem_count=0` — fully green (first time).** The 4
  NON-#300 problems above were the **#290** (`run_a016c955`) and **#296**
  (`run_685ae8f4`) `divergent_ideation` runs wedged at their final job (same #289
  pattern). The runs' owner (this operator pack) gave them go-ahead, drove both
  to completion, and **finalized both via `recovery complete-stalled`** (→
  `completed`, `IDEATION_SYNTHESIS.md` `readback_verified`); that cleared the 4
  residual problems, so global doctor is now green. Both runs produced full
  option-sets (pushed branches `origin/striatum/issue-290-parallel-fanin` /
  `issue-296-codex-mcp-injection`; headlines commented on #290/#296). Landing
  the artifacts to `main` is deferred behind #299 (base-drift → `run integrate`
  non-FF). The recovery defect itself is filed as **#308** (bug, ready-for-agent):
  the autonomous sweep should auto-finalize an `agent_exited_unsealed` job whose
  artifacts are reconstructable, instead of escalating to `needs_operator`. See
  [[project_292_complete_stalled]].
- **Open operator decisions (not blockers):** version bump / tag for D204+D205
  (per the release convention) — deferred to the operator. (The earlier
  "clean up the 4 residual #290/#296 problems" decision is RESOLVED — doctor is
  green.)

## 2026-06-16 delta — #296 + #290 IMPLEMENTED + DEPLOYED (the two design picks)

The #290/#296 divergent-design picks were implemented from their synthesis,
landed off `origin/main`, deployed (daemon restarted, running image == installed),
and the issues CLOSED. `doctor` stayed green throughout.

- **#296 CLOSED + LIVE** (`d9329618`). codex push (stdin-FIFO) lane now FAILS
  LOUD when the MCP endpoint/token can't resolve (was a silent degrade to bare
  `codex` that no-ops the control plane); precedence locked in by a codex-CLI-gated
  test proving the `-c mcp_servers.striatum.url` override beats a stale config.toml
  section. Bug fix, no D-number; boot-epoch/port-reuse long-tail → follow-up **#316**.
- **#290 CLOSED + LIVE** (`bd79ab51`, **D206**). Fan-in siblings that can't
  fast-forward the run branch are now INTEGRATED via a conflict-free object-DB
  content merge (`merge-tree`→`commit-tree`→CAS `update-ref`, like `run integrate`)
  instead of stranded under a pin; overlap errors loudly. New `doctor`
  `fanin_sibling_unintegrated` warning (running runs only). Deferred join barrier +
  manifest → follow-up **#319**. Direct impl (operator-chosen) — smallest correct slice.
- **Also landed (kept main green):** brief trimmed under its 300-line budget
  (`ea3b237f`, the brief-guard had held CI red 4+ commits); embedded
  refactoring-campaign `REFERENCE.md` re-synced (`f636be15`, drifted in 61ab3ea1 →
  red `TestEmbeddedOptionalSkillMatchesCanonicalSource`).
- **Concurrent-agent ownership (do not duplicate):** another agent is implementing
  **#308** (sweep auto-finalize of a published-but-unsealed final job) + its coupled
  prerequisite **#309** (finalize liveness test → session-liveness not lease-time).
- **Historical live open set at that point:** #298/#299/#300/#301/#302/#303/
  #304/#305/#306/#307/#308/#309/#310/#311 + follow-ups #316/#319. This list is
  superseded by the current tracker snapshot below.

## State

Latest release is **v2.34.0 (2026-06-18)** — see the top delta (six reliability/
security fixes + the RFC 0135 sealed-barrier primitive, opt-in/shadow). The prior
**v2.33.0 (2026-06-16)** tag at `564a8209` packaged the post-v2.32.0 landing set:
doctor integrity legibility **P0+P1** (D204/D205, #300), **#290/D206** fan-in
run-branch integration, **#296** codex push-lane loud-fallback, **#301/#307**
workflowgenerate fixes, **#304** dangling-blocker resolution, and **#311**
recovery-escalation legibility (details in CHANGELOG `v2.33.0`). The prior
**v2.32.0** (2026-06-13) packaged RFC 0118/0119/0120 plus session-recovery edge
fixes; the v2.10.0 → v2.31.0 release burst is indexed in CHANGELOG and the
decision log. Historical open sets such as #212/#263-#267 are no longer current.

## Deep architecture review 2026-06-11 — work-list (mostly closed in v2.34.0)

`STRIATUM_DEEP_ARCHITECTURE_REVIEW_CLAUDE_FABLE_5_2026-06-11.md` (`0e8671ed`,
Claude Fable 5): verdict **ROUGHLY RIGHT-SIZED · ON TRACK**. v2.34.0 closed the
ranked BLOCKER (sweep-error daemon suicide + in-tx git — fixed earlier), the P0
untested spine (#356), the P1 deletion pass (#357, scoped to the ~1K provably-dead
LOC — the review's "4-5K" did not survive verification), conformance honesty (#358),
truth mechanization (#359/#363, incl. this brief's freshness guard + `supervise.rebridge`
on-contract), and the P2 boundary-hygiene batch (#358). **STILL OPEN:** P1
token-out-of-argv (`STRIATUM_MCP_TOKEN` passes through tmux/sudo argv, world-readable
via `/proc/*/cmdline`, `supervisor/pty.go`) and the `docs/operator/` exhaust
relocation (#364, ready-for-human).

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

1. **Keep the reliability recovery gate green:** preserve closed #302/#308/#309
   as regression evidence, then prove the final-review failure shape from
   `run_8489e7d2df3b56e1ed7fdb49ff5c8ba7` no longer needs operator
   requeue/escalation handling.
2. **Keep current-state docs truthful:** after every issue-closeout or release,
   refresh this brief, README status, docs index summaries, and any roadmap/todo
   surface that claims to list current open work.
3. **Triage the 2026-06-16 issue wave:** #322-#327 are newer than the v2.33.0
   brief and should be classified before release planning resumes. #329 is fixed
   but should stay in the regression set for the read-side helper-event drain
   authority path.
4. **Bound doctor warnings:** keep `problem_count=0`, but turn the 219-warning
   channel into named classes with allowed baselines/deltas.

## Blockers / Open Issues (18)

Open GitHub tracker state as of 2026-06-17T00:29Z. #302/#308/#309/#329 were
checked separately and are closed or fixed in this slice; keep them as
regression references, not open work.

- **Ready-for-human / operator decisions:** #298 dirty lane worktree recovery,
  #299 run-branch base drift, #303 terminal-run debris prune, #305 terminal-run
  provenance legibility, #310 lane-owned artifact ACL gap, #311 agy liveness
  wedge.
- **Divergent/fan-in follow-ups:** #306 blob-routed divergent inputs, #316
  codex/MCP boot-epoch defense, #317 same-attempt byline mismatch wedge, #319
  deferred fan-in join barrier, #322 `parallelism.max_active_jobs` ignored,
  #327 sibling-publication fan-in false rejection.
- **Fresh 2026-06-16 triage wave:** #312 `repo add --init` flag mismatch, #313
  operator-by-hand path non-functional, #323 daemon restart orphans claude lane,
  #324 stale endpoint lane spins forever, #325 daemon DB deadlock under parallel
  completion, #326 artifact publication drops undeclared in-scope files.

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
- `make install` does NOT restart the daemon — `systemctl --user restart
  striatumd`, then verify the running `/proc/<pid>/exe` sha.
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
- `CHANGELOG.md` (v2.10.0 → v2.32.0 + Unreleased)
- `docs/reference/command-authority-matrix.md` (lags 16 live methods —
  reconcile on contact, per AGENTS rule)
