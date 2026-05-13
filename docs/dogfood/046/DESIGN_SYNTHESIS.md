---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/046/design/codex/DESIGN.md", "docs/dogfood/046/design/claude_code/DESIGN.md", "docs/dogfood/046/design/gemini/DESIGN.md"]
---
author: designer-unknown-model-002

# RFC 0044 V1 Striatum Corpus Export Design Synthesis

Status: implementation plan
Date: 2026-05-13

## Scope

Implement only the Striatum-side RFC 0044 V1 corpus exporter:

```text
striatum corpus export --since <ref> --out <dir> [--json]
```

The command emits a deterministic, redacted JSONL bundle for Engram's later `ingest-striatum` command. It does not ingest into Engram, start `engram-mcp-stdio`, add Striatum `memory.*` capabilities, add a daemon RPC method, modify workflows, or affect any workflow state transition. The exporter is an augmentation surface, not a runtime dependency.

Where the designs disagree, this synthesis chooses the stricter Striatum-local shape: `src/striatum/corpus/`, required `--out`, no active-run flag in V1, and no Engram imports. Required `--out` keeps the V1 CLI explicit and avoids writing corpus material under `.striatum/`, which is not a durable artifact location.

## CLI Verb Wiring

Add a top-level `corpus` command family in `src/striatum/cli/parser.py` inside `build_parser()`, after the existing `evidence` or `decision` command group:

```python
corpus = sub.add_parser("corpus")
corpus_sub = corpus.add_subparsers(dest="corpus_command", required=True)
corpus_export = corpus_sub.add_parser("export")
corpus_export.add_argument("--since", required=True)
corpus_export.add_argument("--out", required=True)
corpus_export.add_argument("--json", action="store_true")
```

Do not add `--include-active-runs` in V1. RFC 0044's acceptance target is a replay-stable export over durable provenance and existing read surfaces; live-run inclusion makes ordering, redaction, and repeatability harder and can be added later with a separate flag.

In `src/striatum/cli/dispatch.py`, import the package entrypoint near the other CLI helpers:

```python
from striatum.corpus import export_corpus_bundle
```

Then add the dispatch branch inside the `with connect(repo) as conn:` block, near `evidence export` and `run summary`:

```python
if args.command == "corpus" and args.corpus_command == "export":
    return export_corpus_bundle(
        conn,
        repo=repo,
        since=args.since,
        out_text=args.out,
    )
```

The command uses the existing `dispatch.main()` result and error envelope. With `--json`, success prints `{"ok": true, "data": ...}` and `StriatumError` prints `{"ok": false, "error": {"message": "...", "code": <exit_code>}}`. Exit code semantics:

- `0`: bundle written and manifest verification passed.
- `1`: unexpected filesystem, SQLite, JSON, or git failure.
- `6`: generated row, manifest, redaction, duplicate-id, or bundle integrity invariant failed.
- `8`: invalid domain input, including unknown `--since`, `--out` under `.striatum/`, `--out` outside the repository, or a non-directory output target.

The result payload is:

```json
{
  "status": "exported",
  "since": {"ref": "v1.34.0", "commit": "<sha>"},
  "out": "docs/corpus-export/v1.34.0",
  "manifest_path": "docs/corpus-export/v1.34.0/manifest.json",
  "row_counts": {"rfc": 10, "decision_log_row": 4},
  "bundle_sha256": "<sha256-of-canonical-manifest>"
}
```

## Module Layout

Create `src/striatum/corpus/` with one public facade and five internal responsibilities:

```text
src/striatum/corpus/
  __init__.py
  export.py
  types.py
  git.py
  enumerator.py
  redaction.py
  writer.py
  manifest.py
```

`__init__.py` exports only `export_corpus_bundle` and `SCHEMA_VERSION`.

`types.py` defines `SCHEMA_VERSION = "striatum.corpus_export.v1"`, `ROW_SHAPE_VERSION = "striatum.corpus_row.v1"`, the closed `SUB_KINDS` tuple, and frozen dataclasses for `CorpusProvenance`, `CorpusRow`, and `CorpusBundleResult`.

`git.py` is a thin subprocess wrapper for `git rev-parse`, `git status --porcelain`, `git diff --name-only`, `git log`, `git ls-files`, per-file last-touching commit/date, and tracked blob SHA-256. Do not add GitPython.

`enumerator.py` owns all source-specific readers and returns unredacted candidate `CorpusRow` objects from a closed source set. It may call helpers in `src/striatum/cli/run_summary.py`; it must not perform broad repository crawling.

