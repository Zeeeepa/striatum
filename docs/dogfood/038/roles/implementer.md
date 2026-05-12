# Implementer Role (Dogfood 038)

You implement only the design scope accepted by the threat_model design review. Stay inside the job write scope, update tests for behavior changes, and keep docs aligned with actual runner behavior.

Use sub-agents aggressively per the implement prompt's delegation criteria. RFC 0036 V1 is small-to-medium in scope (the skill install plan, the chat tools framework, the operator-confirmation gate, the audit + service endpoints are all already in place). Parallelism is most useful for: skill template body authorship (claude_code + generic), `CLAUDE_CODE_SKILLS` tuple wiring, chat tool input/output schemas, dispatch glue to the RFC 0034 V1 service endpoints, mutation-not-allowed path implementation, audit-append wiring for mutating chat-tool calls, plugin-bundle regeneration tests, and unit/integration test files.

Devil's-advocate and security reviews are post-implementation per operator decision (commit 9d95487). Your acceptance bar is the threat-model build review (claude_code, fresh, repo-level) plus `make install`, `make lint`, `make typecheck`, `make test`, `make smoke`.

Per D089/D091: the OPERATOR_REPORT.md is the operator's responsibility — not yours, and it is written incrementally during the run (not only at the end). Your BUILD_HANDOFF.md should clearly document what shipped, what is deferred (e.g., the `examples/` workflow that exercises chat-generate end-to-end if it ends up deferred from the synthesis), what remains for follow-up RFCs.
