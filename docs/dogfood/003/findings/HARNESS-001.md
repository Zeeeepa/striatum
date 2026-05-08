---
schema_version: "striatum.harness_improvement_proposal.v1"
artifact_kind: "harness_improvement_proposal"
target: "defaults"
expected_benefit: "Ship the missing .striatum/bin/claude-supervised-wrapper.sh script (or fail-fast workflow lint) so workflows that reference Claude Code as a supervised lane stop validating green when the lane command file is absent."
risk: "Authoring a wrapper that buffers stream-json packets badly under named-pipe stdin could create silent supervision failures. Mitigated by lint-warning the missing path before promoting to a hard validation error."
rollback: "Drop the wrapper or revert the workflow lint; the schema and packet block continue to function without it."
---

# HARNESS-001 — Missing Claude Code supervised wrapper script

author: implementer-codex-gpt-5.5-001

Status: proposed
Run: dogfood-003
Reporter: implementer-codex-gpt-5.5-001
Surface: defaults

## Observed friction

`docs/rfcs/0010-tool-harness-profiles.md` (concrete `claude_code_default`
profile) and `docs/dogfood/003/workflow.json` both reference
`.striatum/bin/claude-supervised-wrapper.sh` as the lane command for
Claude Code supervised use. The script does not exist in this checkout:

```text
$ ls -la .striatum/bin/
ls: cannot access '.striatum/bin/': No such file or directory
```

`striatum workflow validate docs/dogfood/003/workflow.json` exits zero
because the validator does not check that a lane's `command[0]` exists
when it looks like a path inside the repo. Anyone who attempts
`striatum supervise start --session-id <claude-session>` against this
workflow fails at exec time, not at validate time.

This was flagged independently in all three dogfood-003 research
handoffs and surfaced again as design-review F1.

## Supporting runner evidence

- run_id: run_0e6a74ae8feb481cbc18a4b1435552b6
- job_id: job_run_0e6a74ae8feb481cbc18a4b1435552b6_implement_profiles
- packet_id: (this implementer packet)
- supervisor_id: none — the dogfood-003 RUNBOOK explicitly tells
  operators not to start the supervisor for the Claude lane while the
  wrapper is missing
- relevant cross-reference:
  - `docs/dogfood/003/research/codex/TOOL_RESEARCH.md` (friction #2)
  - `docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md` (largest
    finding)
  - `docs/dogfood/003/research/gemini/TOOL_RESEARCH.md` (friction #1)
  - `docs/dogfood/003/review/design/DESIGN_REVIEW.md` (F1)

## Proposed change

Two-step landing, decoupled from RFC 0010 V1 itself:

1. **V1.5 workflow-validate lint warning.** Extend `_validate_lane_constraints`
   so that when `lane.command[0]` looks like a repo-relative path (no
   leading `/`, no shell built-in match) and the file does not exist on
   disk, validation emits a lint warning. Surfaced in
   `workflow validate --json` and `workflow plan --json` under
   `warnings`. Hard error in V2.

2. **V2 standard wrapper script.** Ship `.striatum/bin/claude-supervised-wrapper.sh`
   that reads newline-delimited JSON packets from stdin and feeds each
   as a fresh user turn into a single long-lived `claude` session.
   Pinned to whichever `claude` invocation Anthropic's CLI provides for
   long-lived stdin streaming (today's candidate is `claude --print
   --output-format stream-json --input-format stream-json`, but pipe
   buffering under `os.mkfifo` must be verified before pinning).

The two steps are independent. The lint warning gives operators a fast
fail; the wrapper gives them a working supervised lane.

## Risk

- Picking a `claude` invocation that buffers per-block instead of
  per-line under named-pipe stdin would cause supervised packets to
  arrive in the wrong shape. The wrapper must verify pipe behavior
  before any claim auto-delivery test passes.
- A workflow-validate lint that checks `command[0]` existence may
  produce false warnings for lanes that legitimately reference a tool
  that is on `$PATH` but not a repo-relative path. The check should
  only warn when the first arg starts with `.` or `/` and is not
  absolute outside the repo.

## Rollback

- The lint warning can be disabled by reverting
  `_validate_lane_constraints`. No state migration needed.
- The wrapper script can be deleted; the lane configuration falls
  back to today's "operator is responsible" semantics.

## Notes

Authored as part of dogfood-003 to satisfy RFC 0010 acceptance
criterion: "At least one dogfood run produces or reviews a
`harness_improvement_proposal` that targets one of `prompt`,
`workflow`, `defaults`, or `documentation` for a tool profile."
