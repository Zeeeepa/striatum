---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/plans/rfc-0078-remaining-work.md", "docs/operator/workflows/rfc-0078-remaining-work/workflow.json", "docs/operator/workflows/rfc-0078-go-cli-rpc-router/workflow.json", "docs/operator/workflows/rfc-0078-go-web-service-cutover/workflow.json", "docs/operator/workflows/rfc-0078-workflow-artifact-parity/workflow.json", "docs/operator/workflows/rfc-0078-python-test-migration/workflow.json", "docs/operator/workflows/rfc-0078-go-only-packaging-release/workflow.json", "docs/operator/workflows/rfc-0078-docs-guardrails-final-deletion/workflow.json"]
---

# RFC 0078 Remaining Work Scaffold Summary
author: operator-codex-gpt-5.5-001

## Result

RFC 0078 remaining work is split into six dedicated executable workflows plus
one umbrella workflow. Six sub-agents were launched, which is the effective
maximum live sub-agent count available in this environment after closing the
previous completed agent threads.

## Execution Update

The six dedicated gates were executed on 2026-05-25 with six parallel
sub-agents and integrated into one working tree. Landed output includes:

- generated Go CLI RPC route metadata and dispatch through daemon RPC;
- shared Go artifact contracts plus expanded workflow validation and generator
  reuse;
- Go web service/security/static/SSE scaffolding and route-retirement
  guardrails;
- Go release archives, root `VERSION`, Go-only package/fresh-clone smokes, and
  release CI changes;
- row-level pytest migration/deletion ledgers;
- a Python-trace report/strict guardrail for final deletion readiness.

Final deletion is blocked, not accepted. `make python-trace-guardrail` still
reports active Python source, pytest, packaging, scripts, and current guidance.

## Dedicated Gates

1. `docs/operator/workflows/rfc-0078-go-cli-rpc-router/workflow.json`
   - Generated Go CLI RPC router gate.
2. `docs/operator/workflows/rfc-0078-go-web-service-cutover/workflow.json`
   - Go local web/service replacement or route-retirement gate.
3. `docs/operator/workflows/rfc-0078-workflow-artifact-parity/workflow.json`
   - Workflow validation, lint, generator, catalog, and artifact parity gate.
4. `docs/operator/workflows/rfc-0078-python-test-migration/workflow.json`
   - Pytest-to-Go/shell/browser migration gate.
5. `docs/operator/workflows/rfc-0078-go-only-packaging-release/workflow.json`
   - Go-only packaging, release, install, CI, and smoke gate.
6. `docs/operator/workflows/rfc-0078-docs-guardrails-final-deletion/workflow.json`
   - Docs, decisions, guardrails, final Python deletion, and acceptance gate.

## Umbrella

`docs/operator/workflows/rfc-0078-remaining-work/workflow.json` coordinates the
six gates and records the dependency shape. It is a scaffold tracker; the
dedicated workflows are the executable implementation gates.

## Validation

All seven workflow files validated with:

```text
STRIATUM_TEST_HARNESS=1 STRIATUM_DAEMON_REQUIRED=0 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate --allow-same-model-pairing <workflow.json>
```

Integrated validation also passed for `go test ./...`, `make check`,
`go generate ./pkg/cli/routes` freshness, the Go release/package/fresh-clone
smoke scripts, frontend API-client tests, and doc-link/current-brief tests.
The strict Python deletion guardrail fails as designed while blockers remain.

## Remaining Order

Start from the Python-trace and coverage-ledger blockers, not from scaffold.
Port or retire active Python runtime/test/script/doc rows, then re-run strict
guardrail mode before declaring RFC 0078 accepted.
