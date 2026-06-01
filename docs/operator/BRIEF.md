---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-06-01b_rfc0101-phase1-landed"
supersedes: "brief_2026-06-01_robust-autonomous-execution"
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
- **Closed:** #114 (validate dup keys), #119 (validate agent_loop cap +
  artifact kind), #122 (help lists authoring verbs), #129 (`.claude` lock
  teardown), #113 (examples off retired one-shot commands → agent-loop shape).
  **Landed:** #123 (auto branch-confirm verifies the branch).
- **Deployed, behavioral confirmation pending a live lane:** #80, #136, #83,
  #117. **#135** kept open — the prototyped liveness gate did **not** close the
  same-token impersonation vector; the real fix is per-session token binding
  (thread rpc `AuthContext` into handlers), tracked under RFC 0096.

## Current Frontier

RFC 0101 **Phase 1 is done and live**; the frontier is now **Phase 2 — adapter
conformance + persistent turn-driver** (the keystone that breaks the
self-hosting paradox), then **#121** (same-attempt repo-write recovery, the
acute blocker), **RFC 0096 V2** (PG-less lane sandbox; #70/#87/#135), and
Phases 3-5. Phase 2's conformance harness has open design questions (real
installed-CLI vs fake-agent in CI) and should be designed deliberately, not
fire-and-forgotten. The umbrella **RFC 0101** and operator-side **RFC 0102**
remain `proposed`. **RFC 0097** (run self-orchestration) sits on top and
hard-depends on RFC 0095 + 0096.

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

1. **RFC 0101 Phase 2 (keystone) — design it deliberately.** Adapter conformance
   fixture against the *installed* CLI + persistent turn-driver (#95/#85/#76/#70/
   #113-done/#118/#125/#139), promoting the #101/#121 bootstrap to a contract
   clause. Open question to resolve first: real-CLI vs fake-agent conformance in
   CI. Then **#121** (same-attempt repo-write recovery on RFC 0095 attempt
   primitives — the acute blocker), then bounded self-recovery (Phase 3) and loud
   escalation (Phase 4). Confirm the deployed Phase-1 states (#80/#136/#83/#117)
   against a real lane and close them.
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
