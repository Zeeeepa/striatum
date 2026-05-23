---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/rfcs/0056-consumer-repo-directory-structure-opinions.md", "docs/CONSUMER_REPO_LAYOUT.md", "src/striatum/scaffold/__init__.py", "src/striatum/cli/parser.py", "src/striatum/cli/dispatch.py", "src/striatum/bootstrap.py", "tests/test_scaffold_ddd_layout.py"]
---

# RFC 0056 Layout Boundary Map
author: rfc0056-layout-mapper-codex-gpt-5-001

## Current Policy Surface

TODO item 47, ROADMAP section 5.8, RFC 0056, and
`docs/CONSUMER_REPO_LAYOUT.md` agree on the live boundary: Phase B created an
opt-in consumer-repo layout scaffold for `striatum/workflows/` and
`striatum/<workflow-slug>/`; workflow-file generation and artifact-root
`.gitignore` policy remain outside that scaffold.

The source matches the docs:

- `scaffold_striatum_layout()` creates only directories, skips existing
  directories, errors on file targets, validates the workflow slug, and
  explicitly does not write workflow JSON, artifact files, or `.gitignore`
  entries.
- `init --with-striatum-layout` calls that scaffold after ordinary
  operational scratch initialization. The baseline `init` path writes only the
  `.striatum/` ignore entry through `_ensure_gitignore_entry()`.
- `workflow generate` is the explicit workflow-file authoring surface. It
  requires a target path and `--artifact-root`; it is not an implicit side
  effect of `init --with-striatum-layout`.

## Test Surface

Before this closure, scaffold tests covered directory creation, dry-run,
existing-directory skips, file-target errors, unsafe workflow slugs, the CLI
envelope, and flag no-ops.

This closure adds a narrow guardrail in `tests/test_scaffold_ddd_layout.py`:

- direct `scaffold_striatum_layout()` leaves `striatum/workflows/` and
  `striatum/<workflow-slug>/` empty and does not create `.gitignore`;
- CLI `init --with-striatum-layout` still includes the baseline `.striatum/`
  ignore entry, but it does not add `striatum/<workflow-slug>/` to
  `.gitignore` and leaves both scaffold directories empty.

## Source Change Needed

No production source change is needed. The implementation is already aligned
with RFC 0056's accepted boundary; the test change prevents that boundary from
drifting silently.
