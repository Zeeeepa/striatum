# V1 MVP Build Review - Gemini

Date: 2026-05-06
Reviewer: Gemini Pro
Branch reviewed: local agent-runner/v1-mvp working tree
Verdict: accept_with_findings

## Summary

The V1 MVP build has a solid local-first architecture and correctly implements the core workflow orchestration logic. The fixes for the prior Codex review are comprehensive and well-tested, particularly around review-gate semantics (`reject`, `needs_revision`), dependency materialization from workflow edges, and exact artifact matching. The system now correctly handles non-accepting verdicts, preventing workflows from falsely completing and correctly routing revision cycles.

The codebase demonstrates good practices for a local-first CLI tool, including transactional state management in SQLite, clear separation of concerns, and safe defaults for write scopes and lease handling. The test suite is thorough and covers the critical state transitions added in the latest revision.

The branch is in a strong state for the V1 MVP. One minor finding is raised regarding artifact versioning metadata, but it does not impact runtime safety or correctness and does not block acceptance.

## Findings

### G001 [P3] Multiple artifact versions for the same path can accumulate

- **Affected File/Section:** `agent-runner/src/agent_runner/schema.py` (artifacts table), `agent-runner/src/agent_runner/db.py`
- **Issue:** The current design allows for multiple artifact records to point to the same file path (`repo_path`). This occurs naturally during revision cycles (`needs_revision`), where a new job attempt creates a new version of an artifact, resulting in a new row in the `artifacts` table with a new `job_id` and `content_sha256`.
- **Consequence:** This is not a runtime bug, as the data flow between jobs is explicitly declared in the workflow, not dynamically discovered from the artifacts table. However, it leads to an accumulation of artifact metadata for the same file path. This can make manual database inspection, post-run analysis, or debugging more complex, as there is no explicit marker for which artifact version is the "latest" or "active" one for a given path. The `doctor` command does not report on this condition.
- **Proposed Fix:** For a future version, consider adding a `state` column to the `artifacts` table (e.g., `active`, `superseded`) to explicitly manage the lifecycle of artifact versions. When a new version of an artifact at a specific path is published, the previous one(s) could be marked as `superseded`. Alternatively, the `doctor` command could be enhanced to report multiple artifact versions for the same path as a low-severity warning. This finding is minor and does not need to be addressed for the V1 MVP.

## Codex Finding Resolution Check

The fixes for the prior Codex review (`V1_MVP_BUILD_REVIEW_codex_2026_05_06.md`) have been implemented and verified.

- **F001: Non-accept verdicts still complete the gate:** **resolved**. The `db.py` and `cli.py` logic was updated to correctly handle `reject`, `needs_revision`, and `accept*` verdicts. `reject` now fails the run, `needs_revision` correctly follows declared cycles or creates a human checkpoint, and only accepting verdicts unblock downstream jobs. This is confirmed by new tests.
- **F002: Build review did not use independent reviewers:** **resolved**. This was a process issue. The current review by Gemini Pro is part of the corrected process requiring independent reviewers.
- **F003: Workflow edges validate but do not drive dependencies:** **resolved**. `workflow.py` has been updated to make the top-level `edges` array the authoritative source for materializing job dependencies, with validation to ensure consistency if the legacy `needs` key is present. This is confirmed by new tests.
- **F004: Required artifacts can be satisfied by the wrong file:** **resolved**. The `verify_required_artifacts` function in `db.py` now correctly matches on `logical_name`, `artifact_kind`, and `repo_path`, ensuring that jobs can only be completed when the exact declared artifacts have been published. This is confirmed by new tests.

## Verification

Tests were executed from the `agent-runner/` directory. All 18 tests passed, including the new tests covering the Codex review findings.

```bash
$ make test
.venv/bin/python -m pytest
============================= test session starts ==============================
platform linux -- Python 3.12.3, pytest-9.0.3, pluggy-1.6.0
rootdir: /home/halbritt/git/engram/agent-runner
configfile: pyproject.toml
testpaths: tests
collected 18 items

tests/test_cli_mvp.py ..................                                 [100%]

============================= 18 passed in 14.26s ==============================
```

## Review Result

This branch is safe to synthesize and merge. The implemented fixes are correct and robust, and the overall quality of the V1 MVP is high. The one minor finding is suitable for deferral.
