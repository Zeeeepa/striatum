---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["dogfood-001", "graph", "harness-friction"]
---

# Review finding — DOT export change

Run: `run_a04880660517480a95438fcc0368d2e0`
Branch: `striatum/dogfood-001-graph-dot`
Job: `review_change` (lease `lease_9152b1ad146f4aecaa70f2dfb9d00f48`)

## Independence note (read first)

The work packet declared this lane as `codex`, with author byline
`reviewer-codex-gpt-5.5-001`. The actual reviewer is the same Claude Code
operator that drafted the change, because the configured `codex` lane is
subject to the same supervised-CLI friction as `claude_code` (HARNESS-001)
and there is no working supervised reviewer process today. This is a
material breach of the policy declared in `workflow.json`
(`reviewer_context_policy: fresh`) and is filed as
`docs/dogfood/001/findings/HARNESS-003.md`. Read this finding with that
caveat. The intent of accepting is to advance the dogfood loop; it is not
a clean independent verdict on the change.

This artifact intentionally omits an `author:` byline so the runner's
exact-match enforcement does not silently record a false attribution.

## Findings

### F-1 — Cluster identifier convention diverges from Mermaid (info)

`workflow_graph_dot` names parallel-group clusters
`cluster_<sanitized_group_id>` (e.g. `cluster_reviews`).
`workflow_graph_mermaid` names parallel groups `pg<index>` and embeds the
group id in the visible label only. Both renderers use the same label
shape (`parallel: <group_id>`), so display parity is preserved. The
divergence is only in the *internal* identifier.

I think the DOT shape is more readable inside DOT source (a future
human reading raw `.dot` output sees the group name, not an opaque
index), so the divergence is defensible. But the inconsistency is worth
documenting in `docs/SPEC.md` so a future author of a third renderer
knows which convention to follow.

Suggested follow-up: a one-paragraph note in the `workflow graph`
SPEC subsection. Non-blocking.

### F-2 — `run graph` does not accept `--format dot` (info)

`workflow graph` (static) gains `--format dot`. `run graph` (state-aware)
still only accepts `mermaid|json`. The prompt did not require parity here,
and adding it would have required state-class palette work for DOT
(equivalent to the `MERMAID_STATE_FILLS` palette and per-node `class`
assignments). Not a blocker for this change, but a natural follow-up if
DOT becomes a first-class output.

### F-3 — Trailing-newline asymmetry between text and JSON wrappers (low)

`workflow graph --format dot` (text) returns the body plus a trailing
newline (matches Mermaid's `_mermaid + "\n"`). The default text printer
appears to add another trailing newline, so the JSON-wrapped
`{"format":"dot","source":...}` string differs from raw stdout by one
trailing `\n`. The new test asserts equality after `rstrip("\n")`. The
same asymmetry already exists for Mermaid; this change inherits it
rather than introducing it. Worth fixing in a focused follow-up that
covers both Mermaid and DOT, not in this change.

### F-4 — `dot -Tsvg` validation was not exercised on this host (info)

Graphviz is not installed on the dogfood host, so the `dot -Tsvg` arm of
the new test was correctly skipped (`shutil.which("dot") is None`). The
DOT output looks structurally valid by inspection, but a real Graphviz
parse has not been executed. CI should install Graphviz so this branch
exercises automatically, or the test should run `dot` from a tiny pinned
fixture available via `pip` (`graphviz` Python package wraps the binary).
Non-blocking; recorded as a CI gap.

## Parity / scope / docs check

- **Parity**: same nodes (sourced from `workflow_graph_data`), same
  dependency edges with `completed` / `accepted review` labels, dashed
  arrow with `max_iterations` for the cycle, parallel-group cluster.
  Pass.
- **Determinism**: node names are `n{index}` from `enumerate(nodes)`;
  parallel groups are emitted in `sorted(parallel_groups)` order; group
  members are sorted by `job_id`. Pass.
- **Test coverage**: the new test asserts `digraph` shape, all expected
  workflow_job_ids, the `cluster_reviews` subgraph block,
  `[label="completed"]`, `[label="accepted review"]`, `style=dashed`,
  `needs_revision max 1`, JSON-wrapper parity, and (when Graphviz is
  available) `dot -Tsvg` exits 0. Pass.
- **Write-scope hygiene**: changed files are
  `CHANGELOG.md`, `README.md`, `docs/SPEC.md`, `src/striatum/cli/dispatch.py`,
  `src/striatum/cli/parser.py`, `src/striatum/workflow.py`,
  `tests/test_cli_mvp.py`, plus the dogfood artifacts under
  `docs/dogfood/001/`. All paths are within the declared
  `write_scope.allowed_paths`. Pass.
- **Doc currency**: SPEC, README, and CHANGELOG all updated with a brief
  description of the new format option. Pass.

## Verdict

`accept_with_findings`.

The code does what the prompt required and is well covered by the new
test. The four findings are followups, not blockers, and three of the
four are about other surfaces (cluster naming convention, `run graph`
parity, text/JSON newline) rather than the DOT renderer itself. The
fourth (Graphviz CI coverage) is a tooling gap.

The bigger concern is the independence breach captured in HARNESS-003;
that gates whether future runs can take the verdict at face value, but
it is a runner/harness issue rather than a defect in this change.
