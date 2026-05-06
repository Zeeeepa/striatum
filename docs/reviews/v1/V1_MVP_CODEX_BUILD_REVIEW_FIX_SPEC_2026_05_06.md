# V1 MVP Codex Build Review Fix Spec

Date: 2026-05-06
Source review: `agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW_codex_2026_05_06.md`
Target branch: `agent-runner/v1-mvp`
Status: implementation spec

## Summary

The Codex build review is accepted as a reject-for-revision gate. The current
MVP shape is close, but it must not allow non-accepting review verdicts to
unlock downstream work, and it must not claim the V1 build review gate has
passed without independent build review artifacts. The same revision should
also fix dependency materialization from workflow `edges` and exact required
artifact matching.

## Finding Disposition

F001, non-accept verdicts still complete the gate: accepted with one
clarification. Review work may finish recording a `needs_revision` verdict, but
the dependency gate is not satisfied. Only `accept` and
`accept_with_findings` satisfy review dependencies and enqueue ordinary
downstream jobs. `needs_revision` follows a declared bounded cycle when one
matches; otherwise it opens a human checkpoint and blocks progress. `reject`
fails the reviewed path and must not enqueue downstream jobs.

F002, build review did not use independent reviewers: accepted as a process
blocker, not a code patch. The branch needs separate fresh-lane build review
artifacts for Claude, Codex, and Gemini, plus an updated build synthesis citing
them. Implementation workers should not fabricate these artifacts. A human
coordinator must launch or collect them before the branch is considered ready.

F003, workflow edges validate but do not drive dependencies: accepted. Top-level
`edges` are the authoritative dependency declaration for V1. Per-job `needs`
may remain accepted for backward compatibility during this branch, but run
materialization must use `edges` and should reject conflicts between `edges` and
`needs`.

F004, required artifacts can be satisfied by the wrong file: accepted. Required
artifact verification must match `logical_name`, `kind`, and exact repo-relative
`path` for every required expected artifact.

## Implementation Plan

### `agent-runner/src/agent_runner/workflow.py`

- Keep JSON-only validation and top-level required fields unchanged.
- Strengthen `validate_workflow`:
  - Require every edge to have string `from` and `to` job ids.
  - Accept only `on: "completed"` for ordinary V1 edges unless a later spec
    expands gate types.
  - Build a normalized dependency set from `edges`.
  - If any job also declares `needs`, require that each `needs` entry is present
    in the edge set for that job. Reject workflows where `needs` and `edges`
    disagree so fixtures cannot silently diverge.
  - Validate cycle entries have string `from`, string `to`, string
    `on_verdict`, and integer `max_iterations >= 1`. For this revision, require
    `on_verdict == "needs_revision"`.
- Change `create_run` dependency materialization:
  - Insert `job_dependencies` from `workflow["edges"]`, not from job `needs`.
  - Store useful gate metadata in `gate_json`, for example
    `{"on":"completed","from":"review_codex","to":"findings_ledger"}`.
  - For review upstream jobs, include an accepted-verdict gate:
    `{"on":"completed","requires_verdict":["accept","accept_with_findings"]}`.
- Add small helpers rather than large abstractions:
  - `workflow_job_map(workflow) -> dict[str, JsonValue]`
  - `edge_dependency_pairs(workflow) -> list[tuple[str, str, JsonObject]]`
  - `validate_needs_match_edges(workflow)`.

### `agent-runner/src/agent_runner/db.py`

- Replace `maybe_enqueue_downstream`'s simple `upstream.state == "completed"`
  check with a dependency satisfaction helper:
  - Non-review upstream dependency is satisfied when upstream state is
    `completed`.
  - Review upstream dependency is satisfied only when upstream state is
    `completed` and the latest verdict for that job is `accept` or
    `accept_with_findings`.
  - Failed, blocked, waiting-human, stale, rejected, and revision-requested
    upstream paths are not satisfied.
- Keep `maybe_complete_run` conservative:
  - It should complete a run only when all jobs are terminal successful states
    (`completed`, `skipped`, `canceled` as currently defined).
  - If any job is `failed`, mark the run `failed` with a `stop_reason` if the
    run is still `running`.
- Update `verify_required_artifacts`:
  - For each required expected artifact, require an artifact row for the same
    `job_id`, `logical_name`, `artifact_kind`, and `repo_path`.
  - Error messages should name all three expected fields so agents can fix the
    right publish command.
