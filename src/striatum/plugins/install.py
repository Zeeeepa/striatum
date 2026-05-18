"""RFC 0025 V1 Step 1: plugin bundle install pipeline.

Mirrors :mod:`striatum.skills.install` shape but produces
agent-CLI plugin bundles (Claude Code today; Codex/Gemini in
Steps 2-3) instead of loose skill files. Each bundle is
self-describing — its ``.manifest.json`` lives inside the bundle
directory so deleting the directory uninstalls cleanly.

V1 Step 1 ships:

- ``claude_code`` profile only.
- ``--with-marketplace`` writes ``.striatum/plugins/marketplace.json``.
- Manifest at ``<bundle>/.manifest.json``.

Steps 2-3 will add ``codex`` and ``gemini`` profiles via the same
pipeline; the dispatch hook is in :func:`_build_plan`.
"""

from __future__ import annotations

import hashlib
import json
from importlib import resources
from pathlib import Path
from typing import Any, Mapping

from striatum.errors import InvalidTransitionError
from striatum.primitives import utc_now
from striatum.skills.context import gather_template_context

MANIFEST_SCHEMA_VERSION = "striatum.plugins.manifest.v1"

ALLOWED_PROFILES: frozenset[str] = frozenset({"claude_code", "codex", "gemini"})
ALLOWED_SCOPES: frozenset[str] = frozenset({"project", "user"})
ALL_PROFILES_ORDER: tuple[str, ...] = ("claude_code", "codex", "gemini")

_PROFILE_SKILLS: tuple[str, ...] = (
    "workflow",
    "scaffold",
    "claim-loop",
    "supervise",
    "recover",
    "mcp",
)
_PROFILE_COMMANDS: tuple[str, ...] = (
    "claim-next",
    "status",
    "why",
    "dashboard",
    "doctor",
)
# Backwards-compat aliases.
_CLAUDE_CODE_SKILLS = _PROFILE_SKILLS
_CLAUDE_CODE_COMMANDS = _PROFILE_COMMANDS


