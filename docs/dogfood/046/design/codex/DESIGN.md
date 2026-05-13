# RFC 0044 V1 Striatum Corpus Export Design

author: designer-unknown-model-001
Date: 2026-05-13
Status: handoff

## Scope

This design covers only the Striatum-side exporter for RFC 0044 V1:

```text
striatum corpus export --since <ref> --out <path> [--json]
```

The command emits a deterministic, redacted JSONL bundle that Engram's future `ingest-striatum` command can consume. Engram ingestion, `engram-mcp-stdio`, retrieval tools, Striatum operator memory checks, skills, daemon RPC methods, and Engram schema migrations are out of scope.

The boundary is augmentation, not dependency. The exporter reads repository provenance and existing Striatum summary surfaces; no Striatum state transition imports Engram, calls Engram, waits for Engram, or adds `memory.*` capability vocabulary.

## Existing Anchors

The CLI wiring should mirror current argparse and dispatch style:

- [src/striatum/cli/parser.py](/home/halbritt/git/striatum/src/striatum/cli/parser.py:23) creates top-level command parsers with `sub = parser.add_subparsers(dest="command", required=True)`.
- [src/striatum/cli/parser.py](/home/halbritt/git/striatum/src/striatum/cli/parser.py:290) wires nested command families such as `run`, and [src/striatum/cli/parser.py](/home/halbritt/git/striatum/src/striatum/cli/parser.py:312) defines `run summary --run-id --path --json`.
- [src/striatum/cli/parser.py](/home/halbritt/git/striatum/src/striatum/cli/parser.py:508) wires `recovery stale-leases --json` as another small nested read command.
- [src/striatum/cli/dispatch.py](/home/halbritt/git/striatum/src/striatum/cli/dispatch.py:62) imports CLI command helpers near the top, and [src/striatum/cli/dispatch.py](/home/halbritt/git/striatum/src/striatum/cli/dispatch.py:434) dispatches `run summary` to `run_summary_export(...)`.
- [src/striatum/cli/dispatch.py](/home/halbritt/git/striatum/src/striatum/cli/dispatch.py:68) wraps `StriatumError` in the stable JSON error envelope `{"ok": false, "error": {"message": ..., "code": ...}}` whenever `--json` is present.
- [src/striatum/cli/evidence.py](/home/halbritt/git/striatum/src/striatum/cli/evidence.py:24) already establishes the default-deny redaction posture: fields not explicitly classified as safe are replaced with `<redacted-free-text>`.
- [src/striatum/cli/run_summary.py](/home/halbritt/git/striatum/src/striatum/cli/run_summary.py:41) exposes `run_summary_snapshot(...)`, and [src/striatum/cli/run_summary.py](/home/halbritt/git/striatum/src/striatum/cli/run_summary.py:23) exposes `run_summary_export(...)`.

The Engram ingestion precedent is:

- [/home/halbritt/git/engram/migrations/001_raw_evidence.sql](/home/halbritt/git/engram/migrations/001_raw_evidence.sql:4) defines `source_kind` as a PostgreSQL enum and raw tables with `source_kind`, `external_id`, `raw_payload`, and `content_text`.
- [/home/halbritt/git/engram/migrations/003_source_kind_claude.sql](/home/halbritt/git/engram/migrations/003_source_kind_claude.sql:1) and [/home/halbritt/git/engram/migrations/005_source_kind_gemini.sql](/home/halbritt/git/engram/migrations/005_source_kind_gemini.sql:1) add enum values with `ALTER TYPE source_kind ADD VALUE IF NOT EXISTS`.
- [/home/halbritt/git/engram/src/engram/gemini_export.py](/home/halbritt/git/engram/src/engram/gemini_export.py:323) inserts into `sources` and uses `ON CONFLICT (source_kind, external_id) DO NOTHING`; [/home/halbritt/git/engram/src/engram/gemini_export.py](/home/halbritt/git/engram/src/engram/gemini_export.py:346) then verifies existing `content_hash` / `raw_payload` for idempotent conflict detection.

The new Striatum exporter should therefore emit rows shaped for Engram's raw-source-plus-content pipeline without importing Engram or assuming Engram internals.

## CLI Wiring

Add a top-level `corpus` command family in [src/striatum/cli/parser.py](/home/halbritt/git/striatum/src/striatum/cli/parser.py:1):

```python
corpus = sub.add_parser("corpus")
corpus_sub = corpus.add_subparsers(dest="corpus_command", required=True)
export = corpus_sub.add_parser("export")
export.add_argument("--since", required=True)
export.add_argument("--out", required=True)
export.add_argument("--json", action="store_true")
```

