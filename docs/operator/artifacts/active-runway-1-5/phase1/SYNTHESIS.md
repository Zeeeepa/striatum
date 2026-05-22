---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/active-runway-1-5/phase1/todo55/PLAN.md", "docs/operator/artifacts/active-runway-1-5/phase1/todo56/PLAN.md", "docs/operator/artifacts/active-runway-1-5/phase1/todo59/PLAN.md", "docs/operator/artifacts/active-runway-1-5/phase1/todo60/PLAN.md"]
---

# Phase 1 TODO 55/56/59/60 Implementation Synthesis
author: coordinator-codex-001
status: ready
date: 2026-05-22

## Decision Boundary

D124-D127 unblock implementation but do not relax product boundaries:

- TODO 55 must store accepted workflow-lint risk as daemon-owned accepted-risk
  state, decision-linked and bound to an immutable snapshot or workflow
  fingerprint. Workflow-file metadata is not authority.
- TODO 56 must keep global auto-finalize dry-run by default. Live mode remains
  workflow opt-in or explicit operator force, and default-on remains gated by
  dogfood evidence.
- TODO 59 must implement Corpus Contract V2 without making augmentation a
  runtime dependency. External memory remains optional agent-side fetch by
  reference.
- TODO 60 may only implement read-only local Git snapshots in core. Commits,
  pushes, PR/provider calls, provider SDKs, and hosted identifiers stay out of
  scope.

## Recommended Order

1. **TODO 60 read-only `git.snapshot` daemon read.** This is the safest
   daemon-contract slice: one read method, no migration, no workflow mutation,
   and a sharply bounded local `git` allowlist. It proves the method contract,
   MCP visibility, CLI daemon-routed read, authority matrix, and generated
   registry update path without adding durable state.
2. **TODO 56 Slice 1 skipped-candidate cause classes.** This is disjoint from
   daemon contracts and migrations. It stabilizes auto-finalize refusal
   vocabulary and is a prerequisite for TODO 56 visibility and circuit-breaker
   slices.
3. **TODO 55 accepted-risk daemon authority.** This is the first stateful
   mutation slice. It needs a migration, daemon methods, contract/registry/MCP
   updates, authority matrix updates, lint parity, and admin-capability tests.
   It should start after TODO 60 has shaken out contract generation in a
   read-only method.
4. **TODO 56 Slice 2 then Slice 3.** Lane-finalization visibility can follow
   cause classes after status/dashboard/web projection vocabulary is stable.
   The circuit breaker should land last because it adds durable state and
   depends on stable cause names.
5. **TODO 59 Corpus Contract V2.** Implement after the narrower daemon/read and
   auto-finalize slices unless a release needs V2 earlier. It changes corpus
   manifest/schema/archive/verification semantics and may interact with TODO 60
   through optional `git_snapshot_hash`, so it benefits from a stable
   `git.snapshot` response shape.

## Parallelism

The immediate safe parallel batch is:

- **Batch A1:** TODO 60 first slice.
- **Batch A2:** TODO 56 Slice 1 cause classes.

These can proceed in parallel because their write scopes are naturally
disjoint. TODO 60 owns daemon method contracts, Go read/MCP tests, CLI routing,
and authority docs. TODO 56 Slice 1 owns recovery auto-finalize cause
enumeration, the Python PG auto-finalize handler, and focused recovery tests.

