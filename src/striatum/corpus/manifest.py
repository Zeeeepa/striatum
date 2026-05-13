"""Manifest construction and verification for corpus bundles."""

from __future__ import annotations

import hashlib
import json
import sqlite3
from datetime import datetime, timezone
from importlib import metadata
from pathlib import Path

from striatum.corpus import git as git_helpers
from striatum.corpus.types import ROW_SHAPE_VERSION, SCHEMA_VERSION, SOURCE_KIND, SUB_KINDS
from striatum.errors import StriatumError


def striatum_version() -> str:
    try:
        return metadata.version("striatum-orchestrator")
    except metadata.PackageNotFoundError:
        return "unknown"


def generated_at_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def repo_local_schema_version(conn: sqlite3.Connection) -> int:
    row = conn.execute("PRAGMA user_version").fetchone()
    return int(row[0]) if row is not None else 0


def build_manifest(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    since_ref: str,
    since_commit: str,
    files: dict[str, dict[str, int | str]],
    row_counts: dict[str, int],
    missing_optional_sources: list[str],
    generated_at: str | None = None,
) -> dict[str, object]:
    return {
        "schema_version": SCHEMA_VERSION,
        "striatum_version": striatum_version(),
        "repo_root": str(repo.resolve()),
        "git_head": git_helpers.head_commit(repo),
        "git_dirty": git_helpers.is_dirty(repo),
        "since_ref": since_ref,
        "since_commit": since_commit,
        "generated_at": generated_at or generated_at_now(),
        "schema": {
            "row_shape_version": ROW_SHAPE_VERSION,
            "sub_kinds": list(SUB_KINDS),
        },
        "source_kinds": [SOURCE_KIND],
        "row_counts": {kind: int(row_counts.get(kind, 0)) for kind in SUB_KINDS},
        "files": files,
        "repo_local_schema_version": repo_local_schema_version(conn),
        "missing_optional_sources": missing_optional_sources,
        "daemon_audit_included": False,
    }


def write_manifest(out: Path, manifest: dict[str, object]) -> tuple[Path, str]:
    path = out / "manifest.json"
    body = json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=False).encode("utf-8") + b"\n"
    tmp = out / ".manifest.json.tmp"
    tmp.write_bytes(body)
    tmp.replace(path)
    canonical = json.dumps(manifest, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode("utf-8")
    return path, hashlib.sha256(canonical).hexdigest()


def verify_manifest(manifest: dict[str, object], row_counts: dict[str, int]) -> None:
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise StriatumError("invalid corpus manifest schema_version", exit_code=6)
    counts = manifest.get("row_counts")
    if not isinstance(counts, dict):
        raise StriatumError("invalid corpus manifest row_counts", exit_code=6)
    for kind in SUB_KINDS:
        if int(counts.get(kind, -1)) != int(row_counts.get(kind, 0)):
            raise StriatumError(f"manifest row count mismatch: {kind}", exit_code=6)
