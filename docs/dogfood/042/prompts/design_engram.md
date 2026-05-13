# Track B Design Prompt: Engram Phase 1 RFC 0044

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/042/track_b/design/<lane>/`).

**MUST READ before designing**: all `.md` files under `~/git/engram/` — `README.md`, `AGENTS.md`, `CLAUDE.md`, `SPEC.md`, `DECISION_LOG.md`, `ROADMAP.md`, `BUILD_PHASES.md`, `HUMAN_REQUIREMENTS.md`, plus everything under `~/git/engram/docs/` (`claims_beliefs.md`, `ingestion.md`, `segmentation.md`, schemas, specs, design docs, RFCs).

Cite Engram's actual vocabulary (claims, beliefs, ingestion, segmentation, etc.) accurately. If a synthesis invents schemas not present in Engram's docs, the design review must bounce.

Design **RFC 0044 V1 acceptance criteria** for Engram Phase 1: a read-only Engram MCP server over the Striatum software-building corpus (commits, decisions, operator reports, RFCs, audit chain, run summaries). Augment Engram's current mission; do not replace it.

Cover:

- Engram corpus ingestion path for Striatum artifacts (pull mode? push mode? cron sweep? operator-triggered?).
- New `striatum` corpus alongside Engram's existing personal-life corpus. Cross-corpus retrieval requires explicit capability.
- Engram MCP server topology — standalone in `~/git/engram/agent-runner/`? Wrapped as Striatum chat tools per RFC 0036 pattern?
- Capability vocabulary for Engram MCP — Engram-local or shared with Striatum's RFC 0030 set?
- Striatum-side wiring: how the operator session bootstraps Engram retrieval (MCP server registration? CLI verb?).
- Augmentation-not-dependency: Striatum must run without Engram.

Do NOT redesign Engram's claims / beliefs / ingestion / segmentation schemas. Augment, don't replace.

State what cannot be claimed (cross-machine, hosted, multi-tenant).

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.

One-shot supervised invocation. Write the artifact directly.
