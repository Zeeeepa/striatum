from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


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


def _data(payload: dict[str, Any]) -> dict[str, Any]:
    return cast(dict[str, Any], payload["data"])


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


def test_corpus_export_replays_with_stable_jsonl_hashes(tmp_path: Path) -> None:
    base = _git_repo(tmp_path)
    _legacy_init_repo(tmp_path)
    prepared = _data(_run(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])
    _run(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/test")
    _run(tmp_path, "run", "start", "--run-id", run_id)
    _run(tmp_path, "run", "summary", "--run-id", run_id, "--path", "docs/dogfood/999/RUN_SUMMARY.md")

    _run(tmp_path, "corpus", "export", "--since", base, "--out", "exports/one")
    _run(tmp_path, "corpus", "export", "--since", base, "--out", "exports/two")

    jsonl_names = sorted(path.name for path in (tmp_path / "exports/one").glob("*.jsonl"))
    assert jsonl_names
    for name in jsonl_names:
        assert (tmp_path / "exports/one" / name).read_bytes() == (tmp_path / "exports/two" / name).read_bytes()

    first_manifest = json.loads((tmp_path / "exports/one/manifest.json").read_text(encoding="utf-8"))
    second_manifest = json.loads((tmp_path / "exports/two/manifest.json").read_text(encoding="utf-8"))
    first_manifest.pop("generated_at", None)
    second_manifest.pop("generated_at", None)
    first_manifest["repo_root"] = "<repo>"
    second_manifest["repo_root"] = "<repo>"
    assert first_manifest == second_manifest
    assert first_manifest["row_counts"]["run_summary"] >= 1
