---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["rfc-0046", "design-review", "ergonomics_dx"]
---

author: reviewer-unknown-model-001

# Design Review — RFC 0046 V1 lane evidence guard

Posture: `ergonomics_dx`. Evaluates `DESIGN_SYNTHESIS.md` on whether a
first-time implementer / operator can act on the decisions without
re-deriving missing context.

## Verdict

`accept_with_findings`. The decision shape, implementation order, and
acceptance matrix are sound and map cleanly to RFC 0046. Two ambiguities
in the guard logic and three naming / discoverability gaps should be
resolved in the build round but do not block acceptance of the design.

## Findings

### F1 — `F-*` decision tags are undefined (severity: low)

The synthesis labels decisions `F-schema`, `F-guard`, `F-override`,
`F-event`, `F-test`. The prefix is never expanded. A first-time reader
opening this file in isolation cannot tell whether `F-` denotes
"feature", "finding", or a back-reference to the design rounds. A one-
line legend ("F-* identifies decision facets carried through the design
rounds") would remove the friction.

### F2 — "model byline" detection criteria is under-specified (severity: medium)

`F-guard` says: *"If it matches a model byline (`<role>-<model>-<ord>`),
look up matching `process_executions` rows."* This synthesis is the
spec for the guard's branching predicate, yet:

- The synthesis itself is signed `author: designer-unknown-model-002`.
  Does `unknown-model` qualify as a model byline? The acceptance matrix
  exercises "model byline" vs. "operator byline" as a binary, but the
  detector is left to the implementer.
- The regex / parser is not pinned. `reviewer-claude-code-001`,
  `reviewer-unknown-model-001`, and `operator-human-001` all share the
  `<a>-<b>-<n>` shape; the differentiator (model registry? presence in
  a known-model set? non-`human` middle field?) needs to be named.

Recommend: state the predicate explicitly — e.g. "the middle segment is
non-empty and not equal to `human`, `operator`, or `unknown`" — or
point at a helper symbol (`is_model_byline()`) that the build round
must produce.

### F3 — "cover the artifact path" is undefined (severity: medium)

`F-guard` rejects when "no `process_executions` rows cover the artifact
path". "Cover" is the central matching predicate but the synthesis
never defines it. Plausible readings differ:

- Exact path equality on a `path` column.
- Path-prefix containment (allowing a single `process_executions` row
  to attest a directory of artifacts).
- Session / lane scope match plus path equality.
- Any `process_executions` row in the session, regardless of path.

Each variant has different false-negative / false-positive behavior.
The build round will pick one by default; the design should pin it now
so the review of the build artifact can verify the choice.

### F4 — Naming triangulation: attestation vs. lane-evidence vs. process-execution (severity: low)

Three nouns are in flight for the same concept:

| Surface | Phrase used |
|---------|-------------|
| Schema column | `attestation_override_rationale` |
| Error code | `lane_evidence_missing` |
| Event | `provenance.publish_without_process_execution` |
| CLI flag | `--allow-no-process-execution` |
| Decision name | `F-guard` (lane evidence) |

A first-time operator reading the error, then grepping the schema,
then grepping events will hop across three vocabularies. Recommend
consolidating to one canonical noun ("process execution") and treating
the column name (`attestation_override_rationale`) as the outlier to
fix — `process_execution_override_rationale` aligns with the rest, and
also addresses the implicit empty-string-vs-NULL question by tying the
column name to the same concept the rationale is overriding.

### F5 — Override flag semantics: empty rationale vs. missing flag (severity: medium)

`F-override` says: *"Override with empty rationale refuses (exit 2).
Override + rationale stores rationale on the artifact row + emits
event."* Two CLI scenarios are not distinguished:

1. `--allow-no-process-execution` without `--override-rationale` at all.
2. `--allow-no-process-execution --override-rationale ""`.

Both presumably exit 2, but the user-facing error should differ
("missing required argument" vs. "rationale must be non-empty").
Pinning the argparse coupling (is `--override-rationale` declared
`required=True` on the parser, or enforced post-parse?) would prevent
two builds disagreeing on the exit-2 reason and would also make
`publish-artifact --help` self-document the dependency.

### F6 — Error-message wording is a placeholder (severity: low)

`ArtifactError("lane_evidence_missing: ...")` leaves the actual user-
facing string to the implementer. RFC 0046 promises the remediation is
"named" in the error; ergonomics depends entirely on that wording.
Recommend the design fix the error template now, e.g.:

```
lane_evidence_missing: no process_executions row covers
  artifact path '<path>' for model byline '<byline>'. Re-run with
  --allow-no-process-execution --override-rationale '<why>' to
  publish anyway (event provenance.publish_without_process_execution).
```

### F7 — Operator-byline bypass is implicit (severity: low)

The acceptance matrix lists "Operator-byline publish → passes through
unchanged regardless." The `F-guard` decision text never states the
bypass; a reader skimming the Decisions block would assume the guard
fires for every publish. Add a sentence to `F-guard`: "Operator
bylines (middle segment `human`) skip the lookup entirely."

## What's working

- Implementation order matches the decision order — easy to walk top-
  to-bottom while building.
- Acceptance matrix is enumerable (5 cases) and maps 1:1 to the RFC
  0046 acceptance criteria.
- Migration touches both `migrations.py` and `daemon_pg/sql/`, so
  Postgres parity is not left as a follow-up.
- CLI surface is two flags rather than a mode switch — a single
  combined `--allow ... --rationale ...` pair is easier to discover in
  `--help` than a sub-command.
- Stored override rationale + event emission means the override path
  is auditable rather than silently bypassing the guard.

## Recommended build-round preconditions

Before the build round begins, the synthesis (or its successor design
note) should pin:

1. The model-byline predicate (F2).
2. The `process_executions`-to-artifact match rule (F3).
3. The canonical noun (F4) — rename the column if needed.
4. The error-message template (F6).

F1, F5, and F7 are documentation-only and can be folded into the build
PR without blocking the implementer.
