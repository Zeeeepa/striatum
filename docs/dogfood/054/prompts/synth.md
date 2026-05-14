# Synth — RFC 0050 V1 phase scope

**Canonical input:** `docs/design/UI_REWORK.md` (1845-line Claude
Design handoff). Treat it as the design output — do NOT re-derive.

**RFC 0050** confirms three-phase landing:
- **V1 (this dogfood):** shared components, templates partial,
  service.py payload shaping, dashboard.py text-mode parity,
  CSS semantic tokens, regression tests.
- **V1.5 (next dogfood):** template extensions (run_detail,
  job_detail, artifact_view, run_posture_verdicts, doctor,
  view_file).
- **V2 (after that):** islands (recovery-panel, override modal,
  copy_on_click), workflow-graph-editor `require_attested_lane`
  per-node data binding.

Produce `docs/dogfood/054/DESIGN_SYNTHESIS.md` that names the exact
V1 deliverables. Sections:

1. **Decisions** — for each V1 deliverable, which file lands and
   what shape (don't paste code; name the file + 1-2 sentences).
2. **Implementation order** — list the V1 work items in the order
   the implementer should touch them so each step has its
   dependencies already in place.
3. **Acceptance** — point at UI_REWORK.md §9 V1-applicable rows +
   the three regression tests RFC 0050 mandates
   (test_dashboard_web_parity, test_byline_regression,
   test_override_rationale_regression).

500-900 words. Cite the source-of-truth doc paths verbatim
(`docs/design/UI_REWORK.md §5`, etc).
