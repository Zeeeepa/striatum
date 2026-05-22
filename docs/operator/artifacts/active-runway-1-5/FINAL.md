---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/plans/active-runway-1-5.md", "docs/operator/artifacts/active-runway-1-5/phase1/SYNTHESIS.md", "docs/operator/artifacts/active-runway-1-5/phase2/CLI_CUTOVER.md", "docs/operator/artifacts/active-runway-1-5/phase3/TMUX_METADATA_SLICE.md", "docs/operator/artifacts/active-runway-1-5/phase4/SYNTHESIS.md", "docs/operator/artifacts/active-runway-1-5/phase5/todo61/PLAN.md", "docs/operator/artifacts/active-runway-1-5/phase5/todo62/PLAN.md", "docs/operator/artifacts/active-runway-1-5/phase5/todo63/PLAN.md"]
---

# Active Runway 1-5 Final Closure
author: closer-codex-001
status: ready
date: 2026-05-22

## Closure

The active runway has produced the bounded planning artifacts needed to move
from current operator state into implementation. It did not add product policy
beyond D124-D129, and it preserves the standing boundaries: no hosted services,
no telemetry, no transcript capture, no live SQLite authority, no Engram/runtime
memory dependency, and no CLI workflow-control retirement before MCP/UI parity
is tested.

## Phases Driven

1. **Phase 1, TODO 55/56/59/60 implementation follow-ups.** Four planning
   packets were produced for daemon-owned accepted-risk persistence, dry-run
   auto-finalize hardening, Corpus Contract V2, and read-only local Git
   snapshots. The phase synthesis orders the first implementation work around a
   low-risk daemon read plus auto-finalize cause classes, then stateful risk
   acceptance, auto-finalize visibility/circuit breaker, and Corpus V2.
2. **Phase 2, CLI retirement and parity map.** The cutover ledger classifies
   CLI verbs as bootstrap, diagnostics, local file authoring, lifecycle,
   parity-backed, fixture-only, or retired compatibility refusals. It identifies
   the UI parity gaps that must land before operator docs and skills can hide
   workflow-control CLI verbs.
3. **Phase 3, RFC 0075 tmux metadata slice.** The tmux plan scopes passive
   metadata and attach-command visibility for live interactive sessions. It
   keeps pane text observational only and leaves protocol liveness to RFC 0077,
   which is already accepted and landed.
4. **Phase 4, RFC 0074 Phase A catalog/generator scaffold.** The catalog and
   example plans converge on a metadata-plus-one-example patch sequence:
   graph-shape metadata, role packs, adversary packs, read-only discovery
   surfaces, one implementation-panel example, tests, and docs. RFC 0076
   `code_doc_audit` is included as catalog metadata, with no new audit schema
   or issue queue.
5. **Phase 5, TODO 61/62/63 residual cleanup.** Three cleanup plans bound the
   next legacy/PG/client-boundary work: prune skipped legacy SQLite fixture
   tests, replace stale literal encodings with repo-policy helpers without
   changing refusal wording, and reduce direct-PG client imports through
   daemon-routed surfaces.

## Ordered Implementation Batches

1. **Batch A1: TODO 60 read-only `git.snapshot`.** Add one daemon read method,
   Go handler, MCP exposure, daemon-routed CLI read, authority matrix updates,
   generated method tables, and no-mutation/no-network tests. This is the first
   contract slice because it has no migration and no workflow mutation.
2. **Batch A2: TODO 56 skipped-candidate cause classes.** Add stable
   auto-finalize cause names while preserving existing reason strings. This can
   land beside A1 because it does not touch daemon method contracts.
3. **Batch B: TODO 55 accepted-risk daemon authority.** Add daemon-owned
   accepted-risk state, lint/list/accept handlers, snapshot/fingerprint
   binding, admin-capability checks, MCP/CLI routing, matrix updates, and
   duplicate/fingerprint tests. Serialize this after A1 because both touch the
   daemon contract and MCP discovery surfaces.
4. **Batch C1: TODO 56 lane-finalization visibility.** Project
   `lane_finalization` through status, dashboard, and web recovery views after
   skipped causes are stable.
