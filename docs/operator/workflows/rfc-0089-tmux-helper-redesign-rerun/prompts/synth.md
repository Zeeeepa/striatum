# Design synthesis - RFC 0089 tmux helper redesign

Read `TASK.md`, RFC 0089, and all three designs under
`docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun/artifacts/design/`.

Write one buildable synthesis at
`docs/operator/workflows/rfc-0089-tmux-helper-redesign-rerun/artifacts/DESIGN_SYNTHESIS.md`.
Pick a concrete approach, not a menu. Include:

- launch metadata shape for tmux-backed lanes;
- tmux liveness probe semantics and failure classes;
- changes to status, delivery reconciliation, doctor/dashboard/recovery, and
  stop behavior;
- exact tests and fixtures;
- exact doc updates, if any;
- what remains out of scope after Phase 1.

This node is interrogable. After publishing, remain live and answer adversarial
review questions from your own design reasoning. Publish the synthesis artifact
and complete.
