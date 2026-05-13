---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0044", "track_b", "design"]
---

author: reviewer-claude-opus-003

# Track B Design Review — Engram Phase 1 (ergonomics_dx)

Review target: `docs/dogfood/042/track_b/DESIGN_SYNTHESIS.md` (RFC 0044 V1
synthesis, codex synthesizer, eight-decision form). Posture per work packet:
`ergonomics_dx`. Lens: a first-time Striatum operator (Claude Code, codex,
or gemini) opening a fresh session against a repo where Engram has just been
wired in. Are the affordances discoverable and consistent?

## Verdict: accept_with_findings

The synthesis lands the four ergonomics-critical points: augmentation-not-
replacement is mechanically enforced, Engram's claims/beliefs schema is
preserved (Striatum corpus stays raw-evidence in V1, no belief consolidation),
the read-only retrieval path is reachable from a single skill plus four
narrowly-shaped MCP tools, and the Striatum-runs-without-Engram fallback has a
concrete acceptance test. The findings below are polish items the RFC author
can resolve at drafting time without re-running the design phase.

## What the synthesis gets right (load-bearing for the verdict)

### 1. Augmentation-not-replacement boundary is concretely enforced

D8 names three mechanical rules — zero Engram client imports under
`src/striatum/cli/**`, zero `memory.*` capability additions to Striatum's
RFC 0030 daemon RPC registry, and an "Engram off" acceptance test producing
byte-identical Striatum artifacts. These are auditable post-merge (grep is
the test). The synthesis also pins per-call timeouts (3s session-start,
2s search, 5s fetch_reference) and asserts no Striatum state transition
ever blocks on Engram. For a first-time operator, this means: nothing they
do via `striatum ack/complete/verdict/publish-artifact` can stall because
Engram isn't running. That is the right ergonomic floor.

### 2. Engram's existing schema vocabulary is cited verbatim

The "Engram Vocabulary Citations (Ground Truth)" block at the top is
load-bearing. It pulls `source_kind` enum values, the
claims-bound-to-segment-with-`evidence_message_ids` shape, the bitemporal
status-tracked stability-classed beliefs, the segmenter's local
ik-llama endpoint, the `nomic-embed-text` 768-dim HNSW + pgvector storage,
and the postgresql:///engram local socket — each anchored to a specific
file under `~/git/engram/`. The decisions downstream (D2 in particular)
are derived from those citations rather than invented. This satisfies
RFC 0041's design-phase directive ("cite specific Engram concepts
accurately from the actual docs, not invented from analogy").

### 3. The corpus_id-as-orthogonal-discriminator decision is the right one

D2 separates `source_kind` (ingest taxonomy: which parser produced this
row) from `corpus_id` (biographical scope: personal vs striatum), and
explicitly rejects the two seductive shortcuts — overloading
`source_kind='capture'` (which would collide with the reclassification
flow) and stashing a discriminator inside `raw_payload` (which would
force JSONB filters at retrieval time). For a first-time operator on the
default Striatum token, retrieval defaults to `corpus_id='striatum'` and
cross-corpus queries refuse without an explicit capability. This is the
ergonomically correct default: an operator never accidentally pulls
personal-life rows into a software-building session.

### 4. The four-tool MCP surface is small and read-only

`engram.search`, `engram.fetch_reference`, `engram.describe_corpus`,
`engram.health`. Every tool is read-only. Every result carries
`corpus_id`, `source_kind`, `sub_kind`, `privacy_tier`, and a
reconstructable `provenance` block. There is no `claim_create`, no
`belief_revise`, no raw SQL surface. A first-time operator can learn the
surface in five minutes from the `striatum-engram` skill.

### 5. The capability matrix is unambiguous

D5's four capabilities — `memory.read_striatum`, `memory.describe`,
`memory.read_personal`, `memory.read_cross_corpus` — with explicit
defaults (first two yes, last two no) makes the cross-corpus question a
deliberate operator decision, not an accident. The Engram-local
capability registry living in `~/.config/engram/` separately from
Striatum's `${XDG_RUNTIME_DIR}/striatum/` token store means the operator
session juggles two registries, which the skill body teaches.

### 6. Discoverability surface is balanced

