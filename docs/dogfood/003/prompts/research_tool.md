# Research Tool Harness Behavior

Read the work packet first. Include the packet's exact `author:` line near the
top of the artifact you publish.

Research the tool family named in the job objective. Start from
`docs/dogfood/003/SOURCES.md`, RFC 0010's concrete profile candidate, and the
existing `docs/research/0010-tool-harness-profiles/` note for your tool.
Verify the relevant official docs are still current and call out any drift
from the RFC text.

Use this generic delegation instruction when your tool supports native
subagents or equivalent workers:

> Spawn the maximum number of useful agents for independent research subtasks.
> Use them for separable source-reading, feature inventory, risk analysis, or
> comparison work. Do not delegate overlapping artifact writes. Return only
> synthesized findings to the parent session, and keep the parent session
> accountable for the final Striatum artifact.

Write one research report to the expected artifact path. Include:

- source list with official URLs;
- feature inventory relevant to Striatum harness profiles;
- confirmation or correction of the RFC 0010 concrete profile for this tool;
- coverage of the RFC 0010 extended fields: `supervision`,
  `workspace_isolation`, `agent_loop_budget`, `approval_mode`,
  `output_format`, `memory_files`, `mcp_servers`, and `turn_caps` where
  relevant;
- recommended lane command, environment, and wrapper requirements;
- what should remain generic, advisory, forbidden, or unsupported;
- risks, missing docs, and unknowns;
- whether native subagents should remain internal to the parent session;
- any harness friction encountered during the research.

Publish the report as the required `handoff` artifact and complete the job.
If the tool docs cannot be reached or are too ambiguous to support a design,
publish what you found and block with a clear reason.
