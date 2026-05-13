# Track B Implement: Draft RFC 0044 Body (codex)

Blocked until `review_engram_design` returns an accepting verdict.

Write `docs/rfcs/0044-engram-phase-1-implementation-spec.md` from `docs/dogfood/042/track_b/DESIGN_SYNTHESIS.md`.

The RFC 0044 body must include:

- Status: `proposed`
- Context links to RFC 0041 (which scoped the design phase), RFC 0036 (chat tools pattern), RFC 0030 (RPC capability vocabulary), and the Engram repo as authoritative external dependency
- Problem (cite RFC 0041's framing)
- Goals (acceptance-criteria-bearing)
- Non-Goals
- Proposal (detailed enough for a future dogfood to implement)
- Acceptance Criteria (concrete bullets)
- Implementation Plan (multi-step if needed)
- Open Questions
- Domain Modeling

Cite Engram's actual concepts (claims, beliefs, ingestion, segmentation, schemas) from `~/git/engram/` docs. If you invent schema names that aren't in Engram's actual docs, the build review will reject.

**Do NOT update `docs/rfcs/README.md`, `docs/TODO.md`, or `CHANGELOG.md`** — the `consolidate_phase_1` job handles those after all three tracks complete.

**Use sub-agents aggressively**: spawn one sub-agent per RFC section (Context, Problem, Goals, Non-Goals, Proposal, Acceptance Criteria, Implementation Plan, Open Questions, Domain Modeling) so they run in parallel. Reconcile their output into the final RFC body yourself. Also dispatch a sub-agent to read all `.md` files under `~/git/engram/` and summarize Engram's vocabulary, schemas, and capability boundaries — feed that summary to the per-section sub-agents.

Output: `docs/rfcs/0044-engram-phase-1-implementation-spec.md` + `docs/dogfood/042/track_b/build/HANDOFF.md`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifacts and exit normally.
