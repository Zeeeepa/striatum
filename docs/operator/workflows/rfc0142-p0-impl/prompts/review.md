# Task — Review: does the P0 build correctly discharge C1–C5?

You are a **fresh-session reviewer**. Read `SEED.md`,
`committee-output/COLLABORATION_LEDGER_cycle_1.md` (the binding constraints), the
upstream **`DRAFT.md`**, and the **actual source diff** the draft produced (inspect
the changed files in the worktree: `go/pkg/pgtest/`, the new `*_pg_test.go`). Review
the CODE, not just the handoff.

## Check, concretely

1. **C1 (escape-proof role):** Is the SUT executed through a dedicated
   non-superuser, non-owner LOGIN role — NOT `SET ROLE` inside a privileged
   connection? Would `RESET ROLE` / `SET ROLE NONE` escape back to the owner? Does
   the red test actually assert `42501` after `RESET ROLE`?
2. **C2 (ownership fidelity):** Are the recent runtime tables (0038/0041/0042)
   `striatumd_rw`-owned in the fixture? Is "owner-held" derived from the same
   source as the static guard (no drift)? Is there a real differential
   green-control (legal runtime `ALTER` succeeds) AND red (owner-held `ALTER`
   `42501`s)?
3. **C3 (non-superuser bootstrap):** Does Phase A grant then **revoke** the
   transfer membership before Phase B? Does it avoid silently needing superuser?
4. **C4 (isolation self-check):** Does it assert no owner-role membership/inherit
   via `pg_has_role`/`pg_auth_members` at probe time, after C3's grant is revoked?
   Does it abort loudly if isolation fails (so a red `42501` is only trusted then)?
5. **C5 (search_path):** Pinned?
6. **Correctness & boundary:** Does it `go build` / `go vet` clean (reason about
   it; the verifier confirms)? Is it test-harness + test code only (no migration /
   bundle / daemon change, no later-layer symbols)? Is the `_pg_test.go` guarded to
   skip without a PG cluster while keeping real assertions (not weakened to pass)?
   Is the diff minimal and idiomatic?

## Verdict

Record a `finding` (the verdict path). Use:
- **`needs_revision`** if any binding constraint (C1–C4 especially) is not
  genuinely discharged, the red test is fabricated/wrong-reason, the green control
  is missing, the boundary is violated, or it won't build/vet. List each defect
  precisely so the single revision can fix it. (One revision cycle is available.)
- **`accept`** / **`accept_with_findings`** if C1–C5 are discharged and the code is
  correct and in-boundary (note any minor non-blocking nits as findings).

Write only your review finding artifact at the declared path. Do not modify source.
