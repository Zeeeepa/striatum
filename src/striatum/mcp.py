"""Minimal local stdio JSON-RPC wrapper for Striatum commands.

Wire framing
------------

The wrapper supports two on-the-wire framings:

* ``framed`` -- LSP/MCP-style ``Content-Length: N\\r\\n\\r\\n<body>``. This is
  the default for real MCP clients (Claude Desktop, IDE integrations) and the
  only safe choice for bodies that may contain newlines.
* ``line`` -- one JSON-RPC object per text line. This is the legacy local
  shape used by tests and hand-rolled scripts.

In ``auto`` mode, the framing is detected from the very first message: if it
starts with a ``Content-Length`` header, both reads and writes lock into
framed mode; otherwise they lock into line mode. Operators can pin either
mode via ``--framing``.
"""

from __future__ import annotations

import argparse
import json
import uuid
import sys
from pathlib import Path
from typing import Any, BinaryIO, TextIO, cast
from urllib.parse import parse_qs, urlparse

from striatum.api import invoke
from striatum.db import JsonObject, json_dumps

JsonRpcId = str | int | None
FramingMode = str  # "auto" | "framed" | "line"

JSONRPC_VERSION = "2.0"
SERVER_NAME = "striatum-local"
SERVER_VERSION = "0.1.0"

ERROR_PARSE = -32700
ERROR_INVALID_REQUEST = -32600
ERROR_METHOD_NOT_FOUND = -32601
ERROR_INVALID_PARAMS = -32602
ERROR_INTERNAL = -32603

CONTENT_LENGTH_HEADER = "content-length"


TOOL_ARGV: dict[str, list[str]] = {
    "init": ["init"],
    "workflow_validate": ["workflow", "validate", "$path"],
    "workflow_plan": ["workflow", "plan", "$path"],
    "run_prepare": ["run", "prepare", "--workflow", "$workflow"],
    "run_start": ["run", "start", "--run-id", "$run_id"],
    "run_summary": ["run", "summary", "--run-id", "$run_id", "--path", "$path"],
    "branch_confirm": ["branch", "confirm", "--run-id", "$run_id", "--branch", "$branch"],
    "register_session": ["register-session", "--run-id", "$run_id", "--role", "$role", "--lane", "$lane"],
    "claim_next": ["claim-next", "--session-id", "$session_id"],
    "ack": ["ack", "--session-id", "$session_id", "--message-id", "$message_id", "--lease-id", "$lease_id"],
    "heartbeat": ["heartbeat", "--session-id", "$session_id", "--lease-id", "$lease_id"],
    "release": [
        "release",
        "--session-id",
        "$session_id",
        "--message-id",
        "$message_id",
        "--lease-id",
        "$lease_id",
        "--reason",
        "$reason",
    ],
    "send": ["send", "--session-id", "$session_id", "--kind", "$kind"],
    "block": [
        "block",
        "--session-id",
        "$session_id",
        "--job-id",
        "$job_id",
        "--lease-id",
        "$lease_id",
        "--kind",
        "$kind",
        "--severity",
        "$severity",
        "--description",
        "$description",
    ],
    "publish_artifact": [
        "publish-artifact",
        "--session-id",
        "$session_id",
        "--job-id",
        "$job_id",
        "--lease-id",
        "$lease_id",
        "--kind",
        "$kind",
        "--logical-name",
        "$logical_name",
        "--path",
        "$path",
    ],
    "submit_review": [
        "submit-review",
        "--session-id",
        "$session_id",
        "--job-id",
        "$job_id",
        "--lease-id",
        "$lease_id",
        "--path",
        "$path",
        "--verdict",
        "$verdict",
    ],
    "complete_job": ["complete", "--session-id", "$session_id", "--job-id", "$job_id", "--lease-id", "$lease_id"],
    "verdict": [
        "verdict",
        "--session-id",
        "$session_id",
        "--job-id",
        "$job_id",
        "--lease-id",
        "$lease_id",
        "--verdict",
        "$verdict",
    ],
    "evidence_export": ["evidence", "export", "--run-id", "$run_id", "--path", "$path"],
    "status": ["status"],
    "why": ["why", "$id"],
    "doctor": ["doctor"],
    "recovery_stale_leases": ["recovery", "stale-leases", "--run-id", "$run_id"],
}

