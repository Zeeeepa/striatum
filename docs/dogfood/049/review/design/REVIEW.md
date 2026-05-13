---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0039", "phase-2", "design"]
---

author: operator

# Design Review — RFC 0039 Phase 2 (Steps 3-6) Synthesis

Operator-composed after claude reviewer produced no on-disk artifact in 13 min (4th instance of claude-no-artifact anti-pattern).

## Summary

The synthesis at `docs/dogfood/049/DESIGN_SYNTHESIS.md` reconciles 3 design proposals for completing RFC 0039 Phase 2:

- Step 3: `striatum daemon start --core go` flag wiring
- Step 4: All mutating RPC methods (claim-next, ack, publish-artifact, complete, verdict, override-verdict, submit-review, recovery, supervise) in Go
- Step 5: Supervisor lifecycle in Go (go/pkg/supervisor/)
- Step 6: 4-platform cross-compile + CI matrix + wheel package-data

## Track split

- Track A (codex): Steps 3+4 — CLI integration + mutating verbs
- Track B (claude): Steps 5+6 — supervisor + distribution + CI

Avoids codex/codex anti-pattern (5 instances documented).

## Findings

- F1 (low): Supply-chain footprint grows with `creack/pty` — synthesis should explicitly call out the go.sum update + audit checklist.
- F2 (low): Step 6's CI matrix wiring needs explicit `make` target naming.

## Verdict

`accept_with_findings` (low). Synthesis is concrete, 2-track split is sound.
