# Adjudicated constraint-extraction flow

A runnable fixture for the `adjudicated_constraint_extraction` workflow shape
(RFC 0098). It is the productive-refusal sibling of the RFC 0093 collaboration
shapes: instead of a single publish gate, the adjudicator's `needs_revision`
must compile load-bearing objections into binding constraints that the next
revision discharges and final review typechecks.

Regenerate with:

```
striatum workflow generate \
  --shape adjudicated_constraint_extraction \
  --lane-set local \
  --scaffold-root examples/adjudicated-constraint-extraction-flow \
  --artifact-root examples/adjudicated-constraint-extraction-flow/artifacts \
  --option topic="constraint-extraction design"
```

## The eight phases

Each phase declares exactly one `phase_synthesis` job (enforced by
`workflow validate` and `run.prepare`, GH #66):

| Phase | Synthesis job | Role | Purpose |
|-------|---------------|------|---------|
| `survey` | `survey_synthesis` | `convener` | Frame the problem and existing constraints |
| `convener_synthesis` | `convener_synthesis` | `convener` | Publish the candidate synthesis (re-opened on `needs_revision`) |
| `cross_exam` | `cross_exam_synthesis` | `cross_examiner` | Posture-specific adversarial challenge (product / implementation / privacy / eval / operations) |
| `adjudication` | `adjudicate` | `adjudicator` | Convert load-bearing challenges into a binding `constraints[]` table |
| `revision_synthesis` | `revision_synthesis` | `revision_convener` | Discharge each adjudicated constraint explicitly |
| `constraint_discharge_review` | `discharge_review_synthesis` | `adjudicator` | Confirm each binding constraint cleared |
| `spec_publication` | `spec_publication` | `spec_author` | Author the spec from the latest cleared constraint ledger |
| `final_review` | `final_review_synthesis` | `final_reviewer` | Discharge typecheck; fails closed on undischarged binding constraints |

## Load-bearing structure

- **`adjudication → revision_synthesis` carries the constraint table.** A
  `needs_revision` verdict re-opens `convener_synthesis` as an RFC 0083 bounded
  cycle (`max_iterations`, the `adjudicate → convener_draft` cycle edge). The
  slice-1 contract refuses a `needs_revision` `collaboration_ledger.v1.1` with an
  empty `constraints[]` (exit code 6), so a naked refusal cannot publish.
- **Cycle-aware logical names.** Every artifact re-published inside the revision
  cycle — the convener candidate/synthesis, the cross-exam findings ledger, the
  adjudication ledger, the revision synthesis, and the discharge re-review — uses
  a `${cycle}`-templated `logical_name` and `path` (e.g.
  `collaboration_ledger_${cycle}`, `revision_synthesis_${cycle}`). A fixed name
  would collide on the append-only artifacts table on republish (RFC 0098
  Acceptance #4 / GH #84). `survey`, `spec_publication`, and the final summary
  run once, so they keep fixed names.
- **`spec_publication`** consumes the latest cleared constraint ledger as binding
  input — the spec begins from adjudicated constraints, not the original proposal.
- **`final_review`** is a typecheck. It emits a `constraint_discharge` table and
  (in slice 3) fails closed on any binding constraint that is `missing` or
  `partial` without an accepted risk; it does not re-run the forum.

The fixture uses the single `local` process lane so it validates and prepares
without external adapters. For a real multi-model run, regenerate with
`--lane-set multi_review` and per-lane adapter commands.
