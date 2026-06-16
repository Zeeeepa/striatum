---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: high
tags:
  - workflow-shape-governance
  - divergent-ideation
  - reliability-fixture
---

# Workflow Shape Audit Review
author: workflow-shape-auditor-codex-via-live-striatum-mcp-001
date: 2026-06-16
status: finding
verdict: accept_with_findings
target: divergent_ideation and workflow shape support-tier claims

## Summary

`divergent_ideation` should not remain advertised as broadly `supported` on the
current proof set. The shape has a real RFC 0105 fixture and the generator emits
an ordinary flat workflow graph, so the concept should not be deleted. The
operator-facing support tier is still too strong: the fixture proves structural
DAG readiness and branch-death recovery, while the recent reliability incidents
and D206 follow-up show the dangerous surface is artifact-bearing fan-in,
worktree/run-branch integration, final-job recovery, and support-tier honesty.

Recommendation: freeze `divergent_ideation` for production/default use and
demote the catalog claim to `experimental` or `supported: structural lifecycle
only` until the graduation gate covers artifact publication, fan-in durability,
worktree anchoring, doctor invariants, and final-job recovery.

## Should `divergent_ideation` Remain Supported, Be Demoted, Or Be Frozen?

Freeze and demote.

It should remain available as a supervised/manual shape because it is a useful
design-space widening pattern and its generator does not add daemon methods or
special state transitions. It should not remain in the same `supported` tier as
smaller, convergent shapes until its support evidence matches the operator claim.

The catalog and workflow-type guide say `divergent_ideation` is `supported`
because it has a green RFC 0105 unattended-reliability fixture. D199 records the
same basis. That basis is true but incomplete: the fixture proves double
fan-out/join scheduling and recovery from one dead branch in either fan-out. It
does not prove the artifact-bearing, git-integrated, multi-lane production path
that failed in the recent reset context. The operator brief records that the
#290/#296 divergent-ideation runs wedged at final jobs, that #290 exposed
fan-in sibling work stranded outside the run branch, and that D206 landed only
the per-completion fan-in integration slice while the post-completion join
barrier and join manifest remain follow-ups.

## What Its Reliability Fixture Did And Did Not Prove

What it proved:

- The graph shape can drive frame -> diverge branches -> converge -> deepen
  branches -> final synthesis to `completed` through production mutation
  handlers.
- The first join waits for every diverge branch and the final join waits for
  every deepen branch.
- A hard dead-lane fault in a diverge branch or in a deepen branch is requeued
  by the production sweep on the same attempt, without escalation, and the
  downstream join does not fire early.

What it did not prove:

- It did not run the generated workflow tree end to end from `workflow generate`
  output; the fixture seeds a reduced workflow in the test harness.
- It did not publish required artifacts. The fixture explicitly uses
  document-only draft jobs with no required artifacts, so it bypasses
  `artifact.publish`, finding/synthesis front matter, immutable logical names,
  body reconstructability, and review/verdict gates.
- It did not exercise per-job worktree creation, artifact reads from worktrees,
  run-branch anchoring, fan-in sibling git integration, stranded in-scope path
  warnings, or doctor ref-safety checks.
- It did not prove multi-model semantics beyond generator metadata. The
  round-robin generator test checks lane distribution, but the conformance
  fixture uses synthetic sessions and a single display model.
- It did not cover final synthesis failure after all fan-in inputs exist, the
  `agent_exited_unsealed`/auto-finalize class, or run-drive behavior.
- It did not cover higher cardinality beyond the fixture's branch_count=3 and
  deepen_count=2, while the public shape defaults to 5/3 and allows 8/5.

That means the fixture is a good structural lifecycle test, not a sufficient
support proof for an unattended operator shape that promises durable artifacts.

## Fan-In/Fan-Out And Write-Scope Risks

The generated graph is intentionally ordinary: `parallel_group` fan-outs plus
edges, fresh diverge/deepen sessions, unique branch/deepen artifact subtrees,
and a convergence/final lane. That is the right implementation direction
because it keeps the shape inside existing scheduler semantics.

The risk is that the shape multiplies every known boundary:

- It has two fan-outs and two joins, so any dependency-readiness, stale-lease, or
  recovery bug gets a second chance to wedge the run.
- It fans multiple repo-writing artifact jobs into a downstream reader. D206
  fixed the specific "later sibling pinned but not integrated into the run
  branch" bug, but the decision itself leaves the post-completion join barrier
  and join manifest as follow-ups.
