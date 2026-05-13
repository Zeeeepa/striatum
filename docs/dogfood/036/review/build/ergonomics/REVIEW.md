---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0034", "workflow-generator", "build"]
---

author: reviewer-claude-opus-001

# Ergonomics-DX Review: RFC 0034 Workflow Generator V1

Scope: implementation under `src/striatum/workflow_generator/`,
`src/striatum/workflow_templates/`, CLI parser/dispatch additions,
`workflow init` rewire, service catalog and generation endpoints, and
documentation updates landed on `striatum/dogfood-036-rfc-0034-workflow-generator`.

This review is fresh-context and posture-restricted to developer
ergonomics: are the new affordances discoverable and consistent from a
first-time operator's perspective? The operator denied direct invocation
of the striatum CLI during this review session (consistent with the
prompt's "do not call striatum CLI; the operator publishes otherwise"
instruction), so the assessment is based on reading the parser, dispatch,
generator, write helper, catalog, tests, and docs. The shape of every
ergonomic claim below is derivable from source.

## Verdict

`accept_with_findings`. The V1 slice the BUILD_HANDOFF lists is shipped
in coherent form: the generator core (`src/striatum/workflow_generator/core.py:211`)
calls `validate_workflow` before returning success and re-validates after
filesystem write (`src/striatum/workflow_generator/write.py:41`). Refuse-
to-overwrite, repo-relative path enforcement, `.striatum/` containment,
catalog read endpoints, mutation-gated write endpoint with
`confirm_write: true`, and `workflow init --style` dispatch through the
generator are all wired. Docs (`docs/SPEC.md:101-122`,
`docs/UBIQUITOUS_LANGUAGE.md:61,64`, `docs/CLI_REFERENCE.md:33-42`,
`docs/HOW_TO_HUMAN.md:133-146`, `docs/WORKFLOW_TYPES.md:36-37,51-60`,
`docs/WRITING_WORKFLOWS.md:37-54`, `docs/rfcs/README.md:49`,
`docs/TODO.md:221-227`) consistently describe the shipped surface and
flag the deferred web chooser and chat tool. The findings below are
non-blocking cleanup; none of them gate the V1 acceptance.

## What works well from a first-time-operator perspective

- **`workflow generate --help` carries a usable example.** The
  description on `src/striatum/cli/parser.py:230-236` shows the full
  invocation including `--shape`, `--lane-set`, `--artifact-root`,
  `--dry-run`, and `--json`, plus the explicit note that
  `workflow validate` remains authoritative. This is the kind of help
  text that bootstraps a first-time operator.
- **`field_path` is carried through the spec boundary consistently.**
  Every refusal in `core.py` (unknown spec field, unsupported
  schema_version, unknown shape/lane_set/modifier/option, missing
  lane, malformed lane command, posture failure, custom-plan
  validation, harness-profile body, constraint values, path escape)
  attaches a `field_path` and propagates it through the CLI envelope
  (`src/striatum/cli/dispatch.py:88-98`) and through the service
  generator-error helper (`src/striatum/service.py:722-733`). The hint
  field is also used for the over-the-wire error envelope. Operators
  who receive an error get a path they can act on.
- **Validation-on-return is enforced in both paths.** The pure
  generator validates before returning a `GeneratedWorkflow`
  (`core.py:240-243`); the writer additionally calls `load_workflow`
  on the just-written file (`write.py:41`). A generator bug cannot
  produce an invalid `workflow.json` that bypasses
  `workflow validate`.
- **Refuse-to-overwrite is enforced at two layers.** `workflow init`
  refuses an existing target (`src/striatum/cli/workflow_init.py:18-21`).
  `write_generated_workflow` checks every target before writing any
  (`write.py:25-31`), so a partial-overwrite race cannot land.
- **The local API preview is mutation-free.** The non-`confirm_write`
  POST is rejected with a clean `field_path` of `confirm_write`
  (`service.py:709-711`), and the bare `POST /workflows/generate`
  without `--allow-mutations` is rejected with `405` and the structured
  `server.allow_mutations` field path (`service.py:695-707`). This is
  exactly the contract RFC 0034 §8 promises for AI operators.
