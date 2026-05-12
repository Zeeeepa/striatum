"""Bundled workflow template catalog loader."""

from __future__ import annotations

import json
from functools import lru_cache
from importlib.resources import files
from typing import Any

from striatum.db import JsonObject
from striatum.workflow_generator.core import GeneratorError

CATALOG_SCHEMA_VERSION = "striatum.workflow_templates.v1"


@lru_cache(maxsize=1)
def load_catalog() -> JsonObject:
    """Load and validate the package-data workflow template catalog."""
    try:
        raw = files("striatum.workflow_templates").joinpath("catalog.json").read_text(encoding="utf-8")
        catalog = json.loads(raw)
    except Exception as exc:  # noqa: BLE001
        raise GeneratorError("workflow template catalog could not be loaded") from exc
    if not isinstance(catalog, dict):
        raise GeneratorError("workflow template catalog root must be an object")
    if catalog.get("schema_version") != CATALOG_SCHEMA_VERSION:
        raise GeneratorError("workflow template catalog has unsupported schema_version")
    for key in ("shapes", "lane_sets"):
        entries = catalog.get(key)
        if not isinstance(entries, list):
            raise GeneratorError(f"workflow template catalog {key} must be a list")
        for index, entry in enumerate(entries):
            if not isinstance(entry, dict):
                raise GeneratorError(f"workflow template catalog {key}[{index}] must be an object")
            if not isinstance(entry.get("template_id"), str) or not entry["template_id"]:
                raise GeneratorError(f"workflow template catalog {key}[{index}] missing template_id")
            expected_kind = "shape" if key == "shapes" else "lane_set"
            if entry.get("kind") != expected_kind:
                raise GeneratorError(f"workflow template catalog {key}[{index}] has wrong kind")
    return catalog


def list_templates(kind: str | None = None) -> list[JsonObject]:
    """Return sorted catalog entries, optionally filtered by kind."""
    if kind not in {None, "shape", "lane_set"}:
        raise GeneratorError(
            "template kind must be shape or lane_set",
            field_path="kind",
            hint="Use `workflow templates list --kind shape` or `--kind lane_set`.",
        )
    catalog = load_catalog()
    entries: list[JsonObject] = []
    if kind in {None, "shape"}:
        entries.extend(_copy_entries(catalog["shapes"]))
    if kind in {None, "lane_set"}:
        entries.extend(_copy_entries(catalog["lane_sets"]))
    return sorted(entries, key=lambda item: (str(item["kind"]), str(item["template_id"])))


def get_template(template_id: str) -> JsonObject:
    """Return one catalog entry by id."""
    for entry in list_templates():
        if entry["template_id"] == template_id:
            return entry
    raise GeneratorError(
        f"unknown workflow template: {template_id}",
        field_path="template_id",
        hint="Run `striatum workflow templates list` to see available templates.",
    )


def _copy_entries(entries: Any) -> list[JsonObject]:
    return [dict(entry) for entry in entries]

