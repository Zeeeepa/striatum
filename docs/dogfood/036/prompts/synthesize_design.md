# Synthesize Design Prompt

Produce `docs/dogfood/036/DESIGN_SYNTHESIS.md`. The file must start with a `striatum.synthesis.v1` front matter block (JSON-encoded values; quote strings; JSON arrays for lists):

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/036/design/codex/DESIGN.md", "docs/dogfood/036/design/claude_code/DESIGN.md", "docs/dogfood/036/design/gemini/DESIGN.md"]
---
```

The byline appears AFTER the front matter block, as a plain Markdown line `author: <slug>` (lowercase, no bold/italics/heading/quotes).

Read all three design artifacts and synthesize ONE implementation plan for RFC 0034 (workflow generator + template catalog). The synthesis must explicitly choose, not enumerate.

Required sections:

- **Accepted Implementation Scope** — map each RFC 0034 §Acceptance Criteria bullet 1:1 to a concrete code-and-test plan, with one named owner per bullet (which `src/striatum/` module, which test file).
- **Deferred Scope** — web `/workflows/new` chooser UI (RFC 0034 §9; deferred to a follow-up dogfood), chat-assisted scaffolding tool (RFC 0034 §10; deferred), target-repo local catalog extensions (RFC 0034 §6 V1.5; deferred), automatic repository inspection for suggested shapes (deferred indefinitely). Each line says why deferred and where it lands.
- **WorkflowGenerationSpec Schema** — concrete value-object fields, JSON-serializable, with required vs optional and `field_path` error coverage.
- **GeneratedWorkflow Envelope** — concrete shape returned identically by Python API + CLI `--json` + local API (`workflow`, `files`, `metadata`, `warnings`, `validation`).
- **Built-In Shapes** — for each of `minimal`, `review`, `code_change`, `human_checkpoint`, `evidence_backed`, `multi_review_synthesis`, `custom`: a one-line graph description and one-line catalog summary.
- **Built-In Lane Sets** — for each of `local`, `single_agent`, `author_reviewer`, `multi_review`, `custom`: a one-line lane topology and one-line catalog summary.
- **Lane-Modifier Compatibility Matrix** — full table covering every modifier × lane-set cell with `required` / `allowed` / `forbidden` / `warning` decisions. Field-specific error message shape for `forbidden` cells.
- **Catalog Metadata Layout** — package-data tree (`src/striatum/workflow_templates/...`), shape entry shape, lane-set entry shape, loader semantics (cached at startup, never fetched remotely).
- **Custom-Plan Compiler** — closed block vocabulary, refusal cases (each with `field_path`), edge/cycle safety rules, immediate validation of compiled `workflow.json`.
- **Public Python API** — `generate_workflow(spec) -> GeneratedWorkflow`, error-envelope shape, `workflow validate` invocation before return.
- **CLI Surface** — exact verb shapes, required vs optional flags, `--dry-run` semantics (writes nothing), refuse-to-overwrite posture, `--json` output, help-text quality bar.
- **Local API Surface** — `GET /workflow-templates`, `GET /workflow-templates/<id>`, `POST /workflows/generate/preview`, `POST /workflows/generate` (requires `confirm_write: true`, behind `--allow-mutations`), structured field errors with `field_path`.
- **`workflow init --style` Backwards-Compatibility** — `minimal|review|code-change` keeps working; dispatches through `generate_workflow` internally.
- **Schema Migration** — none expected for V1 (catalog is package data; workflow.json schema is unchanged). State this explicitly so the implementer doesn't invent one.
- **Test Strategy** — unit-level tests per RFC 0034 acceptance criteria, lane-modifier compatibility matrix coverage, custom-plan compiler refusal-case coverage, CLI `--dry-run` write-nothing tests, local API preview/write tests with `confirm_write` gating.
- **Documentation Deltas** — SPEC / WORKFLOW_TYPES / WRITING_WORKFLOWS / CLI_REFERENCE / UBIQUITOUS_LANGUAGE / HOW_TO_HUMAN / RFC 0034 status / CHANGELOG.
- **Staging Plan** — what lands in this dogfood (V1 generator core + catalog + CLI + local API + custom-plan compiler + `workflow init --style` rewire) vs deferred to a future dogfood (web chooser UI + chat tool + local catalog extensions).
- **Human-Decision Questions** — any open questions the implementer cannot resolve from the synthesis alone.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim AFTER the front matter and a blank line.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
