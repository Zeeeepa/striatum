---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["threat_model", "rfc-0050", "v1", "build", "adversarial-check"]
---

author: reviewer-unknown-model-001


# Build Review — RFC 0050 V1 Fix-up (Gemini Adversarial)

**Verdict:** accept

The fix-up successfully addresses the four critical findings identified in the previous round. The system now demonstrates robust provenance honesty by closing the forgery loopholes and ensuring that attestation state is handled with recording-time semantics for artifacts.

## Adversarial Re-attack Results

### 1. Byline Forgery Loophole (Truthfulness Rule 1) — **FIXED**
The vulnerability where forged model bylines were rendered as "attested" even in unattested sessions has been closed.
- **UI Sanitization:** Both the `_components.html` macro and `BylineLine.tsx` React component now explicitly check the `attested` flag. If `False`, they refuse to render the provided `author_line` and instead force an `author: operator` string.
- **Publication Gatekeeper:** `striatum.artifacts:publish_artifact` now enforces that any byline present in a Markdown file must match the *expected* byline for the current session. An operator in an unattested session is expected to produce `author: operator`, preventing them from publishing a file with a forged `author: reviewer-...` line.
- **Attestation Logic:** `service.py` now derives artifact attestation from the recorded `author_line` and `attestation_override_rationale`, which are verified at publication time.

### 2. Verdict Forgery via "Inferred Override" — **FIXED**
The dangerous sequence-based inference in `service.py` (which falsely labeled second-attempt model verdicts as operator overrides) has been removed.
- **Explicit Provenance:** `_shape_verdict_rows` now relies exclusively on the `source` column and explicit `verdict.overridden` events to identify overrides.
- **Honesty:** Natural revision cycles are now correctly labeled as `natural` (or `natural-revision` in some contexts), preserving the truth that the model produced the verdict.

### 3. Attestation-Drift Honesty — **FIXED (Artifacts)**
The UI no longer re-computes artifact attestation from live supervisor state, which previously caused correctly-attested artifacts to "drift" into an unattested state once the session closed.
- **Recording-Time Semantics:** `_shape_artifact_rows` now uses `_recorded_artifact_attestation_chip`, which evaluates the attestation based on the metadata captured at the time of publication (the `author_line` and override rationale). This ensures a stable and honest history for archived artifacts.

### 4. Override Rationale Omission (Dashboard) — **FIXED**
The terminal dashboard now correctly surfaces the mandatory override rationale.
- **Evidence:** `src/striatum/dashboard.py:_verdict_chip` now includes the truncated rationale in the chip string, and `_render_verdict_overrides` adds it as a detailed line in the output.

## Final Assessment

The adversarial surface of the provenance UI has been significantly hardened. While the system still relies on the CLI as a trusted gatekeeper for the database, the UI components are now "provenance-aware" and refuse to render misleading strings even if a forged byline were to reach the state store.

The implementation meets the requirements for RFC 0050 V1 provenance honesty.
