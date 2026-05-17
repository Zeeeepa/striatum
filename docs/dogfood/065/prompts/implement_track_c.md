# Implement Track C prompt - client/service/MCP boundary

Produce `docs/dogfood/065/build/track_c/HANDOFF.md` as a handoff artifact.
Use a title block with `author: implementer-codex-codex-001`.

Track C owns the client/service/MCP boundary and daemon method contract edits.
Stay inside the workflow write scope. Do not edit Track A Go core files except
the generated registry artifact explicitly assigned to this track. Do not edit
SQLite eradication internals or docs decisions.

Implement per synthesis. Required work items:

1. Add daemon-owned repository resolution (`repo.resolve` or the locked
   equivalent) with authorization semantics defined in the synthesis.
2. Remove normal client-side PostgreSQL repository resolution from CLI/web
   service paths.
3. Route daemon-mapped `/v1/invoke` production mutations through daemon RPC,
   not through `striatum.api.invoke`.
4. Keep local workflow authoring helpers explicit and local.
5. Classify `LocalRpcServer`, local `api.invoke`, and invoke-backed chat tools
   as local-authoring or legacy/test surfaces.
6. Port `dogfood.publish_on_behalf` / `dogfood.surgical_recovery` to PG or hide
   them from production daemon MCP until ported.
7. Regenerate method tables and Go registry artifacts when contract changes.

Handoff must include:

- Files changed.
- Tests run and results.
- Production MCP tool list before/after for dogfood composites.
- Any Track A dependency for implementing newly exposed Go methods.
- Confirmation that `.striatum/`, README.md, OPERATOR_REPORT.md, workflow,
  prompts, and roles were not edited.
