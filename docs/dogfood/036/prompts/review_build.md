# Review Build Prompt (ergonomics_dx posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0034", "workflow-generator", "build"]
---
```

Review the implementation under the **ergonomics_dx** posture. Verify behavior, tests, docs, and workflow compatibility. Actually try the new surfaces from the first-time-operator perspective; inspect the repository within the review write scope policy (repo-level access).

ergonomics_dx posture (per RFC 0018 and the Striatum workflow module): "This is a developer-ergonomics review. Evaluate the artifact's surface from a first-time-user perspective; verdict acceptance means the affordances are discoverable and consistent."

**The web `/workflows/new` chooser UI and the chat-assisted scaffolding tool are EXPLICITLY DEFERRED** to a follow-up dogfood. **Do not refuse the build for their absence** as long as:

- the generator core, catalog, CLI surface, local API endpoints, and custom-plan compiler are shipped;
- the deferral is clearly documented in `docs/dogfood/036/BUILD_HANDOFF.md` with a pointer to the follow-up dogfood;
- existing functionality is not regressed.

Required checks (try each surface):

- **`striatum workflow templates list [--json]`**: does it work? Does the human-readable output show useful catalog metadata? Does `--json` produce a structured payload? Does `--kind shape|lane_set` filter correctly?
- **`striatum workflow templates show <id> [--json]`**: does it print a useful detail view? Does `--json` produce a structured payload?
- **`striatum workflow generate <path> --shape <s> --lane-set <l> --artifact-root <p> --dry-run`**: does dry-run write nothing? Is the printed envelope readable? Does `--json` produce a parseable envelope?
- **Non-dry-run `striatum workflow generate <path> ...`**: does it write `workflow.json`, roles, and prompts atomically enough for local use? Does it revalidate the written `workflow.json`? Does it refuse to overwrite existing paths?
- **Required-flag error quality**: running `striatum workflow generate <path>` with missing flags should fail loudly with `field_path`-bearing errors, not crash or silently default.
- **Custom-plan compiler errors**: a plan with `shape: "custom"` and an unbounded cycle should refuse with a `field_path` pointing at the cycle entry; an unknown block kind should refuse with `field_path` pointing at the offending block.
- **Symmetric envelope**: Python API + CLI `--json` + local API preview/write must return the same `GeneratedWorkflow` envelope. Verify by inspecting test fixtures + an actual CLI `--json` invocation.
- **`workflow init --style minimal|review|code-change` backwards-compat**: does the legacy verb still work? Does it dispatch through the generator? Is the output shape preserved enough that existing users see no regression?
- **Refuse-to-overwrite**: a non-dry-run generate against an existing path should refuse with a clear error.
- **Validation-on-return**: a generator bug should never return a `GeneratedWorkflow` whose `workflow.json` fails `workflow validate`. Spot-check the generator code path for the validation call before return.
- **Catalog metadata quality**: open the catalog files. Are `summary` and `recommended_for` specific and actionable, or boilerplate?
- **Help text quality**: `striatum workflow --help`, `striatum workflow templates --help`, `striatum workflow generate --help` should each give a usable example invocation, not just flag listings.
- **Documentation honesty**: SPEC, CLI_REFERENCE, WORKFLOW_TYPES, WRITING_WORKFLOWS, UBIQUITOUS_LANGUAGE, HOW_TO_HUMAN, RFC 0034 status reflect actual shipped behavior. No claims about web UI / chat tool / hosted marketplace / repository inspection (all deferred).
- **Tests cover happy paths and failure modes**: every built-in shape × every compatible lane set validates, every modifier × lane-set cell is decided, every custom-plan refusal case is tested.
- **Write scopes and fixtures do not normalize** generation that bypasses validation or writes directly to `.striatum/`.

Use `needs_revision` for: behavior gaps in the shipped scope, missing tests for the ergonomic surfaces above, `field_path` errors absent on refusal cases, validation-on-return missing, `workflow init --style` regression, or documentation that overstates the V1 shipped scope. Use `accept_with_findings` for non-blocking cleanup or follow-up dogfood scope.

Stay inside the review write scope (`docs/dogfood/036/review/build/ergonomics/`). Do not modify the implementation. Do not call striatum CLI; the operator publishes otherwise.
