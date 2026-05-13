# Track B Synthesis Prompt: Engram Phase 1 RFC 0044

Produce `docs/dogfood/042/track_b/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/042/track_b/design/codex/DESIGN.md", "docs/dogfood/042/track_b/design/claude_code/DESIGN.md", "docs/dogfood/042/track_b/design/gemini/DESIGN.md"]
---
```

Byline AFTER the block.

Reconcile the 3 Engram designs into ONE concrete plan for RFC 0044 V1:

- Ingestion path for the Striatum corpus.
- Corpus separation (Striatum vs personal-life — Engram's existing corpus stays).
- Engram MCP server topology choice.
- Capability vocabulary for the new memory tools.
- Striatum-side operator wiring.
- Augmentation-not-dependency boundary (Striatum must run without Engram).
- Cite Engram's actual schemas from `~/git/engram/` docs accurately.

Choose; do not enumerate. The output is a SPECIFIC plan ready to author RFC 0044 against.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.
