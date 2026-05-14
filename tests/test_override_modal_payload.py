from __future__ import annotations

from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "src/striatum/web/static/override_verdict.js"


def _script() -> str:
    return SCRIPT.read_text(encoding="utf-8")


def test_override_modal_posts_literal_invoke_argv() -> None:
    js = _script()

    assert 'fetch("/v1/invoke"' in js
    assert 'method: "POST"' in js
    assert 'JSON.stringify({ argv: argv })' in js
    assert '"override-verdict"' in js
    assert '"--session-id"' in js
    assert '"--job-id"' in js
    assert '"--auto-fresh-session"' in js
    assert '"--findings-artifact-id"' in js
    assert '"--json"' in js


def test_override_modal_payload_collects_only_allowed_form_fields() -> None:
    js = _script()
    start = js.index("function collectPayload")
    end = js.index("function buildArgv")
    collect_payload = js[start:end]

    assert "form.elements.verdict" in collect_payload
    assert "form.elements.rationale" in collect_payload
    assert "form.elements.findings_artifact_id" in collect_payload
    assert "form.elements.auto_fresh_session" in collect_payload
    assert "form.elements.session_id" not in collect_payload
    assert "form.elements.job_id" not in collect_payload
    assert "sessionId" not in collect_payload
    assert "jobId" not in collect_payload
    assert "new FormData" not in collect_payload


def test_override_modal_refuses_empty_rationale_before_fetch() -> None:
    js = _script()

    validation = "if (!payload.rationale)"
    fetch_call = "postInvoke(buildArgv(context, payload))"
    assert validation in js
    assert js.index(validation) < js.index(fetch_call)
    assert "Rationale is required." in js


def test_override_modal_has_dialog_aria_escape_and_focus_trap() -> None:
    js = _script()

    assert 'document.createElement("dialog")' in js
    assert 'dialog.setAttribute("aria-modal", "true")' in js
    assert 'dialog.setAttribute("aria-labelledby", titleId)' in js
    assert 'dialog.setAttribute("aria-describedby", descId)' in js
    assert 'event.key === "Escape"' in js
    assert "trapFocus(dialog, event)" in js
    assert "priorFocus.focus()" in js
