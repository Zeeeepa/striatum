---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["AGENTS.md", "docs/SPEC.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/operator/BRIEF.md", "docs/UBIQUITOUS_LANGUAGE.md", "docs/operator/artifacts/ordered-backlog-2026-05-23/phase-06-generic-language/REPORT.md"]
---

# TODO 16 Generic-Language Scan
author: generic-language-codex-gpt-5-001

## Commands

```bash
rg -n --hidden --glob '!*.pyc' --glob '!.git/**' --glob '!.venv/**' --glob '!docs/dogfood/**' --glob '!docs/ENGRAM_INCUBATION_CONTEXT.md' --glob '!examples/rfc-0014-operational-artifact-home/**' --glob '!prompts/P00*' --glob '!prompts/P0*' "\bEngram\b|\bengram\b|ENGRAM|agent-runner|Engram-specific|Engram-style|Engram repo root|marker names|TARGET_REPO=\.\.|TARGET_REPO=\." .
rg -n --hidden --glob '!*.pyc' --glob '!.git/**' --glob '!docs/dogfood/**' --glob '!docs/ENGRAM_INCUBATION_CONTEXT.md' --glob '!examples/rfc-0014-operational-artifact-home/**' --glob '!prompts/P00*' --glob '!prompts/P0*' "Engram-style|Engram repo root|Engram-specific paths|marker names|agent-runner/|phase3_tmux_agents|STRIATUM_MEMORY_ROADMAP|Engram Phase 3|Engram Phase 1|Engram operator|engram-style" .
```

## Safe Fix Now

- `docs/rfcs/0056-consumer-repo-directory-structure-opinions.md` used
  `Engram-style dogfood corpus` inside a generic layout recommendation.
  This RFC is accepted current product guidance, not frozen incubation
  provenance, so the phrase should become generic structured-run wording.
- `tests/test_doc_links.py` had a correct stale-phrase guardrail, but it only
  scanned `README.md`, `docs/CONSUMER_REPO_LAYOUT.md`, and
  `scripts/striatum_tmux_design.sh`. The RFC 0056 hit shows the guardrail
  should cover the curated current Markdown doc set plus the historical tmux
  script, with explicit historical allowlisting.

## No Action

- `docs/SPEC.md`, `docs/HOW_TO_AGENT.md`, `docs/HOW_TO_HUMAN.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, and `docs/ROADMAP.md` references to Engram
  describe optional augmentation or external/historical context and preserve
  the no-runtime-dependency boundary.
- `docs/INTERVIEW_LOG.md`, `docs/RFC_0014_DOGFOOD_FIX_SPEC.md`,
  `docs/design/V1_MVP_DESIGN_INPUT_*.md`, older prompts, and issue
  reproductions are historical/reference material.
- `CHANGELOG.md` entries are release history and should not be rewritten for
  vocabulary cleanup unless they misstate current behavior outside their
  release context.

## Shared-Doc Updates To Report

No direct edits are needed to `docs/TODO.md`, `docs/ROADMAP.md`, or
`docs/operator/BRIEF.md` for this sweep. TODO 16 should remain open as
standing hygiene.