- **Catalog metadata is specific, not boilerplate.** Every entry in
  `src/striatum/workflow_templates/catalog.json` has a one-sentence
  `summary` that names what the shape does (e.g., "Collect several
  independent reviews before a final recommendation.") and a
  `recommended_for` list with concrete use cases. The `graph_preview`
  blocks let a chooser UI render a shape without re-deriving from
  source. The boilerplate test ("recommended_for: [proposal review,
  bug triage, documentation review]") passes the squint test.
- **`workflow init --style` still works and dispatches through the
  generator.** `workflow_init.py:24-32` builds a `default_spec` with
  `lane_set="local"` and returns the legacy envelope shape that
  existing users see (`status`, `path`, `workflow_path`, `style`,
  `files`). The compatibility seam is documented in
  `docs/SPEC.md:108-109`. `tests/test_workflow_generator.py:133-141`
  pins the envelope and the `cycles[0].on_verdict` so regressions are
  loud.
- **The catalog loader is defensive about its own contract.**
  `catalog.py:25-39` rejects unknown schema_version, non-list shapes
  or lane_sets, missing template_id, and wrong `kind`. A future
  packager who corrupts the catalog gets a clear error rather than a
  cryptic `KeyError` during CLI use.
- **Documentation is honest about deferred scope.** The RFC 0034 file
  retains the "V1 implementation note" paragraph naming exactly the
  shipped slice; `docs/TODO.md` item 18 enumerates the remaining web
  chooser, chat tool, and target-repo extension; `BUILD_HANDOFF.md`
  lists the same deferred items. There are no claims of a `/workflows/new`
  UI or chat assistance in the user-facing docs.
- **Decision log carries the acceptance.** `docs/DECISION_LOG.md:25`
  (D090) records the V1 acceptance, the deferral, and the contract
  that generated workflows remain ordinary `striatum.workflow.v1` JSON
  with no migration. The revisit trigger names exactly the deferred
  items.

## Findings (non-blocking)

### F1. Custom-plan refusal coverage is sparse in tests (medium)

The work-packet acceptance criteria call for "every custom-plan refusal
case is tested." The shipped test
`tests/test_workflow_generator.py:77-92` covers only the custom happy
path; the negative cases enforced by `_compile_custom`,
`_custom_edges`, `_custom_cycles`, and `_safe_artifact_path` (unknown
block `kind`, duplicate block id, missing lane binding, lane binding to
absent lane, review-only field on non-review block, edge referencing
missing block, cycle source that is not a review block, non-positive
`max_iterations`, base-edge cycle, derived path escaping
`artifact_root`, `.striatum/` writes) have no explicit refusal tests.

Why this matters for ergonomics: every one of these paths is a place
where an operator hand-writing a custom plan will get its first error.
Without a regression net, a future refactor can quietly turn a useful
field-pointing refusal into a generic crash. Add one parametrized test
per case asserting both the message and the `field_path`. The fixtures
are small; the cost is low.

A related sub-gap: `tests/test_workflow_generator.py:50-63` validates
one (shape, lane_set) combination per shape rather than every
compatible cell from `docs/WORKFLOW_TYPES.md` and the catalog's
`default_lane_sets` list. Iterate over the catalog's
`default_lane_sets` for each shape to make "every built-in shape ×
every compatible lane set validates" true by construction.

### F2. Help text is uneven across the new verbs (medium)

`workflow generate` has a usable description with an example invocation
on `src/striatum/cli/parser.py:230-236`. The other new verbs do not:

- `workflow` (parent) — no description.
- `workflow templates` (parent) — no description.
- `workflow templates list` — only argparse-default flag listing.
- `workflow templates show` — only argparse-default flag listing.

A first-time operator who types `striatum workflow templates --help`
sees only "list" and "show" with no hint of what they expose or how
to feed the output back into `workflow generate`. The fix is mechanical:
add `description=...` strings with one concrete example each (e.g.,
`striatum workflow templates show code_change --json`). Cross-link
`workflow generate` in the `templates` description and vice versa.
The catalog browser is the discovery entry point; uneven help text at
the entry point is the highest-leverage ergonomic gap I found.

### F3. Write path returns a wrapper envelope, not the bare `GeneratedWorkflow` (medium)

The work-packet check requires that "Python API + CLI `--json` + local
API preview/write must return the same `GeneratedWorkflow` envelope."
Preview is symmetric:

- `GeneratedWorkflow.to_json()` →
  `{workflow, files, metadata, warnings, validation}`.
- `POST /workflows/generate/preview` data →
  `{workflow, files, metadata, warnings, validation}`
  (`service.py:715-716`).
- `striatum workflow generate ... --dry-run --json` data →
  same (`dispatch.py:768-769`).

Write is asymmetric:

- `POST /workflows/generate` data →
  `{status, path, workflow_path, files: [string], generated:
  {workflow, files, metadata, warnings, validation}}`
  (`write.py:49-55`).
- `striatum workflow generate ... --json` (non-dry-run) data → same.

RFC 0034 §8 explicitly shows the write response carrying the bare
envelope at `data`. A tool author who reads the RFC, learns
`data.workflow`/`data.files` on preview, then writes against the
service expecting the same access path will break — the envelope is
nested under `data.generated` on write, and `data.files` becomes a
list of relative path strings rather than the structured
`{path, content}` array preview returns. Pick one of:

- have `write_generated_workflow` return the bare envelope plus a
  `wrote: ["..."]` field instead of nesting `generated`, then update
  the docs;
- or document the wrapper shape explicitly in RFC §8 / SPEC and add a
  test asserting both shapes so the asymmetry is a deliberate contract.

The fix is small either way; the asymmetry is the kind of thing that
will trip up the chat tool and `/workflows/new` clients when they
arrive.

### F4. Base-edge cycle error points at the array, not the offending edge (low)

`_assert_acyclic` raises `"custom plan base edges contain a cycle"`
with `field_path="spec.plan.edges"` on `core.py:717`. The RFC's stated
ergonomic property ("a plan with `shape: "custom"` and an unbounded
cycle should refuse with a `field_path` pointing at the cycle entry")
implies the error should point at a specific edge index. Walk the
topological-sort failure to identify the first node that participates
in the cycle and report `spec.plan.edges[i]` for one offending edge.
This is the kind of small-but-real difference between an operator
fixing a 30-edge plan in one shot vs. binary-searching it.

### F5. `--lane-command` JSON-array shape is brittle for the common case (low)

`_parse_keyed_json_arrays` (`dispatch.py:773-788`) requires the value
to be a JSON-encoded array, so a simple two-word command becomes
`--lane-command author='["codex","exec"]'`. That works, but mis-quoting
the JSON (single quotes outside a shell that respects them, missing
inner quotes) is a frequent first-time mistake and the resulting error
is `--lane-command value must be a non-empty JSON string array`, which
is true but unhelpful when the failure is shell-quoting. Consider
allowing a simpler shape for the common case (e.g.,
`--lane-command author=codex,exec` or repeated `--lane-arg
author=codex --lane-arg author=exec`) while keeping the JSON form for
arguments that need quoting. The current shape is acceptable; the
ergonomic finding is that the error message could short-circuit
shell-quoting confusion ("if your value starts with `[`, you probably
forgot a quote in your shell; pass the array as JSON").

### F6. `multi_review` reviewer counting is implicit (low)

`_reviewer_count` (`core.py:662-669`) reads `options.reviewer_count`,
then falls back to `len(options.review_postures)`, then to 2. The
catalog's `multi_review` lane-set lists only
`required_options: ["lanes.author.command"]`. A first-time operator
generating a `multi_review_synthesis` workflow with `--lane-set
multi_review` and no `reviewer_count` will silently get 2 reviewers and
must add a `reviewer_1` + `reviewer_2` pair to lanes. The implicit
default is reasonable, but document it in the catalog entry's
`recommended_for` or `summary` and surface it in the dry-run preview's
warnings list. Today, the operator has to read the source to know
where the count came from.

### F7. `worktree_isolated` warning is asymmetric across shapes (very low)

`_validate_modifier_matrix` (`core.py:592-593`) emits a warning when
`worktree_isolated` is combined with `multi_review_synthesis` ("no
effect on review-only jobs except synthesis"), but does not emit a
similar warning when the same modifier is combined with the `review`
shape — which is also a draft-then-review path with a single synthesis
write-target. Either drop the special case to the `review` shape too,
or document the difference in the modifier table. Easier path: drop
the warning entirely; the modifier is additive and harmless, and
warnings about "no effect" rarely help operators in the moment.

## Spot-checks that pass

- `validate_workflow` is called inside `generate_workflow` before the
  return (`core.py:240-243`). Tested transitively by every shape pass
  in `tests/test_workflow_generator.py:50-63`.
- The write helper rejects unsafe relative paths
  (`write.py:58-69`) — `..`, leading `/`, backslash, `.git`, and
  `.striatum` cannot escape; `target.resolve().relative_to(repo)`
  catches anything left.
- Refuse-to-overwrite has tests on both surfaces: the legacy `workflow
  init` path is asserted by the existing init suite (envelope contract
  in `tests/test_workflow_generator.py:133-141` plus the
  `target.exists()` check on `workflow_init.py:18`), and the generator
  CLI write is exercised in `tests/test_workflow_generator.py:115-130`.
  A direct test that the second write against the same `workflows/demo`
  raises `GeneratorError("workflow generate refuses to overwrite ...")`
  would be a cheap belt-and-suspenders addition.
- The service exposes the same generator error envelope as the CLI;
  `_send_generator_error` (`service.py:722-733`) preserves
  `field_path`, `hint`, and `ref` for tool callers. AI operators can
  iterate on a spec without scraping prose.
- `workflow init --style code-change` produces the bounded revision
  cycle (`tests/test_workflow_generator.py:140-141`).
- The catalog's `code_change` graph_preview matches the generator
  output (`draft -> review -> apply` plus the
  `review -> draft on needs_revision` cycle), so chooser tooling
  showing the preview will not lie.

## Documentation honesty check

The user-visible docs name only the shipped surface:

- `docs/SPEC.md:101-122` — generator API, catalog package data, CLI
  surfaces, service endpoints, mutation gate. No web UI claim.
- `docs/UBIQUITOUS_LANGUAGE.md:61,64` — adds `workflow template catalog`
  and `workflow generation spec` entries.
- `docs/CLI_REFERENCE.md:16-18,33-42` — names `templates list`,
  `templates show`, `generate`, plus the dry-run / overwrite-refusal
  behavior. References that the web chooser is future work.
- `docs/HOW_TO_HUMAN.md:133-146` — operator-facing recipe with the
  exact dry-run-then-write sequence.
- `docs/WORKFLOW_TYPES.md:36-37,51-60` — names the generator inline
  with the existing starter table. Roadmap section retains the
  "Roadmap To A Chooser" path but does not assert the chooser exists.
- `docs/WRITING_WORKFLOWS.md:37-54,189-194` — generator recipe plus
  the closed custom-plan block vocabulary.
- `docs/rfcs/README.md:49` — RFC 0034 is `accepted (V1)` with the
  scope summary.
- `docs/TODO.md:221-227` — item 18 enumerates exactly the deferred
  pieces.
- `docs/dogfood/036/BUILD_HANDOFF.md` — explicitly defers the web
  chooser and chat scaffolding, lists "no overwrite / `--force`
  semantics" as deferred.

No doc overclaims the shipped slice. No doc asserts hosted marketplace,
telemetry, or remote fetch. The RFC 0034 file itself retains the V1
implementation note paragraph on lines 102-107.

## Suggested follow-up actions (not required for V1 acceptance)

1. Add parametrized refusal tests for the custom-plan compiler (F1).
2. Add `description=` strings and example invocations to
   `workflow`, `workflow templates`, `workflow templates list`, and
   `workflow templates show` parsers (F2).
3. Decide write-path envelope shape and document it; add an assertion
   test asserting the agreed shape (F3).
4. Improve base-edge cycle error to point at an offending edge index
   (F4).
5. Soften the `--lane-command` JSON requirement for simple values, or
   improve the error message to point at likely shell-quoting issues
   (F5).
6. Document `reviewer_count` defaulting on the `multi_review` catalog
   entry or surface it via dry-run warnings (F6).
7. Drop or generalize the `worktree_isolated` no-effect warning (F7).
