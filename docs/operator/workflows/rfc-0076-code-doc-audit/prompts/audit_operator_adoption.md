# Audit Operator Adoption

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`. Stay within the packet's write scope.

Audit whether an operator or first adopter can use Striatum without
private project memory:

- day-zero setup, install, daemon startup, and first-run smoke;
- workflow selection, lane selection, and work-packet guidance;
- CLI/MCP transition guidance and dashboard/operator surfaces;
- tmux/session introspection, stall handling, and recovery paths;
- error messages and denial vocabulary;
- places where file artifacts help provenance but must not become the
  control plane;
- product, UI, API, or guide gaps that block adoption.

Produce evidence-backed findings with stable `AUD-###` ids. Each material
finding must include severity, category, status, claim, evidence, impact,
recommended action, and follow-up path. Product-shape findings are allowed
when supported by concrete docs, command, source, or test evidence.
Downgrade unevidenced concerns to observations or open questions.

Preserve historical fixtures and dogfood records as provenance. Avoid
treating terminal output, transcripts, marker files, or private operator
memory as authority.
