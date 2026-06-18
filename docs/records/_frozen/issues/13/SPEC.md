
    # GH #13 -- LOW: workflow editor — require_attested_lane not purged when job type changes

    Source: <https://github.com/halbritt/striatum/issues/13> (filed 2026-05-14).
    Labels: bug, rfc-0050.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    Found by gemini adversarial review in dogfood-056. Full write-up: `docs/dogfood/056/review/build/gemini/REVIEW.md` Finding 5.

## Issue

In `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`, the `require_attested_lane` field is only visible in the inspector for `review` jobs. If a job's `type` is changed from `review` to `generic`, the `require_attested_lane` property is **not purged** from the internal state.

`jobNodeLabel` (around L146) will still render `require_attested_lane=true` for a `generic` job if it was previously a `review` job, even though the checkbox is now hidden in the inspector.

## Impact

- Confusing UI: node body shows a field the inspector hides.
- Invalid workflow JSON saved to disk: server-side validator catches it (`workflow validate` exit 8), but the operator finds out late.
- Same "ghost field" pattern noted by claude (operator-composed review F3) — independently observed.

## Mitigation

Update `handleJobChange` or the inspector to explicitly purge `require_attested_lane` (and any other field not applicable to the new job type) when `type` changes. Add a Vitest case covering: set type=review + require_attested_lane=true → change type → assert require_attested_lane removed from serialized output.

Bundle with the V2 ergonomics polish pass.
