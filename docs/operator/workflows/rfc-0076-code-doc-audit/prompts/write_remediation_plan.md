# Write Remediation Plan

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`. Stay within the packet's write scope.

Read the synthesis artifact and write a task-oriented remediation plan.
Map every material finding, and every high or critical finding in
particular, to one follow-up path:

- already covered by an existing TODO or RFC;
- needs a new RFC;
- needs a decision-log update;
- needs a docs-only correction;
- needs source and test work;
- historical only, no action;
- accepted risk or wontfix requiring owner decision.

For each planned item, include the source `AUD-###` ids, severity,
evidence summary, owner surface, recommended next action, and the exact
existing TODO/RFC/decision/docs/source/test/wontfix path when known.

Do not implement fixes, update decisions, or rewrite historical fixtures
in this job. Do not rely on terminal output, transcripts, marker files,
tmux panes, or provider hooks as authority.
