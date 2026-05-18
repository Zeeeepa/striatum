"""``striatum daemon migrate-repo-local --help`` compatibility coverage."""

from __future__ import annotations

import pytest

from striatum.cli.parser import build_parser


def _migrate_repo_local_help(capsys: pytest.CaptureFixture[str]) -> str:
    parser = build_parser()
    try:
        parser.parse_args(["daemon", "migrate-repo-local", "--help"])
    except SystemExit as exc:
        assert exc.code == 0
    return capsys.readouterr().out


def _help_for(args: list[str], capsys: pytest.CaptureFixture[str]) -> str:
    parser = build_parser()
    try:
        parser.parse_args([*args, "--help"])
    except SystemExit as exc:
        assert exc.code == 0
    return capsys.readouterr().out


def test_help_includes_description(capsys: pytest.CaptureFixture[str]) -> None:
    out = _migrate_repo_local_help(capsys)
    assert "Retired compatibility command" in out
    assert "SQLite import windows are closed" in out


def test_help_documents_every_migrate_flag(
    capsys: pytest.CaptureFixture[str],
) -> None:
    out = _migrate_repo_local_help(capsys)
    expected = (
        "--from",
        "--to",
        "--repo",
        "--postgres-url",
        "--dry-run",
        "--verify-cutover",
        "--keep-sqlite-readonly",
        "--no-keep-sqlite-readonly",
        "--confirm-delete",
        "--json",
    )
    for flag in expected:
        assert flag in out, f"flag {flag!r} missing from --help output"


def test_help_mentions_default_and_env_and_confirm_delete(
    capsys: pytest.CaptureFixture[str],
) -> None:
    out = _migrate_repo_local_help(capsys)
    assert "retired compatibility option" in out
    assert "legacy SQLite files are left untouched" in out
    assert "STRIATUM_DAEMON_DB_URL" in out


def test_rfc0053_help_uses_operator_and_reader_facing_terms(
    capsys: pytest.CaptureFixture[str],
) -> None:
    init_help = _help_for(["init"], capsys)
    assert "reader-facing" in init_help
    assert "doc layout" in init_help
    assert "human-facing doc layout" not in init_help

    requeue_help = _help_for(["recovery", "requeue-stale"], capsys)
    assert "after operator" in requeue_help
    assert "inspection; pair with --justification" in requeue_help
    assert "after manual inspection" not in requeue_help


def test_operator_current_brief_help_documents_local_read_options(
    capsys: pytest.CaptureFixture[str],
) -> None:
    out = _help_for(["operator", "current-brief"], capsys)
    assert "current operator brief metadata" in out
    assert "--operator-docs-root" in out
    assert "--json" in out
