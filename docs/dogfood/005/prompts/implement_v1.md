# Implement V1 Slice

Verify a human acceptance decision exists under
`docs/dogfood/005/decisions/`. If it does not, call
`striatum block --severity human_checkpoint` and explain.

If accepted, ship the combined V1 build slice exactly as the
accepted design specifies.

The three-step shape (each landable in one PR-shaped diff this
job produces):

1. **Post-exit output validation + diagnostic envelope.** Modify
   `run_process_adapter` to inspect required `expected_artifacts`
   and review-job verdict status after `process.communicate`.
   Insert blocker rows with the structured reason vocabulary.
   Build the diagnostic envelope verbatim from the synthesis.
   Emit the event type the synthesis pinned.

2. **Configurable timeout.** Add `--timeout-seconds` to
   `adapter run`. Add `lanes.<id>.adapter_timeout_seconds`
   workflow validation. Wrap `process.communicate` in a
   `try/except TimeoutExpired` that SIGTERMs (then SIGKILLs after
   5s) and marks `process_executions.state='timed_out'`.

3. **Liveness reconciliation + doctor.** New
   `striatum recovery process-reconcile --run-id <id> [--json]`
   subcommand. Two new doctor checks. `status --run-id` adds
   `process_health` summary.

Tests in `tests/test_process_adapter.py` (new file) cover every
failure mode plus the happy path plus the issue #1 reproduction
shape.

Run `make lint`, `make typecheck`, `make test` before publishing.
Do not skip failing tests; fix them.

Doc updates per the synthesis's pinned list (SPEC,
UBIQUITOUS_LANGUAGE, RFC 0014 status flip, DECISION_LOG D-row,
TODO F-row, README, CHANGELOG).

Publish `docs/dogfood/005/BUILD_HANDOFF.md` listing changed files,
tests run, deferred work, and any harness friction. Do not run
the build review yourself.

Stay inside the work-packet write scope. The scope forbids
`.striatum/state.sqlite3` only; other `.striatum/*` writes are not
part of the V1 slice.

Native subagents (Codex sub-agents, etc.) for independent codebase
inspection or test planning are fine; final edits stay in the
parent session.
