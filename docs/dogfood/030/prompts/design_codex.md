# Codex Design Prompt

Produce `docs/dogfood/030/design/codex/DESIGN.md`.

Design an implementation plan for RFC 0026 and RFC 0027 together. Your plan must cover:

- the domain model and vocabulary changes;
- workflow schema additions and validation behavior;
- CLI/API changes and any web/status/evidence/run-summary surfaces;
- SQLite migrations and compatibility with existing `.striatum/state.sqlite3` databases;
- artifact author-line validation, lane attestation, and operator-label handling;
- provenance-mode staging from advisory behavior through patch capture, hash-bound reviews, apply gates, receipts, and sealed enforcement;
- compatibility risks for existing examples and dogfood workflows;
- test plan, including adversarial tests;
- staging plan that can land useful parts without overclaiming sealed provenance.

Explicitly separate what RFC 0026 can ship independently from what RFC 0027 must defer until a real local authority boundary exists. If the work packet supplies an `author:` line, copy it exactly into the artifact title block.
