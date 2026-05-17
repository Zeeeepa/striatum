# Track review prompt - dogfood 065

Produce the review artifact path declared in the work packet as a finding
artifact with valid `striatum.finding.v1` front matter. Set
`verdict_intent` honestly.

Use the work packet's expected `author:` line exactly.

Review only the assigned track. Do not review by trusting the handoff alone:
inspect the changed files inside that track's write scope and the relevant
tests. Treat cross-track edits as findings even if the implementation is
otherwise good.

Mandatory checks for all tracks:

1. The track stayed inside its write scope and did not touch `.striatum/`,
   `docs/dogfood/065/README.md`, `docs/dogfood/065/OPERATOR_REPORT.md`,
   workflow, prompts, or roles.
2. The handoff lists changed files and tests run.
3. Any deferred risk is explicitly named rather than hidden as "future work".
4. The implementation preserves daemon-owned PostgreSQL as live state.
5. No production path opens or creates repo-local SQLite.

Track-specific checks:

- Track A: Go schema/method/migration freshness, non-skipping CORE=go gate,
  audit-chain continuity, capability denial parity.
- Track B: production `connect_registry()` tripwire, PG-only dashboard/sweep/MCP
  resource paths, migration/test-only SQLite exceptions.
- Track C: daemon-owned repo resolution, `/v1/invoke` daemon routing, local API
  quarantine, production MCP tool list correctness, generated contract parity.
- Track D: D105/D107 coherence, RFC/doc status consistency, no over-claiming
  Go parity.

Use `needs_revision` for any authority regression, scope violation, silent test
skip, or false production-complete claim.
