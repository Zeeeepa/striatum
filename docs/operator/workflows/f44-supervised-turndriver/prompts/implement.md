# Implementation — F44 (interrogable)
Read TASK.md, .../artifacts/DESIGN_SYNTHESIS.md, and the design-panel findings.
Implement the smallest converged scope in Go: (1) generator findable for
daemon-spawned supervised lanes (per synthesis), (2) generator failure routes
through OnFailure/park+escalate instead of crashing RunTurnDriver, (3) liveness if
in scope. Add turndriver/supervision tests; the generator-failure test must use a
missing/fake binary (no live gemini). Run `cd go && gofmt -l . && go test ./...`.
Write .../artifacts/build/HANDOFF.md (what landed/deferred, exact verification
commands incl. "remove path.conf drop-in and confirm a supervised gemini lane still
finds gemini"). Stay live for build-review interrogation. Emit submit-handoff when done.
