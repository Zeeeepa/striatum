---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0010 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: run_0e6a74ae8feb481cbc18a4b1435552b6
Decision artifact:
`docs/dogfood/003/decisions/RFC_0010_ACCEPTANCE.md`
(decision_id `dec_6abd3957ab1748949ff0967221b346c4`).

## Scope landed

The reviewed V1 slice of RFC 0010 from `DESIGN_SYNTHESIS.md`, narrowed
per design-review F2 to a single lint-warning rule.

## Files changed

- `src/striatum/workflow.py`
  - New constants: `HARNESS_PROFILE_TOOL_FAMILIES`,
    `_HARNESS_PROFILE_REQUIRED_FIELDS`, `_HARNESS_PROFILE_KNOWN_FIELDS`.
  - New function: `_validate_harness_profiles(workflow, *, warnings)`
    returning the declared profile id set; raises on hard schema
    violations, accumulates lint warnings on unknown profile fields.
  - `_validate_lane_constraints` extended with optional
    `harness_profile_ids` parameter; rejects `harness_profile_id`
    references that are empty strings or undeclared profile ids.
  - `validate_workflow` extended with optional
    `warnings: list[str] | None` keyword and now invokes
    `_validate_harness_profiles` then `_validate_lane_constraints` with
    the declared id set.
  - `plan_workflow` allocates a warnings list and surfaces it under a
    `warnings` key on the plan result when non-empty.
- `src/striatum/db.py`
  - `build_packet` now appends a `harness_profile` block to the work
    packet when the lane has a `harness_profile_id` reference.
  - New helper `_harness_profile_view(workflow, *, lane_id)` performs
    the passthrough projection (per design-review F5 naming).
- `src/striatum/cli/dispatch.py`
  - `workflow validate` collects warnings and exposes them under the
    `warnings` key of the JSON envelope when present.
  - Imports `validate_workflow` from `striatum.workflow`.
- `examples/harness-profiles/workflow.json` (new)
  - Reference fixture with three lanes (`generic`, `codex`,
    `claude_code`) and three profiles
    (`generic_default`, `codex_default`, `claude_code_default`).
- `examples/harness-profiles/roles/author.md` (new) — placeholder role.
- `examples/harness-profiles/prompts/demo.md` (new) — placeholder prompt.
- `tests/test_harness_profiles.py` (new) — 19 tests covering validation,
  lint warnings, packet exposure (referenced lane and lane without
  reference), CLI `workflow validate --json` envelope, and fixture
  loading for both `examples/harness-profiles/workflow.json` and
  `docs/dogfood/003/workflow.json` (per F4).
- `docs/SPEC.md` — new "Harness Profiles (RFC 0010 V1)" section under
  Workflow Config.
- `docs/UBIQUITOUS_LANGUAGE.md` — added `harness profile`,
  `tool family`, `native delegation`, `harness improvement proposal`.
- `README.md` — new "Harness Profiles (RFC 0010 V1)" section.
- `CHANGELOG.md` — Unreleased entry under Added.
- `docs/rfcs/0010-tool-harness-profiles.md` — status to
  `accepted (V1)`, "V1 Implementation Slice" section added.
- `docs/DECISION_LOG.md` — D056 row recording RFC 0010 acceptance.
- `docs/TODO.md` — F4 row marking RFC 0010 V1 done.

## What is intentionally out of scope (V1)

These remain deferred per the synthesis and the acceptance follow-up:

- Provider wrappers (`.striatum/bin/claude-supervised-wrapper.sh`).
  Captured separately as
  `docs/dogfood/003/findings/HARNESS-001.md`
  (target `defaults`, severity proposed).
- Strict (non-lint) rejection of unknown profile fields.
- Profile reference by file path.
- Workflow-validate enforcement of `supervision.compatible !=
  "verify_pipe_behavior_first"` for supervised lanes.
- Workflow-validate enforcement of `feature_flags.native_worktree:
  forbidden` by inspecting lane commands.
- Doctor checks scanning `~/.gemini/agents/*.md` and
  `~/.codex/agents/*.toml` for remote/A2A subagents.
- Job-level `harness_profile_id` overrides.
- First-class registration of native sub-agents as Striatum sessions.

## Tests run

- `make test` (full suite): **178 passed** in 127.35s. Up from 159
  before this slice (+19 new tests).
- `tests/test_harness_profiles.py`: 19 passed in 1.69s.
- `make lint`: clean (`ruff check .`).
- `make typecheck`: clean (`mypy`).

## Validation against design-review findings

| Finding | Status | Notes |
|---|---|---|
| F1 (harness_improvement_proposal) | done | `docs/dogfood/003/findings/HARNESS-001.md` published as artifact `art_985f067502ed4c0aad428e5c569ab67e`. |
| F2 (narrow lint scope) | done | V1 has exactly one lint rule: unknown sibling fields on profile bodies. Supervised-lane and missing-lane-command lints deferred to V1.5. |
| F3 (DECISION_LOG + TODO updates) | done | D056 added; F4 row added. |
| F4 (load dogfood-003 fixture in tests) | done | `test_dogfood_003_fixture_validates_with_four_profiles`. |
| F5 (rename "compact" to "passthrough") | done | SPEC, db.py docstring, and decision body all use "passthrough projection". |
| F6 (`prompt_envelope_path` validation rule) | done | Validates non-empty, no leading `/`, no `..`. SPEC documents the rule. |
| F7 (lane-only profile reference) | done | SPEC explicitly says "Profiles are referenced at lane level only; job-level overrides are reserved for a future RFC." |

## How to verify in this checkout

```bash
make test
.venv/bin/striatum --repo . workflow validate examples/harness-profiles/workflow.json --json
.venv/bin/striatum --repo . workflow plan examples/harness-profiles/workflow.json --json
```

`workflow validate` returns `{"valid": true, ...}` with no warnings on
the reference fixture. Adding an unknown sibling field to a profile body
in a copy of the fixture would surface a `warnings` array in both
`validate` and `plan` output without rejecting the workflow.

## Deferred work

Captured in `harness_improvement_proposal` artifacts and TODO/RFC notes,
not in this slice:

1. **Claude supervised wrapper** — see HARNESS-001.
2. **V1.5 lint warnings** — supervised-lane `verify_pipe_behavior_first`
   warning and missing-lane-command-path warning.
3. **V2 strict validation rollout** — graduate unknown-field warnings
   to errors after dogfood evidence accumulates.
