# V1 MVP Findings Ledger

Date: 2026-05-06

This ledger normalizes findings from `docs/reviews/v1/V1_MVP_DESIGN_REVIEW.md`.

| ID | Severity | Finding | Disposition | Accepted Delta |
|----|----------|---------|-------------|----------------|
| V1-F001 | P1 | Gemini's reduced command set conflicts with P001 required `ack`, `heartbeat`, `release`, `block`, `complete`, and `verdict` behavior. | accepted_with_modification | Keep the commands, but implement lazy lease expiry and no background daemon. |
| V1-F002 | P1 | Branch confirmation must block claimability, not only git mutation. | accepted | `claim-next` must fail or return no claimable work until confirmation and run start. |
| V1-F003 | P1 | Completion must verify required artifacts. | accepted | `complete` and `verdict` check required artifacts before terminal states. |
| V1-F004 | P2 | Expired leases need different treatment for review-only and repo-write work. | accepted | Review-only leases can requeue; repo-write stale leases block. |
| V1-F005 | P2 | Adapter core must not encode provider assumptions. | accepted | Lanes are command arrays and capabilities; stdout is non-authoritative. |
| V1-F006 | P2 | Direct SQLite writes cannot be perfectly prevented. | accepted | Use CLI contract, constraints, triggers, and `doctor` checks. |
| V1-F007 | P3 | Coordinator command allowlists are needed but not a state-MVP blocker. | deferred | Keep as spec requirement; implement when coordinator chat skills are built. |

## Owner Checkpoints

- Branch creation/selection still requires human confirmation before source or
  build-file edits.

## Proposed Decision Log Entries

- Add a V1 design decision for lazy lease expiry with stale repo-write leases
  blocking instead of automatic requeue.
- Add a V1 design decision that process/tmux adapters are launch boundaries,
  while SQLite remains authoritative and terminal output is not parsed as
  state.

