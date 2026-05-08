---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["dogfood-008", "rfc-0016"]
---

# RFC 0016 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08

Verdict intent: **accept**.

The synthesis is implementation-ready and matches RFC 0016 § 1–6
plus § 7 (`--format ascii` on `run graph`). The renderer is pure
and testable; refactor preserves existing `run_graph` behavior;
`--no-graph` regression with a golden file guards backwards
compatibility.

## Compliance

- D028 / no transcripts: the renderer doesn't read or capture
  any external state.
- TTY gating: color suppressed on `--once`, on non-TTY, and when
  `NO_COLOR` is set.
- `--no-graph` byte-identical to today's frames (golden test).

## Findings

(None blocking.)

### F1 (info) — `--graph-style fancy` falls back to `layered` in V1

Synthesis explicitly defers the Unicode renderer to V2 and falls
back to `layered`. Operators who pass `--graph-style fancy` get
the layered ASCII output. Acceptable for V1.

### F2 (info) — Memoization deferred

Workflows are O(tens) of jobs; layout is cheap. V1 ships uncached.
Accept; revisit if profiling shows the dashboard refresh budget
is tight.

## Verdict

**accept.** Ready to record human acceptance.
