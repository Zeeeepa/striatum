# Packaging And Release Cutover

Read RFC 0078, `pyproject.toml`, `Makefile`, `.github/workflows/`, scripts,
and install docs. Produce
`docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/packaging/HANDOFF.md`.

Define the Go-only distribution path, CI target changes, smoke-test
replacement, version source, and Python package retirement sequence. Implement
only a safe, non-overlapping first slice if it does not break current tests.
