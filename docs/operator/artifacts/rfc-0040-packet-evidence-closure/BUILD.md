---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/daemon_pg/handlers/reads/_read_model.py", "src/striatum/daemon_pg/handlers/reads/list_artifacts.py"]
---

# RFC 0040 Packet Evidence Build
author: implementer-codex-gpt-5-001

## Change

Closed the bounded PostgreSQL packet-evidence projection gap:

- `src/striatum/daemon_pg/handlers/reads/_read_model.py` now shapes artifact
  author data with `line` and `actual_author_line` populated from the
  recorded artifact byline.
- `src/striatum/daemon_pg/handlers/reads/list_artifacts.py` now passes the
  workflow snapshot into the shared artifact-summary helper so list output can
  include the declared lane display model.
- The existing `author_line` compatibility key remains for `list.artifacts`
  clients.
- Display model is derived from the workflow lane when the workflow body is
  available.

## Tests Added

- `tests/daemon_pg/handlers/reads/test_list_read_handlers.py` asserts
  PostgreSQL artifact listings expose `author.line`,
  `author.actual_author_line`, the compatibility `author.author_line`, and
  `author.display_model`.
- `tests/test_corpus_redaction.py` asserts evidence redaction preserves the
  packet-safe author identity shape while default-redacting the legacy
  compatibility key.

## Explicit Non-Changes

No production dogfood composite method was added. The D110 removal of
SQLite-bound `dogfood.publish_on_behalf` and `dogfood.surgical_recovery`
remains intact.

## Validation

- `PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate docs/operator/workflows/rfc-0040-packet-evidence-closure/workflow.json --json`
  - Result: valid.
- `.venv/bin/python -m pytest -q tests/test_corpus_redaction.py::test_evidence_redaction_preserves_packet_author_identity_shape tests/daemon_pg/handlers/reads/test_list_read_handlers.py::test_list_artifacts_filters_and_scopes_repository tests/test_mcp_mutation_capabilities.py::test_mcp_structured_error_uses_nested_rpc_error_codes`
  - Result: `3 passed`.
- `.venv/bin/python -m pytest -q tests/daemon_pg/handlers/reads/test_list_read_handlers.py tests/test_corpus_redaction.py tests/test_mcp_mutation_capabilities.py`
  - Result: `76 passed`.
- `git diff --check`
  - Result: passed.
