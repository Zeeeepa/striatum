from __future__ import annotations

import ast
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

import pytest

from striatum import daemon

ROOT = Path(__file__).resolve().parents[2]
CUTOVER_COMPLETED_KEY = "pg_cutover_completed_at"


@dataclass(frozen=True)
class SQLiteClassification:
    category: str
    reason: str


ALLOWED_CATEGORIES = frozenset(
    {
        "migration",
        "service transition",
        "adapter transition",
        "dogfood fixture",
        "bootstrap/admin",
        "test fixture",
    }
)


PRODUCTION_SQLITE_QUARANTINE = {
    # The legacy repo-local SQLite substrate itself and the one-way
    # migration paths that still have to read historical stores.
    Path("src/striatum/db.py"): SQLiteClassification(
        "migration",
        "legacy repo-local SQLite engine retained for migration and test fixtures",
    ),
    Path("src/striatum/migrations.py"): SQLiteClassification(
        "migration",
        "pre-D094 repo-local schema migrations retained for migration fixtures",
    ),
    Path("src/striatum/daemon_pg/repo_local_migration.py"): SQLiteClassification(
        "migration",
        "pre-D094 repo-local SQLite to daemon Postgres migration command",
    ),
    # CLI/API/read DTO surfaces that still carry SQLite compatibility while
    # the local service and corpus/export paths finish their daemon DTO move.
    Path("src/striatum/cli/dispatch.py"): SQLiteClassification(
        "service transition",
        "daemon-first CLI with test-harness legacy fallback dispatch",
    ),
    Path("src/striatum/cli/evidence.py"): SQLiteClassification(
        "service transition",
        "legacy evidence-export reader pending daemon DTO replacement",
    ),
    Path("src/striatum/cli/introspect.py"): SQLiteClassification(
        "service transition",
        "legacy status/why/doctor readers pending daemon DTO replacement",
    ),
    Path("src/striatum/cli/list_commands.py"): SQLiteClassification(
        "service transition",
        "legacy list readers pending daemon DTO replacement",
    ),
    Path("src/striatum/cli/run_summary.py"): SQLiteClassification(
        "service transition",
        "legacy run-summary reader pending daemon DTO replacement",
    ),
    Path("src/striatum/recovery/auto.py"): SQLiteClassification(
        "service transition",
        "legacy recovery sweep retained for fixture and service transition",
    ),
    Path("src/striatum/legacy_sqlite/service.py"): SQLiteClassification(
        "service transition",
        "gated subprocess-fixture web fallbacks and legacy page reads isolated from primary service code",
    ),
    # Adapter, supervisor, artifact, byline, and workflow helpers whose
    # production authority has moved to daemon/Postgres but whose legacy
    # functions still support adapters and test harnesses.
    Path("src/striatum/artifacts.py"): SQLiteClassification(
        "adapter transition",
        "legacy artifact publisher used by adapter/test-harness paths",
    ),
    Path("src/striatum/cli/mutations.py"): SQLiteClassification(
        "adapter transition",
        "legacy workflow-loop mutations retained for adapter/test fixtures",
    ),
    Path("src/striatum/cli/recovery.py"): SQLiteClassification(
        "adapter transition",
        "legacy recovery mutations retained for adapter/test fixtures",
    ),
    Path("src/striatum/cli/worktree.py"): SQLiteClassification(
        "adapter transition",
        "legacy worktree helpers retained for adapter/test fixtures",
    ),
    Path("src/striatum/process_adapter.py"): SQLiteClassification(
        "adapter transition",
        "legacy process adapter table helpers retained during supervisor transition",
    ),
    Path("src/striatum/process_completion.py"): SQLiteClassification(
        "adapter transition",
        "legacy process-completion reconciliation retained during supervisor transition",
    ),
    Path("src/striatum/supervisor.py"): SQLiteClassification(
        "adapter transition",
        "legacy supervised wrapper helper retained during supervisor transition",
    ),
    Path("src/striatum/workflow.py"): SQLiteClassification(
        "adapter transition",
        "legacy run prepare and workflow event helpers retained for fixtures",
    ),
    Path("src/striatum/dogfood/operator_tools.py"): SQLiteClassification(
        "dogfood fixture",
        "operator dogfood recovery tools are compatibility fixtures",
    ),
    # Bootstrap/admin surfaces may inspect or initialize legacy files while
    # guiding an operator into the daemon/Postgres runtime.
    Path("src/striatum/daemon.py"): SQLiteClassification(
        "bootstrap/admin",
        "legacy RFC 0028 daemon registry and bootstrap helpers",
    ),
}


