---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["README.md", "docs/INDEX.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/rfcs/0055-marketing-readme-and-architecture-graphics.md"]
---

# RFC 0055 SVG Polish No-Action Closure
author: deferred18-rfc0055-codex-gpt-5-001

## Verdict

No README or docs-asset change is required for deferred item 18 in this
slice. RFC 0055 Phase B remains optional polish, and the current README
already satisfies the accepted Phase A goal with a front-page Mermaid
architecture diagram plus a terminal-friendly ASCII architecture view.

## Evidence

- RFC 0055 explicitly chose Mermaid as the Phase A default because it renders
  natively on GitHub, lives in Markdown, and stays reviewable in diffs. The
  same RFC lists SVG as higher polish but heavier contributor burden, and
  scopes Phase B as optional once the shape is stable.
- TODO #46 says Phase A shipped in v1.55.0: README vision/value framing,
  Mermaid system-architecture diagram, and demoted docs link table. It
  classifies the SVG polish follow-up as optional.
- Roadmap §5.8 repeats the same status: RFC 0055 Phase A shipped and the SVG
  polish follow-up is still optional.
- README lines 23-55 carry the front-page Mermaid "At a Glance" architecture
  diagram showing the AI operator, human principal, Striatum runner,
  `striatumd`, Postgres, `.striatum/` scratch, target source, and durable
  artifacts.
- README lines 59-85 add an ASCII architecture stack that is readable in
  terminals and plain Markdown reviews.
- `docs/INDEX.md` already describes the README as the top-level pitch,
  daemon/Postgres quick start, project status, and documentation table. There
  is no missing index entry or broken docs pointer requiring an asset change.

## Classification

Close deferred item 18 as optional no-action for now. Adding an SVG would
duplicate the existing Mermaid and ASCII architecture explanations while
introducing a new asset/source maintenance question. That is not a real docs
need under the current product boundary.

The next useful trigger for reconsidering SVG is not "more polish" by itself;
it is a concrete adopter/evaluator problem, such as GitHub Mermaid rendering
proving inadequate, a documentation site needing a branded scalable image, or
the architecture model stabilizing enough to justify generated SVG with a
tracked source file.

## Validation Evidence

Commands run for this closure:

- `PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate docs/operator/workflows/deferred-18-rfc0055-svg-closure/workflow.json --json`
  -> valid.
- `PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_doc_links.py`
  -> passed.
- `PYTHONPATH=src .venv/bin/python - <<'PY' ... validate_artifact_front_matter(...)`
  -> work-plan and synthesis front matter valid.

## Shared Updates To Queue

None required for this worker. TODO #46 and roadmap §5.8 already mark the SVG
follow-up as optional, and the task explicitly forbids editing shared
TODO/ROADMAP/BRIEF status docs.
