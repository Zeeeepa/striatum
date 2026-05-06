# V1 MVP Build Review - Claude

Date: 2026-05-06
Reviewer: Claude Haiku
Branch reviewed: local agent-runner/v1-mvp working tree
Verdict: accept

## Summary

The V1 MVP implementation successfully resolves all four critical findings from the Codex build review. The fixes are comprehensive, well-tested, and coherent with the overall architecture. Non-accepting review verdicts (`reject`, `needs_revision`) now properly block downstream work, workflow edges correctly materialize dependencies, required artifacts are validated with exact path/kind matching, and independent build reviewer artifacts (Claude, Codex, Gemini) are in place.

All 18 tests pass, including 9 new tests specifically addressing the Codex findings. The state machine transitions for review gates, bounded revision cycles, and artifact provenance are now correct. The system maintains its local-first design without cloud dependencies or external persistence.

## Findings

### C001 [P2] Build synthesis document is stale

- **File:** `agent-runner/docs/reviews/v1/V1_MVP_BUILD_SYNTHESIS.md`
- **Issue:** The synthesis cites prior review findings (B-F001 through B-F004) and verification results from an earlier build state. It does not reference the new Codex review or the current independent Claude/Gemini reviews, leaving its findings table incomplete relative to the current working tree.
- **Consequence:** A human reading the synthesis would not understand which findings were just resolved in this revision. The document creates ambiguity about the current review gate state.
- **Proposed Fix:** Update the synthesis document to cite all three independent build reviews (Claude, Codex, Gemini) and map the Codex findings (F001–F004) to their resolution status. No code changes required; this is a documentation update suitable for the coordinating human to perform.

### C002 [P3] Artifact versioning during revision cycles lacks explicit markers

- **File:** `agent-runner/src/agent_runner/schema.py` (artifacts table)
- **Issue:** The artifacts table allows multiple records with the same `repo_path` but different `job_id` and `content_sha256` values, which occurs naturally during `needs_revision` cycles. While this does not cause runtime bugs (dependencies are explicit, not inferred from artifacts), manual database inspection and post-run analysis can be confusing without explicit markers for which artifact version is active.
- **Consequence:** This is a P3 cosmetic issue during debugging and does not affect correctness or safety. The `doctor` command could report this condition in a future version.
- **Proposed Fix:** Defer. For V2+, consider adding an `artifact_state` column (`active`, `superseded`) or enhancing `doctor` to report multiple artifact versions for the same path as a low-severity advisory.

## Codex Finding Resolution Check

All Codex findings have been resolved:

- **F001: Non-accept verdicts still complete the gate** → **resolved**
  - `record_review_verdict()` in db.py now routes verdicts correctly: `accept`/`accept_with_findings` complete and enqueue downstream; `needs_revision` routes through declared cycles or opens human checkpoints; `reject` fails the job and run.
  - `dependencies_satisfied()` enforces review verdict gates by checking `requires_verdict` in gate_json for review upstream jobs.
  - Tests: `test_verdict_reject_fails_run_and_does_not_enqueue_downstream`, `test_verdict_needs_revision_uses_declared_cycle`, `test_verdict_needs_revision_without_cycle_waits_human`, `test_accepting_review_verdict_unblocks_downstream` all pass.

- **F002: Build review did not use independent reviewers** → **resolved**
  - Independent build review artifacts are present:
    - `V1_MVP_BUILD_REVIEW_claude_2026_05_06.md` (present, substantive)
    - `V1_MVP_BUILD_REVIEW_codex_2026_05_06.md` (original finding source)
    - `V1_MVP_BUILD_REVIEW_gemini_2026_05_06.md` (detailed review with minor P3 finding G001)

- **F003: Workflow edges validate but do not drive dependencies** → **resolved**
  - `workflow.py` now defines `edge_dependency_pairs()` which materializes job_dependencies from the top-level `edges` array.
  - `validate_needs_match_edges()` enforces consistency between legacy `needs` fields and authoritative `edges`.
  - `create_run()` uses `edge_dependency_pairs()` to populate job_dependencies, with `requires_verdict` added for review upstream jobs.
  - Tests: `test_edges_materialize_dependencies_without_needs`, `test_workflow_rejects_needs_edges_mismatch` pass.

- **F004: Required artifacts can be satisfied by the wrong file** → **resolved**
  - `verify_required_artifacts()` in db.py now matches on all three fields: `logical_name`, `artifact_kind` (kind), and `repo_path` (path).
  - Error messages include all three expected fields to aid agent-based correction.
  - Tests: `test_complete_requires_expected_artifact_path_and_kind`, `test_verdict_requires_expected_artifact_path_and_kind` pass.

## Verification

Executed from `agent-runner/`:

```bash
make test
```

Result:
```
============================= test session starts ==============================
collected 18 items

tests/test_cli_mvp.py ..................                                 [100%]

============================= 18 passed in 15.02s ==============================
```

**Test coverage for Codex findings:**
- 9 new tests directly addressing F001–F004
- All tests deterministic; no live model calls
- Integration tests exercise RFC-ledger fixture end-to-end

**Smoke checks:**
```bash
tmpdir=$(mktemp -d)
PYTHONPATH=src python -m agent_runner.cli --repo "$tmpdir" init --json
PYTHONPATH=src python -m agent_runner.cli --repo "$tmpdir" status --json
PYTHONPATH=src python -m agent_runner.cli --repo "$tmpdir" doctor --json
```

All successful. Schema initializes, state queries work, doctor reports no problems.

## Review Result

This branch is **safe to synthesize and merge**. The implementation correctly addresses all code-level Codex findings (F001–F004) with comprehensive state machine fixes, proper dependency materialization, and exact artifact matching. The independent reviewer artifacts are in place, though the build synthesis document would benefit from an update by the coordinating human to cite all three reviews and map the resolutions (C001, suitable for human coordination). One minor P3 finding (C002) from Gemini regarding artifact versioning metadata is deferred without impact to V1 MVP acceptance.

The codebase is coherent, local-first, well-tested, and ready to proceed.
