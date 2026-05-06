# RFC 0014 Dogfood Fix Review - Codex

Date: 2026-05-06
Reviewer: Codex GPT-5.5
Branch reviewed: `agent-runner/rfc-0014-validation`
Commit reviewed: `03b24c6`
Verdict: needs_revision

## Scope

Reviewed the implementation of
`agent-runner/docs/RFC_0014_DOGFOOD_FIX_SPEC.md`, focusing on the new
`submit-review`, `status`, `why`, `evidence export`, workflow revision policy,
adapter constraints, and fixture updates.

Verification run during review:

```bash
git diff --check origin/master...HEAD
cd agent-runner
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
```

Results:

- whitespace check passed;
- `25 passed in 13.69s`;
- RFC 0014 workflow validation returned `valid: true`.

## Findings

### F001 [P1] `submit-review` can leave an immutable bad artifact behind

File: `agent-runner/src/agent_runner/cli.py:707-733`

`submit_review` runs `ack_work`, `publish_artifact`, and
`record_review_verdict` as separate transactions. If the verdict step fails
after publication, for example because `--kind` / `--logical-name` does not
match the job's required artifact or the job is not actually a review job, the
artifact row has already been inserted.

Because artifacts are append-only and unique by `(run_id, job_id,
logical_name)`, that can wedge the review job and prevent publishing the
correct required artifact without manual DB repair.

Required fix: pre-validate the review job and expected artifact tuple before
publishing, or refactor this path into one transaction with rollback.

### F002 [P2] `declared_cycle` policy is accepted but not enforced

File: `agent-runner/src/agent_runner/workflow.py:321-323`

The validator accepts `root_review_needs_revision: declared_cycle`, but it does
not verify that root review jobs actually have matching `needs_revision`
cycles. At runtime, a missing cycle still falls through to the generic
human-checkpoint path, so a workflow can claim explicit cycle routing while
behaving like the old surprise checkpoint.

Required fix: reject `declared_cycle` workflows unless the relevant review jobs
have declared cycles, or remove the enum until that validation exists.

## Review Result

The implementation is close, but these two findings should be fixed before the
dogfood fixes are considered ready. F001 is blocking because it can create an
irrecoverable state using the new convenience command.
