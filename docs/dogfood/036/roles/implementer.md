# Implementer Role (Dogfood 036)

You implement only the design scope accepted by the ergonomics_dx design review. Stay inside the job write scope, update tests for behavior changes, and keep docs aligned with actual runner behavior.

Use sub-agents aggressively per the implement prompt's delegation criteria. RFC 0034 V1 is medium in scope. Parallelism is most useful for: generator core (value objects + envelope + validation-on-return), shape compilers (one per built-in shape), lane-set compilers (one per built-in lane set), lane-modifier matrix (validation rules per modifier × lane set), catalog package-data + loader, CLI verbs (`workflow templates list/show`, `workflow generate`), local service endpoints (read + mutation-gated preview/write), custom-plan compiler with closed block vocabulary, `workflow init --style` rewiring, and unit test files.

The web `/workflows/new` chooser UI and the chat-assisted scaffolding tool are EXPLICITLY DEFERRED to a follow-up dogfood. Do not author them here. Document the deferred UI + chat work in the BUILD_HANDOFF with a clear pointer to the follow-up dogfood.

Your acceptance bar is the ergonomics_dx build review (claude_code, fresh, repo-level) plus `make install`, `make lint`, `make typecheck`, `make test`, `make smoke`.

Per D089: the OPERATOR_REPORT.md is the operator's responsibility — not yours. Your BUILD_HANDOFF.md should clearly document what shipped, what deferred, what remains for the follow-up dogfood.
