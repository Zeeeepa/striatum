"""Bundled static asset lookup for the local web service."""

from __future__ import annotations

from dataclasses import dataclass
from importlib.resources import files
from typing import Any, Callable


@dataclass(frozen=True)
class StaticAsset:
    data: bytes
    content_type: str


JsonObject = dict[str, Any]


@dataclass(frozen=True)
class StaticAssetRouteContext:
    send_json: Callable[[int, JsonObject], None]
    send_response: Callable[[int], None]
    send_header: Callable[[str, str], None]
    end_headers: Callable[[], None]
    write_body: Callable[[bytes], object]


class StaticAssetError(Exception):
    def __init__(self, status_code: int, message: str) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.message = message


def load_static_asset(relative: str) -> StaticAsset:
    if not relative or ".." in relative or relative.startswith("/"):
        raise StaticAssetError(400, "invalid asset path")
    try:
        asset = files("striatum.web.static").joinpath(relative)
        if not asset.is_file():
            raise StaticAssetError(404, "asset not found")
        data = asset.read_bytes()
    except StaticAssetError:
        raise
    except (FileNotFoundError, ModuleNotFoundError, OSError) as exc:
        raise StaticAssetError(404, "asset not found") from exc
    return StaticAsset(data=data, content_type=content_type_for_path(relative))


CONTENT_SECURITY_POLICY = (
    "default-src 'self'; script-src 'self'; style-src 'self'; "
    "img-src 'self' data:; connect-src 'self'"
)


def serve_static_asset(ctx: StaticAssetRouteContext, relative: str) -> None:
    try:
        asset = load_static_asset(relative)
    except StaticAssetError as exc:
        ctx.send_json(
            exc.status_code,
            {"ok": False, "error": {"code": exc.status_code, "message": exc.message}},
        )
        return
    ctx.send_response(200)
    ctx.send_header("Content-Type", asset.content_type)
    ctx.send_header("Content-Length", str(len(asset.data)))
    ctx.send_header("Content-Security-Policy", CONTENT_SECURITY_POLICY)
    ctx.send_header("Connection", "close")
    ctx.end_headers()
    try:
        ctx.write_body(asset.data)
    except BrokenPipeError:
        return


def content_type_for_path(relative: str) -> str:
    suffix = relative.rsplit(".", 1)[-1].lower()
    return {
        "html": "text/html; charset=utf-8",
        "css": "text/css; charset=utf-8",
        "js": "application/javascript; charset=utf-8",
        "json": "application/json",
        "svg": "image/svg+xml",
        "png": "image/png",
    }.get(suffix, "application/octet-stream")
