---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/design/UI_REWORK.md", "docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md", "CHANGELOG.md"]
---

author: designer-unknown-model-001

# DESIGN SYNTHESIS — RFC 0050 V1 operator UI scope

This synthesis treats `docs/design/UI_REWORK.md` as the canonical design input.
It does not re-derive the design ceremony; `docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md`
explicitly adopts that handoff and scopes dogfood-054 to V1 primitives plus
dashboard parity.

## Decisions

- **Shared component library:** ship
  `src/striatum/web/frontend/src/shared/components/RunStatePill.tsx`,
  `JobStatePill`, `VerdictChip`, `LaneAttestationChip`, `PostureChip`,
  `BylineLine`, `LaneEvidenceChip`, and `ExpectedArtifactsTable`, with shared
  enums in `src/striatum/web/frontend/src/shared/types.ts`. The closed enum
  behavior comes from `docs/design/UI_REWORK.md §5`; `LaneEvidenceChip` follows
  RFC 0050 V1 by rendering only `not_yet_correlated` muted until the later
  process-execution-to-artifact lookup exists.

- **Jinja2 component partial:** add
  `src/striatum/web/templates/_components.html` with macros for
  `run_state_pill`, `job_state_pill`, `verdict_chip`,
  `lane_attestation_chip`, `posture_chip`, and `byline_line`. Refactor the V1
  server-rendered chip usages called out in `docs/design/UI_REWORK.md §8.1`
  only where needed for run-list, run-detail, and job-detail payload parity;
  the larger V1.5 screen restructures stay out of this phase.

- **Expected artifacts partial:** add
  `src/striatum/web/templates/_expected_artifacts_table.html`. It renders the
  work packet's declared `expected_artifacts` next to matching published
  artifact rows, including required status, expected byline, actual byline,
  hash, and byline-drift state, per `docs/design/UI_REWORK.md §5.8` and
  `docs/design/UI_REWORK.md §8.4`.

- **Service payload shaping:** update `src/striatum/service.py` page shapers for
  `run_list`, `run_detail`, and `job_detail` so templates receive canonical chip
  payloads: run/job state, verdict provenance and rationale, lane attestation
  reason, expected artifacts, actual artifacts, and parsed process-adapter
  diagnostic envelopes. This is shape work only; RFC 0050 says V1 does not add a
  new mutation path outside the existing service gate.

- **Process evidence stub:** render a muted provenance-evidence value on
  `artifact_view.html` and parsed blocker envelopes on `job_detail.html` only to
  the extent needed for V1 tests. `docs/design/UI_REWORK.md §8.5` is binding:
  stdout/stderr and model output are never rendered, and green evidence waits
  for GH #5 / V1.7.

- **Dashboard parity:** update `src/striatum/dashboard.py` to share the same
  vocabulary in text mode: `RUN_STATE_ORDER`, `ATTESTATION_REASON_ORDER`,
  attestation chips, verdict override suffixes, blocker kind lines, expected
  artifact rows when width permits, and process-envelope blocks. The dashboard
  must consume the same `next_actions` source as the web view, per
  `docs/design/UI_REWORK.md §8.9` and RFC 0050 V1.

- **CSS semantic tokens:** extend `src/striatum/web/static/base.css` with the
  semantic status, attestation, override, and muted evidence tokens from
  `docs/design/UI_REWORK.md §7.3`. Keep literal colors inside token definitions;
  component CSS should use tokens, stable `data-component` attributes, compact
  table density, and no new dominant palette.

- **Tests:** add the V1 RFC-mandated regressions:
  `tests/test_dashboard_web_parity.py::test_dashboard_web_parity`,
  a byline regression test that proves unattested sessions render
  `author: operator` in web and dashboard surfaces, and an override-rationale
  regression that proves `operator_override` rationales render beside the chip
  in both surfaces. Existing `tests/test_next_actions_v141_burndown.py` remains
  the backend-hook prerequisite documented in `CHANGELOG.md`.

## Implementation order

1. Add shared type definitions and component renderers from
   `docs/design/UI_REWORK.md §5`, including `LaneEvidenceChip` in muted-only V1
   form.
2. Add `templates/_components.html`, then refactor current inline chip markup to
   the macros before changing page payload shapes.
3. Add the semantic tokens and compact table styles in `base.css`, because the
   macros and TS components should reference tokens from their first commit.
4. Shape `service.py` payloads for run list, run detail, and job detail:
   canonical chips first, then expected-artifacts rows, then parsed diagnostic
   envelopes.
5. Add `_expected_artifacts_table.html` and the minimal process-evidence stub
   needed by V1 truthfulness checks.
6. Extend `dashboard.py` text renderers to match the web chip vocabulary and
   reuse `next_actions`.
7. Add the three RFC 0050 V1 regression tests, then broaden with any
   `docs/design/UI_REWORK.md §9` rows that are V1-applicable and cheap to pin.

## Acceptance

Acceptance is the V1 subset of `docs/design/UI_REWORK.md §9`, plus RFC 0050's
three named regressions. The implementer should treat `docs/design/UI_REWORK.md §9.3`
as the byline truthfulness requirement, `docs/design/UI_REWORK.md §9.4` as the
override-rationale requirement, and `docs/design/UI_REWORK.md §9.9` plus
`§9.10` as dashboard/web parity and shared-next-actions requirements.

The explicit V1 test names are:

- `tests/test_dashboard_web_parity.py::test_dashboard_web_parity`
- `test_byline_regression`
- `test_override_rationale_regression`

Passing V1 means the shared components, Jinja2 partials, `service.py` payload
shape, `dashboard.py` text-mode vocabulary, CSS tokens, and the three
regression tests all land together. V1.5 template restructuring and V2 islands
remain follow-up dogfoods under RFC 0050.
