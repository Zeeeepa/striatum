# RFC 0119 Holder Case — The Case For Acceptance

author: holder-unknown-model-001

Status of this document: holder case for the RFC 0119 ratification gate. It
argues that RFC 0119 (`docs/rfcs/0119-warm-tier-memory-boundary.md`) should be
accepted. It does **not** flip the RFC status — the adjudicator ledger decides
whether the gate clears after falsifiers challenge this claim.

## Claim

RFC 0119 should be accepted, and decision **D178** recorded, because it
authorizes the minimum boundary change needed to stand up a warm-tier memory
adjunct plus a striatum-native read-only hot tier **while preserving every
invariant `spec.md` currently pins**. Specifically, the proposal: (a) keeps the
three corpus invariants intact and generalizes the import ban to any external
consumer; (b) confines the new hot-tier read to **scaffold time only**, never
inside a state transition; and (c) preserves the durable-provenance boundary by
making only run *exhaust* and *unsynthesized intermediates* eviction-eligible,
with durable provenance staying canonical in git.

## Evidence

### 1. The three corpus invariants are preserved

The spec pins three non-negotiable invariants for the augmentation boundary
(`docs/reference/spec.md:1241-1245`, restated verbatim in the RFC at
`docs/rfcs/0119-warm-tier-memory-boundary.md:32-38`):

1. No `import engram` / `from engram` (no external-consumer import) in Striatum
   source.
2. No `memory.*` capability in the daemon method registry.
3. No state transition (`ack`, `publish-artifact`, `complete`, `verdict`,
   recovery, `run prepare`, `run start`, `corpus export`) that fails when an
   external memory consumer is missing, unreachable, or misconfigured.

RFC 0119 holds each one and, where it widens anything, widens *toward* stricter
coverage rather than relaxing it:

- **No external-consumer import.** The RFC explicitly generalizes invariant (1)
  from "no `import engram`" to "no `import` of any external memory consumer,
  including the warm-tier adjunct" (RFC `:40-41`). The warm tier (`hippo`/
  `fornix`) is reached **only via delivered inert content** through the existing
  pull-only `corpus export` path (RFC Goal 4, `:53-54`; Non-Goals `:61-62`).
  Nothing in the daemon links or calls a consumer client.

