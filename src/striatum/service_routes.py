"""Route dispatch helpers for the local service handler."""

from __future__ import annotations

from typing import Any
from urllib.parse import parse_qs, urlsplit

from striatum.service_command_policy import is_read_command


def dispatch_get(handler: Any) -> None:
    if not handler._authenticate():
        return
    parsed = urlsplit(handler.path)
    path = parsed.path
    query = parse_qs(parsed.query)
    if path == "/v1/health":
        handler._handle_health()
        return
    if path == "/v1/runs":
        handler._handle_daemon_read("status", {}, legacy_argv=["status"])
        return
    if path == "/v1/doctor":
        handler._handle_doctor(query)
        return
    if path == "/v1/repo/tree":
        handler._handle_repo_tree(query)
        return
    if path.startswith("/v1/runs/"):
        handler._handle_run_subpath(path[len("/v1/runs/"):], query)
        return
    if path.startswith("/v1/artifacts/") and path.endswith("/raw"):
        artifact_id = path[len("/v1/artifacts/"):-len("/raw")]
        handler._handle_artifact_raw(artifact_id)
        return
    if path == "/workflow-templates":
        kind = query.get("kind", [None])[0]
        handler._handle_workflow_templates(kind)
        return
    if path.startswith("/workflow-templates/"):
        handler._handle_workflow_template_show(path[len("/workflow-templates/"):])
        return
    if handler.state.web_enabled and (path == "/" or path == ""):
        handler._render_run_list_page()
        return
    if handler.state.web_enabled and path == "/doctor":
        handler._render_doctor_page()
        return
    if handler.state.web_enabled and path == "/escalations":
        handler._render_escalation_list_page(query)
        return
    if handler.state.web_enabled and path.startswith("/escalations/"):
        handler._render_escalation_detail_page(path[len("/escalations/"):])
        return
    if handler.state.web_enabled and (path == "/cross-repo" or path == "/cross-repo/"):
        handler._render_cross_repo_page()
        return
    if handler.state.web_enabled and path == "/chat":
        handler._render_chat_index_page()
        return
    if handler.state.web_enabled and path.startswith("/chat/"):
        handler._render_chat_subpath(path[len("/chat/"):])
        return
    if handler.state.web_enabled and path == "/view":
        handler._render_view_path("")
        return
    if handler.state.web_enabled and path.startswith("/view/"):
        handler._render_view_path(path[len("/view/"):])
        return
    if handler.state.web_enabled and path == "/workflows":
        handler._render_workflows_index_page()
        return
    if handler.state.web_enabled and path.startswith("/workflows/"):
        handler._render_workflow_detail_page(path[len("/workflows/"):])
        return
    if handler.state.web_enabled and path.startswith("/run/"):
        handler._render_run_subpath(path[len("/run/"):])
        return
    if handler.state.web_enabled and (path == "/dogfood" or path == "/dogfood/"):
        handler._render_dogfood_index_page()
        return
    if handler.state.web_enabled and path.startswith("/dogfood/"):
        rest = path[len("/dogfood/"):]
        # rest = "<id>/" → list page; "<id>/<rel_path>" → file page
        parts = rest.split("/", 1)
        dogfood_id = parts[0]
        if not dogfood_id:
            handler._render_dogfood_index_page()
            return
        if len(parts) == 1 or parts[1] == "":
            handler._render_dogfood_run_page(dogfood_id)
            return
        rel_path = parts[1].rstrip("/")
        raw = query.get("raw", ["0"])[0] in ("1", "true", "yes")
        handler._render_dogfood_file_page(dogfood_id, rel_path, raw=raw)
        return
    if handler.state.web_enabled and path.startswith("/static/"):
        relative = path[len("/static/"):]
        handler._serve_static_asset(relative)
        return
    if path == "/" or path == "":
        handler._send_json(
            404,
            {
                "ok": False,
                "error": {
                    "code": 404,
                    "message": (
                        "not found; pass --web to enable the local UI "
                        "(RFC 0013 V1)"
                    ),
                },
            },
        )
        return
    handler._send_json(404, _error(404, "not found"))


