---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-01c_rfc0101-phase1-2-landed"
supersedes: "brief_2026-06-01b_rfc0101-phase1-landed"
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
release is **v2.8.0 (2026-05-29)**; `Unreleased` in `CHANGELOG.md` holds a
post-v2.8.0 runner/DX fix batch (#100-#111, #80/#85 partials) not yet bumped.

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

## Landed 2026-06-01 (autonomous run, deployed at schema 19)

A bootstrap-via-subagent burn-down (vehicle: subagents/single-implementer for
foundational fixes, then dogfood) landed and **deployed** on `main`:

- **RFC 0101 Phase 1 — honest liveness: COMPLETE.** Slice 1 — the supervisor
  helper meters PTY output volume (OQ1 threshold, env-tunable) and the daemon
  auto-heartbeats the active lease on meaningful output (#80/#136 mechanism).
  Slice 2 — precise classifier states `working_protocol/working_local/
  working_tool/quiet/dead`, **#117** dead-at-spawn (Protocol `dead`, keeps
  `StallClass=agent_mcp_discovery_stall`), **#83** in-tool tool-call recording
  with a visible deadline, and the `last_pty_activity_at` producer. Projection-
  only states (never persisted → no CHECK-constraint risk). **Migration 0019**
  owner-applied (schema 18→19) — daemon redeployed, doctor green.
- **RFC 0101 Phase 2 — fake-agent conformance fixture: LANDED** (`go/pkg/
  adapterconformance/`). C0 **AdapterContract golden = the #101 regression gate**
  (claude/codex/agy must use argv bootstrap, not `pty_submit`) + C2 env-leak
  golden (hermetic, ride `make check`); a fake-agent in-process-daemon runner
  proves **C3–C10 + C7** live and **arms the taxonomy** (each broken mode →
  its exact `FailureClass`), PG-gated (skip without `STRIATUM_PG_TEST_URL`, run
  in CI; confirmed green ~13s). Deferred = the live-CLI **Tier B** (C1 pre-flight,
  C11 PTY launch, C12 work-tree scan, skip-ledger, the `striatum-adapter-
  conformance` driver binary + Make Tier-B wiring) and #118 turn-driver.
- **Closed:** #113, #114, #116 (clean supervise-start error), #119, #122, #124
  (no spurious auto-finalize rec), #129. **Landed:** #123 (auto branch-confirm).
- **Deployed, behavioral confirmation pending a live lane:** #80, #136, #83,
  #117. **#135** kept open — the prototyped liveness gate did **not** close the
  same-token impersonation vector; the real fix is per-session token binding
  (thread rpc `AuthContext` into handlers), tracked under RFC 0096.

## Current Frontier

RFC 0101 **Phase 1 (honest liveness) is done+live** and **Phase 2's fake-agent
conformance fixture is landed** (operator chose fake-agent over live-CLI in CI;
C0 #101 gate + C3–C10/C7 live + armed taxonomy). The frontier is now: **#121**
(same-attempt repo-write recovery — the acute blocker; note a lease-transfer-
without-attempt-bump verb already landed for #82, so investigate the residual
gap before building); the **Phase 2 Tier-B remainder** (C1/C11/C12 + driver
binary + Make wiring); **Phase 3** (autonomous recovery supervisor, on RFC 0095
attempt primitives) → **Phase 4/5**; and **RFC 0096 V2** (PG-less lane sandbox;
#70/#87/#135 token-binding). The umbrella **RFC 0101** and operator-side **RFC
0102** remain `proposed`. **RFC 0097** (run self-orchestration) sits on top and
hard-depends on RFC 0095 + 0096. The conformance harness also reproduced a
`40P01`/#103-class interrogation deadlock (noted on #137).

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

## Next Actions

1. **#121 — same-attempt repo-write recovery** (the acute blocker). The
   fake-agent conformance fixture is landed; #121 is the next high-value unit.
   First investigate the residual gap vs the already-landed #82 lease-transfer-
   without-attempt-bump verb, then build/​wire on RFC 0095 attempt primitives.
   Then **Phase 3** (autonomous recovery supervisor) → **Phase 4/5**, the **Phase
   2 Tier-B remainder** (C1/C11/C12 + driver binary), and **RFC 0096 V2**
   (#70/#87/#135 token-binding). Confirm the deployed Phase-1 states
   (#80/#136/#83/#117) against a real supervised lane and close them.
2. **RFC 0102 levers:** narrow the operator loop to one control surface
   (CLI / daemon MCP — no tmux/psql/systemctl/hand-rolled drivers in the normal
   path), add one high-signal `attention` view, and operate in
   `(run, workflow_job_id)` identifiers, not the `sess_/sup_/lease_` zoo.
3. **Burn the backlog:** 35 open issues (#70-#138), clustered by owning RFC
   (lifecycle→0095, security→0096, liveness→0091, agy, artifact-contracts,
   CLI/DX). Ship each fix as a separate commit; bootstrap product fixes via
   subagents or single-implementer runs while orchestrated dogfoods are blocked.
4. **Reconcile the RFC index** status column (0095/0098/0100 → landed) and bump
   the version + promote `CHANGELOG.md` `Unreleased` when the next unit lands.

## Blockers

- **#121** — repo-write implement lanes can wedge on a welcome-screen stall with
  no same-attempt recovery verb; the run can't self-recover. RFC 0101 L3 / Phase
  3 seed. (Phase 1 honest-liveness is now deployed, so a stalled lane is at least
  classified honestly — but autonomous recovery is not yet built.)
- **#95** — agy self-driving session closes after the first turn and
  re-registers a duplicate unattested session, breaking multi-turn (interrogating)
  review/synthesis.
- **Self-hosting paradox** — orchestrated dogfoods *through* the runner stay
  blocked until RFC 0095/0096 robustness fully lands; product fixes are
  bootstrapped via subagents / minimal single-implementer runs in the meantime.

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
