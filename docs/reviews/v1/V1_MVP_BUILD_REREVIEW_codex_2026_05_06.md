# V1 MVP Build Re-Review - Codex

Date: 2026-05-06
Reviewer: Codex GPT-5.5
Branch reviewed: `agent-runner/v1-mvp`
Commit reviewed: `93edc15`
Base reviewed against: `origin/master`
Verdict: accept_with_findings

## Scope

This is the fresh Codex re-review requested after
`V1_MVP_BUILD_REVIEW_codex_2026_05_06.md` rejected the first MVP build. The
review focused on whether the four Codex findings were fixed and whether the
revised P001 build-review gate now has independent reviewer coverage.

Reviewed:

- `agent-runner/src/agent_runner/cli.py`
- `agent-runner/src/agent_runner/db.py`
- `agent-runner/src/agent_runner/workflow.py`
- `agent-runner/src/agent_runner/schema.py`
- `agent-runner/tests/test_cli_mvp.py`
- `agent-runner/docs/reviews/v1/`

## Finding Resolution

### F001 [P1] Non-accept verdicts still complete the gate

Status: resolved.

`verdict_work` now delegates to `record_review_verdict`. Accepting verdicts
complete the review job and enqueue downstream work. `needs_revision` routes
through a declared bounded cycle or opens a human checkpoint. `reject` fails the
review job and run without enqueueing downstream work.

Relevant tests:

- `test_verdict_reject_fails_run_and_does_not_enqueue_downstream`
- `test_accepting_review_verdict_unblocks_downstream`
- `test_verdict_needs_revision_uses_declared_cycle`
- `test_verdict_needs_revision_without_cycle_waits_human`

### F002 [P1] Build review did not use independent reviewers

Status: resolved after this re-review and the updated synthesis.

Independent build review artifacts are now present for all three lanes:

- `V1_MVP_BUILD_REVIEW_claude_2026_05_06.md`
- `V1_MVP_BUILD_REVIEW_gemini_2026_05_06.md`
- `V1_MVP_BUILD_REREVIEW_codex_2026_05_06.md`

The initial Codex review remains as the reject-for-revision artifact and is
superseded by this re-review for the final gate decision.

### F003 [P2] Workflow edges validate but do not drive dependencies

Status: resolved.

`workflow.py` now treats top-level `edges` as the authoritative dependency
source during run creation. `needs` remains allowed as legacy redundancy, but
`validate_needs_match_edges` rejects divergence between `needs` and `edges`.

Relevant tests:

- `test_edges_materialize_dependencies_without_needs`
- `test_workflow_rejects_needs_edges_mismatch`

### F004 [P2] Required artifacts can be satisfied by the wrong file

Status: resolved.

`verify_required_artifacts` now requires the expected `logical_name`,
`artifact_kind`, and exact repo-relative `repo_path`.

Relevant tests:

- `test_complete_requires_expected_artifact_path_and_kind`
- `test_verdict_requires_expected_artifact_path_and_kind`

## Remaining Findings

No blocking findings remain.

The only residual issue I would carry forward is the same P3 noted by Gemini
and Claude: revision cycles can leave multiple artifact rows for the same
`repo_path` without an explicit active/superseded marker. That is not a V1
correctness blocker because job dependencies are explicit and required artifact
verification is job-scoped.

## Verification

Executed from `agent-runner/`:

```bash
PYTHONPATH=src /Users/halbritt/Documents/GitHub/engram/.venv/bin/python -m pytest -q
```

Result:

```text
18 passed in 9.16s
```

Smoke sequence in a temporary repo:

```bash
agent_runner init --json
agent_runner status --json
agent_runner doctor --json
```

Result: all commands returned `ok: true`; `doctor` reported schema version `1`
and no problems.

## Verdict

Accept with the P3 artifact-versioning metadata issue deferred. The code-level
Codex findings are resolved, the independent review gate is now complete, and
the branch is ready for final human review.
