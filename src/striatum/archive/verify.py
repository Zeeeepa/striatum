"""Standalone run archive verification."""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, cast

from striatum.archive.writer import (
    ARCHIVE_JSON_FILES,
    ARCHIVE_JSONL_FILES,
    ARCHIVE_KINDS,
    ARCHIVE_SCHEMA_VERSION,
)
from striatum.errors import StriatumError
from striatum.primitives import json_dumps


@dataclass(frozen=True)
class _ArchivePayload:
    json_files: dict[str, dict[str, Any]]
    rows: dict[str, list[dict[str, Any]]]


def verify_run_archive(
    bundle: Path,
    *,
    replay: bool = False,
    repo_root: Path | None = None,
) -> dict[str, object]:
    """Verify an existing local run archive without daemon or repository state."""
    root = bundle.resolve()
    if not root.is_dir():
        raise StriatumError(f"run archive not found: {bundle}", exit_code=8)
    manifest_path = root / "manifest.json"
    if not manifest_path.is_file():
        raise StriatumError(f"run archive manifest not found: {manifest_path}", exit_code=8)
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise StriatumError(f"invalid run archive manifest JSON: {exc}", exit_code=6) from exc
    if not isinstance(manifest, dict):
        raise StriatumError("invalid run archive manifest: expected object", exit_code=6)
    _verify_manifest_header(manifest)
    files = _manifest_files(manifest)
    row_counts = _verify_files(root, files)
    _verify_counts(manifest, row_counts)
    expected_bundle_sha = manifest.get("bundle_sha256")
    bundle_sha = _canonical_manifest_sha(manifest)
    if (
        isinstance(expected_bundle_sha, str)
        and expected_bundle_sha
        and expected_bundle_sha != bundle_sha
    ):
        raise StriatumError("run archive bundle_sha256 mismatch", exit_code=6)
    replay_result: dict[str, object] | None = None
    if replay or repo_root is not None:
        payload = _load_archive_payload(root)
        replay_result = _verify_replay(manifest, payload, repo_root=repo_root)
    run_id = manifest.get("run_id")
    result: dict[str, object] = {
        "status": "verified",
        "bundle": str(root),
        "manifest_path": str(manifest_path),
        "schema_version": ARCHIVE_SCHEMA_VERSION,
        "archive_contract_version": 1,
        "run_id": str(run_id) if isinstance(run_id, str) else "",
        "row_counts": {kind: row_counts.get(kind, 0) for kind in ARCHIVE_KINDS},
        "bundle_sha256": bundle_sha,
    }
    if replay_result is not None:
        result["replay"] = replay_result
    return result


def _verify_manifest_header(manifest: dict[str, Any]) -> None:
    if manifest.get("schema_version") != ARCHIVE_SCHEMA_VERSION:
        raise StriatumError("invalid run archive manifest schema_version", exit_code=6)
    contract_version = manifest.get("archive_contract_version")
    if contract_version != 1:
        raise StriatumError("unsupported run archive contract version", exit_code=6)
    if manifest.get("archive_kind") != "run":
        raise StriatumError("invalid run archive kind", exit_code=6)
    run_id = manifest.get("run_id")
    if not isinstance(run_id, str) or not run_id:
        raise StriatumError("invalid run archive run_id", exit_code=6)


def _manifest_files(manifest: dict[str, Any]) -> dict[str, dict[str, int | str]]:
    files = manifest.get("files")
    if not isinstance(files, dict):
        raise StriatumError("invalid run archive manifest files", exit_code=6)
    expected = set(ARCHIVE_JSON_FILES.values()) | set(ARCHIVE_JSONL_FILES.values())
    if set(files) != expected:
        missing = sorted(expected - set(files))
        extra = sorted(set(files) - expected)
        detail = []
        if missing:
            detail.append("missing " + ", ".join(missing))
        if extra:
            detail.append("unexpected " + ", ".join(extra))
        raise StriatumError("invalid run archive file set: " + "; ".join(detail), exit_code=6)
    normalized: dict[str, dict[str, int | str]] = {}
    for filename, metadata in files.items():
        if not isinstance(filename, str) or not isinstance(metadata, dict):
            raise StriatumError("invalid run archive file metadata", exit_code=6)
        if Path(filename).is_absolute() or ".." in Path(filename).parts:
            raise StriatumError("run archive file path escapes bundle", exit_code=6)
        sha = metadata.get("sha256")
        rows = metadata.get("rows")
        bytes_count = metadata.get("bytes")
        if (
            not isinstance(sha, str)
            or not isinstance(rows, int)
            or not isinstance(bytes_count, int)
        ):
            raise StriatumError("invalid run archive file metadata", exit_code=6)
        normalized[filename] = {"sha256": sha, "rows": rows, "bytes": bytes_count}
    return normalized


