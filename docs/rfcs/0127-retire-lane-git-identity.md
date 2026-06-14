# RFC 0127: Retire the lane git identity — the lane as a pure byte producer

Status: accepted (D195)
Date: 2026-06-14
author: proposer-claude-opus-4-8-001
Context: RFC 0125 / D192 (durable gate artifact provenance — the daemon-as-porter
already commits every published lane artifact as the operator), RFC 0103 (lane
sandbox / least-privilege lane OS user), RFC 0096 (supervised-lane trust boundary),
RFC 0008 (per-job worktree isolation), RFC 0117 / D176+D178 (worktree/branch
ref-safety, `refs/striatum/` anchors, `worktree gc`). Identified as the RFC 0125
P2-2 end-state and the `/adhd` provocation ("if the daemon retires the lane's git
identity, the per-job worktree stops being a git worktree"). Grounded reads at `main`:
`go/pkg/mutations/worktree.go` (`HandleWorktreeCreate` does `git worktree add --detach`,
`anchorWorktreeCommitStack` reads the worktree HEAD, `worktreeTarget`),
`go/pkg/mutations/artifact_durability.go` (`publishedArtifactDurabilityProblems` —
`git show HEAD:<path>`, the porter `commitWorktreeArtifacts` from RFC 0125),
`go/pkg/mutations/artifact.go` (`publishArtifact` reads the worktree file or the
`body_base64` envelope), the write-scope guard (`enforceWriteScopeClean`, the
`guard.go` forbidden-before-ignored ordering), `docs/how-to/lane-sandbox.md` (the host
ACL provisioning the lane needs: traversal, `safe.directory`, per-job-worktree write).

## Summary

Today every per-job workspace is a **git worktree** the lane writes into, and the lane
needs git provisioning to use it: `safe.directory` config (the repo is operator-owned,
so git refuses it as "dubious ownership"), write ACLs on `.striatum/worktrees/<id>`,
and an understanding of detached HEAD. After RFC 0125, the **daemon** commits every
published artifact (the porter) — the lane never needs to commit. This RFC removes the
remaining git surface from the lane entirely: the per-job workspace becomes a **plain
daemon-owned directory** (not a git worktree), the daemon stages the base content into
it, the lane reads/writes plain files (or publishes bodies via the MCP envelope), and
the daemon owns **all** git — staging, change detection, write-scope enforcement,
commit, and anchoring.

The lane becomes a **pure byte producer**. "Detached HEAD", ".gitignore on a declared
path", "lane ACL on `.git`", "git dubious-ownership", and "the lane corrupting the
index/reflog/hooks" all stop being concepts, dissolving an entire failure class that
RFC 0125 had to *remediate* at the durability boundary.

## Problem

The git worktree leaks git into the lane sandbox in ways that are pure liability now
that the daemon does the committing:

- **Provisioning burden (#277/#280 class).** Each lane needs `safe.directory`,
  per-worktree write ACLs, and traversal — documented operator runbook steps that fail
  the run when missed.
- **Attack/corruption surface (RFC 0096).** A lane with a real git worktree can plant
  `.git/hooks/*` (executed on the next operator git op), poison the index so
  write-scope dirty checks read inconsistently (`git add -p` partial staging), read
  `.git/reflog`/`ORIG_HEAD` to reconstruct prior-job content, create
  `refs/striatum/*` to spoof daemon provenance pins, or `git push` to an attacker
  remote. None of these are needed for a byte producer.
- **The durability failure class RFC 0125 had to remediate.** Detached HEAD (#281),
  gitignored declared paths (#278), and a lane that cannot enter the worktree (#272)
  are all *artifacts of putting a git worktree in the lane's hands*. RFC 0125's porter
  fixes the symptoms; retiring the worktree removes the cause.
- **Write-scope enforcement reads `git status`** against a tree the lane controls,
  inheriting the `guard.go` forbidden-before-ignored ordering complexity.

## Goals

- The per-job workspace is a **plain daemon-owned directory**; the lane needs no git,
  no `.git`, no `safe.directory`, no ref access. The lane writes files and/or publishes
  bodies over the MCP envelope (`body_base64`, RFC 0125 #272).
- **Per-job isolation** (RFC 0008) holds via a filesystem boundary, not a git worktree.
- **Write-scope** is enforced **daemon-side** against an authoritative change set the
  lane cannot forge — no `git status` from a lane-controlled tree.
- **Durability/provenance** is unchanged in guarantee: the daemon commits the change
  set to the run branch and anchors a durable `refs/striatum/` pin (RFC 0117), via the
  RFC 0125 porter plumbing — now sourcing from a plain directory.
- **The base content is pinned before the lane starts**, so a retrospective
  reconstructs the exact diff (base tree sha XOR published tree) without trusting the
  lane.
- **Migration is opt-in and reversible** — the git-worktree path keeps working until
  the plain-dir path is proven, with no in-flight run broken.

## Design

### The plain per-job directory + base staging

`worktree.create` (renamed in spirit to `workspace.create`) stops running
`git worktree add --detach`. Instead the daemon creates a plain directory under
`.striatum/workspaces/<id>` and **stages the base content** into it from the run
branch HEAD — restricted to the paths the job may read/edit — using `git archive` /
`git checkout-index` into the plain dir (no `.git`). The daemon records the **base tree
sha** it staged in `job_workspaces` before the lane starts (the durable "before"
state). The lane reads and edits plain files; it never sees a git repo.

*Filesystem-engineer option (deferred optimization):* an `overlayfs`
(`lowerdir`=read-only base, `upperdir`=per-job tmpfs) makes the upper dir the exact
change set with zero scanning — but needs unprivileged user-namespace mounts or a
minimal setuid helper. V1 uses a plain staged directory + a daemon-side diff (below);
overlayfs/fanotify are a P2 performance optimization.

### Daemon-side change detection + write-scope (lane-untrusted)

At `work.complete`, the daemon (operator) computes the **change set** = the staged
directory diffed against the recorded base tree sha (it owns both). Write-scope is
enforced against that change set: every changed path must be within
`allowed_paths` and outside `forbidden_paths` — a single daemon-side check on an
authoritative diff, replacing `enforceWriteScopeClean`'s `git status` read and
sidestepping the `guard.go` ordering bug. A daemon-owned **sentinel** (written at job
start, unreadable/unwritable by the lane) provides an unforgeable lower-bound mtime as
a defense-in-depth oracle. The lane never had write authority to git; it cannot poison
the change set.

### Durability via the porter, from a plain dir

The daemon commits the change set to the run branch with the RFC 0125 porter plumbing
(`hash-object -w` → splice onto the run tip's tree → `commit-tree` → CAS `update-ref`),
sourcing blobs from the plain directory rather than a worktree — exactly the
`commitArtifactBodyToRunRef` helper RFC 0125 §Design specified (and which P0-1 there
left for this end-state). It anchors a `refs/striatum/<run>/<job>/<attempt>` pin
(RFC 0117). The base-tree-sha row + the commit give a retrospective the exact diff with
no lane-side git history to trust — collapsing the provenance trust surface from N lane
git identities to **one** daemon identity.

### The git-context question (the real tension)

Some agent CLIs use git for *navigation* (`git log`, `git blame`) even when they don't
commit. A pure byte-producer directory has no `.git`, so those commands fail. The RFC
must decide one of:

- **(A) No git in the lane.** Workflows that need history give the lane a read-only,
  separate **grounding** checkout (already a supported pattern, `r-x` only) — the
  *editable* workspace stays a plain dir. Recommended: it preserves the clean boundary.
- **(B) Read-only git view.** Stage the base as a real but **read-only** checkout
  (lane can `git log`/`blame`, cannot commit/push — enforced by ACL + a `pre-receive`
  deny). More compatible, larger surface; deferred unless a workflow needs it.

## Phasing

| Phase | Scope |
| --- | --- |
| **P0** | `workspace.create` plain-dir path (opt-in `workspace_kind: plain_dir` alongside the legacy `per_job` git worktree) + base-tree-sha recorded in `job_workspaces`. |
| **P1** | Daemon-side change-set diff + write-scope enforcement against it; porter commit (`commitArtifactBodyToRunRef`) sourcing the plain dir. |
| **P2** | overlayfs/fanotify change-capture optimization (optional); grounding-checkout pattern for git-context workflows (option A). |
| **P3** | Flip the default to plain-dir once proven across the adapter conformance matrix; retire the git-worktree lane path (keep `worktree gc` for legacy refs). |

## Test obligations

1. A repo-write job using the plain-dir workspace produces a durable run-branch commit
   + `refs/striatum/` pin with no git in the lane (pgtest + harness).
2. Write-scope refusal fires from the daemon-side diff for an out-of-scope changed path
   (no `git status`), with the same error shape as today.
3. The base-tree-sha + commit reconstruct the exact diff offline.
4. Adapter conformance (claude/codex/agy) completes a real job in a plain-dir workspace
   (the git-context tension is exercised here).
5. Legacy git-worktree workflows still run unchanged during migration.

## Anti-bets / open questions

- **Don't move the workspace into PostgreSQL.** The `/adhd` "workspace is a PG
  large-object store, no filesystem at all" idea is **rejected** — agent CLIs need a
  real filesystem to run; a virtual FS is a huge surface for no boundary gain. The
  plain directory + daemon-side diff is the minimal change.
- **Don't require privileged mounts in V1.** overlayfs/user-namespaces/FUSE are
  attractive (the upper dir *is* the diff) but add privilege + complexity; V1 stays a
  plain dir + diff, with mounts as a P2 optimization.
- The git-context decision (A vs B) is the load-bearing open question — recommend A
  (read-only grounding checkout) to keep the boundary clean; revisit if a flagship
  adapter genuinely needs in-workspace `git blame`.
- Interaction with RFC 0117 `worktree gc` and the existing `refs/striatum/` anchors
  must be pinned before P1 (the pins now anchor daemon-synthesized commits, not lane
  worktree HEADs — RFC 0125's `commitArtifactBodyToRunRef` already faces this).
