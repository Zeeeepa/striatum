---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0017-readme-and-docs-reorganization.md", "docs/dogfood/010/research/MIGRATION_MAP.md", "README.md", "docs/SPEC.md"]
---

# RFC 0017 V1 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0017 (steps 1+2+3 of the RFC's
implementation path). No deferred slice — V1 ships the full
reorganization in one PR because the steps are tightly coupled
(slimming the README without first authoring the new docs would
break links).

## Locked Contracts

### New `README.md` — exactly seven section headers

In this order, no others:

```text
# striatum
(2-paragraph elevator pitch)

## Status
(one paragraph: v1.0.0; all V1 RFCs accepted; link to CHANGELOG)

## Install
(pip install + editable install + smoke; ≤ 20 lines)

## Quick Start (Human Operator)
(5-line bash block + one paragraph; pointer to docs/HOW_TO_HUMAN.md)

## Quick Start (Coding Agent)
(5-line bash block + one paragraph; explains the skill bundle is
the operational answer; pointer to docs/HOW_TO_AGENT.md)

## What It Is For
(one paragraph)

## Documentation Map
(7-row table: file → one-line summary)

## License
(unchanged)
```

Yes, that's eight headers including `# striatum`. The acceptance
criterion in RFC 0017 says "seven sections in this order"; the
implementer reads "the seven `##` section headers", with the title
`# striatum` as the document title (not a section).

Target: `wc -l README.md` ≤ 250.

### New docs (six files)

Each lifted from existing README content with light edits. No
invented prose unless a transition sentence is needed.

| File | Source in current README | Net new prose |
|---|---|---|
| `docs/GETTING_STARTED.md` | Synthesized from § 1 (init), § 3 (prepare), § 4 (start), and the new "I am setting up an agent" pivot | Two-page narrative + new "Are you the operator or are you setting up an agent?" decision tree |
| `docs/HOW_TO_HUMAN.md` | § "Usage Guide" (1–11) + § "Dashboard" | Reorganized header sequence; removes "step N" numbering in favor of named sections |
| `docs/HOW_TO_AGENT.md` | New file; lifts § 5 (register-session), § 6 (claim-loop), § 7 (publish), § 8 (review), § 10 (block) and reframes for the agent's perspective. Cites the RFC 0015 skill bundle as the operational answer | One paragraph at the top: "this doc is the long-form companion to the skill bundle; if you have the bundle loaded, you have everything you need to drive the runner" |
| `docs/WRITING_WORKFLOWS.md` | § "Writing Workflows" + § 2a (custom run scaffold) + § 2b (rendered graph example) | None |
| `docs/CLI_REFERENCE.md` | § "Command Reference" (entire section) | Header note: "may lag the parser; `striatum --help` is authoritative" |
| `docs/INDEX.md` | New file; one row per `docs/*.md` | Brand-new, ≤ 100 lines |
| `docs/dogfood/HISTORICAL.md` | § "Dogfood 001 Usage", § 003, § 004, § 005, § "Bootstrap Tmux Harness" | Header note: "incubation history; current dogfood material is per-id under `docs/dogfood/<id>/`" |

### Dedupe rule

Every `## Behavior Model` paragraph the README sheds is *already*
in `docs/SPEC.md` per the migration map's verification. The
implementer must:

1. Confirm each paragraph is in SPEC. The migration map's
   "verified" column lists the existing SPEC anchors.
2. The single exception is `### Process Adapter Completion
   Guarantees (RFC 0014 V1)` (README line 847). If absent from
   SPEC, append a 5–10 line subsection there citing
   `cli/recovery.py:process_reconcile` and the
   `process_running_*` doctor checks. Do not invent new behavior
   text; lift from the README copy.
3. Delete the README copy of every Behavior Model paragraph.
4. The README's new `## Documentation Map` table replaces the
   long-form behavior model with a 7-row index.

### Cross-link rules

- Internal links use repo-relative paths (`docs/HOW_TO_HUMAN.md`,
  not `https://github.com/.../docs/HOW_TO_HUMAN.md`).
- Section anchors use the Markdown auto-generated form (lowercased,
  spaces → `-`, punctuation stripped). The link-validation test
  resolves *files*, not anchors; anchor checks are deferred.
- README → `docs/`: many. `docs/` → README: zero (the docs are
  the canonical home).
- `AGENTS.md` updates: replace the inline "register-session →
  claim-next → ack → publish → complete" recital with a pointer
  to `docs/HOW_TO_AGENT.md`. AGENTS.md target: ≤ 200 lines (it's
  ~178 today; this trims it further).

