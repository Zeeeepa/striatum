"""Focused tests for generated daemon method reference tables."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path


def test_generate_daemon_method_tables_from_contract(tmp_path: Path) -> None:
    repo_root = Path(__file__).resolve().parents[1]
    output = tmp_path / "daemon_method_tables.md"
    cmd = [
        sys.executable,
        str(repo_root / "scripts" / "generate_daemon_method_tables.py"),
        "--contract",
        str(repo_root / "contracts" / "daemon_methods.json"),
        "--out",
        str(output),
    ]
    subprocess.run(cmd, check=True, cwd=repo_root)

    generated = output.read_text(encoding="utf-8")
    assert "## Daemon Method Registry" in generated
    assert "## CLI Route Translation" in generated
    assert "| `daemon.hello` | none | `daemon_global` | 1 | 1 | no |" in generated
    assert "| `run start` | `run.start` | `admin` | `single_repo` |" in generated
    assert "| `list workflows` | `list.workflows` | `read` | `single_repo` |" in generated
    assert (
        "| `recovery auto-publish` | `recovery.auto_publish_stale_artifacts` "
        "| `recovery` | `single_repo` |"
    ) in generated

    subprocess.run([*cmd, "--check"], check=True, cwd=repo_root)


def test_checked_in_daemon_method_tables_are_current() -> None:
    repo_root = Path(__file__).resolve().parents[1]
    subprocess.run(
        [
            sys.executable,
            str(repo_root / "scripts" / "generate_daemon_method_tables.py"),
            "--check",
        ],
        check=True,
        cwd=repo_root,
    )
