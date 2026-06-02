---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-01d_rfc0101-complete-v2.9.0"
supersedes: "brief_2026-06-01c_rfc0101-phase1-2-landed"
scope_links: ["docs/rfcs/0101-robust-autonomous-workflow-execution.md", "docs/rfcs/0102-operator-attention-economy.md", "docs/rfcs/0095-revision-safe-workflow-lifecycle.md", "docs/rfcs/0098-adjudicated-constraint-extraction-loop.md", "docs/rfcs/0097-full-workflow-run-orchestration.md", "dogfoods/rfc-0101-l2-conformance/OPERATOR_REPORT.md", "dogfoods/rfc-0101-l2-conformance/artifacts/DESIGN_SYNTHESIS.md", "CHANGELOG.md", "docs/rfcs/README.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: operator-claude-001

## State

Striatum's live-state boundary is daemon-owned PostgreSQL; Go is the only
runtime (RFC 0078 closed — no Python runtime/packaging/tests). Repository files
are durable provenance; `.striatum/` is operational scratch only. Latest
release is **v2.9.1 (2026-06-01)** — the full RFC 0101 robust-autonomous-execution
arc (Phases 1–5) **plus** RFC 0096 V2's first security slice (#135 per-session
capability-token binding), deployed at **schema 22**.

Since the prior brief (which stopped at RFC 0078) the work has been the
**live-collaboration → autonomous-execution arc**:

- Interrogating panels + chat UI (v2.4.x), N-party conversation (v2.5.0),
  conversation turn-drivers (v2.6/2.7), and the RFC 0088/0089 agent-loop PTY
  launcher (claude/codex/agy lanes complete agent-loop packets end-to-end).
- **RFC 0093** structured live-collaboration shapes — accepted, V1 landed
  (collaboration_ledger.v1 + adjudicator substance-gate).
- **RFC 0095** revision-safe lifecycle — Phases 1-3 landed in source (#84
  attempt-scoped artifacts deployed at **schema 18**; #65 panel-owned
  interrogation window; closed-session/write-scope/idempotent-publish guards).
- **RFC 0096** supervised-lane trust boundary — Phase 1 landed (allowlist lane
  env, work-tree hygiene); the PG-less lane OS user (V2) is not built.
- **RFC 0098** adjudicated constraint-extraction loop — V1 landed (slices 1-3,
  productive-refusal gate + discharge-verifying final review); #89 closed.
- **RFC 0100** self-describing artifact contracts — P1 landed (front-matter
  schema surfaced + enriched validation errors).

**Doc-vs-source drift to fix on contact:** `docs/rfcs/README.md` still marks
0095 / 0098 / 0100 as `proposed` though their V1/phases have landed and
deployed. Reconcile the RFC index status column when you next touch it (AGENTS
rule: fix the doc when it disagrees with source).

## Landed 2026-06-01 — RFC 0101 COMPLETE (v2.9.0, deployed at schema 21)

The full **RFC 0101 robust-autonomous-execution arc (Phases 1–5)** is landed,
deployed, and released as **v2.9.0** (tag pushed). Vehicle: bootstrap-via-subagent
(worktree + background) with every diff operator-reviewed before integration.

- **Phase 1 — honest liveness** (schema 19): PTY-activity + tool-call boundary
  recording, precise classifier states (`working_protocol/working_local/
  working_tool/quiet/stalled/dead`), lease auto-heartbeat. Closes #80/#83/#117/#136.
- **Phase 2 — adapter conformance** fake-agent fixture (`go/pkg/adapterconformance/`):
  C0 #101 argv-bootstrap gate + C2 env-leak golden + C3–C10/C7 live taxonomy.
- **Phase 0a — 40P01 interrogation deadlock ROOT-FIXED** (`e1e95ac3`): the
  `sessions⇄runs` cycle between the claim path and `interrogation.answer`. Per-run
  advisory lock (taken first, only for live interrogation targets) + `withTxRetryOnDeadlock`
  on all interrogation handlers; 50-iteration regression test; the conformance
  `InterrogationReady` workaround removed. Clears the #130/#131/#133/#134/#137 cluster root.
- **Phase 3 — autonomous in-daemon recovery supervisor** (schema 20): the existing
  60s `recovery.sweep` now ACTS on classifier states with PG-persisted per-job
  budgets — `requeueJobSameAttempt` (dead/closed/absent lane → requeue, no attempt
  bump), stalled-but-active-session recovery (+ close the parked session),
  transfer, leaked-window close. Closes **#121** (the acute blocker) + the
  operator verb `recovery requeue-stale --force` for dead-lane repo-write + `session
  close --requeue-job`. Files: `recovery.go` (`requeueJobSameAttempt`),
  `recovery_decision_tree.go` (`recoverStuckJobs`), migration `0020 job_recovery_state`.