`redaction.py` validates paths, applies source denylist checks, reuses `striatum.cli.evidence.redact_evidence_payload` for structured run/audit payloads, and applies corpus-specific commit-message filters.

`writer.py` writes the nine JSONL files deterministically and atomically, validates row shapes, refuses duplicate `(sub_kind, external_id)` pairs, and returns per-file hashes/counts.

`manifest.py` builds and verifies `manifest.json`, computes the canonical manifest hash, and reads package/git/SQLite metadata.

`export.py` is the orchestration entrypoint `export_corpus_bundle(conn, repo, since, out_text)`.

## Enumeration Sources

Resolve `--since` with `git rev-parse --verify <ref>^{commit}` before writing anything. For file-backed sources, include rows whose source file is changed in `<since>..HEAD` or is dirty in the worktree. For commit rows, use `git log <since>..HEAD`. For optional sources that do not exist, emit zero rows and record the path in `manifest.missing_optional_sources`.

Use this exact file mapping:

| file | sub_kind |
|---|---|
| `rfcs.jsonl` | `rfc` |
| `decision_log_rows.jsonl` | `decision_log_row` |
| `operator_reports.jsonl` | `operator_report` |
| `run_summaries.jsonl` | `run_summary` |
| `audit_chain.jsonl` | `audit_chain_entry` |
| `changelog.jsonl` | `changelog_entry` |
| `ubiquitous_language.jsonl` | `ubiquitous_language_term` |
| `harness_friction_patterns.jsonl` | `harness_friction_pattern` |
| `commits.jsonl` | `commit` |

Per `sub_kind`:

- `rfc`: read `docs/rfcs/[0-9][0-9][0-9][0-9]-*.md`, split on ATX headings, derive `external_id` as `rfc:<####>#<heading-slug>`, and use the heading plus body as content.
- `decision_log_row`: parse the Markdown table under `## Decisions` in `docs/DECISION_LOG.md`; emit `decision:<D###>` with status, decision, reason, consequences, and revisit trigger normalized into paragraphs.
- `operator_report`: read `docs/dogfood/[0-9][0-9][0-9]/OPERATOR_REPORT.md`; split intervention/decision sections into stable file-order entries; emit `dogfood:<###>#intervention-<N>`. If no intervention headings exist, count non-empty paragraph/list blocks in order.
- `run_summary`: route through the existing run-summary surface. For committed `docs/dogfood/[0-9][0-9][0-9]/RUN_SUMMARY.md`, read the durable artifact because it was produced by `striatum run summary`. For live database-backed summaries needed by tests, call `striatum.cli.run_summary.run_summary_snapshot(...)` and `render_run_summary_markdown(...)`, the same helper path used by `run_summary_export(...)`; do not query `runs`, `jobs`, `artifacts`, `verdicts`, or `sessions` directly in the run-summary enumerator. Emit `run:<run-id>`.
- `audit_chain_entry`: best-effort optional metadata only. Read repo-local `events` rows only for structural event metadata and safe ids; read daemon audit-chain rows only when an existing daemon audit source is configured and reachable. Emit `audit:<store>:<row-id>` where `<store>` is `repo` or `daemon`; the extra namespace resolves the RFC table's ambiguity without changing the `audit:` family.
- `changelog_entry`: split `CHANGELOG.md` by `## vX.Y.Z` or `## [vX.Y.Z]`; emit `changelog:<vX.Y.Z>`.
- `ubiquitous_language_term`: parse the `## Core Terms` table in `docs/UBIQUITOUS_LANGUAGE.md`; emit `ulang:<slug>`.
- `harness_friction_pattern`: split `docs/HARNESS_FRICTION_PATTERNS.md` by pattern headings; emit `friction:<slug>`.
- `commit`: shell out to `git log <since>..HEAD` with a fixed format containing full SHA, author date, committer date, parent SHAs, subject, body, and changed path list. Emit `commit:<sha>`. Do not include patch text.

Use each source's last-touching commit timestamp for file-backed `observed_at`; use event/audit timestamp for `audit_chain_entry`; use committer date for `commit`. Normalize timestamps to UTC, second precision, with a `Z` suffix.

## Redaction Policy

The exporter is default-deny. It has a source denylist and a field policy.

The source denylist refuses or ignores:

- `.env`, `.env.*`, `*.pem`, `*.key`, `*.p12`, token files, and editor swap files.
- `.striatum/state.sqlite3`, WAL/SHM files, `.striatum/scratch/`, `.striatum/bin/`, and any `.striatum/` path.
- `transcripts/`, `terminal_output/`, `raw_model_output/`, `**/transcript*.txt`, `**/transcript*.md`, and `**/transcript*.log`.
- SQLite/PostgreSQL blobs, process stdout/stderr logs, supervisor logs, and patch text.
- Any Markdown outside the closed source list above.

