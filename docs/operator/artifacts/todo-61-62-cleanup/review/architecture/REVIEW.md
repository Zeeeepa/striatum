---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["architecture_boundary", "product_boundary", "daemon_authority", "operator_boundary", "todo-61-62-cleanup"]
---

# Architecture-Boundary Review: todo-61-62-cleanup
author: reviewer-claude-code-001

## Scope and Posture

Fresh-context, document-only review of the cleanup tied to TODO items 61
(RFC 0068 Go production daemon port + Python daemon retirement) and 62
(RFC 0069 PostgreSQL-only daemon-global surfaces). Posture is
`custom:architecture_boundary`: look for regressions against the product
boundary, the daemon-authority boundary, and the operator boundary as
defined in `docs/SPEC.md`, `docs/DECISION_LOG.md`, `AGENTS.md`,
`docs/HOW_TO_AGENT.md`, and the curated authority matrix.

Inputs reviewed: `README.md`, `docs/INDEX.md`, `docs/SPEC.md`,
`docs/DECISION_LOG.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/TODO.md`,
`docs/ROADMAP.md`, `docs/operator/BRIEF.md`, `docs/HOW_TO_AGENT.md`,
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md`,
`docs/rfcs/0068-go-production-daemon-port.md`,
`docs/rfcs/0069-pg-only-daemon-global-surfaces.md`, `AGENTS.md`.

No other artifacts, ledgers, run reports, or source files were consulted.

## Overall Verdict

`accept_with_findings` — low severity. The cleanup as represented in the
input document set is consistent with the three boundaries under review.
No product-boundary, daemon-authority, or operator-boundary regression
was identified. The findings below are documentation hygiene and
forward-pointer items, not architecture regressions.

## What the Cleanup Looks Like Across the Doc Set

The three boundaries are mutually reinforced across the documents:

- **Product boundary** (`docs/SPEC.md` § Product Boundary; `AGENTS.md` §
  Product Boundary): the runner is standalone, local-first, daemon-owned
  PostgreSQL is authoritative live state, `.striatum/` is operational
  scratch, no hosted services / telemetry / vendor SDK / transcript
  capture. RFC 0068 and RFC 0069 both restate non-goals that match
  (no hosted services, no telemetry, no permanent dual-core, no SQLite
  as a supported compatibility mode). RFC 0068 § Non-Goals explicitly
  preserves the no-hosted-services rule; RFC 0069 § Non-Goals avoids
  altering the daemon RPC envelope or capability vocabulary. The cleanup
  does not add new external dependencies, hosted surfaces, or telemetry
  hooks.

- **Daemon authority** (`docs/SPEC.md` § State Store / Registry-Backed
  Multi-Repo Coordination; `docs/UBIQUITOUS_LANGUAGE.md` `daemon-required
  CLI`; `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`): the daemon is
  the single writer; CLI, MCP, and web surfaces are clients; production
  fails closed (exit 11 / 12) without a reachable daemon or registered
  repo; D110 / D112 / D113 removed SQLite-bound RPC names; D114 retired
  the no-PostgreSQL daemon MCP resource fallback. The authority matrix
  shows every registered RPC method has a `real` Go handler and `no`
  CLI fallback; SQLite-dependency cells are `no` except for explicitly
  named bootstrap, fixture, or workflow-upgrade-guard cases. The
  `Direct PostgreSQL Bootstrap/Admin Plane` table is an explicit, scoped
  list rather than an escape hatch, with `tests/architecture/
  test_authority_guardrails.py` named as the drift guard.

- **Operator boundary** (`docs/SPEC.md` § Two Roles / D103 / D104;
  `docs/HOW_TO_AGENT.md`; `docs/operator/BRIEF.md`): the AI operator is
  the default driver; the human principal is escalation-only; daemon
  MCP is mandatory (D104); operator runs cannot use the retired
  `--no-daemon` direct-CLI path; `.striatum/` is operational scratch
  only. The brief's `Hazards / Do Not` block explicitly forbids
  reopening repo-local SQLite or the legacy daemon registry in
  production paths and forbids deleting CLI workflow-control verbs
  before MCP/UI parity exists — that ordering protects the operator
  contract during the cleanup.

The Python daemon-module retirement and the daemon-MCP-surface migration
are described in matching terms across `docs/SPEC.md` (Production MCP
surface is native to Go; "the retired local Python stdio MCP wrapper is
not a product surface"), `docs/UBIQUITOUS_LANGUAGE.md` (`daemon core`
entry naming D107/RFC 0068/D111), `docs/operator/BRIEF.md` ("the remaining
Python `mcp.py` wrapper is removed"), `docs/ROADMAP.md` § 1.1 step 7
(`[done] Delete src/striatum/mcp.py`), and the RFCs themselves.

## Findings

### F1 (low) — Project Status version drift between README.md and ROADMAP.md

`README.md` § Project Status pins:

```
| Version | v1.55.0 (see [CHANGELOG.md](../../../../../../CHANGELOG.md)) |
| CI | 1254 passed / 7 skipped / 0 failures on `main` as of v1.55.0 |
```

`docs/ROADMAP.md` § 1 says:

```
- **Latest tag:** `v1.57.0` is the latest released tag and
  `pyproject.toml` version. v1.57.0 packages the GH #25 / #26 / #27
  cluster on top of v1.56.0 (daemon recovery + RFC 0072 + remediation
  follow-through).
