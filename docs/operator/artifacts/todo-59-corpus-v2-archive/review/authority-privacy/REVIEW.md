---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["authority_privacy_boundary", "todo_59", "archive_verify", "documentation"]
---

# TODO 59 Authority And Privacy Review
author: operator [self-declared: codex-operator]

## Scope

Reviewed the TODO 59 map and build handoff, the archive writer/verifier and CLI
surfaces, the daemon/PostgreSQL archive handlers, the Go archive writer, focused
archive/corpus tests, and the current SPEC/TODO/RFC documentation. I did not
rerun the validation commands in this review packet; the handoff reports the
focused archive/corpus/CLI/Go checks passed, with unrelated `make typecheck`
failures outside this slice.

## Verdict

`accept_with_findings` - no blocking authority, privacy, or augmentation
regression found. The implementation keeps archive creation daemon/PostgreSQL
backed, keeps archive verification and inspection local/read-only, preserves
metadata-only archive semantics, and does not introduce Engram, `memory.*`,
hosted service, telemetry, transcript capture, external persistence, or
repo-local SQLite authority.

## Boundary Checks

Daemon/PostgreSQL authority is preserved. `archive.create` remains a registered
daemon RPC read method with single-repo scope in `contracts/daemon_methods.json`
and routes from the CLI through `src/striatum/cli/daemon_rpc_route.py`; the
CLI-local fallback refuses `archive create` instead of opening local state
(`src/striatum/cli/dispatch.py:641`). The Python handler is registered
`read_only=True` and queries `striatumd.*` tables under `repository_id` plus
`run_id` (`src/striatum/daemon_pg/handlers/reads/archive.py:17-136`). The Go
handler follows the same repository-scoped PostgreSQL path and rejects output
escapes and `.striatum/` targets (`go/pkg/reads/archive.go:96-211`).

Local-only verification is preserved. `archive verify` and `archive inspect`
are explicitly exempted from daemon-required routing only for local bundle
verification (`src/striatum/cli/dispatch.py:142-151`, `203-206`). The verifier
reads `manifest.json` and archive files from the supplied bundle, validates file
sets, hashes, counts, row shape, and bundle digest, then runs semantic replay by
default (`src/striatum/archive/verify.py:37-94`). `archive inspect` is a
projection over that verifier and returns semantic/privacy facts without any
daemon or workflow-state mutation (`src/striatum/archive/verify.py:97-136`).

The D126 archive defaults are now enforced for V2 archives. New archive
manifests advertise `archive_contract_version=2`, `verification_depth:
deep_chain`, hybrid defaults with `verify_replay_by_default=true`, and
`artifact_content_policy: metadata_only`
(`src/striatum/archive/writer.py:39-48`, `111-123`;
`go/pkg/reads/archive.go:19-28`, `303-327`). Verification rejects unsupported
V2 defaults while preserving legacy V1 manifest compatibility
(`src/striatum/archive/verify.py:139-171`;
`tests/test_archive_verify.py:439-477`).

Privacy and augmentation boundaries are not weakened. Archives still do not
embed artifact bytes; content hashes are checked only when the operator supplies
`--repo-root` (`src/striatum/archive/verify.py:649-680`). Inspection reports
artifact bytes, transcripts, scratch, and external persistence as absent
(`src/striatum/archive/verify.py:128-133`). Corpus V2 metadata stays
reference-only and optional with `required=false`
(`src/striatum/corpus/manifest.py:195-208`), and the active V2 boundary test
asserts no Engram imports, no `memory.*` strings in the corpus/CLI/daemon
surfaces, and no Engram package dependency (`tests/test_corpus_verify.py:185-201`).

## Findings

### F1. CLI reference still describes replay as opt-in

Severity: low

`docs/CLI_REFERENCE.md:501-513` still documents
`striatum archive verify --bundle <dir> [--replay] [--repo-root <path>]` and
says `--replay` adds semantic checks. Current parser/dispatch behavior makes
semantic replay the default, adds the explicit `--manifest-only` opt-out, and
adds `archive inspect` (`src/striatum/cli/parser.py:916-945`,
`src/striatum/cli/dispatch.py:624-640`). `docs/SPEC.md:744-758` and
`docs/TODO.md:1245-1255` already describe the current behavior.

Impact: operator documentation can still teach the old fast-path/default
relationship and omit the read-only inspection command. This is documentation
drift, not a source authority regression.

### F2. The SPEC names a skipped augmentation-boundary test as the pin

Severity: low

`docs/SPEC.md:726-727` says the augmentation-not-dependency invariants are
pinned by `tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`,
but that file is currently module-skipped as legacy SQLite eradicated
(`tests/test_cli_corpus_export.py:1-5`). A similar active V2 boundary check
exists in `tests/test_corpus_verify.py:185-201`, but the SPEC still points at
the inert guardrail.

Impact: the implementation still preserves augmentation-not-dependency for the
changed corpus/archive surfaces, but the documented canonical guardrail should
be updated or re-enabled in a follow-up.

## Acceptance Notes

The remaining TODO 59 deferrals in the handoff are correctly bounded:
incremental watermarking, future augmentation-reference fetch, optional live
daemon audit-chain cross-check, and artifact-byte embedding are not part of
this archive-default/deep-verification slice.
