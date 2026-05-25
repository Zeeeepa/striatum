# Review Route Coverage

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review generated route coverage against `contracts/daemon_methods.json`.

Produce a finding with:

- verdict: `accept`, `needs_revision`, or `reject`;
- whether every `cli_routes[]` entry is generated, locally handled, or
  explicitly deferred;
- whether generated freshness checks catch contract drift;
- whether params groups cover read, mutation, recovery, supervision, repository,
  cross-repo, and escalation families;
- any command whose old Python behavior is not represented in Go;
- concrete file/line references and re-test commands.

Do not edit implementation files in this job.
