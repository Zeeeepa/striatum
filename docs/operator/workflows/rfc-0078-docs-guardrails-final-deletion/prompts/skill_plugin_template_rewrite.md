# Skill And Plugin Template Rewrite

You own only the write scope in the work packet. Do not edit outside it.

Update generated skill and plugin template material so installed agents learn
the same current contract as active docs:

- Use daemon MCP first for live workflow control.
- Treat CLI verbs as daemon-backed compatibility/bootstrap clients when they
  remain documented.
- Do not teach direct Python module invocation, Python daemon fallback, Python
  MCP wrapper behavior, repo-local SQLite, or transcript/terminal-output
  authority.
- Preserve target-repository freedom to run Python as that repository's own
  workload.
- If Python template paths are being retired rather than edited, name their
  Go replacement or retirement evidence.

Read the current template source before editing; follow local naming,
formatting, and generated-file conventions.

Publish
`docs/operator/artifacts/rfc-0078-docs-guardrails-final-deletion/templates/HANDOFF.md`
with:

- Template files changed or retired.
- Replacement paths for any retired template.
- Smoke or regeneration command used.
- Any template still blocked on Go packaging/release work.
