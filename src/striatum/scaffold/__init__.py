"""RFC 0021 V1: scaffold the DDD-shaped human-facing doc layout.

Public surface:

- :func:`scaffold_ddd_layout` lays the seven canonical DDD docs
  into a target repository's ``docs/`` tree. Existing files are
  preserved; non-file targets (directories, broken symlinks)
  surface as per-file errors instead of being silently skipped.
"""

from __future__ import annotations

from importlib import resources
from pathlib import Path
from typing import Any

__all__ = ["scaffold_ddd_layout"]


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
        Reserved for V1.5. V1 ignores this argument; existing files
        are always reported as ``skipped``.
    dry_run:
        Reserved for V1.5. V1 ignores this argument; the scaffold
        always writes when a target is missing.

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

        - ``"created"`` — the file did not exist; the template was
          written.
        - ``"skipped"`` — the target file already exists. Reason:
          ``"exists"``. The on-disk content is left unchanged.
        - ``"error"`` — the target exists but is not a regular
          file (directory, broken symlink, etc.) OR the write
          failed with an OS error. Reason names the cause. The
          target is **not** modified in this case.
    """
    del force, dry_run  # reserved for V1.5; documented above

    pkg = resources.files("striatum.scaffold.templates.ddd_layout")
    files: list[dict[str, Any]] = []
    for template_rel, target_rel in _DDD_LAYOUT_TEMPLATES.items():
        target = repo / target_rel
        result: dict[str, Any] = {"path": target_rel}
        if target.exists() or target.is_symlink():
            if not target.is_file():
                result["status"] = "error"
                result["reason"] = (
                    "target exists but is not a regular file"
                )
                files.append(result)
                continue
            result["status"] = "skipped"
            result["reason"] = "exists"
            files.append(result)
            continue
        try:
            target.parent.mkdir(parents=True, exist_ok=True)
            template_resource = pkg
            for part in template_rel.split("/"):
                template_resource = template_resource / part
            body = template_resource.read_text(encoding="utf-8")
            target.write_text(body, encoding="utf-8")
            result["status"] = "created"
        except OSError as exc:
            result["status"] = "error"
            result["reason"] = f"{type(exc).__name__}: {exc}"
        files.append(result)
    return {"layout": "ddd", "files": files, "dry_run": False}