Per-field/content rules:

- Curated committed Markdown sources (`rfc`, `decision_log_row`, `operator_report`, committed `run_summary`, `changelog_entry`, `ubiquitous_language_term`, `harness_friction_pattern`) pass through after source-path validation because they are durable provenance.
- Live run-summary snapshots pass through `redact_evidence_payload(...)` before rendering. Verdict rationales, blocker descriptions, and unknown fields therefore become `<redacted-free-text>`.
- Repo-local event/audit payloads are not emitted wholesale. Keep only structural ids/enums/timestamps/hashes: event id, event type, run id, job id, workflow job id, session id, artifact id, verdict id, blocker kind, state, created_at, request hash, response hash, row hash, previous hash, repository id. Drop or redact `description`, `rationale`, `summary`, `prompt`, `objective`, `body`, `error_message`, `original_envelope`, request/response bodies, and arbitrary `payload_json`.
- Daemon audit rows remain metadata-only, but the exporter still drops denial/free-text reason fields if present.
- Commit messages are included because they are intentional git provenance, but strip `Co-Authored-By: ...<...@...>` lines and standalone long token-like base64/hex strings. Changed paths are included; patch text is not.

## JSONL Shape And Ordering

Every JSONL row is locked to RFC 0044 section 3:

```json
{
  "source_kind": "striatum",
  "external_id": "rfc:0040#proposal",
  "sub_kind": "rfc",
  "content": "...",
  "provenance": {
    "path": "docs/rfcs/0040-mcp-driven-dogfood-harness.md",
    "sha256": "...",
    "commit": "..."
  },
  "observed_at": "2026-05-13T00:00:00Z"
}
```

The V1 closed `sub_kind` set is exactly:

```text
rfc
decision_log_row
operator_report
run_summary
audit_chain_entry
changelog_entry
ubiquitous_language_term
harness_friction_pattern
commit
```

Use RFC 0044's `external_id` table with the audit namespace refinement:

| sub_kind | external_id |
|---|---|
| `commit` | `commit:<sha>` |
| `decision_log_row` | `decision:<D###>` |
| `rfc` | `rfc:<####>#<heading-slug>` |
| `operator_report` | `dogfood:<###>#intervention-<N>` |
| `audit_chain_entry` | `audit:<store>:<row-id>` |
| `run_summary` | `run:<run-id>` |
| `changelog_entry` | `changelog:<vX.Y.Z>` |
| `ubiquitous_language_term` | `ulang:<slug>` |
| `harness_friction_pattern` | `friction:<slug>` |

For stable hashes, write files in the fixed order listed above. Within each file, sort rows lexicographically by `(external_id, provenance.path, observed_at)`. Emit compact UTF-8 JSON with `json.dumps(..., ensure_ascii=False, separators=(",", ":"), sort_keys=False)` and fixed insertion-order keys as shown in the RFC row shape. Each JSONL file ends with a final newline; zero-row files are still written.

## Manifest

Write `manifest.json` last and verify it against the JSONL files. `generated_at` is generated once per export using UTC ISO-8601 second precision with `Z`, for example `2026-05-13T12:55:00Z`.

Manifest fields:

```json
{
  "schema_version": "striatum.corpus_export.v1",
  "striatum_version": "1.34.0",
  "repo_root": "/home/halbritt/git/striatum",
  "git_head": "<sha>",
  "git_dirty": false,
  "since_ref": "v1.33.0",
  "since_commit": "<sha>",
  "generated_at": "2026-05-13T12:55:00Z",
  "schema": {
    "row_shape_version": "striatum.corpus_row.v1",
    "sub_kinds": ["rfc", "decision_log_row", "operator_report", "run_summary", "audit_chain_entry", "changelog_entry", "ubiquitous_language_term", "harness_friction_pattern", "commit"]
  },
  "source_kinds": ["striatum"],
  "row_counts": {
    "rfc": 0,
    "decision_log_row": 0,
    "operator_report": 0,
    "run_summary": 0,
    "audit_chain_entry": 0,
    "changelog_entry": 0,
    "ubiquitous_language_term": 0,
    "harness_friction_pattern": 0,
    "commit": 0
  },
  "files": {
    "rfcs.jsonl": {"sha256": "...", "rows": 0, "bytes": 0}
  },
  "repo_local_schema_version": 13,
  "missing_optional_sources": [],
  "daemon_audit_included": false
}
```

