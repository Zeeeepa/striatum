# RFC 0127 P0 — plain-dir workspace (opt-in) (DRAFT)

author: author-agent-002

Implements **Phase P0 of RFC 0127** (retire the lane git identity — the lane as a
pure byte producer, accepted **D195**). Scope is strictly P0: an **opt-in**
`workspace_kind: plain_dir` path on `workspace.create` (today `HandleWorktreeCreate`)
that creates a plain, daemon-owned, `.git`-free directory under
`.striatum/workspaces/<id>`, **stages the run-branch base content** into it, and
records the **base tree sha** in `job_workspaces` **before the lane starts**. The
legacy `per_job` git-worktree path stays the default and is unchanged.

**P1–P3 are intentionally NOT implemented**: no daemon-side change-set diff /
write-scope enforcement against the plain dir, no porter commit
(`commitArtifactBodyToRunRef`) sourcing the plain dir, no overlayfs/fanotify,
no default flip, and no plain-dir wiring into `work.complete` / `worktree.release`
/ `worktree.anchor` / `worktree gc`. P0 is just the opt-in creation + base-tree-sha
recording.

---

## 1. Design note

### Where the base-tree-sha lives: a NEW runtime-owned `job_workspaces` table

P0 adds a **new table** `striatumd.job_workspaces` (created by a numbered runtime
migration, `0028_plain_dir_job_workspaces.sql`) rather than (a) `ALTER`-ing the
existing `job_worktrees` table or (b) routing the column add through an owner
bundle. The columns it needs are exactly the RFC's two new fields plus enough
bookkeeping to be self-contained:

| Column | Type | Note |
| --- | --- | --- |
| `repository_id`, `workspace_id` | `text` | PK `(repository_id, workspace_id)` |
| `run_id`, `job_id`, `lease_id` | `text` | scope; no FK (see ownership) |
| `workspace_kind` | `text CHECK IN ('plain_dir')` | only ever holds opt-in rows |
| `base_branch` | `text` | the ref staged from |
| `base_tree_sha` | `text NOT NULL` | **the durable "before" pin** (the tree sha) |
| `workspace_path` | `text` | relative `.striatum/workspaces/<id>` |
| `state` | `text CHECK IN ('active','released','removed','abandoned')` | mirrors `job_worktrees` |
| `created_at`, `released_at`, `removed_at` | `timestamptz` | lifecycle |

Plus `uq_active_job_workspace (repository_id, job_id) WHERE state='active'` (one
active workspace per job, mirroring `uq_active_job_worktree`) and
`idx_job_workspaces_run`.

### Migration path with ownership evidence (the owner-table hazard the prompt flagged)

The slice prompt warned about the **owner-table migration hazard** (the RFC 0081
incident / D-log "Daemon migrates as runtime role"): a runtime `ALTER` on an
owner-held table crash-loops the daemon, which applies runtime migrations as
`striatumd_rw`.

