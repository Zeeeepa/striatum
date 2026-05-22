---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Plan: Corpus Contract V2 Implementation
author: corpus-planner-gemini-001

This plan outlines the implementation of Corpus Contract V2, focusing on the decisions accepted in D126. The goal is to provide a stable, multi-corpus identity, graduated redaction, and offline-first verification while maintaining the augmentation-not-dependency invariant.

## 1. Schema and Manifest Evolution (V2)

The `manifest.json` for corpus bundles and run archives will graduate to version 2, introducing fields for identity and privacy.

### Manifest V2 Fields
- `corpus_contract_version`: `2`
- `corpus_id`: A composite identity string `{slug}:{sha256_of_origin}`. The hash ensures stability across repository moves.
- `redaction_tier`: `graduated` (defaulting to `public`).
- `augmentation_policy`:
  - `workflow_opt_in`: `true` (enables agent-side fetch by reference).
  - `budget_per_packet_lines`: Default `100`.
- `verification_depth`: `deep_chain` (requires re-computing every event hash).
- `git_snapshot_hash`: (Optional) SHA-256 of the local git HEAD at export time.

### Backward Compatibility
- V2 `verify` commands MUST support V1 bundles.
- If `corpus_contract_version` is missing, it is treated as `1`.
- V1 manifests lack `corpus_id` and `redaction_tier`; they use the historical `SOURCE_KIND` defaults.

## 2. Graduated Redaction Tiers

V2 introduces three named tiers to replace the binary V1 policy:

1. **Public**: Full redaction of all operator prose and sensitive fields. Suitable for public sharing.
2. **Curated**: Allows inclusion of specific artifact kinds (e.g., `decision`, `synthesis`) if they do not contain denied patterns.
3. **Internal**: Preserves structural metadata and non-sensitive operator links for private replay and audit.

The `redaction.py` logic will be refactored to accept a `RedactionTier` parameter.

## 3. Workflow Augmentation by Reference

To support external memory (Engram) without creating a runtime dependency:

- **Work Packets**: Include an `augmentation_references` list of hashes/IDs.
- **Agent Fetch**: Agents may use a new `augmentation.fetch` tool.
- **Fall-back**: If the augmentation service is unavailable, Striatum returns an empty success response or uses a local cache. No part of the workflow should block on memory availability.

## 4. Hybrid Archive and Deep Verification

Archives graduate from simple snapshots to "Hybrid" bundles:

- **Snapshot**: Materialized state of `runs`, `jobs`, `artifacts`, etc.
- **Event Log**: The complete `events` stream for that run.
- **Deep-Chain Verification**:
  - `striatum archive verify --replay` will be the default.
  - Verification must re-calculate the `row_hash` for every event using the `previous_hash` chain.
  - Optional: Cross-check against the daemon's `audit_chain_head` if connected.

## 5. Technical Parity (Python/Go)

The Go daemon must achieve bit-for-bit parity with Python for:
- `corpus export` JSONL generation.
- `archive create` manifest and file hashing.
- Path normalization and timezone handling.

## 6. Verification and Guardrails

- **No-Engram Test**: Extend `tests/test_cli_corpus_export.py` to ensure V2 integration doesn't leak Engram imports into the core.
- **Validation Refusals**: `striatum corpus verify` must refuse bundles where the internal `bundle_sha256` does not match the canonical hash of the manifest.

## 7. First Implementation Slice

1. **Types**: Update `striatum.corpus.types` with V2 constants.
2. **Manifest**: Enhance `striatum.corpus.manifest` to accept V2 fields.
3. **Exporter**: Update `striatum corpus export` to emit the composite `corpus_id`.
4. **Tests**: Add a test verifying a V2 bundle export and its backward-compatible verification.
