---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-05-17_go-daemon-remediation"
supersedes: "brief_2026-05-17_initial-operator-surface"
scope_links: ["docs/operator/plans/rfc-0068-go-daemon-port.md", "docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md", "docs/operator/plans/rfc-0058-operator-progress-surface.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: coordinator-codex-gpt-5.5-001

## State

Striatum's live-state boundary is daemon-owned PostgreSQL. The active
remediation runway is D107/RFC 0068: keep moving production daemon
ownership to Go, retire the Python daemon after parity, and remove
production SQLite fallbacks. Recent checkpoints routed daemon MCP
resources, daemon audit, daemon health, daemon doctor diagnostics,
daemon lifecycle helpers, and workflow-upgrade running-run checks away
from production SQLite paths.

## Next 1-3 Actions

1. Continue RFC 0068 Go parity work only when a concrete unported daemon
   method or conformance gap is visible.
2. Finish RFC 0069 residual dashboard/global diagnostics parity as
   focused read-model slices.
3. Land RFC 0058 operator progress surface V1 so future operators cold
   start from this brief and the linked plans instead of re-reading the
   whole roadmap.

## Blockers

- Product decisions still block accepted-risk durable persistence,
  default live auto-finalize policy, Corpus Contract V2 choices, and
  optional Git/PR authority.
- No human intervention is needed for the remaining Go/PG remediation
  slices unless a product boundary changes.

## Hazards / Do Not

- Do not reopen repo-local SQLite or the legacy daemon registry in
  production paths.
- Do not add hosted services, telemetry, transcript capture, or external
  persistence without a product decision.
- Keep Engram references historical or explicitly optional.

## Pointers

- `docs/operator/plans/rfc-0068-go-daemon-port.md`
- `docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md`
- `docs/operator/plans/rfc-0058-operator-progress-surface.md`
