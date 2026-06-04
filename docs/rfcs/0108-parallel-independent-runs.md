# RFC 0108: Parallel independent runs on one repo — many operators, one repository, no collisions

Status: proposed
Date: 2026-06-02
author: proposer-claude-opus-4-8-001
Context: RFC 0104 (per-run serialization invariant), RFC 0008 (worktree isolation for parallel jobs), RFC 0067 (optional git/PR integration), RFC 0107 (multi-principal trust model), RFC 0105 (unattended-reliability harness), RFC 0028/0032 (multi-repository control plane), RFC 0102 (operator attention economy); `go/pkg/mutations/run.go`, `claim.go`, `worktree.go`, `write_scope_guard.go`, `go/pkg/apply`, `go/pkg/reads/dashboard_all.go`, `go/pkg/workflowauthoring/{workflow.go,lint.go}`.

## Problem

The mission is **showerthought → product in yolo mode**. A natural amplifier is
**multiple operators working the same repository in parallel** — several agents
each building a different thing on one codebase at once. This RFC scopes that to
**independent parallel runs**: N operators each drive their own run/feature on
the same repo simultaneously, the system **prevents collisions** and **integrates
each cleanly**. It explicitly excludes (separate, harder, or wishful problems):
coordinated decomposition of one goal across operators, co-driving a single run,
and lock-free parallel writes to one branch with auto-merge.

This is **not wishful — ~70% of the substrate already exists:**

- **Cross-run concurrency is lock-free by design** once RFC 0104 lands: the
  per-run advisory lock serializes *within* a run, never across runs.
- **Branch-per-run already exists**: `HandleBranchConfirm` (`run.go:314`) +
  `gitCreateOrCheckoutBranch` (`run.go:356`) pin each run to a branch before
  start (`run.go:55` requires it).
- **Per-job worktree isolation already exists** (RFC 0008): the lane field
  `worktree_isolation: off|per_job` (`workflowauthoring/workflow.go:28`),
  `laneWorktreeIsolation`/`worktreeRequired` (`claim.go:346`),
  `HandleWorktreeCreate` running `git worktree add --detach` (`worktree.go:37,84`),
  guarded by `uq_active_job_worktree`.
- The lint rule `repo_write_without_worktree_isolation` (`lint.go:249`) already
  warns *"parallel or revision work can collide in the main worktree."*
- **No "one active run per repo" constraint** exists — concurrent runs on a repo
  are already schema-legal.

### The three real walls (named, not wished away)

1. **The per-repo event hash chain is a single linear chain.** Every mutation's
   `appendEvent` locks `repo_event_chain_heads` `FOR UPDATE` per repository
   (`mutations.go:897`), so all runs on a repo serialize there. This is a brief
   row-lock — fine for a realistic handful of parallel runs — **and it is a
   feature**: it yields one total, tamper-evident order of everything that
   happened on the repo. This RFC characterizes it and does **not** remove it.
2. **Git integration is serial by necessity.** Merging N run-branches into one
   mainline is the VCS merge problem. The honest answer is per-run branches plus a
   **serialized, gated** merge that surfaces conflicts and never auto-resolves.
3. **Collisions must be detected, not coordinated.** "Independent" means the work
   is not divided for the operators; but two runs silently writing the same files
   or branch must be surfaced up front, not discovered at merge.

So the work is mostly **enabling and hardening what exists**, plus a small new
collision-detection and serialized-integration layer.

## Proposal — five phases

### Phase 1 — Prove the substrate composes (a gate, no new behavior)

Extend the RFC 0105 harness from multi-*lane* to multi-*run*: drive ≥2 runs
concurrently on one repo, each on its own branch with per-job worktrees, and
assert both complete + integrate, **no deadlock/corruption**, and the per-repo
audit chain stays **linear and verifying** (the `daemon doctor` chain check).
This proves RFC 0104 + `branch.confirm` + worktrees compose before any policy is
added. Files: `go/pkg/adapterconformance/*` (new multi-run fixture).

