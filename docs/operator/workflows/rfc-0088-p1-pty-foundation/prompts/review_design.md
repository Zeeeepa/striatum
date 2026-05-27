# Design review — RFC 0088 P1 (interrogating panel)
Read TASK.md + RFC 0088 + .../artifacts/DESIGN_SYNTHESIS.md. Posture from the packet.
You MUST interrogate the live synthesizer before your verdict (interrogation.open ->
interrogation.ask -> poll interrogation.show for the interrogation_answer turn ->
interrogation.close; <=3 rounds, exit early when resolved). Write
.../artifacts/review/design/<lane>/REVIEW.md with finding front matter (schema_version
striatum.finding.v1, artifact_kind finding, verdict_intent, severity, tags) + author
byline reviewer-<lane>-001, stating rounds used + stop reason. Finalize with ONE
review.submit (session_id, job_id, lease_id, logical_name, kind, path, verdict).
threat_model: does the owned-PTY byline widen the fabrication surface vs the wrapper?
is the disqualification guard (pid identity, command-snapshot) sound; PTY/stdin leak risk.
ergonomics_dx: submit-sequence robustness across claude versions; failure UX when submit
or readiness detection fails.
