---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0050", "v2", "ui-rework"]
---

author: reviewer-unknown-model-001

# Design review (ergonomics_dx): RFC 0050 V2 synthesis

## Verdict

`accept_with_findings` — severity `low`.

The synthesis tracks `docs/design/UI_REWORK.md` §5.7, §7.7, §8.3, §8.6, §8.8,
and §9 V2-applicable rows, plus the RFC 0050 V2 phase contract. The V1/V1.5
boundary is held — 054, 054b, 055, and 055b deliverables are explicitly
consumed, not redefined. The reactflow ViewportPortal viewport-locked overlay
(GH #6) is explicitly out of scope, and the graph-editor work is correctly
pinned to data-binding only. Findings are non-blocking ergonomic refinements
that tighten implementer affordances around modal accessibility, toast timing,
and bundle-hash discipline.

## Posture

Ergonomics-DX. Acceptance means a first-time implementer can pick up the
synthesis and produce four small interactive surfaces (copy-on-click, override
modal, recovery-panel island, graph-editor field) that a first-time operator
can use cold — without re-scanning `docs/design/UI_REWORK.md`, missing an
accessibility affordance, or accidentally invoking deferred V1.7 / reactflow
v12 work.

## What the synthesis gets right (ergonomics_dx lens)

- **V2 boundary is held explicitly.** The scope-boundary paragraph names
  the four prior dogfoods (054, 054b, 055, 055b) as shipped and excludes
  V1.5 template extensions and target-repository template catalog
  extensions. The RFC's reactflow-v12 non-goal is quoted verbatim:
  "`ViewportPortal` viewport-locked overlays" — so the V2 deliverable for
  `workflow-graph-editor` is explicitly data-binding only. This is the
  load-bearing constraint and the synthesis pins it twice (in scope
  boundary and in decision 6).

- **All four interactive deliverables are named and cited.**
  - `copy_on_click.js` — cites §7.7 with the exact identifier regex
    `^(run|job|sess|art|proc|super|lease)_[0-9a-f]+$`. No per-template
    wiring. Matches `docs/design/UI_REWORK.md` §7.7 verbatim.
  - `override_verdict.js` — cites §8.6 with `<dialog>` semantics,
    `/v1/invoke` POST, focus trap, Escape/close, initial focus, focus
    return, and parameter filtering.
  - `recovery-panel` island — cites §8.3 with the no-JS fallback rule
    quoted verbatim ("The island is optional -- the page renders
    correctly without JS") and bounded to dry-run preview only ("must
    not publish artifacts").
  - `workflow-graph-editor` per-node `require_attested_lane: bool` —
    cites §8.8 and rejects for non-review per SPEC §Reviewer
    Independence. Viewport overlay defers to React Flow v12 + GH #6.

- **Parameter filtering on the override modal is explicit.** Decision 3
  pins the modal to collect "only `verdict`, `rationale`, optional
  `findings_artifact_id`, and `auto_fresh_session`" with "session/job
  identifiers come from server-rendered data attributes, not
  user-editable fields." This forecloses the "sends arbitrary payloads"
  rejection criterion at the source.

- **Recovery recipes stay copy-first.** Decision 2 quotes §5.7
  ("Copy-on-click recipes are always available.") and bounds the island
  to enhancement: "The island should improve copying and previewing, not
  turn every recovery path into a mutation button." The dry-run preview
  is correctly characterized as "read-shaped by intent" — it does not
  cross the V1.41 / RFC 0029 mutation gate.

- **`base.js` initializer is bounded.** Decision 5 says the copy-on-click
  initializer must be "idempotent so pages with island hydration and
  ordinary Jinja2 pages behave the same," and the existing UTC/Local
  toggle stays. This avoids the failure mode where the island bundle
  silently double-registers a global handler or breaks server-rendered
  pages.

- **Visual system constraints are preserved.** Decision 7 holds §7
  invariants: "semantic tokens, visible focus, compact table/panel
  density, and no purple-dominant theme beyond the override semantic
  marker." `base.css` additions are bounded to "small modal, toast, and
  copy-token styles" — no new file, no new vocabulary.

- **Implementation order is dependency-ordered.** Start with
  `copy_on_click.js` + `base.js` (the shared affordance the recovery
  panel and doctor / job recipes all rely on), then
  `override_verdict.js` (self-contained, high provenance risk —
  keyboard + parameter-filter tests land alongside it), then the
  recovery panel island (reuses V1.5 server payload — does not invent
  a second recovery model), and finish with the graph-editor field
  (deliberately boring: field control, node-body render, round-trip
  persistence). This is the order a first-time implementer would
  rediscover, and naming it removes that rediscovery step.

- **Acceptance maps to the canonical §9 rows.** §9.1, §9.2, §9.4, §9.6,
  §9.7, §9.8, and §9.10 are pulled out with one-line summaries. The
  V2-specific assertions (island mount, modal script hooks, graph editor
  node-field rendering, modal focus restoration, no viewport-attestation
  claim) are all named.

## Findings (non-blocking)

### F1 — Modal ARIA wiring is implicit; `<dialog>` alone is not enough

**Where.** Decision 3: "It needs `<dialog>` behavior, Escape/close
handling, initial focus, focus return, and a focus trap."

**Why this matters (ergonomics_dx).** The HTML5 `<dialog>` element
provides an implicit `role="dialog"` but not `aria-labelledby`,
`aria-describedby`, or labels on the form controls inside. The review
prompt's rejection criterion lists "ARIA" as a sibling of "focus trap"
and "arbitrary payloads" — synthesis covers focus trap and parameter
filtering explicitly, but a first-time implementer reading "`<dialog>`
behavior" is likely to assume the native element handles ARIA on its
own. `docs/design/UI_REWORK.md` §9.6 mentions "labels" in the keyboard
/ accessibility acceptance row, but the deliverable description does
not name:

1. `aria-labelledby` pointing at the modal title heading.
2. `aria-describedby` pointing at the rationale-helper text (if any).
3. `aria-label` / `aria-describedby` on the verdict radio group and
   the rationale textarea.
4. `aria-live="polite"` or equivalent on the post-submit feedback (so
   screen readers announce the override outcome).

**Suggested clarification.** Append one line to decision 3: "The modal
title carries an `id` referenced by the `<dialog>`'s `aria-labelledby`,
the rationale textarea has a programmatic `<label>` (not placeholder
text), and the post-submit feedback uses `aria-live='polite'`. The
verdict radio group has a `<fieldset>` + `<legend>` or `role='radiogroup'`
with `aria-labelledby`." This pulls §9.6 "labels" into the deliverable
description so the implementer does not derive it from the acceptance
row alone.

### F2 — Toast duration in `copy_on_click.js` is under-specified

**Where.** Decision 4: "The implementation should add hover/focus cues
and a short confirmation without requiring per-template wiring."

**Why this matters (ergonomics_dx).** `docs/design/UI_REWORK.md` §7.7
pins the toast to "a 1.2-second toast confirming." "Short" is a category;
1.2 seconds is a value. A first-time implementer will pick a default
(commonly 2-3 seconds, sometimes 800 ms) and the surface will diverge
from the canonical spec without anyone noticing until §9.6 a11y review.

**Suggested clarification.** Replace "short confirmation" with "a
1.2-second toast confirming the copy, per `docs/design/UI_REWORK.md`
§7.7." Same effort, same shape, no re-derivation.

### F3 — Bundle-hash discipline cited but not threaded into the deliverable

**Where.** Acceptance bullet: "§9.8 bundle refusal: run the UI build and
preserve committed bundle-hash discipline."

**Why this matters (ergonomics_dx).** The graph-editor extension
(decision 6) is the V2 deliverable that ships through the committed
bundle (`src/striatum/web/static/build/manifest.sha256`). A first-time
implementer who reads decision 6 in isolation will land the TypeScript
change and the round-trip serializer test, then forget to `make
ui-build` and commit the new manifest hash, and CI will refuse the PR.
The synthesis names §9.8 in the acceptance bullets but does not connect
"graph-editor field edit" → "rebuild bundle + commit manifest" inside
decision 6 itself. The recovery-panel island has the same exposure
(it lives under `frontend/src/islands/recovery-panel/`).

**Suggested clarification.** Append to decisions 1 and 6 (the two
deliverables that touch `frontend/`): "Re-run `make ui-build` and
commit the updated `src/striatum/web/static/build/manifest.sha256` and
bundle artefact. CI refuses drift via §9.8." This makes the bundle
step visible at the same scroll position as the code change, not in a
separate acceptance bullet.

### F4 — `recovery auto-publish --dry-run` invoke route is named without an explicit gate-fail rendering rule

**Where.** Decision 1: "The island should call `recovery auto-publish
--run-id <r> --dry-run` through the loopback invoke path only when the
operator asks for a preview, then render the would-publish rows and
gate reasons."

**Why this matters (ergonomics_dx).** `docs/design/UI_REWORK.md` §5.7
specifies that `BlockerTriagePanel` "never invents an unsupported
recovery path. If the blocker doesn't map to a known recipe, the panel
renders the diagnostic envelope and the literal next-action list from
the runner." The synthesis names "gate reasons" but does not pin the
two failure modes a first-time implementer needs to design for:

1. The auto-publish gate refuses every row (no rows would be
   published). The island must render the gate refusal verbatim from
   the dry-run output — not infer or paraphrase.
2. The `/v1/invoke` call itself fails (mutations disabled, network
   error, etc.). The island must fall back to the pre-rendered
   `<code>` recipe and leave a visible error chip — not silently
   succeed or claim a preview was rendered when none was.

**Suggested clarification.** Append to decision 1: "Render the dry-run
output verbatim — including gate-refusal reasons; never paraphrase.
When the `/v1/invoke` call fails (mutations disabled or transport
error), surface the failure as a `not_yet_correlated`-style muted chip
and keep the pre-rendered `<code>` recipe visible as the operator
fallback. The island must never silently substitute its own
mock-preview when the runner refuses."

## Non-findings (things the synthesis is right to leave out)

- **No `PhaseBands` reactflow ViewportPortal work.** Decision 6
  explicitly defers viewport-positioned attestation overlays to React
  Flow v12, citing GH #6. RFC 0050 lists this as a non-goal. Correct.

- **No SSE live region for the next-actions banner.** RFC 0050 V2 lists
  "SSE live region" as an optional V2 extension, not a V2 hard
  requirement. The synthesis is silent on SSE, which is correct — the
  banner ships server-rendered today and the V2 SSE work is deferrable.

- **No mutation outside `/v1/invoke`.** Override modal posts through
  the existing `POST /v1/invoke` whitelist; recovery dry-run preview
  reads through the same path with `--dry-run`. No new mutation
  routes, no D058 / D083 owner-only loopback expansion. Correct.

- **No `--status-compromised` token surfacing.** RFC 0050 reserves the
  token but defers activation to RFC 0047 V1.5 + RFC 0046 V1.7. The
  synthesis does not mention it, which is correct.

- **No V1.7 `process_executions ↔ artifact` correlation.** The
  recovery-panel island is bounded to dry-run preview rendering, not
  provenance-evidence chip activation. The chip stays
  `not_yet_correlated` (muted) until V1.7. Correct.

- **No template-catalog extensions or target-repository workflow
  fixtures.** RFC 0050 V2 is operator-UI surface only; the scope
  boundary explicitly excludes target-repository template catalog
  extensions. Correct.

## Scope-discipline verification

| Prompt rejection criterion | Synthesis behavior | Verdict |
| --- | --- | --- |
| Reactflow ViewportPortal viewport-locked work bled in | Decision 6: "Do not implement the viewport-positioned attestation overlay; that waits for React Flow v12 per GH #6." Scope boundary also rejects "React Flow viewport overlays." | Pass |
| V1 work being redone | Scope boundary names 054 + 054b deliverables (chip vocabulary, byline/provenance primitives, dashboard parity, semantic CSS tokens, attestation-drift / dashboard-rationale honesty fixes) as shipped. V2 consumes them. | Pass |
| V1.5 work being redone | Scope boundary names 055 + 055b deliverables (server-rendered recovery panel, expected-artifacts table, process evidence, artifact provenance trail, posture verdict provenance, doctor recipes, view-file breadcrumb, attestation + override evidence handling) as shipped. V2 enhances, does not redo. | Pass |
| Modal missing focus trap | Decision 3: "It needs `<dialog>` behavior, Escape/close handling, initial focus, focus return, and a focus trap." | Pass |
| Modal missing ARIA | Decision 3 names `<dialog>` semantics and acceptance row §9.6 covers "labels." Implicit only — see F1. | Pass (with F1 refinement) |
| Modal sends arbitrary payloads | Decision 3: "collect only `verdict`, `rationale`, optional `findings_artifact_id`, and `auto_fresh_session`; session/job identifiers come from server-rendered data attributes, not user-editable fields." Tests required to "assert it never sends extra form fields." | Pass |
| Copy-on-click regex diverges from §7.7 | Decision 4 quotes the regex verbatim: `^(run|job|sess|art|proc|super|lease)_[0-9a-f]+$`. | Pass |
| Recovery panel publishes artifacts | Decision 1: "It must not publish artifacts." Dry-run preview only. | Pass |
| Graph editor extends beyond `require_attested_lane` | Decision 6 bounds the field to `require_attested_lane: bool` for review jobs; rejects for non-review jobs per SPEC §Reviewer Independence; renders in node body; round-trips through serializer. | Pass |
| Recovery recipes change from copy-first to mutation-first | Decision 2 quotes §5.7 ("Copy-on-click recipes are always available.") and bounds the island to enhancement. | Pass |
| New chip vocabulary introduced | None. The synthesis consumes the V1 + V1.5 chip set and CSS tokens. `base.css` additions are bounded to "small modal, toast, and copy-token styles." | Pass |
| §9 row coverage missing for V2 deliverables | §9.1, §9.2, §9.4, §9.6, §9.7, §9.8, §9.10 named with V2-specific assertions. | Pass |

## Recommendation to implementer

Treat F1–F4 as pre-implementation clarifications — none block synthesis
acceptance, but addressing them in the implementer task prompt (or a
short synthesis amendment) will:

- **F1.** Make the modal pass `tests/test_web_a11y.py` (Playwright +
  axe-core) on the first try, without iterating against the §9.6 acceptance
  row.
- **F2.** Hold the canonical 1.2-second toast duration so the surface
  does not silently diverge from `docs/design/UI_REWORK.md` §7.7.
- **F3.** Surface the `make ui-build` + `manifest.sha256` commit step at
  the same scroll position as the code change, so a first-time
  implementer does not hit CI bundle-drift refusal.
- **F4.** Pin the two failure modes for the dry-run preview (gate
  refusal, invoke transport error) so the island does not silently
  substitute its own mock or claim a preview was rendered when the
  runner refused.

The synthesis is ready for implementation. The V2 scope is correctly
isolated, the load-bearing rejection criteria (reactflow ViewportPortal,
V1/V1.5 redo, arbitrary modal payloads) are all foreclosed, and the
implementation order maps cleanly onto the four-deliverable surface.
