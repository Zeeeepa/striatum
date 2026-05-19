# Verify -- GH #25

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/25/SPEC.md`
- `docs/issues/25/SCOPE.md`
- `docs/issues/25/build/HANDOFF.md`
- the changed files named by the handoff

## Output

Write `docs/issues/25/review/REVIEW.md` with `striatum.finding.v1`
front matter. Use the exact `author:` line from the work packet.

Include:

- final verdict (`accept`, `accept_with_findings`, `needs_revision`, or
  `reject`);
- per-bullet acceptance verification with file:line evidence for each
  "Acceptance / Definition of done" bullet in `SPEC.md`;
- particular adversarial probes:
  - **No SQLite check anywhere on the `list` path**. grep the changed
    files: if `state.sqlite3` or `repo_not_migrated` still appears
    on the read path, it's a `needs_revision`.
  - **`adopt` and `repo add --init` still refuse** an
    unmigrated-but-state-DB-present setup (regression risk on the
    mutation side).
  - **`--json` round-trip is byte-for-byte unchanged**. Run both
    before+after and diff the JSON.
  - **Daemon-unreachable path** returns a clean
    `daemon_unreachable` (or equivalent), not a stale
    `repo_not_migrated`.
  - **Table format**: cwd marker, column headers, alignment readable
    in a typical 80-col terminal.
- test/verification assessment;
- findings with severity and exact remediation when any gap remains.
