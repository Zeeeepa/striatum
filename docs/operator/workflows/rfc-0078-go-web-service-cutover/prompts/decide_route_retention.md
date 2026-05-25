# Decide Route Retention

You are Worker 2's route decision lane for the RFC 0078 Go web/service
cutover. Stay within the job write scope.

Read the plan, RFC 0078, current operator brief, `src/striatum/service.py`,
`src/striatum/web/`, and the current Go daemon HTTP/MCP/RPC packages.

Produce
`docs/operator/artifacts/rfc-0078-go-web-service-cutover/routes/ROUTE_LEDGER.md`
with author line:

`author: operator [self-declared: route-decider-codex-gpt-5-002]`

The ledger must include:

- every current Python service/web route or route family you can identify;
- classification `port`, `retire`, `blocker`, or `historical_exception`;
- the source files proving the route exists;
- the Go replacement target or the retirement rationale;
- required route tests or guardrails;
- any route whose behavior depends on Python-only templates, static files,
  mutable service state, or compatibility-only dogfood history;
- a statement that daemon-owned PostgreSQL/RPC remains the live authority.

Do not implement code in this job. Do not use terminal output, tmux panes, or
transcripts as workflow authority.