**Evidence, decisive.** Even though `job_worktrees` is itself **runtime-owned**
(it is created by runtime migration `0005`, and per **D173** "these tables are
owned by `striatumd_rw` (created by runtime migrations)"), the repo's static guard
forbids **any** `ALTER`/`DROP TABLE striatumd.*` in a regular migration at or
above the floor, regardless of who owns the table:

```
go/pkg/db/migrations_test.go
  const futureRuntimeMigrationOwnerDDLFloor = 27
  runtimeMigrationOwnerDDLPattern =
    `(?is)\b(?:ALTER|DROP)\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?striatumd\.[a-z_][a-z0-9_]*`
  // TestFutureRuntimeMigrationsDoNotCarryOwnerDDL fails any migration >= 27 that matches.
```

A new runtime migration is version **28** (≥ 27), so an
`ALTER TABLE striatumd.job_worktrees ADD COLUMN …` would fail that guard. This is
the same wall the RFC 0126 P0 slice hit (D187 owner-table-DDL boundary).

That leaves two real options, and I chose the third, cleanest one:

- **(A) Owner bundle that `ALTER`s `job_worktrees`.** Works (the owner role is a
  member of `striatumd_rw`, so it can alter the table), but it is owner-applied
  **out-of-band** with a binary-before-bundle deploy ordering, and it splits the
  table's shape across a runtime migration and an owner bundle. Heavyweight for an
  opt-in P0.
- **(B) Reuse `job_worktrees` for plain dirs.** Rejected: a plain dir lives under
  `.striatum/workspaces` (fails `worktreeTarget`'s `.striatum/worktrees`
  confinement), and every `job_worktrees` consumer — `worktree.release`,
  `worktree.anchor`, `worktree gc`, the RFC 0125 porter — reads a **git worktree
  HEAD**, which a plain dir does not have. Reusing the table would force a
  `workspace_kind` branch through all of that machinery (that is precisely P1–P3).
- **(C, chosen) A NEW runtime-owned `job_workspaces` table via a CREATE-TABLE
  runtime migration.** `CREATE TABLE` is **not** forbidden by the guard (only
  `ALTER`/`DROP` of existing tables is); the new table is owned by `striatumd_rw`
  so the daemon fully manages it with ordinary DML; **no owner bundle, no
  out-of-band step, no crash-loop**. It is opt-in and reversible — only `plain_dir`
  opt-ins ever write a row, so no in-flight `per_job` run is touched — and it
  matches the RFC's own `job_workspaces` naming.

**Ownership-safety, like migrations 16 and 23.** The migration declares **no
foreign keys** (referential integrity to `repositories`/`runs`/`jobs`/`leases` is
enforced in Go in `validatedWorktreeCreateInputs`, exactly the pattern the
interrogations (16) and principals (23) migrations use) and performs **no
`ALTER`**, so `striatumd_rw` applies it cleanly. A role-guarded
`GRANT … TO striatumd_rw` block makes it usable under the two-role posture
(`pgtest` masks a missing grant, so the grant is mandatory). `LatestDaemonDBVersion`
is bumped `27 → 28`.

### Base staging mechanism (no `.git`, no `tar` binary)

`handlePlainDirWorkspaceCreate`:

1. Resolves the durable **tree** sha: `git rev-parse --verify <run-branch>^{tree}`
   (`gitRevParseTree`). Recording the *tree* sha (not the commit) is what lets a
   retrospective reconstruct the exact diff as **base tree XOR published tree**.
2. Stages the content with `git archive --format=tar <tree>` extracted
   **in-process** via Go's `archive/tar` (`extractGitArchive` →
   `extractTarStream`). This produces plain files with **no `.git`**, needs **no
   external `tar` binary**, and confines every entry to the workspace (rejecting
   absolute / `..` / escaping paths defensively even though the archive is
   daemon-controlled). The full archive is read before extraction so the `git`
   child cannot deadlock on a stalled pipe.
3. Re-validates under a transaction (`validatedWorktreeCreateInputs` re-locks the
   job/lease and re-asserts repo-write + `per_job` isolation), refuses a second
   active workspace (`activeWorkspaceForJob` + the partial unique index), inserts
   the `job_workspaces` row with `workspace_kind='plain_dir'` and the recorded
   `base_tree_sha`, and appends a `workspace.created` event — all **before the
   lane starts**.

`worktree_isolation: per_job` remains the **logical** isolation contract;
`workspace_kind` is the **physical realization** (legacy git worktree vs plain
dir). The opt-in is an RPC param to `workspace.create` (`worktree.create`):
omitted/`per_job` → unchanged legacy path; `plain_dir` → the new path; any other
value → `schema_invalid` (closed enum).

`worktreeTarget` was refactored into a shared `confinedScratchTarget(repoRoot,
path, subdir, label)`; `worktreeTarget` and the new `workspaceTarget` are thin
wrappers, so the existing `.striatum/worktrees` confinement messages/behavior are
byte-identical and the new `.striatum/workspaces` confinement reuses the same
symlink/traversal safety.

---

## 2. Edits applied

