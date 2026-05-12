# Synthesize Design Prompt

Produce `docs/dogfood/037/DESIGN_SYNTHESIS.md`. The file must start with a `striatum.synthesis.v1` front matter block (JSON-encoded values; quote strings; JSON arrays for lists):

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/037/design/codex/DESIGN.md", "docs/dogfood/037/design/claude_code/DESIGN.md", "docs/dogfood/037/design/gemini/DESIGN.md"]
---
```

The byline appears AFTER the front matter block, as a plain Markdown line `author: <slug>` (lowercase, no bold/italics/heading/quotes).

Read all three design artifacts and synthesize ONE implementation plan for RFC 0035 (multi-repo test harness for cross-repo workflows). The synthesis must explicitly choose, not enumerate.

Required sections:

- **Accepted Implementation Scope** — map each RFC 0035 §Acceptance Criteria bullet 1:1 to a concrete code-and-test plan, with one named owner per bullet (which `tests/_harness/` module, which e2e test file).
- **Deferred Scope** — Go-client testing surface (RFC 0035 §Open Questions; D084 future), two-repos-with-worktree-isolated-lanes example workflow (follow-up), Docker-based ephemeral Postgres (separate hardening RFC), Windows daemon harness (RFC 0030 V2 scope), cross-machine testing (D083), performance/load testing. Each line says why deferred and where it lands.
- **Harness Module Layout** — concrete file tree under `tests/_harness/`.
- **`MultiRepoHarness` API** — exact method signatures (`__init__`, `start`, `stop`, `reset_daemon_db`, `register_all`, `issue_token`, `mcp_client`, `audit_rows`, `daemon_db_query`, `repo_sqlite_query`, etc.).
- **Fixture Scope** — per-class default + per-function escape hatch via `clean_daemon_db`. State which tests use which.
- **Per-Test DB Reset Semantics** — exact tables truncated (all daemon DB tables except `schema_version`); explicit non-reset of repo registration; per-repo SQLite state cleared per-class with the harness scratch dir.
- **The Five E2E Test Files' Case Lists** — for each of prepare/lifecycle/crash-recovery/MCP-capability-scope/per-repo-write-scope: exact test case names and what they assert.
- **Harness Smoke Test** — `tests/test_multi_repo_harness.py` case list.
- **CI Integration** — `make test-multi-repo` recipe shape; skip-with-message logic when PG is unavailable; CI matrix coverage.
- **Wall-Clock Budget** — < 60s added to local `make test`; per-class scope amortizes daemon startup.
- **Determinism + Cleanup Hygiene** — ephemeral PG dropped, scratch dir removed, Unix socket deleted on stop; SIGTERM + timeout; back-to-back harness instances work.
- **Cross-Platform** — Linux + macOS only; Windows daemon out of scope.
- **No Parallel Production-Code Path** — confirm same daemon binary, same migrations, same RPC envelope, same capability vocabulary, same audit chain helper.
- **Adversarial Test Cases** — the closed set of hostile/scope-mismatch/expired-token/revoked-token/audit-tamper cases.
- **Documentation Deltas** — TODO Open item 19 marked done; SPEC (or HOW_TO_HUMAN if developer-only) note pointing at the harness; CHANGELOG Unreleased entry.
- **Staging Plan** — Step 1 harness skeleton + smoke test; Step 2 prepare + lifecycle e2e; Step 3 crash recovery e2e; Step 4 MCP capability scope e2e; Step 5 per-repo write-scope e2e; Step 6 CI wiring + docs.
- **Human-Decision Questions** — any open questions the implementer cannot resolve from the synthesis alone.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim AFTER the front matter and a blank line.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
