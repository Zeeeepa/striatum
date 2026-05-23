from __future__ import annotations

import json
import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
CONTRACT_PATH = REPO_ROOT / "contracts" / "daemon_methods.json"
LEDGER_PATH = REPO_ROOT / "docs" / "architecture" / "CLI_RETIREMENT_PARITY.md"

ALLOWED_CLASSIFICATIONS = {
    "bootstrap",
    "replaced_by_mcp",
    "replaced_by_mcp_ui",
    "temporary_compatibility",
}
ALLOWED_MCP_PARITY = {"n/a", "mcp_exact", "mcp_registry"}
ALLOWED_UI_PARITY = {
    "n/a",
    "ui_exact",
    "ui_missing",
    "ui_not_product",
    "ui_route_unchecked",
}
NON_RETIREMENT_GATES = {
    "keep_bootstrap_cli",
    "blocked_active_ui_test_gap",
    "blocked_docs_skill_cutover",
    "blocked_mcp_method_test_gap",
    "blocked_retirement_test_gap",
    "blocked_rfc0075_ui_gap",
    "blocked_ui_gap",
}


def _contract() -> dict[str, object]:
    return json.loads(CONTRACT_PATH.read_text(encoding="utf-8"))


def _non_read_cli_routes() -> dict[str, tuple[str, str]]:
    raw = _contract()
    methods = {
        str(entry["method"]): str(entry["required_capability"])
        for entry in raw["methods"]  # type: ignore[index]
        if isinstance(entry, dict)
    }
    routes: dict[str, tuple[str, str]] = {}
    for route in raw["cli_routes"]:  # type: ignore[index]
        if not isinstance(route, dict):
            continue
        method = str(route["method"])
        capability = methods[method]
        if capability == "read":
            continue
        command = str(route["command"])
        subcommand = route.get("subcommand")
        label = command if subcommand is None else f"{command} {subcommand}"
        routes[label] = (method, capability)
    return routes


def _ledger_rows() -> dict[str, dict[str, str]]:
    text = LEDGER_PATH.read_text(encoding="utf-8")
    section = text.split("## Checked CLI Control Ledger", 1)[1]
    section = section.split("## Immediate Guardrail", 1)[0]
    rows: dict[str, dict[str, str]] = {}
    headers: list[str] | None = None
    for line in section.splitlines():
        if not line.startswith("|"):
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) < 8:
            continue
        if cells[0] == "---" or set(cells[0]) == {"-"}:
            continue
        if cells[0] == "CLI command":
            headers = cells
            continue
        if headers is None:
            continue
        row = dict(zip(headers, cells, strict=True))
        command = _strip_code(row["CLI command"])
        row["CLI command"] = command
        row["RPC method"] = _strip_code(row["RPC method"])
        rows[command] = row
    return rows


def _strip_code(value: str) -> str:
    if value.startswith("`") and value.endswith("`"):
        return value[1:-1]
    return value


def test_cli_retirement_parity_ledger_covers_non_read_cli_routes() -> None:
    expected = _non_read_cli_routes()
    rows = _ledger_rows()

    assert set(rows) == set(expected)

    mismatches: list[str] = []
    for label, (method, capability) in sorted(expected.items()):
        row = rows[label]
        if row["RPC method"] != method:
            mismatches.append(f"{label} method {row['RPC method']!r} != {method!r}")
        if row["Capability"] != capability:
            mismatches.append(f"{label} capability {row['Capability']!r} != {capability!r}")
    assert not mismatches, "; ".join(mismatches)


def test_cli_retirement_parity_rows_use_checked_status_vocabularies() -> None:
    invalid: list[str] = []
    for label, row in sorted(_ledger_rows().items()):
        if row["Classification"] not in ALLOWED_CLASSIFICATIONS:
            invalid.append(f"{label} classification {row['Classification']!r}")
        if row["MCP parity"] not in ALLOWED_MCP_PARITY:
            invalid.append(f"{label} MCP parity {row['MCP parity']!r}")
        if row["UI parity"] not in ALLOWED_UI_PARITY:
            invalid.append(f"{label} UI parity {row['UI parity']!r}")
        if row["Retirement gate"] not in NON_RETIREMENT_GATES:
            invalid.append(f"{label} retirement gate {row['Retirement gate']!r}")

    assert not invalid, "; ".join(invalid)


def test_cli_retirement_parity_does_not_claim_unblocked_retirement() -> None:
    rows = _ledger_rows()

    retireable = [
        label
        for label, row in sorted(rows.items())
        if not row["Retirement gate"].startswith("blocked_")
        and row["Retirement gate"] != "keep_bootstrap_cli"
    ]
    assert not retireable

    covered_without_ui = [
        label
        for label, row in sorted(rows.items())
        if row["Classification"] == "replaced_by_mcp_ui" and row["UI parity"] != "ui_exact"
    ]
    assert not covered_without_ui


def test_cli_retirement_parity_active_ui_rows_cite_active_tests() -> None:
    missing: list[str] = []
    for label, row in sorted(_ledger_rows().items()):
        refs = _test_refs(row["Checked evidence"])
        if row["UI parity"] == "ui_exact" and not refs:
            missing.append(f"{label} has ui_exact without test evidence")
        for ref in refs:
            if not _test_ref_is_active(ref):
                missing.append(f"{label} cites inactive or missing test {ref}")
    assert not missing, "; ".join(missing)


def _test_refs(cell: str) -> list[str]:
    if cell == "none":
        return []
    return [ref for raw in cell.split(",") if (ref := raw.strip())]


def _test_ref_is_active(ref: str) -> bool:
    path_part, _, test_name = ref.partition("::")
    if not path_part or not test_name:
        return False
    path = REPO_ROOT / path_part
    if not path.is_file():
        return False
    text = path.read_text(encoding="utf-8")
    if path.suffix == ".go":
        return re.search(rf"^func {re.escape(test_name)}\(", text, flags=re.MULTILINE) is not None
    first_lines = "\n".join(text.splitlines()[:20])
    if "allow_module_level=True" in first_lines:
        return False
    return re.search(rf"^def {re.escape(test_name)}\(", text, flags=re.MULTILINE) is not None
