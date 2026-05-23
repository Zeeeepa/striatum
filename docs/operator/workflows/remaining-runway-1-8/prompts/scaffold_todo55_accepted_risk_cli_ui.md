# Scaffold TODO 55 Accepted-Risk CLI/UI Polish

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on the client polish that remains after the daemon accepted-risk
substrate landed: CLI and UI surfaces over `workflow.lint`,
`workflow.accept_risk`, and `workflow.accepted_risks.list`.

The scaffold must include:

- current-state assumptions from D124 and the operator brief;
- the smallest CLI polish slice for accept/list/display/refusal behavior;
- the smallest UI polish slice for showing lint risks and recording accepted
  risks with decision-artifact linkage;
- MCP/read-write parity notes when they affect client behavior;
- explicit non-scope for workflow-file metadata as live authority;
- implementation write scopes, serialization points, tests, and rollback;
- handoff notes for validation and boundary review.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
