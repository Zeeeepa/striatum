"""Scaffold optional target-repository layouts.

Public surface:

- :func:`scaffold_ddd_layout` lays the seven canonical DDD docs
  into a target repository's ``docs/`` tree. Existing files are
  preserved by default; non-file targets (directories, broken
  symlinks) surface as per-file errors instead of being silently
  skipped. V1.5 adds ``force`` (overwrite existing regular files)
  and ``dry_run`` (preview-only, no filesystem mutation).
- :func:`scaffold_striatum_layout` lays down the RFC 0056 consumer
  repository directories without writing workflow files or gitignore
  policy.
"""

from __future__ import annotations

import hashlib
import re
from importlib import resources
from pathlib import Path
from typing import Any

__all__ = ["scaffold_ddd_layout", "scaffold_striatum_layout"]

_SAFE_WORKFLOW_SLUG = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")


# Per Finding 5 in dogfood-017's design review, templates live in
# real subdirectories under ``ddd_layout/`` — the skill-bundle
# precedent shows this works fine with setuptools package-data
# patterns. The map is (template-relative-source -> repo-relative
# target). Add new entries by updating this dict and shipping a
# matching template file.
_DDD_LAYOUT_TEMPLATES: dict[str, str] = {
    "SPEC.md.tmpl": "docs/SPEC.md",
    "PRD.md.tmpl": "docs/PRD.md",
    "DECISION_LOG.md.tmpl": "docs/DECISION_LOG.md",
    "UBIQUITOUS_LANGUAGE.md.tmpl": "docs/UBIQUITOUS_LANGUAGE.md",
    "DDD.md.tmpl": "docs/DDD.md",
    "rfcs/README.md.tmpl": "docs/rfcs/README.md",
    "rfcs/0001-template.md.tmpl": "docs/rfcs/0001-template.md",
}


def scaffold_ddd_layout(
    repo: Path,
    *,
    force: bool = False,
    dry_run: bool = False,
) -> dict[str, Any]:
    """Copy the ``ddd_layout`` templates into the target repo.

    Parameters
    ----------
    repo:
        The target repository root. Templates are written under
        ``<repo>/docs/`` per the per-template mapping.
    force:
        V1.5: when True, an existing regular-file target is
        *overwritten* with the template body. The envelope entry
        records ``status: "overwritten"`` plus a ``prior_sha256``
        field naming what was clobbered. Non-regular-file targets
        (directories, broken symlinks) still surface as
        ``status: "error"`` regardless of force — the operator has
        to resolve those manually.
    dry_run:
        V1.5: when True, no filesystem mutation happens. The
        envelope reports what *would* happen via the ``would_*``
        status vocabulary (``would_create``, ``would_skip``,
        ``would_overwrite``, ``would_error``) and sets the top-
        level ``dry_run`` field to True. Combine with ``force=True``
        to preview destructive overwrites.

    Returns
    -------
    dict
        A JSON-serializable envelope of shape::

            {
              "layout": "ddd",
              "files": [
                {"path": "docs/SPEC.md", "status": "created"},
                {"path": "docs/DECISION_LOG.md",
                 "status": "skipped", "reason": "exists"},
                ...
              ],
              "dry_run": False,
            }

        Per-file ``status`` values:

        - ``"created"`` — the file did not exist; the template
          was written.
        - ``"skipped"`` — the target file already exists; on-disk
          content unchanged. Reason: ``"exists"``.
        - ``"overwritten"`` (V1.5, force=True) — existing regular
          file replaced with the template body. Includes
          ``prior_sha256``.
        - ``"error"`` — the target exists but is not a regular
          file (directory, broken symlink, etc.) OR the write
          failed with an OS error. Reason names the cause. The
          target is **not** modified.
        - ``"would_create"`` / ``"would_skip"`` /
          ``"would_overwrite"`` / ``"would_error"`` (V1.5,
          dry_run=True) — preview-only counterparts of the above.
          A ``dry_run=True`` envelope contains only ``would_*``
          values; a ``dry_run=False`` envelope contains only the
          non-``would_*`` values.
    """
    pkg = resources.files("striatum.scaffold.templates.ddd_layout")
    files: list[dict[str, Any]] = []
    for template_rel, target_rel in _DDD_LAYOUT_TEMPLATES.items():
        target = repo / target_rel
        result: dict[str, Any] = {"path": target_rel}
        # Branch 1: target exists.
        if target.exists() or target.is_symlink():
            if not target.is_file():
                # Non-regular file: error in both modes; force does NOT
                # bulldoze. Operator has to resolve manually.
                result["status"] = "would_error" if dry_run else "error"
                result["reason"] = "target exists but is not a regular file"
                files.append(result)
                continue
            if force:
                # Read prior content for the audit trail before deciding
                # whether to write.
                try:
                    prior_bytes = target.read_bytes()
                except OSError as exc:
                    result["status"] = "would_error" if dry_run else "error"
                    result["reason"] = f"{type(exc).__name__}: {exc}"
                    files.append(result)
                    continue
                result["prior_sha256"] = hashlib.sha256(prior_bytes).hexdigest()
                if dry_run:
                    result["status"] = "would_overwrite"
                    files.append(result)
                    continue
                # Force-write path.
                try:
                    body = _read_template_body(pkg, template_rel)
                    target.write_text(body, encoding="utf-8")
                    result["status"] = "overwritten"
                except OSError as exc:
                    result["status"] = "error"
                    result["reason"] = f"{type(exc).__name__}: {exc}"
                files.append(result)
                continue
            # No force: existing file → skipped.
            result["status"] = "would_skip" if dry_run else "skipped"
            result["reason"] = "exists"
            files.append(result)
            continue
        # Branch 2: target missing.
        if dry_run:
            result["status"] = "would_create"
            files.append(result)
            continue
        try:
            target.parent.mkdir(parents=True, exist_ok=True)
            body = _read_template_body(pkg, template_rel)
            target.write_text(body, encoding="utf-8")
            result["status"] = "created"
        except OSError as exc:
            result["status"] = "error"
            result["reason"] = f"{type(exc).__name__}: {exc}"
        files.append(result)
    return {"layout": "ddd", "files": files, "dry_run": dry_run}


