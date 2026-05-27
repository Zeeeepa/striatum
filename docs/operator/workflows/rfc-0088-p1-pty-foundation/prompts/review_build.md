# Build review — RFC 0088 P1 (interrogating panel)
Read TASK.md + RFC 0088 + .../artifacts/build/HANDOFF.md + the diff (supervisor/pty.go,
mutations/supervision_control.go, mutations/mutations.go, mutations/claim.go,
agentloop/*). Posture from the packet. Interrogate the live implementer before your
verdict (open -> ask -> poll interrogation.show -> close; <=3 rounds). Run the
verification commands incl. `cd go && go vet ./... && go test ./...` and, if feasible,
the live claude interactive-PTY proof. Write .../artifacts/review/build/<lane>/REVIEW.md
with finding front matter + byline + rounds/stop-reason. Finalize with ONE review.submit.
threat_model: owned-PTY byline cannot be earned by an unowned/forged process; no
control-state leak to the child; the submit channel cannot be driven by the child.
ergonomics_dx: a real interactive claude lane works without -p and publishes a model
byline; clear failure when the agent never becomes ready.
