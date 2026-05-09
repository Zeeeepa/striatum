# Implement prompt: RFC 0021 V1

Implement against `docs/dogfood/017/DESIGN_SYNTHESIS.md` modulo any
findings from `review/design/DESIGN_REVIEW.md`.

Deliverables:

1. `src/striatum/scaffold/__init__.py` exposing
   `scaffold_ddd_layout(repo, *, force, dry_run) -> dict`.
2. `src/striatum/scaffold/templates/ddd_layout/*.md.tmpl` — seven
   template files per the RFC.
3. `pyproject.toml` `[tool.setuptools.package-data]` includes the
   templates so they ship in the wheel.
4. `src/striatum/cli/parser.py` — `--with-ddd-layout` flag on `init`.
5. `src/striatum/cli/dispatch.py` — wire scaffold call after
   `.striatum/` init, nest envelope under `ddd_layout` key.
6. `tests/test_scaffold_ddd_layout.py` — test plan from synthesis.
7. Doc updates:
   - `docs/SPEC.md` § "Initialization" subsection on the scaffold.
   - `docs/UBIQUITOUS_LANGUAGE.md` — `scaffold layout`, `scaffold
     file status` entries.
   - `docs/DECISION_LOG.md` — D070 row.
   - `docs/TODO.md` — F17 marked done.
   - `docs/rfcs/0021-ddd-layout-scaffold-on-init.md` — status
     `accepted (V1)`.
   - `docs/rfcs/README.md` — index updated.
   - `CHANGELOG.md` — `## 1.8.0 — 2026-05-09` section.
   - `pyproject.toml` and `src/striatum/__init__.py` — bump to 1.8.0.
8. `docs/dogfood/017/BUILD_HANDOFF.md` listing changes.

Constraints: stay inside write_scope; `make lint`, `make typecheck`,
`make test` must pass; report results in BUILD_HANDOFF.md.