### `tests/test_doc_links.py`

```python
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MD_LINK = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")


def _markdown_files() -> list[Path]:
    files = [ROOT / "README.md", ROOT / "AGENTS.md", ROOT / "CLAUDE.md"]
    files.extend(sorted((ROOT / "docs").glob("*.md")))
    files.extend(sorted((ROOT / "docs" / "rfcs").glob("*.md")))
    return [f for f in files if f.exists()]


def test_relative_doc_links_resolve() -> None:
    failures: list[str] = []
    for path in _markdown_files():
        text = path.read_text(encoding="utf-8")
        for match in MD_LINK.finditer(text):
            target = match.group(2).strip()
            if target.startswith(("http://", "https://", "mailto:", "#")):
                continue
            target_path, _, _ = target.partition("#")
            if not target_path:
                continue
            resolved = (path.parent / target_path).resolve()
            if not resolved.exists():
                failures.append(f"{path.relative_to(ROOT)} -> {target}")
    assert not failures, "broken relative links:\n" + "\n".join(failures)


def test_readme_has_human_and_agent_quick_start_headings() -> None:
    text = (ROOT / "README.md").read_text(encoding="utf-8")
    assert "## Quick Start (Human Operator)" in text, (
        "README must keep the human/agent split as first-class headings"
    )
    assert "## Quick Start (Coding Agent)" in text


def test_readme_under_line_budget() -> None:
    lines = (ROOT / "README.md").read_text(encoding="utf-8").splitlines()
    assert len(lines) <= 250, f"README is {len(lines)} lines; budget is 250"
```

Test scope:

- Walk `README.md`, `AGENTS.md`, `CLAUDE.md`, every `docs/*.md`,
  every `docs/rfcs/*.md`. **Exclude** `docs/dogfood/<id>/` per-run
  artifacts — they may legally reference removed README anchors
  in their historical narrative.
- Ignore `http://` / `https://` / `mailto:` / pure-anchor
  (`#section`) links.
- Resolve repo-relative path components only; anchor segments
  after `#` are dropped before resolution.

This adds three tests; total test count moves from 260 → 263.

### CHANGELOG entry

`1.1.0` section (this RFC ships as a minor bump on top of v1.0.0):

```markdown
## 1.1.0 — 2026-05-08

### Changed

- RFC 0017 V1 (dogfood-010): documentation reorganization. README
  trimmed from ~1,000 lines to ≤ 250 with seven canonical sections
  (Status, Install, Quick Start (Human Operator), Quick Start
  (Coding Agent), What It Is For, Documentation Map, License).
  Behavior model, sequential 1–11 usage walkthrough, dogfood-NNN
  history, per-RFC subsections, and command reference moved out of
  README into `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`,
  `docs/HOW_TO_AGENT.md`, `docs/WRITING_WORKFLOWS.md`,
  `docs/CLI_REFERENCE.md`, `docs/INDEX.md`, and
  `docs/dogfood/HISTORICAL.md`. AGENTS.md slimmed to point at
  `docs/HOW_TO_AGENT.md` rather than reciting the verbs inline.
  Three new tests in `tests/test_doc_links.py` enforce relative-
  link integrity, the README line budget, and the human/agent
  quick-start heading split. Documentation only — no behavior
  change, no schema move.
```

## Acceptance Criteria (mirrors RFC 0017 § "Acceptance Criteria")

- `wc -l README.md` ≤ 250.
- README has the seven `##` section headers in the synthesis order.
- All six new `docs/*.md` files exist.
- `docs/dogfood/HISTORICAL.md` exists.
- `grep -c '^## Behavior Model' README.md` = 0.
- `grep -c '^## Dogfood' README.md` = 0.
- `make lint && make typecheck && make test` clean.
- Test count = 263 (260 baseline + 3 new in `test_doc_links.py`).
- `pyproject.toml` and `src/striatum/__init__.py` at `1.1.0`.
- CHANGELOG has the `1.1.0` section.

## Deferred

- `docs/CLI_REFERENCE.md` auto-generation from the parser tree
  (V1.5 follow-up).
- Anchor-level link validation (V1 validates files only).
- A separate CONTRIBUTING.md (AGENTS.md continues to serve).
- Doc site (mkdocs / Sphinx).

## Acceptance Gate

Implementation job blocks until human acceptance recorded under
`docs/dogfood/010/decisions/`.
