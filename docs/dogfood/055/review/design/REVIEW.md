---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0050", "v1.5", "ui-rework"]
---

author: reviewer-unknown-model-001

# Design review (ergonomics_dx): RFC 0050 V1.5 synthesis

## Verdict

`accept_with_findings` — severity `low`.

The synthesis tracks `docs/design/UI_REWORK.md` §4 + §8 V1.5 scope and the
RFC 0050 §V1.5 phase contract. Every V1.5 essential is present, no V2
work has bled in, V1 primitives are consumed (not redefined), and the new
partials follow the v1.41 byline-honest pattern surfaced by the 054b
fix-up. The two findings below are non-blocking ergonomic refinements to
help the implementer not drift toward V2 boundaries.

## Posture

Ergonomics-DX. Acceptance means a first-time implementer can pick up the
synthesis and produce screens that a first-time operator can use cold —
without rediscovering the V1.5/V2 split, the byline-honesty discipline, or
the canonical-doc citations.

## What the synthesis gets right (ergonomics_dx lens)

- **V1.5 essentials are all named and cited.**
  - `run_detail.html` — next-actions banner + recovery panel + sessions
    strip, citing `docs/design/UI_REWORK.md` §4.2 and RFC 0050 V1.5.
  - `_recovery_panel.html` — new partial for grouped blocker triage,
    server-rendered (not an island), with plain CLI recipe text.
  - `_session_chip.html` — new partial composing the V1
    `LaneAttestationChip` and `BylineLine` macros, ties to
    `docs/design/UI_REWORK.md` §8.7.
  - `job_detail.html` — `_expected_artifacts_table.html`, process evidence
    section reading `blockers.payload_json` per `docs/design/UI_REWORK.md`
    §5.9, and an override-modal stub.
  - `artifact_view.html` — byline integrity + muted provenance evidence +
    operator-on-behalf trail, per `docs/design/UI_REWORK.md` §4.11.
  - `run_posture_verdicts.html` — provenance + attestation columns, with
    override rows visually distinct, per `docs/design/UI_REWORK.md` §4.4.
  - `doctor.html` — per-record recipe text from the closed `docs/design/
    UI_REWORK.md` §4.9 mapping, with docs-only fallback for
    `reviewer_independence_unverified`.
  - `view_file.html` — `runs.branch_name` breadcrumb with the "never
    wrong-link" rule.

- **V2 boundary is held explicitly.** "V1.5 should consume those
  primitives in server-rendered screens. It must not ship V2 islands,
  override-modal logic, or copy-on-click behavior." The recovery panel is
  called out as "a server-rendered partial, not an island." Doctor recipes
  are "copyable text only." Override-modal JavaScript and mutation flow
  are deferred to V2. `workflow-graph-editor` is not mentioned, which is
  correct (it is V2 per RFC 0050 V2 list).

- **V1 primitives are consumed, not redefined.** The synthesis cites the
  V1 `LaneAttestationChip`, `BylineLine`, and `_components.html` macros
  from 054 and treats them as load-bearing. No new chip vocabulary is
  introduced. The `_session_chip.html` partial composes existing V1
  macros rather than reimplementing them.

- **Byline-honesty discipline is preserved.** `_session_chip.html` should
  "expose honest byline/inbox context without claiming that live
  attestation proves artifact authorship" — a direct echo of the 054b F1
  + F3 fixes. `artifact_view.html` byline integrity compares
  `artifacts.author_line` against publish-time `expected_author_line`,
  and provenance evidence stays `not_yet_correlated` (muted) until the
  later `process_executions ↔ artifact` lookup ships. This matches
  RFC 0050's "no falsely green provenance" rule.

- **Override rationale stays prominent.** `run_posture_verdicts.html` is
  required to keep override rationales visible and make override rows
  visually distinct — matching the 054b F2 + F4 fixes and the
  truthfulness rule from `docs/design/UI_REWORK.md` §4.4.

- **Service shaping is staged first.** The implementation order leads
  with `service.py` payload shaping for every new field (blockers,
  sessions, next actions, expected artifacts, process envelopes, byline
  integrity, verdict provenance, doctor recipes, view-file run
  candidates). Templates do not parse raw JSON ad hoc. This matches the
  V1 pattern from 054 and avoids template-side data drift.

- **Dashboard parity is named, not deferred.** `dashboard.py` is given
  its own implementation step ("recovery panel summaries, sessions strip
  data, expected artifacts when width permits, process evidence, and the
  same next-actions recipes"). This satisfies `docs/design/UI_REWORK.md`
  §8.9 and §9.9 dashboard parity, which a first-time implementer might
  otherwise treat as a "later" item.

- **Acceptance maps to the canonical checklist.** The acceptance bullet
  list pulls the V1.5-applicable rows from `docs/design/UI_REWORK.md`
  §9: browser smoke for the V1.5 templates, byline truthfulness,
  override-rationale visibility, blocker recipe honesty, dashboard
  parity, V1.41 next-actions parity, and responsive screenshots for the
  two surfaces that change shape.

## Findings (non-blocking)

### F1 — `override modal stub` boundary is under-specified

**Where.** `job_detail.html` decision: "an override modal stub. The modal
stub may reserve markup and disabled affordance state, but V2 owns the
JavaScript and mutation flow."

**Why this matters (ergonomics_dx).** RFC 0050 V1.5 names "override
modal" as in-scope, but the prompt explicitly lists "override-modal
logic" as V2. The synthesis correctly resolves this by splitting markup
from JS — but "markup and disabled affordance state" leaves three
ambiguities a first-time implementer is likely to resolve toward V2:

1. Is the HTML5 `<dialog>` element OK, or just a placeholder `<button>`?
2. May the stub include the form inputs (rationale textarea, verdict
   radio buttons) from `docs/design/UI_REWORK.md` §8.6, or only the
   trigger button?
3. Does "disabled affordance state" mean the button is always disabled
   in V1.5, or that it follows the enable rule from `docs/design/
   UI_REWORK.md` §4.3 (verdict ∈ {needs_revision, reject} AND state ∈
   {completed, waiting_human}) but never opens because there is no JS?

**Suggested clarification.** Pin the stub to: trigger button only,
following the §4.3 enable rule; no `<dialog>` body, no form inputs, no
`static/override_verdict.js` file. The button's `onclick` is a no-op (or
absent) until V2 lands `override_verdict.js`. This keeps the V1.5
deliverable to "the button is visible and correctly disabled" and lets
V2 own everything inside the modal — including the markup.

