---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: high
tags:
  - reliability-reset
  - final-review
---

# Final Adversarial Review
author: final-reviewer-codex-via-live-striatum-mcp-003

## Verdict

accept_with_findings

The reliability reset plan survives a hostile read, but only as a plan that is
still mid-reset. It is honest about the runner defects that damaged operator
trust: `doctor` was red from historical artifact noise until D204/D205, #289
and #292 still recur around reconstructable unsealed lanes, and the open recovery
cluster #308/#309/#302 is explicitly held for v2.34.0 rather than disguised as
done (docs/operator/BRIEF.md:15-65, docs/operator/BRIEF.md:97-104).

It is sufficiently ruthless to continue because it favors daemon-bound recovery,
doctor signal quality, fan-in reachability, loud MCP fallback failures, and
truth mechanization over feature growth. The product boundary it leans on is
also the right one: live state is daemon-owned PostgreSQL, `.striatum/` is
scratch, repository artifacts are provenance only, and terminal/tmux output is
not workflow state (docs/reference/spec.md:21-39). That is the right constraint
set for restoring trust.

This is not an unconditional accept. The reset cannot be declared finished while
the operator-facing "current" docs still contradict each other. If the next
slice treats the findings below as release gates, the plan is actionable enough.
If they remain advisory P1s, the plan will train operators to distrust the
current-state surface again.

## Findings

### F1 - High - The current operator brief still has two incompatible frontiers

The brief front matter says it is current (docs/operator/BRIEF.md:1-10), and
the top delta says the live open set is #298 through #311 plus #316/#319
(docs/operator/BRIEF.md:89-93). The same file later says the current open set is
#212 plus #263-#267 (docs/operator/BRIEF.md:151-155), then repeats those older
items as "Next Actions" and "Blockers / Open Issues" (docs/operator/BRIEF.md:225-252).
Those cannot all be the current frontier on June 16, 2026.

This matters because the reset's goal is operator trust. A green `doctor` plus a
contradictory current brief is not a trustworthy operating surface; it forces the
operator to know which paragraph is stale. The fix is not prose polish. Collapse
the brief into one authoritative "current now" block, move the older frontier to
an explicitly historical section or delete it, and make every next action map to
one issue cluster, one release gate, and one owner/status.

### F2 - High - The first-read status docs still advertise an obsolete release

The brief says the latest release is v2.33.0, published on 2026-06-16
(docs/operator/BRIEF.md:95-106). The README project-status table still says
v2.9.x with RFCs through 0103 in progress (README.md:300-313). The documentation
index still presents the README as the top-level pitch/status entry and
`operator/BRIEF.md` as the current-state surface (docs/index.md:25-42), so this
is a real reader path, not an obscure stale note.

The plan is honest enough to name this class: the standing work-list explicitly
calls out truth mechanization for README status, docs index, authority matrix,
and stale roadmap/todo material (docs/operator/BRIEF.md:187-190), and the next
actions repeat the remaining guards (docs/operator/BRIEF.md:235-240). But for a
trust reset this should be release-blocking, not P1 background work. An operator
landing on the README should not see a 24-minor-release-old status table while
the brief asks them to believe the current doctor and recovery story.

### F3 - Medium - The recovery gate is named, but not crisp enough yet

The plan correctly refuses to hide the recovery defect: it records #289 recurring
during the D205 apply lane, #292 making `complete-stalled` wait out a renewed
lease, and #308 as the bug that should auto-finalize reconstructable
`agent_exited_unsealed` work (docs/operator/BRIEF.md:40-62). It also says the
#308/#309/#302 recovery cluster is held for v2.34.0
(docs/operator/BRIEF.md:97-104).

The missing piece is a single crisp release gate. The spec says claim and review
completion paths verify live lane backend state and fail closed on missing or
lost supervisors (docs/reference/spec.md:839-850), and stale repo-write leases
require inspection before requeue (docs/reference/spec.md:895-899). The reset
should therefore name the exact acceptance fixture for #308/#309/#302: durable
valid artifacts plus a dead unsealed lane reach terminal completion without
waiting for renewed lease expiry, without operator hand-finishing, and with a
green doctor afterward.

### F4 - Low - The divergent ideation support story is mostly supported

The #290/#296 branch of the reset is much stronger than the state-doc story.
The brief says the design picks landed and deployed, with #290 fixing fan-in
sibling integration and #296 making Codex MCP fallback fail loudly
(docs/operator/BRIEF.md:68-93). The catalog documents `divergent_ideation` as a
supported shape with a green RFC 0105 unattended-reliability fixture
(docs/reference/workflow-catalog.md:105-112). The workflow-type guide says the
fixture proves double fan-out/join completion and branch-death self-recovery
(docs/reference/workflow-types.md:538-541). The implementation and tests back
that shape claim with ordinary fan-out edges, fresh sessions, convergence and
deepen jobs, multi-model round-robin checks, and production-handler recovery
fixture coverage (go/pkg/workflowgenerate/shapes_divergent.go:9-26,
go/pkg/workflowgenerate/shapes_divergent_test.go:52-130,
go/pkg/adapterconformance/divergent_ideation_test.go:3-31).

Keep the deferred join barrier and manifest follow-up visible, but this area
does not block acceptance of the reset plan.

## Required Conditions Before Calling The Reset Complete

1. The current-state surface has one issue frontier: `docs/operator/BRIEF.md`,
   README status, docs index, TODO/roadmap disposition, and command-authority
   pointers agree or explicitly label older material as historical.
2. The #308/#309/#302 fixture passes as a release gate and demonstrates the
   intended recovery behavior without operator hand-finishing.
3. `doctor` stays green after that fixture, and warning classes are bounded by
   policy rather than left as a growing tolerated pile.
4. New workflow shapes, broader auto-spawn authority, and feature work remain
   frozen until the truth and recovery gates are closed or explicitly deferred
   by a new owner decision.

## Final Rationale

The plan is honest because it names the failures that still hurt. It is ruthless
because it prioritizes recovery correctness, doctor signal quality, and
current-state truth over new product surface. It is actionable if the next slice
treats stale operator truth as a blocking trust defect. Therefore:
`accept_with_findings`.
