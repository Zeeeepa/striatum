---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: author-claude-opus-001

# DRAFT_HANDOFF — Add `--format dot` to `striatum workflow graph`

Run: `run_a04880660517480a95438fcc0368d2e0`
Branch: `striatum/dogfood-001-graph-dot`
Job: `draft_change`

## What changed

| File | Why |
| --- | --- |
| `src/striatum/workflow.py` | New `workflow_graph_dot(workflow)` and the helpers `_dot_node_line`, `_dot_label`, `_dot_cluster_id`. Mirrors `workflow_graph_mermaid`'s shape, sourced from `workflow_graph_data`. Added `import re`. |
| `src/striatum/cli/parser.py` | `workflow graph --format` choices now include `dot` (default still `mermaid`). |
| `src/striatum/cli/dispatch.py` | New `dot` branch in the workflow-graph dispatch; imports `workflow_graph_dot`. |
| `tests/test_cli_mvp.py` | New `test_workflow_graph_exports_dot` covering: `digraph` shape, expected workflow_job_ids, parallel-group cluster, completed/accepted-review labels, dashed `needs_revision` cycle, JSON wrapper parity, and (only when `dot` is on PATH) `dot -Tsvg` validation. |
| `docs/SPEC.md` | Mentions the new `--format dot` option in the workflow-graph subsection. |
| `README.md` | Mentions the new format option in the workflow tools paragraph. |
| `CHANGELOG.md` | New entry under `## Unreleased / ### Added`. |

## Test count

- Before: 142 passing.
- After: 143 passing.
- Lint and mypy both clean.
- `dot -Tsvg` was not exercised on this host (Graphviz is not installed); the
  test correctly skipped that branch via `shutil.which("dot") is None`.

## Quick verification

```bash
.venv/bin/striatum --repo . workflow graph examples/rfc-ledger-cleanup/workflow.json --format dot
```

Yields a valid `digraph striatum_workflow { ... }` block with:

- A `subgraph cluster_reviews { label="parallel: reviews"; ... }` for the
  parallel-review group.
- Solid arrows for dependency edges (`[label="completed"]` and
  `[label="accepted review"]`).
- A dashed arrow for the `needs_revision` cycle
  (`n5 -> n4 [style=dashed, label="needs_revision max 1"];`).

## Open questions for the reviewer

1. **Cluster identifier shape.** I used `cluster_<sanitized_group_id>` and
   fall back to `cluster_pg<index>` if the group_id has no usable id
   characters. Is the explicit sanitization preferable, or should we
   match the existing Mermaid convention of `pg<index>` regardless of
   the group name?
2. **Node label split.** I use a literal `\n` (DOT line break) between
   the job_id and the type/role/lane line, mirroring Mermaid's
   `<br/>`. Is that the right shape, or should the type/role/lane go on
   a single line?
3. **`run graph` parity.** The prompt only required `workflow graph
   --format dot`. `run graph` still only supports `mermaid|json`. If
   parity with `workflow graph` is desired, that's a small follow-up.
4. **Newline shape.** `workflow_graph_dot` returns the body plus a
   trailing newline (matches Mermaid). The default text printer adds an
   extra trailing newline, so the JSON-wrapped `source` differs from raw
   stdout by one trailing newline. The new test asserts equality after
   `rstrip("\n")`. If we want strict equality, the printer or the
   wrapper would need a small adjustment.

## Harness friction filed during this job

Cross-link:

- `docs/dogfood/001/findings/HARNESS-001.md` — Default `claude_code` lane
  command (`claude --model opus -p`) cannot execute work packets. The
  supervised process exited 14s after `claim-next`, with the lease still
  held and stdout/stderr `DEVNULL`. The remainder of this job was driven
  manually by the operator (the Claude Code instance the user is talking
  to) reusing the held lease.
- `docs/dogfood/001/findings/HARNESS-002.md` — Editable install can
  silently pin Striatum to a stale worktree copy. `pip show striatum`
  reported the editable location as `.claude/worktrees/agent-.../`, the
  on-disk source had migration v5 (drops the artifact_kind CHECK), the
  installed copy did not, and the runner happily ran with
  `LATEST_VERSION = 4`. First `publish-artifact --kind
  harness_improvement_proposal` returned `CHECK constraint failed`. Fixed
  by `pip install -e /home/halbritt/git/striatum`; on the next connect
  `apply_migrations` upgraded the DB to `user_version = 5`.

Both proposals propose specific runner / defaults / docs changes; see
each artifact for details.
