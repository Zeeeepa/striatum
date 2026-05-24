# Go CLI Cutover

Read RFC 0078, `contracts/daemon_methods.json`, `src/striatum/cli/`, and the
existing Go packages. Produce
`docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/cli/HANDOFF.md`.

Define the smallest Go `striatum` CLI that can replace the Python console
script. Separate daemon-routed command families from local workflow-authoring
helpers and retired compatibility commands. Implement only a safe,
non-overlapping first slice if it can be done without duplicating daemon
authority.
