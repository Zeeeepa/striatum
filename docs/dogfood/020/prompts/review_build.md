# Build review (devils_advocate)

Verify:
1. `striatum serve --web` returns HTML pages at /, /run/<id>,
   /run/<id>/job/<id>, /run/<id>/artifact/<id>, /doctor.
2. CSP header includes `default-src 'self'`; no `unsafe-inline`.
3. Dark mode renders correctly (override via @media or test in
   isolation).
4. SVG graph renders nodes for a test workflow; click navigates;
   hover tooltip exists.
5. Hash-route redirect: GET /#/run/<id> behaves correctly.
6. JSON API + SSE unchanged.
7. Mutation buttons gated on STRIATUM_WEB_MUTATIONS=1 still work.
8. Lint / typecheck / full test pass.

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