- **Latest substantive release:** v1.57.0 — RFC 0073 implementation
```

This is not a boundary regression — both versions describe the same
daemon-mandatory contract — but a new operator who reads README first
(per `AGENTS.md` § Start Here) will believe v1.55.0 is current and may
discount RFC 0072 (blob storage), RFC 0073 (blob diagnostics), and the
recent `repo list` cleanup until they reach the roadmap. Suggest: bump
the README's `Version` and `CI` rows to `v1.57.0` as part of this
cleanup, or add a "see ROADMAP §1" pointer to the row.

The same row also lists "RFC 0050 ergonomics polish" as an "Active RFC",
which `docs/ROADMAP.md` § 5.1 marks `✅ completed`. Refresh the same
table for the same reason.

### F2 (low) — `docs/SPEC.md` § CLI surface vs. CLI-retirement intent

`docs/SPEC.md` § CLI still enumerates the full operator CLI as
"Required commands". `docs/operator/BRIEF.md` and `docs/ROADMAP.md`
§ 1.1 ("Active Operator track: HTTP/SSE MCP daemon and CLI retirement")
make CLI retirement the active operator track. There is no concrete
regression — SPEC describes the current contract and BRIEF.md
correctly forbids premature deletion ("Do not delete CLI workflow-control
verbs before MCP/UI parity exists and is covered by tests") — but a
reader who lands on SPEC.md without reading BRIEF.md or ROADMAP.md will
not see that CLI verbs are forward-classified as
bootstrap / diagnostics / temporary compatibility. Suggest: add a one-
sentence forward pointer in `docs/SPEC.md` § CLI noting that operator-
driven runs use daemon MCP per D104 and that the CLI verb list is the
current contract surface, not the long-term operator control plane.

### F3 (info) — `docs/operator/BRIEF.md` scope_links cite plans outside the input set

The brief frontmatter declares:

```yaml
scope_links: ["docs/operator/plans/rfc-0068-go-daemon-port.md",
              "docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md",
              "docs/rfcs/0050-go-daemon-http-sse-mcp.md"]
