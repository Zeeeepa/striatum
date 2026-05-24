# Test Migration Cutover

Read RFC 0078, pytest configuration, Python tests, and Go tests. Produce
`docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/tests/HANDOFF.md`.

Map pytest files to Go, shell, or browser replacements. Identify redundant
Python tests already covered by Go and tests that need new equivalents before
deletion. Implement only a safe, non-overlapping first test migration if clear.
