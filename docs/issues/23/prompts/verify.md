# Verify -- GH #23

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/23/SPEC.md`
- `docs/issues/23/SCOPE.md`
- `docs/issues/23/build/HANDOFF.md`
- the changed files named by the handoff
- `docs/issues/22/SPEC.md` only as far as is needed to verify the
  cross-issue "daemon stop falls back to the pidfile" bullet did not
  silently regress.

## Output

Write `docs/issues/23/review/REVIEW.md` with `striatum.finding.v1`
front matter. Use the exact `author:` line from the work packet.

Include:

- final verdict (`accept`, `accept_with_findings`, `needs_revision`, or
  `reject`);
- per-bullet acceptance verification with file:line evidence for each
  "Acceptance / Definition of done" bullet in `SPEC.md`. Bullets that
  cannot be fully verified must be reported as `needs_revision` with
  the concrete gap;
- particular adversarial probes:
  - **Race**: does `daemon status` ever see a missing pidfile between
    bind-socket and write-pidfile? Recommend write-pidfile *before*
    bind-socket if there's a meaningful gap.
  - **Permissions**: does the pidfile end up with mode 0600 and owned
    by the running user, even if `runtime_dir()` was created earlier
    with a different umask?
  - **Atomicity**: is the write atomic via temp + rename, or is there
    a window where the file is empty / half-written?
  - **Cleanup-on-crash**: is a stale pidfile from a previous crash
    correctly overwritten on the next start, not appended to?
  - **Test coverage**: do the new tests actually exercise the file's
    presence on disk, not just a mocked return value?
- test/verification assessment;
- findings with severity and exact remediation when any gap remains.
