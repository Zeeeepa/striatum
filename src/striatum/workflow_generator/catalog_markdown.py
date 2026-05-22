"""Markdown renderer for the bundled workflow template catalog."""

from __future__ import annotations

import hashlib
import os
from pathlib import Path
from typing import Any, Mapping

from striatum.primitives import JsonObject
from striatum.workflow_generator.catalog import list_templates
from striatum.workflow_generator.core import GeneratorError


def render_catalog_markdown() -> str:
    """Render the bundled workflow template catalog as Markdown."""
    shapes = list_templates(kind="shape")
    lane_sets = list_templates(kind="lane_set")
    role_packs = list_templates(kind="role_pack")
    adversary_packs = list_templates(kind="adversary_pack")

    lines: list[str] = [
        "# Workflow Template Catalog",
        "",
        "Generated from the bundled Striatum workflow template catalog.",
        "",
        "## Workflow Shapes",
        "",
    ]
    for entry in shapes:
        lines.extend(_shape_section(entry))
        lines.append("")

    lines.extend(["## Lane Sets", ""])
    for entry in lane_sets:
        lines.extend(_lane_set_section(entry))
        lines.append("")

    if role_packs:
        lines.extend(["## Role Packs", ""])
        for entry in role_packs:
            lines.extend(_role_pack_section(entry))
            lines.append("")

    if adversary_packs:
        lines.extend(["## Adversary Packs", ""])
        for entry in adversary_packs:
            lines.extend(_adversary_pack_section(entry))
            lines.append("")

    return "\n".join(lines).rstrip() + "\n"


def write_catalog_markdown(
    path: Path,
    *,
    repo: Path,
    force: bool = False,
    check: bool = False,
) -> JsonObject:
    """Write the catalog Markdown file inside *repo*."""
    markdown = render_catalog_markdown()
    digest = hashlib.sha256(markdown.encode("utf-8")).hexdigest()
    target, relative = _safe_markdown_target(repo, path)
    exists = target.exists()
    current = target.read_text(encoding="utf-8") if exists else None
    if current == markdown:
        return {
            "status": "up_to_date",
            "path": str(target),
            "relative_path": relative,
            "sha256": digest,
        }
    if check:
        return {
            "status": "stale" if exists else "missing",
            "path": str(target),
            "relative_path": relative,
            "sha256": digest,
        }
    if exists and not force:
        raise GeneratorError(
            "workflow templates render-md refuses to overwrite an existing Markdown file",
            field_path="path",
            hint="rerun with --force to update the file, or --check to test for drift",
        )

    target.parent.mkdir(parents=True, exist_ok=True)
    tmp = target.with_name(f".{target.name}.tmp.{os.getpid()}")
    try:
        tmp.write_text(markdown, encoding="utf-8")
        os.replace(tmp, target)
    finally:
        try:
            tmp.unlink()
        except FileNotFoundError:
            pass
    return {
        "status": "updated" if exists else "created",
        "path": str(target),
        "relative_path": relative,
        "sha256": digest,
    }


def _shape_section(entry: JsonObject) -> list[str]:
    template_id = _required_str(entry, "template_id", "template")
    display_name = _required_str(entry, "display_name", template_id)
    summary = _optional_str(entry, "summary")
    lines = [f"### {display_name} (`{template_id}`)", ""]
    if summary:
        lines.extend([summary, ""])
    lines.extend(_metadata_lines(entry, keys=("recommended_for", "default_lane_sets", "required_options")))

    preview = entry.get("graph_preview")
    if isinstance(preview, Mapping):
        mermaid = render_graph_preview_mermaid(preview, field_path=f"{template_id}.graph_preview")
        if mermaid is None:
            lines.extend(["No fixed graph preview.", ""])
        else:
            lines.extend(["```mermaid", mermaid.rstrip("\n"), "```", ""])
    return _strip_trailing_blank(lines)


