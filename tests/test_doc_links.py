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


def _is_historical_dogfood_link(target: str) -> bool:
    """True when ``target`` references a file inside a numbered
    ``docs/dogfood/<id>/`` run directory.

    Per RFC 0072 step 8, per-run dogfood bodies live in
    S3-compatible blob storage at
    ``dogfood-historical/<dogfood_id>/<rel_path>``; only the
    ``HISTORICAL.md`` and ``FRICTION_LOG.md`` indexes remain
    git-tracked. Older docs that reference per-run bodies (~25
    long-standing pointers in DECISION_LOG, SPEC, INDEX, DOC_MAP,
    FRONTEND_DEVELOPMENT, and a handful of RFCs) keep the
    historical anchor for context even though the file is no
    longer on disk. See docs/BLOB_TRANSITION.md.
    """
    parts = target.lstrip("./").split("/")
    while parts and parts[0] in ("..", ""):
        parts = parts[1:]
    if len(parts) < 2 or parts[0] != "dogfood":
        return False
    second = parts[1]
    return second not in {"HISTORICAL.md", "FRICTION_LOG.md"}


def test_relative_doc_links_resolve() -> None:
    """Every Markdown link in the curated doc set must point at a file
    that exists on disk. URLs, pure-anchor links, and (per RFC 0072)
    historical ``dogfood/<id>/...`` paths are skipped."""
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
            normalized = target_path.replace("\\", "/")
            if _is_historical_dogfood_link(normalized):
                continue
            resolved = (path.parent / target_path).resolve()
            if not resolved.exists():
                failures.append(f"{path.relative_to(ROOT)} -> {target}")
    assert not failures, "broken relative links:\n" + "\n".join(failures)


def test_readme_has_human_and_agent_quick_start_headings() -> None:
    """The human/agent split is a first-class README contract per
    RFC 0017; this guard prevents a silent regression.

    Updated for the marketing-framing rewrite: the H2 headings now use
    title case (``## Quick Start``, ``## The Two Roles``). The
    contract being guarded is "both roles surface explicitly in the
    README"; the heading-case check is intentionally case-insensitive
    so a future copy-edit can re-title these sections without breaking
    the test.
    """
    text = (ROOT / "README.md").read_text(encoding="utf-8")
    lower = text.lower()
    assert "## quick start" in lower, "README must keep a quick-start section"
    assert "## the two roles" in lower, (
        "README must explicitly cover both AI operator and human principal "
        "roles (RFC 0053)"
    )
    assert "AI operator" in text, "README must name the AI operator role"
    assert "Human principal" in text, "README must name the human principal role"


