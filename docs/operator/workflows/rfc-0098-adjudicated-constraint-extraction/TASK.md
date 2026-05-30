# TASK — Implement RFC 0098 (Adjudicated Constraint-Extraction Loop)

Land **RFC 0098**
(`docs/rfcs/0098-adjudicated-constraint-extraction-loop.md`) as a working slice,
end to end, with all gates green. This is a real implementation run on the
Striatum codebase itself (Go-only runtime, RFC 0078), building directly on the
RFC 0093 V1 substance-gate substrate that already landed
(`collaboration_ledger.v1`, the `adjudicator` role, the `falsification_gate` /
`cross_examination` shapes).

## What RFC 0098 adds

RFC 0093 made adjudicator refusal **honest** — a hollow exchange scores
`needs_revision` and the commit stays withheld. RFC 0098 makes refusal
**productive**: a `needs_revision` verdict must *compile the objections into
binding constraints* that the next revision discharges and final review verifies.
It promotes the pattern the Engram entity-relationship forum run (#89) reached by
hand — adjudicator extracts a numbered constraint table, the convener discharges
each row, the final reviewer typechecks discharge instead of relitigating — into
a first-class shape.

The shape is `adjudicated_constraint_extraction`, an 8-phase RFC 0045 workflow:
survey → convener_synthesis → cross_exam → adjudication → revision_synthesis →
constraint_discharge_review → spec_publication → final_review.

## Scope (build, smallest-blast-radius first — RFC §"Implementation Plan")

Build in this order; **defer cleanly** at any boundary if the run risks wedging,
and record the deferral in the handoff:

1. **`collaboration_ledger.v1.1` + the productive-refusal gate.** Additive
   schema: a typed, sourced `constraints[]` table + `branches[]` posture-
   disposition matrix on the existing `collaboration_ledger.v1`. The gate:
   `review.submit` / `publish-artifact` **rejects** (exit 6) a ledger whose
   `verdict: needs_revision` carries an empty `constraints[]`. While here, make
   the contract **accept** the clearing verbs it advertises and natural ledger
   front matter (closes #88 / #79 for this shape). Pure contract/validation work,
   no daemon method. **This is the V1 target.**
2. **Shape fixture + generator.** Register `adjudicated_constraint_extraction` in
   the collaboration shape pack; `workflow generate --shape
   adjudicated_constraint_extraction` emits the 8-phase graph; starter fixture
   under `examples/adjudicated-constraint-extraction-flow/`. Requires cycle-aware
   logical names (#84) so `*_synthesis_${cycle}` republish does not collide —
   if #84 is unresolved in the running daemon, **defer this slice** and record it.
3. **Discharge-verifying final review.** The `constraint_discharge` finding shape
   + a `final_review` that fails closed on any undischarged `binding: true`
   constraint, without re-running prior phases.

Slice 4 (first-class `constraint.*` objects + coverage metrics) is **explicitly
deferred** to a future run. A single implementer should land slice 1 cleanly;
slices 2–3 are stretch. Land only what the design synthesis and panel converge on.

## Acceptance criteria (from RFC §"Acceptance Criteria")

- `collaboration_ledger.v1.1` is **additive**: every valid RFC 0093 V1 ledger
  still validates; `publish-artifact` exits 6 on invalid new fields; the D028
  no-stdout guard covers them.
- `review.submit` / `publish-artifact` **rejects** a `collaboration_ledger` whose
  `verdict: needs_revision` carries an empty `constraints[]`, and **accepts** one
  with ≥1 binding constraint or unresolved-question row. Seeded transcripts prove
  both directions.
- The contract accepts the clearing verbs it advertises and natural ledger front
  matter (#88 / #79 regression fixtures pass).
- (Slice 2, if landed) `workflow generate --shape
  adjudicated_constraint_extraction` produces a `striatum.workflow.v1.1` graph
  passing **both** `workflow validate` **and** `run.prepare` (same phase rules,
  #66): exactly one `phase_synthesis` job per phase; the
  `adjudication → revision_synthesis` constraint edge is legal.
- (Slice 3, if landed) `final_review` fails closed when any `binding: true`
  constraint is `missing` or unaccepted-`partial`, and passes when all are
  `discharged` or `accepted_risk` with owner/stage — without re-running prior
  phases.
- No new daemon method, no floor-control primitive, no economy/reputation store,
  no vendor SDK import; RFC 0078 Go-only guardrails + the command-authority
  matrix guardrail tests stay green.

## Constraints

- **Scope discipline:** slice 1 is the target; 2–3 are stretch; 4 is deferred.
  Defer any slice that risks the run and record it in the handoff.
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
# ledger v1.1 additive + productive-refusal gate tests pass under live PG (RFC 0080 pgtest):
STRIATUM_PG_TEST_URL=postgres:///postgres go -C go test ./pkg/artifactcontracts/... ./pkg/mutations/...
# (slice 2, if landed) the generated fixture validates AND prepares:
striatum workflow validate examples/adjudicated-constraint-extraction-flow/workflow.json
```

Write the build handoff with what landed, what was deferred, and the exact
verification commands you ran with their results.
