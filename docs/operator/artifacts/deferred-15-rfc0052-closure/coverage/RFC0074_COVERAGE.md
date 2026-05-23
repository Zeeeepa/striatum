---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0052-committee-deliberation-workflow.md", "docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md", "docs/operator/artifacts/rfc-0074-phase-a-catalog/CLOSURE.md", "docs/operator/artifacts/deferred-15-rfc0052-closure/map/MAP.md"]
---

# RFC 0074 Coverage Comparison
author: rfc0052-comparator-codex-gpt-5-001

## Coverage Boundary

RFC 0074 does not close RFC 0052. It deliberately separates a broad workflow
catalog expansion from full committee-deliberation semantics.

RFC 0074 covers:

- graph-shape, role-pack, and adversary-pack vocabulary;
- metadata-first catalog discovery;
- a lightweight `implementation_panel` path using existing artifact kinds;
- ordinary workflow fixtures that can validate without new runtime behavior.

RFC 0074 explicitly does not cover:

- RFC 0052 typed debate artifacts;
- `committee_deliberation` phase semantics;
- debate/panel daemon RPC methods;
- arbitrator-gated topic closure;
- panel vote aggregation as a runtime contract;
- committee-specific validator and recovery behavior.

The Phase A closure reinforces that boundary: RFC 0074 landed read-only
catalog discovery and one example, while deferring generator behavior,
chooser pack selection, cost warnings, and RFC 0052 debate/panel integration.

## Reusable Current Primitives

RFC 0074 still helps RFC 0052 by proving a near-term lighter path:
operators can run an implementation-panel workflow today with `handoff`,
`finding`, `findings_ledger`, `synthesis`, and `decision` artifacts. That is
useful product surface, but it is not the full committee protocol.

The current primitives can support a bounded RFC 0052 Phase A implementation,
but only after the committee-specific contracts are made exact.

## Classification Matrix

| Option | Classification | Reason |
|---|---|---|
| Schedule production implementation now | No | RFC 0052 still has sketch schemas, illustrative method names, and open questions that affect storage, validation, and recovery. |
| Close as covered by RFC 0074 primitives | No | RFC 0074's implementation-panel example is intentionally lighter and uses existing artifact kinds without committee runtime semantics. |
| Require a new bounded implementation RFC/design | Yes | The next artifact should specify exact schemas, validator rules, daemon/composition behavior, recovery, A/B evidence, and integration with RFC 0074 Phase D. |

## Recommended Queue Position

Keep RFC 0052 proposed, unblocked, and unscheduled until it becomes a product
priority. When scheduled, start with a bounded Phase A implementation
RFC/workflow rather than a direct source patch.
