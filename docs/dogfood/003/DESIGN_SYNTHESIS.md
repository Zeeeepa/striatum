---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0010-tool-harness-profiles.md", "docs/research/0010-tool-harness-profiles/claude_code.md", "docs/research/0010-tool-harness-profiles/codex.md", "docs/research/0010-tool-harness-profiles/gemini_cli.md", "docs/dogfood/003/research/codex/TOOL_RESEARCH.md", "docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md", "docs/dogfood/003/research/gemini/TOOL_RESEARCH.md", "docs/dogfood/003/workflow.json", "src/striatum/workflow.py", "src/striatum/db.py"]
---

# RFC 0010 Tool Harness Profiles — V1 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Inputs:

- `docs/rfcs/0010-tool-harness-profiles.md`
- `docs/research/0010-tool-harness-profiles/{claude_code,codex,gemini_cli}.md`
- `docs/dogfood/003/research/{codex,claude_code,gemini}/TOOL_RESEARCH.md`
- `docs/dogfood/003/workflow.json` (in-tree harness_profiles fixture)
- Current `src/striatum/workflow.py` and `src/striatum/db.py` (validate +
  build_packet)

This synthesis defines the smallest reviewed slice of RFC 0010 that can be
implemented in one follow-up job, the surface it exposes to workflows and
work packets, the validation behavior, the fixture set, the test plan, and
the explicit deferrals.

## V1 Schema Shape

### Top-level `harness_profiles`

Add an **optional** top-level field on workflows. Workflows without this
field continue to behave exactly as before; nothing is required to opt
in.

```json
{
  "schema_version": "striatum.workflow.v1",
  "harness_profiles": {
    "<profile_id>": { /* profile body */ }
  }
}
```

`harness_profiles` is **not** added to `REQUIRED_TOP_LEVEL` in
`src/striatum/workflow.py`. It is a new optional key validated only when
present.

### Profile body

V1 profile body fields:

| Field | Type | V1 disposition |
|---|---|---|
| `tool_family` | string | required when profile is declared |
| `strategy_version` | string | required when profile is declared |
| `native_delegation` | object | optional |
| `feature_flags` | object | optional |
| `accountability` | object | optional |
| `supervision` | object | optional |
| `workspace_isolation` | object | optional |
| `agent_loop_budget` | object | optional |
| `approval_mode` | string | optional |
| `output_format` | string | optional |
| `memory_files` | array of string | optional |
| `mcp_servers` | array of object | optional |
| `turn_caps` | object | optional |
| `prompt_envelope_path` | string | optional |
| `fallback_profile_id` | string | optional |

V1 validation behavior:

- `tool_family` must be one of `generic`, `codex`, `claude_code`,
  `gemini_cli` (closed set in V1; documented as extensible). Unknown
  families produce a `WorkflowError`.
- `strategy_version` must be a non-empty string.
- `accountability.native_subagents`, when set, must be the literal
  string `"internal_to_parent_session"`. Any other value is a
  `WorkflowError`. (V1 cannot accept `"first_class_session"`; that
  would require a separate decision and code path.)
- `accountability.first_class_registration`, when set, must be
  `"not_supported"` in V1.
- `fallback_profile_id`, when set, must reference a profile declared
  in the same workflow.
- **Unknown sibling fields under `harness_profiles.<id>` are
  accepted as lint warnings, not errors, in V1.** This matches the
  RFC 0010 open question marked "lint-warning rollout recommended."
  The validator emits a warning to stderr (or a `warnings` field on
  `workflow plan --json`) but does not refuse the workflow.

### Lane reference

Add an optional `harness_profile_id` field on each lane:

```json
{
  "lanes": {
    "codex": {
      "adapter": "process",
      "command": ["codex", "exec", "..."],
      "harness_profile_id": "codex_default"
    }
  }
}
```

V1 validation behavior:

- `harness_profile_id`, when set, must reference a profile declared in
  `harness_profiles`. A reference to an undeclared profile produces a
  `WorkflowError("lane <id> references undeclared harness profile
  <profile_id>")`.
- Lanes without `harness_profile_id` produce work packets with no
  `harness_profile` block (i.e., identical to today).

## How Lanes Reference Profiles

Profiles are declared once at workflow scope and referenced by id from
each lane. There is no separate profile file format in V1 — RFC 0010's
"reusable profile files referenced by path" question is deferred. The
schema reserves the lane field name `harness_profile_id` and the
top-level field name `harness_profiles`; future RFCs can add a
`harness_profiles_path` style reference without breaking V1 fixtures.