5. **Batch C2: TODO 56 circuit breaker.** Add durable consecutive-failure
   breaker state, reset behavior, restart persistence, and events after the
   cause vocabulary and visibility projection are pinned.
6. **Batch D: TODO 59 Corpus Contract V2.** Land V2 manifest identity,
   graduated redaction, hybrid archive defaults, deep-chain verification, V1
   verification compatibility, and augmentation-by-reference guardrails.
   Integrate optional Git snapshot hashes only after `git.snapshot` has a
   stable response envelope.
7. **Batch E: CLI retirement UI/MCP parity.** Keep CLI verbs available while
   adding reviewer console, checkpoint/escalation UI, recovery panel, decision
   recorder, export buttons, cross-repo console, branch-confirm flag parity,
   prepare-only flow, liveness rendering, and parity tests. Hide verbs from
   operator docs only after those tests pass.
8. **Batch F: RFC 0075 tmux metadata.** Add live-interactive tmux metadata and
   attach-command projection through supervisor status/dashboard surfaces,
   without parsing or publishing pane contents.
9. **Batch G: RFC 0074 Phase A.** Add catalog loader support for role/adversary
   packs, metadata entries, read-only CLI/web discovery, the
   `implementation-panel-flow` example, tests, and docs. Defer generator shape
   implementation and web chooser pack selection to Phase B.
10. **Batch H: TODO 61/62/63 cleanup.** Run the three bounded cleanup tracks:
    legacy skipped-test pruning, PG-only literal cleanup, then daemon
    client/service boundary reduction. Keep historical SQLite fixtures and
    operator-facing refusal diagnostics intact.

## Parallelism And Serialization

The first safe parallel pair is **Batch A1** and **Batch A2**. A1 owns daemon
read contracts and Git snapshot tests; A2 owns auto-finalize refusal vocabulary
and focused recovery tests.

RFC 0074 example scaffolding can proceed in parallel with cleanup batches if
write scopes are kept to `examples/implementation-panel-flow/`, catalog tests,
and docs. TODO 61 refusal-fixture, view-fixture, and detail-panel-fixture lanes
can also run in parallel, followed by a short guardrail update pass.

Serialize all changes touching:

- `contracts/daemon_methods.json`;
- generated Go registry and daemon method tables;
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`;
- MCP `tools/list` / `tools/call` visibility tests;
- CLI daemon route maps;
- PostgreSQL migration registries and migration numbers;
- auto-finalize cause names once Batch A2 lands;
- workflow catalog loader/schema behavior.

TODO 55 and TODO 56 circuit-breaker migrations must receive migration numbers
serially. TODO 55 and TODO 60 must not edit daemon method contracts in parallel.
Corpus V2 should wait for `git.snapshot` if it wants to reference a snapshot
hash.

## Validation Evidence

This workflow produced the required phase artifacts under
`docs/operator/artifacts/active-runway-1-5/`:

- Phase 1: four TODO implementation plans plus `phase1/SYNTHESIS.md`;
- Phase 2: `phase2/CLI_CUTOVER.md`;
- Phase 3: `phase3/TMUX_METADATA_SLICE.md`;
- Phase 4: catalog plan, example plan, and `phase4/SYNTHESIS.md`;
- Phase 5: TODO 61, TODO 62, and TODO 63 cleanup plans.

Validation run during closure:

```bash
PYTHONPATH=src python3 -m striatum.cli workflow validate --allow-same-model-pairing docs/operator/workflows/active-runway-1-5/workflow.json
```

Result: `valid: true`, `workflow_id: active-runway-1-5`.

Run-state evidence from `striatum status --run-id
run_b2e013582e0aeba267dd7a47cc66ccf1 --json` during closure showed 13
completed jobs, this finalizer as the only running job, no claimable jobs, no
open blockers, no human checkpoints, and a dry-run auto-finalize projection
with one candidate. The closer session heartbeat succeeded and extended the
lease before publication.

## Final Recommendation

Start implementation with Batches A1 and A2 in parallel. After they land, use
the shared serialization points to sequence the stateful daemon-contract and
migration work. Treat CLI retirement as a parity-and-tests program, not a
deletion program, and keep the catalog work metadata-first until the Phase A
example and discovery surfaces validate cleanly.
