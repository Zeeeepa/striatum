# Verify -- GH #26

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/26/SPEC.md`
- `docs/issues/26/SCOPE.md`
- `docs/issues/26/build/HANDOFF.md`
- `docs/rfcs/0073-daemon-doctor-blob-parity.md`
- the changed files named by the handoff

## Output

Write `docs/issues/26/review/REVIEW.md` with `striatum.finding.v1` front matter.

Include:

- final verdict (`accept`, `accept_with_findings`, `needs_revision`, or `reject`);
- per-bullet acceptance verification with file:line evidence;
- adversarial probes:
  - **Unconfigured path**: `data.blob` returns `{"configured": false}` when `STRIATUM_BLOB_ENDPOINT` is unset (and only that).
  - **Reachable path**: when configured and Garage is up, `data.blob.reachable: true` and the bucket round-trip probe completes successfully (or reports a structured error).
  - **Cycle safety** (Option A only): doctor doesn't recurse into doctor.
  - **JSON shape preservation**: `daemon_diagnostics` shape is unchanged byte-for-byte; the blob block lives outside it.
- test/verification assessment;
- findings with severity and exact remediation when any gap remains.