## Work-Packet Exposure

When a job's lane references a declared profile, `build_packet` in
`src/striatum/db.py` adds a compact `harness_profile` block to the work
packet:

```json
{
  "harness_profile": {
    "profile_id": "codex_default",
    "tool_family": "codex",
    "strategy_version": "2026-05-08",
    "native_delegation": { /* full object as declared */ },
    "feature_flags": { /* full object as declared */ },
    "accountability": { /* full object as declared */ },
    "supervision": { /* if declared */ },
    "approval_mode": "default",
    "output_format": "json",
    "memory_files": ["AGENTS.md"]
  }
}
```

The block is a **compact projection** of the declared profile; it
includes:

- `profile_id` (the key under `harness_profiles`).
- `tool_family`, `strategy_version`, `native_delegation`,
  `feature_flags`, `accountability` (all five if declared).
- Any of the extended fields (`supervision`, `workspace_isolation`,
  `agent_loop_budget`, `approval_mode`, `output_format`, `memory_files`,
  `mcp_servers`, `turn_caps`) that are declared.
- Unknown sibling fields are passed through verbatim so dogfood
  artifacts can capture them; the lint warning at validation time is
  the gate.

When the lane has no `harness_profile_id`, the packet has no
`harness_profile` key at all (not `null`, absent — preserves current
packet shape for existing workflows).

## Validation Behavior And Backwards Compatibility

- `validate_workflow` accepts workflows that omit `harness_profiles`
  entirely (no change for existing workflows).
- `validate_workflow` accepts workflows that declare
  `harness_profiles` and reference profiles correctly.
- `validate_workflow` rejects:
  - profile body that is not a JSON object;
  - profile body missing `tool_family` or `strategy_version`;
  - profile body with `tool_family` outside the V1 closed set;
  - profile body with `accountability.native_subagents !=
    internal_to_parent_session`;
  - profile body with `accountability.first_class_registration !=
    not_supported`;
  - lane `harness_profile_id` referencing an undeclared profile;
  - profile `fallback_profile_id` referencing an undeclared profile.
- `validate_workflow` warns (not errors) on:
  - unknown sibling fields in the profile body;
  - unknown sibling fields in the profile's `feature_flags`;
  - `supervision.compatible == "verify_pipe_behavior_first"` when the
    referencing lane is used in a `striatum supervise start` flow
    (deferred for V1.5; V1 emits the warning at validate time).
- `build_packet` does not re-validate; it trusts whatever
  `validate_workflow` accepted.

## Fixture Profiles

The first build slice ships at least three fixture profiles. These can
live either in the dogfood-003 workflow (already present) or under
`examples/harness-profiles/`. **Recommendation: ship as a single
`examples/harness-profiles/` workflow fixture** so tests can import the
declared profiles without depending on dogfood-003 staying in tree.

Concretely, V1 ships:

- `examples/harness-profiles/workflow.json` — a minimal workflow with
  three lanes (`codex`, `claude_code`, `generic`) referencing the three
  fixture profiles, plus one trivial job per lane that publishes a
  fixture artifact. The workflow is exclusively a profile-validation
  fixture, not a full dogfood flow.
- `examples/harness-profiles/profiles.json` (optional) — a JSON file
  documenting the profile bodies separately from the workflow, for
  reuse in tests.

Profile bodies in the V1 fixture (sourced verbatim from RFC 0010 with
the extended fields the prior research notes confirmed):

- `generic_default` — `tool_family: "generic"`, `native_delegation.mode:
  "off"`, all feature flags `off`/`not_supported`, `supervision.compatible:
  true`, `supervision.wrapper_required: false`. Used as the safe default
  when no provider-specific profile fits.
- `codex_default` — full body from RFC 0010 §"codex_default", including
  `workspace_isolation.state_dir_per_job: true`,
  `agent_loop_budget.max_iterations: 8`, `output_format: "json"`,
  `memory_files: ["AGENTS.md"]`.
- `claude_code_default` — full body from RFC 0010 §"claude_code_default",
  including `supervision.wrapper_required: true`,
  `feature_flags.headless_print_mode: "forbidden_for_supervised_lanes"`,
  `memory_files: ["CLAUDE.md", "AGENTS.md"]`.

