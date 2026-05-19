"""Historical dogfood browse surface (RFC 0072 step 8 follow-on).

The bulk-migration of ``docs/dogfood/`` into blob storage left the
content addressable only by `(dogfood_id, rel_path)`; there are no
striatumd.artifacts rows for these files because they predate the
daemon's run/job/session foreign-key chain. This module adds a thin
web surface that lets a human principal browse the historical content
through the same striatum web server they use for live runs.

Routes (wired in service_routes.py):

    GET /dogfood/                          → render_dogfood_index_page
    GET /dogfood/<id>/                     → render_dogfood_run_page
    GET /dogfood/<id>/<rel_path>           → render_dogfood_file_page
    GET /dogfood/<id>/<rel_path>?raw=1     → serve_dogfood_file_raw

Every render path is daemon-mediated: the corpus.list_historical_*
and corpus.fetch_historical_dogfood_file RPCs do the S3 round-trip,
verify sha256, and hand the body back as base64.
"""

from __future__ import annotations

import base64
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

__all__ = [
    "DogfoodHistoricalContext",
    "render_dogfood_index_page",
    "render_dogfood_run_page",
    "render_dogfood_file_page",
    "serve_dogfood_file_raw",
]

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
HtmlSender = Callable[[int, str], None]
TemplateEnvFactory = Callable[[], Any]
ResponseStarter = Callable[[int], None]
HeaderSender = Callable[[str, str], None]
HeaderFinisher = Callable[[], None]
BodyWriter = Callable[[bytes], object]


@dataclass(frozen=True)
class DogfoodHistoricalContext:
    repo: Path
    send_json: JsonSender
    send_html: HtmlSender
    jinja_env: TemplateEnvFactory


@dataclass(frozen=True)
class DogfoodHistoricalRawContext:
    repo: Path
    send_json: JsonSender
    send_response: ResponseStarter
    send_header: HeaderSender
    end_headers: HeaderFinisher
    write_body: BodyWriter


def render_dogfood_index_page(ctx: DogfoodHistoricalContext) -> None:
    """Render the top-level dogfood-historical index."""

    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        payload = call_repo_method(ctx.repo, "corpus.list_historical_dogfoods", {})
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
        return
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return

    raw_ids = payload.get("dogfood_ids") if isinstance(payload, Mapping) else None
    ids = sorted(_str_list(raw_ids))
    try:
        html = ctx.jinja_env().get_template("dogfood_historical_index.html").render(
            dogfood_ids=ids,
        )
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"template render failed: {exc}"))
        return
    ctx.send_html(200, html)


def render_dogfood_run_page(ctx: DogfoodHistoricalContext, dogfood_id: str) -> None:
    """List every migrated file in one dogfood run."""

    if not _is_safe_id(dogfood_id):
        ctx.send_json(400, _error(400, "invalid dogfood_id"))
        return

    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        payload = call_repo_method(
            ctx.repo,
            "corpus.list_historical_dogfood_files",
            {"dogfood_id": dogfood_id},
        )
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
        return
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return

    raw_files = payload.get("files") if isinstance(payload, Mapping) else None
    files = [
        _shape_file_row(item)
        for item in (raw_files or [])
        if isinstance(item, Mapping)
    ]
    files.sort(key=lambda item: item["rel_path"])

    try:
        html = ctx.jinja_env().get_template("dogfood_historical_run.html").render(
            dogfood_id=dogfood_id,
            files=files,
        )
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"template render failed: {exc}"))
        return
    ctx.send_html(200, html)


