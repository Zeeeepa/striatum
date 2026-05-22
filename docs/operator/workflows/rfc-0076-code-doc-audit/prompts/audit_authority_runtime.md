# Audit Authority Runtime

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`. Stay within the packet's write scope.

Audit whether current source behavior preserves Striatum's authority
model:

- daemon-owned PostgreSQL is live state;
- repository files are provenance, not the message bus;
- repo-local SQLite, marker files, tmux panes, terminal output, provider
  hooks, and transcripts do not become authority;
- MCP/RPC methods, capability tokens, denial vocabulary, leases,
  heartbeat, stale-lease recovery, run state, artifact validation,
  byline rules, and front-matter schemas fail closed;
- Go/Python daemon-transition claims match current behavior;
- tests pin the authority boundary;
- examples do not teach retired behavior as current practice.

Produce evidence-backed findings with stable `AUD-###` ids. Each material
finding must include severity, category, status, claim, evidence, impact,
recommended action, and follow-up path. Prefer source, tests, generated
contracts, and daemon metadata over prose. Downgrade unevidenced concerns
to observations or open questions.

Preserve historical fixtures and dogfood records as provenance. Flag only
current claims or examples that mislead operators about live behavior.
