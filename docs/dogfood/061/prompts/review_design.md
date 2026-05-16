# Design review (claude `ergonomics_dx`)

Read `docs/dogfood/061/DESIGN_SYNTHESIS.md` and the three designer
inputs. Produce `docs/dogfood/061/review/design/REVIEW.md`
(front-matter `finding.v1`, byline `author: reviewer-claude-001`,
`verdict_intent: accept|accept_with_findings|needs_revision|reject`).

## Bouncing checklist (any one ⇒ `needs_revision`)

1. Reconciliation hook is a menu, not a locked module + function.
2. Atomic-transaction discipline missing — publish/verdict/complete
   not inside one `conn.transaction()`.
3. Feature-flag check is inside `publish_artifact_inline` instead
   of at hook entry.
4. `lane_finalization=auto_from_artifact` audit marker absent from
   the locked event payload.
5. Acceptance tests not concretely named (function + fixture).
6. Single-track discipline violated (dual-track proposed).
7. Locked file path doesn't exist in main (citation invalid).

## Ergonomics_dx checks

- The refusal-fall-through path preserves the existing lane-stall
  blocker hint exactly. Operator UX is identical to today for
  missing/malformed/byline-mismatch cases.
- Auto-finalize does not silently change the dashboard contract.
- The audit-chain row contains both `decision='allowed'` AND
  `lane_finalization=auto_from_artifact` so a reviewer can grep
  the difference from agent-initiated finalize.

## Verdict guidance

- `accept` — synthesis is locked and reviewers can build to it.
- `accept_with_findings` — minor doc nits only.
- `needs_revision` — any bouncing-checklist item fires.
- `reject` — synthesis fundamentally misframed (reserved for cycle
  exhaustion). Cite specific RFC §s contradicted.

## Write scope

`docs/dogfood/061/review/design/REVIEW.md`.
