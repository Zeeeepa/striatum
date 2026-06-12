# RFC 0119: Warm-Tier Memory Boundary and Striatum Hot Tier

Status: proposed
Date: 2026-06-11
author: rfc-author-claude-opus-4.8-001
Context: RFC 0044/0057 (corpus export + augmentation boundary), spec.md
§ "Corpus Export And Augmentation Boundary", the `hippo` warm-tier adjunct
(separate repo). Companion: hippo RFC 0001.

## Problem

striatum produces rich **workflow exhaust** — artifacts (decision/finding/
synthesis/ledgers/reports), lane agent-loop trajectories, and the orchestrating
operator AI's reasoning — but none of it is queryable across models or sessions.
What one lane learns is lost to the next. The exhaust either clutters the git
tree or evaporates (PTY scratch, operator transcripts). Operators rediscover the
same history; cross-model learning never compounds.

A warm-tier memory service (`hippo`/`fornix`, a separate local-first adjunct)
can turn that exhaust into queryable cross-model memory. But standing it up
brushes against two lines in `spec.md`:

1. The **Corpus Export And Augmentation Boundary** (verbatim invariants below).
2. The rule that **PTY logs are private diagnostics only** and there is **no
   durable transcript capture/export without an explicit product decision**.

This RFC authorizes the minimum boundary change to enable the warm tier and a
striatum-native **hot tier**, while *preserving* the durable-provenance model.

### The invariants this RFC must not break (verbatim, spec.md)

```
- No `import engram` or `from engram` in Striatum source.
- No `memory.*` capability in the Striatum daemon method registry.
- No state transition (`ack`, `publish-artifact`, `complete`, `verdict`,
  recovery, `run prepare`, `run start`, `corpus export`) that fails when
  an external memory consumer is missing, unreachable, or misconfigured.
```

(Generalize the first to "no `import` of any external memory consumer,"
including the warm-tier adjunct.)

## Goals

1. Authorize **lane-trajectory** and **operator-AI-log** capture as feedstocks,
   exported (redacted) via the pull-only `corpus export` path — never streamed.
2. Authorize a **read-only striatum-native hot tier**: a `RecallMemory`
   projection (Postgres full-text over artifacts — none exists today) and a
   **scaffold-time** digest injection into a lane worktree / bootstrap packet.
3. Authorize an **eviction policy**: run exhaust + unsynthesized intermediates
   may stop being committed to the git tree (moved to PG/blobs, RFC 0072
   pattern); **durable provenance stays canonical in git**.
4. Keep the warm tier strictly an **augmentation** reached only via delivered
   inert content (the `fornix` boundary).

## Non-Goals

- A `memory.*` capability in the daemon registry. The hot tier read is named
  `recall.*`/a read on existing surfaces, not `memory.*`, and it is **not** a
  retrieval call inside any state transition (it runs at scaffold time only).
- Any runtime dependency on the warm tier. If the adjunct is absent, everything
  works (a thinner shelf, never a wedge).
- Making the warm tier authoritative. Canonical authority is unchanged: git for
  durable provenance, PG/blobs for the rest, the Claude Code transcript store
  for operator logs.

## Design

### D1 — Three axes (resolve the durable-provenance tension)

Pin in `spec.md`: **canonical authority** (never the warm tier) · **index scope**
(everything, durable provenance included) · **eviction scope** (only run exhaust
+ unsynthesized intermediates leave the git tree). This *preserves* "repository
files are durable provenance" — only the noisy exhaust is evicted.

### D2 — Feedstock export classes

Extend `corpus export` with two optional, redacted classes:

- `lane_trajectory` — agent-loop transcripts (tmux capture / `conversation_
  trajectories`), redacted before they enter the bundle.
- (operator-AI logs are **out of striatum's boundary** — they live in the Claude
  Code transcript store and are ingested by the warm tier directly, not via
  striatum. No striatum change needed; documented here for completeness.)

Both remain pull-only and deterministic; an absent consumer changes nothing.

### D3 — Hot tier: `RecallMemory` + scaffold injection

- A **read-only** projection `RecallMemory(repository_id, query, limit)` over
  artifacts, ranked by Postgres full-text (`websearch_to_tsquery` + `ts_rank`)
  + recency + provenance weight. No pgvector, no ML in the daemon. Seam:
  `go/pkg/reads/` (new `memory.go`), feeding from the immutable artifact stream.
- A **scaffold-time** digest pass: at `HandleWorktreeCreate`
  (`go/pkg/mutations/worktree.go`) / `buildPacket` (`go/pkg/mutations/claim.go`),
  render `.striatum/memory/relevant.md` into the new worktree from the daemon's
  *own* PG. This is **not** a state transition and **not** a `memory.*`
  capability; it is a plain SQL read + file write during scaffold, fail-soft to
  an empty shelf. Each digest carries an honest provenance header.

### D4 — Naming and registry

The read is `recall.*` (or a read on an existing listing surface), **not**
`memory.*`. Update `contracts/daemon_methods.json`, regenerate routes
(`go generate ./pkg/cli/routes/ ./pkg/rpc/`), add the
`docs/reference/command-authority-matrix.md` row (capability `read`,
`single_repo`), and a guardrail test asserting: absent warm tier ⇒ all state
transitions still succeed; and no `memory.*` ever enters the registry.

## Work breakdown

1. Spec amendment: add §"Warm-Tier Memory" with D1–D4; keep the three corpus
   invariants, generalized to any external consumer.
2. `lane_trajectory` export class (redacted) behind a default-off flag.
3. `RecallMemory` read + FTS index migration + tests.
4. Scaffold injection at worktree create + honest-header digest renderer + tests.
5. Authority-matrix row + guardrail tests (fail-soft + no-`memory.*`).

## Acceptance

- RFC accepted; decision **D178** recorded in `docs/decisions/decision-log.md`.
- The three corpus invariants still hold (Go guardrails green).
- `RecallMemory` + scaffold injection landed; with the warm tier absent, the
  full suite (including state transitions) is green.
- A guardrail test proves no `memory.*` capability and no external-consumer
  import leaked into the daemon.
- Durable-provenance invariant ("repository files are durable provenance")
  preserved; only exhaust is eviction-eligible.

## Issue disposition

- File companion issues for: lane-trajectory redaction, the FTS index migration,
  the eviction policy per-kind table.

## Open questions

- Whether eviction is per-artifact-kind configurable (a manifest policy table)
  or a fixed exhaust list. Default: fixed exhaust list (progress_note,
  operator_report, *_ledger, unsynthesized design candidates); revisit if too
  coarse.
- Whether scaffold injection should also enrich the operator bootstrap packet
  (lower-risk, human-facing) before lane worktrees. Recommended: yes, first.
