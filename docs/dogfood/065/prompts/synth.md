# Synthesis prompt - dogfood 065

Produce `docs/dogfood/065/synthesis/DESIGN_SYNTHESIS.md` as a synthesis
artifact. Use front matter:

```text
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["codex_design", "claude_design", "gemini_design"]
---
```

Use a title block with `author: designer-codex-codex-001`.

Reconcile the three design handoffs into one locked plan. Do not edit
`README.md`, `OPERATOR_REPORT.md`, this workflow, prompts, or roles.

Required output:

1. One-paragraph scope statement.
2. Track table with Track A/B/C/D, lane owner, exact path ownership, forbidden
   paths, deliverable artifact, and test gate.
3. Cross-track dependency rules. If one track needs a file another track owns,
   it must write that need into its handoff rather than editing across scope.
4. Acceptance criteria for Go daemon parity/schema, SQLite eradication,
   client/service/MCP boundary, and docs/decision consolidation.
5. Verification plan: exact commands or tests to run after each track and
   after integration.
6. Deferrals and blockers.

Bouncing conditions:

- Broad path ownership that would let a track edit another track's files.
- Any production fallback to SQLite.
- Any claim that Go is the default production daemon before the conformance
  suite passes and a follow-up decision/default flip lands.
