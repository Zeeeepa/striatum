# Review Authority Boundary

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review the generated Go CLI RPC router implementation for authority-boundary
risk.

Produce a finding with:

- verdict: `accept`, `needs_revision`, or `reject`;
- whether daemon-backed CLI routes mutate or read live workflow state only
  through daemon RPC;
- whether any direct PostgreSQL, Python, SQLite, marker-file, terminal-output,
  or transcript authority path was introduced;
- whether local workflow-authoring commands are explicit and bounded;
- concrete file/line references for required fixes;
- focused re-test commands.

Do not edit implementation files in this job.
