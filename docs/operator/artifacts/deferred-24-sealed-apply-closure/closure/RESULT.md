---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/SPEC.md", "docs/DECISION_LOG.md", "docs/UBIQUITOUS_LANGUAGE.md", "docs/rfcs/0027-sealed-patch-provenance-mode.md", "docs/rfcs/0031-daemon-owned-supervision-and-sealed-apply-boundary.md", "docs/rfcs/0068-go-production-daemon-port.md", "docs/architecture/COMMAND_AUTHORITY_MATRIX.md", "contracts/daemon_methods.json", "src/striatum/daemon_apply/apply_service.py", "src/striatum/daemon_rpc/registry.py", "src/striatum/daemon_pg/mcp_dispatch.py", "go/pkg/rpc/registry_methods.go", "go/pkg/rpc/server.go", "go/pkg/apply/signing_key.go", "tests/test_mcp_mutation_capabilities.py", "tests/test_mcp_capability_scope_e2e.py", "tests/architecture/test_authority_guardrails.py", "go/pkg/rpc/registry_contract_test.go", "go/pkg/mcp/http_test.go", "go/pkg/apply/service_test.go"]
---

# Deferred 24 Sealed Apply Closure
author: deferred24-sealed-apply-codex-gpt-5-001
status: closed
date: 2026-05-23

## Result

Deferred item 24 is closed without source, contract, test, RFC, TODO, roadmap,
or operator-brief edits. The current sealed-apply state is intentionally
fail-closed:

- `apply.reviewed_patch` is removed from the production daemon RPC contract.
- Stale MCP/RPC calls to `apply.reviewed_patch` must return and audit as
  `method_unknown`.
- `apply.receipt.show`, `apply.receipt.verify`, and `daemon.key.rotate`
  remain supported foundation surfaces.
- Full reviewed-patch apply must not be reintroduced as a production mutation
  until a new accepted sealed-apply RFC or product decision defines the full
  gate.

## Evidence

- `docs/SPEC.md` excludes malicious-local-operator-resistant sealed apply from
  the current product boundary.
- D112 says `apply.reviewed_patch` is removed, not retained as a fail-closed
  registered method, because the full apply gate and stronger key custody are
  incomplete.
- D112's revisit trigger requires a new accepted sealed-apply RFC or product
  decision that defines the full apply gate, authority model, key custody, and
  operator UX before the mutation can return.
- `docs/TODO.md` item 61 and `docs/ROADMAP.md` both repeat the current state:
  D112 removed `apply.reviewed_patch`; stale direct calls return/audit as
  `method_unknown`.
- RFC 0027 remains proposed/partial: it documents the sealed-patch provenance
  design and says phase-2 guardrails shipped, while the full apply gate and
  receipt pipeline remain follow-up.
- RFC 0031 is accepted as the daemon-owned supervision/apply foundation, but
  its implementation note says Go currently owns fallback signing-key rotation
  and that OS keyring custody plus the full reviewed-patch mutation remain
  deferred.
- RFC 0068 records the Go production daemon cutover rule: unsupported methods
  are removed from production surfaces, and D112 removed
  `apply.reviewed_patch` until a future sealed-apply decision reintroduces it.
- `contracts/daemon_methods.json` lists `apply.receipt.show` and
  `apply.receipt.verify`; it does not list `apply.reviewed_patch`.
