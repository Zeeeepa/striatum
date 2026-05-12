# Claude Code Design Prompt

Produce `docs/dogfood/036/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0034 emphasizing the first-time-user experience (ergonomics_dx posture is the lens here). The reviewer will judge whether the affordances are discoverable and consistent from a first-time operator's perspective.

Focus on:

**Discoverability:**

- How does a first-time operator know that a generator exists? `striatum workflow --help` should surface it. `striatum workflow templates list` should be the obvious starting point.
- How does the operator pick a shape without combinatorial paralysis? Catalog metadata (`display_name`, `summary`, `recommended_for`) must be specific and actionable, not boilerplate.
- How does the operator know which lane set fits their environment? `recommended_for` on each lane-set entry should give a concrete heuristic.

**Choose-shape -> choose-lane-set -> fill-required-fields -> preview-graph -> review-JSON -> save flow:**

- The CLI should encourage `--dry-run` as the default first pass: see what would be written before committing.
- The Python API and local API must return the same `GeneratedWorkflow` envelope so an AI assistant or chat tool can present the same preview the CLI does.
- The web `/workflows/new` chooser UI implements this flow visually, but it is EXPLICITLY DEFERRED to a follow-up dogfood; the V1 slice ships CLI + Python API + local API only, with the UI documented as deferred coverage.

**CLI ergonomics:**

- required flags are named clearly (`--shape`, `--lane-set`, `--artifact-root`)
- optional flags compose cleanly (`--lane-modifier <m>` repeatable, `--plan <path>` only for `shape: custom`)
- error messages include `field_path` so the operator can fix a spec without scraping prose
- help text on every new verb is short, concrete, and gives an example invocation
- `--json` output is the structured machine-readable surface; the human surface is short prose with the same field labels

**Local API surface that AI / operator-surrogate clients call:**

- the API must serve the SAME generator + envelope the CLI uses
- read endpoints (`GET /workflow-templates`, `GET /workflow-templates/<id>`) are safe for AI operators to call freely
- mutation-gated write endpoint (`POST /workflows/generate`) requires `confirm_write: true` and is behind `--allow-mutations`; preview endpoint (`POST /workflows/generate/preview`) is non-mutating
- structured field errors with `field_path` so an AI client can repair a spec from the error without scraping prose

**Backwards-compatibility:**

- `workflow init --style minimal|review|code-change` must still work
- existing users who type that command should not see surprise; the new generator runs under the hood
- the new `workflow generate` is a richer, explicit surface for users who want shape × lane-set × modifiers

**Trust / safety boundary (kept light per ergonomics_dx scope):**

- generator output is always JSON-only and immediately valid under `workflow validate`
- refuse-to-overwrite posture on non-dry-run writes; explicit `--force` left for a future RFC
- no hosted marketplace, no remote template fetch, no telemetry; all catalog data is local package data

**Concrete touch points in `src/striatum/`:**

- new `workflow_generator` module
- new `workflow_templates` package-data tree
- CLI route additions in `cli/workflow.py` or `cli/dispatch.py`
- local service routes — identify the existing surface and explain how to register the new endpoints

**Explicitly deferred to a follow-up dogfood:**

- web `/workflows/new` chooser UI flow (RFC 0034 §9)
- chat-assisted scaffolding tool (RFC 0034 §10)
- target-repo local catalog extensions (RFC 0034 §6 V1.5)
- automatic repository inspection / suggested shapes

State this deferral plainly in your design so the synthesis can carry it forward.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-claude-opus-001`
- Wrong: `**Author:** ...`, `Author: ...`, etc.

The `handoff` kind does not require YAML front matter. Schema-bearing artifacts later in this dogfood do.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
