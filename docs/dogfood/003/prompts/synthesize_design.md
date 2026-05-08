# Synthesize Tool Harness Profile Design

Read RFC 0010, the prior research notes, the three dogfood-003 refreshed
research artifacts, the workflow fixture's `harness_profiles` map, and current
Striatum workflow validation/work-packet code. Produce a design synthesis that
can drive review and a small implementation.

Use native subagents for independent source inspection if available, especially
for:

- checking workflow validation patterns;
- checking work-packet construction patterns;
- comparing the three tool research artifacts;
- identifying tests that should change.

Do not let subagents write overlapping files. The parent session owns the
final synthesis artifact.

Write `docs/dogfood/003/DESIGN_SYNTHESIS.md` with:

- recommended V1 `harness_profiles` schema shape, including how to handle
  RFC 0010's extended fields and whether unknown fields are errors or lint
  warnings for the first rollout;
- how lanes reference profiles;
- exact work-packet exposure;
- validation behavior and backwards compatibility;
- how the generic, Codex, Claude Code, and Gemini CLI fixture profiles should
  be represented in tests or dogfood docs;
- explicit deferral of provider wrappers, remote services, transcript parsing,
  and first-class native subagent registration;
- proposed updates to RFC 0010, SPEC, README, and glossary;
- a small build slice that can be implemented in one follow-up job;
- deferred items and open questions.

Publish the synthesis artifact and complete the job.
