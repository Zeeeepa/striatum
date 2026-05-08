---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["dogfood-003", "rfc-0010"]
---

# RFC 0010 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Target: `docs/dogfood/003/DESIGN_SYNTHESIS.md`
Read: target synthesis, RFC 0010, the three refreshed research handoffs
under `docs/dogfood/003/research/`. No other repository files inspected
beyond what the artifact_augmented scope permits.

Verdict intent: **accept_with_findings**.

The synthesis is implementation-ready. The build slice is sized
appropriately, the schema is conservatively additive, and the deferrals
are explicit. The findings below are improvement-grade, not blockers; a
human acceptance decision can reasonably proceed on this design.

## Assessment Against Review Criteria

### Generic product boundary preserved

**Yes.** No provider-specific code paths in core scheduling. Profiles
are passthrough configuration; the validator enforces a closed
`tool_family` set but does not branch on it. Lane commands remain the
only place provider names appear in execution paths, which is unchanged
behavior.

### Native subagents stay internal to parent sessions

**Yes, hard-enforced.** Validation rejects
`accountability.native_subagents != "internal_to_parent_session"` and
`accountability.first_class_registration != "not_supported"` as
`WorkflowError`s. D021 is preserved.

### Validation and packet exposure are appropriately small

**Yes, with one caveat (F2).** The validator changes are local to
`workflow.py` and add roughly one new validation function plus an
extension to `_validate_lane_constraints`. The packet change is local
to `build_packet` in `db.py`. No DB schema changes. No supervisor or
adapter changes. The RFC 0008/0009 boundaries are not crossed.

The caveat: the lint-warning system is new API surface. See F2.

### Accounts for RFC 0010 concrete profiles and extended fields

**Yes.** The schema accepts all extended fields surfaced by the
2026-05-08 research (`supervision`, `workspace_isolation`,
`agent_loop_budget`, `approval_mode`, `output_format`, `memory_files`,
`mcp_servers`, `turn_caps`) without overfitting any of them to one
provider. Treating them as optional and unknown-field-tolerant is the
correct V1 posture.

### Backed by current docs and refreshed research

**Yes.** The synthesis cites the three refreshed handoffs and the
RFC 0010 sections directly. The handoffs themselves are dated
2026-05-08 (today) and confirm the RFC's concrete profile examples
without drift.

### Fixture profiles cover the four tools while keeping defaults portable

**Partially.** RFC 0010 acceptance criteria call for generic, codex,
and claude_code fixtures. The synthesis ships those three under
`examples/harness-profiles/` and leaves `gemini_cli_default` in the
dogfood-003 fixture only. That meets the letter of the criterion.
However, the dogfood-003 workflow already exercises Gemini's profile
(it claims four lanes), so the test plan should at least *load* the
dogfood-003 fixture to confirm the four-profile shape validates
cleanly. See F4.

### Tests and docs that must change before acceptance

The synthesis lists them. Validating against my review:

