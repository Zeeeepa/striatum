# Research: scaffold V1.5 force/dry-run shape

author: researcher-codex-gpt-5.5-001
date: 2026-05-09

## Existing surfaces

### `scaffold_ddd_layout` API (`src/striatum/scaffold/__init__.py`)

```python
def scaffold_ddd_layout(
    repo: Path,
    *,
    force: bool = False,
    dry_run: bool = False,
) -> dict[str, Any]:
```

V1 reserved both keyword arguments with `del force, dry_run`
(line ~52). The docstring already documents intended semantics:

- `force=True` (V1.5): overwrite existing files.
- `dry_run=True` (V1.5): no filesystem mutation; envelope
  describes what *would* happen.

The per-file decision flow (lines ~84–109):

```
for template_rel, target_rel in _DDD_LAYOUT_TEMPLATES.items():
    target = repo / target_rel
    if target.exists() or target.is_symlink():
        if not target.is_file():
            status = "error"; reason = "not regular file"
        else:
            status = "skipped"; reason = "exists"
        files.append(...)
        continue
    try:
        target.parent.mkdir(parents=True, exist_ok=True)
        body = template_resource.read_text(...)
        target.write_text(body, ...)
        status = "created"
    except OSError as exc:
        status = "error"; reason = "OSError: ..."
```

V1.5 inserts a `force` branch in the existing-file path and a
`dry_run` short-circuit before the actual write/mkdir.

### CLI flag wiring

- `parser.py` line ~26: `init.add_argument("--with-ddd-layout", action="store_true", ...)`.
- `dispatch.py` `init` branch: `init_result["ddd_layout"] = scaffold_ddd_layout(repo)` — no flag passthrough today.

V1.5 adds two new boolean flags on the `init` subparser:

- `--ddd-layout-force`
- `--ddd-layout-dry-run`

Both are no-ops without `--with-ddd-layout`.

### Test precedent

`tests/test_scaffold_ddd_layout.py` patterns:

- Use `tmp_path` for the target.
- Call `scaffold_ddd_layout(tmp_path, ...)` and inspect the envelope.
- Subprocess-level tests for the CLI surface.

V1.5 adds:

1. `test_scaffold_force_overwrites_existing_file`
2. `test_scaffold_force_does_not_clobber_non_regular_file`
3. `test_scaffold_dry_run_writes_no_files`
4. `test_scaffold_dry_run_envelope_reflects_flag`
5. `test_scaffold_dry_run_status_vocabulary`
6. `test_scaffold_force_and_dry_run_together_no_writes`
7. `test_init_cli_flags_thread_through_to_envelope`

## Status vocabulary options

V1 vocabulary: `created`, `skipped`, `error`.

V1.5 options:

- **A. Add `overwritten` for force-write success + `would_create` / `would_skip` / `would_overwrite` for dry-run.** Wide vocabulary; explicit.
- **B. Reuse `created` + a top-level `dry_run: true` flag.** Narrower vocabulary; relies on top-level flag for interpretation.

Recommendation: **A**. Self-describing per entry; tooling can filter on `would_overwrite` to flag unexpected-overwrite cases at preview time.

## Summary table

| V1.5 surface | File:line | Action |
| --- | --- | --- |
| `scaffold_ddd_layout` impl | `scaffold/__init__.py:48-110` | Honor `force` + `dry_run`; new statuses |
| `parser.py` flags | `cli/parser.py:26` | Add `--ddd-layout-force` + `--ddd-layout-dry-run` |
| `dispatch.py` passthrough | `cli/dispatch.py:172` | Read flags, pass to `scaffold_ddd_layout` |
| Tests | `tests/test_scaffold_ddd_layout.py` | 7 new cases |
| Docs | various | RFC status, CHANGELOG, version bump |
