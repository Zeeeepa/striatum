# Active Documentation Rewrite

You own only the write scope in the work packet. Do not edit outside it.

Rewrite current operator, adopter, and architecture docs for the post-RFC 0078
world. The live product story must be Go-only Striatum runtime, daemon-owned
PostgreSQL, daemon MCP/RPC, and local web UI/operator surfaces. Python may
remain only as target-repository workload language, historical provenance, or
explicitly retired compatibility history.

Check at least:

- `README.md`
- `AGENTS.md`
- `docs/SPEC.md`
- `docs/USING_STRIATUM.md`
- `docs/GETTING_STARTED.md`
- `docs/POSTGRES_TRANSITION.md`
- `docs/CLI_REFERENCE.md`
- `docs/MCP.md`
- `docs/HOW_TO_AGENT.md`
- `docs/operator/BRIEF.md`
- architecture docs listed in the workflow context

Edit only current docs. Do not change RFC files or the decision log in this
job. Do not remove historical references unless a current doc presents them as
live instructions.

Publish
`docs/operator/artifacts/rfc-0078-docs-guardrails-final-deletion/docs/HANDOFF.md`
with:

- Files changed.
- Python-era instructions removed or reclassified.
- Current runtime claim checklist.
- Validation commands run.
- Remaining docs that need a separate decision.
