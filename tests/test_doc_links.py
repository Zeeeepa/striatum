from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MD_LINK = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")


def _markdown_files() -> list[Path]:
    """Return the curated set of Markdown files this test guards.

    Per-run dogfood artifacts under ``docs/dogfood/<id>/`` are
    excluded — they may legally reference removed README anchors
    in their historical narrative.
    """
    files = [ROOT / "README.md", ROOT / "AGENTS.md", ROOT / "CLAUDE.md"]
    files.extend(sorted((ROOT / "docs").glob("*.md")))
    files.extend(sorted((ROOT / "docs" / "rfcs").glob("*.md")))
    files.append(ROOT / "docs" / "dogfood" / "HISTORICAL.md")
    files.append(ROOT / "docs" / "dogfood" / "FRICTION_LOG.md")
    return [f for f in files if f.exists()]


def test_relative_doc_links_resolve() -> None:
    """Every Markdown link in the curated doc set must point at a file
    that exists on disk. URLs and pure-anchor links are skipped."""
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
    """The human/agent split is a first-class README contract per
    RFC 0017; this guard prevents a silent regression."""
    text = (ROOT / "README.md").read_text(encoding="utf-8")
    assert "## Quick Start (Human Operator)" in text, (
        "README must keep the human-operator quick-start heading"
    )
    assert "## Quick Start (Coding Agent)" in text, (
        "README must keep the coding-agent quick-start heading"
    )


def test_readme_under_line_budget() -> None:
    """RFC 0017 caps the README at 250 lines; the cap exists so the
    README stays first-contact material instead of growing back into
    a SPEC duplicate."""
    lines = (ROOT / "README.md").read_text(encoding="utf-8").splitlines()
    assert len(lines) <= 250, (
        f"README is {len(lines)} lines; RFC 0017 budget is 250"
    )


# DECISION_LOG row word budget. Per docs/DOC_MAP.md, a D-row is a
# receipt — one to two sentences per cell. The cap exists so the log
# does not silently grow into RFC reference material; detail belongs
# in the RFC and the dogfood BUILD_HANDOFF, both of which are linked
# from the row.
#
# Enforced from D055 onward (the cleanup boundary). Earlier rows are
# grandfathered as-is — rewriting them would scrub historical record.
DECISION_ROW_WORD_BUDGET = 200
DECISION_ROW_BUDGET_FROM = 55


def test_decision_log_rows_under_word_budget() -> None:
    text = (ROOT / "docs" / "DECISION_LOG.md").read_text(encoding="utf-8")
    failures: list[str] = []
    for line in text.splitlines():
        if not line.startswith("| D"):
            continue
        cells = [c.strip() for c in line.strip("|").split("|")]
        if not cells:
            continue
        row_id = cells[0]
        try:
            row_num = int(row_id.lstrip("D"))
        except ValueError:
            continue
        if row_num < DECISION_ROW_BUDGET_FROM:
            continue
        prose_cells = cells[2:] if len(cells) > 2 else []
        word_count = sum(len(c.split()) for c in prose_cells)
        if word_count > DECISION_ROW_WORD_BUDGET:
            failures.append(f"{row_id}: {word_count} words")
    assert not failures, (
        "DECISION_LOG rows over budget (see docs/DOC_MAP.md): "
        + ", ".join(failures)
    )
