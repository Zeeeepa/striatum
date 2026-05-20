"""Interactive loop wrapper for non-native CLIs (RFC 0049).

This script:
1. Long-polls work.await_packet.
2. If work is claimed, executes the wrapped command in a subprocess, feeding the packet JSON to stdin.
3. Automatically marks the work complete (work.complete) on exit code 0.
4. Releases the lease (work.release) on non-zero exit codes.
"""

from __future__ import annotations

import argparse
import json
import os
import socket
import subprocess
import sys
import time
import uuid
from pathlib import Path
from typing import Any

from striatum.cli.daemon_required import resolve_socket_path
from striatum.daemon_runtime import read_runtime_token


class RpcError(Exception):
    def __init__(self, code: str, message: str, details: Any = None) -> None:
        super().__init__(f"{code}: {message}")
        self.code = code
        self.message = message
        self.details = details or {}


def call_daemon(method: str, params: dict[str, Any], socket_path: Path) -> dict[str, Any]:
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
        sock.settimeout(60.0)  # Wait longer since await_packet is long-polling
        sock.connect(str(socket_path))
        stream = sock.makefile("rwb")
        
        # Handshake
        hello = {
            "jsonrpc": "2.0",
            "schema_version": 1,
            "request_id": uuid.uuid4().hex,
            "method": "daemon.hello",
            "params": {
                "client": {
                    "name": "striatum-mcp-loop-wrapper",
                    "version": "0.1.0",
                    "supported_envelope": [1],
                    "supported_framings": ["json"],
                }
            }
        }
        stream.write(json.dumps(hello).encode("utf-8") + b"\n")
        stream.flush()
        hello_resp_line = stream.readline()
        hello_resp = json.loads(hello_resp_line.decode("utf-8"))
        if not hello_resp.get("ok"):
            raw_error = hello_resp.get("data") or {}
            raise RpcError(
                raw_error.get("code", "version_incompatible"),
                raw_error.get("message", "daemon.hello refused"),
            )
        
        # Call actual method
        capability_token = read_runtime_token()
        envelope: dict[str, Any] = {
            "schema_version": 1,
            "request_id": uuid.uuid4().hex,
            "method": method,
            "params": params,
        }
        if capability_token:
            envelope["capability_token"] = capability_token
            
        stream.write(json.dumps(envelope).encode("utf-8") + b"\n")
        stream.flush()
        resp_line = stream.readline()
        resp = json.loads(resp_line.decode("utf-8"))
        if not resp.get("ok"):
            raw_error = resp.get("data") or {}
            raise RpcError(
                raw_error.get("code", "rpc_error"),
                raw_error.get("message", "RPC call failed"),
                details=raw_error.get("details"),
            )
        return resp.get("data") or {}


def main() -> int:
    parser = argparse.ArgumentParser(description="Wrap a non-interactive CLI in an MCP interactive loop")
    parser.add_argument("command", nargs="+", help="The command to execute for each packet")
    args = parser.parse_args()

    session_id = os.environ.get("STRIATUM_SESSION_ID")
    run_id = os.environ.get("STRIATUM_RUN_ID")
    repo_root = os.environ.get("STRIATUM_REPO")

    if not session_id or not run_id or not repo_root:
        print("Error: STRIATUM_SESSION_ID, STRIATUM_RUN_ID, and STRIATUM_REPO must be in environment.", file=sys.stderr)
        return 1

    socket_path = resolve_socket_path()
    print(f"Starting MCP loop wrapper for session {session_id} on run {run_id}", flush=True)
    print(f"Wrapped command: {args.command}", flush=True)

    while True:
        try:
            # Check if run is still running first
            run_summary = call_daemon(
                "run.summary",
                {"repository_id": repo_root, "run_id": run_id},
                socket_path
            )
            run_state = run_summary.get("run", {}).get("state")
            if run_state != "running":
                print(f"Run state is {run_state}, exiting interactive loop.", flush=True)
                break

            print("Waiting for packet...", flush=True)
            res = call_daemon(
                "work.await_packet",
                {
                    "repository_id": repo_root,
                    "session_id": session_id,
                    "lease_seconds": 3600,
                },
                socket_path
            )

            status = res.get("status")
            if status == "no_work":
                time.sleep(1.0)
                continue

            if status != "claimed":
                print(f"Unexpected await_packet status: {status}", flush=True)
                time.sleep(1.0)
                continue

            packet = res.get("packet")
            if not packet:
                print("Claimed response is missing packet field", flush=True)
                continue

            packet_id = res.get("packet_id")
            lease_id = res.get("lease_id")
            job_id = packet.get("job", {}).get("job_id")

            print(f"Claimed packet {packet_id} (job {job_id}). Invoking wrapped command...", flush=True)

            # Spawn subprocess
            proc = subprocess.Popen(
                args.command,
                stdin=subprocess.PIPE,
                stdout=sys.stdout,
                stderr=sys.stderr,
                text=True,
                cwd=repo_root,
            )
            # Write packet JSON to stdin
            proc.communicate(input=json.dumps(packet))

            if proc.returncode == 0:
                print(f"Command succeeded. Completing job {job_id}...", flush=True)
                call_daemon(
                    "work.complete",
                    {
                        "repository_id": repo_root,
                        "session_id": session_id,
                        "job_id": job_id,
                        "lease_id": lease_id,
                        "summary": "Completed via mcp_loop_wrapper",
                    },
                    socket_path
                )
            else:
                print(f"Command failed with code {proc.returncode}. Releasing job {job_id}...", flush=True)
                call_daemon(
                    "work.release",
                    {
                        "repository_id": repo_root,
                        "session_id": session_id,
                        "lease_id": lease_id,
                        "reason": f"Wrapped command failed with exit code {proc.returncode}",
                    },
                    socket_path
                )

        except RpcError as exc:
            print(f"RPC Error: {exc}", file=sys.stderr, flush=True)
            time.sleep(2.0)
        except Exception as exc:
            print(f"Unexpected error: {exc}", file=sys.stderr, flush=True)
            time.sleep(2.0)

    return 0


if __name__ == "__main__":
    sys.exit(main())
