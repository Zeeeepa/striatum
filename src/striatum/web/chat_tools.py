"""RFC 0023 V1.5: closed-set read-only tools for the chat surface.

Six tools, all read-only:

- ``read_file(path)`` — read a repo-relative file.
- ``list_dir(path)`` — list directory entries.
- ``striatum_status(run_id?)`` — current run state.
- ``striatum_why(target_id)`` — investigate a job/artifact/blocker.
- ``git_log(limit?)`` — recent commits.
- ``git_diff(path?)`` — working-tree diff.

The closed set is enforced in :func:`execute_tool`; unknown tool names
return an error string rather than executing. Path safety mirrors the
``/view/<path>`` route handler. All tool results are wrapped in
``BEGIN/END`` delimiters per design-review F1 so the system prompt can
instruct the model to treat content between the delimiters as data,
not instructions (prompt-injection defense in depth).
"""

from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any

__all__ = [
    "ANTHROPIC_TOOLS",
    "OPENAI_TOOLS",
    "TOOL_NAMES",
    "execute_tool",
    "wrap_tool_result",
]


# Result size caps (per design-review F2).
READ_FILE_MAX_BYTES = 64 * 1024
GIT_DIFF_MAX_BYTES = 64 * 1024
LIST_DIR_MAX_ENTRIES = 1000
GIT_LOG_MAX_LIMIT = 50
GIT_LOG_DEFAULT_LIMIT = 10
STATUS_TIMEOUT_SECONDS = 15.0
GIT_TIMEOUT_SECONDS = 15.0