Gemini's `gemini_cli_default` is **not required for V1** because the
acceptance criteria in RFC 0010 only call for "generic, Codex, and
Claude Code" fixtures. Gemini's profile is left in dogfood-003's
fixture as the fourth advisory profile but does not block the build
slice. V2 promotes it to `examples/harness-profiles/`.

## Explicit Deferrals (Not In V1)

These are decisions the build slice **must not implement**, even though
they are within RFC 0010's larger scope:

- **Provider wrappers.** No `.striatum/bin/claude-supervised-wrapper.sh`
  ships; the file remains a documented requirement in RFC 0010 and a
  high-severity harness friction artifact. The supervisor flow keeps
  failing fast on missing-binary today; V1 adds a workflow-validate
  *lint warning* for missing repo-relative lane command files but no
  hard error.
- **Remote services / hosted coordination.** No A2A subagent support,
  no MCP server registration, no telemetry. Profile fields document
  intent only.
- **Transcript parsing.** Adapter-level transcripts remain
  "off"-by-default (D028); profiles do not change that.
- **Native tool worktree ownership.** RFC 0008 remains authoritative
  for worktrees. Profile feature flag `native_worktree: forbidden`
  documents the boundary; V1 does not enforce by inspecting lane
  commands.
- **First-class native subagent registration.** Profiles enforce
  `accountability.native_subagents == "internal_to_parent_session"`;
  any future change requires a separate decision.
- **Strict (non-lint) validation of unknown profile fields.** V1 ships
  lint-warning behavior; V2 can graduate to errors after dogfood
  evidence accumulates.
- **Profile reference by file path.** V1 keeps profiles inline in
  workflow JSON. Future schema can add `harness_profiles_path` /
  per-profile-file references without breaking V1 fixtures.

## Proposed Updates To Other Docs

- **RFC 0010** — transition from `proposed` to `accepted` once the
  human acceptance decision is recorded. Add a "V1 Implementation"
  section that points to the build slice and the fixture path. Move
  the relevant Open Questions to "Resolved in V1" or "Deferred to V2".
- **`docs/SPEC.md`** — under "Workflow Authoring", add a short
  "Harness profiles (optional)" section that documents the schema
  shape, the lane reference field, and the work-packet block. Cross-
  reference RFC 0010.
- **`docs/UBIQUITOUS_LANGUAGE.md`** — define `harness profile`,
  `tool family`, `native delegation`, `harness improvement proposal`.
- **`README.md`** — section "Workflows" already mentions RFC 0010 as
  a fixture; update to point at `examples/harness-profiles/` as the
  reference fixture and note that profiles are advisory in V1.
- **`CHANGELOG.md`** — add an entry: "RFC 0010 V1 lands: optional
  `harness_profiles` workflow field, lane `harness_profile_id`
  reference, work-packet `harness_profile` block. Generic, Codex, and
  Claude Code fixture profiles."

## Build Slice (One Follow-Up Job)

The implementation job should make these changes in a single PR-shaped
diff:

1. **`src/striatum/workflow.py`**
   - Add `HARNESS_PROFILE_TOOL_FAMILIES = {"generic", "codex",
     "claude_code", "gemini_cli"}`.
   - Add `_validate_harness_profiles(workflow)` invoked from
     `validate_workflow` after `_validate_lane_constraints`.
   - Extend `_validate_lane_constraints` to validate `harness_profile_id`
     against the declared profile set.
   - Lint-warning collection: introduce a small `WorkflowWarnings` or
     similar accumulator, returned alongside `validate_workflow` from a
     new `validate_workflow_with_warnings(workflow) -> tuple[None, list[str]]`
     (or attached to the workflow plan output, whichever fits the
     existing CLI shape with the least churn). The plan-output route is
     simpler.

2. **`src/striatum/db.py`**
   - In `build_packet`, after computing `lane_config`, project the
     harness profile (if `harness_profile_id` is set) into a compact
     `harness_profile` block and add to the packet.
   - No DB schema change required (workflows are stored verbatim in
     `workflow_snapshots.workflow_json`).

3. **`src/striatum/cli/introspect.py` and CLI**
   - `workflow validate --json` includes a `warnings` array when
     present (existing `--json` envelope just gains a sibling key).
   - `workflow plan --json` likewise.

