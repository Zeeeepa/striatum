# Design review — RFC 0088 P2+P3 (interrogating panel)

Read TASK.md + RFC 0088 + .../artifacts/DESIGN_SYNTHESIS.md. Posture from
the packet. You MUST interrogate the live synthesizer before your verdict
(interrogation.open -> interrogation.ask -> poll interrogation.show for
the interrogation_answer turn -> interrogation.close; <=3 rounds, exit
early when resolved). Write
.../artifacts/review/design/<lane>/REVIEW.md with finding front matter
(schema_version striatum.finding.v1, artifact_kind finding, verdict_intent,
severity, tags) + author byline reviewer-<lane>-001, stating rounds used +
stop reason. Finalize with ONE review.submit (session_id, job_id,
lease_id, logical_name, kind, path, verdict). threat_model: forgery
surface (claude bundle reuse for agy: does it leak the wrong skills?),
deletion safety (no remaining caller of turn_driver / --print wrapper /
single_shot), codex-submit-driver robustness (TUI dialogs, version
fragility). ergonomics_dx: per-adapter submit-driver structure (DRY across
claude/agy/codex); rollback story if codex agent_loop is flaky; failure
UX when an adapter's PTY launch dies (the stderr-capture hook from P1).