def dispatch_post(handler: Any) -> None:
    if not handler._authenticate():
        return
    parsed = urlsplit(handler.path)
    if (
        handler._requires_same_origin(parsed.path)
        and not handler._verify_same_origin_mutation()
    ):
        return
    if handler.state.web_enabled and parsed.path == "/chat/new":
        handler._handle_chat_new()
        return
    if handler.state.web_enabled and parsed.path.startswith("/workflows/accept-risk/"):
        handler._handle_workflow_accept_risk(
            parsed.path[len("/workflows/accept-risk/"):]
        )
        return
    if handler.state.web_enabled and parsed.path.startswith("/workflows/run/"):
        handler._handle_workflow_run_now(parsed.path[len("/workflows/run/"):])
        return
    if parsed.path == "/workflows/generate/preview":
        handler._handle_workflow_generate(preview=True)
        return
    if parsed.path == "/workflows/generate":
        handler._handle_workflow_generate(preview=False)
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/run/")
        and parsed.path.endswith("/branch-confirm")
    ):
        run_id = parsed.path[len("/run/"):-len("/branch-confirm")]
        handler._handle_run_branch_confirm(run_id)
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/run/")
        and parsed.path.endswith("/cancel")
        and "/job/" not in parsed.path
    ):
        run_id = parsed.path[len("/run/"):-len("/cancel")]
        handler._handle_run_cancel(run_id)
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/run/")
        and parsed.path.endswith("/pause")
    ):
        run_id = parsed.path[len("/run/"):-len("/pause")]
        handler._handle_run_pause(run_id)
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/run/")
        and parsed.path.endswith("/resume")
    ):
        run_id = parsed.path[len("/run/"):-len("/resume")]
        handler._handle_run_resume(run_id)
        return
    if (
        handler.state.web_enabled
        and "/job/" in parsed.path
        and (parsed.path.endswith("/cancel") or parsed.path.endswith("/retry"))
    ):
        handler._handle_job_action(parsed.path)
        return
    parts = parsed.path.strip("/").split("/")
    if (
        handler.state.web_enabled
        and len(parts) == 4
        and parts[0] == "run"
        and parts[2] == "recovery"
    ):
        handler._handle_recovery_action(parts[1], parts[3])
        return
    if (
        handler.state.web_enabled
        and len(parts) == 6
        and parts[0] == "run"
        and parts[2] == "job"
        and parts[4] == "worktree"
    ):
        handler._handle_worktree_action(parts[1], parts[3], parts[5])
        return
    if (
        handler.state.web_enabled
        and len(parts) == 5
        and parts[0] == "run"
        and parts[2] == "artifact"
        and parts[4] == "commit-apply"
    ):
        handler._handle_artifact_commit_apply(parts[1], parts[3])
        return
    if (
        handler.state.web_enabled
        and len(parts) == 4
        and parts[0] == "sessions"
        and parts[2] == "supervise"
    ):
        handler._handle_supervise_action(parts[1], parts[3])
        return
    if (
        handler.state.web_enabled
        and len(parts) == 3
        and parts[0] == "cross-repo"
        and parts[2] == "cancel"
    ):
        handler._handle_cross_repo_cancel(parts[1])
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/escalations/")
        and parsed.path.endswith("/resolve")
    ):
        escalation_id = parsed.path[len("/escalations/"):-len("/resolve")]
        handler._handle_escalation_resolve(escalation_id)
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/chat/")
        and parsed.path.endswith("/send")
    ):
        session_id = parsed.path[len("/chat/"):-len("/send")]
        handler._handle_chat_send(session_id)
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/chat/")
        and "/confirm-tool/" in parsed.path
    ):
        rest = parsed.path[len("/chat/"):]
        session_id, _, tool_id = rest.partition("/confirm-tool/")
        handler._handle_chat_confirm_tool(session_id, tool_id)
        return
    if (
        handler.state.web_enabled
        and parsed.path.startswith("/chat/")
        and parsed.path.endswith("/stop")
    ):
        session_id = parsed.path[len("/chat/"):-len("/stop")]
        handler._handle_chat_stop(session_id)
        return
    if parsed.path != "/v1/invoke":
        handler._send_json(404, _error(404, "not found"))
        return
    body = handler._read_json_body()
    if body is None:
        return
    argv = body.get("argv")
    if not isinstance(argv, list) or not all(isinstance(part, str) for part in argv):
        handler._send_json(400, _error(400, "argv must be a list of strings"))
        return
    if not handler.state.allow_mutations and not is_read_command(argv):
        verb = " ".join(argv[:2]) if argv else ""
        handler._send_json(
            405,
            _error(405, f"command requires --allow-mutations: {verb}"),
        )
        return
    if (
        handler.state.web_enabled
        and argv
        and argv[0] == "override-verdict"
        and not handler._verify_override_verdict_context(argv, body)
    ):
        return
    handler._handle_invoke(argv)


def _error(code: int, message: str) -> dict[str, Any]:
    return {"ok": False, "error": {"code": code, "message": message}}
