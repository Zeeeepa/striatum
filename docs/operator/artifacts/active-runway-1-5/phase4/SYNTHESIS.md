---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md", "docs/operator/artifacts/active-runway-1-5/phase4/catalog/PLAN.md", "docs/operator/artifacts/active-runway-1-5/phase4/examples/PLAN.md"]
---

# Synthesis: RFC 0074 Phase A Patch Sequence

author: catalog-planner-gemini-001
status: accepted
date: 2026-05-22

## Executive Summary

This synthesis defines the Phase A implementation for RFC 0074. The goal is to expand the Striatum workflow catalog from simple graph shapes to a richer vocabulary of **graph shapes**, **role packs**, and **adversary packs**. Phase A is strictly focused on **metadata-plus-one-example**: adding the catalog definitions and one runnable reference workflow without changing generator core or runtime validation.

## Phase A Patch Sequence

The implementation is divided into six logical patches to ensure clean review and disjoint testing.

### Patch 1: Catalog Loader Foundation
- **Changes**: Update `src/striatum/workflow_generator/catalog.py`.
- **Logic**: Extend the `Catalog` model and loader to support optional `role_packs` and `adversary_packs` arrays. Add validation for pack IDs, kinds, and child role/posture objects.
- **API**: Add `list_role_packs()`, `get_role_pack()`, `list_adversary_packs()`, and `get_adversary_pack()`.

### Patch 2: Catalog Metadata Expansion
- **Changes**: Update `src/striatum/workflow_templates/catalog.json`.
- **Content**:
  - Add 6 graph shapes: `implementation_panel`, `strategy_review`, `premortem`, `release_readiness`, `incident_response`, `code_doc_audit` (from RFC 0076).
  - Add 6 role packs: `implementation_panel`, `strategy_panel`, `premortem`, `release_readiness`, `incident_response`, `authority_docs_operator_audit`.
  - Add 7 adversary packs: `security_privacy`, `maintainer_cost`, `operator_ergonomics`, `premortem`, `product_strategy`, `provenance_integrity`, `code_doc_audit_postures`.

### Patch 3: CLI & Web Discovery
- **Changes**: Update `src/striatum/cli/workflow.py` and web template handlers.
- **CLI**: Extend `workflow templates list --kind` and `show <id>` to support packs. Update `render-md` to include pack sections.
- **Web**: Update `GET /workflow-templates` to include packs in the response.

### Patch 4: Reference Example: Implementation Panel
- **Changes**: Create `examples/implementation-panel-flow/`.
- **Content**:
  - `workflow.json`: Hand-authored 11-job graph (3 proposals, 3 scorecards, tradeoff ledger, arbitrator, dissent review, decision).
  - `prompts/`: 7 task prompts defining scorecard dimensions and role intents.
  - `README.md`: Fixture documentation.

### Patch 5: Metadata & Example Validation
- **Changes**: Create `tests/test_implementation_panel_flow.py` and update `tests/test_workflow_generator.py`.
- **Metadata Tests**: Verify loader validation, duplicate ID rejection, and markdown rendering.
- **Example Tests**: Verify the implementation-panel graph validates, referenced files exist, and write scopes are disjoint.

### Patch 6: Documentation Alignment
- **Changes**: Update `docs/WORKFLOW_TYPES.md` and `examples/README.md`.
- **Content**: Link the new graph shapes and role/adversary axes into the operator selection guide.

## Disjoint Write Scopes & Serialization Points

| Category | Write Scope | Serialization Point |
|---|---|---|
| **Core Logic** | `src/striatum/workflow_generator/catalog.py` | Shared (Patch 1) |
| **Data** | `src/striatum/workflow_templates/catalog.json` | Shared (Patch 2) |
| **CLI/Web** | `src/striatum/cli/`, `src/striatum/web/` | Disjoint (Patch 3) |
| **Examples** | `examples/implementation-panel-flow/` | Disjoint (Patch 4) |
| **Tests** | `tests/` | Disjoint (Patch 5) |
| **Docs** | `docs/`, `examples/README.md` | Disjoint (Patch 6) |

## Deferred to Phase B

- **Generator Implementation**: `workflow generate` support for the 6 new shapes.
- **Generator Flags**: `--role-pack` and `--adversary-pack` flags.
- **UI Generation Wizard**: Interactive pack selection in the web builder.
- **Cost Estimation**: Logic for estimating costs of multi-review/panel shapes.

## RFC 0076 Integration

RFC 0076 (Code/Doc Audit) is integrated into the Phase A catalog as a first-class citizen:
- The hand-authored RFC 0076 workflow is generalized into the `code_doc_audit` catalog entry.
- The `authority_docs_operator_audit` role pack and `code_doc_audit_postures` adversary pack are added.
- No new schemas (e.g., `audit_finding`) are introduced; audits continue to use `finding` and `findings_ledger`.

## Validation Commands

```bash
# Validate the new catalog loader and metadata
PYTHONPATH=src pytest tests/test_workflow_generator.py

# Validate the new reference example
PYTHONPATH=src python3 -m striatum.cli workflow validate examples/implementation-panel-flow/workflow.json
PYTHONPATH=src pytest tests/test_implementation_panel_flow.py

# Verify CLI exposure
striatum workflow templates list --kind role_pack
striatum workflow templates show implementation_panel
```

## Guardrails

- **Metadata Only**: Do not add generator code for new shapes in Phase A.
- **No Schema Bloat**: Do not add new artifact kinds or front-matter schemas.
- **Local-First**: Do not add external template fetching or remote catalog syncing.
- **Independence**: Role and adversary packs are documentation/authoring concepts; do not bind them to runtime daemon state or model identity.
- **Safety**: Maintain `forbidden_paths: [".striatum/"]` in all example jobs.
