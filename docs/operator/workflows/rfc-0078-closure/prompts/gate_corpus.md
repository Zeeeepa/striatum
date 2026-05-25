# Gate A — Corpus redaction port + Python corpus retirement

You are the implementer for RFC 0078 Gate A. Read first:
`docs/operator/plans/rfc-0078-remaining-work.md` (Gate A),
`docs/rfcs/0078-go-only-runtime-and-python-removal.md`,
`docs/operator/artifacts/rfc-0078-python-test-migration/coverage-ledger/COVERAGE_LEDGER.md`
(corpus rows), and `go/pkg/reads/exports.go`.

## Decision (already made — do not re-litigate)

Corpus export/archive/historical-dogfood behavior is **already live in Go**
(`corpus.export`, `archive.create`, `corpus.migrate_historical_dogfood_file`,
`corpus.list/fetch_historical_dogfood*`). The **only load-bearing gap** is
redaction: `go/pkg/reads/exports.go:231` says redaction-tier compliance "stays
in the Python handler." Port that tier into Go, then **retire** the standalone
Python corpus modules as superseded.

## Steps

1. Read the Python redaction behavior in `src/striatum/corpus/redaction.py`
   and how `src/striatum/daemon_pg/handlers/reads/corpus_export.py` /
   `src/striatum/archive/writer.py` apply it (redaction tiers, depth,
   field-level rules).
2. Port the redaction-tier compliance into Go so `corpus.export`,
   `evidence.export`, and `archive.create` produce redaction-equivalent output
   to the Python path. Put the logic in `go/pkg/reads/` (extend `exports.go`
   and/or a new `redaction.go`). Remove the "stays in the Python handler"
   deferral comment once parity lands.
3. Add focused Go tests covering each redaction tier and the field-level rules.
4. Delete the superseded Python corpus surface: `src/striatum/corpus/**`
   (enumerator, manifest, writer, verify, git, types, migrate_historical_dogfoods,
   __init__) and `src/striatum/archive/*.py` IF nothing outside the Python CLI
   path still imports them. Do not delete Python files that Gate D still needs
   as test targets — only the corpus/archive runtime modules.

## Constraints

- Stay within `write_scope.allowed_paths`. Daemon-owned PostgreSQL stays the
  only live substrate; do not open Postgres directly.
- Do not reintroduce SQLite or Python runtime bridges.

## Validate

```bash
cd go && go test ./pkg/reads ./pkg/mutations ./pkg/blob
```

## Required artifact

Publish `docs/operator/artifacts/rfc-0078-closure/corpus/SUMMARY.md`
(`artifact_kind: synthesis`, schema `striatum.synthesis.v1`) listing: the Go
files changed, the redaction tiers ported, the Python files deleted, and the
exact validation command output. Use the byline supplied in your work packet.
