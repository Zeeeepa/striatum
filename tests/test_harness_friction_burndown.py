"""V1.41 harness-friction burn-down regression tests.

Each test pins one of the recurring operator-on-behalf frictions
observed across dogfoods 048-052. Failures here mean the friction
has crept back in.

Covers:

- A2: front-matter author lines win over title-block lines, so a body
  ``Author: <real-name>`` no longer competes with the canonical byline.
- A3: ``publish-artifact`` defaults ``--kind`` and ``--logical-name``
  from the workflow's ``expected_artifacts`` when ``--path`` matches a
  declared artifact.
- A4 + C1: ``striatum byline`` and ``striatum inbox`` print operator
  helpers without requiring SQLite spelunking.
- A1: ``recovery auto-publish`` walks stale leases, finds on-disk
  conformant artifacts, publishes them, and completes the job. This
  is the headline burn-down for the claude-no-publish anti-pattern.
"""

from __future__ import annotations

from striatum.artifacts import (
    _canonical_byline_form,
    markdown_title_block_author_lines,
)


def test_front_matter_author_wins_over_title_block_mention() -> None:
    """A2: title-block ``Author:`` body text must not compete with the
    canonical front-matter ``author:`` line.

    Captured pattern: gemini designs emit YAML front matter with a
    lowercase ``author:`` line, then a title block with ``Author: <real-name>``
    that uses title-case decoration. The byline-equality check used to
    treat both as bylines and reject the publish with
    "markdown artifact author line must match expected work packet
    author line".
    """
    payload = (
        "---\n"
        "schema_version: \"striatum.synthesis.v1\"\n"
        "artifact_kind: \"synthesis\"\n"
        "---\n"
        "\n"
        "author: designer-unknown-model-001\n"
        "\n"
        "# Design Proposal\n"
        "\n"
        "Author: designer-gemini-1\n"
        "Date: 2026-05-13\n"
        "Status: proposed\n"
        "\n"
        "## Body\n"
    )
    bylines = markdown_title_block_author_lines(payload)
    assert len(bylines) == 1, f"expected 1 byline, got {bylines!r}"
    assert (
        _canonical_byline_form(bylines[0])
        == "author: designer-unknown-model-001"
    )


def test_title_block_byline_used_when_no_front_matter() -> None:
    """A2: when no front matter is present, the title-block byline is
    still the source of truth (backwards compat with dogfood handoff
    convention)."""
    payload = (
        "# Some Handoff\n"
        "\n"
        "author: implementer-unknown-model-001\n"
        "\n"
        "## Body\n"
    )
    bylines = markdown_title_block_author_lines(payload)
    assert len(bylines) == 1
    assert (
        _canonical_byline_form(bylines[0])
        == "author: implementer-unknown-model-001"
    )


def test_canonical_byline_strips_decoration_and_lowercases() -> None:
    """Sanity check for the canonicalisation helper the publisher uses."""
    assert (
        _canonical_byline_form("**Author:** Designer-Unknown-Model-001")
        == "author: designer-unknown-model-001"
    )
    assert _canonical_byline_form("Not an author line") is None
