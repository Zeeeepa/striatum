# TODO 59 Archive Verification Follow-Up Handoff
author: operator [self-declared: codex-operator]

## Summary

Implemented the bounded TODO 59 archive follow-up:

- New run archive manifests now advertise `archive_contract_version=2`,
  `verification_depth=deep_chain`, hybrid archive defaults with replay on by
  default, and `artifact_content_policy=metadata_only`.
- `archive verify` and `verify_run_archive()` now run local semantic replay by
  default. The explicit fast path is `--manifest-only` / `replay=False`.
- V2 archive verification rejects unsupported advertised defaults while legacy
  V1 archive manifests remain verifiable with default replay.
- Added read-only `archive inspect --bundle <dir>` over the same local verifier
  to report semantic checks and privacy-relevant archive metadata without
  daemon reachability or workflow-state mutation.

## Changed Files

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

## Validation

Ran:

```bash
.venv/bin/pytest tests/test_archive_verify.py
.venv/bin/pytest tests/test_corpus_manifest.py tests/test_corpus_verify.py tests/daemon_pg/handlers/reads/test_archive.py tests/daemon_pg/handlers/reads/test_corpus_export.py
.venv/bin/pytest tests/test_cli_daemon_rpc_route.py::test_archive_create_routes_to_daemon_rpc tests/test_list_commands.py
.venv/bin/ruff check src/striatum/archive src/striatum/cli tests/test_archive_verify.py
.venv/bin/python -m mypy src/striatum/archive src/striatum/cli
make lint
go test ./pkg/reads
git diff --check
```

Results:

- Archive suite: 35 passed.
- Corpus/read-handler focused suite: 54 passed.
- CLI route/list focused suite: 1 passed, 1 skipped.
- Scoped ruff and `make lint`: passed.
- Scoped mypy for changed archive/CLI modules: passed.
- Go reads package: passed.
- `git diff --check`: passed.

Also ran `make typecheck`; it failed on pre-existing typing errors outside
this archive slice (`auto_finalize.py`, `tests/_harness/mcp.py`) plus the
existing corpus manifest `int(object)` narrowing issue. I did not repair those
unrelated failures in this packet.

## Remaining Deferrals

- Incremental watermarking remains unimplemented.
- Augmentation-reference packet/fetch surfaces remain unimplemented and must
  stay optional.
- Optional live daemon audit-chain cross-check remains deferred.
- Artifact bytes are still not embedded in run archives; content hash checks
  remain opt-in through `--repo-root`.
