# Review — RFC 0126 P0 implementation

You are a fresh independent reviewer of the RFC 0126 P0 draft (build-owned
review generation). Read `docs/rfcs/0126-multi-reviewer-revision-coherence.md`
(P0 only) and the published DRAFT.md, then inspect the actual worktree changes.

Record a finding (accept / needs_revision) judging ONLY:

1. **Correctness of scope.** Exactly P0: the `review_generation` column + build
   counter, the stamp in `applyVerdict`, the same-tx bump in `reopenJobForAttempt`,
   and the removal of the verdict DELETE. No P1–P3 leakage.
2. **The owner-table migration hazard.** Did the author verify whether
   `striatumd.verdicts` is owner-held vs runtime-owned, and route the column add
   accordingly (owner bundle if owner-held)? A runtime `ALTER` on an owner table
   will crash-loop the daemon — this is the single highest-risk point. Reject if
   the ownership was not verified or the path is wrong.
3. **Append-only history.** The DELETE is removed and prior-round verdicts
   survive a revision (a generation non-match, not a clear).
4. **Atomicity.** The generation bump shares the attempt-bump transaction.
5. **Build + tests.** `make -C go build` passes and a P0 pgtest exists proving
   the bump + history survival.

needs_revision if any of the above is wrong, unverified, or out of scope. Be
specific: name the file:line and the exact defect.
