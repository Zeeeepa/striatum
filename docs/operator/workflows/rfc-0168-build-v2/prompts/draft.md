Read the workflow packet and all required context docs before editing.

Implement the RFC 0168 P0 build inside the declared write scope, starting from
current `main`, not from the canceled first build branch. Treat D272's follow-up
as binding: provider-owned credential selectors must fail closed when uncovered
and resolving in-repo, while ordinary non-credential lane env such as `AGY_HOME`
or `FIXTURE_CONFIG_DIR` must still launch.

This v2 build exists to close the first run's blockers:

- F1: complete S1-S3/P1-P5 return, scrub, proof, reaper, quarantine, and retry
  semantics, including fail-closed `/proc` read errors.
- F2: generation must be enforced on attestation and control-frame/report paths,
  not merely injected into environment or metadata.
- F3: per-job worktrees must be granted to the leased uid at worktree creation or
  lease attach.

Do not collide with current `main`: RFC 0171 already owns runtime schema
`0046_generated_records.sql`, so recheck and use the next free runtime migration
slot. Preserve all RFC 0171 generated-records/docket code and docs.

Publish the required DRAFT.md with a concise implementation ledger: files
changed, design assertions/gates discharged, tests run, and residuals for the
reviewer/operator.
