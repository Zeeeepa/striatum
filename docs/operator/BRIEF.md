---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-22_v2.35.0-release"
supersedes: "brief_2026-06-18_v2.34.1-release"
scope_links: ["docs/operator/plans/provenance-durability-campaign-2026-06-14.md", "docs/operator/plans/rfc-0126-0128-implementation-campaign-2026-06-14.md", "docs/rfcs/0126-multi-reviewer-revision-coherence.md", "docs/decisions/decision-log.md", "CHANGELOG.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: operator-claude-opus-4-8-001

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

## 2026-06-21 delta — RFC 0142 P0 built + verified via a 3-stage self-host dogfood (PR #553; runner defect #551)

Drove **RFC 0142** (safe-by-construction DB-change deployment, proposed PR #538)
through Striatum's own runner in three chained stages:

- **Stage 1 — design committee** (`falsification_gate`, claude holder/adjudicator +
  **agy/gemini** falsifiers; codex auth dead this session). Genuine cross-model
  falsification hardened the P0 spec into 5 verification-gated binding constraints
  (C1 escape-proof LOGIN role / C2 bootstrap ownership fidelity for 0038-0042 /
  C3 non-superuser bootstrap / C4 isolation self-check / C5 search_path), verdict
  `accept_with_findings`. Output preserved at
  `docs/operator/workflows/rfc0142-design-falsification/committee-output/`.
- **Stage 2 — `code_change` run** (draft→review→apply, `accept_with_findings`) built
  P0: `pgtest.TwoRole` + the `42501` red oracle + green control, test-harness only.
- **Stage 3 — verifier**: `striatum verifier run` minted passing sealed **ASSERTED**
  receipts (go-build/go-vet/go-test) under strict bwrap; the PG-backed two-role suite
  **passes live, 8/8, under a non-superuser owner DSN — the #442/D248 `42501`
  reproduces.** → **PR #553** (additive safety net; gated on RFC 0142 acceptance).

**Runner defect surfaced + filed: #551** — a `falsification_gate`
`collaboration_ledger` published via `artifact.publish`+`review.verdict` was
registered (verdict captured, gate cleared) but **never git-anchored to the run
branch**; the downstream `commit_proposal` correctly escalated a `human_checkpoint`
rather than fabricate a blind proposal. Stage 1 was banked (RFC 0137 precedent) and
its output recovered to main. Also cleared **43 pre-existing doctor orphans** (old
terminal runs) via `worktree anchor` + 2 acknowledged-loss baseline entries — doctor
green throughout. Refreshed expired `striatum-lane` claude creds.

## 2026-06-21 delta — lane-perms ACL cluster: #537/#539 FIXED, #512 → RFC 0143 (PR pending)

Committee-run permission cluster surfaced by the prompt-committee dogfood:

- **#537 (lane can't write) + #539 (daemon/operator can't manage) — FIXED.**
  `striatum repo add --init` (live path `repo.add` → `repositories.Service.Add`;
  also `repo.init`) now provisions the committee POSIX ACLs when lanes run as a
  non-owner OS user (`STRIATUM_LANE_OS_USER`): `setfacl -R -m u:<lane>:rwx
  -m d:u:<lane>:rwx -m d:u:<owner>:rwx` on the repo tree and `.striatum/worktrees`.
  Same convention older repos carry; new helper `go/pkg/admin/repo_acl.go`
  (mirrors `socket_acl.go`/`scratch_acl.go`). Best-effort/idempotent, no-op for
  owner-run lanes / missing lane user / no setfacl; outcome surfaced as
  `committee_acl_provisioned` / `committee_acl_error`. NB the system has **no**
  shared lane/owner group — the convention is POSIX ACLs, and the inheritable
  owner default ACL is what makes lane-created committee dirs daemon/operator-
  manageable without sudo (#539). NOT deployed (PR pending); no daemon restart
  needed (filesystem provisioning at adopt time).
- **#512 (lane can't reseal across a daemon boot-epoch rotation) — routed RFC,
  NOT implemented.** Security/authz: the lane's credential-resolution fallback
  reaches the daemon's owner-only `0600` runtime `client-token`, which is the
  full-authority **bootstrap admin** token; group-reading it (the issue's literal
  suggestion) widens admin-token exposure to every lane and dissolves the
  session-bound token trust model (#135/#296). The alternative (a durable
  lane-readable session-scoped reseal token) is a new credential-distribution
  mechanism. Both are decisions, not triage edits → **RFC 0143** stub on the PR
  branch (`docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`,
  status proposed). No token code touched. Interim recovery stays the operator
  requeue (`supervise stop` → `session close` → `recovery auto`).

## 2026-06-20 delta — #515 builtin verifier verifies a nested Go module (merged + CLI deployed)

The RFC 0134/0141 **builtin verifier** could not mint a passing receipt for
striatum's OWN repo. The three bugs #515 named (multi-main `go build -o`,
newer-toolchain offline, worktree `git fsck`) were **already fixed by #494**;
the live cause was different — the verifier ran `go build|vet|test ./...` from
`--cwd` (repo root) but striatum's Go module lives in `go/`, so every go builtin
failed `directory prefix . does not contain main module`. Fix **PR #528 (merged
`9e1b6475`, closes #515)**: `goModuleDir` runs go-* checks in the module dir while
still binding the WHOLE worktree read-only (so module tests reading `../../VERSION`
resolve and git discovery works); conservative — absent/ambiguous trees fall back to
cwd and fail legibly, never guess. The receipt seals `working_subdir`. A red
`verifier run` now also surfaces a bounded `stderr_tail` (the meta-fix for the
blind hand-repro that misdiagnosed #515). **Result:** all four builtins now mint
passing **ASSERTED** receipts against a clean `main`, run from the repo root
(go-* show `workdir=go`). Bugfix to D239's impl — no migration, no D-number.

**Red `main` fixed en route — PR #529 (merged `d26a9619`).** `main` was red for
EVERY PR since #522 changed `daemon install` to refuse (exit 1) when a system unit
exists but left #514's `TestDaemonInstallRespectsExistingSystemUnit` asserting the
old exit-0 contract; a one-test sync unblocked the whole repo.

**Deployed CLI-only.** The fix is lane-side; `striatumd` only READS receipts
(ignores `working_subdir`, never recomputes the seal), so the installed
`~/.local/bin/striatum` (v2.34.1, from `9e1b6475`) was updated WITHOUT touching
`striatumd` — no daemon restart, the live RFC 0137 dogfood untouched, and the
post-#522 daemon-install refusal sidestepped. Prior CLI backed up at
`~/.local/bin/striatum.bak-pre515`. All work done in isolated `/tmp` worktrees off
`origin/main`; `doctor ok`, `main` green.

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

## 2026-06-16 delta — #300 P1 LANDED + DEPLOYED (doctor artifact problems → 0, D205)

Historical (superseded — doctor is green now). `striatum doctor`'s artifact check
took its 42 residual historical-loss problems to `problem_count: 0` via three
additive read-only rules (D205): default-branch history awareness, an
`artifact_superseded_on_default_branch` warning, and a sha-bound
`artifact_acknowledged_loss` baseline (`docs/operator/doctor-acknowledged-loss.json`);
an unlisted genuine loss still reds `ok`. Detail in CHANGELOG `v2.33.0`, D204/D205,
and [[project_doctor_integrity_legibility]].

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

Latest release is **v2.35.0 (2026-06-22)** — a large feature-wave cut (207 commits
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
- `CHANGELOG.md` (v2.10.0 → v2.32.0 + Unreleased)
- `docs/reference/command-authority-matrix.md` (lags 16 live methods —
  reconcile on contact, per AGENTS rule)
