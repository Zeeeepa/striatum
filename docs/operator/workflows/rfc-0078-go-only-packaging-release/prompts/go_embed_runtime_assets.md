# Go Embed Runtime Assets

Read RFC 0078, prior web and packaging handoffs, `pyproject.toml` package-data
configuration, `go/pkg/db/sql/`, `go/pkg/workflowtemplates/`, current web
static/template locations, skill/plugin templates, and release archive
requirements. Produce
`docs/operator/artifacts/rfc-0078-go-only-packaging-release/embed-assets/HANDOFF.md`.

Implement or design Go `embed` ownership for runtime assets required by a
Python-free distribution:

- daemon SQL migrations;
- workflow template catalog data;
- local web static/template assets if retained;
- skill, plugin, scaffold, or operator-guide templates that must ship with
  the Go binary;
- tests proving assets are available from a clean release archive without
  Python package data.

Do not move unrelated web implementation code. If an asset depends on a route
retention decision not owned by this workflow, name the decision and leave an
explicit TODO in the handoff instead of making silent feature changes.
