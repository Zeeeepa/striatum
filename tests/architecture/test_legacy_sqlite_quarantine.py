from __future__ import annotations

import ast
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

ALLOWLIST = {
    Path("src/striatum/daemon_rpc/server.py"): {"connect"},
}


def test_daemon_pg_does_not_import_legacy_sqlite_db_module() -> None:
    offenders = _db_imports_under(ROOT / "src" / "striatum" / "daemon_pg")

    assert offenders == {}


def test_daemon_rpc_db_imports_are_explicitly_quarantined() -> None:
    offenders = _db_imports_under(ROOT / "src" / "striatum" / "daemon_rpc")
    allowed = {path: names for path, names in offenders.items() if path in ALLOWLIST and names <= ALLOWLIST[path]}

    assert offenders == allowed


def _db_imports_under(root: Path) -> dict[Path, set[str]]:
    offenders: dict[Path, set[str]] = {}
    for path in sorted(root.rglob("*.py")):
        rel = path.relative_to(ROOT)
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, ast.ImportFrom) and node.module == "striatum.db":
                offenders.setdefault(rel, set()).update(alias.name for alias in node.names)
            if isinstance(node, ast.Import):
                for alias in node.names:
                    if alias.name == "striatum.db":
                        offenders.setdefault(rel, set()).add(alias.name)
    return offenders
