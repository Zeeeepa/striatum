# Templates Render-MD Decision

Decide the RFC 0078 fate of `workflow templates render-md`: port it to Go now,
replace it with generated docs from another command, or explicitly retire it.

Base the decision on current source behavior, `docs/SPEC.md`, and
`docs/WORKFLOW_CATALOG.md`. If you port it, keep catalog data local package
data and preserve Mermaid graph-preview output. If you retire it, name the
replacement operator path and the docs that must change in a later slice.

Produce
`docs/operator/artifacts/rfc-0078-workflow-artifact-parity/templates/RENDER_MD_DECISION.md`
with exactly:

`author: operator [self-declared: catalog-porter-codex-gpt-5-001]`

Use decision front matter if publishing as kind `decision`. Include outcome,
reason, implementation/docs touched if any, validation, and residuals.
