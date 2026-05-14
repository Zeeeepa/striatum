---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0050", "v1", "build", "operator-on-behalf"]
---

author: reviewer-unknown-model-002



# Build Review — RFC 0050 V1 (Claude, ergonomics_dx)

**Verdict:** accept_with_findings

The five RFC 0050 V1 non-negotiable regressions land cleanly on the
**shipped server-rendered surface** (Jinja2 macros in
`src/striatum/web/templates/_components.html` plus the four touched
page templates, plus `src/striatum/dashboard.py`). The three named
regression tests
(`tests/test_byline_regression.py`,
`tests/test_override_rationale_regression.py`,
`tests/test_dashboard_web_parity.py`) pin the truthfulness rules end
to end. From a first-time-operator perspective the chip vocabulary,
attestation reasons, override suffix, and V1.41 burn-down verbs are
discoverable and consistent across dashboard and web.

What pulls the verdict down to `accept_with_findings` are
ergonomics_dx gaps in the surface: a latent override-collapse
regression in the React `VerdictChip`, dashboard/web parity drift
around operator-self-declared bylines and the posture chip macro,
keyboard reach inconsistencies between Jinja and TSX, and an
across-the-board absence of the copy-on-click identifier affordance
that `docs/design/UI_REWORK.md §5.4` and `§7.7` make explicit. None
of these break the V1 regression contract today; all of them affect
discoverability for new operators and should be tightened before
V1.5.

## Required regression check status

1. **Byline regression (truthfulness rule 1)** — **PASS** (server +
   dashboard). `src/striatum/service.py:286-302` (`_byline_line`)
   passes through `author_line` from the artifact row and tags the
   payload with `attested`; the Jinja macro at
   `src/striatum/web/templates/_components.html:72-84` escapes and
   renders the value verbatim with `byline-unattested` styling.
   `src/striatum/web/templates/run_detail.html:99-101` overrides the
   unattested rail entry to render the literal
   `author: operator`, and the on-disk artifact byline for any
   unattested session is itself written as `author: operator` by
   the runner (proven by `tests/test_byline_regression.py:14-33`).
   The dashboard path mirrors at
   `src/striatum/dashboard.py:464-474`: when
   `_lane_attestation_chip(...)` starts with `unattested:`,
   `_byline_line` returns `author: operator` (or
   `author: operator [self-declared: <label>]` when present).
   `tests/test_byline_regression.py:38-63` confirms zero
   `author: <role>-<lane>` strings appear on either surface for
   unattested sessions.

2. **Override rationale prominence (truthfulness rule 2)** — **PASS**
   on the shipped Jinja + dashboard surface; **latent fail** in the
   React TSX component. The Jinja macro
   (`src/striatum/web/templates/_components.html:36-37`) inlines
   `override · {{ rationale }}` directly inside the chip, never in a
   `<details>`. `src/striatum/web/templates/job_detail.html:46-54`
   additionally renders the rationale as a separate
   `<p class="override-rationale">` block (redundant but
   defensive). The dashboard prints the rationale on its own line
   (`src/striatum/dashboard.py:666-679`,
   `_render_verdict_overrides`). `tests/test_override_rationale_regression.py:62-73`
   pins both surfaces.

   Latent: `src/striatum/web/frontend/src/shared/components/VerdictChip.tsx:21-37`
   wraps the override variant in a default-closed `<details>` —
   `UI_REWORK §9.4` forbids exactly that (`no ancestor
   <details>:not([open])` for the rationale node). The TSX
   component is not mounted by any island today, so the regression
   test passes; the moment an island mounts it on `job_detail.html`,
   §9.4 fails. See **F1** below.

