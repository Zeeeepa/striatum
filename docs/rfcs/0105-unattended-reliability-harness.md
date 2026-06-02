# RFC 0105: Standing unattended-reliability harness — the yolo gate

Status: proposed
Date: 2026-06-02
author: proposer-claude-opus-4-8-001
Context: RFC 0101 (robust autonomous execution + the Phase 5 chaos suite), RFC 0103 (production-hardening acceptance / fault-class matrix), RFC 0095 (revision-safe lifecycle), RFC 0097 (full run orchestration); `go/pkg/adapterconformance/chaos_test.go`; `go/Makefile` `CORE_COVERAGE_FLOOR`.

## Problem

The product mission is **showerthought → product in full yolo mode with minimal
human intervention**. That makes one property existential: a run must **complete,
or fail loudly, unattended** — because there is no human babysitting it. Yet the
multi-lane revision lifecycle still wedges (the #65/#84/#120/#121/#131/#134
family), and there is **no standing hermetic gate** that would catch a regression
in it:

- Unit coverage is floored at **20%** (`go/Makefile:14` `CORE_COVERAGE_FLOOR`),
  and the load-bearing logic fails in *integration*, not in units — RFC 0101's
  chaos suite already caught a real bug that deployment and unit tests missed.
- The RFC 0101 chaos suite (`adapterconformance/chaos_test.go`) exercises the
  **recovery** loop (dead/stalled lanes → requeue or escalate), but **not the
  full multi-lane revision lifecycle** (design panel → needs_revision → re-open
  → re-review → synthesize → complete) under fault.
- **RFC 0103's acceptance** *describes* a one-time fault-class matrix (W1
  isolation / W3 churn / W4 reviewer-replacement, both seats) but nothing keeps
  it true after it is demonstrated once.

The single most valuable missing artifact named in the architecture review is a
green, hermetic, end-to-end gate for the real multi-lane revision flow.

## Proposal

Turn RFC 0103's acceptance matrix into a **permanent CI gate** by extending the
existing chaos/conformance harness rather than building a new one:

1. **Lifecycle fixtures.** Extend `go/pkg/adapterconformance` to drive the real
   fake-agent lifecycle through the in-process daemon for a run with **≥2 lanes +
   a `needs_revision` cycle**: implement → review(s) → needs_revision → re-open →
   re-review → synthesize → complete. One fixture per *supported* workflow shape
   (see RFC 0106).
2. **Fault matrix.** For each fixture, inject the RFC 0103 fault classes —
   (a) lane death mid-task, (b) transport/daemon-socket churn (W3), (c) reviewer
   replacement / interrogation-window survival across a reviewer attempt (W4) —
   using the suite's existing deterministic time-warp.
3. **The assertion (the gate).** Each (shape × fault) cell must **either**
   self-recover to `completed` with the attempt preserved **or** escalate loudly
   (`needs_operator` run state + `escalation_inbox` row, per RFC 0101 Phase 4)
   **within budget** — and **never** silently wedge (no live job stuck with no
   lease, no session, and no escalation).
4. **Standing gate, not a one-shot.** Wire it into `go/Makefile` `check` and
   `.github/workflows/ci.yml` so it runs on every commit (PG-gated, like the
   existing chaos suite). A regression in the revision lifecycle turns CI red.
5. **Reusable graduation fixture.** Expose a per-shape reliability fixture entry
   point so RFC 0106 can gate a shape's promotion to `supported` on a green cell.

This operationalizes RFC 0103's acceptance — it does not re-litigate it. RFC 0103
defines the bar; RFC 0105 builds the standing regression gate that keeps the bar
met, which is what the yolo mission requires.

## Acceptance

- The extended suite runs in CI and is required for `make check`.
- For every shape currently marked `supported` (RFC 0106), every fault-matrix
  cell completes-or-escalates-loud unattended within budget; a deliberately
  re-introduced wedge (e.g. reverting the RFC 0104 run lock, or a leaked
  interrogation window) turns the gate **red** — demonstrating teeth.
- **Mission acceptance (outside CI, the real proof):** a genuine multi-lane
  code-change-with-revision dogfood driven through the runner **unattended, 10×
  consecutively, zero operator rescue.**

## Non-goals

- Not raising `CORE_COVERAGE_FLOOR` as the primary lever — the gate is
  *behavioral* (does the run finish-or-fail-loud), not line-coverage. A modest
  floor bump on `pkg/mutations` is welcome but secondary.
- Not testing real model adapters in CI — the fake agent (`adapterconformance/testagent`)
  stays the hermetic driver; live-adapter conformance remains the separate,
  non-hermetic track.
- Not new recovery mechanism — it *tests* the RFC 0101 recovery/escalation that
  already exists.

## Relationship to prior RFCs

- **RFC 0101** built the recovery/escalation substrate and the Phase-5 chaos
  suite; this RFC extends that suite from recovery-only to the full revision
  lifecycle and makes it a standing gate.
- **RFC 0103** defines the acceptance fault matrix (W1/W3/W4, both seats); this
  RFC is its permanent CI realization. RFC 0103 proves it once; RFC 0105 keeps it
  proven.
- **RFC 0104** removes the deadlock confound so a red cell here means a genuine
  lifecycle bug, not a lock race. **RFC 0106** consumes the per-shape fixture as
  its graduation gate.
