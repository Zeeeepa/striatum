---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["security", "provenance", "attestation"]
---

# Security Review Of RFC 0026 And RFC 0027 Design

author: reviewer-codex-gpt-5.5-004
date: 2026-05-11
status: needs_revision
target: docs/dogfood/030/DESIGN_SYNTHESIS.md
verdict: needs_revision

## Summary

The synthesis is directionally strong: it keeps byline honesty separate from
source-byte provenance, refuses to call advisory patch objects sealed, and
defers hard containment until the operator is mechanically denied writes to
protected paths, active scratch, and signing material. Those choices avoid the
most dangerous false security claim.

I am not accepting the plan yet because the lane-attestation invariant is still
under-specified at the exact boundary that will carry the new trust signal.
The implementation plan says attestation is derived from an active
`process_supervisors` row with a live pid, but it does not require the helper to
prove that the supervisor row is bound to the session's declared lane command
from the run's workflow snapshot.

## Blocking Finding

### SEC-001: Lane Attestation Must Verify The Supervisor-Lane Binding, Not Only Liveness

The proposed Phase 1 work defines a session as attested when it has a
`starting` or `attached` supervisor row and the recorded pid is alive. That is
not sufficient as an implementation contract for a security-significant byline
gate. The helper must also verify that the active supervisor belongs to the
same run and session and was spawned from the lane command declared by that
session's `lane_id` in the immutable workflow snapshot.

Without that explicit check, future command override paths, stale supervisor
state, migration mistakes, or a bug in `supervise start` could allow "some live
process for this session" to become "the declared Codex/Gemini/Claude lane is
attested." The design already says the runner has no model-token proof; that
makes the remaining process-binding claim especially important. If the process
binding is ambiguous, `require_attested_lane` can become a false security gate
rather than a friction point against accidental forgery.

The synthesis should specify the concrete `LaneAttestation` fields and checks:
`run_id`, `session_id`, `session.lane_id`, supervisor state, pid liveness,
supervisor command identity, and whether the command identity matches the
lane's snapshotted command plus runner-expanded environment contract. If
`starting` remains accepted, the plan should also define what must already be
true in that state; otherwise, byline attestation should require `attached`.

Minimum revision needed:

1. Define `session_lane_attestation` as a snapshot-backed binding check, not
   just a supervisor-row liveness check.
2. State whether `starting` is truly eligible for model-lane bylines. If yes,
   define the readiness facts that make it safe; if not, restrict byline
   attestation to `attached`.
3. Add negative tests for a live supervisor whose recorded command does not
   match the session lane, a supervisor row in the wrong run/session, and a
   live-but-not-ready `starting` supervisor if that state remains eligible.

## Non-Blocking Notes

The `attested_bylines` provenance mode needs a crisp operator-facing
definition. Since Phase 1 makes byline honesty apply broadly, the mode should
say exactly what additional promise it carries beyond `advisory`, or the docs
should avoid implying a stronger workflow guarantee than honest rendering.

The receipt plan handles overclaiming correctly by requiring `authority:
advisory` or `authority: apply_gate_only` before hard containment. Keep that
label in every receipt, evidence export, and web/status surface until signing
material and protected paths are outside operator write authority.

## Verdict

`needs_revision`. The implementation should not proceed until the attestation
binding invariant and its negative tests are made explicit.
