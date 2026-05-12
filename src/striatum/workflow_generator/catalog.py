"""Bundled workflow template catalog loader."""

from __future__ import annotations

import json
from functools import lru_cache
from importlib.resources import files
from typing import Any

from striatum.db import JsonObject
from striatum.workflow import HARNESS_PROFILE_TOOL_FAMILIES
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
    fragments = catalog.get("harness_profile_fragments", [])
    if not isinstance(fragments, list):
        raise GeneratorError("workflow template catalog harness_profile_fragments must be a list")
    seen_ids: set[str] = set()
    for index, fragment in enumerate(fragments):
        if not isinstance(fragment, dict):
            raise GeneratorError(f"harness_profile_fragments[{index}] must be an object")
        profile_id = fragment.get("profile_id")
        if not isinstance(profile_id, str) or not profile_id:
            raise GeneratorError(f"harness_profile_fragments[{index}] missing profile_id")
        if profile_id in seen_ids:
            raise GeneratorError(f"harness_profile_fragments[{index}] duplicate profile_id {profile_id!r}")
        seen_ids.add(profile_id)
        if fragment.get("kind") != "harness_profile_fragment":
            raise GeneratorError(f"harness_profile_fragments[{index}] has wrong kind")
        family = fragment.get("tool_family")
        if family not in HARNESS_PROFILE_TOOL_FAMILIES:
            raise GeneratorError(
                f"harness_profile_fragments[{index}] has unknown tool_family {family!r}; "
                f"expected one of {sorted(HARNESS_PROFILE_TOOL_FAMILIES)!r}"
            )
        instruction = fragment.get("native_delegation_instruction")
        if not isinstance(instruction, str) or not instruction.strip():
            raise GeneratorError(
                f"harness_profile_fragments[{index}] missing non-empty native_delegation_instruction"
            )
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


def list_harness_fragments() -> list[JsonObject]:
    """Return the catalog's per-model harness-profile fragments (RFC 0040 §5/§6)."""
    catalog = load_catalog()
    return _copy_entries(catalog.get("harness_profile_fragments", []))


def get_harness_fragment(profile_id: str) -> JsonObject:
    """Return one harness-profile fragment by ``profile_id``."""
    for entry in list_harness_fragments():
        if entry["profile_id"] == profile_id:
            return entry
    raise GeneratorError(
        f"unknown harness profile fragment: {profile_id}",
        field_path="profile_id",
        hint=(
            "Bundled fragments: claude_code_default, codex_default, "
            "gemini_default, generic_default."
        ),
    )


def get_harness_fragment_by_tool_family(tool_family: str) -> JsonObject | None:
    """Return the default fragment for ``tool_family``, or None when absent."""
    for entry in list_harness_fragments():
        if entry.get("tool_family") == tool_family:
            return entry
    return None


def _copy_entries(entries: Any) -> list[JsonObject]:
    return [dict(entry) for entry in entries]

