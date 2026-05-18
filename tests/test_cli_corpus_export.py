from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

ROOT = Path(__file__).resolve().parents[1]


def _run(repo: Path, *args: str, check: bool = True) -> dict[str, Any]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(f"failed: {result.args}\n{result.stdout}\n{result.stderr}")
    payload = cast(dict[str, Any], json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def _git_repo(repo: Path) -> str:
    subprocess.run(["git", "init", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=repo, check=True)
    (repo / "README.md").write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", "README.md"], cwd=repo, check=True)
    subprocess.run(["git", "commit", "-m", "seed"], cwd=repo, check=True, capture_output=True)
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip()


def _legacy_init_repo(repo: Path) -> None:
    from striatum.legacy_sqlite.db import init_repo

    init_repo(repo)


def test_corpus_export_cli_success_and_manifest(tmp_path: Path) -> None:
    base = _git_repo(tmp_path)
    _legacy_init_repo(tmp_path)
    (tmp_path / "docs/rfcs").mkdir(parents=True)
    (tmp_path / "docs/rfcs/0044-demo.md").write_text("# Demo\n\nBody\n", encoding="utf-8")

    payload = _run(tmp_path, "corpus", "export", "--since", base, "--out", "exports/corpus")
    data = payload["data"]
    assert payload["ok"] is True
    assert data["status"] == "exported"
    assert data["manifest_path"] == "exports/corpus/manifest.json"
    assert (tmp_path / "exports/corpus/rfcs.jsonl").exists()
    manifest = json.loads((tmp_path / "exports/corpus/manifest.json").read_text(encoding="utf-8"))
    assert "repo_local_schema_version" not in manifest
    assert manifest["state_authority"]["substrate"] == "legacy_sqlite_fixture"
    assert manifest["bundle_sha256"] == data["bundle_sha256"]


def test_corpus_export_invalid_since_returns_json_error_code_8(tmp_path: Path) -> None:
    _git_repo(tmp_path)
    _legacy_init_repo(tmp_path)
    payload = _run(tmp_path, "corpus", "export", "--since", "missing-ref", "--out", "exports", check=False)
    assert payload["ok"] is False
    assert payload["returncode"] == 8
    assert payload["error"]["code"] == 8


def test_corpus_export_rejects_bad_output_targets(tmp_path: Path) -> None:
    base = _git_repo(tmp_path)
    _legacy_init_repo(tmp_path)
    outside = tmp_path.parent / "outside-corpus"
    for out in [".striatum/corpus", str(outside)]:
        payload = _run(tmp_path, "corpus", "export", "--since", base, "--out", out, check=False)
        assert payload["ok"] is False
        assert payload["error"]["code"] == 8

    (tmp_path / "not-a-dir").write_text("x", encoding="utf-8")
    payload = _run(tmp_path, "corpus", "export", "--since", base, "--out", "not-a-dir", check=False)
    assert payload["returncode"] == 8


def test_no_engram_imports_or_memory_capabilities_in_striatum() -> None:
    paths = [
        ROOT / "src/striatum/corpus",
        ROOT / "src/striatum/cli",
        ROOT / "src/striatum/daemon_rpc",
        ROOT / "src/striatum/daemon_pg",
        ROOT / "src/striatum/mcp.py",
        ROOT / "src/striatum/service.py",
    ]
    bodies = "\n".join(
        path.read_text(encoding="utf-8") if path.is_file() else "\n".join(p.read_text(encoding="utf-8") for p in path.rglob("*.py"))
        for path in paths
    )
    assert "import engram" not in bodies
    assert "from engram" not in bodies
    assert "memory." not in bodies
    assert "engram" not in (ROOT / "pyproject.toml").read_text(encoding="utf-8").lower()
