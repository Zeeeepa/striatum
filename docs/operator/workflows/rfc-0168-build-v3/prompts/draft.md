Read the workflow packet and all required context docs before editing.

Implement the RFC 0168 P0 blocker-closure build inside the declared write
scope, starting from current `origin/main`. Use canceled branch
`striatum/rfc-0168-build-v2` only as reference material; do not manually merge,
cherry-pick, or replay rejected code unless the final v2 blockers are fully
closed in this run.

This v3 build exists to close the final v2 security blockers:

- F1: uid return must wait for complete S1-S3 and P1-P5 proof. Kill failure
  must fail closed or quarantine; provider credential-store absence, tmux
  socket cleanup, HOME and reseal-token cleanup, per-lease ACL cleanup, and
  worktree cleanup must be proved before return; P4 and P5 cannot be deferred
  or unconditional; operator retry from quarantined or stuck states must rerun
  P1-P5 and return only on clean proof.
- F2: `supervise.report` must compare live lane uid generation against
  `lane_uid_leases` before heartbeat or terminal metadata updates. Add
  stale-generation negative tests for helper reports.
- F3: relative provider credential selectors must resolve against lane launch
  cwd or repo root and fail closed when in-repo. Cover relative
  `CLAUDE_SECURESTORAGE_CONFIG_DIR` and `ANTHROPIC_CONFIG_DIR`; keep ordinary
  relative non-credential env allowed where intended.

Preserve accepted v2 pieces that were correct: RFC 0171 files, runtime ordinal
frontier, owner bundle ordinal, supervisor-scratch MCP bearer path, absolute
provider credential directory guards, positive controls for `AGY_HOME` and
`FIXTURE_CONFIG_DIR`, and explicit selected run-as uid access to worktrees and
workspaces.

Publish the required DRAFT.md with a concise ledger: files changed, how each
final v2 blocker is closed, tests run, and residual operator work.
