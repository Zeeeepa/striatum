# Design review: RFC 0021 V1 (DDD layout scaffold)

author: reviewer-claude-opus-001
date: 2026-05-09
verdict: accept_with_findings

Devil's-advocate review of `docs/dogfood/017/DESIGN_SYNTHESIS.md`.
Goal: argue against the V1 design's claims, accept only those that
survive strong counterarguments.

## Verdict

**accept_with_findings** — V1 is implementable as written. Two
findings carried forward; one note recorded for V1.5 follow-up.

## Sweep

### 1. Are the seven templates the right set?

**Counterargument:** an operator who adopts striatum into a repo
that already has a `README.md` would expect the scaffold to also
seed `CLAUDE.md` / `AGENTS.md` (the agent context files), since
RFC 0019 cross-references them as the operator-facing model
exposure surface.

**Survives?** Partially. AGENTS.md/CLAUDE.md are *agent-facing*
config; RFC 0015 (`--with-skills`) ships those. The DDD scaffold
is the *human-facing* layer. Composability via
`--with-skills + --with-ddd-layout` covers both. **OK.**

**Finding 1.** The synthesis should explicitly call this out: a
first-time operator runs both flags. Add to BUILD_HANDOFF.

### 2. Is the idempotency contract airtight?

**Counterargument:** what if a target repo has `docs/SPEC.md` as
a *symlink*? `target.exists()` follows symlinks — a broken
symlink returns False, so the scaffold writes through the
broken symlink and silently breaks the operator's setup.

**Counterargument 2:** what if `docs/SPEC.md` is a directory?
`target.exists()` returns True; the scaffold reports `skipped`
but the directory is unrelated. The operator thinks the file is
there and doesn't notice for weeks.

**Survives?** No. The `target.exists()` check is too coarse.

**Finding 2 (acceptance-blocking).** Use `target.is_file()` — and
when `target` exists but is *not* a file, report
`{"status": "error", "reason": "target exists but is not a regular
file"}` instead of `skipped`. This is one extra branch in the
per-file logic and prevents a silent footgun.

### 3. Is the envelope shape consistent with `--with-skills`?

**Counterargument:** the synthesis omits `dry_run` from the
envelope (deferred), but tooling that parses both shapes will
expect the key. A consumer doing `result.get("dry_run", False)`
works either way, but `result["dry_run"]` raises KeyError.

**Survives?** The synthesis explicitly says V1 includes
`dry_run` in the return shape (always `False`). Ambiguous —
tighten the wording. **OK with note.**

### 4. Does `force=False` close enough loops?

**Counterargument:** an operator who realises they accidentally
edited a stub file and want to restart cannot. They have to
delete the file manually.

**Survives?** Yes — that's the *correct* shape. The operator
*does* own the file from generation onward; making `--force`
overwrite is a footgun for the much more common case (operator
ran the flag against a real repo by accident). V1 deferring
`--force` is the right call.

### 5. Does the package-data wiring correctly include `.md.tmpl`?

**Counterargument:** the `[tool.setuptools.package-data]` block
uses string keys with dots. A typo (`striatum.scafold.templates`
missing the `f`) silently produces an empty package and the
scaffold reports an empty file list with no error.

**Survives?** Partially. The skill-bundle precedent has the
same risk; `tests/test_skills_install.py` catches it via
`importlib.resources.files`. The synthesis includes
`test_scaffold_templates_discoverable_via_importlib_resources`
which catches a typo.

**Finding 3 (note).** Test 10 should explicitly assert *seven*
files are discoverable — not "at least one." A package-data
typo that includes only the top-level templates but excludes
the nested ones (e.g., `*.md.tmpl` works but
`**/*.md.tmpl` fails) would still pass a "at least one"
assertion. V1 has no nested templates per the `__` convention,
but the test should pin the count anyway.

### 6. Test plan completeness

**Counterargument:** the test plan does not cover the
filesystem-error path (write to a read-only mount, disk full).
The synthesis mentions error handling in §5 but no test pins
the behavior.

**Survives?** Worth adding. **Finding 4.** Add
`test_scaffold_filesystem_error_per_file_status_error`: mock
`Path.write_text` to raise `OSError` for one file, assert that
file's envelope entry has `status: "error"` and the rest are
`status: "created"`.

### 7. The `__` directory-separator convention

**Counterargument:** the synthesis introduces `__` →`/` mapping
in the file naming because "setuptools package-data does not
preserve subdirectories reliably across all build backends." Is
that actually true? The skill-bundle precedent has nested
directories (`claude_code/`, `gemini/`, `generic/` under
`templates/`) and works fine.

**Survives?** No. The skill-bundle precedent shows nested
directories DO work with `[tool.setuptools.package-data]`
patterns like `**/*.md.tmpl`. The `__` convention adds
unnecessary complexity.

**Finding 5 (acceptance-blocking).** Drop the `__` convention.
Use real subdirectories: `rfcs/README.md.tmpl` and
`rfcs/0001-template.md.tmpl` literally under
`templates/ddd_layout/rfcs/`. The dispatch code becomes
simpler (mirror the directory tree directly).

## Findings summary

| # | Severity | Action |
| --- | --- | --- |
| 1 | note | BUILD_HANDOFF mentions composability with `--with-skills` |
| 2 | acceptance-blocking | Use `target.is_file()`; non-file existence → status error |
| 3 | note | Test 10 asserts exactly seven files discoverable |
| 4 | accept_with_findings | Add filesystem-error per-file test |
| 5 | acceptance-blocking | Drop `__` convention; use real subdirectories |

## Decision

Accept V1 with findings 2 and 5 addressed in the implementation,
findings 3 and 4 added to the test plan, and finding 1 noted in
BUILD_HANDOFF. The synthesis is otherwise sound.
