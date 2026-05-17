"""Repository file-view presentation helpers."""

from __future__ import annotations

from pathlib import Path
from typing import Any

JsonObject = dict[str, Any]

__all__ = ["ViewFileError", "view_file_payload"]

_BINARY_EXTS = {
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".pdf",
    ".zip",
    ".tar",
    ".gz",
    ".ico",
    ".woff",
    ".woff2",
    ".ttf",
    ".eot",
    ".mp3",
    ".mp4",
    ".mov",
    ".bin",
    ".so",
}


class ViewFileError(Exception):
    def __init__(self, status_code: int, message: str) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.message = message


def view_file_payload(repo: Path, subpath: str) -> JsonObject:
    if not subpath:
        raise ViewFileError(404, "path not found")
    if subpath.startswith("/") or ".." in Path(subpath).parts or "\x00" in subpath:
        raise ViewFileError(400, "invalid path")
    target = (repo / subpath).resolve()
    repo_root = repo.resolve()
    try:
        target.relative_to(repo_root)
    except ValueError as exc:
        raise ViewFileError(400, "path escapes repo root") from exc
    rel_parts = target.relative_to(repo_root).parts
    if rel_parts and rel_parts[0] in (".git", ".striatum"):
        raise ViewFileError(404, "hidden path")
    if not target.exists():
        raise ViewFileError(404, "path not found")
    if not target.is_file():
        raise ViewFileError(404, "directory listing not in V1; view a file directly")
    try:
        raw = target.read_bytes()
    except OSError as exc:
        raise ViewFileError(500, str(exc)) from exc

    rel_path = target.relative_to(repo_root).as_posix()
    ext = target.suffix.lower()
    is_binary = (ext in _BINARY_EXTS) or (b"\x00" in raw[:1024])
    ctx: JsonObject = {
        "rel_path": rel_path,
        "size_bytes": len(raw),
        "mode": None,
        "rendered_html": None,
        "text_body": None,
        "lang": ext.lstrip(".") or "text",
        "message": None,
        "run_breadcrumb": None,
    }
    if is_binary:
        ctx["message"] = f"Binary file ({len(raw)} bytes); use the raw API to download."
    elif ext == ".md":
        from striatum.web.markdown import render as md_render

        ctx["rendered_html"] = md_render(raw.decode("utf-8", errors="replace"))
    else:
        ctx["text_body"] = raw.decode("utf-8", errors="replace")
    return ctx