**Landed** (`go/pkg/adapterconformance/multirun_test.go`):
`TestMultiRunConcurrentComposeNoDeadlock` drives batches of independent run
lifecycles concurrently on one repo and asserts all complete, no `40P01`, and
the per-repo event chain stays linear + verifying;
`TestMultiRunPerJobWorktreesComposeIsolated` proves two concurrent repo-write
runs each get their own confirmed branch + detached worktree;
`TestMultiRunSharedCheckoutTurnsRed` is the isolation-off red boundary. The gate
records one substrate property worth keeping in view for Phase 2: the per-repo
`repo_event_chain_heads` `FOR UPDATE` serializes appenders by locking that
singleton row, but a `SELECT ... FOR UPDATE` over **zero** rows locks nothing,
so the chain's *genesis* must be appended serially (it always is in production —
repo registration + `run prepare`/`start` precede any concurrent claim). The
fixture models that with a serial warmup run; if Phase 2/3 ever admit
concurrency against a chain that is still empty, that genesis would need an
explicit per-repo serialization point.

### Phase 2 — Isolation by default under concurrency

When more than one run is active on a repo, **require** per-run branch + per-job
worktree isolation for repo-write jobs instead of leaving it opt-in: promote the
`repo_write_without_worktree_isolation` lint warning to an enforced precondition
at `run.start`/`run.prepare` when a sibling run is already active (or auto-set
`worktree_isolation: per_job`). No two concurrent runs ever share the main
checkout. Files: `run.go` (HandleRunStart precondition), `claim.go`
(`laneWorktreeIsolation`), reusing `HandleWorktreeCreate`.

