---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/operator/BRIEF.md", "docs/SPEC.md", "docs/UBIQUITOUS_LANGUAGE.md", "docs/rfcs/0041-engram-memory-layer-for-striatum-operators.md", "docs/rfcs/0044-engram-phase-1-implementation-spec.md", "docs/rfcs/0057-corpus-contract-v2.md", "tests/test_cli_corpus_export.py", "tests/test_corpus_verify.py"]
---

# Engram-Side Memory Tools External Work Closure
author: deferred27-engram-side-codex-gpt-5-001
status: closed
date: 2026-05-23

## Result

Deferred item 27 is closed for Striatum core.

The remaining RFC 0044 Phase 1 work is Engram-side external consumer work:
`engram ingest-striatum`, `engram-mcp-stdio`, the four read-only retrieval
tools, tenant/corpus capability checks, and Engram-local `memory.*`
capabilities belong in `~/git/engram/`, not in this repository.

No Striatum source, Go, test, TODO, roadmap, operator brief, decision-log, or
RFC edits are required for this closure. The current Striatum boundary text is
already current for the deferred-27 question.

## Evidence

- `docs/TODO.md` item 23 says Striatum-side RFC 0044 V1 corpus export is
  done and that the Engram ingester, standalone MCP server, retrieval tools,
  and `memory.*` capabilities remain a separate follow-up at `~/git/engram/`,
  explicitly not in Striatum's TODO scope.
- `docs/TODO.md` item 32 queues the tenant-aware RFC 0044 Phase 1 work as an
  external Engram follow-up and says not to add Engram ingester or MCP code to
  Striatum.
- `docs/ROADMAP.md` section 5.7 treats the Engram roadmap as an external
  consumer request, not a Striatum runtime dependency. It says the core
  Striatum side has shipped corpus export, V2 manifest metadata, and optional
  reference-only augmentation packet metadata.
- `docs/ROADMAP.md` blocked table item 32 lists Engram-side RFC 0044 Phase 1
  under external repo `~/git/engram/` with unblock criterion "Engram-side
  work; not Striatum's TODO."
- `docs/SPEC.md` says external memory or retrieval systems may ingest
  read-only corpus exports, but the runner must not import such consumers,
  register `memory.*` capabilities, or call retrieval during state
  transitions.
- `docs/UBIQUITOUS_LANGUAGE.md` defines Engram as the first optional
  augmentation consumer of `striatum corpus export`, not the product boundary
  or a runtime dependency.
- RFC 0044 names the Striatum export as already shipped and says Steps 1 and
  2, the Engram corpus ingester and MCP server, land in the Engram
  repository. Step 3 is the completed Striatum export contract; Steps 4 and 5
  are optional follow-ups and must not make Striatum depend on Engram.
- RFC 0057's non-goals keep Engram-side ingester, retrieval tools, MCP server,
  schema migrations, and memory capability vocabulary out of Striatum.
- `tests/test_corpus_verify.py::test_corpus_v2_surface_keeps_augmentation_boundary_local`
  actively pins the boundary across corpus, CLI, daemon RPC, daemon PG,
  workflow, and Go claim/run surfaces: no Engram imports and no `memory.*`
  Striatum capability strings.
- `tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`
  records the older V1 guardrail shape; the file is now module-skipped as
  legacy SQLite, so the active enforcement lives in the V2 corpus verify test.

## Classification

Engram-side Phase 1 is not a Striatum TODO.

The correct implementation venue is the external Engram repository. A future
Engram workflow should implement and test:

- `engram ingest-striatum --bundle <dir> [--repo <name>]`;
- `engram-mcp-stdio`;
- `engram.search`, `engram.fetch_reference`,
  `engram.describe_corpus`, and `engram.health`;
- Engram-local `memory.read_striatum`, `memory.describe`,
  `memory.read_personal`, `memory.read_cross_tenant`, and
  `memory.read_cross_corpus` checks;
- tenant/corpus authorization tests proving default Striatum operator access
  cannot read personal memory or future application tenants/corpora.

Striatum's durable contract is the local corpus export bundle and optional
reference-only packet metadata. Striatum should continue to run unchanged when
Engram is missing, slow, unreachable, or unconfigured.

## Changed Files

- `docs/operator/plans/deferred-27-engram-side-closure.md`
- `docs/operator/workflows/deferred-27-engram-side-closure/workflow.json`
- `docs/operator/workflows/deferred-27-engram-side-closure/prompts/classify_engram_side_memory_tools.md`
- `docs/operator/artifacts/deferred-27-engram-side-closure/closure/EXTERNAL_ENGRAM_WORK.md`

No shared `docs/TODO.md`, `docs/ROADMAP.md`, `docs/operator/BRIEF.md`,
decision-log, RFC, source, Go, or test files were edited.

## Validation

- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate docs/operator/workflows/deferred-27-engram-side-closure/workflow.json --json`
  - Result: `{"data":{"valid":true,"workflow_id":"deferred-27-engram-side-closure"},"ok":true}`.
- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/pytest -q tests/test_corpus_verify.py::test_corpus_v2_surface_keeps_augmentation_boundary_local`
  - Result: `1 passed in 0.03s`.
- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/pytest -q tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`
  - Result: exit code 4 with `found no collectors`, plus `1 skipped`; the
    file is module-skipped as `legacy sqlite eradicated`. This is not the
    active boundary gate.
- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/pytest -q tests/test_cli_corpus_export.py`
  - Result: exit code 5 with `1 skipped in 0.02s` for the same module-level
    skip.
- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python - <<'PY' ... validate_artifact_front_matter(...)`
  - Result: `front matter valid`.
- `PYTHONDONTWRITEBYTECODE=1 python3 - <<'PY' ... scoped whitespace check`
  - Result: `scoped whitespace valid`.

## Shared Updates To Queue

None required from this worker. The current TODO, roadmap, SPEC, and
ubiquitous-language boundary text already classify Engram-side memory tools as
external optional augmentation work.
