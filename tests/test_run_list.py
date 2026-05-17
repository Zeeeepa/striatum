from __future__ import annotations

from pathlib import Path

from striatum.web.run_list import (
    parse_github_remote,
    repo_relative_source_path,
    workflow_tree_url,
)


def test_parse_github_remote_accepts_common_github_shapes() -> None:
    assert parse_github_remote("git@github.com:owner/repo.git") == "https://github.com/owner/repo"
    assert parse_github_remote("ssh://git@github.com/owner/repo") == "https://github.com/owner/repo"
    assert parse_github_remote("https://github.com/owner/repo.git") == "https://github.com/owner/repo"


def test_parse_github_remote_rejects_non_github_or_non_repo_shapes() -> None:
    assert parse_github_remote(None) is None
    assert parse_github_remote("https://example.com/owner/repo") is None
    assert parse_github_remote("https://github.com/owner") is None


def test_workflow_tree_url_links_to_workflow_directory() -> None:
    assert (
        workflow_tree_url(
            "https://github.com/owner/repo",
            "feature/x",
            "examples/wf/workflow.json",
        )
        == "https://github.com/owner/repo/tree/feature%2Fx/examples/wf"
    )
    assert (
        workflow_tree_url("https://github.com/owner/repo", "main", "workflow.json")
        == "https://github.com/owner/repo/tree/main"
    )


def test_repo_relative_source_path_normalizes_paths_inside_repo(tmp_path: Path) -> None:
    workflow_path = tmp_path / "examples" / "workflow.json"
    workflow_path.parent.mkdir()
    workflow_path.write_text("{}", encoding="utf-8")

    assert repo_relative_source_path(tmp_path, str(workflow_path)) == "examples/workflow.json"
    assert repo_relative_source_path(tmp_path, "examples/workflow.json") == "examples/workflow.json"
