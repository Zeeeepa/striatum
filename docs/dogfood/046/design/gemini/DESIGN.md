author: operator

# DESIGN: RFC 0044 V1 Striatum-side Corpus Export

This document specifies the Striatum-side implementation of RFC 0044 V1: the `striatum corpus export` command and the `src/striatum/corpus/` module. These components produce a redacted, content-stable JSONL bundle of the Striatum operational corpus for ingestion by Engram.

## CLI Verb Wiring

The `corpus export` command is registered as a sub-command of a new `corpus` top-level verb.

### Parser Registration

In `src/striatum/cli/parser.py`, we add the `corpus` parser and the `export` sub-command, following the pattern of `evidence export` (L470).

```python
    corpus = sub.add_parser("corpus")
    corpus_sub = corpus.add_subparsers(dest="corpus_command", required=True)
    corpus_export = corpus_sub.add_parser("export")
    corpus_export.add_argument(
        "--since",
        required=True,
        help="Git reference (tag, hash, or branch) to export corpus items since.",
    )
    corpus_export.add_argument(
        "--out",
        help="Target directory for the export bundle. Defaults to 'striatum-corpus-export-<timestamp>/'.",
    )
    corpus_export.add_argument("--json", action="store_true", help="Emit result envelope as JSON.")
```

### Dispatching

In `src/striatum/cli/dispatch.py`, we add the dispatch logic following the pattern of `evidence export` (L556) or `run summary` (L434).

```python
        if args.command == "corpus" and args.corpus_command == "export":
            from striatum.corpus.writer import corpus_export
            return corpus_export(
                conn,
                repo=repo,
                since=args.since,
                out_path_text=args.out,
            )
```

## Package Layout: `src/striatum/corpus/`

The implementation is split into specialized modules to separate concerns and ensure the augmentation-not-dependency invariant.

- `src/striatum/corpus/`:
  - `__init__.py`: Package exports.
  - `enumerator.py`: Logic for sourcing each `sub_kind` from git, disk, and SQLite.
  - `redaction.py`: Default-deny redaction policy and field-level filters.
  - `writer.py`: CLI entry point, directory orchestration, and JSONL emission.
  - `manifest.py`: `manifest.json` generation and SHA-256 integrity checks.

## Corpus Enumeration

The `enumerator` module sources items since the `--since` git reference.

| `sub_kind` | Source Logic |
|---|---|
| `rfc` | Walk `docs/rfcs/*.md`. Use `git log --since` to filter and find `observed_at`. |
| `decision_log_row` | Parse `docs/DECISION_LOG.md` Markdown table. Cite D### IDs. |
| `operator_report` | Walk `docs/dogfood/**/OPERATOR_REPORT.md`. |
| `run_summary` | Invoke `striatum run summary --json` for each run since `<ref>`. |
| `audit_chain_entry` | Read `events` table from SQLite for events since `<ref>`. |
| `changelog_entry` | Parse `CHANGELOG.md` version sections. |
| `ubiquitous_language_term` | Parse `docs/UBIQUITOUS_LANGUAGE.md` terms table. |
| `harness_friction_pattern` | Parse `docs/HARNESS_FRICTION_PATTERNS.md` table. |
| `commit` | `git log --pretty=format` for commits since `<ref>`. |

### Enumeration Citations

- **Git integration**: Reuses `striatum.cli.mutations.current_git_branch` and shells out to `git` via `subprocess.run` (mirroring `striatum.process_adapter`).
- **Run summaries**: Calls `striatum.cli.run_summary.run_summary_export` (which returns a `JsonObject`) but with a temporary path or capturing the result directly.
- **SQLite reads**: Uses `conn.execute(...)` (mirroring `striatum.cli.introspect`).

## Redaction Policy

The `redaction` module implements a **default-deny** policy, mirroring the `EVIDENCE_POLICY` pattern in `src/striatum/cli/evidence.py`.

### Explicit Denylist
- No `.env` files.
- No `.striatum/state.sqlite3` binary blobs.
- No `transcripts/` or `terminal_output/`.
- No `raw_model_output/` artifacts.
- No `payload_json` from `audit_chain_entry` unless fields are explicitly safe-listed.

### Per-Field Rules
- `rfc.content`: Safe (curated Markdown).
- `decision_log_row.Decision`: Safe (curated Markdown).
- `operator_report.content`: Safe (curated Markdown).
- `run_summary`: Redacted via `striatum.cli.evidence.redact_evidence_payload`.
- `audit_chain_entry.payload_json`: Redacted by default; only `event_type`, `run_id`, `job_id`, `created_at` are safe.

## JSONL Emission

Each line follows the RFC 0044 §3 schema, citing the `source_kind` enum precedent from Engram. Content is deterministically ordered by `observed_at` and `external_id` to ensure stable hashes.

### Row Shape (Citing RFC 0044 §3)

```json
{
  "source_kind": "striatum",
  "external_id": "<sub_kind>:<id>",
  "sub_kind": "...",
  "content": "...",
  "provenance": {
    "path": "repo-relative/path",
    "sha256": "...",
    "commit": "..."
  },
  "observed_at": "ISO-8601"
}
```

- `source_kind`: Set to `"striatum"`, following the Engram `source_kind` enum precedent (RFC 0044 §2).
- `external_id`: Stable identifier in the format `<sub_kind>:<id>`.
- `content`: Redacted free-text or curated Markdown.
- `provenance`: Git metadata for the source file.
- `observed_at`: The `git log` commit date or SQLite `created_at` timestamp.

## Manifest

`manifest.json` is generated last by `striatum.corpus.manifest`.

Fields (RFC 0044 §3):
- `striatum_version`: `striatum.__version__`.
- `repo_path`: `Path.cwd().resolve()`.
- `git_head`: Current commit hash.
- `dirty_tree`: Boolean from `git status --porcelain`.
- `since`: The `<ref>` argument.
- `file_hashes`: Mapping of `.jsonl` filename to SHA-256.
- `row_counts`: Mapping of `sub_kind` to line count.
- `generated_at`: Current UTC timestamp.

## Augmentation-Not-Dependency

- **No Engram Imports**: Verified by `grep -r "engram" src/striatum/corpus/`.
- **No RPC Reference**: `src/striatum/daemon.py` registry is not updated.
- **Decoupled Lifecycle**: `corpus export` is an offline batch command. It does not block the daemon or any workflow transition.

## Testing Strategy

- **Unit Tests**:
  - `tests/test_corpus_enumerator.py`: Mock git and filesystem to verify `since` filtering.
  - `tests/test_corpus_redaction.py`: Verify that sensitive fields in events and summaries are redacted.
  - `tests/test_corpus_manifest.py`: Verify hash stability and manifest schema.
- **Integration Test**:
  - `tests/test_corpus_export_integration.py`: Run `striatum corpus export` against the current Striatum repository. Verify that two consecutive runs produce identical JSONL content (modulo `generated_at` in manifest).

### Stability Check
A successful integration test MUST verify:
`sha256(export_1/rfcs.jsonl) == sha256(export_2/rfcs.jsonl)`
