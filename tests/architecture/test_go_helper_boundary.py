from __future__ import annotations

import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]

FORBIDDEN_IMPORTS = (
    "github.com/halbritt/striatum/go/pkg/db",
    "github.com/halbritt/striatum/go/pkg/rpc",
    "github.com/halbritt/striatum/go/pkg/mutations",
    "github.com/halbritt/striatum/go/pkg/reads",
    "github.com/halbritt/striatum/go/pkg/apply",
    "github.com/halbritt/striatum/go/pkg/crossrepo",
)


def test_go_supervisor_helper_transitive_deps_stay_out_of_domain_authority_packages() -> None:
    deps = _go_helper_deps()
    forbidden = [
        imported
        for imported in deps
        if any(
            imported == prefix or imported.startswith(f"{prefix}/")
            for prefix in FORBIDDEN_IMPORTS
        )
    ]

    assert forbidden == []


def _go_helper_deps() -> list[str]:
    go = shutil.which("go")
    if go is None:
        pytest.skip("go toolchain unavailable; cannot inspect helper transitive deps")
    result = subprocess.run(
        [go, "list", "-deps", "./cmd/striatum-supervisor-helper"],
        cwd=ROOT / "go",
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]
