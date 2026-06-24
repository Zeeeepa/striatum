# Design-Run Seed — RFC 0052 (FRESH v1)

> Fresh v1 `falsification_gate` design run for RFC 0052 (committee deliberation
> workflow shape — arbitration, panels, adversarial review). **Required context
> docs** (read in full first):
> - `docs/rfcs/0052-committee-deliberation-workflow.md` — the RFC (the two costs it targets, the arbitrator role, the goals).

## Status note

RFC 0052 was **deferred/unscheduled (D225)** — kept live as a tracked future
capability. The maintainer re-confirmed it's wanted by launching this design run.
Scope a P0 that proves the shape end-to-end.

## Charter

The deliverable (committed `PROPOSAL.md`) is the falsifiable spec the
`rfc-0052-build` run executes: a **committee deliberation** workflow *shape*
where N producer roles deliberate to convergence under a named **arbitrator**,
with typed front-mattered durable **debate-turn** artifacts, before a downstream
consumer reads the output.

## What it must fix (the two dogfood costs)

1. **Iteration latency.** Every disagreement today is an artifact round-trip
   (publish → claim-next → ack → read → publish verdict); three-lane design
   phases stack round-trips. The committee makes disagreement a first-class
   *phase*, not a revision trigger.
2. **Reviewer co-blindness (D095–D102).** Same-lane implementer+reviewer share
   blind spots and rubber-stamp; cross-lane posture-mismatch treats real findings
   as baseline. The fix is **lane composition**, not posture labels (RFC 0018).

## The hard core to PROVE

- **The shape is generatable + runnable.** It compiles on the `workflow.v1.1`
  schema and the daemon mutation registry like the existing
  `falsification_gate` / `implementation_panel` / `cross_examination` /
  `adjudicated_constraint_extraction` shapes — jobs/edges/phases/roles the engine
  actually runs.
- **Deliberation TERMINATES.** A hard turn/round bound; the arbitrator can
  declare stalemate. No infinite debate.
- **The arbitrator is BOUNDED.** Record consensus / escalate to a panel / declare
  stalemate / sustain-or-end — defined so it cannot over-reach (impose a verdict
  = arbitrator capture) or under-reach (rubber-stamp non-consensus).
- **Co-blindness is STRUCTURALLY solved.** The shape enforces cross-model
  diversity + producer≠arbitrator, not merely recommends it.
- **Real value over existing shapes.** Not isomorphic to `cross_examination` or
  `implementation_panel` (the cross_examination-isomorphism trap from prior
  shape-graduation lessons). State precisely how it differs.
- **Debate turns are provenance, not transcript.** Typed front-mattered durable
  artifacts (schema-valid, anchored) — NOT a captured transcript (the product
  boundary forbids durable transcript capture; D028).

## Falsifier guidance

- **Falsifier 1 (shape-soundness / termination):** a non-terminating debate; an
  arbitrator over-reach/under-reach; a schema-invalid shape the compiler rejects;
  a deadlock under max_active_jobs / disjoint-write-scopes.
- **Falsifier 2 (co-blindness / value / provenance):** a co-blind committee that
  still rubber-stamps; an isomorphism to an existing shape; debate turns that are
  really a transcript; a latency claim that just moves the cost.

The adjudicator gates clearing on termination + bounded arbitrator + a runnable
schema-valid shape + structurally-solved co-blindness + real value + debate-turns
as provenance. Single v1 revision cycle; a second `needs_revision` routes to the
operator. Keep the local-first boundary.
