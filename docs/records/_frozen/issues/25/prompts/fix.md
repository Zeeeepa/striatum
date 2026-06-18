# Implement -- GH #25

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/25/SPEC.md`
- `docs/issues/25/SCOPE.md`
- the source modules named in `SCOPE.md`

## Deliverables

Per `docs/issues/25/SPEC.md` "Acceptance / Definition of done":

1. `striatum repo list` (no --json) on a daemon-registered repo prints
   a human-readable table of registered repositories (columns named
   in `SCOPE.md`). Cwd-match is marked if it's listed.
2. The SQLite-presence pre-flight is removed from the `list` path.
   `adopt` / `repo add --init` retain it (or the equivalent
   mutation-side check).
3. `--json` shape is unchanged byte-for-byte.
4. Daemon-unreachable surfaces a clean `daemon_unreachable` error;
   no more `repo_not_migrated` from `repo list`.
5. Tests pin both the table output and the daemon-unreachable error.

## Constraints

- Stay inside `write_scope.allowed_paths`.
- The table renderer should be small and inline — do not introduce a
  new table-rendering dependency.
- If `SCOPE.md` named a shared pre-flight helper, do NOT delete it;
  only gate it on the mutation/list axis named there.
- Use the exact `author:` line from the work packet in the handoff.

## Handoff

Write `docs/issues/25/build/HANDOFF.md` with the
`striatum.handoff.v1` front matter. Cite each definition-of-done bullet
closed, files changed, tests run / not run, and residual risk.
