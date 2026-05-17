"""Bundled static asset lookup for the local web service."""

from __future__ import annotations

from dataclasses import dataclass
from importlib.resources import files


@dataclass(frozen=True)
class StaticAsset:
    data: bytes
    content_type: str


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
