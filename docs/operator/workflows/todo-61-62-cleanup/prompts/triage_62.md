# Triage TODO 62

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Produce the TODO 62 cleanup triage artifact only. Include the current daemon
doctor symptom: `daemon doctor --authority --json` reports
`daemon_repo_state_missing` for `.striatum/state.sqlite3` even though
PostgreSQL is authoritative and `.striatum/` is only operational scratch.

Identify any remaining daemon-global registry probes, SQLite live-state
assumptions, doctor/status/dashboard/MCP fallback paths, or tests that still
encode the retired SQLite substrate as production state.

Do not modify source files. Cite concrete files, tests, and commands the
implementer should use.
