# GH #16 — Prompt: add complete operator initialization prompt

Source: <https://github.com/halbritt/striatum/issues/16> (filed 2026-05-14).
Captured here verbatim so the runner's `context.docs` is self-contained
and reviewers don't need GH API access mid-run.

---

## Problem

The repo has a reusable operator boundary prompt at `prompts/OPERATOR_BOUNDARY_PROMPT.md`, but it is not a complete prompt for initializing a fresh Striatum operator session.

The existing prompt is useful as a guardrail: the operator drives the Striatum control plane and does not perform designer/implementer/reviewer/synthesizer work inline. But it is not enough to instantiate an operator. A fresh operator session needs a full boot prompt that establishes mission, inputs, environment, workflow state, operating rules, recovery rules, reporting expectations, and how to begin.

## Current state

- `prompts/README.md` marks `OPERATOR_BOUNDARY_PROMPT.md` as reusable.
- `OPERATOR_BOUNDARY_PROMPT.md` says to paste it into an operator session when the session must drive control-plane commands only.
- The prompt does not collect or render run-specific context such as target repo, workflow path, branch, run id, daemon/Postgres state, Striatum version, active capabilities, or expected operator report path.
- The prompt is a boundary guard, not a complete operator initialization prompt.
- The prompt also contains stale/narrow wording such as RFC0026/RFC0027-specific examples and direct SQLite references.

## Expected outcome

Add a current, reusable, complete operator initialization prompt, likely `prompts/OPERATOR_INITIALIZATION_PROMPT.md`, that can be copied into a fresh AI operator session before driving a Striatum run.

This prompt should be self-contained enough that a capable AI session can become the Striatum operator for a specific run without reading historical dogfood prompts first.

It should preserve the existing operator-boundary rules while adding the full initialization frame and first-action sequence.

## Required shape

The prompt should include a fill-in block for:

- Striatum repo path.
- Striatum version / command path.
- Target repository path.
- Workflow path.
- Intended branch / branch-confirmation policy.
- Existing run id if resuming, otherwise whether to prepare/start a new run.
- Daemon/Postgres state and whether direct mode is allowed for this run.
- Required docs to read first.
- Expected artifact root.
- Operator report path and update cadence.
- Whether the operator may use MCP/chat tools, CLI only, or both.
- Whether native sub-agents may be used for operator-side read-only audits.
- Any current blockers, known open GitHub issues, or deferred work to preserve.
- Commit/push policy.

## Required behavior

The prompt should explicitly instruct the operator to:

- Read the project instructions and canonical docs before acting.
- Check `git status --short --branch` before state-changing work.
- Confirm the Striatum command path/version.
- Inspect current run/workflow state when resuming.
- Validate the workflow before preparing or starting a run.
- Prepare/start/register/claim/ack/publish/complete only through Striatum commands or approved MCP/chat tools.
- Keep role work in role sessions; the operator may coordinate but must not author role artifacts.
- Use `status`, `why`, `doctor`, `dashboard`, `run summary`, and documented recovery/checkpoint commands for failures.
- Update the operator report incrementally, especially before compaction or handoff.
- Preserve unrelated user changes and never edit `.striatum/` or the state substrate directly.
- Stop for explicit human decision only when the workflow reaches a human checkpoint or the prompt says a decision is required.

## Required first-action sequence

The initialized operator should have a clear first sequence, for example:

1. Load the project instructions and listed canonical docs.
2. Check repository state and Striatum version.
3. Inspect daemon/Postgres or direct-mode status as specified by the filled-in block.
4. Validate the workflow.
5. Inspect or create the run.
6. Start or resume execution.
7. Record/update `OPERATOR_REPORT.md`.
8. Continue driving the workflow until blocked or complete.

## Definition of done

- `prompts/OPERATOR_INITIALIZATION_PROMPT.md` exists and is marked `Status: reusable`.
- It is a complete initialization prompt, not merely a short boundary warning.
- `prompts/README.md` lists it separately from `OPERATOR_BOUNDARY_PROMPT.md` and explains when to use each.
- The old boundary prompt is either kept as a focused guardrail or refactored so the new initialization prompt reuses/points to it.
- The prompt is generic and does not hardcode RFC0026/RFC0027 or Engram-specific paths.
- The prompt reflects the current daemon/Postgres transition accurately, including any RFC0048 caveat where needed.
- A fresh operator session can use it to start or resume a run without reading historical dogfood prompts first.
