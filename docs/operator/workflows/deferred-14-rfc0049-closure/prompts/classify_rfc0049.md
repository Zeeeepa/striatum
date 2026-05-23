# Classify RFC 0049

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Classify deferred item 14 / RFC 0049 interactive Claude lane via MCP as
reopened, closed, or shelved.

Required inputs:

- `AGENTS.md`
- `docs/operator/BRIEF.md`
- `docs/TODO.md` item 37
- `docs/ROADMAP.md` RFC 0049 section
- `docs/rfcs/0049-interactive-claude-lane-mcp-control-plane.md`
- current RFC 0050 MCP and RFC 0075 tmux-observable MCP-session docs
- `docs/MCP.md`
- current source/tests for `work.await_packet`, `session.report`,
  agent-loop bootstrap, tmux attach metadata, and the Claude supervised wrapper

Checks:

- Determine whether D106's reopen conditions are satisfied.
- Separate generic landed MCP/tmux prerequisites from RFC 0049's
  Claude-specific capability experiment.
- Confirm whether the current wrapper still uses per-packet `claude --print`.
- Confirm whether `long_lived`, `fresh_strategy`, or equivalent
  Claude-specific lifecycle fields are implemented outside the RFC text.
- Run focused validation that proves the scaffold is valid and the relevant
  MCP/tmux/wrapper guardrails still pass.

Publish
`docs/operator/artifacts/deferred-14-rfc0049-closure/classification/REPORT.md`
as a `striatum.synthesis.v1` artifact with evidence, commands, and a
classification verdict. Do not edit shared TODO, roadmap, brief, RFC, or
decision-log files.
