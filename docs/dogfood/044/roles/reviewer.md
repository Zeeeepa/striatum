# Reviewer Role (Dogfood 044)

One design review (gating implement) plus a 3-way build review at the
end.

## Design review (claude, `ergonomics_dx`)

Does the synthesized design map F1-F6 to concrete function names + file
paths? Is composite-tool atomicity discoverable from an operator's
first-time MCP call? Is backward compatibility for existing MCP tools
and daemon RPC envelope-v1 explicitly asserted?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — systems posture. Dispatch authorization
  preserved, audit-chain integrity, composite-tool rollback actually
  rolls back, watcher race + signal correctness, surgical-recovery
  cannot be bypassed.
- **claude** `ergonomics_dx` — operator UX. Composite tool errors
  name the failing composed step, MCP path is first-time-discoverable,
  operator workflow stays decluttered.
- **gemini** `adversarial threat_model` — break-the-build. Composite
  atomicity edge cases (mid-step crash, daemon crash mid-transaction);
  watcher race exploits (rotated log, watcher-start-before-wrapper,
  SIGTERM during heartbeat); audit-chain gaps (orphan allow-row, or
  dispatch row with no audit row).

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0040", "v1-5", "dogfood-044"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

`schema_version` must be the exact string `"striatum.finding.v1"`
(not `"1"`). `artifact_kind` is `"finding"`. `verdict_intent` is one of
`accept | accept_with_findings | needs_revision | reject` (not
`verdict`). `severity` is one of `low | medium | high | critical`.
`tags` is a JSON array. The `author:` byline is a plain markdown line
AFTER the front-matter block — not inside it.

**IMPORTANT — write the REVIEW.md / finding artifact directly.** If
`striatum ack` is denied, write the artifact and exit normally; the
operator publishes on your behalf. Do not ask the operator clarifying
questions and exit. Per dogfood-037 intervention #5 + dogfood-041
friction patterns.
