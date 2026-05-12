# Implement Prompt

Implementation is blocked until `review_design_threat` returns an accepting verdict. Do not start implementation from RFC 0032 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/035/DESIGN_SYNTHESIS.md` and the resolved threat-model review findings. Stay inside the workflow write scope.

Expected behavior:

**RFC 0032 cross-repo + MCP mutation:**
- workflow `repositories` block validator extensions in `src/striatum/workflow.py`
- daemon-mediated cross-repo run lifecycle (`run prepare`/`start`/`summary`/`cancel`/`dashboard --run-id`)
- per-repo write-scope enforcement when a job targets a non-primary registered repo
- MCP `tools/call` wired to the RFC 0030 method registry with capability gating
- per-token `tools/list` filtering (effective tool set = method registry ∩ token capabilities)
- default-deny for unknown methods and missing capabilities
- audit row appended for every mutating `tools/call` (including denials) with the documented denial vocabulary
- cross-repo run lifecycle reconciliation on daemon crash (preparing → started or aborted)
- one-repo-unregistered-mid-run pauses the run with a human checkpoint (no data-loss-by-accident)
- daemon Postgres migration v3 if needed (`cross_repo_runs` table); repo-local migration if `runs` needs a `cross_repo_run_id` column
- documentation updates to SPEC, MCP, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, HOW_TO_HUMAN, RFC 0032 status, CHANGELOG, README

Do NOT:

- author a multi-repo / cross-repo END-TO-END integration test harness (deferred to TODO Open item 19, follow-up RFC for multi-repo test harness);
- ship cross-machine multi-tenant semantics (out of V2 scope);
- claim atomic file-system mutations across two repos, cryptographic non-repudiation, model-token authorship, or malicious-local-root resistance (RFC 0031 threat model is the AI-guardrail framing);
- add devils_advocate or security review jobs to this dogfood's workflow (deferred per operator decision in commit 9d95487);
- retire direct repo-local CLI mode (separate future RFC).

**Test coverage requirements (with explicit deferral):**

Ship:
- unit tests for each new module
- mock-based tests for daemon-mediated coordination paths
- schema/validator tests for the `repositories` workflow block
- per-repo write-scope enforcement tests using mocked registered repos
- MCP capability + `tools/list` filtering tests
- daemon-crash-mid-cross-repo-prepare reconciliation test (single-repo simulation acceptable; multi-repo END-TO-END deferred)

Defer (document in BUILD_HANDOFF):
- multi-repo END-TO-END integration tests against a real two-repo daemon harness
- cross-repo cycle accounting under real daemon coordination
- cross-platform path identity verification

The deferred coverage lands after the multi-repo test harness RFC (TODO Open item 19) is written and implemented. Document the deferred coverage clearly in `docs/dogfood/035/BUILD_HANDOFF.md` with a pointer to that follow-up.

## Maximize sub-agent usage where it helps

Per the harness profile, native sub-agent delegation is **encouraged**. Use it aggressively for the parts of this implementation that are independent enough to parallelize.

Spawn sub-agents in parallel for work that meets all of these:

- the sub-task can be specified by a self-contained brief (file paths, expected behavior, test fixtures, ~1 page of context);
- it does not depend on the in-flight output of another sub-agent;
- you (the parent session) can independently verify its output.

Good candidates in this implementation:

- one sub-agent per major module — workflow validator extensions, daemon RPC route map updates, cross-repo run lifecycle handlers, per-repo write-scope enforcement, MCP `tools/call` capability gate, per-token `tools/list` filter, audit append wiring for MCP mutations;
- one sub-agent per new test file — validator tests, capability gate tests, `tools/list` filter tests, write-scope enforcement tests, mocked daemon-coordination tests;
- one sub-agent per doc surface — SPEC section, MCP section, UBIQUITOUS_LANGUAGE entries, CLI_REFERENCE updates, HOW_TO_HUMAN cross-repo walkthrough, RFC 0032 status block update;
- exploratory sub-agents to read existing modules (`daemon_rpc/*`, `daemon_apply/`, `daemon_supervisor/`, `mcp.py`, `workflow.py`, `daemon_pg/*`) and produce one-page summaries of integration points.

Do NOT delegate (parent session owns these):

- the BUILD_HANDOFF.md authorship;
- the integration step where sub-agents' outputs are reconciled;
- any `make lint`/`typecheck`/`test`/`smoke` invocation;
- final commit-shape and scope discipline.

## Verification

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke` after all changes are in place.

## Handoff

Produce `docs/dogfood/035/BUILD_HANDOFF.md` summarizing changes, new modules, schema migrations, tests added/passing, deferred multi-repo coverage with pointer to the follow-up RFC (TODO Open item 19), and any human-decision items the threat-model review did not pre-resolve. If sub-agents were used, briefly note which sub-tasks were delegated.

The byline must be `author: implementer-codex-gpt-5.5-001` (or whatever the work packet supplies) — plain Markdown line, lowercase `author:`, no decoration.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
