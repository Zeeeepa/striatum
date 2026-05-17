from __future__ import annotations

import hashlib
import json
from importlib import import_module
from pathlib import Path

import pytest

from striatum.archive import (
    ARCHIVE_JSON_FILES,
    ARCHIVE_JSONL_FILES,
    verify_run_archive,
    write_run_archive,
)
from striatum.errors import StriatumError
from striatum.primitives import json_dumps


def _archive(tmp_path: Path) -> Path:
    out = tmp_path / "archives" / "run_1"
    completed_event = {
        "repository_id": "repo_a",
        "event_id": 1,
        "run_id": "run_1",
        "event_type": "run.completed",
        "payload_json": {},
        "created_at": "2026-05-17T00:00:00Z",
        "previous_hash": None,
    }
    completed_event["row_hash"] = _event_hash(completed_event, previous_hash=None)
    write_run_archive(
        out,
        repo_root=tmp_path,
        repository_id="repo_a",
        run_id="run_1",
        run={
            "repository_id": "repo_a",
            "run_id": "run_1",
            "workflow_snapshot_id": "wf_1",
            "repo_root": str(tmp_path),
            "state": "completed",
        },
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
            "events": [completed_event],
        },
        generated_at="2026-05-17T00:00:00Z",
    )
    return out


def _event_hash(row: dict[str, object], *, previous_hash: str | None) -> str:
    payload_json = row.get("payload_json")
    event_payload = payload_json if isinstance(payload_json, dict) else {}
    payload = {
        "previous_hash": previous_hash,
        "repository_id": row.get("repository_id"),
        "event_id": row.get("event_id"),
        "run_id": row.get("run_id"),
        "event_type": row.get("event_type"),
        "actor_session_id": row.get("actor_session_id"),
        "job_id": row.get("job_id"),
        "message_id": row.get("message_id"),
        "artifact_id": row.get("artifact_id"),
        "lease_id": row.get("lease_id"),
        "payload_json": {
            str(key): item
            for key, item in event_payload.items()
            if key != "_event_chain"
        },
        "created_at": str(row.get("created_at")),
    }
    return hashlib.sha256(json_dumps(payload).encode("utf-8")).hexdigest()


def _read_jsonl(archive: Path, kind: str) -> list[dict[str, object]]:
    rows: list[dict[str, object]] = []
    for line in (archive / ARCHIVE_JSONL_FILES[kind]).read_text(encoding="utf-8").splitlines():
        if line:
            loaded = json.loads(line)
            assert isinstance(loaded, dict)
            rows.append(loaded)
    return rows


def _write_jsonl(archive: Path, kind: str, rows: list[dict[str, object]]) -> None:
    body = "\n".join(
        json.dumps(row, ensure_ascii=False, separators=(",", ":"), sort_keys=True)
        for row in rows
    )
    (archive / ARCHIVE_JSONL_FILES[kind]).write_text(body + "\n", encoding="utf-8")
    _refresh_manifest(archive)


def _refresh_manifest(archive: Path) -> None:
    manifest_path = archive / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    assert isinstance(manifest, dict)
    for _kind, filename in ARCHIVE_JSON_FILES.items():
        data = (archive / filename).read_bytes()
        manifest["files"][filename] = {
            "sha256": hashlib.sha256(data).hexdigest(),
            "rows": 1,
            "bytes": len(data),
        }
    for kind, filename in ARCHIVE_JSONL_FILES.items():
        data = (archive / filename).read_bytes()
        rows = sum(1 for line in data.decode("utf-8").splitlines() if line)
        manifest["files"][filename] = {
            "sha256": hashlib.sha256(data).hexdigest(),
            "rows": rows,
            "bytes": len(data),
        }
        manifest["row_counts"][kind] = rows
    manifest.pop("bundle_sha256", None)
    manifest["bundle_sha256"] = hashlib.sha256(
        json.dumps(
            manifest,
            ensure_ascii=False,
            separators=(",", ":"),
            sort_keys=True,
        ).encode("utf-8")
    ).hexdigest()
    manifest_path.write_text(json.dumps(manifest, sort_keys=True), encoding="utf-8")


def test_verify_run_archive_accepts_writer_output(tmp_path: Path) -> None:
    result = verify_run_archive(_archive(tmp_path))

    assert result["status"] == "verified"
    assert result["run_id"] == "run_1"
    assert result["archive_contract_version"] == 1
    row_counts = result["row_counts"]
    assert isinstance(row_counts, dict)
    assert row_counts["artifacts"] == 1
    assert row_counts["events"] == 1


def test_verify_run_archive_replay_accepts_writer_output(tmp_path: Path) -> None:
    result = verify_run_archive(_archive(tmp_path), replay=True)

    replay = result["replay"]
    assert isinstance(replay, dict)
    assert replay["status"] == "verified"
    assert replay["artifact_content_hashes_checked"] == 0


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


def test_verify_run_archive_replay_rejects_broken_reference(tmp_path: Path) -> None:
    archive = _archive(tmp_path)
    artifacts = _read_jsonl(archive, "artifacts")
    artifacts[0]["job_id"] = "missing_job"
    _write_jsonl(archive, "artifacts", artifacts)

    with pytest.raises(StriatumError, match="broken reference"):
        verify_run_archive(archive, replay=True)


def test_verify_run_archive_replay_rejects_broken_event_chain(tmp_path: Path) -> None:
    archive = _archive(tmp_path)
    events = _read_jsonl(archive, "events")
    second = {
        "repository_id": "repo_a",
        "event_id": 2,
        "run_id": "run_1",
        "event_type": "job.completed",
        "payload_json": {},
        "created_at": "2026-05-17T00:00:01Z",
        "previous_hash": "not-rowhash",
    }
    second["row_hash"] = _event_hash(second, previous_hash="not-rowhash")
    events.append(second)
    _write_jsonl(archive, "events", events)

    with pytest.raises(StriatumError, match="event chain"):
        verify_run_archive(archive, replay=True)


def test_verify_run_archive_replay_rejects_tampered_event_hash(
    tmp_path: Path,
) -> None:
    archive = _archive(tmp_path)
    events = _read_jsonl(archive, "events")
    events[0]["event_type"] = "run.started"
    _write_jsonl(archive, "events", events)

    with pytest.raises(StriatumError, match="row_hash"):
        verify_run_archive(archive, replay=True)


def test_verify_run_archive_replay_optionally_checks_artifact_hash(
    tmp_path: Path,
) -> None:
    archive = _archive(tmp_path)
    artifact_path = tmp_path / "docs" / "out.md"
    artifact_path.parent.mkdir()
    artifact_path.write_text("different content\n", encoding="utf-8")

    result = verify_run_archive(archive, replay=True)
    replay = result["replay"]
    assert isinstance(replay, dict)
    assert replay["artifact_content_hashes_checked"] == 0

    with pytest.raises(StriatumError, match="artifact content hash mismatch"):
        verify_run_archive(archive, replay=True, repo_root=tmp_path)


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
            "--replay",
            "--json",
        ]
    )

    result = dispatch_mod.dispatch(args)

    assert isinstance(result, dict)
    assert result["status"] == "verified"
