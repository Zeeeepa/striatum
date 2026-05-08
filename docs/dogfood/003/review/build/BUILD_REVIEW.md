---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["dogfood-003", "rfc-0010"]
---

# RFC 0010 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Run: run_0e6a74ae8feb481cbc18a4b1435552b6
Inputs read (fresh context, repo-level access):

- `docs/dogfood/003/BUILD_HANDOFF.md`
- `docs/dogfood/003/DESIGN_SYNTHESIS.md`
- `docs/dogfood/003/review/design/DESIGN_REVIEW.md`
- `docs/dogfood/003/decisions/RFC_0010_ACCEPTANCE.md`
- `docs/dogfood/003/findings/HARNESS-001.md`
- `src/striatum/workflow.py` (full file, focus on validation + plan)
- `src/striatum/db.py` (`build_packet` and `_harness_profile_view`)
- `src/striatum/cli/dispatch.py` (`workflow validate` branch)
- `tests/test_harness_profiles.py` (full)
- `examples/harness-profiles/workflow.json`
- `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `README.md`,
  `CHANGELOG.md`, `docs/rfcs/0010-tool-harness-profiles.md`,
  `docs/DECISION_LOG.md`, `docs/TODO.md`

Verdict intent: **accept**.

The implementation matches the accepted design slice and the design-
review follow-ups. Validation and packet exposure are correct, the
fixture loads cleanly, and backwards compatibility is preserved by
construction. Findings below are informational; nothing blocks the run.

## Schema validation correctness

- `_validate_harness_profiles` in `src/striatum/workflow.py` correctly
  rejects:
  - non-object `harness_profiles` map (`"must be an object"`);
  - non-string or empty profile id keys;
  - non-object profile body;
  - missing `tool_family` and `strategy_version`;
  - `tool_family` outside the closed set `{generic, codex,
    claude_code, gemini_cli}`;
  - empty `strategy_version`;
  - `accountability.native_subagents` other than
    `internal_to_parent_session`;
  - `accountability.first_class_registration` other than
    `not_supported`;
  - non-string or absolute `prompt_envelope_path`, and paths
    containing `..`;
  - empty `fallback_profile_id`, or `fallback_profile_id` referencing
    an undeclared profile.
- `_validate_lane_constraints` correctly rejects empty
  `harness_profile_id` and references to undeclared profiles.
- Tests in `tests/test_harness_profiles.py` cover all of the above
  individually. Passing locally: 19/19.

Malformed-reference coverage: the test
`test_lane_reference_to_undeclared_profile_raises` exercises the
positive case (lane points at a missing profile id). The test
`test_lane_with_empty_profile_reference_raises` exercises the
empty-string case. No regression in the existing test suite (178
total tests pass).

## Work-packet shape and backwards compatibility

`_harness_profile_view` in `src/striatum/db.py` constructs a passthrough
projection: `{"profile_id": <id>}` plus the entire profile body
verbatim. The function is defensive — if any link in the chain
(`lane_id` → `lanes` map → lane object → `harness_profile_id` → profile
map → profile body) is malformed it returns `None` and the packet
omits the `harness_profile` key. That defensive shape is correct given
the input has already passed `validate_workflow`; the redundant checks
exist because `build_packet` reads the snapshotted `workflow_json` and
should not assume validation ran.

Backwards compatibility:

- Workflows without `harness_profiles` produce packets without a
  `harness_profile` key. Verified by
  `test_packet_omits_harness_profile_when_lane_has_no_reference`.
- Existing `tests/test_cli_mvp.py` (75 tests, including packet shape
  assertions) passes unchanged. No existing fixture under `examples/`
  declares `harness_profiles`; existing workflows are not perturbed.
- The DB schema is unchanged. Workflow JSON is stored verbatim in
  `workflow_snapshots.workflow_json`, and the projection happens at
  packet-build time only.

## Provider-specific behavior stays in profiles

Verified:

- No code path branches on `tool_family` or `profile_id` in the
  scheduler, supervisor, adapter, or worktree code.
- The validator enforces a closed `tool_family` set but does not
  branch on it; rejection is structural, not provider-specific.
- The packet projection is a passthrough — the runner does not
  interpret `feature_flags`, `supervision`, or any other declared
  field. Profile content is configuration for the agent, not for
  Striatum.

## Native subagent guidance remains advisory and internal

Verified:

- `accountability.native_subagents = "internal_to_parent_session"` is
  the only accepted value in V1; any other rejected at validate time.
- `accountability.first_class_registration = "not_supported"` is the
  only accepted value in V1.
- `feature_flags.subagents` (and similar) are passthrough strings; the
  runner does not register native subagents as Striatum sessions.
- D021 is preserved by construction.

## Tests for malformed references and no-profile workflows

Adequately covered in `tests/test_harness_profiles.py`:

- `test_workflow_without_harness_profiles_validates` — no-profile
  workflows still validate.
- `test_lane_reference_to_undeclared_profile_raises` — undeclared
  reference rejected.
- `test_lane_with_empty_profile_reference_raises` — empty ref
  rejected.
- `test_packet_omits_harness_profile_when_lane_has_no_reference` —
  no-profile lane produces no packet block.
- Plus 15 other validation and surface-level tests.

## Docs accuracy and generic language

- `docs/SPEC.md` — new "Harness Profiles (RFC 0010 V1)" section is
  accurate. Wording is generic ("native sub-agents", "tool family")
  and avoids privileging any provider. The example block in SPEC uses
  a Codex profile but documents it as illustrative, not canonical.
- `docs/UBIQUITOUS_LANGUAGE.md` — four new entries (`harness profile`,
  `tool family`, `native delegation`, `harness improvement proposal`)
  are correctly defined in generic terms.
- `README.md` — new section reads correctly and points operators at
  `examples/harness-profiles/workflow.json`.
- `CHANGELOG.md` — Unreleased entry is accurate and well-scoped.
- `docs/rfcs/0010-tool-harness-profiles.md` — status updated to
  `accepted (V1)` with a clear "V1 Implementation Slice" section that
  enumerates what landed and what is deferred.
- `docs/DECISION_LOG.md` — D056 row records the acceptance with full
  context (decision, rationale, evidence, "revisit" triggers).
- `docs/TODO.md` — F4 row marks RFC 0010 V1 done. Consistent with
  prior RFC tracking convention.
- All seven design-review follow-ups (F1–F7) are addressed in
  `BUILD_HANDOFF.md` with traceable evidence.

## Findings

### F1 (info) — `_harness_profile_view` is duplicated in spirit

**Issue.** The synthesis explicitly named a `harness_profile_packet_view`
helper for `workflow.py`. The implementer instead placed the helper in
`db.py` as `_harness_profile_view` (private, used only by
`build_packet`). This is a defensible call — it keeps the helper next
to its only caller and avoids a second module import — but the
synthesis's location is no longer matched by the code.

**Recommendation.** None required. Note the deviation in the next RFC
0010 update or in operator-facing docs if the helper becomes public
(e.g., needed by a future `workflow plan` view of profiles). For now
the private placement is fine.

### F2 (info) — Profile keys must be non-empty strings

**Issue.** `_validate_harness_profiles` checks both `isinstance(profile_id,
str)` and non-empty, then iterates a `dict` — but the `dict.items()`
loop guarantees keys are already strings (Python's JSON loader
produces `str` keys for object keys). The non-empty check is the load-
bearing one; the `isinstance` check is defensive but technically
redundant after `json.loads`.

**Recommendation.** None required. Defensive checks at validation
boundaries are good hygiene; matches the rest of the file's style.

### F3 (info) — Test naming convention

**Issue.** The test file uses `_run_cli`/`_data` helpers (leading
underscore) while `tests/test_cli_mvp.py` and
`tests/test_harness_v2_fixes.py` use `run_cli`/`data` (no underscore).
This is a minor stylistic inconsistency.

**Recommendation.** None blocking. If the test helpers ever migrate to
a shared `tests/_helpers.py` (or similar), the names will be
normalised then.

## Verdict

**accept.** The build slice is correct, well-tested, and matches the
accepted design plus all seven design-review follow-ups. Findings F1–F3
are informational only.