def _verify_files(root: Path, files: dict[str, dict[str, int | str]]) -> dict[str, int]:
    row_counts = {kind: 0 for kind in ARCHIVE_KINDS}
    json_filename_to_kind = {filename: kind for kind, filename in ARCHIVE_JSON_FILES.items()}
    jsonl_filename_to_kind = {filename: kind for kind, filename in ARCHIVE_JSONL_FILES.items()}
    for filename, expected in files.items():
        path = root / filename
        try:
            data = path.read_bytes()
        except OSError as exc:
            raise StriatumError(f"run archive file missing: {filename}", exit_code=6) from exc
        if len(data) != expected["bytes"]:
            raise StriatumError(f"run archive byte count mismatch: {filename}", exit_code=6)
        if hashlib.sha256(data).hexdigest() != expected["sha256"]:
            raise StriatumError(f"run archive file hash mismatch: {filename}", exit_code=6)
        if filename in json_filename_to_kind:
            _verify_json_file(filename, data)
            rows = 1
            if expected["rows"] != rows:
                raise StriatumError(f"run archive row count mismatch: {filename}", exit_code=6)
            row_counts[json_filename_to_kind[filename]] = rows
        elif filename in jsonl_filename_to_kind:
            rows = _verify_jsonl_file(filename, data)
            if expected["rows"] != rows:
                raise StriatumError(f"run archive row count mismatch: {filename}", exit_code=6)
            row_counts[jsonl_filename_to_kind[filename]] = rows
    return row_counts


def _verify_json_file(filename: str, data: bytes) -> None:
    try:
        loaded = json.loads(data.decode("utf-8"))
    except json.JSONDecodeError as exc:
        raise StriatumError(f"invalid run archive JSON file: {filename}", exit_code=6) from exc
    if not isinstance(loaded, dict):
        raise StriatumError(f"invalid run archive JSON object: {filename}", exit_code=6)


def _verify_jsonl_file(filename: str, data: bytes) -> int:
    rows = 0
    if not data:
        return 0
    try:
        lines = data.decode("utf-8").splitlines()
    except UnicodeDecodeError as exc:
        raise StriatumError(f"invalid UTF-8 in run archive file: {filename}", exit_code=6) from exc
    for line in lines:
        if line == "":
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError as exc:
            raise StriatumError(f"invalid JSONL row in run archive file: {filename}", exit_code=6) from exc
        if not isinstance(row, dict):
            raise StriatumError(f"invalid JSONL row object in run archive file: {filename}", exit_code=6)
        rows += 1
    return rows


def _verify_counts(manifest: dict[str, Any], row_counts: dict[str, int]) -> None:
    counts = manifest.get("row_counts")
    if not isinstance(counts, dict):
        raise StriatumError("invalid run archive row_counts", exit_code=6)
    for kind in ARCHIVE_KINDS:
        if int(counts.get(kind, -1)) != int(row_counts.get(kind, 0)):
            raise StriatumError(f"run archive row count mismatch: {kind}", exit_code=6)


def _load_archive_payload(root: Path) -> _ArchivePayload:
    json_files: dict[str, dict[str, Any]] = {}
    rows: dict[str, list[dict[str, Any]]] = {}
    for kind, filename in ARCHIVE_JSON_FILES.items():
        json_files[kind] = _load_json_file(root / filename, filename)
    for kind, filename in ARCHIVE_JSONL_FILES.items():
        rows[kind] = _load_jsonl_rows(root / filename, filename)
    return _ArchivePayload(json_files=json_files, rows=rows)


def _load_json_file(path: Path, filename: str) -> dict[str, Any]:
    try:
        loaded = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise StriatumError(f"invalid run archive JSON file: {filename}", exit_code=6) from exc
    if not isinstance(loaded, dict):
        raise StriatumError(f"invalid run archive JSON object: {filename}", exit_code=6)
    return cast(dict[str, Any], loaded)


