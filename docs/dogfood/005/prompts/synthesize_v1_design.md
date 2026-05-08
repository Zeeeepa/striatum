# Synthesize V1 Design

Take the research handoff at
`docs/dogfood/005/research/CURRENT_ADAPTER.md` plus
`docs/rfcs/0014-process-adapter-completion-guarantees.md` and
produce `docs/dogfood/005/DESIGN_SYNTHESIS.md` with the locked V1
shape.

Lock these contracts (the implementer follows them verbatim):

1. **Diagnostic envelope JSON schema.**
   - Field names, types, optionality.
   - Where it is stored (recommendation: blocker `payload_json`).
   - What it must NOT contain (no child stdout/stderr; D028).

2. **Blocker-reason vocabulary.** Pin the exact strings. At least:
   - `process_outputs_missing`
   - `process_review_verdict_missing`
   - `process_exit_nonzero`
   - `process_timeout_exceeded`
   - `process_lost_with_outputs_missing`

3. **Event types.**
   - `process_adapter.outputs_missing` (or whatever the synthesis
     decides) plus shape.

4. **CLI surface.**
   - `striatum adapter run --timeout-seconds <n>` — exact flag
     name, default, validation rules, error handling on timeout.
   - `striatum recovery process-reconcile --run-id <id> [--json]`
     — exact subcommand path, output shape.

5. **Workflow validation.**
   - `lanes.<id>.adapter_timeout_seconds` — type, range, default
     when omitted.

6. **Doctor checks.**
   - `process_running_but_pid_gone` — exact check predicate.
   - `process_running_with_expired_lease` — exact check predicate.

7. **Status field.**
   - `process_health` summary on `striatum status --run-id` —
     exact key shape.

8. **Schema impact.**
   - Whether `process_executions.state` enum needs new values
     (`'timed_out'`, `'lost'`) — and if so, whether a migration is
     required or if the column is already string-typed without a
     CHECK.

9. **Test plan.** Exhaustive list of test cases with file paths and
   what each asserts. The implementer copies this list.

10. **Reproduction shape.** A small fixture workflow that exercises
    the issue #1 reproduction shape (process exits 0 without
    producing the required artifact). Decide whether this lives
    under `examples/process-adapter-failure-fixture/` or stays in
    `tests/`.

11. **Doc update list.** Concrete files and what changes go where.
    Should match RFC 0014's "Acceptance Criteria" section.

12. **Deferred items.** Heartbeat-based timeouts, adapter result
    envelope file, automatic reconcile cadence — explicitly
    out of scope for V1.

Use native subagents for independent code inspection if available.
The parent session owns the synthesis artifact.

Do not write product code from a synthesis job.
