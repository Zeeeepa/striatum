---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Static Assets And Templates Handoff
author: operator [self-declared: static-porter-codex-gpt-5-002]

## Changed Files

- `go/pkg/webassets/assets.go`
- `go/pkg/webassets/static/base.css`
- `go/pkg/webassets/static/app.js`
- `go/pkg/webassets/templates/page.html`
- `go/web/static/base.css`
- `go/web/static/app.js`
- `go/web/templates/page.html`

## Embedded Assets

`go/pkg/webassets` embeds the current Go-owned static/template seed used by
the retained routes. The package exposes `LoadStatic` for `/static/*` and
`RenderPage` for the minimal server-rendered HTML shell. The `go/web/` copy is
the human-facing source layout for this gate; the package-local embedded copy
is required because Go `embed` cannot read files outside its package tree.

## CSP

The CSP remains self-only for scripts/styles and self/data for images:
`default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'`.
The Go service attaches this CSP to JSON, HTML, and static responses, matching
the local operator-only expectation without inline script/style allowances.

## Validation

- `go test ./pkg/webassets ./pkg/websse ./pkg/webservice ./pkg/webtest ./pkg/webguardrails` passed.

## Deferred

The rich Python Jinja templates and React island bundles are not fully ported.
The retained Go shell is sufficient for route/security cutover tests, but full
operator UI deletion needs a follow-up HTML/island parity slice.