OPTIONAL_ARGS: dict[str, list[tuple[str, str]]] = {
    "register_session": [("capability", "--capability"), ("parent_session_id", "--parent-session-id")],
    "claim_next": [("lease_seconds", "--lease-seconds")],
    "heartbeat": [("extend_seconds", "--extend-seconds")],
    "send": [("body_json", "--body-json")],
    "submit_review": [("logical_name", "--logical-name"), ("kind", "--kind"), ("rationale", "--rationale")],
    "complete_job": [("summary", "--summary")],
    "verdict": [("findings_artifact_id", "--findings-artifact-id"), ("rationale", "--rationale")],
    "status": [("run_id", "--run-id")],
    "doctor": [("run_id", "--run-id")],
}

BOOLEAN_FLAGS: dict[str, list[tuple[str, str]]] = {
    "branch_confirm": [("create", "--create"), ("use_current", "--use-current")],
    "register_session": [("fresh", "--fresh")],
    "release": [("requeue", "--requeue")],
}


def main(argv: list[str] | None = None) -> int:
    """Run the stdio JSON-RPC wrapper."""
    parser = argparse.ArgumentParser(prog="python -m striatum.mcp")
    parser.add_argument("--repo", default=".", help="target repository root")
    parser.add_argument(
        "--framing",
        choices=("auto", "framed", "line"),
        default="auto",
        help=(
            "wire framing for JSON-RPC messages: 'framed' uses Content-Length headers "
            "(LSP/MCP style), 'line' uses one JSON object per line, 'auto' detects "
            "from the first inbound message and locks the response shape to match"
        ),
    )
    args = parser.parse_args(argv)
    serve_stdio(
        repo=Path(args.repo),
        stdin=sys.stdin,
        stdout=sys.stdout,
        framing=args.framing,
    )
    return 0


class _ModeHolder:
    """Mutable holder for the resolved framing mode.

    The first read locks ``mode`` to either ``"framed"`` or ``"line"`` and that
    value is reused for every write thereafter. ``"auto"`` is the unresolved
    starting state.
    """

    __slots__ = ("mode",)

    def __init__(self, initial: FramingMode) -> None:
        self.mode = initial


def serve_stdio(
    *,
    repo: Path,
    stdin: TextIO | BinaryIO,
    stdout: TextIO | BinaryIO,
    framing: FramingMode = "auto",
) -> None:
    """Serve JSON-RPC requests from ``stdin`` until EOF.

    ``framing`` selects the wire shape:

    * ``"auto"`` -- detect from the first inbound message; lock both reads
      and writes to that shape thereafter.
    * ``"framed"`` -- always read and write Content-Length-framed messages.
    * ``"line"`` -- always read and write line-delimited messages.

    ``stdin`` / ``stdout`` may be either text or binary streams; the function
    upgrades text streams to their underlying buffered binary streams when
    available so framing byte counts stay correct.
    """
    if framing not in ("auto", "framed", "line"):
        raise ValueError(f"unknown framing {framing!r}")

    server = LocalRpcServer(repo=repo)
    in_stream = _as_binary_stream(stdin, mode="r")
    out_stream = _as_binary_stream(stdout, mode="w")
    mode = _ModeHolder(framing)

    while True:
        body = read_message(in_stream, mode)
        if body is None:
            break
        response = server.handle_line(body)
        if response is not None:
            write_message(out_stream, json_dumps(response), mode)


def _as_binary_stream(stream: TextIO | BinaryIO, *, mode: str) -> BinaryIO:
    """Return a binary view of a stdio stream.

    Real ``sys.stdin``/``sys.stdout`` are text wrappers but expose a
    ``buffer`` attribute that is the underlying ``BufferedReader`` /
    ``BufferedWriter``. Tests pass raw ``BytesIO`` instances, which we use
    directly. Anything else is wrapped just enough to satisfy the read/write
    surface we need.
    """
    buffer = getattr(stream, "buffer", None)
    if buffer is not None:
        return cast(BinaryIO, buffer)
    # Already bytes-oriented (BytesIO in tests, or a hand-rolled BinaryIO).
    if hasattr(stream, "read") and hasattr(stream, "write"):
        return cast(BinaryIO, stream)
    raise TypeError(f"unsupported stdio stream for {mode!r}: {type(stream)!r}")