def test_readme_under_line_budget() -> None:
    """RFC 0017 originally capped the README at 250 lines; the
    marketing rewrite (motivation + architecture diagram + workflow
    shapes block + role coverage) intentionally raised the ceiling.
    Hold the new ceiling at 400 lines so the README does not silently
    grow back into a SPEC duplicate."""
    lines = (ROOT / "README.md").read_text(encoding="utf-8").splitlines()
    assert len(lines) <= 400, (
        f"README is {len(lines)} lines; budget is 400 (raised from 250 "
        "for the marketing rewrite; grow this deliberately, not silently)"
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


# GH #15 — post-D094 / RFC 0043 state model. Current product docs
# must not claim that `.striatum/state.sqlite3` is the authoritative
# live state. Historical RFC text, the per-repo migration runbook,
# and references that describe the V1 substrate or the post-migration
# tombstone explicitly are still allowed.
STALE_SQLITE_AUTHORITY_PATTERNS = (
    re.compile(r"live state is\s+`?\.striatum/state\.sqlite3", re.IGNORECASE),
    re.compile(
        r"`?\.striatum/state\.sqlite3`?\s+(?:is|remains)\s+(?:the\s+)?authoritative",
        re.IGNORECASE,
    ),
    re.compile(
        r"repo-local retired local state\s+(?:is|remains)\s+(?:the\s+)?authoritative",
        re.IGNORECASE,
    ),
)
# Files allowed to keep stale wording. Historical incubation prompts,
# RFC source-of-truth text, and the dogfood archive document V1
# behavior verbatim and are not part of current product docs.
STALE_SQLITE_AUTHORITY_ALLOW = {
    Path("docs/rfcs"),
    Path("docs/dogfood"),
    Path("docs/ENGRAM_INCUBATION_CONTEXT.md"),
    Path("docs/INTERVIEW_LOG.md"),
    Path("docs/PRIOR_ART.md"),
    Path("docs/RFC_0014_DOGFOOD_FIX_SPEC.md"),
    # DECISION_LOG.md records the historical D006/D007/D086 carve-outs
    # plus the D094 supersession in the same file; the historical rows
    # remain as decision provenance and are not rewritten.
    Path("docs/DECISION_LOG.md"),
}


def _is_under_allowlist(path: Path, root: Path) -> bool:
    rel = path.resolve().relative_to(root)
    for allow in STALE_SQLITE_AUTHORITY_ALLOW:
        if rel == allow or any(rel.parts[: len(allow.parts)] == allow.parts for _ in [None]):
            if rel.parts[: len(allow.parts)] == allow.parts:
                return True
    return False


def test_current_product_docs_do_not_claim_sqlite_authority() -> None:
    """Post-D094 / RFC 0043 the authoritative live state is the daemon-
    owned PostgreSQL substrate; current product docs must not state that
    `.striatum/state.sqlite3` is authoritative live state. Historical
    RFCs, dogfood scaffolds, and incubation provenance are allowlisted
    so the regression test does not rewrite frozen artifacts.
    """
    failures: list[str] = []
    for path in _markdown_files():
        if _is_under_allowlist(path, ROOT):
            continue
        text = path.read_text(encoding="utf-8")
        for pattern in STALE_SQLITE_AUTHORITY_PATTERNS:
            for match in pattern.finditer(text):
                line_no = text.count("\n", 0, match.start()) + 1
                failures.append(
                    f"{path.relative_to(ROOT)}:{line_no}: {match.group(0)!r}"
                )
    assert not failures, (
        "current product docs must not claim retired local state is authoritative live "
        "state (D094 / RFC 0043 moved workflow state to the daemon-owned "
        "PostgreSQL substrate); see docs/POSTGRES_TRANSITION.md:\n"
        + "\n".join(failures)
    )


def test_operator_glossary_uses_daemon_authority_boundary() -> None:
    text = (ROOT / "docs" / "UBIQUITOUS_LANGUAGE.md").read_text(encoding="utf-8")
    stale = (
        "runs `striatum` CLI verbs against a target repository's "
        "`.striatum/state.sqlite3`"
    )
    assert stale not in text, (
        "operator glossary must describe the post-D094 daemon/PostgreSQL "
        "authority boundary, not the retired repo-local retired local state target"
    )


CURRENT_DOC_STALE_GENERIC_PHRASES = (
    "Corpus Contract V2 RFC in design",
    "Engram-style",
    "Engram repo root",
)
CURRENT_DOC_STALE_GENERIC_ALLOW = {
    Path("docs/dogfood"),
    Path("docs/ENGRAM_INCUBATION_CONTEXT.md"),
    Path("docs/INTERVIEW_LOG.md"),
    Path("docs/PRIOR_ART.md"),
    Path("docs/RFC_0014_DOGFOOD_FIX_SPEC.md"),
}
CURRENT_DOC_STALE_GENERIC_EXTRA_FILES = (
    ROOT / "scripts" / "striatum_tmux_design.sh",
)


def test_current_product_docs_do_not_regress_stale_engram_framing() -> None:
    """Historical Engram references remain allowed, but current status and
    layout guidance must not frame Striatum as Engram-specific.
    """
    checked = [*_markdown_files(), *CURRENT_DOC_STALE_GENERIC_EXTRA_FILES]
    failures: list[str] = []
    for path in checked:
        if any(
            path.resolve().relative_to(ROOT).parts[: len(allow.parts)] == allow.parts
            for allow in CURRENT_DOC_STALE_GENERIC_ALLOW
        ):
            continue
        text = path.read_text(encoding="utf-8")
        for phrase in CURRENT_DOC_STALE_GENERIC_PHRASES:
            index = text.find(phrase)
            if index != -1:
                line_no = text.count("\n", 0, index) + 1
                failures.append(f"{path.relative_to(ROOT)}:{line_no}: {phrase!r}")
    assert not failures, "stale current-doc generic language:\n" + "\n".join(failures)


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
