---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["README.md", "docs/SPEC.md", "docs/USING_STRIATUM.md", "docs/GETTING_STARTED.md", "docs/POSTGRES_TRANSITION.md", "docs/CLI_REFERENCE.md", "docs/MCP.md", "docs/HOW_TO_AGENT.md", "docs/operator/BRIEF.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/final/SUMMARY.md"]
---

# Active Documentation Handoff
author: operator [self-declared: docs-rewriter-codex-gpt-5-001]

## Result

Active install and release docs were rewritten by the packaging gate to point
at Go release archives and root Makefile targets. The docs still must not
claim RFC 0078 is complete: Python source, pytest coverage, Python scripts,
and `pyproject.toml` remain tracked and block final deletion.

## Current Runtime Claim Checklist

- Go daemon authority: current and documented.
- Daemon-owned PostgreSQL live state: current and documented.
- Daemon MCP/RPC for live workflow control: current and documented.
- Local web UI/operator surface: current, but still Python-owned and blocked
  on the Go web/service cutover.
- Go-only Striatum runtime: not yet true.
- Python-free install/release guidance: partially true for the normal release
  path; blocked traces remain for legacy Python surfaces and current guidance.

## Python-Era Instructions

The new guardrail reports active Python runtime guidance in current docs and
tooling as `blocked`, not historical. Those references should be rewritten by
the Go-only packaging/release and docs gate after the Go replacements land.

## Validation

- `make python-trace-report`: report mode passed after integration;
  `blocked=429`, `unclassified=0`.
- `make python-trace-guardrail`: failed as expected
  with `blocked=429`, `unclassified=0`.

## Remaining Separate Decisions

- Whether `/chat/*` and historical `/dogfood/*` web routes are retained in Go
  or retired.
- Whether PyPI receives a transition/deprecation artifact after Go-only
  packaging lands.
- The replacement aggregate validation command after pytest is removed.
