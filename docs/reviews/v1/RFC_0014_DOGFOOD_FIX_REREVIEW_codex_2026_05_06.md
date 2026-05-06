# RFC 0014 Dogfood Fix Re-Review - Codex

Date: 2026-05-06
Reviewer: Codex GPT-5.5
Branch reviewed: `agent-runner/rfc-0014-validation`
Commit reviewed: `e293817`
Verdict: needs_revision

## Scope

Re-reviewed the RFC 0014 dogfood fix implementation after
`agent-runner/docs/reviews/v1/RFC_0014_DOGFOOD_FIX_REVIEW_codex_2026_05_06.md`
recorded two findings:

- F001: `submit-review` can leave an immutable bad artifact behind.
- F002: `declared_cycle` policy is accepted but not enforced.

Those two findings appear addressed in the current branch:

- `submit-review` now prevalidates the review job and required artifact tuple
  before publishing.
- `declared_cycle` now validates that root review jobs have matching
  `needs_revision` cycles.

Verification run during re-review:

```bash
git diff --check origin/master...HEAD
cd agent-runner
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
```

Results:

- whitespace check passed;
- `28 passed in 15.28s`;
- RFC 0014 workflow validation returned `valid: true`.

## Findings

### F003 [P1] Evidence export is not actually redacted

File: `agent-runner/src/agent_runner/cli.py:925-932`

`evidence_export` labels the output as redacted, but it serializes
`status_payload` and `snapshot` directly. Those include free-text blocker
`description` values and verdict `rationale` values from agent or user input,
which can contain private corpus excerpts, local paths, or model output.

Since this command is intended to create commit-ready evidence artifacts,
default export should omit or redact free-text fields unless explicitly
requested.

## Review Result

The original review findings are resolved, but F003 is blocking for the default
evidence export because it can leak unredacted free text into committed
artifacts.
