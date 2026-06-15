---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0127", "p0", "plain-dir-workspace", "round-2", "ownership-safe"]
---

# Review (round 2) — RFC 0127 P0: plain-dir workspace (opt-in)

author: reviewer-agent-002

## Verdict: accept_with_findings (severity: low)

Round 1 (`needs_revision`) returned this slice because the implementation was real
and building but **stranded uncommitted** in the draft per-job worktree — the run
branch carried only `DRAFT.md`. **That blocker is resolved.** The re-draft committed
the full implementation to the run branch, and §4 of the draft now honestly
documents the throwaway-cluster PG approach and what was *not* run. I reviewed the
**committed** state (branch HEAD), built it, and independently reproduced the
load-bearing PG-gated acceptance test. The slice is correct, ownership-safe, and
ready to integrate; the remaining items below are **non-blocking** notes for P1+ and
the operator's authoritative independent review.

## The round-1 blocker is fixed (evidence)

```
$ git log --oneline main..striatum/rfc-0127-p0-plain-dir-workspace
2f5fd90b feat(workspace): RFC 0127 P0 opt-in plain-dir job workspace (D195)
b23bd02f striatum: durable artifact publication (job …_draft)
```

Branch HEAD `2f5fd90b` now carries the code that round 1 found missing:

```
 docs/campaigns/rfc-0127/artifacts/DRAFT.md      |  71 +++---
 go/pkg/db/migrations.go                         |   3 +-   # LatestDaemonDBVersion 27 -> 28
 go/pkg/db/migrations_test.go                    |  52 +++++
 go/pkg/db/read_authority_inventory.go           |   1 +
 go/pkg/db/sql/0028_plain_dir_job_workspaces.sql |  53 +++++  # new migration (was absent on branch)
 go/pkg/db/write_authority_inventory.go          |   1 +
 go/pkg/mutations/worktree.go                    | 288 +++++++++++++++++++++++-
 go/pkg/mutations/worktree_test.go               | 196 ++++++++++++++++
```

All three round-1 required revisions are satisfied: (1) the implementation is on the
run branch; (2) it is committed (not loose in a worktree); (3) §4 reports the
PG-test reality rather than claiming PASS for un-reproducible checks.

## What I verified independently (from the *committed* tree)

