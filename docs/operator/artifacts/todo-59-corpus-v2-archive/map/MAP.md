---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/DECISION_LOG.md", "docs/SPEC.md", "docs/rfcs/0057-corpus-contract-v2.md", "docs/rfcs/0066-replay-archive-corpus-v2-foundations.md", "docs/operator/plans/todo-59-corpus-v2-archive.md"]
---

# TODO 59 Corpus V2 Archive Map
author: operator [self-declared: codex-operator]

## Completed

- Corpus verification is local and daemon-independent. `verify_corpus_bundle`
  accepts implied-V1 bundles, rejects future contract versions, checks declared
  bytes, JSONL hashes, row counts, and `bundle_sha256`, and returns V2 identity
  fields when present.
- New corpus manifests now default to `corpus_contract_version=2` with
  `corpus_id` as `slug:sha256`, `legacy_corpus_alias`, `redaction_tier`,
  `augmentation_policy` using `reference_only` / workflow opt-in /
  `required=false`, `verification_depth=deep_chain`, hybrid archive metadata,
  `state_authority`, and optional `git_snapshot_hash`.
- The corpus V2 validation surface covers the D126 augmentation boundary:
  source scans assert no Engram imports, no `memory.*` capability strings in
  the corpus/CLI/daemon surfaces, and no Engram dependency in `pyproject.toml`.
- Run archive creation is PostgreSQL/daemon backed and repository scoped.
  `archive.create` is registered as read-only, refuses output escapes, reads
  run/workflow rows plus run-scoped sessions, jobs, queue messages, leases,
  work packets, artifacts, verdicts, blockers, command requests, process
  executions, job worktrees, supervisors, pointers, and events, then writes a
  local self-verifying directory.
- Run archive verification is local and read-only. Plain verify checks manifest
  schema, file set, paths, hashes, byte counts, JSON/JSONL shape, row counts,
  and archive `bundle_sha256` without requiring daemon state.
- Archive semantic replay exists behind `--replay` or implicit `repo_root`.
  It checks run/repository consistency, archived-row references, duplicate and
  missing row-family ids, supervisor pointer integrity, event ordering,
  event-row hash recomputation, event-chain continuity, and optional artifact
  content hashes when `--repo-root` is supplied.

## Remaining

- Replay is not yet the archive verification default. `archive verify` still
  exposes replay as an opt-in CLI flag, while D126 and the V2 corpus manifest
  metadata say `verify_replay_by_default=true`.
- Archive default enforcement is incomplete. Corpus manifests advertise hybrid
  archive defaults, but the archive writer manifest remains
  `striatum.run_archive.v1` / `archive_contract_version=1` and does not carry
  or enforce a D126 archive-default contract.
- Incremental watermarking is still absent from manifest construction and
  verification. RFC 0057's watermark decision surface remains unimplemented.
- Read-only semantic inspection is not yet a named operator surface beyond
  replay verification. There is no archive/corpus describe or inspect command
  that emits structured semantic findings without creating or mutating a
  bundle.
- Optional augmentation references are represented as manifest policy only.
  No workflow packet augmentation-reference schema or agent-side fetch
  handoff surface has landed.
- Optional daemon audit-chain cross-check remains unimplemented. Current replay
  verifies archived event chains offline but does not compare them against a
  live daemon audit chain.

## Deferred/Out-of-Scope

- Comparative replay remains out of scope per D126; no next slice should add
  replay against hosted state, external stores, or another repository's live
  database.
- Hosted services, telemetry, transcript capture, external persistence,
  runtime retrieval dependencies, Engram imports, and `memory.*` daemon
  capabilities remain out of scope.
- Artifact byte embedding in run archives is out of the current V1 archive
  shape. Content verification stays optional via `--repo-root` unless a later
  archive-contract decision changes the bundle boundary.
- SQLite authority is out of scope. Corpus and archive live-state reads must
  continue through daemon/PostgreSQL or local bundle files; `.striatum/` stays
  operational scratch only.
- Engram-side ingestion, retrieval, MCP, embedding, and scoring behavior remain
  external to Striatum.

## Recommended Next Slice

Land the smallest archive-default/deep-verification slice:

1. Make archive verification replay by default while preserving an explicit
   opt-out only if the product wants a fast manifest-only mode.
2. Add archive manifest fields that align with D126's hybrid defaults and
   `deep_chain` verification semantics, then have verification reject archives
   that advertise unsupported defaults.
3. Extend tests so plain `archive verify` exercises semantic replay failures
   without `--replay`, while manifest-only behavior is covered only by the
   explicit opt-out path if one exists.

Do not combine this with watermarking or augmentation fetch. Those are separate
contract surfaces and can follow once archive defaults match D126.

## Validation Targets

- `pytest tests/test_archive_verify.py`
- `pytest tests/test_corpus_manifest.py`
- `pytest tests/test_corpus_verify.py`
- `pytest tests/daemon_pg/handlers/reads/test_archive.py`
- `pytest tests/daemon_pg/handlers/reads/test_corpus_export.py`
- After source changes, run the nearest full gate: `make test`.
