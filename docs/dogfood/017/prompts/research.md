# Research prompt: init flow + skill-bundle packaging

Map:

1. The `init` dispatch path in `src/striatum/cli/dispatch.py` — where
   `init_repo()` runs, where the JSON envelope is assembled, where
   `--with-skills` slots in.
2. The `--with-skills` envelope shape — keys, statuses, file list
   structure. RFC 0021's `--with-ddd-layout` envelope mirrors this.
3. The skill-bundle packaging: `[tool.setuptools.package-data]` block
   in `pyproject.toml`, the template files under
   `src/striatum/skills/templates/`, and how `importlib.resources` is
   used to read them at runtime.
4. The pattern for "create file unless it exists": the existing
   skill-install path's per-file decision logic and how it reports
   `skipped` vs `created`.
5. Test precedents: which test files cover skill-install behavior
   (`tests/test_skills*.py` likely), what fixtures they use, and what
   shape `tests/test_scaffold_ddd_layout.py` should follow.

Deliverable: `docs/dogfood/017/research/SCAFFOLD_SHAPE.md` (~50-100
lines) with file:line citations for each touchpoint.