Keep `--since` required for V1 so the operator is explicit about the corpus window. `--out` is required by this work packet even though RFC 0044 permits a default directory. The path should be resolved through the same repo-relative safety posture used by artifacts where possible, but unlike `publish-artifact`, `--out` may be a directory not declared in a workflow write scope because this is an operator read/export command. It must still refuse paths under `.striatum/` and paths outside the repository unless a later RFC accepts external export directories.

In [src/striatum/cli/dispatch.py](/home/halbritt/git/striatum/src/striatum/cli/dispatch.py:1):

- import `export_corpus_bundle` from `striatum.corpus`;
- add the dispatch branch after other read-like command families:

```python
if args.command == "corpus" and args.corpus_command == "export":
    return export_corpus_bundle(conn, repo=repo, since=args.since, out_text=args.out)
```

The normal `dispatch.main(...)` envelope then prints `{"ok": true, "data": ...}` for `--json` and prints the returned dict for non-JSON commands, matching the current behavior for dict results. Exporter failures should raise `StriatumError` subclasses so JSON errors stay consistent with [src/striatum/cli/dispatch.py](/home/halbritt/git/striatum/src/striatum/cli/dispatch.py:68).

Recommended exit codes:

- `0`: bundle written and manifest hashes verified.
- `1`: unexpected filesystem, SQLite, JSON, or git failure.
- `6`: invalid export content or redaction/schema invariant violation, mirroring artifact/front-matter validation refusal.
- `8`: invalid CLI/domain input such as unknown `--since` ref, output path under `.striatum/`, or unparseable generated row shape, matching workflow validation style.

The command is read/export shaped. It should not require a session, lease, run id, or daemon capability.

## Package Layout

Create `src/striatum/corpus/` rather than the RFC's draft `corpus_export` name so the CLI noun maps cleanly to the module namespace:

```text
src/striatum/corpus/
  __init__.py
  export.py
  enumerator.py
  redaction.py
  writer.py
  manifest.py
  git.py
  types.py
```

Responsibilities:

- `types.py`: `CorpusRow`, `CorpusProvenance`, `CorpusBundleResult`, `SUB_KINDS`, `SCHEMA_VERSION = "striatum.corpus_export.v1"`.
- `git.py`: wrappers for `git rev-parse`, `git diff --quiet`, `git log --format`, `git ls-files`, `git show`, and blob hashing. Keep this thin and test with monkeypatched subprocess.
- `enumerator.py`: source-specific iterators that return unredacted candidate rows from repository files and existing Striatum summary functions.
- `redaction.py`: default-deny row validator/redactor. It receives `CorpusRow` objects, not arbitrary JSON blobs.
- `writer.py`: deterministic JSONL writer using `json.dumps(..., sort_keys=True, separators=(",", ":"), ensure_ascii=False)` and atomic temp-file-to-final rename per file.
- `manifest.py`: builds `manifest.json`, computes per-file hashes, row counts, git/SQLite metadata, and verifies the written bundle.
- `export.py`: orchestration entrypoint `export_corpus_bundle(conn, repo, since, out_text)` used by CLI dispatch.

The public package surface in `__init__.py` should export only `export_corpus_bundle` and the schema version.

## JSONL Line Shape

Every line must match RFC 0044 section 3:

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

V1 closed `sub_kind` set:

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

Use content-stable `external_id` values exactly as RFC 0044 specifies:

| `sub_kind` | `external_id` |
|---|---|
| `commit` | `commit:<sha>` |
| `decision_log_row` | `decision:<D###>` |
| `rfc` | `rfc:<####>#<heading-slug>` |
| `operator_report` | `dogfood:<###>#intervention-<N>` |
| `audit_chain_entry` | `audit:<row-id>` |
| `run_summary` | `run:<run-id>` |
| `changelog_entry` | `changelog:<vX.Y.Z>` |
| `ubiquitous_language_term` | `ulang:<slug>` |
| `harness_friction_pattern` | `friction:<slug>` |

Ordering must be deterministic for replay-stable hashes:

1. Write files in fixed RFC order: `rfcs.jsonl`, `decision_log_rows.jsonl`, `operator_reports.jsonl`, `run_summaries.jsonl`, `audit_chain.jsonl`, `changelog.jsonl`, `ubiquitous_language.jsonl`, `harness_friction_patterns.jsonl`, `commits.jsonl`.
2. Within each file, sort by `external_id`, then `provenance.path`, then `observed_at`.
3. Normalize timestamps to UTC `Z`.
4. Avoid absolute paths inside per-row provenance except for manifest repository identity.

