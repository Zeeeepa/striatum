# Synth — RFC 0050 V2 phase scope

**Canonical input:** `docs/design/UI_REWORK.md` (1845 lines).

V1 and V1.5 have shipped (dogfoods 054, 054b, 055). V2 is the
interactive layer.

**V2 deliverables** (per RFC 0050 §V2 + UI_REWORK.md §5 + §8):

- `frontend/src/islands/recovery-panel/` new island. Dry-run
  preview for `recovery auto-publish` (calls the verb with
  `--dry-run` from the loopback API, shows would-publish list).
  Copy-on-click for the CLI recipes the panel surfaces.
- `static/override_verdict.js` — modal + `POST /v1/invoke`.
  Modal is keyboard-driven, has focus trap, ARIA roles, sends
  only the allowed parameters (verdict, rationale,
  findings_artifact_id, auto_fresh_session).
- `static/copy_on_click.js` — identifier copy-on-click
  affordance (run_id, job_id, art_id, sess_id). Visible cue
  on hover/focus.
- `workflow-graph-editor` extension: per-node
  `require_attested_lane` field **DATA BINDING ONLY** (store
  + render in node body). NO viewport-locked overlay (GH #6:
  reactflow ViewportPortal not available until v12).
- `base.js` extension to wire copy-on-click globally.
- `base.css` token additions if needed for modal / panel.

Produce `docs/dogfood/056/DESIGN_SYNTHESIS.md`:

1. Decisions per deliverable.
2. Implementation order.
3. Acceptance — point at UI_REWORK.md §9 V2-applicable rows.

500-900 words. Cite source-of-truth paths.
