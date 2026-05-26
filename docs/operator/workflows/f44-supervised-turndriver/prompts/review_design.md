# Design review — F44 (interrogating panel)
Read TASK.md + .../artifacts/DESIGN_SYNTHESIS.md. Posture from the packet. You MUST
interrogate the live synthesizer before your verdict (interrogation.open ->
interrogation.ask -> poll interrogation.show for the interrogation_answer turn ->
interrogation.close; <=3 rounds, exit early). Write .../artifacts/review/design/<lane>/REVIEW.md
with finding front matter (schema_version striatum.finding.v1, artifact_kind finding,
verdict_intent, severity, tags) + author byline reviewer-<lane>-001, stating rounds
used + stop reason. Finalize with ONE review.submit (session_id,job_id,lease_id,path,verdict).
threat_model: PATH-injection / wrong-binary risk, D145 boundary. ergonomics_dx:
does it remove the operator systemd workaround; failure UX.
