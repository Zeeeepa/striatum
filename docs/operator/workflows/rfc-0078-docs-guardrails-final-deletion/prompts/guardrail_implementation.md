# Python-Trace Guardrail Implementation

You own only the write scope in the work packet. Do not edit outside it.

Implement the final RFC 0078 guardrail that fails when active Striatum Python
runtime traces return. The guardrail must be precise enough to allow:

- Target repositories using Python as their own workload.
- Historical dogfood/provenance artifacts that are explicitly historical.
- Retired names that current docs mention only as refusal or migration history.

It must fail on active Striatum traces such as:

- Production Python source for Striatum runtime behavior.
- Active pytest coverage as the primary validation surface.
- Python package/release/install instructions for current Striatum.
- Python daemon, Python MCP wrapper, direct Python CLI authority, or SQLite
  fallback resurrection.
- Current operator instructions that require Python Striatum setup.

Prefer a checked-in script or Go test plus CI/Makefile wiring if that matches
current repository patterns. Keep allowlists narrow, named, and documented.

Publish
`docs/operator/artifacts/rfc-0078-docs-guardrails-final-deletion/guardrails/HANDOFF.md`
with:

- Guardrail entry points.
- Exact forbidden patterns/classes.
- Allowed historical/provenance exceptions.
- Commands run and result.
- Any remaining active Python trace that blocks deletion.
