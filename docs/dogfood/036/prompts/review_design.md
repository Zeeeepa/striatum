# Review Design Prompt (ergonomics_dx posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0034", "workflow-generator", "catalog"]
---
```

Review `docs/dogfood/036/DESIGN_SYNTHESIS.md` under the **ergonomics_dx** posture. Use only an accepting verdict (`accept` or `accept_with_findings`) if the plan makes the workflow generator's affordances discoverable and consistent from a first-time-user perspective.

The ergonomics_dx posture (per RFC 0018 and the Striatum workflow module): "This is a developer-ergonomics review. Evaluate the artifact's surface from a first-time-user perspective; verdict acceptance means the affordances are discoverable and consistent."

**The web `/workflows/new` chooser UI and the chat-assisted scaffolding tool are EXPLICITLY DEFERRED** to a follow-up dogfood. Do not refuse the design for their absence as long as the synthesis lists them as deferred coverage with clear pointers.

In scope for this review:

- **CLI verb naming**: are `workflow templates list`, `workflow templates show`, `workflow generate` intuitive? Does each verb's help text give an example invocation? Does the operator have to guess what `--lane-set` accepts or does `templates list` show them?
- **Required vs optional flags**: are required flags named clearly? Does an attempt with missing flags fail loudly with `field_path`-bearing errors, or silently?
- **`--dry-run` as safe default**: does the synthesis encourage operators to dry-run first? Is `--dry-run` ergonomic enough to be the first thing the operator types?
- **Catalog metadata quality bar**: does `recommended_for` give a specific, actionable heuristic per shape and per lane set, or is it boilerplate? Does `summary` actually explain a shape in one line a first-time operator can act on?
- **Custom-plan compiler error messages**: does each refusal case carry `field_path`? Can an operator who tried `shape: "custom"` with an unbounded cycle figure out where the cycle is from the error alone?
- **Symmetric envelope across surfaces**: Python API + CLI `--json` + local API preview/write must return the same `GeneratedWorkflow` envelope. Does the synthesis specify this symmetry? Will an AI client get the same field shape the CLI does?
- **Refuse-to-overwrite**: non-dry-run writes refuse to overwrite existing paths. Does the synthesis make this the default? Is the future `--force` flag explicitly named as deferred?
- **`workflow init --style` backwards-compatibility**: does the synthesis specify that the legacy verb still works and dispatches through the new generator? Will existing users see a regression?
- **Validation-on-return guarantee**: does the synthesis specify that every generated `workflow.json` immediately passes `workflow validate` or the generator returns a structured error? Is generation-bug-becomes-invalid-starter-file impossible?
- **Lane-modifier compatibility matrix**: is the matrix complete (every modifier × lane-set cell decided), with `required` / `allowed` / `forbidden` / `warning` per cell? Will a first-time operator who picks an incompatible combination get a useful field-specific error?
- **Scope discipline**: does the synthesis stay inside RFC 0034 V1, or wander into the web chooser UI, chat tool, hosted marketplace, or repository inspection territory?

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. Focus on actionable ergonomic gaps, not aesthetic preferences. For `accept_with_findings`, the findings must be non-blocking and explicitly say so.

Stay inside the review write scope (`docs/dogfood/036/review/design/ergonomics/`). Do not modify the synthesis. Do not call striatum CLI; the operator publishes otherwise.
