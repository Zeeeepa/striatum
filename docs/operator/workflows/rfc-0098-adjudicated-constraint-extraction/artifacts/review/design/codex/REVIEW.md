---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags:
  - "threat-model"
  - "rfc-0098"
  - "design-review"
---

# Design Review - RFC 0098 Threat Model
author: reviewer-codex-gpt-5.5-xhigh-002

## Verdict

accept_with_findings

The design is acceptable for the RFC 0098 V1 scope. The main trust boundaries
are explicit: lane-authored constraint text is untrusted, the daemon/artifact
contract provides structural validation only, revision packets carry validated
constraints forward, and final review is the independent discharge check rather
than a second forum. The residual risks are semantic and reviewer-quality risks,
not new daemon-authority or persistence risks.

## Review Scope

I reviewed the packet's referenced task/RFC material only:

- `docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/TASK.md`
- `docs/rfcs/0098-adjudicated-constraint-extraction-loop.md`
- `docs/rfcs/0093-structured-live-collaboration-workflow-shapes.md`
- `docs/rfcs/0095-revision-safe-workflow-lifecycle.md`
- `docs/rfcs/0082-interrogation-sessions.md`
- `docs/rfcs/0086-multiparty-conversation.md`

I also completed the required live interrogation before verdict:
`intg_ceca45e07284b06439d031d146b867e1`.

The synthesizer confirmed the intended boundary split: model-authored
constraint rows are untrusted; the daemon enforces only structure; revision
packets consume structurally valid constraints; final review verifies material
discharge. The synthesizer named hollow constraint evasion as the highest-risk
attack surface.

## Trust Boundaries

1. **Lane-authored constraints.** The adjudicator lane can emit arbitrary
   natural language, including structurally valid but meaningless rows. This
   text must remain untrusted input.
2. **Artifact contract.** The daemon can safely enforce schema, verdict
   vocabulary, productive-refusal row presence, and field shape. It must not be
   treated as a semantic judge of whether a constraint is good.
3. **Constraint propagation.** The revision packet/spec-publication boundary is
   where validated rows become binding inputs. Constraint ids, source links,
   binding flags, cycle/attempt identity, and accepted-risk metadata must not be
   dropped or silently rewritten.
4. **Final review.** Final review is the last semantic gate. It should typecheck
   discharge of each binding constraint instead of re-running the whole debate.
5. **Dialogue provenance.** Interrogation/conversation records are curated
   authored text. The design must preserve D028 by referencing dialogue turns or
   typed findings, not raw provider output.

## Attack Surfaces

### F1 - Hollow Constraint Evasion

Severity: medium

The structural gate can prove a `needs_revision` ledger contains at least one
productive row, but it cannot prove that row carries a real objection. A hollow
constraint such as "ensure correctness" can satisfy shape requirements while
failing RFC 0098's purpose.

This is acknowledged in both the RFC and the live interrogation. The proposed
mitigation is appropriate for V1: adjudicator rubric, adversarial panel review,
and final discharge review remain responsible for rejecting hollow constraints.
No daemon-side semantic scoring should be added.

Acceptance condition: examples, prompts, and review guidance must continue to
make "structurally valid but semantically hollow" a review failure, not a
schema failure.

### F2 - Constraint Continuity Across Revision

Severity: medium

The strongest implementation risk is loss or drift between the adjudication
ledger and the revision/final-review packets. If a row's id, source finding,
binding flag, expected verification stage, or accepted-risk owner/stage is
omitted during packet construction, the final reviewer can no longer typecheck
the objection lifecycle.

RFC 0098 recognizes this by depending on revision-safe lifecycle work and
cycle-aware logical names. The acceptance criteria also require the final review
to fail closed on missing or partial binding constraints.

Acceptance condition: the latest cleared constraint ledger should be the single
input of record for revision synthesis, spec publication, and final review.
Branch dispositions are useful summaries, but binding constraints must win if
the two disagree.

### F3 - Accepted-Risk Laundering

Severity: low

`accepted_risk` is necessary for explicit non-discharge, but it is also an
escape hatch. A lane could convert a real gap into risk language and clear the
final gate unless owner and stage fields are mandatory and reviewed.

The RFC's final-review discharge table handles this by requiring accepted risks
to name owner/stage. That is enough for V1, provided tests cover missing owner,
missing stage, and partial-without-risk cases.

## Final Assessment

I do not see a design blocker in the scoped material. RFC 0098 keeps authority
where it belongs: PostgreSQL-backed daemon state for workflow control, artifact
contracts for structure, and reviewer roles for semantic judgment. The remaining
risks are the expected residual risks of a structural anti-theater gate.

Verdict: accept_with_findings.
