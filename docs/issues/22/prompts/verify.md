# Verify -- GH #22

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/22/SPEC.md`
- `docs/issues/22/SCOPE.md`
- `docs/issues/22/build/HANDOFF.md`
- the changed files named by the handoff
- `docs/issues/23/SPEC.md` only as far as is needed to verify the
  "daemon stop must work without migrating" acceptance bullet did not
  silently rely on a #23-side fix.

## Output

Write `docs/issues/22/review/REVIEW.md` with `striatum.finding.v1`
front matter. Use the exact `author:` line from the work packet.

Include:

- final verdict (`accept`, `accept_with_findings`, `needs_revision`, or
  `reject`);
- per-bullet acceptance verification with file:line evidence for each
  "Acceptance / Definition of done" bullet in `SPEC.md`. Bullets that
  cannot be fully verified must be reported as `needs_revision` with
  the concrete gap;
- particular adversarial probes:
  - does the new owner-path *still* gate the `unsafe_privileges` check
    for runtime connections? (regression risk);
  - does `daemon stop` succeed even with a fresh PG that has zero
    migrations applied, or a PG that's intentionally unreachable?
  - does the hint string still appear in the failure mode, and does it
    name the new shape?
- test/verification assessment;
- findings with severity and exact remediation when any gap remains.
