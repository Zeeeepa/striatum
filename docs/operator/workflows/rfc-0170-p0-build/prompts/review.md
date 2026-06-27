# Review — RFC 0170 P0 implementation

You are the **reviewer** (fresh session). Judge the draft against the SPEC
(`docs/operator/artifacts/rfc-0170-p0-design-v5/commit/proposal/PROPOSAL.md`) and the cleared
ledger (`…/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`). Verdict: **accept** /
**accept_with_findings** / **needs_revision**. Verify the draft IN ITS WORKTREE.

Confirm each gate is implemented faithfully (SPEC §7 assertion ledger):

- **G3 substrate** — migration is `go/pkg/db/sql/0045_cullable_entity.sql` (free slot, under
  `sql/`); table has `PRIMARY KEY (kind, ref)` + both CHECKs; **no FK, no owner DDL** (≥27);
  `GRANT SELECT,INSERT,UPDATE` to `striatumd_rw`, **no DELETE**; `RESERVATIONS.toml` row
  `ordinal=45`; **both** authority-inventory rows (`ReadClassRuntimeOperational` +
  `ClassRuntimeDML`); and **grep proves no `SELECT *`** against `cullable_entity` (C1–C3/PC1-2).
- **G1 predicate** — the known-set corpus test asserts **zero false positives** on the
  preserved set (A2), `decision:D267` **nominated** (A1), `decision:D081` the documented
  **#618-withheld** member (A1′/BC-618); structural-status-only (A3); bare/non-resolving incl.
  `D084` withheld (A4/A7); clauses 2↔4 reconciled with the fixed lexicon + link/row regex (A4′);
  **fully-static tree-local** pathspec, no external/open-issue term, `docs/rfcs/`+`docs/decisions/`
  eligible (A8); no candidacy from `verdicts.superseded_by_decision_id` (A5/PC3); no `kind=branch`
  (A6).
- **G2 safety** — the cull fold's only write is the `cullable_entity` UPSERT and it rides
  **off the wait-gating path** (O(1) slot-check/spawn/skip, never joins the detached scan) (B1/B4);
  panic recover seam in the detached goroutine top frame (B2); `DefaultCullFoldTimeout=10s <
  DefaultSweepInterval` with a static test; L4 compute-then-commit writes zero on
  timeout/non-return; the **B5 HANG A/B + refresh-not-deferred** test is present with its
  on-wait-path negative control; the **BC-619 late-return-zero-write guard** is present.
- **G4 forward-compat** — `(kind,ref)` ON CONFLICT + extensible `candidacy_state` CHECK admit a
  later writer with no schema break (D1); **no smuggled P1+ action** (tombstone/deletion/page/
  doctor-class/run-admission) — P0 stays observe-only.

Run `cd go && go build ./... && go vet ./...` in the draft's worktree and confirm green;
spot-check the test files **compile** and exercise the real predicate/sweep paths (not stubs).

Return **needs_revision** with the specific gap if any gate-critical assertion or build-carry is
unimplemented, the build is broken, a `SELECT *` slipped in, owner-DDL/FK crept into the
migration, the fold sits on the wait-gating path, or a P0-forbidden action was smuggled in.
Otherwise **accept_with_findings**, noting any minor residual for the verifier stage. Write only
the review-only finding at `docs/operator/artifacts/rfc-0170-p0-build/review/REVIEW.md`.
