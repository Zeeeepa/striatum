# Docs Install Release Changes

Read RFC 0078, the prior docs and packaging handoffs, README, day-zero usage
docs, release docs, CLI reference, Postgres transition docs, and agent/human
guides. Produce
`docs/operator/artifacts/rfc-0078-go-only-packaging-release/docs-install/HANDOFF.md`.

Rewrite active operator-facing install and release guidance for the Go-only
distribution:

- release archives or local Go builds are the primary install paths;
- PyPI, `pip install striatum-orchestrator`, `.venv`, wheel/sdist, pytest,
  ruff, and mypy are not current Striatum runtime guidance after the gate;
- `striatum` and `striatumd` binary names match the version/module decision;
- smoke and validation commands match the replacement scripts and Make
  targets;
- historical Python references remain only where clearly labeled provenance.

If implementation jobs have not landed the corresponding targets yet, stage
the docs as a handoff with exact edits to apply after those targets exist
rather than claiming the cutover is complete.