`striatum_version` comes from `importlib.metadata.version("striatum-orchestrator")`, falling back to `"unknown"`. `repo_root` is `Path(args.repo).resolve()`. `git_head`, `git_dirty`, `since_ref`, and `since_commit` come from `git.py`. `repo_local_schema_version` comes from `PRAGMA user_version` on the existing connection. `files[*].sha256` and `files[*].bytes` are computed from finalized bytes. `bundle_sha256` in the CLI result is the SHA-256 of canonical manifest bytes with `sort_keys=True`.

Verification must re-read each JSONL file, parse every row, assert `source_kind == "striatum"`, assert `sub_kind` is in the closed set, assert row counts match, assert file hashes match, and assert no duplicate `(sub_kind, external_id)`.

## Augmentation-Not-Dependency Checks

No code in `src/striatum/` may import Engram or register Striatum memory capabilities for this V1 exporter. Add regression checks that fail on:

```text
^(import|from) engram
memory\.
```

in `src/striatum/corpus/`, `src/striatum/cli/`, `src/striatum/daemon_rpc/`, `src/striatum/daemon_pg/`, `src/striatum/mcp.py`, and `src/striatum/service.py`, except for test/doc literals that explicitly describe the absence check. Also assert `pyproject.toml` gains no Engram dependency.

The exporter must succeed with `/home/halbritt/git/engram` missing and `engram-mcp-stdio` absent from `PATH`. Striatum workflow commands such as `ack`, `publish-artifact`, `complete`, `verdict`, recovery, and run prepare/start must not import or call anything from the corpus package.

## Tests

Add the following focused tests:

- `tests/test_corpus_enumerator.py`: RFC heading split, decision-log table parsing, operator-report intervention splitting, changelog version splitting, ubiquitous-language table parsing, friction-pattern splitting, commit enumeration from patched git output, and optional missing friction file behavior.
- `tests/test_corpus_redaction.py`: deny `.env` and transcript-looking paths, refuse `.striatum/` sources, redact verdict rationales and blocker descriptions from run-summary/audit payloads, drop arbitrary event `payload_json`, strip commit co-author emails and token-like lines, and verify curated Markdown passes through.
- `tests/test_corpus_writer.py`: fixed file order, fixed row key order, lexicographic row ordering, final newline, zero-row files, duplicate `(sub_kind, external_id)` refusal, atomic temp-file rename, and per-file SHA-256 calculation.
- `tests/test_corpus_manifest.py`: package-version fallback, git head/dirty/since fields, `PRAGMA user_version`, row counts, file hashes, `generated_at` UTC `Z` format, and manifest verification failure on tampered JSONL.
- `tests/test_cli_corpus_export.py`: parser accepts `corpus export --since <ref> --out <dir> --json`; invalid `--since` emits the standard JSON error envelope with code 8; output under `.striatum/` or outside the repo is refused; no Engram import/capability grep matches.
- `tests/test_corpus_export_integration.py`: create a real temporary Striatum run, export a run summary through the existing `run_summary_export` path, run `striatum corpus export --since <base> --out <dir> --json` twice, and assert byte-identical JSONL files plus equal manifests after removing `generated_at`.

Also add one implementation-regression assertion for the run-summary mandate:

```text
rg -n "FROM runs|FROM verdicts|FROM artifacts|FROM jobs|FROM sessions" src/striatum/corpus/enumerator.py
```

must return no matches in the run-summary code path. The exporter may use SQLite for audit metadata and `PRAGMA user_version`; it must not reconstruct run summaries from ad hoc table joins.

## Implementation Sequence

1. Add parser and dispatch branches plus a stub `export_corpus_bundle(...)` returning a manifest-shaped result.
2. Implement `types.py`, `writer.py`, and `manifest.py` first; lock row shape, file names, hashes, and verification.
3. Implement `git.py` and file-backed enumerators for commits, RFCs, decisions, operator reports, changelog, ubiquitous language, and friction patterns.
4. Implement run-summary enumeration through `run_summary_snapshot(...)` / `render_run_summary_markdown(...)` and committed `RUN_SUMMARY.md` artifacts.
5. Implement optional audit-chain metadata enumeration with zero-row behavior when daemon audit state is unavailable.
6. Add redaction checks and augmentation-not-dependency grep tests.
7. Add CLI and replay-stability integration coverage.

This plan intentionally keeps the exporter a closed-source-set batch command. The implementation should resist any drift toward a general Markdown crawler, transcript exporter, Engram client, or Striatum workflow dependency.