`striatum-engram` skill installed in the RFC 0015 bundle (visible at
session start), `striatum operator memory check` (read-only,
informational, always exits 0), `striatum corpus export` (the export
boundary), plus documentation in HOW_TO_HUMAN.md and HOW_TO_AGENT.md.
There is no daemon RPC method, no `workflow.json` field, no chat-tool
expansion. The operator finds Engram via the skill, learns the four
tools, and adds the MCP server config to `~/.claude/settings.json` once
(the snippet is given verbatim in D7). This is the right shape: the
runner is not in the business of writing the operator's MCP config.

## Findings (low severity — RFC-authoring resolves)

The following do not block acceptance. They are ergonomic refinements
the implementer should address while authoring `docs/rfcs/0044-engram-
phase-1-implementation-spec.md`.

### F1. `engram-mcp-stdio --health-check` shape is underspecified

The synthesis cites `engram-mcp-stdio --health-check` twice (D7 §2 for
the check verb, and the acceptance criterion C3). But D4 describes
`engram-mcp-stdio` as a stdio MCP transport binary, and D6 lists
`engram.health` as a *tool* on that transport. It is not obvious whether
`--health-check` is (a) a separate CLI mode that bypasses MCP and exits,
(b) a subcommand that opens the MCP transport, calls `engram.health`,
and exits, or (c) a degenerate stdio handshake that resolves on the
health tool. The implementer should pick (b) and write that down — it is
the cheapest path that keeps the binary single-purpose and makes
`striatum operator memory check` a one-line subprocess call with no
shared-handler imports.

### F2. `reference_id` shape is not pinned

`engram.search` returns objects carrying `reference_id`, which the
operator then passes to `engram.fetch_reference(reference_id)`. The
synthesis does not specify whether `reference_id` is the row's
`external_id` (already content-stable per D3), a Postgres surrogate
key, a composite `(corpus_id, source_kind, external_id, sub_kind)`,
or an opaque UUID minted at retrieval time. For an operator chaining
tool calls or pasting reference IDs into a `findings/` note, the shape
matters. Recommendation: make `reference_id` equal to
`<source_kind>:<external_id>` (i.e. `striatum:rfc:0044#summary`) so the
ID is human-readable, stable across re-ingest, and self-documenting.
This is a five-line addition to D3 / D6.

### F3. Privacy-tier vocabulary is opaque to a first-time operator

Every search result carries `privacy_tier`, and "Open Items 3" defers
default-tier choice to the RFC author. But the synthesis does not name
the tier vocabulary anywhere — the skill body cannot teach what
`privacy_tier: 2` means if the values are not enumerated. Two options
that resolve this:

1. List the tiers (Tier 0/1/2/3 or `public`/`commit_safe`/
   `operator_internal`/`restricted` or whatever Engram uses) once in
   the synthesis-derived RFC and have the `striatum-engram` skill body
   cite the same enum.
2. Or drop `privacy_tier` from the V1 result shape (since the Striatum
   corpus is by-construction redacted at the export boundary — no
   transcripts, no model output) and reintroduce it in Phase 3 when
   write-side ingestion brings in operator-report free-text that
   genuinely needs tiering.

Option 1 is the better fit because RFC 0041 §"Augmentation-Not-
Replacement" already implies a tier model, but the choice should be
explicit. Without it, the operator sees an unlabeled field and has no
prior for what to do with it.

### F4. Negative smoke "score floor" is referenced but not defined

Acceptance criterion E3 says negative-smoke queries must return "top-5
hits empty or below the documented score floor." The synthesis does
not document the score floor, define how it is computed, or where it
lives in operator-facing docs. For a first-time operator who runs
`engram.search("best pizza in Berlin")` and gets a low-score result, is
that a Striatum-corpus contamination bug, a retrieval-quality bug, or
working as intended? The implementer should pin the floor (a numeric
threshold derived from the simple weighted scorer Engram already commits
to per its SPEC §"Key design properties") and surface it in
`engram.describe_corpus` output so the operator can ask "what's my
score floor" without reading source.

### F5. Codex / gemini MCP config snippet is delegated

D7 §4 gives the Claude Code MCP config snippet verbatim and says "(or
the codex / gemini equivalent)." For Phase 1 this is acceptable —
non-Claude operators are RFC 0040 / harness-profile territory — but the
documentation acceptance criterion C5 should explicitly require that
`docs/HOW_TO_HUMAN.md` (or a sibling doc) carry the codex and gemini
snippets, or explicitly defer them to a future RFC with a named issue.
A first-time codex operator should not have to grep `engram-mcp-stdio
--help` to discover the launch shape.

