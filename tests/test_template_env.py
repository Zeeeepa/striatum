from __future__ import annotations

from striatum.web.template_env import escape_html, jinja_env


def test_escape_html_matches_service_escaping_contract() -> None:
    assert escape_html("""<&>"'""") == "&lt;&amp;&gt;&quot;&#x27;"


def test_jinja_env_loads_bundled_templates() -> None:
    template = jinja_env().get_template("run_list.html")

    assert template.name == "run_list.html"
