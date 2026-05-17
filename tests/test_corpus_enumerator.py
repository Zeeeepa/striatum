from __future__ import annotations

import subprocess
from pathlib import Path

from striatum.corpus.enumerator import (
    enumerate_changelog,
    enumerate_decisions,
    enumerate_operator_docs,
    enumerate_operator_reports,
    enumerate_rfcs,
    enumerate_ubiquitous_language,
    split_atx_sections,
    split_operator_report,
)


def _git_repo(repo: Path) -> str:
    subprocess.run(["git", "init", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=repo, check=True)
    (repo / "README.md").write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", "README.md"], cwd=repo, check=True)
    subprocess.run(["git", "commit", "-m", "seed"], cwd=repo, check=True, capture_output=True)
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip()


def test_rfc_rows_split_atx_headings_with_stable_external_ids(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    path = tmp_path / "docs/rfcs/0044-demo.md"
    path.parent.mkdir(parents=True)
    path.write_text("# Title\n\nIntro\n\n## Proposal\n\nBody\n", encoding="utf-8")
    rows = enumerate_rfcs(tmp_path, {"docs/rfcs/0044-demo.md"})

    assert [row.external_id for row in rows] == ["rfc:0044#title", "rfc:0044#proposal"]
    assert rows[0].sub_kind == "rfc"


def test_decision_log_rows_parse_decisions_table(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    path = tmp_path / "docs/DECISION_LOG.md"
    path.parent.mkdir(parents=True)
    path.write_text(
        "## Decisions\n\n| ID | Status | Decision | Reason | Consequences | Revisit Trigger |\n"
        "|---|---|---|---|---|---|\n| D123 | accepted | Do it | Because | Works | Later |\n",
        encoding="utf-8",
    )
    rows = enumerate_decisions(tmp_path, {"docs/DECISION_LOG.md"})

    assert rows[0].external_id == "decision:D123"
    assert "Decision: Do it" in rows[0].content


def test_operator_report_splits_intervention_sections_and_fallback() -> None:
    assert split_operator_report("# Report\n\n## Intervention A\n\nDid thing") == [
        "Intervention A\n\nDid thing"
    ]
    assert split_operator_report("author: operator\n\nFirst\n\nSecond") == [
        "author: operator",
        "First",
        "Second",
    ]


def test_operator_report_rows_use_dogfood_ids(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    path = tmp_path / "docs/dogfood/046/OPERATOR_REPORT.md"
    path.parent.mkdir(parents=True)
    path.write_text("# Report\n\n## Intervention A\n\nDid thing\n", encoding="utf-8")
    rows = enumerate_operator_reports(tmp_path, {"docs/dogfood/046/OPERATOR_REPORT.md"})

    assert rows[0].external_id == "dogfood:046#intervention-1"


def test_operator_docs_emit_front_matter_metadata_columns(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    path = tmp_path / "docs/operator/BRIEF.md"
    path.parent.mkdir(parents=True)
    path.write_text(
        "---\n"
        'schema_version: "striatum.operator_brief.v1"\n'
        'artifact_kind: "operator_brief"\n'
        'brief_id: "brief_2026-05-17_daemon-port"\n'
        'supersedes: "brief_2026-05-16_architecture-review"\n'
        'scope_links: ["docs/operator/plans/rfc-0068-go-daemon-port.md"]\n'
        "context_budget_lines: 300\n"
        'retrieval_priority: "high"\n'
        'status: "current"\n'
        "---\n\n"
        "# Operator Brief\n\nCurrent state.\n",
        encoding="utf-8",
    )

    rows = enumerate_operator_docs(tmp_path, {"docs/operator/BRIEF.md"})

    assert len(rows) == 1
    row_json = rows[0].to_json()
    assert rows[0].external_id == "operator_brief:brief_2026-05-17_daemon-port"
    assert row_json["artifact_kind"] == "operator_brief"
    assert row_json["retrieval_priority"] == "high"
    assert row_json["supersedes"] == "brief_2026-05-16_architecture-review"
    assert rows[0].content == "# Operator Brief\n\nCurrent state."


def test_changelog_and_ubiquitous_language_parsers(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    changelog = tmp_path / "CHANGELOG.md"
    changelog.write_text("# Changelog\n\n## v1.2.3\n\nChanged\n", encoding="utf-8")
    language = tmp_path / "docs/UBIQUITOUS_LANGUAGE.md"
    language.parent.mkdir(parents=True, exist_ok=True)
    language.write_text(
        "## Core Terms\n\n| Term | Definition |\n|---|---|\n| Run | One workflow attempt |\n",
        encoding="utf-8",
    )

    assert enumerate_changelog(tmp_path, {"CHANGELOG.md"})[0].external_id == "changelog:v1.2.3"
    assert enumerate_ubiquitous_language(tmp_path, {"docs/UBIQUITOUS_LANGUAGE.md"})[0].external_id == "ulang:run"


def test_run_summary_guard_has_no_forbidden_uppercase_joins() -> None:
    body = Path("src/striatum/corpus/enumerator.py").read_text(encoding="utf-8")
    assert "FROM runs" not in body
    assert "FROM verdicts" not in body
    assert "FROM artifacts" not in body
    assert "FROM jobs" not in body
    assert "FROM sessions" not in body


def test_split_atx_sections_ignores_empty_sections() -> None:
    assert split_atx_sections("# Title\n\nBody\n\n## Empty\n") == [("Title", "Body")]
