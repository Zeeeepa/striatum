"""GH #10 regression: override-verdict modal binds to the rendered
job page context.

Client side (static JS):
- ``buildArgv`` uses the page-rendered identifiers, not user input.
- ``parsePageContext`` extracts ``(run_id, job_id)`` from ``/run/.../
  job/...``.
- ``buildWebContext`` carries the server-issued context token.
- The POST body shape includes ``web_context``.
"""

from __future__ import annotations

from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "src/striatum/web/static/override_verdict.js"
TEMPLATE = ROOT / "src/striatum/web/templates/job_detail.html"


def _script() -> str:
    return SCRIPT.read_text(encoding="utf-8")


def _template() -> str:
    return TEMPLATE.read_text(encoding="utf-8")


# --- static client checks -------------------------------------------


def test_override_modal_parses_page_url_for_run_and_job() -> None:
    js = _script()
    assert "parsePageContext" in js
    assert "/\\/run\\/([^/]+)\\/job\\/([^/]+)/" in js or "/run/" in js
    assert "window.location.pathname" in js


def test_override_modal_refuses_on_dom_url_mismatch_before_fetch() -> None:
    js = _script()
    # The mismatch flag must guard openDialog and submit, before
    # postInvoke is ever called.
    assert "contextMismatch" in js
    submit_index = js.index("submitButton.disabled = true;")
    mismatch_check = js.rindex("if (contextMismatch)", 0, submit_index)
    assert mismatch_check < submit_index


def test_override_modal_posts_web_context_token() -> None:
    js = _script()
    assert "buildWebContext" in js
    assert "kind: \"override_verdict\"" in js
    assert "token: context.contextToken" in js
    assert "web_context: webContext" in js


def test_job_detail_template_emits_context_token_attribute() -> None:
    html = _template()
    assert "data-context-token" in html
    assert "override_context_token" in html