**Landed** (`go/pkg/mutations/run.go`): `HandleRunStart` now enforces the
precondition. When the run transitions `ready -> running` and **another run on
the same repo is already `running`**, the start is **refused** if the run has a
repo-write job on a lane without `worktree_isolation: per_job` — the exact job
that would scribble the shared main checkout. The refusal carries the stable
error code **`concurrent_run_isolation_required`** (RFC 0111 catalog) whose
suggestion names the fix: set `worktree_isolation: per_job` on the repo-write
lane (each run then gets its own detached worktree), or wait for the active run
to finish. `enforceConcurrentRunIsolation` reuses the *exact* isolation decision
buildPacket/`HandleWorktreeCreate` already make (`laneWorktreeIsolation` over the
frozen snapshot × `isRepoWrite` over the job's stored write_scope), so the gate
and the runtime never disagree.

The precondition is **race-free**: `HandleRunStart` takes a per-repository
advisory lock (`lockRepo`) as the first statement of its transaction, so
concurrent starts serialize and two runs can never both observe "no sibling
active" and race onto the shared checkout. `lockRepo` is strictly wider than
RFC 0104's per-(repo,run) `lockRun`, and no mutation takes both, so they cannot
form a lock-ordering cycle. The single-run case (no sibling active),
per_job-isolated runs, and document-only runs all start unaffected — the gate
bites only the genuine shared-checkout hazard. Gates
(`go/pkg/adapterconformance/multirun_test.go`):
`TestMultiRunIsolationRequiredWhenSiblingActive` (sequential: first unisolated
run starts alone; a second is refused while it is active; a per_job run and a
document-only run start fine beside it) and
`TestMultiRunConcurrentUnisolatedStartsResolveToOne` (race: N unisolated starts
fired concurrently resolve to **exactly one** `running` + N−1
`concurrent_run_isolation_required`, no `40P01`, chain stays linear).

### Phase 3 — Cross-run collision detection at start

At `run.prepare`/`run.start`, detect overlap with other *active* runs on the
repo: same target branch, or overlapping declared `write_scope` path prefixes.
Warn, or refuse without an explicit `--allow-overlap`. Lift the existing
per-workflow write-scope reasoning (`workflowauthoring/lint.go` write-isolation
rules, `write_scope_guard.go`) to a **cross-run** check keyed on the set of
active runs. Files: `run.go`, a new `reads`-side "active runs + their
branch/write-scope" query, `write_scope_guard.go`.

**Landed** (`go/pkg/mutations/run.go`): `HandleRunStart` runs
`evaluateCrossRunCollision` right after the Phase 2 isolation check, inside the
same `lockRepo`-held transaction, so the active-runs snapshot cannot race a
concurrent start. It draws the line between a **definite** and a **potential**
collision:

- **Same target branch → REFUSE** with the stable code `cross_run_collision`
  (RFC 0111 catalog) unless the operator passes `--allow-overlap`. Two runs
  cannot share one git branch — they would clobber each other and collide at
  integration — so this is a hard stop. (The CLI flag flows through the generic
  param parser as `allow_overlap`; no route change was needed.)
- **Overlapping repo-write `allowed_paths` → WARN, non-blocking.** On distinct
  branches with per_job worktrees the runs do not collide at write time; their
  changes only risk a *merge* conflict at integration (the VCS problem Phase 4
  serializes). So the overlap is surfaced as a `warnings[]` entry on the
  `run.start` result (the RFC 0102 attention principle) naming the colliding run
  and path, not a refusal. `--allow-overlap` suppresses the warnings too.

Path overlap reuses the `write_scope_guard.go` normalization
(`normalizeScopePath`) and is bidirectional prefix containment (`a==b`, `a` under
`b`, `b` under `a`, or either is the repo root `.`). Gates
(`go/pkg/adapterconformance/multirun_test.go`):
`TestMultiRunSameBranchRefusedWhileSiblingActive` (same-branch refused;
`--allow-overlap` overrides; distinct branch starts free) and
`TestMultiRunWriteScopeOverlapWarns` (a prefix-overlapping run starts with a
warning naming the sibling; a disjoint run starts clean).

### Phase 4 — Serialized, gated integration

Each run lands on its own branch through the existing `git.commit_apply` /
sealed-apply path (`go/pkg/apply`); integrating to mainline is an explicit
**serialized** step (one merge at a time, ordered by the event chain) that
**surfaces conflicts and never auto-resolves**. Lean on RFC 0067 (optional
git/PR) so each run can become a branch/PR a maintainer or an integration gate
merges. Files: `go/pkg/apply`, `run.go` (an integration verb/gate), the RFC 0067
surface.

**Landed** (`go/pkg/mutations/integrate.go`, method `run.integrate`): the
serialized, gated integration step. It merges a **completed** run's branch into a
target mainline branch (`--into`), **one run at a time per repository** —
serialized on the same per-repo `lockRepo` the Phase 2/3 gates take, held across
the merge so a concurrent integration cannot interleave — and **never
auto-resolves**: a conflicting merge is refused with `merge_conflict` (RFC 0111
catalog) naming the conflicting paths, leaving mainline untouched.

The merge is **pure git plumbing** and never mutates a working tree or index:
`git merge-tree --write-tree <into> <run-branch>` is a read-only 3-way merge
simulation that both detects conflicts and produces the merged tree; on a clean
result `git commit-tree` builds the merge commit (two parents, a dedicated
`striatum-integrator` identity) and a compare-and-swap `git update-ref` advances
the mainline ref to it — exactly as a fast-forwarding push would. The operator's
checkout (whatever branch it is on, in whatever worktree) is never touched; only
the mainline ref moves, which is what makes integration safe to run against a
live repository with other runs' per-job worktrees checked out. The integration
is recorded as a `run.integrated` event **before** the ref advance (git is not
transactional with the DB; append-then-update-ref keeps a failure from leaving
mainline advanced-but-unrecorded), and re-integrating a run into the same target
is an idempotent no-op. No schema migration was needed — integration lives in the
event chain. Gate (`go/pkg/adapterconformance/multirun_test.go`):
`TestMultiRunSerializedIntegrationMergesCleanAndSurfacesConflict` (two runs
changing different files integrate serially and mainline carries both; a
conflicting pair surfaces `merge_conflict` and mainline is byte-for-byte
unchanged; re-integration is a no-op). The maintainer/PR merge surface (RFC 0067)
and populating the Phase 5 `integration_status` from the `run.integrated` event
remain small follow-ups.

### Phase 5 — Repo concurrency view + attribution

Extend `dashboard.all` (`reads/dashboard_all.go`) to a repo-scoped
**concurrent-runs view**: per run, the operator/principal, branch, write-scope,
lane states, and integration status — so operators and the maintainer see the
parallel fan-out on one surface (the RFC 0102 attention principle). Per-run
operator/principal attribution leans on RFC 0107. Files: `reads/dashboard_all.go`,
`reads/status.go`, the web view.

**Landed** (`go/pkg/reads/concurrent_runs.go`): `repoConcurrentRuns` returns,
for every `running` run on the repo (the live parallel fan-out), a view row with
its **branch**, the **repo-write paths** it intends to touch (the union of its
repo-write jobs' `allowed_paths`), its **lane sessions** (operator label, role,
lane, state), and the **live collisions** with other active runs — a shared
branch (`kind:"branch"`) or an overlapping repo-write scope (`kind:"write_scope"`).
It is the read-side reflection of exactly what the Phase 2/3 run.start gates
enforce: the collision computation reuses the same branch-equality +
bidirectional path-prefix-overlap logic, so the dashboard and the gate agree on
what collides. `integration_status` is a `"in_flight"` placeholder until Phase 4
populates real integration state. Surfaced as `concurrent_runs` on both
`dashboard.all` (per repository) and `status` (repo-level, independent of an
optional `run_id` filter — an operator viewing one run still sees the whole
fan-out). SELECT-only, like the rest of the projection. Gate
(`go/pkg/reads/concurrent_runs_test.go`):
`TestRepoConcurrentRunsSurfacesFanOutAndCollisions` (four runs — a shared branch,
an overlapping scope, and a disjoint document-only run — resolve to exactly the
expected per-run collision sets) and `TestRepoConcurrentRunsExcludesTerminalRuns`
(only `running` runs appear). The web view that renders this surface is a
follow-up.

## Acceptance

- **Phase 1 gate:** the multi-run harness fixture runs in CI — 2+ concurrent runs
  on one repo, isolated branches/worktrees, both complete + integrate, no
  `40P01`, audit chain linear + verifying. A deliberately-induced shared checkout
  (isolation off) turns it red.
- **Phase 2/3:** `run.start` refuses (or warns + `--allow-overlap`) a second
  concurrent run sharing a branch or overlapping write-scope; tested in
  `mutations` + `workflowauthoring`.
- **Phase 4:** two run-branches integrate via the serialized gate; a conflicting
  pair surfaces the conflict rather than corrupting mainline.
- **Phase 5:** `dashboard.all`/`status` shows all active runs on a repo with
  operator/branch/integration columns.
- **End-to-end:** two operators' runs driven concurrently on one repo,
  unattended, both reach integration without collision — "two showerthoughts →
  two product branches, at once."

## Non-goals

- **No coordinated work decomposition** — operators do not share a work pool or
  divide one goal here; that is a separate, harder RFC.
- **No auto-merge of conflicting parallel changes** — integration is serialized
  and gated; conflicts surface to a human/operator.
- **No change to the per-repo linear event chain** — it remains the single
  tamper-evident total order (the provenance backbone).
- No new hosted service / SaaS / external persistence (D094/D005/D028 intact).

## Relationship to prior RFCs

- **RFC 0104** (lock-free cross-run) is the hard prerequisite — Phase 1 cannot be
  proven without it.
- **RFC 0008** (worktree isolation) and **RFC 0067** (git/PR) supply the
  per-run-branch + per-job-worktree substrate Phases 2 and 4 build on.
- **RFC 0105** (reliability harness) is extended to multi-run for Phase 1's gate.
- **RFC 0107** (multi-principal) supplies the operator/principal identity Phase 5
  attributes runs to; this RFC is the *concurrency/isolation* complement to that
  RFC's *identity* model.
- **RFC 0028/0032** (multi-repository control plane) already coordinate *across*
  repos; this RFC adds safe parallelism *within* one repo.
