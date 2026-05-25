# Go PostgreSQL Harness Migration

Read the coverage ledger, RFC 0078, Go daemon/PostgreSQL tests, and pytest
coverage for daemon PG lifecycle, daemon RPC, audit chains, repository
registration, workflow lifecycle, recovery, leases, blockers, and
cross-repository behavior.

Produce:
`docs/operator/artifacts/rfc-0078-python-test-migration/pg-harness/PG_HARNESS.md`

Use this title block exactly:

```text
# Go PostgreSQL Harness Migration
author: operator [self-declared: pg-harness-codex-gpt-5-001]
```

Implement or refine only the PostgreSQL-harness slice named by the ledger.
Prefer reusable Go helpers and Go integration tests. Do not create Python
fixtures, pytest wrappers, SQLite fallbacks, or transcript-based assertions.

The artifact must list:

- pytest rows replaced or retired;
- Go files added or changed;
- command evidence, usually `cd go && go test ./...` or a narrower package
  command plus reason;
- remaining PG harness blockers, if any.
