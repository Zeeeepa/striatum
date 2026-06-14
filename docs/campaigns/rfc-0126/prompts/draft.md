# Draft — RFC 0126 P0: build-owned review generation

You are implementing **Phase P0 of RFC 0126 (multi-reviewer revision coherence,
accepted D194)** in the Striatum Go daemon. Read `docs/rfcs/0126-multi-reviewer-revision-coherence.md`
in full before starting; the decision is recorded as **D194** in
`docs/decisions/decision-log.md`.

## The P0 slice (and ONLY this slice)

Per the RFC's Phasing table, P0 is:

1. Add a `review_generation` (integer) column to `striatumd.verdicts` and a
   build-job-owned `review_generation` counter (on the build/synthesis job
   whose revision invalidates its reviewers). Both start at 1.
2. Stamp every verdict with the **reviewed build job's current generation** at
   record time, inside `applyVerdict` (the single INSERT chokepoint in
   `go/pkg/mutations/review.go` — confirm this is still the chokepoint).
3. Increment the build job's `review_generation` **in the same transaction** as
   `reopenJobForAttempt` (the attempt bump in
   `go/pkg/mutations/revision_routing.go`).
4. **Stop** the `DELETE FROM striatumd.verdicts` that `resetJobToBlockedCore`
   issues on revision (verdict history becomes append-only; staleness becomes a
   generation non-match, not a clear).

Do **not** implement P1–P3 (work-packet stamping / write-boundary rejection /
the obligation gate / retiring the legibility heuristic) in this slice — they
are separate runs.

## Load-bearing gotchas (these have bitten real runs — honor them)

- **OWNER-TABLE MIGRATION HAZARD.** `striatumd.verdicts` is created in
  `go/pkg/db/sql/0005_repo_local_workflow_state.sql`. The daemon applies regular
  runtime migrations **as the runtime role** (`striatumd_rw`), which **cannot
  `ALTER` an owner-held table** — a runtime migration that does so will
  **crash-loop the daemon** (the RFC 0081 incident; D-log "Daemon migrates as
  runtime role"). Determine the table's owner first. If `verdicts` is
  owner-held, the column add MUST go through an **owner/admin bundle**
  (`go/pkg/db/sql/owner/NNNN_*.sql`, see owner bundle 0007 for the pattern), NOT
  a plain runtime migration. If it is runtime-owned, a normal migration is fine.
  State which path you took and why in the DRAFT.
- **Same-transaction bump.** The generation increment and the attempt bump must
  be atomic (one tx) so a verdict cannot be stamped against a half-updated
  generation.
- **Migrations are append-only + numbered.** Add a new migration file with the
  next free number; never edit a shipped migration. Update
  `go/pkg/db/LatestDaemonDBVersion` (and any schema-version test) if you add a
  runtime migration.
- **No new graduated artifact shape** (RFC 0106 freeze) — the generation is a
  column on existing rows.
- **Reconstructability gate (#285).** The run-completion gate now hash-verifies
  required artifact bodies (`verifyRequiredArtifactReconstructable`); your change
  must not break it.

## Deliverable (write into DRAFT.md, and the code into the worktree)

Produce `docs/campaigns/rfc-0126/artifacts/DRAFT.md` containing:

1. A short design note: the exact column(s) added, the migration path chosen
   (runtime vs owner bundle) **with the ownership evidence**, and the three code
   edit sites (`applyVerdict`, `reopenJobForAttempt`, the removed DELETE).
2. The diff/patch you applied (or a precise file:line description of each edit).
3. The P0 test obligations you added or intend to add: at minimum a pgtest that
   a revision bumps the build's generation and the prior-round verdict rows
   **survive** (no DELETE) but are non-current (generation mismatch).

Write the actual code edits into the worktree (you are on a feature branch with
repo-write scope). Keep the change minimal and behavior-preserving for the
non-revision path. Run `make -C go build` and the targeted pgtests you add
before completing; report the results in DRAFT.md.

Stay strictly inside the P0 scope and your `write_scope.allowed_paths`.
