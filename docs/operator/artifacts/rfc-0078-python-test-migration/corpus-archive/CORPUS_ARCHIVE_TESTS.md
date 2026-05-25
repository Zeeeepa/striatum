---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Corpus And Archive Test Migration
author: operator [self-declared: corpus-archive-codex-gpt-5-001]

## Corpus/Archive Behavior Covered

- Archive output path guardrails.
- Evidence export path guardrails and Markdown rendering.
- Archive manifest version, contract version, verification depth, artifact content policy, defaults, and row counts.
- Corpus export contract header, limit handling, and current artifact column projection.
- Blob bucket naming/config/key behavior through existing Go blob tests.

## Rows Replaced, Retired, Or Blocked

- Covered: `tests/daemon_pg/handlers/reads/test_archive.py`, `tests/daemon_pg/handlers/reads/test_corpus_export.py`, and part of `tests/test_corpus_manifest.py`.
- Needs replacement: corpus enumerator, redaction, writer, verifier, and archive replay verifier tests.
- Needs replacement: migration 0010 artifact blob update invariants in `tests/daemon_pg/test_migration_0010_artifact_blob_update.py`.

## Files Changed

- `go/pkg/reads/corpus_migration_test.go`

## Command Evidence

- `cd go && go test ./pkg/reads` passed as part of the focused Go command.

## Remaining Blockers

- There is no tracked Go `corpus` writer/redaction/verifier package in this worker's scope.
- Archive verification is still manifest-shape coverage, not deep replay verification.