def install(
    *,
    target: Path,
    profile: str = "claude_code",
    scope: str = "project",
    namespace: str = "striatum",
    force: bool = False,
    dry_run: bool = False,
    with_marketplace: bool = True,
    home: Path | None = None,
) -> dict[str, Any]:
    """Render the plugin bundle for ``profile`` into ``target``."""
    if profile not in ALLOWED_PROFILES:
        allowed = ", ".join(sorted(ALLOWED_PROFILES))
        raise InvalidTransitionError(
            f"unknown plugin profile {profile!r}; expected one of: {allowed}"
        )
    if scope not in ALLOWED_SCOPES:
        allowed = ", ".join(sorted(ALLOWED_SCOPES))
        raise InvalidTransitionError(
            f"unknown plugin scope {scope!r}; expected one of: {allowed}"
        )

    bundle_root = _bundle_root(target=target, profile=profile, scope=scope, namespace=namespace, home=home)
    context = dict(gather_template_context())
    context["namespace"] = namespace
    plan = _build_plan(profile=profile, namespace=namespace, context=context)
    manifest_path = bundle_root / ".manifest.json"
    existing_manifest = _load_manifest(manifest_path)
    existing_files: dict[str, dict[str, Any]] = {}
    if existing_manifest is not None:
        for entry in existing_manifest.get("files", []):
            if isinstance(entry, dict) and isinstance(entry.get("path"), str):
                existing_files[str(entry["path"])] = entry

    file_results: list[dict[str, Any]] = []
    next_manifest_files: list[dict[str, Any]] = []

    for entry in plan:
        rel_path = entry["path"]
        rendered_text = entry["rendered"]
        rendered_sha = _sha256(rendered_text.encode("utf-8"))
        absolute = bundle_root / rel_path
        manifest_entry = existing_files.get(rel_path)
        on_disk: bytes | None = None
        if absolute.exists():
            on_disk = absolute.read_bytes()
        on_disk_sha = _sha256(on_disk) if on_disk is not None else None

        decision: str
        if on_disk is None:
            decision = "written"
        elif on_disk_sha == rendered_sha:
            decision = "skipped_unchanged"
        elif manifest_entry is None:
            decision = "written"
        else:
            recorded_sha = str(manifest_entry.get("sha256") or "")
            if on_disk_sha == recorded_sha:
                decision = "written"
            else:
                decision = "refused_modified" if not force else "written"

        if dry_run and decision == "written":
            decision = "dry_run"

        if decision == "written":
            absolute.parent.mkdir(parents=True, exist_ok=True)
            absolute.write_bytes(rendered_text.encode("utf-8"))

        if decision in ("written", "skipped_unchanged"):
            next_manifest_files.append(
                {
                    "path": rel_path,
                    "sha256": rendered_sha if decision == "written" else (on_disk_sha or rendered_sha),
                    "template": entry["template"],
                    "template_sha256": entry["template_sha256"],
                }
            )
        elif decision == "refused_modified" and manifest_entry is not None:
            next_manifest_files.append(
                {
                    "path": rel_path,
                    "sha256": str(manifest_entry.get("sha256") or rendered_sha),
                    "template": entry["template"],
                    "template_sha256": str(
                        manifest_entry.get("template_sha256") or entry["template_sha256"]
                    ),
                }
            )
        elif decision == "dry_run" and manifest_entry is not None:
            next_manifest_files.append(dict(manifest_entry))

        file_results.append(
            {
                "path": rel_path,
                "status": decision,
                "sha256_before": on_disk_sha,
                "sha256_after": rendered_sha,
            }
        )

    next_manifest_files.sort(key=lambda row: str(row["path"]))
    manifest_payload: dict[str, Any] = {
        "schema_version": MANIFEST_SCHEMA_VERSION,
        "striatum_version": context["striatum_version"],
        "generated_at": utc_now(),
        "profile": profile,
        "namespace": namespace,
        "scope": scope,
        "files": next_manifest_files,
    }

    if not dry_run:
        manifest_path.parent.mkdir(parents=True, exist_ok=True)
        manifest_path.write_text(
            json.dumps(manifest_payload, indent=2) + "\n", encoding="utf-8"
        )

    marketplace_result: dict[str, Any] | None = None
    if with_marketplace and not dry_run and scope == "project":
        if profile == "gemini":
            # Gemini has no marketplace concept (RFC § 2.3); skip
            # but surface the decision so JSON callers can detect it.
            marketplace_result = {
                "skipped": True,
                "reason": "gemini has no marketplace concept",
            }
        else:
            marketplace_result = _write_marketplace(
                target=target.resolve(), profile=profile, namespace=namespace, force=force,
            )

    return {
        "profile": profile,
        "scope": scope,
        "namespace": namespace,
        "bundle_root": str(bundle_root),
        "manifest_path": str(manifest_path),
        "files": file_results,
        "marketplace": marketplace_result,
        "dry_run": dry_run,
    }


def uninstall(
    *,
    target: Path,
    profile: str = "claude_code",
    scope: str = "project",
    namespace: str = "striatum",
    force: bool = False,
    home: Path | None = None,
) -> dict[str, Any]:
    """Remove a previously installed bundle by reading its manifest.

    Refuses to delete operator-edited files without ``--force``.
    """
    if profile not in ALLOWED_PROFILES:
        allowed = ", ".join(sorted(ALLOWED_PROFILES))
        raise InvalidTransitionError(
            f"unknown plugin profile {profile!r}; expected one of: {allowed}"
        )
    bundle_root = _bundle_root(target=target, profile=profile, scope=scope, namespace=namespace, home=home)
    manifest_path = bundle_root / ".manifest.json"
    manifest = _load_manifest(manifest_path)
    if manifest is None:
        return {"profile": profile, "status": "not_installed", "removed": []}
    removed: list[dict[str, Any]] = []
    refused: list[dict[str, Any]] = []
    for entry in manifest.get("files", []):
        rel_path = str(entry.get("path"))
        absolute = bundle_root / rel_path
        if not absolute.exists():
            continue
        recorded_sha = str(entry.get("sha256") or "")
        on_disk_sha = _sha256(absolute.read_bytes())
        if on_disk_sha != recorded_sha and not force:
            refused.append({"path": rel_path, "reason": "modified"})
            continue
        absolute.unlink()
        removed.append({"path": rel_path})
    if not refused:
        manifest_path.unlink(missing_ok=True)
        # Best-effort: walk the bundle bottom-up and remove every
        # now-empty directory.
        if bundle_root.exists():
            for path in sorted(bundle_root.rglob("*"), key=lambda p: -len(p.parts)):
                if path.is_dir():
                    try:
                        path.rmdir()
                    except (OSError, FileNotFoundError):
                        pass
            try:
                bundle_root.rmdir()
            except (OSError, FileNotFoundError):
                pass
    return {
        "profile": profile,
        "status": "uninstalled" if not refused else "refused",
        "bundle_root": str(bundle_root),
        "removed": removed,
        "refused": refused,
    }