- `go/pkg/rpc/registry_methods.go` matches the contract and has only receipt
  methods in the `apply.*` set.
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` documents the removed-name
  rule and has active rows only for apply receipt reads and daemon key
  rotation.
- `src/striatum/daemon_pg/mcp_dispatch.py` handles unknown MCP tool names
  before authorization, writes metadata-only audit/request-log rows, and
  returns `method_unknown`.
- `go/pkg/rpc/server.go` rejects any method absent from the Go registry with
  `method_unknown` and records the denied authorization state for audit.

## Classification

Close item 24 as fail-closed removal preserved. No new sealed-apply RFC should
be written in this closure; the point of the closure is to keep the removed
mutation out of production.

A future reintroduction needs a new accepted sealed-apply RFC or product
decision because the accepted foundation does not yet specify the production
mutation. That future artifact must define at least:

- exact patch artifact and digest identity;
- reviewer verdict binding to the patch digest and base/result tree;
- base-tree drift refusal and write-scope checks;
- verification-job and blocker/run-state eligibility;
- signing-key custody, rotation, degraded-trust behavior, and receipt format;
- CLI/MCP/UI operator UX and capability grants;
- audit semantics and stale-call behavior during version skew.

Until then, re-adding `apply.reviewed_patch` to `contracts/daemon_methods.json`,
generated Go registry files, MCP discovery, CLI routes, or authority matrices
would be a product regression.

## Guard Coverage

The current production fail-closed status is guarded enough for this
artifact-only closure:

- Python MCP dispatch has a retired-name test:
  `tests/test_mcp_mutation_capabilities.py::test_daemon_mcp_retired_apply_reviewed_patch_is_default_denied_and_audited`.
- PostgreSQL/MCP end-to-end coverage asserts the retired name audits as
  `method_unknown`:
  `tests/test_mcp_capability_scope_e2e.py::test_retired_apply_reviewed_patch_denied_as_unknown_and_audited`.
- Capability-list tests assert unsupported production methods are hidden from
  MCP discovery.
- Architecture guardrails assert active registry methods match the authority
  matrix; because the removed method is absent from the registry and table, it
  cannot silently become an active supported method without updating guarded
  surfaces.
- Go registry tests assert generated method metadata matches
  `contracts/daemon_methods.json`; Go handler coverage requires every active
  method to have a real handler.
- Go apply tests cover the supported signing-key rotation/key-loading
  foundation, not a reviewed-patch mutation.

The old direct Python daemon RPC test module contains a specific
`apply.reviewed_patch` assertion but is skipped as legacy cleanup. I did not
add another Python-only test because the production boundary is now the Go
daemon plus MCP/RPC contract, and the active MCP/architecture/Go tests guard
the supported surface.

## Changed Files

- `docs/operator/plans/deferred-24-sealed-apply-closure.md`
- `docs/operator/workflows/deferred-24-sealed-apply-closure/workflow.json`
- `docs/operator/workflows/deferred-24-sealed-apply-closure/prompts/classify_sealed_apply_status.md`
- `docs/operator/artifacts/deferred-24-sealed-apply-closure/closure/RESULT.md`

No shared TODO, roadmap, operator brief, decision-log, RFC, contract, source,
Go, or test files were edited.

## Validation Evidence

Commands run for this closure:

- `PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate docs/operator/workflows/deferred-24-sealed-apply-closure/workflow.json --json`
  - Result: `{"data":{"valid":true,"workflow_id":"deferred-24-sealed-apply-closure"},"ok":true}`.
- `PYTHONPATH=src .venv/bin/python -m striatum.cli workflow plan docs/operator/workflows/deferred-24-sealed-apply-closure/workflow.json --json`
  - Result: valid plan; 1 job, 0 edges, 0 cycles, 1 claim step.
- `PYTHONPATH=src .venv/bin/python -m striatum.cli workflow lint docs/operator/workflows/deferred-24-sealed-apply-closure/workflow.json --json`
  - Result: `valid: true`, `warning_count: 0`, coverage level `strong`.
- `PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_mcp_mutation_capabilities.py::test_daemon_mcp_tools_list_filters_by_capability tests/test_mcp_mutation_capabilities.py::test_daemon_mcp_retired_apply_reviewed_patch_is_default_denied_and_audited tests/test_mcp_capability_scope_e2e.py::test_retired_apply_reviewed_patch_denied_as_unknown_and_audited tests/architecture/test_authority_guardrails.py::test_authority_matrix_covers_active_registry_methods tests/architecture/test_authority_guardrails.py::test_registry_methods_have_explicit_authority_path`
  - Result: 5 passed.
- `go test ./pkg/rpc -run 'TestRegistryMatchesDaemonMethodsContract|TestMethodsETagMatchesDaemonMethodsContract|TestHelloUsesDynamicSealedApplyStatus'`
  - Result: `ok github.com/halbritt/striatum/go/pkg/rpc`.
- `go test ./pkg/mcp -run 'TestHTTPHandlerToolsCallUnknownDaemonMethodReturnsMCPError|TestHTTPHandlerToolsListUsesBearerTokenAndHidesUnauthorized'`
  - Result: `ok github.com/halbritt/striatum/go/pkg/mcp`.
- `go test ./pkg/apply -run 'Test'`
  - Result: `ok github.com/halbritt/striatum/go/pkg/apply`.
- `go test ./cmd/striatumd -run 'TestGoDaemonMethodCoverageIsExplicit|TestRegisterHandlersWiresKeyRotateHook'`
  - Result: `ok github.com/halbritt/striatum/go/cmd/striatumd`.
- `PYTHONPATH=src .venv/bin/python -c "from pathlib import Path; from striatum.artifact_contracts import validate_artifact_front_matter; [validate_artifact_front_matter(kind=k, path=Path(p), payload=Path(p).read_bytes()) for k,p in [('work_plan','docs/operator/plans/deferred-24-sealed-apply-closure.md'),('synthesis','docs/operator/artifacts/deferred-24-sealed-apply-closure/closure/RESULT.md')]]; print('front matter valid')"`
  - Initial result: failed because this work plan used invalid `scope_kind:
    "deferred_item"`.
  - Follow-up fix: changed the work plan `scope_kind` to the schema-valid
    `phase`.
  - Rerun result: `front matter valid`.
- `git diff --check -- docs/operator/plans/deferred-24-sealed-apply-closure.md docs/operator/workflows/deferred-24-sealed-apply-closure docs/operator/artifacts/deferred-24-sealed-apply-closure`
  - Result: passed.

## Shared Updates To Queue

None required from this worker. Shared status docs already say D112 removed
`apply.reviewed_patch` and that reintroduction requires a future sealed-apply
decision. A later operator status refresh may cite this closure artifact as
the bounded deferred-item evidence.
