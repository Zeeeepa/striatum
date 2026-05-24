---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["tests/", "go/", "pyproject.toml", "Makefile"]
---

# Test Migration Handoff
author: operator [self-declared: test-porter-codex-gpt-5-001]

## Snapshot

At workflow start:

- pytest surface: 176 Python test/helper files and 1129 `test_` functions;
- Go surface: 60 Go test files and 255 `Test*` functions;
- pytest/ruff/mypy configuration still lives in `pyproject.toml`;
- root `Makefile` still centers Python packaging and pytest.

## Coverage Migration Priorities

High-risk gaps before Python test deletion:

- artifact/front-matter schemas and parser strictness;
- full workflow validation/lint matrices;
- recovery side-effect integration;
- corpus/redaction/export;
- local web/service routes;
- skills, plugins, and scaffold installers;
- Go or shell replacement for Python test harness fixtures.

Already partly covered in Go:

- daemon RPC registry, capability metadata, handler coverage;
- daemon startup/admin/token/pidfile basics;
- repository service, migrations, audit race tests;
- MCP HTTP and capability filtering;
- supervision, liveness, worktree, recovery scheduler basics;
- status/dashboard/detail/listing/read-model packages.

## First Slice Landed

New Go tests were added for the initial Go CLI scaffold and the Go artifact
contract parity slice. This is not a pytest replacement yet; it is the first
safe wedge for the replacement aggregate validation path.

## Retire Candidates

Python wheel/package smoke tests, `_daemongo` launcher tests, historical
local-state fixtures, Python MCP/daemon compatibility tests, and placeholder
Go-daemon pytest smoke files can retire after matching Go/shell coverage lands.
