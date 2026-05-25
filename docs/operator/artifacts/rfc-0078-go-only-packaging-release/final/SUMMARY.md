---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0078-go-only-packaging-release/version-module/DECISION.md", "docs/operator/artifacts/rfc-0078-go-only-packaging-release/makefile-ci-release/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-packaging-release/embed-assets/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-packaging-release/smoke/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-packaging-release/docs-install/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-packaging-release/pypi-retirement/DECISION.md"]
---

# RFC 0078 Packaging Release Gate Summary
author: operator [self-declared: packaging-release-closer-codex-gpt-5-001]

## Result

The packaging/release/install gate is materially landed for the Go-only
archive path, but it does not unblock final Python deletion by itself.

## Decisions

- Root `VERSION` is the release version source.
- First Go-only production release is `v2.0.0`.
- The Go module remains nested under `go/` for this gate.
- `striatum`, `striatumd`, and `striatum-supervisor-helper` remain separate
  binaries.
- PyPI publishing stops for the normal production release workflow.

## Landed Changes

- Active Makefile build/test/install/release/smoke targets use Go binaries.
- CI uses setup-go/setup-node and no Python virtual environment.
- Release CI builds `.tar.gz` archives and `SHA256SUMS`, not wheel/sdist/PyPI.
- Go archive build/check scripts and Go-only smoke scripts exist.
- Active install/release docs point to Go archives or local Go builds.

## Blockers Before Python Deletion

- Go CLI RPC parity is still incomplete, so archive smoke cannot yet drive a
  full registered-run lifecycle from the Go binary alone.
- Go web/service and retained template/asset embedding decisions remain open.
- `pyproject.toml`, Python source, and Python tests remain until their
  dedicated RFC 0078 deletion/parity gates close.

## Validation

- `striatum workflow validate --allow-same-model-pairing docs/operator/workflows/rfc-0078-go-only-packaging-release/workflow.json` passed.
- `git diff --check` passed.
- `bash -n` passed for the Go archive/smoke scripts and compatibility smoke
  wrappers.
- `go test ./cmd/striatum ./cmd/striatum-supervisor-helper ./pkg/db ./pkg/workflowtemplates`
  passed.
- `go test ./...` passed after integrating the parallel RFC 0078 gates.
- A direct helper version build passed:
  `striatum-supervisor-helper --version` printed `2.0.0`.
- Active install/release/smoke doc scan passed for the retired PyPI/wheel
  phrases listed in CI.
- Host-platform Go archive build/check passed with
  `scripts/build_go_release_archives.sh --target <host-triple>` and
  `scripts/check_go_release_archives.sh`.
- `scripts/go_package_smoke.sh` passed, with PostgreSQL integration skipped
  because `STRIATUM_DAEMON_DB_URL` is not set.
- `scripts/go_fresh_clone_smoke.sh` passed, with the same PostgreSQL skip.
- `make release-check` passed after the archive checker was corrected to run
  binary version probes only for the host-compatible archive while still
  verifying all four archive checksums and structures.
