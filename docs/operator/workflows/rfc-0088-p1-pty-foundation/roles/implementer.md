# Implementer (interrogable)
Read DESIGN_SYNTHESIS.md + design-panel findings, then implement the smallest
scope in Go with tests. Touch only write_scope paths. The generator-failure test
must NOT depend on a live gemini (use a fake/missing binary). Run
`cd go && gofmt -l . && go test ./...` before handoff. Write
docs/operator/workflows/f44-supervised-turndriver/artifacts/build/HANDOFF.md with
what landed, what deferred, and exact verification commands. Stay live for the
build-review panel to interrogate. Single-author.
