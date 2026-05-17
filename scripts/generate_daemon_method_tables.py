#!/usr/bin/env python3
"""Generate daemon method reference tables from the method contract."""

from __future__ import annotations

import argparse
import difflib
import json
import pathlib
import sys
from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class MethodContract:
    method: str
    required_capability: str | None
    repository_scope_mode: str
    params_schema_version: int
    audit_class: str
    min_envelope: int
    deprecated: bool


@dataclass(frozen=True)
class CliRouteContract:
    command: str
    subcommand: str | None
    method: str
    params: str


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contract", default="contracts/daemon_methods.json")
    parser.add_argument("--out", default="docs/architecture/DAEMON_METHOD_TABLES.md")
    parser.add_argument(
        "--check",
        action="store_true",
        help="fail when the generated output differs from --out",
    )
    args = parser.parse_args()

    contract_path = pathlib.Path(args.contract)
    output_path = pathlib.Path(args.out)
    payload = load_contract_payload(contract_path)
    methods = load_contract(payload)
    cli_routes = load_cli_routes(payload, methods)
    rendered = render_markdown(
        methods,
        cli_routes=cli_routes,
        source=contract_path.as_posix(),
    )

    if args.check:
        try:
            current = output_path.read_text(encoding="utf-8")
        except FileNotFoundError:
            print(f"{output_path} does not exist", file=sys.stderr)
            return 1
        if current != rendered:
            diff = difflib.unified_diff(
                current.splitlines(keepends=True),
                rendered.splitlines(keepends=True),
                fromfile=str(output_path),
                tofile=f"{output_path} (generated)",
            )
            sys.stderr.writelines(diff)
            return 1
        return 0

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(rendered, encoding="utf-8")
    return 0


def load_contract_payload(path: pathlib.Path) -> dict[str, Any]:
    raw = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError("daemon method contract must be a JSON object")
    return raw


def load_contract(raw: dict[str, Any]) -> list[MethodContract]:
    raw_methods = raw.get("methods")
    if not isinstance(raw_methods, list):
        raise ValueError("daemon method contract must contain a 'methods' list")

    methods: list[MethodContract] = []
    seen: set[str] = set()
    for index, item in enumerate(raw_methods):
        if not isinstance(item, dict):
            raise ValueError(f"contract methods[{index}] must be an object")
        method = str(item.get("method", "")).strip()
        if not method:
            raise ValueError(f"contract methods[{index}] is missing method")
        if method in seen:
            raise ValueError(f"duplicate daemon RPC method in contract: {method}")
        seen.add(method)
        methods.append(
            MethodContract(
                method=method,
                required_capability=_capability(item.get("required_capability"), method),
                repository_scope_mode=_required_str(item, "repository_scope_mode", method),
                params_schema_version=_positive_int(item, "params_schema_version", method),
                audit_class=_required_str(item, "audit_class", method),
                min_envelope=_positive_int(item, "min_envelope", method),
                deprecated=_required_bool(item, "deprecated", method),
            )
        )
    return methods


def _capability(value: object, method: str) -> str | None:
    if value is None:
        return None
    if not isinstance(value, str) or not value:
        raise ValueError(f"{method}: required_capability must be a non-empty string or null")
    return value


def _required_str(item: dict[str, Any], field: str, method: str) -> str:
    value = item.get(field)
    if not isinstance(value, str) or not value:
        raise ValueError(f"{method}: {field} must be a non-empty string")
    return value


def _positive_int(item: dict[str, Any], field: str, method: str) -> int:
    value = item.get(field)
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        raise ValueError(f"{method}: {field} must be a positive integer")
    return value


def _required_bool(item: dict[str, Any], field: str, method: str) -> bool:
    value = item.get(field)
    if not isinstance(value, bool):
        raise ValueError(f"{method}: {field} must be a boolean")
    return value


@dataclass(frozen=True)
class CliRoute:
    command: str
    method: str
    required_capability: str | None
    repository_scope_mode: str


