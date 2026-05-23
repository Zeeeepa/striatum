---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/rfcs/0058-operator-progress-surface.md", "docs/operator/plans/rfc-0058-operator-progress-surface.md", "src/striatum/cli/operator.py", "tests/test_operator_current_brief.py"]
---

# Deferred 22 RFC 0058 Operator Tree Closure
author: deferred22-rfc0058-codex-gpt-5-001
status: closed
date: 2026-05-23

## Result

Deferred item 22 is closed without source, CLI, or shared-doc changes.
Operator-tree initialization and brief rotation remain optional future work,
not current product debt.

The current accepted RFC 0058 surface is already complete for V1 and V1.5:
operator artifact schemas exist, corpus metadata exists, `docs/operator/` is
seeded, `operator_brief` context-budget overruns are schema errors, and
`striatum operator current-brief` provides the local read-only cold-start
helper. The remaining init/rotation idea is a write surface, not a correction
to the landed read path.

## Classification

No bounded non-breaking helper is warranted now. A real init/rotation command
would need to choose and test all of these behaviors together:

- resolved operator-docs root precedence across defaults, workflow overrides,
  and the future `striatum.toml` field;
- collision detection for non-empty target trees;
- force behavior and prior-content audit evidence;
- regular-file and symlink safety for `BRIEF.md`;
- CLI help and operator documentation for a write command;
- compatibility with the daemon-required boundary while preserving the fact
  that operator Markdown is durable provenance, not live workflow state.

Implementing only part of that surface would create a misleading command. The
safe closure is to leave manual Markdown rotation plus the existing
`operator current-brief` reader as the supported path until a separate
accepted slice needs the write command.

## Evidence

- `docs/TODO.md` item 65 says RFC 0058 V1/V1.5 are done and that
  operator-tree init/rotation is deferred outside the accepted V1.5 slice.
- `docs/ROADMAP.md` TODO 65 repeats that `operator current-brief` is the
  current local read helper and optional init/rotation remains deferred.
- `docs/rfcs/0058-operator-progress-surface.md` explicitly says V1.5 does not
  implement operator-tree initialization or brief rotation.
- The same RFC's target-repository adoption section describes future write
  behavior as collision-checked and force/audit-aware; that is larger than a
  safe incidental helper.
- `src/striatum/cli/operator.py` is read-only by design and already rejects a
  missing brief, symlink, non-regular brief, invalid front matter, and
  non-current status.
- `tests/test_operator_current_brief.py` covers the default root, custom root,
  missing file, symlink, and non-current status cases.

## Changed Files

- `docs/operator/plans/deferred-22-rfc0058-operator-tree-closure.md`
- `docs/operator/workflows/deferred-22-rfc0058-operator-tree-closure/workflow.json`
- `docs/operator/workflows/deferred-22-rfc0058-operator-tree-closure/prompts/classify_operator_tree_init_rotation.md`
- `docs/operator/artifacts/deferred-22-rfc0058-operator-tree-closure/RESULT.md`

No shared TODO, ROADMAP, BRIEF, RFC, source, or test files were edited.

## Validation

- `PYTHONPATH=src python3 -m striatum.cli workflow validate docs/operator/workflows/deferred-22-rfc0058-operator-tree-closure/workflow.json --json`
  - Result: `{"data":{"valid":true,"workflow_id":"deferred-22-rfc0058-operator-tree-closure"},"ok":true}`.
- `PYTHONPATH=src python3 -m striatum.cli workflow plan docs/operator/workflows/deferred-22-rfc0058-operator-tree-closure/workflow.json --json`
  - Result: valid plan; 1 job, 0 edges, 0 cycles, 1 claim step.
- `PYTHONPATH=src python3 -m striatum.cli workflow lint docs/operator/workflows/deferred-22-rfc0058-operator-tree-closure/workflow.json --json`
  - Result: `valid: true`, `warning_count: 0`, coverage level `strong`.
- `.venv/bin/python -m pytest -q tests/test_operator_current_brief.py tests/cli/test_parser_help.py::test_operator_current_brief_help_documents_local_read_options tests/test_artifact_schemas.py::test_operator_brief_scope_links_are_bounded tests/test_artifact_schemas.py::test_operator_brief_context_budget_lines_is_schema_error`
  - Result: 8 passed.
- `PYTHONPATH=src python3 -m striatum.cli --repo . operator current-brief --json`
  - Result: `ok: true`; current brief id
    `brief_2026-05-23_next-todos`.
- `PYTHONPATH=src python3 - <<'PY' ... validate_artifact_front_matter(...)`
  - Result: work plan and synthesis artifact front matter validated.
- `awk '/[ \t]$/ { print FILENAME ":" FNR ": trailing whitespace"; bad=1 } END { exit bad }' ...`
  - Result: passed for the four new files.
- `git diff --check`
  - Result: passed for tracked-file diff.

## Shared-Doc Updates To Report

No immediate shared-doc edit is required. When the operator next refreshes
shared status docs, deferred item 22 can be described as independently closed:
operator-tree init/rotation remains optional future work, with no current
source change.
