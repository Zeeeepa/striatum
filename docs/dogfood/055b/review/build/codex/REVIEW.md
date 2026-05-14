---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0050", "v1-5", "build-review", "provenance"]
---

author: reviewer-unknown-model-001

# Build Review: RFC 0050 V1.5 Fix-up Correctness

## Verdict

Needs revision.

I could not confirm that Gemini's three findings are closed at the cited
file:lines from the review packet's document-only evidence. The packet supplied
the prior Gemini review, the original V1.5 handoff, the canonical design handoff,
and RFC 0050. It did not supply a fix-up handoff, source diff, or focused test
output that demonstrates the post-review corrections at the cited code lines.

## Trust Boundaries Reviewed

- Artifact file content remains untrusted relative to runner provenance state.
- Operator-on-behalf publish remains a separate provenance source from model
  process execution.
- Historical row provenance remains distinct from current session/supervisor
  health.
- UI recovery recipes remain promises to the operator and must not imply CLI
  preconditions are satisfied unless the command will work as shown.

## Findings

### Finding 001: Closure Evidence For Byline Forgery Is Missing

Gemini's first finding says `src/striatum/service.py` rendered artifact
attestation from a byline regex, allowing an operator-on-behalf publish with a
model-shaped `author:` line to appear attested. The RFC and design handoff
require no model-author forgery and explicitly require operator-on-behalf
provenance to stay visible.

The supplied handoff predates or does not describe the fix-up in enough detail.
It says artifact provenance was implemented and mentions regression tests, but
it does not show that the artifact chip now refuses green attestation when
`attestation_override_rationale` is present or when no process evidence exists.

Threat model impact: the untrusted artifact-content boundary remains unproven.
If the original regex-derived path is still active at the cited line, a forged
model-looking byline can still cross into trusted UI provenance.

Required evidence: cite the updated `service.py` branch that distinguishes
operator-on-behalf or no-process publishes from attested model publishes, plus a
focused regression test where `--allow-no-process-execution` with a canonical
model-shaped byline does not render as attested.

### Finding 002: Closure Evidence For Historical Attestation Drift Is Missing

Gemini's second finding says verdict and session attestation were recomputed
from current supervisor state rather than the row-time evidence. The canonical
design is explicit that attestation must be read at the time the artifact or
verdict row was recorded, not from the session's current state.

The supplied handoff says posture verdict rows gained provenance and attestation
columns, but it does not show that verdict attestation is persisted or otherwise
derived from row-time evidence. Its verification list does not name a focused
historical-attestation regression test.

Threat model impact: temporal provenance remains unproven. A once-attested
verdict could still be degraded into the same warning state as a never-attested
or forged row after supervisor shutdown, obscuring audit history.

Required evidence: cite the updated row-shaping logic for verdict attestation
and a test where a verdict issued while attested remains distinguishable after
the session or supervisor later stops.

### Finding 003: Closure Evidence For Override Rationale Surfacing Is Incomplete

Gemini's third finding says `LaneEvidenceChip` was hard-coded to
`not_yet_correlated`, hiding available `attestation_override_rationale` data.
RFC 0050 requires muted `not_yet_correlated` until process evidence correlation
ships, but it also requires operator override rationales to remain visible and
not be silently collapsed.

The supplied handoff lists `tests/test_override_rationale_regression.py`, and
the RFC requires override rationale visibility. That is directionally aligned,
but the document does not prove that artifact-level no-process publish
rationales, posture override rationales, and any evidence-chip-adjacent rationale
surfaces are all covered at Gemini's cited location.

Threat model impact: operator-on-behalf action can remain visually under-specified
even when the database has a rationale, weakening auditability.

Required evidence: cite the template/service payload fields that render
`attestation_override_rationale` or verdict override rationale inline, and the
specific tests that cover both artifact publish overrides and verdict overrides.

## New Provenance Regressions

No additional provenance regression is proven from the supplied documents alone.
The unresolved issue is that the packet's evidence is insufficient to accept the
fix-up as closed. A follow-up review with the actual fix-up handoff, source diff,
or focused test output should be able to resolve this quickly.
