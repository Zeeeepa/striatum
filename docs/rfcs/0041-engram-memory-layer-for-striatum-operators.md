# RFC 0041: Engram as an Optional Memory Layer for Striatum Operators

Status: proposed (design-shape only — no acceptance criteria fully specified;
the design phase consumes this and produces the concrete plan)
Date: 2026-05-13
Context:
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D006, D007, D058, D083, D087, D092),
[`RFC 0032`](0032-cross-repo-workflows-and-mcp-mutation-capabilities.md),
[`RFC 0036`](0036-mcp-harness-for-daemon-v2-mutation-surface.md),
[`RFC 0040`](0040-mcp-driven-dogfood-harness.md),
`~/git/engram/` (sibling repository — Engram, the memory project Striatum was
extracted from per `docs/ENGRAM_INCUBATION_CONTEXT.md`).

## Problem

Two pains are converging.

**On the Striatum side**: the AI operator driving dogfoods (this Claude Code
session, in current practice) carries context only within a single session.
Across sessions the operator has a thin `~/.claude/memory/` auto-memory layer
(useful but drift-prone over weeks) plus whatever the harness compacts into
the session-start summary. The runner's own decision log, operator reports,
RFCs, commit history, audit chain, and run summaries are a rich corpus that
the operator already produces — but doesn't read back as memory. Compaction
events lose detail. Across sessions, "what friction patterns recurred", "why
did D092 supersede D073", and "which RFC moved the no-node rule" require the
operator to rediscover from scratch rather than recall. The cumulative cost
is large: every fresh session re-reads CLAUDE.md / AGENTS.md / INDEX.md and
gets a compacted summary, but the *operational* memory (specific friction
shapes, prior interventions, the path through prior dogfoods) is lossy.

**On the Engram side**: the original project (a sibling repo at
`~/git/engram/`) halted mid-design when the provenance problem got hard. The
original target dataset is the operator's personal life — chat logs, product
discussions, relationship questions, daily decisions. That domain has squishy
ground truth: "is Jennifer really avoidant" has no objective answer, so the
belief / provenance / confidence model had to do real philosophical work
before any serving layer could ship. The build halted there.

The convergence: the Striatum dogfood corpus is a **dataset with free
provenance**. Every claim in a decision log row is grounded in a commit hash.
Every artifact in an operator report has a sha256. Every verdict in the audit
chain is a signed transition. The "is this claim true" question collapses to
"is this commit in the tree." For a memory system, that's the easiest
grounding case available.

RFC 0041 proposes that Engram **augment** its current mission (personal-life
memory with the provenance problem still being designed) by **also serving
as memory for Striatum operators** over the structurally-easier
software-building corpus. The personal-life mission stays the long-term goal;
software-building is a faster path to a working serving layer and validates
the architecture under cleaner conditions.

This RFC is **design-shape only**: it does not propose acceptance criteria
or a concrete implementation. It scopes the design phase that follows,
including required reading and a roadmap. The actual technical decisions
(retrieval model, embedding choice, claim schema, serving topology) get
made by the design agents downstream and synthesized into a follow-up RFC
(tentatively RFC 0042 V1) before any implementation work begins.

## Goals

- Frame Engram as an **optional augmentation layer** for Striatum operators.
- Preserve Engram's existing mission (personal-life memory + provenance
  research). Do not redirect Engram entirely. The personal-life dataset
  stays the long-term goal; software-building is the near-term validation
  surface.
- Use the design phase to ground all proposals in Engram's actual current
  state (read all `.md` files under `~/git/engram/`) rather than guessing at
  its architecture.
- Define the integration surface: read-only retrieval first, write-side
  later, augmentation-not-replacement throughout.
- Produce a multi-phase roadmap so the integration lands incrementally
  without coupling Striatum's critical path to Engram availability.

## Non-Goals

- A finished memory architecture for Engram. The design phase produces
  that; this RFC scopes the phase.
