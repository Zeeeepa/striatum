Produce the final synthesis for RFC 0115 agent-loop token telemetry.

Required reading:

- `docs/operator/workflows/rfc-0115-agent-loop-telemetry/artifacts/DRAFT.md`
- `docs/operator/workflows/rfc-0115-agent-loop-telemetry/artifacts/review/REVIEW.md`
- `docs/rfcs/0115-precise-token-usage-telemetry.md`

Output `docs/operator/workflows/rfc-0115-agent-loop-telemetry/artifacts/SUMMARY.md`.

The summary must:

- State whether the reviewed plan is ready to implement or needs human
  revision.
- List the recommended implementation slices in dependency order.
- Name the test gates and privacy checks required before committing code.
- Identify which work belongs in `~/git/token-dashboard` and which, if any,
  belongs in `~/git/striatum`.
- Avoid proposing tracked wrapper revival or `claude --print`.
