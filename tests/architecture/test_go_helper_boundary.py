from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

HELPER_FILES = (
    Path("go/cmd/striatum-supervisor-helper/main.go"),
    Path("go/pkg/supervisor/helper.go"),
    Path("go/pkg/supervisor/helper_protocol.go"),
)

FORBIDDEN_IMPORTS = (
    "github.com/halbritt/striatum/go/pkg/db",
    "github.com/halbritt/striatum/go/pkg/rpc",
    "github.com/halbritt/striatum/go/pkg/mutations",
    "github.com/halbritt/striatum/go/pkg/reads",
    "github.com/halbritt/striatum/go/pkg/apply",
    "github.com/halbritt/striatum/go/pkg/crossrepo",
)

IMPORT_RE = re.compile(r'"([^"]+)"')


def test_go_supervisor_helper_stays_out_of_domain_authority_packages() -> None:
    offenders: dict[Path, list[str]] = {}
    for rel_path in HELPER_FILES:
        imports = _go_imports(ROOT / rel_path)
        forbidden = [
            imported
            for imported in imports
            if any(
                imported == prefix or imported.startswith(f"{prefix}/")
                for prefix in FORBIDDEN_IMPORTS
            )
        ]
        if forbidden:
            offenders[rel_path] = forbidden

    assert offenders == {}


def _go_imports(path: Path) -> list[str]:
    imports: list[str] = []
    in_block = False
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.split("//", 1)[0].strip()
        if not line:
            continue
        if in_block:
            if line == ")":
                in_block = False
                continue
            imports.extend(IMPORT_RE.findall(line))
            continue
        if line == "import (":
            in_block = True
            continue
        if line.startswith("import "):
            imports.extend(IMPORT_RE.findall(line))
    return imports
