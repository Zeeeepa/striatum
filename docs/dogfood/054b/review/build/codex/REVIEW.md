---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0050", "v1", "fixup-review", "provenance"]
---

author: reviewer-unknown-model-001

# Build Review - RFC 0050 V1 Fix-Up

Verdict: accept

## Scope

Reviewed the four Gemini adversarial findings from
`docs/dogfood/054/review/build/gemini/REVIEW.md` against the cited
implementation surfaces. This review focused on provenance trust boundaries:
artifact bylines, lane attestation, verdict source attribution, and override
rationale rendering.

## Threat Boundaries And Attack Surfaces

- Artifact disk content is attacker-controlled until canonicalized by the
  runner. The UI must not render a forged model byline for an unattested
  session.
- Verdict rows are persisted workflow provenance. The UI must not infer an
  operator override from verdict ordering alone.
- Lane attestation can drift after publication. Artifact provenance must use
  recorded artifact metadata rather than recomputing from the current session.
- Override verdicts cross the operator/model trust boundary. The dashboard and
  web surfaces must show the rationale beside the override marker.

## Finding Closure

1. Byline forgery is closed. The Jinja macro now forces `author: operator`
   when `attested is sameas false` instead of rendering the supplied model
   line, and the React `BylineLine` component mirrors that behavior.
2. Verdict forgery by inferred override is closed. `_shape_verdict_rows`
   defaults missing sources to `natural`; it only treats a missing-source row
   as `operator_override` when a matching `verdict.overridden` event exists.
3. Artifact attestation drift is closed for the cited artifact path.
   `_shape_artifact_rows` derives the artifact attestation chip from the
   recorded `author_line`, so a previously attested artifact does not become
   unattested merely because the live session later changes state.
4. Dashboard rationale omission is closed. `_verdict_chip` includes a
   truncated rationale in the override suffix, and `_render_verdict_overrides`
   also emits an explicit rationale line.

## Regression Check

- `pytest tests/test_byline_regression.py tests/test_override_rationale_regression.py -q`
  passed: 6 tests.

No new provenance regression was found in the reviewed fix-up surfaces.
