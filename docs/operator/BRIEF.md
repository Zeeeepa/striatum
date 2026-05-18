---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-05-17_go-daemon-remediation"
supersedes: "brief_2026-05-17_initial-operator-surface"
scope_links: ["docs/operator/plans/rfc-0068-go-daemon-port.md", "docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: coordinator-codex-gpt-5.5-001

## State

Striatum's live-state boundary is daemon-owned PostgreSQL. The active
remediation runway is D107/RFC 0068: Go is the production/default daemon,
and remaining cleanup is legacy SQLite fixture conversion/deletion plus
continued guardrail cleanup around retired Python daemon and production SQLite
fallbacks. Recent
checkpoints routed daemon MCP
resources, daemon audit, daemon health, daemon doctor diagnostics,
daemon lifecycle helpers, and workflow-upgrade running-run checks away
from production SQLite paths; workflow upgrade now also fails closed when
PostgreSQL state is unknown, with no repo-local SQLite fallback. The RFC 0058
V1 operator surface is now landed, and `dashboard.all` now carries
per-active-run progress fields for phase state, auto-finalize dry-run
visibility, and supervisor stalls. The compact
terminal dashboard now renders production text frames from daemon/PostgreSQL
DTOs, with legacy SQLite access isolated under paired test-harness fixtures.
Go `status` now matches the PostgreSQL/Python read-model shape, and RFC 0058
V1.5 has landed with `striatum operator current-brief` plus strict
`operator_brief` context-budget validation. Go `workflow.generate --shape
multi_phase` now emits the same V1.1 phased workflow graph as Python, closing
that explicit generator parity gap. Production daemon MCP `tools/list` now
hides local workflow-file authoring methods in both Python and Go; direct
calls to removed dogfood composite names and `apply.reviewed_patch` audit as
`method_unknown`.
SQLite registry-probe guardrails now classify every remaining direct
`striatum.daemon.connect_registry()` caller. Daemon MCP resource fallback
without a PostgreSQL connection is retired and fails closed before the legacy
registry can open. Writable SQLite import windows are
closed: retired `daemon migrate` and `daemon migrate-repo-local` spellings
refuse before importing SQLite migration code. Production daemon CLI/admin
dispatch now uses PostgreSQL-only helpers instead of importing the legacy
Python daemon module, and the CLI-side legacy registry wrapper is gone.
`striatum daemon start` now launches the Go daemon; the Python daemon selector
is retired.

## Next 1-3 Actions

1. Keep RFC 0068/RFC 0069 Go and PostgreSQL conformance green; only add
   parity work when a concrete method, DTO, registry probe, or conformance
   gap is visible.
2. Convert or delete legacy SQLite fixtures; keep guardrails asserting the
   retired Python daemon module remains absent.
3. Keep `make daemon-go-conformance` green and keep removed method names
   returning/auditing as `method_unknown`.

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