- Replacing `~/.claude/memory/` or Striatum's `.striatum/retired-local-state`
  state store. Engram is augmentation; if Engram is unavailable, Striatum
  must still run.
- Hosted-mode Engram, multi-tenant access, or remote retrieval. Per D083:
  single-user single-machine. Engram lives in `~/git/engram/` as a local
  service.
- Rewriting Engram's existing claims / beliefs / embeddings schemas. The
  design phase augments them with Striatum-domain corpus support; it does
  not redesign them.
- Promoting Engram-backed memory above the audit chain / decision log /
  operator reports as the source of truth. Those repository files remain
  authoritative; Engram derives from them.
- Cross-machine memory sync. Each operator's Engram instance is local.

## Design-Phase Directive

The design agents driving the next dogfood for this RFC **MUST**, before
proposing anything, read the markdown documentation under `~/git/engram/`.
Specifically:

- All `.md` files at `~/git/engram/` (README, AGENTS.md, CLAUDE.md, SPEC.md,
  DECISION_LOG.md, ROADMAP.md, BUILD_PHASES.md, HUMAN_REQUIREMENTS.md,
  CHANGELOG.md, etc.).
- All `.md` files under `~/git/engram/docs/` (claims_beliefs.md,
  ingestion.md, segmentation.md, and any other surfaces).
- The contents of `~/git/engram/docs/specs/`, `~/git/engram/docs/design/`,
  `~/git/engram/docs/rfcs/`, `~/git/engram/docs/schema/`,
  `~/git/engram/docs/process/`, `~/git/engram/docs/howto/`.
- The structure of `~/git/engram/migrations/` and
  `~/git/engram/agent-runner/` to understand the service shape and any
  existing automation.
- `~/git/engram/docker-compose.yml` to understand the runtime
  topology.

If a design agent does not read those files, it has no grounding for
proposals and the design will be rejected by the design review. The
synthesis is required to cite specific Engram concepts (claims, beliefs,
ingestion, segmentation, schemas) accurately from the actual docs, not
invented from analogy.

**The design agents' goal is to AUGMENT Engram's current mission, not
change it entirely.** Concretely:

- Engram's existing personal-life ingestion pipeline stays.
- Engram's existing claims / beliefs / provenance model stays.
- Engram's existing storage substrate stays (whatever it is per its
  actual docs).
- A new ingestion path for Striatum's software-building corpus
  (commits, decisions, operator reports, RFCs, audit chain, run
  summaries) is added.
- A new query / retrieval surface scoped to the Striatum corpus is
  added (so personal-life and software-building corpus retrievals
  remain separate by default).
- A new MCP server interface exposes Engram's retrieval surface to
  Striatum operators (claude_code, codex, gemini, any frontier-model
  CLI speaking MCP).

If the design synthesis proposes changes to Engram's core architecture
(provenance model, claim schema, belief lifecycle) the design review must
flag it as out-of-scope and bounce to `needs_revision`. The mission is
augmentation, not redesign.

## Two Operator Surfaces

This RFC names two distinct operator-side consumers of the Engram memory
layer. Both are optional augmentations.

### 1. Striatum's built-in operator (this Claude Code session in current practice)

The AI driving dogfoods. Currently uses `~/.claude/memory/` for auto-memory
plus per-session context loading. Augmentation: Engram-backed retrieval over
the Striatum corpus so the operator can ask:

- "What friction patterns recurred across dogfoods 036-039?"
- "Which RFC moved the no-node toolchain rule and why?"
- "Has surgical_recovery been invoked before? With what reason?"
- "What did the build review for dogfood-037 say about test coverage?"
- "Which dogfoods touched the daemon RPC capability vocabulary?"

The operator's session-start brief includes "if Engram is available, retrieve
context relevant to the dogfood / RFC / decision in scope before starting
work." Memory is augmentation, not replacement — the operator's first-class
sources remain AGENTS.md, the active workflow.json, and the explicit
`context_docs` block. Engram retrieval is supplementary.

### 2. Frontier-model CLI as Operator (Codex, Gemini, etc.)

