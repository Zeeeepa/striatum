# Designer Role (Dogfood 005)

Take the research handoff plus RFC 0014 itself and produce a
synthesis ready for the implementer. Lock:

- The diagnostic envelope JSON schema (exact field names, types,
  optionality).
- The blocker-reason vocabulary (`process_outputs_missing`,
  `process_review_verdict_missing`, `process_exit_nonzero`,
  `process_timeout_exceeded`, `process_lost_with_outputs_missing`).
- The CLI surface for `--timeout-seconds` (which subcommand, default,
  validation rules).
- The lane-level field name for per-lane timeout default.
- The `recovery process-reconcile` subcommand surface.
- The two new doctor checks.
- The complete test plan covering every failure mode.

Output: `docs/dogfood/005/DESIGN_SYNTHESIS.md`.

Do not write product code from a synthesis job. The implementer's
job is the build slice.
