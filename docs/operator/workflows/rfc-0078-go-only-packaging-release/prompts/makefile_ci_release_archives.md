# Makefile CI Release Archive Cutover

Read RFC 0078, the version/module decision if it exists, root `Makefile`,
`go/Makefile`, `.github/workflows/`, `scripts/`, and the prior packaging
handoff. Produce
`docs/operator/artifacts/rfc-0078-go-only-packaging-release/makefile-ci-release/HANDOFF.md`.

Replace or scaffold the Go-only build and release path:

- root `VERSION` wiring if the decision allows it;
- Make targets for `go test ./...`, release binary builds, archive creation,
  checksums, install into a local prefix, and aggregate validation;
- CI that uses setup-go and current frontend checks without creating a Python
  virtual environment;
- release archive naming for OS/arch triples;
- checksum manifest generation and verification;
- explicit retirement of wheel/sdist/twine/PyPI publishing from release CI.

Keep the edit bounded to the work packet write scope. Do not delete
`pyproject.toml` in this job. If current tests would break, land additive
targets and document the remaining deletion blocker in the handoff.
