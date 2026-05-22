from __future__ import annotations

import ast
from dataclasses import dataclass
from pathlib import Path



ROOT = Path(__file__).resolve().parents[2]


TRACK2_FIRST_BATCH_TESTS = frozenset(
    {
        Path("tests/test_artifact_schemas.py"),
        Path("tests/test_cli_mvp.py"),
        Path("tests/test_process_adapter.py"),
        Path("tests/test_service.py"),
    }
)


RESIDUAL_LEGACY_SQLITE_TEST_IMPORTS: dict[Path, set[str]] = {
    Path("tests/daemon_pg/handlers/recovery_evidence/conftest.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/fixtures/v1_repo_local_sqlite/build_fixture.py"): {
        "from striatum.legacy_sqlite.migrations import apply_migrations"
    },
    Path("tests/test_chat_tools.py"): {"from striatum.legacy_sqlite.db import init_repo"},
    Path("tests/test_cli_corpus_export.py"): {
        "from striatum.legacy_sqlite.db import init_repo"
    },
    Path("tests/test_cli_run_cancel.py"): {"from striatum.legacy_sqlite.db import connect"},
    Path("tests/test_corpus_export_integration.py"): {
        "from striatum.legacy_sqlite.db import init_repo"
    },
    Path("tests/test_daemon_rpc.py"): {
        "from striatum.legacy_sqlite.db import connect, init_repo"
    },
    Path("tests/test_dashboard.py"): {
        "from striatum.legacy_sqlite.db import connect, ensure_initialized"
    },
    Path("tests/test_dashboard_web_parity.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_decision_propagation.py"): {
        "from striatum.legacy_sqlite.migrations import apply_migrations"
    },
    Path("tests/test_dogfood_publish_on_behalf.py"): {
        "from striatum.legacy_sqlite.db import connect",
        "from striatum.legacy_sqlite.db import init_repo",
    },
    Path("tests/test_dogfood_surgical_recovery.py"): {
        "from striatum.legacy_sqlite.db import connect, expire_leases, transaction",
        "from striatum.legacy_sqlite.db import init_repo",
    },
    Path("tests/test_gh14_terminal_blocker_recovery.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_gh8_v16_rebuild_idempotent.py"): {
        "from striatum.legacy_sqlite.migrations import apply_migrations"
    },
    Path("tests/test_harness_profiles.py"): {
        "from striatum.legacy_sqlite.db import init_repo"
    },
    Path("tests/test_harness_v2_fixes.py"): {
        "from striatum.legacy_sqlite.db import connect",
        "from striatum.legacy_sqlite.db import init_repo",
    },
    Path("tests/test_job_detail_expected_artifacts.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_lane_evidence_guard.py"): {
        "from striatum.legacy_sqlite.migrations import apply_migrations"
    },
    Path("tests/test_list_commands.py"): {"from striatum.legacy_sqlite.db import init_repo"},
    Path("tests/test_pause_resume.py"): {"from striatum.legacy_sqlite.db import connect"},
    Path("tests/test_posture_verdicts_override_provenance.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_recovery_dry_run_no_side_effects.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_recovery_extended.py"): {"from striatum.legacy_sqlite.db import connect"},
    Path("tests/test_recovery_panel_dry_run.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_recovery_resume.py"): {"from striatum.legacy_sqlite.db import connect"},
    Path("tests/test_retry_job.py"): {"from striatum.legacy_sqlite.db import connect"},
    Path("tests/test_review_postures_introspection.py"): {
        "from striatum.legacy_sqlite.db import connect",
        "from striatum.legacy_sqlite.migrations import apply_migrations",
    },
    Path("tests/test_reviewer_policy.py"): {"from striatum.legacy_sqlite.db import connect"},
    Path("tests/test_run_detail_recovery_panel.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_session_close.py"): {
        "from striatum.legacy_sqlite.db import connect",
        "from striatum.legacy_sqlite.db import init_repo",
    },
    Path("tests/test_skills_install.py"): {
        "from striatum.legacy_sqlite.db import connect, init_repo"
    },
    Path("tests/test_supervise.py"): {
        "from striatum.legacy_sqlite.db import connect",
        "from striatum.legacy_sqlite.db import init_repo",
    },
    Path("tests/test_supervised_progress_watcher.py"): {
        "from striatum.legacy_sqlite.db import connect",
        "from striatum.legacy_sqlite.db import init_repo",
    },
    Path("tests/test_web_run_cancel.py"): {"from striatum.legacy_sqlite.db import connect"},
    Path("tests/test_web_run_pause_resume.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_web_run_posture_verdicts.py"): {
        "from striatum.legacy_sqlite.db import connect"
    },
    Path("tests/test_web_ui.py"): {"from striatum.legacy_sqlite.db import init_repo"},
    Path("tests/test_worktree_isolation.py"): {
        "from striatum.legacy_sqlite.db import connect",
        "from striatum.legacy_sqlite.db import init_repo",
    },
}