3. **LaneEvidenceChip muted** — **PASS**.
   `src/striatum/web/frontend/src/shared/types.ts:385-391`
   constrains `LaneEvidenceState` to the single literal
   `"not_yet_correlated"`, so the React component can never type-
   check into a green state.
   `src/striatum/web/frontend/src/shared/components/LaneEvidenceChip.tsx:3-15`
   renders only that label. The Jinja macro
   (`src/striatum/web/templates/_components.html:86-96`) accepts a
   `provenance_evidence` parameter but every caller passes the
   default — `src/striatum/web/templates/job_detail.html:73`
   explicitly passes `"not_yet_correlated"`, and
   `src/striatum/service.py:278-283` returns the muted shape from
   `_muted_lane_evidence_chip()`, wired into expected-artifact rows
   at `service.py:416` and artifact rows at `service.py:479`. The
   dashboard mirror at `src/striatum/dashboard.py:75` exposes the
   constant `LANE_EVIDENCE_NOT_YET_CORRELATED` and
   `dashboard.py:477-479` returns it unconditionally; the only
   render site (`dashboard.py:641`) calls
   `_lane_evidence_chip()` with no argument. No code path produces
   a green / `evidence_present` state.

4. **No transcript capture surfaces** — **PASS**. Grep over
   `src/striatum/web/templates/` for `stdout|stderr|transcript|live
   terminal` returns zero matches. The
   `ProcessExecutionEvidence` partial is not added in V1 (the
   dashboard renders only the privacy-safe envelope at
   `dashboard.py:634-663`, omitting any child stdout/stderr) and
   the templates never propose a live-output panel. D028 honored.

5. **Dashboard ↔ web vocabulary parity** — **PASS**.
   `tests/test_dashboard_web_parity.py:106-132` asserts that the
   set `{unattested, no_attached_supervisor, needs_revision,
   inspect_packet_with_inbox, derive_expected_byline,
   recovery_auto_publish}` appears in the status payload, the
   `dashboard --once` text, and the `/run/<id>` HTML on the same
   seeded fixture. The dashboard's canonical orderings
   (`RUN_STATE_ORDER` at `dashboard.py:28-37`,
   `ATTESTATION_REASON_ORDER` at `dashboard.py:65-73`,
   `VERDICT_ORDER` at `dashboard.py:53-58`) match
   `src/striatum/web/frontend/src/shared/types.ts:265-345`
   (`RUN_STATES`, `ATTESTATION_REASONS`, `VERDICT_KINDS`). Override
   suffix is `(override)` in `dashboard.py:447-449` and the Jinja
   macro emits the same literal `override` in `_components.html:37`.

## V1.41 next_actions consumption

The introspect hook in
`src/striatum/cli/introspect.py:877-926` emits the three V1.41
burn-down verbs (`inspect_packet_with_inbox`,
`derive_expected_byline`, `recovery_auto_publish`) as bare stem
strings into `status_payload["next_actions"]`. Both consumers
forward them verbatim:

- `src/striatum/service.py:1352` passes
  `status_payload.get("next_actions") or []` straight into
  `run_detail.html`; the template at
  `src/striatum/web/templates/run_detail.html:37-46` renders one
  `<li>{{ action }}</li>` per verb inside the
  `next-actions-banner` region (focusable via `tabindex="0"`).
- `src/striatum/dashboard.py:277` extracts `next_actions` and
  `dashboard.py:554-562` (`_render_next_actions`) prints `-
  <verb>` lines in the right column.

The verbatim-consumption contract is met; the DX concern is the
verbs themselves (see **F5** below).

## Findings (ergonomics_dx)

### F1 — `VerdictChip.tsx` puts override rationale in a default-closed `<details>` (latent §9.4 fail)

`src/striatum/web/frontend/src/shared/components/VerdictChip.tsx:21-37`
renders the override variant as `<details class="verdict-chip
verdict-chip--override">...</details>` with no `open` attribute.
`docs/design/UI_REWORK.md §9.4` is explicit: assert *no ancestor
`<details>:not([open])`* wraps the rationale node. Today only the
Jinja macro is mounted (`_components.html:36-37` renders the
rationale inline), so the regression test passes. The moment any
island mounts `VerdictChip` on `job_detail.html`, the override
visibility regression flips to fail. Default to `open` (or
restructure as `<span>` + inline rationale matching the macro)
before any island consumes this component.

### F2 — `run_detail.html:99-101` hard-codes `author: operator` and never threads `operator_label`

