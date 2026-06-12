---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-12_v2.31.0-rfc0120-landed"
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
run-completion provenance gate, P0-1 through P1-6), the **RFC 0119
acceptance** (D179), **RFC 0120 Phase 1 + Phase 2** (#248, D180:
await-packet idle exit plus notify-only wake bus), the **session recovery edge
fixes** (#253/#254/#255), and a CLI-reference/doc-truth pass. The prior brief
stopped at v2.9.3 (2026-06-02); everything below is the delta.

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

**Issue burn-down:** 32 → 9 open. The ready-for-human cluster
(#220/#215/#214/#223/#222/#201/#243) all closed by 2026-06-10, and the
2026-06-12 landing wave closed #240, #248, #253, #254, #255, and #258.
Every survivor is a live operational finding from real multi-run load, not a
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
  Release the accumulated post-tag work as the next minor.
- **Live housekeeping:** `doctor` is OK (0 problems) but still warns that
  the local Codex config points at a stale MCP endpoint unless launched
  through `striatum codex`. The remaining worktree-ref-safety residue is now
  issue #259: missing-on-disk terminal worktree rows need a supported retire
  path.

## Next Actions

1. **Stabilize run drive launch:** triage #260 first. Provider-auth preflight
   now blocks otherwise-valid Codex launches with
   `lane_provider_preflight_unexpected_result`; until that is resolved, AFK
   self-hosting runs can wedge before claim.
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
4. **Housekeeping:** implement #259's missing-on-disk worktree-row retire
   path, then tag the next release for the post-v2.31.0 landing set.

## Blockers / Open Issues (9)

All operational findings from live multi-run load: **#260** provider-auth
preflight false-positive blocks valid Codex launches, **#259** missing-on-disk
worktree rows cannot be retired, **#257** mid-run write-scope drift blocker
policy, **#247** committee operator-neutrality (design), **#246** abandoned-run
GC, **#244** owner-table migrations crash-loop a two-role prod daemon
(CI-blind), **#242** run-drive commits via the operator's shared git index,
**#217** blob-store-gated, **#212** parked auto-spawn (do not implement).

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
- `CHANGELOG.md` (v2.10.0 → v2.31.0 + Unreleased)
- `docs/reference/command-authority-matrix.md` (lags 16 live methods —
  reconcile on contact, per AGENTS rule)
