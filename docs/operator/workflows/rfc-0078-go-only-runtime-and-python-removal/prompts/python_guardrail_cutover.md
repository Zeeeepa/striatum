# Python Deletion Guardrails

Read RFC 0078 and current architecture guardrails. Produce
`docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/guardrails/HANDOFF.md`.

Design and, if safe, implement tracked-head checks that fail when active
Striatum Python source, tests, packaging, or operator instructions return.
Avoid blocking target-repository workflows that intentionally run Python as the
target project's own command.