def render_dogfood_file_page(
    ctx: DogfoodHistoricalContext,
    dogfood_id: str,
    rel_path: str,
) -> None:
    """Render one historical file. Markdown bodies render through the
    existing md pipeline; other types show metadata + a raw-download
    link so the operator can fetch the bytes."""

    if not _is_safe_id(dogfood_id):
        ctx.send_json(400, _error(400, "invalid dogfood_id"))
        return
    if not _is_safe_rel_path(rel_path):
        ctx.send_json(400, _error(400, "invalid rel_path"))
        return

    payload = _fetch_dogfood_blob(ctx.send_json, ctx.repo, dogfood_id, rel_path)
    if payload is None:
        return

    content_type = str(payload.get("content_type") or "application/octet-stream")
    body = _decode_blob_body(payload, ctx.send_json)
    if body is None:
        return

    rendered_md: str | None = None
    body_text: str | None = None
    if content_type.startswith("text/markdown"):
        try:
            text = body.decode("utf-8")
            from striatum.web.markdown import render as md_render

            rendered_md = md_render(text)
        except UnicodeDecodeError:
            body_text = None
    elif content_type.startswith("text/") or content_type == "application/json":
        try:
            body_text = body.decode("utf-8")
        except UnicodeDecodeError:
            body_text = None

    raw_href = f"/dogfood/{dogfood_id}/{rel_path}?raw=1"
    try:
        html = ctx.jinja_env().get_template("dogfood_historical_file.html").render(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            content_type=content_type,
            sha256=str(payload.get("sha256") or ""),
            size_bytes=int(payload.get("size_bytes") or 0),
            rendered_md=rendered_md,
            body_text=body_text,
            raw_href=raw_href,
        )
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"template render failed: {exc}"))
        return
    ctx.send_html(200, html)


def serve_dogfood_file_raw(
    ctx: DogfoodHistoricalRawContext,
    dogfood_id: str,
    rel_path: str,
) -> None:
    """Serve the raw bytes of a historical-dogfood file."""

    if not _is_safe_id(dogfood_id):
        ctx.send_json(400, _error(400, "invalid dogfood_id"))
        return
    if not _is_safe_rel_path(rel_path):
        ctx.send_json(400, _error(400, "invalid rel_path"))
        return

    payload = _fetch_dogfood_blob(ctx.send_json, ctx.repo, dogfood_id, rel_path)
    if payload is None:
        return

    body = _decode_blob_body(payload, ctx.send_json)
    if body is None:
        return

    content_type = str(payload.get("content_type") or "application/octet-stream")
    ctx.send_response(200)
    ctx.send_header("Content-Type", content_type)
    ctx.send_header("Content-Length", str(len(body)))
    ctx.send_header("Content-Security-Policy", "default-src 'none'")
    ctx.send_header("Connection", "close")
    ctx.end_headers()
    try:
        ctx.write_body(body)
    except BrokenPipeError:
        return


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _fetch_dogfood_blob(
    send_json: JsonSender,
    repo: Path,
    dogfood_id: str,
    rel_path: str,
) -> dict[str, Any] | None:
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        payload = call_repo_method(
            repo,
            "corpus.fetch_historical_dogfood_file",
            {"dogfood_id": dogfood_id, "rel_path": rel_path},
        )
    except ServiceDaemonRpcError as exc:
        send_json(exc.status, _rpc_error(exc.code, exc.message))
        return None
    except Exception as exc:  # noqa: BLE001
        send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return None
    if not isinstance(payload, dict):
        send_json(500, _error(500, "daemon returned unexpected response shape"))
        return None
    return payload


def _decode_blob_body(payload: Mapping[str, Any], send_json: JsonSender) -> bytes | None:
    body_b64 = payload.get("body_base64")
    if not isinstance(body_b64, str):
        send_json(500, _error(500, "blob payload missing body_base64"))
        return None
    try:
        return base64.b64decode(body_b64, validate=True)
    except (ValueError, TypeError) as exc:
        send_json(500, _error(500, f"blob body base64 decode failed: {exc}"))
        return None


def _shape_file_row(raw: Mapping[str, Any]) -> dict[str, Any]:
    return {
        "rel_path": str(raw.get("rel_path") or ""),
        "size_bytes": int(raw.get("size_bytes") or 0),
        "last_modified": str(raw.get("last_modified") or ""),
    }


def _str_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    return [str(item) for item in value if isinstance(item, (str, int))]


def _is_safe_id(value: str) -> bool:
    if not value or "/" in value or "\x00" in value:
        return False
    if value in (".", ".."):
        return False
    return True


def _is_safe_rel_path(value: str) -> bool:
    if not value or "\x00" in value:
        return False
    if value.startswith("/") or value.startswith("./") or ".." in value.split("/"):
        return False
    return True


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


def _rpc_error(code: str, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}
