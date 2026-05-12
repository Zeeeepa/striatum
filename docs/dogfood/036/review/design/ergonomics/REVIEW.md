---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0034", "workflow-generator", "catalog"]
---

author: reviewer-gemini-pro-001

# Design review (ergonomics_dx): RFC 0034 workflow generator V1

## Verdict

`accept` — severity `low`.

The synthesis specifies a workflow-generator V1 whose CLI, Python API, and local API affordances are discoverable and consistent from a first-time-operator perspective. The deferred web UI + chat-assisted scaffolding are explicitly scoped out without blocking the core CLI/API utility.

## What the synthesis gets right (ergonomics_dx lens)

- **Verb naming is intuitive.** `striatum workflow templates list/show` and `striatum workflow generate <path>` align with the existing Striatum noun-verb pattern. Help-text directives ("point first-time users to `workflow templates list`", "include exactly one copy-paste dry-run example") make `--help` a real entry point rather than flag listing.
- **`--dry-run` is the documented safe-default first pass.** The CLI explicitly returns the full envelope without filesystem writes; the operator can preview the generated tree before committing.
- **`field_path` is required on every error envelope.** The spec section enumerates `spec.shape`, `spec.lane_set`, `spec.workflow_id`, `spec.scaffold_root`, etc., so an operator (or AI client) can repair an invalid spec from the error alone without scraping prose.
- **Symmetric envelope across surfaces.** Python API + CLI `--json` + local API preview/write all return the same `GeneratedWorkflow` shape. AI/operator-surrogate clients see the same field shape the CLI does.
- **Refuse-to-overwrite is the default.** Non-dry-run writes refuse to clobber an existing path; the V1 explicitly does not ship `--force`, deferring it to a future RFC. Good.
- **Validation-on-return is enforced.** Every generated `workflow.json` is run through `validate_workflow` before the generator returns success. Generation bugs cannot become invalid starter files.
- **`workflow init --style` backwards-compatibility is honored.** The legacy verb stays and dispatches through the new generator. The synthesis even commits to "byte-equivalent generated workflow JSON unless a deliberate decision changes it" — a measurable backwards-compat bar.
- **Lane-command ergonomic gap is already covered.** The CLI surface includes `--lane-command <lane_id>=<json-array>` and `--lane-display-model <lane_id>=<display-name>` flags, so operators using `author_reviewer` or other multi-lane shapes do not have to fall back to manual JSON editing to assign real adapters. (This was the first concern surfaced during a scan of the design phase artifacts; the synthesis added it explicitly.)
- **Catalog metadata quality bar is set.** `recommended_for` is required to be specific (e.g. `["small implementation", "docs/code edits"]`) rather than boilerplate ("flexible workflows"). The synthesis names this requirement so the catalog entries are reviewable.
- **Lane-modifier compatibility matrix is closed and decidable.** Each modifier × lane-set cell is one of `required` / `allowed` / `forbidden` / `warning`. Incompatible combinations raise `GeneratorError(field_path=...)` rather than silently producing odd workflows.
- **Custom-plan compiler safety is explicit.** The closed block vocabulary (`draft | review | synthesis | implementation | test | human_checkpoint | support_ledger | evidence_audit | final_review`) and the refusal cases (unknown block, unbounded cycle, edge to/from nonexistent block, etc.) make `shape: "custom"` mean "compose from known safe building blocks" — not "freehand raw JSON".
- **Scope discipline is honest.** Deferred items (web UI, chat tool, target-repo catalog extensions, automatic repository inspection) are listed with their landing places. Nothing wanders into hosted marketplace, telemetry, remote template fetch, or repository inspection territory.

## Non-blocking findings (severity low)

These do not block implementation. They are recorded so the implementer can fold them in opportunistically.

1. **Help-text examples must include lane-command flags.** The synthesis names `--lane-command` and `--lane-display-model` as flags but does not explicitly require the single copy-paste `--help` example to include them when the chosen lane set has more than one lane. Concrete suggestion: for `author_reviewer`, the `workflow generate --help` example should show both `--lane-command author=...` and `--lane-command reviewer=...` so the first-time operator sees the full shape. Implementer choice.

2. **Catalog `summary` ≠ catalog `recommended_for`.** Both fields are required, but the synthesis does not say what differentiates a one-liner shape `summary` from a `recommended_for` array. Concrete suggestion: define `summary` as the shape's mechanic ("draft → review → revise once → apply") and `recommended_for` as concrete use cases ("small code changes", "docs edits"). Not a blocker; catalog entries can stabilize through normal review.

3. **Backwards-compatibility test surface should explicitly cover `--json` output.** The synthesis commits to byte-equivalent generated `workflow.json` for `workflow init --style {minimal,review,code-change}`, but the `--json` envelope returned by the rewired verb is not separately exercised. Concrete suggestion: snapshot the pre-rewire `--json` envelope shape (keys + types) and assert post-rewire matches. Implementer judgment whether to add this beyond the planned tests in `tests/test_workflow_init_backcompat.py`.

## Out of scope per posture

The ergonomics_dx posture evaluates affordance discoverability and consistency for a first-time user. **Out of scope** for this review:

- Threat-model concerns (capability tokens, prompt-injection, audit chains). RFC 0034 V1 ships only local generation; mutation-gating is reused from the existing `--allow-mutations` flag and is not new in this dogfood.
- Web `/workflows/new` chooser UI — explicitly deferred to a follow-up dogfood. The synthesis lists this as deferred coverage with a clear pointer.
- Chat-assisted scaffolding tool — explicitly deferred to a follow-up dogfood.
- Target-repo local catalog extensions (RFC 0034 §6 V1.5) — deferred.
- Automatic repository inspection for suggested shapes — deferred indefinitely.

## Implementation may proceed

The synthesis is implementation-ready. The three non-blocking findings above are ergonomic polish that can be folded into the implementation or addressed in normal follow-up iteration.

## Acceptance bar checklist

- [x] CLI verbs are intuitive and align with existing Striatum noun-verb patterns
- [x] Required vs optional flags are clearly named
- [x] `--dry-run` is the documented safe-default first pass
- [x] Error messages carry `field_path`
- [x] Catalog metadata is required to be specific, not boilerplate
- [x] Symmetric envelope across Python API + CLI `--json` + local API
- [x] Validation-on-return is enforced
- [x] Refuse-to-overwrite is the default
- [x] `workflow init --style` backwards-compatibility is preserved
- [x] Lane-modifier compatibility matrix is closed and decidable
- [x] Custom-plan compiler uses a closed block vocabulary with safety rules
- [x] Deferred scope is named with landing places
