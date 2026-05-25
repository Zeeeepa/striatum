# Build Mutation Parameter Adapters

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement parameter adapters for generated mutation routes.

Scope examples include run lifecycle, session registration/close, work claim and
completion, artifact publication, review verdicts/overrides, recovery,
decision/checkpoint/escalation, branch confirmation, worktree, supervision, and
repo mutation routes. Use the route-contract artifact as the final grouping
authority.

Requirements:

- keep every mutation as a daemon RPC call with the required capability;
- preserve write-scope, lease, review, and recovery authority in the daemon;
- do not introduce local state mutation, direct DB access, or Python fallback;
- add focused tests for parser-to-params behavior and method selection;
- record commands run and any deferred mutation families in the handoff.
