"""Route orchestration for the historical dogfood browse surface."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from striatum.web import dogfood_historical

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
HtmlSender = Callable[[int, str], None]
TemplateEnvFactory = Callable[[], Any]
ResponseStarter = Callable[[int], None]
HeaderSender = Callable[[str, str], None]
HeaderFinisher = Callable[[], None]
BodyWriter = Callable[[bytes], object]


@dataclass(frozen=True)
class DogfoodRouteContext:
    repo: Path
    send_json: JsonSender
    send_html: HtmlSender
    jinja_env: TemplateEnvFactory
    send_response: ResponseStarter
    send_header: HeaderSender
    end_headers: HeaderFinisher
    write_body: BodyWriter

    def page_context(self) -> dogfood_historical.DogfoodHistoricalContext:
        return dogfood_historical.DogfoodHistoricalContext(
            repo=self.repo,
            send_json=self.send_json,
            send_html=self.send_html,
            jinja_env=self.jinja_env,
        )

    def raw_context(self) -> dogfood_historical.DogfoodHistoricalRawContext:
        return dogfood_historical.DogfoodHistoricalRawContext(
            repo=self.repo,
            send_json=self.send_json,
            send_response=self.send_response,
            send_header=self.send_header,
            end_headers=self.end_headers,
            write_body=self.write_body,
        )


def render_dogfood_path(
    ctx: DogfoodRouteContext,
    rest: str,
    query: Mapping[str, list[str]],
) -> None:
    """Dispatch ``/dogfood`` and ``/dogfood/<id>/<path>`` page requests."""

    if rest in {"", "/"}:
        dogfood_historical.render_dogfood_index_page(ctx.page_context())
        return
    parts = rest.split("/", 1)
    dogfood_id = parts[0]
    if not dogfood_id:
        dogfood_historical.render_dogfood_index_page(ctx.page_context())
        return
    if len(parts) == 1 or parts[1] == "":
        dogfood_historical.render_dogfood_run_page(ctx.page_context(), dogfood_id)
        return
    rel_path = parts[1].rstrip("/")
    raw = query.get("raw", ["0"])[0] in {"1", "true", "yes"}
    if raw:
        dogfood_historical.serve_dogfood_file_raw(
            ctx.raw_context(),
            dogfood_id,
            rel_path,
        )
        return
    dogfood_historical.render_dogfood_file_page(
        ctx.page_context(),
        dogfood_id,
        rel_path,
    )


__all__ = [
    "DogfoodRouteContext",
    "render_dogfood_path",
]
