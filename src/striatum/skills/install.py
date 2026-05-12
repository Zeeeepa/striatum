"""RFC 0015 V1: install pipeline for the agent skill bundle.

The pipeline is pure I/O: render packaged templates against the
context returned by :mod:`striatum.skills.context`, compare each
rendered file against an on-disk manifest, and write or refuse
based on edit-detection rules.

V1 ships two profiles:

- ``claude_code`` — five SKILL.md files under
  ``.claude/skills/<ns>striatum-<skill>/``.
- ``generic`` — a single concatenated guide at
  ``<ns>STRIATUM_AGENT_GUIDE.md`` at the target root.

``codex`` and ``gemini`` are reserved for step 3 of the RFC's path.
"""

from __future__ import annotations

import hashlib
import json
import re
from importlib import resources
from pathlib import Path
from typing import Any, Mapping

from striatum.db import utc_now
from striatum.errors import InvalidTransitionError, NotFoundError
from striatum.skills.context import gather_template_context

MANIFEST_SCHEMA_VERSION = "striatum.skills.manifest.v1"

ALLOWED_PROFILES: frozenset[str] = frozenset(
    {"claude_code", "codex", "gemini", "generic"}
)
ALLOWED_SCOPES: frozenset[str] = frozenset({"project", "user"})

# Order in which `--profile all` fans out. Stable so install
# results are deterministic under serialization.
ALL_PROFILES_ORDER: tuple[str, ...] = (
    "claude_code",
    "codex",
    "gemini",
    "generic",
)

# Bare skill names. Directories are formed as ``<namespace><skill>``,
# so the default namespace ``striatum-`` produces directories like
# ``.claude/skills/striatum-workflow/``. Templates live at
# ``templates/claude_code/<bare>.md.tmpl``.
CLAUDE_CODE_SKILLS: tuple[str, ...] = (
    "workflow",
    "scaffold",
    "claim-loop",
    "supervise",
    "recover",
    "mcp",
)


def install(
    *,
    target: Path,
    profile: str = "claude_code",
    scope: str = "project",
    namespace: str = "striatum-",
    force: bool = False,
    dry_run: bool = False,
    home: Path | None = None,
) -> dict[str, Any]:
    """Render the skill bundle for ``profile`` into ``target``.

    Parameters mirror the CLI surface. ``home`` is the user-scope
    base directory (defaults to ``Path.home()``); tests override
    it to keep filesystem effects sandboxed.
    """
    if profile not in ALLOWED_PROFILES:
        allowed = ", ".join(sorted(ALLOWED_PROFILES))
        raise InvalidTransitionError(
            f"unknown skills profile {profile!r}; expected one of: {allowed}"
        )
    if scope not in ALLOWED_SCOPES:
        allowed = ", ".join(sorted(ALLOWED_SCOPES))
        raise InvalidTransitionError(
            f"unknown skills scope {scope!r}; expected one of: {allowed}"
        )

    base = _scope_base(target=target, scope=scope, home=home)
    context = gather_template_context()
    plan = _build_plan(profile=profile, namespace=namespace, base=base, context=context)
    manifest_path = _manifest_path(profile=profile, namespace=namespace, base=base)
    existing_manifest = load_manifest(manifest_path)
    existing_files: dict[str, dict[str, Any]] = {}
    if existing_manifest is not None:
        for entry in existing_manifest.get("files", []):
            if isinstance(entry, dict) and isinstance(entry.get("path"), str):
                existing_files[str(entry["path"])] = entry

    file_results: list[dict[str, Any]] = []
    next_manifest_files: list[dict[str, Any]] = []
    target_root = base["root"]

    for entry in plan:
        rel_path = entry["path"]
        rendered_text = entry["rendered"]
        rendered_sha = _sha256(rendered_text.encode("utf-8"))
        absolute = target_root / rel_path
        manifest_entry = existing_files.get(rel_path)
        on_disk: bytes | None = None
        if absolute.exists():
            on_disk = absolute.read_bytes()
        on_disk_sha = _sha256(on_disk) if on_disk is not None else None

        decision: str
        if on_disk is None:
            # Brand new file. Always written (no `--force` required).
            decision = "written"
        elif on_disk_sha == rendered_sha:
            decision = "skipped_unchanged"
        elif manifest_entry is None:
            # Existing file with no manifest entry → treat as new and
            # write (the operator did not opt into managed state).
            decision = "written"
        else:
            recorded_sha = str(manifest_entry.get("sha256") or "")
            if on_disk_sha == recorded_sha:
                # Manifest matches disk; we have a content change to
                # apply (template churn or version bump).
                decision = "written"
            else:
                # On-disk content has drifted from the manifest. Honor
                # operator edits unless --force is set.
                decision = "refused_modified" if not force else "written"

        if dry_run and decision in {"written"}:
            decision = "dry_run"

        if decision == "written":
            absolute.parent.mkdir(parents=True, exist_ok=True)
            absolute.write_bytes(rendered_text.encode("utf-8"))

        # Manifest reflects the post-state: if we wrote, record the
        # rendered SHA; if we refused, keep the existing manifest
        # entry intact; if dry-run, leave the manifest alone.
        if decision in {"written", "skipped_unchanged"}:
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
    manifest_payload = {
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
            json.dumps(manifest_payload, indent=2, sort_keys=False) + "\n",
            encoding="utf-8",
        )

    return {
        "profile": profile,
        "scope": scope,
        "namespace": namespace,
        "manifest_path": str(manifest_path),
        "files": file_results,
        "dry_run": dry_run,
    }


