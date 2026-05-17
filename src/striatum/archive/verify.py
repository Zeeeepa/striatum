"""Standalone run archive verification."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any, cast

from striatum.archive.writer import (
    ARCHIVE_JSON_FILES,
    ARCHIVE_JSONL_FILES,
    ARCHIVE_KINDS,
    ARCHIVE_SCHEMA_VERSION,
)
from striatum.errors import StriatumError


def verify_run_archive(bundle: Path) -> dict[str, object]:
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
    run_id = manifest.get("run_id")
    return {
        "status": "verified",
        "bundle": str(root),
        "manifest_path": str(manifest_path),
        "schema_version": ARCHIVE_SCHEMA_VERSION,
        "archive_contract_version": 1,
        "run_id": str(run_id) if isinstance(run_id, str) else "",
        "row_counts": {kind: row_counts.get(kind, 0) for kind in ARCHIVE_KINDS},
        "bundle_sha256": bundle_sha,
    }


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