### F6. `striatum operator memory check` exit-0 semantics need a clear output contract

D7 §2 says the check verb "prints status, always exits 0 even if Engram
is unreachable." This is correct (it is informational). But the
synthesis does not specify the output format. For a first-time operator,
the most ergonomic shape is a two-line summary plus a third line
explicitly stating "exit 0 by design; do not pipe into scripts as a
liveness probe." Without that third line, an operator (or a CI shim)
might wrap `striatum operator memory check && ...` and miss outages.
Recommendation: print to stdout a single JSON object matching
`engram.health`'s return shape, plus a stderr note that exit code is
intentionally unconditional. The implementer can pick whichever, but
the contract should be in the RFC, not deferred to the implementation.

### F7. Skill-bundle body should teach two failure modes explicitly

D7 §1 says the `striatum-engram` skill is "short and harmless when
Engram is offline." Two specific failure modes are worth teaching in
the skill body itself rather than leaving for the operator to discover:

1. What to do when `engram.search` returns a capability-refused error
   (which is the expected behavior if the operator tries
   `corpus="personal"` without `memory.read_personal`). The right
   action is "stop, do not loop, this is a deliberate refusal."
2. What to do when `engram.health` reports `corpus_status:
   striatum=empty` mid-session. The right action is "re-run
   `engram ingest-striatum` from the operator's shell; the session
   continues without retrieval until then."

These are two lines each in the skill template. They land
discoverability of failure recovery at the surface most operators
will read first.

### F8. Minor: RFC-number lineage

RFC 0041 §"Followup RFCs" originally pegs Phase 1 as "RFC 0042
(proposed in the design phase for this RFC): Phase 1 implementation
spec with concrete acceptance criteria." The synthesis correctly
re-targets to RFC 0044 (presumably because 0042/0043 were taken). The
RFC 0044 body should carry a single sentence in the "Context" or
"Supersedes" block recording this re-number so future readers
following the RFC 0041 → 0044 chain do not assume something
intermediate was skipped. Pure bookkeeping; flagged only because it is
an operator-discoverability concern for anyone reading the RFC index
top-to-bottom.

## Spot checks against Engram's actual schemas (synthesis claim → schema verification)

Per the review policy `access_scope: artifact_augmented`, I did not
re-read `~/git/engram/` directly; I verified internal consistency of
the cited shapes:

- **claims = insert-only, bound to segment, with `evidence_message_ids`,
  Phase 3 derives beliefs from them** — D2 + Non-Goals §"No claim or
  belief creation from the Striatum corpus in V1" preserve this. ✓
- **beliefs = bitemporal, status-tracked, stability-classed, never
  derive from other beliefs** — same Non-Goals row pins this. ✓
- **`source_kind` enum amended only via numbered SQL migration in
  filename order** — D2 §1 specifies `013_source_kind_striatum.sql`
  (provisional) following the `003_source_kind_claude.sql` /
  `005_source_kind_gemini.sql` precedent. ✓
- **Engram's no-egress invariant** — D4 §3 inherits the OS-level
  enforcement per D018, and D8 §3 makes "no outbound network from any
  Engram corpus-reading process" a Phase 1 Non-Goal. ✓
- **PostgreSQL local socket auth, 127.0.0.1 only** — D4 §2 carries
  `postgresql:///engram` and binding constraints. ✓

## Suggested non-blocking follow-up

The implementer's RFC should add a short "First-Time Operator Tour"
section under "Acceptance Criteria" (or as a sibling to "Ubiquitous
Language Additions") walking through:

1. `striatum operator memory check` (informational health).
2. `engram.describe_corpus()` (one MCP call to see what's there).
3. `engram.search("which RFC moved the no-node toolchain rule")`
   (a positive-smoke query the operator can run end-to-end).
4. What to do when each fails — Engram not running, corpus empty,
   capability refused.

This is the cheapest ergonomic-discoverability investment: it gives
the operator a 60-second "does it work" checklist that exercises every
surface introduced by V1. None of it requires new design; it is a
docs-only addition.

## Posture summary

The synthesis satisfies the ergonomics_dx bar: the affordances are
discoverable (one skill, one check verb, one export verb, four MCP
tools, one MCP config snippet), the augmentation-not-replacement
boundary is mechanically enforced, Engram's claims/beliefs schema is
preserved, and the Striatum-runs-without-Engram fallback has a concrete
acceptance test. The findings above are RFC-authoring polish, not
design-phase blockers.
