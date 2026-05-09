"""RFC 0023 V1.5 tests: chat_tools.execute_tool + per-tool schemas.

Direct unit tests of the tool dispatcher. Covers the closed-set
membership check, path safety, size caps, hidden-path refusal, and
wrap_tool_result delimiters.
"""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path

from striatum.web.chat_tools import (
    ANTHROPIC_TOOLS,
    OPENAI_TOOLS,
    TOOL_NAMES,
    execute_tool,
    wrap_tool_result,
)


def _git_init(repo: Path) -> None:
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    subprocess.run(["git", "checkout", "-qb", "main"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.email", "t@e.com"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.name", "t"], cwd=repo, check=True)
    (repo / "seed.txt").write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", "."], cwd=repo, check=True)
    subprocess.run(["git", "commit", "-qm", "seed", "--no-gpg-sign"], cwd=repo, check=True)


# --- closed-set ------------------------------------------------------


def test_execute_tool_closed_set_refuses_unknown(tmp_path: Path) -> None:
    out = execute_tool("rm_rf", {"path": "/"}, repo=tmp_path)
    assert out.startswith("[error] unknown tool")


def test_tool_names_match_schemas() -> None:
    """ANTHROPIC_TOOLS + OPENAI_TOOLS expose the same set as TOOL_NAMES."""
    anthropic_names = {t["name"] for t in ANTHROPIC_TOOLS}
    openai_names = {t["function"]["name"] for t in OPENAI_TOOLS}
    assert anthropic_names == TOOL_NAMES == openai_names


# --- read_file -------------------------------------------------------


def test_read_file_basic(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "hello.md").write_text("# hi\n", encoding="utf-8")
    out = execute_tool("read_file", {"path": "hello.md"}, repo=tmp_path)
    assert "# hi" in out


def test_read_file_path_traversal_refused(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("read_file", {"path": "../../etc/passwd"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_read_file_dotgit_hidden(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("read_file", {"path": ".git/HEAD"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_read_file_nonexistent(tmp_path: Path) -> None:
    out = execute_tool("read_file", {"path": "nope.md"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_read_file_size_cap(tmp_path: Path) -> None:
    _git_init(tmp_path)
    big = "x" * (70 * 1024)
    (tmp_path / "big.txt").write_text(big, encoding="utf-8")
    out = execute_tool("read_file", {"path": "big.txt"}, repo=tmp_path)
    assert "[truncated" in out
    # The body before the truncation marker should be at most the cap.
    body, _, _ = out.partition("[truncated")
    assert len(body.encode("utf-8")) <= 64 * 1024 + 5


# --- list_dir --------------------------------------------------------


def test_list_dir_filters_hidden(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "subdir").mkdir()
    (tmp_path / "alpha.txt").write_text("a", encoding="utf-8")
    out = execute_tool("list_dir", {"path": "."}, repo=tmp_path)
    assert ".git" not in out
    assert ".striatum" not in out
    assert "alpha.txt" in out
    assert "subdir" in out


def test_list_dir_directory_not_a_directory(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "f.txt").write_text("x", encoding="utf-8")
    out = execute_tool("list_dir", {"path": "f.txt"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_list_dir_traversal_refused(tmp_path: Path) -> None:
    out = execute_tool("list_dir", {"path": "../.."}, repo=tmp_path)
    assert out.startswith("[error]")


# --- git_log / git_diff ---------------------------------------------


def test_git_log_default_limit(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("git_log", {}, repo=tmp_path)
    assert "seed" in out


def test_git_log_respects_limit(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("git_log", {"limit": 1}, repo=tmp_path)
    assert len(out.splitlines()) == 1


def test_git_diff_clean_tree(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("git_diff", {}, repo=tmp_path)
    assert out == "[no uncommitted changes]"


def test_git_diff_dirty_tree(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "seed.txt").write_text("changed\n", encoding="utf-8")
    out = execute_tool("git_diff", {}, repo=tmp_path)
    assert "changed" in out


# --- striatum_status / striatum_why ---------------------------------


def test_striatum_status_in_initialized_repo(tmp_path: Path) -> None:
    _git_init(tmp_path)
    # Initialize striatum so the API can answer.
    env = os.environ.copy()
    env["PYTHONPATH"] = str(Path(__file__).resolve().parents[1] / "src")
    import sys
    subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init"],
        cwd=tmp_path, env=env, check=True, capture_output=True,
    )
    out = execute_tool("striatum_status", {}, repo=tmp_path)
    parsed = json.loads(out)
    assert parsed.get("ok") is True


# --- wrap_tool_result -----------------------------------------------


def test_wrap_tool_result_includes_delimiters() -> None:
    wrapped = wrap_tool_result("read_file", {"path": "x.md"}, "the content")
    assert "<tool_result_begin" in wrapped
    assert "<tool_result_end" in wrapped
    assert "the content" in wrapped


def test_wrap_tool_result_arg_serialization_stable() -> None:
    """Args are JSON-serialized with sorted keys for determinism."""
    a = wrap_tool_result("foo", {"a": 1, "b": 2}, "r")
    b = wrap_tool_result("foo", {"b": 2, "a": 1}, "r")
    assert a == b
