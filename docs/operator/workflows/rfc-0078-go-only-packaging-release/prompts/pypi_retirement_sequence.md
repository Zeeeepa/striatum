# PyPI Retirement Sequence

Read RFC 0078, prior packaging and docs handoffs, `pyproject.toml`, release CI,
README install sections, `docs/RELEASING.md`, and changelog/release policy.
Produce
`docs/operator/artifacts/rfc-0078-go-only-packaging-release/pypi-retirement/DECISION.md`.

The artifact must use `striatum.decision.v1` front matter if that schema is
available in the current runner. Include the exact expected byline from the
work packet.

Decide the Python distribution retirement sequence:

- whether the next release immediately stops publishing PyPI artifacts;
- whether a one-time PyPI deprecation release is allowed, and if so what it
  may contain without restoring Python runtime authority;
- how release notes should point users from `striatum-orchestrator` to Go
  release archives;
- whether `pyproject.toml` deletion is blocked on any external packaging
  metadata;
- which CI and docs checks prove PyPI is retired.

Do not edit product release files in this job unless the work packet has been
expanded by the operator. This job should create a decision-quality artifact
that downstream implementation can follow.
