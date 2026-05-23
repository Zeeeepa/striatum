---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/operator/BRIEF.md", "docs/rfcs/0052-committee-deliberation-workflow.md", "docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md"]
---

# RFC 0052 Readiness Map
author: rfc0052-mapper-codex-gpt-5-001

## Current State

TODO item 43 and ROADMAP section 5.8 agree: RFC 0052 is proposed,
unblocked by the completed RFC 0048 substrate flip, and unscheduled. The
operator brief does not make RFC 0052 part of the next 1-3 active actions.

RFC 0052 defines a high-cost committee-deliberation shape for phases where
ordinary review and synthesis are not enough. The proposed shape includes:

- typed debate artifacts: `debate_turn`, `arbitration_ruling`,
  `panel_vote`, `panel_verdict`, and `debate_synthesis`;
- an arbitrator role with bounded authority to record consensus, escalate to
  a panel, rule on objections, call timeout, or declare stalemate;
- optional panel escalation with fresh rotated lanes and aggregation rules;
- an adversarial interrogator/defendant variant;
- bounded rounds, terminal actions, and an A/B validation experiment before
  accepting the implementation.

## Existing Primitives Available

Current Striatum already has several primitives RFC 0052 can build on:

- daemon-owned PostgreSQL live state with repository-scoped workflow state;
- durable Markdown artifacts with front-matter validation;
- workflow-authored lanes, roles, write scopes, expected artifacts, and
  bounded cycles;
- review postures, fresh-session review policy, and same-model pairing lint;
- lane evidence and operator-on-behalf override audit behavior;
- decision artifacts, escalation artifacts, and human-principal escalation
  taxonomy, including `committee_stalemate`;
- RFC 0074 catalog metadata and a lightweight implementation-panel example
  using current artifact kinds.

## Not Design-Ready

RFC 0052 is intentionally not yet a production implementation design. Its
artifact schemas and daemon methods are sketches, and the RFC names open
questions that affect implementation behavior:

- consensus arithmetic;
- arbitrator lane rotation;
- adversarial pairing rules;
- artifact volume bounds;
- tight-loop or ordinary lease semantics;
- cost estimation at validation time.

The RFC also leaves several production contracts unresolved: exact
front-matter schemas, whether new daemon RPC methods are required, how
committee phase state compiles into ordinary jobs or RFC 0045 phases, how
topic closure is stored and enforced, and how panel verdicts map to the
existing verdict and decision tables.

## Readiness Classification

RFC 0052 should not be implemented directly from the V0 proposal. The right
next step is a bounded implementation RFC/design workflow for Phase A. That
workflow should turn the sketches into exact schemas, method names or
composition rules, validator checks, recovery semantics, and A/B acceptance
evidence before production source changes are scheduled.