def manifest_path_for(
    *,
    target: Path,
    profile: str,
    scope: str = "project",
    namespace: str = "striatum-",
    home: Path | None = None,
) -> Path:
    """Compute the manifest path the runner would write."""
    base = _scope_base(target=target, scope=scope, home=home)
    return _manifest_path(profile=profile, namespace=namespace, base=base)


def load_manifest(path: Path) -> dict[str, Any] | None:
    """Read a manifest file or return ``None`` when missing/malformed."""
    if not path.exists():
        return None
    try:
        text = path.read_text(encoding="utf-8")
    except OSError:
        return None
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        return None
    if not isinstance(payload, dict):
        return None
    if payload.get("schema_version") != MANIFEST_SCHEMA_VERSION:
        return None
    return payload


def skill_files_present(
    *,
    target: Path,
    manifest: Mapping[str, Any],
    scope: str | None = None,
    home: Path | None = None,
) -> list[str]:
    """Return manifest paths that are missing on disk.

    The manifest's ``scope`` field is honored (unless overridden) so
    that a user-scope manifest is checked against the user-scope
    root rather than against the target repo.
    """
    effective_scope = str(scope or manifest.get("scope") or "project")
    base = _scope_base(target=target, scope=effective_scope, home=home)
    missing: list[str] = []
    for entry in manifest.get("files", []):
        if not isinstance(entry, dict):
            continue
        rel = entry.get("path")
        if not isinstance(rel, str):
            continue
        absolute = base["root"] / rel
        if not absolute.exists():
            missing.append(rel)
    return missing


def bundled_template_sha256(template_relpath: str) -> str:
    """Return the SHA256 of the packaged template at the given relpath.

    ``template_relpath`` is rooted at ``striatum.skills.templates``,
    e.g. ``claude_code/striatum-workflow.md.tmpl``.
    """
    pkg = resources.files("striatum.skills.templates")
    target_resource = pkg.joinpath(template_relpath)
    return _sha256(target_resource.read_bytes())


# ---------------------------------------------------------------------------
# internals
# ---------------------------------------------------------------------------


def _scope_base(
    *, target: Path, scope: str, home: Path | None
) -> dict[str, Any]:
    """Resolve the directory + presentation prefix for a given scope."""
    if scope == "user":
        root = (home if home is not None else Path.home()).resolve()
    else:
        root = target.resolve()
    return {"root": root, "scope": scope}


def _build_plan(
    *,
    profile: str,
    namespace: str,
    base: Mapping[str, Any],
    context: Mapping[str, Any],
) -> list[dict[str, Any]]:
    if profile == "claude_code":
        return _plan_claude_code(namespace=namespace, context=context)
    if profile == "codex":
        return _plan_codex(namespace=namespace, context=context)
    if profile == "gemini":
        return _plan_gemini(namespace=namespace, context=context)
    if profile == "generic":
        return _plan_generic(namespace=namespace, context=context)
    raise InvalidTransitionError(f"unsupported profile {profile!r}")


