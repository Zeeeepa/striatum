# Build review — F44 (interrogating panel)
Read TASK.md + .../artifacts/build/HANDOFF.md + the diff (supervision_control.go,
turn_driver.go, turndriver/loop.go). Posture from the packet. Interrogate the live
implementer before your verdict (open -> ask -> poll interrogation.show -> close;
<=3 rounds). Run the verification commands incl. `cd go && go test ./...`. Write
.../artifacts/review/build/<lane>/REVIEW.md with finding front matter + byline +
rounds/stop-reason. Finalize with ONE review.submit. threat_model: exec resolution
safety, no control-state leak to the child, crash-safety of the loop. ergonomics_dx:
operator no longer needs the systemd drop-in; clear escalation on generator failure.