def read_message(stream: BinaryIO, mode: _ModeHolder) -> str | None:
    """Read a single JSON-RPC body string from ``stream``.

    On the first call when ``mode`` is ``"auto"``, this peeks the first
    non-empty line: if it begins with ``Content-Length:`` (case-insensitive)
    we treat the message as a Content-Length-framed payload and lock
    ``mode.mode`` to ``"framed"``; otherwise we treat the line itself as the
    body and lock to ``"line"``.

    Returns ``None`` at EOF.
    """
    # In "framed" mode we always parse headers first, but a stray blank line
    # between messages is permitted (some clients write a trailing CRLF).
    first = _read_first_nonempty_line(stream)
    if first is None:
        return None

    is_header = _looks_like_header(first)
    if mode.mode == "auto":
        mode.mode = "framed" if is_header else "line"

    if mode.mode == "framed":
        if not is_header:
            # Pinned framed mode but the peer sent a bare line: surface it
            # as the body so the dispatcher returns a JSON-RPC parse error
            # rather than the server hanging on a missing Content-Length.
            return first.decode("utf-8", errors="replace")
        return _read_framed_body(stream, first)

    # Line mode: each non-empty line is one JSON-RPC message. Even if it
    # happens to look like a Content-Length header, line mode treats it
    # verbatim -- pinning to line is an explicit operator choice.
    return first.decode("utf-8", errors="replace")


def write_message(stream: BinaryIO, body: str, mode: _ModeHolder) -> None:
    """Write one JSON-RPC body string to ``stream`` in the active framing.

    ``mode`` must already be resolved (``"framed"`` or ``"line"``); ``"auto"``
    is treated as ``"line"`` for safety in the corner case where a server
    writes a notification before seeing any request.
    """
    if mode.mode == "framed":
        encoded = body.encode("utf-8")
        header = f"Content-Length: {len(encoded)}\r\n\r\n".encode("ascii")
        stream.write(header)
        stream.write(encoded)
    else:
        stream.write(body.encode("utf-8"))
        stream.write(b"\n")
    stream.flush()


def _read_first_nonempty_line(stream: BinaryIO) -> bytes | None:
    """Return the first non-empty line from ``stream`` or ``None`` at EOF.

    Trailing ``\\r\\n`` or ``\\n`` is stripped. Empty lines (typical between
    framed messages) are skipped.
    """
    while True:
        raw = stream.readline()
        if raw == b"":
            return None
        stripped = raw.rstrip(b"\r\n")
        if stripped == b"":
            continue
        return stripped


def _looks_like_header(line: bytes) -> bool:
    """Return ``True`` if ``line`` is an LSP/MCP framing header line."""
    if b":" not in line:
        return False
    name, _, _ = line.partition(b":")
    return name.strip().lower() == CONTENT_LENGTH_HEADER.encode("ascii")


def _read_framed_body(stream: BinaryIO, first_header: bytes) -> str:
    """Read the rest of a Content-Length framed message after the first header.

    Reads any additional header lines until a blank line, then reads exactly
    ``Content-Length`` bytes for the body and decodes as UTF-8.
    """
    headers = [first_header]
    while True:
        raw = stream.readline()
        if raw == b"":
            break
        stripped = raw.rstrip(b"\r\n")
        if stripped == b"":
            break
        headers.append(stripped)

    length: int | None = None
    for header in headers:
        name, _, value = header.partition(b":")
        if name.strip().lower() == CONTENT_LENGTH_HEADER.encode("ascii"):
            try:
                length = int(value.strip())
            except ValueError as exc:
                raise ValueError(f"invalid Content-Length header: {value!r}") from exc
            break
    if length is None:
        raise ValueError("framed message missing Content-Length header")
    if length < 0:
        raise ValueError(f"negative Content-Length: {length}")

    body = _read_exact(stream, length)
    return body.decode("utf-8")


def _read_exact(stream: BinaryIO, length: int) -> bytes:
    """Read exactly ``length`` bytes from ``stream``.

    Raises ``EOFError`` if the stream closes early -- a framed peer that
    promises N bytes and delivers fewer is a wire protocol violation.
    """
    buf = bytearray()
    remaining = length
    while remaining > 0:
        chunk = stream.read(remaining)
        if not chunk:
            raise EOFError(
                f"unexpected EOF: expected {length} bytes, got {len(buf)}"
            )
        buf.extend(chunk)
        remaining = length - len(buf)
    return bytes(buf)


