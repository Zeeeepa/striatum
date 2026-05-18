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


def test_make_targets_check_wheel_size_before_package_smoke() -> None:
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")

    assert "package-wheel-size:" in makefile
    assert "scripts/check_wheel_size.py" in makefile
    assert "metadata-check package-wheel-size package-smoke" in makefile


def test_smoke_scripts_do_not_fall_back_to_repo_local_sqlite() -> None:
    forbidden = [
        "STRIATUM_DAEMON_REQUIRED=0",
        "STRIATUM_TEST_HARNESS=1",
        "using legacy test-harness fixture path",
        'test -f "$TARGET/.striatum/state.sqlite3"',
    ]

    for script_name in ("fresh_clone_smoke.sh", "package_smoke.sh"):
        script = (ROOT / "scripts" / script_name).read_text(encoding="utf-8")
        for needle in forbidden:
            assert needle not in script
        assert "PostgreSQL is required for daemon-owned Striatum state" in script
    package_smoke = (ROOT / "scripts" / "package_smoke.sh").read_text(
        encoding="utf-8"
    )
    assert 'GIT_SHA="$SOURCE_GIT_SHA" GIT_DIRTY="$SOURCE_GIT_DIRTY"' in package_smoke
    assert "daemon_version=$PACKAGE_VERSION" in package_smoke
    assert "git_sha=$SOURCE_GIT_SHA" in package_smoke


def test_packaged_daemon_rpc_contract_matches_root_contract() -> None:
    root_contract = ROOT / "contracts" / "daemon_methods.json"
    packaged_contract = ROOT / "src/striatum/daemon_rpc/daemon_methods.json"
    pyproject = (ROOT / "pyproject.toml").read_text(encoding="utf-8")

    assert packaged_contract.read_text(encoding="utf-8") == root_contract.read_text(
        encoding="utf-8"
    )
    assert '"striatum.daemon_rpc" = ["daemon_methods.json"]' in pyproject


def test_go_daemon_build_stamps_release_provenance() -> None:
    makefile = (ROOT / "go" / "Makefile").read_text(encoding="utf-8")

    assert "PACKAGE_VERSION ?=" in makefile
    assert "GIT_SHA ?=" in makefile
    assert "-X main.daemonVersion=$(PACKAGE_VERSION)" in makefile
    assert "-X main.buildGitSHA=$(GIT_SHA)" in makefile
    assert "-ldflags \"$(GO_DAEMON_LDFLAGS)\"" in makefile


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


def test_wheel_size_script_accepts_file_or_directory_and_refuses_drift(
    tmp_path: Path,
) -> None:
    dist = tmp_path / "dist"
    dist.mkdir()
    wheel = dist / "striatum-0.0.0-py3-none-any.whl"
    wheel.write_bytes(b"x" * 16)
    script = ROOT / "scripts/check_wheel_size.py"

    ok_file = subprocess.run(
        [sys.executable, str(script), "--wheel", str(wheel), "--max-bytes", "16"],
        check=False,
        text=True,
        capture_output=True,
    )
    assert ok_file.returncode == 0

    ok_dir = subprocess.run(
        [sys.executable, str(script), "--wheel", str(dist), "--max-bytes", "16"],
        check=False,
        text=True,
        capture_output=True,
    )
    assert ok_dir.returncode == 0

    too_large = subprocess.run(
        [sys.executable, str(script), "--wheel", str(dist), "--max-bytes", "15"],
        check=False,
        text=True,
        capture_output=True,
    )
    assert too_large.returncode == 1
    assert "exceeds limit" in too_large.stderr