- Add a cycle helper used by verdict handling:
  - `request_revision_for_cycle(conn, review_job, verdict) -> JsonObject`.
  - Read the workflow snapshot for the run and find a cycle where
    `from == review_job.workflow_job_id`, `on_verdict == "needs_revision"`.
  - If no matching cycle exists, create an open blocker, move the job to
    `waiting_human`, and leave the run `running` but unable to complete until
    human action resolves the checkpoint.
  - Enforce `max_iterations` by counting existing jobs for the cycle target
    `workflow_job_id` or by the current review job attempt. If exhausted, open a
    human checkpoint and do not enqueue further work.
  - For the V1 fixture shape, create the next attempt of the cycle target job
    and the next attempt of the cycle review job, with a dependency from the
    next review attempt to the next target attempt. Enqueue the target attempt.
    Use deterministic ids such as `job_<run_id>_<workflow_job_id>_a<attempt>`
    for attempts greater than one.
- Emit explicit events:
  - `verdict.recorded` for all verdicts.
  - `job.completed` only for accepted review jobs and for review jobs that
    successfully hand off to a declared revision cycle.
  - `revision.requested` when `needs_revision` creates a new attempt.
  - `job.failed` and `run.failed` for `reject`.
  - `human_checkpoint.opened` when a revision cannot be routed.

### `agent-runner/src/agent_runner/cli.py`

- Remove the direct import and use of `maybe_enqueue_downstream` from
  `verdict_work`; verdict behavior must go through the new review-gate helper.
- Change `verdict_work` state behavior:
  - `accept` and `accept_with_findings`: verify required artifacts, record the
    verdict, complete the review job/message, release the lease, enqueue
    downstream jobs, and return
    `{"status":"completed","job_id":...,"verdict":...}`.
  - `needs_revision`: verify artifacts, record the verdict, release the lease,
    route through a matching cycle, and return
    `{"status":"revision_requested","job_id":...,"verdict":"needs_revision","next_job_id":...}`.
    If no route exists, return `{"status":"waiting_human", ...}` after opening
    the checkpoint.
  - `reject`: verify artifacts, record the verdict, release the lease, mark the
    review job failed, mark the run failed, and return
    `{"status":"failed","job_id":...,"verdict":"reject"}`.
- Keep the CLI argument surface unchanged. No new commands are required for
  this fix.
- Update `doctor` to report dependency/materialization problems:
  - Job dependencies whose `gate_json` is invalid JSON.
  - Review dependencies whose upstream job completed with no accepting verdict.
  - Required artifact rows whose logical name exists but kind/path do not match
    expected artifacts.

### `agent-runner/src/agent_runner/schema.py`

- No new table is required for the minimal V1 fix.
- Schema changes are allowed if implementation is cleaner, but keep them
  minimal. If schema text changes, bump `SCHEMA_VERSION` and `schema_meta`
  together.
- Existing append-only triggers for `events` and `artifacts` remain mandatory.
- Existing unique constraints on `(run_id, workflow_job_id, attempt)` and active
  work messages must remain enforced.

### `agent-runner/src/agent_runner/artifacts.py`

- Publishing scope validation can stay as-is.
- Do not try to enforce expected artifact kind/path at publish time unless the
  implementation can do so without blocking optional artifacts. The required
  check belongs in `verify_required_artifacts` before completion/verdict.

## State Machine And SQLite Invariants

Review verdict gates:

- `accept`: `running -> completed`; message `acked -> completed`; lease
  `active -> released`; downstream dependencies may become claimable.
- `accept_with_findings`: same as `accept`.
- `needs_revision` with available cycle budget: record verdict, complete the
  review work item, release the lease, create/enqueue the next revision attempt,
  and do not satisfy ordinary downstream dependencies from the review job.
- `needs_revision` without route or budget: record verdict, release the lease,
  move the job to `waiting_human`, open a blocker, and do not enqueue downstream
  work.
- `reject`: record verdict, release the lease, move the job to `failed`, mark
  the run `failed`, and do not enqueue downstream work.

Dependency satisfaction:

- A dependency row is satisfied only through a helper that considers both job
  state and `gate_json`.
- Review dependencies require an accepting verdict even if the review job row is
  `completed`.
- Ordinary completed dependencies remain satisfied by state alone.
- Root job selection at `run start` must use materialized `job_dependencies`;
  because dependencies now come from `edges`, a workflow using only `edges`
  must not start every job as a root.

Artifact invariants:

- `artifacts` remains append-only.
- Completion and verdict require every expected artifact with
  `"required": true` to have a matching artifact row by `(job_id,
  logical_name, artifact_kind, repo_path)`.
- An artifact with the right logical name but wrong path or kind is not enough
  to complete a job.

## CLI Behavior Changes

