#!/usr/bin/env python3
"""Generate the Go daemon RPC registry from the daemon method contract."""

from __future__ import annotations

import argparse
import difflib
import json
import pathlib
import sys
from dataclasses import dataclass
from typing import Any


CAPABILITY_EXPR = {
    None: "nil",
    "read": "CapPtr(CapabilityRead)",
    "write": "CapPtr(CapabilityWrite)",
    "review": "CapPtr(CapabilityReview)",
    "claim": "CapPtr(CapabilityClaim)",
    "apply": "CapPtr(CapabilityApply)",
    "admin": "CapPtr(CapabilityAdmin)",
    "recovery": "CapPtr(CapabilityRecovery)",
    "surgical_recovery": "CapPtr(CapabilitySurgicalRecovery)",
}

SCOPE_MODE_EXPR = {
    "single_repo": "ScopeSingleRepo",
    "cross_repo": "ScopeCrossRepo",
    "daemon_global": "ScopeDaemonGlobal",
}


@dataclass(frozen=True)
class MethodContract:
    method: str
    required_capability: str | None
    repository_scope: bool
    repository_scope_mode: str
    params_schema_version: int
    audit_class: str
    min_envelope: int
    deprecated: bool


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contract", default="contracts/daemon_methods.json")
    parser.add_argument("--out", default="go/pkg/rpc/registry_methods.go")
    parser.add_argument(
        "--check",
        action="store_true",
        help="fail when the generated output differs from --out",
    )
    args = parser.parse_args()

    contract_path = pathlib.Path(args.contract)
    output_path = pathlib.Path(args.out)
    methods = load_contract(contract_path)
    rendered = render_go(methods, source=contract_path.as_posix())

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


def load_contract(path: pathlib.Path) -> list[MethodContract]:
    raw = json.loads(path.read_text(encoding="utf-8"))
    raw_methods = raw.get("methods") if isinstance(raw, dict) else raw
    if isinstance(raw_methods, dict):
        mapped_methods: list[dict[str, Any]] = []
        for method, metadata in raw_methods.items():
            if not isinstance(metadata, dict):
                raise ValueError(f"contract method {method!r} metadata must be an object")
            mapped_methods.append({"method": method, **metadata})
        raw_methods = mapped_methods
    if not isinstance(raw_methods, list):
        raise ValueError("daemon method contract must be a list or contain a 'methods' list")

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

        required_capability = item.get("required_capability")
        if required_capability is not None and required_capability not in CAPABILITY_EXPR:
            raise ValueError(f"{method}: unknown required_capability {required_capability!r}")

        repository_scope_mode = item.get("repository_scope_mode")
        repository_scope_value = item.get("repository_scope")
        if repository_scope_value is not None and not isinstance(repository_scope_value, bool):
            raise ValueError(f"{method}: repository_scope must be boolean when present")
        if repository_scope_mode is None and repository_scope_value is None:
            raise ValueError(f"{method}: repository_scope_mode is required when repository_scope is absent")
        if repository_scope_mode is None:
            repository_scope_mode = "single_repo" if repository_scope_value else "daemon_global"
        if repository_scope_mode not in SCOPE_MODE_EXPR:
            raise ValueError(f"{method}: unknown repository_scope_mode {repository_scope_mode!r}")
        repository_scope = (
            repository_scope_value
            if repository_scope_value is not None
            else repository_scope_mode == "single_repo"
        )

        methods.append(
            MethodContract(
                method=method,
                required_capability=required_capability,
                repository_scope=repository_scope,
                repository_scope_mode=repository_scope_mode,
                params_schema_version=int(item.get("params_schema_version", 1)),
                audit_class=str(item.get("audit_class", "metadata")),
                min_envelope=int(item.get("min_envelope", 1)),
                deprecated=bool(item.get("deprecated", False)),
            )
        )
    return methods


def render_go(methods: list[MethodContract], *, source: str) -> str:
    lines = [
        f"// Code generated by scripts/generate_go_rpc_registry.py from {source}; DO NOT EDIT.",
        "",
        "package rpc",
        "",
        "var methodEntries = []MethodEntry{",
    ]
    lines.extend(render_entry(method) for method in methods)
    lines.append("}")
    lines.append("")
    return "\n".join(lines)


def render_entry(method: MethodContract) -> str:
    deprecated = ", Deprecated: true" if method.deprecated else ""
    return (
        f"\t{{Method: {go_string(method.method)}, "
        f"RequiredCapability: {CAPABILITY_EXPR[method.required_capability]}, "
        f"RepositoryScope: {go_bool(method.repository_scope)}, "
        f"RepositoryScopeMode: {SCOPE_MODE_EXPR[method.repository_scope_mode]}, "
        f"ParamsSchemaVersion: {method.params_schema_version}, "
        f"AuditClass: {go_string(method.audit_class)}, "
        f"MinEnvelope: {method.min_envelope}{deprecated}}},"
    )


def go_string(value: str) -> str:
    return json.dumps(value, ensure_ascii=True)


def go_bool(value: bool) -> str:
    return "true" if value else "false"


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"generate_go_rpc_registry.py: {exc}", file=sys.stderr)
        raise SystemExit(1)