## Corpus Enumeration

The `--since <ref>` contract should be git-first. Resolve it with `git rev-parse --verify <ref>` and use `git diff --name-only <since>..HEAD` plus selected always-read files. Commits use `git log <since>..HEAD`. If `<ref>` is invalid, raise a validation error before writing.

### `rfc`

Source files: `docs/rfcs/[0-9][0-9][0-9][0-9]-*.md`.

Enumerator:

- include RFC files changed since `<ref>`;
- split content into heading sections using Markdown ATX headings;
- derive RFC number from filename;
- derive heading slug by lowercasing heading text and replacing non-alphanumeric runs with `-`;
- emit `rfc:<####>#<heading-slug>`;
- use `observed_at` from the file's latest commit timestamp when available, otherwise export `generated_at`.

The content should include the heading and section body. This preserves retrieval granularity without forcing Engram to know Striatum's RFC format.

### `decision_log_row`

Source file: `docs/DECISION_LOG.md`.

Enumerator:

- parse the decision table under `## Decisions`;
- emit one row per `D###`;
- external id `decision:<D###>`;
- content is a normalized paragraph with decision id, status, decision, reason, consequences, and revisit trigger;
- provenance path is `docs/DECISION_LOG.md`.

This is deliberately a structured Markdown-table parser, not a regex over the whole file. Use a small table parser that handles escaped pipes and trims backtick/Markdown wrappers. Rows without a `D###` id are ignored.

### `operator_report`

Source files: `docs/dogfood/[0-9][0-9][0-9]/OPERATOR_REPORT.md`.

Enumerator:

- include changed reports since `<ref>` and, if the current dogfood report exists, include it when dirty;
- split entries under intervention/decision sections into stable numbered rows;
- external id `dogfood:<###>#intervention-<N>`;
- content includes the local heading plus entry text.

If a report has no explicit intervention section, emit section-level rows with monotonically assigned `intervention-<N>` in file order and keep that assignment stable by counting only non-empty paragraphs/list blocks.

### `run_summary`

Source: existing Striatum summary surface only.

RFC 0044 mandates run summaries go through the existing `striatum run summary --json` interface rather than ad hoc free-text SQLite reads. In code, the exporter can satisfy the same requirement without forking a subprocess by calling the same helper the CLI dispatch uses:

- [src/striatum/cli/dispatch.py](/home/halbritt/git/striatum/src/striatum/cli/dispatch.py:434) routes `run summary` to `run_summary_export(...)`;
- [src/striatum/cli/run_summary.py](/home/halbritt/git/striatum/src/striatum/cli/run_summary.py:41) computes the structured snapshot;
- [src/striatum/cli/run_summary.py](/home/halbritt/git/striatum/src/striatum/cli/run_summary.py:169) renders durable Markdown.

Design choice: call `run_summary_snapshot(...)` and `render_run_summary_markdown(...)` inside `enumerator.py`, not raw SQLite tables. Do not call `run_summary_export(...)` because that helper writes a Markdown artifact and inserts a `run_summary.exported` event at [src/striatum/cli/run_summary.py](/home/halbritt/git/striatum/src/striatum/cli/run_summary.py:35). The corpus exporter must be read-only with respect to workflow state.

Which runs: select runs whose `created_at`, `started_at`, or `completed_at` is newer than the commit timestamp of `<since>`, plus runs referenced by changed dogfood artifacts. Emit `run:<run-id>`.

Redaction: pass the structured snapshot through the corpus redactor before rendering content so free-text rationale/blocker fields remain redacted.

### `audit_chain_entry`

V1 should export daemon audit metadata when available, not repo-local transcript or process output.

Sources:

- local daemon SQLite audit rows from [src/striatum/daemon.py](/home/halbritt/git/striatum/src/striatum/daemon.py:215) when the RFC 0028 registry database exists;
- daemon PostgreSQL audit rows appended by [src/striatum/daemon_rpc/request_log.py](/home/halbritt/git/striatum/src/striatum/daemon_rpc/request_log.py:78) when daemon V2 is configured.

Implementation should be best-effort and optional. If no daemon registry/DB is configured, emit zero `audit_chain_entry` rows and record that in the manifest. Do not make export depend on daemon availability.

Allowed fields: audit id, timestamp, command/method, decision/auth result, repository id, request hash, response hash, row hash, previous hash, segment id. Deny request/response bodies and params by default.

