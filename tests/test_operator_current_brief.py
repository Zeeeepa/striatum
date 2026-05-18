"""Tests for the RFC 0058 V1.5 operator current-brief CLI."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def run_cli(repo: Path, *args: str) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    env.pop("STRIATUM_TEST_HARNESS", None)
    env.pop("STRIATUM_DAEMON_REQUIRED", None)
    return subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )


def write_brief(
    repo: Path,
    *,
    root: str = "docs/operator",
    status: str = "current",
) -> Path:
    brief = repo / root / "BRIEF.md"
    brief.parent.mkdir(parents=True, exist_ok=True)
    brief.write_text(
        "---\n"
        'schema_version: "striatum.operator_brief.v1"\n'
        'artifact_kind: "operator_brief"\n'
        'brief_id: "brief_2026-05-17_test"\n'
        "supersedes: null\n"
        'scope_links: ["docs/operator/plans/rfc-0058-operator-progress-surface.md"]\n'
        "context_budget_lines: 300\n"
        'retrieval_priority: "high"\n'
        f'status: "{status}"\n'
        "---\n\n"
        "# Operator Brief\n"
        "author: coordinator-codex-gpt-5.5-001\n",
        encoding="utf-8",
    )
    return brief


def test_current_brief_json_reads_default_path_without_daemon(tmp_path: Path) -> None:
    (tmp_path / "AGENTS.md").write_text("# instructions\n", encoding="utf-8")
    (tmp_path / "CLAUDE.md").write_text("# claude\n", encoding="utf-8")
    brief = write_brief(tmp_path)

    result = run_cli(tmp_path, "operator", "current-brief", "--json")

    assert result.returncode == 0, result.stderr
    envelope = json.loads(result.stdout)
    data = envelope["data"]
    assert data["path"] == str(brief)
    assert data["brief_id"] == "brief_2026-05-17_test"
    assert data["supersedes"] is None
    assert data["scope_links"] == [
        "docs/operator/plans/rfc-0058-operator-progress-surface.md"
    ]
    assert data["context_budget_lines"] == 300
    assert data["retrieval_priority"] == "high"
    assert data["cold_start_paths"] == [
        "AGENTS.md",
        "CLAUDE.md",
        "docs/operator/BRIEF.md",
        "docs/operator/plans/rfc-0058-operator-progress-surface.md",
    ]


def test_current_brief_text_reads_custom_operator_docs_root(tmp_path: Path) -> None:
    brief = write_brief(tmp_path, root="ops")

    result = run_cli(
        tmp_path,
        "operator",
        "current-brief",
        "--operator-docs-root",
        "ops",
    )

    assert result.returncode == 0, result.stderr
    assert f"path: {brief}" in result.stdout
    assert "brief_id: brief_2026-05-17_test" in result.stdout
    assert "retrieval_priority: high" in result.stdout
    assert "cold_start_paths: AGENTS.md, ops/BRIEF.md" in result.stdout


def test_current_brief_refuses_missing_file(tmp_path: Path) -> None:
    result = run_cli(tmp_path, "operator", "current-brief", "--json")

    assert result.returncode == 8
    envelope = json.loads(result.stdout)
    assert "operator brief not found" in envelope["error"]["message"]


def test_current_brief_refuses_symlink(tmp_path: Path) -> None:
    target = write_brief(tmp_path, root="archive")
    brief = tmp_path / "docs" / "operator" / "BRIEF.md"
    brief.parent.mkdir(parents=True, exist_ok=True)
    brief.symlink_to(target)

    result = run_cli(tmp_path, "operator", "current-brief", "--json")

    assert result.returncode == 8
    envelope = json.loads(result.stdout)
    assert "not a symlink" in envelope["error"]["message"]


def test_current_brief_refuses_non_current_status(tmp_path: Path) -> None:
    write_brief(tmp_path, status="superseded")

    result = run_cli(tmp_path, "operator", "current-brief", "--json")

    assert result.returncode == 8
    envelope = json.loads(result.stdout)
    assert "is not current" in envelope["error"]["message"]