class LocalRpcServer:
    """Small JSON-RPC request handler backed only by striatum.api.invoke."""

    def __init__(self, *, repo: Path) -> None:
        self.repo = repo

    def handle_line(self, line: str) -> JsonObject | None:
        """Handle a single JSON-RPC request line."""
        try:
            request = json.loads(line)
        except json.JSONDecodeError as exc:
            return error_response(None, ERROR_PARSE, f"invalid JSON: {exc.msg}")
        if not isinstance(request, dict):
            return error_response(None, ERROR_INVALID_REQUEST, "JSON-RPC request must be an object")
        return self.handle_request(cast(JsonObject, request))

    def handle_request(self, request: JsonObject) -> JsonObject | None:
        """Handle a parsed JSON-RPC request object."""
        request_id = request.get("id")
        response_id = request_id if isinstance(request_id, str | int) or request_id is None else None
        if request.get("jsonrpc") != JSONRPC_VERSION:
            return error_response(response_id, ERROR_INVALID_REQUEST, "jsonrpc must be '2.0'")
        method = request.get("method")
        if not isinstance(method, str):
            return error_response(response_id, ERROR_INVALID_REQUEST, "method is required")
        params = request.get("params", {})
        if params is None:
            params = {}
        if not isinstance(params, dict):
            return error_response(response_id, ERROR_INVALID_PARAMS, "params must be an object")
        try:
            result = self.dispatch(method, cast(JsonObject, params))
        except ValueError as exc:
            return error_response(response_id, ERROR_INVALID_PARAMS, str(exc))
        except Exception as exc:  # pragma: no cover - defensive JSON-RPC boundary
            return error_response(response_id, ERROR_INTERNAL, str(exc))
        if "id" not in request:
            return None
        return {"jsonrpc": JSONRPC_VERSION, "id": response_id, "result": result}

    def dispatch(self, method: str, params: JsonObject) -> JsonObject:
        """Dispatch supported MCP-like methods."""
        if method == "initialize":
            return initialize_result()
        if method == "tools/list":
            return {"tools": tool_specs()}
        if method == "tools/call":
            return self.call_tool(params)
        if method == "resources/list":
            return {"resources": resource_specs()}
        if method == "resources/read":
            return self.read_resource(params)
        if method == "striatum/invoke":
            return self.invoke_args(params)
        raise ValueError(f"unknown method {method!r}")

    def call_tool(self, params: JsonObject) -> JsonObject:
        """Call a named tool by mapping arguments to a Striatum command."""
        name = required_string(params, "name")
        arguments = params.get("arguments", {})
        if not isinstance(arguments, dict):
            raise ValueError("arguments must be an object")
        result = call_tool(name=name, arguments=cast(JsonObject, arguments), repo=self.repo)
        return {
            "content": [{"type": "text", "text": json_dumps(result)}],
            "structuredContent": result,
            "isError": not bool(result.get("ok")),
        }

    def read_resource(self, params: JsonObject) -> JsonObject:
        """Read a local Striatum resource through the command API."""
        uri = required_string(params, "uri")
        result = read_resource(uri=uri, repo=self.repo)
        return {"contents": [{"uri": uri, "mimeType": "application/json", "text": json_dumps(result)}]}

    def invoke_args(self, params: JsonObject) -> JsonObject:
        """Invoke raw Striatum command args through the local API."""
        args_value = params.get("args")
        if not isinstance(args_value, list) or not all(isinstance(item, str) for item in args_value):
            raise ValueError("args must be a list of strings")
        repo = Path(str(params.get("repo", self.repo)))
        return invoke(cast(list[str], args_value), repo=repo)


