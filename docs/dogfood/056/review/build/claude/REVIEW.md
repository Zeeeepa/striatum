---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0050", "v2", "build-review", "operator-composed"]
---

author: reviewer-unknown-model-002

# Build Review: RFC 0050 V2 — Ergonomics Pass

**Reviewer:** Claude (ergonomics_dx posture)
**Composed by:** operator (claude lane stalled — recurring no-publish anti-pattern;
review composed by operator after reading `docs/dogfood/056/build/HANDOFF.md`,
the on-disk implementations under `src/striatum/web/static/` +
`src/striatum/web/frontend/src/islands/`, and cross-checking with codex's natural
accept_with_findings and gemini's adversarial accept_with_findings on the same
surface.)

## Verdict

**accept_with_findings**

The V2 implementation closes the 6 deliverables named in the synthesis with
discoverable affordances. Override modal, copy-on-click, recovery-panel island,
and graph-editor data binding are all reachable from the first-time-user
surfaces. Several non-blocking ergonomic findings kept for follow-up.

## Affordances Reviewed

### Override modal — keyboard-accessible

`override_verdict.js` uses native `<dialog>`, traps focus, closes on Escape,
returns focus on dismiss, and refuses submission without a non-empty
rationale. Field set is fixed (verdict, rationale, optional findings_artifact_id,
auto_fresh_session) — no operator-editable session/job IDs. Matches
UI_REWORK.md §8.6 + §9.6.

### Copy-on-click — discoverable and idempotent

`copy_on_click.js` initializes on `DOMContentLoaded`; recovery recipes carry
`data-copy`; hover + focus cues give visual confirmation; the 1.2s toast
matches §7.7. Identifier matcher regex
`^(run|job|sess|art|proc|super|lease)_[0-9a-f]+$` lifts the rule verbatim.

### Recovery-panel island — no-JS fallback preserved

The island enhances the server-rendered panel rather than replacing it.
Idle/loading/result/error states render distinctly; dry-run preview shows
would-publish rows + gate reasons. The island never publishes — matches
§8.3 contract.

### Workflow-graph-editor — data-binding only

`require_attested_lane` field renders in node body + textual summary;
round-trips through serializer; tests cover load/edit/save. No viewport
overlay (correct: deferred to React Flow v12 per GH #6).

## Findings

### F1 (info): Recovery panel error-state copy mid-discoverable

When `/v1/invoke` returns an error, the island shows the error string but
does not surface the dry-run CLI recipe so the operator could copy it and
run it directly. **Recommendation (follow-up):** on error, render the
equivalent CLI verb in a `<code data-copy>` so the operator can fall back
without re-deriving the recipe.

### F2 (info): Override modal submit-button feedback

After the modal POSTs, the success state navigates to job_detail but the
button itself does not show a transient "submitting…" cue. With slow
connections an operator may double-click. **Recommendation (follow-up):**
disable the submit button on click + brief inline spinner.

### F3 (info): Graph-editor "ghost field" cleanup

Aligned with gemini Finding 5: if a job's `type` changes from `review` to a
non-review type after `require_attested_lane` was set, the field is not
purged. From the ergonomics angle this surfaces as a confusing readout in
the node body. **Recommendation:** purge on type change in `handleJobChange`.

## Cross-checks with other reviewers

- **codex (threat_model, natural):** `accept_with_findings` — no V2-scope
  regressions introduced.
- **gemini (adversarial, on-behalf):** `accept_with_findings` — 5 adversarial
  findings (HIGH CSRF on /v1/invoke, MEDIUM modal param tampering + dry-run
  side effects, LOW clipboard hijack + ghost fields). The HIGH CSRF
  finding deserves a v1.48 security-hardening dogfood.

## Final verdict

**accept_with_findings** — V2 ships. F1-F3 + gemini findings recorded for
follow-up.
