---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/DECISION_LOG.md", "docs/rfcs/0040-mcp-driven-dogfood-harness.md", "CHANGELOG.md"]
---

# RFC 0040 Packet Evidence Residuals
author: analyst-codex-gpt-5-001

## Classification

TODO 28 no longer describes missing RFC 0040 runtime implementation. Composite
failure observability has landed: failed dogfood helper paths preserve
`failed_step`, partial `composition_steps`, nested `specific_error`, rollback
event details, and MCP structured error code paths.

The remaining work is provenance and packet design:

- Preserve packet-derived byline evidence in read/export surfaces without
  trusting terminal output, transcripts, or scratch files.
- Keep operator-on-behalf provenance explicit when process evidence is
  missing.
- Do not restore `dogfood.publish_on_behalf` or
  `dogfood.surgical_recovery` as production daemon methods. D110 removed the
  SQLite-bound composites from the production contract; a PostgreSQL-native
  replacement requires a separate accepted design.

## Source Finding

The PostgreSQL read model had one bounded gap: artifact summaries carried the
recorded byline under compatibility keys (`author.author_line`), while
evidence/run-summary policy consumes the structured packet identity keys
`author.line` and `author.actual_author_line`. That meant packet-authored
byline evidence could be present in the artifact row but absent from redacted
evidence and run-summary rendering.

## Closure Target

Close the residual by projecting the recorded artifact byline into
`author.line` and `author.actual_author_line`, while preserving
`author.author_line` for existing list clients. Keep the evidence redaction
policy default-deny for unknown compatibility fields.
