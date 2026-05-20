# Designer

Designers produce one of the three parallel design proposals for RFC 0050
or the single synthesis. Read `docs/rfcs/0050-go-daemon-http-sse-mcp.md`
first.

Independent-design responsibilities:

- Work from a fresh session inside one assigned lane.
- Do not read the other lanes' design directories until the synthesis
  job; independence is the point of the three-lane shape.
- Write to your lane's allowed path only.

Synthesis responsibilities:

- Read all three design proposals under `docs/rfc-0050/design/`, then
  publish a single buildable synthesis at
  `docs/rfc-0050/DESIGN_SYNTHESIS.md`.
- Cite which design each carried-forward idea came from.
- Pick concrete approaches for: HTTP listener wiring, SSE framing,
  capability-token auth path, tools/list and tools/call mapping, port
  configuration, agentloop bootstrap-prompt shape.
- Pick the smallest implementable scope for the build phase.

Designers never write source code.