- **Phase 4 — loud structured escalation** (schema 21): a `needs_operator` run
  state. On budget exhaustion the sweep raises a `recovery_exhausted` blocker +
  `escalation_inbox` row (structured `striatum.recovery_escalation.v1` payload) and
  flips the run `running → needs_operator`; `escalation resolve` clears it; doctor
  surfaces needs_operator runs as **problems**. Files: `recovery_escalation.go`,
  migration `0021 run_needs_operator`.
- **Phase 5 — fault-injection chaos suite** (`adapterconformance/chaos_test.go`,
  PG-gated): drives the real fake-agent lifecycle through the in-process daemon,
  injects dead/stalled-lane faults (deterministic time-warp), runs the sweep, and
  asserts self-recover (fresh lane completes, attempt preserved) OR escalate-loudly
  within budget — RFC 0101 acceptance #2/#3/#4. **It caught a real bug in deployed
  Phase 3 recovery** (a duplicate-message 23505 sweep-wedge: `HandleClaimNext`
  doesn't stamp `jobs.current_message_id`, so requeue must resolve the live work
  message directly — now fixed in `recovery.go`).

## Current Frontier

RFC 0101 is **complete + live (v2.9.0, schema 21)**: recovery + escalation run
autonomously in the daemon; the chaos suite is the hermetic regression gate. The
frontier is now the **capstone + backlog**:

- **RFC 0097 self-hosting milestone (acceptance #5): PROVEN LIVE 2026-06-01**
  (`8e9ac86b`). `dogfoods/rfc-0097-self-hosting/` — a minimal single-claude-lane
  document dogfood — was driven end-to-end through the production handlers
  hands-off (prepare → confirm → start → register → supervise → lane self-drives
  claim → publish → complete → **run auto-finalized to `completed`**); artifact
  `content_sha256` matched the on-disk file exactly. #101 confirmed live (lane
  bootstrapped past the update screen). The chaos suite already proved this at the
  DAEMON level; this closes the full live CLI proof. Finding: a self-driving
  claude lane needs `--dangerously-skip-permissions` (bare `["claude"]` parks on
  an MCP permission prompt) — a stale-scaffold mistake, not a runner bug (#113
  CLOSED already fixed the real examples). See
  `dogfoods/rfc-0097-self-hosting/OPERATOR_REPORT.md`.
