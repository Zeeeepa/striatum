# Implement -- GH #24

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/24/SPEC.md`
- `docs/issues/24/SCOPE.md`
- the source modules named in `SCOPE.md`
- `~/.claude/skills/striatum-supervise/SKILL.md` and
  `~/.claude/skills/striatum-claim-loop/SKILL.md` -- you will be
  editing these directly per the scope.

## Deliverables

Per `docs/issues/24/SPEC.md` "Acceptance / Definition of done":

1. `packet_id` is surfaced from `claim-next` in whichever shape
   `SCOPE.md` chose (top-level, next_steps, or implicit-lookup).
2. `supervise send` returns a useful error pointing at the right ID
   when given the wrong-kind ID.
3. `release --requeue` either honors the requeue for repo_write jobs
   or refuses with a non-zero exit code naming the recovery verb. The
   silent-blocked path is gone.
4. Both skill SKILL.md files contain a worked, copy-pasteable example
   that goes from "fresh session" to "supervised agent reads a packet."
   Re-generate the skills via `striatum skills install` and confirm the
   user-visible files match.
5. Tests pin both behaviors (integration test for the worked
   example; behavioral test for release --requeue).

## Constraints

- Stay inside `write_scope.allowed_paths`.
- Do NOT introduce a third recovery verb if `--requeue` can be made
  consistent. Prefer fixing existing semantics over adding new ones.
- Preserve the existing `lease_id` / `message_id` semantics — they
  are not packet IDs; they're separate identifiers and the fix should
  make that distinction loud.
- Use the exact `author:` line from the work packet in the handoff.

## Handoff

Write `docs/issues/24/build/HANDOFF.md` with the
`striatum.handoff.v1` front matter. Cite each definition-of-done bullet
closed, files changed, tests run / not run, and residual risk. Include
a literal copy-pasteable example showing the fixed `claim-next` →
`supervise send` flow.