- **No `memory.*` capability.** The hot-tier read is named `recall.*` (or a read
  on an existing listing surface), explicitly **not** `memory.*` (RFC Non-Goals
  `:59-61`; D4 `:102-108`). D4 commits to an authority-matrix row with capability
  `read` / `single_repo` and a **guardrail test asserting no `memory.*` ever
  enters the registry** (RFC `:104-108`), which is the same mechanism the spec
  already uses to pin these invariants ("pinned by Go guardrails around the
  corpus export path and the daemon method registry", `spec.md:1247-1248`).

- **No state transition that fails when the consumer is absent.** The warm tier
  is "strictly an augmentation … never a runtime dependency" (RFC Goal 4 `:53-54`;
  Non-Goals `:62-63`: "If the adjunct is absent, everything works (a thinner
  shelf, never a wedge)"). Scaffold injection is **fail-soft to an empty shelf**
  (RFC D3 `:96-99`), and the acceptance bar requires that "with the warm tier
  absent, the full suite (including state transitions) is green" plus a guardrail
  proving "no external-consumer import leaked into the daemon" (RFC Acceptance
  `:122-126`). This matches the spec's augmentation-boundary framing exactly:
  the export "is an **augmentation boundary**, not a runtime dependency"
  (`spec.md:1234`), and the runner "does not call retrieval during state
  transitions" (`spec.md:63-65`).

### 2. The hot-tier read runs at scaffold time only

The `RecallMemory(repository_id, query, limit)` projection is a **read-only**
Postgres full-text rank over the *daemon's own* immutable artifact stream
(`websearch_to_tsquery` + `ts_rank` + recency/provenance weight, "no pgvector,
no ML in the daemon"), seated in `go/pkg/reads/` (RFC D3 `:88-93`).

Crucially, it is invoked **only at scaffold time** — at `HandleWorktreeCreate`
(`go/pkg/mutations/worktree.go`) / `buildPacket` (`go/pkg/mutations/claim.go`),
rendering `.striatum/memory/relevant.md` into the new worktree from the daemon's
own PG (RFC D3 `:94-99`). The RFC states plainly: "This is **not** a state
transition and **not** a `memory.*` capability; it is a plain SQL read + file
write during scaffold, fail-soft to an empty shelf" (RFC `:97-99`), and again in
Non-Goals: the hot-tier read "is **not** a retrieval call inside any state
transition (it runs at scaffold time only)" (RFC `:60-61`). It therefore never
sits on the `ack` / `publish-artifact` / `complete` / `verdict` / recovery /
`run prepare` / `run start` / `corpus export` path that invariant (3) protects,
and it reads only striatum-owned PG — not the warm tier — so an absent adjunct
cannot wedge a scaffold. The digest carries an "honest provenance header" (RFC
D3 `:99`, Acceptance `:124`).

### 3. The durable-provenance boundary is preserved

The spec's durable-provenance rule is unambiguous: "Repository artifacts are
durable provenance only" (`spec.md:37`), and PTY logs are "operational scratch,
not transcript provenance … private diagnostics" that are never published,
exported, or treated as workflow truth (`spec.md:2447-2454`).

RFC 0119 keeps this intact and makes it sharper via D1's three axes
(RFC `:69-74`): **canonical authority** is never the warm tier; **index scope**
covers everything (durable provenance included); **eviction scope** touches
*only* run exhaust + unsynthesized intermediates. The RFC is explicit that this
"*preserves* 'repository files are durable provenance' — only the noisy exhaust
is evicted" (RFC `:74`), and that "**durable provenance stays canonical in git**"
(RFC Goal 3 `:52`). The eviction list is bounded (progress_note,
operator_report, `*_ledger`, unsynthesized design candidates) and follows the
existing RFC 0072 PG/blobs pattern (RFC Open questions `:137-141`). Operator-AI
logs stay **out of striatum's boundary** entirely — they live in the Claude Code
transcript store and are ingested by the warm tier directly, requiring no
striatum change (RFC D2 `:82-84`). The acceptance bar restates the guarantee:
the durable-provenance invariant is "preserved; only exhaust is
eviction-eligible" (RFC Acceptance `:127-128`). Feedstock export (the optional,
redacted `lane_trajectory` class) remains pull-only and deterministic, never
streamed (RFC D2 `:77-86`), consistent with "Striatum does not stream runtime
events to any external consumer" (`spec.md:1228-1230`).

## Proposed decision-log entry D178 (one paragraph)

> **D178 — Warm-tier memory boundary and striatum-native hot tier (RFC 0119
> accepted).** Striatum authorizes a warm-tier memory adjunct (`hippo`/`fornix`)
> and a read-only striatum-native hot tier without weakening the
> augmentation-boundary invariants. `spec.md` gains a §"Warm-Tier Memory"
> pinning three axes — canonical authority (never the warm tier), index scope
> (everything, durable provenance included), and eviction scope (only run
> exhaust + unsynthesized intermediates leave the git tree). `corpus export`
> may carry an optional, default-off, redacted `lane_trajectory` feedstock
> class; it stays pull-only and deterministic. A read-only `RecallMemory`
> projection (Postgres FTS over the daemon's own artifact stream, no pgvector/ML)
> feeds a scaffold-time digest into new worktrees and the operator bootstrap
> packet, fail-soft to an empty shelf and carrying an honest provenance header.
> The read is named `recall.*` (capability `read`, `single_repo`), **never**
> `memory.*`, and runs at scaffold time only — never inside a state transition.
> The three corpus invariants are retained and generalized to any external
> memory consumer, and pinned by Go guardrails asserting (a) no external-consumer
> import in daemon source, (b) no `memory.*` in the method registry, and (c) all
> state transitions succeed with the warm tier absent. Durable provenance stays
> canonical in git; only run exhaust is eviction-eligible.

## Why the gate should clear

Every line the RFC touches in `spec.md` it touches *additively*: it adds a
§"Warm-Tier Memory", an optional default-off export class, a read-only
projection, and a scaffold-time file write — each fenced by a guardrail test
that fails closed. No existing invariant is relaxed; invariant (1) is
strengthened in scope. The proposal's own acceptance criteria (RFC `:119-128`)
are precisely the falsification surface: green guardrails with the warm tier
absent, proof of no `memory.*` and no consumer import, and a preserved
durable-provenance invariant. Falsifiers should attack those three claims
directly — corpus-invariant preservation, scaffold-time-only confinement, and
the eviction/durable-provenance split — and this case is built to survive that
challenge.