def scaffold_striatum_layout(
    repo: Path,
    *,
    workflow_slug: str = "code-change",
    dry_run: bool = False,
) -> dict[str, Any]:
    """Create the RFC 0056 consumer-repo Striatum directories.

    The scaffold is intentionally non-destructive: it creates only
    directories, reports existing directories as skipped, and reports
    files/symlinks at target paths as errors. It does not write
    workflow JSON, artifact files, or `.gitignore` entries because
    RFC 0056 leaves artifact-root commit policy to the operator.
    """

    directories = ["striatum/workflows"]
    if _SAFE_WORKFLOW_SLUG.fullmatch(workflow_slug):
        directories.append(f"striatum/{workflow_slug}")
    else:
        return {
            "layout": "striatum",
            "workflow_slug": workflow_slug,
            "directories": [
                {
                    "path": "striatum/<workflow_slug>",
                    "status": "would_error" if dry_run else "error",
                    "reason": (
                        "workflow_slug must be 1-64 chars and contain only "
                        "letters, digits, dot, underscore, or hyphen; it must "
                        "start with a letter or digit"
                    ),
                }
            ],
            "dry_run": dry_run,
        }

    entries: list[dict[str, Any]] = []
    for directory in directories:
        target = repo / directory
        result: dict[str, Any] = {"path": directory}
        if target.exists() or target.is_symlink():
            if target.is_dir() and not target.is_symlink():
                result["status"] = "would_skip" if dry_run else "skipped"
                result["reason"] = "exists"
            else:
                result["status"] = "would_error" if dry_run else "error"
                result["reason"] = "target exists but is not a directory"
            entries.append(result)
            continue
        parent_blocker = _first_non_directory_parent(repo, directory)
        if parent_blocker is not None:
            result["status"] = "would_error" if dry_run else "error"
            result["reason"] = f"parent exists but is not a directory: {parent_blocker}"
            entries.append(result)
            continue
        if dry_run:
            result["status"] = "would_create"
            entries.append(result)
            continue
        try:
            target.mkdir(parents=True, exist_ok=True)
            result["status"] = "created"
        except OSError as exc:
            result["status"] = "error"
            result["reason"] = f"{type(exc).__name__}: {exc}"
        entries.append(result)
    return {
        "layout": "striatum",
        "workflow_slug": workflow_slug,
        "directories": entries,
        "dry_run": dry_run,
    }


def _first_non_directory_parent(repo: Path, relative_path: str) -> str | None:
    current = repo
    parts = Path(relative_path).parts[:-1]
    walked: list[str] = []
    for part in parts:
        current = current / part
        walked.append(part)
        if current.exists() or current.is_symlink():
            if not current.is_dir() or current.is_symlink():
                return "/".join(walked)
    return None


def _read_template_body(pkg: Any, template_rel: str) -> str:
    """Read a template file from package resources by ``a/b/c.md.tmpl``-
    style relative path."""
    template_resource = pkg
    for part in template_rel.split("/"):
        template_resource = template_resource / part
    return str(template_resource.read_text(encoding="utf-8"))
