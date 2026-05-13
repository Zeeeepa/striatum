# Synth — RFC 0039 V1.6

Reconcile the three design proposals at
`docs/dogfood/051/design/{codex,claude_code,gemini}/DESIGN.md`. Produce
a single synthesis at `docs/dogfood/051/DESIGN_SYNTHESIS.md` that:

- Picks one implementation approach per F (F-pty, F-pid-recycling,
  F-perms, F-store, F-ci) and names the reason for the choice.
- Lists exact files the implementer will touch and the order to touch
  them in.
- Names acceptance verifiers: Go-level tests in
  `go/pkg/supervisor/supervisor_test.go` and
  `go/pkg/db/supervisor_pointers_test.go`, plus the CI hard-fail check.
- Calls out the dependency add (creack/pty) and points the implementer
  at the supply-chain footprint section of the codex design.

400-800 words. Lead with a "Decisions" section and follow with
"Implementation order" + "Acceptance".
