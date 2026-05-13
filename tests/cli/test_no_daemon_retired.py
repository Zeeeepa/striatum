"""RFC 0043 §3 — ``--no-daemon`` is retired.

Parsing the flag must return the standard argparse ``unrecognized
arguments`` error, exit code 2.
"""

from __future__ import annotations

import pytest

from striatum.cli.parser import build_parser


def test_no_daemon_flag_is_unknown(capsys: pytest.CaptureFixture[str]) -> None:
    parser = build_parser()
    with pytest.raises(SystemExit) as exc:
        parser.parse_args(["--no-daemon", "status"])
    assert exc.value.code == 2
    err = capsys.readouterr().err
    assert "unrecognized arguments: --no-daemon" in err


def test_daemon_flag_still_parses() -> None:
    parser = build_parser()
    args = parser.parse_args(["--daemon", "status"])
    assert args.daemon is True
    # The retirement of --no-daemon must not leave a phantom attribute on
    # the Namespace — argparse should refuse to set it.
    assert not hasattr(args, "no_daemon")


def test_help_short_circuits_without_state(capsys: pytest.CaptureFixture[str]) -> None:
    parser = build_parser()
    with pytest.raises(SystemExit) as exc:
        parser.parse_args(["--help"])
    assert exc.value.code == 0
    out = capsys.readouterr().out
    assert "--daemon" in out
    assert "--no-daemon" not in out
