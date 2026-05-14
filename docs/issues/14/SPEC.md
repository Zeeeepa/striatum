
    # GH #14 -- Recovery cannot clear terminal-run process_exit_nonzero blocker without lease

    Source: <https://github.com/halbritt/striatum/issues/14> (filed 2026-05-14).
    Labels: bug.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    ## Summary

Issue #7 was closed, but the same Engram blocker still cannot be cleared with Striatum 1.48.1. The run is terminal/completed, the review job already published its artifact, recorded an `accept` verdict, and emitted `job.completed`, but an old `process_exit_nonzero` blocker remains open because the process adapter exited nonzero after completion.

## Environment

- Striatum checkout: `/home/halbritt/git/striatum`
- Version installed in Engram venv: `striatum 1.48.1`
- Global PATH install: `/home/halbritt/.local/bin/striatum -> striatum 1.48.1`
- Engram repo: `/home/halbritt/git/engram`
- Engram branch: `master` at `fad02ec Implement RFC 0044 memory integration`

## Affected Engram State

- Run: `run_9cadfc4d2e4646848e2d6539c23322b2`
- Job: `job_run_9cadfc4d2e4646848e2d6539c23322b2_rfc0027_web_state_re_review`
- Session: `sess_e7f56088ea2444a78c886fd9c96bf0ad`
- Process: `proc_f9d0d0acb16d4766ba6b3fcec8e97bac`
- Blocker: `blk_6dd92e18a3da4cc5ac2c4f1445755b99`
- Artifact: `docs/reviews/rerun-backlog-focused-reviews-2026-05-13/RFC0027_WEB_STATE_RE_REVIEW.md`
- Verdict: `accept` / `verdict_a4d97e72583e487095afa4e4f8598367`

Relevant event order from `striatum why blk_6dd... --json`:

1. `artifact.published`
2. `queue.acked`
3. `verdict.recorded` with verdict `accept`
4. `job.completed`
5. `process.exited` with exit code `1`
6. `process_adapter.outputs_missing` opens blocker even though `missing_artifact_paths: []` and `review_verdict_missing: false`

## Reproduction Attempts

From `/home/halbritt/git/engram`:

```sh
STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1 \
  .venv/bin/striatum --repo . recovery resume \
  --blocker-id blk_6dd92e18a3da4cc5ac2c4f1445755b99 \
  --session-id sess_e7f56088ea2444a78c886fd9c96bf0ad \
  --force --complete \
  --summary "Accepted RFC0027 web-state re-review evidence already published and job completed; clearing stale process_exit_nonzero blocker after Striatum recovery update." \
  --json
```

Observed:

```json
{"error":{"code":4,"message":"process-adapter blocker job has no current lease to resume"},"ok":false}
```

Dry and non-dry autonomous recovery both recognize the blocker as eligible but take no action:

```sh
STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1 \
  .venv/bin/striatum --repo . recovery auto \
  --run-id run_9cadfc4d2e4646848e2d6539c23322b2 \
  --autonomous-process-reconcile --eligible-after 0 --json
```

Observed:

```json
{"actions":[],"still_stuck":[{"blocker_id":"blk_6dd92e18a3da4cc5ac2c4f1445755b99","reason":"blocker_recovery_eligible"}]}
```

`checkpoint resolve --action continue` also cannot clear it because the blocker is not a `human_checkpoint`.

## Expected Behavior

For a terminal run where a process-adapter blocker was opened after the job already completed and required review outputs exist, Striatum should provide a public recovery path to close the stale blocker without requiring a current lease.

One acceptable behavior would be a recovery mode that verifies:

- run is terminal/completed,
- job has `job.completed`,
- expected artifact exists and was published,
- required verdict exists,
- blocker payload has `missing_artifact_paths: []` and `review_verdict_missing: false`,

then resolves the stale `process_exit_nonzero` blocker as terminal cleanup.

## Current Impact

Engram has no human checkpoints and no active/stale/lost processes, but `striatum status` and `doctor` continue to report this open blocker indefinitely. Manual SQLite mutation was intentionally avoided.

Related: #7
