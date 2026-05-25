# Review Operator Ergonomics

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review the generated Go CLI RPC router for operator-facing regressions.

Produce a finding with:

- verdict: `accept`, `needs_revision`, or `reject`;
- whether common command shapes keep expected flag names, output modes, and
  error messages;
- whether unknown commands, missing required flags, daemon unreachable,
  repo-not-registered, and capability-refusal paths remain understandable;
- whether local commands are visible as local behavior rather than hidden daemon
  routes;
- concrete file/line references for required fixes;
- focused re-test commands.

Do not edit implementation files in this job.
