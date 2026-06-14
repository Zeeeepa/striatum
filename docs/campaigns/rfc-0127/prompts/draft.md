# Draft — RFC 0127 P0: plain-dir workspace (opt-in)

Implement **Phase P0 of RFC 0127 (retire the lane git identity, accepted D195)**.
Read `docs/rfcs/0127-retire-lane-git-identity.md` (P0 only) and D195 first.

## The P0 slice (and ONLY this slice)

Add an **opt-in** `workspace_kind: plain_dir` path to `workspace.create`
(today `HandleWorktreeCreate` in `go/pkg/mutations/worktree.go` runs
`git worktree add --detach`). When `plain_dir` is selected the daemon creates a
plain directory under `.striatum/workspaces/<id>` and **stages the base
content** into it from the run-branch HEAD (`git archive` / `git checkout-index`
into the plain dir — no `.git`), recording the **base tree sha** in
`job_workspaces` before the lane starts. The legacy `per_job` git-worktree path
stays the default and unchanged.

Do NOT implement P1–P3 (daemon-side diff + write-scope against the plain dir, the
porter commit from the plain dir, overlayfs, the default flip). This slice is
just the opt-in plain-dir creation + base-tree-sha recording.

## Gotchas
- **Owner-table hazard:** if `job_workspaces` needs a new column
  (`workspace_kind`, `base_tree_sha`), check its owner before adding via a
  runtime migration — a runtime ALTER on an owner-held table crash-loops the
  daemon (see the RFC 0081 incident / D-log "Daemon migrates as runtime role").
  Route owner-held DDL through an owner bundle (`go/pkg/db/sql/owner/`).
- Migrations are append-only + numbered; bump `LatestDaemonDBVersion` if you add
  a runtime migration.
- Reversible + opt-in: no in-flight `per_job` run may break.

## Deliverable
Write `docs/campaigns/rfc-0127/artifacts/DRAFT.md` (design note + the migration
path with ownership evidence + edit sites + the P0 test: a job created with
`workspace_kind: plain_dir` gets a `.git`-free directory with the base content
staged and the base tree sha recorded). Write the code into the worktree
(feature branch, repo-write scope). `make -C go build` + targeted pgtest before
completing; report results. Do not merge to main.
