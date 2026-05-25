# Coverage Ledger Refinement

Read RFC 0078, the RFC 0078 cutover ledger, the existing test-migration
handoff, current pytest files under `tests/`, Go tests under `go/`, frontend
tests, smoke scripts, and CI/release scripts.

Produce:
`docs/operator/artifacts/rfc-0078-python-test-migration/coverage-ledger/COVERAGE_LEDGER.md`

Use this title block exactly:

```text
# RFC 0078 Python Test Migration Coverage Ledger
author: operator [self-declared: coverage-ledger-codex-gpt-5-001]
```

For every active pytest file or pytest-only behavior class, record:

- source pytest file or behavior class;
- product behavior protected;
- current Go, shell, or browser replacement if it exists;
- required replacement if it does not exist;
- migration owner slice: `pg_harness`, `cli_tests`, `web_tests`,
  `workflow_artifact_tests`, `corpus_archive_tests`, `packaging_smoke`, or
  `final_deletion_readiness`;
- status: `covered`, `needs_replacement`, `retire`, `historical_exception`,
  or `blocked`;
- exact deletion gate and validation command.

Do not edit product code in this job. The output is the gate for all parallel
migration jobs.
