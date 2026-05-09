# Research: init flow + skill-bundle packaging precedent

author: researcher-codex-gpt-5.5-001
date: 2026-05-09

## Existing surfaces

### `init` dispatch (`src/striatum/cli/dispatch.py`)

- `dispatch()` lines 152-171: `init` branch calls `init_repo(repo)`,
  builds `init_result = {"state_dir": ..., "db": ...}`, then if
  `--with-skills` is passed, nests the skills envelope under
  `init_result["skills"]`. RFC 0021's `--with-ddd-layout` slots in
  next, nesting under `init_result["ddd_layout"]`.
- `_skills_install_dispatch` (lines 104-149) is the precedent: a
  thin wrapper that calls `striatum.skills.install` and returns the
  envelope. The scaffold equivalent
  (`_scaffold_ddd_layout_dispatch` or just inline) is even thinner:
  no fan-out, no profile, no namespace — one layout name in V1.

### `--with-skills` envelope shape

`striatum.skills.install()` (lines 60-…) returns:

```python
{
    "profile": "claude_code",
    "scope": "project",
    "namespace": "striatum-",
    "files": [{"path": "...", "status": "created"|"skipped", ...}],
    "dry_run": False,
}
```

RFC 0021's V1 envelope mirrors:

```python
{
    "layout": "ddd",
    "files": [{"path": "...", "status": "created"|"skipped", "reason": "..."}],
    "dry_run": False,
}
```

`status` values per file: `created`, `skipped` (with `reason: "exists"`
in V1; `reason: "<filesystem-error message>"` in error paths).

### Skill-bundle packaging precedent

- `pyproject.toml` line 43:
  ```toml
  [tool.setuptools.package-data]
  "striatum.skills.templates" = ["*.md.tmpl", "**/*.md.tmpl"]
  ```
  RFC 0021 adds:
  ```toml
  "striatum.scaffold.templates" = ["*.md.tmpl", "**/*.md.tmpl"]
  ```
- `src/striatum/skills/install.py` line 270:
  `pkg = resources.files("striatum.skills.templates")`
  → the runtime resource lookup. RFC 0021's `scaffold_ddd_layout`
  uses the same idiom against `striatum.scaffold.templates.ddd_layout`.
- The skill templates use `.md.tmpl` extension; the install path
  performs Jinja-like substitution. RFC 0021 V1 performs *literal*
  copy: the `.tmpl` suffix is stripped on write but the body is
  unchanged. (V1.5 may add substitution.)

### Per-file decision logic precedent

`install.py` lines ~107-180 walk the plan and decide per-file:
- If `force=True` → write.
- If file exists and on-disk SHA matches the rendered template's
  SHA → record as `unchanged`/`skipped`.
- If file exists and SHA differs → for skills, this is the
  `manifest`-aware update path. RFC 0021 simplifies: file exists
  → `skipped` with `reason: "exists"`. No SHA check, no manifest.

V1 of the scaffold deliberately does *not* track a manifest — the
operator owns the files from generation onward (per RFC 0021's
non-goals).

### Test precedent

- `tests/test_skills_install.py` is the precedent. Patterns:
  - Use `tmp_path` for the target.
  - Call `install(target=tmp_path, profile=..., dry_run=...)`.
  - Assert envelope shape, file existence, and idempotency.
  - Use `importlib.resources.files` to verify package-data is
    discoverable in the installed package.
- `tests/test_scaffold_ddd_layout.py` follows this shape with
  the V1 test cases from synthesis.

## Summary table

| Touchpoint | File:line | V1 action |
| --- | --- | --- |
| Templates | `src/striatum/scaffold/templates/ddd_layout/*.md.tmpl` | New tree (7 files) |
| Public API | `src/striatum/scaffold/__init__.py` | `scaffold_ddd_layout(repo, *, force, dry_run) -> dict` |
| CLI flag | `src/striatum/cli/parser.py:14` | Add `--with-ddd-layout` to `init` |
| Dispatch | `src/striatum/cli/dispatch.py:155` | Call after skills, nest under `ddd_layout` |
| Package data | `pyproject.toml:43` | Add `striatum.scaffold.templates` line |
| Tests | `tests/test_scaffold_ddd_layout.py` | New file |

V1 is small and well-bounded; the skill-bundle precedent does most
of the heavy lifting in shape.
