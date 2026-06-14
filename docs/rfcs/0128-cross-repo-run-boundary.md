# RFC 0128: Cross-repo run boundary — keep runs single-repo, decompose the rest

Status: proposed
Date: 2026-06-14
author: proposer-claude-opus-4-8-001
Context: GH #280 (cross-repo workflow jobs need lane write provisioning for secondary
repos), RFC 0108 (parallel independent runs), RFC 0125 / D192 (#280 documented as an
operator boundary in `docs/how-to/lane-sandbox.md`), the product boundary in
`docs/reference/spec.md` ("a generic orchestration tool for **target repositories**")
and `AGENTS.md` ("Do not introduce hosted services, cloud APIs, ... or external
persistence without an explicit product decision"; "no hosted/multi-tenant anything").
Driven by the Hippo S12 cross-repo hardening job (one run was asked to write three
repos: `hippo`, `striatum`, `striatum-warmtier`) and the `/adhd` divergent pass, in
which nearly every cognitive frame — even when asked to *support* multi-repo —
converged on **decomposition into coordinated single-repo runs** rather than a
cross-repo transaction.

## Summary

A Striatum run is scoped to **one** registered target repository: the run branch, the
durable `refs/striatum/` anchors, the daemon-owned PostgreSQL state, and the per-repo
lane ACLs are all per-repo. The Hippo S12 job needed one run to write three repos;
because cross-repo touch points live only in free-text prompts, the daemon could not
provision or preflight secondary-repo write access, and **the lane silently narrowed
its scope** to the one repo it could write (#280's actual pain).

This RFC makes a **product decision**: the **single-repo run is the invariant**, and
cross-repo outcomes are achieved by **decomposition** — a coordinator that spawns one
single-repo run per repo, wired by typed cross-run artifact handoffs under a shared
campaign id. It ships the missing **fail-fast guardrail** (the daemon refuses, at
prepare/validate, a job whose declared write-scope or prompt reaches outside its
registered repo — instead of letting the lane silently narrow), and it **declines**
first-class multi-repo atomic writes, recording that design as an explicitly-deferred
option behind a future product trigger.

## Problem

The concrete #280 failure is **silent scope narrowing**, not a missing transaction: a
cross-repo prompt produced a run that wrote one repo and quietly dropped the other two,
with the gap invisible until a retrospective. The daemon cannot help because:

- Cross-repo intent is **undeclared** — it is free text in the prompt, so the daemon
  cannot provision lane ACLs for secondary repos or preflight them.
- Provenance, durability, and authority are **per-repo by construction** (one run
  branch, one repo's `refs/striatum/`, one repo's PG-scoped state, one repo's lane
  ACLs). A single run writing N repos would need per-repo tokens, per-repo provenance,
  and **cross-repo atomicity** (what if 2 of 3 commit and the run fails) — a large new
  surface.
- First-class multi-repo writes brush the product boundary: Striatum is a *generic
  orchestration tool for target repositories*, and its standing anti-bets forbid scope
  creep without an explicit decision.

The Hippo case itself shows the decomposition is adequate: the operator handled the
secondary repos out of band and the run still produced correct, durable per-repo work —
the failure was the **silence**, not the inability to atomically write three repos.

## Decision

1. **The single-repo run is the invariant.** A run writes exactly its one registered
   target repository.
2. **Fail fast, never silently narrow.** The daemon refuses cross-repo reach at the
   earliest possible point with an actionable error.
3. **Decompose for cross-repo outcomes.** A coordinator spawns one single-repo run per
   repo, wired by typed cross-run artifact handoffs under a shared campaign id; each
   run keeps its own provenance chain.
4. **Decline first-class multi-repo atomic writes** for now; record the design as a
   deferred option (below) behind an explicit product trigger.

## Design

### Fail-fast single-repo guardrail (the must-have, closes #280)

- **Validate-time lint.** `run validate` / `workflow lint` scans declared
  `write_scope.allowed_paths` across all lanes and **fails** (structured error, exit 7)
  if any path resolves outside the run's registered repo root. It additionally **warns**
  when a free-text prompt field contains a path token or `org/repo` slug that doesn't
  match the registered target — turning today's hidden cross-repo intent into a
  surfaced signal *before* a lane spawns. (The `/adhd` regulator + inversion frames both
  landed here.)
- **Dispatch-time refusal.** A job whose resolved write reaches outside its repo root
  enters a `scope_violation` terminal state with the offending path — the lane can no
  longer silently narrow. This is the daemon-side counterpart to the documented
  operator boundary in `lane-sandbox.md`.
- **Read-only artifact federation (the common "I need data from repo B" case).** A
  secondary repo may be registered as a **read-only input source**: the daemon serves
  its `refs/striatum/` provenance / artifacts as named inputs to the primary run's jobs
  **without** granting lane write ACLs. This satisfies the frequent cross-repo *read*
  need while keeping the run single-repo-write.

### Decomposition for genuine cross-repo *writes*

- **Coordinator + satellite runs.** A coordinator (a single-repo run on a meta/primary
  repo, or an operator) spawns one single-repo run per target repo via the existing run
  machinery, in a declared dependency order. Each satellite run is autonomous, with its
  own run branch and provenance.
- **Typed cross-run handoff.** A satellite run publishes a structured handoff artifact
  (commit SHA + summary) that the daemon records under a shared **campaign id** stamped
  into each run's provenance; a dependent run reads it as a named input. (The `/adhd`
  logistics "distribution-center decomposition" + markets "correspondent repo" + the
  10-year-old's "todo card in the other folder's inbox" all converge here.) This reuses
  artifacts + run spawn; it adds no shared transaction and no new authoritative state
  beyond a campaign-id annotation.
- **No cross-repo atomicity is promised.** Each repo's run lands or fails
  independently; partial completion is visible (per-run state + the campaign ledger),
  not a silent half-write. This is the honest trade: coordination without a distributed
  transaction across sovereign git repos.

### Declined for now: first-class multi-repo atomic writes

Recorded so a future product decision has the design ready, **not** to be built without
that decision. If a strong case emerges (many real cross-repo jobs, atomicity genuinely
required), the shape the `/adhd` regulator/logistics/markets frames converged on is:

- A declared `secondary_repos` manifest in the workflow fixture (each a **registered**
  repo UUID + per-repo write-scope) — preflighted at run start, refused if any repo is
  unregistered or unprovisionable.
- **Consent lives in the secondary repo**, not the initiating run: a `peer_write_acl`
  row in the secondary repo's own daemon state names the run pattern allowed to write
  it — so a repo cannot be written by an outside run unless it pre-declared willingness
  (unforgeable from outside).
- Per-repo capability tokens minted at run.start; per-repo `refs/striatum/` provenance
  on each repo (self-contained, auditable from any single repo).
- **Saga atomicity:** each repo's changes land on a per-repo staging branch; a
  coordinator settles all (fast-forward) on a unanimous signal, or emits a durable
  **partial-commit incident** record naming which repos succeeded — rollback is a
  staging-branch deletion / prepared compensation branch.

This is deferred because the surface (per-repo consent, distributed settlement,
partial-failure incidents) is large and the product does not yet clearly need it.

## Non-goals / anti-bets

- **No hosted/multi-tenant coordination service** (standing anti-bet). The coordinator
  is a run + the campaign-id annotation, not a new service.
- **No distributed transaction across git repos** in the shipped design — atomicity is
  explicitly not promised; decomposition makes partial states visible instead.
- **No new persistence** beyond the campaign-id annotation and (if the deferred option
  is ever taken) the per-repo manifest/consent rows.

## Phasing

| Phase | Scope |
| --- | --- |
| **P0** | Validate-time cross-repo lint (write-scope outside repo root ⇒ fail; foreign prompt slug ⇒ warn). Closes the legible half of #280. |
| **P1** | Dispatch-time `scope_violation` terminal state (no silent narrowing). |
| **P2** | Read-only artifact federation (secondary repo as a registered read-only input source). |
| **P3** | Decomposition ergonomics: campaign-id annotation + typed cross-run handoff artifact + a `to-issues`-style helper to split a cross-repo ask into coordinated single-repo runs. |
| **(deferred)** | First-class multi-repo manifest + per-repo consent + saga — only behind an explicit product decision. |

## Test obligations

1. A workflow whose lane write-scope reaches outside the registered repo **fails**
   `run validate` (exit 7) with the offending path.
2. A job that resolves a write outside its repo root enters `scope_violation`, not a
   silent narrow (pgtest/harness).
3. A secondary repo registered as a read-only input source serves its artifacts to a
   primary run with **no** lane write ACL granted.
4. The deferred multi-repo path has no code surface until the product trigger fires
   (a guard test asserting no `secondary_repos` manifest is honored).

## Open question (the product decision this RFC frames)

Is single-run multi-target-repo writing ever in scope? This RFC recommends **no** —
keep runs single-repo, ship the guardrail + decomposition — and asks the maintainer to
ratify that, or to set the explicit trigger under which the deferred first-class design
would be built.
