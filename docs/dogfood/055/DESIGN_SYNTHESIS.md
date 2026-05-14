---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/design/UI_REWORK.md", "docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md", "docs/dogfood/054/build/HANDOFF.md", "docs/dogfood/054b/build/HANDOFF.md"]
---

author: designer-unknown-model-001

# DESIGN SYNTHESIS — RFC 0050 V1.5 screen extensions

This synthesis treats `docs/design/UI_REWORK.md` as the canonical design
handoff and `docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md` as
the phase contract. Dogfood 054 and 054b already shipped the V1 primitives:
shared chip vocabulary, `_components.html`, service shaping for the first
pages, dashboard parity, semantic CSS tokens, and the four provenance-honesty
fixes. V1.5 should consume those primitives in server-rendered screens. It
must not ship V2 islands, override-modal logic, or copy-on-click behavior.

## Decisions

- **`run_detail.html`:** restructure the run home around the V1.5 elements
  named by `docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md`:
  "next-actions banner + recovery panel + sessions strip." The banner consumes
  the V1.45.0 `next_actions` list, including `inspect_packet_with_inbox`,
  `derive_expected_byline`, and `recovery_auto_publish`; the recovery panel is
  a server-rendered partial, not an island. This follows
  `docs/design/UI_REWORK.md` §4.2, which says triage decisions are made here
  and puts blockers plus next actions above the fold.

- **`_recovery_panel.html`:** add
  `src/striatum/web/templates/_recovery_panel.html` for grouped blocker
  triage. It should render human checkpoints first, blocked rows second, and
  include plain CLI recipe text derived from service payloads. The V2 recovery
  island, dry-run preview UI, and copy-on-click are deliberately excluded even
  though `docs/design/UI_REWORK.md` §8.3 names them as future optional work.

- **`_session_chip.html`:** add
  `src/striatum/web/templates/_session_chip.html` for the run sessions strip
  and the job detail session context. It should compose the V1
  `LaneAttestationChip` and `BylineLine` macros and expose honest byline/inbox
  context without claiming that live attestation proves artifact authorship.
  This is the V1.5 part of `docs/design/UI_REWORK.md` §8.7.

- **`job_detail.html`:** extend the job page with
  `src/striatum/web/templates/_expected_artifacts_table.html`, a process
  evidence section, and an override modal stub. The expected-artifacts table is
  the visible source of truth for declared logical name, kind, path, required
  flag, expected author line, published status, hash, actual byline, and byline
  drift. The process evidence section renders only privacy-safe diagnostic
  envelopes from `blockers.payload_json`, as required by
  `docs/design/UI_REWORK.md` §5.9. The modal stub may reserve markup and
  disabled affordance state, but V2 owns the JavaScript and mutation flow.

- **`artifact_view.html`:** extend the artifact page with byline integrity,
  muted provenance evidence, and operator-on-behalf trail surfaces. Byline
  integrity compares `artifacts.author_line` with the publish-time expected
  author line. Provenance remains `not_yet_correlated` until the later
  process-execution-to-artifact lookup exists; the page may surface
  `recovery.auto_published` and
  `provenance.publish_without_process_execution` event context as an
  operator-on-behalf trail. This implements `docs/design/UI_REWORK.md` §4.11
  without emitting a green provenance claim.

- **`run_posture_verdicts.html`:** add provenance and attestation columns, and
  make override rows visually distinct from natural verdicts. Attestation is
  creation-time evidence, not the session's current state. Override rationales
  stay visible, matching `docs/design/UI_REWORK.md` §4.4 and the dogfood-054b
  fix that removed inferred override claims.

- **`doctor.html`:** extend each problem record with a per-record recipe from
  the closed mapping in `docs/design/UI_REWORK.md` §4.9. Recipes are copyable
  text only in V1.5. Records without mechanical recovery, such as
  `reviewer_independence_unverified`, should link to docs rather than inventing
  a command.

- **`view_file.html`:** add a path-context breadcrumb that heuristically
  matches the viewed path against `runs.branch_name`. Per
  `docs/design/UI_REWORK.md` OQ-6, the rule is "never wrong-link": hide the
  run breadcrumb unless the match is unambiguous.

- **`service.py` and `dashboard.py`:** shape all new page payloads in
  `src/striatum/service.py` so templates do not parse raw JSON ad hoc. Extend
  `src/striatum/dashboard.py` for text-mode parity where V1.5 adds operator
  signal: recovery panel summaries, sessions strip data, expected artifacts
  when width permits, process evidence, and the same next-actions recipes.

## Implementation Order

1. Shape service payloads for run detail blockers, sessions, next actions,
   expected artifacts, process envelopes, byline integrity, verdict provenance,
   doctor recipes, and view-file run candidates.
2. Add `_recovery_panel.html`, `_session_chip.html`, and
   `_expected_artifacts_table.html`.
3. Wire `run_detail.html`, then `job_detail.html`, then `artifact_view.html`.
4. Extend `run_posture_verdicts.html`, `doctor.html`, and `view_file.html`.
5. Add `dashboard.py` parity for the same vocabulary.
6. Add focused tests from `docs/design/UI_REWORK.md` §9 that apply to V1.5
   screens.

## Acceptance

Use the V1.5-applicable rows in `docs/design/UI_REWORK.md` §9: browser smoke
for `run_detail.html`, `job_detail.html`, `run_posture_verdicts.html`,
`doctor.html`, `artifact_view.html`, and `view_file.html`; byline truthfulness;
override-rationale visibility; blocker recipe honesty; dashboard parity; and
browser/CLI parity for V1.41 next actions. Responsive screenshot coverage
should include `/run/<id>` with a populated recovery panel and
`/run/<id>/job/<wfjob>` with expected artifacts populated.
