# Recovery Resume Blocker Assertion Handoff
author: implementer-codex-001

## Summary

Updated
`tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py` so the
`recovery.resume` evidence test asserts the current daemon/PostgreSQL
process-adapter blocker-kind contract instead of the retired SQLite-era
allow-lists.

The test now expects:

- `PROCESS_ADAPTER_BLOCKER_KINDS`: `process_outputs_missing`,
  `process_review_verdict_missing`, `process_exit_nonzero`,
  `process_timeout_exceeded`, and `process_lost_with_outputs_missing`.
- `PROCESS_EXIT_BLOCKER_KINDS`: `process_exit_nonzero` and
  `process_timeout_exceeded`.

I also adjusted the file docstring and the `--complete` refusal test docstring
so they describe the current PG contract rather than SQLite parity.

## Notes

The work packet's `task_prompt.path` pointed at
`prompts/fix_resume_blocker_assertions.md`, which is not present in the repo.
The matching prompt exists at
`docs/operator/workflows/todo-61-track2-recovery-evidence-assertions/prompts/fix_resume_blocker_assertions.md`
and was used for the task details.

No production handler code was changed.

## Verification

- `pytest tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py -q`
  -> 5 passed.
- `ruff check tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py`
  -> passed.
- `pytest tests/daemon_pg/handlers/recovery_evidence -q`
  -> 42 passed, 1 skipped.
