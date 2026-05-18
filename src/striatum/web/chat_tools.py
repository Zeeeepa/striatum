"""RFC 0023 V1.5 / RFC 0036 / RFC 0040: closed-set tools for the chat surface.

Read-only tools (always available):

- ``read_file(path)`` — read a repo-relative file.
- ``list_dir(path)`` — list directory entries.
- ``striatum_status(run_id?)`` — current run state.
- ``striatum_why(target_id)`` — investigate a job/artifact/blocker.
- ``git_log(limit?)`` — recent commits.
- ``git_diff(path?)`` — working-tree diff.
- ``list_workflows()`` — every workflow.json in the repo.
- ``generate_workflow_preview(spec)`` — preview a generated workflow.
- ``run_summary(run_id, path)`` — export a JSON run summary.
- ``evidence_export(run_id, path)`` — export the evidence bundle.

Mutating tools (require ``--allow-mutations``):

- ``generate_workflow_write(spec, confirm_write)`` — RFC 0036 V1.
- RFC 0040 dogfood-lifecycle tools: ``run_prepare``, ``run_start``,
  ``register_session``, ``supervise_start``, ``claim_next``, ``ack``,
  ``publish_artifact``, ``verdict``, ``complete``, ``supervise_stop``.

The closed set is enforced in :func:`execute_tool`; unknown tool names
return an error string rather than executing. Path safety mirrors the
``/view/<path>`` route handler. All tool results are wrapped in
``BEGIN/END`` delimiters per design-review F1 so the system prompt can
instruct the model to treat content between the delimiters as data,
not instructions (prompt-injection defense in depth).

Dogfood-lifecycle tools are thin shells over the existing CLI-shaped
verbs and route mapped daemon methods through daemon RPC. Per RFC 0040 §1
each chat-tool entry reuses the existing capability bound to its underlying
RPC method; the chat surface's mutation gate (the ``mutation`` field on each
entry plus ``--allow-mutations`` at the service) is the local-MCP equivalent
of the daemon's capability-token filtering.
"""

from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any