TEST_SQLITE_QUARANTINE_PREFIXES = {
    Path("tests"): SQLiteClassification(
        "test fixture",
        "tests may build or inspect legacy SQLite fixtures",
    ),
}


DAEMON_RPC_DB_IMPORT_ALLOWLIST: dict[Path, set[str]] = {}


NEUTRAL_DB_REEXPORTS = frozenset(
    {
        "ADAPTER_ENFORCEMENT_LEVELS",
        "DB_NAME",
        "JsonObject",
        "STATE_DIR",
        "WORKTREES_SUBDIR",
        "adapter_constraint_enforcement",
        "adapter_enforcement_satisfies",
        "db_path",
        "json_dumps",
        "json_loads",
        "lane_worktree_isolation",
        "new_id",
        "path_allowed",
        "repo_relative_path",
        "sha256_bytes",
        "state_dir",
        "utc_now",
    }
)


DAEMON_CONNECT_REGISTRY_CALLERS: dict[str, str] = {}


def test_daemon_pg_does_not_import_legacy_sqlite_db_module() -> None:
    offenders = _db_imports_under(ROOT / "src" / "striatum" / "daemon_pg")

    assert offenders == {}


def test_daemon_rpc_db_imports_are_explicitly_quarantined() -> None:
    offenders = _db_imports_under(ROOT / "src" / "striatum" / "daemon_rpc")
    allowed = {
        path: names
        for path, names in offenders.items()
        if path in DAEMON_RPC_DB_IMPORT_ALLOWLIST
        and names <= DAEMON_RPC_DB_IMPORT_ALLOWLIST[path]
    }

    assert offenders == allowed


def test_neutral_helpers_are_not_imported_through_legacy_db_module() -> None:
    offenders = {
        path: names & NEUTRAL_DB_REEXPORTS
        for path, names in _db_imports_under(ROOT / "src" / "striatum").items()
        if names & NEUTRAL_DB_REEXPORTS
    }

    assert offenders == {}


def test_production_sqlite_references_are_quarantined_by_category() -> None:
    offenders = _sqlite_references_under(ROOT / "src")
    classified_paths = set(PRODUCTION_SQLITE_QUARANTINE)

    assert not (set(offenders) - classified_paths), _format_unclassified(
        offenders,
        set(offenders) - classified_paths,
    )
    assert not (classified_paths - set(offenders)), (
        "stale SQLite quarantine entries should be removed: "
        + ", ".join(str(path) for path in sorted(classified_paths - set(offenders)))
    )
    assert all(
        classification.category in ALLOWED_CATEGORIES
        and classification.reason
        for classification in PRODUCTION_SQLITE_QUARANTINE.values()
    )


def test_service_primary_module_no_longer_opens_legacy_sqlite() -> None:
    offenders = _sqlite_references_under(ROOT / "src" / "striatum")

    assert Path("src/striatum/service.py") not in offenders


def test_daemon_connect_registry_callers_are_explicitly_classified() -> None:
    callers = _direct_callers(ROOT / "src" / "striatum" / "daemon.py", "connect_registry")

    assert callers == set(DAEMON_CONNECT_REGISTRY_CALLERS)
    assert all(reason for reason in DAEMON_CONNECT_REGISTRY_CALLERS.values())


def test_production_sources_do_not_import_legacy_python_daemon() -> None:
    offenders = _legacy_daemon_imports_under(ROOT / "src" / "striatum")

    assert offenders == set()