### F2 — V1.5-applicable §9 rows are described by topic, not row number

**Where.** Acceptance section: "browser smoke for `run_detail.html`,
`job_detail.html`, `run_posture_verdicts.html`, `doctor.html`,
`artifact_view.html`, and `view_file.html`; byline truthfulness;
override-rationale visibility; blocker recipe honesty; dashboard parity;
and browser/CLI parity for V1.41 next actions."

**Why this matters (ergonomics_dx).** A first-time implementer scanning
`docs/design/UI_REWORK.md` §9 must rediscover that:

- "byline truthfulness" → §9.3 (and 9.7 partial).
- "override-rationale visibility" → §9.4 (and 9.7 partial).
- "blocker recipe honesty" → §9.5 (and 9.7 partial).
- "dashboard parity" → §9.9.
- "browser/CLI parity for V1.41 next actions" → §9.10.
- "responsive screenshot coverage" → §9.2.
- "browser smoke" → §9.1, only the rows for V1.5 templates.

Naming the row numbers verbatim removes that re-derivation and prevents
the implementer from missing the responsive screenshot commit step (§9.2
explicitly requires `tests/responsive_refs/<png>.sha256` committed
alongside the PNGs).

**Suggested clarification.** Append row numbers to each bullet, e.g.
"§9.3 byline truthfulness regression; §9.4 override-rationale rendering;
§9.5 no-blocker-promises regression; §9.9 dashboard parity; §9.10 V1.41
next-actions parity; §9.2 responsive screenshots + committed
`tests/responsive_refs/*.sha256`."

## Non-findings (things the synthesis is right to leave out)

- **No copy-on-click handler.** Doctor recipes are text only; recovery
  panel recipes are pre-rendered `<code>` blocks. V2 owns the click
  handler from `docs/design/UI_REWORK.md` §7.7. Correct.
- **No SSE incremental update on the next-actions banner.** V1.5 is
  request/response; the banner reads from the V1.45.0 `next_actions`
  list. SSE is V2 optional. Correct.
- **No `process_executions ↔ artifact` correlation.** Provenance stays
  `not_yet_correlated` (muted). Correlation is the V1.7 RFC 0046 / GH
  #5 follow-up. Correct.
- **No `workflow-graph-editor` extensions.** Not mentioned. V2 owns the
  `require_attested_lane` field. Correct.
- **No `--status-compromised` chip activation.** RFC 0050 reserves the
  token but defers activation to RFC 0047 V1.5. Correct.

## Scope-discipline verification

| Prompt rejection criterion | Synthesis behavior | Verdict |
| --- | --- | --- |
| V1 primitives being redefined | Cites and consumes V1 `_components.html`, `LaneAttestationChip`, `BylineLine` macros; introduces no new chip vocabulary. | Pass |
| V2 islands bled in | "Server-rendered partial, not an island." Recovery panel and session strip are partials, not `frontend/src/islands/*`. | Pass |
| V2 override-modal logic bled in | "V2 owns the JavaScript and mutation flow." Markup-only stub. (See F1 for sharper boundary.) | Pass (with F1 refinement) |
| V2 copy-on-click bled in | "V2 recovery island, dry-run preview UI, and copy-on-click are deliberately excluded." Doctor recipes "copyable text only." | Pass |
| V2 workflow-graph-editor bled in | Not mentioned. `workflow-graph-editor` per-node attestation field stays V2. | Pass |
| V1.5 essential `run_detail` recovery panel missing | Present as `_recovery_panel.html` partial. | Pass |
| V1.5 essential `job_detail` expected-artifacts + process-evidence missing | Present as `_expected_artifacts_table.html` + process evidence section reading `blockers.payload_json`. | Pass |
| V1.5 essential `artifact_view` provenance trail missing | Present: byline integrity + muted provenance evidence + operator-on-behalf trail surfaces. | Pass |
| V1.5 essential `run_posture_verdicts` provenance + attestation columns missing | Present with override rows visually distinct. | Pass |
| V1.5 essential doctor recipes missing | Present per `docs/design/UI_REWORK.md` §4.9 closed mapping; docs-only fallback for `reviewer_independence_unverified`. | Pass |
| V1.5 essential `view_file` breadcrumb missing | Present with "never wrong-link" rule cited from §4.12 / OQ-6. | Pass |
| V1 work being redone | Synthesis explicitly consumes 054 + 054b primitives; no redefinition. | Pass |
| New partials drift from v1.41 byline pattern | `_session_chip.html` composes V1 macros and is explicitly bound to honest byline/inbox context (no live-attestation→authorship claim). Echoes 054b F1 + F3. | Pass |

## Recommendation to implementer

Treat F1 and F2 as pre-implementation clarifications — neither blocks
synthesis acceptance, but addressing them in the implementer task prompt
(or in a quick synthesis amendment) will tighten the V1.5/V2 boundary
and make §9 row coverage discoverable without re-scanning the canonical
doc.