`src/striatum/web/templates/run_detail.html:99-101` emits
`{{ ui.byline_line("author: operator", none, false) }}` for any
session whose `lane_attestation != "attested"`. The dashboard
mirror at `src/striatum/dashboard.py:469-474` consults
`row.get("operator_label")` and prints
`author: operator [self-declared: <label>]` when present. Result:
the dashboard truthfully reveals a self-declared operator label,
the web run-detail page silently flattens it. Parity drift — both
surfaces should use the same conditional.

### F3 — `run_detail.html:84` bypasses the `posture_chip` macro

`run_detail.html:84` renders posture chips as
`<span class="posture-chip">{{ posture }}</span>` inside the
`verdicts_by_posture` list, instead of `{{ ui.posture_chip(posture)
}}`. This loses the macro's
`aria-label="review posture: ..."`, the posture-specific class
(`posture-security`, `posture-ergonomics-dx`, …) defined in
`base.css:409-418`, and the `custom:<name>` formatting. Visually
every posture pill renders neutral; screen readers announce nothing
useful. Macro is right there one line above the call.

### F4 — Jinja chip macros are not keyboard-reachable per-chip

The TypeScript components set `tabIndex={0}` on every chip
(`RunStatePill.tsx:33`, `LaneAttestationChip.tsx:49`,
`VerdictChip.tsx:45`, `BylineLine.tsx:25`,
`LaneEvidenceChip.tsx:10`, `PostureChip.tsx:13`). The shipped
Jinja macros in `_components.html` emit plain `<span>` elements
with no `tabindex` attribute, so they are not focusable on the
server-rendered surface that actually ships on
`run_list.html`/`run_detail.html`/`job_detail.html`. UI_REWORK §9.6
expects each status chip to be keyboard-reachable with a visible
focus ring. The next-actions banner gets `tabindex="0"`
(`run_detail.html:38`); the inline chips do not. Two paths: either
add `tabindex="0"` to each macro span, or document that chip-level
keyboard reach is React-island only.

### F5 — V1.41 burn-down verbs surface as raw stems with no recipe

`status_payload["next_actions"]` includes the literal strings
`inspect_packet_with_inbox`, `derive_expected_byline`,
`recovery_auto_publish`. The web renders them as `<li>` text
(`run_detail.html:42-44`) and the dashboard as `- <stem>` lines
(`dashboard.py:559-561`). A first-time operator sees three
underscored verbs with no copy-on-click recipe, no
`--run-id`/`--session-id` argument bindings, no man-page link.
RFC 0050 V1 mandates verbatim consumption, so this review is not
asking for translation — but it is asking for a follow-up to
promote each verb into a runnable recipe (UI_REWORK §3 ‘V1.41
burn-down verbs’ and §8.7 reference this). Even a `<details>` per
verb with the relevant `striatum <verb> --run-id <id>` template
would close the gap.

### F6 — No copy-on-click for mono identifiers anywhere on the run/job pages