```

A fresh-context reviewer with a document-only access scope cannot verify
the two `docs/operator/plans/...` documents exist or are current. Inside
the input set the brief is internally consistent and within its 300-line
context budget. No regression — the brief's own body is self-contained
— but operators following the scope_links should verify those plan
documents still describe the cleanup accurately. This is also a
reminder that RFC 0050 is not in the review-input set even though it is
named in the brief's scope_links and named as the active operator track
in `docs/ROADMAP.md` § 1.1; a future reviewer should consider including
it.

### F4 (info) — `docs/HOW_TO_AGENT.md` does not yet describe the HTTP/SSE MCP agent path

`docs/SPEC.md` § Local API And MCP Boundary describes the production
HTTP/SSE MCP surface and the runtime-directory `mcp-http-endpoint`
publication. `docs/operator/BRIEF.md` notes that the `agent-loop`
supervisor is now a PTY bootstrapper that lets an agent connect to
HTTP/SSE MCP natively. `docs/HOW_TO_AGENT.md` still teaches the
CLI-driven loop and the supervised-wrapper flow, with daemon MCP /
chat tools listed as a parallel operator-side affordance. This is not
a regression — the supervised-wrapper flow is still part of the
contract and is what most existing agents use — but `HOW_TO_AGENT.md`
will need an "MCP-native agent" section before the CLI workflow-control
verbs can be retired (per the BRIEF.md hazard). Track as a follow-up,
not a blocker.

### F5 (info) — Authority matrix correctly classifies survivors, but the legacy SQLite footprint is now invisible in this document set

`docs/architecture/COMMAND_AUTHORITY_MATRIX.md` shows that every active
RPC method has a `real` Go handler and `no` CLI fallback; SQLite
dependency is `no` for production methods. The
`Direct PostgreSQL Bootstrap/Admin Plane` table names the only
allowed direct-PostgreSQL callers, and the CLI-Only Or Out-Of-Band
table classifies CLI survivors. RFC 0068 § Retirement Gate says
"the production daemon RPC retirement ledger is empty". Within the
input set, the cleanup is complete.

The remaining `striatum.legacy_sqlite` quarantine, in-memory unit
fixtures, and one-way migration fixtures named in `docs/TODO.md`
items 61–62 and `docs/ROADMAP.md` § 5.9 step 13 are not visible in
the authority matrix because they sit outside the production
authority surface. This is the right modeling choice for a matrix
that records production authority. Operators should not interpret
the matrix's silence as "all legacy SQLite is gone"; the TODO/ROADMAP
status of 🟡 on items 61–62 is the authoritative signal for the
remaining cleanup. No regression — just a reminder to keep TODO and
ROADMAP as the source of truth on residuals.

## What I Looked For And Did Not Find

- **Hosted-service or telemetry creep**: none in the input set. RFC 0068
  § Non-Goals and the cleanup-touched docs preserve the local-first
  product boundary.
- **`api.invoke` reintroduced as a production authority**: not in this
  set. `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` Finding 7 and
  `docs/SPEC.md` § Local API And MCP Boundary both name `api.invoke` as
  legacy/authoring-only, and D117 records the wrapper removal.
- **SQLite fallback resurrection in production paths**: not in this set.
  D110 / D112 / D113 / D114 / D117 each name a SQLite removal step, and
  the authority matrix shows `no` CLI fallback across all production
  methods.
- **Operator-on-behalf scope creep**: not in this set. `docs/SPEC.md`
  artifact-publication invariants and the operator brief preserve the
  `--allow-no-process-execution --override-rationale` requirement for
  model-bylined operator publishes.
- **Human-principal-as-default regression**: not in this set. D103, the
  glossary `operator` / `human principal` entries, and
  `docs/HOW_TO_AGENT.md` consistently make the AI operator the default
  driver and the principal escalation-only.
- **CLI retirement happening before MCP/UI parity**: not in this set.
  `docs/operator/BRIEF.md` explicitly forbids it; `docs/ROADMAP.md`
  § 1.1 step 6 keeps the retirement gated on parity.

## Suggested Disposition

`accept_with_findings`. F1 and F2 are bounded documentation polish items;
F3, F4, and F5 are forward pointers. None of them block the cleanup or
indicate a missing boundary control.

Recommended follow-ups (not required for accept):

1. Refresh `README.md` § Project Status to `v1.57.0` and update the
   "Active RFCs" row to reflect what `docs/ROADMAP.md` § 5.1 has
   already closed (F1).
2. Add a one-sentence forward pointer in `docs/SPEC.md` § CLI noting
   that the daemon-MCP-mandatory contract from D104 is the operator
   control plane and that the CLI verb list is the current — not
   long-term — operator surface (F2).
3. When the next cleanup ships, consider including `docs/rfcs/0050-
   go-daemon-http-sse-mcp.md` and the two `docs/operator/plans/...`
   documents in the architecture-boundary review's input set so
   `docs/operator/BRIEF.md` scope_links can be verified at review time
   (F3).
