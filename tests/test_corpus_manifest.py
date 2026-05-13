from __future__ import annotations

import re
import sqlite3
import subprocess
from pathlib import Path

import pytest

from striatum.corpus.manifest import build_manifest, generated_at_now, verify_manifest, write_manifest
from striatum.corpus.types import SUB_KINDS
from striatum.errors import StriatumError


def _git_repo(repo: Path) -> None:
    subprocess.run(["git", "init", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=repo, check=True)
    (repo / "README.md").write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", "README.md"], cwd=repo, check=True)
    subprocess.run(["git", "commit", "-m", "seed"], cwd=repo, check=True, capture_output=True)


def test_manifest_includes_schema_git_sqlite_and_counts(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    conn = sqlite3.connect(":memory:")
    conn.execute("PRAGMA user_version = 13")
    files: dict[str, dict[str, int | str]] = {"rfcs.jsonl": {"sha256": "abc", "rows": 1, "bytes": 10}}
    row_counts = {kind: 0 for kind in SUB_KINDS}
    row_counts["rfc"] = 1

    manifest = build_manifest(
        conn,
        repo=tmp_path,
        since_ref="HEAD",
        since_commit="abc",
        files=files,
        row_counts=row_counts,
        missing_optional_sources=["docs/HARNESS_FRICTION_PATTERNS.md"],
        generated_at="2026-05-13T00:00:00Z",
    )

    assert manifest["schema_version"] == "striatum.corpus_export.v1"
    assert manifest["repo_local_schema_version"] == 13
    assert manifest["row_counts"] == row_counts
    assert manifest["missing_optional_sources"] == ["docs/HARNESS_FRICTION_PATTERNS.md"]


def test_generated_at_is_utc_z_second_precision() -> None:
    assert re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", generated_at_now())


def test_manifest_verification_rejects_count_mismatch(tmp_path: Path) -> None:
    manifest: dict[str, object] = {"schema_version": "striatum.corpus_export.v1", "row_counts": {"rfc": 2}}
    row_counts = {kind: 0 for kind in SUB_KINDS}
    row_counts["rfc"] = 1
    with pytest.raises(StriatumError, match="row count mismatch"):
        verify_manifest(manifest, row_counts)


def test_write_manifest_returns_canonical_sha(tmp_path: Path) -> None:
    path, digest = write_manifest(tmp_path, {"b": 2, "a": 1})
    assert path.name == "manifest.json"
    assert len(digest) == 64
    assert path.read_text(encoding="utf-8").endswith("\n")