Future direction: any frontier-model CLI speaking MCP can act as the
Striatum operator (currently only Claude Code is wired). Each such CLI gets
the same Engram MCP server interface. This is the more interesting case
because non-Claude operators don't have `~/.claude/memory/` at all; Engram
is the only persistent memory available to them.

The MCP server interface for Engram exposes a small closed set of tools
(scoped during the design phase, not predetermined here): retrieval over
the Striatum corpus, optionally retrieval over the personal-life corpus
when the operator has the appropriate capability token. Mutation tools
(write claims, derive beliefs) are deferred to a later phase — V1 is
retrieval-only.

## Integration Surface (design phase resolves shape)

The design agents must scope each of these and pick concrete shapes:

1. **Corpus ingestion**: how does the Striatum corpus (decisions, operator
   reports, RFCs, commits, audit chain, run summaries) flow into Engram?
   Pull mode (Engram cron job polls Striatum)? Push mode (Striatum emits
   events to Engram on `run.completed`)? Both? Defer write-side V1?
2. **Embedding choice**: Engram's existing embeddings stay; what's added
   for the Striatum corpus? Local embeddings (no network), per existing
   Engram posture.
3. **Claim / belief schemas for software-building**: do existing Engram
   schemas cover commits, decisions, audit chains? If not, what's the
   minimum augmentation?
4. **MCP server topology**: standalone Engram MCP server in
   `~/git/engram/agent-runner/` (or wherever its existing service runs)?
   Or wrap Engram retrieval as Striatum chat tools per RFC 0036 V1
   pattern? Either works; design phase picks.
5. **Capability vocabulary for Engram MCP**: new capabilities (`memory.read`,
   `memory.read_personal`, etc.) added to RFC 0030's closed set? Or
   Engram-local capabilities decoupled from Striatum's? Probably
   Engram-local since the personal-life corpus is out of scope for
   Striatum auth.
6. **Striatum-side wiring**: how does the operator's session bootstrap
   Engram-backed retrieval? An MCP server registration in
   `~/.claude/settings.json`? A `striatum operator memory init` CLI verb?
   A workflow.json `memory_provider` field?
7. **Augmentation-not-dependency boundary**: if Engram is unavailable,
   Striatum runs fine. The operator's prompts and CLI verbs must work
   without Engram. How is that enforced (timeouts, optional retrieval
   tools, graceful degradation)?
8. **Test-set seeding**: what's the smallest corpus that exercises the
   pipeline end-to-end? Recommendation: dogfoods 035-040 + decision log
   D080-D092 + RFCs 0030-0040 + the most recent ~20 commits. Small enough
   to iterate quickly, big enough to produce real retrieval signal.

## Roadmap (4 phases)

Each phase is scoped to a single RFC + dogfood + decision-log entry. The
design phase for **this** RFC produces the Phase 1 scope. Phases 2-4 are
proposed here so the design agents have context for what's coming.

### Phase 1 — Read-only Engram MCP server over the Striatum corpus

- Engram ingestion path for the Striatum corpus (commits, decisions,
  RFCs, operator reports, audit chain, run summaries).
- Engram MCP server exposes retrieval over the Striatum corpus.
- Striatum operator's session optionally consumes the retrieval.
- No write-side. No personal-life corpus changes. No Engram architectural
  changes.
- Test set: dogfoods 035-040 + decisions D080-D092 + RFCs 0030-0040.

### Phase 2 — Operator-side context injection

- The Striatum operator's session-start brief auto-retrieves relevant
  context for the active dogfood / RFC / decision before starting.
- The operator's prompts include "retrieve from Engram" as an
  early-step convention rather than a fallback.
- The chat-tool surface (RFC 0036 V1) gains Engram retrieval as a new
  closed-set tool family.
- Frontier-model CLI operators (codex, gemini) get the same MCP
  interface.

### Phase 3 — Write-side: dogfoods produce Engram claims

- `run.completed` events trigger Engram ingestion of the new run's
  artifacts.
