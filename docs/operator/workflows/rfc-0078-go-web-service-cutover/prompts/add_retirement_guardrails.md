# Add Retirement Guardrails

You are the retirement-guardrail lane for the RFC 0078 local web/service
cutover. Stay within the job write scope.

Read the plan, route ledger if present, RFC 0078, current guardrail tests, and
the Python service/web modules.

Add the smallest useful guardrail slice that prevents retired service/web
surfaces from returning. Guardrails may be Go tests, shell checks, or static
checks, but they must be runnable in normal repository validation and must not
depend on transcripts or terminal scraping.

Guard against:

- importing Python service/web modules from current Go-only operator paths;
- restoring Python local service as a live state authority;
- adding route handlers that use repo-local SQLite, `.striatum/` live state,
  terminal output, or transcripts;
- keeping historical dogfood routes as current product routes without a route
  ledger exception;
- serving non-loopback or hosted semantics without a decision.

Produce
`docs/operator/artifacts/rfc-0078-go-web-service-cutover/guardrails/HANDOFF.md`
with author line:

`author: operator [self-declared: retirement-guard-codex-gpt-5-002]`

Include changed files, validation command results, remaining unguarded route
classes, and any recommended future decision.
