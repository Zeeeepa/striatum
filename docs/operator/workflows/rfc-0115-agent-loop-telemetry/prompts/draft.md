Draft a concrete implementation plan for RFC 0115.

Required reading:

- `docs/rfcs/0115-precise-token-usage-telemetry.md`
- `docs/explanation/mcp.md`
- `docs/how-to/how-to-agent.md`
- `docs/reference/roadmap.md` section 9.3
- `CHANGELOG.md` v2.29.0 `claude --print` retirement notes

Output `docs/operator/workflows/rfc-0115-agent-loop-telemetry/artifacts/DRAFT.md`.

The draft must:

- State the desired outcome in operational terms: precise dashboard token burn
  for supported supervised agent-loop lanes.
- Identify implementation slices that do not depend on tracked
  `.striatum/bin` wrappers or `claude --print`.
- Define how Claude usage should be sourced from local metrics/OTel when
  available and from PTY counter scrape only as a lower-fidelity fallback.
- Define how agy/Gemini usage should be sourced from structured CLI/provider
  usage metadata when available and from PTY counter scrape only as a fallback.
- Specify token-dashboard ingest changes, precedence keys, and tests.
- Preserve privacy boundaries: no prompts, transcripts, tool payloads,
  capability tokens, bearer tokens, auth JSON, cookies, API keys, or provider
  credentials in telemetry or public dashboard data.
- Call out unknowns that need live CLI/provider inspection.
