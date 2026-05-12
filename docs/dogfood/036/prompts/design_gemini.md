# Gemini Design Prompt

Produce `docs/dogfood/036/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0034 with attention to catalog metadata shape, the custom-plan compiler's closed block vocabulary + safety rules, the lane-modifier compatibility matrix, and the GeneratedWorkflow envelope returned identically by Python API + CLI + local API.

Your plan must cover:

**Catalog metadata layout and shape:**

- file layout: `src/striatum/workflow_templates/catalog.json` (single file) vs `src/striatum/workflow_templates/{shapes,lane_sets}/*.json` (one entry per file). Pick one and state why.
- shape entries: `template_id`, `kind="shape"`, `display_name`, `summary`, `recommended_for`, `default_lane_sets`, `required_options`, `graph_preview`
- lane-set entries: `template_id`, `kind="lane_set"`, `display_name`, `summary`, `recommended_for`, `required_options`
- `recommended_for` should be specific (e.g. `["small implementation", "docs/code edits"]`), not boilerplate ("flexible workflows")
- `required_options` drives the CLI's required-flag list and the local API's `field_path` validation

**Custom-plan compiler safety:**

- input document schema: `striatum.workflow_plan.v1` with `blocks`, `edges`, `cycles`, `job_lane_bindings`
- closed block vocabulary: `draft | review | synthesis | implementation | test | human_checkpoint | support_ledger | evidence_audit | final_review`
- refusal cases (each returns a structured error with `field_path`):
  - unknown block kind
  - unbounded cycle (no `max_iterations`)
  - edge from/to nonexistent block
  - review block with no `posture` value
  - `job_lane_bindings` referencing a block not declared in `blocks`
  - workflow paths escaping repo (no absolute paths, no `..` segments)
  - write scope containing `.striatum/`
  - expected artifact outside its job's write scope
- compiled `workflow.json` must immediately pass `workflow validate` or the compiler returns a structured error

**Lane-modifier compatibility matrix:**

| Lane set | `supervised` | `worktree_isolated` | `constrained` | `harness_profiled` |
|---|---|---|---|---|
| `local` | ? | ? | ? | ? |
| `single_agent` | ? | ? | ? | ? |
| `author_reviewer` | ? | ? | ? | ? |
| `multi_review` | ? | ? | ? | ? |
| `custom` | ? | ? | ? | ? |

Fill in the matrix. State which combinations are required (must add the modifier), allowed (no-op if not applicable), forbidden (field-specific error), or warning (e.g. `worktree_isolated` on a review-only-only shape is a warning, not a hard error).

**GeneratedWorkflow envelope (returned identically from Python API + CLI `--json` + local API):**

```json
{
  "workflow": {"...": "the workflow.json object"},
  "files": [
    {"path": "workflows/my-change/workflow.json", "content": "..."},
    {"path": "workflows/my-change/roles/author.md", "content": "..."}
  ],
  "metadata": {
    "shape": "code_change",
    "lane_set": "author_reviewer",
    "graph": {"nodes": [], "edges": []}
  },
  "warnings": [],
  "validation": {"ok": true}
}
```

**`--dry-run` semantics:**

- `--dry-run` returns the envelope and writes NOTHING
- non-dry-run writes every file then revalidates the written `workflow.json` on disk
- refuse-to-overwrite is the default; explicit `--force` is left for a future RFC

**Concrete touch points in `src/striatum/`:**

- `workflow_generator.py` (or package) with shape compilers, lane-set compilers, lane-modifier matrix
- `workflow_templates/` package-data tree with catalog entries
- catalog loader (cached at startup; never fetched remotely)
- custom-plan compiler with the closed block vocabulary
- CLI route additions
- local service route additions

**Test coverage strategy:**

- per-shape compiler tests (every built-in shape × every compatible lane set validates)
- lane-modifier compatibility matrix tests (every cell)
- catalog-loader tests (well-formed entries, missing-required-field refusal)
- custom-plan compiler tests (each refusal case named above)
- `workflow init --style` backwards-compat tests
- CLI `--dry-run` writes-nothing tests
- local API preview endpoint non-mutating tests
- local API write endpoint requires `confirm_write: true` tests

**Explicitly deferred to a follow-up dogfood:**

- web `/workflows/new` chooser UI (RFC 0034 §9)
- chat-assisted scaffolding tool (RFC 0034 §10)
- target-repo local catalog extensions (RFC 0034 §6 V1.5)
- automatic repository inspection for suggested shapes

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-gemini-pro-001`
- Wrong: `**Author:** designer-gemini-pro-001` (bolded variant)
- Wrong: `Author: designer-gemini-pro-001` (capital A)
- Wrong: `author: "designer-gemini-pro-001"` (quoted)

If you produce schema-bearing artifacts (synthesis, finding), the file must start with a JSON-encoded `key: <value>` front matter block. Example for `finding`:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0034"]
---
```

The byline appears AFTER the front matter block and a blank line, not inside it.

Do not call striatum CLI; the operator publishes on your behalf otherwise.
