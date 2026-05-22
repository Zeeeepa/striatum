---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["rfc-0050", "review_authority", "daemon-authority", "mcp-gates", "cli-retirement"]
---

# Authority Review
author: operator [self-declared: codex-operator]

## Verdict

Accept.

The parity slice preserves the RFC 0050 authority boundary. The selected
escalation inbox/detail/resolve UI is scoped to existing daemon methods
(`escalation.list`, `escalation.show`, and `escalation.resolve`) and the
handoff reports that the implementation uses daemon-routed web helpers rather
than shelling out, invoking legacy CLI-shaped routes, opening PostgreSQL
directly, or reading repo-local SQLite.

## Findings

No blocking findings.

The slice keeps CLI workflow-control retirement behind the required gate. Both
the parity-slice artifact and the handoff state that `inbox`,
`escalation list`, `escalation show`, and `escalation resolve` were not hidden,
renamed, deleted, or de-documented. The handoff only claims a UI-first
documentation preference for escalation handling, which is consistent with the
classification rule that deletion requires tested MCP/operator-UI parity and a
recorded survivor category.

The MCP/UI capability boundary is also acceptable for this slice. The selected
methods already exist as MCP methods, with read methods exposed to read-capable
tokens and `escalation.resolve` reserved for admin-capable tokens. The listed
tests cover wrong-capability and wrong-repository refusal, same-origin checks,
the `--allow-mutations` gate, path-id validation, daemon error preservation,
and a SQLite tripwire.

The hidden-method policy remains intact. The slice does not change
`contracts/daemon_methods.json`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`,
`go/pkg/mcp/capabilities.go`, or `src/striatum/cli/parser.py`, and the planned
MCP regression check preserves hidden local-authoring methods from production
MCP discovery.

## Residual Gate

This review does not authorize CLI deletion. Any later hide/delete step still
needs its own cutover artifact proving the exact replacement path passed parity
tests and recording the survivor category for the affected CLI verbs.
