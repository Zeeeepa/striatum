# Synthesize Design Prompt

Produce `docs/dogfood/031/DESIGN_SYNTHESIS.md` with valid `striatum.synthesis.v1` front matter.

Read all three design artifacts and synthesize one implementation plan for the RFC 0028 V1 acceptance-criteria slice. The synthesis must explicitly choose, not just enumerate.

Required sections:

- accepted implementation scope, mapped 1:1 to the RFC 0028 §Acceptance Criteria bullets, with one explicit owner per bullet;
- deferred scope (cross-repo workflows, sealed-mode apply, signing keys, remote serving, full Go rewrite, etc.) and why it is deferred;
- registry storage decision: pick A, B, C, or D from RFC 0028 §6 with named rationale and a migration plan for existing `.striatum/state.sqlite3` repositories;
- implementation language decision for V1 from RFC 0028 §7 with named rationale;
- phased plan with testable milestones aligned to RFC 0028 §8 steps 1–6, naming which step this dogfood lands and which are explicitly deferred;
- schema/migration changes, CLI client/direct-mode behavior, MCP surface and capability defaults, supervision migration, resident recovery scheduler, audit log shape, doctor/status/dashboard/web/evidence surfaces, docs, and fixture changes;
- compatibility and upgrade risks, including direct CLI fallback guarantees during the phased migration;
- test matrix, including adversarial cases for hostile local clients, MCP mutation default-deny, daemon restart with active supervised processes, multi-repo dashboard correctness, registry tamper, and symlink/path-traversal repo registration;
- staging plan that avoids overclaiming sealed provenance, lane attestation, or apply authority beyond what RFC 0026/0027 currently provide;
- human-decision questions that must be answered before implementation proceeds, mapped to RFC 0028 §Open Questions where applicable.

If the designs disagree, choose one path and explain the tradeoff. If a guarantee is advisory, label it advisory. Do not let the synthesis quietly expand scope beyond the V1 acceptance criteria.