New files:
- **`go/pkg/db/sql/0028_plain_dir_job_workspaces.sql`** — the `job_workspaces`
  CREATE-TABLE migration + indexes + role-guarded grant (ownership-safe: no
  `ALTER`, no FK).

Modified:
- **`go/pkg/db/migrations.go`** — `LatestDaemonDBVersion 27 → 28` + the version-28
  label.
- **`go/pkg/db/write_authority_inventory.go`** — `"job_workspaces": ClassRuntimeDML`
  (required by `TestWriteAuthorityInventoryComplete`).
- **`go/pkg/db/read_authority_inventory.go`** — `"job_workspaces": ReadClassRuntimeSensitive`
  (required by `TestReadAuthorityInventoryComplete`).
- **`go/pkg/db/migrations_test.go`** — `TestMigrationTwentyEightJobWorkspacesIsOwnershipSafe`
  (asserts the new table + base-tree column + grant, and no `ALTER`/`DROP`/FK).
- **`go/pkg/mutations/worktree.go`**
  - constants: `workspacesSubdir`, `workspaceKindPerJob`, `workspaceKindPlainDir`.
  - `HandleWorktreeCreate`: parse + validate the `workspace_kind` enum; dispatch
    to the plain-dir path when `plain_dir`, otherwise the unchanged git path.
  - `handlePlainDirWorkspaceCreate` (new): the staging + record flow above.
  - `activeWorkspaceForJob` (new): single-active-workspace guard.
  - `stagePlainDirBaseContent`, `gitRevParseTree`, `extractGitArchive`,
    `extractTarStream`, `gitArchiveErrorMessage` (new): the `git archive`→tar→plain
    dir staging helpers.
  - `worktreeTarget` refactored to `confinedScratchTarget`; new `workspaceTarget`.
- **`go/pkg/mutations/worktree_test.go`** — tests (see §3).

Out of this job's write scope (`go/`, `docs/rfcs/`, `docs/decisions/`,
`docs/campaigns/rfc-0127/`): `docs/reference/spec.md`, `docs/reference/todo.md`,
and `CHANGELOG.md` are **not** updated here. They are the right home for the
land-on-`main` step (the apply job / merge), since P0 adds an opt-in path that is
not yet the default and not yet wired into any workflow. No new decision entry is
needed: **D195** already accepts RFC 0127 and this is its P0 implementation; the
table choice follows the existing D173/D187 ownership precedent. The RFC doc
itself is frozen provenance and is left unedited.

---

## 3. Test obligations & the required P0 test

**Required P0 test — `TestHandleWorktreeCreateStagesPlainDirWorkspace`** (PG-gated,
`worktree_test.go`). Through the real handler: a job created with
`workspace_kind: plain_dir` gets

1. a directory under `.striatum/workspaces/<id>` with the **base content staged**
   (`seed.txt` present),
2. **no `.git`** in it,
3. `base_tree_sha` returned == `git rev-parse <run-branch>^{tree}`, and
4. a `job_workspaces` row recording `workspace_kind=plain_dir`, that
   `base_tree_sha`, and `state=active`,
5. **and zero `job_worktrees` rows** (the opt-in does not touch the legacy table).

Supporting tests:
- `TestStagePlainDirBaseContentStagesTreeWithoutGit` (hermetic, no DB) — proves the
  `git archive`→tar staging + tree-sha return + `.git`-free directory directly,
  including a nested subdirectory.
- `TestWorkspaceTargetConfinesPathToStateWorkspaces` (hermetic) — `.striatum/workspaces`
  confinement, rejecting repo-escaping and `.striatum/worktrees` paths.
- `TestHandleWorktreeCreateRejectsUnknownWorkspaceKind` (hermetic) — closed enum.
- `TestHandleWorktreeCreateDefaultStillCreatesGitWorktree` (PG-gated) — the
  **legacy default is unchanged**: omitting `workspace_kind` still creates a real
  git worktree (`.git` pointer present) and records one active `job_worktrees` row.
- `TestMigrationTwentyEightJobWorkspacesIsOwnershipSafe` (hermetic) — the migration
  is ownership-safe.

