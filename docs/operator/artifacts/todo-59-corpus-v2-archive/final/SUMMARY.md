---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/todo-59-corpus-v2-archive/map/MAP.md", "docs/operator/artifacts/todo-59-corpus-v2-archive/build/HANDOFF.md", "docs/operator/artifacts/todo-59-corpus-v2-archive/review/authority-privacy/REVIEW.md", "docs/operator/plans/todo-59-corpus-v2-archive.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/rfcs/0066-replay-archive-corpus-v2-foundations.md"]
---

# TODO 59 Corpus V2 Archive Follow-Up Summary
author: operator [self-declared: codex-operator]

## Final State

The TODO 59 archive follow-up closed the bounded archive-default, deep
verification, and semantic-inspection slice described by D126 and RFC 0066.

- Archive-default enforcement: closed for the current slice. New run archives
  now advertise `archive_contract_version=2`, `verification_depth=deep_chain`,
  hybrid archive defaults with replay enabled by default, and
  `artifact_content_policy=metadata_only`. V2 verification rejects unsupported
  advertised defaults while legacy V1 archive manifests remain verifiable.
- Deep verification: closed for the current slice. `archive verify` and
  `verify_run_archive()` now run local semantic replay by default. The explicit
  fast path is `--manifest-only` / `replay=False`.
- Semantic inspection: closed for the current slice. `archive inspect --bundle`
  is a read-only local projection over the archive verifier and reports
  semantic checks plus privacy-relevant archive metadata without daemon
  reachability or workflow-state mutation.
- Incremental watermarking: deferred. It remains unimplemented in manifest
  construction and verification.
- Augmentation-reference fetch: deferred. Corpus V2 manifests preserve
  reference-only optional augmentation policy, but no workflow-packet schema or
  agent-side fetch handoff surface has landed.

## Implementation Evidence

The implementation handoff reports changes to:

- `src/striatum/archive/writer.py`
- `src/striatum/archive/verify.py`
- `src/striatum/archive/__init__.py`
- `src/striatum/cli/parser.py`
- `src/striatum/cli/dispatch.py`
- `go/pkg/reads/archive.go`
- `go/pkg/reads/archive_test.go`
- `tests/test_archive_verify.py`
- `docs/SPEC.md`
- `docs/TODO.md`
- `docs/rfcs/0066-replay-archive-corpus-v2-foundations.md`
- `docs/operator/artifacts/todo-59-corpus-v2-archive/build/HANDOFF.md`

The handoff records these validation results:

- `.venv/bin/pytest tests/test_archive_verify.py`: 35 passed.
- Focused corpus and daemon read-handler suite:
  `.venv/bin/pytest tests/test_corpus_manifest.py tests/test_corpus_verify.py tests/daemon_pg/handlers/reads/test_archive.py tests/daemon_pg/handlers/reads/test_corpus_export.py`:
  54 passed.
- Focused CLI route/list suite:
  `.venv/bin/pytest tests/test_cli_daemon_rpc_route.py::test_archive_create_routes_to_daemon_rpc tests/test_list_commands.py`:
  1 passed, 1 skipped.
- `.venv/bin/ruff check src/striatum/archive src/striatum/cli tests/test_archive_verify.py`:
  passed.
- `.venv/bin/python -m mypy src/striatum/archive src/striatum/cli`: passed.
- `make lint`: passed.
- `go test ./pkg/reads`: passed.
- `git diff --check`: passed.

The handoff also records `make typecheck` as failing on pre-existing typing
errors outside this archive slice: `auto_finalize.py`, `tests/_harness/mcp.py`,
and an existing corpus manifest `int(object)` narrowing issue. Those failures
were not repaired in this packet.

## Review Disposition

The authority/privacy review verdict is `accept_with_findings` with low
severity. It found no blocking authority, privacy, or augmentation regression.
The accepted implementation keeps archive creation daemon/PostgreSQL backed,
keeps archive verification and inspection local/read-only, preserves
metadata-only archive semantics, and does not introduce Engram, `memory.*`,
hosted service, telemetry, transcript capture, external persistence, or
repo-local SQLite authority.

The review findings were accepted as low-severity documentation drift, then
addressed in the operator follow-up while closing this packet:

- F1: `docs/CLI_REFERENCE.md` now documents replay-by-default verification,
  the `--manifest-only` fast path, and `archive inspect`.
- F2: `docs/SPEC.md` and `docs/TODO.md` now point at the active V2
  augmentation-boundary guardrail in `tests/test_corpus_verify.py`.

## Remaining Deferrals

- Incremental watermarking remains a TODO 59 follow-up owned by the corpus and
  archive contract surface.
- Augmentation-reference packet/fetch remains a TODO 59 follow-up owned by the
  workflow packet and agent handoff surface. It must stay optional; Striatum
  must continue to run when augmentation sources are missing, slow,
  unreachable, or unconfigured.
- Optional live daemon audit-chain cross-check remains deferred. Current replay
  verifies archived event chains offline and does not compare them against live
  daemon audit state.
- Artifact bytes remain outside the run archive bundle boundary. Archive
  content hash checks remain opt-in through `--repo-root` unless a later
  archive-contract decision changes the bundle shape.
- The low-severity documentation findings from the authority/privacy review
  were corrected during closure; no additional documentation-only blocker is
  carried by this slice.

## Authority Statement

This closure treats daemon/PostgreSQL state, repository files, and local bundle
verification as the relevant authority surfaces described by D126 and RFC 0066.
Terminal output, tmux panes, provider hooks, marker files, and `.striatum/`
scratch were not used as workflow authority.
