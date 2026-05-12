# Implement Prompt

Implementation is blocked until `review_design_ergonomics` returns an accepting verdict. Do not start implementation from RFC 0034 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/036/DESIGN_SYNTHESIS.md` and the resolved ergonomics_dx review findings. Stay inside the workflow write scope.

Expected behavior:

**RFC 0034 V1 slice:**

- generator core: `WorkflowGenerationSpec` value object + `GeneratedWorkflow` envelope + public Python API `generate_workflow(spec) -> GeneratedWorkflow`
- built-in shapes: `minimal`, `review`, `code_change`, `human_checkpoint`, `evidence_backed`, `multi_review_synthesis`, `custom`
- built-in lane sets: `local`, `single_agent`, `author_reviewer`, `multi_review`, `custom`
- lane modifiers: `supervised`, `worktree_isolated`, `constrained`, `harness_profiled` with full compatibility matrix and field-specific errors for `forbidden` cells
- catalog package-data tree (`src/striatum/workflow_templates/...`) with shape + lane-set entries; cached loader at startup, never fetched remotely
- CLI surface: `striatum workflow templates list [--kind] [--json]`, `striatum workflow templates show <id> [--json]`, `striatum workflow generate <path> --shape <s> --lane-set <l> --artifact-root <p> [--lane-modifier <m>]... [--plan <p>] [--dry-run] [--json]`
- local service API: `GET /workflow-templates`, `GET /workflow-templates/<id>`, `POST /workflows/generate/preview` (non-mutating), `POST /workflows/generate` (requires `confirm_write: true`, behind `--allow-mutations`)
- custom-plan compiler with closed block vocabulary (`draft | review | synthesis | implementation | test | human_checkpoint | support_ledger | evidence_audit | final_review`) and refusal cases (each with `field_path`)
- immediate validation: every generated `workflow.json` passes `workflow validate` before the generator returns success; bug in the generator must not become an invalid starter file
- refuse-to-overwrite on non-dry-run writes; explicit `--force` deferred to a future RFC
- `workflow init --style minimal|review|code-change` rewired to dispatch through `generate_workflow` while preserving backwards-compatible output shape where practical
- documentation updates to SPEC, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, WORKFLOW_TYPES, WRITING_WORKFLOWS, HOW_TO_HUMAN, RFC 0034 status, CHANGELOG, README

Do NOT:

- author the web `/workflows/new` chooser UI (RFC 0034 §9 deferred to a follow-up dogfood);
- author the chat-assisted scaffolding tool (RFC 0034 §10 deferred to a follow-up dogfood);
- ship target-repo local catalog extensions (RFC 0034 §6 V1.5 deferred);
- add automatic repository inspection for suggested shapes (deferred indefinitely);
- introduce a hosted marketplace, remote template fetch, or telemetry (out of scope by design boundary);
- add devils_advocate / threat_model / security review jobs to this dogfood's workflow (this dogfood's posture is ergonomics_dx; deeper postures are not in scope here);
- retire `workflow init --style` (it stays as backwards-compatible sugar over the generator).

**Test coverage requirements:**

- per-shape compiler tests (every built-in shape × every compatible lane set validates)
- lane-modifier compatibility matrix tests (every cell covered)
- catalog-loader tests (well-formed entries, missing-required-field refusal)
- custom-plan compiler tests (each refusal case: unknown block kind, unbounded cycle, edge from/to nonexistent block, review block with no posture, invalid lane bindings, unsafe paths, `.striatum/` write attempts)
- `workflow init --style` backwards-compat tests
- CLI `--dry-run` writes-nothing tests
- CLI refuse-to-overwrite tests
- local API preview endpoint non-mutating tests
- local API write endpoint requires `confirm_write: true` tests
- field-path error envelope tests

## Maximize sub-agent usage where it helps

Per the harness profile, native sub-agent delegation is **encouraged**. Use it aggressively for the parts of this implementation that are independent enough to parallelize.

Spawn sub-agents in parallel for work that meets all of these:

- the sub-task can be specified by a self-contained brief (file paths, expected behavior, test fixtures, ~1 page of context);
- it does not depend on the in-flight output of another sub-agent;
- you (the parent session) can independently verify its output.

Good candidates in this implementation:

- one sub-agent per shape compiler (`minimal`, `review`, `code_change`, `human_checkpoint`, `evidence_backed`, `multi_review_synthesis`);
- one sub-agent per lane-set compiler (`local`, `single_agent`, `author_reviewer`, `multi_review`);
- one sub-agent for the lane-modifier compatibility matrix + per-cell field-specific errors;
- one sub-agent for the custom-plan compiler with closed block vocabulary;
- one sub-agent for the catalog package-data tree + loader;
- one sub-agent for the CLI route additions (`templates list`, `templates show`, `generate`);
- one sub-agent for the local service route additions (preview + write);
- one sub-agent for `workflow init --style` rewiring;
- one sub-agent per major test file (per-shape, per-lane-set, modifier-matrix, custom-plan, CLI, local API);
- one sub-agent per doc surface (SPEC, CLI_REFERENCE, WORKFLOW_TYPES, WRITING_WORKFLOWS, UBIQUITOUS_LANGUAGE, HOW_TO_HUMAN, RFC 0034 status, CHANGELOG, README);
- exploratory sub-agents to read existing modules (`workflow.py`, `cli/workflow.py`, `scaffold/templates/`, existing local service surface) and produce one-page summaries of integration points.

Do NOT delegate (parent session owns these):

- the BUILD_HANDOFF.md authorship;
- the integration step where sub-agents' outputs are reconciled;
- any `make lint`/`typecheck`/`test`/`smoke` invocation;
- final commit-shape and scope discipline.

## Verification

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke` after all changes are in place.

## Handoff

Produce `docs/dogfood/036/BUILD_HANDOFF.md` summarizing changes, new modules, tests added/passing, deferred web UI + chat tool with pointers to the follow-up dogfood, and any human-decision items the ergonomics_dx review did not pre-resolve. If sub-agents were used, briefly note which sub-tasks were delegated.

The byline must be `author: implementer-codex-gpt-5.5-001` (or whatever the work packet supplies) — plain Markdown line, lowercase `author:`, no decoration.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
