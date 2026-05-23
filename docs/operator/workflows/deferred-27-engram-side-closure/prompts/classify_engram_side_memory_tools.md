# Deferred 27 Engram-Side Memory Tools Closure

Classify deferred item 27: Engram-side RFC 0044 Phase 1 ingester, MCP server,
retrieval tools, and memory capabilities.

Read the context docs named by the workflow, especially TODO items 23 and 32,
roadmap section 5.7, the roadmap blocked table's item 32 row, SPEC's Corpus
Export And Augmentation Boundary, RFC 0041, RFC 0044, RFC 0057, and the
augmentation-boundary tests. Preserve the hard Striatum invariants:

- no `import engram` or `from engram` in Striatum source;
- no `memory.*` Striatum daemon capability;
- no retrieval call during Striatum state transitions;
- no hosted service, telemetry, transcript capture, external persistence, or
  runtime dependency on Engram.

Write a `synthesis` artifact at
`docs/operator/artifacts/deferred-27-engram-side-closure/closure/EXTERNAL_ENGRAM_WORK.md`
with valid `striatum.synthesis.v1` front matter and this exact byline:

`author: deferred27-engram-side-codex-gpt-5-001`

The artifact must record the evidence, classification, validation commands and
results, changed files, and any shared status updates to queue for a later
operator pass. Do not edit `docs/TODO.md`, `docs/ROADMAP.md`,
`docs/operator/BRIEF.md`, `docs/DECISION_LOG.md`, RFC files, source, Go, or
tests unless a Striatum boundary invariant is actually stale.
