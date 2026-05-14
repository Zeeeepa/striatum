# Synth — RFC 0050 V1.5 phase scope

**Canonical input:** `docs/design/UI_REWORK.md` (1845-line handoff).
Treat it as the design output — do NOT re-derive.

**V1 already shipped** (`docs/dogfood/054/build/HANDOFF.md` +
`docs/dogfood/054b/build/HANDOFF.md`): shared TypeScript
components + Jinja `_components.html` macros + service.py payload
shaping + dashboard.py text-mode parity + CSS semantic tokens +
4 V1 non-negotiable fixes (byline forgery, inferred-override
removal, attestation recording-time, dashboard rationale).

**V1.5 (this dogfood):** template extensions that consume the V1
primitives. Per RFC 0050 §V1.5 + UI_REWORK.md §4 (Screen
Specifications) + §8 (Implementation Map):

- `run_detail.html` restructure: next-actions banner + recovery
  panel + sessions strip. Consume the V1.45.0 `next_actions`
  list including `inspect_packet_with_inbox`,
  `derive_expected_byline`, `recovery_auto_publish`.
- `job_detail.html` extend: `_expected_artifacts_table.html`
  partial (new) + process-evidence section + override-modal
  stub (the modal itself ships in V2).
- `artifact_view.html` extend: byline integrity surface +
  provenance + operator-on-behalf trail (the
  `recovery.auto_published` + `provenance.publish_without_process_execution`
  events from V1.41 / RFC 0046 should surface here).
- `run_posture_verdicts.html` extend: provenance + attestation
  columns; override rows visually distinct (closes codex's
  dogfood-054 needs_revision finding deferred to V1.5).
- `doctor.html` extend: per-record recipes (operator next
  actions per problem record).
- `view_file.html` breadcrumb: heuristic match against
  `runs.branch_name` to link back to the run context. Never
  wrong-link (per UI_REWORK.md OQ-6 default).
- New partials: `_recovery_panel.html`, `_session_chip.html`,
  `_expected_artifacts_table.html`.
- `service.py` payload shaping for the new sections.
- `dashboard.py` text-mode parity for any new dashboard widgets.

**V2 scope (next dogfood):** islands (`recovery-panel`),
override modal, copy-on-click. NOT in this dogfood.

Produce `docs/dogfood/055/DESIGN_SYNTHESIS.md`:

1. **Decisions** — for each template / partial, which file lands
   + 1-2 sentences on shape.
2. **Implementation order** — dependency-correct sequence.
3. **Acceptance** — point at UI_REWORK.md §9 V1.5-applicable rows.

500-900 words. Cite source-of-truth doc paths verbatim.
