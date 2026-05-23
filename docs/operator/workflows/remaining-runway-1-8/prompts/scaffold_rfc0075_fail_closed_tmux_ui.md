# Scaffold RFC 0075 Fail-Closed Tmux And UI Polish

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on the RFC 0075 work that remains after `session.report`, MCP activity
liveness, and tmux attach metadata slices: fail-closed tmux requirements for
live interactive lanes and operator UI polish.

The scaffold must include:

- the live-interactive lane condition that requires daemon-created tmux-backed
  PTY supervision;
- clear fail-closed behavior when `tmux` is unavailable, including remediation
  text and fixture/headless carve-outs;
- UI/status/dashboard expectations for attach commands and liveness classes;
- no-transcript/no-pane-authority tests and review points;
- implementation write scopes, serialization points, and focused tests;
- explicit non-scope for parsing terminal text or publishing transcripts.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
