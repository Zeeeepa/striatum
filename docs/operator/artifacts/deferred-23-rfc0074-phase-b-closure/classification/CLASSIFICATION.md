---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/deferred-23-rfc0074-phase-b-closure/surface/SURFACE_MAP.md", "docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md", "docs/operator/artifacts/rfc-0074-phase-a-catalog/CLOSURE.md", "docs/operator/artifacts/deferred-15-rfc0052-closure/final/SUMMARY.md"]
---

# RFC 0074 Phase B Classification
author: deferred23-classifier-codex-gpt-5-001

## Classification

RFC 0074 Phase B generator pack behavior is ready to schedule as a bounded
implementation workflow. It does not require a new product RFC before the
narrow generator slice begins.

The safe scheduled scope is:

- generate `implementation_panel` only, using the already-validating example
  as the behavioral reference;
- accept a single `role_pack` and a single `adversary_pack` from the bundled
  catalog;
- accept bounded `proposal_count` and `score_dimensions` options with
  conservative defaults;
- emit ordinary validated `workflow.json` trees with existing job types,
  review postures, roles, prompts, and artifact kinds;
- update Python and Go generator parity plus CLI/service tests in the same
  patch series.

## Why A New RFC Is Not Required

RFC 0074 already defines the role/adversary pack vocabulary, the
`implementation_panel` graph, the Phase B generator acceptance criteria, and
the boundary with RFC 0052. Phase A proved the catalog metadata and the
ordinary-workflow example. The remaining generator work is an implementation
of that accepted shape, not a new runtime semantic.

The bounded generator slice does not need:

- a PostgreSQL migration;
- new daemon state;
- new workflow live-state transitions;
- new artifact kinds;
- new front-matter schemas;
- hosted template retrieval;
- model-provider identity semantics;
- RFC 0052 committee-deliberation artifacts or daemon methods.

## Required Scope Split

Do not schedule all Phase B/C language as one source patch.

| Surface | Classification | Reason |
|---|---|---|
| Generator shape `implementation_panel` | Ready to schedule | Existing example validates on current primitives, and Python/Go generator parity is straightforward but must be tested together. |
| `role_pack` / `adversary_pack` generator options | Ready to schedule | Catalog metadata already has the closed IDs; keep this to one pack each and reject unknown IDs with field-specific errors. |
| `proposal_count` / `score_dimensions` | Ready to schedule with defaults | Required for useful panel generation, but keep validation local to generator options and do not make packs workflow-schema fields. |
| Service/API pack display | Ready to schedule with generator | Read endpoints already list packs; preview/write envelopes can expose selected pack metadata after generator support lands. |
| Browser chooser selectors | Separate bounded UI follow-up | Current source has no active chooser route/island to extend. Build only after generator/API behavior is stable. |
| Cost/artifact-volume warnings | Separate bounded UI or generator-lint follow-up | Useful before write, but the policy and presentation are not needed to prove generator correctness. |
| RFC 0052 debate semantics | Not in this work | Deferred item 15 already classified RFC 0052 as requiring its own bounded implementation design. |

## Scheduling Contract

The next implementation workflow should have these acceptance checks:

1. `workflow generate --shape implementation_panel --role-pack
   implementation_panel_roles --adversary-pack maintainer_cost --dry-run
   --json` succeeds and returns a valid generated workflow.
2. The same generation path works through Go daemon
   `workflow.generate.preview`.
3. Generated output validates with `workflow validate` and uses only current
   artifact kinds.
4. Invalid pack IDs, incompatible packs, invalid `proposal_count`, and invalid
   `score_dimensions` produce field-specific errors in Python and Go.
5. Metadata in the generated envelope records selected graph shape, lane set,
   role pack, adversary pack, proposal count, and score dimensions.
6. Service/template response tests still list graph shapes, role packs, and
   adversary packs separately.
7. Documentation in the generated prompt stubs or catalog entry makes the
   panel artifact volume visible before write.

## Guardrails

- Do not add RFC 0052 artifact kinds: `debate_turn`,
  `arbitration_ruling`, `panel_vote`, `panel_verdict`, or
  `debate_synthesis`.
- Do not add committee phase validation or panel daemon methods.
- Do not make role/adversary packs daemon state, runtime gates, lane/model
  identity, or workflow schema requirements.
- Do not edit shared TODO, roadmap, or operator brief status from this
  closure packet.