def _load_jsonl_rows(path: Path, filename: str) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except UnicodeDecodeError as exc:
        raise StriatumError(f"invalid UTF-8 in run archive file: {filename}", exit_code=6) from exc
    for line in lines:
        if line == "":
            continue
        try:
            loaded = json.loads(line)
        except json.JSONDecodeError as exc:
            raise StriatumError(f"invalid JSONL row in run archive file: {filename}", exit_code=6) from exc
        if not isinstance(loaded, dict):
            raise StriatumError(f"invalid JSONL row object in run archive file: {filename}", exit_code=6)
        rows.append(cast(dict[str, Any], loaded))
    return rows


def _verify_replay(
    manifest: dict[str, Any],
    payload: _ArchivePayload,
    *,
    repo_root: Path | None,
) -> dict[str, object]:
    repository_id = _manifest_text(manifest, "repository_id")
    run_id = _manifest_text(manifest, "run_id")
    _verify_run_repository_consistency(
        repository_id=repository_id,
        run_id=run_id,
        manifest=manifest,
        payload=payload,
    )
    indexes = _build_indexes(payload)
    _verify_references(indexes=indexes, payload=payload)
    _verify_events(payload.rows["events"], indexes=indexes)
    artifact_hashes_checked = _verify_artifact_content_hashes(
        payload.rows["artifacts"],
        repo_root=repo_root,
    )
    return {
        "status": "verified",
        "artifact_content_hashes_checked": artifact_hashes_checked,
    }


def _manifest_text(manifest: dict[str, Any], field: str) -> str:
    value = manifest.get(field)
    if not isinstance(value, str) or not value:
        raise StriatumError(f"invalid run archive {field}", exit_code=6)
    return value


def _verify_run_repository_consistency(
    *,
    repository_id: str,
    run_id: str,
    manifest: dict[str, Any],
    payload: _ArchivePayload,
) -> None:
    run = payload.json_files["run"]
    workflow_snapshot = payload.json_files["workflow_snapshot"]
    if run.get("repository_id") != repository_id or run.get("run_id") != run_id:
        raise StriatumError("run archive replay repository/run mismatch: run", exit_code=6)
    if workflow_snapshot.get("repository_id") != repository_id:
        raise StriatumError(
            "run archive replay repository mismatch: workflow_snapshot",
            exit_code=6,
        )
    workflow_snapshot_id = run.get("workflow_snapshot_id")
    if not isinstance(workflow_snapshot_id, str) or not workflow_snapshot_id:
        raise StriatumError(
            "run archive replay missing workflow snapshot reference",
            exit_code=6,
        )
    if workflow_snapshot.get("workflow_snapshot_id") != workflow_snapshot_id:
        raise StriatumError(
            "run archive replay workflow snapshot reference is broken",
            exit_code=6,
        )
    manifest_repo_root = manifest.get("repo_root")
    run_repo_root = run.get("repo_root")
    if (
        isinstance(manifest_repo_root, str)
        and isinstance(run_repo_root, str)
        and manifest_repo_root != run_repo_root
    ):
        raise StriatumError("run archive replay repository root mismatch", exit_code=6)

    for kind, rows in payload.rows.items():
        for index, row in enumerate(rows, start=1):
            context = f"{kind} row {index}"
            if row.get("repository_id") != repository_id:
                raise StriatumError(
                    f"run archive replay repository mismatch: {context}",
                    exit_code=6,
                )
            if kind != "job_dependencies" and row.get("run_id") != run_id:
                raise StriatumError(
                    f"run archive replay run mismatch: {context}",
                    exit_code=6,
                )


def _build_indexes(payload: _ArchivePayload) -> dict[str, dict[str, dict[str, Any]]]:
    return {
        "sessions": _index_rows(payload.rows["sessions"], "session_id", "sessions"),
        "jobs": _index_rows(payload.rows["jobs"], "job_id", "jobs"),
        "queue_messages": _index_rows(
            payload.rows["queue_messages"],
            "message_id",
            "queue_messages",
        ),
        "leases": _index_rows(payload.rows["leases"], "lease_id", "leases"),
        "work_packets": _index_rows(payload.rows["work_packets"], "packet_id", "work_packets"),
        "artifacts": _index_rows(payload.rows["artifacts"], "artifact_id", "artifacts"),
    }


def _index_rows(
    rows: list[dict[str, Any]],
    id_field: str,
    kind: str,
) -> dict[str, dict[str, Any]]:
    index: dict[str, dict[str, Any]] = {}
    for position, row in enumerate(rows, start=1):
        value = row.get(id_field)
        if not isinstance(value, str) or not value:
            raise StriatumError(
                f"run archive replay invalid {id_field}: {kind} row {position}",
                exit_code=6,
            )
        if value in index:
            raise StriatumError(
                f"run archive replay duplicate {id_field}: {value}",
                exit_code=6,
            )
        index[value] = row
    return index


