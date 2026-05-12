# Synthesize Design Prompt

Produce `docs/dogfood/035/DESIGN_SYNTHESIS.md`. The file must start with a `striatum.synthesis.v1` front matter block (JSON-encoded values; quote strings; JSON arrays for lists):

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/035/design/codex/DESIGN.md", "docs/dogfood/035/design/claude_code/DESIGN.md", "docs/dogfood/035/design/gemini/DESIGN.md"]
---
```

The byline appears AFTER the front matter block, as a plain Markdown line `author: <slug>` (lowercase, no bold/italics/heading/quotes).

Read all three design artifacts and synthesize ONE implementation plan for RFC 0032 (cross-repository workflows + MCP mutation capabilities). The synthesis must explicitly choose, not enumerate.

Required sections:

- **Accepted Implementation Scope** — map each RFC 0032 §Acceptance Criteria bullet 1:1 to a concrete code-and-test plan, with one named owner per bullet (which `src/striatum/` module, which test file).
- **Deferred Scope** — multi-repo / cross-repo END-TO-END integration tests (deferred to TODO Open item 19, multi-repo test harness RFC), cross-machine multi-tenant semantics (no follow-up planned; deferred indefinitely), Python → Go port (D084 future), bundled / Dockerized Postgres (RFC 0033 follow-up). Each line says why deferred and where it lands.
- **`repositories` Workflow Block Schema** — concrete validator shape, primary-repository declaration, per-job `repository` field rules, cross-repo edges/cycles semantics, max_iterations global-to-cycle accounting.
- **Cross-Repo Run Lifecycle** — daemon-mediated atomic creation, two-phase commit semantics inside the daemon, crash reconciliation (preparing → started or aborted), `run start` / `run summary` / `run cancel` / `dashboard` semantics across participating repos, per-repo failure isolation (one repo unavailable mid-run pauses the run with a human checkpoint).
- **MCP Mutation Capability Wiring** — capability vocabulary maps to MCP `tools/call`; per-token `tools/list` filter computes effective tool set = method registry ∩ token capabilities; default-deny for unknown methods or missing capability; no global `--allow-mutations` flag.
- **Capability Token Lifecycle for Mutation** — issuance (admin-only), expiry (short-lived recommended for mutation; documented operator UX), revocation, rotation; how revocation races with in-flight calls.
- **Audit Shape for MCP Mutations** — audit row appended for every mutating `tools/call` including denials, with `capability_missing` / `token_revoked` / `token_expired` denial vocabulary; integration with RFC 0030's `rpc_request_log` + hash chain.
- **Daemon DB + Repo-Local DB Coordination** — `cross_repo_runs` table in daemon DB, `cross_repo_run_id` stored in each repo's `runs` row, transaction ordering, rollback on partial failure.
- **Schema Migration** — daemon Postgres migration v3 if any new tables are needed (`cross_repo_runs`, MCP audit additions); repo-local migration if `runs` table needs `cross_repo_run_id` column.
- **Test Strategy (with explicit deferral)** — unit tests, mock-based tests for daemon-mediated coordination, schema/validator tests, per-repo write-scope enforcement tests using mocked registered repos, MCP capability + `tools/list` filtering tests. **Multi-repo END-TO-END integration tests are deferred to TODO Open item 19** (multi-repo test harness RFC); the synthesis must list them as deferred coverage with a pointer to that follow-up.
- **Documentation Deltas** — SPEC / MCP / UBIQUITOUS_LANGUAGE / CLI_REFERENCE / HOW_TO_HUMAN / RFC 0032 status / CHANGELOG.
- **Staging Plan** — what lands in this dogfood vs deferred to a future dogfood.
- **Human-Decision Questions** — any open questions the implementer cannot resolve from the synthesis alone.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim AFTER the front matter and a blank line.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
