# Implement -- GH #26

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/26/SPEC.md`
- `docs/issues/26/SCOPE.md`
- `docs/rfcs/0073-daemon-doctor-blob-parity.md`
- the source modules named in `SCOPE.md`

## Deliverables

Per `docs/issues/26/SPEC.md` "Acceptance / Definition of done":

1. `daemon doctor --json` surfaces `data.blob = {configured, reachable, ...}` in both code paths (with and without `--repo`).
2. Bucket-aware fields appear when `--repo` is supplied.
3. Non-`--json` form prints a one-line summary.
4. Tests pin both unconfigured and configured-reachable.
5. `make smoke` and `make pg-test` still pass.

## Constraints

- Stay inside `write_scope.allowed_paths`.
- Do NOT widen the daemon_diagnostics shape; the blob block goes at the same level (`data.blob`), not inside `daemon_diagnostics`.
- If Option A (sub-RPC) was chosen: bound the sub-RPC at the call site so doctor-calling-doctor cycles can't recurse.
- Use the exact `author:` line from the work packet in the handoff.

## Handoff

Write `docs/issues/26/build/HANDOFF.md` with `striatum.handoff.v1` front matter. Cite each definition-of-done bullet closed, files changed, tests run, and residual risk.
