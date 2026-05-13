author: implementer-unknown-model-001

# RFC 0044 V1 Striatum Corpus Export Handoff

Status: implemented
Date: 2026-05-13

## Shipped Scope

Implemented the Striatum-side RFC 0044 V1 corpus exporter:

- Added `striatum corpus export --since <ref> --out <dir> [--json]`.
- Added the new `src/striatum/corpus/` package with separate modules for types, git helpers, enumeration, redaction, writer, manifest, and export orchestration.
- Export writes the fixed nine JSONL files plus `manifest.json`, verifies row counts and SHA-256s, and returns the standard CLI JSON envelope.
- Output is refused outside the repository, under `.striatum/`, or when the target is a file.
- `--since` is resolved through `git rev-parse --verify <ref>^{commit}` before writing.
- No Engram imports, no `memory.*` Striatum capability additions, and the corpus package is lazily imported only by the `corpus export` dispatch branch.

## Files Touched

- `src/striatum/cli/parser.py`
- `src/striatum/cli/dispatch.py`
- `src/striatum/corpus/__init__.py`
- `src/striatum/corpus/types.py`
- `src/striatum/corpus/git.py`
- `src/striatum/corpus/enumerator.py`
- `src/striatum/corpus/redaction.py`
- `src/striatum/corpus/writer.py`
- `src/striatum/corpus/manifest.py`
- `src/striatum/corpus/export.py`
- `tests/test_corpus_enumerator.py`
- `tests/test_corpus_redaction.py`
- `tests/test_corpus_writer.py`
- `tests/test_corpus_manifest.py`
- `tests/test_cli_corpus_export.py`
- `tests/test_corpus_export_integration.py`
- `tests/test_web_ui.py`

## Verification

Passed:

- `python -m pytest tests/test_corpus_enumerator.py tests/test_corpus_redaction.py tests/test_corpus_writer.py tests/test_corpus_manifest.py tests/test_cli_corpus_export.py tests/test_corpus_export_integration.py -q` — 31 passed.
- `rg -n "^(import|from) engram" src/striatum/corpus src/striatum/cli src/striatum/daemon_rpc src/striatum/daemon_pg src/striatum/mcp.py src/striatum/service.py` — no matches.
- `rg -n "memory\\." src/striatum/corpus src/striatum/cli src/striatum/daemon_rpc src/striatum/daemon_pg src/striatum/mcp.py src/striatum/service.py` — no matches.
- `rg -n "FROM runs|FROM verdicts|FROM artifacts|FROM jobs|FROM sessions" src/striatum/corpus/enumerator.py` — no matches.
- `make lint` — passed.
- `make typecheck` — passed.

Full suite:

- `make test` ran to completion with 739 passed and 33 skipped, but failed one pre-existing documentation budget assertion: `tests/test_doc_links.py::test_decision_log_rows_under_word_budget` reports `docs/DECISION_LOG.md` row D094 at 439 words. `docs/DECISION_LOG.md` is outside this packet's write scope, so it was not edited.

## Deviations

- `tests/test_web_ui.py` was adjusted from `Traversable.read_text(errors=...)` to `read_bytes().decode(..., errors=...)` so `make typecheck` passes under the current `importlib.resources` typing surface. This is test-only compatibility plumbing.
- Live run summaries use `run_summary_snapshot(...)` and `render_run_summary_markdown(...)` as required, with a corpus-specific wrapper around `redact_evidence_payload(...)` because the evidence redactor is keyed to nested status/doctor payloads rather than the whole run-summary snapshot shape.
- Repo-local audit and live run-summary provenance use the sentinel path `<repo-local-state>` rather than `.striatum/state.sqlite3`, preserving the no-`.striatum/` export boundary while still identifying the source class.
