---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-16_300-p1-doctor-ok"
supersedes: "brief_2026-06-16_deploy-and-triage"
scope_links: ["docs/operator/plans/provenance-durability-campaign-2026-06-14.md", "docs/operator/plans/rfc-0126-0128-implementation-campaign-2026-06-14.md", "docs/rfcs/0126-multi-reviewer-revision-coherence.md", "docs/decisions/decision-log.md", "CHANGELOG.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: operator-claude-opus-4-8-001

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

## 2026-06-16 delta — deploy confirmed + full backlog triage

Operator actions only this delta (no source change): deploy verification +
GitHub triage.

- **D201–D204 are DEPLOYED.** The running daemon (pid 635208,
  `/proc/<pid>/exe` → `~/.local/bin/striatumd`, both stamped 2026-06-15 21:42,
  NOT `(deleted)`) was started from the post-D204 binary, so the whole landing
  set is live: **#291/D201** (hung-supervised-session stall recovery),
  **#295+#293/D202** (`run start`/`run drive` auto-drive lifecycle),
  **#297/D203** (loud stranded-in-scope paths), **#294**, and **#300/D204**
  doctor integrity legibility. The #292 "Daemon redeploy required" note below
  is discharged; no further restart pending.
- **Doctor legibility live:** `striatum doctor` problems dropped ~278 → **42**;
  preserved-on-default-branch / terminal-run / legacy-pre-blob-storage findings
  are now `warnings` (173), not `ok=false` problems. The residual 42
  (`artifact_anchor_missing_file` + `artifact_blob_metadata_missing`, content
  absent from every durable ref AND the default-branch tip) is the known
  historical-loss baseline; reaching `ok=true` is the open **P1 of #300**
  (history-aware preservation + an acknowledge/prune tier — decide with #303).
- **Full backlog triage (2026-06-16):** every open issue now carries a
  disposition label. **ready-for-agent (bugs):** #301 (`workflow generate`
  sets `per_job` on only the first multi-lane lane → `validate` rejects its
  own output), #302 (recovery sweep doesn't reclaim a `tmux_pane_dead`-but-
  `active` session → wedged lane). **ready-for-human:** #298 (canceled-run
  dirty-worktree disposition), #299 (`run.integrate` base-drift semantics),
  #300 (doctor legibility P1), #303 (prune-verb retention/safety), plus the
  pre-existing #290 / #296.
- **#290 / #296 divergent-design runs: scaffolded, NOT started.** The
  `docs/campaigns/issue-290-parallel-fanin-design/` and
  `issue-296-codex-mcp-injection-design/` workflows (claude + `codex --yolo` +
  agy, per-job worktree isolation) `validate` clean but await an **explicit
  operator go-ahead** before `run prepare` / `run start`.
- **Stale below:** the "Blockers / Open Issues (6)" + "Next Actions" #263–#267
  sets in the older sections are CLOSED. The live open set is
  #290/#296/#298/#299/#300/#301/#302/#303.

## 2026-06-15 delta — #292 stalled-job finalize path (Unreleased, not yet tagged)

Work after the 2026-06-14 delta, on `main`:

- **#292 closed + landed** (`recovery.complete_stalled`, **D200**). New daemon
  verb / CLI `recovery complete-stalled <run-id> <job-id>` that non-destructively
  completes a job whose agent published its durable artifacts then died before
  `work.complete` (the `recovery_exhausted` + `needs_operator` dead-end the #289
  work surfaced but could not exit). Verifies required artifacts present +
  body-reconstructable (RFC 0125 P0-3, worktree-independent), then completes the
  job server-side and reuses the #207 path to resolve the moot blocker/escalation
  and restore the run to `running`. Refuses verdict-capable jobs (RFC 0118),
  refuses a live-leased job (finalizes a dead lane only); `--force` relaxes the
  blocker precondition, `--dry-run` previews. 5 pgtests, adversarially reviewed,
  CI-lint clean, built off `origin/main`. **No schema change.**