def _verify_references(
    *,
    indexes: dict[str, dict[str, dict[str, Any]]],
    payload: _ArchivePayload,
) -> None:
    sessions = indexes["sessions"]
    jobs = indexes["jobs"]
    messages = indexes["queue_messages"]
    leases = indexes["leases"]
    packets = indexes["work_packets"]
    artifacts = indexes["artifacts"]

    for row in payload.rows["sessions"]:
        _check_ref(row, "parent_session_id", sessions, "sessions")
    for row in payload.rows["jobs"]:
        _check_ref(row, "current_message_id", messages, "queue_messages")
        _check_ref(row, "current_lease_id", leases, "leases")
    for row in payload.rows["job_dependencies"]:
        _check_ref(row, "job_id", jobs, "jobs", required=True)
        _check_ref(row, "depends_on_job_id", jobs, "jobs", required=True)
    for row in payload.rows["queue_messages"]:
        _check_ref(row, "job_id", jobs, "jobs")
        _check_ref(row, "target_session_id", sessions, "sessions")
        _check_ref(row, "current_lease_id", leases, "leases")
    for row in payload.rows["leases"]:
        if row.get("resource_type") == "job":
            _check_ref(row, "resource_id", jobs, "jobs", required=True)
        _check_ref(row, "owner_session_id", sessions, "sessions", required=True)
    for row in payload.rows["work_packets"]:
        _check_ref(row, "job_id", jobs, "jobs", required=True)
        _check_ref(row, "message_id", messages, "queue_messages", required=True)
        _check_ref(row, "lease_id", leases, "leases", required=True)
        _check_ref(row, "session_id", sessions, "sessions", required=True)
    for row in payload.rows["artifacts"]:
        _check_ref(row, "job_id", jobs, "jobs")
        _check_ref(row, "session_id", sessions, "sessions")
    for row in payload.rows["verdicts"]:
        _check_ref(row, "job_id", jobs, "jobs", required=True)
        _check_ref(row, "session_id", sessions, "sessions", required=True)
        _check_ref(row, "findings_artifact_id", artifacts, "artifacts")
    for row in payload.rows["blockers"]:
        _check_ref(row, "job_id", jobs, "jobs")
        _check_ref(row, "session_id", sessions, "sessions")
    for row in payload.rows["process_executions"]:
        _check_ref(row, "job_id", jobs, "jobs", required=True)
        _check_ref(row, "session_id", sessions, "sessions", required=True)
        _check_ref(row, "lease_id", leases, "leases", required=True)
        _check_ref(row, "packet_id", packets, "work_packets", required=True)
    for row in payload.rows["job_worktrees"]:
        _check_ref(row, "job_id", jobs, "jobs", required=True)
        _check_ref(row, "lease_id", leases, "leases", required=True)


def _check_ref(
    row: dict[str, Any],
    field: str,
    target: dict[str, dict[str, Any]],
    target_kind: str,
    *,
    required: bool = False,
) -> None:
    value = row.get(field)
    if value is None or value == "":
        if required:
            raise StriatumError(
                f"run archive replay broken reference: missing {field}",
                exit_code=6,
            )
        return
    if not isinstance(value, str) or value not in target:
        raise StriatumError(
            f"run archive replay broken reference: {field} -> {target_kind}",
            exit_code=6,
        )


def _verify_events(
    events: list[dict[str, Any]],
    *,
    indexes: dict[str, dict[str, dict[str, Any]]],
) -> None:
    previous_event_id: int | None = None
    event_chain: list[tuple[int, str | None, str | None]] = []
    row_hash_indexes: dict[str, int] = {}
    for index, row in enumerate(events):
        event_id = _event_id(row)
        if previous_event_id is not None and event_id <= previous_event_id:
            raise StriatumError("run archive replay event ordering is not monotonic", exit_code=6)
        previous_event_id = event_id
        _check_ref(row, "actor_session_id", indexes["sessions"], "sessions")
        _check_ref(row, "job_id", indexes["jobs"], "jobs")
        _check_ref(row, "message_id", indexes["queue_messages"], "queue_messages")
        _check_ref(row, "artifact_id", indexes["artifacts"], "artifacts")
        _check_ref(row, "lease_id", indexes["leases"], "leases")
        previous_hash, row_hash = _event_hashes(row)
        _verify_event_row_hash(row, previous_hash=previous_hash, row_hash=row_hash)
        if row_hash is not None:
            if row_hash in row_hash_indexes:
                raise StriatumError(
                    "run archive replay event chain has duplicate row_hash",
                    exit_code=6,
                )
            row_hash_indexes[row_hash] = index
        event_chain.append((event_id, previous_hash, row_hash))

    for index, (event_id, previous_hash, _row_hash) in enumerate(event_chain):
        if previous_hash is None:
            continue
        expected_index = row_hash_indexes.get(previous_hash)
        if expected_index is not None and expected_index != index - 1:
            raise StriatumError("run archive replay event chain is broken", exit_code=6)
        if expected_index is None and index > 0:
            previous_event_id, _previous_hash, previous_row_hash = event_chain[index - 1]
            if previous_row_hash is not None and event_id == previous_event_id + 1:
                raise StriatumError("run archive replay event chain is broken", exit_code=6)


