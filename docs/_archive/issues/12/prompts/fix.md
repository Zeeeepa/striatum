
# Implement Issue Workflow

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/12/SPEC.md`, `docs/issues/13/SPEC.md`
- upstream scope/design artifacts produced by this workflow
- `docs/ROADMAP.md` and `docs/TODO.md`
- source/tests/docs named by the issue specs and upstream artifacts

Implement only the #12 copy-on-click allowlist and #13 graph-editor stale-field purge. Do not pull in #9-#11 security-hardening work.

## Deliverables

- source or documentation changes required by the captured issue specs;
- focused tests or verification artifacts when behavior changes;
- the handoff artifact declared in the work packet, citing changed files,
  tests run, tests not run, and residual risk.

Use the exact `author:` line from the work packet in the handoff. Stay
inside `write_scope.allowed_paths`.
