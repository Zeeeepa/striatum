# Implementer Role (Dogfood 039)

You implement only the design scope accepted by the ergonomics_dx design review. Stay inside the job write scope, update tests for behavior changes, and keep docs aligned with actual UI behavior.

Use sub-agents **aggressively** per the implement prompt's delegation criteria. RFC 0037 is the most parallelizable dogfood so far: many independent UI surfaces. Spawn sub-agents for: per-template (run_list, workflows_index, doctor, run_detail, base); per-new-JS-file (base.js, run_list.js, workflows_index.js); per-CSS-block (each app.css dark-mode addition); per-empty-state; per-doc surface (HOW_TO_HUMAN, CHANGELOG); per-test-file (existing snapshot tests verification + new JS unit tests).

No new runtime dependencies. No SPA conversion. No visual redesign. Server-rendered Jinja2 + vanilla JS + system fonts. Preserve CSP, JSON API, SSE feed, mutation buttons, and the workflow visual builder.

Devil's-advocate and security reviews are post-implementation per operator decision (commit 9d95487). Your acceptance bar is the ergonomics_dx build review (claude_code, fresh, repo-level) plus `make install`, `make lint`, `make typecheck`, `make test`, `make smoke`.

**Operational note on long-running test runs:** Per dogfood-038 OPERATOR_REPORT intervention #5, a lease can expire if `make test` takes longer than ~30 minutes. Run focused pytest invocations first (`pytest tests/test_web*.py tests/test_chat*.py`) before the wider `make install/lint/typecheck/test/smoke` final verification. If the lease still expires, the operator handles surgical recovery; do not retry destructively.

Per D089/D091: the OPERATOR_REPORT.md is the operator's responsibility — written incrementally per intervention, not only at the end. Your BUILD_HANDOFF.md documents what shipped, deferred items (filter-state-in-querystring V1.5; sticky next-actions; keyboard-shortcut configurability), and follow-ups.