def test_legacy_service_owns_page_read_payload_fallbacks() -> None:
    service_source = (ROOT / "src" / "striatum" / "service.py").read_text(encoding="utf-8")
    root_compat_path = ROOT / "src" / "striatum" / "service_legacy.py"
    legacy_source = (
        ROOT / "src" / "striatum" / "legacy_sqlite" / "service.py"
    ).read_text(encoding="utf-8")

    assert not root_compat_path.exists()
    service_tree = ast.parse(service_source)
    top_level_legacy_imports = [
        node
        for node in service_tree.body
        if _imports_module(node, "striatum.legacy_sqlite")
    ]
    assert top_level_legacy_imports == []
    assert "def _legacy_service(" in service_source
    assert "striatum.service_legacy" not in service_source
    assert "_legacy_shape_artifact_rows" not in service_source
    assert '_LazyLegacyCallable("_byline_line")' not in service_source
    assert '_LazyLegacyCallable("legacy_shape_artifact_rows")' not in service_source
    assert '_LazyLegacyCallable("legacy_view_file_run_breadcrumb")' not in service_source

    page_payload_builders = {
        "legacy_run_detail_payload",
        "legacy_job_detail_payload",
        "legacy_run_posture_verdicts_payload",
        "legacy_artifact_view_payload",
    }

    for name in page_payload_builders:
        assert f"def {name}(" in legacy_source
        assert f"def _{name}(" not in service_source
        assert f"def {name}(" not in service_source


def test_primary_service_lazy_loads_legacy_api_wrapper() -> None:
    service_source = (ROOT / "src" / "striatum" / "service.py").read_text(encoding="utf-8")
    service_tree = ast.parse(service_source)
    top_level_api_imports = [
        node
        for node in service_tree.body
        if _imports_module(node, "striatum.api") or _imports_from_striatum(node, "api")
    ]

    assert top_level_api_imports == []

    invoke = next(
        node
        for node in service_tree.body
        if isinstance(node, ast.FunctionDef) and node.name == "invoke"
    )
    assert any(_imports_from_striatum(node, "api") for node in ast.walk(invoke))


@pytest.mark.parametrize(
    "call",
    [
        lambda: daemon.run_daemon_foreground(max_sweeps=1),
        lambda: daemon.daemon_status(),
        lambda: daemon.daemon_stop(),
        lambda: daemon.daemon_sweep_once(),
        lambda: daemon.health(),
        lambda: daemon.daemon_audit(),
        lambda: daemon.read_doctor(repo=None, verbose=True),
    ],
)
def test_production_daemon_global_surfaces_refuse_without_postgres_url(
    call: Callable[[], object],
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(registry))
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    monkeypatch.setenv("STRIATUM_DAEMON_DB_URL", "")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "1")
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")

    with pytest.raises(daemon.DaemonRegistryError, match="daemon PostgreSQL URL is not configured"):
        call()

    assert not registry.exists()


def test_legacy_sqlite_registry_ignores_obsolete_standalone_escape(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(registry))
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "1")
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.setenv("STRIATUM_ALLOW_LEGACY_SQLITE_REGISTRY", "1")

    with pytest.raises(daemon.DaemonRegistryError, match="legacy SQLite daemon registry is disabled"):
        daemon.connect_registry()

    assert not registry.exists()


def test_legacy_sqlite_registry_allows_test_harness_pair_only(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(registry))
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    conn = daemon.connect_registry()
    conn.close()

    assert registry.exists()


def test_legacy_sqlite_registry_refuses_after_pg_cutover_marker(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(registry))
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")

    conn = daemon.connect_registry()
    with daemon.registry_transaction(conn):
        conn.execute(
            "INSERT INTO daemon_meta(key, value) VALUES(?, ?)",
            (CUTOVER_COMPLETED_KEY, "2026-05-11T00:00:00Z"),
        )
    conn.close()

    with pytest.raises(daemon.DaemonRegistryError, match="cut over to PostgreSQL"):
        daemon.connect_registry()


