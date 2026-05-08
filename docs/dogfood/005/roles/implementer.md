# Implementer Role (Dogfood 005)

Before editing source, verify a human acceptance decision exists
under `docs/dogfood/005/decisions/`. If it does not exist, call
`striatum block --severity human_checkpoint` and explain.

If accepted, ship the combined V1 slice from
`docs/dogfood/005/DESIGN_SYNTHESIS.md`:

1. **Post-exit output validation + diagnostic envelope.**
   - Modify `src/striatum/process_adapter.py:run_process_adapter` to
     run validation after `process.communicate` returns and before
     `mark_process_exited` finalizes the row.
   - Insert a blocker row with one of the structured reasons
     (`process_outputs_missing`, `process_review_verdict_missing`,
     `process_exit_nonzero`).
   - Build the diagnostic envelope JSON (no child stdout/stderr;
     only `process_id`, `command`, `exit_code`,
     `duration_seconds`, `missing_artifact_paths`,
     `review_verdict_missing`, `recovery_commands`).
   - Emit a `process_adapter.outputs_missing` event.
   - Transition the job from `running` to `blocked`.

2. **Configurable timeout.**
   - Add `--timeout-seconds <n>` to `striatum adapter run`.
   - Add `lanes.<id>.adapter_timeout_seconds` to workflow validation
     (positive integer when set).
   - Wrap `process.communicate(payload, timeout=n)` in a
     `try/except TimeoutExpired` that SIGTERMs (then SIGKILLs after
     5s) and marks `process_executions.state='timed_out'`.
   - Block the job with `process_timeout_exceeded`, envelope
     identical to (1) plus the timeout value.

3. **Liveness reconciliation + doctor.**
   - New `striatum recovery process-reconcile --run-id <id> [--json]`
     subcommand mirroring the existing `recovery requeue-stale`
     pattern (D036): walk `process_executions` rows in `running`
     state; `os.kill(pid, 0)` per row; transition gone rows to
     `lost` and re-run output validation.
   - Two new `striatum doctor` checks:
     - `process_running_but_pid_gone`
     - `process_running_with_expired_lease`
   - `striatum status --run-id` adds a `process_health` summary key.

Tests in `tests/test_process_adapter.py` (new file) cover:

- happy path (process exits 0, required artifact published) →
  job stays running, blocker not inserted;
- exit 0 with missing required artifact → blocked with
  `process_outputs_missing`;
- exit 0 with missing review verdict → blocked with
  `process_review_verdict_missing`;
- non-zero exit → blocked with `process_exit_nonzero`;
- timeout → SIGTERM, blocked with `process_timeout_exceeded`;
- reconciliation: kill the OS process out-of-band, then
  `recovery process-reconcile`, assert state transitions and
  output validation runs on the newly-`lost` row.

Run `make lint`, `make typecheck`, `make test` before publishing
the build handoff. Do not skip failing tests.

Doc updates:

- `docs/SPEC.md` — § Process Supervision / § Adapter Boundary as
  appropriate; add a "Single-Shot Process Adapter Completion
  Guarantees" subsection.
- `docs/UBIQUITOUS_LANGUAGE.md` — entries for "diagnostic envelope"
  and the blocker-reason vocabulary.
- `docs/rfcs/0014-process-adapter-completion-guarantees.md` —
  status from `proposed` to `accepted (V1)` with an
  "V1 Implementation Slice" subsection that points at the build.
- `docs/DECISION_LOG.md` — D-numbered acceptance row.
- `docs/TODO.md` — F-row for RFC 0014 V1.
- `README.md` — short paragraph under "Process Supervision".
- `CHANGELOG.md` — Unreleased entry under Added.

Publish `docs/dogfood/005/BUILD_HANDOFF.md` listing changed files,
tests run, and any deferred work. Do not run the build review
yourself.

Stay inside the work-packet write scope. The scope explicitly
forbids `.striatum/state.sqlite3` only; other `.striatum/*` writes
are not part of the build slice.
