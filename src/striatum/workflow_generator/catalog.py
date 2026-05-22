"""Bundled workflow template catalog loader."""

from __future__ import annotations

import json
from functools import lru_cache
from importlib.resources import files
from typing import Any

from striatum.primitives import JsonObject
from striatum.workflow import HARNESS_PROFILE_TOOL_FAMILIES
from striatum.workflow_generator.core import GeneratorError

CATALOG_SCHEMA_VERSION = "striatum.workflow_templates.v1"
TEMPLATE_COLLECTIONS = {
    "shape": "shapes",
    "lane_set": "lane_sets",
    "role_pack": "role_packs",
    "adversary_pack": "adversary_packs",
}
MULTI_PHASE_SHAPE_ENTRY: JsonObject = {
    "template_id": "multi_phase",
    "kind": "shape",
    "display_name": "Multi-phase workflow",
    "summary": "Run phase-scoped parallel tracks behind explicit synthesis gates.",
    "recommended_for": ["large work split into ordered design, build, review, or release phases"],
    "default_lane_sets": ["author_reviewer", "multi_review", "single_agent"],
    "required_options": ["workflow_id", "artifact_root", "phases"],
    "graph_preview": {
        "nodes": [
            {"id": "phase_1_track", "label": "Phase 1 track"},
            {"id": "phase_1__synthesis", "label": "Phase 1 synthesis"},
            {"id": "phase_2_track", "label": "Phase 2 track"},
            {"id": "phase_2__synthesis", "label": "Phase 2 synthesis"},
        ],
        "edges": [
            {"from": "phase_1_track", "to": "phase_1__synthesis"},
            {"from": "phase_1__synthesis", "to": "phase_2_track"},
            {"from": "phase_2_track", "to": "phase_2__synthesis"},
        ],
        "cycles": [],
    },
}


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
    for expected_kind, key in TEMPLATE_COLLECTIONS.items():
        entries = catalog.get(key, [])
        if key in {"shapes", "lane_sets"} and not isinstance(catalog.get(key), list):
            raise GeneratorError(f"workflow template catalog {key} must be a list")
        if not isinstance(entries, list):
            raise GeneratorError(f"workflow template catalog {key} must be a list")
        for index, entry in enumerate(entries):
            if not isinstance(entry, dict):
                raise GeneratorError(f"workflow template catalog {key}[{index}] must be an object")
            if not isinstance(entry.get("template_id"), str) or not entry["template_id"]:
                raise GeneratorError(f"workflow template catalog {key}[{index}] missing template_id")
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
    _ensure_multi_phase_shape(catalog)
    return catalog


def list_templates(kind: str | None = None) -> list[JsonObject]:
    """Return sorted catalog entries, optionally filtered by kind."""
    if kind not in {None, *TEMPLATE_COLLECTIONS}:
        raise GeneratorError(
            "template kind must be shape, lane_set, role_pack, or adversary_pack",
            field_path="kind",
            hint=(
                "Use `workflow templates list --kind shape`, `--kind lane_set`, "
                "`--kind role_pack`, or `--kind adversary_pack`."
            ),
        )
    catalog = load_catalog()
    entries: list[JsonObject] = []
    for candidate_kind, key in TEMPLATE_COLLECTIONS.items():
        if kind in {None, candidate_kind}:
            entries.extend(_copy_entries(catalog.get(key, [])))
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


def _ensure_multi_phase_shape(catalog: JsonObject) -> None:
    shapes = catalog["shapes"]
    if not any(isinstance(entry, dict) and entry.get("template_id") == "multi_phase" for entry in shapes):
        shapes.append(MULTI_PHASE_SHAPE_ENTRY)
