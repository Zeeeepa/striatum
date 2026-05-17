"""Standalone corpus bundle verification."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any, cast

from striatum.corpus.manifest import verify_manifest
from striatum.corpus.types import SCHEMA_VERSION, SUB_KINDS
from striatum.corpus.writer import verify_jsonl_files
from striatum.errors import StriatumError


def verify_corpus_bundle(bundle: Path) -> dict[str, object]:
    """Verify an existing local RFC 0044 corpus bundle."""
    root = bundle.resolve()
    if not root.is_dir():
        raise StriatumError(f"corpus bundle not found: {bundle}", exit_code=8)
    manifest_path = root / "manifest.json"
    if not manifest_path.is_file():
        raise StriatumError(f"corpus manifest not found: {manifest_path}", exit_code=8)
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise StriatumError(f"invalid corpus manifest JSON: {exc}", exit_code=6) from exc
    if not isinstance(manifest, dict):
        raise StriatumError("invalid corpus manifest: expected object", exit_code=6)
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise StriatumError("invalid corpus manifest schema_version", exit_code=6)
    contract_version = int(manifest.get("corpus_contract_version") or 1)
    if contract_version > 1:
        raise StriatumError(
            f"unsupported corpus_contract_version: {contract_version}",
            exit_code=6,
        )
    files = _manifest_files(manifest)
    _verify_declared_bytes(root, files)
    row_counts = verify_jsonl_files(root, files)
    verify_manifest(manifest, row_counts)
    expected_bundle_sha = manifest.get("bundle_sha256")
    bundle_sha = _canonical_manifest_sha(manifest)
    if (
        isinstance(expected_bundle_sha, str)
        and expected_bundle_sha
        and expected_bundle_sha != bundle_sha
    ):
        raise StriatumError("corpus bundle_sha256 mismatch", exit_code=6)
    return {
        "status": "verified",
        "bundle": str(root),
        "manifest_path": str(manifest_path),
        "schema_version": SCHEMA_VERSION,
        "corpus_contract_version": contract_version,
        "row_counts": {kind: row_counts.get(kind, 0) for kind in SUB_KINDS},
        "bundle_sha256": bundle_sha,
    }


def _manifest_files(manifest: dict[str, Any]) -> dict[str, dict[str, int | str]]:
    files = manifest.get("files")
    if not isinstance(files, dict):
        raise StriatumError("invalid corpus manifest files", exit_code=6)
    normalized: dict[str, dict[str, int | str]] = {}
    for filename, metadata in files.items():
        if not isinstance(filename, str) or not isinstance(metadata, dict):
            raise StriatumError("invalid corpus manifest file metadata", exit_code=6)
        sha = metadata.get("sha256")
        rows = metadata.get("rows")
        bytes_count = metadata.get("bytes")
        if (
            not isinstance(sha, str)
            or not isinstance(rows, int)
            or not isinstance(bytes_count, int)
        ):
            raise StriatumError("invalid corpus manifest file metadata", exit_code=6)
        normalized[filename] = {"sha256": sha, "rows": rows, "bytes": bytes_count}
    return normalized


def _verify_declared_bytes(root: Path, files: dict[str, dict[str, int | str]]) -> None:
    for filename, metadata in files.items():
        path = root / filename
        try:
            actual = path.stat().st_size
        except OSError as exc:
            raise StriatumError(f"corpus file missing: {filename}", exit_code=6) from exc
        if actual != metadata["bytes"]:
            raise StriatumError(f"corpus byte count mismatch: {filename}", exit_code=6)


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
