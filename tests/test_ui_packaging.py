from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def test_vite_react_plugin_is_build_only_dependency() -> None:
    package = json.loads(
        (ROOT / "src/striatum/web/frontend/package.json").read_text(encoding="utf-8")
    )

    assert "@vitejs/plugin-react" not in package["dependencies"]
    assert package["devDependencies"]["@vitejs/plugin-react"]


def test_ui_make_targets_clean_and_size_check_bundles() -> None:
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")

    assert "ui-clean:" in makefile
    assert "ui-build: ui-clean" in makefile
    assert "ui-bundle-size:" in makefile
    assert "ui-check-bundle: ui-build ui-verify-bundle ui-bundle-size" in makefile


def test_ui_bundle_size_script_accepts_override_and_refuses_drift(tmp_path: Path) -> None:
    build = tmp_path / "build"
    build.mkdir()
    (build / "asset.js").write_bytes(b"x" * 16)
    script = ROOT / "scripts/check_ui_bundle_size.py"

    ok = subprocess.run(
        [sys.executable, str(script), "--root", str(build), "--max-bytes", "16"],
        check=False,
        text=True,
        capture_output=True,
    )
    assert ok.returncode == 0

    too_large = subprocess.run(
        [sys.executable, str(script), "--root", str(build), "--max-bytes", "15"],
        check=False,
        text=True,
        capture_output=True,
    )
    assert too_large.returncode == 1
    assert "exceeds limit" in too_large.stderr