def _event_id(row: dict[str, Any]) -> int:
    value = row.get("event_id")
    if type(value) is not int:
        raise StriatumError("run archive replay invalid event_id", exit_code=6)
    return value


def _event_hashes(row: dict[str, Any]) -> tuple[str | None, str | None]:
    previous_hash = _text_or_none(row.get("previous_hash"))
    row_hash = _text_or_none(row.get("row_hash"))
    payload = row.get("payload_json")
    if isinstance(payload, dict):
        chain = payload.get("_event_chain")
        if isinstance(chain, dict):
            previous_hash = previous_hash or _text_or_none(chain.get("previous_hash"))
            row_hash = row_hash or _text_or_none(chain.get("row_hash"))
    return previous_hash, row_hash


def _verify_event_row_hash(
    row: dict[str, Any],
    *,
    previous_hash: str | None,
    row_hash: str | None,
) -> None:
    if row_hash is None:
        return
    computed = _canonical_event_hash(row, previous_hash=previous_hash)
    if computed != row_hash:
        raise StriatumError("run archive replay event row_hash is invalid", exit_code=6)


def _canonical_event_hash(row: dict[str, Any], *, previous_hash: str | None) -> str:
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
        "payload_json": _event_payload(row.get("payload_json") or {}),
        "created_at": str(row.get("created_at")),
    }
    return hashlib.sha256(json_dumps(payload).encode("utf-8")).hexdigest()


def _event_payload(value: object) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise StriatumError("run archive replay invalid event payload_json", exit_code=6)
    return {str(key): item for key, item in value.items() if key != "_event_chain"}


def _text_or_none(value: object) -> str | None:
    if isinstance(value, str) and value:
        return value
    return None


def _verify_artifact_content_hashes(
    artifacts: list[dict[str, Any]],
    *,
    repo_root: Path | None,
) -> int:
    if repo_root is None:
        return 0
    root = repo_root.resolve()
    checked = 0
    for row in artifacts:
        repo_path = row.get("repo_path")
        expected_hash = row.get("content_sha256")
        if not isinstance(repo_path, str) or not repo_path:
            raise StriatumError("run archive replay invalid artifact repo_path", exit_code=6)
        if not isinstance(expected_hash, str) or not expected_hash:
            raise StriatumError("run archive replay invalid artifact content_sha256", exit_code=6)
        artifact_path = _safe_repo_path(root, repo_path)
        try:
            data = artifact_path.read_bytes()
        except OSError as exc:
            raise StriatumError(
                f"run archive replay artifact content missing: {repo_path}",
                exit_code=6,
            ) from exc
        actual_hash = hashlib.sha256(data).hexdigest()
        if actual_hash != expected_hash:
            raise StriatumError(
                f"run archive replay artifact content hash mismatch: {repo_path}",
                exit_code=6,
            )
        checked += 1
    return checked


def _safe_repo_path(root: Path, repo_path: str) -> Path:
    candidate = Path(repo_path)
    if candidate.is_absolute() or ".." in candidate.parts:
        raise StriatumError("run archive replay artifact path escapes repo root", exit_code=6)
    resolved = (root / candidate).resolve()
    try:
        resolved.relative_to(root)
    except ValueError as exc:
        raise StriatumError("run archive replay artifact path escapes repo root", exit_code=6) from exc
    return resolved


def _canonical_manifest_sha(manifest: dict[str, Any]) -> str:
    canonical_manifest = dict(manifest)
    canonical_manifest.pop("bundle_sha256", None)
    body = json.dumps(
        cast(dict[str, object], canonical_manifest),
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")
    return hashlib.sha256(body).hexdigest()
