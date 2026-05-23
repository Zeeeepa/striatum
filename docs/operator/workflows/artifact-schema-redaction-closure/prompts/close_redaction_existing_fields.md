# Close Redaction Coverage For Existing Fields

Read the current evidence and corpus redaction implementation and tests:

- `docs/SPEC.md` evidence redaction contract.
- `src/striatum/evidence_presentation.py`.
- `src/striatum/corpus/redaction.py`.
- `tests/test_corpus_redaction.py`.
- `tests/daemon_pg/handlers/reads/test_evidence_export.py`.

Inspect existing artifact/evidence fields for concrete free-text leaks. If a
gap exists, apply the narrowest source/test change that closes it. Produce
`docs/operator/artifacts/artifact-schema-redaction-closure/redaction/REPORT.md`
with the change summary and focused test evidence.