def load_cli_routes(raw: dict[str, Any], methods: list[MethodContract]) -> list[CliRoute]:
    raw_routes = raw.get("cli_routes")
    if not isinstance(raw_routes, list):
        raise ValueError("daemon method contract must contain a 'cli_routes' list")
    by_method = {method.method: method for method in methods}
    routes: list[CliRoute] = []
    seen: set[tuple[str, str | None]] = set()
    for index, route in enumerate(raw_routes):
        if not isinstance(route, dict):
            raise ValueError(f"contract cli_routes[{index}] must be an object")
        route_contract = _cli_route_contract(route, index)
        key = (route_contract.command, route_contract.subcommand)
        if key in seen:
            raise ValueError(
                f"duplicate CLI route in contract: "
                f"{format_command(route_contract.command, route_contract.subcommand)}"
            )
        seen.add(key)
        method_name = route_contract.method
        contract = by_method.get(method_name)
        if contract is None:
            raise ValueError(
                f"CLI route {format_command(route_contract.command, route_contract.subcommand)!r} emits "
                f"unregistered method {method_name!r}"
            )
        routes.append(
            CliRoute(
                command=format_command(route_contract.command, route_contract.subcommand),
                method=method_name,
                required_capability=contract.required_capability,
                repository_scope_mode=contract.repository_scope_mode,
            )
        )
    return routes


def _cli_route_contract(route: dict[str, Any], index: int) -> CliRouteContract:
    fields = set(route)
    expected = {"command", "subcommand", "method", "params"}
    if fields != expected:
        missing = sorted(expected - fields)
        unexpected = sorted(fields - expected)
        details: list[str] = []
        if missing:
            details.append("missing " + ", ".join(missing))
        if unexpected:
            details.append("unexpected " + ", ".join(unexpected))
        raise ValueError(
            f"contract cli_routes[{index}] has invalid fields: "
            + "; ".join(details)
        )
    command = _required_str(route, "command", f"cli_routes[{index}]")
    subcommand_value = route["subcommand"]
    if subcommand_value is not None and not isinstance(subcommand_value, str):
        raise ValueError(f"cli_routes[{index}]: subcommand must be a string or null")
    method = _required_str(route, "method", f"cli_routes[{index}]")
    params = _required_str(route, "params", f"cli_routes[{index}]")
    return CliRouteContract(
        command=command,
        subcommand=subcommand_value,
        method=method,
        params=params,
    )


def render_markdown(
    methods: list[MethodContract],
    *,
    cli_routes: list[CliRoute],
    source: str,
) -> str:
    lines = [
        f"<!-- Code generated by scripts/generate_daemon_method_tables.py from {source}; DO NOT EDIT. -->",
        "",
        "# Daemon Method Tables",
        "",
        "## Daemon Method Registry",
        "",
        "| RPC method | Capability | Scope | Params schema | Min envelope | Deprecated |",
        "|---|---|---|---:|---:|---:|",
    ]
    lines.extend(render_method_row(method) for method in methods)
    lines.extend(
        [
            "",
            "## CLI Route Translation",
            "",
            "| CLI command | RPC method | Capability | Scope |",
            "|---|---|---|---|",
        ]
    )
    lines.extend(render_cli_route_row(route) for route in cli_routes)
    lines.append("")
    return "\n".join(lines)


def render_method_row(method: MethodContract) -> str:
    return (
        f"| `{method.method}` | {format_capability(method.required_capability)} | "
        f"`{method.repository_scope_mode}` | {method.params_schema_version} | "
        f"{method.min_envelope} | {format_bool(method.deprecated)} |"
    )


def render_cli_route_row(route: CliRoute) -> str:
    return (
        f"| `{route.command}` | `{route.method}` | "
        f"{format_capability(route.required_capability)} | "
        f"`{route.repository_scope_mode}` |"
    )


def format_command(command: str, subcommand: str | None) -> str:
    return command if subcommand is None else f"{command} {subcommand}"


def format_capability(value: str | None) -> str:
    return "none" if value is None else f"`{value}`"


def format_bool(value: bool) -> str:
    return "yes" if value else "no"


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"generate_daemon_method_tables.py: {exc}", file=sys.stderr)
        raise SystemExit(1)