I created a detached worktree of the committed branch HEAD in `/tmp` (no change to
the operator's tree) and ran, against `striatum-lane`'s sandbox:

- `go build ./...` (all of `go/`) — **PASS** (exit 0).
- `go vet ./pkg/db ./pkg/mutations` — **clean**.
- `gofmt -l` on all six changed `.go` files — **clean**.
- Hermetic: `TestWorkspaceTargetConfinesPathToStateWorkspaces`,
  `TestStagePlainDirBaseContentStagesTreeWithoutGit`,
  `TestHandleWorktreeCreateRejectsUnknownWorkspaceKind`,
  `TestMigrationTwentyEightJobWorkspacesIsOwnershipSafe` — **PASS**.

Because the PG-gated suite is the load-bearing P0 check, I did **not** take §4 on
trust: I stood up a private throwaway PostgreSQL 16.14 cluster (trust auth, bound
`127.0.0.1`, datadir under `/tmp`, `pg_ctl` stop + `rm -rf` after — the live daemon
DB on `:5432` was never touched) and re-ran the gated suites for real:

- `TestHandleWorktreeCreateStagesPlainDirWorkspace` (the **required P0 acceptance
  test**) — **PASS** (1.11s). Confirms: staged base content present, **no `.git`**,
  returned `base_tree_sha == git rev-parse <run-branch>^{tree}`, a `job_workspaces`
  row with `workspace_kind=plain_dir`/`state=active`, and **zero `job_worktrees`
  rows** (the opt-in does not touch the legacy table).
- `TestHandleWorktreeCreateDefaultStillCreatesGitWorktree` — **PASS** (1.14s). The
  legacy default is unchanged: omitting `workspace_kind` still creates a real git
  worktree (`.git` pointer) and records one active `job_worktrees` row.
- `pkg/db` migration / inventory / apply / owner-bundle suite incl.
  `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` — **PASS** (5.0s).
- `pkg/mutations` `Worktree|Workspace|PlainDir|Anchor|Release|Porter|Reseal`
  regression cluster — **PASS** (53.5s): the `confinedScratchTarget` refactor did
  not regress the legacy anchor/release/gc/porter paths.

So §4's claims are corroborated from committed state, not just asserted.

## Why the design is sound

- **Ownership-safe migration (the round-1 hazard the prompt flagged).** `0028` is a
  `CREATE TABLE IF NOT EXISTS striatumd.job_workspaces` with **no `ALTER`, no
  `DROP`, no foreign key**, plus a `pg_roles`-guarded `GRANT … TO striatumd_rw`.
  `CREATE TABLE` is not matched by the `>= 27` owner-DDL guard
  (`runtimeMigrationOwnerDDLPattern`), the new table is runtime-owned, and
  referential integrity is enforced in Go — exactly the migrations-16/23 precedent.
  This genuinely dodges the RFC 0081 owner-table crash-loop; the dedicated
  `TestMigrationTwentyEightJobWorkspacesIsOwnershipSafe` pins it.
- **Closed-enum opt-in.** `workspace_kind` defaults to `per_job`; `plain_dir`
  dispatches to the new path; any other value → `schema_invalid`. The legacy git
  path is byte-for-byte unchanged.
- **Genuinely unreachable by default.** Grepping the tree, `workspace_kind` /
  `plain_dir` / `job_workspaces` appear only in the handler, migration, the two
  authority inventories, and tests — **no workflow template or `workflow.json` sets
  `workspace_kind: plain_dir`**, so no in-flight `per_job` run is affected and an
  operator cannot accidentally select the not-yet-completable opt-in. D195 already
  accepts RFC 0127, so the deferred decision-log/spec edits are correctly left to
  land-time.
- **Staging is `.git`-free and tar-binary-free.** `git archive --format=tar <tree>`
  read fully into memory (no pipe deadlock) then extracted in-process via
  `archive/tar`, recording the **tree** sha (not the commit) so a retrospective can
  reconstruct base-XOR-published. The confinement refactor (`confinedScratchTarget`)
  preserves the `.striatum/worktrees` messages and adds parallel `.striatum/workspaces`
  confinement with the same symlink/traversal safety.

## Non-blocking findings (for P1+, do not gate this slice)

1. **`extractTarStream` confines entry *location* but not symlink *targets*; it is
   safe only for single-tree `git archive` input.** The `tar.TypeSymlink` branch
   does `os.Symlink(header.Linkname, dest)` without validating `Linkname`, and the
   path-confinement check (`strings.HasPrefix(dest, target+sep)`) tests the *cleaned
   string*, not the *resolved* path. For a general untrusted tar this is the classic
   symlink-traversal write-through (a symlinked directory component followed by a
   later file entry). **It is safe for this caller**: the source is a
   daemon-controlled `git archive` of one pinned tree, and a git tree cannot store
   both a symlink and a directory at the same name, so no later entry can traverse an
   earlier-created symlink. Recommend a one-line comment marking `extractTarStream`
   single-tree-git-archive-only, so a future reuse for untrusted input adds
   resolved-path (lstat-per-component) confinement before trusting it.

2. **Single-active guarantee comes from the unique index, not the `FOR UPDATE`
   probe.** `activeWorkspaceForJob` does `SELECT … FOR UPDATE` on a possibly-absent
   row, which does not gap-lock; two concurrent creates both pass the probe and the
   `uq_active_job_workspace` partial unique index is what actually rejects the
   second. The loser surfaces a raw PG unique-violation rather than the clean
   `"job already has an active workspace"` message. Correct (the index is the real
   guard, mirroring the legacy path) — just a later UX nicety to map the violation to
   the friendly error.

3. **The code landed because the re-draft hand-committed, not via a declared
   mechanism.** The round-1 root cause (the `draft` job declares only `DRAFT.md` as an
   artifact, so the porter never commits loose `go/` edits) is **moot for this run** —
   the author committed the `go/` files directly inside the draft worktree
   (repo-write scope), so they are on the branch. But the *workflow* still has no
   declared step that commits code; the `apply` prompt is a stub whose only artifact
   is `SUMMARY.md`. Not gating now (code is on the branch and verified), but worth the
   operator pinning a durable committing path before relying on this workflow shape
   again.

4. **A `plain_dir` job has no completion/release/gc path yet (correct for P0).** P1–P3
   (daemon-side diff + write-scope, porter commit from the plain dir, `work.complete`
   / `worktree.release` / `worktree.anchor` / `worktree gc` wiring, default flip) are
   intentionally deferred and documented in §4. Acceptable precisely because nothing
   selects the opt-in; keep it unreachable until P1 lands so an operator cannot strand
   a job.

## Summary

The round-1 delivery gap is closed: the implementation is committed on
`striatum/rfc-0127-p0-plain-dir-workspace`, builds clean, is ownership-safe by the
project's own guard, and the required P0 acceptance test passes — independently
reproduced here on a throwaway cluster, not taken on trust. Accepting with the four
non-blocking notes above carried forward to P1 and the operator's independent review.
