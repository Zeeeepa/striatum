Review the upstream RFC 0115 implementation draft.

Required reading:

- `docs/operator/workflows/rfc-0115-agent-loop-telemetry/artifacts/DRAFT.md`
- `docs/rfcs/0115-precise-token-usage-telemetry.md`
- `docs/explanation/mcp.md`
- `docs/how-to/how-to-agent.md`
- `CHANGELOG.md` v2.29.0 `claude --print` retirement notes

Output `docs/operator/workflows/rfc-0115-agent-loop-telemetry/artifacts/review/REVIEW.md`.

Review for:

- Any accidental dependency on live `claude --print`, `-p`, or tracked
  `.striatum/bin` wrapper code.
- Whether the plan fits daemon-owned PTY agent-loop supervision and MCP state
  transitions.
- Whether telemetry stays local, scrubbed, and outside workflow provenance.
- Whether matching/precedence keys are precise enough to avoid double counting.
- Whether tests are sufficient for no-counter fallback, structured usage, and
  privacy scans.

Record a verdict using Striatum's supported review vocabulary.
