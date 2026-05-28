# Releasing striatum

Status: draft policy
Date: 2026-05-25

Striatum releases are Go binary archives plus a checksum manifest. The
legacy Python distribution name is retired for current
production packaging; do not publish Python package artifacts from the normal
release workflow.

## What a release means

A pushed `v*` tag triggers `.github/workflows/release.yml`. The workflow
checks that the tag matches the root `VERSION` file, runs Go validation,
builds Linux/macOS archives for `amd64` and `arm64`, verifies `SHA256SUMS`,
runs the Go package smoke, and creates a GitHub Release.

Each archive contains:

- `bin/striatum`
- `bin/striatumd`
- `bin/striatum-supervisor-helper`
- `VERSION`, `LICENSE`, `README.md`, and `INSTALL.txt`

The release archive is the install contract. PyPI may receive at most a
separate one-time deprecation notice if accepted by the RFC 0078 retirement
decision; it must not restore Python runtime authority.

## Versioning policy

Striatum follows relaxed semver:

- **Major (`vX.0.0`)**: breaking product or packaging transitions. The first
  Go-only archive release is `v2.0.0`.
- **Minor (`vX.Y.0`)**: new behavior or meaningful operator-visible changes.
- **Patch (`vX.Y.Z`)**: fixes to an already-tagged release.

The root `VERSION` file is the single version source for release builds.

## Pre-release checklist

Before pushing a `v*` tag:

1. `make release-check` passes locally.
2. CI on `main` is green.
3. `CHANGELOG.md` has an adopter-facing block for the target version.
4. `VERSION` contains the tag without the leading `v`.

## Tag command

```bash
git tag -s v2.0.0 -m "v2.0.0"
git push origin v2.0.0
```

The release workflow builds and uploads the archives and `SHA256SUMS`.
