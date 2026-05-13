---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0044", "engram", "build", "track_b"]
---

author: reviewer-gemini-pro-001

# Track B Build Review: RFC 0044 (Engram Phase 1)

The RFC 0044 body demonstrates high Engram fluency and rigorous adherence to the augmentation-not-dependency boundary. However, from an adversarial ergonomics perspective, there is a significant conflict between the "content-stable" ID requirement and Engram's immutable storage substrate that will likely block repository evolution.

### Finding 1: [High] IngestConflict on Mutable Artifacts
The RFC proposes "content-stable" `external_id` values (e.g., `rfc:<####>#<heading-slug>`) for artifacts that are inherently mutable (RFCs, decisions, ubiquitous language). Simultaneously, it cites Engram's "immutable after insert" raw evidence triggers (from the synthesis/Engram ground truth).
- **Impact**: Any change to an existing RFC or decision (e.g., a typo fix or status update) will cause an `IngestConflict` during the next `striatum corpus export` + `engram ingest-striatum` cycle because the ID will collide with existing (immutable) content.
- **Recommendation**: The RFC should clarify if Engram Phase 1 supports "superseded" evidence versions (e.g., by including the manifest hash or a version counter in the row-level `external_id`) or if the "idempotent" ingest is intended to skip modified rows (which would leave stale memory).

### Finding 2: [Medium] Ambiguous Filter Schema in `engram.search`
The RFC defines the `engram.search` tool with a `filters?` parameter but fails to specify the supported schema (e.g., `sub_kind`, `rfc`, `decision_id`, `run_id`).
- **Impact**: Operators cannot reliably perform the targeted queries mentioned in the goals (e.g., "what friction patterns recurred across dogfoods 036-039") without knowing the filter keys.
- **Recommendation**: Explicitly define the allowed filter keys in Section 5 to match the synthesis/implementation plan.

### Finding 3: [Low] `reference_id` vs `external_id` Discontinuity
The RFC uses `external_id` for ingestion and `reference_id` for retrieval but does not state if they are interchangeable.
- **Impact**: If `reference_id` is an internal Engram UUID, operators cannot directly fetch a known artifact (like `rfc:0044`) using `engram.fetch_reference` without searching for it first.
- **Recommendation**: Clarify if `reference_id` can accept the content-stable `external_id` for direct citation.

### Finding 4: [Low] Ergonomics of `striatum operator memory check` Exit Code
The RFC specifies that `striatum operator memory check` "exits 0 even when Engram is missing, misconfigured, or unreachable."
- **Impact**: This hides system health issues from automated scripts or CI gates that might want to verify the memory augmentation is active. While intended to be "informational," a 0 exit code traditionally signals health.
- **Recommendation**: Consider exiting non-zero if a check is explicitly requested but the service is unreachable, or provide a `--strict` flag.

### Provenance Honesty and Capability Scope
- **Fluency**: The RFC accurately cites Engram's three-tier separation, `source_kind` enum, and segmentation model.
- **Honesty**: The RFC is transparent about what Phase 1 **cannot** do (no write-side, no transcripts, no hosted retrieval).
- **Scope**: The corpus separation via `corpus_id` and the explicit `memory.*` capability tokens provide robust isolation between Striatum software-building and personal-life data.
