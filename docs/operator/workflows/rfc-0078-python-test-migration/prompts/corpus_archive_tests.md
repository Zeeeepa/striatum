# Corpus And Archive Test Migration

Read the coverage ledger, RFC 0078, corpus export/redaction/manifest/archive
pytest files, Go archive/read code, blob-related Go tests, and corpus/archive
runbooks.

Produce:
`docs/operator/artifacts/rfc-0078-python-test-migration/corpus-archive/CORPUS_ARCHIVE_TESTS.md`

Use this title block exactly:

```text
# Corpus And Archive Test Migration
author: operator [self-declared: corpus-archive-codex-gpt-5-001]
```

Port corpus and archive coverage to Go tests. Preserve local-first export,
redaction, manifest identity, archive verification, replay verification, blob
boundary behavior, and augmentation-not-dependency invariants. Do not add
external retrieval consumers or hosted persistence.

The artifact must list:

- corpus/archive behavior covered;
- pytest rows replaced, retired, or blocked;
- Go files added or changed;
- validation command evidence;
- remaining blockers before deleting Python corpus/archive tests.
