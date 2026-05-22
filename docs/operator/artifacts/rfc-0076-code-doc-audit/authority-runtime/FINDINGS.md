---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["authority-runtime", "mcp", "work-packet"]
---

# Authority Runtime Findings
author: authority-auditor-codex-001

## Scope

This audit checked current source behavior, generated contracts, tests, and
the RFC 0076 example workflow for regressions against Striatum's live
authority boundary: daemon-owned PostgreSQL, daemon RPC/MCP authorization,
no repo-local SQLite authority, no terminal/transcript authority, lease-gated
workflow mutations, and artifact validation.

## Findings

### AUD-001: Work packets expose workflow-local prompt paths as repo-root paths

severity: medium
category: implementation_gap
status: open

claim: `work.claim_next` copies `task_prompt.path` from workflow JSON into the
packet without resolving it relative to the workflow file or generated
workflow directory, so agents receive prompt references that can be missing
from the target repo root.

evidence:

- `go/pkg/mutations/claim.go:272` builds the packet with
  `task_prompt` copied from stored job capability requirements.
- `src/striatum/daemon_pg/handlers/context.py:765` does the same on the
  Python PG packet builder path.
- `src/striatum/daemon_pg/handlers/run_lifecycle/run_prepare.py:134-137`
  stores `job.task_prompt` as-is.
- `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json:119-121`
  declares `prompts/audit_authority_runtime.md`.
- The actual prompt exists at
  `docs/operator/workflows/rfc-0076-code-doc-audit/prompts/audit_authority_runtime.md:1`;
  `test -e prompts/audit_authority_runtime.md` returned exit status `1`.
- The live packet for this job exposed `task_prompt.path` as
  `prompts/audit_authority_runtime.md`, which failed when read from repo root.

impact: Fresh supervised agents are told to read a missing task prompt even
though the workflow is valid and the prompt exists beside the workflow. That
does not create alternate live-state authority, but it weakens work-packet
reliability and can force agents to infer paths or block.

recommended_action: Define the `task_prompt.path` resolution contract and
make packet generation emit either a repo-relative resolved path or explicit
`workflow_relative_path` plus `resolved_path`. Add validation/test coverage
using a workflow stored below `docs/operator/workflows/...` with prompt files
in its local `prompts/` directory.

follow_up: source/test work

### AUD-002: Go MCP `tools/call` can execute hidden workflow-authoring methods with a write token

severity: medium
category: authority
status: open

claim: Native Go MCP hides local workflow-authoring methods from `tools/list`,
but `tools/call` forwards arbitrary registered method names directly to daemon
RPC; a caller with write capability can invoke hidden file-writing workflow
methods such as `workflow.generate`.

evidence:

- `go/pkg/mcp/capabilities.go:60-70` classifies
  `workflow.validate`, `workflow.plan`, `workflow.graph`,
  `workflow.templates.*`, `workflow.init`, `workflow.generate`, and
  `workflow.upgrade` as hidden production tools for listing.
- `go/pkg/mcp/tools.go:27-38` builds a daemon RPC envelope for the requested
  method name and calls `HandleWithoutHandshake`; it does not reject
  `isHiddenProductionTool(name)` before routing.
- `go/pkg/mutations/mutations.go:82-84` registers real Go handlers for
  `workflow.init`, `workflow.generate`, and `workflow.upgrade`.
- `go/pkg/mcp/http_test.go:289-290` only asserts the hidden
  `workflow.generate` handler is not reached "for a token without write
  capability"; there is no matching test that a write-capable token is denied
  because the method is hidden/production-unsupported.
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md:213-219` classifies
  workflow authoring helpers as local authoring/service cleanup debt, and
  `docs/SPEC.md:1206` says CLI-local workflow authoring reads stay outside
  live workflow-state mutation authority.

impact: MCP discovery and MCP execution disagree. Operators may treat the
effective tool list as the supported production surface, while crafted
`tools/call` requests can still reach hidden workflow file writers if the
token has write capability. The write token is still an authorization gate,
so this is not a token bypass, but it is an authority-boundary ambiguity for
local file mutation through native daemon MCP.

recommended_action: Decide whether hidden workflow-authoring methods are
callable-but-undiscovered or fail-closed over daemon MCP. If they should be
unsupported in production MCP, add a `tools/call` deny path for hidden
production tools and tests covering both read-only and write-capable tokens.
If callable, update the matrix and MCP docs to state that hiding is only UX
filtering and that write tokens can invoke these file-authoring methods by
name.

follow_up: decision clarification plus source/test work

## Guardrail Checks

- `pytest tests/architecture/test_authority_guardrails.py tests/daemon_rpc/test_daemon_method_contract.py tests/test_cli_daemon_rpc_route.py tests/test_mcp_mutation_capabilities.py -q` passed: 95 tests.
- `go test ./pkg/rpc ./pkg/mcp ./cmd/striatumd` passed when run from the nested Go module at `go/`.
- The same Go package command failed from the repository root with
  `go: cannot find main module`; this is a command-location issue, not a test
  failure.

## Boundary Notes

- I did not find evidence that production CLI/RPC routes reopen repo-local
  SQLite for live workflow state. Current guardrails keep CLI route fallback
  empty, generated daemon contracts aligned, and active Go daemon handlers
  covered.
- I did not find evidence that retired RPC names such as
  `apply.reviewed_patch`, `dogfood.publish_on_behalf`, or
  `dogfood.surgical_recovery` remain production methods; tests cover
  `method_unknown` behavior at MCP/RPC boundaries.
