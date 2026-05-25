---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Gate A — Corpus redaction port and Python corpus retirement

author: implementer-codex-gpt-5-001

## Outcome

Go now owns the redaction rules for the active corpus/evidence/archive export
path. The old `go/pkg/reads/exports.go` deferral that left redaction-tier
compliance in Python is removed, and the standalone Python corpus/archive
runtime modules are retired.

## Go files changed

- `go/pkg/reads/redaction.go` — added redaction-tier validation, corpus source
  path denial, token/co-author commit message redaction, event metadata
  redaction, evidence default-deny field redaction, run-summary redaction, and
  archive row redaction.
- `go/pkg/reads/redaction_test.go` — added focused coverage for each redaction
  tier and the field-level redaction rules.
- `go/pkg/reads/exports.go` — wired evidence export through default-deny
  redaction, upgraded corpus export to the active tier-aware projection, and
  removed the Python-handler deferral comment.
- `go/pkg/reads/archive.go` — wired archive rows through the redaction layer
  before writing JSON/JSONL archive files.
- `go/pkg/reads/corpus_migration_test.go` — updated the corpus export contract
  assertion for the tier-aware Go response.

## Redaction tiers ported

- `public`
- `curated`
- `internal`

The default remains `public`. Unknown tiers fail schema validation.

## Field-level rules ported

- Corpus source paths deny `.env*`, private key/database/swap suffixes,
  `.striatum`, transcript directories/files, terminal output, and raw model
  output.
- Event/audit payloads retain only structural metadata such as ids, state,
  timestamps, hashes, and repository identity.
- Evidence payloads are default-deny: safe scalar fields are preserved,
  prose-bearing fields are replaced with `<redacted-free-text>`, and nested
  maps/lists in scalar-safe fields are redacted to block synthetic prompt
  injection.
- Run summaries keep renderer shape while redacting blocker/verdict/session
  prose and clearing live duration for open runs.
- Archive rows redact known prose fields in sessions, jobs, verdicts,
  blockers, events, queue messages, work packets, command requests, and
  supervisor/process metadata.
- Commit messages drop co-author email lines, drop standalone long token-like
  lines, and replace embedded long token-like strings with `<redacted-token>`.

## Python files deleted

- `src/striatum/archive/__init__.py`
- `src/striatum/archive/verify.py`
- `src/striatum/archive/writer.py`
- `src/striatum/corpus/__init__.py`
- `src/striatum/corpus/enumerator.py`
- `src/striatum/corpus/git.py`
- `src/striatum/corpus/manifest.py`
- `src/striatum/corpus/migrate_historical_dogfoods.py`
- `src/striatum/corpus/redaction.py`
- `src/striatum/corpus/types.py`
- `src/striatum/corpus/verify.py`
- `src/striatum/corpus/writer.py`

## Validation

Command:

```bash
cd go && go test ./pkg/reads ./pkg/mutations ./pkg/blob
```

Output:

```text
ok  	github.com/halbritt/striatum/go/pkg/reads	(cached)
ok  	github.com/halbritt/striatum/go/pkg/mutations	0.164s
ok  	github.com/halbritt/striatum/go/pkg/blob	(cached)
```
