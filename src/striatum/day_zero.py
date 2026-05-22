"""Day-zero setup helpers for local Striatum adoption."""

from __future__ import annotations

import os
import subprocess
import sys
import uuid
from pathlib import Path
from typing import Any, Literal
from urllib.parse import urljoin
from urllib.request import Request, urlopen

import striatum
from striatum.bootstrap import init_operational_scratch
from striatum.cli.daemon_required import daemon_socket_is_reachable, resolve_socket_path
from striatum.daemon_pg.config import resolve_config
from striatum.daemon_pg.connection import connect, connect_and_migrate, doctor as pg_doctor
from striatum.daemon_runtime import mcp_endpoint_file, read_runtime_token, token_file
from striatum.primitives import json_dumps, json_loads

ServiceManager = Literal["auto", "systemd", "launchd"]

SYSTEMD_UNIT_NAME = "striatumd.service"
LAUNCHD_LABEL = "io.striatum.striatumd"
MCP_HTTP_URL_ENV = "STRIATUM_DAEMON_MCP_HTTP_URL"
MCP_HTTP_URL_COMPAT_ENV = "STRIATUM_MCP_URL"
STARTER_WORKFLOW_GUIDANCE = (
    "The suggested starter workflow is a safe first-run scaffold; see "
    "docs/WORKFLOW_TYPES.md to choose a different workflow shape."
)


def service_install(
    *,
    manager: ServiceManager = "auto",
    dry_run: bool = False,
) -> dict[str, Any]:
    resolved = _resolve_manager(manager)
    target = _service_path(resolved)
    content = _service_content(resolved)
    result: dict[str, Any] = {
        "manager": resolved,
        "path": str(target),
        "dry_run": dry_run,
        "content": content,
    }
    if dry_run:
        result["status"] = "would_write"
        return result
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")
    result["status"] = "written"
    if resolved == "systemd":
        daemon_reload = _run(["systemctl", "--user", "daemon-reload"])
        result["daemon_reload"] = daemon_reload
    return result


def service_start(
    *,
    manager: ServiceManager = "auto",
    dry_run: bool = False,
) -> dict[str, Any]:
    resolved = _resolve_manager(manager)
    command = _service_start_command(resolved)
    if dry_run:
        return {"manager": resolved, "dry_run": True, "command": command, "status": "would_start"}
    result = _run(command)
    return {
        "manager": resolved,
        "dry_run": False,
        "command": command,
        "status": _status_from_returncode(result),
        "process": result,
    }


def service_status(*, manager: ServiceManager = "auto") -> dict[str, Any]:
    resolved = _resolve_manager(manager)
    command = _service_status_command(resolved)
    result = _run(command)
    return {
        "manager": resolved,
        "command": command,
        "status": _status_from_returncode(result),
        "process": result,
    }


def adopt(
    repo: Path,
    *,
    profile: str = "claude_code",
    dry_run: bool = False,
    with_skills: bool = True,
    with_plugins: bool = True,
    with_ddd_layout: bool = True,
    register: bool = True,
    postgres_url: str | None = None,
) -> dict[str, Any]:
    repo = repo.resolve()
    inspection = _inspect_repo(repo)
    result: dict[str, Any] = {
        "repo": str(repo),
        "dry_run": dry_run,
        "inspection": inspection,
        "suggested_workflow": str(
            repo / "striatum" / "workflows" / "first-workflow" / "workflow.json"
        ),
        "suggested_workflow_guidance": STARTER_WORKFLOW_GUIDANCE,
    }
    if dry_run:
        result["init"] = {
            "status": "would_init" if not inspection["state_db_exists"] else "would_skip"
        }
    else:
        state_dir = init_operational_scratch(repo)
        result["init"] = {
            "status": "scratch_initialized",
            "state_dir": str(state_dir),
            "scratch_dir": str(state_dir / "scratch"),
        }
    if with_skills:
        result["skills"] = _skills_install(repo, profile=profile, dry_run=dry_run)
    if with_plugins and profile != "generic":
        result["plugins"] = _plugin_install(repo, profile=profile, dry_run=dry_run)
    if with_ddd_layout:
        from striatum.scaffold import scaffold_ddd_layout

        result["ddd_layout"] = scaffold_ddd_layout(repo, dry_run=dry_run)
    if register:
        cfg = resolve_config(postgres_url=postgres_url)
        if cfg.url is None:
            result["registration"] = {
                "status": "postgres_url_missing",
                "hint": "configure STRIATUM_DAEMON_DB_URL, then rerun striatum adopt or striatum repo add --init",
            }
        elif dry_run:
            result["registration"] = {
                "status": "would_register_repo",
                "postgres_url_source": cfg.source,
                "redacted_url": cfg.redacted_url,
            }
        elif inspection["state_db_exists"]:
            result["registration"] = {
                "status": "sqlite_migration_required",
                "state_db_path": str(repo / ".striatum" / "state.sqlite3"),
                "hint": (
                    "Legacy SQLite state found. Migration to PostgreSQL is required. "
                    "Archive or remove the legacy .striatum/state.sqlite3 file, then "
                    "register the repository with striatum adopt or striatum repo add --init."
                ),
            }
        else:
            from striatum.daemon_pg.repositories import repo_add_pg

            conn = connect_and_migrate(postgres_url=cfg.url, daemon_version=striatum.__version__)
            try:
                result["registration"] = repo_add_pg(conn, repo, init=True)
            finally:
                conn.close()
    return result