RESIDUAL_SQLITE_REFERENCE_TESTS: dict[Path, set[str]] = {
    Path("tests/daemon_pg/handlers/recovery_evidence/conftest.py"): {
        "import sqlite3",
        "usage sqlite3",
    },
    Path("tests/fixtures/v1_repo_local_sqlite/build_fixture.py"): {
        "import sqlite3",
        "usage sqlite3",
    },
    Path("tests/test_byline_regression.py"): {"import sqlite3", "usage sqlite3"},
    Path("tests/test_decision_propagation.py"): {"import sqlite3", "usage sqlite3"},
    Path("tests/test_gh8_v16_rebuild_idempotent.py"): {"import sqlite3", "usage sqlite3"},
    Path("tests/test_lane_evidence_guard.py"): {"import sqlite3", "usage sqlite3"},
    Path("tests/test_override_rationale_regression.py"): {
        "import sqlite3",
        "usage sqlite3",
    },
    Path("tests/test_service.py"): {"import sqlite3", "usage sqlite3"},
}


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


PRODUCTION_SQLITE_QUARANTINE: dict[Path, SQLiteClassification] = {}


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


































def test_legacy_python_daemon_module_is_deleted() -> None:
    assert not (ROOT / "src" / "striatum" / "daemon.py").exists()
    assert DAEMON_CONNECT_REGISTRY_CALLERS == {}


def test_legacy_sqlite_package_is_deleted_from_production_sources() -> None:
    assert not (ROOT / "src" / "striatum" / "legacy_sqlite").exists()


def test_production_sources_do_not_import_legacy_python_daemon() -> None:
    offenders = _legacy_daemon_imports_under(ROOT / "src" / "striatum")

    assert offenders == set()


def test_production_sources_do_not_import_legacy_sqlite_package() -> None:
    offenders = _legacy_sqlite_imports_under(ROOT / "src" / "striatum")

    assert offenders == {}




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




def test_test_sqlite_references_are_classified_as_test_fixtures() -> None:
    offenders = _sqlite_references_under(ROOT / "tests")

    assert offenders
    assert offenders == RESIDUAL_SQLITE_REFERENCE_TESTS


def test_first_track2_batch_has_no_legacy_state_imports() -> None:
    offenders = {
        path: imports
        for path, imports in _legacy_state_imports_under(ROOT / "tests").items()
        if path in TRACK2_FIRST_BATCH_TESTS
    }

    assert offenders == {}


def test_first_track2_batch_has_no_broad_module_level_legacy_skip() -> None:
    offenders = {
        path: reason
        for path in TRACK2_FIRST_BATCH_TESTS
        if (reason := _module_level_skip_reason(ROOT / path)) is not None
    }

    assert offenders == {}


def test_residual_legacy_sqlite_test_imports_are_explicit_future_batches() -> None:
    offenders = _legacy_state_imports_under(ROOT / "tests")
    offenders.pop(Path("tests/architecture/test_legacy_sqlite_quarantine.py"), None)

    assert offenders == RESIDUAL_LEGACY_SQLITE_TEST_IMPORTS


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


def _legacy_state_imports_under(root: Path) -> dict[Path, set[str]]:
    offenders: dict[Path, set[str]] = {}
    legacy_modules = {
        "striatum.db",
        "striatum.legacy_sqlite",
        "striatum.migrations",
    }
    facade_names = {"db", "migrations"}
    for path in sorted(root.rglob("*.py")):
        rel = path.relative_to(ROOT)
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    if any(
                        alias.name == module or alias.name.startswith(f"{module}.")
                        for module in legacy_modules
                    ):
                        offenders.setdefault(rel, set()).add(f"import {alias.name}")
            elif isinstance(node, ast.ImportFrom):
                if node.module and any(
                    node.module == module or node.module.startswith(f"{module}.")
                    for module in legacy_modules
                ):
                    imported = ", ".join(alias.name for alias in node.names)
                    offenders.setdefault(rel, set()).add(
                        f"from {node.module} import {imported}"
                    )
                elif node.module == "striatum":
                    imported_names = [
                        alias.name for alias in node.names if alias.name in facade_names
                    ]
                    if imported_names:
                        offenders.setdefault(rel, set()).add(
                            "from striatum import " + ", ".join(imported_names)
                        )
    return offenders


def _module_level_skip_reason(path: Path) -> str | None:
    tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
    for node in tree.body:
        if not isinstance(node, ast.Expr) or not isinstance(node.value, ast.Call):
            continue
        call = node.value
        if _dotted_name(call.func) != "pytest.skip":
            continue
        if not any(
            keyword.arg == "allow_module_level" and isinstance(keyword.value, ast.Constant)
            and keyword.value.value is True
            for keyword in call.keywords
        ):
            continue
        if call.args and isinstance(call.args[0], ast.Constant):
            return str(call.args[0].value)
        return "<unknown>"
    return None


def _legacy_sqlite_imports_under(root: Path) -> dict[Path, set[str]]:
    offenders: dict[Path, set[str]] = {}
    for path in sorted(root.rglob("*.py")):
        rel = path.relative_to(ROOT)
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    if alias.name == "striatum.legacy_sqlite" or alias.name.startswith(
                        "striatum.legacy_sqlite."
                    ):
                        offenders.setdefault(rel, set()).add(f"import {alias.name}")
            elif isinstance(node, ast.ImportFrom):
                if node.module and (
                    node.module == "striatum.legacy_sqlite"
                    or node.module.startswith("striatum.legacy_sqlite.")
                ):
                    imported = ", ".join(alias.name for alias in node.names)
                    offenders.setdefault(rel, set()).add(
                        f"from {node.module} import {imported}"
                    )
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
