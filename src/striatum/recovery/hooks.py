"""RFC 0020 V1: escalation hook runners (marker_file, webhook, shell).

The runners are pure side-effect wrappers; they do not raise on
failure — they return a status dict the auto-sweep folds into the
sweep envelope's ``escalations[]`` list.
"""

from __future__ import annotations

import json
import subprocess
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, Mapping

from striatum.errors import WorkflowError


def run_marker_file_hook(
    *, target: Path, path: str, body: str
) -> dict[str, Any]:
    """Append a Markdown body to ``target/path``.

    Refuses paths inside ``.striatum/`` or outside the target tree.
    Mirrors the boundary ``evidence export`` enforces.
    """
    if path.startswith(".striatum/") or path.startswith("/"):
        raise WorkflowError(
            "marker_file path must be repo-relative and outside .striatum/"
        )
    if ".." in path.split("/"):
        raise WorkflowError("marker_file path may not contain '..'")
    abs_path = (target / path).resolve()
    target_resolved = target.resolve()
    if not str(abs_path).startswith(str(target_resolved)):
        raise WorkflowError("marker_file path resolves outside the target tree")
    abs_path.parent.mkdir(parents=True, exist_ok=True)
    with abs_path.open("a", encoding="utf-8") as fh:
        fh.write(body)
        if not body.endswith("\n"):
            fh.write("\n")
    return {"kind": "marker_file", "wrote": True, "path": str(abs_path)}


def run_webhook_hook(
    *, url: str, payload: Mapping[str, Any], timeout: float = 10.0
) -> dict[str, Any]:
    """POST ``payload`` as JSON to ``url``. Failures do not raise."""
    if not (url.startswith("http://") or url.startswith("https://")):
        return {"kind": "webhook", "wrote": False, "url": url, "error": "scheme"}
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return {
                "kind": "webhook",
                "wrote": True,
                "url": url,
                "status": int(getattr(resp, "status", 0) or 0),
            }
    except (urllib.error.HTTPError, urllib.error.URLError, OSError) as exc:
        return {"kind": "webhook", "wrote": False, "url": url, "error": str(exc)}


def run_shell_hook(
    *,
    command: list[str],
    env: Mapping[str, str] | None = None,
    cwd: Path | None = None,
    timeout: float = 30.0,
) -> dict[str, Any]:
    """Run ``command`` with stdout/stderr to DEVNULL (D028).

    No transcript capture; the hook is fire-and-record-exit-code.
    """
    if not command:
        return {"kind": "shell", "wrote": False, "error": "empty_command"}
    try:
        proc = subprocess.run(
            command,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            env=dict(env) if env else None,
            cwd=str(cwd) if cwd else None,
            timeout=timeout,
            check=False,
        )
    except (subprocess.TimeoutExpired, OSError) as exc:
        return {"kind": "shell", "wrote": False, "error": str(exc)}
    return {"kind": "shell", "wrote": True, "exit_code": int(proc.returncode)}
