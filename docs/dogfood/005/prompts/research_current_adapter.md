# Research The Current Adapter Path

Read the work packet first. Include the exact `author:` line near the
top of the artifact you publish.

Survey:

- `src/striatum/process_adapter.py` — especially `run_process_adapter`,
  `prepare_process_launch`, `mark_process_running`,
  `mark_process_exited`, `mark_process_failed`, and the helpers around
  `process_executions` row state.
- `src/striatum/db.py` — schema for `process_executions`, `blockers`,
  `events`; helpers like `transaction`, `insert_event`, anything that
  records blocker rows today.
- `src/striatum/cli/dispatch.py` and `parser.py` — how
  `adapter run` is wired; how `recovery` subcommands look today.
- `src/striatum/cli/recovery.py` — the existing
  `recovery requeue-stale` pattern; the doctor check pattern.
- `src/striatum/cli/introspect.py` — `doctor` and `status`
  surfaces.
- Existing tests under `tests/test_supervise.py`,
  `tests/test_recovery_extended.py` — patterns the new tests
  should follow.

Verify RFC 0014's claims line by line:

- Does `run_process_adapter` actually skip post-exit validation?
- Does it actually have no timeout?
- Are external kills actually unreconciled today?
- What blocker reasons already exist? Where do they live in code?
- What does the existing diagnostic envelope (if any) look like?
- Do `process_executions` rows already have a `state` column with
  the right vocabulary, or does the RFC's `'timed_out'` and
  `'lost'` need new schema?

Publish `docs/dogfood/005/research/CURRENT_ADAPTER.md` covering:

- exact source citations (file:line) for each check;
- inventory of code paths that need to change for steps 1, 2, 3;
- any RFC drift (something the RFC said incorrectly or omitted);
- recommended minimal-touch implementation order.

Keep it focused. The synthesis job consumes this; design reviewer
needs to see the citations to verify nothing is invented.

Use native subagents for independent file reads if useful; keep the
parent session accountable for the published artifact.
