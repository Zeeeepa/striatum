---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/inventory/CUTOVER_LEDGER.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/cli/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/web/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/workflow-authoring/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/tests/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/packaging/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/docs/HANDOFF.md", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/guardrails/HANDOFF.md"]
---

# RFC 0078 Cutover Summary
author: operator [self-declared: rfc0078-closer-codex-gpt-5-001]

## Run

Workflow: `docs/operator/workflows/rfc-0078-go-only-runtime-and-python-removal/workflow.json`

Run id: `run_ef93ee9055bb77e40d2ae2c846337176`

The scaffold uses `max_active_jobs: 20`. Codex accepted six live sub-agents in
this environment; the seventh spawn was refused by the agent-thread limit, so
six is the effective maximum live sub-agent count for this execution.

## Landed

- Scaffolded the RFC 0078 max-parallel workflow and prompt set.
- Prepared and started the workflow through daemon RPC.
- Added the first Go `striatum` CLI scaffold at `go/cmd/striatum`.
- Ported `workflow validate` to the Go CLI with JSON output and same-model
  refusal handling.
- Expanded Go artifact kind/front-matter parity for operator, Git/PR, and
  auto-finalize gate artifacts.
- Tightened Go escalation artifact front matter and duplicate-field refusal.
- Added Go tests for the new CLI and artifact-contract slices.

## Not Complete

RFC 0078 cannot honestly be marked complete yet. Active blockers remain:

- no full Go CLI parser/RPC route implementation;
- no Go local web/service replacement;
- Python packaging/release/install still owns the shipped product;
- pytest still carries most behavioral coverage;
- workflow validation/generator parity remains incomplete;
- skills/plugins/scaffold/corpus/archive surfaces still need Go ports or
  explicit retirements;
- guardrails cannot be enabled while active Python surfaces remain.

## Next Gates

1. Build the Go CLI RPC router from `contracts/daemon_methods.json`.
2. Decide web route retention, especially `/chat/*`, `/dogfood/*`, `/view/*`,
   and generic `/v1/invoke`.
3. Move artifact contracts into a dedicated Go package and port Python schema
   tests.
4. Expand Go workflow validation/lint parity and make generation reuse it.
5. Start packaging cutover with a root `VERSION`, Go build/install targets,
   and release archive smoke tests.
6. Convert the highest-risk pytest suites before deleting Python files.
