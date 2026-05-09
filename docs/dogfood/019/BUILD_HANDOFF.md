---
title: "RFC 0021 V1.5 build handoff (dogfood-019)"
date: 2026-05-09
---

# Build handoff: RFC 0021 V1.5 (--force + --dry-run)

author: implementer-codex-gpt-5.5-001

## Scope

V1.5 ships exactly the two flags pinned in the synthesis. Design
review accepted clean (no findings).

## Files

### Modified

- `src/striatum/scaffold/__init__.py`:
  - Removed the `del force, dry_run` no-op; both kwargs are now
    honored.
  - Added `import hashlib` for the `prior_sha256` audit field.
  - Per-file decision flow expanded:
    - Existing non-regular-file → `error` (or `would_error` in
      dry-run).
    - Existing regular file + `force=True` → read `prior_sha256`,
      then either `would_overwrite` (dry-run) or `overwritten`
      (real write).
    - Existing regular file + no force → `skipped` (or `would_skip`).
    - Missing target + dry-run → `would_create`.
    - Missing target + no dry-run → `created`.
  - New helper `_read_template_body(pkg, template_rel)` factors
    out the package-resource traversal so the force-write and
    initial-create paths share one implementation.
  - Top-level envelope's `dry_run` field reflects the flag (was
    hardcoded `False`).
- `src/striatum/cli/parser.py` — added `--ddd-layout-force` and
  `--ddd-layout-dry-run` flags on the `init` subparser.
- `src/striatum/cli/dispatch.py` — `init` branch reads both flags
  via `getattr` and passes them as keyword arguments to
  `scaffold_ddd_layout`.

### Tests

- `tests/test_scaffold_ddd_layout.py` adds nine V1.5 cases:
  1. `test_scaffold_force_overwrites_existing_file`
  2. `test_scaffold_force_does_not_clobber_non_regular_file`
  3. `test_scaffold_dry_run_writes_no_files`
  4. `test_scaffold_dry_run_envelope_reflects_flag`
  5. `test_scaffold_dry_run_status_vocabulary_empty_repo`
  6. `test_scaffold_dry_run_status_vocabulary_partial_overlap`
  7. `test_scaffold_force_and_dry_run_together_no_writes`
  8. `test_init_cli_flags_thread_through_to_envelope`
  9. `test_init_v15_flags_noop_without_with_ddd_layout`

  All 22 scaffold tests (13 V1 + 9 V1.5) pass.

### Docs

- `docs/DECISION_LOG.md` — D072 row.
- `docs/TODO.md` — F19 row.
- `docs/rfcs/0021-ddd-layout-scaffold-on-init.md` — status →
  `accepted (V1+V1.5)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.10.0 — 2026-05-09` section.
- `pyproject.toml` and `src/striatum/__init__.py` — bumped to
  `1.10.0`.

## Smoke

```
$ cd /tmp/scaffold-v15-smoke && mkdir -p docs && echo "operator content" > docs/SPEC.md
$ striatum init --with-ddd-layout --ddd-layout-dry-run --json | jq '.data.ddd_layout'
{
  "dry_run": true,
  "files": [
    {"path": "docs/SPEC.md", "reason": "exists", "status": "would_skip"},
    {"path": "docs/PRD.md", "status": "would_create"},
    ... (5 more would_create) ...
  ],
  "layout": "ddd"
}
$ striatum init --with-ddd-layout --ddd-layout-force --json | jq '.data.ddd_layout'
{
  "dry_run": false,
  "files": [
    {"path": "docs/SPEC.md", "prior_sha256": "02406ec7...", "status": "overwritten"},
    {"path": "docs/PRD.md", "status": "created"},
    ... (5 more created) ...
  ],
  "layout": "ddd"
}
```

## Test results

- `tests/test_scaffold_ddd_layout.py`: 22 / 22 pass.
- `make lint`: clean.
- `make typecheck`: 62 source files, no issues.
- Full `make test`: pending — running while this handoff is
  drafted.

## Out of scope (V1.6 candidates)

- Template parameter substitution (`--ddd-layout-vars
  project_name=Foo`).
- Multi-layout profiles (`--ddd-layout-profile {ddd, adr_only}`).
- `striatum scaffold sync` upgrade verb.
- Doctor check for missing layout.

These all defer until operator dogfood evidence shows they're
wanted.

## Acceptance summary

| V1.5 acceptance gate | How it's satisfied |
| --- | --- |
| `--force` overwrites existing regular file | `test_scaffold_force_overwrites_existing_file` (status `overwritten` + `prior_sha256` recorded; on-disk content matches template) |
| `--force` does NOT clobber non-regular-file targets | `test_scaffold_force_does_not_clobber_non_regular_file` (directory target stays a directory; status `error`) |
| `--dry-run` writes no files | `test_scaffold_dry_run_writes_no_files` |
| `--dry-run` envelope flag reflects | `test_scaffold_dry_run_envelope_reflects_flag` |
| `--dry-run` uses `would_*` vocabulary | `test_scaffold_dry_run_status_vocabulary_empty_repo`, `test_scaffold_dry_run_status_vocabulary_partial_overlap` |
| `--force` + `--dry-run` together | `test_scaffold_force_and_dry_run_together_no_writes` (status `would_overwrite`; on-disk unchanged) |
| CLI flags thread through dispatch | `test_init_cli_flags_thread_through_to_envelope` |
| V1.5 flags are no-ops without `--with-ddd-layout` | `test_init_v15_flags_noop_without_with_ddd_layout` |
| Zero regression for plain `--with-ddd-layout` | Existing V1 tests still pass; envelope shape preserved when both flags are False. |

V1.5 closes RFC 0021's reserved-API gap. The `scaffold_ddd_layout`
public surface is now fully documented and exercised.