def _lane_set_section(entry: JsonObject) -> list[str]:
    template_id = _required_str(entry, "template_id", "template")
    display_name = _required_str(entry, "display_name", template_id)
    summary = _optional_str(entry, "summary")
    lines = [f"### {display_name} (`{template_id}`)", ""]
    if summary:
        lines.extend([summary, ""])
    lines.extend(_metadata_lines(entry, keys=("recommended_for", "required_options")))
    return _strip_trailing_blank(lines)


def _role_pack_section(entry: JsonObject) -> list[str]:
    template_id = _required_str(entry, "template_id", "template")
    display_name = _required_str(entry, "display_name", template_id)
    summary = _optional_str(entry, "summary")
    lines = [f"### {display_name} (`{template_id}`)", ""]
    if summary:
        lines.extend([summary, ""])
    lines.extend(_metadata_lines(entry, keys=("recommended_for", "default_shapes", "roles")))
    return _strip_trailing_blank(lines)


def _adversary_pack_section(entry: JsonObject) -> list[str]:
    template_id = _required_str(entry, "template_id", "template")
    display_name = _required_str(entry, "display_name", template_id)
    summary = _optional_str(entry, "summary")
    lines = [f"### {display_name} (`{template_id}`)", ""]
    if summary:
        lines.extend([summary, ""])
    lines.extend(_metadata_lines(entry, keys=("recommended_for", "default_shapes", "postures")))
    return _strip_trailing_blank(lines)


def _metadata_lines(entry: JsonObject, *, keys: tuple[str, ...]) -> list[str]:
    lines: list[str] = []
    labels = {
        "recommended_for": "Recommended for",
        "default_lane_sets": "Default lane sets",
        "required_options": "Required options",
        "default_shapes": "Default shapes",
        "roles": "Roles",
        "postures": "Postures",
    }
    for key in keys:
        values = _string_list(entry.get(key), key)
        if not values:
            continue
        if key == "recommended_for":
            rendered = "; ".join(values)
        else:
            rendered = ", ".join(f"`{value}`" for value in values)
        lines.append(f"- {labels[key]}: {rendered}")
    if lines:
        lines.append("")
    return lines


def render_graph_preview_mermaid(
    graph_preview: Mapping[str, Any],
    *,
    field_path: str = "graph_preview",
) -> str | None:
    """Render one catalog ``graph_preview`` object as a Mermaid flowchart."""
    nodes = _preview_object_list(graph_preview, "nodes", field_path)
    if not nodes:
        return None
    edges = _preview_object_list(graph_preview, "edges", field_path)
    cycles = _preview_object_list(graph_preview, "cycles", field_path)

    node_names: dict[str, str] = {}
    for index, node in enumerate(nodes):
        node_id = _required_str(node, "id", f"{field_path}.nodes[{index}]")
        if node_id in node_names:
            raise GeneratorError(
                f"workflow template catalog {field_path}.nodes[{index}] duplicates id {node_id!r}",
                field_path=f"{field_path}.nodes[{index}].id",
            )
        node_names[node_id] = f"n{index}"

    lines = ["flowchart TD"]
    for index, node in enumerate(nodes):
        node_id = _required_str(node, "id", f"{field_path}.nodes[{index}]")
        label = _optional_str(node, "label") or node_id
        lines.append(f'  {node_names[node_id]}["{_mermaid_label(label)}"]')
    for index, edge in enumerate(edges):
        from_id = _required_str(edge, "from", f"{field_path}.edges[{index}]")
        to_id = _required_str(edge, "to", f"{field_path}.edges[{index}]")
        _require_known_node(from_id, node_names, f"{field_path}.edges[{index}].from")
        _require_known_node(to_id, node_names, f"{field_path}.edges[{index}].to")
        edge_label = _optional_str(edge, "label")
        if edge_label:
            lines.append(
                f"  {node_names[from_id]} -->|{_mermaid_label(edge_label)}| {node_names[to_id]}"
            )
        else:
            lines.append(f"  {node_names[from_id]} --> {node_names[to_id]}")
    for index, cycle in enumerate(cycles):
        from_id = _required_str(cycle, "from", f"{field_path}.cycles[{index}]")
        to_id = _required_str(cycle, "to", f"{field_path}.cycles[{index}]")
        _require_known_node(from_id, node_names, f"{field_path}.cycles[{index}].from")
        _require_known_node(to_id, node_names, f"{field_path}.cycles[{index}].to")
        label = _cycle_label(cycle)
        lines.append(f"  {node_names[from_id]} -.->|{_mermaid_label(label)}| {node_names[to_id]}")
    return "\n".join(lines) + "\n"


