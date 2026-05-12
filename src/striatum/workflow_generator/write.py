"""Filesystem writes for generated workflow trees."""

from __future__ import annotations

import os
from pathlib import Path

from striatum.db import JsonObject
from striatum.workflow import load_workflow
from striatum.workflow_generator.core import GeneratedWorkflow, GeneratorError


def write_generated_workflow(generated: GeneratedWorkflow, *, repo: Path) -> JsonObject:
    """Write generated files atomically and validate the written workflow."""
    repo = repo.resolve()
    files = generated.files
    targets: list[tuple[Path, str, str]] = []
    for entry in files:
        path = entry["path"]
        content = entry["content"]
        if not isinstance(path, str) or not isinstance(content, str):
            raise GeneratorError("generated files must carry string path and content")
        target = _safe_target(repo, path)
        targets.append((target, path, content))
    existing = [relative for target, relative, _ in targets if target.exists()]
    if existing:
        raise GeneratorError(
            "workflow generate refuses to overwrite existing files",
            field_path="files",
            hint=", ".join(sorted(existing)[:5]),
        )
    written: list[str] = []
    try:
        for target, relative, content in targets:
            target.parent.mkdir(parents=True, exist_ok=True)
            tmp = target.with_name(f".{target.name}.tmp.{os.getpid()}")
            tmp.write_text(content, encoding="utf-8")
            os.replace(tmp, target)
            written.append(relative)
        workflow_path = repo / str(generated.metadata["workflow_path"])
        load_workflow(workflow_path)
    except Exception:
        for relative in written:
            try:
                (repo / relative).unlink()
            except OSError:
                pass
        raise
    return {
        "status": "created",
        "path": str(repo / str(generated.metadata["scaffold_root"])),
        "workflow_path": str(workflow_path),
        "files": written,
        "generated": generated.to_json(),
    }


def _safe_target(repo: Path, relative: str) -> Path:
    if "\x00" in relative or relative.startswith("/") or "\\" in relative:
        raise GeneratorError("generated path must be repo-relative POSIX path", field_path="files.path")
    parts = Path(relative).parts
    if ".." in parts or parts[:1] in {(".git",), (".striatum",)} or ".git" in parts or ".striatum" in parts:
        raise GeneratorError("generated path escapes allowed repository area", field_path="files.path")
    target = (repo / relative).resolve()
    try:
        target.relative_to(repo)
    except ValueError as exc:
        raise GeneratorError("generated path escapes repository", field_path="files.path") from exc
    return target