---

## 4. Build & test results (verified by author-agent-002)

Environment: this lane runs as the sandboxed OS user `striatum-lane`, which
PostgreSQL's `pg_hba.conf` rejects and which cannot read the live daemon's DSN, so
`STRIATUM_PG_TEST_URL` is unset and the live daemon DB is unreachable from the
lane. To execute the PG-gated tests **for real** I stood up a **private throwaway
PostgreSQL 16.14 cluster** (`/usr/lib/postgresql/16/bin/initdb` + `pg_ctl`, trust
auth, bound `127.0.0.1:5433`, datadir under `/tmp`), pointed `STRIATUM_PG_TEST_URL`
at it, ran the targeted suites, then `pg_ctl -m immediate stop` + `rm -rf` the
datadir. Nothing persistent; the live daemon DB on `:5432` was never touched.

Commands run from the feature-branch worktree and their results:

- `make -C go build` — **PASS** (all three binaries: `striatum`, `striatumd`,
  `striatum-supervisor-helper`).
- `go vet ./pkg/db/ ./pkg/mutations/` — **PASS** (the `govet` linter the
  `make lint` suite enables).
- `gofmt -l` on every changed `.go` file — **clean** (no reformatting needed).
- `pkg/db` (throwaway PG): `TestMigration*` incl.
  `TestMigrationTwentyEightJobWorkspacesIsOwnershipSafe`, the live migration
  integration test asserting schema version `== 28`, `TestReadAuthorityInventoryComplete`,
  `TestWriteAuthorityInventoryComplete`, `TestApplyMigrations*`, `TestOwnerBundle*`
  — **PASS** (`ok ... 7.25s`).
- `pkg/mutations` (throwaway PG): the **required P0 test**
  `TestHandleWorktreeCreateStagesPlainDirWorkspace` **PASS** (1.88s) and
  `TestHandleWorktreeCreateDefaultStillCreatesGitWorktree` **PASS** (1.51s), plus
  the hermetic `TestWorkspaceTargetConfinesPathToStateWorkspaces`,
  `TestStagePlainDirBaseContentStagesTreeWithoutGit`,
  `TestHandleWorktreeCreateRejectsUnknownWorkspaceKind` — **PASS**.
- `pkg/mutations` regression (throwaway PG): the broader
  `Worktree|Anchor|Release|SweepRunPins|Porter|Reseal|Workspace|PlainDir` cluster
  — **PASS** (`ok ... 48.0s`), confirming the `confinedScratchTarget` refactor did
  not regress the legacy git-worktree anchor/release/gc/porter paths.

**Not run in the lane sandbox (operator/CI to confirm):**
- `golangci-lint` / `make -C go lint` — the pinned `golangci-lint v2.12.2` binary
  is not installed in the sandbox and `go install` has no network here, so the
  `staticcheck`/`errcheck`/`ineffassign` linters were not exercised (only `govet`
  via `go vet`, which is clean). The change adds one deliberate
  `//nolint:gosec` on the `io.Copy` of the daemon-controlled `git archive` of a
  pinned tree in `extractTarStream`.
- `make -C go test` full suite (only the worktree/workspace/migration-adjacent
  subsets above were run, against the throwaway cluster).

This was **not merged to `main`**; it sits on the feature branch
`striatum/rfc-0127-p0-plain-dir-workspace` for review.

### Follow-ups for P1+ (explicitly deferred)

- Wire `plain_dir` workspaces into `work.complete`: the daemon-side change-set diff
  (staged dir vs `base_tree_sha`) + write-scope enforcement, replacing the
  `git status` read; and the porter commit (`commitArtifactBodyToRunRef`) sourcing
  the plain dir + `refs/striatum/` anchoring. Until then a `plain_dir` job has no
  completion/release/gc path — P0 only creates the workspace.
- Path-restrict staging to the job's readable/editable paths (P0 stages the full
  base tree, which is a correct superset of the "before" state).
- `docs/reference/spec.md` / `docs/reference/todo.md` / `CHANGELOG.md` updates at
  land time.
