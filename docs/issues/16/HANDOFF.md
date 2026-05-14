# GH #16 Handoff

author: implementer-unknown-model-001

## Summary

Implemented the reusable operator initialization prompt, differentiated it from
the boundary guardrail in `prompts/README.md`, and trimmed the boundary prompt
to generic guardrail language.

## Definition Of Done Evidence

- DoD-1: `prompts/OPERATOR_INITIALIZATION_PROMPT.md` exists and is marked
  `Status: reusable` at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:1` and
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:3`.
- DoD-2: The new prompt is a complete initialization prompt with fill-in
  context, mission, required reading, operating rules, first-action sequence,
  and recovery guidance at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:16`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:47`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:61`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:86`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:121`, and
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:142`.
- DoD-3: `prompts/README.md` lists the initialization prompt separately from
  the boundary prompt and explains when to use each at
  `prompts/README.md:23` and `prompts/README.md:28`.
- DoD-4: The old boundary prompt is kept as a focused guardrail at
  `prompts/OPERATOR_BOUNDARY_PROMPT.md:7`, with the new initialization prompt
  pointing to it at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:54` and
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:81`.
- DoD-5: The new prompt uses generic target repository, workflow, lane,
  session, artifact, and runner-state language at
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:12`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:22`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:35`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:52`, and
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:137`. A forbidden-token scan over
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md` and
  `prompts/OPERATOR_BOUNDARY_PROMPT.md` found no stale issue-specific,
  project-specific, named substrate, hardcoded home path, or historical
  dogfood-example tokens.
- DoD-6: The prompt reflects the daemon/Postgres transition and current escape
  caveat at `prompts/OPERATOR_INITIALIZATION_PROMPT.md:27`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:92`, and
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:96`.
- DoD-7: A fresh operator can start or resume without historical dogfood
  prompts because the prompt supplies required reading, state checks,
  validation, run creation/resume, report setup, and continuation steps at
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:63`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:83`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:88`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:100`,
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:123`, and
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md:140`.

## Verification

- Ran a forbidden-token scan over
  `prompts/OPERATOR_INITIALIZATION_PROMPT.md` and
  `prompts/OPERATOR_BOUNDARY_PROMPT.md` for the scope's stale/historical
  examples, project-specific name, named state substrate, hardcoded home paths,
  and issue-specific dogfood ordinals; it returned no matches.