def _plan_codex(*, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    """Codex CLI: flat agent docs at ``.codex/agents/<ns><skill>.md``.

    Reuses the ``claude_code`` Markdown bodies verbatim — the
    skill content is identical; only the destination layout
    differs. The manifest records the same ``template_sha256``
    as the ``claude_code`` profile would.
    """
    plan: list[dict[str, Any]] = []
    for skill in CLAUDE_CODE_SKILLS:
        template_rel = f"claude_code/{skill}.md.tmpl"
        rendered = _render(template_rel=template_rel, context=context)
        plan.append(
            {
                "path": f".codex/agents/{namespace}{skill}.md",
                "rendered": rendered,
                "template": template_rel,
                "template_sha256": bundled_template_sha256(template_rel),
            }
        )
    return plan


def _plan_gemini(*, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    """Gemini CLI: single concatenated guide at the repo root.

    RFC 0015 § "Profile coverage" permits the generic-shape
    fallback for V1 until Gemini CLI's skill-discovery convention
    stabilizes; we use a distinct filename
    (``STRIATUM_GEMINI_GUIDE.md``) so ``--profile all`` does not
    collide with the ``generic`` profile.
    """
    template_rel = "gemini/STRIATUM_GEMINI_GUIDE.md.tmpl"
    rendered = _render(template_rel=template_rel, context=context)
    return [
        {
            "path": f"{namespace}STRIATUM_GEMINI_GUIDE.md",
            "rendered": rendered,
            "template": template_rel,
            "template_sha256": bundled_template_sha256(template_rel),
        }
    ]


def _plan_claude_code(*, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    plan: list[dict[str, Any]] = []
    for skill in CLAUDE_CODE_SKILLS:
        template_rel = f"claude_code/{skill}.md.tmpl"
        rendered = _render(template_rel=template_rel, context=context)
        plan.append(
            {
                # Namespace prepends to the bare skill name so the
                # default ``striatum-`` produces ``striatum-workflow``,
                # ``striatum-scaffold``, etc. Operator override via
                # ``--namespace`` keeps the directories collision-free.
                "path": f".claude/skills/{namespace}{skill}/SKILL.md",
                "rendered": rendered,
                "template": template_rel,
                "template_sha256": bundled_template_sha256(template_rel),
            }
        )
    return plan


def _plan_generic(*, namespace: str, context: Mapping[str, Any]) -> list[dict[str, Any]]:
    template_rel = "generic/STRIATUM_AGENT_GUIDE.md.tmpl"
    rendered = _render(template_rel=template_rel, context=context)
    return [
        {
            "path": f"{namespace}STRIATUM_AGENT_GUIDE.md",
            "rendered": rendered,
            "template": template_rel,
            "template_sha256": bundled_template_sha256(template_rel),
        }
    ]


def _manifest_path(
    *, profile: str, namespace: str, base: Mapping[str, Any]
) -> Path:
    root = Path(base["root"])
    if profile == "claude_code":
        return (
            root
            / ".claude"
            / "skills"
            / f"{namespace}workflow"
            / ".manifest.json"
        )
    if profile == "codex":
        return root / ".codex" / "agents" / f"{namespace}workflow.manifest.json"
    if profile == "gemini":
        return root / f"{namespace}STRIATUM_GEMINI_GUIDE.manifest.json"
    if profile == "generic":
        return root / f"{namespace}STRIATUM_AGENT_GUIDE.manifest.json"
    raise InvalidTransitionError(f"unsupported profile {profile!r}")


def _render(*, template_rel: str, context: Mapping[str, Any]) -> str:
    raw = _read_template(template_rel)
    expanded = _expand_helpers(raw=raw, context=context)
    try:
        return expanded.format_map(_StrictFormatMap(context))
    except KeyError as exc:
        missing = exc.args[0] if exc.args else "<unknown>"
        raise NotFoundError(
            f"template {template_rel!r} references unknown context key {missing!r}"
        ) from exc


def _read_template(template_rel: str) -> str:
    pkg = resources.files("striatum.skills.templates")
    return pkg.joinpath(template_rel).read_text(encoding="utf-8")


def _expand_helpers(*, raw: str, context: Mapping[str, Any]) -> str:
    """Expand the small set of helper placeholders the templates use.

    Helpers are expanded *before* ``str.format_map`` so the result
    is a plain template with only ``{striatum_version}`` left for
    the formatter. This keeps the renderer's surface deliberately
    small.
    """
    verbs = context.get("verbs") or {}
    boundaries = context.get("boundaries") or ()
    front_matter_kinds = context.get("front_matter_kinds") or ()

    def render_verbs(group: str) -> str:
        rows = (verbs.get(group) or {}) if isinstance(verbs, Mapping) else {}
        if not rows:
            return "_(no verbs registered)_"
        # Preserve the curated insertion order from VERB_TABLE.
        lines = []
        for verb, summary in rows.items():
            lines.append(f"- `striatum {verb}` — {summary}")
        return "\n".join(lines)

    def render_bullets(values: Any) -> str:
        if not isinstance(values, (list, tuple)):
            return ""
        return "\n".join(f"- {value}" for value in values)

    out = raw
    out = out.replace("{verbs_scaffold}", render_verbs("scaffold"))
    out = out.replace("{verbs_claim_loop}", render_verbs("claim_loop"))
    out = out.replace("{verbs_supervise}", render_verbs("supervise"))
    out = out.replace("{verbs_recover}", render_verbs("recover"))
    out = out.replace("{boundaries_bulletlist}", render_bullets(boundaries))
    out = out.replace(
        "{front_matter_kinds_list}", render_bullets(front_matter_kinds)
    )
    return out


class _StrictFormatMap(dict[str, Any]):
    """``dict`` subclass that raises ``KeyError`` for unknown keys.

    ``str.format_map`` swallows missing keys when given a custom
    mapping; subclassing ``dict`` and overriding ``__missing__`` is
    the only way to keep "unknown placeholder" loud.
    """

    def __missing__(self, key: str) -> str:  # noqa: D401 - dict hook
        raise KeyError(key)


def _sha256(payload: bytes | None) -> str:
    if payload is None:
        return ""
    return hashlib.sha256(payload).hexdigest()


_URL_RE = re.compile(r"https?://", re.IGNORECASE)


def template_contains_url(text: str) -> bool:
    """Whether ``text`` contains an HTTP(S) URL (used by tests)."""
    return bool(_URL_RE.search(text))
