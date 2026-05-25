---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/skills/templates/", "src/striatum/plugins/templates/", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/packaging/HANDOFF.md"]
---

# Skill And Plugin Template Handoff
author: operator [self-declared: template-rewriter-codex-gpt-5-001]

## Result

No template content was changed.

The checked template prose already teaches daemon MCP first, treats CLI verbs
as daemon-backed compatibility fallbacks, and warns against bypassing the
daemon or treating `.striatum/` scratch as workflow state. It does not teach
direct Python module invocation, Python daemon fallback, Python MCP wrapper
behavior, or repo-local SQLite authority.

## Blocker

The template implementation paths themselves are still Python package data
under `src/striatum/skills/templates/` and `src/striatum/plugins/templates/`.
Final RFC 0078 closure needs Go packaging/embedding or explicit template
installer retirement before those Python package paths can be deleted.

## Replacement Evidence Needed

- Go-owned skill installer or embedded template assets.
- Go-owned plugin installer or embedded template assets.
- Smoke/regeneration command that proves installed agent bundles still contain
  the daemon MCP-first guidance.

## Validation

- Reviewed template references with `rg -n -i` over
  `src/striatum/skills/templates` and `src/striatum/plugins/templates`.
- `make python-trace-report`: template paths are part of active guidance
  scanning and produced no unclassified findings.