4. **`examples/harness-profiles/`**
   - New `workflow.json` with three lanes, three profiles, three
     trivial jobs.
   - Optional `README.md` explaining the fixture.

5. **`docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `README.md`,
   `CHANGELOG.md`** — updates as listed above.

6. **Tests**
   - `tests/test_harness_profiles.py` (new file):
     - profile validation: minimal valid profile, missing
       `tool_family`, missing `strategy_version`, unknown
       `tool_family`.
     - accountability strictness: rejects `native_subagents !=
       internal_to_parent_session`.
     - lane reference validation: undeclared profile raises
       `WorkflowError`; declared profile passes.
     - lint warnings: unknown sibling fields produce warnings, not
       errors; warnings surface in plan output.
     - work-packet exposure: lane with profile produces packet with
       `harness_profile` block; lane without profile produces packet
       with no `harness_profile` key.
     - fixture loading: `examples/harness-profiles/workflow.json`
       validates and plans cleanly.
   - `tests/test_cli_mvp.py` (extend):
     - `claim-next` against a profile-using lane returns the packet
       with the `harness_profile` block; against a non-profile lane,
       returns packet without the key (backwards compat).

7. **No changes** to:
   - SQLite schema (workflow JSON is already stored verbatim).
   - Adapter/process module (profiles are not adapter contracts).
   - Supervisor/RFC 0009 module (profiles are advisory packet content).
   - RFC 0008 worktree code (profiles do not own filesystem isolation).

## Generic, Codex, Claude Code, Gemini Fixture Test Coverage

The V1 fixture under `examples/harness-profiles/workflow.json` should
cover:

- **`generic_default`**: minimum profile, all feature flags `off` /
  `not_supported`. Asserts that `supervision.wrapper_required: false`
  and the profile validates with no warnings.
- **`codex_default`**: tests the `workspace_isolation` and
  `agent_loop_budget` extension fields; asserts the resulting work
  packet exposes them verbatim.
- **`claude_code_default`**: tests `supervision.wrapper_required:
  true` and `feature_flags.headless_print_mode:
  "forbidden_for_supervised_lanes"`. Asserts that workflow validation
  emits a lint warning if (and only if) the lane command starts with
  a path inside the repo and that path is missing.

Gemini's profile remains in the dogfood-003 fixture as advisory
content; V1 build slice does not test it. V2 ships the fixture under
`examples/harness-profiles/`.

## Test Plan Summary

| Scenario | Test file | Expected |
|---|---|---|
| Workflow with no `harness_profiles` | existing tests | unchanged behavior |
| Workflow with valid profiles + lane refs | new | validates, packet has block |
| Profile missing `tool_family` | new | `WorkflowError` |
| Profile with unknown `tool_family` | new | `WorkflowError` |
| Profile with bad `accountability` | new | `WorkflowError` |
| Lane references undeclared profile | new | `WorkflowError` |
| Profile has unknown sibling field | new | warning, not error |
| Packet for lane with profile | new | `harness_profile` block matches |
| Packet for lane without profile | new | no `harness_profile` key |
| Fixture workflow validates and plans | new | green |

## Deferred Items And Open Questions

Carry forward to V2 / future RFCs:

- Strict (non-lint) profile validation rollout once dogfood evidence
  accumulates.
- Profile reference by file path.
- Workflow-validate enforcement of `supervision.compatible !=
  "verify_pipe_behavior_first"` for supervised lanes (currently a
  warning).
- Workflow-validate enforcement of `feature_flags.native_worktree:
  forbidden` by inspecting lane commands.
- Doctor check that scans `~/.gemini/agents/*.md` and
  `~/.codex/agents/*.toml` for remote/A2A subagents.
- A `harness_profile_id` override at job level (currently lane-only).
- First-class registration of native sub-agents as Striatum sessions
  (needs its own RFC).
- The proposed `actual_runner` packet field surfaced by all three
  research handoffs (separate RFC; not RFC 0010 scope).
- Standard `claude-supervised-wrapper.sh` script (separate RFC 0009 /
  RFC 0010 follow-up).

## Acceptance Decision Gate

Per the dogfood-003 SKILL.md and `prompts/implement_profiles.md`, the
implementation job must block until a human acceptance decision is
recorded under `docs/dogfood/003/decisions/`. This synthesis explicitly
**does not** authorize implementation; it produces the design that the
review and human acceptance gate will evaluate.

If accepted, the build slice as scoped above is the next claim.
