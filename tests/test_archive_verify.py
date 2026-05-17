from __future__ import annotations

import json
from importlib import import_module
from pathlib import Path

import pytest

from striatum.archive import ARCHIVE_JSONL_FILES, verify_run_archive, write_run_archive
from striatum.errors import StriatumError


def _archive(tmp_path: Path) -> Path:
    out = tmp_path / "archives" / "run_1"
    write_run_archive(
        out,
        repo_root=tmp_path,
        repository_id="repo_a",
        run_id="run_1",
        run={"repository_id": "repo_a", "run_id": "run_1", "state": "completed"},
        workflow_snapshot={
            "repository_id": "repo_a",
            "workflow_snapshot_id": "wf_1",
            "workflow_json": {"schema_version": "striatum.workflow.v1"},
        },
        rows={
            "artifacts": [
                {
                    "repository_id": "repo_a",
                    "artifact_id": "art_1",
                    "run_id": "run_1",
                    "repo_path": "docs/out.md",
                    "content_sha256": "abc123",
                }
            ],
            "events": [
                {
                    "repository_id": "repo_a",
                    "event_id": 1,
                    "run_id": "run_1",
                    "event_type": "run.completed",
                    "previous_hash": None,
                    "row_hash": "rowhash",
                }
            ],
        },
        generated_at="2026-05-17T00:00:00Z",
    )
    return out


def test_verify_run_archive_accepts_writer_output(tmp_path: Path) -> None:
    result = verify_run_archive(_archive(tmp_path))

    assert result["status"] == "verified"
    assert result["run_id"] == "run_1"
    assert result["archive_contract_version"] == 1
    row_counts = result["row_counts"]
    assert isinstance(row_counts, dict)
    assert row_counts["artifacts"] == 1
    assert row_counts["events"] == 1


def test_verify_run_archive_rejects_tampered_jsonl(tmp_path: Path) -> None:
    archive = _archive(tmp_path)
    events = archive / ARCHIVE_JSONL_FILES["events"]
    events.write_text(events.read_text(encoding="utf-8") + '{"extra":true}\n', encoding="utf-8")

    with pytest.raises(StriatumError, match="(hash|byte count) mismatch"):
        verify_run_archive(archive)


def test_verify_run_archive_rejects_manifest_row_count_mismatch(tmp_path: Path) -> None:
    archive = _archive(tmp_path)
    manifest_path = archive / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest["row_counts"]["events"] = 99
    manifest.pop("bundle_sha256", None)
    manifest_path.write_text(json.dumps(manifest), encoding="utf-8")

    with pytest.raises(StriatumError, match="row count mismatch"):
        verify_run_archive(archive)


def test_archive_verify_cli_is_local_read_only(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    archive = _archive(tmp_path)
    from striatum.cli.parser import build_parser

    dispatch_mod = import_module("striatum.cli.dispatch")

    def fail_daemon_required(*_args: object, **_kwargs: object) -> None:
        raise AssertionError("archive verify should not require daemon")

    monkeypatch.setattr(dispatch_mod, "enforce_daemon_required", fail_daemon_required)
    args = build_parser().parse_args(
        [
            "--repo",
            str(tmp_path),
            "archive",
            "verify",
            "--bundle",
            str(archive.relative_to(tmp_path)),
            "--json",
        ]
    )

    result = dispatch_mod.dispatch(args)

    assert isinstance(result, dict)
    assert result["status"] == "verified"