def _preview_object_list(
    graph_preview: Mapping[str, Any],
    key: str,
    field_path: str,
) -> list[Mapping[str, Any]]:
    raw = graph_preview.get(key, [])
    if not isinstance(raw, list):
        raise GeneratorError(
            f"workflow template catalog {field_path}.{key} must be a list",
            field_path=f"{field_path}.{key}",
        )
    objects: list[Mapping[str, Any]] = []
    for index, item in enumerate(raw):
        if not isinstance(item, Mapping):
            raise GeneratorError(
                f"workflow template catalog {field_path}.{key}[{index}] must be an object",
                field_path=f"{field_path}.{key}[{index}]",
            )
        objects.append(item)
    return objects


def _cycle_label(cycle: Mapping[str, Any]) -> str:
    label = _optional_str(cycle, "label")
    if label:
        return label
    on_verdict = _optional_str(cycle, "on_verdict")
    max_iterations = cycle.get("max_iterations")
    if on_verdict and isinstance(max_iterations, int) and not isinstance(max_iterations, bool):
        return f"{on_verdict} max {max_iterations}"
    if on_verdict:
        return on_verdict
    return "cycle"


def _safe_markdown_target(repo: Path, path: Path) -> tuple[Path, str]:
    repo = repo.resolve()
    raw = str(path)
    if "\x00" in raw:
        raise GeneratorError("Markdown output path cannot contain NUL bytes", field_path="path")
    candidate = path.expanduser()
    if not candidate.is_absolute():
        candidate = repo / candidate
    target = candidate.resolve()
    if target.suffix.lower() != ".md":
        raise GeneratorError("Markdown output path must end with .md", field_path="path")
    try:
        relative = target.relative_to(repo)
    except ValueError as exc:
        raise GeneratorError("Markdown output path must stay inside the repository", field_path="path") from exc
    if any(part in {".git", ".striatum"} for part in relative.parts):
        raise GeneratorError("Markdown output path cannot be inside .git or .striatum", field_path="path")
    if target.exists() and not target.is_file():
        raise GeneratorError("Markdown output path exists and is not a regular file", field_path="path")
    return target, relative.as_posix()


def _require_known_node(node_id: str, node_names: Mapping[str, str], field_path: str) -> None:
    if node_id not in node_names:
        raise GeneratorError(
            f"workflow template catalog graph preview references unknown node {node_id!r}",
            field_path=field_path,
        )


def _required_str(entry: Mapping[str, Any], key: str, field_path: str) -> str:
    value = entry.get(key)
    if not isinstance(value, str) or value == "":
        raise GeneratorError(
            f"workflow template catalog {field_path}.{key} must be a non-empty string",
            field_path=f"{field_path}.{key}",
        )
    return value


def _optional_str(entry: Mapping[str, Any], key: str) -> str | None:
    value = entry.get(key)
    if isinstance(value, str) and value != "":
        return value
    return None


def _string_list(value: Any, field_path: str) -> list[str]:
    if value is None:
        return []
    if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
        raise GeneratorError(
            f"workflow template catalog {field_path} must be a list of strings",
            field_path=field_path,
        )
    return value


def _mermaid_label(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')


def _strip_trailing_blank(lines: list[str]) -> list[str]:
    while lines and lines[-1] == "":
        lines.pop()
    return lines