- `agent_runner verdict --verdict accept` and `accept_with_findings` continue to
  return success status `completed`.
- `agent_runner verdict --verdict needs_revision` returns success status
  `revision_requested` when it creates a bounded revision attempt.
- `agent_runner verdict --verdict needs_revision` returns success status
  `waiting_human` when it records the finding but cannot route a revision. It
  must not return `completed` in that case.
- `agent_runner verdict --verdict reject` returns success status `failed` after
  recording the verdict and failing the run. The command itself exits 0 because
  the verdict was recorded successfully.
- `agent_runner claim-next` must not expose downstream jobs whose upstream
  review verdict was `needs_revision` or `reject`.
- `agent_runner doctor --json` should flag inconsistent review gates and
  required-artifact mismatches.

## Test Plan

Add or update tests in `agent-runner/tests/test_cli_mvp.py`.

- `test_verdict_reject_fails_run_and_does_not_enqueue_downstream`:
  complete `draft`, claim and ack `review_codex`, publish its required review
  artifact, record `reject`, then assert status includes a failed job/run and a
  ledger session receives `no_work`.
- `test_verdict_needs_revision_uses_declared_cycle`:
  drive the fixture to `final_review`, record `needs_revision`, assert the
  command returns `revision_requested`, assert a second synthesis attempt is
  claimable, and assert the run is still running.
- `test_verdict_needs_revision_without_cycle_waits_human`:
  use a small temporary workflow with a review job that has no matching cycle;
  assert `waiting_human`, an open blocker, and no downstream work.
- `test_accepting_review_verdict_unblocks_downstream`:
  keep the existing accept flow but assert the next dependent job becomes
  claimable only after `accept` or `accept_with_findings`.
- `test_edges_materialize_dependencies_without_needs`:
  create a temporary workflow that removes every job `needs` array but keeps
  equivalent `edges`; after start, assert only the root job is claimable.
- `test_workflow_rejects_needs_edges_mismatch`:
  create a workflow whose `needs` disagrees with `edges`; validation should
  fail with workflow error exit code 8.
- `test_complete_requires_expected_artifact_path_and_kind`:
  publish an in-scope artifact with the right logical name but wrong path or
  wrong kind; `complete` should fail. Publish the exact expected path/kind and
  assert completion succeeds.
- `test_verdict_requires_expected_artifact_path_and_kind`:
  same as above for a review job using `verdict`.
- `test_doctor_reports_bad_review_gate_state`:
  seed or drive an inconsistent completed review with no accepting verdict and
  assert `doctor` returns `ok: false`.

Keep tests deterministic. Do not call live model CLIs.

## Verification Commands

Run from `agent-runner/`:

```bash
make test
```

If the Makefile is not available in the current checkout, use the existing
direct test invocation:

```bash
PYTHONPATH=src python -m pytest -q
```

Recommended smoke check after tests:

```bash
tmpdir="$(mktemp -d)"
PYTHONPATH=src python -m agent_runner.cli --repo "$tmpdir" init --json
PYTHONPATH=src python -m agent_runner.cli --repo "$tmpdir" status --json
PYTHONPATH=src python -m agent_runner.cli --repo "$tmpdir" doctor --json
```

## Non-Goals

- Do not add hosted services, cloud APIs, telemetry, or external persistence.
- Do not implement a background queue daemon; lease and gate checks remain lazy
  through CLI commands.
- Do not add Slack, web UI, TUI, MCP, plugin marketplaces, commits, pushes,
  rebases, or merges.
- Do not broaden transcript capture.
- Do not rewrite canonical docs as part of this fix unless a human explicitly
  requests a separate documentation update.
- Do not fabricate Claude or Gemini review artifacts.
- Do not perform a broad refactor of the CLI or storage layer beyond the files
  needed for these findings.

## Human Checkpoints And Blockers

- F002 requires human coordination. A human must obtain fresh independent build
  reviews and update the build synthesis before declaring the V1 MVP branch
  ready:
  - `agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW_claude_2026_05_06.md`
  - `agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW_codex_2026_05_06.md`
  - `agent-runner/docs/reviews/v1/V1_MVP_BUILD_REVIEW_gemini_2026_05_06.md`
  - `agent-runner/docs/reviews/v1/V1_MVP_BUILD_SYNTHESIS.md`
- If implementation discovers that bounded revision cycles require more than a
  single target/review retry pair for the V1 fixture, pause and ask for a
  smaller state-machine decision before inventing a general graph-rewrite
  engine.
- If schema changes are needed, confirm whether this branch needs migrations
  for existing `.agent_runner/state.sqlite3` files or only a clean MVP schema.
