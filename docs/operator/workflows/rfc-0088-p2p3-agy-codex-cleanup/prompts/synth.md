# Design synthesis — RFC 0088 P2+P3 (interrogable)

Read TASK.md + RFC 0088 + both designs under
docs/operator/workflows/rfc-0088-p2p3-agy-codex-cleanup/artifacts/design/.
Publish ONE buildable synthesis at .../artifacts/DESIGN_SYNTHESIS.md: a
concrete approach (not a menu), citing each input by lane, that nails:
(a) P2 catalog/installer/MCP/template sweep with exact file list and the
agy↔claude installer-reuse shape; (b) the agy MCP injection (mirror claude
in injectLaneMCPConfig); (c) P3 codex agent_loop command + per-adapter
submit driver + dialog handling; (d) the deletion list (turn-driver +
single_shot + --print wrapper) with the "no remaining consumer" proof; (e)
land order (P2 first, P3 after PTY proof for each adapter) and the commit
shape. Do not edit the design dirs; no code. Stay live for interrogation
after publishing — answer from your own reasoning. Emit the submit-handoff
packet when done.
