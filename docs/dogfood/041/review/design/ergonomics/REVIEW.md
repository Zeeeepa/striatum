---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0038", "web-ui", "frontend-toolchain"]
---

author: reviewer-claude-opus-001

# RFC 0038 Design Synthesis — Ergonomics-DX Review

## Verdict

`accept_with_findings`. The synthesis is implementable, the affordances
are well-scoped, the island deployment shape is preserved, accessibility
coverage is comprehensive, and the staging plan front-loads risk
correctly. The findings below are non-blocking ergonomic refinements
that should be captured before the implementer phase begins so the build
phase has explicit guidance rather than rediscovering open questions.

## Posture

Ergonomics-DX. Acceptance means a first-time operator can discover the
new affordances, understand what each step is asking for, and recover
from missteps without re-reading the RFC. I evaluated the synthesis as
the input that drives the implementer prompts, not as a finished UI
spec — the question is whether the implementers have enough to produce
something a new operator can use cold.

## Summary of Strengths

1. **Toolchain honesty is excellent.** Vite + React + TypeScript + React
   Flow + Shiki, with the explicit pushback against the broader Gemini
   stack ("Do not add Radix, lucide, clsx, Tailwind helpers, ESLint, or
   an accessibility plugin in the first pass unless the component
   implementer demonstrates a concrete gap"). This is the right
   posture for a local-first tool with a supply-chain concern.
2. **Island deployment shape is preserved.** Every island mounts into a
   Jinja2-owned slot via `createRoot()`; CSP shape (no inline scripts,
   no `unsafe-inline`, no `unsafe-eval`, no CDN, no external runtime
   fetch) is restated as a hard rule. The dual mount via `data-props`
   for scalar props and adjacent `<script type="application/json">`
   for large payloads is the right shape and avoids oversized escaped
   attributes — exactly the kind of detail that prevents a build-time
   surprise.
3. **Server-rendered fallback for the code viewer is the right call.**
   Markdown stays server-rendered, non-Markdown text renders a `<pre>`
   fallback before the island mounts. First paint shows readable text
   even if JavaScript fails — a real ergonomic safety net.
4. **Build determinism framing is honest.** The synthesis names the
   `manifest.sha256` check as "a drift detector that prevents source
   and committed bundle output from silently diverging" rather than
   overselling it as a supply-chain guarantee. That keeps the operator
   expectation correct.
5. **Staging plan is correct.** Toolchain + CI → Edit-button promotion
   → code viewer (read-only) → tree browser (read-only) → chooser
   (mutation) → graph editor (mutation) → docs. Read-only islands ship
   before mutation-heavy authoring surfaces; build/package risk is
   front-loaded.
6. **Accessibility checklist is comprehensive.** `<dialog>`-with-
   `showModal()` for the chooser confirm step, WAI-ARIA tree semantics
   with roving tabindex and `aria-level`, focus management on open/
   close, `prefers-reduced-motion` disabling graph animation,
   `aria-hidden` on line numbers, exact `aria-label`s on Copy/Raw/Wrap,
   and the RFC 0037 skip link preservation on every new route. This is
   the right baseline.
7. **Disjoint write scopes are clear.** `implement_toolchain_codex` owns
   Python, Makefile, CI, pyproject, templates, service routes, scripts.
   `implement_components_claude` owns TypeScript, frontend tests,
   contributor and operator docs. The boundary collisions in past
   dogfoods are unlikely to recur with this split.

## Findings

### F1. Tree browser breadcrumbs not specified (medium)

The prompt explicitly asks for "breadcrumbs" in the tree navigation
grammar. The synthesis specifies WAI-ARIA tree semantics, roving
tabindex, search over loaded entries, and per-directory retry states,
but does **not** call out a breadcrumb trail of clickable ancestor
segments at the top of the browser.

For an operator who clicks two levels into `docs/dogfood/041/design/`,
the only way back to a higher ancestor is to either collapse upward in
the tree or edit the URL. A breadcrumb of `docs / dogfood / 041 /
design` with each segment a link to `/view/<that-path>` is the standard
ergonomic affordance.

**Recommendation (non-blocking):** the implementer prompt for the tree
browser island should add: "Render a breadcrumb path above the tree,
where each ancestor segment is a link that navigates to that
directory's expanded view. The root segment links to `/view/`."

### F2. Tree browser landing copy and empty/loading states absent (low)

A first-time operator who arrives at `/view/` with no path sees only
the root-level entry list. The synthesis specifies the data shape and
the failure live region but does not say:

- What introductory text (if any) sits above the tree explaining what
  this surface is for ("Browse repository files — click a directory
  to expand, click a file to view with syntax highlighting").
- What renders for an empty directory.
- What renders during the lazy load between click and entries arriving
  (skeleton row, spinner, "Loading…" text).

These are small but they meaningfully change whether the surface feels
finished. An implementer left to guess will pick "nothing" by default.

**Recommendation (non-blocking):** the implementer prompt should
specify a one-sentence header, an empty-directory state ("This
directory has no entries"), and a loading placeholder (a single
shimmer row or "Loading…" text node with `aria-live="polite"` so
screen readers announce arrival).

### F3. Chooser wizard copy quality is unaddressed (medium)

The prompt explicitly calls out "copy quality (`recommended_for`
specific, not boilerplate)". The synthesis describes the radio cards
as rendering shape `summary` + `recommended_for`, but does not address
whether those strings are operator-readable today or whether the
implementer should review and tighten template copy as part of this
work.

RFC 0034 templates were authored before this UI consumed them. It is
plausible that some `recommended_for` strings are generic ("teams that
want X") rather than concrete ("a 3-pane review workflow with author,
reviewer, and integrator over a single artifact"). Without a copy
audit step, the wizard surface will inherit whatever quality is there.

**Recommendation (non-blocking):** the toolchain or components
implementer prompt (whichever ends up touching the template catalog)
should include a step to read each shape/lane-set `recommended_for`
and `summary` field, rewrite any that are boilerplate, and prefer
concrete operator-facing phrasing. This is a small write scope
extension into the YAML catalog if any rewrites are needed.

### F4. Wizard back-navigation and step-gating semantics undefined (medium)

The synthesis lists the six steps in order but does not specify:

1. **Forward gating** — when can the operator advance to step N+1?
   Step 1 needs a shape selected. Step 5 (preview) probably runs
   automatically on entry. Step 6 (confirm) requires the preview to
   have succeeded without warnings (or warnings acknowledged). The
   chooser island's `disabled` state on the "Next" button is the
   primary mechanism here and it deserves to be spelled out.
2. **Backward navigation** — if the operator clicks Back from step 5
   to step 4 and edits a field, does the preview auto-invalidate? Does
   step 5 re-run `POST /workflows/generate/preview` on re-entry, or
   does it show the stale rendering until the operator re-requests?

These choices are not interchangeable from an ergonomics standpoint:
stale preview that looks fresh is a footgun.

**Recommendation (non-blocking):** the chooser implementer prompt
should state: "Each step's Next button is disabled until the step's
required selections are made. Editing any field on steps 1–4 after
visiting step 5 invalidates the preview and forces a fresh
`POST /workflows/generate/preview` call when step 5 is re-entered.
The preview render must include a small badge or timestamp showing
when the preview was generated so the operator can tell stale from
fresh at a glance."

### F5. Graph editor inspector widget grammar is lossy (high among the findings)

This is the single most material finding. The prompt asks explicitly
about "radio buttons for postures, dropdowns for enums, multi-select
for tags, file pickers for paths. Are widget choices ergonomically
right?" The RFC §5d also specified "dropdowns for enums, multi-select
for tags, file picker for paths".

The synthesis condenses this to: "A right inspector edits role, lane,
job type, review posture, required postures, write scope, expected
artifacts, parallel group, and edge verdict fields." It then says
"Client validation is ergonomic only. Server `validate_workflow()`
remains authoritative."

It does **not** specify the widget per field. The components
implementer would have to invent the widget grammar from scratch and
risk shipping a uniform "every field is a `<input type="text">`"
inspector — which is the exact problem RFC 0038 is fixing.

Recommended widget mapping the implementer should be handed:

| Field | Widget | Notes |
| --- | --- | --- |
| `role` | dropdown | Closed vocabulary from workflow schema |
| `lane` | dropdown | Closed vocabulary, source-of-truth from server `/v1/lanes` or schema |
| `job.type` | dropdown | Closed vocabulary (`build`, `review`, `synth`, etc.) |
| `review_policy.posture` | radio set | Small finite vocabulary — radios make the choice fully visible |
| `review_policy.required_postures` | multi-select chips | Operators want to add and remove multiple values |
| `write_scope.mode` | radio set | Small finite vocabulary |
| `write_scope.allowed_paths` | repeating path field with add/remove | Each row is a text input; future enhancement is a typeahead against the repo tree |
| `write_scope.forbidden_paths` | repeating path field with add/remove | Same shape |
| `expected_artifacts[]` | structured list editor | Each entry has `kind`, `logical_name`, `path`, `author_line`, `required`; render as repeating card with structured inputs, not raw JSON |
| `parallel_group` | text or optional dropdown | Free-form for now |
| Edge `on` verdict | dropdown | Closed verdict vocabulary |
| Edge fan-in / quorum (if used) | numeric input + dropdown | Match schema |

**Recommendation (non-blocking but high value):** the components
implementer prompt should include the table above (or an equivalent
explicit per-field widget assignment). Without it, the inspector
risks shipping as a structured text-field form, which is precisely
what gap 4 in the RFC problem statement calls out.

### F6. Graph editor keyboard edge creation not addressed (medium)

The accessibility checklist names "keyboard add/delete/save paths"
for nodes but does not mention edge creation. Creating edges by
keyboard alone is the hardest interaction in any graph editor —
React Flow's default keyboard story for edges is weak — and dropping
it tacitly excludes screen-reader users from authoring workflow
graphs.

**Recommendation (non-blocking):** the components implementer prompt
should require either a per-node "Add edge from this node" action
(focuses node → opens a small selector of valid target nodes → press
Enter to confirm) or a tabular edge editor pane synchronized with
the canvas. The textual fallback region required by the a11y checklist
should expose edges textually too, not only nodes.

### F7. Code viewer copy-feedback, raw-link target, and narrow viewport unstated (low)

The Shiki island contract is otherwise solid. Three small gaps:

- **Copy button feedback** — when the operator clicks Copy, do they get
  a brief confirmation ("Copied"), a button-state swap, or nothing?
  Current spec is silent. The `aria-label` is named but not the
  `aria-live` confirmation.
- **Raw link target** — does Raw open in the same tab (navigating away
  from the viewer) or a new tab? `target="_blank"` with
  `rel="noopener"` is conventional for "open raw bytes" and saves the
  operator's place in the viewer.
- **Narrow viewport** — line-number column plus long lines plus
  syntax-highlight spans will overflow on a 600px-wide viewport. The
  Wrap toggle helps but the default behavior on narrow viewports is
  not stated. Default to wrap-off and horizontal scroll inside the
  block (not the page) is conventional.

**Recommendation (non-blocking):** the code-viewer implementer prompt
should add: "Copy button swaps to `Copied` for 2 seconds on success
and announces via `aria-live`. Raw link opens in a new tab with
`rel="noopener"`. Default behavior on viewport widths under 640px is
horizontal scroll inside the `<pre>` block (not the page), with Wrap
remaining the operator-controlled toggle."

### F8. Edit-button visual treatment is a deliberate de-promotion vs. RFC text (low)

RFC 0038 §5a calls for promoting Edit to a "primary button". The
synthesis chooses `secondary-button` instead, explicitly so "the
run-now mutation remains the dominant primary action." Both readings
are ergonomically defensible:

- The RFC wants Edit visible (it currently is `class="muted"`); any
  styled button satisfies that.
- The synthesis preserves the action hierarchy: Run is the dominant
  intent on a workflow detail page; Edit is a secondary tool action.

I read this as **acceptable** — `secondary-button` is a real upgrade
from `class="muted"` and keeps the visual hierarchy honest. The
finding is only that the synthesis should note this is a deliberate
divergence from the RFC's "primary button" language so the implementer
doesn't override it back to primary when reading the RFC.

**Recommendation (non-blocking):** add a one-line note in the
synthesis (or the toolchain implementer prompt) saying: "The Edit
affordance is promoted from muted to `secondary-button`, not primary,
to preserve Run as the dominant action. This is a deliberate
divergence from RFC 0038 §5a's 'primary button' phrasing."

### F9. Prop-contract synchronization between codex and claude is process-thin (medium)

The synthesis says: "Define TypeScript prop types first, then mirror
the same fields in Python dictionaries. Do not let both implementers
edit the same component source files." That states the order but not
the mechanism. Two real questions for the build phase:

- Where do the TypeScript prop types live such that codex (who writes
  Jinja2 templates with `data-props='{{ props | tojson }}'`) can read
  them?
- When a prop shape changes mid-build, what's the synchronization
  ritual? A handoff document? A shared `prop-contract.md` per island?

This matters because codex's templates and claude's components touch
the same wire format from opposite sides. Without an explicit
contract surface they will drift, and the failures will surface only
at integration time.

**Recommendation (non-blocking):** the synthesis or the toolchain
handoff (`docs/dogfood/041/build/toolchain/HANDOFF.md`) should
designate a single shared file — e.g.
`src/striatum/web/frontend/src/shared/types.ts` (claude scope) —
where every island's prop interface is exported, and a parallel
Python module or comment block (codex scope) that mirrors the shape
of `tojson` payloads. Any prop-shape change goes through the shared
types file first; the template change follows. Both implementer
prompts should reference this single source of truth by path.

### F10. Wizard "shape: custom" exclusion creates a discoverability cliff (low)

V1 ships the chooser wizard with built-in shapes only; `shape:
"custom"` is deferred. A first-time operator browsing the shape radio
cards will see only the built-ins — fine — but if they expected the
"custom graph plan" path from RFC 0034 they will have no signal that
it's a real future feature, only an absence.

**Recommendation (non-blocking):** the chooser implementer should add
a disabled-with-tooltip card or a footer note ("Custom graph plans
are CLI-only in V1; web authoring is coming after the graph editor
stabilizes") so the absence is explained rather than mysterious.

## What I Verified

- The synthesis maps every RFC 0038 acceptance criterion to a concrete
  plan with an owner (the table in §"Accepted Scope").
- The dependency set is narrow and the rationale for narrowing is
  stated.
- The island layout, mount pattern, CSP posture, and bundle hash
  manifest are concrete enough to implement.
- The Python and TypeScript write scopes do not overlap.
- The staging plan front-loads build risk and read-only islands before
  mutation-heavy ones.
- The a11y checklist names the right primitives for each island.
- The synthesis explicitly resolves every "Open Question" from RFC
  0038, leaving no operator-decision blocker for implementation.

## What I Did Not Verify (Out of Scope for This Posture)

- Code-level changes against `src/striatum/service.py` or existing web
  templates.
- Whether react-flow or Shiki package names and versions are
  ecosystem-current as of 2026-05-12.
- Performance characteristics of the tree browser on a large repo
  (loaded-only search may degrade on deep trees with many entries per
  level).
- Security review of the new `GET /v1/repo/tree` endpoint — the
  rejection list looks complete but a security-posture review should
  confirm.

These are correctness, security, and conservatism postures and belong
to other review lanes.

## Findings Are Non-Blocking

Per the `accept_with_findings` verdict, none of the above require the
synthesis to be revised before the build phase begins. The implementer
prompts are the right place to absorb F1–F10; the synthesis already
includes the much harder structural decisions (toolchain, island
shape, build determinism, accessibility baseline, disjoint scopes,
staging order). The findings are quality-of-implementation refinements
that an attentive implementer would discover during the build, but
writing them down now avoids rediscovery cost and gives both
implementers a more complete handoff.
