# V1 MVP Build Review Synthesis

Date: 2026-05-06
Status: accepted with deferred P3 follow-up

## Inputs

Independent build reviews:

- `docs/reviews/v1/V1_MVP_BUILD_REVIEW_claude_2026_05_06.md`
- `docs/reviews/v1/V1_MVP_BUILD_REVIEW_gemini_2026_05_06.md`
- `docs/reviews/v1/V1_MVP_BUILD_REREVIEW_codex_2026_05_06.md`

Earlier review/fix artifacts:

- `docs/reviews/v1/V1_MVP_BUILD_REVIEW.md`
- `docs/reviews/v1/V1_MVP_BUILD_REVIEW_codex_2026_05_06.md`
- `docs/reviews/v1/V1_MVP_CODEX_BUILD_REVIEW_FIX_SPEC_2026_05_06.md`

Implementation and verification surfaces:

- `src/agent_runner/`
- `examples/rfc-ledger-cleanup/`
- `tests/test_cli_mvp.py`

## Review Verdicts

| Review | Verdict | Synthesis |
|--------|---------|-----------|
| Initial parent/coordinator review | `accept_with_findings` | Historical artifact. Useful for early fixes, but superseded by the independent review pass required by revised P001. |
| Initial Codex review | `reject_for_revision` | Accepted. Produced F001-F004 and drove the fix spec. Superseded by the fresh Codex re-review after fixes. |
| Claude review | `accept` | Accepted. Confirms F001-F004 are resolved. Raises C001 stale synthesis, resolved by this document, and C002 artifact-version metadata, deferred as P3. |
| Gemini review | `accept_with_findings` | Accepted. Confirms F001-F004 are resolved. Raises G001 artifact-version metadata, deferred as P3. |
| Codex re-review | `accept_with_findings` | Accepted. Confirms F001-F004 are resolved and that the independent review gate now has Claude, Gemini, and Codex coverage. |

## Accepted Findings And Fixes

| Finding | Disposition | Fix |
|---------|-------------|-----|
| F001: Non-accept verdicts complete the gate | accepted, fixed | `verdict_work` now routes through `record_review_verdict`: accepting verdicts complete and enqueue downstream work; `needs_revision` follows a declared cycle or opens a human checkpoint; `reject` fails the job and run. |
| F002: Build review lacked independent reviewers | accepted, fixed | Independent Claude and Gemini reviews were added, and this fresh Codex re-review supersedes the initial rejecting Codex review for the final gate decision. |
| F003: Workflow edges validate but do not drive dependencies | accepted, fixed | `create_run` materializes `job_dependencies` from top-level `edges`. `validate_needs_match_edges` rejects disagreement between legacy `needs` and authoritative `edges`. |
| F004: Required artifacts can be satisfied by the wrong file | accepted, fixed | `verify_required_artifacts` now matches required artifacts by `logical_name`, `artifact_kind`, and exact repo-relative `repo_path`. |
| C001: Build synthesis was stale | accepted, fixed | This synthesis replaces the stale 9-test synthesis and cites the current independent review set. |
| C002/G001: Multiple artifact versions for one path lack active/superseded metadata | accepted, deferred | Not a V1 correctness blocker because dependencies are explicit and artifact verification is job-scoped. Carry forward as V2/doctor enhancement. |

## Verification After Fixes

Executed from `agent-runner/`:

```bash
PYTHONPATH=src /Users/halbritt/Documents/GitHub/engram/.venv/bin/python -m pytest -q
```

Result:

```text
18 passed in 9.16s
```

Smoke sequence in a temporary repo:

```text
agent_runner init --json
agent_runner status --json
agent_runner doctor --json
```

Result: all smoke commands returned `ok: true`; `doctor` reported schema
version `1` and no problems.

## Deferred Gaps

- Process/tmux launch implementation.
- CLI `--request-id` idempotency surface over `command_requests`.
- Actual `git switch` behavior in `branch confirm`.
- Richer `doctor` checks for artifact hashes, branch drift, and artifact
  version lifecycle.
- Explicit active/superseded artifact metadata for repeated repo paths during
  revision cycles.

These are deferred because the P001 state-machine MVP is now represented,
reviewed, and tested without expanding into provider-specific launch behavior,
automatic git automation, or dashboard surfaces.

## Final Review Result

No unresolved blocking build review findings remain. The V1 MVP branch has
passed the independent Claude, Gemini, and Codex review gate and is ready for
final human review.
