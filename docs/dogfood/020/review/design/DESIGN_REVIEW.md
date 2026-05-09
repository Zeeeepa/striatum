# Design review: RFC 0022 V1

author: reviewer-claude-opus-001
date: 2026-05-09
verdict: accept_with_findings

Devil's-advocate review of `DESIGN_SYNTHESIS.md`.

## Verdict

**accept_with_findings** — V1 is implementable. Two findings
(both notes; no acceptance-blockers).

## Sweep

### CSP impact

Jinja2 server-side rendering produces inline HTML; no inline
`<script>` or `<style>` is required as long as the synthesis
sticks to external CSS files and external `<script defer>`
references. Confirmed in §5 (mutation buttons load via
`<script defer src="/static/mutations.js">`) and §3 (CSS lives
in `base.css`, not inline). **CSP unchanged. Accept.**

### SVG layout for revision cycles

The synthesis says "topological depth via longest path." If
the workflow has cycles (RFC's `cycles` block — review
`needs_revision` loops), longest-path is undefined.

**Survives?** Partially. The `workflow.py` graph data already
treats cycles separately from the main DAG; `cycles` are
rendered as separate badges, not edges in the main flow. The
synthesis should clarify: layout uses only the *forward DAG*
edges from `graph.edges`; `graph.cycles` are rendered as
chip annotations on the source review node, not as edges.

**Finding 1 (note):** synthesis §4 should explicitly call out
that cycles are not rendered as graph edges. Implementation
choice — implementer should pin this in BUILD_HANDOFF.

### Hash-route compatibility

The synthesis's JS-island fallback (`window.location.hash`
read-on-load + `replace`) is correct given that browsers don't
send `#` to the server. There's no way to do a real 302; the
JS island is the closest we can get. Operators with bookmarked
`#/run/<id>` URLs see one extra navigation hop. Acceptable.

### Dark mode

The palette in §3 covers all status colors readably in both
modes. `--status-failed` goes from `#d1242f` (light) to
`#f85149` (dark) — both have adequate contrast against
respective backgrounds. **Accept.**

### Zero regression on JSON API + SSE

§9 states this explicitly. The new routes are added *before*
the `/static/*` catch-all, and after the `/v1/*` JSON
branches. No `/v1/*` path is intercepted. **Accept.**

### Test plan completeness

13 cases cover the rendered HTML for each page, CSP, dark
mode palette presence, SVG layout, navigation, the hash JS
island, Jinja2 environment construction, and the mutation
gating. Reasonable coverage.

**Finding 2 (note):** add a test that the legacy
`/static/index.html` request still returns *something* (the
old SPA shell), or explicitly returns 404 with a clear
message. The transition behavior matters for operators with
old browser caches.

### Jinja2 as runtime dep

~250 KB wheel impact + one transitive (markupsafe, also
~30 KB). Striatum becomes a 2-package install. Trade-off
worth taking for HTML correctness over `str.format`-shaped
XSS hazards.

## Findings summary

| # | Severity | Action |
| --- | --- | --- |
| 1 | note | Synthesis §4 clarifies cycles are not rendered as graph edges — only forward DAG edges from `graph.edges`. |
| 2 | note | Add a test for `/static/index.html` legacy request behavior. |

## Decision

Accept V1 with both findings folded into the implementation.
Both are notes; the implementer can proceed.