def first_run_smoke(repo: Path) -> dict[str, Any]:
    repo = repo.resolve()
    socket = resolve_socket_path()
    token_value = _read_runtime_token_value()
    token = _runtime_token_check(token_value)
    postgres = pg_doctor()
    repository_id = _lookup_repository_id(repo)
    checks: list[dict[str, Any]] = [
        {
            "id": "daemon_socket",
            "ok": daemon_socket_is_reachable(socket),
            "socket": str(socket),
            "hint": "run striatum daemon service install/start or striatum daemon start",
        },
        _go_binary_check(),
        token,
        {
            "id": "postgres",
            "ok": bool(postgres.get("ok")),
            "status": postgres.get("status"),
            "hint": "run striatum daemon doctor --apply-migrations --repair-grants",
        },
        {
            "id": "repo_registration",
            "ok": repository_id is not None,
            "repository_id": repository_id,
            "hint": "run striatum adopt or striatum repo add --init",
        },
    ]
    mcp_check = _mcp_capability_check(repository_id=repository_id, token=token_value)
    checks.append(mcp_check)
    checks.append(
        _sample_read_route_check(repository_id=repository_id, token=token_value, socket=socket)
    )
    return {
        "schema_version": "striatum.first_run_diagnostic.v1",
        "mode": "first_run",
        "repo": str(repo),
        "ok": all(bool(check.get("ok")) for check in checks),
        "checks": checks,
        "postgres": postgres,
    }


def _inspect_repo(repo: Path) -> dict[str, Any]:
    state_db = repo / ".striatum" / "state.sqlite3"
    return {
        "exists": repo.exists(),
        "is_git": (repo / ".git").exists(),
        "striatum_dir_exists": (repo / ".striatum").exists(),
        "state_db_exists": state_db.exists(),
        "state_tombstone_exists": (repo / ".striatum" / "state.sqlite3.tombstone").exists(),
        "ddd_docs_exist": (repo / "docs" / "SPEC.md").exists(),
        "workflows_dir_exists": (repo / "workflows").exists(),
    }


def _skills_install(repo: Path, *, profile: str, dry_run: bool) -> dict[str, Any]:
    from striatum.cli.dispatch import _skills_install_dispatch

    return _skills_install_dispatch(
        target=repo,
        profile=profile,
        scope="project",
        namespace="striatum-",
        force=False,
        dry_run=dry_run,
    )


def _plugin_install(repo: Path, *, profile: str, dry_run: bool) -> dict[str, Any]:
    from striatum.plugins import install as plugin_install
    from striatum.plugins.install import ALL_PROFILES_ORDER

    if profile != "all":
        return plugin_install.install(
            target=repo,
            profile=profile,
            scope="project",
            namespace="striatum",
            force=False,
            dry_run=dry_run,
        )
    return {
        "profile": "all",
        "results": [
            plugin_install.install(
                target=repo,
                profile=sub_profile,
                scope="project",
                namespace="striatum",
                force=False,
                dry_run=dry_run,
            )
            for sub_profile in ALL_PROFILES_ORDER
        ],
    }


def _read_runtime_token_value() -> str | None:
    try:
        return read_runtime_token()
    except Exception:
        return None


