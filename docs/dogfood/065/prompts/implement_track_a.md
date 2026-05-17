# Implement Track A prompt - Go daemon core parity and schema

Produce `docs/dogfood/065/build/track_a/HANDOFF.md` as a handoff artifact.
Use a title block with `author: implementer-codex-codex-001`.

Track A owns Go daemon core parity and schema freshness only. Stay inside the
workflow write scope. Do not edit contracts, generated registry files owned by
Track C, Python service/client code, docs decisions, or parent-owned dogfood
files.

Implement per synthesis. Required work items:

1. Make the Go daemon support the current daemon PostgreSQL schema version, or
   fail closed with a precise rebuild/migration hint when stale.
2. Expose migration hash/source freshness through `--describe` and/or
   `daemon.hello` as locked by synthesis.
3. Close production `not_implemented` placeholders that belong to Go core
   parity, or hide them from production clients in coordination with Track C.
4. Preserve RFC 0030 envelope, request-id, version-skew, authorization,
   capability scope, and metadata-only audit-chain semantics.
5. Add a `CORE=go` conformance target/test path that cannot pass by skipping
   every Go assertion.
6. Add tests for schema-version parity, stale generated artifact refusal,
   capability denial parity, and audit-chain continuity.

Handoff must include:

- Files changed.
- Tests run and results.
- Remaining Go gaps by method or subsystem.
- Any Track C dependency for contract/generated-registry updates.
- Confirmation that `.striatum/`, README.md, OPERATOR_REPORT.md, workflow,
  prompts, and roles were not edited.
