---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: author-claude-opus-001

# APPLY_HANDOFF — DOT export change finalized

Run: `run_a04880660517480a95438fcc0368d2e0`
Branch: `striatum/dogfood-001-graph-dot`
Job: `apply_change`

## Final test count

- `make lint` → ruff: clean (`All checks passed!`).
- `make typecheck` → mypy: clean (`no issues found in 35 source files`).
- `make test` → 143 passed (was 142; the new
  `tests/test_cli_mvp.py::test_workflow_graph_exports_dot` is the +1).

## Disposition of reviewer findings

The reviewer's verdict was `accept_with_findings` with four findings, all
`severity: low` or `info`. Per the apply prompt, info-only items may be
deferred with rationale. All four are deferred. Each is captured below
and is suitable as a follow-up issue.

- **F-1 (info) — Cluster identifier convention diverges from Mermaid.**
  Deferred. The DOT renderer uses
  `cluster_<sanitized_group_id>`; Mermaid uses `pg<index>`. Both
  preserve the visible label (`parallel: <group_id>`). The DOT shape is
  more readable inside DOT source. Worth a one-paragraph clarification in
  `docs/SPEC.md` later, but not a code change here.

- **F-2 (info) — `run graph` does not accept `--format dot`.**
  Deferred. `run graph` is state-aware and would need a DOT analogue
  for `MERMAID_STATE_FILLS` / per-node class assignments. Out of scope
  for this change.

- **F-3 (low) — Trailing-newline asymmetry between text and JSON
  wrappers.** Deferred. The asymmetry already existed for Mermaid;
  this change inherits it rather than introducing it. A focused fix
  should cover both renderers and the printer in one PR.

- **F-4 (info) — `dot -Tsvg` validation was not exercised on this host.**
  Deferred to CI. Graphviz isn't installed locally; the test correctly
  skipped that branch via `shutil.which("dot") is None`. Suggested
  follow-up: add `graphviz` to the CI image so the SVG branch
  exercises automatically.

No CHANGELOG.md edit was made during apply (no further code changes
beyond the draft).

## Manual verification performed

```bash
.venv/bin/striatum --repo . workflow graph \
  examples/rfc-ledger-cleanup/workflow.json --format dot
```

Output starts with `digraph striatum_workflow {`, contains all expected
workflow_job_ids, the `cluster_reviews` subgraph, solid `[label=
"completed"]` and `[label="accepted review"]` arrows, and the dashed
`[style=dashed, label="needs_revision max 1"]` cycle edge. Graphviz is
not installed on this host, so `dot -Tsvg` was not run; the structural
checks in the test cover the shape.

## Friction surfaces hit during review→apply

The apply step itself was uneventful — the protocol from
`claim-next` → `ack` → `publish-artifact` → `complete` worked as
documented when driven by a CLI operator. The friction recorded for
this run is concentrated in the earlier steps and lives in:

- `docs/dogfood/001/findings/HARNESS-001.md` — Default `claude_code`
  lane (`claude --model opus -p`) cannot execute work packets.
- `docs/dogfood/001/findings/HARNESS-002.md` — Editable install can
  silently pin Striatum to a stale worktree copy, hiding migrations.
- `docs/dogfood/001/findings/HARNESS-003.md` — Reviewer-independence
  and byline policies are advisory, not enforced.
- `docs/dogfood/001/review/HARNESS-004.md` — Reviewer role doc says
  to file harness proposals under `docs/dogfood/001/findings/`, but
  the review job's `write_scope` only allows
  `docs/dogfood/001/review/`. Doc and runner contradict.

The pattern across all four: the dogfood scaffold *says* one thing, the
runner *enforces* another, and the operator only learns the truth at
publish time. That's the most actionable signal from this run, and the
reason the dogfood was worth doing even though the actual code change
was small.

## What's next for the operator

After this job completes, run:

```bash
.venv/bin/striatum --repo . evidence export \
  --run-id run_a04880660517480a95438fcc0368d2e0 \
  --path docs/dogfood/001/EVIDENCE.md --json

.venv/bin/striatum --repo . run summary \
  --run-id run_a04880660517480a95438fcc0368d2e0 \
  --path docs/dogfood/001/RUN_SUMMARY.md --json

.venv/bin/striatum --repo . supervise stop \
  --session-id sess_52019fa306be49e8a37ffb80accc2bac \
  --reason "dogfood 001 done" --json
```

Then commit and tag per the runbook.
