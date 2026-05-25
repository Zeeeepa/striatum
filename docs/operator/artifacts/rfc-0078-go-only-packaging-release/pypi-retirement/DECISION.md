---
schema_version: striatum.decision.v1
decision_id: "rfc-0078-pypi-retirement"
run_id: "rfc-0078-go-only-packaging-release"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0078 PyPI retirement sequence"
created_at: "2026-05-25T00:00:00Z"
---

# RFC 0078 PyPI Retirement Sequence
author: operator [self-declared: pypi-retirement-owner-codex-gpt-5-001]

## Decision

The next production release stops publishing PyPI artifacts. The current
release workflow must publish GitHub Go archives and `SHA256SUMS` only.

A one-time PyPI deprecation release is allowed only as a separate, explicit
operator action after this gate. It may contain metadata and a notice pointing
users to GitHub Go release archives, but it must not reintroduce Striatum
runtime behavior, daemon authority, console scripts, or Python package data as
the production install path.

## Release Notes Direction

Release notes should say that `striatum-orchestrator` is retired for current
production use and that users should install the `striatum_<version>_<os>-<arch>.tar.gz`
archive matching their platform, then verify it with `SHA256SUMS`.

## Deletion Blocker

`pyproject.toml` deletion remains blocked until the Python deletion workflow
either removes the remaining Python source/tests or records a historical
provenance exception. This packaging gate removes PyPI from active release CI
and docs but deliberately does not delete `pyproject.toml`.

## Proofs Required

- `.github/workflows/release.yml` has no PyPI publish job.
- Active release checks build Go archives and checksums.
- Active install docs do not instruct users to install Striatum from PyPI.
