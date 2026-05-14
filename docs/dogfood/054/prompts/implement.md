# Implement — RFC 0050 V1

Blocked until `review_design` returns an accepting verdict.

**Canonical inputs:**
- `docs/design/UI_REWORK.md` — full spec (1845 lines). V1 scope per
  RFC 0050.
- `docs/dogfood/054/DESIGN_SYNTHESIS.md` — phase scope you must
  hit.
- `docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md` —
  governing RFC.

**Write scope:** `src/striatum/web/`, `src/striatum/service.py`,
`src/striatum/dashboard.py`, `tests/`,
`docs/dogfood/054/build/`. No writes to `.striatum/`, `go/`, prior
dogfoods.

**V1 deliverables** (per synthesis + UI_REWORK.md §5 + §8 + §9):

1. **Shared components** under
   `src/striatum/web/frontend/src/shared/components/`:
   - `RunStatePill` (closed enum: prepared,
     needs_branch_confirmation, ready, running, paused,
     completed, failed, canceled; reserve compromised token for
     V1.7).
   - `JobStatePill` (closed enum: queued, blocked, ready,
     claimed, running, completed, failed, canceled, skipped,
     stale_lease, waiting_human).
   - `VerdictChip` (variants accept / accept_with_findings /
     needs_revision / reject; **provenance slot** for
     natural vs operator-override).
   - `LaneAttestationChip` (attested / unattested with reason
     sub-text: session_missing, no_attached_supervisor, pid_gone,
     pid_identity_mismatch, lane_command_missing, run_mismatch,
     session_mismatch).
   - `PostureChip` (threat_model, ergonomics_dx, adversarial,
     neutral).
   - `BylineLine` (renders canonical
     `author: <role>-<model>-<ord>` or
     `author: operator [self-declared: <label>]`; refuses
     free-text substitution).
   - `LaneEvidenceChip` — V1 always renders `not_yet_correlated`
     muted state. NEVER green pre-correlation per RFC 0050.
   - `ExpectedArtifactsTable` (declared path / kind /
     logical_name / required).
   - TypeScript types in `frontend/src/shared/types.ts` matching
     UI_REWORK.md §5 closed enums.

2. **Jinja2 macros partial:**
   `src/striatum/web/templates/_components.html` — same
   vocabulary as the TS components so server-rendered surfaces
   and islands speak the same chips.

3. **service.py page-payload shaping** for `run_list`, `run_detail`,
   `job_detail`: include attestation reason, verdict provenance,
   override rationale, byline line in the page payload so the
   templates can hand it to `_components.html` macros directly.

4. **dashboard.py text-mode parity:** use the same chip vocabulary
   as ASCII glyphs; consume the V1.45.0 `next_actions` list
   (`inspect_packet_with_inbox`, `derive_expected_byline`,
   `recovery_auto_publish`) verbatim. No new output format —
   render the new chips in the existing dashboard structure.

5. **CSS semantic tokens** in `src/striatum/web/static/base.css`:
   `--status-running`, `--status-blocked`, `--status-completed`,
   `--status-failed`, `--status-canceled`, `--status-paused`,
   `--attestation-attested`, `--attestation-warn`,
   `--override-marker`, `--evidence-not-yet-correlated`. Reserve
   `--status-compromised` but do not use it (V1.7).

6. **Regression tests:**
   - `tests/test_dashboard_web_parity.py` — for a single fixture
     run, the dashboard and `/run/<id>` page payload contain the
     same chips with the same labels.
   - `tests/test_byline_regression.py` — an unattested-session
     fixture renders `author: operator` (NOT a model byline) on
     both surfaces. No template path emits a model byline for
     that session.
   - `tests/test_override_rationale_regression.py` — a
     fixture verdict with `source = 'operator_override'` renders
     the rationale beside the pill on both surfaces.

**Sub-agents:** use them aggressively — one per concern (components,
Jinja partial, service.py payload, dashboard.py, CSS, each test
file). Reconcile sub-agent output before writing HANDOFF.

**Tools to know:**
- `striatum byline --session-id <s> --job-id <j>` returns the
  canonical expected byline (V1.41 burn-down).
- `striatum inbox --session-id <s>` returns the current packet
  shape — use this for the fixture data shape.
- `striatum publish-artifact` defaults `--kind` and
  `--logical-name` from `expected_artifacts` (V1.41).
- For test fixtures, leverage existing `tests/test_cli_mvp.py`
  helpers and `tests/_harness/`.

**HANDOFF:** `docs/dogfood/054/build/HANDOFF.md`. Byline must
match the expected_author_line from your packet (use
`striatum byline` to derive it). Summarize shipped scope, test
results, deviations.

**V2.0 follow-ups:** Anything that should land in V1.5 or V2 of
this RFC, name explicitly. Do NOT silently expand scope.
