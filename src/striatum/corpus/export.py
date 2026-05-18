"""Corpus export orchestration entrypoint."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from striatum.corpus import git as git_helpers
from striatum.corpus.enumerator import enumerate_rows
from striatum.corpus.manifest import build_manifest, verify_manifest, write_manifest
from striatum.corpus.types import CorpusBundleResult, SUB_KINDS
from striatum.corpus.writer import verify_jsonl_files, write_jsonl_bundle
from striatum.errors import StriatumError


def export_corpus_bundle(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    since: str,
    out_text: str,
) -> dict[str, object]:
    """Export a deterministic RFC 0044 corpus bundle."""
    since_commit = git_helpers.resolve_commit(repo, since)
    out = _validate_out(repo, out_text)
    rows, missing_optional = enumerate_rows(conn, repo=repo, since_commit=since_commit)
    files = write_jsonl_bundle(out, rows)
    row_counts = verify_jsonl_files(out, files)
    manifest = build_manifest(
        repo=repo,
        since_ref=since,
        since_commit=since_commit,
        files=files,
        row_counts={kind: row_counts.get(kind, 0) for kind in SUB_KINDS},
        missing_optional_sources=missing_optional,
        state_authority={
            "substrate": "legacy_sqlite_fixture",
            "repository_schema_version": _repo_local_schema_version(conn),
        },
    )
    verify_manifest(manifest, row_counts)
    manifest_path, bundle_sha256 = write_manifest(out, manifest)
    result = CorpusBundleResult(
        since_ref=since,
        since_commit=since_commit,
        out=out,
        manifest_path=manifest_path,
        row_counts={kind: row_counts.get(kind, 0) for kind in SUB_KINDS},
        bundle_sha256=bundle_sha256,
    )
    return result.to_json(repo=repo)


def _repo_local_schema_version(conn: sqlite3.Connection) -> int:
    row = conn.execute("PRAGMA user_version").fetchone()
    return int(row[0]) if row is not None else 0


def _validate_out(repo: Path, out_text: str) -> Path:
    out = (repo / out_text).resolve() if not Path(out_text).is_absolute() else Path(out_text).resolve()
    repo_resolved = repo.resolve()
    try:
        rel = out.relative_to(repo_resolved)
    except ValueError as exc:
        raise StriatumError("--out must stay inside the repository", exit_code=8) from exc
    if rel.parts and rel.parts[0] == ".striatum":
        raise StriatumError("--out must not be under .striatum", exit_code=8)
    if out.exists() and not out.is_dir():
        raise StriatumError("--out must be a directory", exit_code=8)
    return out
