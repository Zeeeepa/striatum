# RFC 0078 Remaining Work Scaffold

Status: active
Date: 2026-05-25
author: operator-codex-gpt-5.5-001

## Purpose

RFC 0078 is scaffolded and the first Go CLI/artifact-contract slice has
landed. The remaining work is now split into six executable gates so each can
run with high parallelism and produce its own acceptance evidence before the
final Python deletion gate.

## Gates

1. **Go CLI RPC router.** Generate or otherwise centralize Go CLI route
   metadata from `contracts/daemon_methods.json`, add daemon RPC client
   support, and route daemon-owned command families without duplicating daemon
   authority.
2. **Go web/service cutover.** Decide retained routes, implement the local
   loopback HTTP/security/static/SSE service in Go, and retire routes that do
   not justify a Go port.
3. **Workflow/artifact parity.** Finish Go parity for workflow validation,
   lint, generation, template catalog, artifact contracts, and accepted
   authoring helpers.
4. **Python test migration.** Move pytest coverage into Go, shell, browser, or
   explicit retirement records, preserving behavior rather than file shape.
5. **Go-only packaging/release.** Move versioning, build, install, smoke,
   release archives, CI, and embedded assets to Go-owned surfaces.
6. **Docs, guardrails, and deletion.** Supersede decisions/docs, rewrite
   active operator guidance, enable Python-trace guardrails, delete remaining
   Python product traces, and close RFC 0078.

## Scaffolds

- `docs/operator/workflows/rfc-0078-go-cli-rpc-router/workflow.json`
- `docs/operator/workflows/rfc-0078-go-web-service-cutover/workflow.json`
- `docs/operator/workflows/rfc-0078-workflow-artifact-parity/workflow.json`
- `docs/operator/workflows/rfc-0078-python-test-migration/workflow.json`
- `docs/operator/workflows/rfc-0078-go-only-packaging-release/workflow.json`
- `docs/operator/workflows/rfc-0078-docs-guardrails-final-deletion/workflow.json`

The umbrella workflow
`docs/operator/workflows/rfc-0078-remaining-work/workflow.json` exists to
drive and consolidate those six gates.