- **Decision numbering:** **D199** was taken by the `divergent_ideation`
  graduation (the 2026-06-14 delta's "next free D-number is D199" is now stale);
  this work took **D200**. Next free D-number is **D201**.
- **Daemon redeploy required:** `recovery.complete_stalled` is a new handler, so
  it does not take effect until `systemctl --user restart striatumd` (or a
  reboot). Verify the running `/proc/<pid>/exe` sha == installed after restart.

## 2026-06-14 delta — RFC 0125 follow-ups (Unreleased, not yet tagged)

Work after the v2.32.0 brief below, all on `main`. The #285/#286 daemon change
is **deployed** (daemon restarted 2026-06-14, running `/proc/<pid>/exe` sha
`dbbbe8c7` == installed, clean startup, read path verified); a **version bump is
not yet cut** (deliberately — package the next release when ready):

- **RFC 0125 provenance-durability campaign closed out** (#270–#284) — see
  `docs/operator/plans/provenance-durability-campaign-2026-06-14.md`.
- **Three follow-up RFCs accepted:** RFC 0126 (multi-reviewer revision
  coherence, **D194**), RFC 0127 (retire the lane git identity, **D195**),
  RFC 0128 (cross-repo run boundary, **D196**). Next free D-number is **D199**
  (D197/D198 taken by the #287/#289 friction fixes below).
  (Note: **D193** was taken by the hippo-S12 git-eviction taxonomy merge.)
- **Dogfood-friction fixes #287/#288/#289 landed + closed** (`741d5c8b` #288,
  `37276a73` #289, `fc651158` #287 — all direct fixes, code-reviewed, pgtested,
  CI-lint clean, off `origin/main`). These fix the `code_change` dogfood pipeline
  itself: **#288** (`workflow generate` DX — `.striatum/scratch` scaffold root,
  `--option workflow_id/artifact_root`, batched lane-command errors, single_agent
  code_change validates out of the box); **#289** (`agent_exited_unsealed`
  recovery class + smaller budget + inspect-the-worktree escalation, **D198**);
  **#287** (opt-in `write_scope.publish_source_changes` — the lane's source edits
  land on the run branch, **D197**; generator opts `code_change` in by default).
  The daemon (#287/#289) was restarted + verified to pick these up.
- **#285 landed** (`579c1c34`) — RFC 0125 P0-3 body-reconstructability completion
  gate (`verifyRequiredArtifactReconstructable`, fail key
  `required_artifact_unreconstructable`, degrade ladder warns-not-wedges).
- **#286 landed** (`d9ae5411`) — RFC 0125 P1-1 content-addressed RUN_LEDGER in
  the completion record. Both pgtested + adversarially reviewed; #285/#286 closed.
- **RFC implementation dogfoods scaffolded** (`b8d6be77`,
  `docs/campaigns/rfc-012{6,7,8}/` + `docs/operator/plans/rfc-0126-0128-implementation-campaign-2026-06-14.md`).
  **0126 P0 launched live** (`run_806f8cc15784dd1f85ae2900eb70de45`); operator
  review gate applies before integrating its feature branch.
- **#285/#286 deployed + verified** (daemon pid restarted, sha `dbbbe8c7`, no
  startup errors). **Pending:** promote CHANGELOG `Unreleased` → a version bump
  (the block also holds #217 + the concurrent #264/#266 set) when the maintainer
  says go; review + merge `rfc-0126-p0-review` (the dogfood-produced RFC 0126 P0
  implementation, verified); optionally launch the 0127/0128 dogfoods.

## State

Latest release is **v2.32.0 (2026-06-13)**, deployed from the signed
`v2.32.0` tag at `0f44933a` (release workflow `27469264131`, published
2026-06-13T14:24Z). It packages the post-v2.31.0 landing set: the **RFC 0118
implementation** (#240, run-completion provenance gate, P0-1 through P1-6),
the **RFC 0119 acceptance** (D179), **RFC 0120 Phase 1 + Phase 2** (#248,
D180: await-packet idle exit plus notify-only wake bus), the **session
recovery edge fixes** (#253/#254/#255), the
provider-auth/worktree-row/run-drive teardown/config preflight fixes
(#259/#260/#261/#262), and a CLI-reference/doc-truth pass. Runtime schema
remains 26 with owner bundles 0001–0006. `main` is now one commit past the
release tag with the #217 doctor artifact-anchor integrity check, which is
closed and belongs to the next release block. The prior full brief stopped at
v2.9.3 (2026-06-02); everything below is the delta.

**The 2026-06-03 → 06-07 release burst (v2.10.0 → v2.31.0, 22 minors):**

- **Autonomy cluster (v2.10–v2.14):** RFC 0103 workstreams closed; RFC 0104
  per-run serialization invariant; RFC 0105 standing reliability harness
  (D161). **Unattended DoD met:** `scripts/dod/driver.py` drove 10/10
  consecutive clean zero-rescue runs, verified in the daemon.
- **RFC 0111** in-band failure legibility (v2.16.0, D165): 72-code error
  catalog + `rpc.Error.Suggestion` remediation on every denial.
- **RFC 0110** daemon→PG auth + DB-enforced write boundary: v3 flip live on
  prod (v2.18.x, D164). A leaked runtime DSN can no longer forge artifacts
  or rewrite the hash chain.
- **RFC 0108** parallel independent runs, all 5 phases (v2.19–v2.23):
  isolation, collision, multi-run view, gated `run.integrate`.
- **RFC 0106** shape governance: new-shape FREEZE + one genuine reliability
  fixture per graduation. Graduated since: `implementation_panel` (D166),
  `falsification_gate` (D168), `cross_examination` (D169/D170 isomorphism
  proof), `adjudicated_constraint_extraction` (D172, v2.28.0, 4-cell
  interrogation fixture). Catalog: 7 supported / 6 experimental. Freeze holds.
- **RFC 0112** explicit interrogation consumers: accepted (D171) +
  implemented (v2.27.0). Lane run-as (cross-user lanes) landed alongside.
- **Seats:** agy graduated (D163) → demoted (D174) → re-promoted supported
  (D177). Supported seats = **codex + agy**. Claude has NO installed-CLI
  conformance fixture (see review finding below).
- **Triage-execution waves** v2.29.0 (17 issues) + v2.30.0 (13 issues from
  multi-run load): the **#198 daemon-load convoy** root-caused — the 60s
  recovery sweep held run advisory locks while shelling tmux/proc probes;
  fixed with a pre-transaction liveness oracle. Plus #197 transient-load
  classification, #193 bounded status payloads, #203 revision-cycle
  auto-publish integrity. RFC 0115 supervised token-usage telemetry landed.
- **RFC 0116** zero-operator-touch DAG (D175): **`striatum run drive`** —
  foreground idempotent reconcile loop (productized driver.py) — accepted
  and implemented. Daemon auto-spawn explicitly DEFERRED (#212, three-part
  evidence trigger).
- **RFC 0117** worktree/branch ref-safety (D176): completed job commit
  stacks always reachable from a durable ref (FF-or-pin under
  `refs/striatum/`); closed the **#186 silent data-loss** incident and #184;
  `worktree gc` companion (D178).
- **v2.31.0 — RFC 0114** identity read-scope (#164 CLOSED): owner bundle
  0006 transfers `principals`/`principal_clients`/`client_sessions`
  ownership + SECURITY DEFINER projections; doctor
  `pg_read_scope.posture=partial_projection_gated` (derived, not
  hard-coded). `private_read_denial` stays false — RFC 0113 R2/R3 open.

**Issue burn-down:** 32 → 6 open. The ready-for-human cluster
(#220/#215/#214/#223/#222/#201/#243) all closed by 2026-06-10, and the
2026-06-12/13 landing wave closed #240, #248, #253, #254, #255, #258,
#259, #260, #261, #262, and #217. The current open set is parked #212 and
five fresh 2026-06-13 `needs-triage` reports (#263-#267).

## Deep architecture review 2026-06-11 — the standing work-list

`STRIATUM_DEEP_ARCHITECTURE_REVIEW_CLAUDE_FABLE_5_2026-06-11.md`
(`0e8671ed`, Claude Fable 5, partitioned exhaustive read at `076c5eec`):
verdict **ROUGHLY RIGHT-SIZED · ON TRACK** (medium-high confidence),
reversing 06-02's OVERBUILT·DRIFTING. The core state machine is earned,
incident-pinned complexity; risk has migrated to **verification capacity**
and the **orientation/meta layer** (this brief's 22-release staleness was a
named SERIOUS finding — this revision is part of the remedy). Its ranked
asks:

- **BLOCKER:** sweep-error daemon suicide — any sweep iteration error
  cancels the whole daemon (`pkg/recovery/scheduler.go:53-55` +
  `cmd/striatumd/main.go:698-701`); plus in-tx git subprocesses in the sweep
  (`recovery.go:2051`, the #198 convoy class one layer down). Aggravated by
  #246 (abandoned runs accumulate, adding sweep load).
- **P0 untested spine:** `work.heartbeat` has zero behavioral tests;
  `worktree.create` composition untested; 4 packet-content blocks
  unasserted; `escalation_resolve.go:156` re-drives completion **without
  `lockRun`** (the one unguarded RFC 0104 door).
- **P1 deletion pass (~4-5K LOC):** inert crossrepo pkg; dead supervisor
  liveness twin; one-shot migration RPCs; auto-finalize circuit breaker;
  conversation gating + per-poll query; 10 deprecated aliases; installer
  template dedupe (~800 lines); 6 stale example dirs.
- **P1 token-out-of-argv:** lane env incl. `STRIATUM_MCP_TOKEN` passes
  through tmux/sudo argv — world-readable via `/proc/*/cmdline`
  (`supervisor/pty.go`).
- **P1 conformance honesty:** scheduled installed-CLI CI runs agy only;
  claude (the flagship adapter) has no fixture; the "tier cannot lie" guard
  checks registry↔registry, not registry↔CI.
- **P1 truth mechanization:** guard-test BRIEF freshness / README status
  (still says v2.9.x) / docs index / authority matrix (missing 16 live
  methods; `supervise.rebridge` bypassed the contract); retire
  roadmap.md/todo.md to archive.
- **P2:** relocate `docs/operator/` exhaust (44% of tracked files);
  boundary-hygiene batch (CLI suggestion surfacing, read deadlines,
  `seenRequests` bound, FIFO delivered-lie, `--apply-blob-creation` no-op,
  CSP-dead dashboard JS).

## Current Frontier

- **RFC 0120 (await-packet idle exit + wake boundary, D180) — LANDED.**
  Phase 1 terminal idle envelopes carry `idle_behavior=exit_session`;
  bootstrap no longer tells lanes to poll after `no_work`; the PTY receiver
  exits the lane cleanly. Phase 2 landed on main in `81b51959`: the
  notify-only wake bus adds read-shaped `wake.wait`, post-commit wake hints
  for work/message/turn availability, and `run drive` wake waits with bounded
  missed-notification fallback. Wake events stay hints over committed state,
  never authoritative. The earlier `issue-248-wake-bus-implementation` runs
  were canceled/superseded dogfood attempts; do not drive them as live work.
- **RFC 0119 (warm-tier memory boundary, D179) — accepted, implementation
  gated.** Authorizes the `hippo`/`fornix` warm-tier adjunct (separate
  repo, `~/git/hippo`) + a striatum-native read-only hot tier (`recall.*`
  over the daemon's own artifact stream, scaffold-time digest injection,
  default-off redacted `lane_trajectory` export, `progress_note`-only git
  eviction). D179 lists hard test obligations before any Go lands; no
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

1. **Triage the fresh 2026-06-13 reports:** #263 Codex generated-workflow
   lanes exit before claim, #264 missing supervisor env file, #265
   `supervise trajectory --tail` parsing, #266 token-in-argv exposure
   (security-relevant), and #267 one-shot Codex lane stalls before ack.
2. **Review P0s:** contain the sweep (error → log+backoff+skip, never
   daemon cancel; git out of the sweep tx; #246 abandoned-run GC) and test
   the spine (heartbeat, worktree.create, packet blocks, escalation-redrive
   `lockRun` + guard coverage).
3. **Review P1s:** deletion pass, token-out-of-argv, conformance honesty
   (claude installed-CLI fixture + codex in the cron), truth mechanization
   (brief-staleness CI guard landed 2026-06-12 —
   `TestOperatorBriefStaysCurrent` reuses the bootstrap probe; remaining:
   guard README status / docs index / authority matrix against the
   contract).
4. **Package the next release block deliberately:** #217 landed after the
   v2.32.0 tag, so the next release should include it plus any accepted fixes
   from #263-#267 with matching changelog coverage.

## Blockers / Open Issues (6)

Open tracker state as of 2026-06-13: **#263** generated Codex lanes validate
but exit before claim; **#264** supervised lane dies sourcing a missing
`/tmp/striatum-supervisor-env` file; **#265** `supervise trajectory --tail`
rejects a numeric line count; **#266** injected `STRIATUM_MCP_TOKEN` exposed
in lane process argv; **#267** one-shot Codex lane stalls before ack; and
**#212** parked auto-spawn (do not implement).

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