class DaemonRpcServer:
    """Daemon MCP surface.

    Without a PostgreSQL connection this preserves the RFC 0028 resources-only
    behavior. With a connection, RFC 0032 mutation tools are exposed through
    the daemon RPC method registry and capability checks.
    """

    def __init__(self, *, pg_conn: Any | None = None, repo_root: Path | None = None, substrate_schema: int = 1) -> None:
        self.pg_conn = pg_conn
        self.repo_root = repo_root
        self.substrate_schema = substrate_schema

    def handle_request(self, request: JsonObject) -> JsonObject | None:
        request_id = request.get("id")
        response_id = request_id if isinstance(request_id, str | int) or request_id is None else None
        if request.get("jsonrpc") != JSONRPC_VERSION:
            return error_response(response_id, ERROR_INVALID_REQUEST, "jsonrpc must be '2.0'")
        method = request.get("method")
        if not isinstance(method, str):
            return error_response(response_id, ERROR_INVALID_REQUEST, "method is required")
        params = request.get("params", {})
        if params is None:
            params = {}
        if not isinstance(params, dict):
            return error_response(response_id, ERROR_INVALID_PARAMS, "params must be an object")
        try:
            if method == "initialize":
                result = initialize_result()
            elif method == "tools/list":
                result = {"tools": self.daemon_tool_specs(cast(JsonObject, params))}
            elif method == "tools/call":
                if self.pg_conn is None:
                    return error_response(response_id, ERROR_METHOD_NOT_FOUND, f"unknown method {method!r}")
                result = self.call_daemon_tool(cast(JsonObject, params))
            elif method == "resources/list":
                from striatum.daemon import daemon_mcp_resources

                token_value = params.get("token")
                if token_value is not None and not isinstance(token_value, str):
                    raise ValueError("token must be a string")
                result = {"resources": daemon_mcp_resources(token=token_value)}
            elif method == "resources/read":
                result = self.read_resource(cast(JsonObject, params))
            else:
                return error_response(response_id, ERROR_METHOD_NOT_FOUND, f"unknown method {method!r}")
        except ValueError as exc:
            return error_response(response_id, ERROR_INVALID_PARAMS, str(exc))
        except Exception as exc:  # pragma: no cover - defensive JSON-RPC boundary
            return error_response(response_id, ERROR_INTERNAL, str(exc))
        if "id" not in request:
            return None
        return {"jsonrpc": JSONRPC_VERSION, "id": response_id, "result": result}

    def read_resource(self, params: JsonObject) -> JsonObject:
        uri = required_string(params, "uri")
        token_value = params.get("token")
        if token_value is not None and not isinstance(token_value, str):
            raise ValueError("token must be a string")
        from striatum.daemon import daemon_mcp_read_resource

        result = daemon_mcp_read_resource(uri, token=token_value)
        return {"contents": [{"uri": uri, "mimeType": "application/json", "text": json_dumps(result)}]}

    def daemon_tool_specs(self, params: JsonObject) -> list[JsonObject]:
        if self.pg_conn is None:
            return []
        from striatum.daemon_rpc.capability import authorize
        from striatum.daemon_rpc.registry import METHOD_REGISTRY

        token_value = params.get("token")
        if token_value is not None and not isinstance(token_value, str):
            raise ValueError("token must be a string")
        repository_id = _optional_string(params, "repository_id")
        tools: list[JsonObject] = []
        for entry in sorted(METHOD_REGISTRY.values(), key=lambda item: item.method):
            if entry.required_capability is None or entry.method.startswith("daemon."):
                continue
            auth = authorize(
                self.pg_conn,
                required=entry.required_capability,
                repository_id=repository_id if entry.effective_repository_scope_mode == "single_repo" else None,
                token=token_value,
            )
            if auth.decision == "allowed":
                tools.append(
                    {
                        "name": entry.method,
                        "description": f"Invoke daemon RPC method `{entry.method}`.",
                        "required_capability": entry.required_capability,
                        "repository_scope_mode": entry.effective_repository_scope_mode,
                    }
                )
        return tools

    def call_daemon_tool(self, params: JsonObject) -> JsonObject:
        if self.pg_conn is None:
            raise ValueError("daemon MCP mutation tools require daemon PostgreSQL")
        from striatum.daemon_pg.mcp_dispatch import dispatch_mcp_tool_call

        name = required_string(params, "name")
        arguments_value = params.get("arguments", {})
        if not isinstance(arguments_value, dict):
            raise ValueError("arguments must be an object")
        arguments = cast(JsonObject, arguments_value)
        request_id = str(params.get("request_id") or arguments.get("request_id") or f"mcp_{uuid.uuid4().hex}")
        token_value = params.get("token", arguments.get("token"))
        if token_value is not None and not isinstance(token_value, str):
            raise ValueError("token must be a string")
        return dispatch_mcp_tool_call(
            pg_conn=self.pg_conn,
            name=name,
            arguments=arguments,
            token=token_value,
            request_id=request_id,
            repo_root=self.repo_root,
            substrate_schema=self.substrate_schema,
        )


