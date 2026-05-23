---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/architecture/CLI_RETIREMENT_PARITY.md", "docs/operator/BRIEF.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md"]
---

# RFC 0050 / RFC 0075 Cutover Gap Map
author: operator

## Current Facts

The native Go daemon MCP path exists and is the lane-agent control-plane
target. The Python MCP wrapper is already gone. The fake MCP agent loop,
`session.report`, activity/liveness projections, tmux attach metadata, and
fail-closed tmux opt-in have landed.

The remaining cutover blockers in the parity ledger are policy and surface
classification issues, not missing daemon methods. MCP dispatch is exact for
the live-control methods in the ledger.

## Gap Classes

Lane-agent MCP-only actions: `session.register`, `session.close`,
`work.await_packet`, `work.ack`, `work.heartbeat`, `work.release`,
`work.send_message`, `work.block`, `work.complete`, `artifact.publish`,
`review.submit`, `worktree.create`, and `worktree.release`.

Human/operator UI actions: local commit apply, recovery inspection and
recovery transitions, cross-repo cancel, escalation/checkpoint/decision/run
lifecycle actions, and operator override flows.

Bootstrap/admin survivors: repository registration/removal and daemon admin
commands.

Diagnostics/read survivors: status, dashboard, doctor, run/job/artifact reads,
supervisor reads, and cross-repo reads.

Compatibility survivors: CLI verbs can remain for scripts, debugging, and
manual recovery, but docs and skills must not present them as the normal lane
loop after the cutover.
