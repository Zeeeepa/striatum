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


CLI_ROUTE_SAMPLES: tuple[tuple[str, str | None], ...] = (
    ("status", None),
    ("why", None),
    ("doctor", None),
    ("dashboard", None),
    ("list", "runs"),
    ("list", "sessions"),
    ("list", "jobs"),
    ("list", "artifacts"),
    ("list", "workflows"),
    ("run", "prepare"),
    ("run", "start"),
    ("run", "pause"),
    ("run", "resume"),
    ("run", "cancel"),
    ("run", "retry-job"),
    ("run", "summary"),
    ("run", "graph"),
    ("register-session", None),
    ("session", "close"),
    ("claim-next", None),
    ("ack", None),
    ("heartbeat", None),
    ("release", None),
    ("send", None),
    ("block", None),
    ("complete", None),
    ("publish-artifact", None),
    ("verdict", None),
    ("submit-review", None),
    ("override-verdict", None),
    ("recovery", "stale-leases"),
    ("recovery", "requeue-stale"),
    ("recovery", "cancel-job"),
    ("recovery", "process-reconcile"),
    ("recovery", "resume"),
    ("recovery", "auto"),
    ("recovery", "auto-publish"),
    ("recovery", "auto-finalize"),
    ("recovery", "watch"),
    ("evidence", "export"),
    ("corpus", "export"),
    ("archive", "create"),
    ("decision", "record"),
    ("checkpoint", "resolve"),
    ("inbox", None),
    ("escalation", "list"),
    ("escalation", "show"),
    ("escalation", "resolve"),
    ("branch", "confirm"),
    ("worktree", "create"),
    ("worktree", "release"),
    ("worktree", "list"),
    ("supervise", "start"),
    ("supervise", "send"),
    ("supervise", "stop"),
    ("supervise", "status"),
    ("supervise", "list"),
)


@dataclass(frozen=True)
class MethodContract:
    method: str
    required_capability: str | None
    repository_scope_mode: str
    params_schema_version: int
    audit_class: str
    min_envelope: int
    deprecated: bool


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

    repo_root = pathlib.Path(__file__).resolve().parents[1]
    contract_path = pathlib.Path(args.contract)
    output_path = pathlib.Path(args.out)
    methods = load_contract(contract_path)
    rendered = render_markdown(
        methods,
        cli_routes=load_cli_routes(repo_root, methods),
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


def load_contract(path: pathlib.Path) -> list[MethodContract]:
    raw = json.loads(path.read_text(encoding="utf-8"))
    raw_methods = raw.get("methods") if isinstance(raw, dict) else raw
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


def load_cli_routes(repo_root: pathlib.Path, methods: list[MethodContract]) -> list[CliRoute]:
    sys.path.insert(0, str(repo_root / "src"))
    from striatum.cli.daemon_rpc_route import _LOOKUP  # noqa: PLC0415

    by_method = {method.method: method for method in methods}
    routes: list[CliRoute] = []
    for command, subcommand in CLI_ROUTE_SAMPLES:
        translator = _LOOKUP.get((command, subcommand)) or _LOOKUP.get((command, None))
        if translator is None:
            raise ValueError(f"CLI route sample has no translator: {command} {subcommand}")
        method_name, _params = translator(_args_for_route(command, subcommand), repo_root)
        contract = by_method.get(method_name)
        if contract is None:
            raise ValueError(
                f"CLI route {format_command(command, subcommand)!r} emits "
                f"unregistered method {method_name!r}"
            )
        routes.append(
            CliRoute(
                command=format_command(command, subcommand),
                method=method_name,
                required_capability=contract.required_capability,
                repository_scope_mode=contract.repository_scope_mode,
            )
        )
    return routes


def _args_for_route(command: str, subcommand: str | None) -> argparse.Namespace:
    return argparse.Namespace(
        command=command,
        run_command=subcommand if command == "run" else None,
        workflow_command=subcommand if command == "workflow" else None,
        list_command=subcommand if command == "list" else None,
        recovery_command=subcommand if command == "recovery" else None,
        evidence_command=subcommand if command == "evidence" else None,
        corpus_command=subcommand if command == "corpus" else None,
        archive_command=subcommand if command == "archive" else None,
        decision_command=subcommand if command == "decision" else None,
        checkpoint_command=subcommand if command == "checkpoint" else None,
        escalation_command=subcommand if command == "escalation" else None,
        branch_command=subcommand if command == "branch" else None,
        session_command=subcommand if command == "session" else None,
        worktree_command=subcommand if command == "worktree" else None,
        supervise_command=subcommand if command == "supervise" else None,
        cross_repo_command=subcommand if command == "cross-repo" else None,
        id="target_1",
        run_id="run_1",
        job_id="job_1",
        session_id="sess_1",
        lease_id="lease_1",
        message_id="msg_1",
        workflow="workflow.yaml",
        role="worker",
        lane="codex",
        fresh=True,
        capability=["read"],
        parent_session_id=None,
        operator_label=None,
        force_non_fresh=False,
        lease_seconds=600,
        extend_seconds=600,
        kind="note",
        body_json="{}",
        severity="blocked",
        description="blocked on dependency",
        summary="done",
        logical_name="artifact",
        path="docs/out.md",
        verdict="accept",
        findings_artifact_id=None,
        rationale="reviewed",
        allow_no_process_execution=False,
        override_rationale=None,
        auto_fresh_session=False,
        state="running",
        workflow_job_id="job_a",
        limit=10,
        format="json",
        graph_orient="tb",
        graph_style="layered",
        reason="operator requested",
        cascade=False,
        force=False,
        justification=None,
        blocker_id="blocker_1",
        escalation_id="esc_1",
        resolution_note="resolved by principal",
        complete=False,
        dry_run=True,
        autonomous_review_requeue=True,
        autonomous_process_reconcile=True,
        max_requeues_per_sweep=1,
        checkpoint_timeout_seconds=60,
        eligible_after_seconds=30,
        interval_seconds=5,
        exit_on_terminal=True,
        max_sweeps=1,
        mtime_grace_seconds=30,
        since="HEAD~1",
        out="tmp/export",
        title="Decision",
        outcome="accepted",
        decision_id="decision_1",
        follow_up=None,
        action="continue",
        branch="work/branch",
        create=False,
        use_current=False,
        strict=False,
        worktree_id="worktree_1",
        packet_id="packet_1",
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
