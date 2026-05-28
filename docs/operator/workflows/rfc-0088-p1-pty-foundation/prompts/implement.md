# Implementation — RFC 0088 P1 (interrogable)
Read TASK.md, RFC 0088, .../artifacts/DESIGN_SYNTHESIS.md, and the design-panel
findings. Implement the smallest converged scope in Go: (1) an interactive-PTY launch
path that submits the bootstrap + per-turn prompts via the PTY master to claude in
interactive mode (no -p); (2) owned-PTY persistent sessions earn
author:<role>-<model>-<ordinal> via pid + command-snapshot match
(sessionLaneAttestation / artifactAuthorIdentity / laneAttestation), with a guard that
a mismatched pid identity or command still yields author:operator. Add tests: a submit
driver test using a FAKE/echo TUI binary (no live model), and an attestation-derivation
test (owned-PTY -> lane byline; mismatch -> operator). Do NOT delete the --print wrapper
or the turn-driver (that is P3). Run `cd go && gofmt -l . && go vet ./... && go test ./...`.
Write .../artifacts/build/HANDOFF.md (what landed/deferred + exact verification commands,
incl. the live claude interactive-PTY proof). Stay live for build-review interrogation.
Emit submit-handoff when done.