UI_REWORK §5.4 ("Until that page lands, the link is rendered as a
copy-on-click value rather than an `<a>`.") and §7.7 ("Mono-
identifier handling — copy-on-click value") explicitly call for
copy-on-click on `supervisor_id`, `session_id`, `run_id`, `job_id`,
and the missing-required publish-artifact recipe. Today every such
identifier is rendered as bare `<code>...</code>`:

- `run_list.html:46` (`{{ run.run_id }}`),
- `run_detail.html:7` (`{{ run.run_id }}`),
- `job_detail.html:8` (`{{ job.workflow_job_id }}`),
  `:35` (`{{ job.job_id }}`),
- `_components.html:52` supervisor_id inside attestation-chip,
- `_components.html:140` publish-artifact recipe inside the
  `ExpectedArtifactsTable` partial (mirrored in TSX
  `ExpectedArtifactsTable.tsx:108-110`).

`base.js` has no clipboard handler. `island-code-viewer.js` does
(`navigator.clipboard.writeText`) but only for whole-file copy.
The DX gap: an operator who wants to paste a session/lease ID into
a `striatum recovery` command has to triple-click each `<code>`.
A 30-line vanilla-JS click handler attached to
`code[data-copy-on-click]` would cover every surface.

### F7 — `_components.html` posture-chip `custom:` branch produces a literal `custom : <name>` (extra space)

`_components.html:64-66` renders
`custom · {{ posture_text.split(":", 1)[1] }}`. The Jinja whitespace
between `custom` (line 65) and `·` collapses to a single space, but
the trailing `· <name>` produces `custom · <name>`. The TSX variant
(`PostureChip.tsx:6`) renders `custom - <name>` (hyphen). The two
surfaces disagree on the separator (`·` vs `-`). This is a parity
nit — but the `tests/test_dashboard_web_parity.py` set does not
include `custom:` postures, so it would not catch the drift.
Reconcile to one separator and pin it.

### F8 — `_components.html` attestation-chip emits supervisor_id inline; TSX hides it behind a `<details>` card

The TSX `LaneAttestationChip`
(`LaneAttestationChip.tsx:15-40`) wraps `supervisorId`/
`operatorLabel` in a `<details>` "hover/tap card", keeping the
chip compact. The Jinja macro
(`_components.html:46-55`) always renders supervisor_id and
`operator_label` inline inside the chip body. On narrow viewports
the chip will wrap to multiple lines. Either align Jinja to also
use a card (more work) or document the divergence and ensure the
chip line collapses gracefully.

### F9 — Override-rationale double rendering on job_detail (defensive but redundant)

`job_detail.html:46-54` renders the rationale twice: once inside
`{{ ui.verdict_chip(..., override_rationale) }}` (as the
`chip-detail` span) and again as `<p class="override-rationale">`.
This is defensive against the latent `VerdictChip.tsx` regression
in **F1** but adds vertical noise on every override row. Once F1
is fixed and the inline rationale is locked in (or after the React
component is verified open-by-default), drop one of the two.

### F10 — `run_detail.html:13` `run.state in ('prepared', ...)` references a state the schema no longer has

UI_REWORK §5.1 calls this out explicitly:
> "the prompt's list mentions `prepared`. The live schema does not
> have `prepared`."

`run_detail.html:13` keeps `'prepared'` in the predicate that
gates the pause/resume/cancel buttons. Dead branch — never fires.
Drop it for clarity; otherwise a future reader will spend time
chasing a phantom state.

### F11 — `run_detail.html:97` lane fallback is `"any"`; dashboard fallback is `"?"`

`run_detail.html:97` renders the session role/lane line as
`{{ session.role_id }}/{{ session.lane_id or "any" }}`. The
dashboard at `dashboard.py:589-590` falls back to `"?"`. The
parity test does not enforce that string today, but UI_REWORK §6
treats vocabulary parity as a binding rule. Pick one.

## Notes that are NOT findings

- `service.py:430-454` (`_shape_verdict_rows`) infers
  `operator_override` when the schema lacks a `source` column.
  The Gemini adversarial review flagged this as a forgery
  loophole; from the ergonomics_dx posture it is the only way to
  surface natural-but-late operator overrides on a pre-`source`-
  column database, and the inference is gated to the "accepting
  verdict following a non-accepting verdict" case. Whether to
  accept the inference is a *truthfulness* call, not a DX call;
  leaving it out of this review's verdict.
- The TSX component contracts in
  `src/striatum/web/frontend/src/shared/types.ts:262-446` mirror
  the design ceremony (closed enums, schema-aligned states,
  `displayAuthor?: never` anti-affordance on `BylineLineProps`).
  Strong DX signal: the type system refuses to compile a
  "displayAuthor override" prop.
- `tests/test_dashboard_web_parity.py:106-132` is the right
  shape — it spans status payload + dashboard text + run page
  HTML on one fixture. Suggest extending it to `job_detail.html`
  as well for §9.10 coverage of the override rationale on the
  job page (currently only the override-rationale regression
  test hits it).

## Verdict

`accept_with_findings`. The five RFC 0050 V1 regressions land
truthfully on the surface that actually ships. The findings above
are ergonomics_dx polish — keyboard reach, copy affordance, parity
of fallback strings, and the latent override-collapse risk — that
should be picked up in the V1.5 follow-up so first-time operators
get the discoverability the design ceremony promised.
