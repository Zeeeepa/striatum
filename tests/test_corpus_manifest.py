from __future__ import annotations

import json
import re
import subprocess
from pathlib import Path

import pytest

from striatum.corpus.manifest import build_manifest, default_corpus_id, generated_at_now, verify_manifest, write_manifest
from striatum.corpus.types import SUB_KINDS
from striatum.errors import StriatumError


def _git_repo(repo: Path) -> None:
    subprocess.run(["git", "init", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=repo, check=True)
    (repo / "README.md").write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", "README.md"], cwd=repo, check=True)
    subprocess.run(["git", "commit", "-m", "seed"], cwd=repo, check=True, capture_output=True)


def test_manifest_includes_schema_git_authority_and_counts(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    files: dict[str, dict[str, int | str]] = {"rfcs.jsonl": {"sha256": "abc", "rows": 1, "bytes": 10}}
    row_counts = {kind: 0 for kind in SUB_KINDS}
    row_counts["rfc"] = 1

    manifest = build_manifest(
        repo=tmp_path,
        since_ref="HEAD",
        since_commit="abc",
        files=files,
        row_counts=row_counts,
        missing_optional_sources=["docs/HARNESS_FRICTION_PATTERNS.md"],
        state_authority={
            "substrate": "postgresql",
            "repository_id": "repo_123",
            "repository_schema_version": 13,
            "daemon_schema_version": 8,
        },
        daemon_audit_included=True,
        generated_at="2026-05-13T00:00:00Z",
    )

    assert manifest["schema_version"] == "striatum.corpus_export.v1"
    assert manifest["corpus_contract_version"] == 2
    assert manifest["corpus_id"] == default_corpus_id(tmp_path)
    assert manifest["legacy_corpus_alias"] == "striatum"
    assert manifest["redaction_tier"] == "public"
    assert manifest["verification_depth"] == "deep_chain"
    assert manifest["incremental_export_watermark"] == {
        "strategy": "git_commit_range",
        "scope": "corpus_id",
        "corpus_id": default_corpus_id(tmp_path),
        "since_commit": "abc",
        "high_watermark": manifest["git_head"],
        "dirty_tree": False,
        "advanceable": True,
    }
    assert manifest["augmentation_policy"] == {
        "mode": "reference_only",
        "workflow_opt_in": True,
        "budget_per_packet_lines": 100,
        "required": False,
    }
    assert manifest["hybrid_archive_defaults"] == {
        "snapshot": True,
        "event_log": True,
        "verify_replay_by_default": True,
    }
    assert manifest["state_authority"] == {
        "substrate": "postgresql",
        "repository_id": "repo_123",
        "repository_schema_version": 13,
        "daemon_schema_version": 8,
    }
    assert manifest["daemon_audit_included"] is True
    assert manifest["row_counts"] == row_counts
    assert manifest["missing_optional_sources"] == ["docs/HARNESS_FRICTION_PATTERNS.md"]


def test_manifest_accepts_explicit_v2_identity_and_snapshot_hash(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    row_counts = {kind: 0 for kind in SUB_KINDS}

    manifest = build_manifest(
        repo=tmp_path,
        since_ref="HEAD",
        since_commit="abc",
        files={},
        row_counts=row_counts,
        missing_optional_sources=[],
        state_authority={"substrate": "postgresql"},
        corpus_slug="docs-corpus",
        redaction_tier="curated",
        augmentation_policy={"budget_per_packet_lines": 25},
        git_snapshot_hash="a" * 64,
        generated_at="2026-05-13T00:00:00Z",
    )

    assert str(manifest["corpus_id"]).startswith("docs-corpus:")
    assert manifest["incremental_export_watermark"] == {
        "strategy": "git_commit_range",
        "scope": "corpus_id",
        "corpus_id": manifest["corpus_id"],
        "since_commit": "abc",
        "high_watermark": manifest["git_head"],
        "dirty_tree": False,
        "advanceable": True,
    }
    assert manifest["redaction_tier"] == "curated"
    assert manifest["augmentation_policy"] == {
        "mode": "reference_only",
        "workflow_opt_in": True,
        "budget_per_packet_lines": 25,
        "required": False,
    }
    assert manifest["git_snapshot_hash"] == "a" * 64
    verify_manifest(manifest, row_counts)


def test_generated_at_is_utc_z_second_precision() -> None:
    assert re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", generated_at_now())


def test_manifest_verification_rejects_count_mismatch(tmp_path: Path) -> None:
    manifest: dict[str, object] = {"schema_version": "striatum.corpus_export.v1", "row_counts": {"rfc": 2}}
    row_counts = {kind: 0 for kind in SUB_KINDS}
    row_counts["rfc"] = 1
    with pytest.raises(StriatumError, match="row count mismatch"):
        verify_manifest(manifest, row_counts)


def test_v2_manifest_verification_requires_incremental_watermark(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    row_counts = {kind: 0 for kind in SUB_KINDS}
    manifest = build_manifest(
        repo=tmp_path,
        since_ref="HEAD",
        since_commit="abc",
        files={},
        row_counts=row_counts,
        missing_optional_sources=[],
        state_authority={"substrate": "postgresql"},
        generated_at="2026-05-13T00:00:00Z",
    )
    manifest.pop("incremental_export_watermark")

    with pytest.raises(StriatumError, match="incremental_export_watermark"):
        verify_manifest(manifest, row_counts)


def test_incremental_watermark_marks_dirty_exports_not_advanceable(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    (tmp_path / "README.md").write_text("seed\nuncommitted\n", encoding="utf-8")
    row_counts = {kind: 0 for kind in SUB_KINDS}

    manifest = build_manifest(
        repo=tmp_path,
        since_ref="HEAD",
        since_commit="abc",
        files={},
        row_counts=row_counts,
        missing_optional_sources=[],
        state_authority={"substrate": "postgresql"},
        generated_at="2026-05-13T00:00:00Z",
    )

    assert manifest["git_dirty"] is True
    watermark = manifest["incremental_export_watermark"]
    assert isinstance(watermark, dict)
    assert watermark["dirty_tree"] is True
    assert watermark["advanceable"] is False
    verify_manifest(manifest, row_counts)


def test_write_manifest_returns_canonical_sha(tmp_path: Path) -> None:
    path, digest = write_manifest(tmp_path, {"b": 2, "a": 1})
    assert path.name == "manifest.json"
    assert len(digest) == 64
    assert json.loads(path.read_text(encoding="utf-8"))["bundle_sha256"] == digest
    assert path.read_text(encoding="utf-8").endswith("\n")