TODO 55 should not run in parallel with TODO 60 because both touch
`contracts/daemon_methods.json`, generated Go registry/table outputs, MCP tool
discovery tests, CLI daemon routing, and
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md`. TODO 59 should not share the
first batch because it can touch corpus/archive contracts broadly and may need
to consume the read-only Git snapshot vocabulary.

## Shared Files To Serialize

Serialize any implementation packet touching these files or surfaces:

- `contracts/daemon_methods.json`
- `go/pkg/rpc/registry_methods.go`
- `docs/architecture/DAEMON_METHOD_TABLES.md`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- daemon method generation scripts and architecture guardrail tests
- MCP `tools/list` / `tools/call` visibility tests
- `src/striatum/cli/parser.py` and `src/striatum/cli/daemon_rpc_route.py`
- migration registries in `go/pkg/db/migrations.go` and
  `src/striatum/daemon_pg/migrations.py`

TODO 55 and TODO 56 Slice 3 both add daemon-owned PostgreSQL state, so their
migration numbers must be assigned serially after the current migration head.
TODO 60 has no migration and can land before them.

## Immediate First Batch

### Batch A1: TODO 60 `git.snapshot`

Implement only the read-only local snapshot:

- add `git.snapshot` to the daemon method contract and generated registry/docs;
- implement a Go read handler using `exec.CommandContext` with closed
  allowlisted local `git -C <registered repo>` commands;
- return branch, HEAD, dirty summary, changed paths, and bounded ancestry;
- add daemon-routed `striatum git snapshot --json`;
- expose it through MCP for read-capable repo-scoped tokens;
- add no-mutation and no-network/provider guardrail tests.

Explicit non-scope: commit, push, fetch, PR/provider APIs, hosted links,
provider SDKs, diff hunks, commit bodies, and remote URL collection.

### Batch A2: TODO 56 Cause Classes

Implement only stable skip causes:

- add `src/striatum/recovery/auto_finalize_causes.py`;
- thread `cause` and optional `cause_detail` through auto-finalize skipped and
  artifact-refusal results;
- keep existing `reason` strings backward-compatible;
- add tests pinning one representative per cause class;
- do not add migrations, new RPC methods, UI changes, live default changes, or
  circuit-breaker state in this batch.

## Follow-Up Batches

- **Batch B:** TODO 55 accepted-risk core. Land migration/table, daemon lint
  and accept-risk/list handlers, contract/MCP/CLI routing, matrix updates,
  duplicate-record tests, fingerprint/snapshot binding tests, and
  admin-capability denials. Defer UI and revocation UX unless they fall out
  cleanly.
- **Batch C:** TODO 56 visibility, then circuit breaker. First add
  `lane_finalization` projection/status/dashboard/web rendering. Then add the
  durable circuit-breaker table, reset flag, events, and restart persistence.
- **Batch D:** TODO 59 Corpus Contract V2. Land V2 constants, manifest
  identity/redaction fields, backward-compatible verify, archive/deep-chain
  verification defaults, optional augmentation references, and no-Engram
  guardrails. Integrate optional Git snapshot hash only after Batch A1 defines
  the stable snapshot envelope.

## Validation Commands

For the immediate first batch:

```bash
make lint
make typecheck
go test ./pkg/reads ./pkg/mcp ./pkg/rpc
pytest tests/architecture tests/recovery/test_auto_finalize_causes.py
pytest tests/daemon_pg/handlers/recovery_evidence
PYTHONPATH=src python3 -m striatum.cli workflow validate --allow-same-model-pairing docs/operator/workflows/active-runway-1-5/workflow.json
```

Before merging any daemon-contract slice, also run the repository's generated
method-table check if the edit changes `contracts/daemon_methods.json`.

For later batches, add migration tests for TODO 55 and TODO 56 Slice 3, UI/web
template tests for TODO 56 Slice 2, and corpus/archive verification tests for
TODO 59.

## Risk Notes

TODO 60 is low mutation risk but high boundary risk: the implementation must
prove it never invokes mutating Git, network, provider CLI, or hosted-provider
commands. TODO 56 Slice 1 is low architectural risk and should land early
because it reduces ambiguity for later recovery work. TODO 55 is the highest
authority-risk item because it creates new accepted-risk state that can affect
workflow lint outcomes. TODO 59 is broadest in schema and compatibility impact;
it should be kept as a separate batch with explicit V1 verification
compatibility and augmentation-not-dependency tests.
