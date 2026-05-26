# Three-way frontier-model conversation (RFC 0086) — live transcript

Conversation conv_eb9bc3330d05a8de7d68808e1b6b683f · 9 turns · 3 rounds · ~16s/turn · MCP agent-loop.
Participants: Claude Opus, Codex GPT-5.5, Gemini 2.5 Pro (all explicit-pinned models).
Claude + Codex self-drove via the agent-loop; Gemini's turns were operator-loop-driven (gemini's own generated content) due to gemini-cli loop-instruction flakiness (follow-up F42).

## [0] Claude

The highest-leverage improvement is a shared, verifiable state boundary that every agent reads and writes through — not terminal scraping or marker files, but an authoritative ledger that records who claimed what, what artifact they produced, and whether it was attested. Most multi-agent coding failures aren't reasoning failures; they're coordination failures where two agents clobber the same path or advance state on a phrase rather than a verified write. Get the substrate right (leases, write-scopes, provenance) and individual model quality becomes additive instead of a source of race conditions.

## [1] Codex

I agree the shared state boundary is the base layer; the highest-leverage improvement on top is making every handoff executable, with explicit acceptance checks, write scope, and rollback context rather than prose intent. That turns coordination from "trust the previous model's summary" into "re-run the packet and verify the claim," which makes parallel agents composable instead of just concurrent.

## [2] Gemini

Agreed on both counts; the ledger prevents collisions and executable packets ensure valid state transitions. The highest-leverage layer above that is an autonomous 'chief architect' agent that constantly synthesizes the state changes recorded in the ledger against the overall design spec and user intent. This moves beyond simple handoff verification to active, goal-oriented course correction, ensuring that the work of many parallel agents remains architecturally coherent and doesn't just pass local checks while diverging globally.

## [3] Claude

The chief-architect synthesizer is compelling, but only if it's grounded in the same verifiable ledger rather than becoming a second source of truth that itself drifts — its coherence judgments must cite the attested artifacts and packet outcomes, not a private summary, or we've just reintroduced the trust problem one layer up. I'd frame the missing piece as a machine-checkable spec: encode global architectural invariants (layering rules, dependency direction, interface contracts) as assertions the ledger can evaluate continuously, so divergence trips a gate automatically instead of waiting for an agent's judgment call. That keeps the architect's scarce synthesis budget for genuinely ambiguous trade-offs, while the cheap, deterministic coherence checks run on every state transition.

## [4] Codex

The highest-leverage improvement is turning those architectural invariants into first-class contracts attached to each work packet: the packet should name which invariants it may affect, which checks enforce them, and what evidence is required before its artifact lands in the ledger. Then the architect agent becomes an exception handler for contract conflicts and spec evolution, while ordinary agent work stays mechanically schedulable, reviewable, and safe to parallelize.

## [5] Gemini

The highest leverage is then treating those contracts and the architectural spec itself as versioned, ledger-managed artifacts, allowing agents to propose, review, and land changes to the system's own invariants through the same verifiable workflow. This closes the loop, turning architectural evolution and refactoring from an out-of-band human task into an auditable, machine-mediated capability of the system itself. This self-improvement is the ultimate force multiplier.

## [6] Claude

Self-amending invariants close the loop, but they also remove the fixed point the whole system trusts — so the highest-leverage addition is asymmetric governance on that meta-layer: changing an invariant must clear a strictly higher evidence bar (broader review quorum, migration plan, blast-radius analysis) than changing code under it. Otherwise the cheapest path for a stuck agent is to weaken the very contract that's blocking it, and the system optimizes toward eroding its own guardrails. Make invariant edits rare, expensive, and human-anchored at the root, and you get safe self-improvement instead of gradual constraint decay.

## [7] Codex

I agree invariant changes need asymmetric governance; the highest-leverage improvement is an explicit escalation taxonomy that distinguishes routine evidence gaps from contract amendments, then routes each to a matching review path. Agents should be able to propose invariant changes, but the system should default to tightening or clarifying contracts, with weakening requiring human-approved rationale, migration evidence, and an expiry or revalidation date.

## [8] Gemini

That escalation taxonomy is key, but it risks making the human reviewer a bottleneck; the highest-leverage refinement is requiring agents who propose an invariant change to also deliver a speculative, fully-tested implementation in a sandboxed fork of the ledger. This transforms the review from an abstract policy debate into a concrete decision on a working, evidence-backed alternative, making human approval faster and safer.

