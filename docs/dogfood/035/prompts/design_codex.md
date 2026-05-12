# Codex Design Prompt

Produce `docs/dogfood/035/design/codex/DESIGN.md`.

Design an implementation plan for RFC 0032: cross-repository workflows + MCP mutation capabilities. Sit on top of dogfood-034's daemon RPC + supervision + sealed-apply foundation (`src/striatum/daemon_rpc/`, `daemon_apply/`, `daemon_supervisor/`, the daemon Postgres substrate from dogfood-033). Do not redesign those.

Your plan must cover:

**Workflow schema (cross-repo):**
- `repositories` top-level block: registered `repo_id` references, primary repository declaration, validator rules
- per-job `repository` field; jobs without it target the primary
- write-scope enforcement when a job targets a non-primary repo
- cross-repo `edges` and `cycles` semantics (max_iterations is global to a cycle, not per-repo)
- workflow validator extensions in `src/striatum/workflow.py`

**Cross-repo run lifecycle:**
- `run prepare` against a cross-repo workflow: daemon-mediated atomic creation of one `cross_repo_runs` row + N local repo `runs` rows
- best-effort consistency on daemon crash between daemon-DB write and local-SQLite writes; daemon-startup reconciliation
- `run start`, `run summary`, `run cancel`, `dashboard --run-id <cross_repo_run_id>` semantics
- per-repo `local_run_id` ↔ `cross_repo_run_id` mapping

**MCP mutation capability expansion:**
- MCP `tools/call` wired to the RFC 0030 method registry
- per-token `tools/list` filtering: token-effective-tool-set = method registry ∩ token capabilities
- default-deny: unknown method → standard MCP unknown-method error, audited with documented denial vocabulary
- capability scope: `repo_id`-scoped vs daemon-global; a write-token scoped to repo A cannot call write-paths against repo B
- audit row appended for every mutating `tools/call` including denials, with `capability_missing` / `token_revoked` / `token_expired` denial reasons

**Concrete touch points in `src/striatum/`:**
- `workflow.py` (validator extensions for `repositories` block)
- `daemon_rpc/registry.py` (route map updates)
- `daemon_rpc/server.py` (cross-repo run lifecycle handlers)
- `mcp.py` (capability-gated `tools/call` + `tools/list` filter)
- `migrations.py` and `daemon_pg/migrations.py` if any new tables needed (`cross_repo_runs`, etc.)
- `daemon_pg/sql/0003_cross_repo.sql` if a substrate migration is required

**Multi-repo / cross-repo END-TO-END integration testing is DEFERRED** to a follow-up RFC (`docs/TODO.md` Open item 19, multi-repo test harness). Your design must explicitly list the test coverage strategy:

- unit tests for each new module
- mock-based tests for daemon-mediated coordination paths
- schema/validator tests for the `repositories` workflow block
- per-repo write-scope enforcement tests using mocked registered repos
- MCP capability + `tools/list` filtering tests

Do not design a multi-repo daemon harness. Note the deferred coverage in your DESIGN.md so the synthesis can carry it forward.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value.
- Lowercase `author:` exactly.
- Correct: `author: designer-codex-gpt-5.5-001`
- Wrong: `**Author:** ...`, `Author: ...`, `# author: ...`, `author: "..."`.

The `handoff` artifact kind does not require YAML front matter. Synthesis and finding artifacts later in this dogfood will, with the JSON-encoded block shown in `synthesize_design.md` / `review_design.md`.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
