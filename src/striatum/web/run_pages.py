"""Run page route helpers for the local web UI."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Protocol

from striatum.web.artifacts import (
    ArtifactViewPayloadError,
    artifact_view_template_context,
)
from striatum.web.job_detail import JobDetailPayloadError, job_detail_template_context
from striatum.web.run_list import run_list_view_item
from striatum.web.run_posture_verdicts import (
    RunPostureVerdictsPayloadError,
    run_posture_verdicts_template_context,
)

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
HtmlSender = Callable[[int, str], None]
TemplateEnvFactory = Callable[[], Any]
LegacyFallbackChecker = Callable[[str], bool]
LegacyCallable = Callable[..., Any]


class RunPageState(Protocol):
    repo: Path
    web_context_secret: bytes

    def github_base_url(self) -> str | None: ...

    def default_branch(self) -> str: ...


@dataclass(frozen=True)
class RunPageContext:
    state: RunPageState
    send_json: JsonSender
    send_html: HtmlSender
    jinja_env: TemplateEnvFactory
    legacy_web_read_fallback_enabled: LegacyFallbackChecker
    legacy_run_list_items_for_test_harness: LegacyCallable
    legacy_run_posture_verdicts_payload: LegacyCallable
    legacy_run_detail_payload: LegacyCallable
    legacy_job_detail_payload: LegacyCallable
    legacy_artifact_view_payload: LegacyCallable


def render_run_list_page(ctx: RunPageContext) -> None:
    """Server-side render the run-list page."""
    try:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        try:
            payload = call_repo_method(ctx.state.repo, "list.runs", {"limit": 500})
            items = payload.get("items")
            raw_runs = items if isinstance(items, list) else []
            runs = [
                run_list_view_item(ctx.state, item)
                for item in raw_runs
                if isinstance(item, Mapping)
            ]
            runs.sort(key=lambda item: str(item.get("created_at") or ""), reverse=True)
        except ServiceDaemonRpcError as exc:
            if ctx.legacy_web_read_fallback_enabled(exc.code):
                runs = ctx.legacy_run_list_items_for_test_harness(
                    ctx.state.repo,
                    shape_run=lambda item: run_list_view_item(ctx.state, item),
                )
            else:
                ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
                return
        html = ctx.jinja_env().get_template("run_list.html").render(runs=runs)
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def render_run_subpath(ctx: RunPageContext, subpath: str) -> None:
    """Dispatch /run/<run_id> page subpaths."""
    if not subpath:
        ctx.send_json(404, _error(404, "missing run id"))
        return
    parts = subpath.strip("/").split("/")
    run_id = parts[0]
    if any(char in run_id for char in ("..", "/", "\x00")):
        ctx.send_json(400, _error(400, "invalid run id"))
        return
    if len(parts) == 1:
        render_run_detail_page(ctx, run_id)
        return
    if len(parts) == 3 and parts[1] == "job":
        render_job_detail_page(ctx, run_id, parts[2])
        return
    if len(parts) == 3 and parts[1] == "artifact":
        render_artifact_view_page(ctx, run_id, parts[2])
        return
    if len(parts) == 3 and parts[1] == "posture":
        render_run_posture_verdicts_page(ctx, run_id, parts[2])
        return
    ctx.send_json(404, _error(404, "not found"))


def render_run_posture_verdicts_page(
    ctx: RunPageContext,
    run_id: str,
    posture: str,
) -> None:
    if any(char in posture for char in ("/", "\x00", "..")) or not posture:
        ctx.send_json(400, _error(400, "invalid posture"))
        return
    try:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        try:
            payload = call_repo_method(
                ctx.state.repo,
                "run.posture_verdicts",
                {"run_id": run_id, "posture": posture},
            )
        except ServiceDaemonRpcError as exc:
            if ctx.legacy_web_read_fallback_enabled(exc.code):
                try:
                    payload = ctx.legacy_run_posture_verdicts_payload(
                        ctx.state.repo,
                        run_id=run_id,
                        posture=posture,
                    )
                except KeyError:
                    ctx.send_json(404, _error(404, "run not found"))
                    return
            else:
                ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
                return
        try:
            context = run_posture_verdicts_template_context(
                payload,
                requested_posture=posture,
            )
        except RunPostureVerdictsPayloadError as exc:
            ctx.send_json(500, _error(500, str(exc)))
            return
        html = ctx.jinja_env().get_template("run_posture_verdicts.html").render(
            **context
        )
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def render_run_detail_page(ctx: RunPageContext, run_id: str) -> None:
    try:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method
        from striatum.web import graph_svg

        try:
            payload = call_repo_method(ctx.state.repo, "run.detail", {"run_id": run_id})
        except ServiceDaemonRpcError as exc:
            if ctx.legacy_web_read_fallback_enabled(exc.code):
                try:
                    payload = ctx.legacy_run_detail_payload(ctx.state.repo, run_id=run_id)
                except KeyError:
                    ctx.send_json(404, _error(404, "run not found"))
                    return
            else:
                ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
                return
        run_raw = payload.get("run")
        workflow_raw = payload.get("workflow")
        jobs_raw = payload.get("jobs")
        if (
            not isinstance(run_raw, Mapping)
            or not isinstance(workflow_raw, Mapping)
            or not isinstance(jobs_raw, list)
        ):
            ctx.send_json(
                500,
                _error(500, "daemon run detail DTO missing fields"),
            )
            return
        run = dict(run_raw)
        workflow = dict(workflow_raw)
        jobs = [dict(job) for job in jobs_raw if isinstance(job, Mapping)]
        node_states = graph_svg.compute_node_states_from_jobs(jobs)
        graph_svg_body = graph_svg.render_run_graph(
            workflow,
            node_states,
            run_id=run_id,
            jobs=jobs,
        )
        # _recovery_panel.html references panel.run_id and
        # panel.recipes; an empty dict from the daemon yields a Jinja
        # Undefined that tojson cannot serialize. Backfill the two
        # required keys so the template renders with no recipes when
        # the daemon does not return any.
        recovery_panel_raw = payload.get("recovery_panel")
        if isinstance(recovery_panel_raw, Mapping):
            recovery_panel = dict(recovery_panel_raw)
        else:
            recovery_panel = {}
        recovery_panel.setdefault("run_id", run_id)
        recovery_panel.setdefault("recipes", [])
        html = ctx.jinja_env().get_template("run_detail.html").render(
            run=run,
            jobs=jobs,
            graph_svg=graph_svg_body,
            next_actions=payload.get("next_actions") or [],
            recovery_panel=recovery_panel,
            verdicts_by_posture=payload.get("verdicts_by_posture") or {},
            sessions=payload.get("sessions") or [],
            phase_progress=payload.get("phase_progress") or [],
            current_phase_id=payload.get("current_phase_id"),
            suggested_branch_name=payload.get("suggested_branch_name") or "",
            allow_dirty=bool(payload.get("allow_dirty")),
        )
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def render_job_detail_page(ctx: RunPageContext, run_id: str, job_id: str) -> None:
    try:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        try:
            payload = call_repo_method(
                ctx.state.repo,
                "job.detail",
                {"run_id": run_id, "job_id": job_id},
            )
        except ServiceDaemonRpcError as exc:
            if ctx.legacy_web_read_fallback_enabled(exc.code):
                try:
                    payload = ctx.legacy_job_detail_payload(
                        ctx.state.repo,
                        run_id=run_id,
                        job_ref=job_id,
                    )
                except KeyError:
                    ctx.send_json(404, _error(404, "not found"))
                    return
            else:
                ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
                return
        try:
            context = job_detail_template_context(
                payload,
                web_context_secret=ctx.state.web_context_secret,
            )
        except JobDetailPayloadError as exc:
            ctx.send_json(500, _error(500, str(exc)))
            return
        html = ctx.jinja_env().get_template("job_detail.html").render(**context)
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def render_artifact_view_page(
    ctx: RunPageContext,
    run_id: str,
    artifact_id: str,
) -> None:
    try:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        try:
            payload = call_repo_method(
                ctx.state.repo,
                "artifact.show",
                {
                    "artifact_id": artifact_id,
                    "run_id": run_id,
                    "include_web_context": True,
                },
            )
        except ServiceDaemonRpcError as exc:
            if ctx.legacy_web_read_fallback_enabled(exc.code):
                try:
                    payload = ctx.legacy_artifact_view_payload(
                        ctx.state.repo,
                        run_id=run_id,
                        artifact_id=artifact_id,
                    )
                except KeyError:
                    ctx.send_json(404, _error(404, "not found"))
                    return
            else:
                ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
                return
        try:
            context = artifact_view_template_context(ctx.state.repo, payload)
        except ArtifactViewPayloadError as exc:
            ctx.send_json(500, _error(500, str(exc)))
            return
        html = ctx.jinja_env().get_template("artifact_view.html").render(**context)
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


def _rpc_error(code: str, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


__all__ = [
    "RunPageContext",
    "render_artifact_view_page",
    "render_job_detail_page",
    "render_run_detail_page",
    "render_run_list_page",
    "render_run_posture_verdicts_page",
    "render_run_subpath",
]
