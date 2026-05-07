# Draft prompt — Add `--format dot` to `striatum workflow graph`

## Task

Add Graphviz DOT export to `striatum workflow graph`, alongside the
existing Mermaid (default) and JSON formats.

## Context to read first

- `src/striatum/workflow.py` — current `workflow_graph_data` and
  `workflow_graph_mermaid` functions.
- `src/striatum/cli/parser.py` — current argparse for `workflow graph`
  (look for the `graph` subparser).
- `src/striatum/cli/dispatch.py` — the dispatch route that calls
  `workflow_graph_data` / `workflow_graph_mermaid`.
- `tests/test_cli_mvp.py::test_workflow_graph_exports_mermaid_and_json` —
  the existing test for the Mermaid/JSON paths.
- `docs/SPEC.md` — workflow authoring tools section.

## What to implement

1. **`workflow_graph_dot(workflow)` in `src/striatum/workflow.py`.**
   Returns a string containing valid Graphviz DOT (`digraph striatum_workflow { ... }`).
   - Same nodes and edges as the Mermaid output (use
     `workflow_graph_data(workflow)` as the data source).
   - Each node label includes the workflow_job_id, type, and role/lane.
   - Dependency edges are solid arrows.
   - `needs_revision` cycle edges are dashed arrows with the
     `max_iterations` count as the label.
   - Parallel groups become `subgraph cluster_<group>` blocks with a
     `label` attribute.
   - Use stable node names (`n0`, `n1`, ...) like the Mermaid renderer
     does so output is deterministic.

2. **`--format dot` in the parser.** Extend the `format` choices in the
   `workflow graph` subparser to include `dot`. Default stays `mermaid`.

3. **Dispatch route.** When `args.format == "dot"`, call
   `workflow_graph_dot(workflow)`. When `--json` is set, wrap as
   `{"format": "dot", "source": "<dot text>"}`. Otherwise print raw.

4. **Test.** Add `test_workflow_graph_exports_dot` to
   `tests/test_cli_mvp.py`:
   - Run `striatum workflow graph examples/rfc-ledger-cleanup/workflow.json --format dot`.
   - Assert the output starts with `digraph` and contains the expected
     workflow_job_ids.
   - Assert the cycle edge appears (look for a dashed-arrow attribute).
   - If `dot` (Graphviz) is on PATH (`shutil.which("dot")`), pipe the
     output through `dot -Tsvg -o /dev/null` and assert exit 0. Otherwise
     skip that part.

5. **Docs:**
   - `docs/SPEC.md` workflow-graph subsection: mention the new format.
   - `README.md` Command Reference: mention `--format dot`.
   - `CHANGELOG.md`: top-of-file entry under `### Added`.

## Acceptance

- `make lint typecheck test` passes (current baseline 142 → 143 after the
  new test).
- `striatum workflow graph examples/rfc-ledger-cleanup/workflow.json --format dot`
  emits a `digraph` block.
- If Graphviz is installed: `dot -Tsvg <output> > /dev/null` exits 0.

## Handoff

When done, write `docs/dogfood/001/DRAFT_HANDOFF.md` summarizing:

- Files changed (paths only).
- Test count before/after.
- Any open questions for the reviewer.
- Any harness friction you hit (cross-link
  `docs/dogfood/001/findings/HARNESS-*.md` if you filed any).

Then publish the handoff artifact (kind `handoff`, logical_name
`draft_handoff`) and call `striatum complete`.