def _runtime_token_check(token: str | None) -> dict[str, Any]:
    return {
        "id": "runtime_token",
        "ok": token is not None,
        "path": str(token_file()),
        "hint": "start the daemon once to create the runtime token",
    }


def _go_binary_check() -> dict[str, Any]:
    try:
        from striatum.cli.daemon import _parse_go_describe, resolve_go_binary

        binary = resolve_go_binary()
        proc = subprocess.run(
            [str(binary), "--describe"],
            check=False,
            capture_output=True,
            text=True,
            timeout=5,
        )
        if proc.returncode != 0:
            detail = (proc.stderr or proc.stdout).strip()
            return {
                "id": "go_daemon_binary",
                "ok": False,
                "path": str(binary),
                "status": "describe_failed",
                "error": detail,
                "hint": "rebuild go/bin/striatumd or reinstall the package",
            }
        fields = _parse_go_describe(proc.stdout)
        return {
            "id": "go_daemon_binary",
            "ok": True,
            "path": str(binary),
            "daemon_version": fields.get("daemon_version"),
            "git_sha": fields.get("git_sha"),
            "build_dirty": fields.get("build_dirty"),
            "methods_etag": fields.get("methods_etag"),
            "supported_schema": fields.get("supported_schema"),
            "migration_count": fields.get("migration_count"),
        }
    except Exception as exc:  # noqa: BLE001 - diagnostics must report all failures.
        return {
            "id": "go_daemon_binary",
            "ok": False,
            "status": "failed",
            "error": str(exc),
            "hint": "run make daemon-go-build or reinstall the package",
        }


def _lookup_repository_id(repo: Path) -> str | None:
    cfg = resolve_config()
    if cfg.url is None:
        return None
    try:
        conn = connect(cfg.url)
    except Exception:
        return None
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT repository_id FROM striatumd.repositories WHERE repo_root = %s AND state = 'active'",
                (str(repo),),
            )
            row = cur.fetchone()
    except Exception:
        return None
    finally:
        conn.close()
    if row is None:
        return None
    return str(row[0])


def _mcp_capability_check(*, repository_id: str | None, token: Any) -> dict[str, Any]:
    if repository_id is None or not isinstance(token, str):
        return {
            "id": "mcp_capability",
            "ok": False,
            "status": "skipped",
            "hint": "requires a runtime token and a registered repo",
        }
    endpoint = _mcp_http_endpoint()
    if endpoint is None:
        return {
            "id": "mcp_capability",
            "ok": False,
            "status": "skipped",
            "hint": (
                f"requires native daemon MCP endpoint via {MCP_HTTP_URL_ENV} "
                f"or {mcp_endpoint_file()}"
            ),
        }
    try:
        tools = _mcp_http_tools_list(endpoint=endpoint, repository_id=repository_id, token=token)
    except Exception as exc:  # noqa: BLE001
        return {"id": "mcp_capability", "ok": False, "status": "failed", "error": str(exc)}
    return {
        "id": "mcp_capability",
        "ok": bool(tools),
        "tool_count": len(tools),
        "endpoint": endpoint,
    }


def _mcp_http_endpoint() -> str | None:
    for env_name in (MCP_HTTP_URL_ENV, MCP_HTTP_URL_COMPAT_ENV):
        value = os.environ.get(env_name)
        if value:
            return value.strip()
    endpoint_file = mcp_endpoint_file()
    if not endpoint_file.exists():
        return None
    raw = endpoint_file.read_text(encoding="utf-8").strip()
    if raw == "":
        return None
    try:
        payload = json_loads(raw)
    except ValueError:
        return raw
    if isinstance(payload, dict):
        for key in ("sse_url", "url", "endpoint"):
            value = payload.get(key)
            if isinstance(value, str) and value:
                return value
    return raw


