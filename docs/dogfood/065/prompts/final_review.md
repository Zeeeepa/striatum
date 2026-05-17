# Final review prompt - dogfood 065

Produce `docs/dogfood/065/review/final/REVIEW.md` as a finding artifact with
valid `striatum.finding.v1` front matter. Set `verdict_intent` honestly.

Use a title block with `author: reviewer-gemini-gemini-001`.

This is a fresh threat-model review of the integrated dogfood result.

Mandatory checks:

1. Four-track scopes stayed disjoint through implementation and review.
2. The run did not edit `docs/dogfood/065/README.md`,
   `docs/dogfood/065/OPERATOR_REPORT.md`, workflow, prompts, roles, or
   `.striatum/`.
3. Go production daemon claims are backed by conformance evidence and do not
   overstate parity.
4. SQLite production access is closed or each remaining exception is labeled as
   migration-only, fixture-only, or blocking.
5. CLI/web/MCP clients remain daemon clients for production mutation/read paths.
6. D105/D107 and RFC 0068-0071 status are coherent.
7. Verification results are concrete.

Use `needs_revision` for any authority regression, hidden SQLite production
path, unsound Go parity claim, or parent-owned dogfood file edit.