- New `tests/test_harness_profiles.py` — covers the criteria.
- Extension to `tests/test_cli_mvp.py` for packet exposure — good.
- `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `README.md`,
  `CHANGELOG.md` — all listed.
- RFC 0010 status transition to `accepted` — listed.

Missing from the list:
- `docs/DECISION_LOG.md` should record the RFC 0010 acceptance as a
  D-numbered decision (consistent with how D052/D053/D054 recorded
  RFCs 0003/0004/0005). See F3.
- `docs/TODO.md` should reflect the build slice as a tracked item
  with the appropriate status (F3 also).

## Findings

### F1 (medium) — `harness_improvement_proposal` artifact gap

**Issue.** RFC 0010 acceptance criteria require: "At least one dogfood
run produces or reviews a `harness_improvement_proposal` that targets
one of `prompt`, `workflow`, `defaults`, or `documentation`."

The three research handoffs surface concrete frictions (missing Claude
supervised wrapper script, lane execution mode mismatch, SKILL.md
hardcoded macOS path), but none have been authored as
`harness_improvement_proposal` artifacts in this run. The synthesis
references them implicitly but does not require their publication.

**Recommendation.** Before run completion, the operator should publish
at least one `harness_improvement_proposal`. The natural target is the
missing wrapper script (target: `defaults`) — high signal, scoped
narrowly to `.striatum/bin/`, and directly cited in three artifacts.
This can happen during the implementation job (since the implementer
will encounter the missing wrapper concretely) or as a standalone
publish from any active session. Suggested severity: high.

This finding does not block design acceptance; it ensures the run
itself satisfies the RFC's acceptance criterion before evidence export.

### F2 (medium) — Lint-warning system adds API surface

**Issue.** The synthesis introduces a workflow-level warnings
accumulator (`validate_workflow_with_warnings`, surfaced via
`workflow validate --json` and `workflow plan --json`). This is a new
public CLI surface that didn't exist before V1. While the synthesis
correctly notes that RFC 0010's open question marked "lint-warning
rollout recommended," the build slice could be smaller if V1 simply
silently accepts unknown profile fields and defers the warnings system
to V1.5 / V2.

**Trade-off.**
- *Ship lint-warnings now*: meets RFC 0010's recommendation directly,
  gives operators feedback on profile typos.
- *Defer lint-warnings*: smaller build slice, fewer test cases, less
  CLI surface, but provides no signal when a profile field is
  misspelled.

**Recommendation.** Keep lint-warnings in V1, but constrain them to a
single warning type for V1: "unknown sibling field in profile body".
Defer the supervised-lane warning (`verify_pipe_behavior_first`) and
the missing-lane-command-path warning to V1.5. This collapses the
warnings system to one rule, which is implementable with a small
helper rather than a generic accumulator. The CLI envelope still
gains a `warnings: []` field, but V1 only ever populates it from the
unknown-field check.

### F3 (low) — Doc updates list missing two entries

**Issue.** The synthesis's "Proposed Updates To Other Docs" section
lists RFC 0010, SPEC, UBIQUITOUS_LANGUAGE, README, and CHANGELOG, but
does not list `docs/DECISION_LOG.md` or `docs/TODO.md`. Prior RFC
acceptance commits (D052/D053/D054 for RFCs 0003/0004/0005) recorded
the acceptance as a numbered decision and updated the TODO snapshot.

**Recommendation.** Add both to the implementer's checklist.
DECISION_LOG should get a `Dxxx accepted` row referencing RFC 0010 and
the build slice; TODO should get a status update on whichever F-number
tracks RFC 0010.

### F4 (low) — Dogfood-003 fixture should be load-tested

**Issue.** The synthesis recommends the new fixture lives under
`examples/harness-profiles/` and that the test suite validates it.
The dogfood-003 workflow already declares four profiles inline.
Without a test that loads dogfood-003's workflow.json, a future
schema change could break it silently.

**Recommendation.** Add a test that loads
`docs/dogfood/003/workflow.json` (or the snapshot in
`examples/harness-profiles/`) and asserts that all four declared
profiles validate. This is one extra test case in
`tests/test_harness_profiles.py`.

### F5 (low) — "Compact projection" naming is misleading

**Issue.** The synthesis calls the work-packet `harness_profile`
block a "compact projection" of the profile. In practice the
projection includes every declared field (the five required-when-
declared fields plus any of the eight extended fields the profile
sets). That's effectively a passthrough, not a compaction.

**Recommendation.** Either rename to "passthrough projection" /
"profile view" in the SPEC and synthesis, or actually compact by
omitting `prompt_envelope_path`, `fallback_profile_id`, and any
fields that are pure validator metadata. Recommend the rename — the
passthrough semantics are correct; it's the name that's wrong.

### F6 (low) — `prompt_envelope_path` validation unspecified

**Issue.** The schema lists `prompt_envelope_path` as optional but the
validation rule is unstated. Should the path:

- Be repo-relative (no leading `/`, no `..`)?
- Exist on disk at validate time?
- End in `.md`?

Today's behavior (silently accept any string) is acceptable for V1
but should be made explicit either in the synthesis or in the
implementer's notes.

**Recommendation.** Document the V1 rule as: "if set, must be a
non-empty string with no leading `/` and no `..` segments; existence
is not checked at validate time." That matches how
`expected_artifacts.path` is handled today.

### F7 (info) — Job-level `harness_profile_id` override deferred

**Issue.** The synthesis correctly notes that V1 places
`harness_profile_id` on lanes only, with job-level override deferred.
This is fine for V1; flagging here so operators do not rely on a
job-level field in fixtures.

**Recommendation.** SPEC and UBIQUITOUS_LANGUAGE should explicitly say
"profiles are referenced at lane level; job-level overrides are
reserved for a future RFC." No code change needed; just doc clarity.

## Acceptance Recommendation

**accept_with_findings.** The design is implementation-ready. F1
(harness_improvement_proposal gap) should be addressed during the
run, not before acceptance — the implementer or operator can
publish the proposal alongside the build. F2 narrows lint-warning
scope; the implementer should adopt the narrowed scope. F3–F7 are
documentation or minor naming refinements to capture in the build
slice.

A human can reasonably record an acceptance decision against this
synthesis and proceed to implementation.