- D091 OPERATOR_REPORT.md sections become Engram claims with
  audit-chain-grounded provenance.
- The decision log + RFC status transitions become Engram beliefs with
  decision-log-row provenance.
- The audit chain's hash-chained integrity becomes the verification
  surface for any derived Engram claim.

### Phase 4 — Personal-life corpus integration

- After Phases 1-3 prove the pipeline at scale on the easier domain,
  re-attack the original Engram mission with the validated
  architecture.
- The grounding-from-software-building experience informs how
  squishy-ground-truth claims (Jennifer-avoidant, toothpaste choice)
  get represented.
- Engram's personal-life corpus and Striatum corpus stay separated by
  default; cross-corpus retrieval requires explicit operator capability.

## Augmentation-Not-Replacement Boundary (hard)

Three rules the design phase must honor:

1. **Striatum's `.striatum/retired-local-state` and the daemon DB stay the
   authoritative live state for runs, jobs, leases, queue messages,
   sessions, and audit rows.** Engram derives memory FROM those stores; it
   does not own any of them.
2. **The decision log, operator reports, RFC bodies, and commit history
   stay the authoritative provenance.** Engram derives claims and beliefs
   FROM those documents; it does not replace them.
3. **If Engram is unavailable, Striatum runs.** No Striatum critical-path
   operation may block on an Engram call. Operator retrieval that times
   out falls back to the existing pre-Engram operator behavior.

## Open Questions for the Design Phase

These are explicitly NOT resolved here; the design phase decides:

- Phase 1 scope: how much of the Striatum corpus lands in Engram for the
  first read-only retrieval (everything since dogfood-001? since the
  daemon V2 dogfoods? a configurable cutoff?).
- Whether Phase 1's Engram instance reuses Engram's existing storage
  substrate or stands up a parallel Striatum-corpus-specific one.
- Whether the MCP server is standalone or wraps as Striatum chat tools.
- Whether Engram becomes a `psycopg`-style optional dependency of
  Striatum or stays fully external.
- What the operator's first-line retrieval call shape looks like
  (`memory.retrieve(query, corpus="striatum", k=10)` or similar).
- How retrieval failure / latency degrades gracefully.
- How Striatum's `~/.claude/memory/` per-project auto-memory and Engram's
  cross-session memory layer overlap or specialize.
- Whether the design phase produces RFC 0042 (Phase 1 implementation
  spec) or whether RFC 0041 itself gets revised post-design with concrete
  acceptance criteria.

## Domain Modeling (preliminary, design-phase refines)

This RFC names but does not finalize the following:

- **Memory layer**: an optional augmentation surface providing retrieval
  over a corpus. For Striatum, the corpus is the software-building
  artifacts produced by the runner.
- **Corpus**: a closed set of source documents Engram indexes. V1 corpuses:
  Striatum-software-building (this RFC's scope) and personal-life
  (Engram's existing mission). Cross-corpus retrieval requires explicit
  operator capability.
- **Retrieval**: a single read-only operation returning ranked corpus
  references with their provenance metadata (commit hash, decision-log
  row, audit chain entry, RFC version, etc.).
- **Engram MCP server**: the IPC surface exposing retrieval to MCP
  clients (Striatum operators, frontier-model CLIs).
- **Striatum operator's session brief**: the per-session context-loading
  surface that, in Phase 2, auto-invokes Engram retrieval before starting
  work.

The design phase confirms or adjusts these definitions against Engram's
actual existing vocabulary.

## Followup RFCs

- **RFC 0042** (proposed in the design phase for this RFC): Phase 1
  implementation spec with concrete acceptance criteria.
- **RFC 0043** (Phase 2): operator-side context injection + chat-tool
  expansion.
- **RFC 0044** (Phase 3): write-side dogfood → Engram claim flow.
- **RFC 0045** (Phase 4): personal-life corpus re-attack with the
  validated architecture.

Each follow-up RFC is scoped after its predecessor lands so design
decisions stay grounded in actual operational experience rather than
projection.