- **Backlog** (see `~/.claude/plans/golden-hugging-teacup.md`): **RFC 0096 V2**
  security (#70/#87/#135 token-binding, PG-less lane sandbox), **RFC 0100 P2** DX
  (#126/#128/#132), **RFC 0097** orchestration (#115/#138), **RFC 0099/0102**
  operator side (#92). Confirm-and-close the deployed Phase-1 issues (#80/#83/#117/#136)
  and the fixed-but-open #120/#123 against live behavior.

### Original framing (RFC 0101 L2 dogfood, 2026-05-31)
RFC 0101 frames the recurring dogfood failure taxonomy as one run-level
property and supplies five defense layers: (1) honest liveness, (2) adapter
conformance + persistent turn-driver in CI, (3) bounded autonomous
self-recovery, (4) loud structured escalation, (5) a fault-injection chaos
suite. **RFC 0097** (run self-orchestration) sits on top and hard-depends on
RFC 0095 + 0096.

### RFC 0101 L2 dogfood reality (2026-05-31)

`dogfoods/rfc-0101-l2-conformance/` reached an **accepted design synthesis** —
the 3-model design panel cleared clean after the #120 reviewer-interrogate-grant
fix landed (commit `259482d0`). Then the **implement repo-write lane wedged** on
a Claude welcome-screen stall (#101 class) at spawn, and there is **no
same-attempt recovery verb** to rescue it (#121). #120's fix is proven; **#121
is the acute open blocker** and the named RFC 0101 L3 gap. No harness Go was
built — the self-hosting paradox held again (a broken runner can't reliably
dogfood its own fixes).

## Issue burn-down 2026-06-01 (32 → 17 open)

A `max-closed-count-first` burn-down pass (plan `~/.claude/plans/typed-shimmying-diffie.md`):

- **Phase 1 confirm-and-close (9):** #80/#83/#117/#120/#121/#123/#130/#136/#137 —
  verified fix-commit ancestry + repro/test before closing. **Reopened 3** that were
  over-attributed to the RFC 0101 Phase 0a deadlock commit: #131/#134 are the #65
  interrogation-window-closure family (not the 40P01 deadlock); #133 was a real
  register-session deadlock the Phase 0a fix did not cover.
- **Phase 2 fixes (deployed @ `988b9653`, schema 22):** #142 (`list workflows`
  42703 — column alias), #133 (register-session retries on recovery deadlock),
  #127/#132/#140 (verdict semantics — idempotent complete, synonym vocab, recoverable
  reject; **D158**), #118 (closed obsolete — single_shot/turn-driver retired by D150).
  All with regression tests; full `pkg/mutations` suite green.
- **Deploy lesson:** `make install` does NOT restart the running daemon — needs
  `systemctl --user restart striatumd`; verify the RUNNING `/proc/<pid>/exe` sha,
  not just the file. (Cost a live false-negative on #142.)

**Remaining 17, heavier/architectural (route via dogfood or bootstrap-subagent w/ review):**
#141 (agent-loop receiver reconnect across daemon socket recreation + supervisor
re-bind), #125 (codex readiness-vs-`work.ack` guard); **RFC 0096 V2** security
#135/#70/#87; **RFC 0100 P2** #126/#128; **RFC 0097** #115/#138; **RFC 0099/0102**
#92/#112; the **agy cluster** #139/#76/#85/#95; and the reopened #131/#134
window-closure family (RFC 0095).
These 17 are now **consolidated into [RFC 0103 — self-hosting production hardening](../rfcs/0103-self-hosting-production-hardening.md)**
(seven workstreams W1–W7, each extending a slice-RFC with what-landed /
what-remains / regression-gated acceptance; all 17 labeled `rfc-0103`), which
replaces the ephemeral burn-down plan as the dogfoodable spine for the rest.

## Next Actions

1. **RFC 0097 self-hosting capstone — DONE (proven live 2026-06-01, `8e9ac86b`).**
   The minimal single-claude-lane document dogfood completed end-to-end through
   the production handlers, hands-off, run auto-finalized to `completed`, artifact
   hash matched. #101 confirmed live. The next escalation is a *multi-lane* /
   product-fix dogfood driven through the runner (now the preferred vehicle over
   bootstrap-via-subagent) — gated on agy multi-turn (#95) for any agy seat; use
   claude/codex seats until #95 lands. Lane shape lesson: claude seats need
   `--dangerously-skip-permissions`.
2. **Backlog (dependency-ordered; see `~/.claude/plans/golden-hugging-teacup.md`):**
   **RFC 0096 V2** security — #135 per-session token-binding **DONE/deployed** (v2.9.1,
   schema 22); remaining: lane-env wiring so live lanes USE their session-bound
   token (fully closes #135 live), #70/#87 lane-PG-deny + PG-less lane OS user →
   **RFC 0100 P2** DX (#126/#128/#132, single-implementer)
   → **RFC 0097** orchestration (#115 frozen-snapshot signal, #138 shared-resource
   coordination) → **RFC 0099/0102** operator side (#92 constrained operator
   consumes the Phase-4 escalation). Confirm-and-close the deployed Phase-1 issues
   (#80/#83/#117/#136) and the fixed-but-open #120/#123 against live behavior.
3. **RFC 0102 levers:** narrow the operator loop to one control surface, one
   high-signal `attention` view, `(run, workflow_job_id)` identifiers.

## Blockers

- **#95** — agy self-driving session closes after the first turn and
  re-registers a duplicate unattested session, breaking multi-turn (interrogating)
  review/synthesis. (Relevant to a multi-lane self-hosting dogfood with agy.)
- **Self-hosting paradox — substrate now landed.** RFC 0101 (recovery +
  escalation, schema 21) is the robustness substrate the paradox waited on. The
  chaos suite proves the recovery loop end-to-end at the daemon level; the
  remaining open question is whether a full live multi-lane CLI dogfood clears the
  lane-boundary friction (#101 welcome-screen lane-env hardening, #95 agy).
  Vehicle for foundational fixes stays bootstrap-via-subagent until a live dogfood
  is proven.

**Resolved this release:** #121 (same-attempt repo-write recovery — Phase 3),
the #130/#131/#133/#134/#137 deadlock-cluster root (Phase 0a).

## Hazards / Do Not

- **Operators scaffold dogfoods; they do not implement role artifacts.** Facing
  implementation work → scaffold a fix-up dogfood (or a one-shot single-implementer
  build) and launch it; do not author the role's artifact yourself.
- **Revision-cycling interrogating panels wedge** (the #65/#84/#120/#121
  family). When you need a fix to actually land, a one-shot single-implementer
  build sidesteps the panel revision incoherence.
- Stay on the daemon boundary: do not bypass the daemon, open Postgres directly,
  treat tmux panes / terminal output / marker files as workflow state, or add
  hosted services, telemetry, transcript capture, or external persistence
  without a product decision.
- Trust only returned JSON. Never fabricate session/run/lease ids or results;
  verify every state-changer with a read; make state-changing calls sequentially.

## Pointers

- `docs/rfcs/0101-robust-autonomous-workflow-execution.md` (umbrella)
- `docs/rfcs/0102-operator-attention-economy.md`
- `docs/rfcs/0095-revision-safe-workflow-lifecycle.md`
- `docs/rfcs/0096-supervised-lane-trust-boundary.md`
- `docs/rfcs/0097-full-workflow-run-orchestration.md`
- `docs/rfcs/0098-adjudicated-constraint-extraction-loop.md`
- `docs/rfcs/0099-constrained-operator-mode.md`
- `dogfoods/rfc-0101-l2-conformance/` (workflow, artifacts, OPERATOR_REPORT)
- `CHANGELOG.md` (`Unreleased` + v2.x history)
- `docs/rfcs/README.md` (RFC index — status column lags source; reconcile)
