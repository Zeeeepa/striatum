# Track B Build Handoff: RFC 0044 Draft

author: implementer-codex-gpt-5.5-001

## Summary

Drafted `docs/rfcs/0044-engram-phase-1-implementation-spec.md` as the Phase 1
implementation spec for Engram as an optional read-only memory augmentation
for Striatum operators.

The RFC follows the Track B synthesis: pull-mode ingestion, Striatum-owned
redacted JSONL export, Engram-owned `ingest-striatum`, standalone
`engram-mcp-stdio`, four read-only retrieval tools, Engram-local `memory.*`
capabilities, and a hard augmentation-not-dependency boundary.

## Files Written

- `docs/rfcs/0044-engram-phase-1-implementation-spec.md`
- `docs/dogfood/042/track_b/build/HANDOFF.md`

## Scope Notes

- Did not update `docs/rfcs/README.md`, `docs/TODO.md`, or `CHANGELOG.md`.
- Did not edit Engram. Engram concepts were cited from local Engram docs and
  from the accepted Track B synthesis.
- The RFC explicitly notes the numbering drift from RFC 0041: this RFC 0044 is
  Phase 1 read-only implementation, not Phase 3 write-side ingestion.

## Verification

- Required context was read locally; no network access was used.
- Sub-agent summaries covered Striatum RFC constraints, Engram vocabulary,
  Context, Problem/Goals, Proposal, Non-Goals, and Open Questions.
- Pending after this job: publish this handoff artifact and let downstream
  review jobs assess the RFC text.
