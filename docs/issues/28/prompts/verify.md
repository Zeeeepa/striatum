# Verify -- GH #28

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/28/SPEC.md`
- `docs/issues/28/SCOPE.md`
- `docs/issues/28/build/HANDOFF.md`
- The changed files named by the handoff

## Output

Write `docs/issues/28/review/REVIEW.md` with `striatum.finding.v1` front matter.

Include:

- Final verdict (`accept`, `accept_with_findings`, `needs_revision`, or `reject`);
- Per-bullet acceptance verification with file:line evidence;
- Verification checklist:
  - **Go template catalog & validator parity**: verify that `agy_default` exists in the template catalog and tool families map, and that invalid `model` inputs on lanes are rejected.
  - **Tmux supervision**: verify that the supervisor successfully spawns command execution in named tmux sessions, captures standard output streams, and cleans them up.
  - **Interactive MCP Loop**: verify that the `await_packet` RPC endpoint supports long-polling properly, and that the Python MCP wrapper routes packets correctly.
  - **Stateful Escalation Inbox**: verify that the inbox migration 0011 executes correctly, and that the table correctly tracks escalations and states.
- Test/verification assessment;
- Findings with severity and exact remediation when any gap remains.
