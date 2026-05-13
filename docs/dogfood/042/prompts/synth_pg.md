# Track C Synthesis Prompt: Repo-local PG RFC 0042

Produce `docs/dogfood/042/track_c/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/042/track_c/design/codex/DESIGN.md", "docs/dogfood/042/track_c/design/claude_code/DESIGN.md", "docs/dogfood/042/track_c/design/gemini/DESIGN.md"]
---
```

Byline AFTER the block.

Reconcile the 3 PG migration designs into ONE concrete plan for RFC 0042 V1:

- Schema changes: exact tables, exact `repo_id` column additions, composite keys.
- New CLI verb: `striatum daemon migrate-repo-local` exact shape.
- `.striatum/` directory's new role (operational scratch only).
- CLI behavior without daemon.
- RFC 0039 scope revision (Go daemon as gateway for ALL repo-local ops).
- Migration ordering + rollback.
- Audit chain integrity.
- D006/D007/D028 supersession framing.

Choose; do not enumerate. Output is a SPECIFIC plan ready to author RFC 0042 against.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.