# --- helpers --------------------------------------------------------


def _bundle_root(*, target: Path, profile: str, scope: str, namespace: str, home: Path | None) -> Path:
    if scope == "user":
        base = (home if home is not None else Path.home()).resolve()
        if profile == "claude_code":
            return base / ".claude" / "plugins" / namespace
        if profile == "codex":
            return base / ".codex" / "plugins" / namespace
        if profile == "gemini":
            return base / ".gemini" / "extensions" / namespace
        raise InvalidTransitionError(f"user-scope path undefined for profile {profile!r}")
    return target.resolve() / ".striatum" / "plugins" / profile


def _build_plan(*, profile: str, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    if profile == "claude_code":
        return _plan_claude_code(namespace=namespace, context=context)
    if profile == "codex":
        return _plan_codex(namespace=namespace, context=context)
    if profile == "gemini":
        return _plan_gemini(namespace=namespace, context=context)
    raise InvalidTransitionError(f"unsupported profile {profile!r}")


def _plan_codex(*, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    plan: list[dict[str, Any]] = []
    plan.append(_entry("codex/plugin.json.tmpl", ".codex-plugin/plugin.json", context))
    for skill in _PROFILE_SKILLS:
        plan.append(_entry(
            f"codex/skills/{skill}.md.tmpl",
            f"skills/{namespace}-{skill}/SKILL.md",
            context,
        ))
    for cmd in _PROFILE_COMMANDS:
        plan.append(_entry(
            f"codex/commands/{cmd}.md.tmpl",
            f"commands/{cmd}.md",
            context,
        ))
    plan.append(_entry("codex/hooks/hooks.json.tmpl", "hooks/hooks.json", context))
    plan.append(_entry("codex/mcp.json.tmpl", ".mcp.json", context))
    plan.append(_entry("codex/README.md.tmpl", "README.md", context))
    return plan


def _plan_gemini(*, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    plan: list[dict[str, Any]] = []
    plan.append(_entry("gemini/gemini-extension.json.tmpl", "gemini-extension.json", context))
    plan.append(_entry("gemini/GEMINI.md.tmpl", "GEMINI.md", context))
    for skill in _PROFILE_SKILLS:
        plan.append(_entry(
            f"gemini/skills/{skill}.md.tmpl",
            f"skills/{namespace}-{skill}/SKILL.md",
            context,
        ))
    for cmd in _PROFILE_COMMANDS:
        plan.append(_entry(
            f"gemini/commands/{cmd}.toml.tmpl",
            f"commands/{cmd}.toml",
            context,
        ))
    plan.append(_entry("gemini/agents/striatum-recover.md.tmpl", "agents/striatum-recover.md", context))
    plan.append(_entry("gemini/README.md.tmpl", "README.md", context))
    return plan


def _plan_claude_code(*, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    plan: list[dict[str, Any]] = []
    # plugin.json
    plan.append(_entry("claude_code/plugin.json.tmpl", ".claude-plugin/plugin.json", context))
    # 5 skills
    for skill in _CLAUDE_CODE_SKILLS:
        plan.append(_entry(
            f"claude_code/skills/{skill}.md.tmpl",
            f"skills/{namespace}-{skill}/SKILL.md",
            context,
        ))
    # 5 commands
    for cmd in _CLAUDE_CODE_COMMANDS:
        plan.append(_entry(
            f"claude_code/commands/{cmd}.md.tmpl",
            f"commands/{cmd}.md",
            context,
        ))
    # hooks, mcp, README
    plan.append(_entry("claude_code/hooks/hooks.json.tmpl", "hooks/hooks.json", context))
    plan.append(_entry("claude_code/mcp.json.tmpl", ".mcp.json", context))
    plan.append(_entry("claude_code/README.md.tmpl", "README.md", context))
    return plan


def _entry(template_rel: str, output_rel: str, context: Mapping[str, Any]) -> dict[str, Any]:
    rendered = _render(template_rel=template_rel, context=context)
    return {
        "path": output_rel,
        "rendered": rendered,
        "template": template_rel,
        "template_sha256": _bundled_template_sha256(template_rel),
    }


def _render(*, template_rel: str, context: Mapping[str, Any]) -> str:
    raw = _read_template(template_rel)
    # Reuse the skills helper expansion so skill bodies render the
    # same {verbs_*} / {boundaries_*} substitutions, then format_map
    # for {striatum_version} / {namespace}.
    from striatum.skills.install import _expand_helpers
    expanded = _expand_helpers(raw=raw, context=context)
    return expanded.format_map(_StrictFormatMap(context))


class _StrictFormatMap(dict[str, Any]):
    def __missing__(self, key: str) -> str:
        raise KeyError(key)


def _read_template(template_rel: str) -> str:
    package = "striatum.plugins.templates"
    parts = template_rel.split("/")
    base_pkg = package + "." + ".".join(parts[:-1]).replace("-", "_") if len(parts) > 1 else package
    filename = parts[-1]
    try:
        return resources.files(base_pkg).joinpath(filename).read_text(encoding="utf-8")
    except (FileNotFoundError, ModuleNotFoundError) as exc:
        raise FileNotFoundError(f"plugin template not found: {template_rel}") from exc


def _bundled_template_sha256(template_rel: str) -> str:
    return _sha256(_read_template(template_rel).encode("utf-8"))


def _sha256(data: bytes | None) -> str:
    if data is None:
        return ""
    return hashlib.sha256(data).hexdigest()


def _load_manifest(path: Path) -> dict[str, Any] | None:
    if not path.is_file():
        return None
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None
    if not isinstance(data, dict):
        return None
    return data


def _write_marketplace(*, target: Path, profile: str, namespace: str, force: bool) -> dict[str, Any]:
    """Write or merge ``.striatum/plugins/marketplace.json``.

    V1 Step 1: single profile entry. Future steps merge additional
    profiles into the same `plugins[]` list, preserving any
    operator-added entries.
    """
    path = target / ".striatum" / "plugins" / "marketplace.json"
    existing: dict[str, Any] = {
        "name": "local-striatum",
        "interface": {"displayName": "Local Striatum"},
        "plugins": [],
    }
    if path.exists():
        try:
            loaded = json.loads(path.read_text(encoding="utf-8"))
            if isinstance(loaded, dict):
                existing = loaded
        except (OSError, json.JSONDecodeError):
            pass
    raw_plugins = existing.get("plugins")
    plugins: list[Any] = list(raw_plugins) if isinstance(raw_plugins, list) else []
    new_entry: dict[str, Any] = {
        "name": namespace,
        "source": {"source": "local", "path": f"./{profile}"},
        "policy": {"installation": "AVAILABLE"},
        "category": "Developer Tools",
    }
    # Update or append based on (name, source.path) match.
    matched = False
    for i, p in enumerate(plugins):
        if (
            isinstance(p, dict)
            and p.get("name") == namespace
            and isinstance(p.get("source"), dict)
            and p["source"].get("path") == new_entry["source"]["path"]
        ):
            plugins[i] = new_entry
            matched = True
            break
    if not matched:
        plugins.append(new_entry)
    existing["plugins"] = plugins
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(existing, indent=2) + "\n", encoding="utf-8")
    return {"path": str(path), "appended" if not matched else "updated": True}
