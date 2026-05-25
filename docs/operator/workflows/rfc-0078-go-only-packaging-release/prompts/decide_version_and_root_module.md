# VERSION And Root Go Module Decision

Read RFC 0078, the prior RFC 0078 packaging handoff, `pyproject.toml`,
`Makefile`, `go/Makefile`, `go/go.mod`, release CI, and install docs. Produce
`docs/operator/artifacts/rfc-0078-go-only-packaging-release/version-module/DECISION.md`.

The artifact must use `striatum.decision.v1` front matter if that schema is
available in the current runner. Include the exact expected byline from the
work packet.

Decide:

- whether `VERSION` at repository root becomes the single version source;
- whether the first Go-only release should be `v2.0.0` or a narrower version;
- whether `go/go.mod` stays nested for the first packaging gate or moves to
  repository root before Python deletion;
- whether `striatum` and `striatumd` remain separate release binaries for the
  first Go-only release;
- which downstream jobs are blocked until the decision is accepted.

Do not edit product files in this job. If a choice needs human-principal
authority, publish a decision artifact that states the preferred answer,
risks, and exact follow-up question rather than modifying code.
