from __future__ import annotations

from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]

CURRENT_AGENT_DOCS = [
    REPO_ROOT / "AGENTS.md",
    REPO_ROOT / "docs" / "HOW_TO_AGENT.md",
    REPO_ROOT / "docs" / "MCP.md",
    REPO_ROOT / "docs" / "SPEC.md",
    *sorted((REPO_ROOT / "src" / "striatum" / "skills" / "templates").rglob("*.tmpl")),
    *sorted((REPO_ROOT / "src" / "striatum" / "plugins" / "templates").rglob("*.tmpl")),
]

STALE_CUTOVER_PHRASES = {
    "It does not replace the claim loop",
    "The CLI is the only client",
    "reacts through the normal CLI verbs",
    "Reactions remain CLI-driven",
    "Call the CLI.",
    "core verb sequence for actually doing work",
    "Wraps `striatum",
}


def test_current_agent_docs_are_mcp_first_after_rfc0050_0075_cutover() -> None:
    stale: list[str] = []
    for path in CURRENT_AGENT_DOCS:
        if "__pycache__" in path.parts:
            continue
        text = path.read_text(encoding="utf-8")
        for phrase in STALE_CUTOVER_PHRASES:
            if phrase in text:
                stale.append(f"{path.relative_to(REPO_ROOT)} contains {phrase!r}")
    assert not stale, "\n".join(stale)

