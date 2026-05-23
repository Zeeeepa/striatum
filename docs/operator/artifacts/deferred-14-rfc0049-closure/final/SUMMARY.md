---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/deferred-14-rfc0049-closure/classification/REPORT.md", "docs/operator/plans/deferred-14-rfc0049-closure.md", "docs/operator/workflows/deferred-14-rfc0049-closure/workflow.json"]
---

# RFC 0049 Deferred Item 14 Closure Summary
author: rfc0049-closer-codex-gpt-5-001
date: 2026-05-23
classification: shelved

## Summary

Deferred item 14 / RFC 0049 remains **shelved** under D106. The closure pass
does not reopen the interactive Claude lane via MCP and does not claim the RFC
is implemented.

Current RFC 0050 / RFC 0075 work has delivered generic MCP packet-loop,
agent-loop PTY bootstrap, `session.report`, tmux attach metadata, and
liveness infrastructure. Those pieces reduce future spike cost, but they do
not answer RFC 0049's Claude-specific questions about real interactive Claude
behavior, subscription billing attribution, long-lived lifecycle policy, or
fresh-session semantics.

## Changed Files

- `docs/operator/plans/deferred-14-rfc0049-closure.md`
- `docs/operator/workflows/deferred-14-rfc0049-closure/workflow.json`
- `docs/operator/workflows/deferred-14-rfc0049-closure/prompts/classify_rfc0049.md`
- `docs/operator/workflows/deferred-14-rfc0049-closure/prompts/write_final_summary.md`
- `docs/operator/artifacts/deferred-14-rfc0049-closure/classification/REPORT.md`
- `docs/operator/artifacts/deferred-14-rfc0049-closure/final/SUMMARY.md`

No shared TODO, roadmap, brief, RFC, decision-log, source, or test files were
edited.

## Validation

- Workflow validation:
  `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate --json docs/operator/workflows/deferred-14-rfc0049-closure/workflow.json`
  -> valid.
- Python guardrails:
  `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_claude_supervised_wrapper.py tests/architecture/test_tmux_authority_boundary.py tests/test_dashboard_rfc0075.py`
  -> 16 passed.
- Go guardrails:
  `go test ./pkg/agentloop ./pkg/supervisor ./pkg/sessionliveness ./pkg/mcp`
  from `go/` -> passed.
- Fake MCP packet loop:
  `PYTHONDONTWRITEBYTECODE=1 PYTEST_ADDOPTS='-p no:cacheprovider' PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop`
  -> 1 passed.
- Lifecycle source search:
  `rg -n '"long_lived"|fresh_strategy|claude_code\.lifecycle|lifecycle.*long_lived|"per_packet"' src tests go contracts || true`
  -> no matches.
- Front-matter validation:
  `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python - <<'PY' ... validate_artifact_front_matter(...)`
  -> work-plan and synthesis front matter valid.
- Doc links:
  `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_doc_links.py`
  -> 7 passed.
- Whitespace:
  `git diff --check -- docs/operator/plans/deferred-14-rfc0049-closure.md docs/operator/workflows/deferred-14-rfc0049-closure docs/operator/artifacts/deferred-14-rfc0049-closure`
  -> passed.

## Protected-Doc Updates

None. Existing shared status docs already classify RFC 0049 as shelved, and
the current source behavior supports that classification.
