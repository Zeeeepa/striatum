"""Deterministic local run archive writer."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

from striatum.corpus.manifest import generated_at_now, striatum_version
from striatum.errors import StriatumError

ARCHIVE_SCHEMA_VERSION = "striatum.run_archive.v1"
ARCHIVE_JSON_FILES: dict[str, str] = {
    "run": "run.json",
    "workflow_snapshot": "workflow_snapshot.json",
}
ARCHIVE_JSONL_FILES: dict[str, str] = {
    "sessions": "sessions.jsonl",
    "jobs": "jobs.jsonl",
    "job_dependencies": "job_dependencies.jsonl",
    "queue_messages": "queue_messages.jsonl",
    "leases": "leases.jsonl",
    "work_packets": "work_packets.jsonl",
    "artifacts": "artifacts.jsonl",
    "verdicts": "verdicts.jsonl",
    "blockers": "blockers.jsonl",
    "process_executions": "process_executions.jsonl",
    "job_worktrees": "job_worktrees.jsonl",
    "events": "events.jsonl",
}
ARCHIVE_KINDS: tuple[str, ...] = (
    *ARCHIVE_JSON_FILES.keys(),
    *ARCHIVE_JSONL_FILES.keys(),
)


def write_run_archive(
    out: Path,
    *,
    repo_root: Path,
    repository_id: str,
    run_id: str,
    run: dict[str, Any],
    workflow_snapshot: dict[str, Any],
    rows: dict[str, list[dict[str, Any]]],
    generated_at: str | None = None,
) -> dict[str, object]:
    """Write a run archive directory and return a result envelope."""
    if out.exists() and not out.is_dir():
        raise StriatumError("--out must be a directory", exit_code=8)
    out.mkdir(parents=True, exist_ok=True)

    files: dict[str, dict[str, int | str]] = {}
    row_counts: dict[str, int] = {}
    files[ARCHIVE_JSON_FILES["run"]] = _write_json(out / ARCHIVE_JSON_FILES["run"], run)
    row_counts["run"] = 1
    files[ARCHIVE_JSON_FILES["workflow_snapshot"]] = _write_json(
        out / ARCHIVE_JSON_FILES["workflow_snapshot"],
        workflow_snapshot,
    )
    row_counts["workflow_snapshot"] = 1
    for kind, filename in ARCHIVE_JSONL_FILES.items():
        kind_rows = rows.get(kind, [])
        files[filename] = _write_jsonl(out / filename, kind_rows)
        row_counts[kind] = len(kind_rows)

    manifest = build_manifest(
        repo_root=repo_root,
        repository_id=repository_id,
        run_id=run_id,
        files=files,
        row_counts=row_counts,
        generated_at=generated_at,
    )
    manifest["bundle_sha256"] = _canonical_manifest_sha(manifest)
    manifest_path = out / "manifest.json"
    _write_manifest(manifest_path, manifest)
    return {
        "status": "archived",
        "run_id": run_id,
        "out": _display_path(repo_root, out),
        "manifest_path": _display_path(repo_root, manifest_path),
        "row_counts": row_counts,
        "bundle_sha256": manifest["bundle_sha256"],
    }


def build_manifest(
    *,
    repo_root: Path,
    repository_id: str,
    run_id: str,
    files: dict[str, dict[str, int | str]],
    row_counts: dict[str, int],
    generated_at: str | None = None,
) -> dict[str, object]:
    return {
        "schema_version": ARCHIVE_SCHEMA_VERSION,
        "striatum_version": striatum_version(),
        "repository_id": repository_id,
        "repo_root": str(repo_root.resolve()),
        "run_id": run_id,
        "generated_at": generated_at or generated_at_now(),
        "archive_kind": "run",
        "archive_contract_version": 1,
        "schema": {
            "row_shape_version": 1,
            "json_files": dict(ARCHIVE_JSON_FILES),
            "jsonl_files": dict(ARCHIVE_JSONL_FILES),
        },
        "row_counts": {kind: int(row_counts.get(kind, 0)) for kind in ARCHIVE_KINDS},
        "files": files,
    }


def _write_json(path: Path, payload: dict[str, Any]) -> dict[str, int | str]:
    body = json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True).encode("utf-8") + b"\n"
    _atomic_write(path, body)
    return {"sha256": hashlib.sha256(body).hexdigest(), "rows": 1, "bytes": len(body)}


def _write_jsonl(path: Path, rows: list[dict[str, Any]]) -> dict[str, int | str]:
    lines = [
        json.dumps(row, ensure_ascii=False, separators=(",", ":"), sort_keys=True)
        for row in rows
    ]
    body = ("\n".join(lines) + "\n").encode("utf-8")
    _atomic_write(path, body)
    return {"sha256": hashlib.sha256(body).hexdigest(), "rows": len(rows), "bytes": len(body)}


def _write_manifest(path: Path, manifest: dict[str, object]) -> None:
    body = json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True).encode("utf-8") + b"\n"
    _atomic_write(path, body)


def _atomic_write(path: Path, body: bytes) -> None:
    tmp = path.with_name(f".{path.name}.tmp")
    tmp.write_bytes(body)
    tmp.replace(path)


def _canonical_manifest_sha(manifest: dict[str, object]) -> str:
    canonical = dict(manifest)
    canonical.pop("bundle_sha256", None)
    return hashlib.sha256(
        json.dumps(
            canonical,
            ensure_ascii=False,
            separators=(",", ":"),
            sort_keys=True,
        ).encode("utf-8")
    ).hexdigest()


def _display_path(repo_root: Path, path: Path) -> str:
    try:
        return str(path.resolve().relative_to(repo_root.resolve()))
    except ValueError:
        return str(path)
