# V1 MVP Build Review - Codex

Date: 2026-05-06
Reviewer: Codex GPT-5.5
Branch reviewed: `origin/agent-runner/v1-mvp`
Base reviewed against: `origin/master`
Verdict: reject_for_revision

## Summary

The implementation has a promising V1 shape and its current tests pass, but it
does not yet satisfy the revised P001 one-shot contract or the review-gate
semantics expected by the workflow model. The highest-risk issue is that
non-accepting review verdicts still complete jobs and unblock downstream work.

## Findings

### F001 [P1] Non-accept verdicts still complete the gate

File: `agent-runner/src/agent_runner/cli.py`
Lines: 646-688

`verdict_work` marks every review verdict as `completed` and immediately calls
`maybe_enqueue_downstream`, so `reject` and `needs_revision` unblock dependents
exactly like `accept`. That defeats review gates and the declared bounded
revision loop: a rejecting final review can still let the run finish as
successful.

Recommendation: Gate downstream enqueueing on acceptable verdicts. Route
`needs_revision` through the declared cycle, and make `reject` block or fail the
workflow instead of completing it successfully.

### F002 [P1] Build review did not use independent reviewers

File: `agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW.md`
Lines: 3-5

The current build review says the reviewer is the parent Codex coordinator, and
the branch only adds one `V1_MVP_BUILD_REVIEW.md`. That violates the revised
P001 contract requiring fresh independent Claude, Codex, and Gemini build
review artifacts before synthesis. The branch currently claims the build is
ready without the review gate the one-shot was explicitly revised to require.

Recommendation: Add separate fresh-lane review artifacts, for example:

```text
agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW_claude.md
agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW_codex.md
agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW_gemini.md
```

Then update `V1_MVP_BUILD_SYNTHESIS.md` to cite and synthesize those independent
reviews.

### F003 [P2] Workflow edges validate but do not drive dependencies

File: `agent-runner/src/agent_runner/workflow.py`
Lines: 182-193

`validate_workflow` requires and validates top-level `edges`, but `create_run`
ignores them and only creates `job_dependencies` from each job's ad hoc `needs`
array. Any valid workflow using only `edges` will start all jobs as roots
because no dependencies are materialized.

Recommendation: Either make `edges` authoritative when materializing
`job_dependencies`, or reject workflows whose dependency declarations are
missing from `needs`.

### F004 [P2] Required artifacts can be satisfied by the wrong file

File: `agent-runner/src/agent_runner/db.py`
Lines: 621-636

Completion only checks that an artifact with the expected `logical_name` exists
for the job. It does not verify that the artifact's repo path or kind match the
job's `expected_artifacts` entry, so a job can publish any in-scope file as
`draft` or `review` and complete while the declared output path is missing.

Recommendation: When verifying required artifacts, match at least
`logical_name`, `repo_path`, and `artifact_kind` against each required expected
artifact.

## Verification

Executed from a detached review worktree of `origin/agent-runner/v1-mvp`:

```bash
PYTHONPATH=src /Users/halbritt/Documents/GitHub/engram/.venv/bin/python -m pytest -q
```

Result:

```text
9 passed
```

## Review Result

Do not merge until F001 and F002 are resolved. F003 and F004 should be fixed in
the same revision if this branch is meant to be the V1 MVP rather than a design
spike.
