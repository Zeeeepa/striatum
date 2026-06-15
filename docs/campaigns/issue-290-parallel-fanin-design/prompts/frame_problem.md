Publish a concise problem brief framing the question below for the divergence
branches. State the question, constraints, goals, non-goals, and decision
criteria. Do NOT propose solutions — only frame the space.

## The question (GitHub issue #290, labeled ready-for-human)

When N parallel authors fan in to a single downstream job, only the **first**
completing author fast-forwards the run branch; the sibling authors' artifacts
strand in `refs/striatum/*` pins and the downstream worktrees (seeded from
run-branch HEAD via `git worktree add --detach <baseBranch>`) never see them.

**How should Striatum integrate the outputs of N parallel fan-in authors so
that every sibling's artifacts are reachable by the downstream job(s)?**

## What is true today (evidence from triage)

- FF-or-pin race: `anchorWorktreeCommitStack` (`go/pkg/mutations/worktree.go:994`)
  → `pinWorktreeCommitStack` (`worktree.go:1022`). First completer wins the
  fast-forward; siblings are pinned with no merge.
- Downstream worktrees seed only from run-branch HEAD (`worktree.go:118`); there
  is no logic that merges or reads the sibling pins.
- `run.integrate` (`integrate.go:27`) is RFC 0108 mainline integration —
  unrelated to fan-in.
- No fan-in barrier-detection concept and no conflict policy exist yet.

## Constraints (hard)

- Local-first; the daemon owns all git. Single run branch per run; the
  single-repo run invariant (RFC 0128 / D196) holds — do NOT reach for cross-repo.
- `refs/striatum/*` pins and the RFC 0125 daemon-as-porter durability boundary
  exist and must be preserved; provenance must stay durable and legible.
- Must compose with RFC 0117 (`worktree gc`), RFC 0125 (porter), and RFC 0127
  (plain-dir workspaces, daemon-owned diff). No hosted services, no new
  external persistence.

## Goals

- Every fan-in sibling's declared artifacts are visible to downstream jobs.
- A deterministic, legible conflict policy when siblings touch overlapping paths.
- Minimal new surface; reuses existing porter/anchor/pin machinery where possible.

## Non-goals

- Cross-repo writes (RFC 0128). Changing the single-run invariant.
- Real-time/streaming integration; semantic merge of code (a defined conflict
  disposition is enough).

## Decision criteria

- Correctness: zero stranded artifacts under any completion ordering.
- Conflict clarity: overlapping-path outcome is defined and explainable.
- Simplicity & timing: seed-time vs. a new integrate verb vs. complete-time
  merge — which barrier and which owner.
- Compatibility with RFC 0117 / 0125 / 0127 and the porter trust surface.

## Seed directions (from triage — to widen, not to anchor on)

1. **Seed-time merge**: downstream worktree creation merges all sibling pins.
2. **New fan-in integrate verb**: an explicit barrier that octopus/disjoint-merges
   pins before downstream is enqueued.
3. **Complete-time merge**: the last sibling to complete merges the others' pins
   into the run branch.
