# Verify -- GH #24

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/24/SPEC.md`
- `docs/issues/24/SCOPE.md`
- `docs/issues/24/build/HANDOFF.md`
- the changed files named by the handoff
- the updated `~/.claude/skills/striatum-supervise/SKILL.md` and
  `~/.claude/skills/striatum-claim-loop/SKILL.md`

## Output

Write `docs/issues/24/review/REVIEW.md` with `striatum.finding.v1`
front matter. Use the exact `author:` line from the work packet.

Include:

- final verdict (`accept`, `accept_with_findings`, `needs_revision`, or
  `reject`);
- per-bullet acceptance verification with file:line evidence for each
  "Acceptance / Definition of done" bullet in `SPEC.md`. Bullets that
  cannot be fully verified must be reported as `needs_revision` with
  the concrete gap;
- particular adversarial probes:
  - **Worked example actually works**: try the literal commands from
    the new skill bundle against a fresh test run. If it doesn't
    produce a packet on the FIFO end-to-end, that's a `needs_revision`.
  - **Wrong-kind ID error**: pass `--packet-id <msg_id>` to
    `supervise send` and confirm the error names the wrong-kind
    ID and points at the right field.
  - **release --requeue regression**: confirm the verb either
    re-queues OR refuses with exit code != 0; silently parking in
    blocked is a `reject`.
  - **Lease/message IDs preserved**: confirm `lease_id` and
    `message_id` semantics are unchanged where they should be (ack,
    heartbeat, complete still work with their normal IDs).
- test/verification assessment;
- findings with severity and exact remediation when any gap remains.
