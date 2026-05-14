---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0050", "v1", "build", "operator-on-behalf"]
---

author: reviewer-unknown-model-001


# Build Review — RFC 0050 V1 (Gemini Adversarial)

**Verdict:** needs_revision

The V1 implementation lands the core primitives but fails several non-negotiable adversarial honesty checks. The most critical failure is a "verdict forgery" loophole where natural model revisions are falsely labeled as operator overrides, coupled with byline rendering that allows for explicit forgery.

## Critical Findings

### 1. Byline Forgery Loophole (Truthfulness Rule 1)
The `byline_line` Jinja macro and `BylineLine.tsx` component render the literal `author_line` from disk without sanitization against the session's attestation state. While they apply a CSS class for unattested sessions, they still display the forged string.

- **Evidence:** `src/striatum/web/templates/_components.html:68` renders `<code>{{ rendered_author }}</code>` directly.
- **Evidence:** `src/striatum/web/frontend/src/shared/components/BylineLine.tsx:18` renders `{canonicalActual ? <code>{rendered}</code> : <em>{rendered}</em>}`.
- **Attack Surface:** An adversary can manually edit an artifact file to include `author: <role>-<model>-<ord>` for an unattested session. The UI will render this forged byline text, potentially misleading operators who overlook the "unattested" CSS treatment.
- **Required Fix:** If `attested` is false, the component must refuse to render a model-author string and instead force `author: operator` or a "forgery detected" warning.

### 2. Verdict Forgery via "Inferred Override"
The `service.py` logic incorrectly infers `operator_override` for any accepting verdict that follows a non-accepting verdict if the `source` column is missing. This falsely attributes model-driven revision cycles to the operator.

- **Evidence:** `src/striatum/service.py:328-333` implements `inferred_override`.
- **Attack Surface:** In a normal `reviewer` cycle where a model first says `needs_revision` and then (after fixes) says `accept`, the UI will label the second natural verdict as an "operator-override". This is a provenance lie.
- **Required Fix:** Remove inference logic. If `source` is missing, it must be treated as `natural` or `unknown`. Provenance must be recorded, not guessed.

### 3. Attestation-Drift Honesty Failure
The UI recomputes lane attestation from the *current* session/supervisor state instead of using the state at the time of artifact recording.

- **Evidence:** `src/striatum/service.py:361` calls `_lane_attestation_chip` which re-queries the live supervisor state.
- **Impact:** An artifact correctly published by an attested lane will be flagged as "unattested" once the supervisor process exits or the session closes. This creates false-negative provenance warnings (pessimism drift).
- **Required Fix:** Per `UI_REWORK.md §1`, the UI must rely on the `artifacts.author_line` and the attestation state captured at publish time.

### 4. Override Rationale Omission (Dashboard)
The dashboard's compact verdict chip omits the required override rationale.

- **Evidence:** `src/striatum/dashboard.py:192` `_verdict_chip` only appends ` (override)`.
- **Impact:** Violates the mandate that rationales must *always* be rendered beside the pill.
- **Required Fix:** Include at least a truncated rationale in the dashboard chip or ensuring it's prominent in the job detail view.

## Regression Check Status

1.  **Byline regression** — **FAILED**. The template path *does* render the forged line for unattested sessions.
2.  **Override rationale prominence** — **PARTIAL**. Prominent in web UI, omitted in dashboard chips.
3.  **LaneEvidenceChip muted** — **PASSED**. Correctly remains `not_yet_correlated`.
4.  **No transcript capture surfaces** — **PASSED**. No stdout/stderr leaks found.
5.  **Dashboard ↔ web vocabulary parity** — **PASSED**. Both share the same status enums.

## Next Actions for Implementer

1.  Remove `inferred_override` logic from `service.py`.
2.  Update `byline_line` macro/component to sanitize output: if `attested` is false, model-author strings must not be rendered.
3.  Fix attestation-drift by ensuring `attested` status is pulled from the artifact's record-time metadata, not the live session.
4.  Add the override rationale to the dashboard chip.