__all__ = [
    "ANTHROPIC_TOOLS",
    "DOGFOOD_LIFECYCLE_TOOL_NAMES",
    "OPENAI_TOOLS",
    "TOOL_NAMES",
    "execute_tool",
    "tool_names",
    "tool_schemas",
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
    {
        "name": "list_workflows",
        "description": (
            "List every workflow.json file in the operator's repo with "
            "validation status, job count, and workflow_id. Capped at 100 entries."
        ),
        "parameters": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    },
    {
        "name": "generate_workflow_preview",
        "description": (
            "Preview a generated Striatum workflow from a WorkflowGenerationSpec. "
            "Writes nothing and returns workflow JSON, files, graph metadata, "
            "warnings, and validation."
        ),
        "mutation": False,
        "parameters": {
            "type": "object",
            "properties": {
                "spec": {
                    "type": "object",
                    "description": "WorkflowGenerationSpec JSON object with schema_version striatum.workflow_generator.v1.",
                },
            },
            "required": ["spec"],
            "additionalProperties": False,
        },
    },
    {
        "name": "generate_workflow_write",
        "description": (
            "Write generated workflow files after preview. Requires confirm_write: true, "
            "--allow-mutations, and a separate operator confirmation gesture in the chat UI."
        ),
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "spec": {
                    "type": "object",
                    "description": "WorkflowGenerationSpec JSON object with schema_version striatum.workflow_generator.v1.",
                },
                "confirm_write": {
                    "type": "boolean",
                    "description": "Must be true. Necessary but not sufficient; the chat UI also requires an operator confirmation gesture.",
                },
            },
            "required": ["spec", "confirm_write"],
            "additionalProperties": False,
        },
    },
    # RFC 0040 V1: dogfood-lifecycle chat tools.
    {
        "name": "run_prepare",
        "description": (
            "Prepare a workflow run from a repo-relative workflow.json path. "
            "Wraps `striatum run prepare`. State-mutating; requires --allow-mutations."
        ),
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "workflow_path": {
                    "type": "string",
                    "description": "Repo-relative path to the workflow.json file.",
                },
            },
            "required": ["workflow_path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "run_start",
        "description": (
            "Start a previously-prepared run. Wraps `striatum run start --run-id`."
        ),
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "run_id": {"type": "string", "description": "Run id returned by run_prepare."},
            },
            "required": ["run_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "register_session",
        "description": (
            "Register an agent session against a run. Wraps `striatum register-session`."
        ),
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "run_id": {"type": "string"},
                "role": {"type": "string", "description": "Role id (author/reviewer/...)."},
                "lane": {"type": "string", "description": "Lane id from the workflow."},
                "fresh": {"type": "boolean", "default": True},
                "parent_session_id": {"type": "string"},
                "operator_label": {"type": "string"},
                "capabilities": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Optional capability list (write/review/...).",
                },
            },
            "required": ["run_id", "role", "lane"],
            "additionalProperties": False,
        },
    },
    {
        "name": "supervise_start",
        "description": (
            "Start the supervised wrapper for a registered session. "
            "Wraps `striatum supervise start --session-id`."
        ),
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
            },
            "required": ["session_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "claim_next",
        "description": (
            "Claim the next work packet for a session. Wraps `striatum claim-next`."
        ),
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "lease_seconds": {
                    "type": "integer",
                    "minimum": 60,
                    "maximum": 7200,
                    "default": 1800,
                },
            },
            "required": ["session_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "ack",
        "description": "Acknowledge a claimed packet. Wraps `striatum ack`.",
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "message_id": {"type": "string"},
                "lease_id": {"type": "string"},
            },
            "required": ["session_id", "message_id", "lease_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "publish_artifact",
        "description": (
            "Publish a job artifact. Wraps `striatum publish-artifact`. The "
            "operator-on-behalf composite is documented in RFC 0040; V1 keeps "
            "the primitive chat-tool shape and lets the operator chain "
            "publish + complete or publish + verdict."
        ),
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "job_id": {"type": "string"},
                "lease_id": {"type": "string"},
                "kind": {"type": "string"},
                "logical_name": {"type": "string"},
                "path": {"type": "string"},
            },
            "required": [
                "session_id",
                "job_id",
                "lease_id",
                "kind",
                "logical_name",
                "path",
            ],
            "additionalProperties": False,
        },
    },
    {
        "name": "verdict",
        "description": "Record a reviewer verdict. Wraps `striatum verdict`.",
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "job_id": {"type": "string"},
                "lease_id": {"type": "string"},
                "verdict": {
                    "type": "string",
                    "enum": [
                        "accept",
                        "accept_with_findings",
                        "needs_revision",
                        "reject",
                    ],
                },
                "findings_artifact_id": {"type": "string"},
                "rationale": {"type": "string"},
            },
            "required": ["session_id", "job_id", "lease_id", "verdict"],
            "additionalProperties": False,
        },
    },
    {
        "name": "complete",
        "description": "Complete a job. Wraps `striatum complete`.",
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "job_id": {"type": "string"},
                "lease_id": {"type": "string"},
                "summary": {"type": "string"},
            },
            "required": ["session_id", "job_id", "lease_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "supervise_stop",
        "description": "Stop a supervised wrapper. Wraps `striatum supervise stop`.",
        "mutation": True,
        "parameters": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "reason": {"type": "string"},
            },
            "required": ["session_id", "reason"],
            "additionalProperties": False,
        },
    },
    {
        "name": "run_summary",
        "description": (
            "Export a JSON run summary to a repo-relative path. "
            "Wraps `striatum run summary`. Read-shaped: not gated."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "run_id": {"type": "string"},
                "path": {"type": "string", "description": "Repo-relative destination path."},
            },
            "required": ["run_id", "path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "evidence_export",
        "description": (
            "Export the evidence bundle for a run. Wraps `striatum evidence export`. "
            "Read-shaped: not gated."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "run_id": {"type": "string"},
                "path": {"type": "string", "description": "Repo-relative destination path."},
            },
            "required": ["run_id", "path"],
            "additionalProperties": False,
        },
    },
]


# RFC 0040 V1: dogfood-lifecycle chat-tool names (kept as a separate
# constant so tests and docs can refer to the closed set explicitly).
DOGFOOD_LIFECYCLE_TOOL_NAMES: frozenset[str] = frozenset({
    "run_prepare",
    "run_start",
    "register_session",
    "supervise_start",
    "claim_next",
    "ack",
    "publish_artifact",
    "verdict",
    "complete",
    "supervise_stop",
    "run_summary",
    "evidence_export",
})


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


