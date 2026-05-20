# RFC 0050 — HTTP/SSE MCP fixture

Striatum dogfood fixture for implementing
[RFC 0050](../../docs/rfcs/0050-go-daemon-http-sse-mcp.md): native
HTTP/SSE MCP server in the Go `striatumd` daemon.

**Action covered:** action 1 of the operator brief —
"Implement the HTTP/SSE MCP server in the Go daemon as per RFC 0050."

**Out of scope (follow-on runs):**
- Agentloop PTY refactor (action 2).
- `src/striatum/mcp.py` deletion (action 3).

## Shape

Three parallel design lanes (codex / claude_code / gemini) → synthesis
→ ergonomics_dx design review → implementer (codex) → three parallel
build reviews (threat_model / ergonomics_dx / devils_advocate).

`max_active_jobs: 3` lets design and build-review fan out simultaneously.

## How to run

```bash
striatum --repo /path/to/striatum workflow validate examples/rfc-0050-http-sse-mcp/workflow.json
striatum --repo /path/to/striatum run prepare --workflow examples/rfc-0050-http-sse-mcp/workflow.json --json
striatum --repo /path/to/striatum branch confirm --run-id <id> --create
striatum --repo /path/to/striatum run start --run-id <id> --json
```

Watch with `striatum dashboard --run-id <id>`.

## Artifact layout

```
docs/rfc-0050/
├── design/
│   ├── codex/DESIGN.md
│   ├── claude_code/DESIGN.md
│   └── gemini/DESIGN.md
├── DESIGN_SYNTHESIS.md
├── review/
│   ├── design/REVIEW.md
│   └── build/
│       ├── codex/REVIEW.md
│       ├── claude_code/REVIEW.md
│       └── gemini/REVIEW.md
└── build/HANDOFF.md
```
