# Track A Synthesis Prompt: Go Daemon Steps 1+2

Produce `docs/dogfood/042/track_a/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/042/track_a/design/codex/DESIGN.md", "docs/dogfood/042/track_a/design/claude_code/DESIGN.md", "docs/dogfood/042/track_a/design/gemini/DESIGN.md"]
---
```

Byline AFTER the block.

Reconcile the 3 Go daemon designs into ONE concrete plan for Steps 1+2:

- Layout: `go/cmd/striatumd/`, `go/pkg/{rpc,db}/`, `go/go.mod`, `go/Makefile`.
- RPC envelope-v1 file shapes (per RFC 0039 + RFC 0030 wire protocol).
- Capability table (closed set inherited from RFC 0030).
- Postgres connection + migration loader behavior.
- Audit-chain helper that matches the existing Python `audit_chain` SQL function.
- Test strategy: Go unit tests + RFC 0035 harness `daemon_core="go"` parameter integration.

Choose; do not enumerate. Explicitly defer Steps 3-6 to Phase 2 / future dogfoods.

Implementer split:
- `implement_go_systems_codex` (codex): everything in `go/`, Makefile, daemon-side test files.
- `implement_go_glue_claude` (claude_code): Python harness extensions + docs (HOW_TO_HUMAN, SPEC, UBIQUITOUS_LANGUAGE, RFC 0039 status block).

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.

One-shot supervised invocation.