def tool_names(*, allow_mutations: bool) -> frozenset[str]:
    """Return the effective chat tool set for a service mutation posture."""
    return frozenset(
        str(t["name"])
        for t in _TOOLS
        if allow_mutations or not bool(t.get("mutation"))
    )


def tool_schemas(*, allow_mutations: bool, flavor: str) -> list[dict[str, Any]]:
    """Return provider-shaped schemas, hiding mutating tools when disabled."""
    selected = [t for t in _TOOLS if allow_mutations or not bool(t.get("mutation"))]
    if flavor == "anthropic_messages":
        return [
            {
                "name": t["name"],
                "description": t["description"],
                "input_schema": t["parameters"],
            }
            for t in selected
        ]
    return [
        {
            "type": "function",
            "function": {
                "name": t["name"],
                "description": t["description"],
                "parameters": t["parameters"],
            },
        }
        for t in selected
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


def execute_tool(
    name: str,
    args: dict[str, Any],
    *,
    repo: Path,
    allow_mutations: bool = True,
    operator_confirmed: bool = False,
) -> str:
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
        if name == "list_workflows":
            return _tool_list_workflows(repo)
        if name == "generate_workflow_preview":
            spec = args.get("spec")
            return _tool_generate_workflow_preview(repo, spec if isinstance(spec, dict) else None)
        if name == "generate_workflow_write":
            spec = args.get("spec")
            return _tool_generate_workflow_write(
                repo,
                spec if isinstance(spec, dict) else None,
                confirm_write=args.get("confirm_write") is True,
                allow_mutations=allow_mutations,
                operator_confirmed=operator_confirmed,
            )
        if name in DOGFOOD_LIFECYCLE_TOOL_NAMES:
            return _tool_dogfood_lifecycle(
                name, args, repo=repo, allow_mutations=allow_mutations
            )
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
    from striatum.service_daemon import invoke_argv_through_daemon_or_api

    argv = ["status"]
    if run_id:
        argv.extend(["--run-id", run_id])
    try:
        result = invoke_argv_through_daemon_or_api(argv, repo=repo)
    except Exception as exc:  # noqa: BLE001
        return f"[error] status failed: {type(exc).__name__}: {exc}"
    return json.dumps(result, indent=2, default=str, sort_keys=True)[:READ_FILE_MAX_BYTES]


def _tool_striatum_why(repo: Path, target_id: str) -> str:
    from striatum.service_daemon import invoke_argv_through_daemon_or_api

    if not target_id:
        return "[error] target_id is required"
    try:
        result = invoke_argv_through_daemon_or_api(["why", target_id], repo=repo)
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


def _tool_list_workflows(repo: Path) -> str:
    from striatum.web.workflows import discover

    items = discover(repo)
    if not items:
        return "[no workflow.json files found]"
    lines: list[str] = []
    for item in items[:100]:
        path = str(item.get("path", ""))
        status = str(item.get("status", "?"))
        job_count = item.get("job_count", 0)
        workflow_id = str(item.get("workflow_id") or "")
        lines.append(f"{status:<14} {path:<60} jobs={job_count:<3} {workflow_id}")
    if len(items) > 100:
        lines.append(f"[truncated at 100; {len(items)} total]")
    return "\n".join(lines)


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


def _tool_generate_workflow_preview(repo: Path, spec_body: dict[str, Any] | None) -> str:
    if spec_body is None:
        return "[error] missing spec object"
    from striatum import service_daemon

    try:
        data = service_daemon.call_repo_method(repo, "workflow.generate.preview", {"spec": spec_body})
    except service_daemon.ServiceDaemonRpcError as exc:
        return _service_daemon_error(exc)
    return json.dumps({"ok": True, "data": data}, indent=2, sort_keys=True)


def _tool_generate_workflow_write(
    repo: Path,
    spec_body: dict[str, Any] | None,
    *,
    confirm_write: bool,
    allow_mutations: bool,
    operator_confirmed: bool,
) -> str:
    from striatum.workflow_generator import GeneratorError, WorkflowGenerationSpec, generate_workflow
    from striatum.workflow_generator.write import write_generated_workflow

    if not allow_mutations:
        return (
            "[error] mutations_disabled: service started without --allow-mutations; "
            "ask the operator to restart with --allow-mutations before writing workflows"
        )
    if not confirm_write:
        return "[error] confirm_write_missing: generate_workflow_write requires confirm_write: true"
    if not operator_confirmed:
        return "[error] operator_gesture_missing: chat workflow writes require an operator confirmation gesture"
    if spec_body is None:
        return "[error] missing spec object"
    try:
        spec = WorkflowGenerationSpec.from_json(spec_body)
        generated = generate_workflow(spec)
        written = write_generated_workflow(generated, repo=repo)
    except GeneratorError as exc:
        return _generator_error("workflow_generate_write_failed", exc)
    return json.dumps({"ok": True, "data": written}, indent=2, sort_keys=True)


# --- RFC 0040 V1: dogfood-lifecycle dispatch ------------------------


# Tools that mutate runner state. Held in a tuple so the mapping below
# is easy to inspect from tests.
_DOGFOOD_MUTATING: frozenset[str] = frozenset({
    "run_prepare",
    "run_start",
    "register_session",
    "supervise_start",
    "claim_next",
    "ack",
    "publish_artifact",
    "verdict",
    "complete",
    "supervise_stop",
})


def _tool_dogfood_lifecycle(
    name: str,
    args: dict[str, Any],
    *,
    repo: Path,
    allow_mutations: bool,
) -> str:
    """Thin shell over the existing CLI-shaped dogfood-lifecycle verb."""
    if name in _DOGFOOD_MUTATING and not allow_mutations:
        return (
            "[error] mutations_disabled: service started without "
            "--allow-mutations; restart with --allow-mutations to use "
            f"the {name} dogfood-lifecycle tool"
        )
    try:
        argv = _dogfood_argv(name, args)
    except _ArgError as exc:
        return f"[error] {exc.code}: {exc}"
    from striatum.service_daemon import invoke_argv_through_daemon_or_api

    result = invoke_argv_through_daemon_or_api(argv, repo=repo)
    return json.dumps(result, indent=2, sort_keys=True, default=str)


class _ArgError(Exception):
    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code


def _require_str(args: dict[str, Any], key: str) -> str:
    value = args.get(key)
    if not isinstance(value, str) or not value:
        raise _ArgError("missing_argument", f"argument {key!r} is required (non-empty string)")
    return value


def _optional_str(args: dict[str, Any], key: str) -> str | None:
    value = args.get(key)
    if value is None:
        return None
    if not isinstance(value, str) or not value:
        raise _ArgError("invalid_argument", f"argument {key!r} must be a non-empty string when supplied")
    return value


def _dogfood_argv(name: str, args: dict[str, Any]) -> list[str]:
    if name == "run_prepare":
        return ["run", "prepare", "--workflow", _require_str(args, "workflow_path")]
    if name == "run_start":
        return ["run", "start", "--run-id", _require_str(args, "run_id")]
    if name == "register_session":
        argv = [
            "register-session",
            "--run-id", _require_str(args, "run_id"),
            "--role", _require_str(args, "role"),
            "--lane", _require_str(args, "lane"),
        ]
        # Default `fresh` to True (matches RFC 0040 §1 tool signature).
        fresh = args.get("fresh")
        if fresh is None or fresh is True:
            argv.append("--fresh")
        elif fresh is not False:
            raise _ArgError("invalid_argument", "argument 'fresh' must be a boolean")
        parent = _optional_str(args, "parent_session_id")
        if parent is not None:
            argv.extend(["--parent-session-id", parent])
        label = _optional_str(args, "operator_label")
        if label is not None:
            argv.extend(["--operator-label", label])
        capabilities = args.get("capabilities")
        if capabilities is not None:
            if not isinstance(capabilities, list) or not all(isinstance(c, str) and c for c in capabilities):
                raise _ArgError("invalid_argument", "argument 'capabilities' must be a list of non-empty strings")
            for cap in capabilities:
                argv.extend(["--capability", cap])
        return argv
    if name == "supervise_start":
        return ["supervise", "start", "--session-id", _require_str(args, "session_id")]
    if name == "claim_next":
        argv = ["claim-next", "--session-id", _require_str(args, "session_id")]
        lease = args.get("lease_seconds")
        if lease is not None:
            if not isinstance(lease, int) or lease < 60:
                raise _ArgError("invalid_argument", "argument 'lease_seconds' must be an integer >= 60")
            argv.extend(["--lease-seconds", str(lease)])
        return argv
    if name == "ack":
        return [
            "ack",
            "--session-id", _require_str(args, "session_id"),
            "--message-id", _require_str(args, "message_id"),
            "--lease-id", _require_str(args, "lease_id"),
        ]
    if name == "publish_artifact":
        return [
            "publish-artifact",
            "--session-id", _require_str(args, "session_id"),
            "--job-id", _require_str(args, "job_id"),
            "--lease-id", _require_str(args, "lease_id"),
            "--kind", _require_str(args, "kind"),
            "--logical-name", _require_str(args, "logical_name"),
            "--path", _require_str(args, "path"),
        ]
    if name == "verdict":
        verdict_value = _require_str(args, "verdict")
        if verdict_value not in {"accept", "accept_with_findings", "needs_revision", "reject"}:
            raise _ArgError("invalid_argument", f"unknown verdict {verdict_value!r}")
        argv = [
            "verdict",
            "--session-id", _require_str(args, "session_id"),
            "--job-id", _require_str(args, "job_id"),
            "--lease-id", _require_str(args, "lease_id"),
            "--verdict", verdict_value,
        ]
        findings = _optional_str(args, "findings_artifact_id")
        if findings is not None:
            argv.extend(["--findings-artifact-id", findings])
        rationale = _optional_str(args, "rationale")
        if rationale is not None:
            argv.extend(["--rationale", rationale])
        return argv
    if name == "complete":
        argv = [
            "complete",
            "--session-id", _require_str(args, "session_id"),
            "--job-id", _require_str(args, "job_id"),
            "--lease-id", _require_str(args, "lease_id"),
        ]
        summary = _optional_str(args, "summary")
        if summary is not None:
            argv.extend(["--summary", summary])
        return argv
    if name == "supervise_stop":
        return [
            "supervise", "stop",
            "--session-id", _require_str(args, "session_id"),
            "--reason", _require_str(args, "reason"),
        ]
    if name == "run_summary":
        return [
            "run", "summary",
            "--run-id", _require_str(args, "run_id"),
            "--path", _require_str(args, "path"),
        ]
    if name == "evidence_export":
        return [
            "evidence", "export",
            "--run-id", _require_str(args, "run_id"),
            "--path", _require_str(args, "path"),
        ]
    raise _ArgError("unknown_tool", f"dogfood-lifecycle tool {name!r} has no argv mapping")


def _generator_error(code: str, exc: Exception) -> str:
    error: dict[str, Any] = {"ok": False, "error": {"code": code, "message": str(exc)}}
    field_path = getattr(exc, "field_path", None)
    if isinstance(field_path, str):
        error["error"]["field_path"] = field_path
    hint = getattr(exc, "hint", None)
    if isinstance(hint, str):
        error["error"]["hint"] = hint
    ref = getattr(exc, "ref", None)
    if isinstance(ref, str):
        error["error"]["ref"] = ref
    return json.dumps(error, indent=2, sort_keys=True)


def _service_daemon_error(exc: Exception) -> str:
    error_body: dict[str, Any] = {
        "ok": False,
        "error": {
            "code": getattr(exc, "code", "daemon_error"),
            "message": str(exc),
        },
    }
    status = getattr(exc, "status", None)
    if isinstance(status, int):
        error_body["error"]["status"] = status
    kind = getattr(exc, "kind", None)
    if isinstance(kind, str):
        error_body["error"]["kind"] = kind
    details = getattr(exc, "details", None)
    if isinstance(details, dict) and details:
        error_body["error"]["details"] = details
        for key in ("field_path", "hint", "ref"):
            value = details.get(key)
            if isinstance(value, str):
                error_body["error"][key] = value
    return json.dumps(error_body, indent=2, sort_keys=True)
