# Design synthesis: RFC 0021 V1.5 (--force + --dry-run)

author: designer-codex-gpt-5.5-001
date: 2026-05-09

## Scope

Two flags, no new templates, no new layouts. Parameter
substitution and additional layouts are explicitly deferred to
V1.6+ if dogfood evidence shows operators want them.

## 1. `--force` semantics

When `force=True`:

- A target that exists *as a regular file* is overwritten with
  the template body. Status: `overwritten`. The envelope entry
  records `prior_sha256` (sha256 of the bytes that were
  replaced) so an operator can audit what was clobbered.
- A target that exists but is *not* a regular file (directory,
  broken symlink, FIFO, etc.) still surfaces as
  `{"status": "error", "reason": "target exists but is not a regular file"}`.
  `--force` does not bulldoze; the operator has to delete the
  non-file target manually.
- A target that does not exist is created normally
  (`status: "created"`).

When `force=False` (default), V1 behavior is preserved
byte-for-byte: existing files report `skipped` with `reason:
"exists"`.

## 2. `--dry-run` semantics

When `dry_run=True`:

- *No filesystem mutation* anywhere in the function. Not even
  `parent.mkdir(...)` is called.
- The envelope's top-level `dry_run` field is `true`.
- Per-file status uses the `would_*` vocabulary:
  - Existing file + no force: `would_skip`.
  - Existing file + force: `would_overwrite` (with
    `prior_sha256`).
  - Existing non-file target: `would_error` with the same
    `reason`.
  - Missing target: `would_create`.

When `dry_run=False` (default), V1 behavior is preserved.

## 3. Status vocabulary (V1.5 closed set)

```
created | skipped | error          (V1, unchanged)
overwritten                         (V1.5: --force success)
would_create | would_skip |
  would_overwrite | would_error    (V1.5: --dry-run only)
```

A `dry_run=True` envelope only contains `would_*` and never
contains `created` / `skipped` / `error` / `overwritten`. A
`dry_run=False` envelope only contains `created` / `skipped` /
`error` / `overwritten` and never contains `would_*`.

## 4. Composability: `--force` + `--dry-run`

When both are true:

- `dry_run` wins: no writes happen.
- The envelope reports `would_overwrite` for existing
  regular-file targets.

This is the natural "preview the destructive action" flow.

## 5. CLI flag wiring

Two new flags on the `init` subparser:

```python
init.add_argument(
    "--ddd-layout-force",
    action="store_true",
    help=(
        "With --with-ddd-layout, overwrite existing files "
        "(except non-regular-file targets, which still error). "
        "RFC 0021 V1.5."
    ),
)
init.add_argument(
    "--ddd-layout-dry-run",
    action="store_true",
    help=(
        "With --with-ddd-layout, report what would happen "
        "without writing any files. RFC 0021 V1.5."
    ),
)
```

Validation: passing either flag without `--with-ddd-layout` is
silently a no-op (the scaffold call is gated by
`--with-ddd-layout`). The `dispatch.py` `init` branch reads
both flags via `getattr(args, ..., False)` and threads them as
keyword arguments to `scaffold_ddd_layout(repo, force=...,
dry_run=...)`.

## 6. Public API stability

The `scaffold_ddd_layout(repo, *, force, dry_run)` signature
is unchanged from V1 (V1 already reserved both kwargs). Tooling
that called it with V1 defaults gets V1 behavior. Tooling that
sets `force=True` or `dry_run=True` gets V1.5 behavior. No
deprecation needed.

## 7. Test plan (`tests/test_scaffold_ddd_layout.py`)

V1.5 adds:

1. `test_scaffold_force_overwrites_existing_file` — write a
   stub, call with `force=True`, verify the on-disk content
   matches the template body and status is `overwritten` with
   `prior_sha256`.
2. `test_scaffold_force_does_not_clobber_non_regular_file` —
   target is a directory; force=True still returns `error`
   and does not delete the directory.
3. `test_scaffold_dry_run_writes_no_files` — empty repo +
   `dry_run=True`; verify no `docs/` directory is created.
4. `test_scaffold_dry_run_envelope_reflects_flag` —
   `result["dry_run"] is True`.
5. `test_scaffold_dry_run_status_vocabulary` — empty repo +
   dry-run yields seven `would_create`; partial-overlap +
   dry-run yields a mix.
6. `test_scaffold_force_and_dry_run_together_no_writes` —
   existing file + `force=True, dry_run=True`; status is
   `would_overwrite`; on-disk content unchanged.
7. `test_init_cli_flags_thread_through_to_envelope` —
   subprocess-level test that `--ddd-layout-force` and
   `--ddd-layout-dry-run` appear in the dispatch envelope's
   `ddd_layout` block correctly.
8. `test_init_without_ddd_layout_force_flag_is_noop` — passing
   `--ddd-layout-force` without `--with-ddd-layout` produces
   no `ddd_layout` envelope key.

## 8. Zero-regression contract

Plain `striatum init --with-ddd-layout` (neither V1.5 flag)
produces output byte-identical to v1.9.0. Verified by
existing `test_scaffold_creates_seven_files_in_empty_repo` and
`test_scaffold_idempotent_second_run_all_skipped`.

## 9. Out of scope (still V1.6+)

- `--ddd-layout-vars project_name=Foo` parameter substitution.
- `--ddd-layout-profile {ddd, adr_only, full}` multi-layout.
- `striatum scaffold sync` upgrade verb.
- Doctor check for missing layout.
