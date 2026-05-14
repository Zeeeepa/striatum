# Build Review — RFC 0050 V1

Read:
- `docs/dogfood/054/DESIGN_SYNTHESIS.md`
- `docs/dogfood/054/build/HANDOFF.md`
- The source files the HANDOFF cites.
- `docs/design/UI_REWORK.md` §9 (V1-applicable rows).

Posture supplied in your work packet (`threat_model`,
`ergonomics_dx`, adversarial). Write to assigned
`docs/dogfood/054/review/build/<lane>/REVIEW.md` with v1 finding
front matter.

**Required regression checks** (RFC 0050 non-negotiable):

1. **Byline regression** — there is NO template path that renders
   `author: <role>-<model>-<ord>` for a session with
   `lane_attestation = 'unattested'`. Inspect every shared
   component, Jinja macro, and service.py payload-shaper.
2. **Override rationale prominence** — every `VerdictChip`
   render of a verdict with `source = 'operator_override'`
   shows the rationale beside the pill. No silent substitution
   for the original natural verdict.
3. **LaneEvidenceChip muted** — V1 always renders
   `not_yet_correlated` (muted CSS token). No code path produces
   a green / `evidence_present` state pre-correlation.
4. **No transcript capture surfaces** — no template proposes a
   "live terminal output" panel, no streaming of supervised
   stdout/stderr by default. D028 honored.
5. **Dashboard ↔ web vocabulary parity** — same chip labels +
   same attestation reasons + same next_actions on both
   surfaces.

**V1.41 next_actions consumption check** — the V1.45.0
introspect hook emits `inspect_packet_with_inbox`,
`derive_expected_byline`, `recovery_auto_publish`. Verify the
dashboard and `service.py` payload consume them verbatim.

Cite file:line for every finding. Verdict: accept /
accept_with_findings / needs_revision.