External id: `audit:<row-id>` for SQLite rows; for daemon PG use `audit:<audit_id>` after stringifying. If both stores are present, namespace in provenance with `audit_store: "sqlite"` or `"postgres"` and keep duplicate-id detection fail-closed.

### `changelog_entry`

Source file: `CHANGELOG.md`.

Enumerator:

- split by version headings such as `## v1.33.0` or `## [v1.33.0]`;
- emit `changelog:<vX.Y.Z>`;
- content includes the version heading and body.

### `ubiquitous_language_term`

Source file: `docs/UBIQUITOUS_LANGUAGE.md`.

Enumerator:

- parse the Markdown table under `## Core Terms`;
- emit one row per term as `ulang:<slug>`;
- content is `Term: <term>\nDefinition: <definition>`.

Use the existing product vocabulary source of truth; do not infer terms from code.

### `harness_friction_pattern`

Source file: `docs/HARNESS_FRICTION_PATTERNS.md`.

Enumerator:

- split on headings under the friction-pattern body;
- external id `friction:<slug>`;
- content includes heading and body.

If this file is missing in older checkouts, emit zero rows and note `missing_optional_sources` in the manifest.

### `commit`

Source: `git log <since>..HEAD`.

Enumerator:

- use a fixed format with sha, author date, subject, body, parent shas, and changed paths;
- external id `commit:<sha>`;
- content includes commit subject/body and changed path list;
- provenance commit is the same sha.

Do not include patch text in V1. Commit messages are intentional durable provenance; diffs can contain secrets or generated/transcript-like content.

## Redaction Policy

Use a corpus-specific default-deny policy inspired by [src/striatum/cli/evidence.py](/home/halbritt/git/striatum/src/striatum/cli/evidence.py:24). The corpus exporter handles richer document text than evidence export, so the redactor has two layers.

Layer 1: source denylist. The enumerator must never read or emit:

- `.env`, `.env.*`, private key files, token files, editor swap files;
- `.striatum/state.sqlite3`, WAL/SHM files, `.striatum/scratch/`, `.striatum/bin/`;
- transcript files, raw terminal output, stdout/stderr logs, supervisor output logs;
- raw model output and process adapter diagnostics outside curated artifacts;
- SQLite or PostgreSQL blobs;
- patch text from commits;
- arbitrary files under dogfood directories not in the closed source list.

Layer 2: field policy. Rows may emit only:

- `source_kind`: literal `"striatum"`;
- `sub_kind`: closed enum;
- `external_id`: deterministic id from the table above;
- `content`: curated Markdown/document text after source-specific extraction;
- `provenance`: `path`, `sha256`, `commit`, optional structured ids (`run_id`, `decision_id`, `rfc`, `audit_id`, `dogfood_id`, `git_head`);
- `observed_at`: UTC timestamp.

For run summaries and audit chain rows, structured live-state free text must be redacted:

- `verdicts.rationale`: redacted, matching [src/striatum/cli/evidence.py](/home/halbritt/git/striatum/src/striatum/cli/evidence.py:188);
- blocker `description`: redacted, matching [src/striatum/cli/evidence.py](/home/halbritt/git/striatum/src/striatum/cli/evidence.py:54);
- workflow job `title` or prompt/objective text from SQLite: redacted unless it is already part of a curated repository artifact;
- `payload_json` from blockers, events, process executions, command requests, and daemon request logs: dropped unless a sub-policy marks a specific metadata field safe.

Curated Markdown artifacts are allowed because RFC 0044's corpus is repository provenance, but the source set is closed. The implementation should not recursively ingest all Markdown.

## Manifest

Write `manifest.json` last, after JSONL files are closed and hashed. Shape:

```json
{
  "schema_version": "striatum.corpus_export.v1",
  "source_kind": "striatum",
  "striatum_version": "1.33.0",
  "repository": {
    "path": "/home/halbritt/git/striatum",
    "git_head": "<sha>",
    "dirty": true
  },
  "since": {
    "ref": "<input>",
    "commit": "<resolved-sha>"
  },
  "state": {
    "repo_sqlite_schema_version": 13
  },
  "files": [
    {"path": "rfcs.jsonl", "sha256": "...", "rows": 10},
    {"path": "decision_log_rows.jsonl", "sha256": "...", "rows": 4}
  ],
  "row_counts": {
    "rfc": 10,
    "decision_log_row": 4,
    "operator_report": 1,
    "run_summary": 1,
    "audit_chain_entry": 0,
    "changelog_entry": 2,
    "ubiquitous_language_term": 90,
    "harness_friction_pattern": 8,
    "commit": 12
  },
  "emitted_source_kinds": ["striatum"],
  "missing_optional_sources": [],
  "generated_at": "2026-05-13T12:00:00Z"
}
```