- Its value proposition depends on branch isolation: diverge branches should not
  see each other, and deepen jobs should consume only the convergence ledger and
  problem brief. A fan-in reachability defect silently changes what later jobs
  can read.
- Validator rules require disjoint parallel write scopes or review-only unique
  artifact paths, but validation alone does not prove the runtime porter,
  worktree, branch, and artifact stores preserve every sibling's output.
- D197/D203 show that source/artifact publication is now more explicit and
  louder, but those protections need shape-level coverage before a complex
  fan-out shape claims the same support tier as simpler flows.

The downstream write-scope envelope in this run is narrow. The reset synthesis,
support ledger, evidence audit, and final review may record recommendations
under their artifact paths; actual fixes would need a later implementation job
with scope over workflow generator code, conformance tests, catalog docs, and
decision/TODO updates.

## Shape Catalog Support-Tier Honesty

The current catalog rule is too binary. "`supported` means has a green RFC 0105
fixture" is mechanically checkable, but it lets a narrow fixture certify a broad
operator expectation. For `divergent_ideation`, the support tier currently reads
as "safe to run unattended" even though the cited fixture excludes the
artifact/git/provenance path where recent incidents landed.

Support tiers should say what is proven:

- `supported` should mean generated workflow + production handlers + durable
  artifact path + recovery + doctor invariants are covered for the shape's
  distinctive risk.
- `experimental` should mean useful but supervised, with known unsupported
  runtime or durability surfaces.
- If a middle tier is desired, call it explicitly, for example "structural
  supported" or "supported for lifecycle only." Do not let the main catalog use
  the same label for lifecycle-only and production-artifact-proofed shapes.

The shape-tier guard should compare registry support claims to coverage classes,
not only to the existence of one fixture entry. The fixture entry for a shape
should name the risk cells it covers and the cells still absent.

## Minimum Graduation Gate For Any Future Shape

No future shape should graduate on graph completion alone. The minimum gate
should be:

1. Generate the actual workflow from catalog inputs, validate it, and drive that
   generated workflow rather than a hand-seeded approximation.
2. Publish every expected artifact through `artifact.publish` or
   `review.submit`, including valid front matter for schema-bearing kinds.
3. Exercise the shape's distinctive fan-out/fan-in points with real required
   artifacts, not empty document-only jobs.
4. If any lane can write repo content, require per-job worktree isolation,
   run-branch anchoring, artifact reconstructability, and doctor ref-safety
   assertions after completion.
5. Inject recovery faults in each structurally distinct fan-out/join and in the
   final gate job after all upstream inputs exist.
6. Verify no downstream job becomes claimable until every live predecessor
   attempt has completed and its required artifact body is reconstructable.
7. Assert support-tier docs, workflow catalog metadata, and conformance fixture
   coverage stay in sync.
8. If a shape advertises multi-model signal, prove the lane/family routing claim
   in a fixture or downgrade the claim to an advisory generator property.
9. Run through the same daemon MCP/RPC authority path used by lanes, or state
   clearly that the fixture covers only lower-level mutation handlers.
10. Require a clean terminal `doctor` projection or an explicit documented
    warning baseline for any expected residue.

## Delete/Demote/Freeze Recommendations

- Do not delete `divergent_ideation`; the shape is useful and implemented in
  ordinary runner primitives.
- Freeze new default/recommended production use until the support proof is
  expanded.
- Demote the catalog tier from `supported` to `experimental`, or introduce a
  narrower tier label that does not imply artifact-path reliability.
- Keep the generator available behind explicit operator choice and supervision.
- Add a conformance fixture that drives the generated default shape with
  artifact publication, worktree anchoring, and final-synthesis recovery.
- Add a maximum-cardinality or at least higher-cardinality stress cell before
  claiming the 8-branch/5-deepen envelope is supported.
- Add a guardrail that prevents support-tier promotion unless the fixture
  declares coverage for the shape's distinctive risk cells.
- Record D206's deferred join barrier and join manifest as graduation blockers
  for this shape, not merely nice-to-have follow-ups.

Overall: the incident did not prove `divergent_ideation` is a bad product idea.
It proved the current support label outran the evidence. The reset plan should
treat shape support as a contract with named proof obligations, not as a badge
earned by one green happy/recovery fixture.
