# Striatum Reliability Reset Evaluation Prompt

You are evaluating Striatum because the product concept and direction are
right, but the operational reliability is not yet trustworthy. The recent
`divergent_ideation` work exposed fragility: `striatum doctor` has been red
more often than green, runs have needed recovery ceremony, and the project may
be architected into a corner where each feature adds more repair surface than
operator confidence.

This evaluation is intended to run on `proximal` after the current in-flight
Striatum work has settled. Do not start it while other non-terminal runs are
still active unless the operator explicitly overrides that scheduling boundary.

Your job is not to add features. Your job is to produce an evidence-backed
reliability reset plan that tells the maintainer how to unfuck the project.

## Non-Negotiables

- Treat `docs/reference/spec.md` as the product contract. If a doc claim
  disagrees with source behavior, name the drift.
- Treat daemon-owned Postgres as authoritative live state. Do not inspect or
  mutate Postgres directly.
- Do not touch `.striatum/`, private diagnostics, caches, transcripts, or
  unrelated repo files.
- Do not propose hosted services, telemetry, durable transcript capture,
  provider SDK integration, or external persistence.
- Prefer deletion, simplification, and stronger gates over new orchestration
  machinery.
- Do not accept a green `doctor` at face value; explain what it proves, what it
  no longer proves, and whether any warnings have become normalized noise.
- Do not accept generated workflow support-tier claims at face value; trace the
  exact reliability fixture and the defects that escaped it.
- Every important claim must cite local evidence: file paths, function names,
  command outputs, decision rows, RFCs, tests, or recent operator artifacts.

## Required Reading

Read these first, in this order:

1. `README.md`
2. `docs/index.md`
3. `docs/reference/spec.md`
4. `docs/decisions/decision-log.md`
5. `docs/reference/ubiquitous-language.md`
6. `docs/reference/todo.md`
7. `docs/operator/BRIEF.md`
8. `docs/how-to/how-to-agent.md`
9. `docs/reference/workflow-catalog.md`
10. `docs/reference/workflow-types.md`

Then inspect the code and tests behind the areas that appear most connected to
reliability:

- `go/pkg/reads/doctor*.go`
- `go/pkg/mutations/recovery*.go`
- `go/pkg/mutations/supervision*.go`
- `go/pkg/mutations/worktree*.go`
- `go/pkg/cli/rundrive/`
- `go/pkg/workflowgenerate/shapes_divergent.go`
- `go/pkg/workflowgenerate/*divergent*_test.go`
- `go/pkg/adapterconformance/`
- `contracts/daemon_methods.json`
- `docs/reference/command-authority-matrix.md`

## Commands To Run Unless Blocked

Run commands from the repository root. Capture summaries, not raw dumps:

```bash
git status -sb
striatum operator bootstrap --markdown
striatum status --json
striatum doctor --verbose --json
striatum workflow validate docs/operator/workflows/striatum-reliability-reset-2026-06-16/workflow.json --json
make lint
make typecheck
make test
```

If a command is unavailable, too slow, or blocked by local daemon state, record
the exact failure and continue with source-level evidence.

## Questions To Answer

1. What are the top reliability failure modes right now, sorted by operator
   pain and blast radius?
2. Which failures are architecture problems, which are implementation defects,
   which are test-harness gaps, and which are operator UX / truth-surface gaps?
3. Did `divergent_ideation` reveal a narrow bug, or did it reveal that workflow
   shape complexity is outpacing the runner's proven reliability envelope?
4. What should be frozen immediately until reliability is restored?
5. What should be deleted or demoted, even if it is intellectually attractive?
6. What does `doctor` need to mean for operators to trust it again?
7. Which "supported" shape or adapter claims are not actually supported by
   current unattended fixtures, installed-CLI coverage, or real dogfood history?
8. What is the smallest reset plan that would make Striatum boringly reliable
   for core runs before it grows again?

## Required Output

Produce a concise but complete report with these sections:

1. **Executive Verdict**: one paragraph. Use one of:
   `keep-going-with-freeze`, `simplify-before-growth`,
   `architecture-reset-required`, or `concept-not-viable`.
2. **Failure Taxonomy**: table of findings with severity, evidence, current
   guardrail, why it failed, and recommended disposition.
3. **Doctor Signal Review**: what currently makes doctor red/green, what was
   reclassified, and what must stay red.
4. **Divergent Ideation Postmortem**: what escaped the reliability fixture,
   whether the shape should remain supported, and the minimum graduation gate.
5. **Architecture Corner Check**: name any abstractions, authority boundaries,
   recovery loops, or workflow-shape mechanisms that are too tight, duplicated,
   or fighting each other.
6. **Delete / Freeze / Fix Plan**:
   - Delete or demote list.
   - Freeze list.
   - P0 fixes with exact tests.
   - P1 fixes with exact tests.
7. **Two-Week Reset Plan**: day-by-day or milestone plan for restoring trust.
8. **Definition of Done**: measurable criteria, including doctor posture, run
   reliability, conformance coverage, and release gating.

Be blunt. If the project needs to stop adding shapes, say so. If a feature is
good but premature, say so. If the core concept is good and the fix is a
reliability freeze plus deletion pass, say that.
