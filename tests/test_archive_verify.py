from __future__ import annotations

import hashlib
import json
from importlib import import_module
from pathlib import Path

import pytest

from striatum.archive import (
    ARCHIVE_JSON_FILES,
    ARCHIVE_JSONL_FILES,
    inspect_run_archive,
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
    assert result["repository_id"] == "repo_a"
    assert result["archive_contract_version"] == 2
    assert result["verification_depth"] == "deep_chain"
    assert result["hybrid_archive_defaults"] == {
        "snapshot": True,
        "event_log": True,
        "verify_replay_by_default": True,
    }
    assert result["artifact_content_policy"] == "metadata_only"
    replay = result["replay"]
    assert isinstance(replay, dict)
    assert replay["status"] == "verified"
    row_counts = result["row_counts"]
    assert isinstance(row_counts, dict)
    assert row_counts["artifacts"] == 1
    assert row_counts["events"] == 1
    assert row_counts["command_requests"] == 0
    assert row_counts["process_supervisors"] == 0
    assert row_counts["process_supervisor_pointers"] == 0


def test_verify_run_archive_replay_accepts_writer_output(tmp_path: Path) -> None:
    result = verify_run_archive(_archive(tmp_path), replay=True)

    replay = result["replay"]
    assert isinstance(replay, dict)
    assert replay["status"] == "verified"
    assert replay["artifact_content_hashes_checked"] == 0


def test_verify_run_archive_replay_accepts_supervisor_metadata(
    tmp_path: Path,
) -> None:
    archive = _archive(tmp_path)
    _write_jsonl(
        archive,
        "sessions",
        [
            {
                "repository_id": "repo_a",
                "run_id": "run_1",
                "session_id": "sess_1",
            }
        ],
    )
    _write_jsonl(
        archive,
        "process_supervisors",
        [
            {
                "repository_id": "repo_a",
                "run_id": "run_1",
                "session_id": "sess_1",
                "supervisor_id": "sup_1",
            }
        ],
    )
    _write_jsonl(
        archive,
        "process_supervisor_pointers",
        [
            {
                "repository_id": "repo_a",
                "run_id": "run_1",
                "session_id": "sess_1",
                "supervisor_id": "sup_1",
                "daemon_supervisor_id": "daemon_sup_1",
            }
        ],
    )

    result = verify_run_archive(archive, replay=True)

    replay = result["replay"]
    assert isinstance(replay, dict)
    assert replay["status"] == "verified"


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
        verify_run_archive(archive)


def test_verify_run_archive_manifest_only_skips_semantic_replay(tmp_path: Path) -> None:
    archive = _archive(tmp_path)
    artifacts = _read_jsonl(archive, "artifacts")
    artifacts[0]["job_id"] = "missing_job"
    _write_jsonl(archive, "artifacts", artifacts)

    result = verify_run_archive(archive, replay=False)

    assert result["status"] == "verified"
    assert "replay" not in result


def test_verify_run_archive_manifest_only_rejects_repo_root(tmp_path: Path) -> None:
    with pytest.raises(StriatumError, match="repo_root requires semantic replay"):
        verify_run_archive(_archive(tmp_path), replay=False, repo_root=tmp_path)


@pytest.mark.parametrize(
    ("kind", "id_field", "id_value"),
    [
        ("verdicts", "verdict_id", "verdict_1"),
        ("blockers", "blocker_id", "blocker_1"),
        ("command_requests", "request_id", "request_1"),
        ("process_executions", "process_id", "process_1"),
        ("job_worktrees", "worktree_id", "worktree_1"),
        ("process_supervisors", "supervisor_id", "supervisor_1"),
        ("process_supervisor_pointers", "supervisor_id", "supervisor_1"),
    ],
)
def test_verify_run_archive_replay_rejects_duplicate_row_family_ids(
    tmp_path: Path,
    kind: str,
    id_field: str,
    id_value: str,
) -> None:
    archive = _archive(tmp_path)
    row: dict[str, object] = {
        "repository_id": "repo_a",
        "run_id": "run_1",
        id_field: id_value,
    }
    _write_jsonl(archive, kind, [row, dict(row)])

    with pytest.raises(StriatumError, match=f"duplicate {id_field}"):
        verify_run_archive(archive, replay=True)


@pytest.mark.parametrize(
    ("kind", "id_field"),
    [
        ("verdicts", "verdict_id"),
        ("blockers", "blocker_id"),
        ("command_requests", "request_id"),
        ("process_executions", "process_id"),
        ("job_worktrees", "worktree_id"),
        ("process_supervisors", "supervisor_id"),
        ("process_supervisor_pointers", "supervisor_id"),
    ],
)
def test_verify_run_archive_replay_rejects_missing_row_family_ids(
    tmp_path: Path,
    kind: str,
    id_field: str,
) -> None:
    archive = _archive(tmp_path)
    _write_jsonl(
        archive,
        kind,
        [
            {
                "repository_id": "repo_a",
                "run_id": "run_1",
            }
        ],
    )

    with pytest.raises(StriatumError, match=f"invalid {id_field}"):
        verify_run_archive(archive, replay=True)


@pytest.mark.parametrize("kind", ["process_supervisors", "process_supervisor_pointers"])
def test_verify_run_archive_replay_rejects_supervisor_rows_missing_session(
    tmp_path: Path,
    kind: str,
) -> None:
    archive = _archive(tmp_path)
    _write_jsonl(
        archive,
        kind,
        [
            {
                "repository_id": "repo_a",
                "run_id": "run_1",
                "supervisor_id": "sup_1",
            }
        ],
    )

    with pytest.raises(StriatumError, match="missing session_id"):
        verify_run_archive(archive, replay=True)


def test_verify_run_archive_replay_rejects_pointer_without_supervisor(
    tmp_path: Path,
) -> None:
    archive = _archive(tmp_path)
    _write_jsonl(
        archive,
        "sessions",
        [
            {
                "repository_id": "repo_a",
                "run_id": "run_1",
                "session_id": "sess_1",
            }
        ],
    )
    _write_jsonl(
        archive,
        "process_supervisor_pointers",
        [
            {
                "repository_id": "repo_a",
                "run_id": "run_1",
                "session_id": "sess_1",
                "supervisor_id": "missing_sup",
            }
        ],
    )

    with pytest.raises(StriatumError, match="supervisor_id -> process_supervisors"):
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


def test_verify_run_archive_rejects_unsupported_v2_defaults(tmp_path: Path) -> None:
    archive = _archive(tmp_path)
    manifest_path = archive / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest["verification_depth"] = "manifest_only"
    manifest_path.write_text(json.dumps(manifest, sort_keys=True), encoding="utf-8")

    with pytest.raises(StriatumError, match="verification_depth"):
        verify_run_archive(archive)


def test_verify_run_archive_accepts_legacy_v1_manifest_with_default_replay(
    tmp_path: Path,
) -> None:
    archive = _archive(tmp_path)
    manifest_path = archive / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest["archive_contract_version"] = 1
    manifest.pop("verification_depth", None)
    manifest.pop("hybrid_archive_defaults", None)
    manifest.pop("artifact_content_policy", None)
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

    result = verify_run_archive(archive)

    assert result["archive_contract_version"] == 1
    assert result["verification_depth"] == "deep_chain"
    assert "hybrid_archive_defaults" not in result
    assert isinstance(result["replay"], dict)


def test_inspect_run_archive_reports_semantic_and_privacy_metadata(
    tmp_path: Path,
) -> None:
    result = inspect_run_archive(_archive(tmp_path))

    assert result["status"] == "inspected"
    assert result["archive_contract_version"] == 2
    semantic_checks = result["semantic_checks"]
    assert isinstance(semantic_checks, dict)
    assert semantic_checks["deep_chain_replay"] == "verified"
    assert semantic_checks["comparative_replay"] == "not_performed"
    privacy = result["privacy"]
    assert isinstance(privacy, dict)
    assert privacy["artifact_bytes_embedded"] is False
    assert privacy["transcripts_embedded"] is False
    assert privacy["operational_scratch_embedded"] is False


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
    assert "replay" in result


def test_archive_verify_cli_manifest_only_is_explicit_opt_out(
    tmp_path: Path,
) -> None:
    archive = _archive(tmp_path)
    artifacts = _read_jsonl(archive, "artifacts")
    artifacts[0]["job_id"] = "missing_job"
    _write_jsonl(archive, "artifacts", artifacts)
    from striatum.cli.parser import build_parser

    dispatch_mod = import_module("striatum.cli.dispatch")
    args = build_parser().parse_args(
        [
            "--repo",
            str(tmp_path),
            "archive",
            "verify",
            "--bundle",
            str(archive.relative_to(tmp_path)),
            "--manifest-only",
            "--json",
        ]
    )

    result = dispatch_mod.dispatch(args)

    assert isinstance(result, dict)
    assert result["status"] == "verified"
    assert "replay" not in result


def test_archive_verify_cli_manifest_only_rejects_repo_root(
    tmp_path: Path,
) -> None:
    archive = _archive(tmp_path)
    from striatum.cli.parser import build_parser

    dispatch_mod = import_module("striatum.cli.dispatch")
    args = build_parser().parse_args(
        [
            "--repo",
            str(tmp_path),
            "archive",
            "verify",
            "--bundle",
            str(archive.relative_to(tmp_path)),
            "--manifest-only",
            "--repo-root",
            str(tmp_path),
            "--json",
        ]
    )

    with pytest.raises(StriatumError, match="--repo-root requires semantic replay"):
        dispatch_mod.dispatch(args)


def test_archive_inspect_cli_is_local_read_only(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    archive = _archive(tmp_path)
    from striatum.cli.parser import build_parser

    dispatch_mod = import_module("striatum.cli.dispatch")

    def fail_daemon_required(*_args: object, **_kwargs: object) -> None:
        raise AssertionError("archive inspect should not require daemon")

    monkeypatch.setattr(dispatch_mod, "enforce_daemon_required", fail_daemon_required)
    args = build_parser().parse_args(
        [
            "--repo",
            str(tmp_path),
            "archive",
            "inspect",
            "--bundle",
            str(archive.relative_to(tmp_path)),
            "--json",
        ]
    )

    result = dispatch_mod.dispatch(args)

    assert isinstance(result, dict)
    assert result["status"] == "inspected"
    assert result["semantic_checks"]["deep_chain_replay"] == "verified"
