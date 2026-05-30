# TASK — Implement RFC 0094 (Deferred Collaboration Shapes)

Land **RFC 0094**
(`docs/rfcs/0094-deferred-collaboration-shapes-fog-of-war-and-synaptic-prune.md`)
as a working slice, end to end, with all gates green. This is a real
implementation run on the Striatum codebase itself (Go-only runtime, RFC 0078),
building directly on the RFC 0093 V1 substance-gate substrate that already
landed (`collaboration_ledger.v1`, the `adjudicator` role, the
`falsification_gate` / `cross_examination` shapes).

## What RFC 0094 adds

RFC 0093 shipped the two **pure-composition** collaboration shapes and deferred
the ones that each need one new mechanism. RFC 0094 picks up that deferred set:

- **`post_dialog_hook`** — an optional RFC 0086 conversation-fixture field whose
  close-time *emit-before-teardown* fixes the `synaptic_prune` liveness race
  (fan out follow-up work to the coordinator while participant lanes are still
  live). Reuses `conversation.close` + work-packet delivery — no new method.
- **Work-packet *type* sequencing** — a generator/lint capability that compiles
  "withhold a `proposal`-typed job until a named gate clears" into ordinary
  RFC 0045 phase dependencies. Unblocks `fog_of_war_review`. No new daemon route.
- **`synaptic_prune`** and **`fog_of_war_review`** shapes — generated via
  `workflow generate --shape <id>`.
- **Adjudicator reliability** — a semantic **Check-B** challenge↔rebuttal
  correspondence rubric + an additive `collaboration_ledger` `v1.1`, and an
  opt-in **second-adjudicator-on-disagreement** gate mode (RFC 0093 OQ2).
- **Floor degraded mode + serialized parallel-interrogation policy** (round-robin
  for V1; one active interrogation per live target).

## Scope (build, smallest-blast-radius first — RFC §"Implementation Plan")

Build in this order; **defer cleanly** at any boundary if the run risks wedging,
and record the deferral in the handoff:

1. **`post_dialog_hook` + `synaptic_prune`** — the fixture field, the close-time
   emit, and the prune shape. Self-contained; proves the liveness hook (AC#2,
   AC#5).
2. **Check-B + `collaboration_ledger` `v1.1` + anti-theater corpus** —
   strengthens the gate the whole family shares; testable against seeded
   transcripts (AC#6, AC#8). Additive-only schema change.
3. **Work-packet type sequencing + `fog_of_war_review`** — the generator
   capability + the harder shape; depends on Check-B for honest coverage scoring
   (AC#1, AC#3, AC#4).
4. **Second adjudicator** — opt-in gate mode layered on after the single-path
   ledger extension lands (AC#7).

A single implementer should land slice 1 (and slice 2 if time permits) cleanly;
slices 3–4 are explicit stretch goals. Land only what the design synthesis and
panel converge on.

## Acceptance criteria (from RFC §"Acceptance Criteria")

- `workflow generate --shape fog_of_war_review` / `--shape synaptic_prune`
  produce `striatum.workflow.v1.1` graphs passing `workflow validate` /
  `workflow lint`, wiring only existing RFC 0082/0086 calls in packet `commands`.
- **`post_dialog_hook`:** `conversation.close` emits exactly one prune packet to
  the coordinator with participant session ids + transcript ref **before** the
  preserved-context window is released; a follow-up `interrogation.open` against
  a still-live participant succeeds.
- **Type sequencing:** the `proposal`-typed job in `fog_of_war_review` is
  **unreachable** until the reconstruction-coverage verdict clears; no new daemon
  method/route (command-authority matrix + guardrail tests stay green).
- **`synaptic_prune` liveness:** the fan-out reaches all live participants and
  records ≥2-vote retirements into a `collaboration_ledger`; a dead-target
  fixture proves the shape **refuses cleanly** (records the dead target, does not
  hang). The retired set injects as a negative preamble into a later run.
- **Check-B / anti-theater corpus:** hollow/fluent transcripts and
  cite-the-right-id non-rebuttals score `needs_revision`; landed-and-rebutted and
  genuine-reconstruction clear. The adjudicator records per-pair `correspondence`.
- **Ledger extension is additive:** every RFC 0093 V1 ledger still validates;
  `publish-artifact` exits 6 on invalid new fields; the D028 no-stdout guard
  covers them.
- No new daemon method, no floor-control primitive, no economy/reputation store,
  no vendor SDK import; RFC 0078 Go-only guardrails stay green.

## Constraints

- **Scope discipline:** slices 1–2 are the target; 3–4 are stretch. Defer any
  slice that risks the run and record it in the handoff.
- **Substrate seating:** keep the critical authoring path (synthesis, build) on
  claude + codex; agy is the third design/review seat.
- **Product boundary (AGENTS.md / spec):** daemon-owned PostgreSQL stays the sole
  live-state authority and sole writer; dialog turns stay curated
  authored-text-only (D028 as narrowed by RFC 0092); local-first, no external
  service, no vendor SDK; stay inside `write_scope.allowed_paths`; never write
  `.striatum/`.

## Verification commands (reviewers run these)

```sh
PATH="$HOME/go/bin:$PATH" make -C go check     # vet + lint + race tests
make test                                       # full suite
make lint && make typecheck
# new fixtures validate:
striatum workflow validate examples/<shape>/workflow.json
# ledger + liveness + anti-theater tests pass under live PG (RFC 0080 pgtest):
STRIATUM_PG_TEST_URL=postgres:///postgres?host=/var/run/postgresql go -C go test ./pkg/...
```

Write the build handoff with what landed, what was deferred, and the exact
verification commands you ran with their results.
