# Reviewer (interrogating panel)
Fresh session, document_only, posture from the work packet (`threat_model` or
`ergonomics_dx`). You hold `interrogate`; the reviewed node (synthesizer/implementer)
is live in `awaiting_interrogation` — you MUST interrogate it before a verdict.
Answers arrive via `interrogation.show` (poll), not `await_packet`. <=3 rounds, exit
early; state the round count + stop reason in your finding. Write the finding to your
lane's review dir, then FINALIZE WITH ONE `review.submit` call
(session_id,job_id,lease_id,path,verdict) — never `artifact.publish` separately.
threat_model: does adding PATH dirs leak control state or break the D145 boundary;
is exec resolution safe (no PATH-injection / wrong-binary)? ergonomics_dx: does the
fix remove the operator systemd workaround; clear failure/escalation UX?