def test_test_sqlite_references_are_classified_as_test_fixtures() -> None:
    offenders = _sqlite_references_under(ROOT / "tests")
    unclassified = {
        path
        for path in offenders
        if not any(path.is_relative_to(prefix) for prefix in TEST_SQLITE_QUARANTINE_PREFIXES)
    }

    assert offenders
    assert not unclassified
    assert all(
        classification.category == "test fixture" and classification.reason
        for classification in TEST_SQLITE_QUARANTINE_PREFIXES.values()
    )


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


def _sqlite_references_under(root: Path) -> dict[Path, set[str]]:
    offenders: dict[Path, set[str]] = {}
    for path in sorted(root.rglob("*.py")):
        rel = path.relative_to(ROOT)
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    if alias.name in {"sqlite3", "striatum.db"}:
                        offenders.setdefault(rel, set()).add(f"import {alias.name}")
            elif isinstance(node, ast.ImportFrom):
                if node.module in {"sqlite3", "striatum.db"}:
                    imported = ", ".join(alias.name for alias in node.names)
                    offenders.setdefault(rel, set()).add(
                        f"from {node.module} import {imported}"
                    )
                elif node.module == "striatum":
                    imported_db = [alias.name for alias in node.names if alias.name == "db"]
                    if imported_db:
                        offenders.setdefault(rel, set()).add("from striatum import db")
            else:
                dotted = _dotted_name(node)
                if dotted in {"sqlite3", "striatum.db"}:
                    offenders.setdefault(rel, set()).add(f"usage {dotted}")
    return offenders


def _imports_module(node: ast.AST, module: str) -> bool:
    if isinstance(node, ast.Import):
        return any(alias.name == module or alias.name.startswith(f"{module}.") for alias in node.names)
    if isinstance(node, ast.ImportFrom):
        return node.module == module or bool(node.module and node.module.startswith(f"{module}."))
    return False


def _imports_from_striatum(node: ast.AST, name: str) -> bool:
    return (
        isinstance(node, ast.ImportFrom)
        and node.module == "striatum"
        and any(alias.name == name for alias in node.names)
    )


def _legacy_daemon_imports_under(root: Path) -> set[Path]:
    offenders: set[Path] = set()
    for path in sorted(root.rglob("*.py")):
        rel = path.relative_to(ROOT)
        if rel == Path("src/striatum/daemon.py"):
            continue
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                if any(alias.name == "striatum.daemon" for alias in node.names):
                    offenders.add(rel)
            elif isinstance(node, ast.ImportFrom):
                if node.module == "striatum" and any(alias.name == "daemon" for alias in node.names):
                    offenders.add(rel)
                if node.module == "striatum.daemon":
                    offenders.add(rel)
    return offenders


def _dotted_name(node: ast.AST) -> str | None:
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        parent = _dotted_name(node.value)
        return f"{parent}.{node.attr}" if parent else node.attr
    return None


def _direct_callers(path: Path, called_name: str) -> set[str]:
    tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
    callers: set[str] = set()

    class Visitor(ast.NodeVisitor):
        def __init__(self) -> None:
            self.stack: list[str] = []

        def visit_FunctionDef(self, node: ast.FunctionDef) -> None:  # noqa: N802 - ast visitor API
            self.stack.append(node.name)
            try:
                self.generic_visit(node)
            finally:
                self.stack.pop()

        def visit_Call(self, node: ast.Call) -> None:  # noqa: N802 - ast visitor API
            if _dotted_name(node.func) == called_name and self.stack:
                callers.add(self.stack[-1])
            self.generic_visit(node)

    Visitor().visit(tree)
    return callers


def _format_unclassified(
    offenders: dict[Path, set[str]],
    paths: set[Path],
) -> str:
    lines = ["unclassified production SQLite references:"]
    for path in sorted(paths):
        signals = ", ".join(sorted(offenders[path]))
        lines.append(f"- {path}: {signals}")
    return "\n".join(lines)