`striatum_version` should use the package version source already used by CLI metadata. If there is no stable helper, add a tiny helper in `manifest.py` that reads installed package metadata via `importlib.metadata.version("striatum-orchestrator")` and falls back to `"unknown"`.

`repo_sqlite_schema_version` should be read with `PRAGMA user_version` from the existing `conn`; do not inspect `.striatum/state.sqlite3` as a blob.

Verification after write:

- recompute every file SHA-256;
- assert file row counts match manifest row counts;
- assert all rows parse as JSON objects and all `source_kind` values are `"striatum"`;
- assert no duplicate `(sub_kind, external_id)`;
- return manifest path and hashes in the CLI result.

## Augmentation-Not-Dependency Enforcement

Implementation acceptance should include:

- `rg -n "engram" src/striatum/cli` returns no matches except comments/tests explicitly documenting the absence check. Current checkout returns no matches.
- `rg -n "memory\\.|engram" src/striatum/daemon_rpc src/striatum/daemon_pg src/striatum/mcp.py src/striatum/service.py` returns no runtime method/capability registration.
- The new `src/striatum/corpus/` package imports only Striatum and standard-library modules.
- No Engram package appears in `pyproject.toml` dependencies.
- No daemon RPC registry entry is added in [src/striatum/daemon_rpc/registry.py](/home/halbritt/git/striatum/src/striatum/daemon_rpc/registry.py:1).

The exporter should be callable with Engram absent from `PATH` and with `/home/halbritt/git/engram` missing. Engram schema citations inform row shape only; they are not runtime dependencies.

## Tests

Add focused tests, avoiding broad fixture churn:

- `tests/test_corpus_redaction.py`: default-deny unknown fields; explicit redaction of verdict rationales, blocker descriptions, event payload prose, `.striatum/` paths, `.env` paths, transcript-looking paths, and terminal-output filenames.
- `tests/test_corpus_writer.py`: JSONL line ordering, canonical `json.dumps` settings, duplicate `(sub_kind, external_id)` refusal, atomic write behavior, and SHA-256 manifest verification.
- `tests/test_corpus_manifest.py`: manifest contains schema version, package version fallback, git head, dirty flag, resolved since ref, SQLite `PRAGMA user_version`, row counts, per-file hashes, and generated timestamp.
- `tests/test_corpus_enumerator.py`: table parsing for `DECISION_LOG.md` and `UBIQUITOUS_LANGUAGE.md`; RFC heading split; changelog version split; commit enumeration with patched git output; optional missing `HARNESS_FRICTION_PATTERNS.md`.
- `tests/test_cli_corpus_export.py`: parser accepts `corpus export --since <ref> --out <dir> --json`; invalid ref emits the standard JSON error envelope; output under `.striatum/` is refused.
- Integration test: build a small real Striatum run in a temp repo, call existing run lifecycle commands, export a run summary through `run_summary_snapshot(...)`/renderer path, run `striatum corpus export --since <base> --out <dir> --json` twice, and assert bundle file hashes and `manifest.json` row counts are replay-stable.

For a recent-dogfood replay test in this repository, use a deterministic `--since` ref from the dogfood branch and assert the same bundle hashes on two consecutive exports while allowing `generated_at` to be normalized or excluded from the stability hash. The acceptance target is stable JSONL content, not a stable wall-clock manifest timestamp.

## Implementation Sequence

1. Add parser/dispatch skeleton and a stub `export_corpus_bundle(...)` returning a dry manifest-shaped result.
2. Add `types.py`, `writer.py`, and `manifest.py` with tests first; these define the bundle contract.
3. Add `git.py` and source enumerators for commits, RFCs, decision log, changelog, ubiquitous language, friction patterns, and operator reports.
4. Add run-summary enumeration using `run_summary_snapshot(...)` plus `render_run_summary_markdown(...)`, with redacted structured snapshot input.
5. Add optional audit-chain enumeration with zero-row behavior when daemon audit stores are unavailable.
6. Add integration replay-stability coverage.
7. Run `rg -n "engram" src/striatum/cli` and daemon capability checks as explicit acceptance evidence.

The highest-risk point is accidentally converting this from a curated export into a broad repository crawler. Keep the source set closed, keep redaction default-deny, and keep Engram absent from Striatum runtime imports.
