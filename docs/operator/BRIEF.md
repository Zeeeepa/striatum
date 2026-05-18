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
from production SQLite paths; workflow upgrade now also fails closed when
PostgreSQL state is unknown. The RFC 0058 V1 operator surface is now landed,
and `dashboard.all` now carries per-active-run progress fields for phase
state, auto-finalize dry-run visibility, and supervisor stalls.

## Next 1-3 Actions

1. Route the terminal dashboard's legacy repo-local read path through
   daemon/PostgreSQL DTOs.
2. Continue RFC 0068/RFC 0069 Go and PostgreSQL read-model parity only when
   a concrete method, DTO, or conformance gap is visible.
3. Defer RFC 0058 V1.5 to a focused slice: current-brief CLI, context-budget
   linting, and optional operator-tree init/rotation.

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
