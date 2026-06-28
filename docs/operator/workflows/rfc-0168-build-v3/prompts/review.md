Review the draft implementation against the accepted RFC 0168 and D272 context
and the v3 final-v2-findings context.

Return `needs_revision` for any open security-boundary gap, placeholder proof,
missing typed refusal, over-broad refusal of legitimate non-credential lane
environment, stale docs, schema or owner-bundle collision, or missing focused
test for the gate being claimed.

This review must explicitly verify the final v2 blockers:

- F1: uid return is blocked until complete S1-S3 and P1-P5 proof exists; kill
  failure fail-closes or quarantines; cleanup absence is proved after cleanup;
  P4 and P5 are real checks; quarantined or stuck-state retry reruns P1-P5.
- F2: `supervise.report` checks live lane uid generation before heartbeat or
  terminal metadata updates and has stale-generation negative tests.
- F3: relative provider credential selectors resolving in-repo fail closed,
  including relative `CLAUDE_SECURESTORAGE_CONFIG_DIR` and
  `ANTHROPIC_CONFIG_DIR`, while ordinary relative non-credential env remains
  allowed where intended.

Also confirm accepted v2 work stayed intact: RFC 0171 files are preserved, MCP
bearer material remains under supervisor scratch, selected run-as uid access to
worktrees and workspaces is explicit, and positive controls for `AGY_HOME` or
`FIXTURE_CONFIG_DIR` still launch.

Publish exactly one REVIEW.md finding artifact with a clear verdict, concrete
file and line evidence, and commands run with results. If any final v2 blocker
is still open, the verdict must be `needs_revision`.
