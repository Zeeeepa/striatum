# Go Web And Service Cutover

Read RFC 0078 plus the Python service/web modules and Go daemon/read/mutation
packages. Produce
`docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/web/HANDOFF.md`.

Map current local web/service routes to Go replacements or explicit
retirements. Identify route tests needed before deleting Python web code.
Implement only a safe, non-overlapping first slice if it is clear.
