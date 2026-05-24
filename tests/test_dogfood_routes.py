from __future__ import annotations

from pathlib import Path

import pytest

from striatum.web import dogfood_historical
from striatum.web.dogfood_routes import DogfoodRouteContext, render_dogfood_path


def _ctx(tmp_path: Path) -> DogfoodRouteContext:
    return DogfoodRouteContext(
        repo=tmp_path,
        send_json=lambda _status, _body: None,
        send_html=lambda _status, _body: None,
        jinja_env=lambda: object(),
        send_response=lambda _status: None,
        send_header=lambda _name, _value: None,
        end_headers=lambda: None,
        write_body=lambda _body: None,
    )


def test_render_dogfood_path_dispatches_index_run_and_file(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[str, str, str | None]] = []

    def index(ctx: dogfood_historical.DogfoodHistoricalContext) -> None:
        calls.append(("index", str(ctx.repo), None))

    def run(ctx: dogfood_historical.DogfoodHistoricalContext, dogfood_id: str) -> None:
        calls.append(("run", str(ctx.repo), dogfood_id))

    def file_page(
        ctx: dogfood_historical.DogfoodHistoricalContext,
        dogfood_id: str,
        rel_path: str,
    ) -> None:
        calls.append(("file", dogfood_id, rel_path))

    monkeypatch.setattr(dogfood_historical, "render_dogfood_index_page", index)
    monkeypatch.setattr(dogfood_historical, "render_dogfood_run_page", run)
    monkeypatch.setattr(dogfood_historical, "render_dogfood_file_page", file_page)

    ctx = _ctx(tmp_path)
    render_dogfood_path(ctx, "", {})
    render_dogfood_path(ctx, "046", {})
    render_dogfood_path(ctx, "046/BUILD.md", {})

    assert calls == [
        ("index", str(tmp_path), None),
        ("run", str(tmp_path), "046"),
        ("file", "046", "BUILD.md"),
    ]


def test_render_dogfood_path_dispatches_raw_file(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[str, str, str]] = []

    def raw(
        ctx: dogfood_historical.DogfoodHistoricalRawContext,
        dogfood_id: str,
        rel_path: str,
    ) -> None:
        calls.append(("raw", dogfood_id, rel_path))

    monkeypatch.setattr(dogfood_historical, "serve_dogfood_file_raw", raw)

    render_dogfood_path(_ctx(tmp_path), "046/BUILD.md", {"raw": ["1"]})

    assert calls == [("raw", "046", "BUILD.md")]
