---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-12_v2.31.0-rfc0118-0120-arch-review"
supersedes: "brief_2026-06-01d_rfc0101-complete-v2.9.0"
scope_links: ["STRIATUM_DEEP_ARCHITECTURE_REVIEW_CLAUDE_FABLE_5_2026-06-11.md", "docs/rfcs/0119-warm-tier-memory-boundary.md", "docs/rfcs/0120-await-packet-idle-exit-and-wake-boundary.md", "docs/decisions/decision-log.md", "CHANGELOG.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: operator-claude-fable-5-001

## State

Latest release is **v2.31.0 (2026-06-07)**, deployed (runtime schema 26 +
owner bundles 0001–0006). `main` is ~50 commits past the v2.31.0 tag and
carries unreleased work: the **RFC 0118 implementation** (#240,
run-completion provenance gate, P0-1 through P1-6), the **RFC 0119/0120
acceptances** (D179/D180), **RFC 0120 Phase 1** (await-packet idle exit),
and a CLI-reference/doc-truth pass. The prior brief stopped at v2.9.3
(2026-06-02); everything below is the delta.

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

**Issue burn-down:** 32 → 12 open. The ready-for-human cluster
(#220/#215/#214/#223/#222/#201/#243) all closed by 2026-06-10. Every
survivor is a live operational finding from real multi-run load, not a
feature wish.

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

- **RFC 0120 (await-packet idle exit + wake boundary, D180) — LIVE WORK.**
  Phase 1 implemented on main: terminal idle envelopes carry
  `idle_behavior=exit_session`; bootstrap no longer tells lanes to poll
  after `no_work`; the PTY receiver exits the lane cleanly. `run drive`
  stays the operator-authorized wake surface. **Phase 2 (notify-only wake
  bus) is in flight:** run `run_7a8b4f646d35bf076e673e40724d9fd1`
  (`issue-248-wake-bus-implementation`, draft → review → apply) is
  `running` with the draft job claimable. The workflow scaffold under
  `docs/operator/workflows/issue-248-wake-bus-implementation/` is untracked
  — commit it with the run's work. Wake events stay hints over committed
  state, never authoritative.
- **RFC 0119 (warm-tier memory boundary, D179) — accepted, implementation
  gated.** Authorizes the `hippo`/`fornix` warm-tier adjunct (separate
  repo, `~/git/hippo`) + a striatum-native read-only hot tier (`recall.*`
  over the daemon's own artifact stream, scaffold-time digest injection,
  default-off redacted `lane_trajectory` export, `progress_note`-only git
  eviction). D179 lists hard test obligations before any Go lands; no
  `memory.*` capability, no retrieval-dependent state transition.
- **RFC 0118 (#240)** implementation is on main (frozen verdict provenance
  stamps, override posture/basis, completion provenance gate +
  `needs_operator` escalation, durable `run_completion_record`,
  `recovery.invalidate_job` supersede receipts). Issue open pending live
  verification + close; release the accumulated post-tag work as the next
  minor.
- **Live housekeeping:** doctor reports 6 problems — an unanchored
  completed-job worktree HEAD on `run_6532226d` (`worktree_head_unreachable`
  + `job_completed_without_anchor`; re-run `work.complete` to anchor while
  the worktree exists) — and a stale prepared run
  `run_8e4e5487036601a540ea720f11d2f069` (`striatum/rfc-ledger-cleanup`)
  parked at `needs_branch_confirmation`: confirm or cancel.

## Next Actions

1. **Drive RFC 0120 Phase 2:** `striatum run drive --run-id
   run_7a8b4f646d35bf076e673e40724d9fd1` (draft is claimable now); commit
   the untracked workflow scaffold; verify Phase-2 test obligations from
   D180 (post-commit wake emission, `run drive` wake behavior,
   missed-notification fallback).
2. **Review P0s:** contain the sweep (error → log+backoff+skip, never
   daemon cancel; git out of the sweep tx; #246 abandoned-run GC) and test
   the spine (heartbeat, worktree.create, packet blocks, escalation-redrive
   `lockRun` + guard coverage).
3. **Review P1s:** deletion pass, token-out-of-argv, conformance honesty
   (claude installed-CLI fixture + codex in the cron), truth mechanization
   (make bootstrap brief-staleness fail; guard README/index/authority
   matrix against the contract).
4. **Housekeeping:** anchor the `run_6532226d` worktree; resolve
   `run_8e4e5487`; tag the next release once RFC 0118 verification closes
   #240.

## Blockers / Open Issues (12)

All operational findings from live multi-run load: **#251/#252** codex lane
health (exit-1 with no pty.log diagnostic + orphan supervisors; auth-rot
preflight), **#245** claim race (recovery stops a session before its first
`await_packet`; follow-on to **#241** false `tmux_session_missing`
liveness), **#246** abandoned-run GC (7 stuck runs biting today), **#242**
run-drive commits via the operator's shared git index, **#244** owner-table
migrations crash-loop a two-role prod daemon (CI-blind), **#240**
close-pending (impl landed), **#247** committee operator-neutrality
(design), **#248** Phase 2 in flight, **#217** blob-store-gated, **#212**
parked auto-spawn (do not implement).

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
- `docs/decisions/decision-log.md` (D161–D180 cover this brief's span)
- `CHANGELOG.md` (v2.10.0 → v2.31.0 + Unreleased)
- `docs/reference/command-authority-matrix.md` (lags 16 live methods —
  reconcile on contact, per AGENTS rule)
