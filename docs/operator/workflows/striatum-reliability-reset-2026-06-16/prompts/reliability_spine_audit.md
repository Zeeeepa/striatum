# Reliability Spine Audit

Audit Striatum's core reliability spine: daemon state transitions, run driving,
leases, artifacts, branch/worktree anchoring, completion gates, and test
coverage for those paths.

Use the required context docs plus source inspection. Run the prompt-level
commands from `PROMPT.md` when available, but do not block the audit if the
daemon or local environment is not ready.

Write `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/reliability_spine/REVIEW.md`.
Include the exact author line from your work packet.

Required sections:

- Verdict: `accept`, `accept_with_findings`, `needs_revision`, or `reject`.
- Top reliability failure modes.
- Evidence table: claim, evidence path/command, confidence.
- Gaps in tests or conformance fixtures.
- P0 fixes with exact file/test targets.
- What to freeze until this is fixed.

