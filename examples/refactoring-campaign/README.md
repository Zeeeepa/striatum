# Refactoring campaign (goal selection → plan gate → execution)

A reusable three-stage campaign for **one named, behavior-preserving
refactoring**: a refactor changes code shape — boundaries, ownership,
names, duplication, dependency direction, test seams, or module layout —
and does not change observable behavior. Unverifiable rewrites are out of
scope by construction.

The campaign is **three chained runs**, each on a `supported`-tier shape
(see `docs/reference/workflow-catalog.md`), rather than one mega-run. No
new workflow shape is introduced; this example composes graduated shapes,
which is the direction the RFC 0106 shape freeze points (depth over
breadth — compose, don't mint).

```
stage 0: goal selection      stage 1: plan gate           stage 2: execution
(implementation_panel)       (falsification_gate)         (code_change)

survey ─→ goal A ─┐          plan holder                  execute slices ──→ preservation
       ─→ goal B ─┼→ score   (preflight + step table)     (isolated worktree,   review
       ─→ goal C ─┘   ×3        │                          slice-per-commit)      │
            │          │     falsifier 1 ─→ falsifier 2       ↑    │              │
            └→ tradeoff ledger    │                           └────┼── needs_revision
                  │            adjudicator ledger                  ↓
              arbitrate ─→ dissent ─→ GOAL_DECISION   committed plan → FINAL_REPORT
```

Each stage's terminal artifact is the next stage's input contract:

| Stage | Shape | Terminal artifact | Consumed by |
|---|---|---|---|
| 0 — goal selection | `implementation_panel` graph | `artifacts/00-goal/GOAL_DECISION.md` (`decision`) | stage 1 plan holder |
| 1 — plan gate | `falsification_gate` graph | `artifacts/01-plan/COMMITTED_PLAN.md` (plan + discharged binding constraints) | stage 2 author |
| 2 — execution | `code_change` graph | `artifacts/02-execution/FINAL_REPORT.md` + the executed worktree | operator integrate |

## Why a slice loop inside one job, not slice-per-job

A running run executes its frozen workflow snapshot, but the slice table
is *produced by* stage 1 — so slices cannot be jobs. The execution loop
(smallest change → named verification → ledger evidence → one verified
slice per commit) lives inside the `execute_slices` job, and the
discipline is enforced by the gates around it: the falsified plan before,
the replayed preservation review after.

## Stop semantics

The campaign is designed to stop cheaply at every boundary:

- **Stage 0** can refuse all candidate goals (arbitrator) or falsify the
  selection (dissent reviewer).
- **Stage 1** refuses the gate when a binding constraint cannot be
  discharged without behavior change or a goal change. A refused gate is
  a campaign stop. Plan rework is bounded: one in-run challenge cycle,
  then at most one fresh stage-1 run; needing a third is a stop.
- **Stage 2** treats a slice exceeding its declared cap, a baseline that
  does not reproduce, or behavior change discovered mid-slice as stop
  conditions recorded in the step ledger — never stretched through.

An honest stop with a truthful ledger is a successful run of the
workflow, not a failure.

## How to run

The daemon is a hard prerequisite for every Striatum verb. Validate each
stage, then prepare and start runs in order; later stages read earlier
stages' finalized artifacts from the repository.

```sh
striatum workflow validate examples/refactoring-campaign/stage-0-goal-selection/workflow.json
striatum workflow validate --allow-same-model-pairing examples/refactoring-campaign/stage-1-plan-gate/workflow.json
striatum workflow validate --allow-same-model-pairing examples/refactoring-campaign/stage-2-execution/workflow.json

striatum run prepare --workflow examples/refactoring-campaign/stage-0-goal-selection/workflow.json
striatum run start --run-id <id>
striatum dashboard --run-id <id>
# ...stage 0 finalizes GOAL_DECISION.md, then prepare stage 1, then stage 2.
```

## Adapting this fixture to a real campaign

The shipped lanes are `local` fixture lanes so the workflows validate and
can be driven operator-by-hand. For a real campaign:

- **Bind real lanes.** Replace the fixture lanes with agent-loop lanes
  (see `examples/iterated-interrogating-panel/README.md` for the lane
  shape). Keep the stage-2 reviewer on a different model family from the
  author.
- **Set the artifact root.** In a target repository, change every
  `examples/refactoring-campaign/artifacts/` path to a campaign-scoped
  root, e.g. `striatum/refactoring/<campaign-slug>/`.
- **Set stage 2's write scope from the committed plan.** The placeholder
  `src/example/` in `execute_slices.write_scope.allowed_paths` must become
  the plan's blast radius; add path-shaped frozen surfaces (migrations,
  generated files, docs) to `forbidden_paths`. Frozen surfaces that are
  not path-shaped (exported signatures, CLI output, wire formats) cannot
  be expressed as write scope — they are enforced by the falsifiers and
  the preservation review, which is why those jobs exist.
- **Keep worktree isolation.** The stage-2 author lane declares
  `worktree_isolation: "per_job"`; landing the executed worktree is the
  operator's serialized integrate step after the final report, not part
  of the run.
- **Branch confirmation.** Each workflow uses `branch.mode: confirm`;
  pass `--branch` at prepare time.

## Artifact layout

```
examples/refactoring-campaign/artifacts/
  00-goal/PROBLEM_BRIEF.md                    # survey (handoff)
  00-goal/proposals/GOAL_{A,B,C}.md           # independent goal champions (handoff)
  00-goal/scorecards/SCORECARD_{A,B,C}.md     # fixed-dimension scorecards (finding)
  00-goal/TRADEOFF_LEDGER.md                  # normalized comparison (findings_ledger)
  00-goal/ARBITRATOR_SYNTHESIS.md             # selection or refusal (synthesis)
  00-goal/DISSENT_REVIEW.md                   # falsification attempt (finding)
  00-goal/GOAL_DECISION.md                    # the named goal (decision)
  01-plan/REFACTORING_PLAN.md                 # preflight + step table (work_plan)
  01-plan/falsifier_{1,2}/FALSIFIER.md        # challenges (handoff)
  01-plan/adjudicator/COLLABORATION_LEDGER_${cycle}.md
  01-plan/COMMITTED_PLAN.md                   # plan + discharged constraints (synthesis)
  01-plan/GATE_SUMMARY.md                     # gate verdict summary (synthesis)
  02-execution/STEP_LEDGER.md                 # per-slice evidence (support_ledger)
  02-execution/PRESERVATION_REVIEW.md         # replayed audit (finding)
  02-execution/FINAL_REPORT.md                # campaign provenance (synthesis)
```

All write scopes are disjoint per job
(`parallelism.require_disjoint_write_scopes: true`).
