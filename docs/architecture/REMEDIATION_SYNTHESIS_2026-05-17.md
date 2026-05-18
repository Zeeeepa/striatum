# Architecture Remediation Synthesis — 2026-05-17

Status: active
Date: 2026-05-17
author: coordinator-codex-gpt-5-001

## Inputs

This synthesis reconciles the two root-level remediation drafts:

- `STRIATUM_REMEDIATION_PLAN_CODEX_GPT_5_2026-05-17.md`
- `STRIATUM_REMEDIATION_PLAN_GEMINI_2026-05-17.md`

The product boundary remains `docs/SPEC.md`: Striatum is a standalone,
local-first workflow runner; daemon-owned PostgreSQL is live state; repository
files are provenance; `.striatum/` is operational scratch.

## Synthesis

Both plans point at the same remaining architecture risk: the product says the
daemon/PostgreSQL boundary is authoritative, but a few daemon-global and
compatibility surfaces still rely on the legacy SQLite registry or direct
client database access.

Codex's plan corrects two stale claims from Gemini's input:

- CLI daemon-route exception fallthrough is already fail-closed.
- `repo.add`, `repo.list`, and `repo.remove` already route through PostgreSQL.

The operator then made the daemon-core direction explicit: the Python daemon is
not a product constraint. The desired end state is a Go production daemon, a
Python CLI/web client layer where useful, and no SQLite in production or
compatibility paths. SQLite may remain only as bounded one-way import fixture
material until those fixtures are retired.

The remaining priority is therefore:

1. Supersede D105 and restore the Go production daemon port as the target.
2. Make daemon-global surfaces PostgreSQL-only and Go-owned.
3. Move client-side repository resolution behind daemon RPC.
4. Keep service and MCP mutation paths on the daemon boundary.
5. Remove hardcoded daemon client handshake versions.
6. Delete or port every remaining SQLite-backed production/helper path.

## RFC Mapping

| RFC | Scope | Priority | Status |
|---|---|---:|---|
| RFC 0068 | Go production daemon port; D107 supersedes D105 and retires the Python daemon after parity. | P0 decision | accepted |
| RFC 0069 | PostgreSQL-only daemon-global surfaces: registry gate, daemon sweep, dashboard-all, daemon MCP resources, owned by the Go port target. | P0 | mostly implemented; remaining work is legacy quarantine deletion |
| RFC 0070 | Daemon client/service boundary completion: repo.resolve, `/v1/invoke`, MCP/local API quarantine, dogfood composites. | P1 | mostly implemented; local MCP aliases are disabled, with composite cleanup residual |
| RFC 0071 | Operator diagnostics and cutover evidence: migration cleanup report, authority doctor, generated matrix follow-up. | P2 | diagnostic slice implemented; residual evidence cleanup tracks legacy fixture retirement |

## Deferred

Archive expansion, durable accepted-risk storage, and Git/PR mutation
integration remain blocked on their existing product decisions. They should not
be smuggled into this remediation cycle.