# Flavor-neutral tool schemas. Each dict captures everything both
# Anthropic Messages and OpenAI Chat need; flavor-specific shapes are
# adapted below.
_TOOLS: list[dict[str, Any]] = [
    {
        "name": "read_file",
        "description": (
            "Read the contents of a file in the operator's repository. "
            "Path must be repo-relative; .git/ and .striatum/ are hidden. "
            "Result is capped at 64 KB; longer files are truncated with a marker."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Repo-relative path to the file."},
            },
            "required": ["path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "list_dir",
        "description": (
            "List entries in a directory, repo-relative. .git/ and .striatum/ are hidden. "
            "Result is capped at 1000 entries."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Repo-relative directory path. Use '.' for the repo root."},
            },
            "required": ["path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "striatum_status",
        "description": (
            "Return the current striatum run state. Pass run_id for a "
            "single run; omit for all-runs status."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "run_id": {"type": "string", "description": "Optional run id."},
            },
            "additionalProperties": False,
        },
    },
    {
        "name": "striatum_why",
        "description": (
            "Investigate a striatum target (job, artifact, blocker, run) by id."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "target_id": {"type": "string", "description": "The id to investigate."},
            },
            "required": ["target_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "git_log",
        "description": "Recent git commits. Default 10; max 50.",
        "parameters": {
            "type": "object",
            "properties": {
                "limit": {"type": "integer", "minimum": 1, "maximum": 50, "default": 10},
            },
            "additionalProperties": False,
        },
    },
    {
        "name": "git_diff",
        "description": "Working-tree diff. Pass path to scope to one file/dir; omit for full diff. Capped at 64 KB.",
        "parameters": {
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Optional repo-relative path."},
            },
            "additionalProperties": False,
        },
    },
]


TOOL_NAMES: frozenset[str] = frozenset(t["name"] for t in _TOOLS)


# Flavor-specific adaptations.

ANTHROPIC_TOOLS: list[dict[str, Any]] = [
    {
        "name": t["name"],
        "description": t["description"],
        "input_schema": t["parameters"],
    }
    for t in _TOOLS
]


OPENAI_TOOLS: list[dict[str, Any]] = [
    {
        "type": "function",
        "function": {
            "name": t["name"],
            "description": t["description"],
            "parameters": t["parameters"],
        },
    }
    for t in _TOOLS
]


def wrap_tool_result(name: str, args: dict[str, Any], result: str) -> str:
    """Per design-review F1: wrap tool results in BEGIN/END delimiters
    so the model treats content between them as data, not instructions.

    The delimiter shape is intentionally distinctive and matches the
    instruction string in the chat briefing.
    """
    args_json = json.dumps(args, sort_keys=True, default=str)
    return (
        f"<tool_result_begin name={json.dumps(name)} args={json.dumps(args_json)}>\n"
        f"{result}\n"
        f"<tool_result_end name={json.dumps(name)}>"
    )


def execute_tool(name: str, args: dict[str, Any], *, repo: Path) -> str:
    """Closed-set dispatch. Returns the tool result as a string.

    On error: returns a short error string the model can reason about.
    Never raises; all exception paths produce an error-as-result.
    """
    if name not in TOOL_NAMES:
        return f"[error] unknown tool {name!r}; valid tools: {sorted(TOOL_NAMES)}"
    try:
        if name == "read_file":
            return _tool_read_file(repo, str(args.get("path", "")))
        if name == "list_dir":
            return _tool_list_dir(repo, str(args.get("path", ".")))
        if name == "striatum_status":
            run_id = args.get("run_id")
            return _tool_striatum_status(repo, str(run_id) if run_id else None)
        if name == "striatum_why":
            return _tool_striatum_why(repo, str(args.get("target_id", "")))
        if name == "git_log":
            limit_raw = args.get("limit", GIT_LOG_DEFAULT_LIMIT)
            try:
                limit = int(limit_raw)
            except (TypeError, ValueError):
                limit = GIT_LOG_DEFAULT_LIMIT
            return _tool_git_log(repo, limit)
        if name == "git_diff":
            path = args.get("path")
            return _tool_git_diff(repo, str(path) if path else None)
    except Exception as exc:  # noqa: BLE001
        return f"[error] {type(exc).__name__}: {exc}"
    return f"[error] tool {name!r} not implemented"


# --- Path safety -----------------------------------------------------


def _safe_resolve(repo: Path, rel: str) -> Path | None:
    """Resolve ``rel`` against ``repo``, refusing escapes and hidden roots.

    Returns the resolved Path, or None when refused.
    """
    if not isinstance(rel, str) or rel == "":
        return None
    if rel.startswith("/") or "\x00" in rel or ".." in Path(rel).parts:
        return None
    target = (repo / rel).resolve()
    repo_root = repo.resolve()
    try:
        target.relative_to(repo_root)
    except ValueError:
        return None
    parts = target.relative_to(repo_root).parts
    if parts and parts[0] in (".git", ".striatum"):
        return None
    return target


# --- Tool implementations -------------------------------------------


def _tool_read_file(repo: Path, rel: str) -> str:
    target = _safe_resolve(repo, rel)
    if target is None:
        return f"[error] path {rel!r} is outside repo or hidden"
    if not target.exists():
        return f"[error] path {rel!r} does not exist"
    if not target.is_file():
        return f"[error] path {rel!r} is not a regular file"
    try:
        raw = target.read_bytes()
    except OSError as exc:
        return f"[error] read failed: {type(exc).__name__}: {exc}"
    if b"\x00" in raw[:1024]:
        return f"[error] file {rel!r} appears to be binary"
    truncated = False
    if len(raw) > READ_FILE_MAX_BYTES:
        raw = raw[:READ_FILE_MAX_BYTES]
        truncated = True
    text = raw.decode("utf-8", errors="replace")
    if truncated:
        text += f"\n\n[truncated; file is {target.stat().st_size} bytes]"
    return text


def _tool_list_dir(repo: Path, rel: str) -> str:
    if rel in ("", "."):
        target = repo.resolve()
    else:
        resolved = _safe_resolve(repo, rel)
        if resolved is None:
            return f"[error] path {rel!r} is outside repo or hidden"
        target = resolved
    if not target.exists():
        return f"[error] path {rel!r} does not exist"
    if not target.is_dir():
        return f"[error] path {rel!r} is not a directory"
    try:
        entries = sorted(target.iterdir(), key=lambda p: (not p.is_dir(), p.name))
    except OSError as exc:
        return f"[error] list failed: {type(exc).__name__}: {exc}"
    lines: list[str] = []
    truncated = False
    for entry in entries:
        if entry.name in (".git", ".striatum"):
            continue
        if len(lines) >= LIST_DIR_MAX_ENTRIES:
            truncated = True
            break
        kind = "dir" if entry.is_dir() else "file"
        lines.append(f"{kind} {entry.name}")
    if truncated:
        lines.append(f"[truncated at {LIST_DIR_MAX_ENTRIES} entries]")
    if not lines:
        return "[empty directory]"
    return "\n".join(lines)


def _tool_striatum_status(repo: Path, run_id: str | None) -> str:
    from striatum.api import invoke

    argv = ["status"]
    if run_id:
        argv.extend(["--run-id", run_id])
    try:
        result = invoke(argv, repo=repo)
    except Exception as exc:  # noqa: BLE001
        return f"[error] status failed: {type(exc).__name__}: {exc}"
    return json.dumps(result, indent=2, default=str, sort_keys=True)[:READ_FILE_MAX_BYTES]


def _tool_striatum_why(repo: Path, target_id: str) -> str:
    from striatum.api import invoke

    if not target_id:
        return "[error] target_id is required"
    try:
        result = invoke(["why", target_id], repo=repo)
    except Exception as exc:  # noqa: BLE001
        return f"[error] why failed: {type(exc).__name__}: {exc}"
    return json.dumps(result, indent=2, default=str, sort_keys=True)[:READ_FILE_MAX_BYTES]


def _tool_git_log(repo: Path, limit: int) -> str:
    limit = max(1, min(GIT_LOG_MAX_LIMIT, limit))
    try:
        proc = subprocess.run(
            ["git", "log", f"-{limit}", "--oneline", "--no-color"],
            cwd=repo,
            check=False,
            capture_output=True,
            text=True,
            timeout=GIT_TIMEOUT_SECONDS,
        )
    except (subprocess.TimeoutExpired, FileNotFoundError) as exc:
        return f"[error] git log failed: {type(exc).__name__}: {exc}"
    if proc.returncode != 0:
        return f"[error] git log failed: {proc.stderr.strip()[:500]}"
    output = proc.stdout.strip()
    if not output:
        return "[no commits]"
    return output


def _tool_git_diff(repo: Path, rel: str | None) -> str:
    cmd = ["git", "diff", "--no-color"]
    if rel:
        resolved = _safe_resolve(repo, rel)
        if resolved is None:
            return f"[error] path {rel!r} is outside repo or hidden"
        cmd.append("--")
        cmd.append(str(resolved.relative_to(repo.resolve())))
    try:
        proc = subprocess.run(
            cmd, cwd=repo, check=False,
            capture_output=True, text=True,
            timeout=GIT_TIMEOUT_SECONDS,
        )
    except (subprocess.TimeoutExpired, FileNotFoundError) as exc:
        return f"[error] git diff failed: {type(exc).__name__}: {exc}"
    if proc.returncode != 0:
        return f"[error] git diff failed: {proc.stderr.strip()[:500]}"
    output = proc.stdout
    if not output:
        return "[no uncommitted changes]"
    truncated = False
    if len(output) > GIT_DIFF_MAX_BYTES:
        output = output[:GIT_DIFF_MAX_BYTES]
        truncated = True
    if truncated:
        output += f"\n\n[truncated at {GIT_DIFF_MAX_BYTES} bytes]"
    return output
