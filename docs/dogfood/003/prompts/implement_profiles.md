# Implement Accepted Tool Harness Profile Slice

Before editing code, verify that a human acceptance decision exists for the
design, preferably under `docs/dogfood/003/decisions/`. If it does not exist,
call `striatum block --severity human_checkpoint` and explain that the
implementation needs a recorded acceptance decision.

If accepted, implement the smallest reviewed slice for RFC 0010:

- workflow validation accepts a `harness_profiles` map;
- lanes may declare `harness_profile_id` only when the profile exists;
- work packets include a compact `harness_profile` block for lanes with a
  profile;
- workflows without profiles behave exactly as before;
- profile fixtures exist for at least generic, Codex, and Claude Code
  profiles, either from the dogfood-003 workflow fixture or a separate
  examples/dogfood fixture path;
- tests cover validation, malformed lane references, work-packet exposure,
  fixture loading, and backwards compatibility;
- docs mention the new workflow field, lane reference, packet block, and the
  rule that native subagents remain internal to the parent session.

Do not implement provider-specific wrappers, transcript parsing, hosted
coordination, remote A2A subagents, native tool worktree ownership, or
first-class native subagent registration in this slice. Those remain follow-up
decisions unless the accepted design explicitly narrowed them into scope.

Use native subagents for independent codebase inspection or test planning if
available, but keep final edits in the parent session and stay inside the
write scope.

Write `docs/dogfood/003/BUILD_HANDOFF.md` with changed files, tests run, and
any deferred work. Publish it as the required handoff artifact and complete
the job.
