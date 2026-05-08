"""RFC 0015: self-contained agent skill bundles.

Public surface:

- :func:`install` writes a skill bundle for a given profile/scope.
- :func:`gather_template_context` returns the values the templates
  need (Striatum version, the verb table, the front-matter kinds).
- :data:`MANIFEST_SCHEMA_VERSION` is the manifest schema version
  string the runner emits and validates.
"""

from __future__ import annotations

from striatum.skills.context import gather_template_context
from striatum.skills.install import (
    MANIFEST_SCHEMA_VERSION,
    bundled_template_sha256,
    install,
    load_manifest,
    manifest_path_for,
    skill_files_present,
)

__all__ = [
    "MANIFEST_SCHEMA_VERSION",
    "bundled_template_sha256",
    "gather_template_context",
    "install",
    "load_manifest",
    "manifest_path_for",
    "skill_files_present",
]
