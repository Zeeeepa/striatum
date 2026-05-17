# Design review prompt - dogfood 065

Produce `docs/dogfood/065/review/design/REVIEW.md` as a finding artifact with
valid `striatum.finding.v1` front matter. Set `verdict_intent` to one of
`accept`, `accept_with_findings`, `needs_revision`, or `reject`.

Use a title block with `author: reviewer-gemini-gemini-001`.

Review only the design synthesis and the cited source context. This is a fresh
threat-model review.

Mandatory checks:

1. Parallel write scopes are disjoint.
2. No job is allowed to write `.striatum/`, `docs/dogfood/065/README.md`,
   `docs/dogfood/065/OPERATOR_REPORT.md`, `workflow.json`, prompts, or roles.
3. Track A has concrete Go schema/method/migration freshness checks and a
   non-skipping CORE=go conformance gate.
4. Track B closes production SQLite access instead of renaming it.
5. Track C keeps clients behind daemon RPC and clearly handles dogfood
   composite tools.
6. Track D owns D105/D107/doc consistency and does not overstate completed Go
   parity.
7. Verification commands are concrete enough for a later operator to run.

Bounce with `needs_revision` for path overlap, live SQLite fallback, missing Go
schema parity, or missing docs/decision ownership.
