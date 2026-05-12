# Implementation Design for RFC 0034: Workflow Generator and Template Catalog

author: designer-gemini-pro-001

This document outlines the implementation strategy for the Workflow Generator and Template Catalog as proposed in RFC 0034.

## 1. Catalog Metadata Shape

The template catalog will be defined in a JSON file (`src/striatum/workflow_templates/catalog.json`) distributed as package data. It contains two lists: `shapes` and `lane_sets`.

### Shape Metadata
A shape defines the workflow graph structure.
```json
{
  "template_id": "string",
  "kind": "shape",
  "display_name": "string",
  "summary": "string",
  "recommended_for": ["string"],
  "default_lane_sets": ["string"],
  "required_options": ["string"],
  "graph_preview": {
    "nodes": [
      {"id": "string", "label": "string"}
    ],
    "edges": [
      {"from": "string", "to": "string", "label": "string"}
    ]
  }
}
```

### Lane-Set Metadata
A lane-set defines the execution environment and actor topology.
```json
{
  "template_id": "string",
  "kind": "lane_set",
  "display_name": "string",
  "summary": "string",
  "recommended_for": ["string"],
  "required_options": ["string"]
}
```

---

## 2. Custom-Plan Compiler

When the chosen shape is `custom`, the generator accepts a custom graph plan. This is a JSON document that defines the blocks, edges, cycles, and lane bindings.

### Closed Block Vocabulary
The V1 custom-plan compiler will only accept the following block kinds:
- `draft`
- `review`
- `synthesis`
- `implementation`
- `test`
- `human_checkpoint`
- `support_ledger`
- `evidence_audit`
- `final_review`

### Edge and Cycle Safety Rules
1. **Acyclic Base Graph:** The `edges` block must not contain cycles.
2. **Explicit Bounded Cycles:** All cycles must be explicitly defined in the `cycles` block.
3. **Max Iterations:** Every cycle MUST have a `max_iterations` value > 0.
4. **Valid Endpoints:** `from` and `to` nodes must reference declared `blocks`.
5. **No Infinite Loops:** Back-edges cannot bypass required human checkpoints if specified.
6. **Lane Bindings:** Every node ID in `blocks` must be bound to a lane in `job_lane_bindings`.

---

## 3. Lane-Modifier Compatibility Matrix

Lane modifiers alter the configuration of the selected lane-set.

| Modifier | Target Lane Type | Compatible Shapes/Lane Sets | Validation Error Context |
| :--- | :--- | :--- | :--- |
| `supervised` | Process | `single_agent`, `author_reviewer`, `multi_review` | Error if lane command does not support stdin streams. |
| `worktree_isolated` | Repo-write | Any lane-set with write operations | Warning if applied to a `review-only` shape (no-op). |
| `constrained` | Any | All | Error if `required_enforcement` specifies an unsupported level by the adapter. |
| `harness_profiled`| Any | All | Error if the referenced `profile_id` is invalid or missing. |

**Field-Specific Errors:**
If a user requests an invalid combination (e.g., `supervised` on a `local` fixture lane), the generator will return a structured error pointing to `lane_modifiers[i]`.

---

## 4. GeneratedWorkflow Envelope Shape

The generator API (both Python and CLI and local API) will return a unified `GeneratedWorkflow` object.

```json
{
  "workflow": { ... }, // The complete workflow.json object
  "files": [
    {
      "path": "workflows/my-change/workflow.json",
      "content": "..."
    },
    {
      "path": "workflows/my-change/roles/author.md",
      "content": "..."
    }
  ],
  "metadata": {
    "shape": "code_change",
    "lane_set": "author_reviewer",
    "graph": {
      "nodes": [ ... ],
      "edges": [ ... ]
    }
  },
  "warnings": [
    "Warning message 1"
  ]
}
```

---

## 5. Generation-time Validation

The generator MUST ensure that every produced `workflow.json` immediately passes `workflow validate`.

1. **Compilation Phase:** The spec is compiled into the `GeneratedWorkflow` structure.
2. **Validation Phase:** The generated `workflow` object is passed directly to `striatum.workflow.validate_workflow()`.
3. **Error Handling:** If `validate_workflow()` raises a `WorkflowError`, the generator catches it, maps the error path back to the generation spec (if possible), and returns a structured error.

---

## 6. `--dry-run` and Path Safety

### Dry-Run Semantics
When `--dry-run` is passed via CLI (or `POST /workflows/generate/preview` is called), the system:
1. Compiles the spec.
2. Validates the generated workflow.
3. Returns the `GeneratedWorkflow` envelope.
4. **Writes ZERO files to disk.**

### Path Safety and Overwrite Refusal
When executing a live generation (CLI without `--dry-run` or API with `confirm_write: true`):
1. The system iterates over the `files` array.
2. It checks if any target path already exists.
3. If **any** file exists, it aborts the entire operation with a `FileExistsError` (or structured API equivalent) indicating which file caused the conflict.
4. No partial writes will occur.
5. All file writes MUST reside within the specified `scaffold_root` (repo-relative) and never escape into `.striatum/` or outside the repo.
