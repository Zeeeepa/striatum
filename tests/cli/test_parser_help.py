"""RFC 0043 V1.6 F-help — ``striatum daemon migrate-repo-local --help`` regression.

Asserts the help block carries an explanatory ``description`` and a
``help=`` for every flag, including the three operator hooks called out by
the V1.6 design synthesis:

* ``(default)`` on ``--keep-sqlite-readonly``
* ``--confirm-delete`` mentioned on the delete-mode flag
* ``STRIATUM_DAEMON_DB_URL`` mentioned on ``--postgres-url``

Closes claude dogfood-050 F-dx-1.
"""

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
    # The description sentence framing the command's purpose. Substring
    # only — the exact wording can drift as long as it explains the
    # SQLite → Postgres flow.
    assert "PostgreSQL" in out
    assert "SQLite" in out


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
    """The V1.6 synthesis named three operator hooks explicitly.

    These substrings are the easiest things to spot in a help block and
    the most useful to operators wiring up the migrate command.
    """
    out = _migrate_repo_local_help(capsys)
    assert "(default)" in out, "expected explicit (default) on --keep-sqlite-readonly"
    assert "STRIATUM_DAEMON_DB_URL" in out, (
        "expected --postgres-url help to name the env var"
    )
    assert "--confirm-delete" in out


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
