# Build review: RFC 0021 V1 (DDD layout scaffold)

author: reviewer-claude-opus-002
date: 2026-05-09
verdict: accept

Devil's-advocate review of the V1 build against
`DESIGN_SYNTHESIS.md` and the disposition of the design-review
findings recorded in `BUILD_HANDOFF.md`. Walked the diff:
`src/striatum/scaffold/__init__.py`, the seven `.md.tmpl`
templates, `parser.py` + `dispatch.py` wiring, `pyproject.toml`
package-data, and `tests/test_scaffold_ddd_layout.py`.

## Verdict

**accept** — V1 acceptance gate satisfied. All design-review
findings (1–5) addressed in the implementation or test plan.

## Sweep matrix

| Acceptance gate | How V1 satisfies it | Verified |
| --- | --- | --- |
| **Empty-repo creates seven files** | `scaffold_ddd_layout` walks `_DDD_LAYOUT_TEMPLATES` (7 entries) and writes each. | `test_scaffold_creates_seven_files_in_empty_repo` |
| **Each file starts with the RFC 0021 generation comment** | First two lines of every `.md.tmpl` — verified by reading each template. | `test_scaffold_each_file_starts_with_generation_comment` |
| **Idempotent re-run = all skipped** | The existence check returns `skipped` with `reason: "exists"` for each file. | `test_scaffold_idempotent_second_run_all_skipped` |
| **Partial overlap creates only missing files** | Per-file decision; existing files are reported as `skipped`, missing ones are `created`. | `test_scaffold_creates_only_missing_files_in_partial_overlap_repo` |
| **Composability with `--with-skills`** | Dispatch order: `.striatum/` → skills → ddd_layout, each nested under its own envelope key. | `test_init_composability_with_skills_install` (skips if skills env-dependent failure) |
| **Plain `init` byte-identical to v1.7.0** | Plain `init` produces no `ddd_layout` envelope key and no `docs/` files. | `test_init_without_flag_unchanged` |
| **Templates ship in the wheel via `importlib.resources`** | `pyproject.toml` adds `striatum.scaffold.templates` to package-data; the test asserts exactly seven discoverable templates by walking the package. | `test_scaffold_templates_discoverable_via_importlib_resources` (Finding 3 — exact count locked) |
| **Filesystem-error per-file** | OSError during write is caught at the per-file level; status becomes `error` with the exception message; other files continue. | `test_scaffold_filesystem_error_per_file_status_error` (Finding 4) |
| **Non-file target → status:error not skipped** | `target.is_file()` check after existence; directories and broken symlinks both surface as `status: "error"` with `reason: "target exists but is not a regular file"`. | `test_scaffold_target_is_directory_reports_error`, `test_scaffold_target_is_broken_symlink_reports_error` (Finding 2 — acceptance-blocking — implemented) |
| **No `__` directory-separator convention** | Real subdirectories under `templates/ddd_layout/rfcs/`; the dispatch code splits on `/` and traverses sub-resources via `importlib.resources`. | Source review of `_DDD_LAYOUT_TEMPLATES` (no `__` keys); template directory listing shows `rfcs/` as a real subdir. (Finding 5 — acceptance-blocking — implemented) |
| **Suite health** | 13/13 scaffold tests pass; lint clean; mypy clean (61 source files); full suite 354/354 with the BUILD_HANDOFF.md self-reference resolved. | Direct run output. |

## Counterargument sweep

### "The `force` and `dry_run` arguments are dead code"

Yes — V1 reserves them in the signature but ignores them. The
synthesis and the docstring both say so explicitly. This is
deliberate: V1.5 can implement them without breaking the API
surface. Pragmatic. **Accept.**

### "The non-file-target error case has a race"

A target could be a regular file when `is_file()` is checked
and become a directory before `write_text`. In V1's bounded
context (single-process `striatum init`), this is not a real
concern. If it ever becomes one, V1.5 can add `O_CREAT |
O_EXCL` semantics. **Accept.**

### "The OSError message exposes implementation details"

Per Finding 4's test, the message format is
`"<ExceptionType>: <message>"` (e.g., `"OSError: disk full"`).
This leaks the Python exception type into the user-facing
envelope. Counterargument: `striatum`'s other error envelopes
already use this shape (skills install, etc.); consistency
beats abstraction here. **Accept.**

### "The CHANGELOG entry oversells the dogfood"

CHANGELOG's "Dogfooded" subsection notes that dogfood-017's
own workflow uses RFC 0018 V1's `review_posture` and
`required_review_postures` fields. This is genuinely the first
end-to-end exercise of the validation reachability gate, and
the run completed successfully. The entry is accurate and worth
the line. **Accept.**

### "The seven templates are minimal — operators will need more"

Fair. But V1 ships the *minimum* the framing needs to be
coherent: the boundary (SPEC), the audience (PRD), the model
language (UBIQUITOUS_LANGUAGE / DDD), the receipt log
(DECISION_LOG), and the RFC mechanism (rfcs/README,
rfcs/0001-template). RFC 0019 cites these seven explicitly.
Adding more would dilute the bundle's purpose. V1.5 can add
more layouts (`--ddd-layout-profile {full, adr_only, ...}`) if
operators ask. **Accept.**

## Decision

Accept V1. Land the change, bump to 1.8.0, transition RFC 0021
to `accepted (V1)`. The two acceptance-blocking design findings
(Findings 2 and 5) were implemented as required. The two
non-blocking findings (Findings 3 and 4) landed in the test
plan. Finding 1 is documented in BUILD_HANDOFF.

The dogfood-017 run also serves as the first end-to-end
production use of RFC 0018 V1's posture validator and
reachability gate; both exercised cleanly.