def initialize_result() -> JsonObject:
    """Return a small MCP-compatible initialize payload."""
    return {
        "protocolVersion": "2024-11-05",
        "serverInfo": {"name": SERVER_NAME, "version": SERVER_VERSION},
        "capabilities": {"tools": {}, "resources": {}},
    }


def tool_specs() -> list[JsonObject]:
    """Return simple tool descriptors."""
    return [{"name": name, "description": f"Invoke `striatum {' '.join(argv)}`."} for name, argv in sorted(TOOL_ARGV.items())]


def resource_specs() -> list[JsonObject]:
    """Return read-only local resource descriptors."""
    return [
        {"uri": "striatum://status", "name": "status", "mimeType": "application/json"},
        {"uri": "striatum://doctor", "name": "doctor", "mimeType": "application/json"},
    ]


def call_tool(*, name: str, arguments: JsonObject, repo: Path) -> JsonObject:
    """Map a tool call to existing Striatum command arguments."""
    if name not in TOOL_ARGV:
        raise ValueError(f"unknown tool {name!r}")
    return invoke(build_args(name=name, arguments=arguments), repo=repo)


def build_args(*, name: str, arguments: JsonObject) -> list[str]:
    """Build CLI-style arguments for a named tool."""
    argv: list[str] = []
    for token in TOOL_ARGV[name]:
        if token.startswith("$"):
            argv.append(required_string(arguments, token[1:]))
        else:
            argv.append(token)
    for key, flag in OPTIONAL_ARGS.get(name, []):
        value = arguments.get(key)
        if value is None:
            continue
        if key == "capability":
            if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
                raise ValueError("capability must be a list of strings")
            for item in value:
                argv.extend([flag, item])
        elif key == "body_json" and isinstance(value, dict):
            argv.extend([flag, json_dumps(value)])
        else:
            argv.extend([flag, str(value)])
    if name == "send":
        body = arguments.get("body")
        if "body_json" not in arguments and isinstance(body, dict):
            argv.extend(["--body-json", json_dumps(body)])
    for key, flag in BOOLEAN_FLAGS.get(name, []):
        if bool(arguments.get(key)):
            argv.append(flag)
    return argv


def read_resource(*, uri: str, repo: Path) -> JsonObject:
    """Read a resource by invoking an existing read command."""
    parsed = urlparse(uri)
    if parsed.scheme != "striatum":
        raise ValueError("resource URI must use the striatum scheme")
    query = parse_qs(parsed.query)
    host = parsed.netloc
    path_parts = [part for part in parsed.path.split("/") if part]
    if host == "status" and not path_parts:
        args = ["status", *optional_run_id(query)]
    elif host == "doctor" and not path_parts:
        args = ["doctor", *optional_run_id(query)]
    elif host == "why" and len(path_parts) == 1:
        args = ["why", path_parts[0]]
    else:
        raise ValueError(f"unsupported resource URI {uri!r}")
    return invoke(args, repo=repo)


def optional_run_id(query: dict[str, list[str]]) -> list[str]:
    """Return optional --run-id args from a URI query."""
    values = query.get("run_id", [])
    if not values:
        return []
    return ["--run-id", values[0]]


def _optional_string(values: JsonObject, key: str) -> str | None:
    value = values.get(key)
    if value is None:
        return None
    if not isinstance(value, str | int | float | bool):
        raise ValueError(f"argument {key!r} must be a scalar value")
    return str(value)


def _daemon_tool_result(
    name: str, *, ok: bool, audit_id: int | None, error: str | None = None
) -> JsonObject:
    result: JsonObject = {
        "content": [{"type": "text", "text": name}],
        "structuredContent": {
            "ok": ok,
            "method": name,
            "audit_id": audit_id,
        },
        "isError": not ok,
    }
    if error is not None:
        cast(JsonObject, result["structuredContent"])["error"] = error
    return result


def required_string(values: JsonObject, key: str) -> str:
    """Read a required string-ish argument."""
    value = values.get(key)
    if value is None:
        raise ValueError(f"missing required argument {key!r}")
    if not isinstance(value, str | int | float | bool):
        raise ValueError(f"argument {key!r} must be a scalar value")
    return str(value)


def error_response(request_id: JsonRpcId, code: int, message: str) -> JsonObject:
    """Return a JSON-RPC error response."""
    return {"jsonrpc": JSONRPC_VERSION, "id": request_id, "error": {"code": code, "message": message}}


if __name__ == "__main__":
    raise SystemExit(main())