def _mcp_http_tools_list(*, endpoint: str, repository_id: str, token: str) -> list[dict[str, Any]]:
    headers = {
        "Accept": "text/event-stream",
        "Authorization": f"Bearer {token}",
    }
    sse = urlopen(Request(endpoint, headers=headers), timeout=5)
    try:
        first_event = _read_sse_event(sse)
        message_url = _mcp_message_url(endpoint, first_event)
        _mcp_jsonrpc_over_sse(
            message_url=message_url,
            sse=sse,
            token=token,
            method="initialize",
            params={
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "striatum-first-run", "version": striatum.__version__},
            },
        )
        result = _mcp_jsonrpc_over_sse(
            message_url=message_url,
            sse=sse,
            token=token,
            method="tools/list",
            params={"repository_id": repository_id, "token": token},
        )
    finally:
        sse.close()
    tools = result.get("tools")
    if not isinstance(tools, list):
        raise RuntimeError("daemon MCP tools/list response did not contain a tools list")
    return [dict(tool) for tool in tools if isinstance(tool, dict)]


def _mcp_jsonrpc_over_sse(
    *,
    message_url: str,
    sse: Any,
    token: str,
    method: str,
    params: dict[str, Any],
) -> dict[str, Any]:
    request_id = f"req_{uuid.uuid4().hex}"
    payload = {
        "jsonrpc": "2.0",
        "id": request_id,
        "method": method,
        "params": params,
    }
    body = json_dumps(payload).encode("utf-8")
    request = Request(
        message_url,
        data=body,
        headers={
            "Accept": "application/json, text/event-stream",
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    with urlopen(request, timeout=5) as response:
        response_body = response.read().decode("utf-8").strip()
    if response_body:
        rpc_response = json_loads(response_body)
    else:
        rpc_response = _read_matching_mcp_response(sse, request_id=request_id)
    if not isinstance(rpc_response, dict):
        raise RuntimeError(f"daemon MCP {method} response was not an object")
    if rpc_response.get("id") != request_id:
        raise RuntimeError(f"daemon MCP {method} response id mismatch")
    error = rpc_response.get("error")
    if isinstance(error, dict):
        raise RuntimeError(str(error.get("message") or error.get("code") or error))
    result = rpc_response.get("result")
    if not isinstance(result, dict):
        raise RuntimeError(f"daemon MCP {method} response result was not an object")
    return result


def _read_matching_mcp_response(sse: Any, *, request_id: str) -> dict[str, Any]:
    for _ in range(10):
        event = _read_sse_event(sse)
        if event.get("event") not in ("message", "response", ""):
            continue
        data = event.get("data")
        if not isinstance(data, str) or data == "":
            continue
        payload = json_loads(data)
        if isinstance(payload, dict) and payload.get("id") == request_id:
            return payload
    raise RuntimeError(f"daemon MCP response {request_id} was not received")


def _mcp_message_url(endpoint: str, event: dict[str, str]) -> str:
    data = event.get("data", "").strip()
    if data == "":
        raise RuntimeError("daemon MCP SSE endpoint did not publish a message URL")
    uri = data
    try:
        payload = json_loads(data)
    except ValueError:
        payload = None
    if isinstance(payload, dict):
        value = payload.get("uri") or payload.get("url")
        if isinstance(value, str) and value:
            uri = value
    return uri if uri.startswith(("http://", "https://")) else urljoin(endpoint, uri)


def _read_sse_event(stream: Any) -> dict[str, str]:
    event_type = "message"
    data_lines: list[str] = []
    while True:
        raw = stream.readline()
        if raw == b"":
            raise RuntimeError("daemon MCP SSE stream closed")
        line = raw.decode("utf-8").rstrip("\r\n")
        if line == "":
            if data_lines:
                return {"event": event_type, "data": "\n".join(data_lines)}
            event_type = "message"
            continue
        if line.startswith(":"):
            continue
        field, _, value = line.partition(":")
        if value.startswith(" "):
            value = value[1:]
        if field == "event":
            event_type = value
        elif field == "data":
            data_lines.append(value)


def _sample_read_route_check(
    *, repository_id: str | None, token: Any, socket: Path
) -> dict[str, Any]:
    if repository_id is None or not isinstance(token, str):
        return {
            "id": "sample_read_route",
            "ok": False,
            "status": "skipped",
            "hint": "requires runtime token and a registered repo",
        }
    if not daemon_socket_is_reachable(socket):
        return {"id": "sample_read_route", "ok": False, "status": "daemon_unreachable"}
    try:
        from striatum.daemon_rpc.envelope import RpcEnvelope

        response = _call_rpc_sequence(
            socket,
            RpcEnvelope(
                schema_version=1,
                request_id=uuid.uuid4().hex,
                method="doctor",
                params={"repository_id": repository_id, "verbose": False},
                capability_token=token,
            ),
        )
    except Exception as exc:  # noqa: BLE001
        return {"id": "sample_read_route", "ok": False, "status": "failed", "error": str(exc)}
    return {
        "id": "sample_read_route",
        "ok": bool(response.get("ok")),
        "response_ok": response.get("ok"),
    }


def _call_rpc_sequence(socket: Path, envelope: Any) -> dict[str, Any]:
    import socket as socket_mod

    from striatum.daemon_rpc.client import hello_envelope

    with socket_mod.socket(socket_mod.AF_UNIX, socket_mod.SOCK_STREAM) as sock:
        sock.settimeout(10.0)
        sock.connect(str(socket))
        stream = sock.makefile("rwb")
        stream.write(
            hello_envelope(
                request_id=uuid.uuid4().hex,
                client_name="striatum-first-run",
                client_version=striatum.__version__,
            ).encode()
            + b"\n"
        )
        stream.flush()
        hello = json_loads(stream.readline().decode("utf-8"))
        if not isinstance(hello, dict) or not hello.get("ok"):
            raise RuntimeError("daemon.hello failed during first-run smoke")
        stream.write(envelope.encode() + b"\n")
        stream.flush()
        response = json_loads(stream.readline().decode("utf-8"))
    if not isinstance(response, dict):
        raise RuntimeError("daemon RPC response was not an object")
    return response


def _resolve_manager(manager: ServiceManager) -> Literal["systemd", "launchd"]:
    if manager == "launchd":
        return "launchd"
    if manager == "systemd":
        return "systemd"
    return "launchd" if sys.platform == "darwin" else "systemd"


def _service_path(manager: Literal["systemd", "launchd"]) -> Path:
    if manager == "launchd":
        return Path.home() / "Library" / "LaunchAgents" / f"{LAUNCHD_LABEL}.plist"
    config_home = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config"))
    return config_home / "systemd" / "user" / SYSTEMD_UNIT_NAME


def _service_content(manager: Literal["systemd", "launchd"]) -> str:
    command = f"{sys.executable} -m striatum.cli daemon start"
    if manager == "launchd":
        return (
            '<?xml version="1.0" encoding="UTF-8"?>\n'
            '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" '
            '"http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n'
            '<plist version="1.0">\n'
            "<dict>\n"
            f"  <key>Label</key><string>{LAUNCHD_LABEL}</string>\n"
            "  <key>ProgramArguments</key>\n"
            "  <array>\n"
            f"    <string>{sys.executable}</string>\n"
            "    <string>-m</string>\n"
            "    <string>striatum.cli</string>\n"
            "    <string>daemon</string>\n"
            "    <string>start</string>\n"
            "  </array>\n"
            "  <key>RunAtLoad</key><true/>\n"
            "  <key>KeepAlive</key><true/>\n"
            "</dict>\n"
            "</plist>\n"
        )
    return (
        "[Unit]\n"
        "Description=Striatum local workflow daemon\n\n"
        "[Service]\n"
        f"ExecStart={command}\n"
        "Restart=on-failure\n"
        "RestartSec=2\n\n"
        "[Install]\n"
        "WantedBy=default.target\n"
    )


def _service_start_command(manager: Literal["systemd", "launchd"]) -> list[str]:
    if manager == "launchd":
        return ["launchctl", "bootstrap", f"gui/{os.getuid()}", str(_service_path(manager))]
    return ["systemctl", "--user", "start", SYSTEMD_UNIT_NAME]


def _service_status_command(manager: Literal["systemd", "launchd"]) -> list[str]:
    if manager == "launchd":
        return ["launchctl", "print", f"gui/{os.getuid()}/{LAUNCHD_LABEL}"]
    return ["systemctl", "--user", "is-active", SYSTEMD_UNIT_NAME]


def _run(command: list[str]) -> dict[str, Any]:
    try:
        proc = subprocess.run(
            command,
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except FileNotFoundError as exc:
        return {
            "argv": command,
            "returncode": 127,
            "stdout": "",
            "stderr": str(exc),
        }
    return {
        "argv": command,
        "returncode": proc.returncode,
        "stdout": proc.stdout,
        "stderr": proc.stderr,
    }


def _status_from_returncode(result: dict[str, Any]) -> str:
    return "ok" if result.get("returncode") == 0 else "failed"
